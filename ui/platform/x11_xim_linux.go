//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// XIM (X Input Method) binding via purego, for the X11 backend's IME
// capability (plan §6 / §8).
//
// XIM is the X11 IME channel: the client creates an input context (XIC) on
// its window, then filters key events through XFilterEvent. When the IME is
// composing (e.g. pinyin), XFilterEvent consumes the key; when a composition
// is committed, XFilterEvent returns false and Xutf8LookupString yields the
// committed UTF-8 text, which we surface as platform.Event{Type: EventIME,
// IMEKind: 1 (commit)}.
//
// We use XIMPreeditNothing|XIMStatusNothing: the input method (ibus/fcitx)
// owns its own candidate popup, so the client only receives committed text —
// the pre-edit UI is the IME's, not ours. This is the "minimal viable XIM"
// client; richer pre-edit callbacks (XNPreeditStartCallback etc.) are a
// future enhancement.
//
// Like the Wayland text-input binding, this is an OPTIONAL capability: if
// XOpenIM fails (no input method server), Window.IME() stays nil and the app
// degrades to plain keyboard text.

// XIM attribute name constants (must be kept alive for the C call).
var ximNames = struct {
	inputStyle   []byte // "inputStyle"
	clientWindow []byte // "clientWindow"
}{
	inputStyle:   append([]byte("inputStyle"), 0),
	clientWindow: append([]byte("clientWindow"), 0),
}

// XIM input style bits (XIM.h).
const (
	ximPreeditNothing = 0x0002
	ximStatusNothing  = 0x0010
)

// XLookupChars status flag (Xutil.h) — Xutf8LookupString sets it when it
// returned text bytes.
const xlookupChars = 0x0002

// ximState holds the open XIM + input context for one X11 window.
type ximState struct {
	im uintptr // XIM
	ic uintptr // XIC
}

// x11Lib gets the XIM-related function pointers appended (lazy, first use).
type ximFuncs struct {
	xOpenIM          func(dpy uintptr, db uintptr, resName *byte, resClass *byte) uintptr
	xCloseIM         func(im uintptr) uintptr
	xCreateIC        func(im uintptr, n1 uintptr, v1 uintptr, n2 uintptr, v2 uintptr, end uintptr) uintptr
	xDestroyIC       func(ic uintptr) uintptr
	xFilterEvent     func(ev *byte, win uintptr) int
	xutf8Lookup      func(ic uintptr, ev *byte, buf *byte, nbytes int, keysym *uintptr, status *int) int
	xSetICFocus      func(ic uintptr) uintptr
	xUnsetICFocus    func(ic uintptr) uintptr
	xSetLocaleMods   func(mods *byte) uintptr
}

var ximFuncsCache *ximFuncs

// loadXIMFuncs resolves XIM symbols once.
func loadXIMFuncs() *ximFuncs {
	if ximFuncsCache != nil {
		return ximFuncsCache
	}
	lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if err != nil {
		return nil
	}
	f := &ximFuncs{}
	purego.RegisterLibFunc(&f.xOpenIM, lib, "XOpenIM")
	purego.RegisterLibFunc(&f.xCloseIM, lib, "XCloseIM")
	purego.RegisterLibFunc(&f.xCreateIC, lib, "XCreateIC")
	purego.RegisterLibFunc(&f.xDestroyIC, lib, "XDestroyIC")
	purego.RegisterLibFunc(&f.xFilterEvent, lib, "XFilterEvent")
	purego.RegisterLibFunc(&f.xutf8Lookup, lib, "Xutf8LookupString")
	purego.RegisterLibFunc(&f.xSetICFocus, lib, "XSetICFocus")
	purego.RegisterLibFunc(&f.xUnsetICFocus, lib, "XUnsetICFocus")
	purego.RegisterLibFunc(&f.xSetLocaleMods, lib, "XSetLocaleModifiers")
	ximFuncsCache = f
	return f
}

// ximOpen creates an XIM input context on dpy/win. Returns nil when no input
// method is available (silent degrade).
func ximOpen(dpy, win uintptr) *ximState {
	f := loadXIMFuncs()
	if f == nil || f.xOpenIM == nil || f.xCreateIC == nil {
		return nil
	}
	// XSetLocaleModifiers("") must be called before XOpenIM; Go does not set
	// a locale by default, so without this XOpenIM fails (returns 0) even
	// when an input method server is running.
	if f.xSetLocaleMods != nil {
		empty := []byte{0}
		f.xSetLocaleMods(&empty[0])
	}
	im := f.xOpenIM(dpy, 0, nil, nil)
	if im == 0 {
		return nil
	}
	// XCreateIC(im, XNInputStyle, PreeditNothing|StatusNothing,
	//            XNClientWindow, win, NULL)
	style := uintptr(ximPreeditNothing | ximStatusNothing)
	ic := f.xCreateIC(im,
		uintptr(unsafe.Pointer(&ximNames.inputStyle[0])), style,
		uintptr(unsafe.Pointer(&ximNames.clientWindow[0])), win,
		0, // NULL terminator
	)
	if ic == 0 {
		f.xCloseIM(im)
		return nil
	}
	return &ximState{im: im, ic: ic}
}

// close tears down the XIM input context.
func (x *ximState) close() {
	if x == nil {
		return
	}
	f := loadXIMFuncs()
	if f == nil {
		return
	}
	if x.ic != 0 && f.xDestroyIC != nil {
		f.xDestroyIC(x.ic)
		x.ic = 0
	}
	if x.im != 0 && f.xCloseIM != nil {
		f.xCloseIM(x.im)
		x.im = 0
	}
}

// filter routes a key event through the IME. Returns (handled, committed):
//   - handled=true  → IME consumed the key (composing); caller must not treat
//     it as a plain key.
//   - handled=false → key is not consumed by the IME; if committed is
//     non-empty it is the finalized text from a finished composition.
func (x *ximState) filter(f *ximFuncs, ev *byte, win uintptr) (handled bool, committed string) {
	if x == nil || x.ic == 0 || f == nil || f.xFilterEvent == nil || f.xutf8Lookup == nil {
		return false, ""
	}
	if f.xFilterEvent(ev, win) != 0 {
		return true, ""
	}
	// Not consumed: try to read finalized UTF-8 text.
	var buf [256]byte
	var keysym uintptr
	var status int
	n := f.xutf8Lookup(x.ic, ev, &buf[0], len(buf), &keysym, &status)
	if n > 0 && status&xlookupChars != 0 {
		return false, string(buf[:n])
	}
	return false, ""
}

// setFocus / unsetFocus activate/deactivate the IME for this window.
func (x *ximState) setFocus(f *ximFuncs) {
	if x == nil || x.ic == 0 || f == nil || f.xSetICFocus == nil {
		return
	}
	f.xSetICFocus(x.ic)
}

func (x *ximState) unsetFocus(f *ximFuncs) {
	if x == nil || x.ic == 0 || f == nil || f.xUnsetICFocus == nil {
		return
	}
	f.xUnsetICFocus(x.ic)
}

// x11Ime adapts the XIM input context to the platform.IME capability.
// SetComposing/Commit are no-ops for XIM in the PreeditNothing/StatusNothing
// mode: the input method owns the candidate popup and the client only
// receives committed text through the event filter.
type x11Ime struct {
	h *x11Host
}

func (im *x11Ime) EnableIME(rect Rect) {
	if im == nil || im.h == nil {
		return
	}
	im.h.ximFocus(true)
}

func (im *x11Ime) SetComposing(text string, cursor int) {
	// XIM: pre-edit is IME-owned (PreeditNothing); nothing to send.
}

func (im *x11Ime) UpdateCursorRect(rect Rect) {
	// XIM PreeditNothing: the input method owns candidate placement; no spot
	// location is sent. (XNSpotLocation would need XIMPreeditPosition.)
}

func (im *x11Ime) SetContentType(purpose ContentPurpose) {
	// XIM has no content-type negotiation; ignored (PreeditNothing mode).
}

func (im *x11Ime) Commit(text string) {
	// XIM: committed text arrives via XFilterEvent/Xutf8LookupString.
}

func (im *x11Ime) DisableIME() {
	if im == nil || im.h == nil {
		return
	}
	im.h.ximFocus(false)
}
