//go:build linux || darwin

package rwgpu

import (
	"sync"

	"github.com/ebitengine/purego"
)

var (
	setViewportOnce sync.Once
	setViewportFn   func(uintptr, float32, float32, float32, float32, float32, float32)
)

func callRenderPassEncoderSetViewport(handle uintptr, x, y, width, height, minDepth, maxDepth float32) {
	proc, ok := procRenderPassEncoderSetViewport.(*unixProc)
	if !ok || proc.fnPtr == 0 {
		return
	}

	setViewportOnce.Do(func() {
		purego.RegisterFunc(&setViewportFn, proc.fnPtr)
	})
	setViewportFn(handle, x, y, width, height, minDepth, maxDepth)
}
