//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !linux && !windows

package raster

// currentTID reports that thread identity is unknown on this platform
// (neither unix.Gettid nor GetCurrentThreadId is available). The hub then
// treats every caller as off the GL thread, so RunSync still works from the UI
// thread; only the "called from inside a job" self-deadlock guard is inactive,
// and RunSync's contract forbids that call. The GL backends with a kept
// binding (EGL on Linux, WGL on Windows) are the platforms above.
func currentTID() int64 {
	return -1
}
