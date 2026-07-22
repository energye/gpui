package core

// CursorKind mirrors platform cursor shapes without importing platform.
// Values must match platform.CursorKind.
type CursorKind int

const (
	CursorDefault CursorKind = iota
	CursorPointer
	CursorText
	CursorNotAllowed
	CursorWait
	CursorMove
	CursorCrosshair
	// CursorInherit means use parent/default resolution.
	CursorInherit CursorKind = -1
)
