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

	started atomic.Bool
	wg      sync.WaitGroup

	metrics  *scheduler.MetricsStore
	inflight atomic.Int32

	// pending is latest-wins when TrySubmit fails (unstarted frame coalesce).
	pending PendingSlot
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
func (l *Loop) Start() {
	if l == nil || !l.started.CompareAndSwap(false, true) {
		return
	}
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
func (l *Loop) Stop() {
	if l == nil || !l.started.Load() {
		return
	}
	select {
	case <-l.quit:
	default:
		close(l.quit)
	}
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

// SubmitReplace tries to enqueue; if full, runs the job on the caller only when
// forceSync is true. For P0 tests we use TrySubmit semantics in App.
func (l *Loop) Depth() int {
	if l == nil {
		return 0
	}
	return l.depth
}

// InFlight returns approximate number of jobs running or queued.
func (l *Loop) InFlight() int {
	if l == nil {
		return 0
	}
	return int(l.inflight.Load()) + len(l.jobs)
}
