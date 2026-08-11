package input

// Key is a cross-platform logical key identifier. Every platform backend
// maps its native keycode/keysym/virtual-key into this table; upper layers
// (focus, shortcuts, kit widgets) never see platform-specific key values.
//
// The table is intentionally stable: adding keys appends, never renumbers,
// so serialized key codes remain comparable across versions.
type Key uint32

const (
	// KeyNone is the zero value (no key).
	KeyNone Key = iota

	// Letters A–Z (logical, independent of physical layout).
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ

	// Digits 0–9 (top row and/or numpad logical digits).
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9

	// Function keys.
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyF13
	KeyF14
	KeyF15
	KeyF16
	KeyF17
	KeyF18
	KeyF19
	KeyF20
	KeyF21
	KeyF22
	KeyF23
	KeyF24

	// Control / navigation keys.
	KeyEnter
	KeyTab
	KeySpace
	KeyBackspace
	KeyDelete
	KeyEscape
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyInsert
	KeyPrintScreen
	KeyCapsLock
	KeyNumLock
	KeyScrollLock
	KeyPause
	KeyMenu

	// Arrow keys.
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight

	// Modifier keys (logical; Modifiers carries the held state).
	KeyShift
	KeyControl
	KeyAlt
	KeyMeta // Win key / Cmd key / Super

	// Punctuation & symbol row (logical glyph positions; physical layout
	// differences are absorbed by the backend mapping).
	KeyMinus
	KeyEqual
	KeyBracketLeft
	KeyBracketRight
	KeyBackslash
	KeySemicolon
	KeyQuote
	KeyGrave      // ` ~
	KeyComma
	KeyPeriod
	KeySlash

	// Keypad (logical numbers/operators; backends may remap to digits when
	// NumLock semantics require it).
	KeyPad0
	KeyPad1
	KeyPad2
	KeyPad3
	KeyPad4
	KeyPad5
	KeyPad6
	KeyPad7
	KeyPad8
	KeyPad9
	KeyPadDecimal
	KeyPadDivide
	KeyPadMultiply
	KeyPadSubtract
	KeyPadAdd
	KeyPadEnter

	// IME-related keys: composition toggles and candidate navigation.
	KeyIMECompose
	KeyIMECandidatePrev
	KeyIMECandidateNext
	KeyIMECandidateSelect
)

func (k Key) String() string {
	names := map[Key]string{
		KeyNone: "none",
		KeyA: "a", KeyB: "b", KeyC: "c", KeyD: "d", KeyE: "e", KeyF: "f",
		KeyG: "g", KeyH: "h", KeyI: "i", KeyJ: "j", KeyK: "k", KeyL: "l",
		KeyM: "m", KeyN: "n", KeyO: "o", KeyP: "p", KeyQ: "q", KeyR: "r",
		KeyS: "s", KeyT: "t", KeyU: "u", KeyV: "v", KeyW: "w", KeyX: "x",
		KeyY: "y", KeyZ: "z",
		Key0: "0", Key1: "1", Key2: "2", Key3: "3", Key4: "4",
		Key5: "5", Key6: "6", Key7: "7", Key8: "8", Key9: "9",
		KeyEnter: "enter", KeyTab: "tab", KeySpace: "space",
		KeyBackspace: "backspace", KeyDelete: "delete", KeyEscape: "escape",
		KeyHome: "home", KeyEnd: "end", KeyPageUp: "pageup", KeyPageDown: "pagedown",
		KeyInsert: "insert", KeyPrintScreen: "printscreen",
		KeyCapsLock: "capslock", KeyNumLock: "numlock", KeyScrollLock: "scrolllock",
		KeyPause: "pause", KeyMenu: "menu",
		KeyArrowUp: "arrowup", KeyArrowDown: "arrowdown",
		KeyArrowLeft: "arrowleft", KeyArrowRight: "arrowright",
		KeyShift: "shift", KeyControl: "control", KeyAlt: "alt", KeyMeta: "meta",
		KeyMinus: "minus", KeyEqual: "equal",
		KeyBracketLeft: "bracketleft", KeyBracketRight: "bracketright",
		KeyBackslash: "backslash", KeySemicolon: "semicolon", KeyQuote: "quote",
		KeyGrave: "grave", KeyComma: "comma", KeyPeriod: "period", KeySlash: "slash",
		KeyPad0: "pad0", KeyPad1: "pad1", KeyPad2: "pad2", KeyPad3: "pad3",
		KeyPad4: "pad4", KeyPad5: "pad5", KeyPad6: "pad6", KeyPad7: "pad7",
		KeyPad8: "pad8", KeyPad9: "pad9", KeyPadDecimal: "paddecimal",
		KeyPadDivide: "paddivide", KeyPadMultiply: "padmultiply",
		KeyPadSubtract: "paddsubtract", KeyPadAdd: "padadd", KeyPadEnter: "paddenter",
		KeyIMECompose: "imecompose", KeyIMECandidatePrev: "imecandprev",
		KeyIMECandidateNext: "imecandnext", KeyIMECandidateSelect: "imecandselect",
	}
	if s, ok := names[k]; ok {
		return s
	}
	for i := KeyF1; i <= KeyF24; i++ {
		if k == i {
			return "f" + itoa(1+int(k-KeyF1))
		}
	}
	return "unknown"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [8]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// KeyEvent is a keyboard press/release in logical key space.
type KeyEvent struct {
	Key     Key    // logical key (KeyNone if only Rune is meaningful)
	Rune    rune   // printable character produced by this key (0 if none)
	Pressed bool   // true = press, false = release
	Repeat  bool   // true = auto-repeat from OS
}