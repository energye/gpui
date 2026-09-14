package progress_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/progress"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type p1Cases struct {
	StepBlockMedium float64 `json:"stepBlockMedium"`
	StepBlockSmall  float64 `json:"stepBlockSmall"`
	StepGapDefault  float64 `json:"stepGapDefault"`
	StepGapCustom   float64 `json:"stepGapCustom"`
	StepsLine       struct {
		Steps   int     `json:"steps"`
		Percent float64 `json:"percent"`
		Active  int     `json:"active"`
	} `json:"stepsLine"`
	StepsRounding struct {
		Steps       int     `json:"steps"`
		Percent     float64 `json:"percent"`
		ActiveRound int     `json:"activeRound"`
		ActiveFloor int     `json:"activeFloor"`
	} `json:"stepsRounding"`
	LinecapDefault string `json:"linecapDefault"`
	Gradient       struct {
		From      string `json:"from"`
		To        string `json:"to"`
		Direction string `json:"direction"`
	} `json:"gradient"`
	PercentPositionDefault struct {
		Align string `json:"align"`
		Type  string `json:"type"`
	} `json:"percentPositionDefault"`
	SuccessSplit struct {
		Percent        float64 `json:"percent"`
		SuccessPercent float64 `json:"successPercent"`
	} `json:"successSplit"`
	CircleSteps struct {
		Count   int     `json:"count"`
		Gap     float64 `json:"gap"`
		Percent float64 `json:"percent"`
		Active  int     `json:"active"`
	} `json:"circleSteps"`
	DashboardSteps struct {
		Count   int     `json:"count"`
		Percent float64 `json:"percent"`
		Active  int     `json:"active"`
	} `json:"dashboardSteps"`
}

func loadP1Cases(t *testing.T) p1Cases {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "p1_cases.json"))
	if err != nil {
		t.Fatalf("read p1_cases.json: %v", err)
	}
	var c p1Cases
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("parse p1 cases: %v", err)
	}
	if c.StepsLine.Steps <= 0 {
		t.Fatal("bad p1 steps")
	}
	return c
}

// PRG-09 steps line: equal blocks + gap, active count via rounding.
func TestProgress_PRD_PRG09_StepsLine(t *testing.T) {
	c := loadP1Cases(t)
	p := progress.NewProgress(c.StepsLine.Percent)
	p.SetSteps(c.StepsLine.Steps)
	if p.Steps() != c.StepsLine.Steps {
		t.Fatalf("steps=%d", p.Steps())
	}
	if math.Abs(p.EffectiveStepGap()-c.StepGapDefault) > 1e-9 {
		t.Fatalf("gap=%v want %v", p.EffectiveStepGap(), c.StepGapDefault)
	}
	if p.ActiveSteps() != c.StepsLine.Active {
		t.Fatalf("active=%d want %d", p.ActiveSteps(), c.StepsLine.Active)
	}
	p.SetStepGap(c.StepGapCustom)
	if math.Abs(p.EffectiveStepGap()-c.StepGapCustom) > 1e-9 {
		t.Fatalf("custom gap=%v", p.EffectiveStepGap())
	}
	p.SetStepGap(c.StepGapDefault)
	sz := p.Layout(rendering.Loose(500, 100))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("steps layout=%+v", sz)
	}
	paintProgress(p, 260, 32)
	// Small preset uses narrower blocks.
	q := progress.NewProgress(50)
	q.SetSteps(3)
	q.SetSize(progress.SizeSmall)
	if math.Abs(q.EffectiveStepGap()-c.StepGapDefault) > 1e-9 {
		t.Fatalf("small gap=%v", q.EffectiveStepGap())
	}
	q.Layout(rendering.Loose(500, 100))
	paintProgress(q, 260, 32)
	// Block width probe from testdata: medium vs small totals differ by blocks.
	med := progress.NewProgress(50)
	med.SetSteps(c.StepsLine.Steps)
	med.SetSize(progress.SizeMedium)
	sm := progress.NewProgress(50)
	sm.SetSteps(c.StepsLine.Steps)
	sm.SetSize(progress.SizeSmall)
	medW := med.Layout(rendering.Loose(2000, 100)).Width
	smW := sm.Layout(rendering.Loose(2000, 100)).Width
	wantDelta := float64(c.StepsLine.Steps) * (c.StepBlockMedium - c.StepBlockSmall)
	if math.Abs((medW-smW)-wantDelta) > 2 {
		t.Fatalf("steps width delta=%v want %v (med %v sm %v)", medW-smW, wantDelta, medW, smW)
	}
	// Per-step array colors: first block uses entry 0.
	q.SetStepColors([]render.RGBA{{R: 0, G: 1, B: 0, A: 1}, {R: 1, G: 0, B: 0, A: 1}})
	if len(q.StepColors()) != 2 {
		t.Fatalf("step colors len=%d", len(q.StepColors()))
	}
	q.Layout(rendering.Loose(500, 100))
	paintProgress(q, 260, 32)
}

// Rounding hook for steps: default round, custom floor.
func TestProgress_PRD_PRG09_StepsRounding(t *testing.T) {
	c := loadP1Cases(t)
	p := progress.NewProgress(c.StepsRounding.Percent)
	p.SetSteps(c.StepsRounding.Steps)
	if p.ActiveSteps() != c.StepsRounding.ActiveRound {
		t.Fatalf("round active=%d want %d", p.ActiveSteps(), c.StepsRounding.ActiveRound)
	}
	p.SetRounding(func(v float64) float64 { return math.Floor(v) })
	if p.ActiveSteps() != c.StepsRounding.ActiveFloor {
		t.Fatalf("floor active=%d want %d", p.ActiveSteps(), c.StepsRounding.ActiveFloor)
	}
	p.SetRounding(nil)
	if p.ActiveSteps() != c.StepsRounding.ActiveRound {
		t.Fatalf("restored active=%d", p.ActiveSteps())
	}
}

// Linecap + gradient (line): butt/square sharp, from→to interpolation.
func TestProgress_PRD_PRG24_LinecapGradient(t *testing.T) {
	c := loadP1Cases(t)
	p := progress.NewProgress(75)
	if string(p.EffectiveStrokeLinecap()) != c.LinecapDefault {
		t.Fatalf("default cap=%q", p.EffectiveStrokeLinecap())
	}
	for _, lc := range []progress.StrokeLinecap{progress.LinecapButt, progress.LinecapSquare, progress.LinecapRound} {
		p.SetStrokeLinecap(lc)
		if p.EffectiveStrokeLinecap() != lc {
			t.Fatalf("cap=%q", p.EffectiveStrokeLinecap())
		}
		sz := p.Layout(rendering.Loose(500, 100))
		if sz.Width <= 0 {
			t.Fatalf("cap %q layout=%+v", lc, sz)
		}
		paintProgress(p, 200, 32)
	}
	from := theme.Hex(c.Gradient.From)
	to := theme.Hex(c.Gradient.To)
	p.SetStrokeLinecap(progress.LinecapRound)
	p.SetStrokeGradient(render.RGBA{R: from.R, G: from.G, B: from.B, A: from.A},
		render.RGBA{R: to.R, G: to.G, B: to.B, A: to.A}, c.Gradient.Direction)
	if !p.HasStrokeGradient() {
		t.Fatal("gradient active")
	}
	got := p.EffectiveFillColor()
	if math.Abs(got.R-from.R) > 1e-9 {
		t.Fatalf("gradient probe=%+v want from", got)
	}
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
	// Circle gradient paints without panic.
	cc := progress.NewProgress(90)
	cc.SetType(progress.TypeCircle)
	cc.SetStrokeGradient(render.RGBA{R: from.R, G: from.G, B: from.B, A: 1},
		render.RGBA{R: to.R, G: to.G, B: to.B, A: 1}, c.Gradient.Direction)
	cc.Layout(rendering.Loose(400, 400))
	paintProgress(cc, 160, 160)
	cc.SetStrokeGradient(render.RGBA{}, render.RGBA{}, "")
	if cc.HasStrokeGradient() {
		t.Fatal("gradient clear")
	}
}

// PercentPosition inner/outer/align matrix.
func TestProgress_PRD_PRG24_PercentPosition(t *testing.T) {
	c := loadP1Cases(t)
	if string(progress.AlignEnd) != c.PercentPositionDefault.Align || string(progress.PosOuter) != c.PercentPositionDefault.Type {
		t.Fatalf("defaults %q/%q", progress.AlignEnd, progress.PosOuter)
	}
	p := progress.NewProgress(50)
	if p.EffectivePercentAlign() != progress.AlignEnd || p.EffectivePercentPosition() != progress.PosOuter {
		t.Fatalf("default pos %v/%v", p.EffectivePercentAlign(), p.EffectivePercentPosition())
	}
	combos := []struct {
		align progress.PercentAlign
		typ   progress.PercentPosType
		inner bool
	}{
		{progress.AlignEnd, progress.PosOuter, false},
		{progress.AlignStart, progress.PosOuter, false},
		{progress.AlignCenter, progress.PosOuter, false},
		{progress.AlignStart, progress.PosInner, true},
		{progress.AlignCenter, progress.PosInner, true},
		{progress.AlignEnd, progress.PosInner, true},
	}
	for _, k := range combos {
		p.SetPercentPosition(k.align, k.typ)
		if p.EffectivePercentAlign() != k.align || p.EffectivePercentPosition() != k.typ {
			t.Fatalf("pos %v/%v", p.EffectivePercentAlign(), p.EffectivePercentPosition())
		}
		if p.IsInnerInfo() != k.inner {
			t.Fatalf("inner=%v want %v for %v/%v", p.IsInnerInfo(), k.inner, k.align, k.typ)
		}
		sz := p.Layout(rendering.Loose(500, 100))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("pos %v/%v layout=%+v", k.align, k.typ, sz)
		}
		if p.Node() == nil {
			t.Fatal("nil node")
		}
		// Inner forces percent text even for success status.
		p.SetStatus(progress.StatusSuccess)
		if k.inner && p.InfoText() == "✓" {
			t.Fatalf("inner must force percent text, got icon for %v", k.align)
		}
		p.SetStatus(progress.StatusAuto)
		paintProgress(p, 220, 40)
	}
	// Outer center stacks info below the rail (taller than track).
	p.SetPercentPosition(progress.AlignCenter, progress.PosOuter)
	sz := p.Layout(rendering.Loose(500, 200))
	if sz.Height <= p.LineHeight()+1 {
		t.Fatalf("outer-center height=%v must exceed track", sz.Height)
	}
}

// Success split: dual color + format second param.
func TestProgress_PRD_PRG24_SuccessSplit(t *testing.T) {
	c := loadP1Cases(t)
	p := progress.NewProgress(c.SuccessSplit.Percent)
	p.SetSuccessPercent(c.SuccessSplit.SuccessPercent)
	if math.Abs(p.EffectiveSuccessPercent()-c.SuccessSplit.SuccessPercent) > 1e-9 {
		t.Fatalf("success=%v", p.EffectiveSuccessPercent())
	}
	var gotSuccess float64 = -1
	p.SetFormat(func(percent, success float64) string {
		gotSuccess = success
		return "fmt"
	})
	if p.InfoText() != "fmt" {
		t.Fatalf("info=%q", p.InfoText())
	}
	if math.Abs(gotSuccess-c.SuccessSplit.SuccessPercent) > 1e-9 {
		t.Fatalf("format success=%v want %v", gotSuccess, c.SuccessSplit.SuccessPercent)
	}
	p.SetFormat(nil)
	// Success segment paints with success color by default.
	tok := theme.Default.Current()
	sc := p.EffectiveSuccessColor()
	if sc != (render.RGBA{R: tok.ColorSuccess.R, G: tok.ColorSuccess.G, B: tok.ColorSuccess.B, A: tok.ColorSuccess.A}) {
		t.Fatalf("success color=%+v", sc)
	}
	p.SetSuccessStrokeColor(render.RGBA{R: 1, G: 0, B: 0, A: 1})
	if p.EffectiveSuccessColor().G != 0 {
		t.Fatalf("override=%+v", p.EffectiveSuccessColor())
	}
	n0 := p.Node()
	p.Layout(rendering.Loose(500, 100))
	paintProgress(p, 200, 32)
	p.SetPercent(70)
	if p.Node() != n0 {
		t.Fatal("SetPercent with success must not rebuild root")
	}
	paintProgress(p, 200, 32)
	// Circle + dashboard success paint.
	for _, typ := range []progress.ProgressType{progress.TypeCircle, progress.TypeDashboard} {
		q := progress.NewProgress(60)
		q.SetType(typ)
		q.SetSuccessPercent(30)
		q.Layout(rendering.Loose(400, 400))
		paintProgress(q, 160, 160)
	}
}

// Circle/dashboard steps + gapPlacement alias compat.
func TestProgress_PRD_PRG24_CircleDashboardSteps(t *testing.T) {
	c := loadP1Cases(t)
	q := progress.NewProgress(c.CircleSteps.Percent)
	q.SetType(progress.TypeCircle)
	q.SetSteps(c.CircleSteps.Count)
	q.SetStepGap(c.CircleSteps.Gap)
	if q.ActiveSteps() != c.CircleSteps.Active {
		t.Fatalf("circle active=%d want %d", q.ActiveSteps(), c.CircleSteps.Active)
	}
	sz := q.Layout(rendering.Loose(400, 400))
	if sz.Width <= 0 {
		t.Fatalf("circle steps layout=%+v", sz)
	}
	paintProgress(q, 160, 160)
	d := progress.NewProgress(c.DashboardSteps.Percent)
	d.SetType(progress.TypeDashboard)
	d.SetSteps(c.DashboardSteps.Count)
	if d.ActiveSteps() != c.DashboardSteps.Active {
		t.Fatalf("dashboard active=%d want %d", d.ActiveSteps(), c.DashboardSteps.Active)
	}
	d.Layout(rendering.Loose(400, 400))
	paintProgress(d, 160, 160)
	// Deprecated gapPosition alias maps left/right to start/end.
	e := progress.NewProgress(40)
	e.SetType(progress.TypeDashboard)
	e.SetGapPosition("left")
	if e.EffectiveGapPlacement() != progress.GapStart {
		t.Fatalf("left->%v", e.EffectiveGapPlacement())
	}
	e.SetGapPosition("right")
	if e.EffectiveGapPlacement() != progress.GapEnd {
		t.Fatalf("right->%v", e.EffectiveGapPlacement())
	}
	e.Layout(rendering.Loose(400, 400))
	paintProgress(e, 160, 160)
}

// Deferred P1 depth: browser/React-only, no desktop mapping.
func TestProgress_PRD_PRG24_SemanticDepth_Staged(t *testing.T) {
	t.Skip("P1 staged: semantic classNames/styles function hooks are React CSS hooks with no desktop paint mapping (spec §6.8 分期)")
}

func TestProgress_PRD_PRG24_GlobalDefaults_Staged(t *testing.T) {
	t.Skip("P1 staged: ConfigProvider global progress defaults ride the React provider chain; kit uses SetProvider/SetTheme per node (spec §6.8 分期)")
}

func TestProgress_PRD_PRG24_PixelKeyframes_Staged(t *testing.T) {
	t.Skip("P1 staged: active CSS keyframes pixel motion is approximated by Ticker sweep; pixel-level keyframe equivalence is browser-only (spec §6.8 分期)")
}

func TestProgress_PRD_PRG24_DebugHash_Skipped(t *testing.T) {
	t.Skip("no desktop mapping: debug examples and ant.design逐像素哈希 are browser-only by spec §6.1 L4 (not a kit DoD gate)")
}
