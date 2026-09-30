//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package noop

import (
	"fmt"
	"image"

	"github.com/energye/gpui/gpu/hal"
)

// Queue implements hal.Queue for the noop backend.
type Queue struct {
	submissionIndex uint64
}

// Submit simulates command buffer submission.
// Returns a monotonically increasing submission index.
func (q *Queue) Submit(_ ...hal.CommandBuffer) (uint64, error) {
	q.submissionIndex++
	return q.submissionIndex, nil
}

// Poll returns the highest submission index known to be completed.
// Noop backend is synchronous — all submissions are immediately complete.
// Backend divergence: webgpu Queue.Poll always returns 0,
// noop returns the submission index (submitted == completed).
func (q *Queue) Poll() uint64 {
	return q.submissionIndex
}

// LastSubmissionIndex returns the most recent submission index.
func (q *Queue) LastSubmissionIndex() uint64 {
	return q.submissionIndex
}

// WriteBuffer simulates immediate buffer writes.
// If the buffer has storage, copies data to it.
func (q *Queue) WriteBuffer(buffer hal.Buffer, offset uint64, data []byte) error {
	b, ok := buffer.(*Buffer)
	if !ok {
		return fmt.Errorf("noop: WriteBuffer: invalid buffer type %T", buffer)
	}
	if b.data != nil {
		copy(b.data[offset:], data)
	}
	return nil
}

// WriteTexture simulates immediate texture writes.
// This is a no-op since textures don't store data.
func (q *Queue) WriteTexture(_ *hal.ImageCopyTexture, _ []byte, _ *hal.ImageDataLayout, _ *hal.Extent3D) error {
	return nil
}

// Present simulates surface presentation.
// Always succeeds. damageRects is accepted and ignored (noop backend).
func (q *Queue) Present(_ hal.Surface, _ hal.SurfaceTexture, _ []image.Rectangle) error {
	return nil
}

// GetTimestampPeriod returns 1.0 nanosecond timestamp period.
func (q *Queue) GetTimestampPeriod() float32 {
	return 1.0
}

// SupportsCommandBufferCopies returns false for the noop backend.
// Writes are handled directly without command buffer batching.
func (q *Queue) SupportsCommandBufferCopies() bool {
	return false
}

// SetSwapchainSuppressed is a no-op on the noop backend.
func (q *Queue) SetSwapchainSuppressed(_ bool) {}
