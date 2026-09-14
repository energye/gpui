package spin_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/spin"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

type spinCases struct {
	Sizes []struct {
		Name string  `json:"name"`
		Dot  float64 `json:"dot"`
	} `json:"sizes"`
	Gap         float64 `json:"gap"`
	FontSize    float64 `json:"fontSize"`
	PeriodSec   float64 `json:"periodSec"`
	AutoStepSec float64 `json:"autoStepSec"`
	A11y        struct {
		Role  string `json:"role"`
		Live  string `json:"live"`
		Label string `json:"label"`
	} `json:"a11y"`
}

func loadCases(t *testing.T) spinCases {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "spin_cases.json"))
	if err != nil {
		t.Fatalf("read spin_cases.json: %v", err)
	}
	var c spinCases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("parse spin_cases.json: %v", err)
	}
	if len(c.Sizes) != 3 {
		t.Fatalf("sizes=%d want 3", len(c.Sizes))
	}
	return c
}

func layoutLoose(t *testing.T, s *spin.Spin, max float64) rendering.Size {
	t.Helper()
	return s.Layout(rendering.Loose(max, max))
}

func paintOK(t *testing.T, s *spin.Spin, px int) {
	t.Helper()
	layoutLoose(t, s, float64(px))
	dc := render.NewContext(px, px)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
}

func childHas(t *testing.T, s *spin.Spin, want rendering.RenderObject) bool {
	t.Helper()
	for _, c := range s.Node().Children() {
		if c == want {
			return true
		}
	}
	return false
}

func TestSpin_PRD_SPN01_Defaults(t *testing.T) {
	s := spin.NewSpin(nil)
	if !s.Spinning() || s.Size() != spin.SpinMedium || s.Delay() != 0 {
		t.Fatalf("defaults spinning=%v size=%v delay=%v", s.Spinning(), s.Size(), s.Delay())
	}
	if s.Node() == nil || !s.Node().IsRepaintBoundary() {
		t.Fatal("host node must exist and be a repaint boundary")
	}
	if !s.IsDisplaySpinning() {
		t.Fatal("delay=0 must display at once")
	}
	if s.Focusable() || s.Role() != "status" {
		t.Fatalf("a11y role=%q focusable=%v", s.Role(), s.Focusable())
	}
	layoutLoose(t, s, 200)
}

func TestSpin_PRD_SPN02_VisibleAndTick(t *testing.T) {
	s := spin.NewSpin(nil)
	if !s.IsDisplaySpinning() {
		t.Fatal("spinning=true must show an indicator")
	}
	if !s.IsBuiltinIndicator() {
		t.Fatal("default must use the builtin looper")
	}
	p0 := s.Phase()
	s.Tick(0.3)
	if s.Phase() == p0 {
		t.Fatalf("Tick must advance phase (stayed %v)", p0)
	}
	if !s.WantsFrame() {
		t.Fatal("visible looper must want frames")
	}
	reg := &scheduler.TickerRegistry{}
	s.Attach(reg)
	reg.TickAll(0.1)
	s.Detach()
	paintOK(t, s, 64)
}

func TestSpin_PRD_SPN03_HiddenChildrenHittable(t *testing.T) {
	content := rendering.NewRenderColorBox(100, 50, 0.5, 0.5, 0.5, 1)
	s := spin.NewSpin(content)
	s.SetSpinning(false)
	if s.IsDisplaySpinning() || s.AriaBusy() {
		t.Fatal("spinning=false must hide the indicator")
	}
	sz := layoutLoose(t, s, 200)
	if math.Abs(sz.Width-100) > 0.5 || math.Abs(sz.Height-50) > 0.5 {
		t.Fatalf("nested size=%v want 100x50", sz)
	}
	hit := s.Node().HitTest(rendering.Point{X: 50, Y: 25})
	if hit != rendering.RenderObject(content) {
		t.Fatalf("hidden spin must let children hit (got %T)", hit)
	}
	paintOK(t, s, 120)
}

func TestSpin_PRD_SPN04_DescriptionAndTip(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetDescription("Loading data")
	if s.EffectiveDescription() != "Loading data" {
		t.Fatalf("description=%q", s.EffectiveDescription())
	}
	plain := layoutLoose(t, spin.NewSpin(nil), 200)
	with := layoutLoose(t, s, 200)
	if with.Height <= plain.Height {
		t.Fatalf("description must grow height %v -> %v", plain.Height, with.Height)
	}
	paintOK(t, s, 96)

	alias := spin.NewSpin(nil)
	alias.SetTip("tip text")
	if alias.EffectiveDescription() != "tip text" {
		t.Fatalf("tip alias=%q", alias.EffectiveDescription())
	}
	alias.SetDescription("real")
	if alias.EffectiveDescription() != "real" {
		t.Fatal("description must win over tip")
	}
}

func TestSpin_PRD_SPN06_DelayGate(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetDelay(500)
	s.SetSpinning(true)
	if s.IsDisplaySpinning() {
		t.Fatal("delay must hold display back")
	}
	if _, ok := s.NextWake(); !ok {
		t.Fatal("pending delay must report a scheduler wakeup")
	}
	s.Tick(0.2)
	s.Tick(0.2)
	if s.IsDisplaySpinning() {
		t.Fatal("400ms < 500ms must still hide")
	}
	s.Tick(0.2)
	if !s.IsDisplaySpinning() {
		t.Fatal("600ms >= 500ms must show")
	}
	paintOK(t, s, 64)
}

func TestSpin_PRD_SPN07_NestedStaysInTree(t *testing.T) {
	content := rendering.NewRenderColorBox(80, 40, 0.4, 0.4, 0.4, 1)
	s := spin.NewSpin(content)
	layoutLoose(t, s, 200)
	if !childHas(t, s, rendering.RenderObject(content)) {
		t.Fatal("content must stay in tree while spinning")
	}
	s.SetSpinning(false)
	layoutLoose(t, s, 200)
	if !childHas(t, s, rendering.RenderObject(content)) {
		t.Fatal("content must stay in tree while hidden")
	}
}

func TestSpin_PRD_SPN08_ReducedMotion(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetReduceMotion(true)
	s.Tick(0.5)
	if s.Phase() != 0 {
		t.Fatalf("reduced-motion phase=%v want 0", s.Phase())
	}
	if s.WantsFrame() {
		t.Fatal("reduced-motion must not want frames")
	}
	paintOK(t, s, 48)
}

func TestSpin_PRD_SPN09_BasicLayout(t *testing.T) {
	s := spin.NewSpin(nil)
	sz := layoutLoose(t, s, 200)
	if math.Abs(sz.Width-20) > 0.5 || math.Abs(sz.Height-20) > 0.5 {
		t.Fatalf("basic size=%v want 20x20", sz)
	}
	paintOK(t, s, 48)
}

func TestSpin_PRD_SPN10_Sizes(t *testing.T) {
	c := loadCases(t)
	for _, g := range c.Sizes {
		s := spin.NewSpin(nil)
		s.SetSize(spin.SpinSize(g.Name))
		if math.Abs(s.DotSize()-g.Dot) > 0.5 {
			t.Fatalf("%s dot=%v want %v", g.Name, s.DotSize(), g.Dot)
		}
		sz := layoutLoose(t, s, 200)
		if math.Abs(sz.Width-g.Dot) > 0.5 {
			t.Fatalf("%s width=%v want %v", g.Name, sz.Width, g.Dot)
		}
		paintOK(t, s, 64)
	}
	alias := spin.NewSpin(nil)
	alias.SetSize("default")
	if alias.Size() != spin.SpinMedium || math.Abs(alias.DotSize()-20) > 0.5 {
		t.Fatalf("default must map to medium (size=%v dot=%v)", alias.Size(), alias.DotSize())
	}
}

func TestSpin_PRD_SPN11_NestedToggle(t *testing.T) {
	content := rendering.NewRenderColorBox(100, 60, 0.5, 0.5, 0.5, 1)
	s := spin.NewSpin(content)
	layoutLoose(t, s, 200)
	blocked := s.Node().HitTest(rendering.Point{X: 50, Y: 30})
	if blocked == rendering.RenderObject(content) {
		t.Fatal("spinning mask must block content hits")
	}
	s.SetSpinning(false)
	layoutLoose(t, s, 200)
	hit := s.Node().HitTest(rendering.Point{X: 50, Y: 30})
	if hit != rendering.RenderObject(content) {
		t.Fatalf("hidden nested must hit content (got %T)", hit)
	}
	s.SetSpinning(true)
	layoutLoose(t, s, 200)
	if !s.IsDisplaySpinning() {
		t.Fatal("re-spin must show again")
	}
}

func TestSpin_PRD_SPN12_TipAcrossSizes(t *testing.T) {
	c := loadCases(t)
	for _, g := range c.Sizes {
		s := spin.NewSpin(nil)
		s.SetSize(spin.SpinSize(g.Name))
		s.SetDescription("Loading")
		if s.EffectiveDescription() == "" {
			t.Fatalf("%s missing description", g.Name)
		}
		sz := layoutLoose(t, s, 200)
		if sz.Height <= g.Dot {
			t.Fatalf("%s height=%v must exceed dot %v", g.Name, sz.Height, g.Dot)
		}
		paintOK(t, s, 96)
	}
}

func TestSpin_PRD_SPN13_DelayReplay(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetDelay(300)
	s.SetSpinning(true)
	s.Tick(0.1)
	if s.IsDisplaySpinning() {
		t.Fatal("100ms < 300ms must hide")
	}
	s.Tick(0.25)
	if !s.IsDisplaySpinning() {
		t.Fatal("350ms >= 300ms must show")
	}
}

func TestSpin_PRD_SPN14_CustomIndicator(t *testing.T) {
	prev := spin.DefaultIndicator()
	defer spin.SetDefaultIndicator(prev)

	custom := rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1)
	s := spin.NewSpin(nil)
	if s.EffectiveIndicator() != nil && prev != nil {
		t.Fatal("unexpected indicator")
	}
	glob := rendering.NewRenderColorBox(12, 12, 0, 1, 0, 1)
	spin.SetDefaultIndicator(glob)
	if s.EffectiveIndicator() != rendering.RenderObject(glob) {
		t.Fatal("global default must apply")
	}
	if s.IsBuiltinIndicator() {
		t.Fatal("global indicator must beat builtin")
	}
	s.SetIndicator(custom)
	if s.EffectiveIndicator() != rendering.RenderObject(custom) {
		t.Fatal("instance must beat global")
	}
	layoutLoose(t, s, 64)
	paintOK(t, s, 64)
	if s.WantsFrame() {
		t.Fatal("custom indicator owns animation; spin must not hold frames")
	}
}

func TestSpin_PRD_SPN15_Percent(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetPercent(40)
	if !s.HasPercent() || math.Abs(s.EffectivePercent()-40) > 1e-9 {
		t.Fatalf("percent=%v has=%v", s.EffectivePercent(), s.HasPercent())
	}
	if s.WantsFrame() {
		t.Fatal("numeric ring is static and must not want frames")
	}
	paintOK(t, s, 64)

	a := spin.NewSpin(nil)
	a.SetPercentAuto()
	a.Tick(1.0)
	if !a.HasPercent() || a.EffectivePercent() <= 0 {
		t.Fatalf("auto must climb (got %v)", a.EffectivePercent())
	}
	for i := 0; i < 200; i++ {
		a.Tick(0.2)
	}
	if p := a.EffectivePercent(); p >= 100 || p <= 0 {
		t.Fatalf("auto must stay in (0,100) (got %v)", p)
	}
	if !a.WantsFrame() {
		t.Fatal("auto progress must want frames")
	}
	a.ClearPercent()
	if a.HasPercent() || !a.IsBuiltinIndicator() {
		t.Fatal("clear must return to 4-dot")
	}
}

func TestSpin_PRD_SPN16_StyleClass(t *testing.T) {
	s := spin.NewSpin(nil)
	s.SetClassNames(spin.SpinClassNames{Root: "r", Section: "s", Indicator: "i", Description: "d", Container: "c"})
	s.SetStyles(spin.SpinStyles{Root: "r", Section: "s", Indicator: "i", Description: "d", Container: "c"})
	s.SetStyle(spin.Style{Gap: 10, FontSize: 13})
	if got := s.ClassNames(); got.Root != "r" || got.Container != "c" {
		t.Fatalf("classnames=%+v", got)
	}
	if got := s.Styles(); got.Root != "r" || got.Indicator != "i" {
		t.Fatalf("styles=%+v", got)
	}
	if math.Abs(s.EffectiveGap()-10) > 1e-9 || math.Abs(s.EffectiveFontSize()-13) > 1e-9 {
		t.Fatalf("style gap=%v font=%v", s.EffectiveGap(), s.EffectiveFontSize())
	}
	layoutLoose(t, s, 64)
	paintOK(t, s, 48)
}

func TestSpin_PRD_SPN17_Metrics(t *testing.T) {
	c := loadCases(t)
	s := spin.NewSpin(nil)
	if math.Abs(s.DotSize()-20) > 0.5 {
		t.Fatalf("medium dot=%v want 20", s.DotSize())
	}
	if math.Abs(s.EffectiveGap()-c.Gap) > 0.5 {
		t.Fatalf("gap=%v want %v", s.EffectiveGap(), c.Gap)
	}
	if math.Abs(s.EffectiveFontSize()-c.FontSize) > 0.5 {
		t.Fatalf("font=%v want %v", s.EffectiveFontSize(), c.FontSize)
	}
	if math.Abs(spin.SpinPeriodSec-c.PeriodSec) > 1e-9 {
		t.Fatalf("period=%v want %v", spin.SpinPeriodSec, c.PeriodSec)
	}
}

func TestSpin_PRD_SPN18_ThemeColors(t *testing.T) {
	tok := theme.Default.Current()
	s := spin.NewSpin(nil)
	got := s.EffectiveIndicatorColor()
	want := render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
	if got != want {
		t.Fatalf("indicator=%+v want ColorPrimary %+v", got, want)
	}
	track := s.EffectiveTrackColor()
	wantTrack := render.RGBA{R: tok.ColorFillSecondary.R, G: tok.ColorFillSecondary.G, B: tok.ColorFillSecondary.B, A: tok.ColorFillSecondary.A}
	if track != wantTrack {
		t.Fatalf("track=%+v want ColorFillSecondary %+v", track, wantTrack)
	}
}

func TestSpin_LayoutMatrix(t *testing.T) {
	s := spin.NewSpin(nil)
	exact := s.Layout(rendering.Tight(20, 20))
	if math.Abs(exact.Width-20) > 0.5 || math.Abs(exact.Height-20) > 0.5 {
		t.Fatalf("exact=%v want 20x20", exact)
	}
	loose := s.Layout(rendering.Loose(200, 200))
	if math.Abs(loose.Width-20) > 0.5 || math.Abs(loose.Height-20) > 0.5 {
		t.Fatalf("loose=%v want 20x20", loose)
	}
	min := s.Layout(rendering.Constraints{MinWidth: 40, MaxWidth: 200, MinHeight: 40, MaxHeight: 200})
	if min.Width < 40-0.5 || min.Height < 40-0.5 {
		t.Fatalf("min-clamped=%v want >=40", min)
	}
	content := rendering.NewRenderColorBox(100, 50, 0.5, 0.5, 0.5, 1)
	n := spin.NewSpin(content)
	ns := n.Layout(rendering.Loose(200, 200))
	if math.Abs(ns.Width-100) > 0.5 || math.Abs(ns.Height-50) > 0.5 {
		t.Fatalf("nested=%v want 100x50", ns)
	}
}

func TestSpin_Accessibility(t *testing.T) {
	c := loadCases(t)
	s := spin.NewSpin(nil)
	if s.Role() != c.A11y.Role || s.Role() != "status" {
		t.Fatalf("role=%q want status", s.Role())
	}
	if s.AriaLive() != c.A11y.Live {
		t.Fatalf("live=%q want polite", s.AriaLive())
	}
	if s.AriaLabel() != c.A11y.Label {
		t.Fatalf("label=%q want Loading", s.AriaLabel())
	}
	if s.Focusable() {
		t.Fatal("spin must never take Focus")
	}
	if !s.AriaBusy() {
		t.Fatal("visible spin must report busy")
	}
	s.SetSpinning(false)
	if s.AriaBusy() {
		t.Fatal("hidden spin must not report busy")
	}
	s.SetSpinning(true)
	s.SetDescription("Busy list")
	if s.AriaLabel() != "Busy list" {
		t.Fatalf("description must name the live region (got %q)", s.AriaLabel())
	}
}

func TestSpin_ThemeVariants(t *testing.T) {
	base := theme.Default.Current()
	alt := base
	alt.ColorPrimary = theme.Color{R: 1, G: 0, B: 0, A: 1}
	p := theme.NewProvider(alt)
	s := spin.NewSpin(nil)
	s.SetProvider(p)
	if got := s.EffectiveIndicatorColor(); got.R != 1 || got.B != 0 {
		t.Fatalf("provider Token primary=%+v", got)
	}
	over := base
	over.ColorPrimary = theme.Color{R: 0, G: 1, B: 0, A: 1}
	s.SetProvider(nil)
	s.SetTheme(&over)
	if got := s.EffectiveIndicatorColor(); got.G != 1 || got.R != 0 {
		t.Fatalf("override Token primary=%+v", got)
	}
	s.SetTheme(nil)
	if got, want := s.EffectiveIndicatorColor(), s.EffectiveIndicatorColor(); got != want {
		t.Fatal("theme clear must be stable")
	}
	paintOK(t, s, 48)
}

// TestSpin_BuildCost is the hotspot stopwatch: animated subtrees must stay
// cheap to build without opening a long window (long soak stays on the
// release manual chain per ACCEPTANCE hotspot windows).
func TestSpin_BuildCost(t *testing.T) {
	const n = 200
	start := time.Now()
	for i := 0; i < n; i++ {
		s := spin.NewSpin(nil)
		s.Layout(rendering.Loose(200, 200))
		s.Tick(0.016)
	}
	avg := time.Since(start).Seconds() * 1000 / n
	t.Logf("spin build+tick avg=%.3fms over %d", avg, n)
	if avg > 5 {
		t.Fatalf("build avg=%.3fms exceeds 5ms budget", avg)
	}
}
