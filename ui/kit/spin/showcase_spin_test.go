package spin_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/spin"
	"github.com/energye/gpui/ui/rendering"
)

type showcaseSpinSpec struct {
	CanvasW     int     `json:"canvasW"`
	Margin      float64 `json:"margin"`
	Gap         float64 `json:"gap"`
	ColGap      float64 `json:"colGap"`
	FullscreenH float64 `json:"fullscreenH"`
	Tolerance   struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseSpinSpec(t *testing.T) showcaseSpinSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s showcaseSpinSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadShowcaseSpinFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("showcase needs a system face for real glyphs: %v", err)
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestSpin_Showcase_MainPaths lays §6.8 P0+P1 main paths on one canvas:
// basic / sizes / description+tip / delay+custom / percent numeric+auto /
// nested / style-class / fullscreen. Three evidences: logic probe,
// pixel assertions (indicator ink + description ink + fullscreen mask),
// golden compare (12/0.5%/64 from testdata). Regenerate with UPDATE_GOLDEN=1.
func TestSpin_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseSpinSpec(t)
	face := loadShowcaseSpinFace(t)
	prev := spin.DefaultIndicator()
	defer spin.SetDefaultIndicator(prev)
	spin.SetDefaultIndicator(nil)

	W := float64(spec.CanvasW)
	margin, gap, colGap := spec.Margin, spec.Gap, spec.ColGap
	rowW := W - 2*margin

	mkDesc := func(s *spin.Spin, d string) *spin.Spin {
		s.SetDescription(d)
		s.SetTextFace(face)
		return s
	}

	// R1 basic (basic.tsx): default medium looper.
	basic := spin.NewSpin(nil)

	// R2 sizes (size.tsx): small/medium/large.
	smallSpin := spin.NewSpin(nil)
	smallSpin.SetSize(spin.SpinSmall)
	mediumSpin := spin.NewSpin(nil)
	largeSpin := spin.NewSpin(nil)
	largeSpin.SetSize(spin.SpinLarge)

	// R3 description + tip alias (tip.tsx).
	descSpin := mkDesc(spin.NewSpin(nil), "Loading...")
	tipSpin := spin.NewSpin(nil)
	tipSpin.SetSize(spin.SpinLarge)
	tipSpin.SetTip("tip text")
	tipSpin.SetTextFace(face)

	// R4 delay (delayAndDebounce.tsx) shown after tick + custom indicator.
	delaySpin := spin.NewSpin(nil)
	delaySpin.SetDelay(500)
	delaySpin.SetSpinning(true)
	delaySpin.Tick(0.2)
	delaySpin.Tick(0.2)
	delaySpin.Tick(0.2)
	customBox := rendering.NewRenderColorBox(16, 16, 1, 0, 0, 1)
	customSpin := spin.NewSpin(nil)
	customSpin.SetIndicator(customBox)

	// R5 percent (percent.tsx): numeric + auto.
	percentNum := spin.NewSpin(nil)
	percentNum.SetPercent(65)
	percentAuto := spin.NewSpin(nil)
	percentAuto.SetPercentAuto()
	percentAuto.Tick(1.0)

	// R6 nested card (nested.tsx): content stays, mask blocks, section on top.
	nestedContent := rendering.NewRenderColorBox(160, 70, 0.6, 0.6, 0.6, 1)
	nestedSpin := spin.NewSpin(nestedContent)
	nestedSpin.SetDescription("Card loading")
	nestedSpin.SetTextFace(face)

	// R7 style-class (style-class.tsx): shallow hooks + overrides.
	styleSpin := mkDesc(spin.NewSpin(nil), "Styled")
	styleSpin.SetClassNames(spin.SpinClassNames{Root: "r", Section: "s", Indicator: "i", Description: "d", Container: "c"})
	styleSpin.SetStyles(spin.SpinStyles{Root: "r", Section: "s", Indicator: "i", Description: "d", Container: "c"})
	styleSpin.SetStyle(spin.Style{Gap: 10, FontSize: 13})

	// R8 fullscreen (fullscreen.tsx, P1): viewport mask + white section.
	fullSpin := mkDesc(spin.NewSpin(nil), "Fullscreen")
	fullSpin.SetFullscreen(true)

	// Logic probe before paint.
	if !basic.IsDisplaySpinning() || !basic.IsBuiltinIndicator() {
		t.Fatal("basic logic probe")
	}
	if smallSpin.DotSize() >= mediumSpin.DotSize() || mediumSpin.DotSize() >= largeSpin.DotSize() {
		t.Fatalf("sizes dot %v %v %v must ascend", smallSpin.DotSize(), mediumSpin.DotSize(), largeSpin.DotSize())
	}
	if descSpin.EffectiveDescription() != "Loading..." || tipSpin.EffectiveDescription() != "tip text" {
		t.Fatal("description/tip probe")
	}
	if !delaySpin.IsDisplaySpinning() {
		t.Fatal("delay must show after 600ms tick")
	}
	if customSpin.EffectiveIndicator() == nil || customSpin.IsBuiltinIndicator() {
		t.Fatal("custom indicator probe")
	}
	if !percentNum.HasPercent() || percentNum.EffectivePercent() != 65 {
		t.Fatal("percent numeric probe")
	}
	if !percentAuto.HasPercent() || percentAuto.EffectivePercent() <= 0 {
		t.Fatal("percent auto probe")
	}
	if fullSpin.Fullscreen() == false || fullSpin.EffectiveMaskColor().A <= 0 {
		t.Fatal("fullscreen probe")
	}
	if styleSpin.ClassNames().Root != "r" || styleSpin.Styles().Root != "r" {
		t.Fatal("style-class probe")
	}

	type placed struct {
		s *spin.Spin
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	layoutRow := func(row []*spin.Spin, maxH float64, fullBleed bool) {
		x := margin
		if fullBleed {
			x = 0
		}
		var sizes []rendering.Size
		maxRowH := 0.0
		for _, s := range row {
			var sz rendering.Size
			if s.Fullscreen() {
				sz = s.Layout(rendering.Constraints{MaxWidth: W, MaxHeight: spec.FullscreenH})
			} else {
				sz = s.Layout(rendering.Loose(rowW, maxH))
			}
			if sz.Width <= 0 || sz.Height <= 0 {
				t.Fatalf("showcase layout=%v want >0", sz)
			}
			sizes = append(sizes, sz)
			if sz.Height > maxRowH {
				maxRowH = sz.Height
			}
		}
		for i, s := range row {
			px := x
			if fullBleed {
				px = 0
			}
			items = append(items, placed{s: s, x: px, y: y, w: sizes[i].Width, h: sizes[i].Height})
			if !fullBleed {
				x += sizes[i].Width + colGap
			}
		}
		y += maxRowH + gap
	}

	layoutRow([]*spin.Spin{basic}, 800, false)
	layoutRow([]*spin.Spin{smallSpin, mediumSpin, largeSpin}, 800, false)
	layoutRow([]*spin.Spin{descSpin, tipSpin}, 800, false)
	layoutRow([]*spin.Spin{delaySpin, customSpin}, 800, false)
	layoutRow([]*spin.Spin{percentNum, percentAuto}, 800, false)
	layoutRow([]*spin.Spin{nestedSpin}, 800, false)
	layoutRow([]*spin.Spin{styleSpin}, 800, false)
	layoutRow([]*spin.Spin{fullSpin}, 800, true)
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: basic indicator carries non-white ink.
	b0 := items[0]
	ink := 0
	for yy := int(b0.y); yy < int(b0.y+b0.h); yy++ {
		for xx := int(b0.x); xx < int(b0.x+b0.w); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 250 || g/257 < 250 || b/257 < 250 {
				ink++
			}
		}
	}
	if ink < 20 {
		t.Fatalf("basic indicator ink=%d want >=20", ink)
	}

	// Pixel assertion 2: description zone carries real glyph ink.
	var descItem *placed
	for i := range items {
		if items[i].s == descSpin {
			descItem = &items[i]
			break
		}
	}
	if descItem == nil {
		t.Fatal("desc item missing")
	}
	indH := descSpin.DotSize()
	gapH := descSpin.EffectiveGap()
	textTop := int(descItem.y + indH + gapH)
	textBottom := int(descItem.y + descItem.h)
	if textBottom-textTop < 4 {
		t.Fatalf("desc zone too thin %d->%d", textTop, textBottom)
	}
	dark := 0
	for yy := textTop; yy < textBottom; yy++ {
		for xx := int(descItem.x); xx < int(descItem.x+descItem.w); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 250 || g/257 < 250 || b/257 < 250 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("description ink=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 3: fullscreen mask is dark-tinted.
	var fullItem *placed
	for i := range items {
		if items[i].s == fullSpin {
			fullItem = &items[i]
			break
		}
	}
	if fullItem == nil {
		t.Fatal("fullscreen item missing")
	}
	darkMask := 0
	midY := int(fullItem.y + fullItem.h/2)
	for xx := 0; xx < int(W); xx += 2 {
		r, g, b, _ := got.At(xx, midY).RGBA()
		if r/257 < 200 || g/257 < 200 || b/257 < 200 {
			darkMask++
		}
	}
	if darkMask < 50 {
		t.Fatalf("fullscreen mask dark=%d want >=50", darkMask)
	}

	path := filepath.Join("testdata", "showcase_spin.png")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("write showcase: %v", err)
		}
		if err := png.Encode(f, got); err != nil {
			f.Close()
			t.Fatalf("encode showcase: %v", err)
		}
		f.Close()
		t.Logf("showcase rewritten: %s (%dx%d)", path, int(W), int(H))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open showcase %s (run once with UPDATE_GOLDEN=1): %v", path, err)
	}
	want, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode showcase: %v", err)
	}
	if !want.Bounds().Eq(got.Bounds()) {
		t.Fatalf("showcase bounds %v want %v (regen with UPDATE_GOLDEN=1)", got.Bounds(), want.Bounds())
	}
	maxDiff := uint32(spec.Tolerance.MaxDiff * 257)
	hardCap := uint32(spec.Tolerance.HardCap * 257)
	bad := 0
	total := got.Bounds().Dx() * got.Bounds().Dy()
	for yy := 0; yy < got.Bounds().Dy(); yy++ {
		for xx := 0; xx < got.Bounds().Dx(); xx++ {
			r1, g1, b1, a1 := got.At(xx, yy).RGBA()
			r2, g2, b2, a2 := want.At(xx, yy).RGBA()
			m := max4spin(diffSpin(r1, r2), diffSpin(g1, g2), diffSpin(b1, b2), diffSpin(a1, a2))
			if m > hardCap {
				t.Fatalf("showcase pixel (%d,%d) diff %d exceeds hard cap", xx, yy, m/257)
			}
			if m > maxDiff {
				bad++
			}
		}
	}
	if float64(bad)/float64(total) > spec.Tolerance.BadFrac {
		t.Fatalf("showcase bad pixels %d/%d exceed %.1f%%", bad, total, spec.Tolerance.BadFrac*100)
	}
}

func diffSpin(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4spin(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
