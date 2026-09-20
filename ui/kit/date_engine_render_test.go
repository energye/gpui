package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestDateEngine_PanelMathMatchesCases(t *testing.T) {
	cases := loadDateCases(t)
	gen := kit.DefaultDateEngineGenerateConfig()
	mx := cases["matrix_feb2024"].(map[string]any)

	grid0 := kit.MonthDateEngineMatrix(gen, 2024, 2, 0)
	first0 := kit.FormatDateEngineDate(grid0[0][0], "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0)
	if first0 != mx["first_ws0"].(string) {
		t.Fatalf("ws0 first = %s want 2024-01-28", first0)
	}
	last0 := kit.FormatDateEngineDate(grid0[5][6], "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0)
	if last0 != mx["last_ws0"].(string) {
		t.Fatalf("ws0 last = %s want 2024-03-09", last0)
	}
	grid1 := kit.MonthDateEngineMatrix(gen, 2024, 2, 1)
	first1 := kit.FormatDateEngineDate(grid1[0][0], "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 1)
	if first1 != mx["first_ws1"].(string) {
		t.Fatalf("ws1 first = %s want 2024-01-29", first1)
	}
	last1 := kit.FormatDateEngineDate(grid1[5][6], "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 1)
	if last1 != mx["last_ws1"].(string) {
		t.Fatalf("ws1 last = %s want 2024-03-10", last1)
	}
	// Leap Feb 29 must sit inside the grid exactly once.
	found := 0
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			if grid0[r][c].SameDay(kit.DateEngineDateOf(2024, 2, 29)) {
				found++
			}
		}
	}
	if found != 1 {
		t.Fatalf("feb29 appears %d times want 1", found)
	}

	// Quarter/month/year/decade panels.
	qs := kit.QuarterDateEngineCells(2024)
	if qs[1].Month != 4 || qs[3].Month != 10 {
		t.Fatalf("quarters = %+v", qs)
	}
	ms := kit.YearMonthDateEngineCells(2024)
	if ms[0].Month != 1 || ms[11].Month != 12 {
		t.Fatal("year must hold 12 months")
	}
	ys := kit.YearDateEngineCells(2020)
	if ys[0].Year != 2020 || ys[9].Year != 2029 {
		t.Fatalf("decade = %+v", ys)
	}
	ds := kit.DecadeDateEngineCells(2000)
	if ds[0] != [2]int{2000, 2009} || ds[9] != [2]int{2090, 2099} {
		t.Fatalf("century = %+v", ds)
	}
	dec := cases["decade"].(map[string]any)
	if kit.DecadeStartOf(2024) != int(dec["decade_2024"].(float64)) {
		t.Fatal("decade floor of 2024 must be 2020")
	}
	if kit.CenturyStartOf(2024) != int(dec["century_2024"].(float64)) {
		t.Fatal("century floor of 2024 must be 2000")
	}

	// Clamp keeps panel navigation inside min/max.
	minD := kit.DateEngineDateOf(2024, 3, 1)
	maxD := kit.DateEngineDateOf(2024, 5, 31)
	cl := kit.ClampDateEnginePanel(gen, kit.DateEngineDateOf(2024, 1, 10), &minD, &maxD)
	if cl.Month != 3 {
		t.Fatalf("clamp low = %+v want March", cl)
	}
	cl = kit.ClampDateEnginePanel(gen, kit.DateEngineDateOf(2024, 9, 10), &minD, &maxD)
	if cl.Month != 5 {
		t.Fatalf("clamp high = %+v want May", cl)
	}
}
