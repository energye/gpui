package gestures

// kTouchSlop is the default movement threshold (logical px) that separates
// a tap from a pan. Matches ENGINE_PHASE_P5 A.1.
const kTouchSlop = 8.0

// PrimaryPointerID is the MVP single-pointer id when the platform event
// has no multi-touch identity.
const PrimaryPointerID = 1
