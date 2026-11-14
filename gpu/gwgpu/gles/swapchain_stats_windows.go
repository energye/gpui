//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build windows

package gles

import "github.com/energye/gpui/gpu/hal"

// PresentStats reports zero partial counts: WGL SwapBuffers has no
// OS-level damage/partial present protocol, so every present is a full
// swap and there is no partial counter to report (协议不可做，非未实现).
func (sc *Swapchain) PresentStats() hal.PresentStats {
	return hal.PresentStats{}
}
