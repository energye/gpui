package render

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// OOMExitThreshold caps consecutive OOM-class failures before a window exits
// with a human-readable error instead of black-looping (multiwindow 1.3).
// Single transient failures (TDR reclaim, concurrent allocation spikes) ride
// through; persistent exhaustion cannot.
//
// Single authority for both OOM exit decisions: the open-time readiness gate
// (waitDeviceReady in present_target.go, which degrades rather than exits)
// and the runtime present loop (ui/embedder, which exits) share this
// threshold, the IsGPUOutOfMemory phrasing, and the exit message below.
// Two decision points stay — open must degrade (high→low→software) while
// runtime must exit — but one threshold and one message speak.
const OOMExitThreshold = 3

// OOMExit tracks consecutive OOM-class failures and latches the
// human-readable exit reason once. Embedded by App and PipelineApp in
// ui/embedder (raster thread notes, UI thread reads the latched error).
// Nil-receiver inert so optional holders stay safe.
type OOMExit struct {
	fails atomic.Int64
	mu    sync.Mutex
	err   error
}

// Note tracks one result: nil and non-OOM errors reset the streak, OOM
// errors advance it. Reports true once the window must exit.
func (o *OOMExit) Note(err error) bool {
	if o == nil {
		return false
	}
	if err == nil || !IsGPUOutOfMemory(err) {
		o.fails.Store(0)
		return false
	}
	if o.fails.Add(1) < OOMExitThreshold {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return true
	}
	o.err = fmt.Errorf("embedder: GPU out of memory persists across %d frames (%v); close other GPU windows or set GPUI_POWER to a less-loaded GPU",
		OOMExitThreshold, err)
	return true
}

// RunErr returns the latched exit reason, if any.
func (o *OOMExit) RunErr() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.err
}
