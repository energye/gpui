package rwgpu

import "testing"

// TestValidateBindGroupEntriesRejectsZeroHandles pins the P2 defense: a bind
// group entry referencing a released resource (handle == 0) must produce a
// catchable Go error, never reach wgpu-native (which would panic in conv.rs
// with "invalid bind group entry" and abort the process).
func TestValidateBindGroupEntriesRejectsZeroHandles(t *testing.T) {
	cases := []struct {
		name    string
		entries []BindGroupEntry
	}{
		{"buffer handle 0", []BindGroupEntry{{Binding: 0, Buffer: &Buffer{}}}},
		{"sampler handle 0", []BindGroupEntry{{Binding: 2, Sampler: &Sampler{}}}},
		{"texture view handle 0", []BindGroupEntry{{Binding: 1, TextureView: &TextureView{}}}},
		{"mixed with one stale view", []BindGroupEntry{
			{Binding: 0, Buffer: &Buffer{handle: 0x1234}},
			{Binding: 1, TextureView: &TextureView{}}, // released → 0
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBindGroupEntries(tc.entries)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			we, ok := err.(*WGPUError)
			if !ok {
				t.Fatalf("expected *WGPUError, got %T", err)
			}
			if we.Op != "CreateBindGroup" {
				t.Fatalf("expected Op=CreateBindGroup, got %q", we.Op)
			}
		})
	}
}

func TestValidateBindGroupEntriesAcceptsHealthy(t *testing.T) {
	entries := []BindGroupEntry{
		{Binding: 0, Buffer: &Buffer{handle: 0x1111}},
		{Binding: 1, TextureView: &TextureView{handle: 0x2222}},
		{Binding: 2, Sampler: &Sampler{handle: 0x3333}},
	}
	if err := validateBindGroupEntries(entries); err != nil {
		t.Fatalf("healthy entries must pass, got: %v", err)
	}
}

func TestValidateBindGroupEntriesEmptyOK(t *testing.T) {
	if err := validateBindGroupEntries(nil); err != nil {
		t.Fatalf("empty entries must pass, got: %v", err)
	}
}