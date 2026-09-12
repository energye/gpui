//go:build linux

package platform

import (
	"encoding/binary"
	"testing"
	"time"
	"unsafe"
)

func mkHierDataKept(t *testing.T, infos []xiHierarchyInfo) (data, raw []byte) {
	t.Helper()
	n := len(infos)
	raw = make([]byte, n*xiInfoSize)
	for i, in := range infos {
		base := i * xiInfoSize
		binary.LittleEndian.PutUint32(raw[base+xiInfoDeviceOff:], uint32(in.deviceid))
		binary.LittleEndian.PutUint32(raw[base+xiInfoAttachOff:], 0)
		binary.LittleEndian.PutUint32(raw[base+xiInfoUseOff:], uint32(in.use))
		binary.LittleEndian.PutUint32(raw[base+12:], 1)
		binary.LittleEndian.PutUint32(raw[base+xiInfoFlagsOff:], uint32(in.flags))
	}
	data = make([]byte, xiHierEventSize)
	binary.LittleEndian.PutUint32(data[xiHierNumInfoOff:], uint32(n))
	binary.LittleEndian.PutUint64(data[xiHierInfoOff:], uint64(uintptr(unsafe.Pointer(&raw[0]))))
	return data, raw
}

func TestParseXIHierarchy(t *testing.T) {
	infos := []xiHierarchyInfo{
		{deviceid: 10, use: xiUseSlaveKeyboard, flags: xiFlagSlaveAdded},
		{deviceid: 11, use: xiUseSlavePointer, flags: xiFlagSlaveRemoved},
	}
	data, raw := mkHierDataKept(t, infos)
	defer func() { _ = raw }()
	got, ok := parseXIHierarchy(data)
	if !ok || len(got) != 2 {
		t.Fatalf("parse = %v ok=%v, want 2 infos", got, ok)
	}
	if got[0].deviceid != 10 || got[0].use != xiUseSlaveKeyboard || got[0].flags != xiFlagSlaveAdded {
		t.Fatalf("info0 = %+v", got[0])
	}
	if _, ok := parseXIHierarchy(make([]byte, 10)); ok {
		t.Fatal("short buffer must not parse")
	}
	if _, ok := parseXIHierarchy(make([]byte, xiHierEventSize)); ok {
		t.Fatal("zero infos must not parse")
	}
}

func TestXIUseToClass(t *testing.T) {
	if x11UseToClass(xiUseMasterKeyboard) != DeviceKeyboard {
		t.Fatal("master keyboard")
	}
	if x11UseToClass(xiUseSlaveKeyboard) != DeviceKeyboard {
		t.Fatal("slave keyboard")
	}
	if x11UseToClass(xiUseMasterPointer) != DeviceMouse {
		t.Fatal("master pointer")
	}
	if x11UseToClass(xiUseSlavePointer) != DeviceMouse {
		t.Fatal("slave pointer fallback is mouse")
	}
	if x11UseToClass(xiUseFloatingSlave) != DeviceMouse {
		t.Fatal("floating fallback is mouse")
	}
}

func TestXIIsPenName(t *testing.T) {
	for _, n := range []string{"Wacom Pen", "STYLUS", "eraser", "Huion Tablet", "xp-pen Artist"} {
		if !x11IsPenName(n) {
			t.Errorf("%q should be pen", n)
		}
	}
	for _, n := range []string{"AT keyboard", "Virtual core pointer", "Touchscreen", "USB mouse", ""} {
		if x11IsPenName(n) {
			t.Errorf("%q should not be pen", n)
		}
	}
}

func TestXIHierarchyToEvents(t *testing.T) {
	st := &x11State{devClasses: make(map[int]DeviceClass), devNames: make(map[int]string)}
	// Added keyboard (display 0 forces the use fallback, no X query).
	added := []xiHierarchyInfo{{deviceid: 10, use: xiUseSlaveKeyboard, flags: xiFlagSlaveAdded}}
	evs := xiHierarchyToEvents(st, added)
	if len(evs) != 1 || evs[0].Type != EventDeviceAdded || evs[0].DeviceClass != DeviceKeyboard {
		t.Fatalf("added = %+v", evs)
	}
	if st.devClasses[10] != DeviceKeyboard {
		t.Fatalf("cache not stored: %v", st.devClasses)
	}
	// Attach-only is ignored.
	if evs := xiHierarchyToEvents(st, []xiHierarchyInfo{{deviceid: 12, use: xiUseSlavePointer, flags: 1 << 4}}); len(evs) != 0 {
		t.Fatalf("attach must be quiet: %+v", evs)
	}
	// Cached touch removal replays touch even though use says pointer.
	st.devClasses[11] = DeviceTouch
	st.devNames[11] = "touchscreen"
	evs = xiHierarchyToEvents(st, []xiHierarchyInfo{{deviceid: 11, use: xiUseSlavePointer, flags: xiFlagSlaveRemoved}})
	if len(evs) != 1 || evs[0].Type != EventDeviceRemoved || evs[0].DeviceClass != DeviceTouch || evs[0].DeviceName != "touchscreen" {
		t.Fatalf("cached removed = %+v", evs)
	}
	if _, ok := st.devClasses[11]; ok {
		t.Fatal("removed device must leave the cache")
	}
	// Uncached removal falls back to use.
	evs = xiHierarchyToEvents(st, []xiHierarchyInfo{{deviceid: 13, use: xiUseSlaveKeyboard, flags: xiFlagMasterRemoved}})
	if len(evs) != 1 || evs[0].DeviceClass != DeviceKeyboard {
		t.Fatalf("uncached removed = %+v", evs)
	}
	// Master added maps by use.
	evs = xiHierarchyToEvents(st, []xiHierarchyInfo{{deviceid: 2, use: xiUseMasterPointer, flags: xiFlagMasterAdded}})
	if len(evs) != 1 || evs[0].Type != EventDeviceAdded || evs[0].DeviceClass != DeviceMouse {
		t.Fatalf("master added = %+v", evs)
	}
}

func TestDeviceEventString(t *testing.T) {
	if (Event{Type: EventDeviceAdded}.Type.String()) != "device-added" {
		t.Fatalf("added string = %q", Event{Type: EventDeviceAdded}.Type.String())
	}
	if (Event{Type: EventDeviceRemoved}.Type.String()) != "device-removed" {
		t.Fatalf("removed string = %q", Event{Type: EventDeviceRemoved}.Type.String())
	}
	if DevicePen.String() != "pen" || DeviceTouch.String() != "touch" {
		t.Fatalf("class strings = %q %q", DevicePen, DeviceTouch)
	}
}

func TestXIHierarchyProbeGating(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, ok := win.Host().(*x11Host)
	if !ok || h.st == nil {
		t.Fatalf("not an x11 host")
	}
	st := h.st
	if st.xiMajor == 0 && st.xiHierarchy {
		t.Fatal("hierarchy selected without an XI opcode")
	}
	if st.xiTouch && !st.xiHierarchy {
		t.Fatal("touch selected but hierarchy missing (same server should give both)")
	}
	// Quiet without hardware changes.
	soak := x11Collect(h, 400*time.Millisecond, nil)
	for _, e := range soak {
		if e.Type == EventDeviceAdded || e.Type == EventDeviceRemoved {
			t.Fatalf("spurious device event without hot-plug: %+v", e)
		}
	}
	t.Logf("xi hierarchy selected=%v major=%d (live plug verification needs hardware)", st.xiHierarchy, st.xiMajor)
}
