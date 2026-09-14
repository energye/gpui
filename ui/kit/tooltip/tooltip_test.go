package tooltip_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/tooltip"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
)

// Queue→Layout→Frame headless: one PRD id per test (TIP-01…TIP-17 here;
// TIP-18…21 live in layout/theme/a11y files).

func TestTooltip_PRD_TIP01_Defaults(t *testing.T) {
	tp := tooltip.NewTooltip("hello")
	if tp.Title() != "hello" {
		t.Fatalf("title=%q", tp.Title())
	}
	if tp.Placement() != tooltip.Top {
		t.Fatalf("placement=%s want top", tp.Placement())
	}
	if !tp.Arrow() || tp.ArrowPointAtCenter() {
		t.Fatal("arrow default true/center false")
	}
	got := tp.Triggers()
	if len(got) != 1 || got[0] != tooltip.TriggerHover {
		t.Fatalf("triggers=%v want [hover]", got)
	}
	if tp.MouseEnterDelay() != 0.1 || tp.MouseLeaveDelay() != 0.1 {
		t.Fatalf("delay=%v/%v want 0.1/0.1", tp.MouseEnterDelay(), tp.MouseLeaveDelay())
	}
	if tp.IsOpen() {
		t.Fatal("default closed")
	}
	if tp.Role() != "tooltip" {
		t.Fatalf("role=%q want tooltip", tp.Role())
	}
	sz := tp.Layout(rendering.Loose(800, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestTooltip_PRD_TIP02_HoverOpen(t *testing.T) {
	tp := tooltip.NewTooltip("prompt text")
	tp.SetTriggerLabel("Hover me")
	tp.SetMouseEnterDelay(0)
	tp.Layout(rendering.Loose(600, 200))
	tp.HoverEnter()
	if !tp.IsOpen() {
		t.Fatal("hover must open")
	}
	if !tp.Hovered() {
		t.Fatal("hovered flag")
	}
	if tp.Popup() == nil || tp.Panel() == nil {
		t.Fatal("popup/panel view")
	}
}

func TestTooltip_PRD_TIP03_LeaveClose(t *testing.T) {
	tp := tooltip.NewTooltip("prompt text")
	tp.SetMouseEnterDelay(0)
	tp.SetMouseLeaveDelay(0)
	tp.Layout(rendering.Loose(600, 200))
	tp.HoverEnter()
	if !tp.IsOpen() {
		t.Fatal("setup open")
	}
	tp.HoverLeave()
	if tp.IsOpen() {
		t.Fatal("leave must close")
	}
}

func TestTooltip_PRD_TIP04_PlacementBottom(t *testing.T) {
	tp := tooltip.NewTooltip("bottom tip")
	tp.SetPlacement(tooltip.Bottom)
	tp.SetAnchor(400, 300)
	tp.SetViewport(1200, 800)
	tp.Layout(rendering.Loose(600, 200))
	if tp.Placement() != tooltip.Bottom || tp.ActualPlacement() != tooltip.Bottom {
		t.Fatalf("placement=%s actual=%s", tp.Placement(), tp.ActualPlacement())
	}
	r := tp.PopupRect()
	_, ay := tp.Anchor()
	if r.Min.Y <= ay {
		t.Fatalf("bottom popup %+v must sit below anchor y=%v", r, ay)
	}
	// All twelve construct without crashing and keep size.
	for _, p := range tooltip.AllPlacements {
		x := tooltip.NewTooltip(string(p))
		x.SetPlacement(p)
		x.SetAnchor(400, 300)
		sz := x.Layout(rendering.Loose(600, 200))
		if sz.Width <= 0 || x.Popup() == nil {
			t.Fatalf("%s layout/popup", p)
		}
	}
	if len(tooltip.AllPlacements) != 12 {
		t.Fatalf("placements=%d want 12", len(tooltip.AllPlacements))
	}
	var _ overlay.Placement = overlay.Top
}

func TestTooltip_PRD_TIP05_ControlledOpen(t *testing.T) {
	tp := tooltip.NewTooltip("controlled")
	tp.SetTriggerModes(tooltip.TriggerClick)
	tp.SetMouseEnterDelay(0)
	calls := 0
	last := false
	tp.SetOnOpenChange(func(v bool) { calls++; last = v })
	tp.Layout(rendering.Loose(600, 200))
	// Take controlled ownership first (mirrors passing the open prop).
	tp.SetOpen(false)
	tp.Click()
	if tp.IsOpen() {
		t.Fatal("controlled trigger must not mutate Open")
	}
	if calls != 1 || !last {
		t.Fatalf("onOpenChange calls=%d last=%v", calls, last)
	}
	tp.SetOpen(true)
	if !tp.IsOpen() {
		t.Fatal("SetOpen(true) must show")
	}
	tp.SetOpen(false)
	if tp.IsOpen() {
		t.Fatal("SetOpen(false) must hide")
	}
}

func TestTooltip_PRD_TIP06_EmptyTitleNeverOpens(t *testing.T) {
	tp := tooltip.NewTooltip("")
	tp.SetTriggerModes(tooltip.TriggerHover, tooltip.TriggerFocus, tooltip.TriggerClick, tooltip.TriggerContextMenu)
	tp.SetMouseEnterDelay(0)
	tp.SetMouseLeaveDelay(0)
	tp.Layout(rendering.Loose(600, 200))
	tp.HoverEnter()
	tp.Focus()
	tp.Click()
	tp.ContextMenu()
	if tp.IsOpen() {
		t.Fatal("empty title must never open")
	}
}

func TestTooltip_PRD_TIP07_ArrowModes(t *testing.T) {
	tp := tooltip.NewTooltip("arrow")
	if !tp.HasArrow() {
		t.Fatal("arrow default on")
	}
	tp.SetArrow(false)
	if tp.HasArrow() {
		t.Fatal("arrow off")
	}
	tp.SetArrowConfig(true, true)
	tp.SetPlacement(tooltip.TopLeft)
	tp.Layout(rendering.Loose(600, 200))
	if tp.EffectivePlacement() != tooltip.Top {
		t.Fatalf("center collapses corner to axis, got %s", tp.EffectivePlacement())
	}
	plain := tooltip.NewTooltip("plain")
	plain.SetPlacement(tooltip.TopLeft)
	plain.Layout(rendering.Loose(600, 200))
	if plain.EffectivePlacement() != tooltip.TopLeft {
		t.Fatalf("no-center keeps corner, got %s", plain.EffectivePlacement())
	}
}

func TestTooltip_PRD_TIP08_ColorPresetAndHex(t *testing.T) {
	tp := tooltip.NewTooltip("color")
	tp.Layout(rendering.Loose(600, 200))
	def := tp.Background()
	tp.SetColor("red")
	red := tp.Background()
	if red == def {
		t.Fatal("preset must move background")
	}
	if !(red.R > 0.8 && red.R > red.G+0.3) {
		t.Fatalf("red bg=%+v want red strongest", red)
	}
	tp.SetColor("#00ff00")
	hex := tp.Background()
	if !(hex.G > 0.8 && hex.G > hex.R+0.3) {
		t.Fatalf("hex bg=%+v want green strongest", hex)
	}
	tp.SetColor("")
	if tp.Background() != def {
		t.Fatal("empty color restores default skin")
	}
}

func TestTooltip_PRD_TIP09_EnterDelayTicks(t *testing.T) {
	tp := tooltip.NewTooltip("delayed")
	tp.SetTriggerLabel("Hover me")
	tp.SetMouseEnterDelay(0.5)
	tp.Layout(rendering.Loose(600, 200))
	tp.AttachTicker(nil)
	tp.HoverEnter()
	if tp.IsOpen() {
		t.Fatal("delay must gate open")
	}
	if tp.Tick(0.2) {
		t.Fatal("tick under delay must not fire")
	}
	if tp.IsOpen() {
		t.Fatal("still gated")
	}
	if !tp.Tick(0.4) {
		t.Fatal("tick past delay must fire")
	}
	if !tp.IsOpen() {
		t.Fatal("open after ticks")
	}
	// Zero delay opens at once.
	zero := tooltip.NewTooltip("instant")
	zero.SetMouseEnterDelay(0)
	zero.Layout(rendering.Loose(600, 200))
	zero.HoverEnter()
	if !zero.IsOpen() {
		t.Fatal("zero delay opens at once")
	}
}

func TestTooltip_PRD_TIP10_BasicExample(t *testing.T) {
	tp := tooltip.NewTooltip("prompt text")
	tp.SetTriggerLabel("Hover me")
	tp.SetMouseEnterDelay(0)
	sz := tp.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("basic layout=%v", sz)
	}
	tp.HoverEnter()
	if !tp.IsOpen() || tp.PanelSize().Width <= 0 {
		t.Fatal("basic must open with a sized panel")
	}
	if tp.TriggerShell() == nil || tp.Node() == nil {
		t.Fatal("shell/node views")
	}
}

func TestTooltip_PRD_TIP11_SmoothTransitionDegraded(t *testing.T) {
	// P0 carries the multi-tip layout without ConfigProvider unique (P1).
	a := tooltip.NewTooltip("first")
	b := tooltip.NewTooltip("second")
	for _, tp := range []*tooltip.Tooltip{a, b} {
		tp.SetMouseEnterDelay(0)
		tp.SetMouseLeaveDelay(0)
		tp.Layout(rendering.Loose(600, 200))
	}
	a.HoverEnter()
	b.HoverEnter()
	if !a.IsOpen() || !b.IsOpen() {
		t.Fatal("both tips open independently")
	}
	a.HoverLeave()
	if a.IsOpen() || !b.IsOpen() {
		t.Fatal("closing one keeps the other")
	}
}

func TestTooltip_PRD_TIP12_PlacementExample(t *testing.T) {
	for _, p := range tooltip.AllPlacements {
		tp := tooltip.NewTooltip("tip " + string(p))
		tp.SetPlacement(p)
		tp.SetAnchor(400, 300)
		tp.SetViewport(1200, 800)
		tp.SetMouseEnterDelay(0)
		tp.Layout(rendering.Loose(600, 200))
		tp.HoverEnter()
		if !tp.IsOpen() {
			t.Fatalf("%s must open", p)
		}
		if tp.Popup() == nil || tp.ActualPlacement() != p {
			t.Fatalf("%s actual=%s", p, tp.ActualPlacement())
		}
	}
}

func TestTooltip_PRD_TIP13_ArrowExample(t *testing.T) {
	show := tooltip.NewTooltip("show arrow")
	show.SetArrow(true)
	hide := tooltip.NewTooltip("hide arrow")
	hide.SetArrow(false)
	center := tooltip.NewTooltip("center arrow")
	center.SetPlacement(tooltip.TopLeft)
	center.SetArrowConfig(true, true)
	for _, tp := range []*tooltip.Tooltip{show, hide, center} {
		tp.Layout(rendering.Loose(600, 200))
	}
	if !show.HasArrow() || hide.HasArrow() {
		t.Fatal("show/hide arrow")
	}
	if center.EffectivePlacement() != tooltip.Top {
		t.Fatalf("center=%s want top", center.EffectivePlacement())
	}
}

func TestTooltip_PRD_TIP14_ShiftExample(t *testing.T) {
	st := overlay.New()
	tp := tooltip.NewTooltip("edge tip")
	tp.SetViewport(400, 300)
	tp.SetAnchor(390, 150)
	tp.SetPlacement(tooltip.Right)
	tp.SetAutoAdjustOverflow(true)
	tp.SetMouseEnterDelay(0)
	tp.SetMouseLeaveDelay(0)
	tp.Layout(rendering.Loose(600, 200))
	tp.AttachOverlay(st)
	tp.HoverEnter()
	if !tp.IsOpen() || st.Len() != 1 {
		t.Fatalf("shift tip open=%v entries=%d", tp.IsOpen(), st.Len())
	}
	r := tp.PopupRect()
	if r.Max.X > 400+0.5 || r.Min.X < -0.5 {
		t.Fatalf("shifted popup %+v must stay in 400px viewport", r)
	}
	tp.HoverLeave()
	if tp.IsOpen() || st.Len() != 0 {
		t.Fatalf("close removes entry: open=%v entries=%d", tp.IsOpen(), st.Len())
	}
	// Overflow without adjustment keeps the wanted side.
	raw := tooltip.NewTooltip("raw edge")
	raw.SetViewport(400, 300)
	raw.SetAnchor(390, 150)
	raw.SetPlacement(tooltip.Right)
	raw.SetAutoAdjustOverflow(false)
	raw.Layout(rendering.Loose(600, 200))
	if raw.ActualPlacement() != tooltip.Right {
		t.Fatalf("no-adjust keeps side, got %s", raw.ActualPlacement())
	}
}

func TestTooltip_PRD_TIP15_ColorfulExample(t *testing.T) {
	names := []string{"pink", "red", "yellow", "orange", "cyan", "green", "blue", "purple", "geekblue", "magenta", "volcano", "gold", "lime"}
	base := tooltip.NewTooltip("base")
	base.Layout(rendering.Loose(600, 200))
	def := base.Background()
	seen := map[string]bool{}
	for _, n := range names {
		tp := tooltip.NewTooltip(n)
		tp.SetColor(n)
		tp.Layout(rendering.Loose(600, 200))
		bg := tp.Background()
		if bg == def {
			t.Fatalf("%s must leave the default skin", n)
		}
		seen[sprintBG(bg)] = true
	}
	if len(seen) < 6 {
		t.Fatalf("colorful wants distinct skins, got %d", len(seen))
	}
	hex := tooltip.NewTooltip("hex")
	hex.SetColor("#ff5500")
	hex.Layout(rendering.Loose(600, 200))
	if hex.Background() == def {
		t.Fatal("custom hex must apply")
	}
}

func TestTooltip_PRD_TIP16_DisabledExample(t *testing.T) {
	tp := tooltip.NewTooltip("")
	tp.SetMouseEnterDelay(0)
	tp.Layout(rendering.Loose(600, 200))
	tp.HoverEnter()
	if tp.IsOpen() {
		t.Fatal("empty title stays shut")
	}
	tp.SetTitle("now has text")
	tp.HoverEnter()
	if !tp.IsOpen() {
		t.Fatal("title restores opening")
	}
}

func TestTooltip_PRD_TIP17_CustomChildExample(t *testing.T) {
	tp := tooltip.NewTooltip("slot")
	customText := rendering.NewRenderText("custom title node")
	customText.FontSize = 14
	trigger := rendering.NewRenderColorBox(90, 32, 0.1, 0.4, 0.9, 1)
	tp.SetTitleNode(customText)
	tp.SetTriggerNode(trigger)
	tp.SetMouseEnterDelay(0)
	sz := tp.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("custom layout=%v", sz)
	}
	if tp.TitleNode() == nil || tp.TriggerNode() == nil {
		t.Fatal("custom slots")
	}
	tp.HoverEnter()
	if !tp.IsOpen() {
		t.Fatal("custom trigger opens")
	}
}

func sprintBG(v interface {
	RGBA() (uint32, uint32, uint32, uint32)
}) string {
	r, g, b, a := v.RGBA()
	return string(rune(r)) + string(rune(g)) + string(rune(b)) + string(rune(a))
}
