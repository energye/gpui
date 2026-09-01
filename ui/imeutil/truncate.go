package imeutil

import "unicode/utf8"

// TruncateSurrounding implements 4000 bytes including NUL, centered at cursor.
// Single source for textinput, Wayland and X11 to avoid B10 divergence.
func TruncateSurrounding(text string, cursorByte int) (string, int) {
	const max = 4000
	if len(text)+1 <= max {
		return text, cursorByte
	}
	budget := max - 1
	half := budget / 2
	start := cursorByte - half
	if start < 0 {
		start = 0
	}
	end := start + budget
	if end > len(text) {
		end = len(text)
		start = end - budget
		if start < 0 {
			start = 0
		}
	}
	for start > 0 && start < len(text) && !utf8.RuneStart(text[start]) {
		start--
	}
	for end > start && end < len(text) && !utf8.RuneStart(text[end]) {
		end--
	}
	if end-start > budget {
		end = start + budget
		for end > start && end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
	}
	newCursor := cursorByte - start
	for newCursor > 0 && newCursor < len(text[start:end]) && !utf8.RuneStart(text[start+newCursor]) {
		newCursor--
	}
	if len(text[start:end])+1 > max {
		end = start + budget
		for end > start && end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
	}
	return text[start:end], newCursor
}

// TruncateSurroundingWithAnchor is the X11 3-arg variant (cursor+anchor).
func TruncateSurroundingWithAnchor(text string, cursor, anchor int) (string, int, int) {
	tr, nc := TruncateSurrounding(text, cursor)
	if anchor < 0 {
		anchor = nc
	}
	// Re-derive anchor relative to truncated window
	// Find start byte of truncated window in original text
	startByte := cursor - nc
	if startByte < 0 {
		startByte = 0
	}
	if startByte > len(text) {
		startByte = len(text)
	}
	// Adjust anchor: original anchor byte - startByte, clamped and rune-snapped
	na := anchor - startByte
	if na < 0 {
		na = nc
	}
	if na > len(tr) {
		na = len(tr)
	}
	for na > 0 && na < len(tr) && !utf8.RuneStart(tr[na]) {
		na--
	}
	return tr, nc, na
}
