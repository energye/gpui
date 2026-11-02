//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	intImage "github.com/energye/gpui/render/internal/image"
)

// mipNoise fills a buffer with per-texel noise: the case where mip prefiltering
// and single-level sampling provably differ, and where a silently unwritten mip
// level would show up as a wrong value rather than a plausible one.
func mipNoise(t *testing.T, w, h int) *intImage.ImageBuf {
	t.Helper()
	img, err := intImage.NewImageBuf(w, h, intImage.FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	d, st := img.Data(), img.Stride()
	seed := uint32(0xC0FFEE ^ uint32(w*131+h*17))
	for y := 0; y < h; y++ {
		for x := 0; x < w*4; x++ {
			seed = seed*1664525 + 1013904223
			d[y*st+x] = byte(seed >> 23)
		}
	}
	img.InvalidatePremulCache()
	img.NotifyPixelsChanged()
	return img
}

func mipCmd(img *intImage.ImageBuf, mip render.MipmapMode) *ImageDrawCommand {
	gen := img.GenerationID()
	w, h := img.Width(), img.Height()
	return &ImageDrawCommand{
		PixelData: img.PremultipliedData(), GenerationID: gen, Src: img, SrcGen: gen,
		Mip: mip, ImgWidth: w, ImgHeight: h, ImgStride: w * 4,
	}
}

// TestImageCache_MipLevelsAndExactBytes pins D12/D13: the created level count is
// the shared hal formula, and the bytes the cache budgets are exactly what the
// VRAM ledger charges for that texture — level-0-only counting was the hole.
func TestImageCache_MipLevelsAndExactBytes(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	sizes := [][2]int{{64, 64}, {70, 8}, {16, 3}, {1, 64}, {33, 17}}
	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		img := mipNoise(t, w, h)
		want := hal.MipLevelCount(uint32(w), uint32(h))
		gen := img.GenerationID()

		if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapNone)); err != nil {
			t.Fatalf("%dx%d none: %v", w, h, err)
		}
		noneEntry := ic.entries[imageCacheKey{gen, 1}]
		if noneEntry == nil {
			t.Fatalf("%dx%d: MipmapNone must create a single-level entry", w, h)
		}
		if noneEntry.bytes != int64(w)*int64(h)*4 {
			t.Fatalf("%dx%d: None entry bytes=%d, want level-0 only %d", w, h, noneEntry.bytes, w*h*4)
		}

		if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapLinear)); err != nil {
			t.Fatalf("%dx%d linear: %v", w, h, err)
		}
		mipEntry := ic.entries[imageCacheKey{gen, want}]
		if mipEntry == nil {
			t.Fatalf("%dx%d: no %d-level entry for MipmapLinear", w, h, want)
		}
		// The number the ledger itself charges, through the same helper the
		// backends call when they account the texture.
		ledger := int64(hal.VramTextureBytes(types.Extent3D{
			Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1,
		}, 1, want, 1, types.TextureFormatRGBA8Unorm))
		if mipEntry.bytes != ledger {
			t.Fatalf("%dx%d levels=%d: 缓存预算=%d 显存账=%d 必须相等（否则 128 条/64 MiB 门判的是没花过的数）",
				w, h, want, mipEntry.bytes, ledger)
		}
		if mipEntry.levels != want {
			t.Fatalf("%dx%d: entry.levels=%d want %d", w, h, mipEntry.levels, want)
		}
		// Every deeper level must actually have been written: an unwritten level
		// is what a sampler would read as plausible-looking garbage.
		if got := ic.Stats().MipLevelUploads; got < uint64(want-1) {
			t.Fatalf("%dx%d: 深层上传=%d，至少应为 %d", w, h, got, want-1)
		}
		img.Dispose()
		ic.drainRetired()
	}
}

// TestImageCache_MipModeSplitsEntries pins D13: a texture's mip chain is fixed at
// creation, so two modes of one picture are two entries — and each is then
// reused rather than rebuilt every frame.
func TestImageCache_MipModeSplitsEntries(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	img := mipNoise(t, 64, 64)
	defer img.Dispose()

	if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapNone)); err != nil {
		t.Fatal(err)
	}
	if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapLinear)); err != nil {
		t.Fatal(err)
	}
	st := ic.Stats()
	if st.Entries != 2 {
		t.Fatalf("entries=%d, want 2（两档共用一条必然后一档静默丢掉 mip 或白建链）", st.Entries)
	}
	if st.MipEntries != 1 {
		t.Fatalf("mipEntries=%d, want 1", st.MipEntries)
	}
	missBefore := st.Misses
	for i := 0; i < 5; i++ {
		if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapLinear)); err != nil {
			t.Fatal(err)
		}
		if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapNone)); err != nil {
			t.Fatal(err)
		}
	}
	if got := ic.Stats().Misses; got != missBefore {
		t.Fatalf("两档各自稳定后 misses %d -> %d（应全命中）", missBefore, got)
	}
}

// TestImageCache_OneChainPerGeneration pins the cache-side idempotence: repeated
// draws of the same content generate exactly one chain, and it is freed when the
// content moves on.
func TestImageCache_OneChainPerGeneration(t *testing.T) {
	intImage.MipChainFlush()
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()
	ic := NewImageCache(device, queue)
	defer ic.Destroy()

	img := mipNoise(t, 128, 128)
	defer img.Dispose()
	base := intImage.MipChainCacheStats().Builds
	for i := 0; i < 6; i++ {
		if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapNearest)); err != nil {
			t.Fatal(err)
		}
	}
	if d := intImage.MipChainCacheStats().Builds - base; d != 1 {
		t.Fatalf("同一代际 6 次绘制建了 %d 条链，want 1", d)
	}

	img.Fill(9, 9, 9, 255)
	img.NotifyPixelsChanged()
	if _, err := ic.GetOrUpload(mipCmd(img, render.MipmapNearest)); err != nil {
		t.Fatal(err)
	}
	st := intImage.MipChainCacheStats()
	if st.Builds-base != 2 {
		t.Fatalf("换代后建链数=+%d, want +2", st.Builds-base)
	}
	if st.Entries != 1 {
		t.Fatalf("链条目=%d, want 1（旧代际的链必须已归还池，等 LRU 就是漏）", st.Entries)
	}
}

// TestSamplerDescriptorForImageFilter_Mip pins the sampler half of the mode: the
// two non-mip modes keep the historic nearest-level select, and an unset
// LodMaxClamp is not acceptable — rwgpu used to pass 0 straight through, which
// wgpu-native reads as "clamp to level 0", silently disabling mip on that
// backend while GL treated 0 as unclamped.
func TestSamplerDescriptorForImageFilter_Mip(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  ImageSamplerKey
		want types.MipmapFilterMode
	}{
		{"none", ImageSamplerKey{}, types.MipmapFilterModeNearest},
		{"nearest", ImageSamplerKey{Mip: render.MipmapNearest}, types.MipmapFilterModeNearest},
		{"linear", ImageSamplerKey{Mip: render.MipmapLinear}, types.MipmapFilterModeLinear},
		{"bicubic masks mip", ImageSamplerKey{Bicubic: true, Mip: render.MipmapLinear}, types.MipmapFilterModeNearest},
	} {
		got := SamplerDescriptorForImageFilter(tc.key)
		if got.MipmapFilter != tc.want {
			t.Fatalf("%s: MipmapFilter=%v want %v", tc.name, got.MipmapFilter, tc.want)
		}
		if got.LodMaxClamp == 0 {
			t.Fatalf("%s: LodMaxClamp 未设：0 在 rwgpu 会被透传成“只采样第 0 级”，gles 却当不夹上限", tc.name)
		}
	}
}

// TestImageCache_MipBudgetCapacity pins what the batch-2 performance gate needs
// in arithmetic, not timings: how many mipmapped thumbnails the declared
// 128-entry / 64-MiB image-cache budget actually holds, per mode. Under the old
// level-0-only accounting these numbers were optimistic by up to 33%; the ledger
// already charged the chain.
func TestImageCache_MipBudgetCapacity(t *testing.T) {
	b := render.DefaultBudget(render.BudgetImageCache)
	type row struct {
		name  string
		w, h  int
	}
	rows := []row{{"缩略图", 256, 256}, {"大图标", 512, 512}, {"长条", 70, 8}, {"宽图", 1024, 256}}
	for _, r := range rows {
		levels := hal.MipLevelCount(uint32(r.w), uint32(r.h))
		one := int64(r.w) * int64(r.h) * 4
		mip := int64(hal.VramTextureBytes(types.Extent3D{Width: uint32(r.w), Height: uint32(r.h), DepthOrArrayLayers: 1}, 1, levels, 1, types.TextureFormatRGBA8Unorm))
		if mip <= one {
			t.Fatalf("%dx%d: 多级字节(%d)应大于单级(%d)", r.w, r.h, mip, one)
		}
		// 细长图的全链面积**超过** 4/3·level0：短边早早夹到 1 像素，长边还在
		// 减半，尾部一串 1×1 级把比例抬到 1.34×（70×8 实测）。所以「4/3 是上界」
		// 只对方正图成立——这正是 hal 与 render 都改成按级精确求和、而不是继续
		// 拿 4/3 当估算的理由。这里只保留一条宽界，防住算错量级。
		if mip > one*2 {
			t.Fatalf("%dx%d: 全链字节 %d 超过 level0 的 2 倍（%d），求和口径不对", r.w, r.h, mip, one)
		}
		ratio := float64(mip) / float64(one)
		if ratio > 4.0/3.0*1.002 { // 留出整数舍入余量，只报真正的反例
			t.Logf("%dx%d: 全链/level0=%.3f > 4/3=%.3f —— 细长图证伪「4/3 即上界」", r.w, r.h, ratio, 4.0/3.0)
		}
		byEntries := int64(b.MaxEntries)
		byBytes := b.MaxBytes / mip
		fit := byEntries
		if byBytes < fit {
			fit = byBytes
		}
		if fit < 1 {
			t.Fatalf("%dx%d mip 条目 %d B 已超预算 %d B，128 条/64 MiB 门一张都装不下", r.w, r.h, mip, b.MaxBytes)
		}
		t.Logf("%dx%d levels=%d 单级=%dB 全链=%dB(%.3f×) → 实际容纳 %d 张（条数上限 %d / 按全链字节上限 %d）；按旧的 level0 字节数会以为装得下 %d 张",
			r.w, r.h, levels, one, mip, float64(mip)/float64(one), fit, byEntries, b.MaxBytes/mip, b.MaxBytes/one)
	}
}
