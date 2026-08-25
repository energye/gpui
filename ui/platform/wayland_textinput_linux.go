//go:build linux

package platform

import (
	"fmt"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// tiRecheckDelay re-sends the activation round this long after keyboard
// focus-in. Rationale (observed on mutter 42.9 + ibus): the FIRST
// enable+commit after window show does not activate the IME engine — the
// user cannot switch engines until the window loses and REGAINS focus (that
// working path is a plain disable→enable round-trip). A delayed re-commit
// reproduces that working sequence automatically.
const tiRecheckDelay = 400 * time.Millisecond

// tiDebug logs text-input protocol traffic when GPUI_IME_DEBUG=1 — the
// evidence trail for compositor/IME misbehavior (enable/disable/commit
// rounds, preedit/commit/done events).
func tiDebug(format string, args ...any) {
	if os.Getenv("GPUI_IME_DEBUG") != "1" {
		return
	}
	fmt.Fprintf(os.Stderr, "[ime-wl] "+format+"\n", args...)
}

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
	tiMgrDestroy      = 0 // zwp_text_input_manager_v3: destroy
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
	mgr, ti                                                  []byte
	mDestroy, mGetTI                                         []byte
	mEnable, mDisable                                        []byte
	mSetSurr, mSetCause, mSetContent, mSetRect, mCommit      []byte
	eEnter, eLeave, ePreedit, eCommitStr, eDeleteSurr, eDone []byte
	sEmpty, sN, sO, sS, sU, sI                               []byte
	sNo, sPreedit, sCommit, sSurr, sIiii, sUu                []byte
}{
	mgr:         append([]byte("zwp_text_input_manager_v3"), 0),
	ti:          append([]byte("zwp_text_input_v3"), 0),
	mDestroy:    append([]byte("destroy"), 0),
	mGetTI:      append([]byte("get_text_input"), 0),
	mEnable:     append([]byte("enable"), 0),
	mDisable:    append([]byte("disable"), 0),
	mSetSurr:    append([]byte("set_surrounding_text"), 0),
	mSetCause:   append([]byte("set_text_change_cause"), 0),
	mSetContent: append([]byte("set_content_type"), 0),
	mSetRect:    append([]byte("set_cursor_rectangle"), 0),
	mCommit:     append([]byte("commit"), 0),
	eEnter:      append([]byte("enter"), 0),
	eLeave:      append([]byte("leave"), 0),
	ePreedit:    append([]byte("preedit_string"), 0),
	eCommitStr:  append([]byte("commit_string"), 0),
	eDeleteSurr: append([]byte("delete_surrounding_text"), 0),
	eDone:       append([]byte("done"), 0),
	sEmpty:      append([]byte(""), 0),
	sN:          append([]byte("n"), 0),
	sO:          append([]byte("o"), 0),
	sS:          append([]byte("s"), 0),
	sU:          append([]byte("u"), 0),
	sI:          append([]byte("i"), 0),
	sNo:         append([]byte("no"), 0),
	// preedit_string/commit_string text is allow-null in the protocol:
	// compositors send NULL to clear the pre-edit. libwayland signature
	// uses "?" for nullable — without it the event is dropped with
	// "NULL string received on non-nullable type" and the editor's
	// compose state never clears.
	sPreedit: append([]byte("?sii"), 0), // preedit_string: text,?s cursor_begin,int cursor_end,int
	sCommit:  append([]byte("?s"), 0),   // commit_string: text,?s
	sSurr:    append([]byte("sii"), 0),  // set_surrounding_text: text,s cursor,int anchor,int
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
	msgTi[tiSetSurroundingText] = wlMessageC{Name: cstr(tiNames.mSetSurr), Signature: cstr(tiNames.sSurr), Types: 0}
	msgTi[tiSetTextChangeCause] = wlMessageC{Name: cstr(tiNames.mSetCause), Signature: cstr(tiNames.sU), Types: 0}
	msgTi[tiSetContentType] = wlMessageC{Name: cstr(tiNames.mSetContent), Signature: cstr(tiNames.sUu), Types: 0}
	msgTi[tiSetCursorRectangle] = wlMessageC{Name: cstr(tiNames.mSetRect), Signature: cstr(tiNames.sIiii), Types: 0}
	msgTi[tiCommit] = wlMessageC{Name: cstr(tiNames.mCommit), Signature: cstr(tiNames.sEmpty), Types: 0}

	// zwp_text_input_v3 events.
	msgTiEv[tiEvEnter] = wlMessageC{Name: cstr(tiNames.eEnter), Signature: cstr(tiNames.sO), Types: uintptr(unsafe.Pointer(&typesTiSurf[0]))}
	// leave carries a wl_surface arg per protocol ("o"), same as enter.
	msgTiEv[tiEvLeave] = wlMessageC{Name: cstr(tiNames.eLeave), Signature: cstr(tiNames.sO), Types: uintptr(unsafe.Pointer(&typesTiSurf[0]))}
	msgTiEv[tiEvPreeditString] = wlMessageC{Name: cstr(tiNames.ePreedit), Signature: cstr(tiNames.sPreedit), Types: 0}
	msgTiEv[tiEvCommitString] = wlMessageC{Name: cstr(tiNames.eCommitStr), Signature: cstr(tiNames.sCommit), Types: 0}
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
	// rect is the last cursor rectangle sent (logical px); resent by
	// refreshTextInput because a commit dropped pre-focus also drops the
	// pending set_cursor_rectangle — without this, candidate windows anchor
	// at the surface origin after keyboard focus-in.
	rect    Rect
	hasRect bool
	// purpose is the content type sent with every state commit
	// (set_content_type(hint=None, purpose)); defaults to PurposeNormal.
	purpose ContentPurpose

	// recheckMu guards the delayed activation round (see refreshTextInput).
	recheckMu    sync.Mutex
	recheckTimer *time.Timer // the pending delayed activation round

	// queue: outbound actions pending the next commit (design D7).
	queue tiPendingQueue
}

// cancelRecheck drops the pending delayed activation round. Called when the
// engine shows signs of life (real preedit/commit traffic): the fallback
// round exists only for the dropped-first-activation case, and firing it
// mid-composition would interrupt the IME.
func (st *wlTIState) cancelRecheck() {
	st.recheckMu.Lock()
	if st.recheckTimer != nil {
		st.recheckTimer.Stop()
		st.recheckTimer = nil
	}
	st.recheckMu.Unlock()
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

// EnableIME opens a session: queues enable + anchor + purpose, then
// flush-commits atomically (design D7: one commit per logical action).
func (im *wlIme) EnableIME(rect Rect) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	st := im.h.win.ti
	if st.ti == 0 || st.win.surface == 0 {
		return
	}
	st.rect, st.hasRect = rect, true
	tiDebug("enable rect=%v", rect)
	st.queue.push(tiPendingAction{kind: tiPendingEnable})
	st.queue.push(tiPendingAction{kind: tiPendingRect, rect: rect})
	st.queue.push(tiPendingAction{kind: tiPendingContent, ct: ContentType{Purpose: st.purpose}})
	st.flushCommit()
}

// SetContentType stores the editing purpose and publishes it immediately.
func (im *wlIme) SetContentType(purpose ContentPurpose) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	st := im.h.win.ti
	if st.ti == 0 || st.win.surface == 0 {
		st.purpose = purpose
		return
	}
	st.purpose = purpose
	st.queue.push(tiPendingAction{kind: tiPendingContent, ct: ContentType{Purpose: purpose}})
	st.flushCommit()
}

// UpdateCursorRect moves the IME anchor to the current caret position.
// Identical consecutive rects are SKIPPED: every commit_state makes the
// compositor re-send the current preedit, so a redundant push feeds a
// protocol echo loop (dozens of duplicate preedit events per keystroke).
func (im *wlIme) UpdateCursorRect(rect Rect) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	st := im.h.win.ti
	if st.ti == 0 {
		return
	}
	if st.hasRect && rect == st.rect {
		st.cancelRecheck() // engine is alive; the fallback round would only disturb it
		return
	}
	st.rect, st.hasRect = rect, true
	tiDebug("cursor-rect %v", rect)
	st.queue.push(tiPendingAction{kind: tiPendingRect, rect: rect})
	st.flushCommit()
}

// sendCursorRect queues the anchor update and flushes (dispatch thread).
func (st *wlTIState) sendCursorRect(rect Rect) {
	st.queue.push(tiPendingAction{kind: tiPendingRect, rect: rect})
	st.flushCommit()
}

// SetComposing reports the surrounding text and caret for IME editing (D2:
// opt-in, local-change-driven only — the router gates this).
func (im *wlIme) SetComposing(text string, cursor int) {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	st := im.h.win.ti
	if st.ti == 0 {
		return
	}
	tiDebug("surrounding %d bytes", len(text))
	st.queue.push(tiPendingAction{kind: tiPendingSurf, text: text, cur: cursor})
	st.flushCommit()
}

// Commit sends the surrounding text with an input-method change cause.
func (im *wlIme) Commit(text string) {
	im.SetComposing(text, len(text))
}

// DisableIME ends the session atomically.
func (im *wlIme) DisableIME() {
	if im == nil || im.h == nil || im.h.win == nil || im.h.win.ti == nil {
		return
	}
	if im.h.win.ti.ti == 0 {
		return
	}
	im.h.win.ti.queue.push(tiPendingAction{kind: tiPendingDisable})
	im.h.win.ti.flushCommit()
	tiDebug("disable round sent")
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

// refreshTextInput re-sends the text-input state (enable + purpose + anchor
// + commit) so the compositor activates the input method NOW that the
// surface holds keyboard focus. mutter 42.9 drops a commit received while
// text_input->surface is NULL (before focus); re-committing on keyboard
// enter is the GTK/Flutter focus-in refresh pattern.
func (w *wlWin) refreshTextInput() {
	if w == nil || w.ti == nil || w.ti.ti == 0 || w.surface == 0 || w.lib == nil {
		return
	}
	tiDebug("keyboard focus-in → refresh (enable+rect+commit)")
	st := w.ti
	st.queue.push(tiPendingAction{kind: tiPendingEnable})
	st.queue.push(tiPendingAction{kind: tiPendingContent, ct: ContentType{Purpose: st.purpose}})
	// Re-send the cursor rectangle: the initial round (sent before keyboard
	// focus) was dropped by mutter along with its pending
	// set_cursor_rectangle — without this the candidate window anchors at
	// the surface origin after focus-in.
	if st.hasRect {
		st.queue.push(tiPendingAction{kind: tiPendingRect, rect: st.rect})
	}
	st.flushCommit()

	// Delayed activation re-check: the first round after window show is
	// dropped by the IME engine (mutter+ibus); only a disable→enable cycle
	// AFTER the engine attached activates switching. Reproduce it once,
	// 400ms out — as a POSTED action, never marshaled from the timer
	// goroutine (design §4.0 C2). A newer round cancels the older timer.
	old := st.recheckTimer
	var tm *time.Timer
	tm = time.AfterFunc(tiRecheckDelay, func() {
		st.recheckMu.Lock()
		active := st.recheckTimer == tm && st.ti != 0 && st.win.surface != 0
		st.recheckTimer = nil
		st.recheckMu.Unlock()
		if !active {
			return // a newer round superseded this one
		}
		tiDebug("delayed re-activation posted (disable+enable+commit)")
		// C2: queue the actions here (thread-safe), then ask the event loop
		// to marshal + flush them on the dispatch thread.
		st.queue.push(tiPendingAction{kind: tiPendingDisable})
		st.queue.push(tiPendingAction{kind: tiPendingEnable})
		if st.hasRect {
			st.queue.push(tiPendingAction{kind: tiPendingRect, rect: st.rect})
		}
		st.queue.push(tiPendingAction{kind: tiPendingContent, ct: ContentType{Purpose: st.purpose}})
		st.postFlush()
	})
	st.recheckTimer = tm
	if old != nil {
		old.Stop()
	}
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

// wlTiEnter: enter(surface) — the text-input is now active on this surface
// (compositor granted IME focus). No pre-edit content yet; the editor's
// compose session starts on the first preedit_string event. Do NOT push a
// fake compose event here — an empty compose would begin (and could end) a
// pre-edit session with no real text.
//
// This event is also the compositor's explicit "text-input activation" ack:
// mutter 42.9 commit_state only calls clutter_input_method_focus_in (which
// makes the IME engine handle Shift-switch etc.) when focus is NOT yet
// focused; the keyboard-enter refresh alone lands too early (focus already
// set → enable_panel only). Re-commit here so the engine actually focuses.
func wlTiEnter(data, ti, surface uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.refreshTextInput()
}

// wlTiLeave: leave(surface) — IME focus left this surface; cancel any active
// composition. Also do not fabricate events; the editor cancels on its own
// commit/delete lifecycle, and an empty compose event would corrupt it.
//
// The compositor deactivates the text-input here; send disable+commit so the
// engine-side focus is dropped cleanly (mirrors the working manual
// blur→refocus cycle that re-arms engine switching).
func wlTiLeave(data, ti, surface uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	w := st.win
	if w.ti != nil && w.lib != nil && w.surface != 0 && w.ti.ti != 0 {
		args := []wlArg{argO(w.surface)}
		w.lib.proxyMarshalArrayFlags(w.ti.ti, tiDisable, 0, 0, 0, &args[0])
		w.lib.proxyMarshalArrayFlags(w.ti.ti, tiCommit, 0, 0, 0, nil)
		w.lib.displayFlush(w.display)
		tiDebug("leave → disable+commit")
	}
}

// wlTiPreedit handles preedit_string(text, commit, index): the compositor's
// current pre-edit string plus the caret byte offset within it. Surfaced as
// IMEKind 0 (compose) with Start=End=index (-1 = end of the pre-edit); the
// protocol commit boolean is advisory only — a real commit always arrives as
// commit_string — so it is not forwarded.
func wlTiPreedit(data, ti, text, commit, index uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	s := ""
	if text != 0 {
		s = goString(text)
	}
	tiDebug("event preedit %q index=%d", s, int(int32(index)))
	// Real engine output: the delayed fallback round is no longer needed —
	// and firing it now would interrupt this composition.
	if st.win.ti != nil {
		st.win.ti.cancelRecheck()
	}
	st.win.pushIME(Event{
		Type:    EventIME,
		IMEKind: 0, // compose
		IMEText: s,
		// Caret byte offset within the pre-edit (begin = end; -1 = end).
		IMEStart: int(int32(index)),
		IMEEnd:   int(int32(index)),
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
	tiDebug("event commit %q", s)
	st.win.pushIME(Event{Type: EventIME, IMEKind: 1, IMEText: s})
}

// wlTiDeleteSurr handles delete_surrounding_text(before, after): the IME asks
// to remove text around the cursor. Surfaced as IMEKind 3
// (delete-surrounding) with Start=-before / End=after (byte counts relative
// to the caret); the textinput editor performs the deletion.
func wlTiDeleteSurr(data, ti, before, after uintptr) {
	st := tiFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.win.pushIME(Event{
		Type:     EventIME,
		IMEKind:  3, // delete-surrounding (input.IMEDeleteSurrounding)
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
	st.recheckMu.Lock()
	if st.recheckTimer != nil {
		st.recheckTimer.Stop()
		st.recheckTimer = nil
	}
	st.recheckMu.Unlock()
	if st.ti != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.ti)
		st.ti = 0
	}
	if st.mgr != 0 && st.lib != nil {
		st.lib.proxyDestroy(st.mgr)
		st.mgr = 0
	}
}
