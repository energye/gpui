package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Descriptions is Ant Design Descriptions (label-value pairs).
// https://ant.design/components/descriptions
type Descriptions struct {
	Root   *primitive.Flex
	Items  [][2]string // label, value
	Column int         // columns per row (default 1)
	Face   text.Face
	Theme  *core.Theme
}

// NewDescriptions creates descriptions from label/value pairs.
func NewDescriptions(pairs ...[2]string) *Descriptions {
	d := &Descriptions{Items: append([][2]string(nil), pairs...), Column: 1}
	d.rebuild()
	return d
}

// Node returns root.
func (d *Descriptions) Node() core.Node {
	if d.Root == nil {
		d.rebuild()
	}
	return d.Root
}

// SetItems replaces label/value pairs and rebuilds.
func (d *Descriptions) SetItems(items [][2]string) {
	d.Items = append([][2]string(nil), items...)
	d.rebuild()
}

// SetFace sets font.
func (d *Descriptions) SetFace(face text.Face) {
	d.Face = face
	d.rebuild()
}

func (d *Descriptions) rebuild() {
	th := DefaultTheme()
	if d.Theme != nil {
		th = d.Theme
	}
	cols := d.Column
	if cols <= 0 {
		cols = 1
	}
	d.Root = primitive.Column()
	d.Root.Gap = 8
	d.Root.CrossAlign = core.CrossStart
	var row *primitive.Flex
	for i, pair := range d.Items {
		if i%cols == 0 {
			row = primitive.Row()
			row.Gap = 16
			row.CrossAlign = core.CrossStart
			d.Root.AddChild(row)
		}
		lab := primitive.NewText(pair[0] + ":")
		lab.FontSize = 14
		lab.Face = d.Face
		lab.Color = th.Color(core.TokenColorTextSecondary)
		val := primitive.NewText(pair[1])
		val.FontSize = 14
		val.Face = d.Face
		val.Color = th.Color(core.TokenColorText)
		cell := primitive.Row(lab, val)
		cell.Gap = 8
		row.AddChild(cell)
	}
}
