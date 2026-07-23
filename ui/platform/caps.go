package platform

// Caps describes optional host capabilities. Missing caps degrade gracefully.
type Caps uint64

const (
	CapWindow Caps = 1 << iota
	CapPointer
	CapKeyboard
	CapTextInput
	CapIME
	CapClipboard
	CapCursor
	CapPresent
	CapSurfaceLifecycle
	CapDarkMode
	CapReduceMotion
	CapFontScale
)

// Has reports whether all bits in c are set.
func (c Caps) Has(want Caps) bool { return c&want == want }

// String returns a short debug description.
func (c Caps) String() string {
	if c == 0 {
		return "Caps(none)"
	}
	names := []struct {
		bit  Caps
		name string
	}{
		{CapWindow, "Window"},
		{CapPointer, "Pointer"},
		{CapKeyboard, "Keyboard"},
		{CapTextInput, "TextInput"},
		{CapIME, "IME"},
		{CapClipboard, "Clipboard"},
		{CapCursor, "Cursor"},
		{CapPresent, "Present"},
		{CapSurfaceLifecycle, "SurfaceLifecycle"},
		{CapDarkMode, "DarkMode"},
		{CapReduceMotion, "ReduceMotion"},
		{CapFontScale, "FontScale"},
	}
	out := "Caps("
	first := true
	for _, n := range names {
		if c.Has(n.bit) {
			if !first {
				out += "|"
			}
			out += n.name
			first = false
		}
	}
	return out + ")"
}

// HeadlessCaps is the test host capability set.
// CapIME is included so composition sequences can be injected in CI
// (Linux true-window host does NOT advertise CapIME until XIM is wired).
// CapClipboard is included with an in-memory clipboard on Headless.
const HeadlessCaps = CapWindow | CapPointer | CapKeyboard | CapTextInput | CapIME | CapClipboard | CapPresent
