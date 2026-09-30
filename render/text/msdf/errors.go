//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package msdf

import "errors"

// Sentinel errors for msdf package.
var (
	// ErrAllocationFailed is returned when glyph allocation in atlas fails.
	ErrAllocationFailed = errors.New("msdf: failed to allocate glyph in atlas")

	// ErrLengthMismatch is returned when keys and outlines have different lengths.
	ErrLengthMismatch = errors.New("msdf: keys and outlines must have same length")
)
