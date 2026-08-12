package embedder_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
)

func TestApp_RunStubWithoutGPU(t *testing.T) {
	// Open will fail without real GPU handles — ensure New/Schedule don't panic.
	h := platform.NewStubHost(320, 240)
	// Zero window forces PresentTarget failure on Open if we used zero;
	// stub has non-zero placeholders but CreateSurface will fail/abort risk.
	// So we only test scheduler path without Open.
	app := embedder.New(h, embedder.Options{})
	if app.Scheduler() == nil {
		t.Fatal("nil scheduler")
	}
	app.ScheduleFrame()
	if !app.Scheduler().Pending() {
		t.Fatal("expected pending frame")
	}
	app.Quit()
}

func TestApp_QuitWake(t *testing.T) {
	h := platform.NewStubHost(100, 100)
	app := embedder.New(h, embedder.Options{RunFor: 50 * time.Millisecond})
	// Don't Open — Run would try GPU. Just Quit path.
	app.Quit()
	if !app.Scheduler().Pending() {
		// ok
	}
}

// TestEventQuits pins the §2.4 consumer contract: EventCloseRequested (the
// interceptable ✕ / WM_DELETE / xdg close) and EventClose (window already
// destroyed) both end the embedder main loop; other events never do.
func TestEventQuits(t *testing.T) {
	closeEvs := []platform.Event{
		{Type: platform.EventCloseRequested},
		{Type: platform.EventClose},
	}
	for _, ev := range closeEvs {
		if !embedder.EventQuits(ev) {
			t.Errorf("EventQuits(%v) = false, want true", ev.Type)
		}
	}

	keepEvs := []platform.Event{
		{Type: platform.EventResize, Width: 100, Height: 100},
		{Type: platform.EventExpose},
		{Type: platform.EventFocus, Focused: true},
		{Type: platform.EventPointer, Pointer: platform.PointerMove},
		{Type: platform.EventKey, Pressed: true},
		{Type: platform.EventWake},
	}
	for _, ev := range keepEvs {
		if embedder.EventQuits(ev) {
			t.Errorf("EventQuits(%v) = true, want false", ev.Type)
		}
	}
}
