package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// SelectOption is one option in a Select.
type SelectOption struct {
	Value string
	Label string
}

// Select is a dropdown selector (B2 base).
type Select struct {
	Wrap        *primitive.Flex
	Root        *primitive.Pressable
	display     *primitive.Text
	decor       *primitive.Decorated
	popup       *primitive.AnchoredPopup
	list        *primitive.Flex
	Options     []SelectOption
	Value       string
	Placeholder string
	Open        bool
	Disabled    bool
	Face        text.Face
	Theme       *core.Theme
	Viewport    core.Size
	Nav         *core.KeyboardNav
	OnChange    func(value string)
}

// NewSelect creates a select with options.
func NewSelect(placeholder string, options ...SelectOption) *Select {
	s := &Select{Placeholder: placeholder, Options: options}
	s.Nav = core.NewKeyboardNav(core.NavVertical, len(options))
	s.rebuild()
	return s
}

// Node returns the composition root (trigger + popup).
func (s *Select) Node() core.Node {
	if s.Wrap == nil {
		s.rebuild()
	}
	return s.Wrap
}

// SetValue selects an option by value.
func (s *Select) SetValue(v string) {
	s.Value = v
	s.refreshLabel()
	if s.OnChange != nil {
		s.OnChange(v)
	}
}

// SetOpen opens/closes the dropdown.
func (s *Select) SetOpen(open bool) {
	s.Open = open
	if s.popup != nil {
		s.popup.UpdateAnchorFromNode(s.Root)
		if s.Viewport.Width > 0 {
			s.popup.Viewport = s.Viewport
		}
		s.popup.SetOpen(open)
	}
}

// SetFace sets the font face for the value label and rebuilds chrome.
func (s *Select) SetFace(face text.Face) {
	s.Face = face
	if s.display != nil {
		s.display.Face = face
		s.display.MarkNeedsLayout()
		s.display.MarkNeedsPaint()
	} else {
		s.rebuild()
	}
}

// Sync repositions while open.
func (s *Select) Sync() {
	if s.Open && s.popup != nil {
		s.popup.UpdateAnchorFromNode(s.Root)
		if s.Viewport.Width > 0 {
			s.popup.Viewport = s.Viewport
		}
		s.popup.SetOpen(true)
	}
}

func (s *Select) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Select) refreshLabel() {
	if s.display == nil {
		return
	}
	label := s.Placeholder
	if label == "" {
		label = "Please select"
	}
	col := s.theme().Color(core.TokenColorTextSecondary)
	for _, o := range s.Options {
		if o.Value == s.Value {
			label = o.Label
			col = s.theme().Color(core.TokenColorText)
			break
		}
	}
	s.display.SetValue(label)
	s.display.Color = col
}

func (s *Select) rebuild() {
	th := s.theme()
	ph := s.Placeholder
	if ph == "" {
		ph = "Please select"
	}
	s.display = primitive.NewText(ph)
	s.display.FontSize = th.SizeOr(core.TokenFontSize, 14)
	s.display.Face = s.Face
	s.display.Color = th.Color(core.TokenColorTextSecondary)

	chev := primitive.NewIcon("chevron-down")
	chev.Size = 12
	sec := th.Color(core.TokenColorTextSecondary)
	if sec.A < 0.3 {
		sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	chev.Color = sec

	row := primitive.Row(s.display, primitive.Spacer(), chev)
	row.CrossAlign = core.CrossCenter
	row.Gap = 8

	h := th.SizeOr(core.TokenControlHeight, 32)
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	s.decor = primitive.NewDecorated(row)
	s.decor.Padding = primitive.Symmetric(padH, 0)
	s.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	s.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	s.decor.BorderColor = th.Color(core.TokenColorBorder)
	s.decor.Background = th.Color(core.TokenColorBgContainer)
	s.decor.MinHeight = h
	s.decor.Height = h
	s.decor.MinWidth = 160

	s.list = primitive.Column()
	s.list.Gap = 2
	s.list.CrossAlign = core.CrossStart
	s.rebuildOptions()

	panel := primitive.NewDecorated(s.list)
	panel.Padding = primitive.All(4)
	panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	panel.Background = th.Color(core.TokenColorBgContainer)
	panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	panel.BorderColor = th.Color(core.TokenColorBorder)
	panel.MinWidth = 160

	s.popup = primitive.NewAnchoredPopup(panel)
	s.popup.Placement = primitive.PlaceBottomStart
	s.popup.Gap = 4
	s.popup.Portal.ID = "select"

	s.Root = primitive.NewPressable(s.decor)
	s.Root.Focusable = true
	s.Root.FocusRingRadius = s.decor.Radius
	s.Root.SetDisabled(s.Disabled)
	s.Root.OnStateChange = func() {
		if s.Root == nil || s.decor == nil || s.Disabled {
			return
		}
		th := s.theme()
		switch {
		case s.Root.State.Focused || s.Open:
			s.decor.BorderColor = th.Color(core.TokenColorPrimary)
		case s.Root.State.Hovered:
			bd := th.Color(core.TokenColorBorderHover)
			if bd.A < 0.5 {
				bd = th.Color(core.TokenColorPrimaryHover)
			}
			s.decor.BorderColor = bd
		default:
			s.decor.BorderColor = th.Color(core.TokenColorBorder)
		}
		s.decor.MarkNeedsPaint()
	}
	s.Root.Click = func() {
		if s.Disabled {
			return
		}
		s.SetOpen(!s.Open)
	}

	s.Wrap = primitive.Column(s.Root, s.popup)
	s.Wrap.CrossAlign = core.CrossStart
	s.refreshLabel()
}

func (s *Select) rebuildOptions() {
	if s.list == nil {
		return
	}
	s.list.ClearChildren()
	th := s.theme()
	s.Nav.SetCount(len(s.Options))
	for i, opt := range s.Options {
		i, opt := i, opt
		lab := primitive.NewText(opt.Label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = s.Face
		lab.Color = th.Color(core.TokenColorText)
		item := primitive.NewPressable(lab)
		// Ant Select option: paddingBlock 5, paddingInline 12
		item.Padding = primitive.Symmetric(12, 5)
		if opt.Value == s.Value {
			item.Color = antItemSelectedFill(th)
			lab.Color = antItemSelectedText(th)
		}
		item.ColorHovered = antItemHoverFill(th)
		item.Click = func() {
			s.SetValue(opt.Value)
			s.Nav.Index = i
			s.SetOpen(false)
			s.rebuildOptions()
		}
		s.list.AddChild(item)
	}
}

// HandleKey for arrow/enter when focused — call from host after DispatchKey if needed.
func (s *Select) HandleKey(ev *core.KeyEvent) {
	if s.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !s.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" {
			s.SetOpen(true)
			ev.Handled = true
		}
		return
	}
	if s.Nav.HandleKey(ev.Key) {
		ev.Handled = true
		return
	}
	if ev.Key == "Enter" || ev.Key == " " {
		if s.Nav.Index >= 0 && s.Nav.Index < len(s.Options) {
			s.SetValue(s.Options[s.Nav.Index].Value)
			s.SetOpen(false)
			s.rebuildOptions()
		}
		ev.Handled = true
	}
	if ev.Key == "Escape" {
		s.SetOpen(false)
		ev.Handled = true
	}
}

// MenuItem is one menu entry.
type MenuItem struct {
	Key   string
	Label string
}

// Menu is a vertical list of selectable items with keyboard nav (B3 base).
type Menu struct {
	Root     *primitive.Decorated
	list     *primitive.Flex
	Items    []MenuItem
	Selected string
	Face     text.Face
	Theme    *core.Theme
	Nav      *core.KeyboardNav
	OnSelect func(key string)
}

// NewMenu creates a menu.
func NewMenu(items ...MenuItem) *Menu {
	m := &Menu{Items: items}
	m.Nav = core.NewKeyboardNav(core.NavVertical, len(items))
	m.rebuild()
	return m
}

// Node returns the root.
func (m *Menu) Node() core.Node {
	if m.Root == nil {
		m.rebuild()
	}
	return m.Root
}

// SetSelected highlights a key.
func (m *Menu) SetSelected(key string) {
	m.Selected = key
	m.rebuild()
}

func (m *Menu) theme() *core.Theme {
	if m.Theme != nil {
		return m.Theme
	}
	return DefaultTheme()
}

func (m *Menu) rebuild() {
	th := m.theme()
	m.list = primitive.Column()
	m.list.Gap = 2
	m.Nav.SetCount(len(m.Items))
	for i, it := range m.Items {
		i, it := i, it
		lab := primitive.NewText(it.Label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = m.Face
		lab.Color = th.Color(core.TokenColorText)
		row := primitive.NewPressable(lab)
		// Ant Menu item: padding 5×12
		row.Padding = primitive.Symmetric(12, 5)
		if it.Key == m.Selected {
			row.Color = antItemSelectedFill(th)
			lab.Color = antItemSelectedText(th)
		}
		row.ColorHovered = antItemHoverFill(th)
		row.Click = func() {
			m.Selected = it.Key
			m.Nav.Index = i
			if m.OnSelect != nil {
				m.OnSelect(it.Key)
			}
			m.rebuild()
		}
		m.list.AddChild(row)
	}
	m.Root = primitive.NewDecorated(m.list)
	m.Root.Padding = primitive.All(4)
	m.Root.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	m.Root.Background = th.Color(core.TokenColorBgContainer)
	m.Root.BorderWidth = 1
	m.Root.BorderColor = th.Color(core.TokenColorBorder)
	m.Root.MinWidth = 160
}

// TabPosition places the tab list (Ant: top | left).
// MessageHost renders a NotifyQueue as stacked toasts (top-right).
type MessageHost struct {
	Portal   *primitive.OverlayPortal
	Queue    *core.NotifyQueue
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	layer    *messageLayer
}

// NewMessageHost creates a message host with its own queue.
func NewMessageHost() *MessageHost {
	h := &MessageHost{Queue: core.NewNotifyQueue(5)}
	h.rebuild()
	h.Queue.OnChange = func() { h.refresh() }
	return h
}

// Node returns the portal node to mount.
func (h *MessageHost) Node() core.Node {
	if h.Portal == nil {
		h.rebuild()
	}
	return h.Portal
}

// Info pushes an info message.
func (h *MessageHost) Info(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "info", DurationMs: 3000})
}

// Success pushes a success message.
func (h *MessageHost) Success(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "success", DurationMs: 3000})
}

// Error pushes an error message.
func (h *MessageHost) Error(text string) {
	h.Queue.Push(core.NotifyItem{Content: text, Kind: "error", DurationMs: 4000})
}

// Sync keeps portal open while items exist.
func (h *MessageHost) Sync() {
	if h.Portal == nil {
		return
	}
	h.refresh()
	h.Portal.SetOpen(h.Queue.Len() > 0)
}

func (h *MessageHost) theme() *core.Theme {
	if h.Theme != nil {
		return h.Theme
	}
	return DefaultTheme()
}

func (h *MessageHost) rebuild() {
	h.layer = &messageLayer{host: h}
	h.layer.Init(h.layer)
	h.layer.Hit = core.HitDefer
	h.Portal = primitive.NewOverlayPortal(h.layer)
	h.Portal.ID = "messages"
	h.Portal.ZOrder = 600
	h.refresh()
}

func (h *MessageHost) refresh() {
	if h.layer == nil {
		return
	}
	h.layer.ClearChildren()
	th := h.theme()
	col := primitive.Column()
	col.Gap = 8
	col.CrossAlign = core.CrossEnd
	for _, it := range h.Queue.Items() {
		tx := primitive.NewText(it.Content)
		tx.FontSize = th.SizeOr(core.TokenFontSize, 14)
		tx.Face = h.Face
		tx.Color = th.Color(core.TokenColorText)
		card := primitive.NewDecorated(tx)
		card.Padding = primitive.Symmetric(12, 9) // Ant Message
		card.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		card.Background = th.Color(core.TokenColorBgContainer)
		card.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
		switch it.Kind {
		case "success":
			card.BorderColor = th.Color(core.TokenColorSuccess)
		case "error":
			card.BorderColor = th.Color(core.TokenColorError)
		case "warning":
			card.BorderColor = th.Color(core.TokenColorWarning)
		default:
			card.BorderColor = th.Color(core.TokenColorPrimary)
		}
		// close on click
		id := it.ID
		press := primitive.NewPressable(card)
		press.Click = func() { h.Queue.Remove(id) }
		col.AddChild(press)
	}
	h.layer.AddChild(col)
}

type messageLayer struct {
	core.NodeBase
	host *MessageHost
}

func (l *messageLayer) TypeID() string { return "kit.MessageLayer" }
func (l *messageLayer) Layout(c core.Constraints) core.Size {
	vw, vh := c.MaxWidth, c.MaxHeight
	if l.host != nil && l.host.Viewport.Width > 0 {
		vw, vh = l.host.Viewport.Width, l.host.Viewport.Height
	}
	if vw >= core.Unbounded/2 {
		vw = 800
	}
	if vh >= core.Unbounded/2 {
		vh = 600
	}
	// content top-right
	for _, child := range l.Children() {
		sz := child.Layout(core.Loose(320, vh))
		child.Base().SetOffset(core.Point{X: vw - sz.Width - 16, Y: 16})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}
func (l *messageLayer) Paint(pc *core.PaintContext)    { l.DefaultPaintChildren(pc) }
func (l *messageLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

// ensure render used for type ref in other files
var _ = render.RGBA{}
