// Package raster runs the L1 raster thread: consumes frame jobs, calls render present.
// Must not import gpu (only render).
package raster

import (
	"runtime"
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

// Loop is a dedicated OS-thread consumer with bounded pipeline depth.
type Loop struct {
	depth int
	jobs  chan FrameJob
	quit  chan struct{}

	// mu serializes Start/Stop lifecycle transitions (R0-7): Stop closes
	// quit, so a later Start must create a fresh quit channel — otherwise
	// the new goroutine sees the closed quit immediately and exits without
	// ever consuming jobs. jobs is never closed and survives restarts.
	mu      sync.Mutex
	started atomic.Bool
	wg      sync.WaitGroup

	metrics  *scheduler.MetricsStore
	inflight atomic.Int32

	// pending is latest-wins when TrySubmit fails (unstarted frame coalesce).
	pending PendingSlot

	// inRaster marks raster-thread execution for G5 thread assertions.
	// Set only by exec on the raster OS thread; UI code must observe false.
	inRaster atomic.Bool
}

// NewLoop creates a raster loop with the given max in-flight jobs (min 1).
func NewLoop(depth int, metrics *scheduler.MetricsStore) *Loop {
	if depth < 1 {
		depth = DefaultPipelineDepth
	}
	return &Loop{
		depth:   depth,
		jobs:    make(chan FrameJob, depth),
		quit:    make(chan struct{}),
		metrics: metrics,
	}
}

// Start launches the raster goroutine locked to an OS thread.
// Safe to call after Stop: a fresh quit channel is created so the new
// goroutine does not exit on the previously closed quit (R0-7). A stale
// pending latest-wins job is requeued so its Done waiter still fires.
func (l *Loop) Start() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started.Load() {
		return
	}
	l.quit = make(chan struct{})
	if pj, ok := l.pending.Take(); ok {
		select {
		case l.jobs <- pj:
		default:
			l.pending.Store(pj)
		}
	}
	l.started.Store(true)
	l.wg.Add(1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer l.wg.Done()
		for {
			select {
			case <-l.quit:
				// Drain remaining jobs so Submit waiters unblock.
				for {
					select {
					case job := <-l.jobs:
						l.exec(job)
					default:
						return
					}
				}
			case job := <-l.jobs:
				l.exec(job)
				// Prefer draining pending latest frame after each job.
				if pj, ok := l.pending.Take(); ok {
					l.exec(pj)
				}
			}
		}
	}()
}

func (l *Loop) exec(job FrameJob) {
	l.inflight.Add(1)
	// G5: mark raster-thread entry so UI/raster assertions observe it.
	l.inRaster.Store(true)
	defer l.inRaster.Store(false)
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

// Stop signals the loop to exit and waits.
// Safe to call twice and to follow with Start (R0-7 restartable).
func (l *Loop) Stop() {
	if l == nil {
		return
	}
	l.mu.Lock()
	if !l.started.Load() {
		l.mu.Unlock()
		return
	}
	select {
	case <-l.quit:
	default:
		close(l.quit)
	}
	l.mu.Unlock()
	l.wg.Wait()
	l.started.Store(false)
}

// TrySubmit enqueues a job without blocking. Returns false if the pipeline is full (backpressure).
func (l *Loop) TrySubmit(job FrameJob) bool {
	if l == nil {
		return false
	}
	select {
	case l.jobs <- job:
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
	return l.inRaster.Load()
}

// AssertRasterThread panics when called off the raster thread.
// Raster-only work (draw/composite/present) calls this first (G5).
func (l *Loop) AssertRasterThread() {
	if l != nil && !l.inRaster.Load() {
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
