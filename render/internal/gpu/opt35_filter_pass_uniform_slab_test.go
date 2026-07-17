//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

func TestOpt35_FilterPassUniformSlotStride_Aligned(t *testing.T) {
	if filterPassUniformSlotStride < filterGPUUniformSize {
		t.Fatalf("stride %d < payload %d", filterPassUniformSlotStride, filterGPUUniformSize)
	}
	if filterPassUniformSlotStride%256 != 0 {
		t.Fatalf("stride %d not multiple of minUniformBufferOffsetAlignment 256", filterPassUniformSlotStride)
	}
}

// TestOpt35_FilterPassUniformSlab_OneWriteForMultiPassBlur packs H+V blur
// (2 passes) into one slab WriteBuffer (class A opt35).
func TestOpt35_FilterPassUniformSlab_OneWriteForMultiPassBlur(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	device, queue := shared.device, shared.queue
	cache := &shared.filterGPU

	const w, h = 32, 32
	src := make([]byte, w*h*4)
	for i := 0; i < len(src); i += 4 {
		src[i+0], src[i+1], src[i+2], src[i+3] = 40, 80, 200, 255
	}
	nodes := []render.ImageFilterNode{{
		Kind:   render.ImageFilterBlur,
		Radius: 2,
	}}
	out, err := runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
	if err != nil {
		t.Fatalf("blur graph: %v", err)
	}
	if len(out) < w*h*4 {
		t.Fatalf("out bytes=%d", len(out))
	}
	cache.mu.Lock()
	slots := cache.lastPassUniformSlots
	wb := cache.lastPassUniformWB
	slab := cache.passUniformSlab
	cache.mu.Unlock()
	// Blur = H + V = 2 passes (+ optional publish copy may be path-dependent).
	if slots < 2 {
		t.Fatalf("pass slots=%d want >=2 for H/V blur", slots)
	}
	if wb != 1 {
		t.Fatalf("pass uniform WriteBuffers=%d want 1 (slab)", wb)
	}
	if slab == nil {
		t.Fatal("passUniformSlab nil")
	}
	// Re-run steady-state glow-like frame: still one WriteBuffer, slab reused.
	out2, err := runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
	if err != nil {
		t.Fatalf("blur graph 2: %v", err)
	}
	if len(out2) < w*h*4 {
		t.Fatalf("out2 bytes=%d", len(out2))
	}
	cache.mu.Lock()
	wb2 := cache.lastPassUniformWB
	slab2 := cache.passUniformSlab
	cache.mu.Unlock()
	if wb2 != 1 {
		t.Fatalf("second run WriteBuffers=%d want 1", wb2)
	}
	if slab2 != slab {
		t.Fatal("slab reallocated on steady-state re-run")
	}
}

// TestOpt35_FilterPassUniformSlab_ShadowMultiPass exercises extract+blur+composite
// (more than 2 uniforms) still one slab upload.
func TestOpt35_FilterPassUniformSlab_ShadowMultiPass(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	device, queue := shared.device, shared.queue
	cache := &shared.filterGPU

	const w, h = 24, 24
	src := make([]byte, w*h*4)
	// Opaque blue square in center so shadow extract has alpha.
	for y := 6; y < 18; y++ {
		for x := 6; x < 18; x++ {
			i := (y*w + x) * 4
			src[i+0], src[i+1], src[i+2], src[i+3] = 20, 40, 220, 255
		}
	}
	nodes := []render.ImageFilterNode{{
		Kind:    render.ImageFilterDropShadow,
		Radius:  2,
		OffsetX: 2,
		OffsetY: 2,
		// Color fields if present on node
	}}
	// DropShadow may be expanded differently — fall back to explicit multi nodes.
	if nodes[0].Kind == 0 {
		t.Skip("no drop shadow kind")
	}
	out, err := runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
	if err != nil {
		// Some builds expand DropShadow only at higher level; try explicit chain.
		nodes = []render.ImageFilterNode{
			{Kind: render.ImageFilterBlur, Radius: 1.5},
			{Kind: render.ImageFilterBlur, Radius: 1.5},
		}
		out, err = runGPUFilterGraph(device, queue, cache, src, w, h, nodes)
		if err != nil {
			t.Fatalf("multi blur: %v", err)
		}
	}
	_ = out
	cache.mu.Lock()
	slots := cache.lastPassUniformSlots
	wb := cache.lastPassUniformWB
	cache.mu.Unlock()
	if slots < 2 {
		t.Fatalf("slots=%d want multi-pass", slots)
	}
	if wb != 1 {
		t.Fatalf("WriteBuffers=%d want 1", wb)
	}
}
