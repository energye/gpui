//go:build linux

package webgpu

import (
	"sync"

	"github.com/ebitengine/purego"
)

// x11Probe holds the lazily-resolved libX11 functions for the live window
// size probe. Resolved once per process (sync.Once): the probe runs on every
// stale-size reconfigure retry during interactive resize drags, so a
// per-call Dlopen would waste CPU on the hot path (Dlopen does symbol
// resolution + refcount bookkeeping each time).
type x11Probe struct {
	lockDisplay   func(dpy uintptr)
	unlockDisplay func(dpy uintptr)
	getGeometry   func(dpy, win uintptr, root *uintptr, x, y *int32, w, h *uint32, border *uint32, depth *uint32) int
}

var (
	x11Once sync.Once
	x11P    x11Probe
)

// loadX11Probe resolves libX11 once. On failure the struct stays zero and
// x11WindowSize falls back to ok=false (the caller keeps the applied size).
func loadX11Probe() {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return
	}
	// The .so handle must stay alive for the registered closures — it is a
	// process-lifetime singleton (never Dlclosed), matching ui/platform.
	purego.RegisterLibFunc(&x11P.lockDisplay, lib, "XLockDisplay")
	purego.RegisterLibFunc(&x11P.unlockDisplay, lib, "XUnlockDisplay")
	purego.RegisterLibFunc(&x11P.getGeometry, lib, "XGetGeometry")
}

// x11WindowSize probes the current client extent of an X11 window via
// XGetGeometry (fresh at call time, µs — the WM's ConfigureNotify stream lags
// the actual window during an interactive resize drag). ok=false when libX11
// or the window is unavailable.
//
// This is the size the wgpu-native surface compares against on acquire: a
// swapchain configured at a stale applied size reports "outdated" and the
// frame is dropped. Reconfiguring at the probed size right before acquiring
// keeps interactive resize drags presenting instead of freezing the content.
func x11WindowSize(display, window uintptr) (w, h int, ok bool) {
	if display == 0 || window == 0 {
		return 0, 0, false
	}
	x11Once.Do(loadX11Probe)
	p := x11P
	if p.getGeometry == nil {
		return 0, 0, false
	}
	// Xlib is not thread-safe: the UI thread owns the display connection, so
	// the probe must hold the display lock while touching X state.
	if p.lockDisplay != nil {
		p.lockDisplay(display)
	}
	defer func() {
		if p.unlockDisplay != nil {
			p.unlockDisplay(display)
		}
	}()
	var (
		rootRet uintptr
		x, y    int32
		wd, ht  uint32
		border  uint32
		depth   uint32
	)
	// XGetGeometry writes root_return unconditionally — a nil root pointer
	// dereferences address 0 in the X server client library.
	if p.getGeometry(display, window, &rootRet, &x, &y, &wd, &ht, &border, &depth) == 0 {
		return 0, 0, false
	}
	return int(wd), int(ht), true
}