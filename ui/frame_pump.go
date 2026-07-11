//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License 2.0
//
//----------------------------------------

package ui

import (
	"sync/atomic"

	"github.com/energye/lcl/lcl"
)

// FramePump orchestrates redraw requests for TGPUControl.
//
// Architecture (following gogpu's Invalidator + AnimationController patterns):
//   - Dedicated goroutine processes invalidation signals from a buffered(1) channel
//   - Each signal dispatches RunOnMainThreadSync(Invalidate) to the LCL main thread
//   - Multiple concurrent RequestRedraw calls coalesce into a single Invalidate
//   - AnimationController uses reference counting for multiple independent animations
//
// This is the foundation for Effect System, State Machine, and Event Handling.
type FramePump struct {
	ctrl *TGPUControl

	// Invalidator — buffered(1) channel for lock-free coalescing.
	// Multiple RequestRedraw calls produce exactly one signal.
	requestCh chan struct{}

	// Stop signal — close to terminate the pump goroutine.
	stopCh chan struct{}

	// Running state
	running atomic.Bool

	// AnimationController — reference-counted animations.
	// When count > 0, the pump keeps scheduling frames.
	animCount atomic.Int32
}

// NewFramePump creates a new FramePump for the given control.
func NewFramePump(ctrl *TGPUControl) *FramePump {
	return &FramePump{
		ctrl:      ctrl,
		requestCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}
}

// Start launches the FramePump goroutine. Safe to call multiple times (idempotent).
func (p *FramePump) Start() {
	if !p.running.CompareAndSwap(false, true) {
		return // already running
	}
	go p.run()
}

// Stop terminates the FramePump goroutine. Safe to call multiple times (idempotent).
func (p *FramePump) Stop() {
	if p.running.CompareAndSwap(true, false) {
		close(p.stopCh)
	}
}

// RequestRedraw requests a single frame redraw.
// Safe to call from any goroutine. Multiple concurrent calls coalesce.
func (p *FramePump) RequestRedraw() {
	select {
	case p.requestCh <- struct{}{}:
	default: // already pending — coalesced
	}
}

// IsAnimating reports whether any animations are currently active.
func (p *FramePump) IsAnimating() bool {
	return p.animCount.Load() > 0
}

// StartAnimation increments the animation counter and requests a frame.
// Returns an AnimationToken that must be Stop()'d when the animation completes.
// Multiple animations can coexist independently.
func (p *FramePump) StartAnimation() *AnimationToken {
	p.animCount.Add(1)
	p.RequestRedraw()
	return &AnimationToken{pump: p}
}

// run is the FramePump goroutine. It processes invalidation signals and
// dispatches Invalidate to the LCL main thread via RunOnMainThreadSync.
func (p *FramePump) run() {
	for {
		select {
		case <-p.requestCh:
			// Dispatch to main thread — blocks until LCL executes the callback.
			// This ensures the Invalidate is posted before we listen for the next signal.
			lcl.RunOnMainThreadSync(func() {
				p.ctrl.Invalidate()
			})
		case <-p.stopCh:
			return
		}
	}
}

// AnimationToken represents an active animation.
// Call Stop() when the animation completes to allow the pump to idle.
type AnimationToken struct {
	pump    *FramePump
	stopped atomic.Bool
}

// Stop signals the animation is complete. Idempotent — safe to call multiple times.
func (t *AnimationToken) Stop() {
	if t.stopped.CompareAndSwap(false, true) {
		t.pump.animCount.Add(-1)
	}
}
