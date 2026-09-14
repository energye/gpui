package border_beam_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	borderbeam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// P1 + N/A per docs/antd/border-beam.md §6.8-§6.9: runnable parts assert
// here; browser-only depth Skips with reasons. BB-18 (L3 golden) is covered
// by TestBorderBeam_GoldenBeam; BB-01…15 stay in border_beam_test.go.

// BB-16 disabled N/A: the beam is decorative and owns no disabled state.
func TestBorderBeam_PRD_BB16_DisabledNA(t *testing.T) {
	t.Skip("N/A per §6.9 BB-16: BorderBeam is a decorative overlay with no disabled skin; business disabled state lives on the wrapped children")
}

// BB-17 keyboard/focus N/A: the beam never takes focus; children self-handle.
func TestBorderBeam_PRD_BB17_KeyboardFocusNA(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	if b.Focusable() {
		t.Fatal("beam must never take focus")
	}
	if b.Role() != "presentation" {
		t.Fatalf("role=%q want presentation", b.Role())
	}
	t.Skip("N/A per §6.9 BB-17: beam takes no Tab and handles no keys; focus and keyboard stay with the wrapped children")
}

// BB-19 L4 human eye: reviewer sign-off, no automated assertion.
func TestBorderBeam_PRD_BB19_HumanEyeNA(t *testing.T) {
	t.Skip("L4 per §6.9 BB-19: side-by-side against the house gallery baseline needs a human reviewer; automated cover is BB-18 golden + showcase")
}

// BB-20 semantic classNames/styles depth: P0 keeps the flat Style hook;
// per-node Record depth stays P1.
func TestBorderBeam_PRD_BB20_SemanticDepth(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetStyle(borderbeam.Style{ClassName: "my-beam"})
	if b.Style().ClassName != "my-beam" {
		t.Fatalf("style=%+v want my-beam", b.Style())
	}
	a := b.Layout(rendering.Loose(400, 400))
	b.SetStyle(borderbeam.Style{ClassName: "late-beam"})
	after := b.Layout(rendering.Loose(400, 400))
	if a != after {
		t.Fatalf("semantic hook moved layout %v -> %v", a, after)
	}
	paintBeamOK(t, b, 160, 120)
	t.Skip("P1 staged per §6.8: functional classNames/styles Record per node maps browser CSS and has no desktop equivalent; flat SetStyle hook stays P0")
}

// BB-20 CSS offset-path / mask-composite pixel level: P0 approximates with a
// short stroked segment along the rounded-rect perimeter (§6.5, §6.7).
func TestBorderBeam_PRD_BB20_OffsetPathPixel(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	b.SetSize(60)
	b.SetLineWidth(3)
	p0 := b.Phase()
	b.Tick(1.0)
	if b.Phase() == p0 {
		t.Fatal("phase must advance for the pixel comparison to mean anything")
	}
	sz := b.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want >0", sz)
	}
	paintBeamOK(t, b, 240, 160)
	t.Skip("P1 staged per §6.7-§6.8: byte-identical CSS offset-path + mask-composite ring is browser-only; P0 uses Tick + path-sampled short-segment stroke (behavior P0, pixel-identical P1)")
}

// BB-20 auto re-measure of children border-radius: P0 uses explicit Set plus
// the theme token; continuous re-measure stays P1 (perf, per FAQ).
func TestBorderBeam_PRD_BB20_AutoRadius(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetBorderRadius(8)
	if math.Abs(b.ResolvedBorderRadius()-8) > 1e-9 {
		t.Fatalf("radius=%v want 8", b.ResolvedBorderRadius())
	}
	b.ClearBorderRadius()
	if math.Abs(b.ResolvedBorderRadius()-6) > 0.5 {
		t.Fatalf("cleared radius=%v want theme 6", b.ResolvedBorderRadius())
	}
	paintBeamOK(t, b, 240, 160)
	t.Skip("P1 staged per §6.8: continuously re-reading computed border-radius after mount is explicitly not re-measured for perf; explicit SetBorderRadius + token fallback stays P0")
}

// BB-20 ConfigProvider global borderBeam: per-instance theme stays P0.
func TestBorderBeam_PRD_BB20_ConfigProvider(t *testing.T) {
	var toks theme.Tokens = theme.DefaultTokens()
	b := borderbeam.NewBorderBeam(boxChild(120, 60))
	b.SetProvider(theme.NewProvider(toks))
	head := b.ResolvedColorStops()[0].Color
	want := render.RGBA{R: toks.ColorPrimary.R, G: toks.ColorPrimary.G, B: toks.ColorPrimary.B, A: toks.ColorPrimary.A}
	if head != want {
		t.Fatalf("head=%+v want %+v", head, want)
	}
	paintBeamOK(t, b, 160, 120)
	t.Skip("P1 staged per §6.8: ConfigProvider global borderBeam defaults need cross-package provider wiring; per-instance SetProvider/SetTheme stays P0")
}

// BB-20 debug examples + pixel hash: explicitly out of scope.
func TestBorderBeam_PRD_BB20_DebugHash(t *testing.T) {
	t.Skip("P1 not counted per §6.8: non-uniform-radius / component-token debug previews plus browser pixel-hash identity are explicitly out of scope (§6.1, §6.8)")
}

// BB-20 non-uniform four-corner radius: P0 keeps the uniform radius.
func TestBorderBeam_PRD_BB20_NonUniformRadius(t *testing.T) {
	b := borderbeam.NewBorderBeam(boxChild(200, 120))
	b.SetBorderRadius(8)
	sz := b.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v want >0", sz)
	}
	if math.Abs(b.ResolvedBorderRadius()-8) > 1e-9 {
		t.Fatalf("radius=%v want 8", b.ResolvedBorderRadius())
	}
	paintBeamOK(t, b, 240, 160)
	t.Skip("P1 staged per §6.8: per-corner radii stay future work; uniform SetBorderRadius covers Card-style P0 (custom-container radius 8)")
}
