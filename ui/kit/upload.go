package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
)

// Upload is Ant Design Upload trigger with optional CapFile picker.
// https://ant.design/components/upload
//
// Rounds:
//  1. button + FileName label
//  2. host FilePicker (platform.FilePicker)
//  3. multi-file / drag-drop (later)
type Upload struct {
	btn      *Button
	FileName string
	// Accept is file extension/MIME filters (state only for R3).
	Accept []string
	// Multiple allows multi-file selection (state only for R3).
	Multiple bool
	// Picker when non-nil is used on click (CapFile). Tests inject fakes.
	Picker interface {
		PickOpen(title string, filters []string) (path, name string, ok bool)
	}
	OnPick func(path, name string)
}

// NewUpload creates an upload trigger button.
func NewUpload(label string) *Upload {
	if label == "" {
		label = "Upload"
	}
	u := &Upload{}
	u.btn = NewButton(label)
	u.btn.SetType(ButtonDefault)
	u.btn.SetOnClick(func() {
		path, name := "", u.FileName
		if u.Picker != nil {
			p, n, ok := u.Picker.PickOpen("Upload", u.Accept)
			if !ok {
				return
			}
			path, name = p, n
		} else if name == "" {
			// Headless / no CapFile: deterministic demo pick (still real state machine).
			path, name = "demo/file.bin", "file.bin"
		}
		u.SetFileName(name)
		if u.OnPick != nil {
			u.OnPick(path, name)
		}
	})
	return u
}

// Node returns button node.
func (u *Upload) Node() core.Node {
	if u == nil || u.btn == nil {
		return nil
	}
	return u.btn.Node()
}

// SetFileName updates the displayed name (host CapFile integration).
func (u *Upload) SetFileName(name string) {
	if u == nil {
		return
	}
	u.FileName = name
	if u.btn != nil && name != "" {
		u.btn.SetLabel(name)
	}
}

// SetFace sets font.
func (u *Upload) SetFace(face text.Face) {
	if u != nil && u.btn != nil {
		u.btn.SetFace(face)
	}
}
