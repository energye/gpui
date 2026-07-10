//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package gpui

// PresentOptions controls pixmap display.
type PresentOptions struct {
	FlipY bool
}

// DefaultPresentOptions returns default present options.
func DefaultPresentOptions() PresentOptions {
	return PresentOptions{FlipY: false}
}

// StartAnimation begins continuous rendering at ~60fps.
func (c *TGPUControl) StartAnimation() {
	// TODO: implement with LCL TTimer
}

// StopAnimation stops continuous rendering.
func (c *TGPUControl) StopAnimation() {
	// TODO: implement with LCL TTimer
}