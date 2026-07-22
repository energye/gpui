package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// ColorPicker is a simplified swatch row.
// https://ant.design/components/color-picker
type ColorPicker struct {
	Root     *primitive.Flex
	Value    render.RGBA
	Swatches []render.RGBA
	OnChange func(c render.RGBA)
}

// NewColorPicker creates a swatch picker.
func NewColorPicker(swatches ...render.RGBA) *ColorPicker {
	if len(swatches) == 0 {
		swatches = []render.RGBA{
			render.Hex("#1677FF"), render.Hex("#52C41A"), render.Hex("#FAAD14"),
			render.Hex("#FF4D4F"), render.Hex("#722ED1"), render.Hex("#13C2C2"),
		}
	}
	cp := &ColorPicker{Swatches: swatches, Value: swatches[0]}
	cp.rebuild()
	return cp
}

// Node returns root.
func (cp *ColorPicker) Node() core.Node {
	if cp.Root == nil {
		cp.rebuild()
	}
	return cp.Root
}

// SetValue selects a color and notifies OnChange.
func (cp *ColorPicker) SetValue(c render.RGBA) {
	cp.Value = c
	if cp.OnChange != nil {
		cp.OnChange(c)
	}
}

func (cp *ColorPicker) rebuild() {
	cp.Root = primitive.Row()
	cp.Root.Gap = 8
	for _, c := range cp.Swatches {
		c := c
		box := primitive.NewBox()
		box.Width, box.Height = 24, 24
		box.Color = c
		p := primitive.NewPressable(box)
		p.ShowFocusRing = true
		p.FocusRingRadius = 4
		p.Click = func() {
			cp.Value = c
			if cp.OnChange != nil {
				cp.OnChange(c)
			}
		}
		cp.Root.AddChild(p)
	}
}
