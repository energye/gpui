package embedder

import (
	"errors"
	"fmt"
	"image"
	"os"
	"sync"
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
	// lastResizeScale tracks the last DPR seen in EventResize; boundary
	// Pictures are only invalidated when the DPR actually changes (pure size
	// changes rely on tryReplay's size/fingerprint checks instead).
	lastResizeScale float64
	// debugRepaint enables R12b overlay on live paints (not cache Replay).
	debugRepaint atomic.Bool
	// debugRepaintDraws accumulates NoteDebugRepaint counts across presents.
	debugRepaintDraws atomic.Int64
	// useRetained: steady frames use CompositeOnly paint + PresentWithAuto damage
	// (W2 R4). Warm-up/resize still full-paint. Default false = full_paint (W0).
	useRetained atomic.Bool

	// pictureTex is the cross-frame layer texture cache for the retained
	// textured-composite path (scene.CompositeFramePacketTextured). Lazy-created
	// on the raster thread; only touched by serialized FrameJobs.
	pictureTex *scene.PictureTextureCache

	// W2 damage / dirty-layer accumulators (steady presents).
	damageMu        sync.Mutex
	damageSumArea   int64
	damageMaxArea   int64
	damageSamples   int64
	damageMultiN    int64 // frames with present_mode damage_multi
	lastDirtyIDs    []uint64
	maxDirtyIDCount int
	lastPresentMode string

	// R16 H-family first-present observation: wall start of Open; recorded once
	// at the first present (warm-up full paint or first loop present).
	firstPresentT0       time.Time
	firstPresentRecorded atomic.Bool
}

// SetDebugRepaint toggles R12b repaint visualization for subsequent presents.
func (a *PipelineApp) SetDebugRepaint(on bool) {
	if a == nil {
		return
	}
	a.debugRepaint.Store(on)
}

// DebugRepaintDraws returns cumulative debug overlay strokes.
func (a *PipelineApp) DebugRepaintDraws() int64 {
	if a == nil {
		return 0
	}
	return a.debugRepaintDraws.Load()
}

// SetPresentPolicy sets window present strategy and metrics present_policy.
// Use scheduler.PresentPolicyRetained for W2 R4/R4b/C2 (CompositeOnly steady frames).
// Default remains full_paint until W6 makes retained the global default.
func (a *PipelineApp) SetPresentPolicy(policy string) {
	if a == nil || policy == "" {
		return
	}
	if m := a.Metrics(); m != nil {
		m.SetPresentPolicy(policy)
	}
	a.useRetained.Store(policy == scheduler.PresentPolicyRetained || policy == scheduler.PresentPolicyHybrid)
}

// DamageStats returns cumulative present damage samples (physical px² from PresentTarget).
// samples is the number of steady presents that reported area≥0.
func (a *PipelineApp) DamageStats() (sumArea, maxArea, samples, multiModeFrames int64) {
	if a == nil {
		return 0, 0, 0, 0
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	return a.damageSumArea, a.damageMaxArea, a.damageSamples, a.damageMultiN
}

// LastDirtyLayerIDs returns a copy of the most recent frame's DirtyLayerIDs.
func (a *PipelineApp) LastDirtyLayerIDs() []uint64 {
	if a == nil {
		return nil
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	if len(a.lastDirtyIDs) == 0 {
		return nil
	}
	out := make([]uint64, len(a.lastDirtyIDs))
	copy(out, a.lastDirtyIDs)
	return out
}

// MaxDirtyLayerIDCount is the peak len(DirtyLayerIDs) observed in one frame.
func (a *PipelineApp) MaxDirtyLayerIDCount() int {
	if a == nil {
		return 0
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	return a.maxDirtyIDCount
}

// LastPresentMode is the last PresentOutcome mode string (full|damage_union|damage_multi|idle).
func (a *PipelineApp) LastPresentMode() string {
	if a == nil {
		return ""
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	return a.lastPresentMode
}

func (a *PipelineApp) noteDamage(area int64, mode string) {
	if a == nil {
		return
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	a.damageSamples++
	a.damageSumArea += area
	if area > a.damageMaxArea {
		a.damageMaxArea = area
	}
	// Idle presents carry no drawing and are not the retained steady-state
	// mode (e.g. an X11 Expose-triggered frame with nothing dirty). Keep the
	// last real present mode so LastPresentMode/gates observe the true
	// retained incremental mode (damage_union/damage_multi) instead of the
	// event-noise idle frame.
	if mode != render.PresentModeIdle.String() {
		a.lastPresentMode = mode
	}
	if mode == "damage_multi" || mode == render.PresentModeDamageMulti.String() {
		a.damageMultiN++
		// R4b: cumulative damage_multi presents into metrics JSON.
		if m := a.sched.Metrics(); m != nil {
			m.NoteDamageMultiFrame()
		}
	}
}

func (a *PipelineApp) noteDirtyIDs(ids []uint64) {
	if a == nil {
		return
	}
	a.damageMu.Lock()
	defer a.damageMu.Unlock()
	a.lastDirtyIDs = append([]uint64(nil), ids...)
	if n := len(ids); n > a.maxDirtyIDCount {
		a.maxDirtyIDCount = n
	}
	// R4b: last-frame dirty layer ids into metrics JSON.
	if m := a.sched.Metrics(); m != nil {
		m.SetDirtyLayerIDs(ids)
	}
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
	a.firstPresentT0 = time.Now()
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
					if a.pictureTex != nil {
						a.pictureTex.Resize(ev.Width, ev.Height)
					}
					if a.pipe.FlushLayout(vp, true) {
						a.layoutFrames.Add(1)
					}
					// Boundary Pictures survive a pure size change: tryReplay
					// re-checks each boundary's current size and content
					// fingerprint (boundary_cache.go) and only mismatches
					// re-record. Clearing here would force a full rerecord
					// wave on EVERY resize step while dragging. DPR change is
					// the exception (physical pixels differ → old textures
					// stale) → invalidate then.
					if sc != a.lastResizeScale {
						if cache := a.pipe.BoundaryCache(); cache != nil {
							cache.Clear()
						}
						a.lastResizeScale = sc
					}
					// Repaint is scoped by layout: nodes whose size actually
					// changed mark themselves paint-dirty (Base.setSize), and
					// TextureCache entries rebuild on size mismatch. No blanket
					// root.MarkNeedsPaint here — that would re-record all 58
					// layers per resize step (each layer = one wgpu submit,
					// re-record frames stall 60ms–1.5s while dragging).
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
		// R3b compositing-bits flush (Flutter updateCompositingBits runs before
		// paint): propagate needsCompositing bottom-up and sample boundary
		// discovery so boundary_count / boundary_max_depth are honest JSON.
		a.pipe.UpdateCompositingBits()
		if m := a.sched.Metrics(); m != nil {
			bc, bd := rendering.CountRepaintBoundaries(a.root)
			m.SetBoundaryDiscovery(bc, bd)
		}
		pkt := rendering.BuildFramePacket(a.root, frameID, scale, float64(w), float64(h))
		if m := a.sched.Metrics(); m != nil {
			m.SetPictureOpCount(scene.CountPictureOps(pkt))
			mh, mm := rendering.TreeMeasureCacheStats(a.root)
			m.SetMeasureCacheStats(mh, mm)
		}
		if a.opts.Overlay != nil {
			a.opts.Overlay.AttachToPacket(pkt)
		}
		if pkt != nil {
			a.noteDirtyIDs(pkt.DirtyLayerIDs)
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
		// Steady frames: force=false → PresentWithAuto.
		// full_paint (W0 default): full tree paint every frame (GPU Clear safe).
		// retained (W2): CompositeOnly paint — only dirty paths; LoadOpLoad keeps static.
		// Warm-up / resize / open: force=true → full clear + full paint.
		force := a.forceFullPresent.Swap(false)
		metrics := a.sched.Metrics()
		// Keep policy visible; default full_paint until caller SetPresentPolicy(retained).
		if metrics != nil && metrics.PresentPolicy() == "" {
			metrics.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
		}
		dbgOn := a.debugRepaint.Load()
		dbgAccum := &a.debugRepaintDraws
		// Retained steady frames: compositeOnly=true (skip clean boundaries).
		compositeOnly := a.useRetained.Load() && !force
		// Retained textured path: drop stale layer textures on force frames
		// (bootstrap/resize) so the next retained frame re-records everything.
		if compositeOnly && a.pictureTex != nil && a.target != nil {
			a.pictureTex.EndFrame()
		}
		// Force frames (bootstrap/resize) paint the whole tree directly and do
		// NOT clear the texture cache: layer views survive a resize (each entry
		// rebuilds itself on size mismatch) and unchanged layers blit their old
		// texture on the next retained frame. Clearing here would force a full
		// 58-layer re-record wave per resize step while dragging — re-record
		// frames stall 60ms–1.5s because every layer submits independently.
		job := raster.FrameJob{
			Run: func() error {
				var frameDraws int64
				var frameVisits int64
				opts := paintPresentTreeOpts{
					debugRepaint:  dbgOn,
					debugDraws:    &frameDraws,
					paintVisits:   &frameVisits,
					compositeOnly: compositeOnly,
				}
				var out render.PresentOutcome
				var err error
				if compositeOnly && pkt != nil {
					out, err = presentPacketTextured(target, pkt, a, clearR, clearG, clearB, clearA, opts)
				} else {
					out, err = presentTreeOpts(target, pipe, root, ov, clearR, clearG, clearB, clearA, force, opts)
				}
				if frameDraws > 0 {
					dbgAccum.Add(frameDraws)
				}
				if metrics != nil && target != nil {
					mode := out.Mode.String()
					area := target.LastDamageAreaPx()
					if os.Getenv("WR_DIAG") == "1" {
						fmt.Fprintf(os.Stderr, "WR_DIAG2 area=%d mode=%s\n", area, mode)
					}
					metrics.NotePresentOutcome(mode, area)
					metrics.SetPaintVisits(frameVisits)
					a.noteDamage(area, mode)
					// M-GPU-*: float render path routing into UI JSON (no ui→gpu).
					if dc := target.Context(); dc != nil {
						st := dc.RenderPathStats()
						metrics.NoteGPUPathStats(st.GPUOps, st.CPUFallbackOps, st.FrameFlushes, st.LastCPUFallbackReason)
					}
					// W1 R3: accumulate boundary skip/rerecord from last paint walk.
					rr, sk := LastBoundaryFrame()
					metrics.NoteBoundaryFrame(rr, sk)
					// R16 H-family: first loop present (warm-up path already recorded).
					if a.presents.Load() == 0 {
						a.recordFirstPresent()
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

// paintPresentTreeOpts optional flags from PipelineApp (debug repaint / retained).
type paintPresentTreeOpts struct {
	debugRepaint  bool
	debugDraws    *int64 // per-frame; nil = no count
	paintVisits   *int64 // per-frame node visits (R2); nil = no count
	compositeOnly bool   // W2 retained steady: skip clean boundaries (LoadOpLoad keeps pixels)
}

func paintPresentTree(dc *render.Context, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force, compositeOnly bool) {
	paintPresentTreeWithOpts(dc, pipe, root, ov, cr, cg, cb, ca, force, compositeOnly, paintPresentTreeOpts{})
}

func paintPresentTreeWithOpts(dc *render.Context, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force, compositeOnly bool, opts paintPresentTreeOpts) {
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
	pc.DebugRepaint = opts.debugRepaint
	pc.DebugRepaintDraws = opts.debugDraws
	if opts.paintVisits != nil {
		pc.PaintVisits = opts.paintVisits
	}
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

// presentPacketTextured is the W2 R4 retained steady-frame present: composite
// the already-built FramePacket via cached layer textures (blit-only frame →
// GPU LoadOpLoad + per-rect scissor) instead of live FlushPaint. Dirty layer
// bounds become TrackDamageRect entries so PresentWithAuto reports a damage
// plan (damage_union / damage_multi) far smaller than the surface.
//
// Degrades to the plain present path when the packet/target is unavailable or
// textures cannot be created (no GPU) — frame content stays correct, damage
// reporting is honest (falls back to vector replay + MSAA full clear).
func presentPacketTextured(target *render.PresentTarget, pkt *scene.FramePacket, a *PipelineApp, cr, cg, cb, ca float64, opts paintPresentTreeOpts) (render.PresentOutcome, error) {
	if target == nil || pkt == nil || a == nil || a.pipe == nil {
		return render.PresentOutcome{}, errors.New("embedder: presentPacketTextured nil")
	}
	dc := target.Context()
	if dc == nil {
		return render.PresentOutcome{}, errors.New("embedder: presentPacketTextured no context")
	}
	tex := a.pictureTex
	if tex == nil {
		tex = scene.NewPictureTextureCache(dc, 0)
		a.pictureTex = tex
	}
	pipe := a.pipe
	draw := func(d *render.Context) {
		// Clear: retained steady frames do not clear (LoadOpLoad keeps pixels);
		// only force frames (handled by presentTreeOpts) full-clear.
		st := scene.CompositeFramePacketTextured(pkt, d, tex)
		for _, r := range st.DamageRects {
			// Dirty layer geometry → damage rects (logical coords; dc scales
			// to physical via deviceScale). Overlay-band dirties share the
			// union path.
			d.TrackDamageRect(r)
		}
		if os.Getenv("WR_DIAG") == "1" {
			fmt.Fprintf(os.Stderr, "WR_DIAG t=%d dirty=%v raster=%d skip=%d rects=%v first=%d createnil=%d flusherr=%d blitfail=%d fd=%v\n",
				time.Now().UnixMilli(), pkt.DirtyLayerIDs, st.RasterLayerCount, tex.FrameSkip.Load(), st.DamageRects, tex.Len(),
				tex.DiagCreateNil.Load(), tex.DiagFlushErr.Load(), tex.DiagBlitFail.Load(), d.FrameDamage())
		}
		// Boundary metrics: texture re-record = rerecord, cached blit = skip.
		lastBoundaryFrame.Store(boundaryFrameSnap{
			Rerecord: tex.FrameRerecord.Load(),
			Skip:     tex.FrameSkip.Load(),
		})
		// Paint-dirty marks are consumed by the layer tree (no live paint).
		pipe.ConsumeNeedsPaint()
		if os.Getenv("WR_FULLDAMAGE") == "1" {
			// Diagnostic: damage the full window every frame so the damage
			// path runs with a full-surface scissor — isolates small-scissor
			// issues.
			d.TrackDamageRect(image.Rect(0, 0, d.Width(), d.Height()))
		}
		if os.Getenv("WR_PROBE") == "1" {
			// Probe: flush the queued composite quads into an offscreen copy and
			// read it back to CPU — pixel truth of the composed scene (public
			// render API only; no swapchain readback needed).
			if pv, prel := d.CreateOffscreenTexture(d.Width(), d.Height()); !pv.IsNil() && prel != nil {
				if perr := d.FlushGPUWithView(pv, uint32(d.Width()), uint32(d.Height())); perr != nil { //nolint:gosec
					fmt.Fprintf(os.Stderr, "WR_PROBE flush err: %v\n", perr)
				}
				if serr := d.SavePNG("/tmp/wr_probe_scene.png"); serr != nil {
					fmt.Fprintf(os.Stderr, "WR_PROBE save err: %v\n", serr)
				}
				prel()
			}
		}
	}
	if os.Getenv("WR_FORCEFULL") == "1" {
		// Diagnostic: force a full present of the textured composite so blits
		// are drawn unscissored — distinguishes composite bugs from the
		// LoadOpLoad/scissor damage-plan.
		err := target.PresentWith(draw)
		return target.LastPresentOutcome(), err
	}
	return target.PresentWithAuto(draw)
}

// SurfaceAreaLogical is width*height of the drawing surface.
func SurfaceAreaLogical(dc *render.Context) int64 {
	if dc == nil {
		return 0
	}
	return int64(dc.Width()) * int64(dc.Height())
}

func presentTree(target *render.PresentTarget, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool) (render.PresentOutcome, error) {
	return presentTreeOpts(target, pipe, root, ov, cr, cg, cb, ca, force, paintPresentTreeOpts{})
}

func presentTreeOpts(target *render.PresentTarget, pipe *rendering.PipelineOwner, root rendering.RenderObject, ov *overlay.State, cr, cg, cb, ca float64, force bool, opts paintPresentTreeOpts) (render.PresentOutcome, error) {
	if target == nil || pipe == nil || root == nil {
		return render.PresentOutcome{}, errors.New("embedder: presentTree nil")
	}
	// force always full-paints; retained steady uses opts.compositeOnly.
	co := opts.compositeOnly && !force
	draw := func(dc *render.Context) {
		paintPresentTreeWithOpts(dc, pipe, root, ov, cr, cg, cb, ca, force, co, opts)
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
	var frameDraws int64
	opts := paintPresentTreeOpts{debugRepaint: a.debugRepaint.Load(), debugDraws: &frameDraws}
	_, _ = presentTreeOpts(a.target, a.pipe, a.root, a.opts.Overlay, a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA, true, opts)
	a.debugRepaintDraws.Add(frameDraws)
	a.presents.Add(1)
	a.recordFirstPresent()
}

// recordFirstPresent publishes the H-family first-present observation exactly
// once (R16): wall ms from Open, whether the warm-up full paint ran, and the
// pipe paint count at that moment (>0 = first frame has content). The first
// caller wins; later frames are ignored.
func (a *PipelineApp) recordFirstPresent() {
	if a == nil || a.firstPresentRecorded.Swap(true) {
		return
	}
	ms := 0.0
	if !a.firstPresentT0.IsZero() {
		ms = time.Since(a.firstPresentT0).Seconds() * 1000
	}
	if m := a.sched.Metrics(); m != nil {
		var paintCount int64
		if p := a.pipe; p != nil {
			paintCount = p.PaintCount
		}
		m.SetFirstPresent(ms, a.opts.WarmUp, paintCount)
	}
}

// HitTestPointer runs overlay-first then main hit testing (logical coords).
func (a *PipelineApp) HitTestPointer(x, y float64) (overlay.Band, rendering.RenderObject, *overlay.Entry) {
	if a == nil {
		return overlay.BandNone, nil, nil
	}
	return overlay.HitTestStack(a.root, a.opts.Overlay, rendering.Point{X: x, Y: y})
}

// InvalidateBoundaryCache drops all Picture-backed boundary caches (R11).
// Call after programmatic size/DPR changes that do not go through EventResize.
func (a *PipelineApp) InvalidateBoundaryCache() {
	if a == nil || a.pipe == nil {
		return
	}
	if cache := a.pipe.BoundaryCache(); cache != nil {
		cache.Clear()
	}
	if a.root != nil {
		a.root.MarkNeedsPaint()
	}
	a.forceFullPresent.Store(true)
	a.ScheduleFrame()
}
