package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TimePicker is a simple HH:MM select via Segmented-like options.
// https://ant.design/components/time-picker
type TimePicker struct {
	Root     *primitive.Flex
	Value    string
	Face     text.Face
	OnChange func(v string)
}

// NewTimePicker creates hour options 00-23 step 1 simplified to common times.
func NewTimePicker() *TimePicker {
	tp := &TimePicker{Value: "09:00"}
	tp.rebuild()
	return tp
}

// Node returns root.
func (tp *TimePicker) Node() core.Node {
	if tp.Root == nil {
		tp.rebuild()
	}
	return tp.Root
}

// SetFace sets font.
func (tp *TimePicker) SetFace(face text.Face) {
	tp.Face = face
	tp.rebuild()
}

// SetValue selects a time string and rebuilds.
func (tp *TimePicker) SetValue(s string) {
	tp.Value = s
	tp.rebuild()
	if tp.OnChange != nil {
		tp.OnChange(s)
	}
}

func (tp *TimePicker) rebuild() {
	opts := []string{"09:00", "10:00", "12:00", "14:00", "16:00", "18:00"}
	seg := NewSegmented(opts...)
	seg.SetFace(tp.Face)
	seg.SetValue(tp.Value)
	seg.OnChange = func(v string) {
		tp.Value = v
		if tp.OnChange != nil {
			tp.OnChange(v)
		}
	}
	tp.Root = primitive.Column(seg.Node())
}
