//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package application is the L1 application shell for multi-window apps.
//
// It glues the L0 platform window layer (ui/platform.Window) with the
// per-window rendering pipeline (ui/embedder.PipelineApp):
//
//	app := application.New(application.Config{Name: "myapp"})
//	win, _ := app.NewWindow(application.WindowOptions{Title: "A", Width: 1200, Height: 800})
//	win.SetRoot(rootRenderObject) // kit 控件树
//	app.Run() // 每窗独立事件泵，主窗关闭 → 全部退出
//
// Windows can also be opened while Run is in progress:
//
//	win, err := app.SpawnWindow(application.WindowOptions{Title: "B"}, rootB)
//	// Run returns after the last window's loop exits.
//
// Each window owns its platform.Window + PipelineApp and pumps events on its
// own goroutine (X11/Wayland/Win32 open per-window native connections, so
// this is thread-safe by construction). The App coordinates lifecycle: the
// first created window is the main window; its close quits the whole app.
package application

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// Config configures the App (application-level, shared by all windows).
type Config struct {
	// Name is the application name; used as the default window title.
	Name string
	// Backend selects the display backend for framework-created windows
	// (DisplayAuto = detection).
	Backend platform.DisplayBackend

	// NewHost overrides window host creation (tests / embedding hosts).
	// When nil, platform.Open creates a real native window. This lets tests
	// inject a StubHost and embedding hosts reuse an existing Host.
	NewHost func(opts WindowOptions) (platform.Host, error)

	// Clear color (0–1). Default dark gray-blue.
	ClearR, ClearG, ClearB, ClearA float64
	// WarmUp runs one full paint before the loop (F11).
	WarmUp bool
	// MaxFrames / RunFor stop conditions (per window).
	MaxFrames int64
	RunFor    time.Duration
	// SnapshotPath (optional) saves a GPU readback PNG of each window's
	// final frame before its loop exits. Single-window verification runs
	// use it; multi-window runs share the path (last window wins), so
	// snapshot verification stays single-window.
	SnapshotPath string
	// OnEvent is called for each platform event (all windows) before
	// default handling.
	OnEvent func(ev platform.Event)
}

// WindowOptions configures one window.
type WindowOptions struct {
	Width, Height int
	Title         string
	// Decorations enables the standard window frame (title bar + borders).
	// false (default) is a frameless plain window; pass true for standard
	// decorations (Wayland CSD / X11 WM frame).
	Decorations bool
	// Resizable controls whether the user can resize the window
	// (min==max size hints when false). Pass true for resizable windows.
	Resizable bool
}

// App is the multi-window application shell.
type App struct {
	cfg Config
	mu  sync.Mutex
	// windows holds every registered window (closed ones stay listed).
	windows []*Window
	quit    atomic.Bool
	// running is true while Run owns the window loops. active counts live
	// loops. notifyCh (capacity 1) wakes the Run waiter on each loop exit.
	// running/active/notifyCh are guarded by mu. firstErr keeps the first
	// window loop error (guarded by errMu).
	running  bool
	active   int
	notifyCh chan struct{}
	errMu    sync.Mutex
	firstErr error
}

// New creates an App. No windows are created until NewWindow.
func New(cfg Config) *App {
	if cfg.ClearA == 0 && cfg.ClearR == 0 && cfg.ClearG == 0 && cfg.ClearB == 0 {
		cfg.ClearR, cfg.ClearG, cfg.ClearB, cfg.ClearA = 0.10, 0.12, 0.16, 1
	}
	return &App{cfg: cfg}
}

// Config returns the app configuration (immutable snapshot).
func (a *App) Config() Config {
	if a == nil {
		return Config{}
	}
	return a.cfg
}

// NewWindow creates a platform window (or adopts the injected host) and
// registers it. The first window is the main window: closing it quits the
// whole app. Call SetRoot before Run.
func (a *App) NewWindow(opts WindowOptions) (*Window, error) {
	if a == nil {
		return nil, errors.New("application: nil app")
	}
	if opts.Width < 1 {
		opts.Width = 640
	}
	if opts.Height < 1 {
		opts.Height = 480
	}
	if opts.Title == "" {
		opts.Title = a.cfg.Name
		if opts.Title == "" {
			opts.Title = "gpui"
		}
	}

	var plat *platform.Window
	if a.cfg.NewHost != nil {
		h, err := a.cfg.NewHost(opts)
		if err != nil {
			return nil, fmt.Errorf("application: new host: %w", err)
		}
		plat = platform.WrapHost(h)
	} else {
		p, err := platform.Open(platform.Options{
			Width:       opts.Width,
			Height:      opts.Height,
			Title:       opts.Title,
			Backend:     a.cfg.Backend,
			Decorations: opts.Decorations,
			Resizable:   opts.Resizable,
		})
		if err != nil {
			return nil, fmt.Errorf("application: open window: %w", err)
		}
		plat = p
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	w := &Window{
		app:  a,
		plat: plat,
		opts: opts,
		main: len(a.windows) == 0,
	}
	a.windows = append(a.windows, w)
	return w, nil
}

// Windows returns a snapshot of the registered windows.
func (a *App) Windows() []*Window {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]*Window, len(a.windows))
	copy(out, a.windows)
	return out
}

// WindowCount returns the number of registered windows.
func (a *App) WindowCount() int {
	if a == nil {
		return 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.windows)
}

// Run starts every window's event loop and blocks until all windows finish
// (main window close quits the rest, or Quit was called). Windows must have
// SetRoot called first; otherwise Run returns an error without starting.
// Windows spawned while Run is in progress (SpawnWindow) join the same wait:
// Run returns after the last window's loop exits. Concurrent Run calls on
// the same App are rejected.
func (a *App) Run() error {
	if a == nil {
		return errors.New("application: nil app")
	}
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return errors.New("application: Run already in progress")
	}
	if len(a.windows) == 0 {
		a.mu.Unlock()
		return nil
	}
	// Guard: every window needs a root before Run.
	for _, w := range a.windows {
		w.mu.Lock()
		noroot := w.pipe == nil
		title := w.opts.Title
		w.mu.Unlock()
		if noroot {
			a.mu.Unlock()
			return fmt.Errorf("application: window %q has no root (call SetRoot before Run)", title)
		}
	}
	if a.notifyCh == nil {
		a.notifyCh = make(chan struct{}, 1)
	} else {
		// Drop a stale wakeup from a previous Run so the waiter below
		// only sees this generation's completions.
		select {
		case <-a.notifyCh:
		default:
		}
	}
	a.running = true
	a.mu.Unlock()

	// Drive initial plus late-registered windows until none remain alive.
	// Each loop runs on its own goroutine; every exit wakes this waiter,
	// which also picks up windows registered mid-run.
	for {
		for _, w := range a.Windows() {
			a.startWindow(w)
		}
		a.mu.Lock()
		if a.active == 0 {
			a.running = false
			for _, w := range a.windows {
				w.mu.Lock()
				w.started = false
				w.mu.Unlock()
			}
			a.mu.Unlock()
			break
		}
		ch := a.notifyCh
		a.mu.Unlock()
		<-ch
	}
	a.errMu.Lock()
	defer a.errMu.Unlock()
	return a.firstErr
}

// startWindow starts w's loop when the App is running and w is startable
// (has a root, not closed, not already started, app not quitting).
// Reports whether the loop was started.
func (a *App) startWindow(w *Window) bool {
	if a == nil || w == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.running || a.quit.Load() {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started || w.closed || w.pipe == nil {
		return false
	}
	w.started = true
	a.active++
	go a.runWindow(w)
	return true
}

// runWindow pumps one window's loop, then destroys that window, records
// its error, and wakes Run. The loop exit (close/timeout/error) always
// destroys the native window right away: otherwise a closed secondary
// window would stay mapped on screen while the main window keeps running
// (only App.Close after Run would remove it). Close is idempotent, and
// startWindow refuses closed windows, so this never restarts or double
// frees. It never holds App.mu while the loop runs or the window closes.
func (a *App) runWindow(w *Window) {
	w.mu.Lock()
	pipe := w.pipe
	title := w.opts.Title
	w.mu.Unlock()
	var err error
	if pipe != nil {
		err = pipe.Run()
	} else {
		err = fmt.Errorf("window %q has no root", title)
	}
	// The loop is done: drop this window's own surface/frame resources
	// now (snapshot already saved inside pipe.Run before it returned).
	w.Close()
	if err != nil {
		a.errMu.Lock()
		if a.firstErr == nil {
			a.firstErr = fmt.Errorf("window %q: %w", title, err)
		}
		a.errMu.Unlock()
	}
	a.mu.Lock()
	a.active--
	ch := a.notifyCh
	a.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// SpawnWindow creates a window, attaches root, and joins it to the App.
// Before Run it only registers (Run starts it with the rest); during Run it
// starts immediately on its own goroutine. Only the first window is main, so
// spawned windows never are: closing one closes just itself.
// Resource exhaustion (more windows than the card holds) surfaces as a clean
// error from window creation or the GPU downgrade chain — never a black
// window. A spawn refused by shutdown still returns the registered window
// with an explaining error.
func (a *App) SpawnWindow(opts WindowOptions, root rendering.RenderObject) (*Window, error) {
	if a == nil {
		return nil, errors.New("application: nil app")
	}
	if root == nil {
		return nil, errors.New("application: nil root")
	}
	if a.quit.Load() {
		return nil, errors.New("application: app quitting")
	}
	w, err := a.NewWindow(opts)
	if err != nil {
		return nil, err
	}
	if err := w.SetRoot(root); err != nil {
		w.Close()
		return nil, err
	}
	if !a.Running() {
		return w, nil
	}
	if !a.startWindow(w) {
		return w, errors.New("application: app stopping, window registered but not started")
	}
	return w, nil
}

// Running reports whether Run currently owns the window loops.
func (a *App) Running() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// Quit requests all windows to exit their event loops. Safe to call from any
// goroutine (e.g. an event handler). Idempotent.
func (a *App) Quit() {
	if a == nil {
		return
	}
	a.quit.Store(true)
	for _, w := range a.Windows() {
		if w.pipe != nil {
			w.pipe.Quit()
		}
	}
}

// Close tears down every window (GPU present target first, then native
// window). Safe after Run returns or to abort before Run.
func (a *App) Close() {
	if a == nil {
		return
	}
	for _, w := range a.Windows() {
		w.Close()
	}
}

// Window is one engine window: platform window + per-window rendering
// pipeline. Create via App.NewWindow; set content via SetRoot.
type Window struct {
	app  *App
	plat *platform.Window
	opts WindowOptions
	main bool

	mu     sync.Mutex
	pipe   *embedder.PipelineApp
	root   rendering.RenderObject
	input  *embedder.InputRouter
	closed bool
	// started marks a loop started at least once since Run began
	// (guarded by App.mu; touched with Window.mu held).
	started bool
	// onEvent is the per-window event observer (guarded by Window.mu).
	onEvent func(ev platform.Event)
}

// Main reports whether this is the first (main) window.
func (w *Window) Main() bool {
	return w != nil && w.main
}

// Platform returns the underlying L0 platform window.
func (w *Window) Platform() *platform.Window {
	if w == nil {
		return nil
	}
	return w.plat
}

// Host returns the platform Host (event pump / size / scale).
func (w *Window) Host() platform.Host {
	if w == nil || w.plat == nil {
		return nil
	}
	return w.plat.Host()
}

// IME returns the window's input-method capability (nil = unsupported).
func (w *Window) IME() platform.IME {
	if w == nil || w.plat == nil {
		return nil
	}
	return w.plat.IME()
}

// Clipboard returns the window's clipboard capability (nil = unsupported).
func (w *Window) Clipboard() platform.Clipboard {
	if w == nil || w.plat == nil {
		return nil
	}
	return w.plat.Clipboard()
}

// Options returns the window creation options.
func (w *Window) Options() WindowOptions {
	if w == nil {
		return WindowOptions{}
	}
	return w.opts
}

// SetRoot attaches the render-object tree (kit content) to the window and
// builds its rendering pipeline. Call once before Run. The root must not be
// nil.
func (w *Window) SetRoot(root rendering.RenderObject) error {
	if w == nil || w.plat == nil {
		return errors.New("application: nil window")
	}
	if root == nil {
		return errors.New("application: nil root")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pipe != nil {
		return errors.New("application: root already set")
	}
	w.root = root
	cfg := w.app.cfg
	w.pipe = embedder.NewPipelineApp(w.plat.Host(), root, embedder.PipelineOptions{
		ClearR:       cfg.ClearR,
		ClearG:       cfg.ClearG,
		ClearB:       cfg.ClearB,
		ClearA:       cfg.ClearA,
		WarmUp:       cfg.WarmUp,
		MaxFrames:    cfg.MaxFrames,
		RunFor:       cfg.RunFor,
		Input:        w.input,
		IME:          w.plat.IME(),
		SnapshotPath: cfg.SnapshotPath,
		OnEvent: func(ev platform.Event) {
			if cfg.OnEvent != nil {
				cfg.OnEvent(ev)
			}
			w.mu.Lock()
			fn := w.onEvent
			w.mu.Unlock()
			if fn != nil {
				fn(ev)
			}
			// Main window close (requested or destroyed) → quit the whole app.
			if embedder.EventQuits(ev) && w.main {
				w.app.Quit()
			}
		},
	})
	return nil
}

// Call before
// SetRoot so the pipeline is wired with it. The router's hit-test is bound
// to this window's HitTestPointer automatically by NewPipelineApp.
func (w *Window) SetInput(r *embedder.InputRouter) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pipe != nil {
		w.pipe.SetInputRouter(r)
	}
	w.input = r
}

// ScheduleFrame requests a frame on this window.
func (w *Window) ScheduleFrame() {
	if w == nil || w.pipe == nil {
		return
	}
	w.pipe.ScheduleFrame()
}

// Pipeline returns the window's rendering pipeline (nil before SetRoot).
// Callers wire per-window content through it (animation tickers, present
// policy) without the shell guessing content needs.
func (w *Window) Pipeline() *embedder.PipelineApp {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pipe
}

// SetEventObserver installs a per-window event observer: it runs for every
// platform event on this window's loop, after the App-wide Config.OnEvent.
// Nil clears it. The observer runs on the window's event-loop goroutine, so
// it must be quick and goroutine-safe against the content it touches.
func (w *Window) SetEventObserver(fn func(ev platform.Event)) {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onEvent = fn
}

// Close tears this window down: rendering pipeline (GPU present target)
// first, then the native platform window. Idempotent.
func (w *Window) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	pipe := w.pipe
	plat := w.plat
	w.mu.Unlock()

	if pipe != nil {
		pipe.Close()
	}
	if plat != nil {
		plat.Close()
	}
}

// Closed reports whether Close has been called.
func (w *Window) Closed() bool {
	if w == nil {
		return true
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}
