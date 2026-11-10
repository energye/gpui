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
