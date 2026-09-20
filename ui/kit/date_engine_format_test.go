package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestDateEngine_FormatParseMatchesCases(t *testing.T) {
	cases := loadDateCases(t)
	en := kit.ResolveDateEngineLocale("en-US")
	d := kit.DateEngineDateOf(2024, 2, 29)
	fm := cases["format"].(map[string]any)
	if got := kit.FormatDateEngineDate(d, "YYYY-MM-DD", false, en, 0); got != fm["february_29_2024"].(string) {
		t.Fatalf("format = %q", got)
	}
	if got := kit.FormatDateEngineDate(kit.DateEngineDateOf(2024, 5, 1), "Q", false, en, 0); got != fm["quarter_q2"].(string) {
		t.Fatalf("quarter = %q want 2", got)
	}
	bd := cases["buddhist"].(map[string]any)
	if got := kit.FormatDateEngineDate(d, "BBBB-MM-DD", true, en, 0); got[:4] != bd["bbbb_2024"].(string) {
		t.Fatalf("buddhist = %q want 2567 prefix", got)
	}
	ps := cases["parse"].(map[string]any)
	got, ok := kit.ParseDateEngineDate("2024-02-29", []string{"YYYY-MM-DD"}, false, en)
	if !ok || !got.SameDay(d) {
		t.Fatal("iso parse must round-trip")
	}
	if ps["iso"].(string) != kit.FormatDateEngineDate(got, "YYYY-MM-DD", false, en, 0) {
		t.Fatal("parsed date must reformat to the case value")
	}
	got, ok = kit.ParseDateEngineDate("29/02/2024", []string{"YYYY-MM-DD", "DD/MM/YYYY"}, false, en)
	if !ok || !got.SameDay(d) {
		t.Fatal("multi-format must fall through to the second pattern")
	}
	if got, ok := kit.ParseDateEngineDate("2024-13-01", []string{"YYYY-MM-DD"}, false, en); ok || got.IsValid() {
		t.Fatal("month 13 must not parse")
	}
	if ps["month13_invalid"].(bool) {
		t.Fatal("case file must mark month13 invalid=false")
	}
	if _, ok := kit.ParseDateEngineDate("2023-02-29", []string{"YYYY-MM-DD"}, false, en); ok {
		t.Fatal("non-leap feb29 must not parse")
	}
	if ps["nonleap_feb29_invalid"].(bool) {
		t.Fatal("case file must mark non-leap feb29 invalid=false")
	}
	// Buddhist parse subtracts 543 back to civil.
	got, ok = kit.ParseDateEngineDate("2567-02-29", []string{"BBBB-MM-DD"}, true, en)
	if !ok || got.Year != 2024 {
		t.Fatalf("buddhist parse = %+v,%v want year 2024", got, ok)
	}
	// Quarter token pins the quarter's first month.
	got, ok = kit.ParseDateEngineDate("2024-Q2", []string{"YYYY-[Q]Q"}, false, en)
	if !ok || got.Month != 4 || got.Year != 2024 {
		t.Fatalf("quarter parse = %+v,%v", got, ok)
	}
}
