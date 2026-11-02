package render

import (
	"testing"

	intImage "github.com/energye/gpui/render/internal/image"
)

// ExportImageBuf reuse must keep ONE ImageBuf while advancing the content
// generation, and must broadcast the retired generation so every consumer cache
// reclaims its own stale texture (P3.6 batch 1c, replacing the old stable-gen +
// one-shot dirty flag contract).
func TestExportImageBuf_RewriteAdvancesGeneration(t *testing.T) {
	dc := NewContext(32, 24)
	defer dc.Close()
	dc.ClearWithColor(RGBA{R: 1, G: 0, B: 0, A: 1})
	var img *ImageBuf
	if !dc.ExportImageBuf(&img) || img == nil {
		t.Fatal("first export failed")
	}
	gen1 := img.GenerationID()
	if gen1 == 0 {
		t.Fatal("expected non-zero gen after first export")
	}
	var retired []uint64
	remove := img.AddGenIDWatcher(func(old uint64) { retired = append(retired, old) })
	defer remove()

	// Re-export after a different clear: same buffer object, new generation, and
	// the previous generation must be reported to the watcher.
	dc.ClearWithColor(RGBA{R: 0, G: 0, B: 1, A: 1})
	if !dc.ExportImageBuf(&img) {
		t.Fatal("second export failed")
	}
	if img.GenerationID() == gen1 {
		t.Fatalf("generation unchanged after in-place rewrite (%d): caches would keep painting the first export", gen1)
	}
	if len(retired) != 1 || retired[0] != gen1 {
		t.Fatalf("retired generations = %v, want [%d]", retired, gen1)
	}
}

// Dispose must retire the generation too, so a cache holding this buffer's
// texture is told instead of left waiting for LRU.
func TestImageBufDisposeReportsRetirement(t *testing.T) {
	img, err := intImage.NewImageBuf(8, 8, intImage.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	gen := img.GenerationID()
	var retired []uint64
	remove := img.AddGenIDWatcher(func(old uint64) { retired = append(retired, old) })
	defer remove()
	img.Dispose()
	if len(retired) != 1 || retired[0] != gen {
		t.Fatalf("retired on dispose = %v, want [%d]", retired, gen)
	}
	if img.GenerationID() != 0 {
		t.Fatalf("generation after dispose = %d, want 0", img.GenerationID())
	}
}

// 池化复用必须换号：回收给下一层的 Pixmap 若带着上一层的号，以号为键的缓存会
// 把上一层的纹理画给这一层（同尺寸即命中，实测旧状两族像素源 200 个号全交集）。
func TestPixmapPoolRecycleRetiresGeneration(t *testing.T) {
	pool := newPixmapPool(8)
	pm := pool.GetForOverwrite(64, 64)
	before := pm.GenerationID()
	var retired []uint64
	remove := pm.AddGenIDWatcher(func(old uint64) { retired = append(retired, old) })
	defer remove()

	pool.Put(pm)
	got := pool.Get(64, 64)
	if got != pm {
		t.Skipf("池未复用同一对象（got=%p want=%p），本钉只针对复用例", got, pm)
	}
	if got.GenerationID() == before {
		t.Fatalf("复用后图号未变（%d）：以该号为键的缓存会把上一层的画面画给这一层", before)
	}
	if len(retired) != 1 || retired[0] != before {
		t.Fatalf("retired=%v, want [%d]", retired, before)
	}
}
