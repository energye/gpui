package rwgpu

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	rwgpuLibOnce sync.Once
	rwgpuLibSkip string
)

// requireLibDirtied reports whether the native library can load. The repo
// vendors libwgpu_native.so under lib/; point there when the environment
// does not (testdata discipline: external toolchain via env, default repo
// path as fallback). String return is the skip reason, empty means loaded.
func requireLibLoaded(t *testing.T) {
	t.Helper()
	rwgpuLibOnce.Do(func() {
		if os.Getenv("WGPU_NATIVE_PATH") == "" {
			if exe, err := os.Executable(); err == nil {
				try := filepath.Join(filepath.Dir(exe), "..", "..", "..",
					"lib", "libwgpu_native.so")
				if st, err := os.Stat(try); err == nil && !st.IsDir() {
					_ = os.Setenv("WGPU_NATIVE_PATH", try)
				}
			}
			if os.Getenv("WGPU_NATIVE_PATH") == "" {
				if st, err := os.Stat("lib/libwgpu_native.so"); err == nil &&
					!st.IsDir() {
					_ = os.Setenv("WGPU_NATIVE_PATH", "lib/libwgpu_native.so")
				}
			}
		}
		if err := Init(); err != nil {
			rwgpuLibSkip = err.Error()
		}
	})
	if rwgpuLibSkip != "" {
		t.Skipf("wgpu native library not loaded (set WGPU_NATIVE_PATH): %s",
			rwgpuLibSkip)
	}
}

// requireInstance creates a wgpu instance for tests that need a real GPU
// device. When the native library is missing (WGPU_NATIVE_PATH unset and no
// system install), it skips instead of failing: external toolchains resolve
// via environment, never hard-coded paths (testdata discipline).
func requireInstance(t *testing.T) *Instance {
	t.Helper()
	requireLibLoaded(t)
	inst, err := CreateInstance(nil)
	if err != nil {
		if errors.Is(err, ErrLibraryNotLoaded) {
			t.Skipf("wgpu native library not loaded (set WGPU_NATIVE_PATH): %v", err)
		}
		t.Fatalf("CreateInstance failed: %v", err)
	}
	return inst
}
