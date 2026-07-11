// Package widget provides reusable UI components.
//
// This is a placeholder package for the future widget system.
// It will contain Button, Label, Layout, and other composable components
// built on top of the render and ui packages.
package widget

// Widget is the base interface for all UI components.
type Widget interface {
	// Render draws the widget using the render context.
	Render(ctx interface{})
}
