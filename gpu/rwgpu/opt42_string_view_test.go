package rwgpu

import (
	"testing"
	"unsafe"
)

// TestOpt42_StringToStringView_NoAllocWarm ensures labels convert without
// allocating a []byte copy (class A opt42).
func TestOpt42_StringToStringView_NoAllocWarm(t *testing.T) {
	label := "session_surface_pass"
	// Cold
	sv := stringToStringView(label)
	if sv.Length != uintptr(len(label)) {
		t.Fatalf("len=%d want %d", sv.Length, len(label))
	}
	if sv.Data == 0 {
		t.Fatal("nil data")
	}
	// Must point at string data
	if sv.Data != uintptr(unsafe.Pointer(unsafe.StringData(label))) {
		t.Fatal("data not string backing")
	}
	allocs := testing.AllocsPerRun(500, func() {
		sv := stringToStringView(label)
		if sv.Length == 0 {
			t.Fatal("empty")
		}
		_ = sv.Data
	})
	if allocs != 0 {
		t.Fatalf("stringToStringView allocs=%v want 0", allocs)
	}
	// empty
	if stringToStringView("").Data != 0 && stringToStringView("").Length != 0 {
		// EmptyStringView may use zero data
	}
	ev := stringToStringView("")
	if ev.Length != 0 {
		t.Fatalf("empty length=%d", ev.Length)
	}
}
