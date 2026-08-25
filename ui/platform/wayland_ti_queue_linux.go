//go:build linux

package platform

import "sync"

// zwp_text_input_v3 pending-state queue (design D7 / §5.1).
//
// The protocol splits every request into two phases: enable /
// set_surrounding_text / set_content_type / set_cursor_rectangle /
// delete_surrounding_text only UPDATE pending state; the commit request
// atomically swaps pending → current on the compositor side. Symmetrically,
// preedit_string / commit_string / delete_surrounding_text EVENTS are
// pending until the done(serial) event commits the server round — events
// of one round must be applied together, in order.
//
// Thread model (§4.0): all queue mutations happen on the dispatch thread
// (wl callbacks run there; poll() drains). The timer-goroutine recheck
// path (C2) posts a synthetic event instead of touching this state.

// tiPendingKind classifies a queued outbound action.
type tiPendingKind uint8

const (
	tiPendingEnable  tiPendingKind = iota
	tiPendingDisable
	tiPendingRect    // set_cursor_rectangle + payload rect
	tiPendingContent // set_content_type + payload purpose
	tiPendingSurf    // surrounding text push + payload text/cursor
)

type tiPendingAction struct {
	kind tiPendingKind
	rect Rect
	ct   ContentType
	text string
	cur  int
}

// tiPendingQueue accumulates outbound actions between commits. Owned by
// the dispatch thread; mu only guards the flush-against-close race.
type tiPendingQueue struct {
	mu   sync.Mutex
	acts []tiPendingAction
}

func (q *tiPendingQueue) push(a tiPendingAction) {
	if q == nil {
		return
	}
	q.mu.Lock()
	q.acts = append(q.acts, a)
	q.mu.Unlock()
}

// drain returns all queued actions and empties the queue.
func (q *tiPendingQueue) drain() []tiPendingAction {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	acts := q.acts
	q.acts = nil
	q.mu.Unlock()
	return acts
}

// marshalPending applies one queued action to the wire (dispatch thread
// only). Returns false when the object is gone.
func (st *wlTIState) marshalPending(a tiPendingAction) bool {
	if st.lib == nil || st.ti == 0 || st.win == nil || st.win.surface == 0 {
		return false
	}
	lib, ti := st.lib, st.ti
	switch a.kind {
	case tiPendingEnable:
		args := []wlArg{argO(st.win.surface)}
		lib.proxyMarshalArrayFlags(ti, tiEnable, 0, 0, 0, &args[0])
	case tiPendingDisable:
		args := []wlArg{argO(st.win.surface)}
		lib.proxyMarshalArrayFlags(ti, tiDisable, 0, 0, 0, &args[0])
	case tiPendingRect:
		r := a.rect
		args := []wlArg{argU(uint32(int32(r.X))), argU(uint32(int32(r.Y))), argU(uint32(int32(r.W))), argU(uint32(int32(r.H)))}
		lib.proxyMarshalArrayFlags(ti, tiSetCursorRectangle, 0, 0, 0, &args[0])
	case tiPendingContent:
		args := []wlArg{argU(tiHintNone), argU(uint32(a.ct.Purpose))}
		lib.proxyMarshalArrayFlags(ti, tiSetContentType, 0, 0, 0, &args[0])
	case tiPendingSurf:
		tb := append([]byte(a.text), 0)
		args := []wlArg{argS(cstr(tb)), argU(uint32(int32(a.cur))), argU(uint32(int32(a.cur)))}
		lib.proxyMarshalArrayFlags(ti, tiSetSurroundingText, 0, 0, 0, &args[0])
	}
	return true
}

// flushCommit drains the queue, marshals everything, then sends commit —
// the atomic state swap. Called from EnableIME/CaretMoved/etc. (all on the
// event-loop thread per C1/C2) and from poll() for posted actions.
func (st *wlTIState) flushCommit() {
	for _, a := range st.queue.drain() {
		if !st.marshalPending(a) {
			return
		}
	}
	if st.lib != nil && st.ti != 0 && st.win != nil {
		st.lib.proxyMarshalArrayFlags(st.ti, tiCommit, 0, 0, 0, nil)
		st.lib.displayFlush(st.win.display)
		tiDebug("flush+%v actions committed", "commit")
	}
}

// postFlush asks the event loop to run flushCommit on the dispatch thread
// (C2: async producers must not touch the wire themselves).
func (st *wlTIState) postFlush() {
	if h := st.win.hostForWake(); h != nil {
		h.postImeFlush(st)
		h.WakeUp()
	}
}
