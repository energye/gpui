// Package clock implements VR4 timing: a bounded frame queue plus a
// pause-aware presentation clock.
//
// Scope is narrow on purpose: the queue and clock never touch containers,
// decoders or pixels (they only move clock.Frame values), so audio can
// grow a second queue later without changing this package. All logic is
// pure Go with only the standard library.
package clock
