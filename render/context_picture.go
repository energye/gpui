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

import "errors"

// Picture replay guards (P3.8 batch 6). A cyclic picture (one that draws
// itself, directly or through a chain) would recurse forever; Skia leaves
// this to the caller, this engine fails closed instead — the offending
// entry is skipped and counted rather than overflowing the stack.
var (
	// ErrPictureCycle reports a picture that (transitively) draws itself.
	ErrPictureCycle = errors.New("render: picture replay cycle")
	// ErrPictureDepth reports a picture nesting deeper than the guard.
	ErrPictureDepth = errors.New("render: picture replay depth exceeded")
)

// maxPictureDepth bounds picture replay recursion; real display lists nest
// a handful of levels (mirrors the SVG use-depth guard discipline).
const maxPictureDepth = 64

// DrawPicture replays a completed picture onto this context between
// Push/Pop so the picture's own state changes cannot leak into what the
// canvas paints next. Cyclic or over-deep payloads are refused: the entry
// draws nothing (fail-closed, the same stance the projective text family
// takes) and the refusal is reported to the registered logger.
func (c *Context) DrawPicture(pic Picture) {
	if c == nil || pic == nil {
		return
	}
	// Cycle check: the picture is already being replayed further up the
	// stack (the stack holds one entry per in-flight DrawPicture on this
	// context).
	for _, inFlight := range c.pictureStack {
		if inFlight == pic {
			c.notePictureRefusal(ErrPictureCycle)
			return
		}
	}
	if len(c.pictureStack) >= maxPictureDepth {
		c.notePictureRefusal(ErrPictureDepth)
		return
	}

	c.pictureStack = append(c.pictureStack, pic)
	c.Push()
	pic.DrawTo(c)
	c.Pop()
	c.pictureStack = c.pictureStack[:len(c.pictureStack)-1]
}

// PictureRefusals returns how many picture entries this context has
// refused (cycles / over-depth) — the fail-closed guard's visible counter.
func (c *Context) PictureRefusals() int { return c.pictureRefusals }

// notePictureRefusal records one refused picture entry. The refusal is
// counted (visible in logs) rather than fatal: the remainder of the frame
// still renders, matching the spec's ignore-and-continue reading.
func (c *Context) notePictureRefusal(reason error) {
	c.pictureRefusals++
	Logger().Warn("picture replay refused", "reason", reason, "count", c.pictureRefusals)
}
