//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux

package raster

import "golang.org/x/sys/unix"

// currentTID returns the calling OS thread's id, so the hub can tell whether a
// caller already runs on the GL thread. Linux twin of threadid_windows.go.
func currentTID() int64 {
	return int64(unix.Gettid())
}
