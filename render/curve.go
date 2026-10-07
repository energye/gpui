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

import ()

// Curve types for 2D geometry operations.

// Rect represents an axis-aligned rectangle.
// Min is the top-left corner (minimum coordinates).
// Max is the bottom-right corner (maximum coordinates).
type Rect struct {
	Min, Max Point
}
