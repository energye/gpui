//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package gpui

// StartAnimation begins continuous rendering (OnDraw style).
// Each frame triggers Invalidate() at the end of onPaint, creating a
// render loop tied to the display refresh rate via SwapBuffers vsync.
// This avoids the timer precision issues of lcl.TTimer.
func (c *TGPUControl) StartAnimation() {
	if c == nil || c.ctrl == nil {
		return
	}
	c.animating = true
	c.Invalidate() // start the render loop
}

// StopAnimation stops continuous rendering.
func (c *TGPUControl) StopAnimation() {
	if c == nil {
		return
	}
	c.animating = false
}