//go:build linux && !nogpu

package render

import (
	"testing"
)

// Backend selection must be explicit and honest: unset means native,
// go means the pure-Go GL path, anything else errors (no guessing,
// no legacy values).
func TestResolveBackend(t *testing.T) {
	t.Setenv("GPUI_BACKEND", "")
	b, err := ResolveBackend()
	if err != nil {
		t.Fatalf("ResolveBackend(unset): %v", err)
	}
	if b != BackendNative {
		t.Fatalf("ResolveBackend(unset) = %v, want native", b)
	}
	for _, v := range []string{"native", "NATIVE", " Native "} {
		t.Setenv("GPUI_BACKEND", v)
		b, err := ResolveBackend()
		if err != nil {
			t.Fatalf("ResolveBackend(%q): %v", v, err)
		}
		if b != BackendNative {
			t.Fatalf("ResolveBackend(%q) = %v, want native", v, b)
		}
	}
	for _, v := range []string{"go", "GO", " Go "} {
		t.Setenv("GPUI_BACKEND", v)
		b, err := ResolveBackend()
		if err != nil {
			t.Fatalf("ResolveBackend(%q): %v", v, err)
		}
		if b != BackendGo {
			t.Fatalf("ResolveBackend(%q) = %v, want go", v, b)
		}
	}
	// Only native|go exist: webgpu/gl/1 and anything else must error.
	for _, v := range []string{"webgpu", "gl", "1", "bogus"} {
		t.Setenv("GPUI_BACKEND", v)
		if _, err := ResolveBackend(); err == nil {
			t.Fatalf("ResolveBackend(%q) must error", v)
		}
	}
	if BackendNative.String() != "native" || BackendGo.String() != "go" {
		t.Fatalf("Backend String = %q/%q, want native/go", BackendNative, BackendGo)
	}
}

// Env wins over code in both directions: an explicit code choice stands
// only when the env is empty; a set env overrides whatever code passed.
func TestResolveBackendForEnvWins(t *testing.T) {
	t.Setenv("GPUI_BACKEND", "")
	if got, err := ResolveBackendFor(BackendGo); err != nil || got != BackendGo {
		t.Fatalf("ResolveBackendFor(go, unset) = %v,%v, want go,nil", got, err)
	}
	if got, err := ResolveBackendFor(BackendNative); err != nil || got != BackendNative {
		t.Fatalf("ResolveBackendFor(native, unset) = %v,%v, want native,nil", got, err)
	}
	t.Setenv("GPUI_BACKEND", "go")
	if got, err := ResolveBackendFor(BackendNative); err != nil || got != BackendGo {
		t.Fatalf("ResolveBackendFor(native, go) = %v,%v, want go,nil", got, err)
	}
	t.Setenv("GPUI_BACKEND", "native")
	if got, err := ResolveBackendFor(BackendGo); err != nil || got != BackendNative {
		t.Fatalf("ResolveBackendFor(go, native) = %v,%v, want native,nil", got, err)
	}
	t.Setenv("GPUI_BACKEND", "bogus")
	if _, err := ResolveBackendFor(BackendNative); err == nil {
		t.Fatal("ResolveBackendFor(native, bogus) must error")
	}
	if _, err := ResolveBackendFor(Backend(7)); err == nil {
		t.Fatal("ResolveBackendFor(7, unset) must error")
	}
}
