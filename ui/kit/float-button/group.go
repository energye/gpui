package float_button

import (
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// FloatButtonTrigger selects the Group menu behavior (antd trigger).
type FloatButtonTrigger string

const (
	FloatButtonTriggerNone  FloatButtonTrigger = "none"
	FloatButtonTriggerClick FloatButtonTrigger = "click"
	FloatButtonTriggerHover FloatButtonTrigger = "hover"
)

// FloatButtonPlacement selects the Group menu expand direction (antd placement).
type FloatButtonPlacement string

const (
	FloatButtonPlacementTop    FloatButtonPlacement = "top"
	FloatButtonPlacementBottom FloatButtonPlacement = "bottom"
	FloatButtonPlacementLeft   FloatButtonPlacement = "left"
	FloatButtonPlacementRight  FloatButtonPlacement = "right"
)

// Default trigger glyphs (kit mapping; override via SetIcon/SetCloseIcon).
const (
	DefaultGroupTriggerIcon = "plus"
	DefaultGroupCloseIcon   = "close"
)

// FloatButtonGroup hosts FloatButtons (docs/antd/float-button.md §6.10).
// Without trigger all children stay visible; with trigger the group is a
// menu driven by open state (controlled via SetOpen or uncontrolled).
type FloatButtonGroup struct {
	children   []*FloatButton
	trigger    FloatButtonTrigger
	placement  FloatButtonPlacement
	open       bool
	controlled bool
	hovered    bool
	disabled   bool

	icon      string
	closeIcon string
	typ       ButtonType
	shape     FloatButtonShape
	shapeSet  bool
	typeSet   bool

	onOpenChange func(bool)
	onClick      func()

	triggerBtn *FloatButton
	node       *rendering.AbsoluteBox
}

// NewFloatButtonGroup creates a group with optional children.
func NewFloatButtonGroup(children ...*FloatButton) *FloatButtonGroup {
	g := &FloatButtonGroup{
		trigger:   FloatButtonTriggerNone,
		placement: FloatButtonPlacementTop,
		typ:       ButtonTypeDefault,
		shape:     FloatButtonShapeCircle,
	}
	g.triggerBtn = NewFloatButton()
	g.triggerBtn.SetIcon(DefaultGroupTriggerIcon)
	g.node = rendering.NewAbsoluteBox(0, 0)
	for _, c := range children {
		g.Add(c)
	}
	g.rebuild()
	return g
}

// Add appends a child.
func (g *FloatButtonGroup) Add(c *FloatButton) {
	if g == nil || c == nil {
		return
	}
	g.children = append(g.children, c)
	g.rebuild()
}

// SetChildren replaces all children.
func (g *FloatButtonGroup) SetChildren(cs ...*FloatButton) {
	if g == nil {
		return
	}
	g.children = append([]*FloatButton(nil), cs...)
	g.rebuild()
}

// Children returns all children (visible or not).
func (g *FloatButtonGroup) Children() []*FloatButton {
	if g == nil {
		return nil
	}
	return append([]*FloatButton(nil), g.children...)
}

// VisibleChildren returns the painted children (menu closed hides them).
func (g *FloatButtonGroup) VisibleChildren() []*FloatButton {
	if g == nil {
		return nil
	}
	if g.trigger == FloatButtonTriggerNone || g.open {
		return g.Children()
	}
	return nil
}

// SetTrigger selects none/click/hover (other values fall back to none).
func (g *FloatButtonGroup) SetTrigger(t FloatButtonTrigger) {
	if g == nil {
		return
	}
	switch t {
	case FloatButtonTriggerClick, FloatButtonTriggerHover:
		g.trigger = t
	default:
		g.trigger = FloatButtonTriggerNone
	}
	g.rebuild()
}

// Trigger returns the effective trigger.
func (g *FloatButtonGroup) Trigger() FloatButtonTrigger {
	if g == nil || (g.trigger != FloatButtonTriggerClick && g.trigger != FloatButtonTriggerHover) {
		return FloatButtonTriggerNone
	}
	return g.trigger
}

// SetPlacement selects top/bottom/left/right (others fall back to top).
func (g *FloatButtonGroup) SetPlacement(p FloatButtonPlacement) {
	if g == nil {
		return
	}
	switch p {
	case FloatButtonPlacementBottom, FloatButtonPlacementLeft, FloatButtonPlacementRight:
		g.placement = p
	default:
		g.placement = FloatButtonPlacementTop
	}
	g.rebuild()
}

// Placement returns the effective placement.
func (g *FloatButtonGroup) Placement() FloatButtonPlacement {
	if g == nil {
		return FloatButtonPlacementTop
	}
	return g.placement
}

// SetOpen drives controlled open state (marks controlled).
func (g *FloatButtonGroup) SetOpen(v bool) {
	if g == nil {
		return
	}
	g.controlled = true
	if g.open == v {
		return
	}
	g.open = v
	g.syncTriggerGlyph()
	g.rebuild()
	if g.onOpenChange != nil {
		g.onOpenChange(v)
	}
}

// SetDefaultOpen sets the initial open state (ignored once controlled).
func (g *FloatButtonGroup) SetDefaultOpen(v bool) {
	if g == nil || g.controlled {
		return
	}
	if g.open == v {
		return
	}
	g.open = v
	g.syncTriggerGlyph()
	g.rebuild()
}

// Open reports the menu state (always true without trigger).
func (g *FloatButtonGroup) Open() bool {
	if g == nil {
		return false
	}
	if g.trigger == FloatButtonTriggerNone {
		return true
	}
	return g.open
}

// IsControlled reports whether SetOpen has taken over.
func (g *FloatButtonGroup) IsControlled() bool { return g != nil && g.controlled }

// Toggle flips uncontrolled open state (controlled: only fires the callback).
func (g *FloatButtonGroup) Toggle() {
	if g == nil || g.trigger == FloatButtonTriggerNone {
		return
	}
	if g.controlled {
		if g.onOpenChange != nil {
			g.onOpenChange(!g.open)
		}
		return
	}
	g.SetDefaultOpen(!g.open)
	if g.onOpenChange != nil {
		// SetDefaultOpen skips the callback; fire once here.
		g.onOpenChange(g.open)
	}
}

// SetOnOpenChange sets the expand/collapse callback.
func (g *FloatButtonGroup) SetOnOpenChange(fn func(bool)) {
	if g == nil {
		return
	}
	g.onOpenChange = fn
}

// SetOnClick sets the trigger click callback (menu mode).
func (g *FloatButtonGroup) SetOnClick(fn func()) {
	if g == nil {
		return
	}
	g.onClick = fn
}

// ClickTrigger clicks the menu trigger (false when none/disabled).
func (g *FloatButtonGroup) ClickTrigger() bool {
	if g == nil || g.trigger == FloatButtonTriggerNone || g.disabled {
		return false
	}
	if g.triggerBtn != nil && (g.triggerBtn.Disabled() || g.triggerBtn.Loading()) {
		return false
	}
	g.Toggle()
	if g.onClick != nil {
		g.onClick()
	}
	return true
}

// HandleOutsideClick closes an open click-menu (overlay outside-close,
// FloatButtonGroup.tsx document capture mapping). Controlled groups only
// fire the callback; the caller applies SetOpen(false).
func (g *FloatButtonGroup) HandleOutsideClick() bool {
	if g == nil || g.trigger != FloatButtonTriggerClick || !g.open {
		return false
	}
	if g.controlled {
		if g.onOpenChange != nil {
			g.onOpenChange(false)
		}
		return true
	}
	g.open = false
	g.syncTriggerGlyph()
	g.rebuild()
	if g.onOpenChange != nil {
		g.onOpenChange(false)
	}
	return true
}

// SetHover drives hover-trigger menus (uncontrolled flips state).
func (g *FloatButtonGroup) SetHover(v bool) {
	if g == nil {
		return
	}
	g.hovered = v
	if g.trigger != FloatButtonTriggerHover {
		return
	}
	if g.controlled {
		if g.onOpenChange != nil {
			g.onOpenChange(v)
		}
		return
	}
	if g.open == v {
		return
	}
	g.open = v
	g.syncTriggerGlyph()
	g.rebuild()
	if g.onOpenChange != nil {
		g.onOpenChange(v)
	}
}

// SetType applies the skin to the trigger and all children.
func (g *FloatButtonGroup) SetType(t ButtonType) {
	if g == nil {
		return
	}
	if t != ButtonTypePrimary {
		t = ButtonTypeDefault
	}
	g.typ = t
	g.typeSet = true
	if g.triggerBtn != nil {
		g.triggerBtn.SetType(t)
	}
	for _, c := range g.children {
		if c != nil {
			c.SetType(t)
		}
	}
	g.rebuild()
}

// SetShape applies the outline to the trigger and all children.
func (g *FloatButtonGroup) SetShape(s FloatButtonShape) {
	if g == nil {
		return
	}
	if s != FloatButtonShapeSquare {
		s = FloatButtonShapeCircle
	}
	g.shape = s
	g.shapeSet = true
	if g.triggerBtn != nil {
		g.triggerBtn.SetShape(s)
	}
	for _, c := range g.children {
		if c != nil {
			c.SetShape(s)
		}
	}
	g.rebuild()
}

// SetIcon sets the closed trigger glyph.
func (g *FloatButtonGroup) SetIcon(name string) {
	if g == nil {
		return
	}
	g.icon = name
	g.syncTriggerGlyph()
}

// SetCloseIcon sets the open trigger glyph.
func (g *FloatButtonGroup) SetCloseIcon(name string) {
	if g == nil {
		return
	}
	g.closeIcon = name
	g.syncTriggerGlyph()
}

// EffectiveTriggerIcon resolves the painted trigger glyph.
func (g *FloatButtonGroup) EffectiveTriggerIcon() string {
	if g == nil {
		return DefaultGroupTriggerIcon
	}
	if g.open {
		if g.closeIcon != "" {
			return g.closeIcon
		}
		return DefaultGroupCloseIcon
	}
	if g.icon != "" {
		return g.icon
	}
	return DefaultGroupTriggerIcon
}

// SetDisabled disables the trigger (children keep their own flags).
func (g *FloatButtonGroup) SetDisabled(v bool) {
	if g == nil || g.disabled == v {
		return
	}
	g.disabled = v
	if g.triggerBtn != nil {
		g.triggerBtn.SetDisabled(v)
	}
}

// Disabled reports the group trigger flag.
func (g *FloatButtonGroup) Disabled() bool { return g != nil && g.disabled }

// ContainerRole is the menu container role (menuitem lives on children).
func (g *FloatButtonGroup) ContainerRole() string { return "menu" }

// FocusTrigger takes trigger focus; false when none/disabled.
func (g *FloatButtonGroup) FocusTrigger() bool {
	if g == nil || g.trigger == FloatButtonTriggerNone || g.triggerBtn == nil {
		return false
	}
	return g.triggerBtn.Focus()
}

// BlurTrigger releases trigger focus.
func (g *FloatButtonGroup) BlurTrigger() {
	if g == nil || g.triggerBtn == nil {
		return
	}
	g.triggerBtn.Blur()
}

// TriggerFocused reports trigger focus.
func (g *FloatButtonGroup) TriggerFocused() bool {
	return g != nil && g.triggerBtn != nil && g.triggerBtn.Focused()
}

// TriggerFocusable reports trigger Tab reachability.
func (g *FloatButtonGroup) TriggerFocusable() bool {
	return g != nil && g.trigger != FloatButtonTriggerNone && g.triggerBtn != nil && g.triggerBtn.Focusable()
}

// KeyActivateTrigger handles Enter/Space/Escape on the open menu.
func (g *FloatButtonGroup) KeyActivateTrigger(key string) bool {
	if g == nil || g.trigger == FloatButtonTriggerNone {
		return false
	}
	if key == "Escape" && g.open {
		g.HandleOutsideClick()
		if g.controlled {
			return true
		}
		return true
	}
	if g.triggerBtn == nil {
		return false
	}
	return g.triggerBtn.KeyActivate(key)
}

// SetProvider forwards the theme source to trigger and children.
func (g *FloatButtonGroup) SetProvider(p *theme.Provider) {
	if g == nil {
		return
	}
	if g.triggerBtn != nil {
		g.triggerBtn.SetProvider(p)
	}
	for _, c := range g.children {
		if c != nil {
			c.SetProvider(p)
		}
	}
}

// Node returns the tree node (layout/paint/hit through it).
func (g *FloatButtonGroup) Node() rendering.RenderObject {
	if g == nil {
		return nil
	}
	g.syncMembers()
	g.layoutGroup()
	return g.node
}

// Layout sizes the group and positions members per placement.
func (g *FloatButtonGroup) Layout(c rendering.Constraints) rendering.Size {
	if g == nil {
		return rendering.Size{}
	}
	g.syncMembers()
	g.layoutGroup()
	return g.node.Layout(c)
}

// ContainsPoint reports whether a node-local point is inside the group box.
func (g *FloatButtonGroup) ContainsPoint(x, y float64) bool {
	if g == nil || g.node == nil {
		return false
	}
	sz := g.node.Size()
	return x >= 0 && y >= 0 && x < sz.Width && y < sz.Height
}

// EffectiveGap is the member spacing (padding token, §6.2.1).
func (g *FloatButtonGroup) EffectiveGap() float64 {
	tok := theme.Default.Current()
	if tok.Padding > 0 {
		return tok.Padding
	}
	return FloatButtonGroupGap
}

func (g *FloatButtonGroup) syncTriggerGlyph() {
	if g == nil || g.triggerBtn == nil {
		return
	}
	g.triggerBtn.SetIcon(g.EffectiveTriggerIcon())
}

// desiredNodes lists trigger (menu mode) plus visible children in paint order.
func (g *FloatButtonGroup) desiredNodes() []rendering.RenderObject {
	var out []rendering.RenderObject
	if g.trigger != FloatButtonTriggerNone && g.triggerBtn != nil {
		out = append(out, g.triggerBtn.Node())
	}
	for _, c := range g.VisibleChildren() {
		if c != nil {
			out = append(out, c.Node())
		}
	}
	return out
}

func sameNodes(a, b []rendering.RenderObject) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (g *FloatButtonGroup) syncMembers() {
	if g == nil || g.node == nil {
		return
	}
	want := g.desiredNodes()
	if sameNodes(g.node.Children(), want) {
		return
	}
	for _, c := range g.node.Children() {
		g.node.RemoveChild(c)
	}
	for _, n := range want {
		g.node.AddChild(n)
	}
}

// layoutGroup positions members and sizes the box (offsets are paint-local,
// positive by construction so hit == layout == paint).
func (g *FloatButtonGroup) layoutGroup() {
	if g == nil || g.node == nil {
		return
	}
	size := FloatButtonSize
	gap := g.EffectiveGap()
	step := size + gap
	kids := g.VisibleChildren()
	for _, c := range kids {
		if c != nil {
			c.syncSize()
		}
	}
	if g.triggerBtn != nil {
		g.triggerBtn.syncSize()
		if s := g.triggerBtn.EffectiveSize(); s > 0 {
			size = s
			step = size + gap
		}
	}
	if g.trigger == FloatButtonTriggerNone {
		n := float64(len(kids))
		h := 0.0
		if n > 0 {
			h = n*size + (n-1)*gap
		}
		g.node.FixedWidth, g.node.FixedHeight = size, h
		for i, c := range kids {
			if c != nil {
				c.Node().SetOffset(rendering.Point{X: 0, Y: float64(i) * step})
			}
		}
		return
	}
	if !g.open {
		g.node.FixedWidth, g.node.FixedHeight = size, size
		if g.triggerBtn != nil {
			g.triggerBtn.Node().SetOffset(rendering.Point{})
		}
		return
	}
	n := float64(len(kids))
	switch g.placement {
	case FloatButtonPlacementBottom:
		g.node.FixedWidth, g.node.FixedHeight = size, (n+1)*size+n*gap
		if g.triggerBtn != nil {
			g.triggerBtn.Node().SetOffset(rendering.Point{})
		}
		for i, c := range kids {
			if c != nil {
				c.Node().SetOffset(rendering.Point{X: 0, Y: float64(i+1) * step})
			}
		}
	case FloatButtonPlacementLeft:
		g.node.FixedWidth, g.node.FixedHeight = (n+1)*size+n*gap, size
		if g.triggerBtn != nil {
			g.triggerBtn.Node().SetOffset(rendering.Point{X: n * step})
		}
		for i, c := range kids {
			if c != nil {
				c.Node().SetOffset(rendering.Point{X: float64(i) * step})
			}
		}
	case FloatButtonPlacementRight:
		g.node.FixedWidth, g.node.FixedHeight = (n+1)*size+n*gap, size
		if g.triggerBtn != nil {
			g.triggerBtn.Node().SetOffset(rendering.Point{})
		}
		for i, c := range kids {
			if c != nil {
				c.Node().SetOffset(rendering.Point{X: float64(i+1) * step})
			}
		}
	default:
		g.node.FixedWidth, g.node.FixedHeight = size, (n+1)*size+n*gap
		if g.triggerBtn != nil {
			g.triggerBtn.Node().SetOffset(rendering.Point{X: 0, Y: n * step})
		}
		for i, c := range kids {
			if c != nil {
				c.Node().SetOffset(rendering.Point{X: 0, Y: float64(i) * step})
			}
		}
	}
}

func (g *FloatButtonGroup) rebuild() {
	if g == nil {
		return
	}
	g.syncTriggerGlyph()
	g.syncMembers()
	g.layoutGroup()
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}
