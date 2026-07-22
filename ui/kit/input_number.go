package kit

import (
	"strconv"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// InputNumber is Ant Design InputNumber (value + step buttons).
// https://ant.design/components/input-number
type InputNumber struct {
	Root           *primitive.Decorated
	input          *Input
	Value          float64
	Min, Max, Step float64
	Disabled       bool
	Face           text.Face
	Theme          *core.Theme
	OnChange       func(v float64)
}

// NewInputNumber creates a number input with steppers.
func NewInputNumber(value float64) *InputNumber {
	n := &InputNumber{Value: value, Step: 1, Min: -1e12, Max: 1e12}
	n.rebuild()
	return n
}

// Node returns root.
func (n *InputNumber) Node() core.Node {
	if n.Root == nil {
		n.rebuild()
	}
	return n.Root
}

// SetValue sets numeric value (clamped to Min/Max).
func (n *InputNumber) SetValue(v float64) {
	if n.Disabled {
		return
	}
	if v < n.Min {
		v = n.Min
	}
	if v > n.Max {
		v = n.Max
	}
	n.Value = v
	if n.input != nil {
		n.input.SetValue(formatNum(v))
	}
	if n.OnChange != nil {
		n.OnChange(v)
	}
}

// SetFace sets font.
func (n *InputNumber) SetFace(face text.Face) {
	n.Face = face
	if n.input != nil {
		n.input.SetFace(face)
	}
}

// SetDisabled toggles disabled state on the input and steppers.
func (n *InputNumber) SetDisabled(d bool) {
	n.Disabled = d
	if n.input != nil {
		n.input.SetDisabled(d)
	}
}

func (n *InputNumber) rebuild() {
	n.input = NewInput(formatNum(n.Value))
	n.input.SetFace(n.Face)
	n.input.SetFixedSize(120, 32)
	n.input.SetDisabled(n.Disabled)
	n.input.SetOnChange(func(s string) {
		if n.Disabled {
			return
		}
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			n.SetValue(v)
		}
	})
	up := NewButton("▲")
	up.SetType(ButtonDefault)
	up.SetFace(n.Face)
	up.SetDisabled(n.Disabled)
	up.SetOnClick(func() { n.SetValue(n.Value + n.Step) })
	dn := NewButton("▼")
	dn.SetType(ButtonDefault)
	dn.SetFace(n.Face)
	dn.SetDisabled(n.Disabled)
	dn.SetOnClick(func() { n.SetValue(n.Value - n.Step) })
	// Compact steppers
	steps := primitive.Column(up.Node(), dn.Node())
	steps.Gap = 0
	row := primitive.Row(n.input.Node(), steps)
	row.Gap = 0
	row.CrossAlign = core.CrossCenter
	n.Root = primitive.NewDecorated(row)
	n.Root.BorderWidth = 0
	n.Root.Hit = core.HitDefer
}
