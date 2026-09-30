//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

// Renderer is the interface for rendering paths to a pixmap.
type Renderer interface {
	// Fill fills a path with the given paint.
	// Returns an error if the rendering operation fails.
	Fill(pixmap *Pixmap, path *Path, paint *Paint) error

	// Stroke strokes a path with the given paint.
	// Returns an error if the rendering operation fails.
	Stroke(pixmap *Pixmap, path *Path, paint *Paint) error
}
