package statistic_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/statistic"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

type statisticFile struct {
	Grouping []struct {
		Value    float64 `json:"value"`
		Expected string  `json:"expected"`
	} `json:"grouping"`
	Precision []struct {
		Value     float64 `json:"value"`
		Precision int     `json:"precision"`
		Expected  string  `json:"expected"`
	} `json:"precision"`
	Separators []struct {
		Value    float64 `json:"value"`
		Decimal  string  `json:"decimal"`
		Group    string  `json:"group"`
		Expected string  `json:"expected"`
	} `json:"separators"`
	Timer []struct {
		DiffMs   int64  `json:"diffMs"`
		Format   string `json:"format"`
		Expected string `json:"expected"`
	} `json:"timer"`
}

func loadStatisticFile(t *testing.T) statisticFile {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "statistic.json"))
	if err != nil {
		t.Fatalf("read statistic.json: %v", err)
	}
	var f statisticFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parse statistic.json: %v", err)
	}
	if len(f.Grouping) == 0 || len(f.Precision) == 0 || len(f.Timer) == 0 {
		t.Fatal("statistic.json missing cases")
	}
	return f
}

func layoutSize(t *testing.T, s *statistic.Statistic, c rendering.Constraints) rendering.Size {
	t.Helper()
	return s.Layout(c)
}

// STA-01: defaults mirror §6.10.
func TestStatistic_PRD_STA01(t *testing.T) {
	s := statistic.NewStatistic()
	if s.Value() != 0 {
		t.Fatalf("value=%v want 0", s.Value())
	}
	if s.IsLoading() {
		t.Fatal("loading default false")
	}
	if s.DecimalSeparator() != "." || s.GroupSeparator() != "," {
		t.Fatalf("seps=%q/%q", s.DecimalSeparator(), s.GroupSeparator())
	}
	if s.DisplayText() != "0" {
		t.Fatalf("display=%q want 0", s.DisplayText())
	}
	if s.TimerType() != statistic.TimerNone || s.Finished() {
		t.Fatal("timer default none/unfinished")
	}
	sz := layoutSize(t, s, rendering.Loose(800, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%+v", sz)
	}
}

// STA-02: builtin grouping comes from testdata.
func TestStatistic_PRD_STA02(t *testing.T) {
	f := loadStatisticFile(t)
	for _, g := range f.Grouping {
		s := statistic.NewStatistic()
		s.SetValue(g.Value)
		if got := s.DisplayText(); got != g.Expected {
			t.Fatalf("value=%v got %q want %q", g.Value, got, g.Expected)
		}
		layoutSize(t, s, rendering.Loose(800, 200))
	}
}

// STA-03: precision pads/truncates per testdata.
func TestStatistic_PRD_STA03(t *testing.T) {
	f := loadStatisticFile(t)
	for _, p := range f.Precision {
		s := statistic.NewStatistic()
		s.SetValue(p.Value)
		s.SetPrecision(p.Precision)
		if got := s.DisplayText(); got != p.Expected {
			t.Fatalf("value=%v prec=%d got %q want %q", p.Value, p.Precision, got, p.Expected)
		}
	}
	s := statistic.NewStatistic()
	s.SetValue(9.3)
	s.SetPrecision(2)
	if got := s.DisplayText(); got != "9.30" {
		t.Fatalf("pad got %q", got)
	}
	s.ClearPrecision()
	if s.HasPrecision() {
		t.Fatal("clear precision")
	}
}

// STA-04: prefix/suffix widen layout by the §6.2 gap.
func TestStatistic_PRD_STA04(t *testing.T) {
	plain := statistic.NewStatistic()
	plain.SetValue(93)
	base := layoutSize(t, plain, rendering.Loose(800, 200))
	s := statistic.NewStatistic()
	s.SetValue(93)
	s.SetPrefix("pre")
	s.SetSuffix("/ 100")
	if s.Prefix() != "pre" || s.Suffix() != "/ 100" {
		t.Fatal("prefix/suffix getters")
	}
	if got := s.FullContentText(); got != "pre93/ 100" {
		t.Fatalf("full=%q", got)
	}
	wide := layoutSize(t, s, rendering.Loose(800, 200))
	if wide.Width <= base.Width+2*3.5 {
		t.Fatalf("affix width %v should exceed base %v by gaps", wide.Width, base.Width)
	}
}

// STA-05: countdown fires onFinish exactly once and sticks at zero.
func TestStatistic_PRD_STA05(t *testing.T) {
	now := int64(1700000000000)
	s := statistic.NewStatisticTimer(statistic.TimerCountdown)
	s.SetNowFunc(func() int64 { return now })
	s.SetValue(now + 2000)
	finishes := 0
	s.SetOnFinish(func() { finishes++ })
	s.Tick(0.016)
	if finishes != 0 || s.Finished() {
		t.Fatal("must not finish early")
	}
	now += 5000
	// First finishing tick returns false (stop), second stays finished.
	if s.Tick(0.016) {
		t.Fatal("finishing tick should request stop")
	}
	s.Tick(0.016)
	if finishes != 1 || !s.Finished() {
		t.Fatalf("finishes=%d finished=%v", finishes, s.Finished())
	}
	if got := s.DisplayText(); got != "00:00:00" {
		t.Fatalf("stuck display=%q", got)
	}
}

// STA-06: loading keeps title, binds ticker, paints skeleton.
func TestStatistic_PRD_STA06(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	s.SetLoading(true)
	if !s.IsLoading() || !s.HasTitle() {
		t.Fatal("loading keeps title")
	}
	// Display still computable while skeleton shows.
	if s.DisplayText() != "112,893" {
		t.Fatalf("display while loading=%q", s.DisplayText())
	}
	reg := &scheduler.TickerRegistry{}
	s.Attach(reg)
	if !reg.HasActive() || !s.WantsFrame() {
		t.Fatal("loading ticker should want frames")
	}
	sz := layoutSize(t, s, rendering.Loose(800, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("loading size=%+v", sz)
	}
	s.Node().Paint(&rendering.PaintContext{})
	s.Detach()
	if reg.HasActive() {
		t.Fatal("detach should remove ticker")
	}
}

// STA-07: styles.content wins, valueStyle is the deprecated fallback.
func TestStatistic_PRD_STA07(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetValue(11.28)
	tok := theme.Default.Current()
	if s.ContentColor() != tok.ColorTextHeading {
		t.Fatal("default content walks heading token")
	}
	red := theme.RGBA(255, 0, 0, 1)
	s.SetValueStyle(statistic.ColorStyle(red))
	if s.ContentColor() != red {
		t.Fatal("valueStyle fallback")
	}
	blue := theme.RGBA(0, 0, 255, 1)
	s.SetContentStyle(statistic.ColorStyle(blue))
	if s.ContentColor() != blue {
		t.Fatal("styles.content wins over valueStyle")
	}
	s.SetContentStyle(statistic.FontSizeStyle(30))
	if s.ContentFontSize() != 30 {
		t.Fatalf("content size=%v", s.ContentFontSize())
	}
}

// STA-08: basic official example (title+value+precision+loading).
func TestStatistic_PRD_STA08(t *testing.T) {
	a := statistic.NewStatistic()
	a.SetTitle("Active Users")
	a.SetValue(112893)
	b := statistic.NewStatistic()
	b.SetTitle("Account Balance (CNY)")
	b.SetValue(112893)
	b.SetPrecision(2)
	c := statistic.NewStatistic()
	c.SetTitle("Active Users")
	c.SetValue(112893)
	c.SetLoading(true)
	if a.DisplayText() != "112,893" || b.DisplayText() != "112,893.00" {
		t.Fatalf("basic %q %q", a.DisplayText(), b.DisplayText())
	}
	if !c.IsLoading() || c.DisplayText() != "112,893" {
		t.Fatal("basic loading")
	}
	for _, s := range []*statistic.Statistic{a, b, c} {
		layoutSize(t, s, rendering.Loose(800, 200))
		s.Node().Paint(&rendering.PaintContext{})
	}
}

// STA-09: unit official example (prefix/suffix).
func TestStatistic_PRD_STA09(t *testing.T) {
	a := statistic.NewStatistic()
	a.SetTitle("Feedback")
	a.SetValue(1128)
	a.SetPrefix("like")
	if a.FullContentText() != "like1,128" {
		t.Fatalf("feedback=%q", a.FullContentText())
	}
	b := statistic.NewStatistic()
	b.SetTitle("Unmerged")
	b.SetValue(93)
	b.SetSuffix("/ 100")
	if b.FullContentText() != "93/ 100" {
		t.Fatalf("unmerged=%q", b.FullContentText())
	}
	layoutSize(t, a, rendering.Loose(800, 200))
	layoutSize(t, b, rendering.Loose(800, 200))
}

// STA-10: animated formatter path (P0 instant final, pixel motion P1).
func TestStatistic_PRD_STA10(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	s.SetFormatter(func(v any) string { return "112,893!" })
	if !s.HasFormatter() || s.DisplayText() != "112,893!" {
		t.Fatalf("formatter=%q", s.DisplayText())
	}
	// Timer also routes through the formatter hook.
	tm := statistic.NewStatisticTimer(statistic.TimerCountup)
	tm.SetNowFunc(func() int64 { return 2000 })
	tm.SetValue(1000)
	tm.SetFormatter(func(v any) string { return "final" })
	if tm.DisplayText() != "final" {
		t.Fatalf("timer formatter=%q", tm.DisplayText())
	}
}

// STA-11: card example (content color + affixes + precision).
func TestStatistic_PRD_STA11(t *testing.T) {
	active := statistic.NewStatistic()
	active.SetTitle("Active")
	active.SetValue(11.28)
	active.SetPrecision(2)
	active.SetPrefix("up")
	active.SetSuffix("%")
	active.SetContentStyle(statistic.ColorStyle(theme.RGBA(63, 134, 0, 1)))
	idle := statistic.NewStatistic()
	idle.SetTitle("Idle")
	idle.SetValue(9.3)
	idle.SetPrecision(2)
	idle.SetPrefix("down")
	idle.SetSuffix("%")
	idle.SetContentStyle(statistic.ColorStyle(theme.RGBA(207, 19, 34, 1)))
	if active.DisplayText() != "11.28" || idle.DisplayText() != "9.30" {
		t.Fatalf("card %q %q", active.DisplayText(), idle.DisplayText())
	}
	if active.ContentColor().R >= 0.5 || idle.ContentColor().R < 0.5 {
		t.Fatal("card content colors")
	}
	layoutSize(t, active, rendering.Loose(800, 200))
	layoutSize(t, idle, rendering.Loose(800, 200))
}

// STA-12: timer countdown/countup with format and callbacks.
func TestStatistic_PRD_STA12(t *testing.T) {
	f := loadStatisticFile(t)
	for _, tc := range f.Timer {
		if got := statistic.FormatTimeStr(tc.DiffMs, tc.Format); got != tc.Expected {
			t.Fatalf("diff=%d fmt=%q got %q want %q", tc.DiffMs, tc.Format, got, tc.Expected)
		}
	}
	now := int64(1700000000000)
	down := statistic.NewStatisticTimer(statistic.TimerCountdown)
	down.SetNowFunc(func() int64 { return now })
	down.SetValue(now + 61000)
	down.SetFormat("mm:ss")
	var changes []float64
	down.SetOnChange(func(d float64) { changes = append(changes, d) })
	down.Tick(0.016)
	if len(changes) != 1 || math.Abs(changes[0]-61000) > 1 {
		t.Fatalf("onChange=%v", changes)
	}
	if down.DisplayText() != "01:01" {
		t.Fatalf("countdown=%q", down.DisplayText())
	}
	up := statistic.NewStatisticTimer(statistic.TimerCountup)
	up.SetNowFunc(func() int64 { return now })
	up.SetValue(now - 61000)
	up.SetFormat("mm:ss")
	up.Tick(0.016)
	if up.DisplayText() != "01:01" || up.Finished() {
		t.Fatalf("countup=%q finished=%v", up.DisplayText(), up.Finished())
	}
	// Day-level literal template.
	down.SetValue(now + 172830000)
	down.SetFormat("D 天 H 时 m 分 s 秒")
	down.SetNowFunc(func() int64 { return now })
	if got := down.DisplayText(); got != "2 天 0 时 0 分 30 秒" {
		t.Fatalf("day=%q", got)
	}
}

// STA-13: shallow semantic styles/classNames.
func TestStatistic_PRD_STA13(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("Monthly Active Users")
	s.SetValue(93241)
	s.SetPrefix("up")
	s.SetSuffix("users")
	s.SetClassNames(statistic.ClassNames{Root: "demo-root", Content: "demo-content"})
	s.SetStyles(statistic.Styles{
		Title:   statistic.ColorStyle(theme.RGBA(24, 144, 255, 1)),
		Content: statistic.ColorStyle(theme.RGBA(9, 88, 217, 1)),
	})
	if s.ClassNames().Root != "demo-root" || s.ClassNames().Content != "demo-content" {
		t.Fatal("classNames hooks")
	}
	if s.TitleColor().B < 0.5 || s.ContentColor().B < 0.5 {
		t.Fatal("shallow style colors")
	}
	layoutSize(t, s, rendering.Loose(800, 200))
	s.Node().Paint(&rendering.PaintContext{})
	// Custom nodes stay hosted without breaking layout.
	host := rendering.NewRenderColorBox(10, 10, 0.1, 0.2, 0.3, 1)
	s.SetPrefixNode(host)
	if s.PrefixHost() != rendering.RenderObject(host) {
		t.Fatal("prefix host")
	}
	layoutSize(t, s, rendering.Loose(800, 200))
}

// STA-15: §6.2 metrics plus Exact/Min/Max layout matrix.
func TestStatistic_PRD_STA15(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("t")
	s.SetValue(112893)
	if v := s.TitleFontSize(); math.Abs(v-14) > 0.5 {
		t.Fatalf("title=%v want 14", v)
	}
	if v := s.ContentFontSize(); math.Abs(v-24) > 0.5 {
		t.Fatalf("content=%v want 24", v)
	}
	if v := s.Gap(); math.Abs(v-4) > 0.5 {
		t.Fatalf("gap=%v want 4", v)
	}
	pref := layoutSize(t, s, rendering.Loose(800, 200))
	exact := layoutSize(t, s, rendering.Tight(pref.Width, pref.Height))
	if math.Abs(exact.Width-pref.Width) > 0.5 || math.Abs(exact.Height-pref.Height) > 0.5 {
		t.Fatalf("exact=%+v pref=%+v", exact, pref)
	}
	mini := layoutSize(t, s, rendering.Constraints{MinWidth: 4, MaxWidth: 800, MinHeight: 4, MaxHeight: 200})
	if mini.Width < 4 || mini.Height < 4 {
		t.Fatalf("min=%+v", mini)
	}
	maxi := layoutSize(t, s, rendering.Constraints{MinWidth: 0, MaxWidth: 40, MinHeight: 0, MaxHeight: 20})
	if maxi.Width > 40.5 || maxi.Height > 20.5 {
		t.Fatalf("max=%+v capped 40x20", maxi)
	}
}

// STA-16: theme drives ink; no brand hardcode as the only default.
func TestStatistic_PRD_STA16(t *testing.T) {
	s := statistic.NewStatistic()
	tok := theme.Default.Current()
	if s.TitleColor() != tok.ColorTextSecondary {
		t.Fatalf("title=%+v want secondary %+v", s.TitleColor(), tok.ColorTextSecondary)
	}
	if s.ContentColor() != tok.ColorTextHeading {
		t.Fatalf("content=%+v want heading %+v", s.ContentColor(), tok.ColorTextHeading)
	}
	// Provider swap flows through without touching the widget.
	alt := theme.NewProvider(theme.DefaultTokens())
	mods := alt.Current()
	mods.ColorTextHeading = theme.RGBA(10, 20, 30, 1)
	alt.SetBase(mods)
	s.SetProvider(alt)
	if s.ContentColor() != mods.ColorTextHeading {
		t.Fatal("provider theme must flow to content")
	}
	// Pinned tokens win over the provider.
	pinned := theme.DefaultTokens()
	pinned.ColorTextSecondary = theme.RGBA(1, 2, 3, 1)
	s.SetTheme(&pinned)
	if s.TitleColor() != pinned.ColorTextSecondary {
		t.Fatal("pinned theme must flow to title")
	}
	// Separator helpers still honor explicit config over theme.
	if got := statistic.FormatNumber(112893.12, false, 0, "-", "_"); got != "112_893-12" {
		t.Fatalf("seps=%q", got)
	}
}

// A11y floor: group role, readable text, never focuses.
func TestStatistic_PRD_A11y(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	if s.Role() != "group" {
		t.Fatalf("role=%q want group", s.Role())
	}
	if s.Focusable() {
		t.Fatal("display control must not take focus")
	}
	s.SetAriaLabel("active users statistic")
	if s.AriaLabel() != "active users statistic" {
		t.Fatal("aria label")
	}
	if s.FullContentText() == "" {
		t.Fatal("value must stay readable")
	}
}
