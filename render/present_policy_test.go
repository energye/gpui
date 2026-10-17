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

import "testing"

// One frame has one pacer: the framework paces on the Linux window systems
// (host supplies real vblank events), the driver paces on Windows/macOS (no
// host vsync source). Only the Go/GLES backend implements the non-blocking
// request as a real EGL swap interval, so only it takes this path — the WebGPU
// backend keeps its own present-mode choice.
func TestNonBlockingPresent(t *testing.T) {
	cases := []struct {
		platform PresentPlatform
		backend  Backend
		want     bool
	}{
		{PresentPlatformX11, BackendGo, true},
		{PresentPlatformWayland, BackendGo, true},
		{PresentPlatformX11, BackendNative, false},
		{PresentPlatformWayland, BackendNative, false},
		{PresentPlatformWin32, BackendNative, false},
		{PresentPlatformWin32, BackendGo, false},
		{PresentPlatformAppKit, BackendNative, false},
		{PresentPlatformAppKit, BackendGo, false},
	}
	for _, c := range cases {
		if got := nonBlockingPresent(c.platform, c.backend); got != c.want {
			t.Errorf("nonBlockingPresent(platform=%d, backend=%d) = %v, want %v",
				c.platform, c.backend, got, c.want)
		}
	}
}
