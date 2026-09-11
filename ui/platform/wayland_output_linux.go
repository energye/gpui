//go:build linux

package platform

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// wl_output scale tracking — P0 scale上报.
//
// The compositor owns pixels per output; the client learns its scale from
// wl_output.scale events scoped by wl_surface.enter/leave. Effective window
// scale = max scale over entered outputs (default 1); changes queue
// EventScale. Everything degrades silently: no outputs bound (old
// compositor) → scale stays 1, zero behavior change.

// wl_output event opcodes.
const (
	wlOutputGeometry    = 0
	wlOutputMode        = 1
	wlOutputDone        = 2
	wlOutputScale       = 3
	wlOutputName        = 4
	wlOutputDescription = 5
)

// wl_surface enter/leave opcodes (the surface listener carries only these).
const (
	wlSurfaceEnter = 0
	wlSurfaceLeave = 1
)

// wlOutputGlobal is one captured wl_output registry global.
type wlOutputGlobal struct {
	name    uint32
	version uint32
}

// wlOutputState tracks one bound wl_output global.
type wlOutputState struct {
	win      *wlWin
	proxy    uintptr
	name     uint32 // registry global id
	scale    int32  // last advertised factor (default 1)
	listener [6]uintptr
	selfPtr  uintptr
}

func outputFrom(data uintptr) *wlOutputState {
	if data == 0 {
		return nil
	}
	return (*wlOutputState)(unsafe.Pointer(data))
}

// ensureOutputMaps lazily creates the scale-tracking tables. The create path
// can reach bindOutputs with a zero wlWin, and callbacks assume non-nil maps
// (a missing table panics on first real output — only reachable against a
// live compositor, which is why the callback-level tests never caught it).
func (w *wlWin) ensureOutputMaps() {
	if w == nil {
		return
	}
	if w.outputs == nil {
		w.outputs = make(map[uint32]*wlOutputState)
	}
	if w.enteredOutputs == nil {
		w.enteredOutputs = make(map[uint32]bool)
	}
	if w.outputsByProxy == nil {
		w.outputsByProxy = make(map[uintptr]uint32)
	}
}

// bindOutputs binds every captured wl_output global and listens for scale.
// Called once after the registry roundtrip; a compositor without outputs
// (or bind failures) leaves scale tracking off — ScaleFactor stays 1.
func (w *wlWin) bindOutputs() {
	if w == nil || w.lib == nil || w.registry == 0 || w.lib.ifaceOutput == 0 {
		return
	}
	if len(w.outGlobals) == 0 {
		return
	}
	w.ensureOutputMaps()
	for _, g := range w.outGlobals {
		if _, dup := w.outputs[g.name]; dup {
			continue
		}
		proxy := w.bind(w.registry, g.name, w.lib.ifaceOutput, minU32(g.version, 3))
		if proxy == 0 {
			continue
		}
		st := &wlOutputState{win: w, proxy: proxy, name: g.name, scale: 1}
		st.selfPtr = uintptr(unsafe.Pointer(st))
		st.listener[wlOutputGeometry] = purego.NewCallback(wlOutputGeometryCB)
		st.listener[wlOutputMode] = purego.NewCallback(wlOutputModeCB)
		st.listener[wlOutputDone] = purego.NewCallback(wlOutputDoneCB)
		st.listener[wlOutputScale] = purego.NewCallback(wlOutputScaleCB)
		st.listener[wlOutputName] = purego.NewCallback(wlOutputNameCB)
		st.listener[wlOutputDescription] = purego.NewCallback(wlOutputDescriptionCB)
		if w.lib.proxyAddListener(proxy, uintptr(unsafe.Pointer(&st.listener[0])), st.selfPtr) != 0 {
			w.lib.proxyDestroy(proxy)
			continue
		}
		w.outputs[g.name] = st
		w.outputsByProxy[proxy] = g.name
	}
}

// installSurfaceListener attaches the enter/leave listener to the content
// surface so output membership (hence scale) tracks window moves across
// outputs. Re-installed on every surface-stack re-create (Show path).
func (w *wlWin) installSurfaceListener() {
	if w == nil || w.lib == nil || w.surface == 0 {
		return
	}
	if w.surfListener[0] == 0 {
		w.surfListener[0] = purego.NewCallback(wlSurfaceEnterCB)
		w.surfListener[1] = purego.NewCallback(wlSurfaceLeaveCB)
	}
	w.lib.proxyAddListener(w.surface, uintptr(unsafe.Pointer(&w.surfListener[0])), w.selfPtr)
}

// evaluateScale recomputes the effective window scale from entered outputs
// and queues EventScale on change. Runs on the event thread.
func (w *wlWin) evaluateScale() {
	if w == nil {
		return
	}
	eff := int32(1)
	for name := range w.enteredOutputs {
		if st, ok := w.outputs[name]; ok && st != nil && st.scale > eff {
			eff = st.scale
		}
	}
	h := w.hostRef
	if h == nil {
		return
	}
	h.mu.Lock()
	changed := h.scale != float64(eff)
	h.scale = float64(eff)
	h.mu.Unlock()
	if changed {
		w.focusMu.Lock()
		w.focusEvents = append(w.focusEvents, Event{Type: EventScale, Scale: float64(eff)})
		w.focusMu.Unlock()
	}
}

// wlOutputScaleCB: scale(int32 factor).
func wlOutputScaleCB(data, output, factor uintptr) {
	st := outputFrom(data)
	if st == nil || st.win == nil {
		return
	}
	st.scale = int32(factor)
	st.win.evaluateScale()
}

// wlSurfaceEnterCB: enter(wl_output).
func wlSurfaceEnterCB(data, surface, output uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	name, ok := w.outputsByProxy[output]
	if !ok {
		return // unknown output (unbound or removed) — keep current scale
	}
	if w.enteredOutputs == nil {
		w.enteredOutputs = make(map[uint32]bool)
	}
	w.enteredOutputs[name] = true
	w.evaluateScale()
}

// wlSurfaceLeaveCB: leave(wl_output).
func wlSurfaceLeaveCB(data, surface, output uintptr) {
	w := winFrom(data)
	if w == nil {
		return
	}
	if name, ok := w.outputsByProxy[output]; ok {
		delete(w.enteredOutputs, name)
		w.evaluateScale()
	}
}

func wlOutputGeometryCB(data, output, x, y, pw, ph, sub, make, model, transform uintptr) {
}
func wlOutputModeCB(data, output, flags, width, height, refresh uintptr) {}
func wlOutputDoneCB(data, output uintptr)                                {}
func wlOutputNameCB(data, output, name uintptr)                          {}
func wlOutputDescriptionCB(data, output, description uintptr)            {}
