package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// TestRenderText_WeightKeepsAdvance: synthetic bold never moves layout;
// measure and layout stay equal, width identical to regular.
func TestRenderText_WeightKeepsAdvance(t *testing.T) {
	face := loadTestFace(t, 16)
	plain := rendering.NewRenderText("AB")
	plain.SetFace(face)
	plain.SetFontSize(16)
	_ = plain.Layout(rendering.Loose(400, 80))
	plainW := spacedLineWidth(t, plain)

	bold := rendering.NewRenderText("AB")
	bold.SetFace(face)
	bold.SetFontSize(16)
	bold.SetFontWeight(text.WeightBold)
	_ = bold.Layout(rendering.Loose(400, 80))
	boldW := spacedLineWidth(t, bold)

	if boldW != plainW {
		t.Fatalf("bold width=%v plain=%v want equal (synthetic advance unchanged)", boldW, plainW)
	}
	if m := bold.MeasureWidth("AB"); m != boldW {
		t.Fatalf("bold measure=%v layout=%v want equal", m, boldW)
	}
}

// TestRenderText_WeightPaintAddsInk: bold paint covers at least as many dark
// pixels as regular on the same text.
func TestRenderText_WeightPaintAddsInk(t *testing.T) {
	face := loadTestFace(t, 16)
	ink := func(w text.FontWeight) int {
		txt := rendering.NewRenderText("H")
		txt.SetFace(face)
		txt.SetFontSize(32)
		txt.R, txt.G, txt.B, txt.A = 0, 0, 0, 1
		if w != 0 {
			txt.SetFontWeight(w)
		}
		_ = txt.Layout(rendering.Loose(200, 80))
		dc := render.NewContext(120, 60)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		txt.Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		n := 0
		for x := 0; x < 120; x++ {
			for y := 0; y < 60; y++ {
				r, g, b := sampleRGB(img.At(x, y))
				if r < 0x8000 && g < 0x8000 && b < 0x8000 {
					n++
				}
			}
		}
		return n
	}
	plain, bold := ink(0), ink(text.WeightBold)
	if plain <= 0 || bold <= 0 {
		t.Fatal("no ink painted in one path")
	}
	if bold < plain {
		t.Fatalf("bold ink=%d plain ink=%d want bold>=plain", bold, plain)
	}
}

// TestRenderText_WeightInvalidatesParentCache: changing weight must
// re-record the parent boundary, not replay stale regular pixels. The parent
// carries six text rows (6 ops > the naive complexity gate) and is settled past
// the access threshold first, so clean frames really do Replay.
//
// Pixel evidence is unavailable for weight on this box: the installed sans
// face has a single grade, so SetFontWeight → DeriveFace → FontSource.Face
// resolves the same file and bold paints identical ink. The freshness anchors
// are the fingerprint (TestTextContentKey_DistinguishesState) and the cache
// decision here — a stale replay would surface as skip=1 on the change frame.
func TestRenderText_WeightInvalidatesParentCache(t *testing.T) {
	face := loadTestFace(t, 16)
	cache := rendering.NewBoundaryCache()
	root := rendering.NewAbsoluteBox(200, 140)
	root.SetRepaintBoundary(true)
	child := rendering.NewRenderText("AB\nAB\nAB\nAB\nAB\nAB")
	child.SetFace(face)
	child.SetFontSize(16)
	child.R, child.G, child.B, child.A = 0, 0, 0, 1
	root.Place(child, 10, 10)
	root.Layout(rendering.Tight(200, 140))

	paint := func() {
		dc := render.NewContext(200, 140)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
	}

	// Frames 1-4 settle (will_change then access hold-backs), frame 5 admits.
	for i := 0; i < 4; i++ {
		paint()
	}
	paint()
	rr, skip, _, _ := cache.FrameCounts()
	if rr != 1 || skip != 0 || !cache.HasValid(root) {
		t.Fatalf("admit frame rr=%d skip=%d valid=%v want 1/0/true", rr, skip, cache.HasValid(root))
	}
	paint()
	if rr, skip, _, _ := cache.FrameCounts(); rr != 0 || skip != 1 {
		t.Fatalf("clean repaint rr=%d skip=%d want 0/1", rr, skip)
	}

	// Weight change: live paint (no stale replay), not cached (will_change).
	child.SetFontWeight(text.WeightBold)
	paint()
	if rr, skip, _, _ := cache.FrameCounts(); rr != 0 || skip != 0 {
		t.Fatalf("weight-change frame rr=%d skip=%d want 0/0", rr, skip)
	}

	// The bold generation re-admits only after the access threshold again.
	for i := 0; i < 3; i++ {
		paint()
	}
	paint()
	if rr, _, _, _ := cache.FrameCounts(); rr != 1 || !cache.HasValid(root) {
		t.Fatalf("re-admit frame rr=%d valid=%v want 1/true", rr, cache.HasValid(root))
	}
	paint()
	if rr, skip, _, _ := cache.FrameCounts(); rr != 0 || skip != 1 {
		t.Fatalf("post-change replay rr=%d skip=%d want 0/1", rr, skip)
	}
}
