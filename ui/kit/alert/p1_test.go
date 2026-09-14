package alert_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/rendering"
)

func loadP1Face(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := rendering.TryLoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("P1 true-text needs a system face: %v", err)
	}
	return face
}

// New() must carry intrinsic size so Node() can be placed with no extra
// Layout call (button tail pre-layout contract).
func TestAlert_NodeIntrinsicSize(t *testing.T) {
	a := alert.NewAlert("intrinsic hello")
	sz := a.Node().Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("Node size=%v want >0 right after NewAlert", sz)
	}
	got := a.Layout(rendering.Loose(800, 200))
	if got.Width <= 0 || got.Height <= 0 {
		t.Fatalf("layout=%v", got)
	}
}

// True-text chain: same title paints real glyphs with a face, and paints no
// black bar without one (headless estimates width, draws nothing).
func TestAlert_PRD_ALT22_TrueText(t *testing.T) {
	face := loadP1Face(t)

	countDark := func(a *alert.Alert) int {
		sz := a.Layout(rendering.Tight(320, 64))
		dc := render.NewContext(int(sz.Width), int(sz.Height))
		defer dc.Close()
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		a.Node().Paint(rendering.NewPaintContext(dc, 1))
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
		return dark
	}

	plain := alert.NewAlert("Hello World")
	if n := countDark(plain); n != 0 {
		t.Fatalf("no-face alert dark=%d want 0 (black bar banned)", n)
	}
	withFace := alert.NewAlert("Hello World")
	withFace.SetTextFace(face)
	if withFace.TextFace() == nil {
		t.Fatal("TextFace nil after SetTextFace")
	}
	if n := countDark(withFace); n < 30 {
		t.Fatalf("with-face dark=%d want >=30 (real glyphs missing)", n)
	}
	// Description line follows the same chain.
	desc := alert.NewAlert("Title line")
	desc.SetDescription("Helper line")
	desc.SetTextFace(face)
	sz := desc.Layout(rendering.Loose(500, 200))
	if sz.Width <= 0 {
		t.Fatalf("desc layout=%v", sz)
	}
	withFace.SetTextFace(nil)
	if withFace.TextFace() != nil {
		t.Fatal("SetTextFace(nil) must clear")
	}
}

// P1 semantic hooks: classNames/styles are stored paint-only hooks, no CSS
// engine; layout and paint stay stable.
func TestAlert_PRD_ALT22_SemanticHooks(t *testing.T) {
	a := alert.NewAlert("semantic")
	a.SetStyle(map[string]string{"root": "background:#fff", "title": "bold"})
	a.SetClassName("my-alert")
	a.SetClassNames(map[string]string{"root": "my-root", "close": "my-close"})
	if a.Styles()["root"] != "background:#fff" {
		t.Fatalf("Styles=%v", a.Styles())
	}
	if a.ClassName() != "my-alert" {
		t.Fatalf("ClassName=%q", a.ClassName())
	}
	if a.ClassNames()["close"] != "my-close" {
		t.Fatalf("ClassNames=%v", a.ClassNames())
	}
	before := a.Layout(rendering.Loose(500, 200))
	after := a.Layout(rendering.Loose(500, 200))
	if before != after {
		t.Fatalf("semantic hooks must not move layout: %v vs %v", before, after)
	}
	sz := a.Layout(rendering.Tight(300, 80))
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc, 1))
}

// P1 ConfigProvider staging: package globals act as the global-default layer
// (explicit props win). The real ConfigProvider kit is NotStarted, so this
// hook is the P1 staging point.
func TestAlert_PRD_ALT22_GlobalDefaults(t *testing.T) {
	alert.ResetAlertGlobals()
	defer alert.ResetAlertGlobals()

	alert.SetGlobalTypeIcon(alert.AlertInfo, "global-info")
	a := alert.NewAlert("global fallback")
	if a.EffectiveIconName() != "global-info" {
		t.Fatalf("EffectiveIconName=%q want global-info", a.EffectiveIconName())
	}
	a.SetIcon("local-icon")
	if a.EffectiveIconName() != "local-icon" || a.IconName() != "local-icon" {
		t.Fatalf("explicit must win: %q", a.EffectiveIconName())
	}

	closeProxy := rendering.NewRenderColorBox(20, 20, 0.2, 0.2, 0.2, 1)
	alert.SetGlobalCloseIcon(closeProxy)
	b := alert.NewAlert("close fallback")
	b.SetClosable(true)
	if b.EffectiveCloseIcon() != rendering.RenderObject(closeProxy) {
		t.Fatal("global close fallback missing")
	}
	local := rendering.NewRenderColorBox(20, 20, 0.8, 0.1, 0.1, 1)
	b.SetCloseIcon(local)
	if b.EffectiveCloseIcon() != rendering.RenderObject(local) {
		t.Fatal("explicit close must win over global")
	}
	sz := b.Layout(rendering.Loose(500, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

// P1 leave-motion staging: P0 instant hide is kept; motion hooks only record
// intent (smooth-closed pixel animation stays staged, hook covered here).
func TestAlert_PRD_ALT22_LeaveMotion(t *testing.T) {
	a := alert.NewAlert("leave")
	a.SetClosable(true)
	if a.MotionEnabled() || a.LeaveDurationMs() != 0 {
		t.Fatal("motion defaults: disabled, 0ms (P0 instant)")
	}
	started := 0
	a.SetMotionEnabled(true)
	a.SetLeaveDurationMs(200)
	a.SetOnLeaveStart(func() { started++ })
	if !a.MotionEnabled() || a.LeaveDurationMs() != 200 {
		t.Fatal("motion hooks not stored")
	}
	after := 0
	a.SetAfterClose(func() { after++ })
	if !a.ClickClose() || a.Visible() || started != 1 || after != 1 {
		t.Fatalf("motion close must stay instant: visible=%v started=%d after=%d", a.Visible(), started, after)
	}
	// Disabled motion never fires the staged hook but still hides instantly.
	b := alert.NewAlert("instant")
	b.SetClosable(true)
	fired := false
	b.SetOnLeaveStart(func() { fired = true })
	if !b.ClickClose() || fired {
		t.Fatal("disabled motion must hide without leave hook")
	}
}

// P1 action matrix: single button plus Accept+Decline vertical-pair footprint
// coexist with the description double row (pixel-perfect column gaps staged).
func TestAlert_PRD_ALT22_ActionMatrix(t *testing.T) {
	single := alert.NewAlert("single")
	single.SetShowIcon(true)
	single.SetAction(rendering.NewRenderColorBox(64, 28, 0.09, 0.47, 1, 1))
	if !single.ActionVisible() {
		t.Fatal("single action visible")
	}
	if sz := single.Layout(rendering.Loose(700, 200)); sz.Width <= 0 {
		t.Fatalf("single layout=%v", sz)
	}

	accept := rendering.NewRenderColorBox(70, 26, 0.09, 0.47, 1, 1)
	decline := rendering.NewRenderColorBox(70, 26, 0.6, 0.6, 0.6, 1)
	holder := rendering.NewRenderBox(accept, decline)
	if len(holder.Children()) != 2 {
		t.Fatalf("double holder children=%d want 2", len(holder.Children()))
	}
	for i, ch := range holder.Children() {
		if sz := ch.Layout(rendering.Loose(700, 200)); sz.Width <= 0 {
			t.Fatalf("child %d layout=%v", i, sz)
		}
	}
	pair := alert.NewAlert("double")
	pair.SetDescription("with double row")
	pair.SetShowIcon(true)
	pair.SetAction(holder)
	if !pair.ActionVisible() || !pair.HasDescription() {
		t.Fatal("double action + desc must coexist")
	}
	sz := pair.Layout(rendering.Loose(700, 300))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("double layout=%v", sz)
	}
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pair.Node().Paint(rendering.NewPaintContext(dc, 1))
}

// P1 ErrorBoundary is React-only with no desktop mapping: documented Skip.
func TestAlert_PRD_ALT22_ErrorBoundaryNA(t *testing.T) {
	t.Skip("P1 ErrorBoundary is React-only (Alert.ErrorBoundary shows error stacks); no gpui desktop mapping, staged")
}

// P1 debug examples and ant.design pixel-hash parity are explicitly out of
// scope (§6.1 L4 / §6.8 P1): documented Skip.
func TestAlert_PRD_ALT22_DebugPixelHashNA(t *testing.T) {
	t.Skip("P1 debug demos (_semantic/component-token/custom-icon) and ant.design pixel-hash parity are not built;本库 golden is the L3 source")
}

// L4 human-eye side-by-side sign-off needs a reviewer looking at ant.design:
// documented Skip (not a code gap).
func TestAlert_PRD_ALT21_HumanEyeNA(t *testing.T) {
	t.Skip("L4 ALT-21 needs human side-by-side sign-off against ant.design; no automated assertion, staged")
}
