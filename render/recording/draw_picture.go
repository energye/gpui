//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package recording

import (
	"fmt"

	"github.com/energye/gpui/render"
)

// DrawPictureCommand replays a nested picture at playback (P3.8 batch 6).
// The payload is the general form of ScenePayload (H21): any completed
// *Recorder, held by reference and read-only at replay — the same carrier
// discipline as ScenePayload, Stroke.PathEffect and the material tail.
// Cycle and depth guards run at replay on the target context
// (render.Context.DrawPicture's picture stack), so a picture that draws
// itself fails closed instead of overflowing the stack.
type DrawPictureCommand struct {
	Pic render.Picture
}

// Type implements Command.
func (DrawPictureCommand) Type() CommandType { return CmdDrawPicture }

// DrawPicture records a nested picture replay. Onto a *Recorder this is
// the recording leg; playback runs CanvasBackend.DrawPicture, which either
// folds the picture into an enclosing recorder or replays it into the live
// context between Push/Pop. A nil picture records nothing.
func (r *Recorder) DrawPicture(pic render.Picture) {
	if r == nil || pic == nil {
		return
	}
	r.commands = append(r.commands, DrawPictureCommand{Pic: pic})
}

// DrawTo implements render.Picture: replay this recorder's display list
// into the context through a CanvasBackend (compose onto the context's
// current CTM/clip; Begin/End bracket the state). The caller —
// render.Context.DrawPicture — owns the Push/Pop wrapping and the
// cycle/depth guards.
func (r *Recorder) DrawTo(dc *render.Context) {
	if r == nil || dc == nil {
		return
	}
	rec := r.FinishRecording()
	_ = rec.Playback(NewCanvasBackend(dc))
}

// DrawPicture replays a nested picture onto the backend's target canvas.
// A *Recorder target folds the command into the surrounding display list
// (one display list reused inside another, backend_canvas.go contract); a
// live *render.Context runs it through Context.DrawPicture, which wraps
// Push/Pop and enforces the cycle/depth guards. Unsupported targets are
// reported like SceneReplay's.
func (b *CanvasBackend) DrawPicture(pic render.Picture) error {
	if pic == nil {
		return nil
	}
	if rr, ok := b.dc.(*Recorder); ok {
		rr.DrawPicture(pic)
		return nil
	}
	ctx, ok := b.dc.(*render.Context)
	if !ok {
		return fmt.Errorf("recording.CanvasBackend.DrawPicture: %w (%T)", ErrUnsupportedPictureTarget, b.dc)
	}
	ctx.DrawPicture(pic)
	return nil
}
