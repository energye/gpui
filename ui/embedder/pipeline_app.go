package embedder

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/painting"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/raster"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
	"github.com/energye/gpui/ui/scheduler"
)

// PipelineOptions configures a demand-driven tree present loop (P3).
type PipelineOptions struct {
	ClearR, ClearG, ClearB, ClearA float64
	// MaxFrames / RunFor stop conditions.
	MaxFrames int64
	RunFor    time.Duration
	// OnEvent optional.
	OnEvent func(ev platform.Event)
	// WarmUp runs one full paint before the loop (F11).
	WarmUp bool
	// Overlay is the optional F13 overlay stack (P5d). Hit-test is overlay-first.
	Overlay *overlay.State
}

// PipelineApp runs layout/paint → FramePacket → async raster present.
// UI never blocks on Present (SubmitLatest).
type PipelineApp struct {
	host  platform.Host
	sched *scheduler.FrameScheduler
	loop  *raster.Loop
	pipe  *rendering.PipelineOwner
	root  rendering.RenderObject
	opts  PipelineOptions

	target *render.PresentTarget

	quit      atomic.Bool
	presents  atomic.Int64
	frameID   atomic.Uint64
	lastStats scene.RasterStats
	// layoutFrames counts flushes that actually laid out (for S2 gate).
	layoutFrames atomic.Int64
}

// Overlay returns the overlay stack (may be nil).
func (a *PipelineApp) Overlay() *overlay.State {
	if a == nil {
		return nil
	}
	return a.opts.Overlay
}

// SetOverlay attaches an overlay stack (P5d).
func (a *PipelineApp) SetOverlay(st *overlay.State) {
	if a == nil {
		return
	}
	a.opts.Overlay = st
}

// NewPipelineApp builds a tree-driven app. Call Open then Run.
func NewPipelineApp(host platform.Host, root rendering.RenderObject, opts PipelineOptions) *PipelineApp {
	if opts.ClearA == 0 && opts.ClearR == 0 && opts.ClearG == 0 && opts.ClearB == 0 {
		opts.ClearR, opts.ClearG, opts.ClearB, opts.ClearA = 0.10, 0.12, 0.16, 1
	}
	s := scheduler.New()
	return &PipelineApp{
		host:  host,
		sched: s,
		loop:  raster.NewLoop(raster.DefaultPipelineDepth, s.Metrics()),
		pipe:  rendering.NewPipelineOwner(root),
		root:  root,
		opts:  opts,
	}
}

// Scheduler returns the frame scheduler (for AddTicker / AnimationController).
func (a *PipelineApp) Scheduler() *scheduler.FrameScheduler {
	if a == nil {
		return nil
	}
	return a.sched
}

// Pipeline returns the pipeline owner.
func (a *PipelineApp) Pipeline() *rendering.PipelineOwner {
	if a == nil {
		return nil
	}
	return a.pipe
}

// Metrics returns metrics store.
func (a *PipelineApp) Metrics() *scheduler.MetricsStore {
	if a == nil {
		return nil
	}
	return a.sched.Metrics()
}

// LastRasterStats returns the last dirty-layer raster stats.
func (a *PipelineApp) LastRasterStats() scene.RasterStats {
	if a == nil {
		return scene.RasterStats{}
	}
	return a.lastStats
}

// LayoutFlushCount returns how many times layout actually ran during Run.
func (a *PipelineApp) LayoutFlushCount() int64 {
	if a == nil {
		return 0
	}
	return a.layoutFrames.Load()
}

// PresentCount returns completed presents.
func (a *PipelineApp) PresentCount() int64 {
	if a == nil {
		return 0
	}
	return a.presents.Load()
}

// ScheduleFrame requests a frame.
func (a *PipelineApp) ScheduleFrame() {
	if a == nil {
		return
	}
	a.sched.ScheduleFrame()
	if a.host != nil {
		a.host.WakeUp()
	}
}

// Quit requests exit.
func (a *PipelineApp) Quit() {
	if a == nil {
		return
	}
	a.quit.Store(true)
	if a.host != nil {
		a.host.WakeUp()
	}
}

// Open creates the present target.
func (a *PipelineApp) Open() error {
	if a == nil || a.host == nil {
		return errors.New("embedder: nil app/host")
	}
	if a.target != nil {
		return nil
	}
	t, err := OpenPresentTarget(a.host)
	if err != nil {
		return err
	}
	a.target = t
	return nil
}

// Close stops raster and releases GPU.
func (a *PipelineApp) Close() {
	if a == nil {
		return
	}
	a.quit.Store(true)
	if a.loop != nil {
		a.loop.Stop()
	}
	if a.target != nil {
		_ = a.target.Close()
		a.target = nil
	}
}

// Run pumps events, ticks animations, builds packets, presents asynchronously.
func (a *PipelineApp) Run() error {
	if a == nil {
		return errors.New("embedder: nil app")
	}
	if a.target == nil {
		if err := a.Open(); err != nil {
			return err
		}
	}
	a.loop.Start()
	defer a.Close()

	w, h := a.host.Size()
	scale := a.host.ScaleFactor()
	if scale <= 0 {
		scale = 1
	}
	vp := rendering.Size{Width: float64(w), Height: float64(h)}

	// Initial layout + optional warm-up full paint (F11).
	if a.pipe.FlushLayout(vp, true) {
		a.layoutFrames.Add(1)
	}
	if a.opts.WarmUp {
		a.presentSyncFull()
	}
	a.ScheduleFrame()

	deadline := time.Time{}
	if a.opts.RunFor > 0 {
		deadline = time.Now().Add(a.opts.RunFor)
	}

	for !a.quit.Load() {
		if !deadline.IsZero() && time.Now().After(deadline) {
			break
		}
		if a.opts.MaxFrames > 0 && a.presents.Load() >= a.opts.MaxFrames {
			break
		}

		timeout := a.sched.WaitTimeout()
		// If tickers active, ensure persistent mode.
		if a.sched.Tickers().HasActive() {
			a.sched.SetMode(scheduler.ModePersistent)
			timeout = a.sched.WaitTimeout()
		}
		// Cap wait by RunFor deadline so IDLE does not block past exit time.
		if !deadline.IsZero() {
			left := time.Until(deadline)
			if left < 0 {
				break
			}
			if timeout < 0 || timeout > left {
				timeout = left
			}
		}

		evs := a.host.WaitEvents(timeout)
		for _, ev := range evs {
			if a.opts.OnEvent != nil {
				a.opts.OnEvent(ev)
			}
			switch ev.Type {
			case platform.EventClose:
				a.quit.Store(true)
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					sc := ev.Scale
					if sc <= 0 {
						sc = a.host.ScaleFactor()
					}
					_ = a.target.Resize(ev.Width, ev.Height, sc)
					vp = rendering.Size{Width: float64(ev.Width), Height: float64(ev.Height)}
					if a.pipe.FlushLayout(vp, true) {
						a.layoutFrames.Add(1)
					}
					// Size change must repaint even when layout early-outs (e.g. height-only).
					if a.root != nil {
						a.root.MarkNeedsPaint()
					}
					a.ScheduleFrame()
				}
			case platform.EventExpose:
				// Present path already full-clears + redraws on demand. Reacting to
				// every Expose after Present causes a busy loop on X11.
				if !a.sched.Pending() {
					a.ScheduleFrame()
				}
			}
		}

		if a.sched.Mode() == scheduler.ModePersistent || a.sched.Tickers().HasActive() {
			a.sched.WaitFramePace(a.host)
		}

		// Advance animations → may MarkNeedsPaint on spinner only.
		if a.sched.Tick() {
			a.ScheduleFrame()
		}
		a.sched.RecomputeMode()

		if !a.sched.Pending() && !a.sched.Tickers().HasActive() {
			continue
		}

		// Layout only if dirty (S2: spinner phase must not layout).
		w, h = a.host.Size()
		vp = rendering.Size{Width: float64(w), Height: float64(h)}
		if a.pipe.FlushLayout(vp, false) {
			a.layoutFrames.Add(1)
		}
		if a.opts.Overlay != nil {
			a.opts.Overlay.Layout(float64(w), float64(h))
		}

		t0 := time.Now()
		a.sched.Metrics().NoteFrameInterval(t0)
		frameID := a.frameID.Add(1)
		scale = a.host.ScaleFactor()
		if scale <= 0 {
			scale = 1
		}
		pkt := rendering.BuildFramePacket(a.root, frameID, scale, float64(w), float64(h))
		if a.opts.Overlay != nil {
			a.opts.Overlay.AttachToPacket(pkt)
		}
		stats := scene.RasterizeDirty(pkt)
		a.lastStats = stats
		a.sched.Metrics().NoteBuildMs(time.Since(t0).Seconds() * 1000)
		if m := a.sched.Metrics(); m != nil {
			m.SetRasterLayerCount(int64(stats.RasterLayerCount))
			// Wave P0: wire cumulative layout/paint flush counters into JSON metrics.
			if a.pipe != nil {
				m.SetLayoutCount(a.pipe.LayoutCount)
				m.SetPaintCount(a.pipe.PaintCount)
			}
			if platform.HostVSync(a.host) != nil {
				m.SetVSyncSource("true")
			} else {
				m.SetVSyncSource("fallback")
			}
		}

		target := a.target
		root := a.root
		pipe := a.pipe
		ov := a.opts.Overlay
		clearR, clearG, clearB, clearA := a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA
		// force=true: PresentWith clears the full swapchain each frame; CompositeOnly
		// partial paint would leave only dirty widgets (static chrome vanishes).
		// True damage/partial present is a P6 concern.
		job := raster.FrameJob{
			Run: func() error {
				return presentTree(target, pipe, root, ov, clearR, clearG, clearB, clearA, true)
			},
		}
		// Async: never block UI on Present.
		_ = a.loop.SubmitLatest(job)
		a.presents.Add(1) // count submit as frame produced; present completes on raster thread
		a.sched.ClearPending()
		// Keep scheduling while tickers run.
		if a.sched.Tickers().HasActive() {
			a.ScheduleFrame()
		}
		a.sched.RecomputeMode()
	}
	return nil
}

func presentTree(target *render.PresentTarget, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool) error {
	if target == nil || pipe == nil || root == nil {
		return errors.New("embedder: presentTree nil")
	}
	// Use PresentClear path with tree paint into DC then present.
	// PresentTarget.PresentClear does clear+present; we need custom draw.
	return target.PresentWith(func(dc *render.Context) {
		dc.SetRGBA(cr, cg, cb, ca)
		dc.DrawRectangle(0, 0, float64(dc.Width()), float64(dc.Height()))
		_ = dc.Fill()
		pc := painting.New(dc, dc.DeviceScale())
		pipe.FlushPaint(pc, force)
		// Overlay band above main (P5d).
		if ov != nil {
			ov.Paint(pc)
		}
	})
}

func (a *PipelineApp) presentSyncFull() {
	if a.target == nil {
		return
	}
	_ = presentTree(a.target, a.pipe, a.root, a.opts.Overlay, a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA, true)
	a.presents.Add(1)
}

// HitTestPointer runs overlay-first then main hit testing (logical coords).
func (a *PipelineApp) HitTestPointer(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry) {
	if a == nil {
		return overlay.BandNone, nil, nil
	}
	return overlay.HitTestStack(a.root, a.opts.Overlay, rendering.Point{X: x, Y: y})
}
