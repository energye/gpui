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

// DrawTextureRef is the live-context side of the recordable texture draw:
// resolve the logical ref into the current device's view and queue the quad.
// A released/unknown ref is a no-op (export backends skip it too).
func (c *Context) DrawTextureRef(ref TextureRef, x, y float64, width, height int) {
	view, ok := ResolveTextureRef(ref)
	if !ok {
		return
	}
	c.DrawGPUTexture(view, x, y, width, height)
}

// DrawTextureRefDevice draws a device-space texture tile: the rect is already
// in device pixels, so the user CTM, device scale, and projection are bypassed
// for the quad (the clip stays in device space and still applies). Cached
// shadow tiles use this on a hit; the recorded counterpart is
// DrawTextureRefCommand{Device: true}.
func (c *Context) DrawTextureRefDevice(ref TextureRef, x, y float64, width, height int) {
	view, ok := ResolveTextureRef(ref)
	if !ok {
		return
	}
	savedMatrix := c.matrix
	savedDevice := c.deviceMatrix
	savedProj := c.projectionActive
	c.matrix = Identity()
	c.deviceMatrix = Identity()
	c.projectionActive = false
	c.DrawGPUTexture(view, x, y, width, height)
	c.matrix = savedMatrix
	c.deviceMatrix = savedDevice
	c.projectionActive = savedProj
}
