package gestures

import "github.com/energye/gpui/ui/input"

// kTouchSlop is the default movement threshold (logical px) that separates
// a tap from a pan. Matches ENGINE_PHASE_P5 A.1.
const kTouchSlop = 8.0

// PrimaryPointerID is the mouse cursor pointer id (0). Touch slots are ≥ 1.
// It aliases input.PrimaryPointerID so gesture code and UI code agree on the
// unified pointer vocabulary (plan §5).
const PrimaryPointerID = input.PrimaryPointerID