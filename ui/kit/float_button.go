package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// FloatButton is Ant Design FloatButton (corner FAB).
// https://ant.design/components/float-button
type FloatButton struct {
	btn *Button
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
	return &FloatButton{btn: b}
}

// Node returns button node.
func (f *FloatButton) Node() core.Node {
	if f == nil || f.btn == nil {
		return nil
	}
	return f.btn.Node()
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
