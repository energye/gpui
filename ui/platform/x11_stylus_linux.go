//go:build linux

package platform

import (
	"math"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// X11 pen pressure (S6-P2 E 组 X11 侧).
//
// XI2 ButtonPress(4)/ButtonRelease(5)/Motion(6) on pen slave devices decode
// into EventStylus: Down on press, Up on release, Move on motion (hover and
// drag share Move). Pressure comes from the device's pressure valuator
// normalized to 0–1; devices without a pressure sensor report 1 per the E 组
// convention (GDK already normalizes pressure the same way, only the missing
// fallback differs). Tilt X/Y ride raw in degrees (0 = unknown); the eraser
// tool is name-detected (separate slave carrying "eraser").
//
// Selection is per pen slave (not AllMaster) so ordinary mouse motion never
// enters the XI path: core Button/Motion already carry the mouse, XI only
// carries pens. Pen slaves are enumerated at Create and on hierarchy Added;
// removal drops the valuator cache but keeps the small stylus ID stable.

const (
	xiButtonPress   = 4
	xiButtonRelease = 5
	xiMotion        = 6

	xiClassValuator = 2
)

// XIDeviceEvent valuator header offsets (LP64, see XInput2.h).
const (
	xiDevSourceOff = 52
	xiDevDetailOff = 56
	xiDevXOff      = 104
	xiDevYOff      = 112

	xiValMaskLenOff = 144
	xiValMaskOff    = 152
	xiValValuesOff  = 160
	xiDevEventMin   = 168
)

// XIValuatorClassInfo offsets (LP64).
const (
	xiValNumberOff = 8
	xiValLabelOff  = 16
	xiValMinOff    = 24
	xiValMaxOff    = 32
)

// stylusAxes caches one pen slave's valuator mapping.
type stylusAxes struct {
	name        string
	eraser      bool
	pressureIdx int // -1 = no sensor
	pressureMin float64
	pressureMax float64
	tiltXIdx    int // -1 = unknown
	tiltYIdx    int // -1 = unknown
}

var (
	atomNameOnce  sync.Once
	atomNameLib   uintptr
	xXGetAtomName func(dpy uintptr, atom uintptr) *byte
	xXFreeStylus  func(ptr unsafe.Pointer) int

	atomNameMu    sync.Mutex
	atomNameCache = make(map[x11AtomKey]string)
)

// x11AtomKey scopes the XGetAtomName cache per server connection: atom IDs
// are server-allocated and may repeat across Displays with other names.
type x11AtomKey struct {
	dpy  uintptr
	atom uintptr
}

func x11ResolveAtomName() bool {
	atomNameOnce.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			lib, err = purego.Dlopen("libX11.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				return
			}
		}
		if _, err := purego.Dlsym(lib, "XGetAtomName"); err != nil {
			return
		}
		if _, err := purego.Dlsym(lib, "XFree"); err != nil {
			return
		}
		purego.RegisterLibFunc(&xXGetAtomName, lib, "XGetAtomName")
		purego.RegisterLibFunc(&xXFreeStylus, lib, "XFree")
		atomNameLib = lib
	})
	return atomNameLib != 0 && xXGetAtomName != nil
}

func x11StylusAtomName(dpy uintptr, atom uintptr) string {
	if atom == 0 || dpy == 0 || !x11ResolveAtomName() || xXGetAtomName == nil {
		return ""
	}
	key := x11AtomKey{dpy: dpy, atom: atom}
	atomNameMu.Lock()
	if s, ok := atomNameCache[key]; ok {
		atomNameMu.Unlock()
		return s
	}
	atomNameMu.Unlock()
	p := xXGetAtomName(dpy, atom)
	if p == nil {
		return ""
	}
	s := x11CString(p)
	if xXFreeStylus != nil {
		xXFreeStylus(unsafe.Pointer(p))
	}
	atomNameMu.Lock()
	atomNameCache[key] = s
	atomNameMu.Unlock()
	return s
}

func x11IsEraserName(name string) bool {
	return strings.Contains(strings.ToLower(name), "eraser")
}

// x11SelectStylusDevice selects ButtonPress/Release/Motion on one pen slave.
// No version negotiation here: Create already fixed XI 2.2 on this Display
// (re-querying a different version is a fatal BadValue).
func x11SelectStylusDevice(dpy, win uintptr, slaveID int) bool {
	if dpy == 0 || win == 0 || slaveID <= 0 || !x11ResolveXI() {
		return false
	}
	if xXISelectEvents == nil {
		return false
	}
	mask := [1]byte{(1 << 4) | (1 << 5) | (1 << 6)}
	sel := xiEventMask{deviceid: int32(slaveID), maskLen: int32(len(mask)), mask: &mask[0]}
	return xXISelectEvents(dpy, win, unsafe.Pointer(&sel), 1) == 0
}

// stylusAxesFromClasses folds one device's valuator classes into the cached
// mapping. Shared by the single-device query and the Create-time seed so the
// seed parses its one AllDevices reply inline instead of re-querying every
// slave (N+1 round-trips → 1).
func stylusAxesFromClasses(dpy uintptr, name string, classes []uintptr) *stylusAxes {
	ax := &stylusAxes{name: name, pressureIdx: -1, tiltXIdx: -1, tiltYIdx: -1}
	ax.eraser = x11IsEraserName(name)
	for _, cp := range classes {
		if cp == 0 {
			continue
		}
		if typ := int(*(*int32)(unsafe.Pointer(uintptr(cp)))); typ != xiClassValuator {
			continue
		}
		raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(cp))), 56)
		if len(raw) < 56 {
			continue
		}
		num := int(readI32(raw, xiValNumberOff))
		labelAtom := uintptr(readU64(raw, xiValLabelOff))
		min := math.Float64frombits(readU64(raw, xiValMinOff))
		max := math.Float64frombits(readU64(raw, xiValMaxOff))
		label := strings.ToLower(x11StylusAtomName(dpy, labelAtom))
		switch {
		case strings.Contains(label, "pressure"):
			ax.pressureIdx = num
			ax.pressureMin, ax.pressureMax = min, max
		case strings.Contains(label, "tilt") && strings.Contains(label, "x"):
			ax.tiltXIdx = num
		case strings.Contains(label, "tilt") && strings.Contains(label, "y"):
			ax.tiltYIdx = num
		}
	}
	return ax
}

func x11IsPenAxes(name string, ax *stylusAxes) bool {
	return ax != nil && (x11IsPenName(name) || ax.pressureIdx >= 0 || ax.tiltXIdx >= 0 || ax.tiltYIdx >= 0)
}

// x11QueryStylusAxes reads one slave's valuator layout. isPen is true for
// pen-named devices or any device carrying pressure/tilt valuators.
func x11QueryStylusAxes(dpy uintptr, slaveID int) (*stylusAxes, bool) {
	if dpy == 0 || slaveID <= 0 || !x11ResolveXIDeviceQuery() || xXIQueryDevice == nil {
		return nil, false
	}
	var ndev int32
	info := xXIQueryDevice(dpy, int32(slaveID), &ndev)
	if info == 0 || ndev <= 0 {
		return nil, false
	}
	defer func() {
		if xXIFreeDeviceInfo != nil {
			xXIFreeDeviceInfo(info)
		}
	}()
	ax := &stylusAxes{pressureIdx: -1, tiltXIdx: -1, tiltYIdx: -1}
	found := false
	count := int(ndev)
	for i := 0; i < count; i++ {
		entry := unsafe.Slice((*byte)(unsafe.Pointer(info+uintptr(i*xiDevSize))), xiDevSize)
		if count > 1 && int(readI32(entry, xiDevIDOff)) != slaveID {
			continue
		}
		var name string
		if p := readU64(entry, xiDevNameOff); p != 0 {
			name = x11CString((*byte)(unsafe.Pointer(uintptr(p))))
		}
		ncls := int(readI32(entry, xiDevNumClassesOff))
		clsPtr := readU64(entry, xiDevClassesOff)
		var classes []uintptr
		if ncls > 0 && ncls < 64 && clsPtr != 0 {
			classes = unsafe.Slice((*uintptr)(unsafe.Pointer(uintptr(clsPtr))), ncls)
		}
		ax = stylusAxesFromClasses(dpy, name, classes)
		found = true
		break
	}
	if !found {
		return nil, false
	}
	if !x11IsPenAxes(ax.name, ax) {
		return ax, false
	}
	// Label-less drivers (pressure valuator 2 by convention) still count:
	// keep -1 so events fall back to 1 rather than misreading another axis.
	return ax, true
}

// x11EnsureStylusDevice caches axes and installs the XI selection for one
// slave when it is a pen. Safe on the event pump thread; no-ops without XI.
func x11EnsureStylusDevice(st *x11State, slaveID int) {
	if st == nil || st.display == 0 || st.window == 0 || st.xiMajor == 0 || slaveID <= 0 {
		return
	}
	st.stylusMu.Lock()
	if st.stylusSel == nil {
		st.stylusSel = make(map[int]bool)
	}
	if st.stylusSel[slaveID] {
		st.stylusMu.Unlock()
		return
	}
	st.stylusMu.Unlock()
	ax, isPen := x11QueryStylusAxes(st.display, slaveID)
	if !isPen || ax == nil {
		return
	}
	if !x11SelectStylusDevice(st.display, st.window, slaveID) {
		return
	}
	st.stylusMu.Lock()
	if st.stylusInfo == nil {
		st.stylusInfo = make(map[int]*stylusAxes)
	}
	st.stylusInfo[slaveID] = ax
	st.stylusSel[slaveID] = true
	st.stylusMu.Unlock()
}

// x11SeedStylus enumerates current slaves and selects pen devices. The one
// AllDevices reply is parsed inline (no per-slave re-query); hot-plug Added
// still resolves its single slave via x11EnsureStylusDevice.
func x11SeedStylus(st *x11State) {
	if st == nil || st.display == 0 || st.window == 0 || st.xiMajor == 0 {
		return
	}
	if !x11ResolveXIDeviceQuery() || xXIQueryDevice == nil {
		return
	}
	var ndev int32
	info := xXIQueryDevice(st.display, xiAllDevices, &ndev)
	if info == 0 || ndev <= 0 || ndev > 128 {
		return
	}
	defer func() {
		if xXIFreeDeviceInfo != nil {
			xXIFreeDeviceInfo(info)
		}
	}()
	for i := 0; i < int(ndev); i++ {
		entry := unsafe.Slice((*byte)(unsafe.Pointer(info+uintptr(i*xiDevSize))), xiDevSize)
		id := int(readI32(entry, xiDevIDOff))
		if id <= 0 {
			continue
		}
		var name string
		if p := readU64(entry, xiDevNameOff); p != 0 {
			name = x11CString((*byte)(unsafe.Pointer(uintptr(p))))
		}
		ncls := int(readI32(entry, xiDevNumClassesOff))
		clsPtr := readU64(entry, xiDevClassesOff)
		var classes []uintptr
		if ncls > 0 && ncls < 64 && clsPtr != 0 {
			classes = unsafe.Slice((*uintptr)(unsafe.Pointer(uintptr(clsPtr))), ncls)
		}
		ax := stylusAxesFromClasses(st.display, name, classes)
		if !x11IsPenAxes(name, ax) {
			continue
		}
		if !x11SelectStylusDevice(st.display, st.window, id) {
			continue
		}
		st.stylusMu.Lock()
		if st.stylusInfo == nil {
			st.stylusInfo = make(map[int]*stylusAxes)
		}
		if st.stylusSel == nil {
			st.stylusSel = make(map[int]bool)
		}
		st.stylusInfo[id] = ax
		st.stylusSel[id] = true
		st.stylusMu.Unlock()
	}
}

// x11StylusOnHierarchy selects pens arriving via hot-plug and drops caches
// for removed ones (small IDs stay stable for the session).
func x11StylusOnHierarchy(st *x11State, infos []xiHierarchyInfo) {
	if st == nil {
		return
	}
	for _, in := range infos {
		added := in.flags&(xiFlagSlaveAdded|xiFlagMasterAdded) != 0
		removed := in.flags&(xiFlagSlaveRemoved|xiFlagMasterRemoved) != 0
		if added {
			x11EnsureStylusDevice(st, in.deviceid)
			continue
		}
		if removed {
			st.stylusMu.Lock()
			delete(st.stylusInfo, in.deviceid)
			delete(st.stylusSel, in.deviceid)
			st.stylusMu.Unlock()
		}
	}
}

// valuatorValue extracts valuator idx from an XI mask/values pair.
func valuatorValue(mask []byte, values []float64, idx int) (float64, bool) {
	if idx < 0 || idx/8 >= len(mask) {
		return 0, false
	}
	if mask[idx/8]&(1<<uint(idx%8)) == 0 {
		return 0, false
	}
	pos := 0
	for i := 0; i < idx; i++ {
		if i/8 < len(mask) && mask[i/8]&(1<<uint(i%8)) != 0 {
			pos++
		}
	}
	if pos >= len(values) {
		return 0, false
	}
	return values[pos], true
}

func normalizeStylusPressure(v, min, max float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 1
	}
	if max <= min {
		return 1
	}
	p := (v - min) / (max - min)
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return 1
	}
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// parseXIStylus decodes one XIDeviceEvent into stylus fields. Pure parser,
// no X calls, unit-testable. Missing pressure falls back to 1; missing tilt
// stays 0.
func parseXIStylus(data []byte, evtype int, ax *stylusAxes) (phase PointerKind, x, y, pressure, tiltX, tiltY float64, detail int, ok bool) {
	switch evtype {
	case xiButtonPress:
		phase = PointerDown
	case xiButtonRelease:
		phase = PointerUp
	case xiMotion:
		phase = PointerMove
	default:
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	if len(data) < xiDevEventMin {
		return 0, 0, 0, 0, 0, 0, 0, false
	}
	detail = int(readI32(data, xiDevDetailOff))
	x = math.Float64frombits(readU64(data, xiDevXOff))
	y = math.Float64frombits(readU64(data, xiDevYOff))
	pressure = 1
	maskLen := int(readI32(data, xiValMaskLenOff))
	maskPtr := readU64(data, xiValMaskOff)
	valsPtr := readU64(data, xiValValuesOff)
	if maskLen <= 0 || maskLen > 32 || maskPtr == 0 || valsPtr == 0 {
		return phase, x, y, pressure, 0, 0, detail, true
	}
	mask := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(maskPtr))), maskLen)
	bits := 0
	for _, b := range mask {
		for k := 0; k < 8; k++ {
			if b&(1<<uint(k)) != 0 {
				bits++
			}
		}
	}
	if bits <= 0 || bits > 64 {
		return phase, x, y, pressure, 0, 0, detail, true
	}
	values := unsafe.Slice((*float64)(unsafe.Pointer(uintptr(valsPtr))), bits)
	if ax != nil {
		if v, present := valuatorValue(mask, values, ax.pressureIdx); present {
			pressure = normalizeStylusPressure(v, ax.pressureMin, ax.pressureMax)
		}
		if v, present := valuatorValue(mask, values, ax.tiltXIdx); present {
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				tiltX = v
			}
		}
		if v, present := valuatorValue(mask, values, ax.tiltYIdx); present {
			if !math.IsNaN(v) && !math.IsInf(v, 0) {
				tiltY = v
			}
		}
	}
	return phase, x, y, pressure, tiltX, tiltY, detail, true
}

// stylusID maps a slave sourceid to the small tool slot (0 = primary pen).
func (st *x11State) stylusID(sourceid int) int {
	st.stylusMu.Lock()
	defer st.stylusMu.Unlock()
	if st.stylusIDs == nil {
		st.stylusIDs = make(map[int]int)
	}
	if id, ok := st.stylusIDs[sourceid]; ok {
		return id
	}
	id := st.stylusNext
	st.stylusIDs[sourceid] = id
	st.stylusNext++
	return id
}

// decodeXIStylus handles one GenericEvent: XI ButtonPress/Release/Motion from
// a selected pen slave → EventStylus. Anything else is ignored.
func (h *x11Host) decodeXIStylus(buf []byte) (Event, bool) {
	st := h.st
	if st == nil || st.xiMajor == 0 || len(buf) < 56 {
		return Event{}, false
	}
	if int32(readU32(buf, xiEvExtensionOff)) != st.xiMajor {
		return Event{}, false
	}
	evtype := int(readU32(buf, xiEvTypeOff))
	if evtype != xiButtonPress && evtype != xiButtonRelease && evtype != xiMotion {
		return Event{}, false
	}
	if xXIGetEventData == nil || xXIFreeEventData == nil {
		return Event{}, false
	}
	if xXIGetEventData(st.display, unsafe.Pointer(&buf[0])) == 0 {
		return Event{}, false
	}
	defer xXIFreeEventData(st.display, unsafe.Pointer(&buf[0]))
	dataPtr := *(*unsafe.Pointer)(unsafe.Pointer(&buf[48]))
	if dataPtr == nil {
		return Event{}, false
	}
	// Source slave first: unselected masters/mice never reach here (their
	// motion was never selected), but a lazy query keeps the gate honest.
	hdr := unsafe.Slice((*byte)(dataPtr), xiDevEventMin)
	sourceid := int(readI32(hdr, xiDevSourceOff))
	st.stylusMu.Lock()
	ax := st.stylusInfo[sourceid]
	selected := st.stylusSel[sourceid]
	st.stylusMu.Unlock()
	if ax == nil && !selected {
		x11EnsureStylusDevice(st, sourceid)
		st.stylusMu.Lock()
		ax = st.stylusInfo[sourceid]
		selected = st.stylusSel[sourceid]
		st.stylusMu.Unlock()
		if ax == nil && !selected {
			return Event{}, false
		}
	}
	data := unsafe.Slice((*byte)(dataPtr), xiDevEventMin)
	phase, x, y, pressure, tiltX, tiltY, _, ok := parseXIStylus(data, evtype, ax)
	if !ok {
		return Event{}, false
	}
	eraser := ax != nil && ax.eraser
	return Event{
		Type:           EventStylus,
		Pointer:        phase,
		X:              x,
		Y:              y,
		StylusID:       st.stylusID(sourceid),
		StylusPressure: pressure,
		StylusTiltX:    tiltX,
		StylusTiltY:    tiltY,
		StylusEraser:   eraser,
	}, true
}
