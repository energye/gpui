//go:build linux

package platform

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// X11 XDND drag-and-drop target (S6-P1 file-first).
//
// Implements the target side of the XDND protocol version 5:
//   - advertise XdndAware=5 so sources send Enter/Position/Leave/Drop
//   - XdndEnter carries up to 3 type atoms inline, more via XdndTypeList
//   - XdndPosition carries root x/y packed as (x<<16)|y + timestamp/action
//   - reply XdndStatus (accept only text/uri-list, want position)
//   - XdndLeave clears the drag, XdndDrop triggers XConvertSelection for
//     text/uri-list and the SelectionNotify carries the file list
//
// Source role is two-fold: the test double (two-window対拖 writes
// dndSrcData directly) and the production path below (StartDragTo fills
// the same slots from a DragOffer, owns XdndSelection, and sends
// Enter/Position/Drop to the target; SelectionRequest serves both).

const xdndVersion = 5

var errXdndNoLib = errXdndLib{}

type errXdndLib struct{}

func (errXdndLib) Error() string { return "x11: xdnd lib not ready" }

var (
	xdndAtomOnce    sync.Once
	xdndAtomLib     uintptr
	xdndGetAtomName func(dpy uintptr, atom uintptr) *byte
	xdndFree        func(ptr unsafe.Pointer) int
)

func xdndAtomReady() bool {
	xdndAtomOnce.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		}
		if err != nil {
			return
		}
		if _, err := purego.Dlsym(lib, "XGetAtomName"); err != nil {
			return
		}
		if _, err := purego.Dlsym(lib, "XFree"); err != nil {
			return
		}
		purego.RegisterLibFunc(&xdndGetAtomName, lib, "XGetAtomName")
		purego.RegisterLibFunc(&xdndFree, lib, "XFree")
		xdndAtomLib = lib
	})
	return xdndAtomLib != 0 && xdndGetAtomName != nil
}

// x11DndAdvertise publishes XdndAware=5 on the window. Best-effort: failure
// leaves the window non-droppable (sources skip us) but never fails Create.
func x11DndAdvertise(st *x11State) {
	if st == nil || st.display == 0 || st.window == 0 || st.atXdndAware == 0 {
		return
	}
	lib := ctlLib.open()
	if !lib.ok() || lib.changeProperty == nil || lib.internAtom == nil {
		return
	}
	typeName := append([]byte("ATOM"), 0)
	typeAtom := lib.internAtom(st.display, &typeName[0], 0)
	if typeAtom == 0 {
		typeAtom = st.atXdndAware
	}
	var v int64 = xdndVersion
	lib.changeProperty(st.display, st.window, st.atXdndAware, typeAtom, 32, 0,
		unsafe.Pointer(&v), 1)
	if st.flush != nil {
		st.flush()
	}
}

// x11DndAtomName converts an atom to its string (e.g. text/uri-list).
// Empty on failure; never panics without a display.
func x11DndAtomName(dpy uintptr, atom uintptr) string {
	if dpy == 0 || atom == 0 || !xdndAtomReady() {
		return ""
	}
	p := xdndGetAtomName(dpy, atom)
	if p == nil {
		return ""
	}
	s := goString(uintptr(unsafe.Pointer(p)))
	if xdndFree != nil {
		xdndFree(unsafe.Pointer(p))
	}
	return s
}

// x11DndReadTypeList reads the source's XdndTypeList property (atoms, 32-bit).
func x11DndReadTypeList(st *x11State, source uintptr) []uintptr {
	if st == nil || st.display == 0 || source == 0 || st.atXdndTypeList == 0 {
		return nil
	}
	lib := clipLib.open()
	if !lib.ok() || lib.getWindowProperty == nil {
		return nil
	}
	var (
		actualType   uintptr
		actualFormat int
		nitems       uint64
		bytesAfter   uint64
		data         *byte
	)
	if lib.getWindowProperty(st.display, source, st.atXdndTypeList, 0, 1<<16, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data) != 0 {
		return nil
	}
	if data == nil || nitems == 0 {
		return nil
	}
	defer func() {
		if lib.freeData != nil {
			lib.freeData(unsafe.Pointer(data))
		}
	}()
	if actualFormat != 32 {
		return nil
	}
	raw := unsafe.Slice((*uint32)(unsafe.Pointer(data)), nitems)
	out := make([]uintptr, 0, len(raw))
	for _, v := range raw {
		if v != 0 {
			out = append(out, uintptr(v))
		}
	}
	return out
}

// x11DndRootToLocal converts root coords to window-local logical px.
// Falls back to the root coords when translation is unavailable (Adopt
// without root, missing binding) so drag positions stay plausible.
func x11DndRootToLocal(st *x11State, rx, ry int) (float64, float64) {
	if st == nil || st.display == 0 || st.window == 0 || st.root == 0 || st.translateCoordinates == nil {
		return float64(rx), float64(ry)
	}
	var dx, dy int32
	var child uintptr
	if st.translateCoordinates(st.display, st.root, st.window, int32(rx), int32(ry), &dx, &dy, &child) == 0 {
		return float64(rx), float64(ry)
	}
	return float64(dx), float64(dy)
}

// x11DndSendClient crafts one XDND ClientMessage to targetWin.
func x11DndSendClient(st *x11State, targetWin, msgType uintptr, l0, l1, l2, l3, l4 uint64) {
	if st == nil || st.display == 0 || targetWin == 0 || msgType == 0 {
		return
	}
	lib := ctlLib.open()
	if !lib.ok() || lib.sendEvent == nil {
		return
	}
	var ev [128]byte
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xClientMessage)
	*(*uintptr)(unsafe.Pointer(&ev[32])) = targetWin
	*(*uintptr)(unsafe.Pointer(&ev[40])) = msgType
	*(*int32)(unsafe.Pointer(&ev[48])) = 32
	*(*uint64)(unsafe.Pointer(&ev[56])) = l0
	*(*uint64)(unsafe.Pointer(&ev[64])) = l1
	*(*uint64)(unsafe.Pointer(&ev[72])) = l2
	*(*uint64)(unsafe.Pointer(&ev[80])) = l3
	*(*uint64)(unsafe.Pointer(&ev[88])) = l4
	lib.sendEvent(st.display, targetWin, 0, 0, &ev[0])
	if st.flush != nil {
		st.flush()
	}
}

func x11DndSendStatus(st *x11State, source uintptr, accept bool) {
	if st == nil || source == 0 {
		return
	}
	var flags uint64
	var action uint64
	if accept {
		flags = 3 // bit0 accept + bit1 want position
		action = uint64(st.atXdndActionCopy)
	}
	x11DndSendClient(st, source, st.atXdndStatus,
		uint64(st.window), flags, 0, 0, action)
}

func x11DndSendFinished(st *x11State, source uintptr, success bool) {
	if st == nil || source == 0 {
		return
	}
	var ok uint64
	if success {
		ok = 1
	}
	x11DndSendClient(st, source, st.atXdndFinished,
		uint64(st.window), ok, uint64(st.atXdndActionCopy), 0, 0)
}

// handleXdndEnter processes XdndEnter: resolves the offered types and emits
// EventDragEnter. Always emits (even without uri-list) so the app can show
// feedback; Status accept follows the MIME phase 2 rule (any offered type
// the source can convert, not just uri-list).
func (h *x11Host) handleXdndEnter(st *x11State, buf []byte) []Event {
	source := uintptr(readU64(buf, xevClientData0Off))
	flags := readU64(buf, xevClientData0Off+8)
	version := int((flags >> 24) & 0xff)
	moreThan3 := flags&1 != 0
	var atoms []uintptr
	if moreThan3 {
		atoms = x11DndReadTypeList(st, source)
	} else {
		for i := 0; i < 3; i++ {
			if a := uintptr(readU64(buf, xevClientData0Off+16+i*8)); a != 0 {
				atoms = append(atoms, a)
			}
		}
	}
	mimes := make([]string, 0, len(atoms))
	for _, a := range atoms {
		if s := x11DndAtomName(st.display, a); s != "" {
			mimes = append(mimes, s)
		}
	}
	st.dndMu.Lock()
	st.dndInside = true
	st.dndSource = source
	st.dndVersion = version
	st.dndMimes = append([]string(nil), mimes...)
	st.dndMu.Unlock()
	x11DndSendStatus(st, source, len(mimes) > 0)
	return []Event{{Type: EventDragEnter, X: 0, Y: 0, MIMETypes: append([]string(nil), mimes...)}}
}

// handleXdndPosition processes XdndPosition: emits EventDragOver with the
// window-local position. A Position without a prior Enter synthesizes the
// Enter first (out-of-order sources, synthetic tests).
func (h *x11Host) handleXdndPosition(st *x11State, buf []byte) []Event {
	source := uintptr(readU64(buf, xevClientData0Off))
	packed := uint32(readU64(buf, xevClientData0Off+16))
	rx, ry := int(packed>>16), int(packed&0xffff)
	// 16-bit coords wrap above 32767 on wide desktops; unwrap as int16.
	if rx >= 32768 {
		rx -= 65536
	}
	if ry >= 32768 {
		ry -= 65536
	}
	ts := uint32(readU64(buf, xevClientData0Off+24))
	lx, ly := x11DndRootToLocal(st, rx, ry)
	st.dndMu.Lock()
	needEnter := !st.dndInside || st.dndSource != source
	if needEnter {
		st.dndInside = true
		st.dndSource = source
		if st.dndMimes == nil {
			st.dndMimes = nil
		}
	}
	st.dndX, st.dndY = lx, ly
	st.dndTimestamp = ts
	mimes := append([]string(nil), st.dndMimes...)
	st.dndMu.Unlock()
	x11DndSendStatus(st, source, len(mimes) > 0)
	var out []Event
	if needEnter {
		out = append(out, Event{Type: EventDragEnter, X: lx, Y: ly, MIMETypes: append([]string(nil), mimes...)})
	}
	out = append(out, Event{Type: EventDragOver, X: lx, Y: ly, MIMETypes: mimes})
	return out
}

// handleXdndLeave clears the drag and emits EventDragLeave. Stale Leaves
// (no drag inside) stay quiet.
func (h *x11Host) handleXdndLeave(st *x11State, buf []byte) []Event {
	source := uintptr(readU64(buf, xevClientData0Off))
	st.dndMu.Lock()
	inside := st.dndInside
	if source != 0 && st.dndSource != 0 && source != st.dndSource {
		st.dndMu.Unlock()
		return nil
	}
	st.dndInside = false
	st.dndSource = 0
	st.dndMimes = nil
	st.dndMu.Unlock()
	if !inside {
		return nil
	}
	return []Event{{Type: EventDragLeave}}
}

// MIME phase 2 budget: per-type and total caps for Drop Data payloads.
// INCR transfers already stream in chunks; the caps bound memory per drop.
const (
	x11DndMaxTypeBytes  = 1 << 20 // 1MB per MIME type
	x11DndMaxTotalBytes = 4 << 20 // 4MB per drop
)

// handleXdndDrop starts the transfer: the offered types are negotiated via
// TARGETS first (XDND standard: the source's TARGETS answer is the truth,
// the Enter-advertised list is only a hint); the answer drives a serial
// per-type conversion queue (uri-list fills Files, every fetched type fills
// Data). Drags with no usable type finish as rejected with no event
// (空拖放静默吞掉).
func (h *x11Host) handleXdndDrop(st *x11State, buf []byte) []Event {
	source := uintptr(readU64(buf, xevClientData0Off))
	ts := uint32(readU64(buf, xevClientData0Off+16))
	st.dndMu.Lock()
	mimes := append([]string(nil), st.dndMimes...)
	inside := st.dndInside
	lx, ly := st.dndX, st.dndY
	st.dndMu.Unlock()
	if source == 0 || !inside {
		return nil
	}
	if ts == 0 {
		ts = st.dndTimestamp
	}
	lib := clipLib.open()
	if !lib.ok() || lib.convertSelection == nil || st.atXdndSelection == 0 || st.atTargetsAtom == 0 {
		x11DndAbortDrop(st, source)
		return nil
	}
	st.dndMu.Lock()
	st.dndPendingSource = source
	st.dndPendingX, st.dndPendingY = lx, ly
	st.dndPendingTime = ts
	st.dndPendingMimes = append([]string(nil), mimes...)
	st.dndPendingQueue = nil
	st.dndPendingAtoms = nil
	st.dndDropFiles = nil
	st.dndDropData = nil
	st.dndDropBytes = 0
	st.dndMu.Unlock()
	// TARGETS negotiation first: the source's answer (ATOM list) is the
	// truth, the Enter-advertised list is only a hint.
	t := ts
	if t == 0 {
		t = xCurrentTime
	}
	lib.convertSelection(st.display, st.atXdndSelection, st.atTargetsAtom,
		st.atXdndSelection, st.window, uintptr(t))
	if lib.flush != nil {
		lib.flush(st.display)
	}
	return nil
}

// x11DndAbortDrop clears the pending drop and finishes the protocol as
// rejected. Callers hold no locks.
func x11DndAbortDrop(st *x11State, source uintptr) {
	if st == nil {
		return
	}
	st.dndMu.Lock()
	st.dndInside = false
	st.dndSource = 0
	st.dndMimes = nil
	st.dndPendingSource = 0
	st.dndPendingQueue = nil
	st.dndPendingAtoms = nil
	st.dndDropFiles = nil
	st.dndDropData = nil
	st.dndDropBytes = 0
	st.dndMu.Unlock()
	x11DndSendFinished(st, source, false)
}

// x11DndFinishDrop emits EventDrop when any payload landed (Files from
// uri-list and/or Data per MIME type); empty drops are swallowed (no
// event). Either way the protocol finishes; success mirrors whether the
// app got something. Callers hold no locks.
func x11DndFinishDrop(st *x11State, pending uintptr, lx, ly float64) []Event {
	if st == nil {
		return nil
	}
	st.dndMu.Lock()
	files := append([]string(nil), st.dndDropFiles...)
	data := st.dndDropData
	st.dndInside = false
	st.dndSource = 0
	st.dndMimes = nil
	st.dndPendingSource = 0
	st.dndPendingQueue = nil
	st.dndPendingAtoms = nil
	st.dndDropFiles = nil
	st.dndDropData = nil
	st.dndDropBytes = 0
	st.dndMu.Unlock()
	if len(files) == 0 && len(data) == 0 {
		x11DndSendFinished(st, pending, false)
		return nil
	}
	x11DndSendFinished(st, pending, true)
	return []Event{{Type: EventDrop, X: lx, Y: ly, Files: files, DropData: data}}
}

// x11DndQueueConversions intersects the TARGETS answer with the offered
// types (uri-list first for Files, then the rest in offer order) and
// starts the serial conversion chain. Returns false when nothing usable
// remains (caller finishes the drop). Callers hold no locks.
func x11DndQueueConversions(st *x11State, targets []uintptr) bool {
	if st == nil {
		return false
	}
	lib := clipLib.open()
	if !lib.ok() || lib.convertSelection == nil {
		return false
	}
	st.dndMu.Lock()
	offered := append([]string(nil), st.dndPendingMimes...)
	if len(offered) == 0 {
		offered = append([]string(nil), st.dndMimes...)
	}
	st.dndMu.Unlock()
	// TARGETS answer wins; fall back to the Enter list when the source
	// answered empty (older sources).
	names := make([]string, 0, len(targets))
	for _, a := range targets {
		if s := x11DndAtomName(st.display, a); s != "" {
			if s == "TARGETS" {
				continue
			}
			names = append(names, s)
		}
	}
	if len(names) == 0 {
		names = append([]string(nil), offered...)
	} else if len(offered) > 0 {
		allow := make(map[string]bool, len(offered))
		for _, m := range offered {
			allow[m] = true
		}
		kept := names[:0]
		for _, n := range names {
			if allow[n] {
				kept = append(kept, n)
			}
		}
		names = kept
	}
	if len(names) == 0 {
		return false
	}
	// uri-list first (Files), then the rest in order.
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if n == mimeURIList {
			ordered = append(ordered, n)
			break
		}
	}
	for _, n := range names {
		if n != mimeURIList {
			ordered = append(ordered, n)
		}
	}
	queue := make([]uintptr, 0, len(ordered))
	atoms := make(map[uintptr]string, len(ordered))
	for _, n := range ordered {
		a := internAtomCached(st.display, n)
		if a == 0 {
			continue
		}
		queue = append(queue, a)
		atoms[a] = n
	}
	if len(queue) == 0 {
		return false
	}
	st.dndMu.Lock()
	st.dndPendingQueue = queue
	st.dndPendingAtoms = atoms
	ts := st.dndPendingTime
	if ts == 0 {
		ts = st.dndTimestamp
	}
	st.dndMu.Unlock()
	x11DndConvertNext(st, ts)
	return true
}

// x11DndConvertNext converts the head of the pending queue. Callers hold
// no locks; a missing convert path skips to FinishDrop.
func x11DndConvertNext(st *x11State, ts uint32) {
	if st == nil {
		return
	}
	lib := clipLib.open()
	st.dndMu.Lock()
	if st.dndPendingSource == 0 || len(st.dndPendingQueue) == 0 {
		st.dndMu.Unlock()
		return
	}
	target := st.dndPendingQueue[0]
	st.dndMu.Unlock()
	if !lib.ok() || lib.convertSelection == nil || st.atXdndSelection == 0 {
		return
	}
	t := ts
	if t == 0 {
		t = xCurrentTime
	}
	lib.convertSelection(st.display, st.atXdndSelection, target,
		st.atXdndSelection, st.window, uintptr(t))
	if lib.flush != nil {
		lib.flush(st.display)
	}
}

// handleXdndSelectionNotify completes one conversion step: the TARGETS
// answer arms the serial queue; each payload appends to Files/Data and
// advances the queue; the last step emits EventDrop. Empty drops are
// swallowed (no event) but still finish the protocol.
func (h *x11Host) handleXdndSelectionNotify(st *x11State, buf []byte) []Event {
	st.dndMu.Lock()
	pending := st.dndPendingSource
	lx, ly := st.dndPendingX, st.dndPendingY
	target := uintptr(readU64(buf, 48))
	st.dndMu.Unlock()
	if pending == 0 {
		return nil
	}
	property := uintptr(readU64(buf, 56))
	if property == 0 {
		// One failed conversion must not stall the queue: drop the head
		// and move on (TARGETS lied or the type vanished mid-flight).
		if x11DndAdvanceQueue(st, target) {
			x11DndConvertNext(st, st.dndPendingTime)
			return nil
		}
		return x11DndFinishDrop(st, pending, lx, ly)
	}
	if target == st.atTargetsAtom && st.atTargetsAtom != 0 {
		// Delete the property BEFORE reading: the source wrote into our
		// window's property slot; without the delete a retried convert
		// (or a stale earlier answer) re-reads old bytes. Matches the
		// payload branch below.
		data, err := x11DndReadProperty(st, property)
		lib := clipLib.open()
		if lib.ok() && lib.deleteProperty != nil && st.atXdndSelection != 0 {
			lib.deleteProperty(st.display, st.window, st.atXdndSelection)
		}
		if err != nil || len(data) == 0 {
			// TARGETS unreadable: fall back to the Enter-advertised list.
			if x11DndQueueConversions(st, nil) {
				x11DndConvertNext(st, st.dndPendingTime)
				return nil
			}
			return x11DndFinishDrop(st, pending, lx, ly)
		}
		if !x11DndQueueConversions(st, x11DndDecodeAtomList(data)) {
			return x11DndFinishDrop(st, pending, lx, ly)
		}
		x11DndConvertNext(st, st.dndPendingTime)
		return nil
	}
	data, err := x11DndReadProperty(st, property)
	lib := clipLib.open()
	if lib.ok() && lib.deleteProperty != nil && st.atXdndSelection != 0 {
		lib.deleteProperty(st.display, st.window, st.atXdndSelection)
	}
	if err == nil && len(data) > 0 {
		x11DndStorePayload(st, target, data)
	}
	if x11DndAdvanceQueue(st, target) {
		x11DndConvertNext(st, st.dndPendingTime)
		return nil
	}
	return x11DndFinishDrop(st, pending, lx, ly)
}

// x11DndDecodeAtomList decodes a TARGETS answer (ATOM[], format 32) into
// atoms. x11DndReadProperty repacks format-32 longs taking only the low
// 32 bits per 8-byte item (LP64), so 4 bytes step here.
func x11DndDecodeAtomList(data []byte) []uintptr {
	out := make([]uintptr, 0, len(data)/4)
	for len(data) >= 4 {
		v := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
		data = data[4:]
		if v != 0 {
			out = append(out, uintptr(v))
		}
	}
	return out
}

// x11DndAdvanceQueue pops the head when it matches the finished target
// (a stale notify for an older target leaves the queue alone). Reports
// whether more conversions remain. Callers hold no locks.
func x11DndAdvanceQueue(st *x11State, target uintptr) bool {
	if st == nil {
		return false
	}
	st.dndMu.Lock()
	defer st.dndMu.Unlock()
	if len(st.dndPendingQueue) > 0 && (target == 0 || st.dndPendingQueue[0] == target) {
		st.dndPendingQueue = st.dndPendingQueue[1:]
	}
	return len(st.dndPendingQueue) > 0
}

// x11DndStorePayload appends one fetched payload: uri-list parses into
// Files (and keeps its raw bytes in Data), every other type lands in Data
// under its MIME name. Oversized singles are dropped, the total is capped;
// failures skip the type, never the drop. Callers hold no locks.
func x11DndStorePayload(st *x11State, target uintptr, data []byte) {
	if st == nil || len(data) == 0 {
		return
	}
	st.dndMu.Lock()
	name := ""
	if st.dndPendingAtoms != nil {
		name = st.dndPendingAtoms[target]
	}
	if name == "" {
		name = x11DndAtomName(st.display, target)
	}
	if name == "" {
		st.dndMu.Unlock()
		return
	}
	if len(data) > x11DndMaxTypeBytes || st.dndDropBytes+len(data) > x11DndMaxTotalBytes {
		st.dndMu.Unlock()
		return
	}
	if name == mimeURIList {
		if files := parseURIList(string(data)); len(files) > 0 {
			st.dndDropFiles = append(st.dndDropFiles, files...)
		}
	}
	if st.dndDropData == nil {
		st.dndDropData = make(map[string][]byte)
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	st.dndDropData[name] = cp
	st.dndDropBytes += len(data)
	st.dndMu.Unlock()
}

// x11DndOwnSelection claims XdndSelection ownership for the test source role.
// Real drag sources own the selection for the drag duration; without it the
// server answers ConvertSelection with property=None (failure).
func x11DndOwnSelection(st *x11State) {
	if st == nil || st.display == 0 || st.window == 0 || st.atXdndSelection == 0 {
		return
	}
	lib := clipLib.open()
	if !lib.ok() || lib.setSelectionOwner == nil {
		return
	}
	lib.setSelectionOwner(st.display, st.atXdndSelection, st.window, xCurrentTime)
	if lib.flush != nil {
		lib.flush(st.display)
	}
}

// x11DndReadProperty reads a window property on our own window (Drop payload).
func x11DndReadProperty(st *x11State, prop uintptr) ([]byte, error) {
	lib := clipLib.open()
	if !lib.ok() || lib.getWindowProperty == nil || st == nil || st.display == 0 {
		return nil, errXdndNoLib
	}
	var (
		actualType   uintptr
		actualFormat int
		nitems       uint64
		bytesAfter   uint64
		data         *byte
	)
	if lib.getWindowProperty(st.display, st.window, prop, 0, 1<<20, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data) != 0 {
		return nil, errXdndNoLib
	}
	if data == nil || nitems == 0 {
		return nil, errXdndNoLib
	}
	defer func() {
		if lib.freeData != nil {
			lib.freeData(unsafe.Pointer(data))
		}
	}()
	var b []byte
	if actualFormat == 8 {
		b = unsafe.Slice(data, nitems)
	} else if actualFormat == 32 {
		// LP64 Xlib returns long[] (8 bytes/elem); only the low 32
		// bits carry the value (same layout as the sync counter).
		// Step 8 bytes per item, not 4 (see x11ReadWindowStates).
		raw := unsafe.Slice((*uint64)(unsafe.Pointer(data)), nitems)
		b = make([]byte, len(raw)*4)
		for i, v := range raw {
			b[i*4] = byte(v)
			b[i*4+1] = byte(v >> 8)
			b[i*4+2] = byte(v >> 16)
			b[i*4+3] = byte(v >> 24)
		}
	} else {
		b = unsafe.Slice(data, nitems)
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out, nil
}

// handleXdndSelectionRequest serves our test-source payload (two-window対拖).
// Real file managers never hit this (we never own XdndSelection in prod).
func (h *x11Host) handleXdndSelectionRequest(st *x11State, buf []byte) {
	if st.atXdndSelection == 0 {
		return
	}
	selection := uintptr(readU64(buf, 48))
	if selection != st.atXdndSelection {
		return
	}
	requestor := uintptr(readU64(buf, 40))
	target := uintptr(readU64(buf, 56))
	property := uintptr(readU64(buf, 64))
	tm := uintptr(readU64(buf, 72))
	if requestor == 0 {
		return
	}
	if property == 0 {
		property = target
	}
	lib := clipLib.open()
	if !lib.ok() || lib.changeProperty == nil || lib.sendEvent == nil {
		return
	}
	st.dndMu.Lock()
	payload := st.dndSrcData
	snapMime := make(map[string]string, len(st.dndSrcMimeData))
	for k, v := range st.dndSrcMimeData {
		snapMime[k] = v
	}
	st.dndMu.Unlock()
	// TARGETS query: announce what the test source can serve (see
	// x11DndServeTargets: payload-gated, flushed before notify).
	if target == st.atTargetsAtom && st.atTargetsAtom != 0 {
		x11DndServeTargets(st, requestor, selection, property, tm, payload, snapMime)
		return
	}
	if target == st.atTextUriList && st.atTextUriList != 0 {
		if payload == "" {
			x11DndSendSelectionNotify(st, requestor, selection, target, 0, tm)
			return
		}
		b := []byte(payload)
		lib.changeProperty(st.display, requestor, property, target, 8, 0,
			unsafe.Pointer(&b[0]), len(b))
		if lib.flush != nil {
			lib.flush(st.display)
		}
		x11DndSendSelectionNotify(st, requestor, selection, target, property, tm)
		return
	}
	if name := x11DndAtomName(st.display, target); name != "" {
		if data, ok := snapMime[name]; ok && data != "" {
			b := []byte(data)
			lib.changeProperty(st.display, requestor, property, target, 8, 0,
				unsafe.Pointer(&b[0]), len(b))
			if lib.flush != nil {
				lib.flush(st.display)
			}
			x11DndSendSelectionNotify(st, requestor, selection, target, property, tm)
			return
		}
	}
	x11DndSendSelectionNotify(st, requestor, selection, target, 0, tm)
}

// x11DndServeTargets answers a TARGETS query with the types the test
// source can actually serve (payload-gated: uri-list only when dndSrcData
// is non-empty, extra MIMEs only when dndSrcMimeData holds them).
// Announcing a type we then refuse (property=None) stalls the serial
// conversion queue on sources that trust TARGETS. The property write is
// flushed before the SelectionNotify so the reply is visible on the
// requestor's connection when the notify lands. Callers hold no locks.
func x11DndServeTargets(st *x11State, requestor, selection, property, tm uintptr, payload string, snapMime map[string]string) {
	lib := clipLib.open()
	if !lib.ok() || lib.changeProperty == nil || lib.sendEvent == nil {
		return
	}
	target := st.atTargetsAtom
	seen := map[uintptr]bool{target: true}
	var filtered []uint64
	filtered = append(filtered, uint64(target))
	if payload != "" && st.atTextUriList != 0 {
		seen[st.atTextUriList] = true
		filtered = append(filtered, uint64(st.atTextUriList))
	}
	for name, data := range snapMime {
		if data == "" {
			continue
		}
		a := internAtomCached(st.display, name)
		if a != 0 && !seen[a] {
			seen[a] = true
			filtered = append(filtered, uint64(a))
		}
	}
	if len(filtered) > 0 {
		lib.changeProperty(st.display, requestor, property, target, 32, 0,
			unsafe.Pointer(&filtered[0]), len(filtered))
	} else {
		lib.changeProperty(st.display, requestor, property, target, 32, 0, nil, 0)
	}
	if lib.flush != nil {
		lib.flush(st.display)
	}
	x11DndSendSelectionNotify(st, requestor, selection, target, property, tm)
}

func x11DndSendSelectionNotify(st *x11State, requestor, selection, target, property, tm uintptr) {
	lib := clipLib.open()
	if !lib.ok() || lib.sendEvent == nil {
		return
	}
	var ev [192]byte
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xSelectionNotify)
	*(*uintptr)(unsafe.Pointer(&ev[24])) = st.display
	*(*uintptr)(unsafe.Pointer(&ev[32])) = requestor
	*(*uintptr)(unsafe.Pointer(&ev[40])) = selection
	*(*uintptr)(unsafe.Pointer(&ev[48])) = target
	*(*uintptr)(unsafe.Pointer(&ev[56])) = property
	*(*uintptr)(unsafe.Pointer(&ev[64])) = tm
	lib.sendEvent(st.display, requestor, 0, 0, &ev[0])
	if lib.flush != nil {
		lib.flush(st.display)
	}
}

// --- Outbound drag source (S6-P1 item 4: 本窗外发拖放) ---
//
// StartDragTo performs a programmatic XDND drop onto an explicit target
// window: the offer payload is staged into the SelectionRequest serving
// slots, XdndSelection is owned for the drag duration, and
// Enter/Position/Drop are sent back-to-back. The target pulls the types
// through the standard TARGETS-negotiated serial queue; XdndFinished
// clears the staged payload (see drainX).
//
// StartDrag snapshots the pointer (XQueryPointer on the root), verifies
// the window below is XdndAware, and drops onto it. No grab/modal loop
// yet — the drop fires immediately, the user gets no cancel gesture.

// StartDragTo implements WindowController.
func (c *x11Controller) StartDragTo(target *Window, offer DragOffer) error {
	s, payload, mimes, err := c.dragBegin(offer)
	if err != nil {
		return err
	}
	tw := x11WindowOf(target)
	if tw == 0 {
		return fmt.Errorf("x11: drag target has no native window")
	}
	atoms, err := x11DndOfferAtoms(s, mimes)
	if err != nil {
		return err
	}
	x11DndStageSource(s, payload)
	x11DndOwnSelection(s)
	rx, ry := x11DndAimInside(target)
	return x11DndDropOnto(s, tw, rx, ry, atoms)
}

// StartDrag implements WindowController.
func (c *x11Controller) StartDrag(offer DragOffer) error {
	s, payload, mimes, err := c.dragBegin(offer)
	if err != nil {
		return err
	}
	tw, rx, ry, err := x11DndTargetUnderPointer(s)
	if err != nil {
		return err
	}
	atoms, err := x11DndOfferAtoms(s, mimes)
	if err != nil {
		return err
	}
	x11DndStageSource(s, payload)
	x11DndOwnSelection(s)
	return x11DndDropOnto(s, tw, rx, ry, atoms)
}

// dragBegin runs the checks shared by both drag entries: live source,
// non-empty payload, resolved atoms, usable lib. The payload is staged
// and the selection owned only after the caller validated its target.
func (c *x11Controller) dragBegin(offer DragOffer) (*x11State, map[string][]byte, []string, error) {
	s := c.st()
	if s == nil || s.display == 0 || s.window == 0 {
		return nil, nil, nil, fmt.Errorf("x11: drag source unavailable")
	}
	payload := offer.OfferPayload()
	if len(payload) == 0 {
		return nil, nil, nil, fmt.Errorf("platform: empty drag offer")
	}
	if s.atXdndEnter == 0 {
		s.resolveAtoms(s.display)
	}
	lib := c.lib()
	if !lib.ok() || lib.sendEvent == nil || s.atXdndEnter == 0 ||
		s.atXdndPosition == 0 || s.atXdndDrop == 0 || s.atXdndSelection == 0 {
		return nil, nil, nil, ErrUnsupported
	}
	return s, payload, offer.MIMETypes(), nil
}

// x11WindowOf returns the native X11 window id of w, or 0 when w is not
// a live X11 window (nil, closed, or another backend).
func x11WindowOf(w *Window) uintptr {
	if w == nil || w.Kind() != PlatformX11 {
		return 0
	}
	h, ok := w.Host().(*x11Host)
	if !ok || h == nil || h.st == nil {
		return 0
	}
	return h.st.window
}

// x11DndStageSource stages the servable payload into the SelectionRequest
// slots (uri-list string + extra MIME map). Callers hold no locks.
func x11DndStageSource(st *x11State, payload map[string][]byte) {
	if st == nil {
		return
	}
	st.dndMu.Lock()
	defer st.dndMu.Unlock()
	st.dndSrcData = string(payload[mimeURIList])
	st.dndSrcMimeData = nil
	for m, v := range payload {
		if m == mimeURIList || len(v) == 0 {
			continue
		}
		if st.dndSrcMimeData == nil {
			st.dndSrcMimeData = make(map[string]string, len(payload))
		}
		st.dndSrcMimeData[m] = string(v)
	}
}

// x11DndClearSource drops the staged source payload after the target
// reports XdndFinished. Late converts already hold their bytes; clearing
// only stops serving stale data to a later stray request. Callers hold
// no locks.
func x11DndClearSource(st *x11State) {
	if st == nil {
		return
	}
	st.dndMu.Lock()
	st.dndSrcData = ""
	st.dndSrcMimeData = nil
	st.dndMu.Unlock()
}

// x11DndOfferAtoms resolves the announced MIME types to atoms (uri-list
// via the resolved atom, the rest interned). Empty means nothing servable.
func x11DndOfferAtoms(s *x11State, mimes []string) ([]uintptr, error) {
	atoms := make([]uintptr, 0, len(mimes))
	for _, m := range mimes {
		var a uintptr
		if m == mimeURIList {
			a = s.atTextUriList
		} else {
			a = internAtomCached(s.display, m)
		}
		if a != 0 {
			atoms = append(atoms, a)
		}
	}
	if len(atoms) == 0 {
		return nil, fmt.Errorf("x11: drag offers no servable type")
	}
	return atoms, nil
}

// x11DndDropOnto sends Enter/Position/Drop to targetWin. More than 3 types
// go via XdndTypeList. (rx,ry) is the root drop position (delivery never
// depends on it).
func x11DndDropOnto(s *x11State, targetWin uintptr, rx, ry int, atoms []uintptr) error {
	if len(atoms) > 3 {
		x11DndPublishTypeList(s, atoms)
	}
	var l2, l3, l4 uint64
	if len(atoms) <= 3 {
		if len(atoms) > 0 {
			l2 = uint64(atoms[0])
		}
		if len(atoms) > 1 {
			l3 = uint64(atoms[1])
		}
		if len(atoms) > 2 {
			l4 = uint64(atoms[2])
		}
	}
	var flags uint64 = uint64(xdndVersion << 24)
	if len(atoms) > 3 {
		flags |= 1
	}
	x11DndSendClient(s, targetWin, s.atXdndEnter, uint64(s.window), flags, l2, l3, l4)
	packed := (uint64(uint32(rx)) << 16) | uint64(uint32(ry))
	x11DndSendClient(s, targetWin, s.atXdndPosition,
		uint64(s.window), 0, packed, 0, uint64(s.atXdndActionCopy))
	x11DndSendClient(s, targetWin, s.atXdndDrop, uint64(s.window), 0, 0, 0, 0)
	return nil
}

// x11DndPublishTypeList publishes the full offered type list on the source
// window for Enter packets that carry more than 3 inline atoms.
func x11DndPublishTypeList(s *x11State, atoms []uintptr) {
	lib := ctlLib.open()
	if !lib.ok() || lib.changeProperty == nil || s.atXdndTypeList == 0 {
		return
	}
	typeAtom := internAtomCached(s.display, "ATOM")
	if typeAtom == 0 {
		typeAtom = s.atXdndTypeList
	}
	raw := make([]uint64, len(atoms))
	for i, a := range atoms {
		raw[i] = uint64(a)
	}
	lib.changeProperty(s.display, s.window, s.atXdndTypeList, typeAtom, 32,
		xPropModeReplace, unsafe.Pointer(&raw[0]), len(raw))
	if s.flush != nil {
		s.flush()
	}
}

// x11DndAimInside returns a root position inside the target window
// (origin + 50,50 offset) or a plausible fallback when the target
// position is unknown. Delivery never depends on the exact point.
func x11DndAimInside(target *Window) (int, int) {
	if target != nil {
		if ctl := target.Controls(); ctl != nil {
			if x, y, ok := ctl.Position(); ok {
				return x + 50, y + 50
			}
		}
	}
	return 100, 100
}

// x11DndTargetUnderPointer snapshots the pointer via XQueryPointer on the
// root and returns the XdndAware window below it plus the root coords.
func x11DndTargetUnderPointer(s *x11State) (uintptr, int, int, error) {
	lib := ctlLib.open()
	if s == nil || s.display == 0 || s.root == 0 {
		return 0, 0, 0, ErrUnsupported
	}
	if s.atXdndAware == 0 {
		s.resolveAtoms(s.display)
	}
	if !lib.ok() || lib.queryPointer == nil || lib.getWindowProperty == nil || s.atXdndAware == 0 {
		return 0, 0, 0, ErrUnsupported
	}
	var root, child uintptr
	var rx, ry, wx, wy int32
	var mask uint
	if lib.queryPointer(s.display, s.root, &root, &child, &rx, &ry, &wx, &wy, &mask) == 0 {
		return 0, 0, 0, fmt.Errorf("x11: pointer position unknown")
	}
	if child == 0 {
		return 0, 0, 0, fmt.Errorf("x11: no window under the pointer")
	}
	if !x11DndCheckAware(s, child) {
		return 0, 0, 0, fmt.Errorf("x11: window under the pointer is not droppable")
	}
	return child, int(rx), int(ry), nil
}

// x11DndCheckAware reports whether win advertises XdndAware (any version).
// False on any lookup failure (bad window, missing lib, no property).
func x11DndCheckAware(s *x11State, win uintptr) bool {
	lib := ctlLib.open()
	if !lib.ok() || lib.getWindowProperty == nil || lib.freeData == nil {
		return false
	}
	if s == nil || s.display == 0 || win == 0 || s.atXdndAware == 0 {
		return false
	}
	var (
		actualType   uintptr
		actualFormat int
		nitems       uint64
		bytesAfter   uint64
		data         *byte
	)
	if lib.getWindowProperty(s.display, win, s.atXdndAware, 0, 1, 0, 0,
		&actualType, &actualFormat, &nitems, &bytesAfter, &data) != 0 {
		return false
	}
	if data != nil {
		lib.freeData(unsafe.Pointer(data))
	}
	return nitems > 0
}
