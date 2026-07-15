//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"golang.org/x/image/font/gofont/goregular"
)

func s65Face(t *testing.T, size float64) text.Face {
	t.Helper()
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatalf("font: %v", err)
	}
	return src.Face(size)
}

func TestS65_LayoutText_UsesShapeCache(t *testing.T) {
	text.ClearShapeResultCache()
	text.ResetShapeResultCacheStats()
	face := s65Face(t, 14)
	eng := NewGlyphMaskEngine()
	color := render.RGBA{R: 0, G: 0, B: 0, A: 1}
	mat := render.Identity()

	b1, err := eng.LayoutText(face, "List item title", 8, 24, color, mat, 1)
	if err != nil {
		t.Fatalf("layout1: %v", err)
	}
	if len(b1.Quads) == 0 {
		t.Fatal("no quads")
	}
	st1 := text.ShapeResultCacheStats()
	if st1.Misses < 1 {
		t.Fatalf("expected layout miss, %+v", st1)
	}

	b2, err := eng.LayoutText(face, "List item title", 8, 48, color, mat, 1)
	if err != nil {
		t.Fatalf("layout2: %v", err)
	}
	if len(b2.Quads) != len(b1.Quads) {
		t.Fatalf("quads %d vs %d", len(b2.Quads), len(b1.Quads))
	}
	st2 := text.ShapeResultCacheStats()
	if st2.Hits < 1 {
		t.Fatalf("expected shape/layout cache hit on 2nd LayoutText, %+v", st2)
	}
}

func TestS65_ScrollReuse_AtlasUploadConverges(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()

	text.ClearShapeResultCache()
	face := s65Face(t, 13)
	eng := NewGlyphMaskEngine()
	color := render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1}
	mat := render.Identity()

	// Fixed vocabulary of list rows (scroll reuses same strings).
	rows := []string{
		"Inbox message subject line 00",
		"Inbox message subject line 01",
		"Inbox message subject line 02",
		"Inbox message subject line 03",
		"Inbox message subject line 04",
		"Inbox message subject line 05",
		"Inbox message subject line 06",
		"Inbox message subject line 07",
	}

	layoutFrame := func(offset int) {
		for i := 0; i < 6; i++ {
			s := rows[(offset+i)%len(rows)]
			_, err := eng.LayoutText(face, s, 12, 20+float64(i)*18, color, mat, 1)
			if err != nil {
				t.Fatalf("layout: %v", err)
			}
		}
	}

	// Cold frame: populate atlas.
	layoutFrame(0)
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("sync cold: %v", err)
	}
	coldBytes, _, _, _ := eng.LastUploadStats()
	hits0, misses0, _, _ := eng.AtlasStats()
	t.Logf("cold uploadBytes=%d atlas hits=%d misses=%d", coldBytes, hits0, misses0)
	if coldBytes == 0 {
		t.Fatal("cold frame should upload glyphs")
	}

	// Warm scroll: same vocabulary, different window offset → atlas hits, tiny/no upload.
	layoutFrame(2)
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("sync warm1: %v", err)
	}
	warm1Bytes, warm1Regions, _, _ := eng.LastUploadStats()
	hits1, misses1, _, _ := eng.AtlasStats()

	layoutFrame(4)
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("sync warm2: %v", err)
	}
	warm2Bytes, warm2Regions, _, _ := eng.LastUploadStats()
	hits2, misses2, _, _ := eng.AtlasStats()

	t.Logf("warm1 uploadBytes=%d regions=%d hits=%d misses=%d", warm1Bytes, warm1Regions, hits1, misses1)
	t.Logf("warm2 uploadBytes=%d regions=%d hits=%d misses=%d", warm2Bytes, warm2Regions, hits2, misses2)

	if hits2 <= hits0 {
		t.Fatalf("expected atlas hits to grow after scroll reuse: hits0=%d hits2=%d", hits0, hits2)
	}
	// After warm-up, upload should be far smaller than cold (ideally 0).
	if warm2Bytes > coldBytes/4 {
		t.Fatalf("scroll reuse should converge uploads: cold=%d warm2=%d", coldBytes, warm2Bytes)
	}
	// Shape/layout cache should also be hot.
	st := text.ShapeResultCacheStats()
	if st.Hits < 1 {
		t.Fatalf("expected layout shape cache hits during scroll, %+v", st)
	}
}

func TestS65_LCD_LayoutCacheSmoke(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	defer cleanup()

	text.ClearShapeResultCache()
	face := s65Face(t, 14)
	eng := NewGlyphMaskEngine()
	eng.SetLCDLayout(text.LCDLayoutRGB)
	color := render.RGBA{R: 0, G: 0, B: 0, A: 1}
	mat := render.Identity()

	s := "LCD body text sample"
	if _, err := eng.LayoutText(face, s, 4, 30, color, mat, 1); err != nil {
		t.Fatalf("lcd layout1: %v", err)
	}
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("sync1: %v", err)
	}
	b1, _, _, _ := eng.LastUploadStats()

	if _, err := eng.LayoutText(face, s, 4, 50, color, mat, 1); err != nil {
		t.Fatalf("lcd layout2: %v", err)
	}
	if err := eng.SyncAtlasTextures(device, queue); err != nil {
		t.Fatalf("sync2: %v", err)
	}
	b2, r2, _, _ := eng.LastUploadStats()
	st := text.ShapeResultCacheStats()
	t.Logf("lcd upload cold=%d warm=%d regions=%d shapeHits=%d", b1, b2, r2, st.Hits)
	if st.Hits < 1 {
		t.Fatalf("lcd second layout should hit shape cache, %+v", st)
	}
	if b2 != 0 || r2 != 0 {
		// Glyph masks already in atlas → hit-only frame.
		t.Fatalf("lcd warm frame should be zero-upload, bytes=%d regions=%d", b2, r2)
	}
}
