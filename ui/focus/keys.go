package focus

// Logical key codes for routing (host maps platform scancodes into these).
// Values match common ASCII / X11 keysyms where practical.
const (
	KeyTab    = 0xFF09
	KeyReturn = 0xFF0D
	KeyEnter  = KeyReturn
	KeySpace  = 0x0020
	KeyShiftL = 0xFFE1
	KeyShiftR = 0xFFE2
	// KeyTabASCII is accepted as Tab when hosts only send ASCII 9.
	KeyTabASCII = 0x0009
	// KeyEnterASCII is ASCII CR.
	KeyEnterASCII = 0x000D
)

// KeyEvent is the focus-layer keyboard event.
type KeyEvent struct {
	KeyCode int
	Rune    rune
	Pressed bool
	// Shift is true if either shift is held (host may set) or manager tracked it.
	Shift bool
}

// IsTab reports Tab key (including ASCII 9).
func (e KeyEvent) IsTab() bool {
	return e.KeyCode == KeyTab || e.KeyCode == KeyTabASCII
}

// IsActivate reports Space or Enter (activation keys).
func (e KeyEvent) IsActivate() bool {
	return e.KeyCode == KeySpace || e.KeyCode == KeyEnter || e.KeyCode == KeyEnterASCII ||
		e.KeyCode == KeyReturn
}

// IsShift reports a shift key itself.
func (e KeyEvent) IsShift() bool {
	return e.KeyCode == KeyShiftL || e.KeyCode == KeyShiftR
}
