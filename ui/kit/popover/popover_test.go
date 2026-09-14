package popover_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/popover"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

type popSpec struct {
	FontSize          float64 `json:"fontSize"`
	InnerPadding      float64 `json:"innerPadding"`
	TitleMinWidth     float64 `json:"titleMinWidth"`
	TitleMarginBottom float64 `json:"titleMarginBottom"`
	Gap               float64 `json:"gap"`
	ArrowSize         float64 `json:"arrowSize"`
	RadiusLG          float64 `json:"radiusLG"`
	LineWidth         float64 `json:"lineWidth"`
	MinTarget         float64 `json:"minTarget"`
	ContrastFloor     float64 `json:"contrastFloor"`
}

func loadPopSpec(t *testing.T) popSpec {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "popover.json"))
	if err != nil {
		t.Fatalf("read popover.json: %v", err)
	}
	var s popSpec
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse popover.json: %v", err)
	}
	if s.FontSize <= 0 || s.TitleMinWidth <= 0 {
		t.Fatalf("bad spec %+v", s)
	}
	return s
}

func closeEnough(got, want float64) bool { return math.Abs(got-want) <= 0.5 }

func TestPopover_PRD_POP01_Defaults(t *testing.T) {
	p := popover.NewPopover("Hover me")
	if p.Placement() != popover.Top {
		t.Fatalf("placement=%s want top", p.Placement())
	}
	if !p.Arrow() {
		t.Fatal("arrow default true")
	}
	got := p.Triggers()
	if len(got) != 1 || got[0] != popover.TriggerHover {
		t.Fatalf("trigger=%v want [hover]", got)
	}
	if p.IsOpen() {
		t.Fatal("default closed")
	}
	if !p.AutoAdjustOverflow() {
		t.Fatal("autoAdjust default true")
	}
	if p.Disabled() || p.Controlled() {
		t.Fatal("default enabled uncontrolled")
	}
	sz := p.Layout(rendering.Loose(800, 600))
	if sz.Width <= 0 || sz.Height <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestPopover_PRD_POP02_Open(t *testing.T) {
	p := popover.NewPopover("Hover me")
	p.SetTitle("Title")
	p.SetContent("Content here")
	p.SetDefaultOpen(true)
	if !p.IsOpen() {
		t.Fatal("defaultOpen must open")
	}
	if p.Title() != "Title" || p.Content() != "Content here" {
		t.Fatalf("title=%q content=%q", p.Title(), p.Content())
	}
	if p.PanelLabel() != "Title" {
		t.Fatalf("panel label=%q want title", p.PanelLabel())
	}
	q := popover.NewPopover("Click me")
	q.SetTrigger(popover.TriggerClick)
	q.SetTitle("T")
	q.SetContent("C")
	q.ClickTrigger()
	if !q.IsOpen() {
		t.Fatal("click must open")
	}
}

func TestPopover_PRD_POP03_Close(t *testing.T) {
	p := popover.NewPopover("Hover me")
	p.SetTitle("T")
	p.SetContent("C")
	p.SetDefaultOpen(true)
	if !p.IsOpen() {
		t.Fatal("precondition open")
	}
	p.SetOpen(false)
	if p.IsOpen() {
		t.Fatal("SetOpen(false) must close")
	}
	q := popover.NewPopover("Click me")
	q.SetTrigger(popover.TriggerClick)
	q.ClickTrigger()
	if !q.IsOpen() {
		t.Fatal("precondition open")
	}
	q.OutsidePress()
	if q.IsOpen() {
		t.Fatal("outside must close")
	}
}

func TestPopover_PRD_POP04_Placement12(t *testing.T) {
	all := []popover.PopoverPlacement{
		popover.Top, popover.TopLeft, popover.TopRight,
		popover.Bottom, popover.BottomLeft, popover.BottomRight,
		popover.Left, popover.LeftTop, popover.LeftBottom,
		popover.Right, popover.RightTop, popover.RightBottom,
	}
	if len(popover.AllPlacements) != 12 {
		t.Fatalf("AllPlacements=%d want 12", len(popover.AllPlacements))
	}
	for _, pl := range all {
		p := popover.NewPopover("anchor")
		p.SetPlacement(pl)
		p.SetTitle("T")
		p.SetContent("C")
		p.Layout(rendering.Loose(800, 600))
		p.SetDefaultOpen(true)
		if !p.IsOpen() {
			t.Fatalf("%s must open", pl)
		}
		anchor := rendering.NewRect(400, 300, 80, 32)
		res := p.Resolve(anchor, p.PanelLaidOut().Width, p.PanelLaidOut().Height, 1000, 800)
		if string(res.Actual) != string(pl) {
			t.Fatalf("%s resolved=%s", pl, res.Actual)
		}
		if res.ArrowX < 0 || res.ArrowY < 0 {
			t.Fatalf("%s arrow=%v,%v", pl, res.ArrowX, res.ArrowY)
		}
	}
}

func TestPopover_PRD_POP05_Controlled(t *testing.T) {
	p := popover.NewPopover("Click me")
	p.SetTrigger(popover.TriggerClick)
	p.SetTitle("T")
	p.SetContent("C")
	p.SetOpen(false)
	calls := 0
	last := true
	p.SetOnOpenChange(func(open bool) { calls++; last = open })
	p.ClickTrigger()
	if p.IsOpen() {
		t.Fatal("controlled must stay closed on click")
	}
	if calls != 1 || !last {
		t.Fatalf("onOpenChange calls=%d last=%v want 1/true", calls, last)
	}
	p.SetOpen(true)
	if !p.IsOpen() {
		t.Fatal("SetOpen(true) must open")
	}
	if calls != 1 {
		t.Fatalf("SetOpen must not callback, calls=%d", calls)
	}
}

func TestPopover_PRD_POP06_ClickToggleOutside(t *testing.T) {
	p := popover.NewPopover("Click me")
	p.SetTrigger(popover.TriggerClick)
	p.SetTitle("T")
	p.SetContent("C")
	if !p.ClickTrigger() || !p.IsOpen() {
		t.Fatal("first click opens")
	}
	if !p.ClickTrigger() || p.IsOpen() {
		t.Fatal("second click closes")
	}
	p.ClickTrigger()
	if !p.IsOpen() {
		t.Fatal("reopen")
	}
	if !p.OutsidePress() || p.IsOpen() {
		t.Fatal("outside closes non-controlled")
	}
	if p.OutsidePress() {
		t.Fatal("outside when closed returns false")
	}
}

func TestPopover_PRD_POP07_ContentAction(t *testing.T) {
	p := popover.NewPopover("Hover me")
	p.SetTitle("Title")
	p.SetContent("Pick action")
	fired := 0
	p.SetContentAction(func() { fired++ })
	p.HoverEnter()
	if !p.IsOpen() {
		t.Fatal("hover must open")
	}
	if !p.PressContentAction() || fired != 1 {
		t.Fatalf("inner action fired=%d", fired)
	}
	if !p.IsOpen() {
		t.Fatal("inner action must not auto close")
	}
	inner := rendering.NewRenderColorBox(60, 28, 0.1, 0.4, 0.9, 1)
	p.SetContentNode(inner)
	if p.ContentNode() == nil {
		t.Fatal("content node missing")
	}
	p.Layout(rendering.Loose(800, 600))
	if !p.PressContentAction() || fired != 2 {
		t.Fatalf("node content action fired=%d", fired)
	}
}

func TestPopover_PRD_POP08_Basic(t *testing.T) {
	p := popover.NewPopover("Hover me")
	p.SetTitle("Title")
	p.SetContent("Content here")
	p.Layout(rendering.Loose(800, 600))
	if p.Title() == "" || p.Content() == "" {
		t.Fatal("basic needs title+content")
	}
	p.HoverEnter()
	if !p.IsOpen() {
		t.Fatal("basic hover opens")
	}
	p.Tick()
	if !p.IsOpen() {
		t.Fatal("tick without leave keeps open")
	}
	p.HoverLeave()
	if !p.IsOpen() {
		t.Fatal("leave arms grace, still open before tick")
	}
	p.Tick()
	if p.IsOpen() {
		t.Fatal("tick after leave closes")
	}
}

func TestPopover_PRD_POP09_ThreeTriggers(t *testing.T) {
	h := popover.NewPopover("Hover")
	h.SetTitle("T")
	h.SetContent("C")
	h.HoverEnter()
	if !h.IsOpen() {
		t.Fatal("hover opens")
	}
	h.HoverLeave()
	h.Tick()

	f := popover.NewPopover("Focus")
	f.SetTrigger(popover.TriggerFocus)
	f.SetTitle("T")
	f.SetContent("C")
	f.FocusTrigger()
	if !f.IsOpen() {
		t.Fatal("focus opens")
	}
	f.BlurTrigger()
	if f.IsOpen() {
		t.Fatal("blur closes")
	}

	c := popover.NewPopover("Click")
	c.SetTrigger(popover.TriggerClick)
	c.SetTitle("T")
	c.SetContent("C")
	c.ClickTrigger()
	if !c.IsOpen() {
		t.Fatal("click opens")
	}
}

func TestPopover_PRD_POP10_PlacementExample(t *testing.T) {
	for _, pl := range popover.AllPlacements {
		p := popover.NewPopover("BTN")
		p.SetPlacement(pl)
		p.SetTitle("T")
		p.SetContent("C")
		p.Layout(rendering.Loose(800, 600))
		p.SetDefaultOpen(true)
		if !p.IsOpen() {
			t.Fatalf("%s constructible", pl)
		}
	}
}

func TestPopover_PRD_POP11_Arrow(t *testing.T) {
	p := popover.NewPopover("anchor")
	p.SetTitle("T")
	p.SetContent("C")
	p.Layout(rendering.Loose(800, 600))
	if !p.Arrow() {
		t.Fatal("default arrow true")
	}
	p.SetArrow(false)
	if p.Arrow() {
		t.Fatal("arrow false")
	}
	p.SetArrowConfig(true, true)
	if !p.Arrow() || !p.PointAtCenter() {
		t.Fatal("arrow config")
	}
	p.SetPlacement(popover.TopLeft)
	p.Layout(rendering.Loose(800, 600))
	anchor := rendering.NewRect(100, 100, 120, 32)
	a := p.Resolve(anchor, p.PanelLaidOut().Width, p.PanelLaidOut().Height, 800, 600)
	p.SetArrowConfig(true, false)
	b := p.Resolve(anchor, p.PanelLaidOut().Width, p.PanelLaidOut().Height, 800, 600)
	_ = a
	_ = b
	if p.EffectivePlacement() != popover.TopLeft {
		t.Fatal("placement kept")
	}
}

func TestPopover_PRD_POP12_Shift(t *testing.T) {
	mk := func(auto bool) *popover.Popover {
		p := popover.NewPopover("anchor")
		p.SetPlacement(popover.Top)
		p.SetTitle("T")
		p.SetContent("C")
		p.SetAutoAdjustOverflow(auto)
		p.SetViewport(300, 200)
		p.Layout(rendering.Loose(800, 600))
		return p
	}
	anchor := rendering.NewRect(140, 10, 60, 24)
	on := mk(true)
	off := mk(false)
	if !on.AutoAdjustOverflow() || off.AutoAdjustOverflow() {
		t.Fatal("auto flag")
	}
	ow, oh := on.PanelLaidOut().Width, on.PanelLaidOut().Height
	rOn := on.Resolve(anchor, ow, oh, 300, 200)
	rOff := off.Resolve(anchor, ow, oh, 300, 200)
	if rOn.Y < 0 {
		t.Fatalf("shift must keep on screen y=%v", rOn.Y)
	}
	if rOff.Y >= 0 {
		t.Fatalf("no-adjust keeps ideal overflow y=%v", rOff.Y)
	}
	var _ overlay.Placement = rOn.Actual
}

func TestPopover_PRD_POP13_ControlClose(t *testing.T) {
	p := popover.NewPopover("Click me")
	p.SetTrigger(popover.TriggerClick)
	p.SetTitle("Title")
	p.SetContent("Close me")
	p.SetOpen(true)
	if !p.IsOpen() {
		t.Fatal("controlled open")
	}
	closed := false
	p.SetContentAction(func() {
		closed = true
		p.SetOpen(false)
	})
	if !p.PressContentAction() || !closed {
		t.Fatal("inner close action")
	}
	if p.IsOpen() {
		t.Fatal("business SetOpen(false) closes")
	}
}

func TestPopover_PRD_POP14_HoverClick(t *testing.T) {
	p := popover.NewPopover("Hover+Click")
	p.SetTriggerModes(popover.TriggerHover, popover.TriggerClick)
	p.SetTitle("T")
	p.SetContent("C")
	if len(p.Triggers()) != 2 {
		t.Fatalf("modes=%v", p.Triggers())
	}
	p.HoverEnter()
	if !p.IsOpen() {
		t.Fatal("hover part opens")
	}
	p.HoverLeave()
	p.Tick()
	if p.IsOpen() {
		t.Fatal("hover part closes")
	}
	p.ClickTrigger()
	if !p.IsOpen() {
		t.Fatal("click part opens")
	}
	p.ClickTrigger()
	if p.IsOpen() {
		t.Fatal("click part closes")
	}
	ctx := popover.NewPopover("menu")
	ctx.SetTrigger(popover.TriggerContextMenu)
	ctx.SetTitle("T")
	ctx.SetContent("C")
	if !ctx.ContextMenu() || !ctx.IsOpen() {
		t.Fatal("contextMenu opens")
	}
}

func TestPopover_PRD_POP16_Metrics(t *testing.T) {
	s := loadPopSpec(t)
	p := popover.NewPopover("x")
	if !closeEnough(p.FontSize(), s.FontSize) {
		t.Fatalf("font=%v want %v", p.FontSize(), s.FontSize)
	}
	if !closeEnough(p.InnerPadding(), s.InnerPadding) {
		t.Fatalf("pad=%v want %v", p.InnerPadding(), s.InnerPadding)
	}
	if !closeEnough(p.TitleMinWidth(), s.TitleMinWidth) {
		t.Fatalf("titleMin=%v want %v", p.TitleMinWidth(), s.TitleMinWidth)
	}
	if !closeEnough(p.TitleMarginBottom(), s.TitleMarginBottom) {
		t.Fatalf("titleGap=%v want %v", p.TitleMarginBottom(), s.TitleMarginBottom)
	}
	if !closeEnough(p.Gap(), s.Gap) {
		t.Fatalf("gap=%v want %v", p.Gap(), s.Gap)
	}
	if !closeEnough(p.ArrowSize(), s.ArrowSize) {
		t.Fatalf("arrow=%v want %v", p.ArrowSize(), s.ArrowSize)
	}
	if !closeEnough(p.Radius(), s.RadiusLG) {
		t.Fatalf("radius=%v want %v", p.Radius(), s.RadiusLG)
	}
	if !closeEnough(p.LineWidth(), s.LineWidth) {
		t.Fatalf("lw=%v want %v", p.LineWidth(), s.LineWidth)
	}
	sz := p.Layout(rendering.Loose(800, 600))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
}

func TestPopover_PRD_POP17_ThemeColors(t *testing.T) {
	tok := theme.Default.Current()
	p := popover.NewPopover("x")
	p.SetTitle("T")
	p.SetContent("C")
	bg := p.PanelBackground()
	wantBG := render.RGBA{R: tok.ColorBgContainer.R, G: tok.ColorBgContainer.G, B: tok.ColorBgContainer.B, A: tok.ColorBgContainer.A}
	if bg != wantBG {
		t.Fatalf("panel bg=%+v want Theme ColorBgContainer %+v", bg, wantBG)
	}
	fg := p.ContentColor()
	wantTx := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: tok.ColorText.A}
	if fg != wantTx {
		t.Fatalf("content=%+v want %+v", fg, wantTx)
	}
	bd := p.BorderColor()
	wantBd := render.RGBA{R: tok.ColorBorder.R, G: tok.ColorBorder.G, B: tok.ColorBorder.B, A: tok.ColorBorder.A}
	if bd != wantBd {
		t.Fatalf("border=%+v want %+v", bd, wantBd)
	}
	mod := theme.DefaultTokens()
	mod.ColorBgContainer = theme.Hex("#141414")
	p.SetTheme(&mod)
	if p.PanelBackground() == bg {
		t.Fatal("SetTheme must move panel background (Theme Token wiring)")
	}
	prov := theme.NewProvider(theme.DefaultTokens())
	q := popover.NewPopover("y")
	q.SetProvider(prov)
	before := q.PanelBackground()
	nt := prov.Current()
	nt.ColorBgContainer = theme.Hex("#141414")
	prov.SetBase(nt)
	if q.PanelBackground() == before {
		t.Fatal("provider must move background")
	}
}

func TestPopover_PRD_POP18_Disabled(t *testing.T) {
	p := popover.NewPopover("Hover me")
	p.SetTitle("T")
	p.SetContent("C")
	p.SetDisabled(true)
	if !p.Disabled() {
		t.Fatal("disabled flag")
	}
	p.HoverEnter()
	if p.IsOpen() {
		t.Fatal("disabled swallows hover")
	}
	p.Tick()
	if p.IsOpen() {
		t.Fatal("still closed after tick")
	}
	c := popover.NewPopover("Click")
	c.SetTrigger(popover.TriggerClick)
	c.SetDisabled(true)
	if c.ClickTrigger() || c.IsOpen() {
		t.Fatal("disabled swallows click")
	}
	if c.Focusable() {
		t.Fatal("disabled not Focusable")
	}
}

func TestPopover_PRD_POP19_KeyboardFocus(t *testing.T) {
	p := popover.NewPopover("Click me")
	p.SetTriggerModes(popover.TriggerClick, popover.TriggerFocus)
	p.SetTitle("Title")
	p.SetContent("Body")
	p.SetAriaLabel("More info")
	if p.Role() != "button" {
		t.Fatalf("trigger Role=%q want button", p.Role())
	}
	if p.PanelRole() != "dialog" {
		t.Fatalf("panel Role=%q want dialog", p.PanelRole())
	}
	if p.AriaName() != "More info" {
		t.Fatalf("Aria=%q", p.AriaName())
	}
	if !p.Focusable() {
		t.Fatal("enabled Focusable")
	}
	mgr := focus.NewManager()
	n := p.FocusNode()
	mgr.Register(n)
	if !mgr.RequestFocus(n) || !p.Focused() {
		t.Fatal("RequestFocus must Focus")
	}
	if !p.IsOpen() {
		t.Fatal("focus trigger opens on Focus")
	}
	if !p.Escape() || p.IsOpen() {
		t.Fatal("Esc closes")
	}
	p.FocusTrigger()
	if !p.IsOpen() {
		t.Fatal("refocus opens")
	}
	if !p.PressKey("Escape") || p.IsOpen() {
		t.Fatal("PressKey Escape closes")
	}
	if !p.PressKey("Enter") || !p.IsOpen() {
		t.Fatal("Enter toggles click trigger")
	}
	tree := p.Semantics()
	if tree == nil {
		t.Fatal("semantics nil")
	}
	foundBtn, foundDlg := false, false
	for _, ch := range tree.Children {
		if string(ch.Role) == "button" && ch.Focusable {
			foundBtn = true
		}
		if string(ch.Role) == "dialog" {
			foundDlg = true
		}
	}
	if !foundBtn || !foundDlg {
		t.Fatalf("reader tree button=%v dialog=%v", foundBtn, foundDlg)
	}
}

func TestPopover_A11y_TargetContrast(t *testing.T) {
	s := loadPopSpec(t)
	p := popover.NewPopover("More")
	p.SetTitle("Title")
	p.SetContent("Body text")
	p.SetAriaLabel("More info")
	if hs := p.HitSize(); hs.Width < s.MinTarget || hs.Height < s.MinTarget {
		t.Fatalf("hit=%+v want >=%.0f", hs, s.MinTarget)
	}
	tok := theme.Default.Current()
	fg := render.RGBA{R: tok.ColorText.R, G: tok.ColorText.G, B: tok.ColorText.B, A: 1}
	bg := p.PanelBackground()
	bg.A = 1
	if r := popover.ContrastRatio(fg, bg); r < s.ContrastFloor {
		t.Fatalf("contrast=%.2f below %.1f", r, s.ContrastFloor)
	}
	if p.Role() != "button" || p.PanelRole() != "dialog" {
		t.Fatal("Role pair")
	}
	if p.AriaName() != "More info" || p.PanelLabel() != "Title" {
		t.Fatal("Aria names")
	}
	if !p.Focusable() || !p.FocusNode().Enabled {
		t.Fatal("Focus enabled")
	}
}

func TestPopover_ThemeMirror_ThreeThemesRTL(t *testing.T) {
	light := theme.DefaultTokens()
	dark := theme.DefaultTokens()
	dark.ColorBgContainer = theme.Hex("#141414")
	dark.ColorText = theme.RGBA(255, 255, 255, 0.85)
	compact := theme.DefaultTokens()
	compact.ControlHeight = 28
	for i, tk := range []theme.Tokens{light, dark, compact} {
		tk := tk
		p := popover.NewPopover("x")
		p.SetTitle("T")
		p.SetContent("C")
		p.SetTheme(&tk)
		_ = p.PanelBackground()
		if p.Radius() != tk.RadiusLG {
			t.Fatalf("theme %d radius", i)
		}
	}
	ltr := popover.NewPopover("x")
	ltr.SetPlacement(popover.Left)
	rtl := popover.NewPopover("x")
	rtl.SetPlacement(popover.Left)
	rtl.SetRTL(true)
	if ltr.EffectivePlacement() == rtl.EffectivePlacement() {
		t.Fatal("RTL must mirror Left to Right")
	}
	if rtl.EffectivePlacement() != popover.Right {
		t.Fatalf("rtl=%s want right", rtl.EffectivePlacement())
	}
	if !rtl.IsRTL() || ltr.IsRTL() {
		t.Fatal("IsRTL flag")
	}
}
