// Package embedder glues Host + FrameScheduler + raster.Loop for L1.
package embedder

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/raster"
	"github.com/energye/gpui/ui/scheduler"
)

// EventQuits reports whether the platform event must end the embedder main
// loop — the §2.4 consumer contract: EventCloseRequested (interceptable ✕:
// the window stays alive unless the app closes it, and EventClose then
// fires) and EventClose (window already destroyed) are treated alike.
func EventQuits(ev platform.Event) bool {
	return ev.Type == platform.EventCloseRequested || ev.Type == platform.EventClose
}

// Options configures App.
type Options struct {
	// Clear is the PresentClear color (0–1). Default dark gray-blue.
	ClearR, ClearG, ClearB, ClearA float64
	// ContinuousClear schedules a clear every frame while running (demo only).
	// Production L1 demand mode leaves this false (IDLE until ScheduleFrame).
	ContinuousClear bool
	// OnEvent is called for each host event before default handling.
	OnEvent func(ev platform.Event)
	// MaxFrames stops after N presents when > 0 (tests / timed demos).
	MaxFrames int64
	// RunFor stops the loop after this duration when > 0.
	RunFor time.Duration
}

// App is the L1 application shell (P0: clear present path).
type App struct {
	host   platform.Host
	sched  *scheduler.FrameScheduler
	loop   *raster.Loop
	target *render.PresentTarget
	opts   Options

	quit     atomic.Bool
	presents atomic.Int64
	// oom tracks consecutive OOM-class present failures for the 1.3
	// exit-instead-of-black-loop contract (UI thread notes and reads).
	oom oomExit
}

// New creates an App. Call Open then Run.
func New(host platform.Host, opts Options) *App {
	if opts.ClearA == 0 && opts.ClearR == 0 && opts.ClearG == 0 && opts.ClearB == 0 {
		opts.ClearR, opts.ClearG, opts.ClearB, opts.ClearA = 0.12, 0.14, 0.18, 1
	}
	s := scheduler.New()
	return &App{
		host:  host,
		sched: s,
		loop:  raster.NewLoop(raster.DefaultPipelineDepth, s.Metrics()),
		opts:  opts,
	}
}

// Scheduler returns the frame scheduler.
func (a *App) Scheduler() *scheduler.FrameScheduler {
	if a == nil {
		return nil
	}
	return a.sched
}

// Metrics returns frame metrics.
func (a *App) Metrics() *scheduler.MetricsStore {
	if a == nil || a.sched == nil {
		return nil
	}
	return a.sched.Metrics()
}

// Open creates the GPU present target from the host native surface.
func (a *App) Open() error {
	if a == nil || a.host == nil {
		return errors.New("embedder: nil app or host")
	}
	if a.target != nil {
		return nil
	}
	t, err := OpenPresentTarget(a.host)
	if err != nil {
		return err
	}
	a.target = t
	// Multiwindow 1.3: report the actual backend + downgrade count once.
	if m := a.Metrics(); m != nil {
		m.NoteGPUBackend(t.GPUBackend(), t.Fallbacks())
	}
	return nil
}

// Target returns the present target (nil before Open).
func (a *App) Target() *render.PresentTarget {
	if a == nil {
		return nil
	}
	return a.target
}

// ScheduleFrame requests a clear/present frame.
func (a *App) ScheduleFrame() {
	if a != nil && a.sched != nil {
		a.sched.ScheduleFrame()
		// ModePersistent runs WaitEvents on a ≤animTick (16ms) timeout, so
		// the loop wakes by itself; a wake byte here would only be consumed
		// by the same thread's next WaitEvents and spin the pacing sleep.
		if a.host != nil && a.sched.Mode() != scheduler.ModePersistent {
			a.host.WakeUp()
		}
	}
}

// Quit requests loop exit.
func (a *App) Quit() {
	if a == nil {
		return
	}
	a.quit.Store(true)
	if a.host != nil {
		a.host.WakeUp()
	}
}

// Close stops the raster loop and releases the present target.
func (a *App) Close() {
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

// Run opens (if needed), starts the raster thread, and runs the UI event loop
// until Quit, MaxFrames, RunFor, or EventClose.
func (a *App) Run() error {
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

	deadline := time.Time{}
	if a.opts.RunFor > 0 {
		deadline = time.Now().Add(a.opts.RunFor)
	}

	// First frame immediately so the window is not blank forever in demand mode.
	a.ScheduleFrame()

	for !a.quit.Load() {
		if !deadline.IsZero() && time.Now().After(deadline) {
			break
		}
		if a.opts.MaxFrames > 0 && a.presents.Load() >= a.opts.MaxFrames {
			break
		}

		if a.opts.ContinuousClear {
			a.sched.ScheduleFrame()
		}

		timeout := a.sched.WaitTimeout()
		evs := a.host.WaitEvents(timeout)
		for _, ev := range evs {
			if a.opts.OnEvent != nil {
				a.opts.OnEvent(ev)
			}
			if EventQuits(ev) {
				a.quit.Store(true)
				continue
			}
			switch ev.Type {
			case platform.EventResize:
				if a.target != nil && ev.Width > 0 && ev.Height > 0 {
					scale := ev.Scale
					if scale <= 0 {
						scale = a.host.ScaleFactor()
					}
					_ = a.target.Resize(ev.Width, ev.Height, scale)
					a.ScheduleFrame()
				}
			case platform.EventExpose:
				a.ScheduleFrame()
			}
		}

		// Pace animating frames (vsync or fallback).
		if a.sched.Mode() == scheduler.ModePersistent || a.opts.ContinuousClear {
			a.sched.WaitFramePace(a.host)
		}

		_ = a.sched.Tick()
		a.sched.RecomputeMode()

		if !a.sched.Pending() && !a.opts.ContinuousClear {
			continue
		}

		// Build + submit clear frame (P0). UI does not Wait Present.
		t0 := time.Now()
		a.sched.Metrics().NoteFrameInterval(t0)
		clearR, clearG, clearB, clearA := a.opts.ClearR, a.opts.ClearG, a.opts.ClearB, a.opts.ClearA
		target := a.target
		done := make(chan error, 1)
		job := raster.FrameJob{
			Run: func() error {
				if target == nil {
					return errors.New("embedder: nil target")
				}
				return target.PresentClear(clearR, clearG, clearB, clearA)
			},
			Done: done,
		}
		// Flutter-like: if pipeline full, replace by waiting for one slot then submit
		// (P0 does not drop frames silently without at least trying).
		if !a.loop.TrySubmit(job) {
			// Backpressure: wait for capacity by blocking Submit.
			a.loop.Submit(job)
		}
		// Do not block UI on present for demand path — optional sync for MaxFrames count:
		// We still need present count; use non-blocking select with short wait only when counting.
		select {
		case err := <-done:
			a.sched.Metrics().NoteBuildMs(time.Since(t0).Seconds() * 1000)
			if err != nil {
				// Persistent GPU exhaustion exits instead of black-looping
				// (multiwindow 1.3: never black-screen, never crash).
				if a.oom.note(err) {
					return a.oom.runErr()
				}
				// Surface errors: keep running unless fatal nil target.
				if a.opts.MaxFrames > 0 {
					return fmt.Errorf("present: %w", err)
				}
			} else {
				a.oom.note(nil)
				a.presents.Add(1)
			}
		case <-time.After(2 * time.Second):
			// Present hung — still clear pending to avoid spin.
		}
		a.sched.ClearPending()
		a.sched.RecomputeMode()
	}
	return a.oom.runErr()
}

// PresentCount returns successful presents observed by Run.
func (a *App) PresentCount() int64 {
	if a == nil {
		return 0
	}
	return a.presents.Load()
}
