//go:build !(js && wasm)

package webgpu

import (
	"fmt"
	"image"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Queue handles command submission and data transfers.
// On the wgpu-native backend, this wraps rwgpu Queue.
type Queue struct {
	r        *rwgpu.Queue
	device   *Device // parent device for lost gate (nil-safe)
	released bool
}

// OnSubmittedWorkDone registers a callback that fires once all GPU work
// submitted before this call has completed (P6, Skia command-buffer refs).
// The returned rwgpu Future must be polled (Device.Poll /
// rwgpu.WaitForFuture) for the callback to fire; the frame path uses the
// existing BeginFrame vsync/drainQueue sync points instead of blocking.
func (q *Queue) OnSubmittedWorkDone() (rwgpu.Future, error) {
	if q == nil || q.released {
		return rwgpu.Future{}, ErrReleased
	}
	if q.r == nil {
		return rwgpu.Future{}, nil
	}
	return q.r.OnSubmittedWorkDone()
}

// Submit submits command buffers for execution.
// Returns a submission index that can be used to track completion.
//
// R7.0: avoid per-submit heap allocation on the dominant 1-CB path and for
// small multi-CB submits (≤8). Semantics unchanged: non-nil CBs are marked
// submitted; only CBs with a live native handle are passed to rwgpu.
// Implements hal.Queue (takes hal.CommandBuffer interfaces, internal unpack;
// wrong-type elements fail before anything is marked or submitted).
func (q *Queue) Submit(commandBuffers ...hal.CommandBuffer) (uint64, error) {
	if err := prepareQueueCall(q); err != nil {
		return 0, err
	}
	// First pass: unpack interfaces (atomic type check before any marking).
	native := make([]*CommandBuffer, 0, len(commandBuffers))
	for _, cb := range commandBuffers {
		if cb == nil {
			native = append(native, nil)
			continue
		}
		wcb, ok := cb.(*CommandBuffer)
		if !ok {
			return 0, fmt.Errorf("wgpu: Submit: not a webgpu command buffer (%T)", cb)
		}
		native = append(native, wcb)
	}
	n := len(native)
	if n == 0 {
		idx, err := q.r.Submit()
		if err != nil {
			if e := mapRWGPUErr(err); e != err {
				return 0, e
			}
			return 0, fmt.Errorf("wgpu: submit failed: %w", err)
		}
		return idx, nil
	}
	// Dominant present/flush path: a single command buffer.
	if n == 1 {
		cb := native[0]
		if cb == nil {
			idx, err := q.r.Submit()
			if err != nil {
				if e := mapRWGPUErr(err); e != err {
					return 0, e
				}
				return 0, fmt.Errorf("wgpu: submit failed: %w", err)
			}
			return idx, nil
		}
		cb.submitted = true
		if cb.r == nil {
			idx, err := q.r.Submit()
			if err != nil {
				if e := mapRWGPUErr(err); e != err {
					return 0, e
				}
				return 0, fmt.Errorf("wgpu: submit failed: %w", err)
			}
			return idx, nil
		}
		idx, err := q.r.Submit(cb.r)
		if err != nil {
			if e := mapRWGPUErr(err); e != err {
				return 0, e
			}
			return 0, fmt.Errorf("wgpu: submit failed: %w", err)
		}
		return idx, nil
	}

	var stack [8]*rwgpu.CommandBuffer
	var rBuffers []*rwgpu.CommandBuffer
	if n <= len(stack) {
		rBuffers = stack[:0]
	} else {
		rBuffers = make([]*rwgpu.CommandBuffer, 0, n)
	}
	for _, cb := range native {
		if cb == nil {
			continue
		}
		// Always mark non-nil command buffers as submitted to prevent reuse,
		// even if the underlying native buffer is nil (e.g., discarded encoding).
		cb.submitted = true
		if cb.r != nil {
			rBuffers = append(rBuffers, cb.r)
		}
	}

	idx, err := q.r.Submit(rBuffers...)
	if err != nil {
		if e := mapRWGPUErr(err); e != err {
			return 0, e
		}
		return 0, fmt.Errorf("wgpu: submit failed: %w", err)
	}

	return idx, nil
}

// Poll returns the last completed submission index. Non-blocking.
// On the wgpu-native backend, returns 0 (wgpu-native does not expose poll on queue).
func (q *Queue) Poll() uint64 {
	return 0
}

// WriteBuffer writes data to a buffer.
// Implements hal.Queue (takes hal.Buffer interface, internal unpack).
func (q *Queue) WriteBuffer(buffer hal.Buffer, offset uint64, data []byte) error {
	if err := prepareQueueCall(q); err != nil {
		return err
	}
	wb, ok := buffer.(*Buffer)
	if !ok {
		return fmt.Errorf("wgpu: WriteBuffer: not a webgpu buffer (%T)", buffer)
	}
	if wb == nil || wb.r == nil {
		return fmt.Errorf("wgpu: WriteBuffer: buffer is nil")
	}
	if err := q.r.WriteBuffer(wb.r, offset, data); err != nil {
		if e := mapRWGPUErr(err); e != err {
			return e
		}
		return err
	}
	return nil
}

// WriteTexture writes data to a texture.
// R7.0: stack-allocate destination/layout/size descriptors (no per-call heap).
// Implements hal.Queue (ImageCopyTexture aliased to hal; .Texture unpacked).
func (q *Queue) WriteTexture(dst *hal.ImageCopyTexture, data []byte, layout *hal.ImageDataLayout, size *hal.Extent3D) error {
	if err := prepareQueueCall(q); err != nil {
		return err
	}
	if dst == nil {
		return fmt.Errorf("wgpu: WriteTexture: destination is nil")
	}
	tex, ok := dst.Texture.(*Texture)
	if !ok || tex == nil || tex.r == nil {
		return fmt.Errorf("wgpu: WriteTexture: destination is nil")
	}

	rDst := rwgpu.ImageCopyTexture{
		Texture:  tex.r,
		MipLevel: dst.MipLevel,
		Origin:   rwgpu.Origin3D(dst.Origin),
		Aspect:   rwgpu.TextureAspect(dst.Aspect),
	}

	var rLayout rwgpu.ImageDataLayout
	var rLayoutPtr *rwgpu.ImageDataLayout
	if layout != nil {
		rLayout = rwgpu.ImageDataLayout{
			Offset:       layout.Offset,
			BytesPerRow:  layout.BytesPerRow,
			RowsPerImage: layout.RowsPerImage,
		}
		rLayoutPtr = &rLayout
	}

	var rSize rwgpu.Extent3D
	var rSizePtr *rwgpu.Extent3D
	if size != nil {
		rSize = rwgpu.Extent3D{
			Width:              size.Width,
			Height:             size.Height,
			DepthOrArrayLayers: size.DepthOrArrayLayers,
		}
		rSizePtr = &rSize
	}

	if err := q.r.WriteTexture(&rDst, data, rLayoutPtr, rSizePtr); err != nil {
		if e := mapRWGPUErr(err); e != err {
			return e
		}
		return err
	}
	return nil
}

// SetSwapchainSuppressed is a no-op on the wgpu-native backend.
func (q *Queue) SetSwapchainSuppressed(_ bool) {}

// LastSubmissionIndex returns the most recent submission index.
// On the wgpu-native backend, submission indices are not tracked. Returns 0.
func (q *Queue) LastSubmissionIndex() uint64 {
	return 0
}

// Release drops the native queue reference obtained via Device.Queue /
// wgpuDeviceGetQueue. Must be called before or as part of Device.Release so
// the logical device can fully tear down and VRAM can be reclaimed.
func (q *Queue) Release() {
	if q == nil || q.released {
		return
	}
	q.released = true
	if q.r != nil {
		q.r.Release()
		q.r = nil
	}
}

// Present presents a surface texture (hal.Queue conformance).
// The Rust backend owns swapchain timing through Surface.Present; this
// delegates to the given surface and ignores damage rects (wgpu-native
// has no damage-aware present). Surface/texture are unpacked; wrong types
// or nils fail without touching GPU state.
func (q *Queue) Present(surface hal.Surface, texture hal.SurfaceTexture, _ []image.Rectangle) error {
	if err := prepareQueueCall(q); err != nil {
		return err
	}
	ws, ok1 := any(surface).(*Surface)
	wst, ok2 := texture.(*SurfaceTexture)
	if !ok1 || !ok2 || ws == nil || wst == nil {
		return fmt.Errorf("wgpu: Present: not a webgpu surface/texture (%T/%T)", surface, texture)
	}
	return ws.Present(wst)
}

// GetTimestampPeriod implements hal.Queue: wgpu-native does not expose it, returns 0.
func (q *Queue) GetTimestampPeriod() float32 { return 0 }

// SupportsCommandBufferCopies implements hal.Queue: Rust submits via command buffers.
func (q *Queue) SupportsCommandBufferCopies() bool { return true }

var _ hal.Queue = (*Queue)(nil)
