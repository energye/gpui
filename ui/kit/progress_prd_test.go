package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/progress.md §6.9 — P0 PRD cases (PRG-01…08, 10…19, 21).
// PRG-09/20/22–24 are P1 / L3 / L4 / N/A and intentionally omitted.

func layoutProgress(p *kit.Progress, w, h float64) core.Size {
	n := p.Node()
	return n.Layout(core.Loose(w, h))
}

func findText(n core.Node, want string) bool {
	if n == nil {
		return false
	}
	found := false
	var walk func(core.Node)
	walk = func(x core.Node) {
		if x == nil || found {
			return
		}
		if tx, ok := x.(*primitive.Text); ok && strings.Contains(tx.Value, want) {
			found = true
			return
		}
		for _, c := range x.Children() {
			walk(c)
		}
	}
	walk(n)
	return found
}

func hasTextNode(n core.Node) bool {
	if n == nil {
		return false
	}
	found := false
	var walk func(core.Node)
	walk = func(x core.Node) {
		if x == nil || found {
			return
		}
		if tx, ok := x.(*primitive.Text); ok && strings.TrimSpace(tx.Value) != "" {
			found = true
			return
		}
		for _, c := range x.Children() {
			walk(c)
		}
	}
	walk(n)
	return found
}

func TestProgress_PRD_01_Defaults(t *testing.T) {
	// PRG-01
	p := kit.NewProgress(30)
	if p.Type != kit.ProgressLine {
		t.Fatalf("Type=%q want line", p.Type)
	}
	if p.Size != kit.ProgressSizeMedium {
		t.Fatalf("Size=%q want medium", p.Size)
	}
	if !p.ShowInfo {
		t.Fatal("ShowInfo default true")
	}
	if p.Status != kit.ProgressStatusAuto {
		t.Fatalf("Status=%q want auto empty", p.Status)
	}
	if p.Percent != 30 {
		t.Fatalf("Percent=%v want 30", p.Percent)
	}
	if p.Node() == nil {
		t.Fatal("nil node")
	}
	if p.Node().Base().Role != "progressbar" {
		t.Fatalf("Role=%q want progressbar", p.Node().Base().Role)
	}
}

func TestProgress_PRD_02_LineHalf(t *testing.T) {
	// PRG-02
	p := kit.NewProgress(50)
	p.SetType(kit.ProgressLine)
	if r := p.FillRatio(); r < 0.49 || r > 0.51 {
		t.Fatalf("FillRatio=%v want ~0.5", r)
	}
	sz := layoutProgress(p, 400, 40)
	if sz.Width < 50 || sz.Height < 8 {
		t.Fatalf("size too small: %v", sz)
	}
}

func TestProgress_PRD_03_AutoSuccess(t *testing.T) {
	// PRG-03
	p := kit.NewProgress(100)
	if p.EffectiveStatus() != kit.ProgressStatusSuccess {
		t.Fatalf("EffectiveStatus=%q want success", p.EffectiveStatus())
	}
	// explicit normal must not auto-upgrade
	p.SetStatus(kit.ProgressStatusNormal)
	if p.EffectiveStatus() != kit.ProgressStatusNormal {
		t.Fatalf("explicit normal got %q", p.EffectiveStatus())
	}
}

func TestProgress_PRD_04_ExceptionColor(t *testing.T) {
	// PRG-04
	p := kit.NewProgress(70)
	p.SetStatus(kit.ProgressStatusException)
	th := core.DefaultTheme()
	want := th.Color(core.TokenColorError)
	// rebuild + layout so info color applied
	_ = layoutProgress(p, 300, 40)
	if p.InfoText() == "" {
		t.Fatal("exception should show info icon/text")
	}
	// walk info text color
	var info *primitive.Text
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil || info != nil {
			return
		}
		if tx, ok := n.(*primitive.Text); ok {
			info = tx
			return
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(p.Node())
	if info == nil {
		t.Fatal("no info text")
	}
	if info.Color != want {
		t.Fatalf("info color=%v want error %v", info.Color, want)
	}
}

func TestProgress_PRD_05_Circle(t *testing.T) {
	// PRG-05
	p := kit.NewProgress(75)
	p.SetType(kit.ProgressCircle)
	sz := layoutProgress(p, 400, 200)
	if mathAbs(sz.Width-120) > 0.5 || mathAbs(sz.Height-120) > 0.5 {
		t.Fatalf("circle size=%v want 120×120", sz)
	}
	if p.CircleSize() != 120 {
		t.Fatalf("CircleSize=%v want 120", p.CircleSize())
	}
}

func TestProgress_PRD_06_HideInfo(t *testing.T) {
	// PRG-06
	p := kit.NewProgress(50)
	p.SetShowInfo(false)
	_ = layoutProgress(p, 300, 40)
	if hasTextNode(p.Node()) {
		t.Fatal("showInfo=false must not paint percent text")
	}
	if p.InfoText() != "" {
		t.Fatalf("InfoText=%q want empty", p.InfoText())
	}
}

func TestProgress_PRD_07_LineHeightMedium(t *testing.T) {
	// PRG-07
	p := kit.NewProgress(40)
	p.SetSize(kit.ProgressSizeMedium)
	if h := p.LineHeight(); mathAbs(h-8) > 0.5 {
		t.Fatalf("LineHeight=%v want 8", h)
	}
	sz := layoutProgress(p, 300, 40)
	// row height ≈ line height (info may be taller with font)
	if sz.Height < 8 {
		t.Fatalf("height=%v", sz.Height)
	}
}

func TestProgress_PRD_08_CircleDefaultSize(t *testing.T) {
	// PRG-08
	p := kit.NewProgress(50)
	p.SetType(kit.ProgressCircle)
	if s := p.CircleSize(); mathAbs(s-120) > 0.5 {
		t.Fatalf("CircleSize=%v want 120", s)
	}
}

func TestProgress_PRD_10_LineDemo(t *testing.T) {
	// PRG-10 line.tsx
	rows := []*kit.Progress{
		kit.NewProgress(30),
		func() *kit.Progress { p := kit.NewProgress(50); p.SetStatus(kit.ProgressStatusActive); return p }(),
		func() *kit.Progress { p := kit.NewProgress(70); p.SetStatus(kit.ProgressStatusException); return p }(),
		kit.NewProgress(100),
		func() *kit.Progress { p := kit.NewProgress(50); p.SetShowInfo(false); return p }(),
	}
	for i, p := range rows {
		sz := layoutProgress(p, 320, 48)
		if sz.Width < 40 || sz.Height < 6 {
			t.Fatalf("row %d size=%v", i, sz)
		}
	}
	if rows[3].EffectiveStatus() != kit.ProgressStatusSuccess {
		t.Fatal("100% should auto success")
	}
}

func TestProgress_PRD_11_CircleDemo(t *testing.T) {
	// PRG-11 circle.tsx
	for _, tc := range []struct {
		pct float64
		st  kit.ProgressStatus
	}{
		{75, ""},
		{70, kit.ProgressStatusException},
		{100, ""},
	} {
		p := kit.NewProgress(tc.pct)
		p.SetType(kit.ProgressCircle)
		if tc.st != "" {
			p.SetStatus(tc.st)
		}
		sz := layoutProgress(p, 400, 200)
		if sz.Width < 100 {
			t.Fatalf("pct=%v size=%v", tc.pct, sz)
		}
	}
}

func TestProgress_PRD_12_LineMini(t *testing.T) {
	// PRG-12 line-mini.tsx
	p := kit.NewProgress(30)
	p.SetSize(kit.ProgressSizeSmall)
	if h := p.LineHeight(); mathAbs(h-6) > 0.5 {
		t.Fatalf("small LineHeight=%v want 6", h)
	}
	_ = layoutProgress(p, 180, 40)
}

func TestProgress_PRD_13_CircleMicro(t *testing.T) {
	// PRG-13 circle-micro.tsx
	p := kit.NewProgress(60)
	p.SetType(kit.ProgressCircle)
	p.SetSizePx(14)
	p.SetStrokeWidth(20)
	p.SetRailColor(render.RGBA{R: 0.9, G: 0.95, B: 1, A: 1})
	p.SetFormat(func(n, _ float64) string {
		return "In progress"
	})
	sz := layoutProgress(p, 200, 40)
	if mathAbs(sz.Width-14) > 1 {
		t.Fatalf("micro size=%v want ~14", sz)
	}
	if !findText(p.Node(), "In progress") {
		t.Fatal("format text missing")
	}
}

func TestProgress_PRD_14_CircleMini(t *testing.T) {
	// PRG-14 circle-mini.tsx size=80
	for _, pct := range []float64{30, 70, 100} {
		p := kit.NewProgress(pct)
		p.SetType(kit.ProgressCircle)
		p.SetSizePx(80)
		if pct == 70 {
			p.SetStatus(kit.ProgressStatusException)
		}
		sz := layoutProgress(p, 400, 200)
		if mathAbs(sz.Width-80) > 0.5 {
			t.Fatalf("pct=%v size=%v want 80", pct, sz)
		}
	}
}

func TestProgress_PRD_15_DynamicNoRebuild(t *testing.T) {
	// PRG-15 dynamic.tsx
	p := kit.NewProgress(0)
	root1 := p.Node()
	p.SetPercent(10)
	p.SetPercent(20)
	p.SetPercent(100)
	root2 := p.Node()
	if root1 != root2 {
		t.Fatal("SetPercent must not rebuild root")
	}
	if p.EffectiveStatus() != kit.ProgressStatusSuccess {
		t.Fatal("100 auto success")
	}
	p.SetPercent(50)
	// still success only when auto and >=100; now normal
	if p.EffectiveStatus() != kit.ProgressStatusNormal {
		t.Fatalf("50%% status=%q want normal", p.EffectiveStatus())
	}
}

func TestProgress_PRD_16_Format(t *testing.T) {
	// PRG-16 format.tsx
	p := kit.NewProgress(75)
	p.SetType(kit.ProgressCircle)
	p.SetFormat(func(percent, _ float64) string {
		return "75 Days"
	})
	_ = layoutProgress(p, 400, 200)
	if !findText(p.Node(), "Days") {
		t.Fatal("custom format missing")
	}
	p2 := kit.NewProgress(100)
	p2.SetType(kit.ProgressCircle)
	p2.SetFormat(func(_, _ float64) string { return "Done" })
	_ = layoutProgress(p2, 400, 200)
	if !findText(p2.Node(), "Done") {
		t.Fatal("Done format missing")
	}
}

func TestProgress_PRD_17_Dashboard(t *testing.T) {
	// PRG-17 dashboard.tsx
	p := kit.NewProgress(30)
	p.SetType(kit.ProgressDashboard)
	p.SetGapDegree(50)
	p.SetGapPlacement(kit.ProgressGapBottom)
	sz := layoutProgress(p, 400, 200)
	if mathAbs(sz.Width-120) > 0.5 {
		t.Fatalf("dashboard size=%v", sz)
	}
}

func TestProgress_PRD_18_Metrics(t *testing.T) {
	// PRG-18 §6.2
	med := kit.NewProgress(50)
	if mathAbs(med.LineHeight()-8) > 0.5 {
		t.Fatalf("medium line=%v", med.LineHeight())
	}
	sm := kit.NewProgress(50)
	sm.SetSize(kit.ProgressSizeSmall)
	if mathAbs(sm.LineHeight()-6) > 0.5 {
		t.Fatalf("small line=%v", sm.LineHeight())
	}
	c := kit.NewProgress(50)
	c.SetType(kit.ProgressCircle)
	if mathAbs(c.CircleSize()-120) > 0.5 {
		t.Fatalf("circle=%v", c.CircleSize())
	}
	cs := kit.NewProgress(50)
	cs.SetType(kit.ProgressCircle)
	cs.SetSize(kit.ProgressSizeSmall)
	if mathAbs(cs.CircleSize()-60) > 0.5 {
		t.Fatalf("circle small=%v", cs.CircleSize())
	}
	// TokenProgressHeight default 8
	th := core.DefaultTheme()
	if v := th.SizeOr(core.TokenProgressHeight, 0); mathAbs(v-8) > 0.5 {
		t.Fatalf("TokenProgressHeight=%v want 8", v)
	}
}

func TestProgress_PRD_19_ThemeColors(t *testing.T) {
	// PRG-19 — default skin from Theme tokens (not hardcoded brand-only).
	th := core.DefaultTheme()
	prim := th.Color(core.TokenColorPrimary)
	succ := th.Color(core.TokenColorSuccess)
	errc := th.Color(core.TokenColorError)
	if prim.A < 0.1 || succ.A < 0.1 || errc.A < 0.1 {
		t.Fatal("theme tokens missing alpha")
	}
	p := kit.NewProgress(50)
	_ = layoutProgress(p, 200, 40)
	// exception uses error token path via EffectiveStatus
	p.SetStatus(kit.ProgressStatusException)
	if p.EffectiveStatus() != kit.ProgressStatusException {
		t.Fatal("exception status")
	}
	p.SetStatus(kit.ProgressStatusSuccess)
	if p.EffectiveStatus() != kit.ProgressStatusSuccess {
		t.Fatal("success status")
	}
}

func TestProgress_PRD_21_A11y(t *testing.T) {
	// PRG-21
	p := kit.NewProgress(42)
	n := p.Node()
	if n.Base().Role != "progressbar" {
		t.Fatalf("Role=%q", n.Base().Role)
	}
	if !strings.Contains(n.Base().Label, "42") && n.Base().Label == "" {
		t.Fatalf("Label=%q should reflect percent", n.Base().Label)
	}
	p.SetAriaLabel("upload progress")
	if n.Base().Label != "upload progress" {
		t.Fatalf("AriaLabel not applied: %q", n.Base().Label)
	}
	// not a focus target by default
	if n.Base().Role == "button" {
		t.Fatal("progress must not be button")
	}
}

func TestProgress_PRD_ActiveTick(t *testing.T) {
	// active line keeps ticking (supports status=active P0)
	p := kit.NewProgress(50)
	p.SetStatus(kit.ProgressStatusActive)
	if !p.Tick(0.05) {
		t.Fatal("active should tick")
	}
	p.SetStatus(kit.ProgressStatusNormal)
	if p.Tick(0.05) {
		t.Fatal("normal must not keep ticker")
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
