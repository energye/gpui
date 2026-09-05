package embedder

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/render"
)

// oomFailThreshold caps consecutive OOM-class present failures before the
// window exits with a human-readable error instead of black-looping
// (multiwindow 1.3). Single transient failures (TDR reclaim, concurrent
// allocation spikes) ride through; persistent exhaustion cannot.
const oomFailThreshold = 3

// oomExit tracks consecutive OOM-class present failures and latches the
// human-readable exit reason once. Embedded by App and PipelineApp (raster
// thread notes, UI thread reads the latched error).
type oomExit struct {
	fails atomic.Int64
	mu    sync.Mutex
	err   error
}

// note tracks one present result: nil and non-OOM errors reset the streak,
// OOM errors advance it. Reports true once the window must exit.
func (o *oomExit) note(err error) bool {
	if o == nil {
		return false
	}
	if err == nil || !render.IsGPUOutOfMemory(err) {
		o.fails.Store(0)
		return false
	}
	if o.fails.Add(1) < oomFailThreshold {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.err != nil {
		return true
	}
	o.err = fmt.Errorf("embedder: GPU out of memory persists across %d frames (%v); close other GPU windows or set GPUI_POWER to a less-loaded GPU",
		oomFailThreshold, err)
	return true
}

// runErr returns the latched exit reason, if any.
func (o *oomExit) runErr() error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.err
}
