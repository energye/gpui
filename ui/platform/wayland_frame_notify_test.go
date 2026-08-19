//go:build linux

package platform

import (
	"testing"
	"time"
)

// TestWaylandFrameNotify: RequestFrameNotify registers a wl_surface.frame
// callback (single in-flight request: repeats are no-ops). After a committed
// frame on the content surface is shown by the compositor, the wl_callback
// done arrives → EventFramePresented is delivered and the callback slot
// clears so a re-request works. The platform-side half of the compositor
// frame-presented loop (ENGINE_FRAME_PRESENT_STANDARD.md 块2). Requires a
// real compositor; skipped with a reason otherwise (never silently green).
func TestWaylandFrameNotify(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()
	h := win.Host()
	fn, ok := h.(FrameNotifier)
	if !ok {
		t.Fatal("wayland host must implement FrameNotifier")
	}
	w := h.(*wlHost).win
	if w == nil || w.csd == nil {
		t.Fatal("frame-notify test needs CSD (shm) to drive a commit")
	}

	fn.RequestFrameNotify()
	if w.frameCB.Load() == 0 {
		t.Fatal("first request must register a wl_callback")
	}
	fn.RequestFrameNotify() // in-flight → no-op
	if w.frameCB.Load() == 0 {
		t.Fatal("second request must keep the in-flight callback")
	}

	// Drive a real commit on the content surface: shm buffer attach + commit.
	// The compositor shows it and answers the pending frame request.
	var s csdSurface
	if err := w.csd.createBufferWith(&s, 64, 64); err != nil {
		t.Fatalf("create shm buffer: %v", err)
	}
	defer s.destroy()
	lib := w.lib
	att := []wlArg{argO(s.buffer), argU(0), argU(0)}
	lib.proxyMarshalArrayFlags(w.surface, wlSurfaceAttach, 0, 0, 0, &att[0])
	dam := []wlArg{argU(0), argU(0), argU(64), argU(64)}
	lib.proxyMarshalArrayFlags(w.surface, wlSurfaceDamage, 0, 0, 0, &dam[0])
	lib.proxyMarshalArrayFlags(w.surface, wlSurfaceCommit, 0, 0, 0, nil)

	deadline := time.Now().Add(3 * time.Second)
	got := false
	for time.Now().Before(deadline) {
		for _, e := range h.WaitEvents(200 * time.Millisecond) {
			if e.Type == EventFramePresented {
				got = true
			}
		}
		if got {
			break
		}
	}
	if !got {
		t.Fatal("compositor frame-presented notice never arrived after a committed frame")
	}
	if w.frameCB.Load() != 0 {
		t.Fatal("callback slot must clear after done (re-requestable)")
	}
	// Re-request works after done.
	fn.RequestFrameNotify()
	if w.frameCB.Load() == 0 {
		t.Fatal("request after done must register again")
	}
}
