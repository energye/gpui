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
		Width:      400,
		Height:     300,
		Title:      "gpui wayland win test",
		Decorations: true, // standard CSD — exercises the chrome paths too
		Backend:    DisplayWayland, // force the wayland backend — Auto would pick X11 when DISPLAY is set
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

// TestWaylandRealWindowControls exercises the wlController (S2): title/size
// round-trips on the tracked state, protocol-impossible ops answer
// ErrUnsupported (§2.5.4 honesty — never a silent fake success), and
// queries the protocol cannot answer return documented zero values (§2.5.3).
func TestWaylandRealWindowControls(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	ctl := win.Controls()
	if ctl == nil {
		t.Fatal("Controls() nil — wlController must be wired (S2)")
	}

	// Title round-trip (tracked; xdg has no query).
	ctl.SetTitle("wayland-test-t2")
	if got := ctl.Title(); got != "wayland-test-t2" {
		t.Errorf("Title() = %q after set, want wayland-test-t2", got)
	}

	// Size: SetSize updates the tracked size optimistically; the compositor
	// confirms with configure (EventResize) shortly after.
	wantW, wantH := 500, 400
	ctl.SetSize(wantW, wantH)
	w, h := ctl.Size()
	if w != wantW || h != wantH {
		t.Errorf("Size() = (%d,%d) after SetSize, want (%d,%d)", w, h, wantW, wantH)
	}

	// Resizable toggle + query.
	ctl.SetResizable(false)
	if ctl.IsResizable() {
		t.Error("IsResizable() = true after SetResizable(false)")
	}
	ctl.SetResizable(true)
	if !ctl.IsResizable() {
		t.Error("IsResizable() = false after SetResizable(true)")
	}

	// Constraints accept and track values (0 = unconstrained).
	ctl.SetMinSize(320, 240)
	ctl.SetMaxSize(1920, 1080)

	// Protocol-impossible ops → ErrUnsupported (honesty contract §2.5.4).
	// Show/Hide are real on wayland (§9 隐藏窗口) — asserted in
	// TestWaylandHideShow; SetPosition/Focus/AlwaysOnTop/SetDecorations
	// have no xdg-shell counterpart.
	for name, err := range map[string]error{
		"SetPosition":    ctl.SetPosition(10, 10),
		"Focus":          ctl.Focus(),
		"SetAlwaysOnTop": ctl.SetAlwaysOnTop(true),
		"SetDecorations": ctl.SetDecorations(true),
	} {
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Errorf("%s on wayland: err=%v, want ErrUnsupported", name, err)
		}
	}

	// Query semantics (§2.5.3): no client-side position → ok=false (zero ≠
	// "at origin"); a fresh window is visible (Hide pulls it down, Show
	// restores — §3.2).
	if x, y, ok := ctl.Position(); ok || x != 0 || y != 0 {
		t.Errorf("Position() = (%d,%d,%v), want (0,0,false)", x, y, ok)
	}
	if !ctl.IsVisible() {
		t.Error("IsVisible() = false on a fresh window, want true (only Hide sets it false)")
	}
	if ctl.IsMinimized() {
		t.Error("IsMinimized() = true on a fresh window")
	}

	// Best-effort ops must not panic when the compositor is quiet.
	ctl.Minimize()
	ctl.Maximize()
	ctl.Unmaximize()
	ctl.SetFullscreen(true)
	ctl.SetFullscreen(false)
	ctl.SetCursor(CursorText)
	ctl.SetCursor(CursorDefault)

	// Interactive move/resize need a valid enter serial; with no pointer
	// interaction they must answer ErrUnsupported, not a fake success.
	for name, err := range map[string]error{
		"RequestMove":       ctl.RequestMove(),
		"RequestResize":     ctl.RequestResize(WindowEdgeRight),
		"RequestResizeNone": ctl.RequestResize(WindowEdgeNone),
	} {
		if err == nil || !strings.Contains(err.Error(), "not supported") {
			t.Errorf("%s: err=%v, want ErrUnsupported (no enter serial / no edge)", name, err)
		}
	}

	// SetIgnoreCursorEvents round-trip (wl_region empty / NULL region) must
	// succeed on a working compositor and not corrupt the event loop.
	if err := ctl.SetIgnoreCursorEvents(true); err != nil {
		t.Errorf("SetIgnoreCursorEvents(true): err=%v, want nil", err)
	}
	if err := ctl.SetIgnoreCursorEvents(false); err != nil {
		t.Errorf("SetIgnoreCursorEvents(false): err=%v, want nil", err)
	}
}
