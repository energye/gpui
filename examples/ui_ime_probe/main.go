//go:build linux

// Command-free probe: opens a real window through ui/platform and checks
// whether the display's input-method capability is available and bindable.
// Works on both Wayland (zwp_text_input_v3) and X11 (XIM). No GPU needed —
// this validates the IME protocol chain end to end.
package main

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/ui/platform"
)

// setInputFocus directs keyboard focus to the probe window so XTest key
// events reach it (and thus the IME).
func setInputFocus(dpy, win uintptr) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	var xSetInputFocus func(dpy uintptr, win uintptr, revertTo int, t uintptr) int
	var xFlush func(dpy uintptr) int
	purego.RegisterLibFunc(&xSetInputFocus, lib, "XSetInputFocus")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	if xSetInputFocus == nil {
		return
	}
	// RevertToParent = 2, CurrentTime = 0.
	xSetInputFocus(dpy, win, 2, 0)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
}

// checkFocus prints the current X input focus window for diagnostics.
func checkFocus(dpy, want uintptr) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	var xGetInputFocus func(dpy uintptr, win *uintptr, revertTo *int) int
	purego.RegisterLibFunc(&xGetInputFocus, lib, "XGetInputFocus")
	if xGetInputFocus == nil {
		return
	}
	var focus uintptr
	var revert int
	xGetInputFocus(dpy, &focus, &revert)
	state := "other"
	if focus == want {
		state = "OUR-WINDOW"
	} else if focus == 0 || focus == 1 {
		state = "none/pointer"
	}
	fmt.Fprintf(os.Stderr, "probe: XGetInputFocus=%x (%s) want=%x\n", focus, state, want)
}

// sendKey delivers a synthetic KeyPress directly to the window via
// XSendEvent (event propagation bypasses focus routing; XIM still filters it).
func sendKey(dpy, win uintptr, r rune) {
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	var (
		xKeysymToKeycode func(dpy uintptr, keysym uintptr) byte
		xSendEvent       func(dpy uintptr, win uintptr, propagate int, mask uintptr, ev *byte) int
		xFlush           func(dpy uintptr) int
	)
	purego.RegisterLibFunc(&xKeysymToKeycode, lib, "XKeysymToKeycode")
	purego.RegisterLibFunc(&xSendEvent, lib, "XSendEvent")
	purego.RegisterLibFunc(&xFlush, lib, "XFlush")
	if xKeysymToKeycode == nil || xSendEvent == nil {
		return
	}
	kc := xKeysymToKeycode(dpy, uintptr(r))
	if kc == 0 {
		return
	}
	// XKeyEvent layout (linux amd64, 96 bytes):
	//   type(0,int)=KeyPress(2) serial(8,ulong) send_event(16,Bool)
	//   display(24,*) window(32) root(40) subwindow(48)
	//   time(56,ulong) x(64) y(68) x_root(72) y_root(76)
	//   state(80,uint) keycode(84,uint) same_screen(88,Bool)
	ev := make([]byte, 96)
	*(*int32)(unsafe.Pointer(&ev[0])) = 2 // KeyPress
	*(*uintptr)(unsafe.Pointer(&ev[24])) = dpy
	*(*uintptr)(unsafe.Pointer(&ev[32])) = win
	*(*uintptr)(unsafe.Pointer(&ev[40])) = win // root placeholder
	*(*uint32)(unsafe.Pointer(&ev[84])) = uint32(kc)
	*(*int32)(unsafe.Pointer(&ev[88])) = 1 // same_screen
	// KeyPressMask = 1<<0
	xSendEvent(dpy, win, 1 /*True*/, 1<<0, &ev[0])
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
}

// synthMotion moves the pointer via XTest (checks event routing reaches the
// window).
func synthMotion(dpy uintptr, x, y int) {
	lib, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libXtst.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return
	}
	var xTestFakeMotion func(dpy uintptr, screen int, x, y int, delay uint) int
	var xFlush func(dpy uintptr) int
	purego.RegisterLibFunc(&xTestFakeMotion, lib, "XTestFakeMotionEvent")
	x11, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	purego.RegisterLibFunc(&xFlush, x11, "XFlush")
	if xTestFakeMotion == nil {
		return
	}
	xTestFakeMotion(dpy, -1, x, y, 0)
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
}

// pressKey synthesizes a key press+release via XTest on the given X11
// display (used to feed the input method real keystrokes).
func pressKey(dpy uintptr, r rune) {
	lib, err := purego.Dlopen("libXtst.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libXtst.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: libXtst unavailable: %v\n", err)
		return
	}
	var (
		xKeysymToKeycode func(dpy uintptr, keysym uintptr) byte
		xTestFakeKey     func(dpy uintptr, keycode uint, isPress int, delay uint) int
		xFlush           func(dpy uintptr) int
	)
	x11, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return
	}
	purego.RegisterLibFunc(&xKeysymToKeycode, x11, "XKeysymToKeycode")
	purego.RegisterLibFunc(&xTestFakeKey, lib, "XTestFakeKeyEvent")
	purego.RegisterLibFunc(&xFlush, x11, "XFlush")
	if xKeysymToKeycode == nil || xTestFakeKey == nil {
		return
	}
	kc := xKeysymToKeycode(dpy, uintptr(r))
	if kc == 0 {
		return
	}
	xTestFakeKey(dpy, uint(kc), 1, 0) // press
	xTestFakeKey(dpy, uint(kc), 0, 0) // release
	xFlush(dpy)
	time.Sleep(50 * time.Millisecond)
}

func main() {
	backend := platform.DetectDisplayBackend()
	fmt.Fprintf(os.Stderr, "probe: backend=%s\n", backend)
	if backend != platform.DisplayWayland && backend != platform.DisplayX11 {
		fmt.Fprintf(os.Stderr, "probe: no display backend (got %s)\n", backend)
		return
	}
	fmt.Fprintf(os.Stderr, "probe: bindings keyboard=%s pointer=%s textinput=%s\n",
		envOn("GPUI_WL_KEYBOARD"), envOn("GPUI_WL_POINTER"), envOn("GPUI_WL_TEXTINPUT"))
	win, err := platform.Open(platform.Options{
		Width: 480, Height: 320, Title: "ime-probe", Backend: backend,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: open failed: %v\n", err)
		os.Exit(1)
	}
	defer win.Close()

	ime := win.IME()
	if ime == nil {
		fmt.Fprintf(os.Stderr, "probe: IME capability NOT available (%s: no input method server?)\n", backend)
	} else {
		fmt.Fprintf(os.Stderr, "probe: IME capability AVAILABLE (%s)\n", backend)
		ime.EnableIME(platform.Rect{X: 10, Y: 10, W: 200, H: 30})
		fmt.Fprintf(os.Stderr, "probe: EnableIME sent\n")
		ime.SetComposing("", 0)
		ime.Commit("")
		if backend == platform.DisplayX11 {
			setInputFocus(win.Host().NativeSurface().Display, win.Host().NativeSurface().Window)
			checkFocus(win.Host().NativeSurface().Display, win.Host().NativeSurface().Window)
			synthMotion(win.Host().NativeSurface().Display, 100, 100)
			sendKey(win.Host().NativeSurface().Display, win.Host().NativeSurface().Window, 'a')
			pressKey(win.Host().NativeSurface().Display, 'a')
			pressKey(win.Host().NativeSurface().Display, 'b')
			pressKey(win.Host().NativeSurface().Display, ' ')
		}
	}
	fmt.Fprintf(os.Stderr, "probe: kind=%s surface=%x/%x\n", win.Kind(),
		win.Host().NativeSurface().Display, win.Host().NativeSurface().Window)

	// Drain events; a real composition would surface as EventIME here.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		evs := win.Host().WaitEvents(200 * time.Millisecond)
		for _, ev := range evs {
			switch ev.Type {
			case platform.EventIME:
				fmt.Fprintf(os.Stderr, "probe: IME event kind=%d text=%q start=%d end=%d\n",
					ev.IMEKind, ev.IMEText, ev.IMEStart, ev.IMEEnd)
			case platform.EventKey:
				fmt.Fprintf(os.Stderr, "probe: KEY code=%d rune=%q pressed=%v\n",
					ev.KeyCode, ev.Rune, ev.Pressed)
			case platform.EventPointer:
				fmt.Fprintf(os.Stderr, "probe: PTR kind=%d at (%.0f,%.0f)\n", ev.Pointer, ev.X, ev.Y)
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "probe: close\n")
				return
			}
		}
	}
	fmt.Fprintf(os.Stderr, "probe: done (no protocol error)\n")
}

func envOn(k string) string {
	if os.Getenv(k) == "1" {
		return "ON "
	}
	return "off"
}
