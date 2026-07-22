package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// FloatButton is Ant Design FloatButton (corner FAB).
// https://ant.design/components/float-button
type FloatButton struct {
	btn *Button
	// Shape: "circle" (default) or "square".
	Shape string
}

// NewFloatButton creates a circular primary FAB.
func NewFloatButton(label string) *FloatButton {
	b := NewButton(label)
	b.SetType(ButtonPrimary)
	b.SetFixedSize(48, 48)
	// circular
	if b.decorated != nil {
		b.decorated.Radius = 24
	}
	return &FloatButton{btn: b, Shape: "circle"}
}

// Node returns button node.
func (f *FloatButton) Node() core.Node {
	if f == nil || f.btn == nil {
		return nil
	}
	return f.btn.Node()
}

// SetShape sets FAB shape ("circle" or "square").
func (f *FloatButton) SetShape(shape string) {
	if f == nil || f.btn == nil {
		return
	}
	f.Shape = shape
	if f.btn.decorated == nil {
		return
	}
	if shape == "square" {
		f.btn.decorated.Radius = 8
	} else {
		f.btn.decorated.Radius = 24
	}
	f.btn.decorated.MarkNeedsPaint()
}

// SetOnClick sets click handler.
func (f *FloatButton) SetOnClick(fn func()) {
	if f != nil && f.btn != nil {
		f.btn.SetOnClick(fn)
	}
}

// SetFace sets font.
func (f *FloatButton) SetFace(face text.Face) {
	if f != nil && f.btn != nil {
		f.btn.SetFace(face)
	}
}

// SyncState refreshes chrome.
func (f *FloatButton) SyncState() {
	if f != nil && f.btn != nil {
		f.btn.SyncState()
	}
}
