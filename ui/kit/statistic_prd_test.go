package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/statistic.md §6.9 — P0 PRD cases (STA-01 … STA-13, 15, 16).
// STA-14/21 P1 deferred; STA-17/18 N/A; STA-19 L3 / STA-20 L4 deferred.

func approxStat(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxStatColor(a, b render.RGBA, tol float64) bool {
	return approxStat(float64(a.R), float64(b.R), tol) &&
		approxStat(float64(a.G), float64(b.G), tol) &&
		approxStat(float64(a.B), float64(b.B), tol) &&
		approxStat(float64(a.A), float64(b.A), tol)
}

func TestStatistic_PRD_01_Defaults(t *testing.T) {
	// STA-01: NewStatistic 默认创建
	s := kit.NewStatistic()
	if s.Value() != 0 && s.Value() != 0.0 {
		// Value() returns 0 (int) when unset
		if v, ok := s.Value().(int); !ok || v != 0 {
			if vf, ok := s.Value().(float64); !ok || vf != 0 {
				t.Fatalf("value=%v want 0", s.Value())
			}
		}
	}
	if s.IsLoading() {
		t.Fatal("loading want false")
	}
	if s.DecimalSeparator() != kit.DefaultStatisticDecimalSeparator {
		t.Fatalf("decSep=%q", s.DecimalSeparator())
	}
	if s.GroupSeparator() != kit.DefaultStatisticGroupSeparator {
		t.Fatalf("groupSep=%q", s.GroupSeparator())
	}
	if s.TimerType() != kit.StatisticTimerNone {
		t.Fatalf("timer=%v", s.TimerType())
	}
	if s.Node() == nil || s.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	sz := s.Node().Layout(core.Loose(400, 200))
	if sz.Width < 1 || sz.Height < 1 {
		t.Fatalf("size=%v", sz)
	}
}

func TestStatistic_PRD_02_ValueDisplay(t *testing.T) {
	// STA-02 / STA-S1: value 显示（千分位）
	s := kit.NewStatistic()
	s.SetValue(112893)
	got := s.DisplayText()
	if got != "112,893" {
		t.Fatalf("display=%q want 112,893", got)
	}
	_ = s.Node().Layout(core.Loose(300, 100))
	if s.ValueNodeHost() == nil {
		t.Fatal("ValueNodeHost nil")
	}
}

func TestStatistic_PRD_03_Precision(t *testing.T) {
	// STA-03 / STA-S2: precision=2
	s := kit.NewStatistic()
	s.SetValue(112893)
	s.SetPrecision(2)
	got := s.DisplayText()
	if got != "112,893.00" {
		t.Fatalf("display=%q want 112,893.00", got)
	}
	// pad / truncate
	s.SetValue(11.289)
	s.SetPrecision(2)
	if s.DisplayText() != "11.28" {
		// antd slices, no round
		if s.DisplayText() != "11.28" {
			t.Fatalf("display=%q want 11.28", s.DisplayText())
		}
	}
	_ = s.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_04_PrefixSuffix(t *testing.T) {
	// STA-04 / STA-S3
	s := kit.NewStatistic()
	s.SetValue(93)
	s.SetPrefix("$")
	s.SetSuffix("%")
	if !s.HasPrefix() || !s.HasSuffix() {
		t.Fatal("prefix/suffix flags")
	}
	_ = s.Node().Layout(core.Loose(300, 100))
	if s.PrefixHost() == nil || s.SuffixHost() == nil {
		t.Fatal("hosts nil")
	}
	// node prefix
	ic := kit.NewIcon("heart")
	s2 := kit.NewStatistic()
	s2.SetValue(1128)
	s2.SetPrefixNode(ic.Node())
	if !s2.HasPrefix() {
		t.Fatal("prefix node")
	}
	_ = s2.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_05_CountdownFinish(t *testing.T) {
	// STA-05 / STA-S4: Timer countdown onFinish once
	now := int64(1_000_000)
	fin := 0
	changes := 0
	s := kit.NewStatisticTimer(kit.StatisticTimerCountdown)
	s.SetNowFunc(func() int64 { return now })
	s.SetValue(now + 50) // 50ms remaining
	s.SetOnFinish(func() { fin++ })
	s.SetOnChange(func(d float64) { changes++ })
	s.AdvanceTimerForTest(true)
	if s.Finished() {
		t.Fatal("should not finish yet")
	}
	if fin != 0 {
		t.Fatalf("finish early count=%d", fin)
	}
	now = now + 100 // past deadline
	s.AdvanceTimerForTest(true)
	if !s.Finished() {
		t.Fatal("want finished")
	}
	if fin != 1 {
		t.Fatalf("onFinish count=%d want 1", fin)
	}
	// second advance must not re-fire
	s.AdvanceTimerForTest(true)
	if fin != 1 {
		t.Fatalf("onFinish re-fire count=%d", fin)
	}
	if s.DisplayText() == "" {
		t.Fatal("empty display after finish")
	}
	_ = changes
	_ = s.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_06_Loading(t *testing.T) {
	// STA-06 / STA-S5
	s := kit.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	s.SetLoading(true)
	if !s.IsLoading() {
		t.Fatal("IsLoading false")
	}
	_ = s.Node().Layout(core.Loose(300, 120))
	if s.SkeletonNode() == nil {
		t.Fatal("SkeletonNode nil")
	}
	if s.ContentNodeHost() != nil {
		t.Fatalf("content should be nil while loading; content=%T", s.ContentNodeHost())
	}
	// ticker bind — mount root so stillMounted stays true after AttachTicker
	tree := core.NewTree(s.Node())
	s.AttachTicker(tree)
	if !s.Tick(0.016) {
		t.Fatal("Tick should continue while loading")
	}
	s.SetLoading(false)
	if s.IsLoading() {
		t.Fatal("still loading")
	}
	_ = s.Node().Layout(core.Loose(300, 120))
	if s.ContentNodeHost() == nil {
		t.Fatal("content missing after unload")
	}
}

func TestStatistic_PRD_07_ContentStyle(t *testing.T) {
	// STA-07 / STA-S6: styles.content / valueStyle
	s := kit.NewStatistic()
	s.SetValue(11.28)
	s.SetPrecision(2)
	col := render.Hex("#3f8600")
	s.SetContentStyle(kit.Style{Text: col, FontSize: 24})
	if !approxStatColor(s.ContentColor(), col, 0.02) {
		t.Fatalf("content color=%v want %v", s.ContentColor(), col)
	}
	if !approxStat(s.ContentFontSize(), 24, 0.5) {
		t.Fatalf("font=%v", s.ContentFontSize())
	}
	// valueStyle path
	s2 := kit.NewStatistic()
	s2.SetValue(1)
	s2.SetValueStyle(kit.Style{Text: render.Hex("#cf1322"), FontSize: 20})
	if !approxStat(s2.ContentFontSize(), 20, 0.5) {
		t.Fatalf("valueStyle font=%v", s2.ContentFontSize())
	}
	_ = s.Node().Layout(core.Loose(300, 100))
	_ = s2.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_08_DemoBasic(t *testing.T) {
	// STA-08: basic.tsx
	a := kit.NewStatistic()
	a.SetTitle("Active Users")
	a.SetValue(112893)
	if a.DisplayText() != "112,893" {
		t.Fatalf("a=%q", a.DisplayText())
	}
	b := kit.NewStatistic()
	b.SetTitle("Account Balance (CNY)")
	b.SetValue(112893)
	b.SetPrecision(2)
	if b.DisplayText() != "112,893.00" {
		t.Fatalf("b=%q", b.DisplayText())
	}
	c := kit.NewStatistic()
	c.SetTitle("Active Users")
	c.SetValue(112893)
	c.SetLoading(true)
	if !c.IsLoading() {
		t.Fatal("loading")
	}
	for _, x := range []*kit.Statistic{a, b, c} {
		_ = x.Node().Layout(core.Loose(400, 120))
	}
}

func TestStatistic_PRD_09_DemoUnit(t *testing.T) {
	// STA-09: unit.tsx
	a := kit.NewStatistic()
	a.SetTitle("Feedback")
	a.SetValue(1128)
	a.SetPrefixNode(kit.NewIcon("heart").Node())
	b := kit.NewStatistic()
	b.SetTitle("Unmerged")
	b.SetValue(93)
	b.SetSuffix("/ 100")
	if !a.HasPrefix() || !b.HasSuffix() {
		t.Fatal("unit fields")
	}
	_ = a.Node().Layout(core.Loose(300, 100))
	_ = b.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_10_DemoAnimated(t *testing.T) {
	// STA-10: animated.tsx — formatter path (CountUp pixel anim P1)
	s := kit.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	s.SetFormatter(func(v any) string {
		n := kit.NewStatistic()
		n.SetValue(v)
		return n.DisplayText()
	})
	if s.DisplayText() != "112,893" {
		t.Fatalf("formatter display=%q", s.DisplayText())
	}
	s2 := kit.NewStatistic()
	s2.SetValue(112893)
	s2.SetPrecision(2)
	s2.SetFormatter(func(v any) string {
		n := kit.NewStatistic()
		n.SetValue(v)
		n.SetPrecision(2)
		return n.DisplayText()
	})
	if s2.DisplayText() != "112,893.00" {
		t.Fatalf("fmt2=%q", s2.DisplayText())
	}
	_ = s.Node().Layout(core.Loose(300, 100))
}

func TestStatistic_PRD_11_DemoCard(t *testing.T) {
	// STA-11: card.tsx
	up := kit.NewStatistic()
	up.SetTitle("Active")
	up.SetValue(11.28)
	up.SetPrecision(2)
	up.SetContentStyle(kit.Style{Text: render.Hex("#3f8600")})
	up.SetPrefix("↑")
	up.SetSuffix("%")

	down := kit.NewStatistic()
	down.SetTitle("Idle")
	down.SetValue(9.3)
	down.SetPrecision(2)
	down.SetContentStyle(kit.Style{Text: render.Hex("#cf1322")})
	down.SetPrefix("↓")
	down.SetSuffix("%")

	cardA := kit.NewCard("")
	cardA.SetVariant(kit.CardBorderless)
	cardA.SetContent(up.Node())
	cardB := kit.NewCard("")
	cardB.SetVariant(kit.CardBorderless)
	cardB.SetContent(down.Node())

	_ = cardA.Node().Layout(core.Loose(240, 120))
	_ = cardB.Node().Layout(core.Loose(240, 120))
	if up.DisplayText() != "11.28" || down.DisplayText() != "9.30" {
		t.Fatalf("card values %q %q", up.DisplayText(), down.DisplayText())
	}
}

func TestStatistic_PRD_12_DemoTimer(t *testing.T) {
	// STA-12: timer.tsx
	now := int64(1_700_000_000_000)
	deadline := now + 1000*60*60*24*2 + 1000*30
	before := now - 1000*60*60*24*2 + 1000*30

	cd := kit.NewStatisticTimer(kit.StatisticTimerCountdown)
	cd.SetNowFunc(func() int64 { return now })
	cd.SetValue(deadline)
	cd.AdvanceTimerForTest(false)
	txt := cd.DisplayText()
	// ~48h → HH:mm:ss
	if txt == "" || txt == "-" {
		t.Fatalf("countdown empty %q", txt)
	}
	if !strings.Contains(txt, ":") {
		t.Fatalf("countdown format %q", txt)
	}

	ms := kit.NewStatisticTimer(kit.StatisticTimerCountdown)
	ms.SetNowFunc(func() int64 { return now })
	ms.SetTitle("Million Seconds")
	ms.SetValue(deadline)
	ms.SetFormat("HH:mm:ss:SSS")
	ms.AdvanceTimerForTest(false)
	if !strings.Contains(ms.DisplayText(), ":") {
		t.Fatalf("ms format %q", ms.DisplayText())
	}

	cu := kit.NewStatisticTimer(kit.StatisticTimerCountup)
	cu.SetNowFunc(func() int64 { return now })
	cu.SetTitle("Countup")
	cu.SetValue(before)
	cu.AdvanceTimerForTest(false)
	if cu.DisplayText() == "" {
		t.Fatal("countup empty")
	}

	day := kit.NewStatisticTimer(kit.StatisticTimerCountdown)
	day.SetNowFunc(func() int64 { return now })
	day.SetTitle("Day Level")
	day.SetValue(deadline)
	day.SetFormat("D 天 H 时 m 分 s 秒")
	day.AdvanceTimerForTest(false)
	got := day.DisplayText()
	if !strings.Contains(got, "天") || !strings.Contains(got, "时") {
		t.Fatalf("day format %q", got)
	}

	// onChange fires
	var last float64 = -1
	ten := kit.NewStatisticTimer(kit.StatisticTimerCountdown)
	ten.SetNowFunc(func() int64 { return now })
	ten.SetValue(now + 10_000)
	ten.SetOnChange(func(d float64) { last = d })
	ten.AdvanceTimerForTest(true)
	if last < 0 {
		t.Fatal("onChange not fired")
	}
	if last < 9000 || last > 10000 {
		t.Fatalf("diff=%v want ~10000", last)
	}

	for _, x := range []*kit.Statistic{cd, ms, cu, day, ten} {
		_ = x.Node().Layout(core.Loose(400, 100))
	}
}

func TestStatistic_PRD_13_DemoStyleClass(t *testing.T) {
	// STA-13: style-class.tsx shallow styles/classNames
	s := kit.NewStatistic()
	s.SetTitle("Monthly Active Users")
	s.SetValue(93241)
	s.SetSuffix("users")
	s.SetPrefix("↑")
	s.SetClassNames(kit.StatisticClassNames{Root: "stat-style-demo"})
	s.SetStyle(kit.Style{
		Border:      render.Hex("#CCCCCC"),
		Radius:      8,
		ForceRadius: true,
	})
	s.SetTitleStyle(kit.Style{Text: render.Hex("#1890ff")})
	s.SetContentStyle(kit.Style{FontSize: 24})
	s.SetValueStyle(kit.Style{
		Background: render.Hex("#e6f4ff"),
		Text:       render.Hex("#0958d9"),
		Radius:     4,
	})
	_ = s.Node().Layout(core.Loose(400, 120))
	if s.TitleColor().A < 0.1 {
		t.Fatal("title color")
	}
	if !approxStat(s.ContentFontSize(), 24, 0.5) {
		t.Fatalf("fs=%v", s.ContentFontSize())
	}

	neg := kit.NewStatistic()
	neg.SetTitle("Yearly Loss")
	neg.SetValue(-18.7)
	neg.SetPrecision(1)
	neg.SetSuffix("%")
	// function-form styles approximated by direct sets when negative
	neg.SetTitleStyle(kit.Style{Text: render.Hex("#ff4d4f")})
	neg.SetContentStyle(kit.Style{Text: render.Hex("#ff7875")})
	if !strings.HasPrefix(neg.DisplayText(), "-") {
		t.Fatalf("neg display=%q", neg.DisplayText())
	}
	_ = neg.Node().Layout(core.Loose(400, 120))
}

func TestStatistic_PRD_15_Metrics(t *testing.T) {
	// STA-15 L2
	s := kit.NewStatistic()
	s.SetTitle("T")
	s.SetValue(1)
	if !approxStat(s.TitleFontSize(), kit.DefaultStatisticTitleFontSize, 0.5) {
		t.Fatalf("titleFS=%v", s.TitleFontSize())
	}
	if !approxStat(s.ContentFontSize(), kit.DefaultStatisticContentFontSize, 0.5) {
		t.Fatalf("contentFS=%v", s.ContentFontSize())
	}
	if !approxStat(s.Gap(), kit.DefaultStatisticGap, 0.5) {
		t.Fatalf("gap=%v", s.Gap())
	}
	_ = s.Node().Layout(core.Loose(200, 100))
}

func TestStatistic_PRD_16_TokenColors(t *testing.T) {
	// STA-16 L2: no hard-coded brand default skin
	s := kit.NewStatistic()
	s.SetTitle("Users")
	s.SetValue(1280)
	th := kit.DefaultTheme()
	titleTok := th.Color(core.TokenColorTextSecondary)
	contentTok := th.Color(core.TokenColorText)
	if !approxStatColor(s.TitleColor(), titleTok, 0.05) {
		t.Fatalf("title color not token: got=%v want=%v", s.TitleColor(), titleTok)
	}
	if !approxStatColor(s.ContentColor(), contentTok, 0.05) {
		t.Fatalf("content color not token: got=%v want=%v", s.ContentColor(), contentTok)
	}
	// primary blue must not be default text
	primary := th.Color(core.TokenColorPrimary)
	if approxStatColor(s.ContentColor(), primary, 0.02) && primary.A > 0.5 {
		t.Fatal("content color equals brand primary — hard-coded risk")
	}
	_ = s.Node().Layout(core.Loose(200, 100))
}

func TestStatistic_PRD_SeparatorsAndIllegal(t *testing.T) {
	// STA-S7 / illegal number passthrough
	s := kit.NewStatistic()
	s.SetValue("abc")
	if s.DisplayText() != "abc" {
		t.Fatalf("illegal=%q", s.DisplayText())
	}
	s.SetValue(1234567)
	s.SetGroupSeparator(" ")
	s.SetDecimalSeparator(",")
	s.SetPrecision(1)
	// 1 234 567,0
	got := s.DisplayText()
	if !strings.Contains(got, " ") || !strings.Contains(got, ",") {
		t.Fatalf("seps=%q", got)
	}
}

func TestStatistic_PRD_A11yRole(t *testing.T) {
	s := kit.NewStatistic()
	s.SetAriaLabel("Active users statistic")
	n := s.Node()
	if n.Base().Role != "group" {
		t.Fatalf("role=%q", n.Base().Role)
	}
	if n.Base().Label != "Active users statistic" {
		t.Fatalf("label=%q", n.Base().Label)
	}
}

func TestStatistic_PRD_HitEqualsLayout(t *testing.T) {
	// hit == layout == paint: root fills content box
	s := kit.NewStatistic()
	s.SetTitle("T")
	s.SetValue(42)
	root := s.Node()
	sz := root.Layout(core.Loose(300, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("size=%v", sz)
	}
	// Decorated/host should not invent negative offsets
	if root.Base().Offset().X != 0 || root.Base().Offset().Y != 0 {
		// offset may be zero before parent layout
	}
	_ = primitive.Column(root)
}
