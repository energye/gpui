//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// zwp_text_input_v3 (stable text-input protocol) binding via purego.
//
// Protocol objects (protocol/stable/text-input/text-input-v3.xml):
//
//	zwp_text_input_manager_v3 (v1)
//	  requests: destroy(0), create_text_input(1)[new_id]
//
//	zwp_text_input_v3 (v1)
//	  requests: destroy(0), enable(1)[surface], disable(2)[surface],
//	            set_surrounding_text(3)[text,cursor,anchor],
//	            set_text_change_cause(4)[cause], set_content_type(5)[hint,purpose],
//	            set_cursor_rectangle(6)[x,y,w,h], commit(7)
//	  events:   enter(0)[surface], leave(1),
//	            preedit_string(2)[text,commit,index],
//	            commit_string(3)[text],
//	            delete_surrounding_text(4)[before,after], done(5)[serial]
//
// The Wayland text-input protocol is the IME channel on Wayland: the
// compositor (GNOME/KDE + IBus/fcitx) drives preedit_string/commit_string
// events; the client enables/disables and reports the surrounding text and
// cursor rect. This binding translates those events into platform.Event
// {Type: EventIME} that ui/input normalizes and InputRouter routes to the
// focused textinput.Editor (plan §6).
//
// It is an OPTIONAL capability: if the compositor does not advertise
// zwp_text_input_manager_v3, wlHost.IME() stays nil and the app degrades to
// plain keyboard text (silent fallback, same as the VSyncWaiter pattern).

// text-input opcodes (zwp_text_input_v3 / manager v1, protocol order).
// Manager requests: destroy(0), get_text_input(1)[new_id, seat].
const (
	tiMgrDestroy     = 0 // zwp_text_input_manager_v3: destroy
	tiMgrGetTextInput = 1 // zwp_text_input_manager_v3: get_text_input(id, seat)

	tiDestroy            = 0 // zwp_text_input_v3: destroy
	tiEnable             = 1
	tiDisable            = 2
	tiSetSurroundingText = 3
	tiSetTextChangeCause = 4
	tiSetContentType     = 5
	tiSetCursorRectangle = 6
	tiCommit             = 7

	tiEvEnter             = 0 // zwp_text_input_v3 events
	tiEvLeave             = 1
	tiEvPreeditString     = 2
	tiEvCommitString      = 3
	tiEvDeleteSurrounding = 4
	tiEvDone              = 5
)

// text-input cause / content-hint constants (subset used by clients).
const (
	tiCauseInputMethod = 5 // cause from input method (zwp_text_input_v3.change_cause)
	tiHintNone         = 0
	tiPurposeNormal    = 0
)

var tiNames = struct {
	mgr, ti          []byte
	mDestroy, mGetTI []byte
	mEnable, mDisable []byte
	mSetSurr, mSetCause, mSetContent, mSetRect, mCommit []byte
	eEnter, eLeave, ePreedit, eCommitStr, eDeleteSurr, eDone []byte
	sEmpty, sN, sO, sS, sU, sI []byte
	sNo, sSui, sSuu, sIiii, sUu []byte
}{
	mgr:     append([]byte("zwp_text_input_manager_v3"), 0),
	ti:      append([]byte("zwp_text_input_v3"), 0),
	mDestroy: append([]byte("destroy"), 0),
	mGetTI:   append([]byte("get_text_input"), 0),
	mEnable:  append([]byte("enable"), 0),
	mDisable: append([]byte("disable"), 0),
	mSetSurr: append([]byte("set_surrounding_text"), 0),
	mSetCause: append([]byte("set_text_change_cause"), 0),
	mSetContent: append([]byte("set_content_type"), 0),
	mSetRect: append([]byte("set_cursor_rectangle"), 0),
	mCommit:  append([]byte("commit"), 0),
	eEnter:   append([]byte("enter"), 0),
	eLeave:   append([]byte("leave"), 0),
	ePreedit: append([]byte("preedit_string"), 0),
	eCommitStr: append([]byte("commit_string"), 0),
	eDeleteSurr: append([]byte("delete_surrounding_text"), 0),
	eDone:    append([]byte("done"), 0),
	sEmpty:   append([]byte(""), 0),
	sN:       append([]byte("n"), 0),
	sO:       append([]byte("o"), 0),
	sS:       append([]byte("s"), 0),
	sU:       append([]byte("u"), 0),
	sI:       append([]byte("i"), 0),
	sNo:      append([]byte("no"), 0),
	sSui:     append([]byte("sui"), 0),
	sSuu:     append([]byte("suu"), 0),
	sIiii:    append([]byte("iiii"), 0),
	sUu:      append([]byte("uu"), 0),
}

var (
	ifaceTiMgr wlInterfaceC
	ifaceTi    wlInterfaceC

	msgTiMgr [2]wlMessageC
	msgTi    [8]wlMessageC
	msgTiEv  [6]wlMessageC

	// types arrays: get_text_input "no" → [ifaceTi, ifaceSeat]; enable/
	// disable "o" → [ifaceSurface]. Indexes patched after interface
	// addresses are known (same pattern as xdg typesTop).
	typesTiMgr  [2]uintptr
	typesTiSurf [1]uintptr
)

// initTIInterfaces fills the in-process interface tables for the text-input
// protocol. Must be called once per process (after loadWayland provides
// ifaceSurface and ifaceSeat).
func initTIInterfaces(ifaceSurface, ifaceSeat uintptr) {
	// zwp_text_input_manager_v3: destroy(""), get_text_input("no").
	msgTiMgr[tiMgrDestroy] = wlMessageC{Name: cstr(tiNames.mDestroy), Signature: cstr(tiNames.sEmpty), Types: 0}
	msgTiMgr[tiMgrGetTextInput] = wlMessageC{Name: cstr(tiNames.mGetTI), Signature: cstr(tiNames.sNo), Types: uintptr(unsafe.Pointer(&typesTiMgr[0]))}
	// Patch typesTiMgr after ifaceTi / ifaceSeat are known — done below.

	// zwp_text_input_v3 requests.
	msgTi[tiDestroy] = wlMessageC{Name: cstr(tiNames.mDestroy), Signature: cstr(tiNames.sEmpty), Types: 0}
	msgTi[tiEnable] = wlMessageC{Name: cstr(tiNames.mEnable), Signature: cstr(tiNames.sO), Types: uintptr(unsafe.Pointer(&typesTiSurf[0]))}
	msgTi[tiDisable] = wlMessageC{Name: cstr(tiNames.mDisable), Signature: cstr(tiNames.sO), Types: uintptr(unsafe.Pointer(&typesTiSurf[0]))}
	msgTi[tiSetSurroundingText] = wlMessageC{Name: cstr(tiNames.mSetSurr), Signature: cstr(tiNames.sSuu), Types: 0}
	msgTi[tiSetTextChangeCause] = wlMessageC{Name: cstr(tiNames.mSetCause), Signature: cstr(tiNames.sU), Types: 0}
	msgTi[tiSetContentType] = wlMessageC{Name: cstr(tiNames.mSetContent), Signature: cstr(tiNames.sUu), Types: 0}
	msgTi[tiSetCursorRectangle] = wlMessageC{Name: cstr(tiNames.mSetRect), Signature: cstr(tiNames.sIiii), Types: 0}
	msgTi[tiCommit] = wlMessageC{Name: cstr(tiNames.mCommit), Signature: cstr(tiNames.sEmpty), Types: 0}

	// zwp_text_input_v3 events.
	msgTiEv[tiEvEnter] = wlMessageC{Name: cstr(tiNames.eEnter), Signature: cstr(tiNames.sO), Types: uintptr(unsafe.Pointer(&typesTiSurf[0]))}
	msgTiEv[tiEvLeave] = wlMessageC{Name: cstr(tiNames.eLeave), Signature: cstr(tiNames.sEmpty), Types: 0}
	msgTiEv[tiEvPreeditString] = wlMessageC{Name: cstr(tiNames.ePreedit), Signature: cstr(tiNames.sSui), Types: 0}
	msgTiEv[tiEvCommitString] = wlMessageC{Name: cstr(tiNames.eCommitStr), Signature: cstr(tiNames.sS), Types: 0}
	msgTiEv[tiEvDeleteSurrounding] = wlMessageC{Name: cstr(tiNames.eDeleteSurr), Signature: cstr(tiNames.sUu), Types: 0}
	msgTiEv[tiEvDone] = wlMessageC{Name: cstr(tiNames.eDone), Signature: cstr(tiNames.sU), Types: 0}

	ifaceTiMgr = wlInterfaceC{
		Name: cstr(tiNames.mgr), Version: 1,
		MethodCount: 2, Methods: uintptr(unsafe.Pointer(&msgTiMgr[0])),
		EventCount: 0, Events: 0,
	}
	ifaceTi = wlInterfaceC{
		Name: cstr(tiNames.ti), Version: 1,
		MethodCount: 8, Methods: uintptr(unsafe.Pointer(&msgTi[0])),
		EventCount: 6, Events: uintptr(unsafe.Pointer(&msgTiEv[0])),
	}
	typesTiMgr[0] = uintptr(unsafe.Pointer(&ifaceTi))
	typesTiMgr[1] = ifaceSeat
	typesTiSurf[0] = ifaceSurface
}

// wlTIState holds the bound text-input objects for one wlWin.
type wlTIState struct {
	lib      *wlLib
	win      *wlWin
	mgr      uintptr // zwp_text_input_manager_v3 proxy
	ti       uintptr // zwp_text_input_v3 proxy
	listener [6]uintptr
	selfPtr  uintptr // *wlTIState for callbacks
}

// bindTextInput binds zwp_text_input_manager_v3 (if advertised) and creates
// a zwp_text_input_v3 for the surface's seat. Returns nil when the compositor
// does not support the protocol (silent degrade).
func (w *wlWin) bindTextInput() *wlTIState {
	if w == nil || w.lib == nil || w.tiMgrName == 0 || w.seat == 0 {
		return nil
	}
	st := &wlTIState{lib: w.lib, win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// Bind the manager from the registry.
	st.mgr = w.bind(w.registry, w.tiMgrName, uintptr(unsafe.Pointer(&ifaceTiMgr)), 1)
	if st.mgr == 0 {
		return nil
	}
	// get_text_input(new_id zwp_text_input_v3, seat).
	args := []wlArg{argNewID(), argO(w.seat)}
	st.ti = w.lib.proxyMarshalArrayCtor(st.mgr, tiMgrGetTextInput, &args[0],
		uintptr(unsafe.Pointer(&ifaceTi)), 1)
	if st.ti == 0 {
		return nil
	}
	// Listener: enter, leave, preedit_string, commit_string,
	// delete_surrounding_text, done.
	st.listener[tiEvEnter] = purego.NewCallback(wlTiEnter)
	st.listener[tiEvLeave] = purego.NewCallback(wlTiLeave)
	st.listener[tiEvPreeditString] = purego.NewCallback(wlTiPreedit)
	st.listener[tiEvCommitString] = purego.NewCallback(wlTiCommit)
	st.listener[tiEvDeleteSurrounding] = purego.NewCallback(wlTiDeleteSurr)
	st.listener[tiEvDone] = purego.NewCallback(wlTiDone)
	if w.lib.proxyAddListener(st.ti, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
		// Listener attach failed — drop the object.
		w.lib.proxyDestroy(st.ti)
		return nil
	}
	return st
}

// --- IME implementation on wlHost ---

// wlIme adapts the zwp_text_input_v3 proxy to the platform.IME capability.
// Events arrive via the callbacks below (translated into platform.Event in
// wlHost.poll through the pending IME queue); the command side
// (Enable/SetComposing/Commit/Disable) drives the wire protocol.
type wlIme struct {
	h *wlHost
}

// EnableIME enables the text input on this surface and reports the caret
// rectangle (logical px, Y-down; converted to compositor coordinates as-is
// since we treat the surface origin as top-left).
func (im *wlIme) EnableIME(rect Rect) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	lib, ti, win := im.h.win.ti.lib, im.h.win.ti.ti, im.h.win.ti.win
	if ti == 0 || win == nil || win.surface == 0 {
		return
	}
	// enable(surface)
	args := []wlArg{argO(win.surface)}
	lib.proxyMarshalArrayFlags(ti, tiEnable, 0, 0, 0, &args[0])
	// set_cursor_rectangle(x,y,w,h)
	rectArgs := []wlArg{argU(uint32(int32(rect.X))), argU(uint32(int32(rect.Y))), argU(uint32(int32(rect.W))), argU(uint32(int32(rect.H)))}
	lib.proxyMarshalArrayFlags(ti, tiSetCursorRectangle, 0, 0, 0, &rectArgs[0])
	// set_content_type(hint, purpose)
	ct := []wlArg{argU(tiHintNone), argU(tiPurposeNormal)}
	lib.proxyMarshalArrayFlags(ti, tiSetContentType, 0, 0, 0, &ct[0])
	// commit
	lib.proxyMarshalArrayFlags(ti, tiCommit, 0, 0, 0, nil)
	lib.displayFlush(win.display)
}

// SetComposing reports the surrounding text and caret for IME editing. The
// Wayland protocol expects the FULL surrounding text; the pre-edit is part
// of it. cursor/anchor are byte offsets. We pass the current buffer with the
// caret at the end (callers keep composition state in textinput.Editor).
func (im *wlIme) SetComposing(text string, cursor int) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	lib, ti := im.h.win.ti.lib, im.h.win.ti.ti
	if ti == 0 {
		return
	}
	tb := append([]byte(text), 0)
	args := []wlArg{argS(cstr(tb)), argU(uint32(cursor)), argU(uint32(cursor))}
	lib.proxyMarshalArrayFlags(ti, tiSetSurroundingText, 0, 0, 0, &args[0])
	lib.proxyMarshalArrayFlags(ti, tiCommit, 0, 0, 0, nil)
	lib.displayFlush(im.h.win.display)
}

// Commit sends the surrounding text with an input-method change cause.
func (im *wlIme) Commit(text string) {
	im.SetComposing(text, len(text))
}

// DisableIME disables the text input on this surface.
func (im *wlIme) DisableIME() {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	lib, ti, win := im.h.win.ti.lib, im.h.win.ti.ti, im.h.win.ti.win
	if ti == 0 || win == nil || win.surface == 0 {
		return
	}
	args := []wlArg{argO(win.surface)}
	lib.proxyMarshalArrayFlags(ti, tiDisable, 0, 0, 0, &args[0])
	lib.proxyMarshalArrayFlags(ti, tiCommit, 0, 0, 0, nil)
	lib.displayFlush(win.display)
}

// pushIME queues an IME event for the next poll (thread-safe: called from
// Wayland dispatch callbacks, drained by wlHost.poll).
func (w *wlWin) pushIME(ev Event) {
	if w == nil {
		return
	}
	w.imeMu.Lock()
	w.imeEvents = append(w.imeEvents, ev)
	w.imeMu.Unlock()
}

// pushKey queues a keyboard event for the next poll (thread-safe).
func (w *wlWin) pushKey(ev Event) {
	if w == nil {
		return
	}
	w.keyMu.Lock()
	w.keyEvents = append(w.keyEvents, ev)
	w.keyMu.Unlock()
}

// --- event callbacks → platform.Event queue ---

// tiFrom maps the callback userdata (*wlTIState) back to the host.
func tiFrom(data uintptr) *wlTIState {
	if data == 0 {
		return nil
	}
	return (*wlTIState)(unsafe.Pointer(data))
}

func wlTiEnter(data, ti, surface uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: ""})
}

func wlTiLeave(data, ti uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	// Leave: cancel any active composition.
	st.win.pushIME(Event{Type: EventIME, IMEKind: 0, IMEText: ""})
}

// wlTiPreedit handles preedit_string(text, commit, index).
// commit==1 → the pre-edit should be treated as final (commit_string follows
// in the same done round); index is the caret byte offset in the pre-edit
// (-1 = end). We surface it as IMEKind 0 (compose) with the text; the
// textinput editor manages the pre-edit lifecycle.
func wlTiPreedit(data, ti, text, commit, index uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	s := ""
	if text != 0 {
		s = goString(text)
	}
	st.win.pushIME(Event{
		Type:   EventIME,
		IMEKind: 0, // compose
		IMEText: s,
		IMEStart: int(int32(index)),
		IMEEnd:   int(int32(commit)),
	})
}

// wlTiCommit handles commit_string(text): final committed text.
func wlTiCommit(data, ti, text uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	s := ""
	if text != 0 {
		s = goString(text)
	}
	st.win.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: s})
}

// wlTiDeleteSurr handles delete_surrounding_text(before, after): the IME asks
// to remove text around the cursor. We surface it as a commit with a special
// marker (negative start = delete before, end = after). The textinput editor
// interprets Start<0 as a delete request.
func wlTiDeleteSurr(data, ti, before, after uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.pushIME(Event{
		Type:   EventIME,
		IMEKind: 2, // caret/delete marker
		IMEStart: -int(int32(before)),
		IMEEnd:   int(int32(after)),
	})
}

// wlTiDone marks the end of a protocol round (serial). We do not need it for
// the editor (events are applied as they arrive); retained for protocol
// completeness.
func wlTiDone(data, ti, serial uintptr) {}

// destroy tears down the text-input objects.
func (st *wlTIState) destroy() {
	if st == nil {
		return
	}
	if st.ti != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.ti)
		st.ti = 0
	}
	if st.mgr != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.mgr)
		st.mgr = 0
	}
}
