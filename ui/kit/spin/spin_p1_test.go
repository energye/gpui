package spin_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/spin"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

func loadP1SpinFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("P1 true-text needs a system face: %v", err)
	}
	return face
}

// SPN-05 fullscreen P1: viewport mask + white section.
func TestSpin_PRD_SPN05_Fullscreen(t *testing.T) {
	s := spin.NewSpin(nil)
	if s.Fullscreen() {
		t.Fatal("fullscreen defaults false (P1)")
	}
	s.SetFullscreen(true)
	if !s.Fullscreen() {
		t.Fatal("SetFullscreen(true) must stick")
	}
	if !s.IsDisplaySpinning() {
		t.Fatal("fullscreen spinning must display")
	}
	tok := theme.Default.Current()
	mc := s.EffectiveMaskColor()
	if mc.R != tok.ColorBgMask.R || mc.G != tok.ColorBgMask.G || mc.B != tok.ColorBgMask.B {
		t.Fatalf("fullscreen mask=%+v want BgMask %+v", mc, tok.ColorBgMask)
	}
	ind := s.EffectiveIndicatorColor()
	if ind.R < 0.9 || ind.G < 0.9 || ind.B < 0.9 {
		t.Fatalf("fullscreen indicator=%+v want white", ind)
	}
	desc := s.EffectiveDescriptionColor()
	if desc.R < 0.9 || desc.G < 0.9 || desc.B < 0.9 {
		t.Fatalf("fullscreen description=%+v want white", desc)
	}
	sz := s.Layout(rendering.Tight(200, 120))
	if sz.Width != 200 || sz.Height != 120 {
		t.Fatalf("fullscreen layout=%v want 200x120 viewport fill", sz)
	}
	hit := s.Node().HitTest(rendering.Point{X: 100, Y: 60})
	if hit == nil {
		t.Fatal("fullscreen mask must be hittable")
	}
	s.SetFullscreen(false)
	if s.Fullscreen() {
		t.Fatal("fullscreen toggle off must clear")
	}
	simple := s.Layout(rendering.Loose(400, 400))
	if simple.Width >= 200 || simple.Height >= 120 {
		t.Fatalf("simple after fullscreen=%v must collapse to section", simple)
	}
	// Paint must not crash with or without a face.
	dc := render.NewContext(200, 120)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.SetFullscreen(true)
	s.Layout(rendering.Tight(200, 120))
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
}

// True-text chain: description paints nothing without a face (no black bar),
// real glyphs with a face. Layout/Node stay non-zero throughout.
func TestSpin_P1_TrueText(t *testing.T) {
	face := loadP1SpinFace(t)
	countZone := func(s *spin.Spin, wantDesc bool) (int, int) {
		sz := s.Layout(rendering.Loose(400, 400))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("layout=%v want >0", sz)
		}
		dc := render.NewContext(int(sz.Width)+2, int(sz.Height)+2)
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(1, 1))
		img := dc.Image()
		indH := s.DotSize()
		gap := s.EffectiveGap()
		// Indicator zone: top indH rows.
		indInk := 0
		for y := 1; y < int(indH)+1 && y < int(sz.Height)+2; y++ {
			for x := 1; x < int(sz.Width)+1; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r/257 < 250 || g/257 < 250 || b/257 < 250 {
					indInk++
				}
			}
		}
		descInk := 0
		if wantDesc {
			top := int(indH + gap)
			for y := 1 + top; y < int(sz.Height)+2; y++ {
				for x := 1; x < int(sz.Width)+1; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					if r/257 < 250 || g/257 < 250 || b/257 < 250 {
						descInk++
					}
				}
			}
		}
		return indInk, descInk
	}

	plain := spin.NewSpin(nil)
	plain.SetDescription("Hello World")
	if plain.TextFace() != nil {
		t.Fatal("TextFace must start nil")
	}
	_, descNoFace := countZone(plain, true)
	if descNoFace != 0 {
		t.Fatalf("no-face desc ink=%d want 0 (black bar banned)", descNoFace)
	}
	with := spin.NewSpin(nil)
	with.SetDescription("Hello World")
	with.SetTextFace(face)
	if with.TextFace() == nil {
		t.Fatal("TextFace nil after SetTextFace")
	}
	indInk, descInk := countZone(with, true)
	if indInk < 10 {
		t.Fatalf("indicator ink=%d want >=10", indInk)
	}
	if descInk < 30 {
		t.Fatalf("with-face desc ink=%d want >=30 (real glyphs missing)", descInk)
	}
	with.SetTextFace(nil)
	if with.TextFace() != nil {
		t.Fatal("SetTextFace(nil) must clear")
	}
	_, descCleared := countZone(with, true)
	if descCleared != 0 {
		t.Fatalf("cleared-face desc ink=%d want 0", descCleared)
	}
	// Node() directly after New() already has size.
	nb := spin.NewSpin(nil)
	ns := nb.Node().Size()
	if ns.Width <= 0 || ns.Height <= 0 {
		t.Fatalf("Node size=%v want >0 right after New", ns)
	}
	if lo := nb.Layout(rendering.Loose(400, 400)); lo.Width <= 0 || lo.Height <= 0 {
		t.Fatalf("Layout=%v want >0", lo)
	}
}

// Percent progressbar semantics (spec §6.6).
func TestSpin_P1_ProgressbarSemantics(t *testing.T) {
	s := spin.NewSpin(nil)
	if s.Role() != "status" {
		t.Fatalf("looper role=%q want status", s.Role())
	}
	s.SetPercent(40)
	if s.Role() != "progressbar" {
		t.Fatalf("percent role=%q want progressbar", s.Role())
	}
	if s.AriaValueMin() != 0 || s.AriaValueMax() != 100 || s.AriaValueNow() != 40 {
		t.Fatalf("aria values %v/%v/%v want 0/100/40", s.AriaValueMin(), s.AriaValueMax(), s.AriaValueNow())
	}
	s.SetPercentAuto()
	s.Tick(1.0)
	if s.Role() != "progressbar" || s.AriaValueNow() <= 0 {
		t.Fatalf("auto role=%q now=%v", s.Role(), s.AriaValueNow())
	}
	s.ClearPercent()
	if s.Role() != "status" {
		t.Fatalf("cleared role=%q want status", s.Role())
	}
}

// P1 semantic classNames/styles function form is React-only: no CSS engine
// on desktop, shallow fields are the P0 mapping. Documented Skip.
func TestSpin_P1_SemanticFuncNA(t *testing.T) {
	t.Skip("P1 classNames/styles function form (info:{props})=>Record is React-only; gpui uses shallow SpinClassNames/SpinStyles, no CSS engine")
}

// P1 pixel-perfect 4-dot keyframes (405deg etc.) stay staged: Ticker phase
// approximates motion, pixel frames are not hashed. Documented Skip.
func TestSpin_P1_KeyframesPixelNA(t *testing.T) {
	t.Skip("P1 4-dot keyframe pixel parity (405deg/offsets) is staged; Ticker phase drives behavior,本库 golden is the L3 source")
}

// P1 ConfigProvider spin globals follow the ConfigProvider kit (NotStarted);
// SetDefaultIndicator is the staging point and is covered by SPN-14.
func TestSpin_P1_ConfigProviderNA(t *testing.T) {
	t.Skip("P1 ConfigProvider spin globals follow the ConfigProvider kit (NotStarted); SetDefaultIndicator is the staged global, covered by SPN-14")
}

// P1 debug demos and ant.design pixel-hash parity are out of scope.
func TestSpin_PRD_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug demos (_semantic) and ant.design pixel-hash parity are not built;本库 golden is the L3 source")
}

// L4 human-eye side-by-side needs a reviewer: documented Skip.
func TestSpin_PRD_SPN22_HumanEyeNA(t *testing.T) {
	t.Skip("L4 SPN-22 needs human side-by-side sign-off against ant.design; no automated assertion")
}
