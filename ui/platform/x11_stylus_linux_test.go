//go:build linux

package platform

import (
	"encoding/binary"
	"math"
	"testing"
	"unsafe"
)

// mkStylusBytes builds an XIDeviceEvent image with the valuator header wired
// to live mask/values. Callers must keep mask/values alive for the parse.
func mkStylusBytes(detail int32, x, y float64, mask []byte, values []float64) []byte {
	buf := make([]byte, 256)
	binary.LittleEndian.PutUint32(buf[56:], uint32(detail))
	binary.LittleEndian.PutUint64(buf[104:], math.Float64bits(x))
	binary.LittleEndian.PutUint64(buf[112:], math.Float64bits(y))
	binary.LittleEndian.PutUint32(buf[144:], uint32(len(mask)))
	if len(mask) > 0 {
		*(*uintptr)(unsafe.Pointer(&buf[152])) = uintptr(unsafe.Pointer(&mask[0]))
	}
	if len(values) > 0 {
		*(*uintptr)(unsafe.Pointer(&buf[160])) = uintptr(unsafe.Pointer(&values[0]))
	}
	return buf
}

func TestParseXIStylusPhases(t *testing.T) {
	ax := &stylusAxes{pressureIdx: -1, tiltXIdx: -1, tiltYIdx: -1}
	downMask := []byte{0}
	downVals := []float64{}
	phase, x, y, p, tx, ty, detail, ok := parseXIStylus(mkStylusBytes(1, 30.5, 40.25, downMask, downVals), xiButtonPress, ax)
	if !ok || phase != PointerDown {
		t.Fatalf("press = %v ok=%v", phase, ok)
	}
	if x != 30.5 || y != 40.25 || detail != 1 {
		t.Fatalf("press payload = (%v,%v) detail=%d", x, y, detail)
	}
	if p != 1 || tx != 0 || ty != 0 {
		t.Fatalf("no-sensor fallback = p=%v tilt=(%v,%v), want 1/0/0", p, tx, ty)
	}
	if _, _, _, _, _, _, _, ok := parseXIStylus(mkStylusBytes(1, 0, 0, nil, nil), xiButtonRelease, ax); !ok {
		t.Fatal("release must parse")
	} else {
		ph, _, _, _, _, _, _, _ := parseXIStylus(mkStylusBytes(1, 0, 0, nil, nil), xiButtonRelease, ax)
		if ph != PointerUp {
			t.Fatalf("release phase = %v, want up", ph)
		}
	}
	if ph, _, _, _, _, _, _, ok := parseXIStylus(mkStylusBytes(0, 1, 2, nil, nil), xiMotion, ax); !ok || ph != PointerMove {
		t.Fatalf("motion = %v ok=%v, want move", ph, ok)
	}
	if _, _, _, _, _, _, _, ok := parseXIStylus(mkStylusBytes(0, 0, 0, nil, nil), 99, ax); ok {
		t.Fatal("unknown evtype must not parse")
	}
	if _, _, _, _, _, _, _, ok := parseXIStylus(make([]byte, 64), xiButtonPress, ax); ok {
		t.Fatal("short buffer must not parse")
	}
}

func TestParseXIStylusPressure(t *testing.T) {
	ax := &stylusAxes{pressureIdx: 2, pressureMin: 0, pressureMax: 1023, tiltXIdx: -1, tiltYIdx: -1}
	// Valuator 2 set with two lower valuators present: values order follows bits.
	mask := []byte{0b00000111}
	values := []float64{100, 200, 512}
	_, _, _, p, _, _, _, ok := parseXIStylus(mkStylusBytes(1, 0, 0, mask, values), xiButtonPress, ax)
	if !ok {
		t.Fatal("pressure event must parse")
	}
	want := 512.0 / 1023.0
	if math.Abs(p-want) > 1e-9 {
		t.Fatalf("pressure = %v, want %v", p, want)
	}
	// Pressure bit clear → sensor present but absent in this sample → fallback 1.
	mask2 := []byte{0b00000011}
	values2 := []float64{100, 200}
	_, _, _, p2, _, _, _, _ := parseXIStylus(mkStylusBytes(1, 0, 0, mask2, values2), xiButtonPress, ax)
	if p2 != 1 {
		t.Fatalf("missing pressure in sample = %v, want 1", p2)
	}
	// Invalid range → fallback 1 (never NaN to the app).
	bad := &stylusAxes{pressureIdx: 0, pressureMin: 5, pressureMax: 5}
	mask3 := []byte{0x01}
	values3 := []float64{5}
	_, _, _, p3, _, _, _, _ := parseXIStylus(mkStylusBytes(1, 0, 0, mask3, values3), xiButtonPress, bad)
	if p3 != 1 {
		t.Fatalf("bad range = %v, want 1", p3)
	}
	// Tilt rides raw, 0 when absent.
	tiltAxes := &stylusAxes{pressureIdx: -1, tiltXIdx: 3, tiltYIdx: 4}
	mask4 := []byte{0b00011000}
	values4 := []float64{12.5, -7.25}
	_, _, _, _, tx, ty, _, _ := parseXIStylus(mkStylusBytes(0, 0, 0, mask4, values4), xiMotion, tiltAxes)
	if tx != 12.5 || ty != -7.25 {
		t.Fatalf("tilt = (%v,%v), want (12.5,-7.25)", tx, ty)
	}
}

func TestValuatorValueOrder(t *testing.T) {
	mask := []byte{0b00010101}
	values := []float64{10, 20, 30}
	if v, ok := valuatorValue(mask, values, 0); !ok || v != 10 {
		t.Fatalf("valuator 0 = %v ok=%v", v, ok)
	}
	if v, ok := valuatorValue(mask, values, 2); !ok || v != 20 {
		t.Fatalf("valuator 2 = %v ok=%v", v, ok)
	}
	if v, ok := valuatorValue(mask, values, 4); !ok || v != 30 {
		t.Fatalf("valuator 4 = %v ok=%v", v, ok)
	}
	if _, ok := valuatorValue(mask, values, 1); ok {
		t.Fatal("valuator 1 bit clear must miss")
	}
	if _, ok := valuatorValue(mask, values, -1); ok {
		t.Fatal("negative idx must miss")
	}
}

func TestNormalizeStylusPressure(t *testing.T) {
	if p := normalizeStylusPressure(512, 0, 1023); math.Abs(p-512.0/1023.0) > 1e-9 {
		t.Fatalf("mid = %v", p)
	}
	if p := normalizeStylusPressure(-10, 0, 100); p != 0 {
		t.Fatalf("low clamp = %v", p)
	}
	if p := normalizeStylusPressure(200, 0, 100); p != 1 {
		t.Fatalf("high clamp = %v", p)
	}
	if p := normalizeStylusPressure(1, 1, 1); p != 1 {
		t.Fatalf("bad range = %v", p)
	}
	if p := normalizeStylusPressure(math.NaN(), 0, 100); p != 1 {
		t.Fatalf("NaN = %v", p)
	}
}

func TestStylusIDStable(t *testing.T) {
	st := &x11State{}
	if id := st.stylusID(12); id != 0 {
		t.Fatalf("first pen id = %d, want 0", id)
	}
	if id := st.stylusID(12); id != 0 {
		t.Fatalf("same slave id = %d, want stable 0", id)
	}
	if id := st.stylusID(15); id != 1 {
		t.Fatalf("second pen id = %d, want 1", id)
	}
}

func TestStylusHierarchyRemoveDrops(t *testing.T) {
	st := &x11State{
		stylusInfo: map[int]*stylusAxes{12: {pressureIdx: 0}},
		stylusSel:  map[int]bool{12: true},
		stylusIDs:  map[int]int{12: 0},
		stylusNext: 1,
	}
	x11StylusOnHierarchy(st, []xiHierarchyInfo{{deviceid: 12, flags: xiFlagSlaveRemoved}})
	if st.stylusInfo[12] != nil || st.stylusSel[12] {
		t.Fatal("removed pen must drop axes/selection")
	}
	if st.stylusIDs[12] != 0 {
		t.Fatal("small IDs stay stable across removal")
	}
}

func TestStylusEventString(t *testing.T) {
	if (Event{Type: EventStylus}.Type.String()) != "stylus" {
		t.Fatalf("stylus string = %q", Event{Type: EventStylus}.Type.String())
	}
}

func TestStylusAxesFromClassesPenName(t *testing.T) {
	// No valuator classes and no X calls: pen-ness falls back to the name.
	ax := stylusAxesFromClasses(0, "Wacom Pen", nil)
	if ax.pressureIdx != -1 || ax.tiltXIdx != -1 || ax.eraser {
		t.Fatalf("empty classes = %+v", ax)
	}
	if !x11IsPenAxes(ax.name, ax) {
		t.Fatal("pen name must count without sensors")
	}
	mouse := stylusAxesFromClasses(0, "AT keyboard", nil)
	if x11IsPenAxes(mouse.name, mouse) {
		t.Fatal("keyboard with no sensors must not count as pen")
	}
}
