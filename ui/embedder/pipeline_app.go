package embedder

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/overlay"
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

// boundaryFrameSnap is the last paintPresentTree frame's boundary cache counters.
type boundaryFrameSnap struct {
	Rerecord int64
	Skip     int64
}

// lastBoundaryFrame is published by paintPresentTree for metrics pickup.
var lastBoundaryFrame atomic.Value // stores boundaryFrameSnap

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
	// forceFullPresent is set on warm-up/resize so the next present full-clears.
	forceFullPresent atomic.Bool
}

// LastBoundaryFrame returns the most recent paintPresentTree boundary skip/rerecord.
func LastBoundaryFrame() (rerecord, skip int64) {
	v := lastBoundaryFrame.Load()
	if v == nil {
		return 0, 0
	}
	s, ok := v.(boundaryFrameSnap)
	if !ok {
		return 0, 0
	}
	return s.Rerecord, s.Skip
}

// BoundaryCache returns the PipelineOwner's long-lived boundary Picture cache.
func (a *PipelineApp) BoundaryCache() *rendering.BoundaryCache {
	if a == nil || a.pipe == nil {
		return nil
	}
	return a.pipe.BoundaryCache()
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
// W0 default present policy is full_paint (steady frames repaint the whole tree).
func NewPipelineApp(host platform.Host, root rendering.RenderObject, opts PipelineOptions) *PipelineApp {
	if opts.ClearA == 0 && opts.ClearR == 0 && opts.ClearG == 0 && opts.ClearB == 0 {
		opts.ClearR, opts.ClearG, opts.ClearB, opts.ClearA = 0.10, 0.12, 0.16, 1
	}
	s := scheduler.New()
	s.Metrics().SetPresentPolicy(scheduler.PresentPolicyFullPaint)
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
	a.forceFullPresent.Store(true) // first present after open is always full
	if a.opts.WarmUp {
		a.presentSyncFull()
		a.forceFullPresent.Store(false) // warm-up already full-cleared swapchain
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
					a.forceFullPresent.Store(true) // resize → full clear + full paint
					a.ScheduleFrame()
				}
			case platform.EventExpose:
				// Damage/full present redraws on demand. Reacting to every Expose
				// after Present causes a busy loop on X11.
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
			// vsync_source is set honestly by FrameScheduler.WaitFramePace
			// (true only after a successful WaitVSync, else fallback).
		}

		target := a.target
		root := a.root
		pipe := a.pipe
		ov := a.opts.Overlay
		clearR, clearG, clearB, clearA := a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA
		// Steady frames: force=false → no full clear, CompositeOnly paint, PresentFrameAuto.
		// Warm-up / resize / open: force=true → full clear + full paint + PresentFrameFull.
		// Static chrome survives via LoadOpLoad when only dirty widgets repaint.
		force := a.forceFullPresent.Swap(false)
		metrics := a.sched.Metrics()
		// W0: keep policy visible on every frame path (default full_paint until W6).
		if metrics != nil && metrics.PresentPolicy() == "" {
			metrics.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
		}
		job := raster.FrameJob{
			Run: func() error {
				out, err := presentTree(target, pipe, root, ov, clearR, clearG, clearB, clearA, force)
				if metrics != nil && target != nil {
					metrics.NotePresentOutcome(out.Mode.String(), target.LastDamageAreaPx())
					// M-GPU-*: float render path routing into UI JSON (no ui→gpu).
					if dc := target.Context(); dc != nil {
						st := dc.RenderPathStats()
						metrics.NoteGPUPathStats(st.GPUOps, st.CPUFallbackOps, st.FrameFlushes, st.LastCPUFallbackReason)
					}
					// W1 R3: accumulate boundary skip/rerecord from last paint walk.
					rr, sk := LastBoundaryFrame()
					metrics.NoteBoundaryFrame(rr, sk)
					if cache := pipe.BoundaryCache(); cache != nil {
						// Lifetime totals also available; frame note uses last snap.
						_ = cache
					}
				}
				return err
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

// PaintPresentTree draws the RO tree into dc for window/GPU present (W0 FullPaint).
//
//	force=true  → full-surface clear + full tree paint; MarkFullRedraw (bootstrap/resize).
//	force=false → no UI-side full clear; still **full tree paint** (CompositeOnly=false).
//
// Why full paint on steady frames: GPU vector Present commonly LoadOpClears the
// surface. CompositeOnly would skip clean RepaintBoundaries and those pixels are
// gone after clear — static chrome/labels vanish in examples. True retained
// (Boundary → RT/Picture + blit Composite) is ENGINE_UI_WIDGET_RENDER W1–W2;
// until then window present must repaint the whole tree every frame for
// correctness. Partial CompositeOnly is available as PaintPresentTreeCompositeOnly
// for unit tests and future Retained policy.
//
// Layer-tree Present walk: scene.CompositeToContext / PaintPresentLayerTree.
func PaintPresentTree(dc *render.Context, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool) {
	paintPresentTree(dc, pipe, root, ov, cr, cg, cb, ca, force, false /* compositeOnly */)
}

// PaintPresentTreeCompositeOnly is the experimental partial-paint path:
// force=false uses FlushPaint CompositeOnly (skip clean boundaries). Safe on CPU
// pixmap tests; **unsafe** as default GPU window present until layer textures
// exist (LoadOpClear drops skipped content). Prefer PaintPresentTree for apps.
func PaintPresentTreeCompositeOnly(dc *render.Context, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool) {
	paintPresentTree(dc, pipe, root, ov, cr, cg, cb, ca, force, true)
}

func paintPresentTree(dc *render.Context, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force, compositeOnly bool) {
	if dc == nil || pipe == nil || root == nil {
		return
	}
	if force {
		dc.SetRGBA(cr, cg, cb, ca)
		dc.DrawRectangle(0, 0, float64(dc.Width()), float64(dc.Height()))
		_ = dc.Fill()
		dc.MarkFullRedraw()
	}
	pc := rendering.NewPaintContext(dc, dc.DeviceScale())
	// W1: reuse PipelineOwner's long-lived Picture cache so clean boundaries
	// skip re-record across frames (Replay still draws — GPU Clear safe).
	cache := pipe.BoundaryCache()
	if cache == nil {
		cache = rendering.NewBoundaryCache()
	}
	pc.BoundaryCache = cache
	pc.UseBoundaryCache = true
	cache.BeginFrame()
	// force clear path always full-paints; steady path full-paints unless
	// compositeOnly (test/Retained experiment).
	paintForce := force || !compositeOnly
	pipe.FlushPaint(pc, paintForce)
	if ov != nil {
		ov.Paint(pc)
	}
	// Publish per-frame boundary stats for metrics pickup (NoteBoundaryFrame).
	lastBoundaryFrame.Store(boundaryFrameSnap{
		Rerecord: cache.FrameRerecord,
		Skip:     cache.FrameSkip,
	})
}

// PaintPresentLayerTree clears (when force) then composites a retained layer
// tree via scene.CompositeToContext. Use when the frame is already recorded into
// PictureLayers (tests / future PipelineApp packet path).
func PaintPresentLayerTree(dc *render.Context, root scene.Layer, cr, cg, cb, ca float64, force bool) scene.CompositeStats {
	if dc == nil {
		return scene.CompositeStats{}
	}
	if force {
		dc.SetRGBA(cr, cg, cb, ca)
		dc.DrawRectangle(0, 0, float64(dc.Width()), float64(dc.Height()))
		_ = dc.Fill()
		dc.MarkFullRedraw()
	}
	return scene.CompositeToContext(root, dc)
}

// DamageAreaLogical returns the area (px²) of dc.FrameDamageUnion in logical pixels.
func DamageAreaLogical(dc *render.Context) int64 {
	if dc == nil {
		return 0
	}
	u := dc.FrameDamageUnion()
	if u.Empty() {
		return 0
	}
	return int64(u.Dx()) * int64(u.Dy())
}

// SurfaceAreaLogical is width*height of the drawing surface.
func SurfaceAreaLogical(dc *render.Context) int64 {
	if dc == nil {
		return 0
	}
	return int64(dc.Width()) * int64(dc.Height())
}

func presentTree(target *render.PresentTarget, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool) (render.PresentOutcome, error) {
	if target == nil || pipe == nil || root == nil {
		return render.PresentOutcome{}, errors.New("embedder: presentTree nil")
	}
	draw := func(dc *render.Context) {
		PaintPresentTree(dc, pipe, root, ov, cr, cg, cb, ca, force)
	}
	if force {
		err := target.PresentWith(draw)
		return target.LastPresentOutcome(), err
	}
	return target.PresentWithAuto(draw)
}

func (a *PipelineApp) presentSyncFull() {
	if a.target == nil {
		return
	}
	_, _ = presentTree(a.target, a.pipe, a.root, a.opts.Overlay, a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA, true)
	a.presents.Add(1)
}

// HitTestPointer runs overlay-first then main hit testing (logical coords).
func (a *PipelineApp) HitTestPointer(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry) {
	if a == nil {
		return overlay.BandNone, nil, nil
	}
	return overlay.HitTestStack(a.root, a.opts.Overlay, rendering.Point{X: x, Y: y})
}
