//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package raster

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// hub is the process-wide single GL thread: every Loop in the process runs its
// frame jobs on this one OS thread (Flutter SurfaceManager / Impeller raster
// thread model).
//
// Why one thread for all windows: EGL admits one binding thread per context,
// and the frame-end present paths keep that binding between calls (gles
// UnlockKeepBinding — Chromium's single-surface frame). A per-window thread
// would find the shared context already current on another window's thread:
// eglMakeCurrent then fails with EGL_BAD_ACCESS (0x3002) and every GL call of
// that window silently no-ops — the picture freezes while the UI keeps ticking.
//
// Head-of-line note: a window whose present blocks (swapchain acquire, driver
// stall) delays the other windows' frames. Frames drop, they do not freeze —
// each loop keeps its own queue and latest-wins pending slot, so a delayed
// window resumes at its newest frame.
type hub struct {
	mu    sync.Mutex
	loops []*Loop
	// snap is the lock-free copy of loops the runner sweeps.
	snap atomic.Pointer[[]*Loop]
	// jobs carries GL-thread work that belongs to no window loop: handoff
	// releases, snapshot readback, teardown destroys.
	jobs chan FrameJob
	wake chan struct{}
	once sync.Once
	// tid is the runner's OS thread id (0 before it starts), so a caller can
	// tell whether it already runs on the GL thread. Thread identity is not
	// available on every platform (see threadid_*.go): a non-positive value
	// means "unknown", and no caller is then treated as the GL thread.
	tid atomic.Int64
}

// hubJobQueue bounds loop-less GL-thread work. Callers are serialized UI-thread
// handoff paths (snapshot, teardown), so a small queue is enough.
const hubJobQueue = 8

var sharedHub = &hub{
	jobs: make(chan FrameJob, hubJobQueue),
	wake: make(chan struct{}, 1),
}

// register adds l to the runner's sweep set, starting the GL thread on first
// use. Safe to call repeatedly with the same loop.
func (h *hub) register(l *Loop) {
	h.mu.Lock()
	found := false
	for _, e := range h.loops {
		if e == l {
			found = true
			break
		}
	}
	if !found {
		h.loops = append(h.loops, l)
		cp := make([]*Loop, len(h.loops))
		copy(cp, h.loops)
		h.snap.Store(&cp)
	}
	h.mu.Unlock()
	h.once.Do(func() { go h.run() })
	h.ping()
}

// unregister removes l from the sweep set. Only called once l's queue has been
// drained (Stop's handshake), so no queued job is left behind unrun.
func (h *hub) unregister(l *Loop) {
	h.mu.Lock()
	for i, e := range h.loops {
		if e == l {
			h.loops = append(h.loops[:i], h.loops[i+1:]...)
			cp := make([]*Loop, len(h.loops))
			copy(cp, h.loops)
			h.snap.Store(&cp)
			break
		}
	}
	h.mu.Unlock()
}

// runJob runs fn on the GL thread and waits for it to return. Used by
// Loop.RunSync when no loop is currently registered: handoff work still has to
// reach the thread that owns the GL binding.
func (h *hub) runJob(fn func() error) error {
	if fn == nil {
		return nil
	}
	if h.isGLThread() {
		// Already the GL thread (called from inside a job): run inline, a
		// handoff would wait on the thread that is waiting.
		return fn()
	}
	h.once.Do(func() { go h.run() })
	done := make(chan error, 1)
	h.jobs <- FrameJob{Run: fn, Done: done}
	h.ping()
	return <-done
}

// isGLThread reports whether the caller already runs on the GL thread. False
// when thread identity is unavailable: the caller may then only be a UI thread
// (RunSync's contract forbids calling it from inside a job on such a platform).
func (h *hub) isGLThread() bool {
	t := h.tid.Load()
	return t > 0 && t == currentTID()
}

// ping wakes the runner without blocking: the cap-1 channel coalesces bursts
// (a wake that arrives while the runner is sweeping is not lost, it just does
// not need to be repeated).
func (h *hub) ping() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}

// run is the GL thread: pinned for its whole life, because a GL context kept
// current on a thread stays valid only while that OS thread lives.
func (h *hub) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	h.tid.Store(currentTID())
	defer h.tid.Store(0)
	for {
		if !h.sweep() {
			<-h.wake
		}
	}
}

// sweep runs queued work without blocking and reports whether it ran anything.
// The caller keeps sweeping while work remains and parks when the process is
// idle, so an idle process costs nothing.
func (h *hub) sweep() bool {
	ran := h.drainHubJobs()
	snap := h.snap.Load()
	if snap == nil {
		return ran
	}
	for _, l := range *snap {
		if l.drainStopped() {
			ran = true
		}
		if job, ok := l.tryTake(); ok {
			l.exec(job)
			ran = true
		}
		// Latest-wins slot: replace means the older unstarted frame is thrown
		// away, so draining it right after the started job keeps the pipeline
		// from carrying a stale frame into the next vsync.
		if pj, ok := l.pending.Take(); ok {
			l.exec(pj)
			ran = true
		}
	}
	return ran
}

// drainHubJobs runs every queued loop-less GL-thread job.
func (h *hub) drainHubJobs() bool {
	ran := false
	for {
		select {
		case job := <-h.jobs:
			if job.Run != nil {
				err := job.Run()
				if job.Done != nil {
					job.Done <- err
					close(job.Done)
				}
			} else if job.Done != nil {
				job.Done <- nil
				close(job.Done)
			}
			ran = true
		default:
			return ran
		}
	}
}
