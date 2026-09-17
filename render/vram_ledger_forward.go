package render

import (
	"github.com/energye/gpui/gpu/webgpu"
)

func vramTestReset() {
	webgpu.VramTestReset()
}

func vramTestAdd(handle uintptr, need uint64) {
	webgpu.VramTestAdd(handle, need)
}
