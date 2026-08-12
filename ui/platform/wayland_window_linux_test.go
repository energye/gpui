//go:build linux

package platform

import (
	"strings"
	"testing"
	"time"
)

// Real-window tests: they open an actual Wayland window through the unified
// L0 API (no GPU — platform layer only). Skipped with a reason when no
// compositor is reachable; never silently green. B layer of
// ENGINE_WINDOW_API.md §2.6.

func openTestWayland(t *testing.T) *Window {
	t.Helper()
	if !HasWaylandDisplay() {
		t.Skipf("wayland real-window test skipped: WAYLAND_DISPLAY not set")
	}
	win, err := Open(Options{
		Width:   400,
		Height:  300,
		Title:   "gpui wayland win test",
		Backend: DisplayWayland, // force the wayland backend — Auto would pick X11 when DISPLAY is set
	})
	if err != nil {
		if strings.Contains(err.Error(), "connect failed") ||
			strings.Contains(err.Error(), "missing wl_compositor") {
			t.Skipf("wayland real-window test skipped: compositor unreachable (%v)", err)
		}
		t.Fatalf("Open(wayland) failed: %v", err)
	}
	return win
}

// TestWaylandRealWindowOpen: the window opens, the Host is ready and answers
// non-zero size — the configure-driven contract a compositor must provide.
func TestWaylandRealWindowOpen(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	if win.Kind() != PlatformWayland {
		t.Fatalf("Kind = %v, want wayland", win.Kind())
	}
	if win.Host() == nil {
		t.Fatal("Host() nil after open")
	}
	if ns := win.Host().NativeSurface(); ns.Kind != PlatformWayland || ns.Display == 0 {
		t.Fatalf("NativeSurface = %+v, want wayland kind + display", ns)
	}
	w, h := win.Host().Size()
	if w < 1 || h < 1 {
		t.Errorf("Size = (%d,%d), want >= 1 (configured by compositor)", w, h)
	}
	if win.Closed() {
		t.Fatal("fresh window must not be closed")
	}
}

// TestWaylandRealWindowEvents asserts the event pump is alive and wired:
// WakeUp must surface as EventWake (cross-thread wake contract, §2.4).
//
// EventResize is legitimately absent here: without a rendering layer the
// compositor answers the first (and only) configure with the requested size —
// identical to Open's request, so the dedup in wlTopConfigure fires no
// resize. Deterministic resize assertions need a buffer attach / SetSize
// (renderer + wlController) and land with S2.
func TestWaylandRealWindowEvents(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	host := win.Host()

	// Deterministic: pump liveness via the wake channel.
	host.WakeUp()
	deadline := time.Now().Add(2 * time.Second)
	gotWake := false
	for time.Now().Before(deadline) && !gotWake {
		for _, e := range host.WaitEvents(200 * time.Millisecond) {
			switch e.Type {
			case EventWake:
				gotWake = true
			default:
				t.Logf("event: %s", e.Type) // opportunistic compositor events (focus/occluded/…)
			}
		}
	}
	if !gotWake {
		t.Error("WakeUp() must surface as EventWake within 2s")
	}

	// Opportunistic: drain what else the compositor already delivered.
	host.WakeUp() // drain the queue
	host.WaitEvents(250 * time.Millisecond)
}

// TestWaylandRealWindowControlsHonest: until the wlController lands (S2),
// Controls() is nil — the app must nil-check, never assume. The moment S2
// lands this test becomes the wlController presence check.
func TestWaylandRealWindowControlsHonest(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	ctl := win.Controls()
	if ctl == nil {
		t.Logf("note: Controls() nil on wayland (wlController lands in S2; OK today)")
		return
	}
	// S2+ path: controller exists — probe a protocol-impossible op; it must
	// be ErrUnsupported, never a silent fake success (§2.5.4).
	if ctl.Title() == "" {
		t.Error("Title() empty on wayland controller")
	}
	if err := ctl.SetPosition(10, 10); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("SetPosition on wayland: err=%v, want ErrUnsupported", err)
	}
}
