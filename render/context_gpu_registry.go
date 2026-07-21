package render

import "sync"

// gpuContextRegistry tracks every Context that holds a per-context GPU session.
// Device-lost abandon (Skia GrDirectContext::abandonContext / Flutter Rasterizer
// rebind) must invalidate ALL of them — not only the window Context the host
// remembers. Offscreen NewContext effect RTs (filter/layer/backdrop) register
// here automatically when ensureGPUCtx succeeds.
//
// Host code may still call DropGPURenderContext; it is idempotent.

var (
	gpuCtxRegMu sync.Mutex
	gpuCtxReg   = map[*Context]struct{}{}
)

func registerGPUContext(c *Context) {
	if c == nil {
		return
	}
	gpuCtxRegMu.Lock()
	gpuCtxReg[c] = struct{}{}
	gpuCtxRegMu.Unlock()
}

func unregisterGPUContext(c *Context) {
	if c == nil {
		return
	}
	gpuCtxRegMu.Lock()
	delete(gpuCtxReg, c)
	gpuCtxRegMu.Unlock()
}

// abandonAllContextGPU drops GPU state on every registered Context.
// Safe under device-lost force-release: DropGPURenderContext only closes
// sessions/filter publishes and nils gpuCtx.
func abandonAllContextGPU() {
	gpuCtxRegMu.Lock()
	list := make([]*Context, 0, len(gpuCtxReg))
	for c := range gpuCtxReg {
		list = append(list, c)
	}
	gpuCtxRegMu.Unlock()
	for _, c := range list {
		c.DropGPURenderContext()
	}
}

// GPUContextCount returns how many Contexts currently hold a GPU session (tests/diagnostics).
func GPUContextCount() int {
	gpuCtxRegMu.Lock()
	n := len(gpuCtxReg)
	gpuCtxRegMu.Unlock()
	return n
}
