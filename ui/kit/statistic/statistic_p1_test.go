package statistic_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/statistic"
	"github.com/energye/gpui/ui/rendering"
)

func loadP1Face(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		face, _, err = rendering.TryLoadDefaultFace(14)
		if err != nil || face == nil {
			t.Skipf("true-text needs a system face: %v", err)
		}
	}
	return face
}

// True-text chain: title+value paint real glyphs with a face (SetFont +
// DrawString via Abs), and paint no black bar without one (headless estimates
// width, draws nothing). Layout/Node stay non-zero in both modes.
func TestStatistic_TrueText_TitleValueNoBar(t *testing.T) {
	countDark := func(s *statistic.Statistic) (int, rendering.Size) {
		sz := s.Layout(rendering.Loose(800, 200))
		if sz.Width <= 0 || sz.Height <= 0 {
			t.Fatalf("layout=%v want non-zero", sz)
		}
		if s.Node() == nil {
			t.Fatal("Node nil")
		}
		if ns := s.Node().Size(); ns.Width <= 0 || ns.Height <= 0 {
			t.Fatalf("node size=%v want non-zero", ns)
		}
		dc := render.NewContext(int(sz.Width), int(sz.Height))
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		s.Node().Paint(rendering.NewPaintContext(dc, 1))
		img := dc.Image()
		dark := 0
		for y := 0; y < int(sz.Height); y++ {
			for x := 0; x < int(sz.Width); x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r/257 < 110 && g/257 < 110 && b/257 < 110 {
					dark++
				}
			}
		}
		return dark, sz
	}

	plain := statistic.NewStatistic()
	plain.SetTitle("Active Users")
	plain.SetValue(112893)
	if n, _ := countDark(plain); n != 0 {
		t.Fatalf("no-face dark=%d want 0 (black bar banned)", n)
	}

	face := loadP1Face(t)
	with := statistic.NewStatistic()
	with.SetTitle("Active Users")
	with.SetValue(112893)
	with.SetTextFace(face)
	if n, _ := countDark(with); n < 30 {
		t.Fatalf("with-face dark=%d want >=30 (real glyphs missing)", n)
	}
	// Prefix/suffix follow the same chain.
	affix := statistic.NewStatistic()
	affix.SetTitle("Unmerged")
	affix.SetValue(93)
	affix.SetPrefix("up")
	affix.SetSuffix("%")
	affix.SetTextFace(face)
	if n, _ := countDark(affix); n < 30 {
		t.Fatalf("affix with-face dark=%d want >=30", n)
	}
	// Clearing the face restores the empty (no-bar) path.
	with.SetTextFace(nil)
	if n, _ := countDark(with); n != 0 {
		t.Fatalf("cleared-face dark=%d want 0", n)
	}
}

// STA-14 is P1 by spec: _semantic.tsx doc preview is staged, not built.
func TestStatistic_PRD_STA14_SemanticPreviewNA(t *testing.T) {
	t.Skip("P1 STA-14 _semantic.tsx doc preview is staged per §6.8 P1; shallow P0 hooks are covered by STA-13")
}

// STA-17 is N/A by spec: Statistic has no disabled look.
func TestStatistic_PRD_STA17_DisabledNA(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("no disabled in spec")
	s.SetValue(1)
	// No SetDisabled API on purpose; display never takes focus.
	if s.Focusable() {
		t.Fatal("statistic must not steal focus")
	}
	if s.Role() != "group" {
		t.Fatalf("role=%q want group", s.Role())
	}
}

// STA-18 is N/A by spec: display control has no keyboard path; children own it.
func TestStatistic_PRD_STA18_KeyboardNA(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetTitle("Active Users")
	s.SetValue(112893)
	if s.Focusable() {
		t.Fatal("display control must not take Tab")
	}
	if s.FullContentText() == "" {
		t.Fatal("value must stay readable without keyboard")
	}
}

// STA-20 is L4: side-by-side sign-off against ant.design needs a human.
func TestStatistic_PRD_STA20_HumanEyeNA(t *testing.T) {
	t.Skip("L4 STA-20 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}

// STA-21 P1 CountUp pixel motion stays staged; P0 formatter instant final is
// covered by STA-10 and asserted here before the Skip.
func TestStatistic_PRD_STA21_CountupPixelNA(t *testing.T) {
	s := statistic.NewStatistic()
	s.SetValue(112893)
	s.SetFormatter(func(v any) string { return "112,893!" })
	if s.DisplayText() != "112,893!" {
		t.Fatalf("P0 formatter final=%q", s.DisplayText())
	}
	t.Skip("P1 CountUp pixel rolling numbers are staged per §6.8 P1; P0 uses formatter instant final")
}

// STA-21 P1 function-form semantic styles stay staged; shallow P0 in STA-13.
func TestStatistic_PRD_STA21_FunctionStylesNA(t *testing.T) {
	t.Skip("P1 semantic classNames/styles function forms are staged per §6.8 P1; shallow P0 hooks covered by STA-13")
}

// STA-21 P1 ConfigProvider global statistic defaults stay staged.
func TestStatistic_PRD_STA21_ConfigProviderNA(t *testing.T) {
	t.Skip("P1 ConfigProvider global statistic defaults are staged per §6.8 P1; per-widget SetProvider/SetTheme is the P0 path (STA-16)")
}

// STA-21 deprecated Statistic.Countdown compat: merged into Timer countdown.
func TestStatistic_PRD_STA21_DeprecatedCountdownCompat(t *testing.T) {
	now := int64(1700000000000)
	s := statistic.NewStatisticTimer(statistic.TimerCountdown)
	s.SetNowFunc(func() int64 { return now })
	s.SetValue(now + 1000)
	if s.TimerType() != statistic.TimerCountdown {
		t.Fatalf("type=%q want countdown", s.TimerType())
	}
	if s.Format() != statistic.DefaultFormat {
		t.Fatalf("format=%q want %q", s.Format(), statistic.DefaultFormat)
	}
	finishes := 0
	s.SetOnFinish(func() { finishes++ })
	s.Tick(0.016)
	now += 5000
	s.Tick(0.016)
	if finishes != 1 || !s.Finished() {
		t.Fatalf("deprecated countdown compat finishes=%d finished=%v", finishes, s.Finished())
	}
}

// STA-21 P1 debug demos and ant.design pixel-hash parity are out of scope.
func TestStatistic_PRD_STA21_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug demos and ant.design pixel-hash parity are not built per §6.1 L4/§6.8 P1;本库 golden is the L3 source")
}
