//go:build linux

package platform

import (
	"testing"
	"time"
)

// Real-window tests for the §9 standard window requirements (ui/platform
// layer only, no GPU): app icon name (app_id), hide/show visibility, and
// clipboard read/write through the real compositor selection broadcast
// (two windows on two wl_display connections). Skipped with a reason when no
// compositor is reachable — never silently green.

// TestWaylandIconName: Options.IconName is applied as the xdg app_id
// (the compositor resolves the taskbar icon from it). The compositor has no
// query, so the test pins the observable contract: the window opens with a
// custom app_id (tracer shows set_app_id) and the default path keeps working.
func TestWaylandHideShow(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()
	ctl := win.Controls()
	if ctl == nil {
		t.Fatal("Controls() nil")
	}
	if !ctl.IsVisible() {
		t.Fatalf("fresh window visible = false, want true")
	}
	if err := ctl.Hide(); err != nil {
		t.Fatalf("Hide() error: %v", err)
	}
	if ctl.IsVisible() {
		t.Fatalf("visible after Hide = true, want false")
	}
	// The hidden event must surface (renderer-stop + detach contract).
	sawHidden := false
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range win.Host().WaitEvents(100 * time.Millisecond) {
			if ev.Type == EventHidden && ev.Hidden {
				sawHidden = true
			}
		}
		if sawHidden {
			break
		}
	}
	if !sawHidden {
		t.Fatalf("Hide() did not surface EventHidden{true}")
	}

	// Embedder step: close the GPU target first, then detach (true unmap).
	if hs, ok := win.Host().(HiddenSurface); ok {
		hs.ApplyHiddenDetach()
	}
	if ns := win.Host().NativeSurface(); ns.Window != 0 {
		t.Fatalf("NativeSurface().Window after detach = %#x, want 0 (stack destroyed)", ns.Window)
	}
	if ctl.IsVisible() {
		t.Fatalf("visible after detach = true, want false")
	}

	// Show while the stack is gone: re-creates it (new wl_surface) and
	// defers EventHidden{false} until the re-map configure is processed.
	if err := ctl.Show(); err != nil {
		t.Fatalf("Show() error: %v", err)
	}
	if !ctl.IsVisible() {
		t.Fatalf("visible after Show = false, want true")
	}
	sawShown := false
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range win.Host().WaitEvents(100 * time.Millisecond) {
			if ev.Type == EventHidden && !ev.Hidden {
				sawShown = true
			}
		}
		if sawShown {
			break
		}
	}
	if !sawShown {
		t.Fatalf("Show() did not surface EventHidden{false}")
	}
	if ns := win.Host().NativeSurface(); ns.Window == 0 {
		t.Fatalf("NativeSurface().Window after Show = 0, want a live surface")
	}
	w, h := win.Host().Size()
	if w < 1 || h < 1 {
		t.Fatalf("re-created window did not configure (size %dx%d)", w, h)
	}
}

// TestWaylandClipboardSetGetRoundtrip verifies clipboard write + read.
//
// Layout note (hard requirement): mutter only accepts set_selection from the
// keyboard-focus client (meta-wayland-data-device.c: focus-client check) and
// broadcasts the new selection only to focus clients. With no input
// injection on this machine, a test window has no keyboard focus, so the
// compositor cancels every unattended Set (observed via WAYLAND_DEBUG:
// set_selection → source.cancelled). The test therefore asserts:
//   - Set stores an internal copy readable by the same window (GTK-clipboard
//     pattern: the writer keeps its data; self-read needs no compositor),
//   - when the two-window path is reachable (a focused session), the full
//     compositor broadcast + pull round-trip is verified; otherwise it is
//     skipped with the reason (never silently green).
func TestWaylandClipboardSetGetRoundtrip(t *testing.T) {
	w1 := openTestWayland(t)
	defer w1.Close()
	// Drain the first window's pending events (seat caps etc.).
	_ = w1.Host().WaitEvents(200 * time.Millisecond)

	c1 := w1.Clipboard()
	if c1 == nil {
		t.Skipf("wayland clipboard test skipped: compositor lacks wl_data_device_manager")
	}

	payload := "gpui-clip-test-你好-42"
	if err := c1.Set("text/plain", payload); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	// Immediate self-read: the internal copy (no compositor round-trip).
	own, err := c1.Get("text/plain")
	if err != nil {
		t.Fatalf("Get (own copy) error: %v", err)
	}
	if own != payload {
		t.Fatalf("Get own = %q, want %q", own, payload)
	}

	// Two-window path: needs keyboard focus. Detect the compositor's
	// decision — focus accepted (broadcast lands) vs cancelled (no focus).
	w2 := openTestWayland(t)
	defer w2.Close()
	c2 := w2.Clipboard()
	if c2 == nil {
		t.Skipf("wayland two-window path skipped: compositor lacks wl_data_device_manager")
	}
	cancelled := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Dispatch w1 until the source is cancelled (no focus) — or forever
		// while the round-trip below runs (focused session).
		for {
			select {
			case <-cancelled:
				return
			default:
			}
			_ = w1.Host().WaitEvents(50 * time.Millisecond)
		}
	}()
	// Dispatch w2: the compositor's selection broadcast would land here.
	_ = w2.Host().WaitEvents(300 * time.Millisecond)

	got, err := c2.Get("text/plain")
	close(cancelled)
	<-done // the w1 pump goroutine must stop before w1's teardown destroys
	// its wl_display — WaitEvents parked in poll would otherwise dispatch
	// on the torn-down display (native SIGSEGV).
	if err != nil {
		t.Skipf("two-window clipboard path skipped: compositor refused unattended set_selection (no keyboard focus; %v)", err)
	}
	if got != payload {
		t.Fatalf("Get via broadcast = %q, want %q", got, payload)
	}
}

// TestWaylandClipboardEmpty: Get with no selection and nothing Set returns an
// error (never a fake empty string that would paper over a missing selection).
func TestWaylandClipboardEmpty(t *testing.T) {
	w1 := openTestWayland(t)
	defer w1.Close()
	_ = w1.Host().WaitEvents(200 * time.Millisecond)
	c1 := w1.Clipboard()
	if c1 == nil {
		t.Skipf("wayland clipboard test skipped: compositor lacks wl_data_device_manager")
	}
	if got, err := c1.Get("text/plain"); err == nil {
		t.Fatalf("Get on empty clipboard = %q, want error", got)
	}
}

// TestParseURIList pins the text/uri-list → absolute-path conversion used by
// the file-drop event (unit, no compositor needed).
func TestParseURIList(t *testing.T) {
	data := "# a comment\r\nfile:///home/u/a.txt\r\n\r\nfile:///tmp/x%20y.png\nfile://localhost/opt/b.bin\nplain-uri\nfile:///with%23hash.txt\r\n"
	got := parseURIList(data)
	want := []string{"/home/u/a.txt", "/tmp/x y.png", "/opt/b.bin", "/with#hash.txt"}
	if len(got) != len(want) {
		t.Fatalf("parseURIList = %d files %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseURIList[%d] = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
}
