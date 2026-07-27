// Package gestures implements a Flutter-style GestureArena (P5a).
//
// Hit-test path order is deepest-first (leaf → root): recognizers on the
// deepest target add to the arena first. One pointer = one arena (MVP
// pointer id defaults to PrimaryPointerID).
//
// Tap vs pan: movement within kTouchSlop keeps both pending; beyond slop
// pan accepts and tap is rejected; up within slop makes tap accept.
//
// Wheel (PointerScroll) bypasses the arena — use ScrollWheel or route
// directly to a viewport.
//
// Nested scroll (P5b) lives in rendering.Scrollable (Parent residual handoff),
// not by re-winning the arena mid-gesture. Hit paths should still list the
// deepest Scrollable first when joining the arena.
//
// Dependency: gestures → platform only (no rendering/gpu).
package gestures
