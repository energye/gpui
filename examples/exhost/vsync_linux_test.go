//go:build linux

package exhost

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

// Compile-time + runtime: Linux hosts implement platform.VSyncWaiter.
func TestX11Host_ImplementsVSyncWaiter(t *testing.T) {
	var _ platform.VSyncWaiter = (*x11Host)(nil)
	var _ platform.VSyncWaiter = (*wlHost)(nil)
	// Method is wired to DRM; without card access it returns error (scheduler fallback).
	h := &x11Host{st: &x11State{w: 1, h: 1, scale: 1}}
	err := h.WaitVSync()
	if err == nil {
		t.Log("DRM vblank available in this environment")
		return
	}
	// Must be a real error path, not panic.
	t.Logf("WaitVSync (no display card expected in some CI): %v", err)
}
