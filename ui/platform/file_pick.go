package platform

// FilePickResult is one chosen path from a host file dialog.
type FilePickResult struct {
	Path string
	Name string // basename; used for Upload label
}

// FilePicker is optional host capability (CapFile).
// Linux thin host may leave this nil; tests inject a fake picker.
type FilePicker interface {
	// PickOpen shows an open dialog; ok=false if cancelled.
	PickOpen(title string, filters []string) (res FilePickResult, ok bool)
}

// CapFile is the capability bit for host file dialogs (FilePicker).
// High bit keeps older Caps values stable if ever serialized.
const CapFile Caps = 1 << 20
