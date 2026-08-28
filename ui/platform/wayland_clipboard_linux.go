//go:build linux

package platform

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

// wl_data_device_manager / wl_data_device / wl_data_source / wl_data_offer
// — clipboard (selection) + external file drag-and-drop. The interface
// structs are exported by libwayland-client (core wayland.xml) and resolved
// in loadWayland; only the request opcodes are needed here (marshal size is
// derived from the interface's method table).
//
// Binding version: 1 everywhere. version 1 covers selection + DnD enter/
// motion/drop; the since-3 members (data_offer.finish/set_actions, source
// actions) are not used — the compositor is told to treat us as a v1 client,
// so it never sends the v3 offer/source events.
const (
	// wl_data_device_manager requests (wayland.xml):
	//   create_data_source(0) get_data_device(1) destroy(2)
	wlDataDevMgrCreateSource  = 0
	wlDataDevMgrGetDataDevice = 1

	// wl_data_device requests: start_drag(0) set_selection(1) release(2).
	wlDataDevSetSelection = 1
	// wl_data_device events:
	//   data_offer(0) enter(1) leave(2) motion(3) drop(4) selection(5)
	wlDataDevEvOffer     = 0
	wlDataDevEvEnter     = 1
	wlDataDevEvLeave     = 2
	wlDataDevEvMotion    = 3
	wlDataDevEvDrop      = 4
	wlDataDevEvSelection = 5

	// wl_data_source requests: offer(0) destroy(1) set_actions(2).
	wlDataSourceOffer   = 0
	wlDataSourceDestroy = 1
	// wl_data_source events: target(0) send(1) cancelled(2).
	wlDataSourceEvSend      = 1
	wlDataSourceEvCancelled = 2

	// wl_data_offer requests: accept(0) receive(1) destroy(2).
	wlDataOfferReceive = 1
	wlDataOfferDestroy = 2
	// wl_data_offer events: offer(0) (mime announced).
	wlDataOfferEvOffer = 0
)

// mimeURIList is the standard DnD payload type for file transfers.
const mimeURIList = "text/uri-list"

// wlDataDeviceState owns the seat's wl_data_device: clipboard (selection) +
// external file DnD. Created after seat capabilities arrive (bindDataDevice);
// nil when the compositor lacks wl_data_device_manager.
type wlDataDeviceState struct {
	lib *wlLib
	win *wlWin

	mgr uintptr // wl_data_device_manager
	dd  uintptr // wl_data_device (bound to the seat)

	ddListener     [6]uintptr
	offerListener  [1]uintptr
	sourceListener [3]uintptr
	selfPtr        uintptr

	mu sync.Mutex
	// offerMu serializes offer/source "active use" (marshal receive + the
	// blocking read) against the deferred proxy destruction (drain). The
	// event thread dispatches freely while an offer is being read — a
	// clipboard read from any goroutine needs the peer's send() callback
	// (dispatched on the event thread) to arrive — so only the wire
	// operations (receive vs destroy marshal) are serialized here.
	offerMu sync.Mutex
	// Clipboard (selection).
	selOffer  uintptr // current selection data_offer (0 = no selection)
	ownSource uintptr // our wl_data_source (held while we own the clipboard)
	ownData   string  // Set() copy — served to peers on send(), readable locally
	cachedData string // last external Get result, for fast second paste without re-reading pipe
	cachedKind string
	cachedOffer uintptr
	// DnD (external file drops).
	dragOffer    uintptr   // data_offer of the in-flight drag (0 = none)
	dragMimes    []string  // mimes announced via wl_data_offer.offer
	dragX, dragY float64   // last drag position (window local, logical px)
	dragInFlight bool      // enter received, drop/leave pending
	// pendingDestroys are offer/source proxies queued for destruction on the
	// event thread (wlHost.poll drains them): Clipboard.Get/Set may run on
	// any goroutine, and a libwayland proxy must never be destroyed while
	// the event thread is dispatching it (double-destroy → dangling pointer
	// → SIGSEGV in wl_proxy_marshal). Each entry carries the destroy request
	// opcode of its interface: wl_data_offer.destroy = 2, wl_data_source.destroy
	// = 1 — sharing one opcode would marshal set_actions on the source
	// (protocol error, connection killed).
	pendingDestroys []pendingDestroy
}

// pendingDestroy is one queued proxy destruction (see wlDataDeviceState).
type pendingDestroy struct {
	p  uintptr // wl_data_offer or wl_data_source proxy
	op uint32  // destroy request opcode for that interface
}

// bindDataDevice binds wl_data_device_manager + wl_data_device (seat) and
// installs the event listeners. Returns nil when the compositor lacks the
// global (clipboard/DnD silently degrade).
func (w *wlWin) bindDataDevice() *wlDataDeviceState {
	if w == nil || w.lib == nil || w.ddMgrName == 0 || w.seat == 0 {
		return nil
	}
	if w.lib.ifaceDataDevMgr == 0 || w.lib.ifaceDataDev == 0 {
		return nil
	}
	lib := w.lib
	st := &wlDataDeviceState{lib: lib, win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	st.mgr = w.bind(w.registry, w.ddMgrName, lib.ifaceDataDevMgr, 1)
	if st.mgr == 0 {
		return nil
	}
	args := []wlArg{argNewID(), argO(w.seat)}
	st.dd = lib.proxyMarshalArrayCtor(st.mgr, wlDataDevMgrGetDataDevice, &args[0], lib.ifaceDataDev, 1)
	if st.dd == 0 {
		lib.proxyDestroy(st.mgr)
		st.mgr = 0
		return nil
	}
	st.ddListener[wlDataDevEvOffer] = purego.NewCallback(wlDDDataOfferCB)
	st.ddListener[wlDataDevEvEnter] = purego.NewCallback(wlDDEnterCB)
	st.ddListener[wlDataDevEvLeave] = purego.NewCallback(wlDDLeaveCB)
	st.ddListener[wlDataDevEvMotion] = purego.NewCallback(wlDDMotionCB)
	st.ddListener[wlDataDevEvDrop] = purego.NewCallback(wlDDDropCB)
	st.ddListener[wlDataDevEvSelection] = purego.NewCallback(wlDDSelectionCB)
	st.offerListener[wlDataOfferEvOffer] = purego.NewCallback(wlOfferMimeCB)
	st.sourceListener[wlDataSourceEvSend] = purego.NewCallback(wlSourceSendCB)
	st.sourceListener[wlDataSourceEvCancelled] = purego.NewCallback(wlSourceCancelledCB)
	if lib.proxyAddListener(st.dd, uintptr(unsafe.Pointer(&st.ddListener[0])), st.selfPtr) != 0 {
		st.destroy()
		return nil
	}
	return st
}

func ddsFrom(data uintptr) *wlDataDeviceState {
	if data == 0 {
		return nil
	}
	return (*wlDataDeviceState)(unsafe.Pointer(data))
}

func (st *wlDataDeviceState) destroy() {
	if st == nil {
		return
	}
	st.destroyNow()
	st.mu.Lock()
	dd, mgr := st.dd, st.mgr
	st.dd, st.mgr = 0, 0
	st.mu.Unlock()
	if dd != 0 {
		st.lib.proxyDestroy(dd)
	}
	if mgr != 0 {
		st.lib.proxyDestroy(mgr)
	}
}

// destroyOffer queues a wl_data_offer for destruction on the event thread
// (the marshal + proxyDestroy run in poll's drainPendingDestroys). Proxy
// destruction must happen on the thread that dispatches the object's events
// — Get/Set may run on any goroutine. Duplicates are coalesced: the same
// offer may be retired by a new selection broadcast and by Get concurrently.
func (st *wlDataDeviceState) destroyOffer(offer uintptr) {
	if offer == 0 {
		return
	}
	st.mu.Lock()
	st.queueDestroyLocked(offer, wlDataOfferDestroy)
	st.mu.Unlock()
	st.wakeForDestroy()
}

// destroySource queues a wl_data_source for destruction on the event thread.
func (st *wlDataDeviceState) destroySource(src uintptr) {
	if src == 0 {
		return
	}
	st.mu.Lock()
	st.queueDestroyLocked(src, wlDataSourceDestroy)
	st.mu.Unlock()
	st.wakeForDestroy()
}

// drainPendingDestroys performs the queued proxy destructions; called from
// wlHost.poll (event thread) once per wake. offerMu serializes the destroy
// marshal against a concurrent Get/DnD receive marshal on the same proxy.
func (st *wlDataDeviceState) drainPendingDestroys() {
	if st == nil || st.lib == nil {
		return
	}
	st.mu.Lock()
	pending := st.pendingDestroys
	st.pendingDestroys = nil
	st.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	st.offerMu.Lock()
	defer st.offerMu.Unlock()
	for _, q := range pending {
		if q.p == st.dd || q.p == st.mgr {
			continue // never destroy the device/manager from pending
		}
		st.lib.proxyMarshalArrayFlags(q.p, q.op, 0, 0, 0, nil)
		st.lib.proxyDestroy(q.p)
	}
}

// destroyNow destroys everything synchronously — only for backend teardown
// (Close), when the event pump is already stopped.
func (st *wlDataDeviceState) destroyNow() {
	if st == nil {
		return
	}
	st.offerMu.Lock()
	defer st.offerMu.Unlock()
	st.drainPendingDestroys()
	st.mu.Lock()
	sel, drag, src := st.selOffer, st.dragOffer, st.ownSource
	st.selOffer, st.dragOffer, st.ownSource = 0, 0, 0
	st.mu.Unlock()
	if sel != 0 {
		st.lib.proxyMarshalArrayFlags(sel, wlDataOfferDestroy, 0, 0, 0, nil)
		st.lib.proxyDestroy(sel)
	}
	if drag != 0 && drag != sel {
		st.lib.proxyMarshalArrayFlags(drag, wlDataOfferDestroy, 0, 0, 0, nil)
		st.lib.proxyDestroy(drag)
	}
	if src != 0 {
		st.lib.proxyMarshalArrayFlags(src, wlDataSourceDestroy, 0, 0, 0, nil)
		st.lib.proxyDestroy(src)
	}
}

// --- wl_data_device event callbacks (event thread) ---

// wlDDDataOfferCB: data_offer(id) — the compositor created a new data offer
// (selection or DnD). Install the offer listener so the announced mime list
// is collected (needed for DnD; the clipboard Get does not depend on it).
func wlDDDataOfferCB(data, dd, id uintptr) {
	st := ddsFrom(data)
	if st == nil || st.lib == nil || id == 0 {
		return
	}
	st.lib.proxyAddListener(id, uintptr(unsafe.Pointer(&st.offerListener[0])), st.selfPtr)
}

// wlOfferMimeCB: wl_data_offer.offer(mime) — one announced mime type.
func wlOfferMimeCB(data, offer, mime uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	s := goString(mime)
	st.mu.Lock()
	if offer == st.dragOffer {
		st.dragMimes = append(st.dragMimes, s)
	}
	st.mu.Unlock()
}

// wlDDEnterCB: enter(serial, surface, x, y, id) — a drag entered the window.
// wl_fixed x/y encode 24.8 fixed-point (divide by 256).
func wlDDEnterCB(data, dd, serial, surface, sx, sy, id uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	// A second enter without leave replaces the previous drag offer.
	if st.dragOffer != 0 && st.dragOffer != id {
		prev := st.dragOffer
		st.dragOffer = 0
		st.mu.Unlock()
		st.destroyOffer(prev)
		st.mu.Lock()
	}
	st.dragOffer = id
	st.dragMimes = nil
	st.dragX = wlFixedToDouble(sx)
	st.dragY = wlFixedToDouble(sy)
	st.dragInFlight = true
	st.mu.Unlock()
}

// wlDDMotionCB: motion(time, x, y) — drag position update (window local).
func wlDDMotionCB(data, dd, time, x, y uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	st.dragX = wlFixedToDouble(x)
	st.dragY = wlFixedToDouble(y)
	st.mu.Unlock()
}

// wlDDLeaveCB: leave() — the drag left the window; abandon the offer.
func wlDDLeaveCB(data, dd uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	off := st.dragOffer
	st.dragOffer = 0
	st.dragMimes = nil
	st.dragInFlight = false
	st.mu.Unlock()
	if off != 0 {
		st.destroyOffer(off)
	}
}

// wlDDDropCB: drop() — the user released a file drop over the window. A
// text/uri-list payload is pulled synchronously here (the compositor feeds
// the pipe asynchronously; readOffer drains it) and reported as an EventDrop
// with the resolved absolute file paths.
func wlDDDropCB(data, dd uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	off := st.dragOffer
	mimes := st.dragMimes
	x, y := st.dragX, st.dragY
	st.dragOffer = 0
	st.dragMimes = nil
	st.dragInFlight = false
	st.mu.Unlock()
	if off == 0 {
		return
	}
	if hasMime(mimes, mimeURIList) {
		// Serialize the receive marshal against the deferred destroy marshal;
		// the drop offer is not pending destruction yet (callbacks produce
		// the pending entries), but concurrent Get+drain may be mid-flight.
		st.offerMu.Lock()
		buf, err := st.readOffer(off, mimeURIList)
		st.offerMu.Unlock()
		if err == nil {
			files := parseURIList(string(buf))
			if len(files) > 0 {
				w := st.win
				w.dndMu.Lock()
				w.dndEvents = append(w.dndEvents, Event{Type: EventDrop, Files: files, X: x, Y: y})
				w.dndMu.Unlock()
			}
		}
	}
	st.destroyOffer(off)
}

// wlDDSelectionCB: selection(offer) — the clipboard selection changed (offer
// may be nil = cleared). The previous selection offer is destroyed.
func wlDDSelectionCB(data, dd, offer uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	old := st.selOffer
	st.selOffer = offer
	// 选区变了，之前缓存的外部数据失效
	if offer != st.cachedOffer {
		st.cachedData = ""
		st.cachedKind = ""
		st.cachedOffer = 0
	}
	st.mu.Unlock()
	if old != 0 && old != offer {
		st.destroyOffer(old)
	}
}

// --- wl_data_source callbacks (our clipboard write) ---

// wlSourceSendCB: send(mime, fd) — a peer wants our clipboard data: write
// the stored payload into fd and close it (the compositor forwards it).
func wlSourceSendCB(data, source, mime, fd uintptr) {
	st := ddsFrom(data)
	if st == nil || fd == 0 {
		return
	}
	st.mu.Lock()
	d := st.ownData
	st.mu.Unlock()
	b := []byte(d)
	for len(b) > 0 {
		n, err := unix.Write(int(fd), b)
		if err != nil {
			break
		}
		b = b[n:]
	}
	_ = unix.Close(int(fd))
}

// wlSourceCancelledCB: cancelled() — the clipboard was taken by another
// client (or our source was otherwise invalidated); drop the source proxy.
func wlSourceCancelledCB(data, source uintptr) {
	st := ddsFrom(data)
	if st == nil {
		return
	}
	st.mu.Lock()
	if st.ownSource == source {
		st.ownSource = 0
		st.ownData = ""
	}
	st.mu.Unlock()
	st.destroySource(source)
}

// --- helpers ---

func hasMime(mimes []string, want string) bool {
	for _, m := range mimes {
		if m == want {
			return true
		}
	}
	return false
}

// parseURIList converts a text/uri-list payload into absolute local paths
// (file:// URLs, %XX-unescaped; blank lines and # comments skipped).
func parseURIList(data string) []string {
	var out []string
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "file://") {
			continue
		}
		p := line[len("file://"):]
		// file://host/path — keep only the path part (host is empty or
		// localhost on practically every source).
		if i := strings.IndexByte(p, '/'); i >= 0 {
			p = p[i:]
		} else {
			continue
		}
		if unesc, err := url.PathUnescape(p); err == nil {
			out = append(out, unesc)
		}
	}
	return out
}

// readOffer pulls a mime payload from a wl_data_offer: pipe → receive →
// drain until EOF (the compositor writes the peer's data into the pipe
// asynchronously). Synchronous, bounded by a read deadline; the offer is
// one-shot (destroyed by the caller).
func (st *wlDataDeviceState) readOffer(offer uintptr, mime string) ([]byte, error) {
	lib := st.lib
	if offer == 0 || lib == nil {
		return nil, fmt.Errorf("wayland: readOffer: no offer")
	}
	p := [2]int{-1, -1}
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		return nil, err
	}
	pin := append([]byte(mime), 0)
	args := []wlArg{argS(cstr(pin)), argO(uintptr(p[1]))}
	lib.proxyMarshalArrayFlags(offer, wlDataOfferReceive, 0, 0, 0, &args[0])
	lib.displayFlush(st.win.display)
	// 立刻关闭写端在客户端的副本，否则读端永远看不到 EOF（peer 关闭后仍有本端写端打开）。
	_ = unix.Close(p[1])
	p[1] = -1

	var buf []byte
	tmp := make([]byte, 8192)
	deadline := time.Now().Add(3 * time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		// 用 Poll 等待可读，避免忙等；外部粘贴通常 <50ms 内到达
		fds := []unix.PollFd{{Fd: int32(p[0]), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(remaining.Milliseconds()))
		if err != nil || n == 0 {
			break
		}
		if fds[0].Revents&(unix.POLLHUP|unix.POLLERR) != 0 {
			// 对端已关闭，尽量把剩余数据读完
			for {
				n, _ := unix.Read(p[0], tmp)
				if n > 0 {
					buf = append(buf, tmp[:n]...)
				} else {
					break
				}
			}
			break
		}
		if fds[0].Revents&unix.POLLIN != 0 {
			n, err := unix.Read(p[0], tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil || n == 0 {
				break
			}
		}
	}
	_ = unix.Close(p[0])
	if p[1] != -1 {
		_ = unix.Close(p[1])
	}
	return buf, nil
}

// --- Clipboard capability (Window.Clipboard) ---

// wlClipboard implements platform.Clipboard over the Wayland selection:
// Set owns a wl_data_source (peers pull via send) and keeps an internal
// copy; Get drains the current selection offer, falling back to the
// internal copy when the compositor reports no selection.
type wlClipboard struct {
	h *wlHost
}

func (c *wlClipboard) st() *wlDataDeviceState {
	if c == nil || c.h == nil || c.h.win == nil {
		return nil
	}
	return c.h.win.dds
}

// Get returns the clipboard content of kind ("" → text/plain).
//
// Two paths:
//   - self-read: we still own the clipboard (ownSource alive → our own Set
//     broadcast); the data is already in local memory and returned directly
//     (GTK keeps and serves its own copy; no compositor round-trip, no dead
//     lock with our own event thread that must dispatch the peer send).
//   - external read: the current selection offer is pulled via receive; the
//     offerMu critical section serializes the receive marshal against the
//     deferred destroy marshal (drain). The blocking fd read happens while
//     the offer is already retired — the peer writes into the pipe on its
//     own time, so the event thread stalls at most the read budget (GTK's
//     Wayland paste is equally synchronous).
func (c *wlClipboard) Get(kind string) (string, error) {
	if kind == "" {
		kind = "text/plain"
	}
	st := c.st()
	if st == nil {
		return "", fmt.Errorf("wayland: clipboard unavailable")
	}
	st.offerMu.Lock()
	defer st.offerMu.Unlock()
	st.mu.Lock()
	if st.selOffer != 0 && st.ownSource == 0 {
		// Fast path：同一 offer 同一 kind 已缓存过，直接返回，不再走 pipe（第二次粘贴秒回）
		if st.cachedOffer == st.selOffer && st.cachedKind == kind && st.cachedData != "" {
			d := st.cachedData
			st.mu.Unlock()
			return d, nil
		}
		offer := st.selOffer
		st.mu.Unlock()
		buf, err := st.readOffer(offer, kind)
		st.mu.Lock()
		// 保留 offer 不销毁，缓存结果供下一次粘贴秒回；下一次 selection 事件会整体替换并清缓存
		if err == nil {
			st.cachedData = string(buf)
			st.cachedKind = kind
			st.cachedOffer = offer
		}
		st.mu.Unlock()
		if err != nil {
			return "", err
		}
		return string(buf), nil
	}
	// Self-read (we own the clipboard) or no selection: serve the local copy.
	d := st.ownData
	if st.selOffer != 0 {
		// Our own Set broadcast offer — retire it (never received).
		st.queueDestroyLocked(st.selOffer, wlDataOfferDestroy)
		st.selOffer = 0
	}
	st.mu.Unlock()
	if d != "" {
		return d, nil
	}
	return "", fmt.Errorf("wayland: clipboard is empty")
}

// queueDestroyLocked appends p to the pending-destroy queue; st.mu must be
// held. Duplicates are coalesced (see destroyOffer).
func (st *wlDataDeviceState) queueDestroyLocked(p uintptr, op uint32) {
	if p == 0 {
		return
	}
	for _, q := range st.pendingDestroys {
		if q.p == p {
			return
		}
	}
	st.pendingDestroys = append(st.pendingDestroys, pendingDestroy{p: p, op: op})
}

// wakeForDestroy nudges the event loop so pending proxy destructions are
// drained promptly (the loop re-polls every ~16ms anyway).
func (st *wlDataDeviceState) wakeForDestroy() {
	if st == nil || st.win == nil {
		return
	}
	if h := st.win.hostForWake(); h != nil {
		h.WakeUp()
	}
}

// Set writes data of kind ("" → text/plain) to the selection. The payload
// is copied locally and served to pulling peers via wl_data_source.send.
func (c *wlClipboard) Set(kind, data string) error {
	if kind == "" {
		kind = "text/plain"
	}
	st := c.st()
	if st == nil {
		return fmt.Errorf("wayland: clipboard unavailable")
	}
	lib := st.lib
	if lib == nil || st.dd == 0 {
		return fmt.Errorf("wayland: clipboard unavailable")
	}

	srcArgs := []wlArg{argNewID()}
	src := lib.proxyMarshalArrayCtor(st.mgr, wlDataDevMgrCreateSource, &srcArgs[0], lib.ifaceDataSource, 1)
	if src == 0 {
		return fmt.Errorf("wayland: create data source failed")
	}
	if lib.proxyAddListener(src, uintptr(unsafe.Pointer(&st.sourceListener[0])), st.selfPtr) != 0 {
		lib.proxyDestroy(src)
		return fmt.Errorf("wayland: source listener failed")
	}
	pin := append([]byte(kind), 0)
	oargs := []wlArg{argS(cstr(pin))}
	lib.proxyMarshalArrayFlags(src, wlDataSourceOffer, 0, 0, 0, &oargs[0])

	st.mu.Lock()
	old := st.ownSource
	st.ownSource = src
	st.ownData = data
	// 新的本端拥有会覆盖外部缓存，下次 Get 走本端快路径
	st.cachedData = ""
	st.cachedKind = ""
	st.cachedOffer = 0
	serial := st.win.lastSerial.Load()
	st.mu.Unlock()
	if old != 0 {
		st.destroySource(old)
	}
	sargs := []wlArg{argO(src), argU(serial)}
	lib.proxyMarshalArrayFlags(st.dd, wlDataDevSetSelection, 0, 0, 0, &sargs[0])
	lib.displayFlush(st.win.display)
	return nil
}

// clipFor returns the clipboard capability for the wayland host, or nil when
// the compositor lacks wl_data_device_manager (silent degrade: ui must fall
// back to its internal clipboard).
func clipFor(h *wlHost) Clipboard {
	if h == nil || h.win == nil || h.win.dds == nil {
		return nil
	}
	return &wlClipboard{h: h}
}