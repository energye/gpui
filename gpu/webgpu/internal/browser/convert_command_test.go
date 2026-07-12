package browser

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// TestLoadOpToJS verifies all load operation mappings.
func TestLoadOpToJS(t *testing.T) {
	tests := []struct {
		op   types.LoadOp
		want string
	}{
		{types.LoadOpClear, "clear"},
		{types.LoadOpLoad, "load"},
		{types.LoadOpUndefined, "load"}, // default fallback
	}
	for _, tc := range tests {
		got := LoadOpToJS(tc.op)
		if got != tc.want {
			t.Errorf("LoadOpToJS(%v) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

// TestStoreOpToJS verifies all store operation mappings.
func TestStoreOpToJS(t *testing.T) {
	tests := []struct {
		op   types.StoreOp
		want string
	}{
		{types.StoreOpStore, "store"},
		{types.StoreOpDiscard, "discard"},
		{types.StoreOpUndefined, "store"}, // default fallback
	}
	for _, tc := range tests {
		got := StoreOpToJS(tc.op)
		if got != tc.want {
			t.Errorf("StoreOpToJS(%v) = %q, want %q", tc.op, got, tc.want)
		}
	}
}

// TestLoadOpStoreOpRoundTrip verifies that every non-undefined load/store op
// produces a non-empty string, preventing silent empty-string bugs at runtime.
func TestLoadOpStoreOpRoundTrip(t *testing.T) {
	loadOps := []types.LoadOp{types.LoadOpLoad, types.LoadOpClear}
	for _, op := range loadOps {
		s := LoadOpToJS(op)
		if s == "" {
			t.Errorf("LoadOpToJS(%v) returned empty string — missing mapping", op)
		}
	}

	storeOps := []types.StoreOp{types.StoreOpStore, types.StoreOpDiscard}
	for _, op := range storeOps {
		s := StoreOpToJS(op)
		if s == "" {
			t.Errorf("StoreOpToJS(%v) returned empty string — missing mapping", op)
		}
	}
}
