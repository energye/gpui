//go:build !(js && wasm)

package webgpu

import (
	"fmt"

	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Queue handles command submission and data transfers.
// On the wgpu-native backend, this wraps rwgpu Queue.
type Queue struct {
	r        *rwgpu.Queue
	device   *Device // parent device for lost gate (nil-safe)
	released bool
}

// Submit submits command buffers for execution.
// Returns a submission index that can be used to track completion.
//
// R7.0: avoid per-submit heap allocation on the dominant 1-CB path and for
// small multi-CB submits (≤8). Semantics unchanged: non-nil CBs are marked
// submitted; only CBs with a live native handle are passed to rwgpu.
func (q *Queue) Submit(commandBuffers ...*CommandBuffer) (uint64, error) {
	if err := prepareQueueCall(q); err != nil {
		return 0, err
	}
	n := len(commandBuffers)
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
		cb := commandBuffers[0]
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
	for _, cb := range commandBuffers {
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
func (q *Queue) WriteBuffer(buffer *Buffer, offset uint64, data []byte) error {
	if err := prepareQueueCall(q); err != nil {
		return err
	}
	if buffer == nil || buffer.r == nil {
		return fmt.Errorf("wgpu: WriteBuffer: buffer is nil")
	}
	if err := q.r.WriteBuffer(buffer.r, offset, data); err != nil {
		if e := mapRWGPUErr(err); e != err {
			return e
		}
		return err
	}
	return nil
}

// WriteTexture writes data to a texture.
// R7.0: stack-allocate destination/layout/size descriptors (no per-call heap).
func (q *Queue) WriteTexture(dst *ImageCopyTexture, data []byte, layout *ImageDataLayout, size *Extent3D) error {
	if err := prepareQueueCall(q); err != nil {
		return err
	}
	if dst == nil || dst.Texture == nil || dst.Texture.r == nil {
		return fmt.Errorf("wgpu: WriteTexture: destination is nil")
	}

	rDst := rwgpu.ImageCopyTexture{
		Texture:  dst.Texture.r,
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
