//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package raster runs the L1 raster thread: consumes frame jobs, calls render present.
// Must not import gpu (only render).
//
// All loops in a process share one pinned OS thread (hub.go): the GL context is
// current-on-one-thread and the frame-end present paths keep it that way, so a
// per-window thread would strand it against the other windows (EGL 0x3002, GL
// silently no-ops, frozen picture). A Loop is the per-window submission handle
// on that thread, not a thread of its own.
package raster

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

// DefaultPipelineDepth is Flutter-like double buffering (in-flight slots).
const DefaultPipelineDepth = 2

// FrameJob is work executed exclusively on the raster OS thread.
type FrameJob struct {
	// Run performs present/raster work. May be nil for a no-op slot.
	Run func() error
	// Done is closed after Run finishes (success or fail). Optional.
	Done chan error
}

// Loop is a window's submission handle on the process-wide GL thread
// (hub.go) with bounded pipeline depth.
type Loop struct {
	depth int
	jobs  chan FrameJob

	// mu serializes Start/Stop lifecycle transitions against each other and
	// against the hub's stop-drain ack. jobs is never closed and survives
	// restarts, so a job submitted while the loop is stopped still runs on the
	// next Start.
	mu      sync.Mutex
	started atomic.Bool
	// stopReq asks the hub to drain jobs after Stop, ack closes when the drain
	// finished (the GL thread is where the remaining jobs have to run).
	stopReq atomic.Bool
	ack     chan struct{}

	metrics  *scheduler.MetricsStore
	inflight atomic.Int32

	// pending is latest-wins when TrySubmit fails (unstarted frame coalesce).
	pending PendingSlot

	// inRaster is the raster-thread nesting depth for G5 thread assertions
	// (0 = off the GL thread). Raised only by exec on the GL thread; UI code
	// must observe 0.
	inRaster atomic.Int32
}

// NewLoop creates a raster loop with the given max in-flight jobs (min 1).
func NewLoop(depth int, metrics *scheduler.MetricsStore) *Loop {
	if depth < 1 {
		depth = DefaultPipelineDepth
	}
	return &Loop{
		depth:   depth,
		jobs:    make(chan FrameJob, depth),
		metrics: metrics,
	}
}

// Start registers the loop with the process-wide GL thread (starting it if this
// is the first loop). Safe to call after Stop: A stale pending latest-wins job
// is requeued so its Done waiter still fires.
func (l *Loop) Start() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if l.started.Load() {
		l.mu.Unlock()
		return
	}
	if pj, ok := l.pending.Take(); ok {
		select {
		case l.jobs <- pj:
		default:
			l.pending.Store(pj)
		}
	}
	l.stopReq.Store(false)
	l.started.Store(true)
	l.mu.Unlock()
	sharedHub.register(l)
}

func (l *Loop) exec(job FrameJob) {
	l.inflight.Add(1)
	// G5: mark raster-thread entry so UI/raster assertions observe it.
	// Nesting-safe: Stop from inside a job drains the queue here, so an inner
	// exec must not clear the outer job's mark.
	l.inRaster.Add(1)
	defer l.inRaster.Add(-1)
	if l.metrics != nil {
		l.metrics.SetPipeline(int(l.inflight.Load()), l.depth)
	}
	t0 := time.Now()
	var err error
	if job.Run != nil {
		err = job.Run()
	}
	if l.metrics != nil {
		l.metrics.NoteRasterMs(time.Since(t0).Seconds() * 1000)
		if err == nil && job.Run != nil {
			l.metrics.NotePresent()
		}
	}
	l.inflight.Add(-1)
	if l.metrics != nil {
		l.metrics.SetPipeline(int(l.inflight.Load()), l.depth)
	}
	if job.Done != nil {
		job.Done <- err
		close(job.Done)
	}
}

// Stop drains the loop's queued jobs on the GL thread and unregisters it.
// Safe to call twice and to follow with Start.
//
// After Stop returns, no job of this loop is running or queued, so the caller
// may take the GPU over (snapshot readback, teardown destroys). The drain
// itself runs on the GL thread: only that thread can issue GL work while a
// context binding is kept.
func (l *Loop) Stop() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if !l.started.Load() {
		l.mu.Unlock()
		return
	}
	l.started.Store(false)
	ack := make(chan struct{})
	l.ack = ack
	l.stopReq.Store(true)
	l.mu.Unlock()

	if sharedHub.isGLThread() {
		// Called from inside a job: the runner cannot ack work it is already
		// executing, so drain here instead of waiting for a sweep.
		l.drainStopped()
	} else {
		sharedHub.ping()
		<-ack
	}
	sharedHub.unregister(l)
}

// tryTake removes one queued job without blocking. GL thread only.
func (l *Loop) tryTake() (FrameJob, bool) {
	select {
	case job := <-l.jobs:
		return job, true
	default:
		return FrameJob{}, false
	}
}

// drainStopped runs the jobs a stopped loop still had queued, then acks the
// Stop caller. GL thread only: those jobs call render present, which needs the
// GL context current on this thread.
func (l *Loop) drainStopped() bool {
	if !l.stopReq.Load() {
		return false
	}
	for {
		job, ok := l.tryTake()
		if !ok {
			l.stopReq.Store(false)
			l.mu.Lock()
			if ack := l.ack; ack != nil {
				l.ack = nil
				close(ack)
			}
			l.mu.Unlock()
			return true
		}
		l.exec(job)
	}
}

// RunSync runs fn on the GL thread and waits for it to return.
//
// Frame work belongs to FrameJob; this is the handoff path for GL that is not
// a frame: warm-up full presents, snapshot readback, teardown destroys. The
// frame-end present paths keep the GL context current on the GL thread
// (Impeller model), so running such work on the UI thread bind-fails with
// EGL_BAD_ACCESS (0x3002) and every GL call silently no-ops.
//
// On a started loop the job joins the queue, so it runs after the presents
// already submitted; with no loop running it still reaches the GL thread.
// Called from the GL thread itself (inside a FrameJob), fn runs inline — the
// check is by thread id, not by loop state: a loop's own job running on the GL
// thread does not make the UI thread a GL thread.
func (l *Loop) RunSync(fn func() error) error {
	if fn == nil {
		return nil
	}
	if l == nil {
		return sharedHub.runJob(fn)
	}
	if sharedHub.isGLThread() {
		return fn()
	}
	if !l.started.Load() {
		return sharedHub.runJob(fn)
	}
	done := make(chan error, 1)
	job := FrameJob{Run: fn, Done: done}
	if !l.TrySubmit(job) {
		l.Submit(job)
	}
	return <-done
}

// TrySubmit enqueues a job without blocking. Returns false if the pipeline is full (backpressure).
func (l *Loop) TrySubmit(job FrameJob) bool {
	if l == nil {
		return false
	}
	select {
	case l.jobs <- job:
		// Wake the GL thread: it parks when the process has no work.
		sharedHub.ping()
		return true
	default:
		return false
	}
}

// SubmitLatest tries to enqueue; if full, stores job in pending (replaces previous
// unstarted pending). Never blocks the UI thread. Returns "queued" or "pending".
func (l *Loop) SubmitLatest(job FrameJob) (queued bool) {
	if l == nil {
		return false
	}
	if l.TrySubmit(job) {
		return true
	}
	l.pending.Store(job)
	return false
}

// Submit enqueues a job, waiting if the pipeline is full (backpressure).
// Prefer TrySubmit + drop/replace pending for Flutter-like coalesce of unstarted frames.
func (l *Loop) Submit(job FrameJob) {
	if l == nil {
		return
	}
	l.jobs <- job
	sharedHub.ping()
}

// Depth returns the number of frames currently queued in the loop.
// Returns 0 for a nil Loop.
func (l *Loop) Depth() int {
	if l == nil {
		return 0
	}
	return l.depth
}

// OnRasterThread reports whether the caller runs inside a raster FrameJob
// (G5 thread assertion helper). UI build code must observe false.
func (l *Loop) OnRasterThread() bool {
	if l == nil {
		return false
	}
	return l.inRaster.Load() > 0
}

// AssertRasterThread panics when called off the raster thread.
// Raster-only work (draw/composite/present) calls this first (G5).
func (l *Loop) AssertRasterThread() {
	if l != nil && l.inRaster.Load() <= 0 {
		panic("raster: AssertRasterThread called off raster thread")
	}
}

// AssertUIThread is UI convention only (G5): UI build/event code calls it to
// document thread intent. It cannot enforce with a shared atomic while a raster
// job runs concurrently on its own OS thread (a global flag would false-fire
// on the UI thread mid-job), so it is a no-op. The sound direction —
// AssertRasterThread inside FrameJob.Run — is enforced via inRaster.
// (T2 lesson: cross-goroutine UI assertion needs goroutine-local state.)
func (l *Loop) AssertUIThread() {}

// Full reports G4 backpressure: true when the bounded queue holds depth jobs.
// The UI thread checks this BEFORE building a packet (build-before-place):
// full means skip this frame's build and retry next vsync instead of wasting
// a build that SubmitLatest would only park in pending.
func (l *Loop) Full() bool {
	if l == nil {
		return false
	}
	return len(l.jobs) >= cap(l.jobs)
}

// HasPending reports whether an unstarted latest-wins job waits in pending.
func (l *Loop) HasPending() bool {
	if l == nil {
		return false
	}
	return l.pending.Has()
}

// TryReserve is the G4 build-before-place probe: false when the pipeline has
// no free slot (queue full or pending occupied). True is advisory only — the
// caller must still SubmitLatest and handle the pending path — but a false
// lets the UI skip the expensive build entirely for this frame.
func (l *Loop) TryReserve() bool {
	if l == nil {
		return false
	}
	if l.Full() || l.pending.Has() {
		return false
	}
	return true
}

// InFlight returns approximate number of jobs running or queued.
func (l *Loop) InFlight() int {
	if l == nil {
		return 0
	}
	return int(l.inflight.Load()) + len(l.jobs)
}
