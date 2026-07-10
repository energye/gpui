//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package gpui

import (
	"github.com/energye/lcl/lcl"
)

// StartAnimation begins continuous rendering at ~60fps using LCL TTimer.
func (c *TGPUControl) StartAnimation() {
	if c == nil || c.timer != nil || c.ctrl == nil {
		return
	}
	owner, ok := c.ctrl.(lcl.IComponent)
	if !ok {
		return
	}
	c.timer = lcl.NewTimer(owner)
	c.timer.SetInterval(16) // ~60fps
	c.timer.SetOnTimer(func(sender lcl.IObject) {
		c.Invalidate()
	})
	c.timer.SetEnabled(true)
}

// StopAnimation stops continuous rendering.
func (c *TGPUControl) StopAnimation() {
	if c == nil || c.timer == nil {
		return
	}
	c.timer.SetEnabled(false)
	c.timer = nil
}