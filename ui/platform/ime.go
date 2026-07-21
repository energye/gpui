package platform

// IME capability contract (W4 / §12.1 C1–C3).
//
// # CapIME — composition events
//
// When CapIME is set, the host may enqueue EventIME (preedit updates and End)
// which platform.Dispatch maps to core.Tree.DispatchIME → EditableText.HandleIME.
// Committed characters are delivered as EventText → DispatchTextInput.
//
// # LinuxHost (true window) — formal degradation
//
// LinuxHost does **not** set CapIME. XIM/XIC is not wired in the thin X11
// adapter. OS CJK candidate UI / composition events are **not** delivered on
// the true window path yet.
//
// What *is* available on Linux true window:
//   - CapTextInput + CapKeyboard
//   - KeyPress → XLookupString → EventKey / EventText (Latin + special keys)
//
// Degraded / test path (must be used until CapIME is advertised):
//  1. App checks host.Caps().Has(CapIME).
//  2. If false: do not claim true-window IME; use Headless InjectIME for CI.
//  3. Headless sets CapIME and implements IMEPositioner for composition tests.
//
// # SetIMEPosition (C3)
//
// Hosts implementing IMEPositioner place the OS candidate window near the caret.
// LinuxHost does not implement IMEPositioner while CapIME is off.
// EditableText.CaretLocalPos + core.AbsoluteOffset supply caret geometry.
//
// See docs/UI_FRAMEWORK_MAP.md §12.1 C1–C4 and §12.3 W4.

// IMEPositioner is an optional Host extension for candidate-window placement.
// Coordinates are logical pixels in the client area (top-left origin).
type IMEPositioner interface {
	SetIMEPosition(x, y float64)
}

// SetIMEPositionIfSupported calls SetIMEPosition when host implements IMEPositioner.
// Returns true if the call was forwarded.
func SetIMEPositionIfSupported(h Host, x, y float64) bool {
	if p, ok := h.(IMEPositioner); ok {
		p.SetIMEPosition(x, y)
		return true
	}
	return false
}
