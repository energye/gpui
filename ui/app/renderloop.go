package app

import "sync/atomic"

// RenderLoop owns the dedicated render OS thread (aligns with gogpu internal/thread).
//
//	Main (Run/Pulse):  WaitEvents · Dispatch · Tick · OnUpdate
//	Render thread:     Present / tree.Frame / GPU (via RunOnRenderThread*)
//
// Call is synchronous by default so Update and Draw stay ordered (no concurrent
// mutation of the live Tree): main waits until present finishes.
type RenderLoop struct {
	th *Thread

	pendingW atomic.Uint32
	pendingH atomic.Uint32
	resize   atomic.Bool
}

// NewRenderLoop starts the render thread.
func NewRenderLoop() *RenderLoop {
	return &RenderLoop{th: NewThread()}
}

// Stop stops the render thread.
func (rl *RenderLoop) Stop() {
	if rl != nil && rl.th != nil {
		rl.th.Stop()
	}
}

// RequestResize queues a size change for the render thread (main thread → render).
func (rl *RenderLoop) RequestResize(w, h uint32) {
	if rl == nil {
		return
	}
	rl.pendingW.Store(w)
	rl.pendingH.Store(h)
	rl.resize.Store(true)
}

// ConsumePendingResize returns pending resize if any (call on render thread).
func (rl *RenderLoop) ConsumePendingResize() (w, h uint32, ok bool) {
	if rl == nil || !rl.resize.Swap(false) {
		return 0, 0, false
	}
	return rl.pendingW.Load(), rl.pendingH.Load(), true
}

// RunOnRenderThread runs f on the render thread and returns its value.
func (rl *RenderLoop) RunOnRenderThread(f func() any) any {
	if rl == nil || rl.th == nil {
		if f != nil {
			return f()
		}
		return nil
	}
	return rl.th.Call(f)
}

// RunOnRenderThreadVoid runs f on the render thread and waits.
func (rl *RenderLoop) RunOnRenderThreadVoid(f func()) {
	if rl == nil || rl.th == nil {
		if f != nil {
			f()
		}
		return
	}
	rl.th.CallVoid(f)
}

// RunOnRenderThreadAsync queues f without waiting.
func (rl *RenderLoop) RunOnRenderThreadAsync(f func()) {
	if rl == nil || rl.th == nil {
		if f != nil {
			f()
		}
		return
	}
	rl.th.CallAsync(f)
}

// IsRunning reports render thread liveness.
func (rl *RenderLoop) IsRunning() bool {
	return rl != nil && rl.th != nil && rl.th.IsRunning()
}
