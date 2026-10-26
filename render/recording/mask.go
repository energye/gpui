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
	"sync/atomic"

	"github.com/energye/gpui/render"
)

// Device-space mask commands.
//
// A render.Mask is an alpha plane in target device pixels, not user-space
// geometry. The recorder therefore snapshots the plane at the call (the caller
// may keep mutating its Mask — InvertMask does) and playback restores a fresh
// copy per run, so replaying one recording twice cannot let the first pass's
// InvertMask poison the second. CanvasBackend only executes these commands
// when the playback target has the same device extent as the recording; a
// mismatch is counted, never silently dropped.

// SetMaskCommand installs a device-space alpha mask for subsequent draws.
// A nil Mask means SetMask(nil).
type SetMaskCommand struct {
	Mask *render.Mask
}

// Type implements Command.
func (SetMaskCommand) Type() CommandType { return CmdSetMask }

// ClearMaskCommand removes the current mask.
type ClearMaskCommand struct{}

// Type implements Command.
func (ClearMaskCommand) Type() CommandType { return CmdClearMask }

// InvertMaskCommand flips the current mask in place (no-op when none is set).
type InvertMaskCommand struct{}

// Type implements Command.
func (InvertMaskCommand) Type() CommandType { return CmdInvertMask }

// ApplyMaskCommand applies a destructive DestinationIn post-process.
type ApplyMaskCommand struct {
	Mask *render.Mask
}

// Type implements Command.
func (ApplyMaskCommand) Type() CommandType { return CmdApplyMask }

// PushMaskLayerCommand opens a masked save-layer composited at PopLayer.
type PushMaskLayerCommand struct {
	Mask *render.Mask
}

// Type implements Command.
func (PushMaskLayerCommand) Type() CommandType { return CmdPushMaskLayer }

// maskSkips counts mask commands that could not be executed because the
// playback target lacked the capability or a matching device extent. It is
// process-wide so a window can prove that no mask disappeared silently.
var maskSkips atomic.Int64

// MaskSkips returns how many mask commands playback had to skip. A non-zero
// value means the target could not reproduce the recording's device space (or
// has no mask capability at all); callers that expect masks to be visible
// must treat it as a failure.
func MaskSkips() int64 { return maskSkips.Load() }

// ResetMaskSkips zeroes the skip counter (tests / per-window measurement).
func ResetMaskSkips() { maskSkips.Store(0) }

// cloneMask snapshots a mask for a command. The command owns this copy; the
// caller keeps its own Mask and may continue mutating it.
func cloneMask(mask *render.Mask) *render.Mask {
	if mask == nil {
		return nil
	}
	return mask.Clone()
}

// SetMask records a device-space alpha mask for subsequent Fill/Stroke draws.
func (r *Recorder) SetMask(mask *render.Mask) {
	r.commands = append(r.commands, SetMaskCommand{Mask: cloneMask(mask)})
}

// ClearMask records removal of the current mask.
func (r *Recorder) ClearMask() {
	r.commands = append(r.commands, ClearMaskCommand{})
}

// InvertMask records an in-place inversion of the current mask.
func (r *Recorder) InvertMask() {
	r.commands = append(r.commands, InvertMaskCommand{})
}

// ApplyMask records a destructive DestinationIn post-process at the surface
// origin.
func (r *Recorder) ApplyMask(mask *render.Mask) {
	r.commands = append(r.commands, ApplyMaskCommand{Mask: cloneMask(mask)})
}

// PushMaskLayer records an isolated layer modulated by mask at PopLayer.
func (r *Recorder) PushMaskLayer(mask *render.Mask) {
	r.commands = append(r.commands, PushMaskLayerCommand{Mask: cloneMask(mask)})
}

// The recorder is a mask-carrying display-list target. This is what lets a
// FullPaint walk record mask post-processes on the UI thread and replay them
// on the raster thread.
var _ render.MaskCanvas = (*Recorder)(nil)
