package input

import "github.com/energye/gpui/ui/platform"

// FromPlatform normalizes one platform event into the cross-platform input
// Event vocabulary. This is the single point where per-platform differences
// (keysyms, virtual key codes, X11/GDK conventions…) are absorbed; upper
// layers never re-parse them.
//
// mods must reflect the current physical modifier state (held keys at event
// time); backends that already carry modifier state should fold it in before
// calling. Events that have no input meaning (EventNone) are dropped (returns
// zero Event with Kind == KindNone; callers may filter on Kind).
func FromPlatform(ev platform.Event, mods Modifiers) Event {
	switch ev.Type {
	case platform.EventClose:
		return Event{Kind: KindClose}
	case platform.EventResize:
		e := Event{Kind: KindResize, Width: ev.Width, Height: ev.Height, Scale: ev.Scale}
		if e.Scale <= 0 {
			e.Scale = 1
		}
		return e
	case platform.EventWake:
		return Event{Kind: KindWake}
	case platform.EventPointer:
		return fromPointer(ev, mods)
	case platform.EventKey:
		return fromKey(ev, mods)
	case platform.EventIME:
		return fromIME(ev, mods)
	default:
		return Event{Kind: KindNone}
	}
}

// fromIME maps a platform IME event into the normalized IME event. The
// platform IMEKind (0=compose, 1=commit, 2=caret) aligns with input.IMEKind.
func fromIME(ev platform.Event, mods Modifiers) Event {
	kind := IMEKind(ev.IMEKind)
	if kind < IMECompose || kind > IMECaretMove {
		kind = IMECompose
	}
	return Event{
		Kind:      KindIME,
		Modifiers: mods,
		IME: IMEEvent{
			Kind:  kind,
			Text:  ev.IMEText,
			Start: ev.IMEStart,
			End:   ev.IMEEnd,
		},
	}
}

// fromPointer maps a platform pointer event. Mouse primary = ID 0; scrolled
// wheel/trackpad produces Kind == KindScroll.
func fromPointer(ev platform.Event, mods Modifiers) Event {
	switch ev.Pointer {
	case platform.PointerScroll:
		return Event{
			Kind:      KindScroll,
			Modifiers: mods,
			Pointer: PointerEvent{
				Kind:    PointerScroll,
				ID:      PrimaryPointerID,
				X:       ev.X,
				Y:       ev.Y,
				ScrollX: ev.ScrollX,
				ScrollY: ev.ScrollY,
			},
		}
	default:
		k := toPointerKind(ev.Pointer)
		return Event{
			Kind:      KindPointer,
			Modifiers: mods,
			Pointer: PointerEvent{
				Kind:   k,
				ID:     PrimaryPointerID,
				X:      ev.X,
				Y:      ev.Y,
				Button: ev.Button,
			},
		}
	}
}

func toPointerKind(k platform.PointerKind) PointerKind {
	switch k {
	case platform.PointerDown:
		return PointerDown
	case platform.PointerUp:
		return PointerUp
	default:
		return PointerMove
	}
}

// fromKey maps a platform key event into logical key space. Non-key payloads
// (an unidentifiable key with no rune) still produce a KindKey event with
// Key == KeyNone so press/release state is retrainable by focus.
func fromKey(ev platform.Event, mods Modifiers) Event {
	return Event{
		Kind:      KindKey,
		Modifiers: mods,
		Key: KeyEvent{
			Key:     mapKey(ev.KeyCode),
			Rune:    ev.Rune,
			Pressed: ev.Pressed,
			Mods:    mods,
		},
	}
}

// mapKey converts a platform key code (keysym / virtual key) into the logical
// Key table. Printable ASCII is treated as the corresponding letter/digit
// only when it is a plain text key; when a rune already exists the Rune field
// carries the text and Key is best-effort.
func mapKey(code int) Key {
	// ASCII letters A–Z.
	if code >= 'a' && code <= 'z' {
		return KeyA + Key(code-'a')
	}
	if code >= 'A' && code <= 'Z' {
		return KeyA + Key(code-'A')
	}
	// ASCII digits 0–9.
	if code >= '0' && code <= '9' {
		return Key0 + Key(code-'0')
	}
	// Punctuation / symbols row (common ASCII).
	switch code {
	case '-', '_':
		return KeyMinus
	case '=', '+':
		return KeyEqual
	case '[', '{':
		return KeyBracketLeft
	case ']', '}':
		return KeyBracketRight
	case '\\', '|':
		return KeyBackslash
	case ';', ':':
		return KeySemicolon
	case '\'', '"':
		return KeyQuote
	case '`', '~':
		return KeyGrave
	case ',', '<':
		return KeyComma
	case '.', '>':
		return KeyPeriod
	case '/', '?':
		return KeySlash
	case ' ':
		return KeySpace
	case '\t':
		return KeyTab
	case '\n', '\r':
		return KeyEnter
	}
	// X11 keysym range 0xff00–0xffff (special keys); other platforms should
	// map their own virtual codes to the same logical positions before here.
	switch code {
	case 0xff08: // BackSpace
		return KeyBackspace
	case 0xff09: // Tab
		return KeyTab
	case 0xff0d: // Return
		return KeyEnter
	case 0xff1b: // Escape
		return KeyEscape
	case 0xff50: // Home
		return KeyHome
	case 0xff51: // Left
		return KeyArrowLeft
	case 0xff52: // Up
		return KeyArrowUp
	case 0xff53: // Right
		return KeyArrowRight
	case 0xff54: // Down
		return KeyArrowDown
	case 0xff55: // Page_Up
		return KeyPageUp
	case 0xff56: // Page_Down
		return KeyPageDown
	case 0xff57: // End
		return KeyEnd
	case 0xff63: // Insert
		return KeyInsert
	case 0xffe1, 0xffe2: // Shift L/R
		return KeyShift
	case 0xffe3, 0xffe4: // Control L/R
		return KeyControl
	case 0xffe9, 0xffea: // Alt
		return KeyAlt
	case 0xffeb, 0xffec: // Super/Meta
		return KeyMeta
	}
	// F1–F24 keysyms: 0xffbe..0xffd5.
	if code >= 0xffbe && code <= 0xffd5 {
		return KeyF1 + Key(code-0xffbe)
	}
	return KeyNone
}

// FromTouch is the normalized multi-touch entry point for backends that
// surface touch slots. Keep KindTouch with an ID ≥ 1 (0 is the mouse).
func FromTouch(ev TouchEvent, mods Modifiers) Event {
	return Event{Kind: KindTouch, Modifiers: mods, Touch: ev}
}

// FromText is the normalized committed-text entry point (keyboard chars,
// paste, or an IME commit surfaced as plain text).
func FromText(s string, mods Modifiers) Event {
	return Event{Kind: KindText, Modifiers: mods, Text: TextEvent{Text: s}}
}

// FromIME is the normalized IME session event entry point.
func FromIME(ev IMEEvent, mods Modifiers) Event {
	return Event{Kind: KindIME, Modifiers: mods, IME: ev}
}