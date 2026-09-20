package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

// stubDateEngineGenerate proves components run on any implementation:
// it delegates every call while recording them.
type stubDateEngineGenerate struct {
	inner kit.DateEngineGenerateConfig
	calls *int
}

func (s stubDateEngineGenerate) Name() string { return "stub" }

func (s stubDateEngineGenerate) DaysInMonth(y, m int) int {
	*s.calls++
	return s.inner.DaysInMonth(y, m)
}

func (s stubDateEngineGenerate) IsLeapYear(y int) bool {
	*s.calls++
	return s.inner.IsLeapYear(y)
}

func (s stubDateEngineGenerate) DayOfWeek(d kit.DateEngineDate) int {
	*s.calls++
	return s.inner.DayOfWeek(d)
}

func (s stubDateEngineGenerate) Compare(a, b kit.DateEngineDate) int {
	*s.calls++
	return s.inner.Compare(a, b)
}

func (s stubDateEngineGenerate) AddDays(d kit.DateEngineDate, n int) kit.DateEngineDate {
	return s.inner.AddDays(d, n)
}

func (s stubDateEngineGenerate) AddMonths(d kit.DateEngineDate, n int) kit.DateEngineDate {
	return s.inner.AddMonths(d, n)
}

func (s stubDateEngineGenerate) AddYears(d kit.DateEngineDate, n int) kit.DateEngineDate {
	return s.inner.AddYears(d, n)
}

func (s stubDateEngineGenerate) AddQuarters(d kit.DateEngineDate, n int) kit.DateEngineDate {
	return s.inner.AddQuarters(d, n)
}

func (s stubDateEngineGenerate) StartOfDay(d kit.DateEngineDate) kit.DateEngineDate {
	*s.calls++
	return s.inner.StartOfDay(d)
}

func (s stubDateEngineGenerate) StartOfMonth(d kit.DateEngineDate) kit.DateEngineDate {
	return s.inner.StartOfMonth(d)
}

func (s stubDateEngineGenerate) StartOfYear(d kit.DateEngineDate) kit.DateEngineDate {
	return s.inner.StartOfYear(d)
}

func (s stubDateEngineGenerate) QuarterOf(d kit.DateEngineDate) int {
	return s.inner.QuarterOf(d)
}

func (s stubDateEngineGenerate) WeekNumber(d kit.DateEngineDate, ws int) int {
	return s.inner.WeekNumber(d, ws)
}

func TestDateEngine_StdMathMatchesCases(t *testing.T) {
	cases := loadDateCases(t)
	gen := kit.DefaultDateEngineGenerateConfig()
	if gen.Name() != "std" {
		t.Fatalf("name = %q want std", gen.Name())
	}
	leap := cases["leap"].(map[string]any)
	for _, tc := range []struct {
		year int
		key  string
	}{
		{2000, "2000"}, {1900, "1900"}, {2024, "2024"}, {2023, "2023"},
	} {
		if gen.IsLeapYear(tc.year) != leap[tc.key].(bool) {
			t.Fatalf("leap %d mismatch", tc.year)
		}
	}
	md := cases["month_days"].(map[string]any)
	if gen.DaysInMonth(2024, 2) != int(md["feb2024"].(float64)) {
		t.Fatal("feb 2024 must have 29 days")
	}
	if gen.DaysInMonth(2023, 2) != int(md["feb2023"].(float64)) {
		t.Fatal("feb 2023 must have 28 days")
	}
	wd := cases["weekday"].(map[string]any)
	if gen.DayOfWeek(kit.DateEngineDateOf(2024, 2, 29)) != int(wd["2024_02_29"].(float64)) {
		t.Fatal("2024-02-29 must be Thursday (4)")
	}
	if gen.DayOfWeek(kit.DateEngineDateOf(2026, 9, 21)) != int(wd["2026_09_21"].(float64)) {
		t.Fatal("2026-09-21 must be Monday (1)")
	}
	if gen.DayOfWeek(kit.DateEngineDateOf(1970, 1, 1)) != int(wd["1970_01_01"].(float64)) {
		t.Fatal("1970-01-01 must be Thursday (4)")
	}
	add := cases["add"].(map[string]any)
	got := gen.AddMonths(kit.DateEngineDateOf(2024, 1, 31), 1)
	if kit.FormatDateEngineDate(got, "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0) != add["jan31_2024_plus1m"].(string) {
		t.Fatal("jan31+1m must clamp to feb29 in leap year")
	}
	got = gen.AddMonths(kit.DateEngineDateOf(2023, 1, 31), 1)
	if kit.FormatDateEngineDate(got, "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0) != add["jan31_2023_plus1m"].(string) {
		t.Fatal("jan31+1m must clamp to feb28 in common year")
	}
	got = gen.AddYears(kit.DateEngineDateOf(2024, 2, 29), 1)
	if kit.FormatDateEngineDate(got, "YYYY-MM-DD", false, kit.ResolveDateEngineLocale("en-US"), 0) != add["feb29_2024_plus1y"].(string) {
		t.Fatal("feb29+1y must clamp to feb28")
	}
	wn := cases["weeknum"].(map[string]any)
	if gen.WeekNumber(kit.DateEngineDateOf(2024, 1, 1), 0) != int(wn["jan1_2024_ws0"].(float64)) {
		t.Fatal("jan1 2024 must open week 1")
	}

	// Swappability: a stub implementation drives the same instance.
	calls := 0
	stub := stubDateEngineGenerate{inner: gen, calls: &calls}
	host := kit.BuildDateEngineWithGenerate(kit.DefaultScopeCtx(), kit.DefaultDateEngineProps(), stub)
	if host.Generate().Name() != "stub" {
		t.Fatal("instance must keep the injected implementation")
	}
	host.Select(kit.DateEngineDateOf(2024, 2, 29))
	if host.Value() == nil || !host.Value().SameDay(kit.DateEngineDateOf(2024, 2, 29)) {
		t.Fatal("select must work through the stub")
	}
	if calls == 0 {
		t.Fatal("stub must observe generate calls")
	}
}
