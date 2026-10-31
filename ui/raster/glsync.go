//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package raster

// RunGLSync runs fn on the process GL thread (the hub) and waits for it to
// return. Half-sync handoff for open-path work that binds shared EGL
// contexts (C7/P2.4e): the frame path keeps its binding across
// Submit→Present, so a contending MakeCurrent from another thread fails
// with EGL_BAD_ACCESS 0x3002 — running the open as one hub job serializes
// it with every frame by construction. Inline fast-path when the caller
// already runs on the GL thread (nested RunSync inside a job).
func RunGLSync(fn func() error) error { return sharedHub.runJob(fn) }
