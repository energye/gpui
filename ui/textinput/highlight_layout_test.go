package textinput

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

type hlGeom struct {
	x, y, w, h float64
}

// effectiveGeom reads highlight geometry the way paint/record see it: paint
// falls back to the built Width/Height when layout never sizes a manual box.
func effectiveGeom(hb *rendering.RenderColorBox) hlGeom {
	o := hb.Offset()
	s := hb.Size()
	w, h := s.Width, s.Height
	if w <= 0 {
		w = hb.Width
	}
	if h <= 0 {
		h = hb.Height
	}
	return hlGeom{o.X, o.Y, w, h}
}

// TestSelectionHighlightSurvivesLayout pins the B/C double-click report:
// flow layout must not resize or reposition owner-managed selection
// highlights, and highlights must clip to the visible window (an unclipped
// giant box records the origin-anchored texture slice, so scrolled text
// never shows its background). Before the fix, the first layout moved every
// highlight to the origin clamped to the container width.
func TestSelectionHighlightSurvivesLayout(t *testing.T) {
	face, _, _ := text.LoadMultiFace(16)
	if face == nil {
		t.Skip("no font face available")
	}
	content := strings.Repeat("a世界bHello你好", 700)
	win := rendering.Size{Width: 1200, Height: 800}

	setup := func() (*rendering.PipelineOwner, *ViewportInputBox, *InputBox, *MultiLineInputBox) {
		edV := New()
		edV.SetTextSimple(content)
		boxV := NewViewportInputBox(edV, 880, 40, 16)
		boxV.SetFace(face)
		edP := New()
		edP.SetTextSimple(content)
		boxP := NewInputBox(edP, 880, 40, 16)
		boxP.SetFace(face)
		edM := New()
		edM.SetTextSimple(content)
		boxM := NewMultiLineInputBox(edM, 880, 90, 16)
		boxM.SetFace(face)
		boxM.SetWrap(true)
		root := rendering.NewAbsoluteBox(1200, 800)
		root.Place(boxV, 296, 180)
		root.Place(boxP, 296, 100)
		root.Place(boxM, 296, 300)
		owner := rendering.NewPipelineOwner(root)
		// No initial flush: like the live window (select-all exists before
		// the first layout pass), the first FlushLayout below must preserve
		// highlight geometry. Fresh per case: a clean root would skip the
		// walk and prove nothing.
		return owner, boxV, boxP, boxM
	}

	viewportCheck := func(boxV *ViewportInputBox) func(t *testing.T, g []hlGeom) {
		return func(t *testing.T, g []hlGeom) {
			if g[0].w < 100 || g[0].w > 880 {
				t.Fatalf("viewport highlight must clip to the visible window, got %+v", g[0])
			}
			scrollX := boxV.Viewport.ScrollOffset().X
			if g[0].x < scrollX-1 || g[0].x+g[0].w < scrollX+100 {
				t.Fatalf("viewport highlight outside visible window (scrollX=%.1f): %+v", scrollX, g[0])
			}
		}
	}

	cases := []struct {
		name string
		pick func(boxV *ViewportInputBox, boxP *InputBox, boxM *MultiLineInputBox) (*Editor, func() []*rendering.RenderColorBox, func(t *testing.T, g []hlGeom))
	}{
		{"viewport", func(boxV *ViewportInputBox, _ *InputBox, _ *MultiLineInputBox) (*Editor, func() []*rendering.RenderColorBox, func(t *testing.T, g []hlGeom)) {
			return boxV.Editor(), func() []*rendering.RenderColorBox { return boxV.highlights }, viewportCheck(boxV)
		}},
		{"plain-scrolled", func(_ *ViewportInputBox, boxP *InputBox, _ *MultiLineInputBox) (*Editor, func() []*rendering.RenderColorBox, func(t *testing.T, g []hlGeom)) {
			return boxP.Editor(), func() []*rendering.RenderColorBox { return boxP.highlights }, nil
		}},
		{"multiline-wrap", func(_ *ViewportInputBox, _ *InputBox, boxM *MultiLineInputBox) (*Editor, func() []*rendering.RenderColorBox, func(t *testing.T, g []hlGeom)) {
			return boxM.Editor(), func() []*rendering.RenderColorBox { return boxM.highlights }, nil
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, boxV, boxP, boxM := setup()
			ed, getHls, check := tc.pick(boxV, boxP, boxM)
			ed.SelectAll()
			geoms := func() []hlGeom {
				// Re-read live: syncHighlight rebuilds the slice.
				hs := getHls()
				if len(hs) == 0 {
					t.Fatalf("%s: no highlight boxes after select-all", tc.name)
				}
				out := make([]hlGeom, 0, len(hs))
				for _, hb := range hs {
					out = append(out, effectiveGeom(hb))
				}
				return out
			}
			before := geoms()
			owner.FlushLayout(win, true)
			after := geoms()
			if len(before) != len(after) {
				t.Fatalf("%s: highlight count changed %d → %d", tc.name, len(before), len(after))
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatalf("%s: highlight %d moved/resized by layout: %+v → %+v", tc.name, i, before[i], after[i])
				}
			}
			if check != nil {
				check(t, after)
			}
		})
	}
}
