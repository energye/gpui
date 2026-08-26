package rwgpu

import (
	"runtime"
	"testing"
	"unsafe"
)

// TestStringViewToString_Cases covers the decode branches and pins the copy
// semantics: the returned string must own its bytes, not alias the source.
func TestStringViewToString_Cases(t *testing.T) {
	msg := "validation error"
	b := append([]byte(msg), 0) // NUL-terminated like C buffers
	ptr := uintptr(unsafe.Pointer(&b[0]))

	if got := stringViewToString(StringView{Data: 0, Length: 10}); got != "" {
		t.Errorf("nil data: %q", got)
	}
	if got := stringViewToString(StringView{Data: ptr, Length: 0}); got != "" {
		t.Errorf("zero length: %q", got)
	}
	if got := stringViewToString(StringView{Data: ptr, Length: ^uintptr(0)}); got != msg {
		t.Errorf("STRLEN decode=%q want %q", got, msg)
	}
	if got := stringViewToString(StringView{Data: ptr, Length: 1<<20 + 1}); got != "" {
		t.Errorf("oversize length: %q", got)
	}

	got := stringViewToString(StringView{Data: ptr, Length: uintptr(len(msg))})
	if got != msg {
		t.Fatalf("explicit length decode=%q want %q", got, msg)
	}
	// Mutating the source must not change the returned string.
	b[0] = 'X'
	if got != msg {
		t.Fatalf("result aliases source: became %q after source mutation", got)
	}
}

// TestAdapterInfo_StringFieldsSurviveFree guards the X01 hole end to end:
// Adapter.Info frees the C buffer (procAdapterInfoFreeMembers) before
// returning, so the Go-side strings must be independent copies. Reads after
// the return would hit reused memory if any field were still a view.
func TestAdapterInfo_StringFieldsSurviveFree(t *testing.T) {
	inst, err := CreateInstance(nil)
	if err != nil {
		t.Fatalf("CreateInstance failed: %v", err)
	}
	defer inst.Release()

	adapter, err := inst.RequestAdapter(nil)
	if err != nil {
		t.Fatalf("RequestAdapter failed: %v", err)
	}
	defer adapter.Release()

	info, err := adapter.Info()
	if err != nil {
		t.Fatalf("Adapter.Info failed: %v", err)
	}

	sink := 0
	for i := 0; i < 200; i++ {
		if i%50 == 0 {
			runtime.GC()
		}
		sink += len(info.Vendor) + len(info.Architecture) +
			len(info.Device) + len(info.Description)
	}
	_ = sink
	t.Logf("vendor=%q architecture=%q device=%q description=%q",
		info.Vendor, info.Architecture, info.Device, info.Description)
}
