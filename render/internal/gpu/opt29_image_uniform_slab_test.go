//go:build !nogpu

package gpu

import (
	"os"
	"testing"
)

func TestOpt29_ImageUniformSlab_OneWriteForManyOpacities(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}

	// Same texture gen, distinct opacities → cannot merge; pre-opt29 = N uniform WriteBuffers.
	px := []byte{255, 0, 0, 255}
	const n = 32
	cmds := make([]ImageDrawCommand, n)
	for i := 0; i < n; i++ {
		x := float32(i * 2)
		cmds[i] = ImageDrawCommand{
			PixelData: px, GenerationID: 99, ImgWidth: 1, ImgHeight: 1, ImgStride: 4,
			DstX: x, DstY: 0, DstW: 2, DstH: 2,
			TLX: x, TLY: 0, TRX: x + 2, TRY: 0, BRX: x + 2, BRY: 2, BLX: x, BLY: 2,
			U0: 0, V0: 0, U1: 1, V1: 1,
			Opacity:        0.1 + float32(i)*0.02,
			ViewportWidth:  200, ViewportHeight: 100,
		}
	}
	w0 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildImageResources(cmds, 200, 100, nil); err != nil {
		t.Fatal(err)
	}
	dw := s.lastSubmitStats.WriteBuffers - w0
	// Expect: 1 vertex WriteBuffer + 1 uniform slab WriteBuffer (+ maybe texture upload not counted as WriteBuffer).
	// Must be far fewer than n uniform writes (n+1 would be old worst case with verts).
	if dw > 4 {
		t.Fatalf("too many WriteBuffers for %d opacity-unique quads: %d (want small, slab coalesced)", n, dw)
	}
	if s.imageUniformSlab == nil || s.imageUniformSlots != n {
		t.Fatalf("slab slots=%d slab=%v", s.imageUniformSlots, s.imageUniformSlab != nil)
	}
	// Second identical build: sticky skip both verts + uniforms.
	w1 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildImageResources(cmds, 200, 100, nil); err != nil {
		t.Fatal(err)
	}
	if s.lastSubmitStats.WriteBuffers != w1 {
		t.Fatalf("identical rebuild should skip WriteBuffers, %d→%d", w1, s.lastSubmitStats.WriteBuffers)
	}
	// Opacity change → one slab rewrite (not n).
	cmds[0].Opacity = 0.99
	w2 := s.lastSubmitStats.WriteBuffers
	if _, err := s.buildImageResources(cmds, 200, 100, nil); err != nil {
		t.Fatal(err)
	}
	d := s.lastSubmitStats.WriteBuffers - w2
	if d < 1 || d > 3 {
		t.Fatalf("opacity change WriteBuffers delta=%d want 1..3", d)
	}
}

func TestOpt29_ImageUniformSlotStride_Aligned(t *testing.T) {
	if imageUniformSlotStride < imageUniformSize {
		t.Fatal("stride < payload")
	}
	if imageUniformSlotStride%256 != 0 {
		t.Fatalf("stride %d not multiple of default minUniformBufferOffsetAlignment 256", imageUniformSlotStride)
	}
}
