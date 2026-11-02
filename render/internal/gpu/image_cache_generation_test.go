//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	intImage "github.com/energye/gpui/render/internal/image"
)

// TestImageCache_RetiredGenerationReclaimsEntry pins the batch-1c contract on
// the cache side: a buffer that rewrites its pixels advances its generation, the
// watcher reports the retired one, and the render-path drain takes that texture
// out of the budget — so playback never accumulates one live entry per frame.
func TestImageCache_RetiredGenerationReclaimsEntry(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	cmd := func() *ImageDrawCommand {
		gen := img.GenerationID()
		return &ImageDrawCommand{
			PixelData:    img.PremultipliedData(),
			GenerationID: gen,
			Src:          img,
			SrcGen:       gen,
			ImgWidth:     16,
			ImgHeight:    16,
			ImgStride:    64,
		}
	}

	img.Fill(10, 10, 10, 255)
	if _, err := ic.GetOrUpload(cmd()); err != nil {
		t.Fatal(err)
	}
	st1 := ic.Stats()
	if st1.Entries != 1 || st1.Watched != 1 {
		t.Fatalf("after first upload: entries=%d watched=%d, want 1/1", st1.Entries, st1.Watched)
	}

	// Rewrite the same buffer: the generation moves, the old texture must go.
	const frames = 8
	used := st1.UsedBytes
	for i := 0; i < frames; i++ {
		img.Fill(byte(20+i*20), 10, 10, 255)
		img.NotifyPixelsChanged()
		if _, err := ic.GetOrUpload(cmd()); err != nil {
			t.Fatal(err)
		}
	}
	st2 := ic.Stats()
	if st2.Entries != 1 {
		t.Fatalf("entries after %d frame rewrites = %d, want 1 (retirement is not reclaiming)", frames, st2.Entries)
	}
	if st2.UsedBytes != used {
		t.Fatalf("usedBytes %d -> %d on same-size rewrite", used, st2.UsedBytes)
	}
	if st2.RetiredGens < frames {
		t.Fatalf("retired generations = %d, want >= %d", st2.RetiredGens, frames)
	}
	if st2.Watched != 1 {
		t.Fatalf("watched sources = %d, want 1 (retired entries must drop their watcher)", st2.Watched)
	}

	// Dispose retires the last generation as well: no zombie registration, no
	// budget left holding the texture.
	img.Dispose()
	ic.drainRetired()
	st3 := ic.Stats()
	if st3.Entries != 0 || st3.UsedBytes != 0 || st3.Watched != 0 {
		t.Fatalf("after dispose+drain: entries=%d usedBytes=%d watched=%d, want 0/0/0",
			st3.Entries, st3.UsedBytes, st3.Watched)
	}
}

// P4：同一像素源被两个消费者（两个 session / 窗口+离屏 target）绘制时，各家
// 必须各自重传。旧的一次性脏旗标下第二家 uploads 停在 1，画面冻在第一帧。
func TestImageCache_TwoConsumersBothReupload(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	img, err := intImage.NewImageBuf(32, 32, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()

	a := NewImageCache(device, queue)
	b := NewImageCache(device, queue)
	const frames = 6
	for i := 0; i < frames; i++ {
		for px := range img.Data() {
			img.Data()[px] = byte(i * 7)
		}
		img.NotifyPixelsChanged()
		for _, c := range []*ImageCache{a, b} {
			gen := img.GenerationID()
			if _, err := c.GetOrUpload(&ImageDrawCommand{
				PixelData: img.PremultipliedData(), GenerationID: gen, Src: img, SrcGen: gen,
				ImgWidth: 32, ImgHeight: 32, ImgStride: 128,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	sa, sb := a.Stats(), b.Stats()
	if sa.Uploads != frames || sb.Uploads != frames {
		t.Fatalf("uploads A=%d B=%d, want %d/%d（第二家少传即冻结在旧帧）", sa.Uploads, sb.Uploads, frames, frames)
	}
	if sa.Entries != 1 || sb.Entries != 1 {
		t.Fatalf("entries A=%d B=%d, want 1/1", sa.Entries, sb.Entries)
	}
}

// P1：两族像素源必须共用一个号空间。旧状各有一个从 1 起的计数器，实测 200 个号
// 交集 200，同尺寸异来源直接命中同一张纹理，第二个来源的像素永远上不了屏。
func TestImageCache_KeySpaceDisjointAcrossSources(t *testing.T) {
	const n = 200
	buf := make(map[uint64]bool, n)
	for i := 0; i < n; i++ {
		b, err := intImage.NewImageBuf(8, 8, intImage.FormatRGBAPremul)
		if err != nil {
			t.Fatal(err)
		}
		buf[b.GenerationID()] = true
		b.Dispose()
	}
	pm := make(map[uint64]bool, n)
	for i := 0; i < n; i++ {
		pm[render.NewPixmap(8, 8).GenerationID()] = true
	}
	overlap := 0
	for k := range pm {
		if buf[k] {
			overlap++
		}
	}
	if overlap != 0 {
		t.Fatalf("ImageBuf 与 Pixmap 图号交集=%d，want 0（撞号会让一个来源画出另一个来源的纹理）", overlap)
	}
}

// P7：批合并谓词按图号分家。换号之后「同一次提交内先画、推进一帧、再画」自然
// 落进两个绑定组，不需要额外的脏旗标比较（旧模型下 clean 组长会吞掉 dirty 组员）。
func TestImagePipeline_DifferentGenerationNeverMerges(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	_ = device
	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	before := p36CmdOf(img)
	img.Fill(200, 10, 10, 255)
	img.NotifyPixelsChanged()
	after := p36CmdOf(img)
	if before.GenerationID == after.GenerationID {
		t.Fatal("原地改写未换号")
	}
	if canMergeImageDraw(before, after) {
		t.Fatal("两个内容版本被并进同一绑定组：后一帧不会上传")
	}
	if !canMergeImageDraw(after, p36CmdOf(img)) {
		t.Fatal("同代际同几何应仍可合批（否则粒子/图集批绘退化为逐张提交）")
	}
	_ = queue
}

func p36CmdOf(img *intImage.ImageBuf) *ImageDrawCommand {
	gen := img.GenerationID()
	return &ImageDrawCommand{
		PixelData:    img.PremultipliedData(),
		GenerationID: gen,
		Src:          img,
		SrcGen:       gen,
		ImgWidth:     img.Width(),
		ImgHeight:    img.Height(),
		ImgStride:    img.Width() * 4,
	}
}

// P5：派生键（brush 填充把源号 XOR 成命名空间号、图样按 tile 序号偏移）也必须
// 跟着源号退役——缓存记的是「像素来自哪个源号」，不是命令自己那个键。
func TestImageCache_DerivedKeyReleasesWithSource(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	img, err := intImage.NewImageBuf(16, 16, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	img.Fill(20, 20, 20, 255)

	derived := func() *ImageDrawCommand {
		gen := img.GenerationID()
		return &ImageDrawCommand{
			PixelData:    img.PremultipliedData(),
			GenerationID: gen ^ 0xC0A7C01A00000001, // brush_fill 的命名空间派生形态
			Src:          img,
			SrcGen:       gen,
			ImgWidth:     16, ImgHeight: 16, ImgStride: 64,
		}
	}
	if _, err := ic.GetOrUpload(derived()); err != nil {
		t.Fatal(err)
	}
	if st := ic.Stats(); st.Entries != 1 {
		t.Fatalf("entries=%d, want 1", st.Entries)
	}
	img.Fill(200, 30, 30, 255)
	img.NotifyPixelsChanged()
	if _, err := ic.GetOrUpload(derived()); err != nil {
		t.Fatal(err)
	}
	st := ic.Stats()
	if st.Entries != 1 {
		t.Fatalf("派生键未随源号退役：entries=%d, want 1", st.Entries)
	}
	if st.Watched != 1 {
		t.Fatalf("watched=%d, want 1", st.Watched)
	}
}

// P8：单一分配器单调且不复用——退役的号再也取不到条目，也就永远不会被误命中。
func TestImageGenerationNeverReused(t *testing.T) {
	img, err := intImage.NewImageBuf(4, 4, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	defer img.Dispose()
	seen := []uint64{}
	for i := 0; i < 50; i++ {
		img.NotifyPixelsChanged()
		seen = append(seen, img.GenerationID())
	}
	set := map[uint64]bool{}
	for i, g := range seen {
		if g == 0 {
			t.Fatalf("第 %d 次改写拿到 0 号", i)
		}
		if set[g] {
			t.Fatalf("图号 %d 被重复使用", g)
		}
		set[g] = true
		if i > 0 && g <= seen[i-1] {
			t.Fatalf("图号未单调递增：%d -> %d", seen[i-1], g)
		}
	}
}
