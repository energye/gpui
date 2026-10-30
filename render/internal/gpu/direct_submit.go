//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// directSubmit is the device-level submission helper for the shared pools that
// record and submit their own command buffers (filter graph, dual-tex blend).
// Those submits happen outside any session frame, so res.Submission — one
// in-flight window per session — cannot cover them. This type pairs two things:
//
//   - res.InflightGate: every resource the command buffer touches is held
//     in-flight, so the pool cannot re-lend it to another window and a retired
//     entry cannot be destroyed while the GPU still reads or writes it.
//   - the hal completion token: taken right after Submit, so the hold ends
//     exactly when that work ends — not at the next frame boundary and not on a
//     blocking wait.
//
// Pumping is non-blocking and happens at the next submit of any shared pool and
// at the session's existing submission-completion points. A submission whose
// backend gives no token stays held until a real drain (WaitIdle proved the GPU
// finished) or device loss, because guessing would reopen the race.
type directSubmit struct {
	mu   sync.Mutex
	gate *res.InflightGate
	live []directCompletion
}

// directCompletion is one outstanding submission awaiting its fence.
type directCompletion struct {
	token hal.CompletionToken
	hold  *res.GateHandle
}

// newDirectSubmit returns the helper for one device registry.
func newDirectSubmit(reg *res.Registry) *directSubmit {
	if reg == nil {
		return nil
	}
	return &directSubmit{gate: res.NewInflightGate(reg)}
}

// submit holds every ref for the duration of the submission and submits the
// command buffers. refs may be empty (nothing pooled is touched), in which case
// it is a plain Queue.Submit.
func (d *directSubmit) submit(queue hal.Queue, refs []res.Ref, cbs ...hal.CommandBuffer) (uint64, error) {
	if queue == nil {
		return 0, fmt.Errorf("gpu: direct submit on nil queue")
	}
	if d == nil || d.gate == nil {
		return queue.Submit(cbs...)
	}
	d.pump()
	hold := d.gate.Hold(refs...)
	idx, err := queue.Submit(cbs...)
	if hold == nil {
		return idx, err
	}
	if err != nil {
		// Nothing reached the GPU: the hold would never be resolved.
		hold.Resolve()
		return idx, err
	}
	tok, terr := queue.OnSubmittedWorkDone()
	if terr != nil || tok == nil {
		d.mu.Lock()
		d.live = append(d.live, directCompletion{hold: hold})
		d.mu.Unlock()
		return idx, nil
	}
	d.mu.Lock()
	d.live = append(d.live, directCompletion{token: tok, hold: hold})
	d.mu.Unlock()
	return idx, nil
}

// pump resolves every completion the GPU has reported finished. Non-blocking;
// safe to call from any thread and on every frame.
func (d *directSubmit) pump() {
	if d == nil {
		return
	}
	d.mu.Lock()
	var done, keep []directCompletion
	for _, c := range d.live {
		if c.token == nil {
			keep = append(keep, c)
			continue
		}
		if c.token.Done() {
			done = append(done, c)
		} else {
			keep = append(keep, c)
		}
	}
	d.live = keep
	d.mu.Unlock()
	// Handles resolve outside the lock: clearing in-flight can fire registry
	// idle callbacks that re-enter the pools (lock order cache → gate → registry).
	for _, c := range done {
		c.hold.Resolve()
	}
}

// drain resolves everything after the caller proved the GPU is idle (WaitIdle /
// device teardown): no fence needs to be waited on.
func (d *directSubmit) drain() {
	if d == nil {
		return
	}
	d.mu.Lock()
	live := d.live
	d.live = nil
	d.mu.Unlock()
	for _, c := range live {
		c.hold.Resolve()
	}
	d.gate.ResolveAll()
}

// invalidate drops every hold without decrementing in-flight (device loss: the
// fence never arrives and the abandon flow owns native teardown).
func (d *directSubmit) invalidate() {
	if d == nil {
		return
	}
	d.mu.Lock()
	live := d.live
	d.live = nil
	d.mu.Unlock()
	for _, c := range live {
		c.hold.Invalidate()
	}
	d.gate.InvalidateAll()
}

// submitGated is the shared-pool submit call: with a gate it holds refs for the
// duration of the submission, without one (bare cache literals in tests,
// registry-less paths) it is a plain Queue.Submit.
func submitGated(d *directSubmit, queue hal.Queue, refs []res.Ref, cbs ...hal.CommandBuffer) (uint64, error) {
	if d != nil && d.gate != nil {
		return d.submit(queue, refs, cbs...)
	}
	if queue == nil {
		return 0, fmt.Errorf("gpu: submit on nil queue")
	}
	return queue.Submit(cbs...)
}
