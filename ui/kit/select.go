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
	AllowClear  bool
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

// Clear resets the selection (when AllowClear or always via API).
func (s *Select) Clear() {
	s.Value = ""
	s.refreshLabel()
	s.rebuildOptions()
	if s.OnChange != nil {
		s.OnChange("")
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

// Popup returns the anchored popup (tests / advanced hosts).
func (s *Select) Popup() *primitive.AnchoredPopup {
	if s == nil {
		return nil
	}
	return s.popup
}

// SetFace sets the font face for the value label and rebuilds chrome.
func (s *Select) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
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
	var n core.Node
	if s.Wrap != nil {
		n = s.Wrap
	}
	return themeOf(s.Theme, n)
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
	if s.display.Color.A < 0.2 {
		s.display.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}

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
	s.decor.MinWidth = 200
	s.decor.Width = 240 // visible control chrome (Ant-like default width)
	s.decor.SetCenterContent(true)
	s.decor.StretchChild = true // row fills chrome; label+chevron lay out inside

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
	s.popup.Portal.ID = "" // auto-id; avoid clobber
	s.popup.DismissOnOutside = true
	s.popup.OnDismiss = func() {
		// Outside dismiss closes AnchoredPopup; keep product Open in sync.
		s.Open = false
		if s.Root != nil && s.Root.OnStateChange != nil {
			s.Root.OnStateChange()
		}
	}

	if s.Root == nil {
		s.Root = primitive.NewPressable(s.decor)
	} else {
		s.Root.ClearChildren()
		s.Root.AddChild(s.decor)
	}
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

	if s.Wrap == nil {
		s.Wrap = primitive.Column(s.Root, s.popup)
	} else {
		s.Wrap.ClearChildren()
		s.Wrap.AddChild(s.Root)
		s.Wrap.AddChild(s.popup)
	}
	s.Wrap.CrossAlign = core.CrossStart
	s.Wrap.MarkNeedsLayout()
	s.Wrap.MarkNeedsPaint()
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
