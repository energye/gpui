// Gallery key mapping: platform event -> backbone key name.
//
// Tab cycles focus, Enter/Space activate, Escape releases press.
package main

import (
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

func galleryKey(ev platform.Event) string {
	if !ev.Pressed {
		return ""
	}
	if ev.Rune == '\t' {
		return "Tab"
	}
	if ev.Rune == '\r' || ev.Rune == '\n' || ev.Rune == ' ' {
		if ev.Rune == ' ' {
			return "Space"
		}
		return "Enter"
	}
	switch ev.KeyCode {
	case int(input.KeyTab):
		return "Tab"
	case int(input.KeyEnter):
		return "Enter"
	case int(input.KeySpace):
		return "Space"
	case int(input.KeyEscape):
		return "Escape"
	}
	// X11 keysym fallback (0xff09 Tab, 0xff0d/0xff8d Enter, 0xff1b Esc, space 0x20).
	switch ev.KeyCode {
	case 0xff09:
		return "Tab"
	case 0xff0d, 0xff8d:
		return "Enter"
	case 0x20:
		return "Space"
	case 0xff1b:
		return "Escape"
	}
	return ""
}
