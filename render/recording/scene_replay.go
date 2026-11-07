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
	"errors"
	"fmt"

	"github.com/energye/gpui/render"
)

// ErrUnsupportedSceneTarget reports a playback target that cannot run a
// scene replay: the display list carries the scene command, but the target
// is neither a live *render.Context nor a *Recorder to fold into.
var ErrUnsupportedSceneTarget = errors.New("recording: scene replay target unsupported")

// ErrUnsupportedPictureTarget reports a playback target that cannot run a
// nested picture: the display list carries the picture command, but the
// target is neither a live *render.Context nor a *Recorder to fold into
// (P3.8 batch 6).
var ErrUnsupportedPictureTarget = errors.New("recording: picture replay target unsupported")

// ScenePayload is the replayable unit of a nested scene command (P3.7 batch 6
// wrapup, H21). Standard shape: Skia SkPicture via SkCanvas::drawPicture /
// Flutter DlDisplayList via drawDisplayList — one recording replayed as one
// entry of another. render/scene owns the only production implementation
// (*scene.Scene); the package dependency runs scene → recording, never back.
type ScenePayload interface {
	// RenderSceneInto replays the payload through a live render.Context.
	RenderSceneInto(dc *render.Context) error
}

// SceneReplayCommand applies a nested scene replay at playback. The payload
// is held by reference and read-only at replay (recording 期取值, 回放只读),
// the same carrier discipline as Stroke.PathEffect (batch 4) and the
// material tail parameter (batch 3e-3). The caller must not mutate the
// payload between record and replay; pipelines serialize record→replay
// within one frame, and every window in this repo rebuilds its scene before
// painting.
type SceneReplayCommand struct {
	Payload ScenePayload
}

// Type implements Command.
func (SceneReplayCommand) Type() CommandType { return CmdSceneReplay }

// RenderScene records a nested scene replay. Onto a *Recorder this is the
// recording leg; playback runs backend.RenderScene, which executes the
// payload against the live target context (or folds into an enclosing
// recording when the target is itself a Recorder, mirroring CanvasBackend's
// composability).
func (r *Recorder) RenderScene(p ScenePayload) {
	if r == nil || p == nil {
		return
	}
	r.commands = append(r.commands, SceneReplayCommand{Payload: p})
}

// RenderScene replays a scene payload onto the backend's target canvas.
// A live *render.Context runs it between Push/Pop so the scene's own
// explicit resets cannot leak into whatever the canvas paints next; a
// *Recorder target folds the command into the surrounding display list.
func (b *CanvasBackend) RenderScene(p ScenePayload) error {
	if p == nil {
		return nil
	}
	if rr, ok := b.dc.(*Recorder); ok {
		rr.RenderScene(p)
		return nil
	}
	ctx, ok := b.dc.(*render.Context)
	if !ok {
		return fmt.Errorf("recording.CanvasBackend.RenderScene: %w (%T)", ErrUnsupportedSceneTarget, b.dc)
	}
	b.dc.Push()
	err := p.RenderSceneInto(ctx)
	b.dc.Pop()
	return err
}
