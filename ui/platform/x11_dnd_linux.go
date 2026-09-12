//go:build linux

package platform

import (
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
// Source role is test-only (two-window対拖): st.dndSrcData holds the uri-list
// payload served on SelectionRequest(XdndSelection). Production file managers
// serve themselves; our windows never initiate outbound drags.

const xdndVersion = 5

var errXdndNoLib = errXdndLib{}

type errXdndLib struct{}

func (errXdndLib) Error() string { return "x11: xdnd lib not ready" }

var (
	xdndAtomOnce sync.Once
	xdndAtomLib  uintptr
	xdndGetAtomName  func(dpy uintptr, atom uintptr) *byte
	xdndFree         func(ptr unsafe.Pointer) int
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

func x11DndHasURIList(mimes []string) bool {
	for _, m := range mimes {
		if m == mimeURIList {
			return true
		}
	}
	return false
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
// feedback; Status accept follows the file-first rule.
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
	x11DndSendStatus(st, source, x11DndHasURIList(mimes))
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
	x11DndSendStatus(st, source, x11DndHasURIList(mimes))
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

// handleXdndDrop starts the file transfer: non-uri-list drags finish as
// rejected with no event (空拖放静默吞掉); uri-list drags ConvertSelection
// and the files arrive via SelectionNotify.
func (h *x11Host) handleXdndDrop(st *x11State, buf []byte) []Event {
	source := uintptr(readU64(buf, xevClientData0Off))
	ts := uint32(readU64(buf, xevClientData0Off+16))
	st.dndMu.Lock()
	mimes := append([]string(nil), st.dndMimes...)
	lx, ly := st.dndX, st.dndY
	st.dndMu.Unlock()
	if source == 0 {
		return nil
	}
	if !x11DndHasURIList(mimes) {
		st.dndMu.Lock()
		st.dndInside = false
		st.dndSource = 0
		st.dndMimes = nil
		st.dndPendingSource = 0
		st.dndMu.Unlock()
		x11DndSendFinished(st, source, false)
		return nil
	}
	if ts == 0 {
		ts = st.dndTimestamp
	}
	st.dndMu.Lock()
	st.dndPendingSource = source
	st.dndPendingX, st.dndPendingY = lx, ly
	st.dndPendingTime = ts
	st.dndPendingMimes = append([]string(nil), mimes...)
	st.dndMu.Unlock()
	lib := clipLib.open()
	if !lib.ok() || lib.convertSelection == nil || st.atXdndSelection == 0 || st.atTextUriList == 0 {
		st.dndMu.Lock()
		st.dndInside = false
		st.dndSource = 0
		st.dndPendingSource = 0
		st.dndMu.Unlock()
		x11DndSendFinished(st, source, false)
		return nil
	}
	// property=X dndSelection (standard target-side property).
	t := ts
	if t == 0 {
		t = xCurrentTime
	}
	lib.convertSelection(st.display, st.atXdndSelection, st.atTextUriList,
		st.atXdndSelection, st.window, uintptr(t))
	if lib.flush != nil {
		lib.flush(st.display)
	}
	return nil
}

// handleXdndSelectionNotify completes a Drop: reads the uri-list property,
// emits EventDrop with files, and replies Finished. Empty payloads are
// swallowed (no event) but still finish the protocol.
func (h *x11Host) handleXdndSelectionNotify(st *x11State, buf []byte) []Event {
	st.dndMu.Lock()
	pending := st.dndPendingSource
	lx, ly := st.dndPendingX, st.dndPendingY
	st.dndMu.Unlock()
	if pending == 0 {
		return nil
	}
	property := uintptr(readU64(buf, 56))
	if property == 0 {
		st.dndMu.Lock()
		st.dndInside = false
		st.dndSource = 0
		st.dndPendingSource = 0
		st.dndMu.Unlock()
		x11DndSendFinished(st, pending, false)
		return nil
	}
	data, err := x11DndReadProperty(st, property)
	lib := clipLib.open()
	if lib.ok() && lib.deleteProperty != nil && st.atXdndSelection != 0 {
		lib.deleteProperty(st.display, st.window, st.atXdndSelection)
	}
	st.dndMu.Lock()
	st.dndInside = false
	st.dndSource = 0
	st.dndMimes = nil
	st.dndPendingSource = 0
	st.dndMu.Unlock()
	if err != nil || len(data) == 0 {
		x11DndSendFinished(st, pending, false)
		return nil
	}
	files := parseURIList(string(data))
	if len(files) == 0 {
		x11DndSendFinished(st, pending, false)
		return nil
	}
	x11DndSendFinished(st, pending, true)
	return []Event{{Type: EventDrop, X: lx, Y: ly, Files: files}}
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
	if !lib.ok() || lib.getWindowProperty == nil || st.display == 0 {
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
		raw := unsafe.Slice((*uint32)(unsafe.Pointer(data)), nitems)
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
	st.dndMu.Unlock()
	// TARGETS query: announce what the test source can serve.
	if target == st.atTargetsAtom && st.atTargetsAtom != 0 {
		list := []uintptr{st.atTargetsAtom, st.atTextUriList}
		var filtered []uintptr
		for _, a := range list {
			if a != 0 {
				filtered = append(filtered, a)
			}
		}
		if len(filtered) > 0 {
			lib.changeProperty(st.display, requestor, property, st.atTargetsAtom, 32, 0,
				unsafe.Pointer(&filtered[0]), len(filtered))
		} else {
			lib.changeProperty(st.display, requestor, property, st.atTargetsAtom, 32, 0, nil, 0)
		}
		x11DndSendSelectionNotify(st, requestor, selection, target, property, tm)
		return
	}
	if target != st.atTextUriList || payload == "" {
		x11DndSendSelectionNotify(st, requestor, selection, target, 0, tm)
		return
	}
	b := []byte(payload)
	if len(b) > 0 {
		lib.changeProperty(st.display, requestor, property, target, 8, 0,
			unsafe.Pointer(&b[0]), len(b))
	} else {
		lib.changeProperty(st.display, requestor, property, target, 8, 0, nil, 0)
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
