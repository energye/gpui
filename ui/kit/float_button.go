package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design FloatButton defaults.
// https://ant.design/components/float-button
const (
	DefaultFloatButtonSize         = 40.0
	DefaultFloatButtonSquareRadius = 8.0 // ≈ borderRadiusLG
)

// FloatButtonShape is Ant shape prop.
type FloatButtonShape int

const (
	FloatButtonCircle FloatButtonShape = iota
	FloatButtonSquare
)

// FloatButton is Ant Design FloatButton (FAB). Positioning is layout-only
// (Stack / absolute offsets in the app) — not OS always-on-top.
//
// Composes kit.Button. Defaults match Ant Design v5 (primary, circle, 40×40).
//
// https://ant.design/components/float-button
type FloatButton struct {
	btn *Button

	// Shape circle (default) or square.
	Shape FloatButtonShape
	// Size edge length in px (0 → DefaultFloatButtonSize).
	Size float64
	// Type visual type; NewFloatButton uses primary.
	Type ButtonType
	// IconName optional icon.
	IconName string
	// Description optional caption (Ant description); used as label when set.
	Description string
	// Label text/glyph when not using description (e.g. "+").
	Label string

	Face    text.Face
	Theme   *core.Theme
	OnClick func()
}

// NewFloatButton creates a primary circular FAB.
func NewFloatButton(label string) *FloatButton {
	f := &FloatButton{
		Label: label,
		Shape: FloatButtonCircle,
		Type:  ButtonPrimary,
	}
	f.rebuild()
	return f
}

// Node returns the root node.
func (f *FloatButton) Node() core.Node {
	if f == nil {
		return nil
	}
	if f.btn == nil {
		f.rebuild()
	}
	return f.btn.Node()
}

// Button exposes the embedded kit.Button (tests / composition).
func (f *FloatButton) Button() *Button {
	if f == nil {
		return nil
	}
	if f.btn == nil {
		f.rebuild()
	}
	return f.btn
}

// SetShape sets circle or square.
func (f *FloatButton) SetShape(shape FloatButtonShape) {
	if f == nil {
		return
	}
	f.Shape = shape
	f.applyMetrics()
}

// SetSize sets edge length (0 → DefaultFloatButtonSize).
func (f *FloatButton) SetSize(px float64) {
	if f == nil {
		return
	}
	f.Size = px
	f.applyMetrics()
}

// SetType sets primary / default / … chrome.
func (f *FloatButton) SetType(t ButtonType) {
	if f == nil {
		return
	}
	f.Type = t
	if f.btn != nil {
		f.btn.SetType(t)
	}
}

// SetIcon sets icon name (empty clears).
func (f *FloatButton) SetIcon(name string) {
	if f == nil {
		return
	}
	f.IconName = name
	f.applyContent()
	f.applyMetrics() // Button.SetIcon rebuilds with default pad — re-apply FAB chrome
}

// SetDescription sets optional caption (Ant description). When non-empty it
// becomes the visible label; pair with SetIcon for icon+text FABs.
func (f *FloatButton) SetDescription(s string) {
	if f == nil {
		return
	}
	f.Description = s
	f.applyContent()
	f.applyMetrics()
}

// SetLabel sets text content when Description is empty.
func (f *FloatButton) SetLabel(s string) {
	if f == nil {
		return
	}
	f.Label = s
	f.applyContent()
	f.applyMetrics()
}

// SetOnClick sets the click handler.
func (f *FloatButton) SetOnClick(fn func()) {
	if f == nil {
		return
	}
	f.OnClick = fn
	if f.btn != nil {
		f.btn.SetOnClick(fn)
	}
}

// SetFace sets the font face.
func (f *FloatButton) SetFace(face text.Face) {
	if f == nil {
		return
	}
	f.Face = face
	if f.btn != nil {
		f.btn.SetFace(face)
	}
}

// SetTheme sets theme override.
func (f *FloatButton) SetTheme(th *core.Theme) {
	if f == nil {
		return
	}
	f.Theme = th
	if f.btn != nil {
		f.btn.Theme = th
		f.btn.applyChrome()
	}
}

// SetDanger toggles danger styling.
func (f *FloatButton) SetDanger(v bool) {
	if f != nil && f.btn != nil {
		f.btn.SetDanger(v)
	}
}

// SyncState refreshes hover/press chrome.
func (f *FloatButton) SyncState() {
	if f != nil && f.btn != nil {
		f.btn.SyncState()
	}
}

// AttachTicker forwards loading tickers.
func (f *FloatButton) AttachTicker(t *core.Tree) {
	if f != nil && f.btn != nil {
		f.btn.AttachTicker(t)
	}
}

func (f *FloatButton) sizePx() float64 {
	if f != nil && f.Size > 0 {
		return f.Size
	}
	return DefaultFloatButtonSize
}

func (f *FloatButton) radius() float64 {
	sz := f.sizePx()
	if f != nil && f.Shape == FloatButtonSquare {
		return DefaultFloatButtonSquareRadius
	}
	return sz / 2
}

func (f *FloatButton) applyMetrics() {
	if f == nil || f.btn == nil {
		return
	}
	sz := f.sizePx()
	h := sz
	if f.Description != "" && h < sz+12 {
		h = sz + 12
	}
	// Drive Button metrics via Style so applyChrome does not overwrite FAB radius.
	f.btn.Style.Radius = f.radius()
	f.btn.Style.ForceRadius = true
	f.btn.Style.Width = sz
	f.btn.Style.Height = h
	if f.Description != "" {
		f.btn.Style.FontSize = 12
	}
	f.btn.SetFixedSize(sz, h)
	f.btn.applyChrome()
	// Description body replaces row — recolor after applyChrome.
	if f.Description != "" && f.btn.decorated != nil && f.btn.label != nil {
		fg := f.btn.label.Color
		for _, c := range f.btn.decorated.Children() {
			if col, ok := c.(*primitive.Flex); ok {
				for _, cc := range col.Children() {
					if ic, ok := cc.(*primitive.Icon); ok {
						ic.Color = fg
					}
					if tx, ok := cc.(*primitive.Text); ok {
						tx.Color = fg
					}
				}
			}
		}
	}
	if f.btn.decorated != nil {
		f.btn.decorated.Radius = f.radius()
		// FAB: no inline padding — content is centered in the fixed square/circle.
		// Regular Button padH=15 would leave ~10px content width and look left-aligned.
		f.btn.decorated.Padding = primitive.EdgeInsets{}
		f.btn.decorated.StretchChild = true // row gets full size; MainAlign=Center centers icon/label
		f.btn.decorated.SetCenterContent(true)
		f.btn.decorated.MarkNeedsLayout()
		f.btn.decorated.MarkNeedsPaint()
	}
	if f.btn.row != nil {
		f.btn.row.MainAlign = core.MainCenter
		f.btn.row.CrossAlign = core.CrossCenter
		// Icon-only: no gap between missing siblings.
		if f.IconName != "" && f.Label == "" && f.Description == "" {
			f.btn.row.Gap = 0
		}
	}
	if f.btn.Root != nil {
		f.btn.Root.FocusRingRadius = f.radius()
	}
	// Re-install after any Button.rebuild that replaced ThemeHook.
	f.installThemeHook()
}

func (f *FloatButton) applyContent() {
	if f == nil || f.btn == nil {
		return
	}
	// SetLabel/SetIcon rebuild Button chrome (default padH=15). applyMetrics must run after.
	if f.Description != "" {
		// Ant description mode: vertical icon + caption, not a horizontal Button row.
		f.btn.SetLabel("")
		f.btn.SetIcon("")
		f.ensureDescBody()
	} else {
		f.btn.SetLabel(f.Label)
		f.btn.SetIcon(f.IconName)
	}
	if f.btn.Root != nil {
		a11y := f.Label
		if a11y == "" {
			a11y = f.Description
		}
		if a11y == "" {
			a11y = f.IconName
		}
		f.btn.Root.Base().Label = a11y
		f.btn.Root.Base().Role = "button"
	}
}

// ensureDescBody builds a centered column (icon + description) inside Button chrome.
func (f *FloatButton) ensureDescBody() {
	if f == nil || f.btn == nil || f.btn.decorated == nil {
		return
	}
	col := primitive.Column()
	col.Gap = 2
	col.MainAlign = core.MainCenter
	col.CrossAlign = core.CrossCenter
	if f.IconName != "" {
		ic := primitive.NewIcon(f.IconName)
		ic.Size = 16
		col.AddChild(ic)
	}
	tx := primitive.NewText(f.Description)
	tx.FontSize = 12
	tx.Face = f.Face
	col.AddChild(tx)
	// Keep btn.label for applyChrome color writes.
	if f.btn.label == nil {
		f.btn.label = primitive.NewText("")
	}
	f.btn.row = nil // avoid stale row
	f.btn.decorated.ClearChildren()
	f.btn.decorated.AddChild(col)
	// Sync caption color after applyChrome in applyMetrics.
	f.btn.label = tx
}

func (f *FloatButton) rebuild() {
	if f == nil {
		return
	}
	label := f.Label
	if f.Description != "" {
		label = f.Description
	}
	if f.btn == nil {
		f.btn = NewButton(label)
	}
	f.btn.Theme = f.Theme
	f.btn.SetType(f.Type)
	if f.Face != nil {
		f.btn.SetFace(f.Face)
	}
	if f.OnClick != nil {
		f.btn.SetOnClick(f.OnClick)
	}
	f.applyContent()
	f.applyMetrics()
	f.installThemeHook()
}

// installThemeHook keeps FAB padding/radius after Button.rebuild (theme / SetIcon).
func (f *FloatButton) installThemeHook() {
	if f == nil || f.btn == nil || f.btn.Root == nil {
		return
	}
	f.btn.Root.SetThemeHook(func(*core.Theme) {
		// Button may have rebuilt with default inline padding — re-assert FAB chrome.
		f.applyMetrics()
	})
}
