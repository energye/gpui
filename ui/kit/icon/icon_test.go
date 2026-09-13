package icon_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/icon"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type iconsFile struct {
	DefaultSize float64 `json:"defaultSize"`
	Glyphs      []struct {
		Name    string `json:"name"`
		TwoTone bool   `json:"twoTone"`
	} `json:"glyphs"`
}

func loadIcons(t *testing.T) iconsFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "icons.json"))
	if err != nil {
		t.Fatalf("read icons.json: %v", err)
	}
	var f iconsFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse icons.json: %v", err)
	}
	if len(f.Glyphs) == 0 {
		t.Fatal("empty glyph list")
	}
	return f
}

func layoutEdge(t *testing.T, ic *icon.Icon, max float64) (w, h float64) {
	t.Helper()
	sz := ic.Layout(rendering.Loose(max, max))
	return sz.Width, sz.Height
}

func paintOK(ic *icon.Icon, size int) {
	dc := render.NewContext(size, size)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	ic.Node().Paint(pc)
}

func TestIcon_P0RegistryMatchesFile(t *testing.T) {
	f := loadIcons(t)
	if f.DefaultSize != icon.DefaultIconSize {
		t.Fatalf("defaultSize=%v want %v", f.DefaultSize, icon.DefaultIconSize)
	}
	for _, g := range f.Glyphs {
		ic := icon.NewIcon(g.Name)
		if !ic.Known() {
			t.Fatalf("%s should be known", g.Name)
		}
	}
}

func TestIcon_PRD_ICO01(t *testing.T) {
	ic := icon.NewIcon("check")
	if ic.EffectiveSize() != 16 || ic.Spin() || ic.Rotate() != 0 {
		t.Fatalf("defaults size=%v spin=%v rotate=%v", ic.EffectiveSize(), ic.Spin(), ic.Rotate())
	}
	if !ic.Decorative() || ic.Role() != "" || ic.Focusable() {
		t.Fatal("decorative defaults")
	}
}

func TestIcon_PRD_ICO02(t *testing.T) {
	ic := icon.NewIcon("check")
	if !ic.Known() {
		t.Fatal("check should be known")
	}
	w, h := layoutEdge(t, ic, 16)
	if math.Abs(w-16) > 0.5 || math.Abs(h-16) > 0.5 {
		t.Fatalf("layout=%vx%v", w, h)
	}
	paintOK(ic, 32)
}

func TestIcon_PRD_ICO03(t *testing.T) {
	ic := icon.NewIcon("no-such-icon-xyz")
	if ic.Known() {
		t.Fatal("unknown should not be known")
	}
	w, h := layoutEdge(t, ic, 16)
	if math.Abs(w-16) > 0.5 || math.Abs(h-16) > 0.5 {
		t.Fatalf("unknown layout=%vx%v", w, h)
	}
	paintOK(ic, 32)
}

func TestIcon_PRD_ICO04(t *testing.T) {
	ic := icon.NewIcon("check")
	ic.SetSize(24)
	w, h := layoutEdge(t, ic, 32)
	if math.Abs(w-24) > 0.5 || math.Abs(h-24) > 0.5 {
		t.Fatalf("layout=%vx%v want 24", w, h)
	}
}

func TestIcon_PRD_ICO05(t *testing.T) {
	ic := icon.NewIcon("sync")
	ic.SetSpin(true)
	a0 := ic.Angle()
	ic.Tick(0.25)
	if ic.Phase() <= 0 || ic.Angle() == a0 {
		t.Fatalf("phase=%v angle %v->%v", ic.Phase(), a0, ic.Angle())
	}
	if !ic.WantsFrame() {
		t.Fatal("spinning icon should want frames")
	}
}

func TestIcon_PRD_ICO06(t *testing.T) {
	ic := icon.NewIcon("sync")
	ic.SetSpin(true)
	ic.SetReduceMotion(true)
	ic.Tick(0.5)
	if ic.Phase() != 0 {
		t.Fatalf("reduced-motion phase=%v want 0", ic.Phase())
	}
	if ic.WantsFrame() {
		t.Fatal("reduced-motion should not want frames")
	}
}

func TestIcon_PRD_ICO07(t *testing.T) {
	ic := icon.NewIcon("check")
	ic.SetRotate(180)
	if ic.Rotate() != 180 || math.Abs(ic.EffectiveAngle()-180) > 1e-9 {
		t.Fatalf("rotate=%v angle=%v", ic.Rotate(), ic.EffectiveAngle())
	}
}

func TestIcon_PRD_ICO08(t *testing.T) {
	ic := icon.NewIcon("check")
	c := render.RGBA{R: 1, G: 0, B: 0, A: 1}
	ic.SetColor(c)
	if got := ic.EffectiveColor(); got != c {
		t.Fatalf("color=%+v want %+v", got, c)
	}
}

func TestIcon_PRD_ICO09(t *testing.T) {
	ic := icon.NewIcon("check")
	if !ic.Decorative() || ic.Focusable() || ic.Role() != "" {
		t.Fatal("decorative default: no tab, no role")
	}
}

func TestIcon_PRD_ICO10(t *testing.T) {
	f := loadIcons(t)
	want := map[string]bool{"home": true, "setting": true, "smile": true, "sync": true, "loading": true}
	n := 0
	for _, g := range f.Glyphs {
		if !want[g.Name] {
			continue
		}
		n++
		ic := icon.NewIcon(g.Name)
		ic.SetSpin(g.Name == "sync" || g.Name == "loading")
		ic.SetRotate(15)
		if !ic.Known() {
			t.Fatalf("%s unknown", g.Name)
		}
		layoutEdge(t, ic, 16)
		paintOK(ic, 32)
	}
	if n != 5 {
		t.Fatalf("basic set=%d want 5", n)
	}
}

func TestIcon_PRD_ICO11(t *testing.T) {
	ic := icon.NewIcon("smile")
	p := render.RGBA{R: 0.09, G: 0.47, B: 1, A: 1}
	ic.SetTwoToneColor(p)
	got, _ := ic.TwoToneColors()
	if got != p {
		t.Fatalf("twotone=%+v want %+v", got, p)
	}
	s := render.RGBA{R: 0, G: 0, B: 0, A: 0.2}
	ic.SetTwoToneColors(p, s)
	gotP, gotS := ic.TwoToneColors()
	if gotP != p || gotS != s {
		t.Fatalf("pair=%+v/%+v", gotP, gotS)
	}
	paintOK(ic, 32)
}

func TestIcon_PRD_ICO12(t *testing.T) {
	ic := icon.NewIcon("check")
	called := false
	ic.SetPainter(func(pc *rendering.PaintContext, size float64, _, _ render.RGBA) {
		called = true
		rendering.FillCircle(pc, size/2, size/2, size/4, 1, 0, 0, 1)
	})
	ic.SetSize(20)
	w, _ := layoutEdge(t, ic, 32)
	if math.Abs(w-20) > 0.5 {
		t.Fatalf("custom layout=%v", w)
	}
	paintOK(ic, 32)
	if !called {
		t.Fatal("painter not invoked")
	}
}

func TestIcon_PRD_ICO13(t *testing.T) {
	fam := icon.CreateFromIconfont(icon.IconfontOptions{Sources: []string{"ico-w0-font"}})
	fam.Register("ico-w0-home", icon.Def{Tag: "ico-w0-font"})
	ic := fam.NewIcon("ico-w0-home")
	if !ic.Known() {
		t.Fatal("iconfont type should resolve")
	}
	paintOK(ic, 32)
}

func TestIcon_PRD_ICO14(t *testing.T) {
	icon.RegisterIconSource("ico-w0-src-a", map[string]icon.Def{"ico-w0-dup": {Tag: "a"}})
	icon.RegisterIconSource("ico-w0-src-b", map[string]icon.Def{"ico-w0-dup": {Tag: "b"}})
	ic := icon.NewIcon("ico-w0-dup")
	if !ic.Known() || ic.DefTag() != "b" {
		t.Fatalf("later source should win, tag=%q", ic.DefTag())
	}
}

func TestIcon_PRD_ICO15(t *testing.T) {
	ic := icon.NewIcon("star")
	w, h := layoutEdge(t, ic, 64)
	if math.Abs(w-16) > 0.5 || math.Abs(h-16) > 0.5 {
		t.Fatalf("default layout=%vx%v want 16", w, h)
	}
}

func TestIcon_PRD_ICO16(t *testing.T) {
	ic := icon.NewIcon("star")
	tok := theme.Default.Current()
	want := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: tok.ColorText.A}
	if got := ic.EffectiveColor(); got != want {
		t.Fatalf("default color=%+v want %+v", got, want)
	}
}

func TestIcon_PRD_ICO17(t *testing.T) {
	ic := icon.NewIcon("star")
	ic.SetDisabled(true)
	tok := theme.Default.Current()
	want := render.RGBA{R: tok.ColorTextDisabled.R, G: tok.ColorTextDisabled.G, B: tok.ColorTextDisabled.B, A: tok.ColorTextDisabled.A}
	if got := ic.EffectiveColor(); got != want {
		t.Fatalf("disabled color=%+v want %+v", got, want)
	}
}

func TestIcon_PRD_ICO18(t *testing.T) {
	ic := icon.NewIcon("search")
	ic.SetAriaLabel("search")
	if ic.Role() != "img" || ic.AriaLabel() != "search" {
		t.Fatalf("role=%q label=%q", ic.Role(), ic.AriaLabel())
	}
}
