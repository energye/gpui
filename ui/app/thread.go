package app

import (
	"runtime"
	"sync/atomic"
)

// Thread is a dedicated OS thread that serializes callbacks (gogpu/Ebiten pattern).
// Used as the GPU/render thread so window WaitEvents stays responsive during
// Present / swapchain work.
//
// Transport is a buffered chan of funcs — not a frame ring buffer.
type Thread struct {
	funcs   chan func()
	done    chan struct{}
	running atomic.Bool
}

// NewThread starts a goroutine locked to an OS thread.
func NewThread() *Thread {
	t := &Thread{
		funcs: make(chan func(), 16),
		done:  make(chan struct{}),
	}
	t.running.Store(true)
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		close(ready)
		for {
			select {
			case f := <-t.funcs:
				if f != nil {
					f()
				}
			case <-t.done:
				// Drain is optional; stop means no more Call.
				return
			}
		}
	}()
	<-ready
	return t
}

type threadResult struct {
	val   any
	panic any
}

// Call runs f on the thread and waits for the result.
func (t *Thread) Call(f func() any) any {
	if t == nil || !t.running.Load() {
		return nil
	}
	done := make(chan threadResult, 1)
	t.funcs <- func() {
		var r threadResult
		func() {
			defer func() { r.panic = recover() }()
			r.val = f()
		}()
		done <- r
	}
	r := <-done
	if r.panic != nil {
		panic(r.panic)
	}
	return r.val
}

// CallVoid runs f on the thread and waits until it finishes.
func (t *Thread) CallVoid(f func()) {
	if t == nil || !t.running.Load() {
		return
	}
	done := make(chan threadResult, 1)
	t.funcs <- func() {
		var r threadResult
		func() {
			defer func() { r.panic = recover() }()
			f()
		}()
		done <- r
	}
	r := <-done
	if r.panic != nil {
		panic(r.panic)
	}
}

// CallAsync queues f without waiting. If the queue is full, runs synchronously
// via CallVoid to avoid dropping GPU work.
func (t *Thread) CallAsync(f func()) {
	if t == nil || !t.running.Load() {
		return
	}
	select {
	case t.funcs <- f:
	default:
		t.CallVoid(f)
	}
}

// Stop signals the thread to exit. In-flight Call still completes.
func (t *Thread) Stop() {
	if t == nil {
		return
	}
	if t.running.Swap(false) {
		close(t.done)
	}
}

// IsRunning reports whether Stop has not been called.
func (t *Thread) IsRunning() bool {
	return t != nil && t.running.Load()
}
