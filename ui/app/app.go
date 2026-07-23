// Package app is the unified application entry for demand-driven UI frames.
//
// Frame modes align with gogpu (ADR-023 style):
//
//	IDLE       — no dirty, no tickers → WaitEvents(-1), 0% busy spin
//	ANIMATING  — HasActiveTickers → onUpdate/Tick every ~16ms; OnDraw only if Dirty
//	CONTINUOUS — ContinuousRender → paint every tick (game loop ONLY)
//
// Kit / product UI must use ContinuousRender=false and drive animation via
// Tree.AddTicker + MarkNeedsPaint (Flutter demand). Continuous is for full-screen
// game/demo loops (mem_anim, particles), not Skeleton/Spin smokes.
//
// Threads (gogpu multi-thread architecture):
//
//	Main (Run/Pulse):  WaitEvents · Dispatch · TickActive · OnUpdate
//	Render (RenderLoop): Present / tree.Frame / GPU — via chan func queue + LockOSThread
//
// Present is hopped synchronously to the render thread so the live Tree is not
// mutated concurrently (main waits until draw finishes).
//
// See docs/UI_APP_SHELL_PLAN.md.
package app

import (
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/platform"
)

// DefaultAnimTick is the wait slice while animations are active (~60 Hz).
const DefaultAnimTick = 16 * time.Millisecond

// Options configures Application.
type Options struct {
	// ContinuousRender forces a paint every loop tick (gogpu CONTINUOUS / game loop).
	// Kit smokes must leave false and use AddTicker + MarkNeedsPaint (ANIMATING).
	ContinuousRender bool
	// AnimTick is the WaitEvents timeout while tickers are active (default 16ms).
	AnimTick time.Duration
	// OnUpdate runs every active tick (ANIMATING/CONTINUOUS), before paint.
	// Delta is seconds since last tick (clamped like gogpu).
	// Always on the main (Run) goroutine — never on the render thread.
	OnUpdate func(dt float64)
	// BeforeDispatch runs on the main goroutine for each host event before
	// platform.Dispatch. Return true to skip default routing (event handled).
	BeforeDispatch func(tree *core.Tree, ev platform.Event) (skip bool)
	// DisableRenderThread runs Present on the main goroutine (tests / single-thread debug).
	// Default false: dedicated render OS thread (gogpu-aligned).
	DisableRenderThread bool
}

// PresentFunc paints one frame for a session. Called only when a draw is needed.
// Implementations typically: BeginFrame → tree.Frame → PresentFrame*.
type PresentFunc func(s *Session) error

// Session is one window's UI binding (tree + host + present).
type Session struct {
	ID      int
	Host    platform.Host
	Tree    *core.Tree
	Present PresentFunc

	// Logical viewport; updated on resize.
	Width, Height int
	// PresentW/H is the size of the last successful present (0 = never).
	// Used to force a redraw when the host size changes.
	PresentW, PresentH int

	// Theme optional; Present may read it.
	Theme *core.Theme

	// OnResize optional.
	OnResize func(w, h int)

	app *Application
}

// Application owns the demand-driven run loop (single-window Phase 1; multi later).
type Application struct {
	opts Options

	session *Session
	nextID  int

	// renderLoop is the dedicated GPU/draw OS thread (nil if DisableRenderThread).
	renderLoop *RenderLoop

	// Animation refcount (gogpu StartAnimation / StopAnimation).
	animating int32

	// Invalidator-style coalesced redraw from any goroutine.
	pendingRedraw atomic.Bool

	running atomic.Bool
	quit    atomic.Bool

	lastFrame time.Time

	// PaintCount is incremented each time Present runs (tests / metrics).
	PaintCount atomic.Int64
	// LoopCount is incremented each runFrame iteration.
	LoopCount atomic.Int64
	// RenderThreadHops counts synchronous present hops to the render thread.
	RenderThreadHops atomic.Int64
}

// New creates an Application.
func New(opts Options) *Application {
	if opts.AnimTick <= 0 {
		opts.AnimTick = DefaultAnimTick
	}
	a := &Application{opts: opts, lastFrame: time.Now()}
	if !opts.DisableRenderThread {
		a.renderLoop = NewRenderLoop()
	}
	return a
}

// RenderLoop returns the dedicated render thread manager (may be nil if disabled).
func (a *Application) RenderLoop() *RenderLoop {
	if a == nil {
		return nil
	}
	return a.renderLoop
}

// Close stops the render thread. Safe to call after Run returns.
func (a *Application) Close() {
	if a == nil {
		return
	}
	a.Quit()
	if a.renderLoop != nil {
		a.renderLoop.Stop()
		a.renderLoop = nil
	}
}

// Attach binds an existing Host + Tree (Headless, LinuxHost, or ExternalHost).
// present may be nil for headless tests that only care about Frame counting via Present stub.
func (a *Application) Attach(host platform.Host, tree *core.Tree, present PresentFunc) *Session {
	if a == nil {
		return nil
	}
	a.nextID++
	w, h := 0, 0
	if host != nil {
		w, h = host.Size()
	}
	s := &Session{
		ID:      a.nextID,
		Host:    host,
		Tree:    tree,
		Present: present,
		Width:   w,
		Height:  h,
		app:     a,
	}
	if tree != nil {
		tree.SetOnDirty(func() { a.RequestRedraw() })
	}
	a.session = s
	// Initial frame required.
	a.RequestRedraw()
	return s
}

// Session returns the primary session (Phase 1 single window).
func (a *Application) Session() *Session {
	if a == nil {
		return nil
	}
	return a.session
}

// RequestRedraw requests a paint pass (gogpu Invalidator). Safe from any goroutine.
// Coalesces concurrent calls; wakes WaitEvents; marks tree dirty once.
func (a *Application) RequestRedraw() {
	if a == nil {
		return
	}
	already := a.pendingRedraw.Swap(true)
	if s := a.session; s != nil && s.Host != nil {
		s.Host.WakeUp()
	}
	if already {
		return
	}
	// Mark dirty if needed. Tree.OnDirty → RequestRedraw is coalesced via pendingRedraw.
	if s := a.session; s != nil && s.Tree != nil && !s.Tree.Dirty() {
		s.Tree.MarkDirty()
	}
}

// StartAnimation increments the animation refcount (keep loop awake for onUpdate).
func (a *Application) StartAnimation() {
	if a == nil {
		return
	}
	atomic.AddInt32(&a.animating, 1)
	if s := a.session; s != nil && s.Host != nil {
		s.Host.WakeUp()
	}
}

// StopAnimation decrements the animation refcount.
func (a *Application) StopAnimation() {
	if a == nil {
		return
	}
	for {
		v := atomic.LoadInt32(&a.animating)
		if v <= 0 {
			return
		}
		if atomic.CompareAndSwapInt32(&a.animating, v, v-1) {
			return
		}
	}
}

// IsAnimating reports whether StartAnimation refcount > 0 or tree has tickers.
func (a *Application) IsAnimating() bool {
	if a == nil {
		return false
	}
	if atomic.LoadInt32(&a.animating) > 0 {
		return true
	}
	if s := a.session; s != nil && s.Tree != nil {
		return s.Tree.HasActiveTickers()
	}
	return false
}

// Quit signals the run loop to exit.
func (a *Application) Quit() {
	if a == nil {
		return
	}
	a.quit.Store(true)
	if s := a.session; s != nil && s.Host != nil {
		s.Host.WakeUp()
	}
}

// Running reports whether Run is active.
func (a *Application) Running() bool {
	return a != nil && a.running.Load()
}

// Run blocks on the demand-driven frame loop until Quit or host close.
// Main-thread work only; Present is synchronized onto the render thread.
func (a *Application) Run() {
	if a == nil || a.session == nil {
		return
	}
	a.running.Store(true)
	a.quit.Store(false)
	a.lastFrame = time.Now()
	defer a.running.Store(false)

	for !a.quit.Load() {
		if !a.runFrame() {
			break
		}
	}
}

// Pulse runs a single frame iteration (external main loops / tests).
// Returns false if the session closed.
func (a *Application) Pulse() bool {
	if a == nil || a.session == nil {
		return false
	}
	return a.runFrame()
}

// runFrame implements gogpu App.runFrame demand modes.
// Returns false on close.
func (a *Application) runFrame() bool {
	s := a.session
	if s == nil || s.Host == nil {
		return false
	}
	a.LoopCount.Add(1)

	continuous := a.opts.ContinuousRender
	animating := a.IsAnimating()
	invalidated := a.pendingRedraw.Swap(false)
	if s.Tree != nil && s.Tree.Dirty() {
		invalidated = true
	}

	// Size mismatch vs last present forces a frame (resize sync).
	if hw, hh := s.Host.Size(); hw > 0 && hh > 0 {
		if s.PresentW == 0 || s.PresentH == 0 || hw != s.PresentW || hh != s.PresentH {
			invalidated = true
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
		}
	}

	// sizeLag: window client size ≠ last present buffer (live resize in progress
	// or waiting for quiet Surface.Configure). Must not WaitEvents(-1) forever or
	// sharp reconfigure never runs after mouse release.
	sizeLag := false
	if hw, hh := s.Host.Size(); hw > 0 && hh > 0 {
		if s.PresentW == 0 || s.PresentH == 0 || hw != s.PresentW || hh != s.PresentH {
			sizeLag = true
			invalidated = true
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
		}
	}

	// IDLE only when fully stable; sizeLag polls at AnimTick until quiet configure.
	if !continuous && !animating && !invalidated && !sizeLag {
		evs := s.Host.WaitEvents(-1)
		if !a.dispatchAll(s, evs) {
			return false
		}
	} else if continuous || animating || sizeLag {
		evs := s.Host.WaitEvents(a.opts.AnimTick)
		if !a.dispatchAll(s, evs) {
			return false
		}
	} else {
		// invalidated: non-blocking drain then draw
		evs := s.Host.WaitEvents(0)
		if !a.dispatchAll(s, evs) {
			return false
		}
	}

	// Drain remaining pending events so drag-resize coalesces to the latest size
	// before we paint (avoids N Configure → N Configure/paint flashes).
	for {
		more := s.Host.WaitEvents(0)
		if len(more) == 0 {
			break
		}
		if !a.dispatchAll(s, more) {
			return false
		}
	}

	// Events may have set dirty / pendingRedraw / size.
	if a.pendingRedraw.Swap(false) {
		invalidated = true
	}
	if s.Tree != nil && s.Tree.Dirty() {
		invalidated = true
	}
	if hw, hh := s.Host.Size(); hw > 0 && hh > 0 {
		if hw != s.Width || hh != s.Height {
			s.Width, s.Height = hw, hh
			invalidated = true
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
			if s.OnResize != nil {
				s.OnResize(hw, hh)
			}
		}
		if s.PresentW != hw || s.PresentH != hh {
			invalidated = true
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
		}
	}

	now := time.Now()
	dt := now.Sub(a.lastFrame).Seconds()
	a.lastFrame = now
	if dt > 0.066 {
		dt = 0.066
	}

	if continuous || animating || invalidated || (s.Tree != nil && s.Tree.HasActiveTickers()) {
		if s.Tree != nil {
			s.Tree.TickActive(dt)
		}
		if a.opts.OnUpdate != nil {
			a.opts.OnUpdate(dt)
		}
		if a.pendingRedraw.Swap(false) {
			invalidated = true
		}
		if s.Tree != nil && s.Tree.Dirty() {
			invalidated = true
		}
	}

	// Paint when continuous, dirty, or size still does not match last present.
	needPaint := continuous
	if s.Tree != nil && (s.Tree.Dirty() || s.Tree.FullPaintRequired()) {
		needPaint = true
	}
	if hw, hh := s.Host.Size(); hw > 0 && hh > 0 && (hw != s.PresentW || hh != s.PresentH) {
		needPaint = true
	}
	if continuous || invalidated || needPaint {
		if needPaint {
			if err := a.present(s); err != nil {
				_ = err
			}
		}
	}
	return true
}

func (a *Application) present(s *Session) error {
	// Snapshot host size on the calling thread (main) before render-thread hop.
	if s != nil && s.Host != nil {
		if hw, hh := s.Host.Size(); hw > 0 && hh > 0 {
			s.Width, s.Height = hw, hh
		}
	}
	run := func() error {
		if a.renderLoop != nil {
			// Consume deferred resize hint; host snapshot above is authoritative.
			_, _, _ = a.renderLoop.ConsumePendingResize()
		}
		a.PaintCount.Add(1)
		var err error
		if s.Present != nil {
			err = s.Present(s)
		} else if s.Tree != nil {
			// Headless default: layout+paint clears dirty.
			pc := &core.PaintContext{Theme: s.Theme, Scale: 1}
			if s.Host != nil {
				pc.Scale = s.Host.ScaleFactor()
			}
			w, h := s.Width, s.Height
			if w <= 0 || h <= 0 {
				if s.Host != nil {
					w, h = s.Host.Size()
				}
			}
			s.Tree.Frame(pc, core.Size{Width: float64(w), Height: float64(h)})
		}
		if err == nil && s != nil {
			s.PresentW, s.PresentH = s.Width, s.Height
		}
		return err
	}
	if a.renderLoop == nil || !a.renderLoop.IsRunning() {
		return run()
	}
	a.RenderThreadHops.Add(1)
	var err error
	a.renderLoop.RunOnRenderThreadVoid(func() {
		err = run()
	})
	return err
}

// coalescePointerMoves collapses runs of PointerMove to the last sample so a
// backlog of motion events (common when a frame is expensive) does not walk the
// tree N times before one present. Down/Up/Scroll/Key are kept in order.
func coalescePointerMoves(evs []platform.Event) []platform.Event {
	if len(evs) < 2 {
		return evs
	}
	out := make([]platform.Event, 0, len(evs))
	var pending platform.Event
	hasPending := false
	flushMove := func() {
		if hasPending {
			out = append(out, pending)
			hasPending = false
		}
	}
	for i := range evs {
		ev := evs[i]
		if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerMove {
			pending = ev
			hasPending = true
			continue
		}
		flushMove()
		out = append(out, ev)
	}
	flushMove()
	return out
}

func (a *Application) dispatchAll(s *Session, evs []platform.Event) bool {
	// Coalesce pointer moves: many MotionNotify during slow paint → apply latest only.
	// Preserves Down/Up order so capture and click synthesis stay correct.
	evs = coalescePointerMoves(evs)
	// Coalesce resize: many ConfigureNotify during drag → one layout update.
	var lastResize *platform.Event
	for i := range evs {
		ev := evs[i]
		if ev.Type == platform.EventClose {
			return false
		}
		if a.opts.BeforeDispatch != nil && s.Tree != nil {
			if a.opts.BeforeDispatch(s.Tree, ev) {
				continue
			}
		}
		resize, close := platform.Dispatch(s.Tree, ev)
		if close {
			return false
		}
		if resize != nil {
			r := *resize
			lastResize = &r
			continue
		}
		if ev.Type == platform.EventRedraw {
			a.pendingRedraw.Store(true)
			// Expose/damage from OS: retained pixels may be invalid → full paint.
			if s.Tree != nil {
				s.Tree.MarkFullPaintRequired()
			}
		}
	}
	if lastResize != nil {
		w, h := lastResize.Width, lastResize.Height
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		s.Width, s.Height = w, h
		if a.renderLoop != nil {
			a.renderLoop.RequestResize(uint32(w), uint32(h))
		}
		if s.Tree != nil {
			s.Tree.MarkFullPaintRequired()
		}
		// Force a present this tick (resize must paint even without tickers).
		a.pendingRedraw.Store(true)
		if s.OnResize != nil {
			s.OnResize(w, h)
		}
	}
	return true
}
