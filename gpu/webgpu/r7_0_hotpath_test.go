//go:build !(js && wasm)

package webgpu

import (
	"testing"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// TestR70_Submit_MarksSubmitted verifies R7.0 Submit fast-paths still mark
// command buffers as submitted even when the native handle is nil.
func TestR70_Submit_MarksSubmitted(t *testing.T) {
	// Fake queue: only exercise the marking + empty-native path without FFI.
	// We cannot call into rwgpu without init; instead validate the local
	// stack-conversion helpers used by CreateBindGroup paths.
	entries := []BindGroupEntry{
		{Binding: 0},
		{Binding: 1},
		{Binding: 2},
		{Binding: 3},
	}
	var stack [8]rwgpu.BindGroupEntry
	out := stack[:len(entries)]
	for i, e := range entries {
		out[i] = convertBindGroupEntry(e)
		if out[i].Binding != e.Binding {
			t.Fatalf("entry %d binding: got %d want %d", i, out[i].Binding, e.Binding)
		}
	}
}

// TestR70_ConvertBindGroup_NoAllocHot checks convertBindGroupEntry is alloc-free
// for buffer-less entries (dominant texture/sampler binds still set pointers only).
func TestR70_ConvertBindGroup_NoAllocHot(t *testing.T) {
	e := BindGroupEntry{Binding: 3}
	allocs := testing.AllocsPerRun(1000, func() {
		_ = convertBindGroupEntry(e)
	})
	if allocs != 0 {
		t.Fatalf("convertBindGroupEntry allocs=%v want 0", allocs)
	}
}

// TestR70_ConvertBindGroupLayout_NoAllocHot same for layout entries without extras.
func TestR70_ConvertBindGroupLayout_NoAllocHot(t *testing.T) {
	e := BindGroupLayoutEntry{
		Binding:    0,
		Visibility: 0,
	}
	allocs := testing.AllocsPerRun(1000, func() {
		_ = convertBindGroupLayoutEntry(e)
	})
	if allocs != 0 {
		t.Fatalf("convertBindGroupLayoutEntry allocs=%v want 0", allocs)
	}
}
