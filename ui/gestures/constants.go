//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gestures

import "github.com/energye/gpui/ui/input"

// kTouchSlop is the default movement threshold (logical px) that separates
// a tap from a pan.
const kTouchSlop = 8.0

// PrimaryPointerID is the mouse cursor pointer id (0). Touch slots are ≥ 1.
const PrimaryPointerID = input.PrimaryPointerID
