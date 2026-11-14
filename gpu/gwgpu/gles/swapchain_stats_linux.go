//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build linux

package gles

import "github.com/energye/gpui/gpu/hal"

// PresentStats reports the surface's cumulative present counters
// (presentFullCount / presentPartialCount, resource_linux.go). Written by
// the queue present paths on the raster thread; render reads it on the same
// thread right after the present it just executed.
func (sc *Swapchain) PresentStats() hal.PresentStats {
	if s, ok := sc.surf.(*Surface); ok {
		return hal.PresentStats{
			PartialCount: s.presentPartialCount,
			FullCount:    s.presentFullCount,
		}
	}
	return hal.PresentStats{}
}
