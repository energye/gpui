package render

import "testing"

func TestGPUContextRegistry_AbandonClears(t *testing.T) {
	// Without GPU accelerator, ensureGPUCtx is a no-op — still verify abandon is safe.
	c1 := NewContext(16, 16)
	c2 := NewContext(16, 16)
	defer c1.Close()
	defer c2.Close()
	// Force register as if GPU were attached (abandon path must tolerate).
	registerGPUContext(c1)
	registerGPUContext(c2)
	if GPUContextCount() < 2 {
		t.Fatalf("count=%d", GPUContextCount())
	}
	abandonAllContextGPU()
	if GPUContextCount() != 0 {
		t.Fatalf("after abandon count=%d want 0", GPUContextCount())
	}
}
