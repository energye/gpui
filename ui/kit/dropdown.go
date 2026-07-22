package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Dropdown is a trigger + menu popup (refined from Select for arbitrary menu).
type Dropdown struct {
	Wrap     *primitive.Flex
	Trigger  *Button
	popup    *primitive.AnchoredPopup
	menu     *Menu
	Open     bool
	Selected string
	Viewport core.Size
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(key string)
}

// NewDropdown creates a dropdown with a labeled trigger.
func NewDropdown(label string, items ...MenuItem) *Dropdown {
	d := &Dropdown{}
	d.Trigger = NewButton(label)
	d.Trigger.SetFace(d.Face)
	d.menu = NewMenu(items...)
	d.menu.Face = d.Face
	d.menu.OnSelect = func(key string) {
		d.Selected = key
		d.SetOpen(false)
		if d.OnSelect != nil {
			d.OnSelect(key)
		}
	}
	d.rebuild()
	return d
}

// Node returns the root.
func (d *Dropdown) Node() core.Node {
	if d.Wrap == nil {
		d.rebuild()
	}
	return d.Wrap
}

// SetOpen toggles the menu.
func (d *Dropdown) SetOpen(open bool) {
	d.Open = open
	if d.popup != nil {
		d.popup.UpdateAnchorFromNode(d.Trigger.Root)
		if d.Viewport.Width > 0 {
			d.popup.Viewport = d.Viewport
		}
		d.popup.SetOpen(open)
	}
}

// SetSelected highlights a menu key.
func (d *Dropdown) SetSelected(key string) {
	d.Selected = key
	if d.menu != nil {
		d.menu.SetSelected(key)
	}
}

// Sync repositions while open.
func (d *Dropdown) Sync() {
	if d.Open {
		d.SetOpen(true)
	}
}

func (d *Dropdown) rebuild() {
	d.popup = primitive.NewAnchoredPopup(d.menu.Node())
	d.popup.Placement = primitive.PlaceBottomStart
	d.popup.Gap = 4
	d.popup.Portal.ID = "dropdown"
	d.Trigger.SetOnClick(func() { d.SetOpen(!d.Open) })
	d.Wrap = primitive.Column(d.Trigger.Node(), d.popup)
	d.Wrap.CrossAlign = core.CrossStart
}
