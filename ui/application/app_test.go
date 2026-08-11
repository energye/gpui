package application

import (
	"sync/atomic"
	"testing"

	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// testRoot is a minimal RenderObject for SetRoot (RenderBox is the concrete
// box implementation in ui/rendering).
func testRoot() rendering.RenderObject {
	return rendering.NewRenderBox()
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	return New(Config{
		Name: "testapp",
		// Never create real native windows in unit tests.
		NewHost: func(opts WindowOptions) (platform.Host, error) {
			return platform.NewStubHost(opts.Width, opts.Height), nil
		},
	})
}

func TestNewDefaults(t *testing.T) {
	app := New(Config{Name: "x"})
	if app.WindowCount() != 0 {
		t.Fatal("new app should have no windows")
	}
	if app.Config().Name != "x" {
		t.Fatal("config name lost")
	}
}

func TestNewWindowRegistersAndMain(t *testing.T) {
	app := newTestApp(t)
	w1, err := app.NewWindow(WindowOptions{Title: "A", Width: 100, Height: 200})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	w2, _ := app.NewWindow(WindowOptions{Title: "B"})
	if app.WindowCount() != 2 {
		t.Fatalf("count = %d, want 2", app.WindowCount())
	}
	if !w1.Main() {
		t.Fatal("first window must be main")
	}
	if w2.Main() {
		t.Fatal("second window must not be main")
	}
	// Defaults applied.
	if w2.opts.Width != 640 || w2.opts.Height != 480 {
		t.Fatalf("default size = %dx%d", w2.opts.Width, w2.opts.Height)
	}
	if w2.opts.Title != "B" {
		t.Fatalf("title = %q", w2.opts.Title)
	}
}

func TestNewWindowDefaultTitleFromApp(t *testing.T) {
	app := newTestApp(t)
	w, err := app.NewWindow(WindowOptions{})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if w.opts.Title != "testapp" {
		t.Fatalf("title = %q, want testapp", w.opts.Title)
	}
}

func TestSetRootOnce(t *testing.T) {
	app := newTestApp(t)
	w, _ := app.NewWindow(WindowOptions{Title: "A"})
	if err := w.SetRoot(testRoot()); err != nil {
		t.Fatalf("SetRoot: %v", err)
	}
	if err := w.SetRoot(testRoot()); err == nil {
		t.Fatal("second SetRoot should error")
	}
}

func TestSetRootNil(t *testing.T) {
	app := newTestApp(t)
	w, _ := app.NewWindow(WindowOptions{Title: "A"})
	if err := w.SetRoot(nil); err == nil {
		t.Fatal("SetRoot(nil) should error")
	}
}

func TestRunNoWindows(t *testing.T) {
	app := newTestApp(t)
	if err := app.Run(); err != nil {
		t.Fatalf("Run with no windows: %v", err)
	}
}

func TestRunRequiresRoot(t *testing.T) {
	app := newTestApp(t)
	_, _ = app.NewWindow(WindowOptions{Title: "A"})
	if err := app.Run(); err == nil {
		t.Fatal("Run without SetRoot should error")
	}
}

func TestWindowCloseIdempotent(t *testing.T) {
	app := newTestApp(t)
	w, _ := app.NewWindow(WindowOptions{Title: "A"})
	_ = w.SetRoot(testRoot())
	w.Close()
	w.Close()
	if !w.Closed() {
		t.Fatal("window should be closed")
	}
	// Hosts from StubHost are still usable after wrap-close (no native teardown).
	if app.WindowCount() != 1 {
		t.Fatalf("closed window still registered: %d", app.WindowCount())
	}
}

func TestAppCloseClosesAll(t *testing.T) {
	app := newTestApp(t)
	w1, _ := app.NewWindow(WindowOptions{Title: "A"})
	w2, _ := app.NewWindow(WindowOptions{Title: "B"})
	_ = w1.SetRoot(testRoot())
	_ = w2.SetRoot(testRoot())
	app.Close()
	if !w1.Closed() || !w2.Closed() {
		t.Fatal("app.Close should close all windows")
	}
}

func TestQuitBeforeRun(t *testing.T) {
	app := newTestApp(t)
	w, _ := app.NewWindow(WindowOptions{Title: "A"})
	_ = w.SetRoot(testRoot())
	app.Quit()
	app.Quit() // idempotent, no panic
	if !app.quit.Load() {
		t.Fatal("quit flag not set")
	}
}

func TestWindowNilSafety(t *testing.T) {
	var w *Window
	if w.Main() || w.Host() != nil || w.IME() != nil || w.Clipboard() != nil || w.Platform() != nil {
		t.Fatal("nil window should return zero values")
	}
	if !w.Closed() {
		t.Fatal("nil window.Closed should be true")
	}
	w.Close() // no panic
	w.ScheduleFrame()
}

func TestOnEventWired(t *testing.T) {
	var got atomic.Int32
	app := New(Config{
		Name:    "t",
		NewHost: func(opts WindowOptions) (platform.Host, error) { return platform.NewStubHost(10, 10), nil },
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventWake {
				got.Add(1)
			}
		},
	})
	w, _ := app.NewWindow(WindowOptions{Title: "A"})
	_ = w.SetRoot(testRoot())
	// The OnEvent closure is wired into the pipeline; we cannot run it without
	// GPU, so assert the config is stored and window is bound.
	if w.app == nil || w.pipe == nil {
		t.Fatal("window not fully bound")
	}
	if got.Load() != 0 {
		t.Fatal("no events should have been delivered without Run")
	}
}
