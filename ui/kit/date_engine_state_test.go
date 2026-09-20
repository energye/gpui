package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestDateEngine_SingleRangeMultiplePresets(t *testing.T) {
	// Single uncontrolled select fires onChange with the display string.
	host := kit.BuildDateEngine(kit.DefaultScopeCtx(), kit.DefaultDateEngineProps())
	var gotStr string
	var gotDate *kit.DateEngineDate
	host.SetOnChange(func(d *kit.DateEngineDate, s string) {
		gotDate, gotStr = d, s
	})
	if !host.Select(kit.DateEngineDateOf(2024, 2, 29)) {
		t.Fatal("valid select must succeed")
	}
	if gotStr != "2024-02-29" || gotDate == nil || !gotDate.SameDay(kit.DateEngineDateOf(2024, 2, 29)) {
		t.Fatalf("onChange = %v %q", gotDate, gotStr)
	}
	if host.Value() == nil {
		t.Fatal("uncontrolled must keep its own value")
	}

	// Controlled value never self-moves; outside drives it.
	cprops := kit.DefaultDateEngineProps()
	chost := kit.BuildDateEngine(kit.DefaultScopeCtx(), cprops)
	keep := kit.DateEngineDateOf(2024, 1, 1)
	chost.SetValue(&keep)
	fired := false
	chost.SetOnChange(func(d *kit.DateEngineDate, s string) { fired = true })
	chost.Select(kit.DateEngineDateOf(2024, 5, 5))
	if !chost.Value().SameDay(keep) {
		t.Fatal("controlled value must not self-move")
	}
	if !fired {
		t.Fatal("controlled select must still notify")
	}

	// Disabled matrix: min/max plus custom func.
	minD := kit.DateEngineDateOf(2024, 3, 1)
	maxD := kit.DateEngineDateOf(2024, 3, 31)
	dprops := kit.DefaultDateEngineProps()
	dprops.MinDate, dprops.MinDateSet = &minD, true
	dprops.MaxDate, dprops.MaxDateSet = &maxD, true
	dprops.DisabledDate = func(c kit.DateEngineDate, from *kit.DateEngineDate, p kit.DateEnginePicker) bool {
		return c.Day == 15
	}
	dhost := kit.BuildDateEngine(kit.DefaultScopeCtx(), dprops)
	if !dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 2, 29)) {
		t.Fatal("before min must disable")
	}
	if !dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 4, 1)) {
		t.Fatal("after max must disable")
	}
	if !dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 3, 15)) {
		t.Fatal("custom func must disable the 15th")
	}
	if dhost.IsDisabledDate(kit.DateEngineDateOf(2024, 3, 10)) {
		t.Fatal("in-window ordinary day must stay enabled")
	}
	if dhost.Select(kit.DateEngineDateOf(2024, 3, 15)) {
		t.Fatal("select must refuse disabled days")
	}

	// Range with order sorts; without order keeps click order.
	rprops := kit.DefaultDateEngineProps()
	rprops.Range = true
	rhost := kit.BuildDateEngine(kit.DefaultScopeCtx(), rprops)
	var rout [2]*kit.DateEngineDate
	rhost.SetOnRangeChange(func(dates [2]*kit.DateEngineDate, strs [2]string) { rout = dates })
	rhost.Select(kit.DateEngineDateOf(2024, 3, 10))
	rhost.Select(kit.DateEngineDateOf(2024, 3, 5))
	if rout[0] == nil || rout[1] == nil || !rout[0].SameDay(kit.DateEngineDateOf(2024, 3, 5)) {
		t.Fatalf("ordered range = %v", rout)
	}
	nprops := kit.DefaultDateEngineProps()
	nprops.Range = true
	nprops.Order, nprops.OrderSet = false, true
	nhost := kit.BuildDateEngine(kit.DefaultScopeCtx(), nprops)
	var nout [2]*kit.DateEngineDate
	nhost.SetOnRangeChange(func(dates [2]*kit.DateEngineDate, strs [2]string) { nout = dates })
	nhost.Select(kit.DateEngineDateOf(2024, 3, 10))
	nhost.Select(kit.DateEngineDateOf(2024, 3, 5))
	if !nout[0].SameDay(kit.DateEngineDateOf(2024, 3, 10)) {
		t.Fatalf("unordered range must keep click order, got %v", nout)
	}

	// Multiple toggles; presets static plus func.
	mprops := kit.DefaultDateEngineProps()
	mprops.Multiple = true
	mhost := kit.BuildDateEngine(kit.DefaultScopeCtx(), mprops)
	mhost.Select(kit.DateEngineDateOf(2024, 1, 1))
	mhost.Select(kit.DateEngineDateOf(2024, 1, 2))
	mhost.Select(kit.DateEngineDateOf(2024, 1, 1))
	if len(mhost.Multiple()) != 1 || !mhost.Multiple()[0].SameDay(kit.DateEngineDateOf(2024, 1, 2)) {
		t.Fatalf("toggle must remove, got %v", mhost.Multiple())
	}
	pprops := kit.DefaultDateEngineProps()
	pprops.Presets = []kit.DateEnginePreset{
		{Label: "static", Value: []kit.DateEngineDate{kit.DateEngineDateOf(2024, 1, 1)}},
		{Label: "func", ValueFunc: func() []kit.DateEngineDate {
			return []kit.DateEngineDate{kit.DateEngineDateOf(2024, 6, 1)}
		}},
	}
	phost := kit.BuildDateEngine(kit.DefaultScopeCtx(), pprops)
	var pstr string
	phost.SetOnChange(func(d *kit.DateEngineDate, s string) { pstr = s })
	if !phost.SelectPreset(0) || pstr != "2024-01-01" {
		t.Fatalf("static preset = %q", pstr)
	}
	if !phost.SelectPreset(1) || pstr != "2024-06-01" {
		t.Fatalf("func preset = %q", pstr)
	}
	if phost.SelectPreset(9) {
		t.Fatal("out-of-range preset must fail")
	}

	// Panel nav + controlled panel.
	ph := kit.BuildDateEngine(kit.DefaultScopeCtx(), kit.DefaultDateEngineProps())
	pv := kit.DateEngineDateOf(2024, 1, 15)
	ph.SetPickerValue(&pv)
	ph.GoPanel(1)
	if ph.PickerValue() == nil || ph.PickerValue().Month != 2 {
		t.Fatalf("panel next must go Feb, got %v", ph.PickerValue())
	}
	ph.DrillDown()
	if ph.Mode() != kit.DateEngineModeMonth {
		t.Fatalf("drill must open month panel, got %q", ph.Mode())
	}
	var panelMode kit.DateEngineMode
	ph.SetOnPanelChange(func(v *kit.DateEngineDate, m kit.DateEngineMode) { panelMode = m })
	ph.DrillDown()
	if panelMode != kit.DateEngineModeYear {
		t.Fatalf("panel callback mode = %q want year", panelMode)
	}
}
