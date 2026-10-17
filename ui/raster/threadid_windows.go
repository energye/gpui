//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build windows

package raster

import "golang.org/x/sys/windows"

// currentTID returns the calling OS thread's id, so the hub can tell whether a
// caller already runs on the GL thread. Windows twin of threadid_linux.go.
func currentTID() int64 {
	return int64(windows.GetCurrentThreadId())
}
