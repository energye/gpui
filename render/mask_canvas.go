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

// MaskCanvas is a Canvas that can carry device-space alpha-mask
// post-processes. It is a separate capability rather than part of Canvas
// because a Mask is a raster-stage plane in target device pixels, not
// user-space geometry: SetMask samples the plane against the target surface,
// and ApplyMask composites it at the surface origin with an identity
// transform. A display list that carries masks therefore has to replay into
// the same device space it was recorded in.
//
// The live *Context implements this directly. recording.Recorder implements
// it by snapshotting every Mask into immutable commands; CanvasBackend replays
// those commands onto a target MaskCanvas. Layer-local recordings and
// user-space mask shaders are outside this contract (Skia keeps SkMask at the
// raster stage; Flutter expresses user-space masks through saveLayer + paint).
type MaskCanvas interface {
	Canvas

	// SetMask sets an alpha mask for subsequent Fill and Stroke operations.
	// Pass nil to clear the mask.
	SetMask(mask *Mask)

	// ClearMask removes the current mask.
	ClearMask()

	// InvertMask inverts the current mask in place. It is a no-op when no
	// mask is set.
	InvertMask()

	// ApplyMask applies a destructive DestinationIn post-process to the
	// target surface using mask. The mask maps 1:1 onto the surface origin.
	ApplyMask(mask *Mask)

	// PushMaskLayer opens an isolated layer whose content is modulated by
	// mask when PopLayer composites it onto the parent.
	PushMaskLayer(mask *Mask)
}

// The live context is the reference MaskCanvas implementation.
var _ MaskCanvas = (*Context)(nil)
