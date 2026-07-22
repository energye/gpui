package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Divider is kit wrapper around primitive.Divider (Ant Divider).
// https://ant.design/components/divider
type Divider struct {
	Root *primitive.Divider
	// host is used when Text is set (line + label + line).
	host *primitive.Flex
	// Dashed is stored for Ant API parity (line style deferred).
	Dashed bool
	Text   string
	Face   text.Face
	Theme  *core.Theme
}

// NewDivider creates a horizontal divider.
func NewDivider() *Divider {
	d := primitive.NewDivider()
	d.ColorToken = core.TokenColorBorder
	return &Divider{Root: d}
}

// Node returns root (text layout host when label set).
func (d *Divider) Node() core.Node {
	if d == nil {
		return nil
	}
	if d.Text != "" {
		if d.host == nil {
			d.rebuildText()
		}
		return d.host
	}
	return d.Root
}

// SetVertical makes a vertical divider.
func (d *Divider) SetVertical(v bool) {
	if d != nil && d.Root != nil {
		d.Root.Vertical = v
	}
}

// SetDashed toggles dashed style flag.
func (d *Divider) SetDashed(v bool) {
	if d != nil {
		d.Dashed = v
	}
}

// SetText sets an inline label (horizontal with text).
func (d *Divider) SetText(label string) {
	if d == nil {
		return
	}
	d.Text = label
	if label == "" {
		d.host = nil
		return
	}
	d.rebuildText()
}

func (d *Divider) rebuildText() {
	th := DefaultTheme()
	if d.Theme != nil {
		th = d.Theme
	}
	lab := primitive.NewText(d.Text)
	lab.FontSize = 14
	lab.Face = d.Face
	lab.Color = th.Color(core.TokenColorTextSecondary)
	left := primitive.NewDivider()
	left.ColorToken = core.TokenColorBorder
	right := primitive.NewDivider()
	right.ColorToken = core.TokenColorBorder
	leftHost := primitive.NewFlexible(1, left)
	leftHost.FillChild = true
	rightHost := primitive.NewFlexible(1, right)
	rightHost.FillChild = true
	d.host = primitive.Row(leftHost, lab, rightHost)
	d.host.Gap = 8
	d.host.CrossAlign = core.CrossCenter
}
