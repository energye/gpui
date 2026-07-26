package platform_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/platform"
)

func TestStubHost_SizeAndSurface(t *testing.T) {
	h := platform.NewStubHost(800, 600)
	w, ht := h.Size()
	if w != 800 || ht != 600 {
		t.Fatalf("Size=%dx%d", w, ht)
	}
	if h.ScaleFactor() != 1 {
		t.Fatalf("scale=%v", h.ScaleFactor())
	}
	ns := h.NativeSurface()
	if ns.Window == 0 {
		t.Fatal("expected non-zero stub window handle")
	}
}

func TestStubHost_WaitEventsTimeout(t *testing.T) {
	h := platform.NewStubHost(100, 100)
	t0 := time.Now()
	evs := h.WaitEvents(20 * time.Millisecond)
	if time.Since(t0) < 15*time.Millisecond {
		t.Fatalf("timeout returned too fast: %v", time.Since(t0))
	}
	_ = evs
}

func TestStubHost_PushAndWake(t *testing.T) {
	h := platform.NewStubHost(100, 100)
	go func() {
		time.Sleep(10 * time.Millisecond)
		h.Push(platform.Event{Type: platform.EventClose})
	}()
	evs := h.WaitEvents(-1)
	if len(evs) != 1 || evs[0].Type != platform.EventClose {
		t.Fatalf("events=%v", evs)
	}
}

func TestPlatformKind_String(t *testing.T) {
	if platform.PlatformX11.String() != "x11" {
		t.Fatal(platform.PlatformX11.String())
	}
}
