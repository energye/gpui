package application

import (
	"testing"
	"time"

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
	if w.Pipeline() != nil {
		t.Fatal("nil window Pipeline should be nil")
	}
	w.SetEventObserver(nil) // no panic
}

func TestSpawnRegistersBeforeRun(t *testing.T) {
	app := newTestApp(t)
	if app.Running() {
		t.Fatal("app should not be running before Run")
	}
	w1, err := app.SpawnWindow(WindowOptions{Title: "A"}, testRoot())
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	w2, err := app.SpawnWindow(WindowOptions{Title: "B"}, testRoot())
	if err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	if app.WindowCount() != 2 {
		t.Fatalf("count = %d, want 2", app.WindowCount())
	}
	if !w1.Main() || w2.Main() {
		t.Fatal("only the first window is main")
	}
	if w1.Pipeline() == nil || w2.Pipeline() == nil {
		t.Fatal("spawned windows should have pipelines after SetRoot")
	}
	if app.Running() {
		t.Fatal("SpawnWindow must not start loops before Run")
	}
}

func TestSpawnNilRoot(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.SpawnWindow(WindowOptions{Title: "A"}, nil); err == nil {
		t.Fatal("SpawnWindow(nil root) should error")
	}
	if app.WindowCount() != 0 {
		t.Fatal("failed spawn must not register a window")
	}
}

func TestSpawnAfterQuit(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.SpawnWindow(WindowOptions{Title: "A"}, testRoot()); err != nil {
		t.Fatalf("SpawnWindow: %v", err)
	}
	app.Quit()
	if _, err := app.SpawnWindow(WindowOptions{Title: "B"}, testRoot()); err == nil {
		t.Fatal("SpawnWindow after Quit should error")
	}
}

func TestSetEventObserver(t *testing.T) {
	app := newTestApp(t)
	w, err := app.NewWindow(WindowOptions{Title: "A"})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if err := w.SetRoot(testRoot()); err != nil {
		t.Fatalf("SetRoot: %v", err)
	}
	w.SetEventObserver(func(ev platform.Event) {})
	w.SetEventObserver(nil) // clear, no panic
	var nilWin *Window
	nilWin.SetEventObserver(nil) // no panic
}

func TestPipelineNilBeforeRoot(t *testing.T) {
	app := newTestApp(t)
	w, err := app.NewWindow(WindowOptions{Title: "A"})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if w.Pipeline() != nil {
		t.Fatal("Pipeline should be nil before SetRoot")
	}
}

// TestRunClosesExitedWindows: every window whose loop exits must be
// destroyed right away — a closed secondary window must not stay mapped
// while the main window keeps running. StubHost loops end via Quit;
// GPU absence surfaces as loop error, which must not skip the close.
func TestRunClosesExitedWindows(t *testing.T) {
	app := newTestApp(t)
	w1, _ := app.NewWindow(WindowOptions{Title: "A"})
	w2, _ := app.NewWindow(WindowOptions{Title: "B"})
	_ = w1.SetRoot(testRoot())
	_ = w2.SetRoot(testRoot())
	// Quit from another goroutine once both loops are up.
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if app.Running() {
				time.Sleep(200 * time.Millisecond)
				app.Quit()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("Run did not return after Quit")
	}
	if !w1.Closed() || !w2.Closed() {
		t.Fatalf("exited loops must destroy windows: w1=%v w2=%v", w1.Closed(), w2.Closed())
	}
	// Closed windows stay registered (never resurrected, never double freed).
	if app.WindowCount() != 2 {
		t.Fatalf("count = %d, want 2", app.WindowCount())
	}
	app.Close() // idempotent after per-loop closes
}
