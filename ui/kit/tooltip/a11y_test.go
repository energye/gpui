package tooltip_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Disabled trigger swallows every path (TIP-20).
func TestTooltip_PRD_TIP20_DisabledTrigger(t *testing.T) {
	tp := tooltip.NewTooltip("nope")
	tp.SetTriggerModes(tooltip.TriggerHover, tooltip.TriggerFocus, tooltip.TriggerClick, tooltip.TriggerContextMenu)
	tp.SetMouseEnterDelay(0)
	tp.SetMouseLeaveDelay(0)
	tp.SetDisabled(true)
	tp.Layout(rendering.Loose(600, 200))
	if tp.Focusable() {
		t.Fatal("disabled trigger leaves Tab order")
	}
	tp.HoverEnter()
	tp.Focus()
	tp.Click()
	tp.ContextMenu()
	if tp.IsOpen() {
		t.Fatal("disabled must never open")
	}
	tp.SetDisabled(false)
	if !tp.Focusable() {
		t.Fatal("reenabled trigger rejoins Tab order")
	}
	tp.HoverEnter()
	if !tp.IsOpen() {
		t.Fatal("reenabled hover opens")
	}
}

// Keyboard + focus main path: focus opens, blur closes, Esc closes click.
func TestTooltip_PRD_TIP21_KeyboardFocus(t *testing.T) {
	tp := tooltip.NewTooltip("focus tip")
	tp.SetTriggerLabel("Focus me")
	tp.SetTriggerModes(tooltip.TriggerFocus)
	mgr := focus.NewManager()
	n := tp.FocusNode()
	mgr.Register(n)
	if !tp.Focusable() {
		t.Fatal("trigger Focusable")
	}
	if n.TabIndex < 0 || !n.Enabled {
		t.Fatal("trigger node must be Tab reachable")
	}
	if !mgr.RequestFocus(n) || !n.HasFocus() {
		t.Fatal("RequestFocus")
	}
	if !tp.IsOpen() || !tp.Focused() {
		t.Fatal("focus must open the tip")
	}
	if tp.TriggerRole() != "button" {
		t.Fatalf("Trigger Role=%q", tp.TriggerRole())
	}
	mgr.Blur()
	if tp.IsOpen() {
		t.Fatal("blur must close the tip")
	}
	// Click path opens and Esc closes.
	c := tooltip.NewTooltip("click tip")
	c.SetTriggerModes(tooltip.TriggerClick)
	c.Layout(rendering.Loose(600, 200))
	c.Click()
	if !c.IsOpen() {
		t.Fatal("click opens")
	}
	if !c.PressKey("Escape") || c.IsOpen() {
		t.Fatal("Escape must close click tip")
	}
	// Direct focus helper parity.
	d := tooltip.NewTooltip("direct")
	d.SetTriggerModes(tooltip.TriggerFocus)
	d.Layout(rendering.Loose(600, 200))
	d.Focus()
	if !d.IsOpen() {
		t.Fatal("Focus() opens")
	}
	d.Blur()
	if d.IsOpen() {
		t.Fatal("Blur() closes")
	}
}

// Four rulers: 44px target, contrast, named readability, reader tree.
func TestTooltip_A11y_TargetContrastSemantics(t *testing.T) {
	tp := tooltip.NewTooltip("A11y tip text")
	tp.SetTriggerLabel("A11y trigger")
	tp.SetAriaLabel("Network tip")
	tp.Layout(rendering.Loose(600, 200))
	// 1. Minimum 44px trigger target (hit floor, visuals keep spec size).
	hs := tp.HitSize()
	if hs.Width < 44-1e-9 || hs.Height < 44-1e-9 {
		t.Fatalf("hit=%v want >=44", hs)
	}
	// 2. Text contrast vs bubble (WCAG AA 4.5 for normal text).
	tok := theme.Default.Current()
	fg := render.RGBA{R: tp.TextColor().R, G: tp.TextColor().G, B: tp.TextColor().B, A: 1}
	bg := render.RGBA{R: tp.Background().R, G: tp.Background().G, B: tp.Background().B, A: 1}
	_ = tok
	if r := contrastTip(fg, bg); r < 4.5 {
		t.Fatalf("contrast=%v want >=4.5", r)
	}
	// 3. Named readability.
	if tp.AriaName() != "Network tip" || tp.AriaLabel() != "Network tip" {
		t.Fatalf("Aria %q/%q", tp.AriaName(), tp.AriaLabel())
	}
	// 4. Reader tree: trigger button with a tooltip bubble when open.
	tp.SetTriggerModes(tooltip.TriggerFocus)
	tp.SetMouseEnterDelay(0)
	tp.Focus()
	tree := tp.Semantics()
	if tree == nil || string(tree.Role) != "button" {
		t.Fatalf("root Role=%+v", tree)
	}
	found := false
	for _, ch := range tree.Children {
		if string(ch.Role) == "tooltip" && ch.Label == "A11y tip text" {
			found = true
		}
	}
	if !found {
		t.Fatalf("reader tree missing tooltip bubble: %+v", tree.Children)
	}
	if !tp.FocusNode().Enabled {
		t.Fatal("trigger Focus node must be enabled")
	}
}

func lumChanTip(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func luminanceTip(c render.RGBA) float64 {
	return 0.2126*lumChanTip(c.R) + 0.7152*lumChanTip(c.G) + 0.0722*lumChanTip(c.B)
}

func contrastTip(fg, bg render.RGBA) float64 {
	l1, l2 := luminanceTip(fg), luminanceTip(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}
