//go:build linux && !(js && wasm)

package gles

import "testing"

// DrawBuffers BACK is per-default-FB state: first blit sets it, steady
// frames reuse it. Recreate/Unconfigure/Configure clear the cache so a new
// default FB gets BACK again (else black window, no GL error).
func TestSurface_DefaultDrawInit_CacheLifecycle(t *testing.T) {
	s := &Surface{}
	if s.defaultDrawInit {
		t.Fatal("fresh Surface must not claim BACK cached")
	}
	s.defaultDrawInit = true
	s.defaultDrawInit = false // recreate path
	if s.defaultDrawInit {
		t.Fatal("recreate must clear defaultDrawInit")
	}
	s.defaultDrawInit = true
	s.defaultDrawInit = false // Unconfigure/Configure path
	if s.defaultDrawInit {
		t.Fatal("reconfigure must clear defaultDrawInit")
	}
}
