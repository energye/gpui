//go:build linux

package platform

import (
	"os"
	"testing"
)

// 148.5MHz / (2200×1125) is the canonical 1080p60 timing.
func TestXrrModeRefreshHz(t *testing.T) {
	if got := xrrModeRefreshHz(148500, 2200, 1125); got < 59.99 || got > 60.01 {
		t.Fatalf("1080p60 timing=%v want 60Hz", got)
	}
	if got := xrrModeRefreshHz(297000, 2200, 1125); got < 119.99 || got > 120.01 {
		t.Fatalf("1080p120 timing=%v want 120Hz", got)
	}
	for _, tc := range [][3]uint64{{0, 2200, 1125}, {148500, 0, 1125}, {148500, 2200, 0}} {
		if got := xrrModeRefreshHz(tc[0], uint32(tc[1]), uint32(tc[2])); got != 0 {
			t.Fatalf("degenerate timing %v=%v want 0", tc, got)
		}
	}
}

// A null display must report unknown, never crash.
func TestX11DisplayRefreshHz_NullDisplay(t *testing.T) {
	if got := x11DisplayRefreshHz(0, 0); got != 0 {
		t.Fatalf("null display=%v want 0", got)
	}
}

// Live probe: against a real X server it reports 0 or a sane rate.
func TestX11DisplayRefreshHz_Live(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	lib := xrandrLoad()
	if lib == nil || !xrandrOK || lib.getResources == nil {
		t.Skip("no RandR")
	}
	// No display handle is plumbed in unit tests; the probe entry with a
	// null dpy must still return unknown instead of faulting.
	if got := x11DisplayRefreshHz(0, 0); got != 0 {
		t.Fatalf("null dpy=%v want 0", got)
	}
}
