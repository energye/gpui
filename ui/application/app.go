// Package application is the L1 application shell for multi-window apps.
//
// It glues the L0 platform window layer (ui/platform.Window) with the
// per-window rendering pipeline (ui/embedder.PipelineApp):
//
//	app := application.New(application.Config{Name: "myapp"})
//	win, _ := app.NewWindow(application.WindowOptions{Title: "A", Width: 1200, Height: 800})
//	win.SetRoot(rootRenderObject) // kit 控件树
//	app.Run()                     // 每窗独立事件泵，主窗关闭 → 全部退出
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
	// OnEvent is called for each platform event (all windows) before
	// default handling.
	OnEvent func(ev platform.Event)
}

// WindowOptions configures one window.
type WindowOptions struct {
	Width, Height int
	Title         string
	// Decorations controls window chrome (title bar + frame). Default: true
	// (Wayland CSD / X11 WM frame). Pass false for frameless.
	Decorations bool
	// Resizable controls whether the user can resize the window
	// (min==max size hints when false). Pass true for resizable windows.
	Resizable bool
}

// App is the multi-window application shell.
type App struct {
	cfg     Config
	mu      sync.Mutex
	windows []*Window
	quit    atomic.Bool
	wg      sync.WaitGroup
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
func (a *App) Run() error {
	if a == nil {
		return errors.New("application: nil app")
	}
	wins := a.Windows()
	if len(wins) == 0 {
		return nil
	}
	// Guard: every window needs a root before Run.
	for _, w := range wins {
		if w.pipe == nil {
			return fmt.Errorf("application: window %q has no root (call SetRoot before Run)", w.opts.Title)
		}
	}
	errs := make(chan error, len(wins))
	for _, w := range wins {
		a.wg.Add(1)
		go func(w *Window) {
			defer a.wg.Done()
			if err := w.pipe.Run(); err != nil {
				errs <- fmt.Errorf("window %q: %w", w.opts.Title, err)
			}
		}(w)
	}
	a.wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
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
		ClearR:    cfg.ClearR,
		ClearG:    cfg.ClearG,
		ClearB:    cfg.ClearB,
		ClearA:    cfg.ClearA,
		WarmUp:    cfg.WarmUp,
		MaxFrames: cfg.MaxFrames,
		RunFor:    cfg.RunFor,
		Input:     w.input,
		OnEvent: func(ev platform.Event) {
			if cfg.OnEvent != nil {
				cfg.OnEvent(ev)
			}
			// Main window close (requested or destroyed) → quit the whole app.
			if embedder.EventQuits(ev) && w.main {
				w.app.Quit()
			}
		},
	})
	return nil
}

// SetInput attaches the unified input router (plan §4/§6). Call before
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
