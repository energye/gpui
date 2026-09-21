// Package behavior hosts the F0-4 interaction facades (P4).
//
// Every facade wraps existing engine capability only: gestures for tap
// arbitration, focus for keyboard traversal, overlay for placement and
// outside-close, textinput concepts via a small controller interface.
// No new GPU code lives here. Behavior never draws; it owns state
// transitions and event routing, while prim owns nodes and paint.
package behavior
