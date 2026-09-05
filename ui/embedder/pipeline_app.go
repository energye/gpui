package embedder

import (
	"errors"
	"fmt"
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
	// Input is the optional unified event router (plan §4). When set,
	// pointer/key events are normalized and routed automatically via the
	// router; the per-example OnEvent input handling is not needed. Windows
	// without Input keep the existing OnEvent path unchanged.
	Input *InputRouter
	// IME is the optional input-method capability; when both are set it is
	// attached to Input for automatic session management (plan I4).
	IME platform.IME
	// Clipboard is the optional cross-platform clipboard capability; when
	// both are set it is auto-injected into focused TextEditTargets so
	// apps no longer hand-wire each box (F-B5, X11 nil->fallback).
	Clipboard platform.Clipboard
	// WarmUp runs one full paint before the loop (F11).
	WarmUp bool
	// Overlay is the optional F13 overlay stack (P5d). Hit-test is overlay-first.
	Overlay *overlay.State
	// SaveLayerMaxOps / SaveLayerMaxArea configure the per-frame SaveLayerBudget
	// (F16, W2 R18). 0 = unlimited. Rejections are counted in savelayer_reject.
	SaveLayerMaxOps  int
	SaveLayerMaxArea float64
	// SnapshotPath (optional) saves a GPU readback PNG of the final frame
	// before Close. Use for AA/pixel verification windows: xwd reads the X11
	// backing store, which can lag/diverge from the composited GPU content.
	SnapshotPath string
}

// boundaryFrameSnap is the last paintPresentTree frame's boundary cache counters.
type boundaryFrameSnap struct {
	Rerecord int64
	Skip     int64
	// Shell partitioning (W2 R21): shell-tagged boundaries counted separately.
	ShellRerecord int64
	ShellSkip     int64
}

// lastBoundaryFrame is published by paintPresentTree for metrics pickup.
var lastBoundaryFrame atomic.Value // stores boundaryFrameSnap

// lastFiltersApplied publishes the per-frame filter layer count from the
// textured composite (raster thread) for metrics pickup (R20).
var lastFiltersApplied atomic.Int64

// resizeCalmWindow: after this long without a resize event the swapchain
// switches back from Mailbox/Immediate to Fifo (vsync).
const resizeCalmWindow = 200 * time.Millisecond

// PipelineApp runs layout/paint → FramePacket → async raster present.
// UI never blocks on Present (SubmitLatest).
type PipelineApp struct {
	host  platform.Host
	sched *scheduler.FrameScheduler
	loop  *raster.Loop
	pipe  *rendering.PipelineOwner
	root  rendering.RenderObject
	opts  PipelineOptions
	input *InputRouter

	target *render.PresentTarget

	quit      atomic.Bool
	presents  atomic.Int64
	frameID   atomic.Uint64
	lastStats scene.RasterStats
	// layoutFrames counts flushes that actually laid out (for S2 gate).
	layoutFrames atomic.Int64
	// forceFullPresent is set on warm-up/resize so the next present full-clears.
	// A counter (not bool): a swapchain rebuild (resize) leaves every buffer
	// undefined except the one written by the first full frame; retained frames
	// LoadOpLoad the other buffers → black artifacts. Full-present for several
	// consecutive frames covers all swapchain buffers (observed black bursts
	// after min/max resize cycles until a second full frame landed).
	forceFullPresent atomic.Int64
	// occluded latches window visibility: when fully obscured/minimized the
	// frame loop stops rendering (Flutter lifecycle paused → stop frames);
	// a visible-again event resumes scheduling.
	occluded atomic.Bool
	// lastResizeScale tracks the last DPR seen in EventResize; boundary
	// Pictures are only invalidated when the DPR actually changes (pure size
	// changes rely on tryReplay's size/fingerprint checks instead).
	lastResizeScale float64
	// lastResizeW/H track the last effective size seen in EventResize. WM
	// resize-drag floods many coalesced requests with the same final size;
	// skipping those avoids a full relayout + full-present storm per request.
	lastResizeW, lastResizeH int
	// pendingResize defers the relayout from the event loop to the render
	// frame boundary (applied in Run's frame step). Flutter/Skia-style:
	// resize events only record state; the frame driven by the scheduler
	// applies the latest size once per frame (read from the host at the frame
	// boundary), so a resize-drag event storm never monopolizes the event
	// loop and rendering keeps its pace while tracking the final size.
	// While set, the frame gate is bypassed (resize-storm immediate render).
	pendingResize bool
	// lowLatency / lastResizeAt drive the swapchain present-mode switch
	// during resize storms: while a resize is fresher than resizeCalmWindow,
	// the target presents with Mailbox/Immediate (SetVsync(false)) so content
	// frames are not blocked ~16.5ms per Fifo vblank; after the storm ends
	// (calm window elapsed with no new resize) the target returns to Fifo.
	lowLatency   atomic.Bool
	lastResizeAt time.Time
	// debugRepaint enables R12b overlay on live paints (not cache Replay).
	debugRepaint atomic.Bool
	// debugRepaintDraws accumulates NoteDebugRepaint counts across presents.
	debugRepaintDraws atomic.Int64
	// useRetained: steady frames use CompositeOnly paint + PresentWithAuto damage
	// (W2 R4). Warm-up/resize still full-paint. Default true = retained (W6).
	useRetained atomic.Bool

	// saveStats / saveBudget wire SaveLayer budget accounting (W2 R18):
	// saveStats accumulates allow/reject outcomes; saveBudget (nil = unlimited)
	// is reset every paint frame and injected into the paint PaintContext.
	saveStats  *rendering.SaveLayerStats
	saveBudget *rendering.SaveLayerBudget
	// lastSaveAllow/Reject deltas from the previous sampling (raster thread)
	// turn the cumulative stats into per-frame increments for NoteSaveLayer.
	lastSaveAllow  int64
	lastSaveReject int64

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

	// cacheInvalidations counts programmatic boundary-cache invalidations
	// issued via InvalidateBoundaryCache (R11 cache_invalidations metric).
	cacheInvalidations atomic.Int64

	// snapshotQueue holds snapshot closures from the UI thread, drained at the
	// END of the next raster job — serialized with presents so readback never
	// races the frame that owns the context/swapchain (§2.7/U21 pixel
	// assertions). UI side waits on each request's done channel.
	snapshotMu    sync.Mutex
	snapshotQueue []func()

	// oom tracks consecutive OOM-class present failures for the 1.3
	// exit-instead-of-black-loop contract (raster notes, Run reads).
	oom oomExit
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

// SnapshotAsync schedules fn to run at the end of the next raster frame and
// waits for it. The raster thread owns the render context and swapchain, so
// GPU readback (context.Image / SavePNG) must execute there — calling it from
// the UI thread races the present that owns the texture and aborts wgpu
// ("invalid texture for image copy texture"). This is the safe §2.7/U21
// pixel-sampling path: deterministic frame point (the next presented frame),
// serialized with the present.
func (a *PipelineApp) SnapshotAsync(fn func()) {
	if a == nil || fn == nil {
		return
	}
	done := make(chan struct{})
	a.snapshotMu.Lock()
	a.snapshotQueue = append(a.snapshotQueue, func() {
		defer close(done)
		fn()
	})
	a.snapshotMu.Unlock()
	a.ScheduleFrame()
	// Bounded spin: the raster thread runs independently, but the frame gate
	// may defer the next present by up to ~2 vsync intervals. Poll instead of
	// blocking the full wait so a deferred frame cannot stall the UI loop.
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		select {
		case <-done:
			return
		case <-time.After(2 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "embedder: snapshot request timed out")
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// drainSnapshots executes queued snapshot closures on the raster thread.
func (a *PipelineApp) drainSnapshots() {
	if a == nil || a.snapshotQueue == nil {
		return
	}
	a.snapshotMu.Lock()
	q := a.snapshotQueue
	a.snapshotQueue = nil
	a.snapshotMu.Unlock()
	for _, fn := range q {
		fn()
	}
}

// SetPresentPolicy sets window present strategy and metrics present_policy.
// Use scheduler.PresentPolicyFullPaint for correctness windows that need a
// full tree repaint every frame (R0/C0/R16/...). Default is retained since W6.
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

// LastShellBoundaryFrame returns the last frame's shell-tagged boundary
// skip/rerecord (W2 R21). A scrolling body must keep the shell rerecord at 0.
func LastShellBoundaryFrame() (shellRerecord, shellSkip int64) {
	v := lastBoundaryFrame.Load()
	if v == nil {
		return 0, 0
	}
	s, ok := v.(boundaryFrameSnap)
	if !ok {
		return 0, 0
	}
	return s.ShellRerecord, s.ShellSkip
}

// lastOverlayBandSnap is the last frame's band-separated dirty observation
// (R8): main-band dirty id count vs overlay-band dirty id count. The main
// count is taken from the packet BEFORE overlay ids are merged in, so an
// overlay open/close must leave it at its pre-overlay level.
type overlayBandSnap struct {
	MainDirtyCount    int
	OverlayDirtyCount int
}

var lastOverlayBandFrame atomic.Value // stores overlayBandSnap

// NoteOverlayBandFrame publishes the last frame's band dirty counts
// (called by the frame loop around AttachToPacket).
func noteOverlayBandFrame(mainDirty, overlayDirty int) {
	lastOverlayBandFrame.Store(overlayBandSnap{
		MainDirtyCount:    mainDirty,
		OverlayDirtyCount: overlayDirty,
	})
}

// LastOverlayBandFrame returns the last frame's band-separated dirty counts.
// MainDirtyCount excludes overlay dirties; OverlayDirtyCount is the overlay
// band portion only.
func LastOverlayBandFrame() (mainDirty, overlayDirty int) {
	v := lastOverlayBandFrame.Load()
	if v == nil {
		return 0, 0
	}
	s, ok := v.(overlayBandSnap)
	if !ok {
		return 0, 0
	}
	return s.MainDirtyCount, s.OverlayDirtyCount
}

// BoundaryCache returns the PipelineOwner's long-lived boundary Picture cache.
func (a *PipelineApp) BoundaryCache() *rendering.BoundaryCache {
	if a == nil || a.pipe == nil {
		return nil
	}
	return a.pipe.BoundaryCache()
}

// PictureTextures returns the retained-path layer texture cache (raster-thread
// owned; read-only inspection from snapshot closures is safe because drains
// run serialized on the raster thread).
func (a *PipelineApp) PictureTextures() *scene.PictureTextureCache {
	if a == nil {
		return nil
	}
	return a.pictureTex
}

// cacheEntryCount is the R14 combined live-entry count of both layer caches
// (boundary Picture cache + retained-path texture LRU).
func (a *PipelineApp) cacheEntryCount() int64 {
	n := int64(0)
	if c := a.BoundaryCache(); c != nil {
		n += int64(c.Len())
	}
	if t := a.PictureTextures(); t != nil {
		n += int64(t.Len())
	}
	return n
}

// cacheEvictions is the cumulative R14 eviction count across both caches
// (explicit budget drops + capacity LRU + generational sweep).
func (a *PipelineApp) cacheEvictions() int64 {
	n := int64(0)
	if c := a.BoundaryCache(); c != nil {
		n += c.Evictions
	}
	if t := a.PictureTextures(); t != nil {
		n += t.Evictions
	}
	return n
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
// W6 default present policy is retained (steady frames composite only dirty
// paths + damage present; warm-up/resize still full-paint).
func NewPipelineApp(host platform.Host, root rendering.RenderObject, opts PipelineOptions) *PipelineApp {
	if opts.ClearA == 0 && opts.ClearR == 0 && opts.ClearG == 0 && opts.ClearB == 0 {
		opts.ClearR, opts.ClearG, opts.ClearB, opts.ClearA = 0.10, 0.12, 0.16, 1
	}
	s := scheduler.New()
	s.Metrics().SetPresentPolicy(scheduler.PresentPolicyRetained)
	app := &PipelineApp{
		host:  host,
		sched: s,
		loop:  raster.NewLoop(raster.DefaultPipelineDepth, s.Metrics()),
		pipe:  rendering.NewPipelineOwner(root),
		root:  root,
		opts:  opts,
	}
	app.useRetained.Store(true) // W6 default: retained steady frames
	app.saveStats = &rendering.SaveLayerStats{}
	if opts.SaveLayerMaxOps > 0 || opts.SaveLayerMaxArea > 0 {
		app.saveBudget = &rendering.SaveLayerBudget{
			MaxOps:  opts.SaveLayerMaxOps,
			MaxArea: opts.SaveLayerMaxArea,
		}
	}
	// Wire the unified input router to this app's hit-test (plan §4).
	if opts.Input != nil {
		opts.Input.SetHitTest(app.HitTestPointer)
		if opts.IME != nil {
			opts.Input.AttachIME(opts.IME)
		}
		if opts.Clipboard != nil {
			opts.Input.AttachClipboard(opts.Clipboard)
		}
		app.input = opts.Input
	}
	return app
}

// SetInputRouter attaches (or replaces) the unified input router after
// construction. The router's hit-test is bound to this app's HitTestPointer.
// Pass nil to detach (fall back to OnEvent input handling).
func (a *PipelineApp) SetInputRouter(r *InputRouter) {
	if a == nil {
		return
	}
	if r != nil {
		r.SetHitTest(a.HitTestPointer)
	}
	a.input = r
}

// SaveLayerStats returns the cumulative SaveLayer allow/reject outcomes (W2 R18).
func (a *PipelineApp) SaveLayerStats() (allow, reject int64) {
	if a == nil || a.saveStats == nil {
		return 0, 0
	}
	return a.saveStats.Allow.Load(), a.saveStats.Reject.Load()
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

// Target returns the present target (nil before Open). Pixel-verification
// windows (§2.7/U21) use Target().Context().Image() to sample the composited
// frame at deterministic phase points — the same readback path SavePNG uses.
func (a *PipelineApp) Target() *render.PresentTarget {
	if a == nil {
		return nil
	}
	return a.target
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
	// ModePersistent already runs WaitEvents on a ≤animTick (16ms) timeout,
	// so the loop wakes by itself. Writing a wake byte here would only be
	// consumed by the same thread's next WaitEvents call, which returns
	// immediately on a wake and turns the pacing sleep into a busy spin:
	// ticker callbacks (and their per-frame work) then run at spin rate
	// while presents stay gated at 60fps. WakeUp stays for IDLE/TRANSIENT
	// waits, which block indefinitely and need a cross-thread interrupt.
	if a.host != nil && a.sched.Mode() != scheduler.ModePersistent {
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
// Open creates the present target and wires the optional platform
// notifications (swapchain size / hidden detach). Called automatically by
// Run when the target is not yet open; examples may call it eagerly.
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
	// The swapchain-resize notification lets the Wayland host declare xdg
	// window geometry in the same wire batch as the new-size buffer (a
	// geometry declared before the buffer catches up leaves mutter with
	// negative frame extents, which corrupt the maximize/unmaximize restore
	// size). Fires on the raster thread inside the present critical section.
	t.SetOnSwapchainResized(func(logicalW, logicalH int) {
		if ps, ok := a.host.(platform.SurfacePresenter); ok {
			ps.OnSurfaceResized(logicalW, logicalH)
		}
	})
	a.target = t
	// Multiwindow 1.3: report the actual backend + downgrade count once.
	if m := a.Metrics(); m != nil {
		m.NoteGPUBackend(t.GPUBackend(), t.Fallbacks())
	}
	return nil
}

// parkNativeSurface implements the hide side of the Wayland hide/show cycle
// (§6.2): wait for the raster thread to drain in-flight presents, close the
// GPU present target (releases the wgpu WSI surface), then destroy the
// platform surface stack via HiddenSurface — the wl_surface is only destroyed
// once no GPU work references it (a detach/present race would hang Present).
// Called on the event thread from the EventHidden{Hidden:true} branch.
func (a *PipelineApp) parkNativeSurface() {
	if a.loop != nil {
		deadline := time.Now().Add(2 * time.Second)
		for a.loop.InFlight() > 0 && time.Now().Before(deadline) {
			time.Sleep(2 * time.Millisecond)
		}
	}
	if a.target != nil {
		_ = a.target.Close()
		a.target = nil
	}
	if hs, ok := a.host.(platform.HiddenSurface); ok {
		hs.ApplyHiddenDetach()
	}
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
	// Raster is drained: no purge can be in flight; drop the layer-texture
	// cache from the OOM purge chain before its device goes away.
	if a.pictureTex != nil {
		render.UnregisterPurgeEvictable(a.pictureTex)
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
	a.forceFullPresent.Store(3) // first presents after open are always full
	a.firstPresentT0 = time.Now()
	if a.opts.WarmUp {
		a.presentSyncFull()
		a.forceFullPresent.Store(0) // warm-up already full-cleared swapchain
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
			// Unified input routing (plan §4): when an InputRouter is
			// attached, pointer/key events are normalized and dispatched by
			// the framework; per-example OnEvent input handling is skipped.
			if a.input != nil && (ev.Type == platform.EventPointer || ev.Type == platform.EventKey || ev.Type == platform.EventIME) {
				a.input.RoutePlatform(ev)
				continue
			}
			if a.opts.OnEvent != nil {
				a.opts.OnEvent(ev)
			}
			if EventQuits(ev) {
				a.quit.Store(true)
				continue
			}
			switch ev.Type {
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					sc := ev.Scale
					if sc <= 0 {
						sc = a.host.ScaleFactor()
					}
					// Coalesced WM resize requests repeat the final size many
					// times while dragging; a full relayout + full present per
					// request collapses fps (measured ~4x on X11/Mutter). Skip
					// requests where neither size nor DPR actually changed.
					if ev.Width == a.lastResizeW && ev.Height == a.lastResizeH && sc == a.lastResizeScale {
						continue
					}
					// Boundary Pictures survive a pure size change: tryReplay
					// re-checks each boundary's current size and content
					// fingerprint (boundary_cache.go) and only mismatches
					// re-record. DPR change is the exception (physical pixels
					// differ → old textures stale) → invalidate then.
					if sc != a.lastResizeScale {
						if cache := a.pipe.BoundaryCache(); cache != nil {
							cache.Clear()
						}
					}
					a.lastResizeW, a.lastResizeH = ev.Width, ev.Height
					a.lastResizeScale = sc
					if os.Getenv("WR_RESIZE_DBG") == "1" {
						fmt.Fprintf(os.Stderr, "DBG ev resize %dx%d sc=%v\n", ev.Width, ev.Height, sc)
					}
					// Defer the actual relayout to the render frame boundary:
					// a resize drag storm floods ConfigureNotify events; applying
					// each one synchronously here would monopolize the event
					// loop. One application per rendered frame tracks the latest
					// size while keeping frame pacing (the frame boundary reads
					// the current host size, so the swapchain is sized to the
					// window, not to a stale event).
					a.pendingResize = true
					a.lastResizeAt = time.Now()
					a.ScheduleFrame()
				}
			case platform.EventExpose:
				// Damage/full present redraws on demand. Reacting to every Expose
				// after Present causes a busy loop on X11.
				if !a.sched.Pending() {
					a.ScheduleFrame()
				}
			case platform.EventResizeSync:
				// Flutter/Skia: the WM's resize-sync request is a promise to
				// deliver a painted frame; schedule one now (the counter
				// advances when the frame is presented via FrameSync).
				if !a.sched.Pending() {
					a.ScheduleFrame()
				}
			case platform.EventFramePresented:
				// Compositor "frame shown" notice (块2): stamps a fresh pacing
				// timestamp only — demand stays with events/tickers, so idle
				// costs nothing (on-demand rendering, 块1).
				a.sched.NoteFramePresented()
			case platform.EventOccluded:
				// Window fully obscured or minimized → stop rendering
				// (Flutter lifecycle paused / Chrome hidden → no frames);
				// visible again → resume.
				a.occluded.Store(ev.Occluded)
				if ev.Occluded {
					a.sched.ClearPending()
				} else {
					a.ScheduleFrame()
				}
			case platform.EventHidden:
				// App-driven Hide/Show (Wayland §6.2): hide parks the render
				// loop, closes the GPU present target (wgpu WSI surface) and
				// then destroys the platform surface stack — the window truly
				// unmaps (xdg has no unmap request; GTK4 parity). Show
				// re-creates the stack on the platform side first; this event
				// is delivered only after the new surface is re-mapped, so
				// Open() below recreates the present target against the NEW
				// wl_surface and frames resume.
				a.occluded.Store(ev.Hidden)
				if ev.Hidden {
					a.sched.ClearPending()
					a.parkNativeSurface()
				} else {
					if a.target == nil {
						if err := a.Open(); err != nil {
							fmt.Fprintf(os.Stderr, "embedder: reopen present target after Show: %v\n", err)
							a.occluded.Store(true)
							continue
						}
					}
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

		// Window not visible (occluded/minimized): stop rendering entirely —
		// no frames are produced until a visible-again event. The vsync
		// listener may keep stamping, but the frame gate stays closed.
		if a.occluded.Load() {
			a.sched.ClearPending()
			continue
		}

		// Resize-storm present-mode restore: once no resize has arrived for
		// the calm window, switch the swapchain back to Fifo (vsync). The
		// switch is record-only (applied at the next present boundary).
		if a.lowLatency.Load() && time.Since(a.lastResizeAt) > resizeCalmWindow {
			a.lowLatency.Store(false)
			if a.target != nil {
				a.target.SetVsync(true)
			}
		}

		// Flutter frame-callback gate (non-blocking): render only when a
		// fresh vsync signal arrived or the software interval elapsed since
		// the last rendered frame. Skipping here just re-enters WaitEvents,
		// which keeps the cadence — the frame loop never blocks on vsync.
		//
		// Resize-storm bypass: an unapplied resize (pendingResize) renders
		// IMMEDIATELY, before the vsync gate. The CSD chrome tracks the
		// pointer synchronously (its buffers are rebuilt on every configure),
		// so a vsync-quantized content would visibly trail the title bar
		// during a fast drag; rendering each resize step now keeps the
		// content width in lockstep with the window. pendingResize clears
		// after the frame, so the gate re-closes until the next resize event
		// (no busy spin).
		if !a.sched.FrameDue() && !a.pendingResize {
			continue
		}

		// Layout only if dirty (S2: spinner phase must not layout).
		// Apply a deferred resize at the frame boundary, exactly once per
		// frame (Flutter/Skia model): the drag storm only records the latest
		// size; the actual relayout happens here so event processing never
		// monopolizes the loop and rendering keeps its pace during the drag.
		// The size used is the CURRENT host size — the same size the content
		// below is laid out and built at — so the swapchain extent matches
		// the rendered content and the window (an acquire is not constantly
		// "outdated" during interactive resize drags).
		w, h = a.host.Size()
		vp = rendering.Size{Width: float64(w), Height: float64(h)}
		resizeScale := a.host.ScaleFactor()
		if resizeScale <= 0 {
			resizeScale = 1
		}
		if a.pendingResize {
			a.pendingResize = false
			// PresentTarget.Resize is record-only: it updates the logical
			// size/context; the wgpu swapchain reconfigure (the expensive part
			// on llvmpipe) runs on the raster thread at the present boundary,
			// serialized with presents — the UI thread never stalls on the
			// surface and never races the raster thread's Configure. The size
			// is the CURRENT host size — the same size the content below is
			// laid out and built at — so the swapchain extent matches the
			// rendered content and the window (an acquire is not constantly
			// "outdated" during interactive resize drags).
			_ = a.target.Resize(w, h, resizeScale)
			if a.pictureTex != nil {
				a.pictureTex.Resize(w, h)
			}
			if a.pipe.FlushLayout(vp, true) {
				a.layoutFrames.Add(1)
			}
			// First storm frame: switch the swapchain to Mailbox/Immediate so
			// content frames present without waiting a Fifo vblank (~16.5ms)
			// per frame — the CSD title bar is a separate subsurface updated
			// per configure, so vsync-quantized content visibly trails it.
			if !a.lowLatency.Swap(true) && a.target != nil {
				a.target.SetVsync(false)
			}
		}
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
		pkt := rendering.BuildFramePacketWithSaveLayer(a.root, frameID, scale, float64(w), float64(h), a.saveStats, a.saveBudget)
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG frame %d viewport=%dx%d dirty=%v\n", frameID, w, h, pkt.DirtyLayerIDs)
		}
		if m := a.sched.Metrics(); m != nil {
			m.SetPictureOpCount(scene.CountPictureOps(pkt))
			mh, mm := rendering.TreeMeasureCacheStats(a.root)
			m.SetMeasureCacheStats(mh, mm)
		}
		if a.opts.Overlay != nil {
			// R8 band-separated dirty observation: sample the main-band
			// dirty count BEFORE AttachToPacket merges overlay ids into
			// DirtyLayerIDs; the overlay portion comes from the mirror list.
			mainDirtyPre := len(pkt.DirtyLayerIDs)
			a.opts.Overlay.AttachToPacket(pkt)
			noteOverlayBandFrame(mainDirtyPre, len(pkt.OverlayDirtyLayerIDs))
		}
		if pkt != nil {
			a.noteDirtyIDs(pkt.DirtyLayerIDs)
		}
		hitchRasterStart := time.Now()
		stats := scene.RasterizeDirty(pkt)
		if os.Getenv("HITCH_DIAG") == "1" {
			HitchNoteStageUI("rasterize", time.Since(hitchRasterStart))
		}
		a.lastStats = stats
		a.sched.Metrics().NoteBuildMs(time.Since(t0).Seconds() * 1000)
		if m := a.sched.Metrics(); m != nil {
			m.SetRasterLayerCount(int64(stats.RasterLayerCount))
			m.SetFilterLayerCount(lastFiltersApplied.Load())
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
		force := a.forceFullPresent.Swap(0) > 0
		metrics := a.sched.Metrics()
		// Keep policy visible; default retained since W6 (correctness windows
		// pin full_paint explicitly via SetPresentPolicy).
		if metrics != nil && metrics.PresentPolicy() == "" {
			metrics.SetPresentPolicy(scheduler.PresentPolicyRetained)
		}
		dbgOn := a.debugRepaint.Load()
		dbgAccum := &a.debugRepaintDraws
		// Retained steady frames: compositeOnly=true (skip clean boundaries).
		// Disable during target post-resize full recovery: the new swapchain
		// buffers are undefined until fully written (render/present_target.go),
		// and compositeOnly skips clean boundaries → black regions.
		compositeOnly := a.useRetained.Load() && !force && !a.inFullRecovery()
		if os.Getenv("WR_RESIZE_DBG") == "1" {
			fmt.Fprintf(os.Stderr, "DBG frame %d comp=%v force=%v recov=%v\n", frameID, compositeOnly, force, a.inFullRecovery())
		}
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
		jobFrameID := a.frameID.Load()
		job := raster.FrameJob{
			Run: func() error {
				if os.Getenv("HITCH_DIAG") == "1" {
					HitchSpanMark(jobFrameID, "run_start")
				}
				var frameDraws int64
				var frameVisits int64
				opts := paintPresentTreeOpts{
					debugRepaint:  dbgOn,
					debugDraws:    &frameDraws,
					paintVisits:   &frameVisits,
					compositeOnly: compositeOnly,
					layerStats:    a.saveStats,
					layerBudget:   a.saveBudget,
				}
				var out render.PresentOutcome
				var err error
				if compositeOnly && pkt != nil {
					out, err = presentPacketTextured(target, pkt, a, clearR, clearG, clearB, clearA, opts)
				} else {
					out, err = presentTreeOpts(target, pipe, root, ov, clearR, clearG, clearB, clearA, force, opts)
				}
				// FrameSync (X11 _NET_WM_SYNC_REQUEST): the counter must advance
				// only when a frame was actually presented — the compositor
				// unstretches on the counter, so advancing it on submit
				// (while the raster thread is still rendering a previous size)
				// would release the stretched placeholder against stale
				// content. The X property write itself is deferred to the
				// event pump thread (Xlib is not thread-safe).
				if err == nil {
					if fs, ok := a.host.(platform.FrameSync); ok {
						fs.NotifyFrameDrawn()
					}
					// Compositor frame-presented notice (块2): request the
					// next "frame shown" notice so pacing follows the display
					// server instead of a client-side waiter. No demand → no
					// next frame → no next request → fully idle (块1).
					if fn := platform.HostFrameNotifier(a.host); fn != nil {
						fn.RequestFrameNotify()
					}
				}
				if frameDraws > 0 {
					dbgAccum.Add(frameDraws)
				}
				if metrics != nil {
					// Boundary/shell/save-stats accumulation does not depend on a
					// resolved PresentTarget: bootstrap warm-up frames paint into
					// the surface but may not have issued a present yet. Sampling
					// here keeps the very first cold shell record (R21) honest.
					rr, sk := LastBoundaryFrame()
					metrics.NoteBoundaryFrame(rr, sk)
					srr, ssk := LastShellBoundaryFrame()
					metrics.NoteShellBoundaryFrame(srr, ssk)
					// W3 R7/R7b: virtual-list bind window + cumulative scroll
					// fresh-mount total (zero until a VirtualList has bound).
					if bind, items := rendering.LastVirtualBind(); bind > 0 || items > 0 {
						metrics.SetVirtualBind(bind, items)
					}
					metrics.SetScrollRerecord(rendering.ScrollRerecordTotal())
					// W6 R14: cache budget observability (combined entries +
					// cumulative evictions of both layer caches).
					metrics.SetCacheBudget(a.cacheEntryCount(), a.cacheEvictions())
					// W2 R18: accumulate SaveLayer budget outcomes this frame
					// (per-frame delta of the cumulative stats).
					if a.saveStats != nil {
						al, rj := a.saveStats.Allow.Load(), a.saveStats.Reject.Load()
						metrics.NoteSaveLayer(al-a.lastSaveAllow, rj-a.lastSaveReject)
						a.lastSaveAllow, a.lastSaveReject = al, rj
					}
					// R16 H-family: first loop present (warm-up path already recorded).
					if a.presents.Load() == 0 {
						a.recordFirstPresent()
					}
				}
				if metrics != nil && target != nil {
					mode := out.Mode.String()
					area := target.LastDamageAreaPx()
					metrics.NotePresentOutcome(mode, area)
					metrics.SetPaintVisits(frameVisits)
					a.noteDamage(area, mode)
					// M-GPU-*: float render path routing into UI JSON (no ui→gpu).
					if dc := target.Context(); dc != nil {
						st := dc.RenderPathStats()
						metrics.NoteGPUPathStats(st.GPUOps, st.CPUFallbackOps, st.FrameFlushes, st.LastCPUFallbackReason)
					}
				}
				// End of job: run any §2.7 snapshot requests — after present completed, so
				// readback sees this frame's composited pixels and cannot race the swapchain.
				a.drainSnapshots()
				if os.Getenv("HITCH_DIAG") == "1" {
					FrameDone(jobFrameID)
				}
				// Persistent GPU exhaustion exits via Run instead of black-looping
				// (multiwindow 1.3: never black-screen, never crash).
				if a.oom.note(err) {
					a.quit.Store(true)
				}
				return err
			},
		}
		// Async: never block UI on Present.
		if os.Getenv("HITCH_DIAG") == "1" {
			HitchSpanMark(jobFrameID, "submit")
		}
		_ = a.loop.SubmitLatest(job)
		a.presents.Add(1) // count submit as frame produced; present completes on raster thread
		a.sched.ClearPending()
		// Keep scheduling while tickers run.
		if a.sched.Tickers().HasActive() {
			a.ScheduleFrame()
		}
		a.sched.RecomputeMode()
	}

	// Final-frame snapshot (AA/pixel verification): xwd reads the X11 backing
	// store which can lag/diverge from composited GPU content, and window
	// presents are zero-readback (the context pixmap stays stale after
	// FlushGPUWithView + EndFrame releases the view). Drain the raster loop,
	// repaint the tree into the same context (the offscreen nil-view flush
	// path reads back into the pixmap with the same GPU session/MSAA), then
	// SavePNG the CPU pixmap.
	if a.opts.SnapshotPath != "" {
		a.loop.Stop() // wait for queued/in-flight presents before touching dc
		if dc := a.target.Context(); dc != nil {
			dc.BeginFrame()
			// Snapshot repaint must honor the same per-frame SaveLayer
			// budget/stats as loop frames — otherwise the captured "final
			// frame" diverges from what actually presented (C6: budgetless
			// repaint wrongly allowed the second offscreen group).
			paintPresentTreeWithOpts(dc, a.pipe, a.root, a.opts.Overlay,
				a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA, true, false,
				paintPresentTreeOpts{layerStats: a.saveStats, layerBudget: a.saveBudget})
			if err := dc.SavePNG(a.opts.SnapshotPath); err != nil {
				fmt.Fprintf(os.Stderr, "snapshot: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "snapshot: %s\n", a.opts.SnapshotPath)
			}
		}
	}
	if err := a.oom.runErr(); err != nil {
		return err
	}
	return nil
}

// PaintPresentTree draws the RO tree into dc for window/GPU present.
//
//	force=true  → full-surface clear + full tree paint; MarkFullRedraw (bootstrap/resize).
//	force=false → no UI-side full clear; still **full tree paint** (CompositeOnly=false).
//
// Steady retained frames go through PaintPresentTreeCompositeOnly +
// PresentWithAuto damage (LoadOpLoad keeps clean pixels); force frames use the
// full path. Partial CompositeOnly without damage present (GPU full Clear) is
// prohibited (§2.1 U11): CompositeOnly must pair with PresentWithAuto.
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
	debugDraws    *int64                     // optional: accumulate debug repaint draws (R12b)
	paintVisits   *int64                     // per-frame node visits (R2); nil = no count
	compositeOnly bool                       // W2 retained steady: skip clean boundaries (LoadOpLoad keeps pixels)
	layerStats    *rendering.SaveLayerStats  // W2 R18: allow/reject outcomes
	layerBudget   *rendering.SaveLayerBudget // W2 R18: per-frame SaveLayer limit (nil = unlimited)
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
	// W2 R18: per-frame SaveLayer budget + allow/reject outcome counting.
	if opts.layerBudget != nil {
		opts.layerBudget.Reset()
	}
	pc.LayerBudget = opts.layerBudget
	pc.LayerStats = opts.layerStats
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
		Rerecord:      cache.FrameRerecord,
		Skip:          cache.FrameSkip,
		ShellRerecord: cache.FrameShellRerecord,
		ShellSkip:     cache.FrameShellSkip,
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
	// Per-frame SaveLayer budget reset for the retained packet path: raster-
	// time OnPaint (RasterExtra) SaveLayer requests are gated during this
	// composite pass, so the whole frame shares one fresh budget — same
	// per-frame semantics paintPresentTreeWithOpts gives the direct path.
	opts.layerBudget.Reset()
	tex := a.pictureTex
	if tex == nil {
		tex = scene.NewPictureTextureCache(dc, 0)
		a.pictureTex = tex
	}
	// Size the texture LRU to the current frame's cacheable layer working
	// set (never shrinks). The default cap of 64 thrashes on dense scenes:
	// every eviction forces an offscreen re-record next frame (C4 measured
	// ~123 re-records/frame at 83ms raster with a 200+ layer shell+body+
	// overlay tree). +25% headroom absorbs transient scroll in/out layers.
	// Publish the live key set so eviction can never victimize an in-tree
	// layer (a live layer not yet blitted this frame looks "old" mid-phase-1).
	liveKeys, liveCount := scene.CollectCacheableKeys(pkt)
	tex.EnsureCapacity(liveCount + liveCount/4 + 16)
	tex.SetLiveKeys(liveKeys)
	pipe := a.pipe
	draw := func(d *render.Context) {
		// Clear: retained steady frames do not clear (LoadOpLoad keeps pixels);
		// only force frames (handled by presentTreeOpts) full-clear.
		st := scene.CompositeFramePacketTextured(pkt, d, tex)
		lastFiltersApplied.Store(int64(st.FiltersApplied))
		for _, r := range st.DamageRects {
			// Dirty layer geometry → damage rects (logical coords; dc scales
			// to physical via deviceScale). Overlay-band dirties share the
			// union path.
			d.TrackDamageRect(r)
		}
		// Boundary metrics: texture re-record = rerecord, cached blit = skip.
		// Shell partitioning (R21): shell-tagged BoundaryLayers report their
		// skip/rerecord via TexturedStats so a scrolling body proves the
		// shell textures never re-record under the retained path too.
		lastBoundaryFrame.Store(boundaryFrameSnap{
			Rerecord:      tex.FrameRerecord.Load(),
			Skip:          tex.FrameSkip.Load(),
			ShellRerecord: st.ShellRerecord,
			ShellSkip:     st.ShellSkip,
		})
		// Paint-dirty marks are consumed by the layer tree (no live paint).
		pipe.ConsumeNeedsPaint()
	}
	return target.PresentWithAuto(draw)
}

func SurfaceAreaLogical(dc *render.Context) int64 {
	if dc == nil {
		return 0
	}
	return int64(dc.Width()) * int64(dc.Height())
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

// inFullRecovery mirrors PresentTarget.InFullRecovery for the compositeOnly
// decision (lock-protected; same effective thread as target.Resize).
func (a *PipelineApp) inFullRecovery() bool {
	return a.target != nil && a.target.InFullRecovery()
}

func (a *PipelineApp) presentSyncFull() {
	if a.target == nil {
		return
	}
	var frameDraws int64
	opts := paintPresentTreeOpts{debugRepaint: a.debugRepaint.Load(), debugDraws: &frameDraws, layerStats: a.saveStats, layerBudget: a.saveBudget}
	_, _ = presentTreeOpts(a.target, a.pipe, a.root, a.opts.Overlay, a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA, true, opts)
	a.debugRepaintDraws.Add(frameDraws)
	// Warm-up full paint also produces boundary/shell cache stats (its very
	// first Store is the cold shell record R21 must report honestly).
	if m := a.sched.Metrics(); m != nil {
		rr, sk := LastBoundaryFrame()
		m.NoteBoundaryFrame(rr, sk)
		srr, ssk := LastShellBoundaryFrame()
		m.NoteShellBoundaryFrame(srr, ssk)
		if bind, items := rendering.LastVirtualBind(); bind > 0 || items > 0 {
			m.SetVirtualBind(bind, items)
		}
		m.SetScrollRerecord(rendering.ScrollRerecordTotal())
		// W6 R14: cache budget observability (same-source sample as the
		// retained frame path above).
		m.SetCacheBudget(a.cacheEntryCount(), a.cacheEvictions())
		if a.saveStats != nil {
			al, rj := a.saveStats.Allow.Load(), a.saveStats.Reject.Load()
			m.NoteSaveLayer(al-a.lastSaveAllow, rj-a.lastSaveReject)
			a.lastSaveAllow, a.lastSaveReject = al, rj
		}
	}
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
	if tex := a.PictureTextures(); tex != nil {
		tex.Clear()
	}
	a.cacheInvalidations.Add(1)
	if a.root != nil {
		a.root.MarkNeedsPaint()
	}
	a.forceFullPresent.Store(3)
	a.ScheduleFrame()
}

// CacheInvalidations returns the cumulative number of programmatic
// boundary-cache invalidations (R11 cache_invalidations metric).
func (a *PipelineApp) CacheInvalidations() int64 {
	if a == nil {
		return 0
	}
	return a.cacheInvalidations.Load()
}
