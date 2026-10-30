//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package res

import "sync"

// InflightGate holds pooled resources in flight for submissions that are NOT
// framed by a Session (the filter and dual-tex caches submit straight to the
// queue). It is the multi-outstanding counterpart to Submission: Submission is
// one per-session window that opens at BeginFrame and closes at SubmitDone;
// a device-level pool may have any number of command buffers in flight at once,
// each with its own completion token.
//
// Lifecycle: Hold(refs) before Queue.Submit → attach the returned GateHandle to
// the hal.CompletionToken → Resolve once the token reports Done. Retired
// entries cannot be destroyed and idle entries cannot be re-lent while a handle
// holds them, which is exactly the race a size flip or purge could otherwise
// hit: the pool hands a texture to window B while window A's copy of it is
// still running on the GPU.
//
// Thread-safe: a device-level pool is shared by every window thread on that
// device, so submits and completions arrive concurrently.
type InflightGate struct {
	mu      sync.Mutex
	reg     *Registry
	pending map[*GateHandle][]uint32
}

// GateHandle is one outstanding hold. Resolve / Invalidate are idempotent: the
// first caller that removes it from the gate does the work, later ones no-op,
// so a token pump and a teardown path can both fire without double-decrementing
// the in-flight counters.
type GateHandle struct {
	gate *InflightGate
}

// NewInflightGate creates a gate bound to reg.
func NewInflightGate(reg *Registry) *InflightGate {
	if reg == nil {
		return nil
	}
	return &InflightGate{reg: reg, pending: make(map[*GateHandle][]uint32)}
}

// Hold marks every ref in flight and returns the handle that must be resolved
// once the GPU reports the submission finished. Nil refs and refs from another
// registry are skipped (same tolerance as Submission.Track).
//
// The registry is never touched while the gate lock is held: in-flight
// transitions can fire idle callbacks that re-enter a cache, and the lock order
// is cache → gate → registry.
func (g *InflightGate) Hold(refs ...Ref) *GateHandle {
	if g == nil || len(refs) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(refs))
	for _, ref := range refs {
		if ref.IsNil() || ref.reg != g.reg {
			continue
		}
		ids = append(ids, ref.id)
	}
	if len(ids) == 0 {
		return nil
	}
	h := &GateHandle{gate: g}
	for _, id := range ids {
		g.reg.markInflight(id, 1)
	}
	g.mu.Lock()
	if g.pending == nil {
		g.pending = make(map[*GateHandle][]uint32)
	}
	g.pending[h] = ids
	g.mu.Unlock()
	return h
}

// Resolve completes the submission: every held entry leaves the in-flight
// state, which is what finally lets a retired entry be destroyed and an idle
// entry be re-lent. No-op on an unknown or already-resolved handle.
func (h *GateHandle) Resolve() { h.settle(false) }

// Invalidate drops the hold without decrementing the in-flight counters — the
// device-loss path, where no fence will ever arrive and the abandon flow owns
// native teardown.
func (h *GateHandle) Invalidate() { h.settle(true) }

func (h *GateHandle) settle(deviceLost bool) {
	if h == nil || h.gate == nil {
		return
	}
	g := h.gate
	g.mu.Lock()
	ids, ok := g.pending[h]
	if !ok {
		g.mu.Unlock()
		return
	}
	delete(g.pending, h)
	g.mu.Unlock()
	for _, id := range ids {
		if deviceLost {
			g.reg.clearInflight(id)
			continue
		}
		g.reg.markInflight(id, -1)
	}
}

// Pending handles returns the number of unresolved holds (diagnostics).
func (g *InflightGate) Pending() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.pending)
}

// ResolveAll settles every outstanding hold (after an explicit device drain:
// WaitIdle proved the GPU finished, so no token needs to be waited on).
func (g *InflightGate) ResolveAll() { g.settleAll(false) }

// InvalidateAll drops every outstanding hold (device loss).
func (g *InflightGate) InvalidateAll() { g.settleAll(true) }

func (g *InflightGate) settleAll(deviceLost bool) {
	if g == nil {
		return
	}
	g.mu.Lock()
	all := g.pending
	g.pending = make(map[*GateHandle][]uint32)
	g.mu.Unlock()
	for _, ids := range all {
		for _, id := range ids {
			if deviceLost {
				g.reg.clearInflight(id)
				continue
			}
			g.reg.markInflight(id, -1)
		}
	}
}
