// Package event provides an event-driven interaction system.
//
// This is a placeholder package for the future event system.
// It will handle mouse, keyboard, touch, and focus events
// with event routing through the widget tree.
package event

// Event is the base type for all events.
type Event interface {
	// Handled returns whether this event has been handled.
	Handled() bool
}
