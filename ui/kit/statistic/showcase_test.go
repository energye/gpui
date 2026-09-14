package statistic_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/statistic"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type showcaseSpec struct {
	CanvasW   int     `json:"canvasW"`
	Margin    float64 `json:"margin"`
	Gap       float64 `json:"gap"`
	Tolerance struct {
		MaxDiff float64 `json:"maxDiff"`
		BadFrac float64 `json:"badFrac"`
		HardCap float64 `json:"hardCap"`
	} `json:"tolerance"`
}

func loadShowcaseSpec(t *testing.T) showcaseSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "showcase_spec.json"))
	if err != nil {
		t.Fatalf("read showcase_spec.json: %v", err)
	}
	var s showcaseSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse showcase spec: %v", err)
	}
	if s.CanvasW <= 0 || s.Gap < 0 {
		t.Fatalf("bad showcase spec %+v", s)
	}
	return s
}

func loadShowcaseFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, desc, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		face, desc, err = rendering.TryLoadDefaultFace(14)
		if err != nil || face == nil {
			t.Skipf("showcase needs a system face for real glyphs: %v", err)
		}
	}
	t.Logf("showcase face: %s", desc)
	return face
}

// TestStatistic_Showcase_MainPaths lays the §6.8 P0 main paths on one canvas:
// basic (value+precision+loading), unit (prefix/suffix), animated formatter,
// card pair (content colors), timer countdown/countup/day, style-class shallow
// hooks, separators. Three evidences: logic probe (display/layout), pixel
// assertions (value/title ink, no bars), golden compare (tolerance from
// testdata/showcase_spec.json). Regenerate with UPDATE_GOLDEN=1.
func TestStatistic_Showcase_MainPaths(t *testing.T) {
	spec := loadShowcaseSpec(t)
	face := loadShowcaseFace(t)
	now := int64(1700000000000)

	mk := func(s *statistic.Statistic) *statistic.Statistic {
		s.SetTextFace(face)
		return s
	}

	// R1 basic (basic.tsx): plain + precision.
	basicActive := mk(statistic.NewStatistic())
	basicActive.SetTitle("Active Users")
	basicActive.SetValue(112893)
	basicBalance := mk(statistic.NewStatistic())
	basicBalance.SetTitle("Account Balance (CNY)")
	basicBalance.SetValue(112893)
	basicBalance.SetPrecision(2)
	// R1 loading variant keeps the title, content shows skeleton.
	basicLoading := mk(statistic.NewStatistic())
	basicLoading.SetTitle("Active Users")
	basicLoading.SetValue(112893)
	basicLoading.SetLoading(true)

	// R2 unit (unit.tsx): prefix + suffix.
	unitLike := mk(statistic.NewStatistic())
	unitLike.SetTitle("Feedback")
	unitLike.SetValue(1128)
	unitLike.SetPrefix("like")
	unitUnmerged := mk(statistic.NewStatistic())
	unitUnmerged.SetTitle("Unmerged")
	unitUnmerged.SetValue(93)
	unitUnmerged.SetSuffix("/ 100")

	// R3 animated (animated.tsx): P0 formatter instant final (pixel motion P1).
	animated := mk(statistic.NewStatistic())
	animated.SetTitle("Active Users")
	animated.SetValue(112893)
	animated.SetFormatter(func(v any) string { return "112,893!" })

	// R4 card (card.tsx): content colors + affixes + precision.
	cardActive := mk(statistic.NewStatistic())
	cardActive.SetTitle("Active")
	cardActive.SetValue(11.28)
	cardActive.SetPrecision(2)
	cardActive.SetPrefix("up")
	cardActive.SetSuffix("%")
	cardActive.SetContentStyle(statistic.ColorStyle(theme.RGBA(63, 134, 0, 1)))
	cardIdle := mk(statistic.NewStatistic())
	cardIdle.SetTitle("Idle")
	cardIdle.SetValue(9.3)
	cardIdle.SetPrecision(2)
	cardIdle.SetPrefix("down")
	cardIdle.SetSuffix("%")
	cardIdle.SetContentStyle(statistic.ColorStyle(theme.RGBA(207, 19, 34, 1)))

	// R5 timer (timer.tsx): countdown + countup + day literal (CJK).
	timerDown := mk(statistic.NewStatisticTimer(statistic.TimerCountdown))
	timerDown.SetTitle("Countdown")
	timerDown.SetNowFunc(func() int64 { return now })
	timerDown.SetValue(now + 3661000)
	timerDown.SetFormat("HH:mm:ss")
	timerDown.Tick(0.016)
	timerUp := mk(statistic.NewStatisticTimer(statistic.TimerCountup))
	timerUp.SetTitle("Countup")
	timerUp.SetNowFunc(func() int64 { return now })
	timerUp.SetValue(now - 61000)
	timerUp.SetFormat("mm:ss")
	timerUp.Tick(0.016)
	timerDay := mk(statistic.NewStatisticTimer(statistic.TimerCountdown))
	timerDay.SetTitle("Day Timer")
	timerDay.SetNowFunc(func() int64 { return now })
	timerDay.SetValue(now + 172830000)
	timerDay.SetFormat("D 天 H 时 m 分 s 秒")
	timerDay.Tick(0.016)

	// R6 style-class (style-class.tsx): shallow styles + classNames.
	styled := mk(statistic.NewStatistic())
	styled.SetTitle("Monthly Active Users")
	styled.SetValue(93241)
	styled.SetPrefix("up")
	styled.SetSuffix("users")
	styled.SetClassNames(statistic.ClassNames{Root: "demo-root", Content: "demo-content"})
	styled.SetStyles(statistic.Styles{
		Title:   statistic.ColorStyle(theme.RGBA(24, 144, 255, 1)),
		Content: statistic.ColorStyle(theme.RGBA(9, 88, 217, 1)),
	})

	// Separators row: custom decimal/group separators (STA-07/STA-16 path).
	seps := mk(statistic.NewStatistic())
	seps.SetTitle("Separators")
	seps.SetValue(112893.12)
	seps.SetDecimalSeparator("-")
	seps.SetGroupSeparator("_")

	rows := []*statistic.Statistic{
		basicActive, basicBalance, basicLoading,
		unitLike, unitUnmerged, animated,
		cardActive, cardIdle,
		timerDown, timerUp, timerDay,
		styled, seps,
	}

	// Logic probe before paint: display text, affixes, timer, loading, styles.
	if basicActive.DisplayText() != "112,893" || basicBalance.DisplayText() != "112,893.00" {
		t.Fatalf("basic %q %q", basicActive.DisplayText(), basicBalance.DisplayText())
	}
	if !basicLoading.IsLoading() || !basicLoading.HasTitle() {
		t.Fatal("loading must keep title")
	}
	if unitLike.FullContentText() != "like1,128" || unitUnmerged.FullContentText() != "93/ 100" {
		t.Fatalf("unit %q %q", unitLike.FullContentText(), unitUnmerged.FullContentText())
	}
	if animated.DisplayText() != "112,893!" || !animated.HasFormatter() {
		t.Fatalf("animated formatter=%q", animated.DisplayText())
	}
	if cardActive.DisplayText() != "11.28" || cardIdle.DisplayText() != "9.30" {
		t.Fatalf("card %q %q", cardActive.DisplayText(), cardIdle.DisplayText())
	}
	if timerDown.DisplayText() != "01:01:01" || timerUp.DisplayText() != "01:01" {
		t.Fatalf("timer %q %q", timerDown.DisplayText(), timerUp.DisplayText())
	}
	if timerDay.DisplayText() != "2 天 0 时 0 分 30 秒" {
		t.Fatalf("day timer=%q", timerDay.DisplayText())
	}
	if styled.ClassNames().Root != "demo-root" || seps.DisplayText() != "112_893-12" {
		t.Fatal("style-class/separators probe")
	}
	if timerDown.Finished() || timerUp.Finished() {
		t.Fatal("timers must not be finished at showcase offsets")
	}

	// Layout rows stacked vertically; every Layout/Node must stay non-zero.
	W := float64(spec.CanvasW)
	margin, gap := spec.Margin, spec.Gap
	rowW := W - 2*margin
	type placed struct {
		s *statistic.Statistic
		x float64
		y float64
		w float64
		h float64
	}
	var items []placed
	y := margin
	for i, s := range rows {
		if s.Node() == nil {
			t.Fatalf("row %d Node nil", i)
		}
		sz := s.Layout(rendering.Loose(rowW, 800))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("row %d layout=%v want non-zero", i, sz)
		}
		ns := s.Node().Size()
		if ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("row %d node size=%v want non-zero", i, ns)
		}
		if sz.Width > rowW+0.5 {
			t.Fatalf("row %d width=%v exceeds row %v", i, sz.Width, rowW)
		}
		items = append(items, placed{s: s, x: margin, y: y, w: sz.Width, h: sz.Height})
		y += sz.Height + gap
	}
	H := y - gap + margin

	dc := render.NewContext(int(W), int(H))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	for _, it := range items {
		it.s.Node().Paint(rendering.NewPaintContext(dc, 1).WithOrigin(it.x, it.y))
	}
	got := dc.Image()

	// Pixel assertion 1: first value zone carries dark glyph ink (real text).
	b0 := items[0]
	dark := 0
	x0, x1 := int(b0.x), int(b0.x+b0.w)
	if x1 > int(W) {
		x1 = int(W)
	}
	for yy := int(b0.y); yy < int(b0.y+b0.h); yy++ {
		for xx := x0; xx < x1; xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 110 && g/257 < 110 && b/257 < 110 {
				dark++
			}
		}
	}
	if dark < 30 {
		t.Fatalf("basic value zone dark=%d want >=30 (real glyphs missing?)", dark)
	}

	// Pixel assertion 2: styled title+value row also carries ink (both chains).
	// Styled ink is blue, so count non-white pixels instead of dark ones.
	st := items[len(items)-2]
	darkSt := 0
	for yy := int(st.y); yy < int(st.y+st.h); yy++ {
		for xx := int(st.x); xx < int(st.x+st.w) && xx < int(W); xx++ {
			r, g, b, _ := got.At(xx, yy).RGBA()
			if r/257 < 200 || g/257 < 200 || b/257 < 200 {
				darkSt++
			}
		}
	}
	if darkSt < 30 {
		t.Fatalf("styled row ink=%d want >=30", darkSt)
	}

	path := filepath.Join("testdata", "showcase_statistic.png")
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
			m := max4u(diffu(r1, r2), diffu(g1, g2), diffu(b1, b2), diffu(a1, a2))
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

func diffu(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func max4u(vs ...uint32) uint32 {
	m := vs[0]
	for _, v := range vs[1:] {
		if v > m {
			m = v
		}
	}
	return m
}
