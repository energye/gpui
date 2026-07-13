//go:build !linux && !darwin

package rwgpu

import "math"

func callRenderPassEncoderSetViewport(handle uintptr, x, y, width, height, minDepth, maxDepth float32) {
	procRenderPassEncoderSetViewport.Call( //nolint:errcheck
		handle,
		uintptr(math.Float32bits(x)),
		uintptr(math.Float32bits(y)),
		uintptr(math.Float32bits(width)),
		uintptr(math.Float32bits(height)),
		uintptr(math.Float32bits(minDepth)),
		uintptr(math.Float32bits(maxDepth)),
	)
}
