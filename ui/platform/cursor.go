package platform

// CursorKind identifies a standard mouse cursor shape.
// Hosts that advertise CapCursor should implement CursorHost.
type CursorKind int

const (
	// CursorDefault is the system arrow / default pointer.
	CursorDefault CursorKind = iota
	// CursorPointer is a hand (clickable controls).
	CursorPointer
	// CursorText is an I-beam (text input).
	CursorText
	// CursorNotAllowed is disabled / no-drop.
	CursorNotAllowed
	// CursorWait is busy / progress.
	CursorWait
	// CursorMove is move / grab.
	CursorMove
	// CursorCrosshair is crosshair.
	CursorCrosshair
)

// CursorHost is optional; set the window cursor shape.
type CursorHost interface {
	SetCursor(kind CursorKind)
}
