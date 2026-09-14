package timeline_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/timeline"
	"github.com/energye/gpui/ui/rendering"
)

// Layout/Node contract: a non-empty timeline carries intrinsic size through
// Layout, and Node() stays non-zero afterwards (empty stays zero by design,
// see TestTimeline_PRD_TL01).
func TestTimeline_LayoutNode_NonZero(t *testing.T) {
	mk := func() *timeline.Timeline {
		return timeline.NewTimeline(
			timeline.TimelineItem{Title: "2015-09-01", Content: "Create services site"},
			timeline.TimelineItem{Content: "Solve network problems"},
		)
	}
	tl := mk()
	sz := tl.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loose layout=%v must be non-zero", sz)
	}
	if ns := tl.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("node size=%v must be non-zero after Layout", ns)
	}
	if tl.ChromeNode() == nil {
		t.Fatal("ChromeNode nil")
	}
	hz := mk()
	hz.SetOrientation(timeline.TimelineHorizontal)
	if hs := hz.Layout(rendering.Loose(400, 400)); hs.Width <= 0 || hs.Height <= 0 {
		t.Fatalf("horizontal layout=%v must be non-zero", hs)
	}
	if ns := hz.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("horizontal node size=%v must be non-zero", ns)
	}
}

// No-face path: layout still estimates width from the heuristic and paint
// must never draw a black bar in place of text (CPU DrawString without a
// face is a no-op, so the text zone keeps the canvas background).
func TestTimeline_NoFace_NoBlackBar(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Title: "2015-09-01", Content: "Create services site"},
		timeline.TimelineItem{Content: "Solve network problems"},
	)
	sz := tl.Layout(rendering.Loose(300, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loose layout=%v must be non-zero without a face", sz)
	}
	const cw, ch = 300, 200
	dc := render.NewContext(cw, ch)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tl.Node().Paint(rendering.NewPaintContext(dc, 1))
	got := dc.Image()
	black := 0
	for yy := 0; yy < ch; yy++ {
		for xx := 0; xx < cw; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 20 && g/257 < 20 && b/257 < 20 {
				black++
			}
		}
	}
	if black != 0 {
		t.Fatalf("text zone has %d near-black pixels without a face (black bar?)", black)
	}
}

// True-text chain: the same nodes paint real glyphs once SetFace is wired,
// and clearing the face returns to the ink-free degraded path.
func TestTimeline_TrueText_FaceChain(t *testing.T) {
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("true-text needs a system face: %v", err)
	}
	t.Logf("true-text face: %s", desc)
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Title: "2015-09-01", Content: "Create services site"},
		timeline.TimelineItem{Content: "Solve network problems"},
	)
	countDark := func() int {
		sz := tl.Layout(rendering.Loose(300, 200))
		dc := render.NewContext(300, 200)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		tl.Node().Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		dark := 0
		for y := 0; y < int(sz.Height) && y < 200; y++ {
			for x := 0; x < 300; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r/257 < 110 && g/257 < 110 && b/257 < 110 {
					dark++
				}
			}
		}
		return dark
	}
	if n := countDark(); n != 0 {
		t.Fatalf("no-face dark=%d want 0 (black bar banned)", n)
	}
	tl.SetFace(face)
	if n := countDark(); n < 30 {
		t.Fatalf("with-face dark=%d want >=30 (real glyphs missing?)", n)
	}
	tl.SetFace(nil)
	if n := countDark(); n != 0 {
		t.Fatalf("cleared-face dark=%d want 0", n)
	}
}

// TestTimeline_PRD_TL22_P1_TitleSpan covers §6.8 P1 titleSpan staging: the
// default 12 reads back and round-trips through SetTitleSpan without moving
// layout or breaking paint. Percent/string spans stay staged.
func TestTimeline_PRD_TL22_P1_TitleSpan(t *testing.T) {
	tl := timeline.NewTimeline(timeline.TimelineItem{Content: "span me"})
	if tl.TitleSpan() != 12 {
		t.Fatalf("default TitleSpan=%v want 12", tl.TitleSpan())
	}
	tl.SetTitleSpan(18)
	if tl.TitleSpan() != 18 {
		t.Fatalf("TitleSpan=%v want 18", tl.TitleSpan())
	}
	before := tl.Layout(rendering.Loose(400, 200))
	tl.SetTitleSpan(18)
	after := tl.Layout(rendering.Loose(400, 200))
	if before != after {
		t.Fatalf("same span must keep layout stable: %v vs %v", before, after)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL22_P1_TitleSpanPercent(t *testing.T) {
	t.Skip("P1 staged: titleSpan percent/string spans (100px/25%) need the fine two-slot layout; P0 default 12 is readable (TL-16 green)")
}

// TestTimeline_PRD_TL22_P1_SemanticHooks covers §6.8 P1 semantic
// classNames/styles: the kit keeps a shallow style hook (stored,
// paint-safe). A deep CSS cascade is staged, not silently dropped.
func TestTimeline_PRD_TL22_P1_SemanticHooks(t *testing.T) {
	tl := timeline.NewTimeline(timeline.TimelineItem{Content: "hooked"})
	tl.SetStyle(timeline.TimelineStyle{})
	before := tl.Layout(rendering.Loose(400, 200))
	tl.SetStyle(timeline.TimelineStyle{})
	after := tl.Layout(rendering.Loose(400, 200))
	if before != after {
		t.Fatalf("style hook must not move layout: %v vs %v", before, after)
	}
	paintTimeline(tl, 128, 128)
}

func TestTimeline_PRD_TL22_P1_MotionPixels(t *testing.T) {
	t.Skip("P1 staged: pixel-level enter/expand motion needs an animation clock; P0 uses instant switch with reduced-motion support (SetReduceMotion green)")
}

func TestTimeline_PRD_TL22_P1_GlobalDefaults(t *testing.T) {
	t.Skip("P1 staged: ConfigProvider global defaults ride on the theme provider path (SetProvider green); no process-wide timeline defaults wired yet")
}

func TestTimeline_PRD_TL22_P1_DebugNA(t *testing.T) {
	t.Skip("P1 out of scope: pending-legacy/horizontal-debug/component-token are doc-only debug pages, and ant.design pixel-hash parity is excluded per §6.1")
}

func TestTimeline_PRD_TL18_DisabledNA(t *testing.T) {
	t.Skip("N/A per §6.9 TL-18: Timeline has no disabled state; controls inside nodes follow their own semantics")
}

func TestTimeline_PRD_TL19_KeyboardNA(t *testing.T) {
	t.Skip("N/A per §6.9 TL-19: the body never takes focus (Focusable false); controls inside nodes follow their own keyboard semantics")
}

func TestTimeline_PRD_TL21_HumanEye(t *testing.T) {
	t.Skip("L4 TL-21 needs human side-by-side sign-off against ant.design; cannot pass in CI")
}
