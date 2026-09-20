package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func colorValue(hex string) kit.ColorModelValue {
	c, _ := kit.ParseColorModelColor(hex)
	return kit.ColorModelValue{Single: c}
}

func TestColorModel_StateMachineMatchesCases(t *testing.T) {
	// CP-S1/S2: drag fires change, release fires complete.
	host := kit.BuildColorModel(kit.DefaultScopeCtx(), kit.DefaultColorModelProps())
	changes := 0
	completes := 0
	var lastCSS string
	host.SetOnChange(func(v kit.ColorModelValue, css string) { changes++; lastCSS = css })
	host.SetOnComplete(func(v kit.ColorModelValue) { completes++ })
	host.BeginDrag()
	if !host.SetHSB(215, 0.91, 1, 1) {
		t.Fatal("SetHSB must succeed")
	}
	if changes != 1 || lastCSS == "" {
		t.Fatalf("change = %d css %q", changes, lastCSS)
	}
	if completes != 0 {
		t.Fatal("complete must wait for release")
	}
	host.EndDrag()
	if completes != 1 {
		t.Fatal("EndDrag must fire complete once")
	}
	host.CommitChange()
	if completes != 2 {
		t.Fatal("CommitChange must fire complete")
	}

	// CP-S4: clear path.
	cprops := kit.DefaultColorModelProps()
	cprops.AllowClear = true
	cprops.DefaultValue = colorValue("#1677ff")
	cprops.DefaultValueSet = true
	chost := kit.BuildColorModel(kit.DefaultScopeCtx(), cprops)
	cleared := 0
	chost.SetOnClear(func() { cleared++ })
	if !chost.Clear() || cleared != 1 {
		t.Fatal("clear must fire onClear once")
	}
	if !chost.Value().IsEmpty() {
		t.Fatal("cleared value must report empty")
	}
	noprops := kit.DefaultColorModelProps()
	nhost := kit.BuildColorModel(kit.DefaultScopeCtx(), noprops)
	if nhost.Clear() {
		t.Fatal("clear without allowClear must fail")
	}

	// CP-S5/S6: disabled matrix.
	dprops := kit.DefaultColorModelProps()
	dprops.Disabled = true
	dhost := kit.BuildColorModel(kit.DefaultScopeCtx(), dprops)
	if dhost.SetHSB(0, 1, 1, 1) {
		t.Fatal("disabled must block SetHSB")
	}
	if dhost.ToggleOpen() {
		t.Fatal("disabled must block open")
	}
	apropos := kit.DefaultColorModelProps()
	apropos.DisabledAlpha = true
	ahost := kit.BuildColorModel(kit.DefaultScopeCtx(), apropos)
	if ahost.HasAlphaSlider() {
		t.Fatal("disabledAlpha must hide alpha slider")
	}
	ahost.SetHSB(215, 0.9, 1, 0.3)
	if got := ahost.CurrentColor().A; got != 1 {
		t.Fatalf("disabledAlpha forces A=1, got %v", got)
	}
	if ahost.DragAlpha(0.2) {
		t.Fatal("disabledAlpha must block alpha drag")
	}

	// CP-S8: controlled value never self-moves; outside drives it.
	vprops := kit.DefaultColorModelProps()
	vhost := kit.BuildColorModel(kit.DefaultScopeCtx(), vprops)
	keep := colorValue("#1677ff")
	vhost.SetValue(keep)
	fired := false
	vhost.SetOnChange(func(v kit.ColorModelValue, css string) { fired = true })
	vhost.SetHSB(0, 1, 1, 1)
	if vhost.Value().Single.ToHexString() != "#1677ff" {
		t.Fatalf("controlled must keep #1677ff, got %s", vhost.Value().Single.ToHexString())
	}
	if !fired {
		t.Fatal("controlled change must still notify")
	}

	// Format/mode switches.
	fhost := kit.BuildColorModel(kit.DefaultScopeCtx(), kit.DefaultColorModelProps())
	if !fhost.SetFormat(kit.ColorModelFormatRGB) || fhost.Format() != kit.ColorModelFormatRGB {
		t.Fatal("format must switch to rgb")
	}
	mprops := kit.DefaultColorModelProps()
	mprops.Modes = []kit.ColorModelMode{kit.ColorModelModeSingle, kit.ColorModelModeGradient}
	mhost := kit.BuildColorModel(kit.DefaultScopeCtx(), mprops)
	mhost.SetHSB(215, 0.9, 1, 1)
	if !mhost.SetMode(kit.ColorModelModeGradient) || !mhost.Value().IsGradient() {
		t.Fatal("mode gradient must convert to stops")
	}
	if !mhost.SetMode(kit.ColorModelModeSingle) || mhost.Value().IsGradient() {
		t.Fatal("mode single must collapse stops")
	}
	if mhost.SetMode("nope") {
		t.Fatal("unknown mode must fail")
	}

	// Presets single + gradient flip mode.
	pprops := kit.DefaultColorModelProps()
	pprops.Modes = []kit.ColorModelMode{kit.ColorModelModeSingle, kit.ColorModelModeGradient}
	blue, _ := kit.ParseColorModelColor("#1677ff")
	red, _ := kit.ParseColorModelColor("#ff0000")
	pprops.Presets = []kit.ColorModelPreset{
		{Label: "singles", Colors: []kit.ColorModelValue{{Single: blue}}},
		{Label: "grad", Colors: []kit.ColorModelValue{{Gradient: true, Stops: []kit.ColorModelStop{{Color: blue, Percent: 0}, {Color: red, Percent: 100}}}}},
	}
	phost := kit.BuildColorModel(kit.DefaultScopeCtx(), pprops)
	if !phost.SelectPreset(0, 0) || phost.Value().Single.ToHexString() != "#1677ff" {
		t.Fatal("single preset must apply")
	}
	if !phost.SelectPreset(1, 0) || !phost.Value().IsGradient() || phost.Mode() != kit.ColorModelModeGradient {
		t.Fatal("gradient preset must flip mode")
	}
	if phost.SelectPreset(9, 0) {
		t.Fatal("out-of-range preset must fail")
	}

	// Gradient stop add/move/remove/select.
	if !phost.AddStop(50) || len(phost.Value().Stops) != 3 {
		t.Fatalf("add stop must grow to 3, got %d", len(phost.Value().Stops))
	}
	if !phost.MoveStop(2, 75) {
		t.Fatal("move stop must succeed")
	}
	if !phost.SelectStop(1) || phost.ActiveStop() != 1 {
		t.Fatal("select stop must track active")
	}
	if phost.RemoveStop(0) || phost.RemoveStop(2) {
		t.Fatal("terminal stops must refuse delete")
	}
	if !phost.RemoveStop(1) || len(phost.Value().Stops) != 2 {
		t.Fatal("middle stop must delete")
	}

	// Panel drags + open/esc + trigger key + arrows.
	qhost := kit.BuildColorModel(kit.DefaultScopeCtx(), kit.DefaultColorModelProps())
	qhost.SetHSB(215, 0.9, 1, 1)
	if !qhost.DragSV(0.5, 0.5) || !qhost.DragHue(0.5) || !qhost.DragAlpha(0.5) {
		t.Fatal("sv/hue/alpha drags must succeed")
	}
	opened := false
	qhost.SetOnOpenChange(func(o bool) { opened = o })
	if !qhost.ToggleOpen() || !opened || !qhost.IsOpen() {
		t.Fatal("toggle must open and notify")
	}
	qhost.Focus(kit.ColorModelFocusHue)
	before := qhost.CurrentColor().H
	if !qhost.PressArrow("right") || qhost.CurrentColor().H <= before {
		t.Fatal("hue arrow must step up")
	}
	if !qhost.PressEsc() || qhost.IsOpen() {
		t.Fatal("esc must close")
	}
	if !qhost.PressTriggerKey() || qhost.Focused() != kit.ColorModelFocusSV {
		t.Fatal("trigger key must open and enter panel")
	}
}
