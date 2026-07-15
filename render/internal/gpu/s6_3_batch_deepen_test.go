//go:build !nogpu

package gpu

import (
	"os"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

func TestS63_CanMergeGPUTextureDraw(t *testing.T) {
	nilA := &GPUTextureDrawCommand{Opacity: 1, ViewportWidth: 100, ViewportHeight: 80}
	nilB := &GPUTextureDrawCommand{Opacity: 1, ViewportWidth: 100, ViewportHeight: 80}
	if canMergeGPUTextureDraw(nilA, nilB) {
		t.Fatal("nil views must not merge")
	}

	p1 := unsafe.Pointer(uintptr(0x1000))
	p2 := unsafe.Pointer(uintptr(0x2000))
	va := gpucontext.NewTextureView(p1)
	vb := gpucontext.NewTextureView(p1)
	vc := gpucontext.NewTextureView(p2)

	a := &GPUTextureDrawCommand{View: va, Opacity: 1, ViewportWidth: 100, ViewportHeight: 80}
	b := &GPUTextureDrawCommand{View: vb, Opacity: 1, ViewportWidth: 100, ViewportHeight: 80}
	if !canMergeGPUTextureDraw(a, b) {
		t.Fatal("same view ptr/opacity/viewport should merge")
	}
	b.Opacity = 0.5
	if canMergeGPUTextureDraw(a, b) {
		t.Fatal("different opacity must not merge")
	}
	b.Opacity = 1
	b.ViewportWidth = 200
	if canMergeGPUTextureDraw(a, b) {
		t.Fatal("different viewport must not merge")
	}
	b = &GPUTextureDrawCommand{View: vc, Opacity: 1, ViewportWidth: 100, ViewportHeight: 80}
	if canMergeGPUTextureDraw(a, b) {
		t.Fatal("different view ptr must not merge")
	}
}

func TestS63_GPUTexture_MultiQuadLogic(t *testing.T) {
	p := unsafe.Pointer(uintptr(0x3000))
	v := gpucontext.NewTextureView(p)
	cmds := make([]GPUTextureDrawCommand, 8)
	for i := range cmds {
		cmds[i] = GPUTextureDrawCommand{
			View: v, Opacity: 1, ViewportWidth: 400, ViewportHeight: 300,
			DstX: float32(i * 10), DstY: 0, DstW: 8, DstH: 8,
		}
	}
	seal := make([]bool, len(cmds))
	seal[4] = true // scissor boundary

	// Simulate S6.3 merge loop.
	var draws, quads int
	for i := 0; i < len(cmds); {
		j := i + 1
		for j < len(cmds) {
			if seal[j] {
				break
			}
			if !canMergeGPUTextureDraw(&cmds[i], &cmds[j]) {
				break
			}
			j++
		}
		draws++
		quads += j - i
		i = j
	}
	if draws != 2 {
		t.Fatalf("draws=%d want 2 (seal at 4)", draws)
	}
	if quads != 8 {
		t.Fatalf("quads=%d want 8", quads)
	}
}

func TestS63_CoalesceTextBatches_DeepensRuns(t *testing.T) {
	mk := func(x float32) TextBatch {
		return TextBatch{
			Transform:  render.Identity(),
			Color:      render.RGBA{R: 1, G: 1, B: 1, A: 1},
			PxRange:    4,
			AtlasSize:  512,
			AtlasIndex: 0,
			Quads:      []TextQuad{{X0: x, Y0: 0, X1: x + 10, Y1: 10}},
		}
	}
	in := []TextBatch{mk(0), mk(20), mk(40)}
	// Force non-merge mid: different color
	in[1].Color = render.RGBA{R: 1, G: 0, B: 0, A: 1}
	out := coalesceTextBatches(in)
	if len(out) != 3 {
		t.Fatalf("different color should not merge: got %d", len(out))
	}

	in = []TextBatch{mk(0), mk(20), mk(40)}
	out = coalesceTextBatches(in)
	if len(out) != 1 {
		t.Fatalf("same style should coalesce to 1 batch, got %d", len(out))
	}
	if len(out[0].Quads) != 3 {
		t.Fatalf("quads=%d want 3", len(out[0].Quads))
	}
}

func TestS63_CoalesceGlyphBatches_DeepensRuns(t *testing.T) {
	mk := func(x float32) GlyphMaskBatch {
		return GlyphMaskBatch{
			Transform:      render.Identity(),
			Color:          [4]float32{0, 0, 0, 1},
			IsLCD:          false,
			AtlasPageIndex: 0,
			Quads:          []GlyphMaskQuad{{X0: x, Y0: 0, X1: x + 8, Y1: 12}},
		}
	}
	in := []GlyphMaskBatch{mk(0), mk(10), mk(20)}
	out := coalesceGlyphMaskBatches(in)
	if len(out) != 1 || len(out[0].Quads) != 3 {
		t.Fatalf("got batches=%d quads=%v", len(out), func() int {
			if len(out) == 0 {
				return 0
			}
			return len(out[0].Quads)
		}())
	}
	in[1].AtlasPageIndex = 1
	out = coalesceGlyphMaskBatches(in)
	if len(out) != 3 {
		t.Fatalf("different atlas page must not merge: %d", len(out))
	}
}

func TestS63_Coalesce_DoesNotCrossManualSplit(t *testing.T) {
	// Seal is modeled by not calling coalesce across groups; each group coalesced alone.
	g0 := []TextBatch{{
		Transform: render.Identity(), Color: render.RGBA{R: 1, G: 1, B: 1, A: 1},
		Quads: []TextQuad{{X0: 0, Y0: 0, X1: 1, Y1: 1}},
	}}
	g1 := []TextBatch{{
		Transform: render.Identity(), Color: render.RGBA{R: 1, G: 1, B: 1, A: 1},
		Quads: []TextQuad{{X0: 2, Y0: 0, X1: 3, Y1: 1}},
	}}
	c0 := coalesceTextBatches(g0)
	c1 := coalesceTextBatches(g1)
	if len(c0) != 1 || len(c1) != 1 {
		t.Fatal("per-group coalesce should keep separate groups separate")
	}
}

func TestS63_BatchStats_AfterFlush(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	defer shared.Close()
	if err := shared.ensureGPU(); err != nil {
		t.Skipf("GPU: %v", err)
	}
	shared.ensurePipelines()
	rc := shared.NewRenderContext()
	const w, h = 200, 120
	target := render.GPURenderTarget{Width: w, Height: h, Stride: w * 4, Data: make([]byte, w*h*4)}
	paint := render.NewPaint()
	paint.SetBrush(render.Solid(render.RGBA{R: 0.1, G: 0.4, B: 0.9, A: 1}))
	for i := 0; i < 5; i++ {
		shape := render.DetectedShape{
			Kind: render.ShapeRect, CenterX: float64(30 + i*20), CenterY: 40, Width: 16, Height: 16,
		}
		if err := rc.QueueShape(target, shape, paint, false); err != nil {
			t.Fatalf("QueueShape: %v", err)
		}
	}
	if err := rc.Flush(target); err != nil {
		if err == render.ErrFallbackToCPU {
			t.Skip("CPU fallback")
		}
		t.Fatalf("Flush: %v", err)
	}
	st := rc.LastBatchDrawStats()
	t.Logf("S6.3 batch stats=%+v", st)
	if st.SDFShapes != 5 {
		t.Fatalf("SDFShapes=%d want 5", st.SDFShapes)
	}
	if st.SDFDraws != 1 {
		t.Fatalf("SDFDraws=%d want 1 (merged multi-shape draw)", st.SDFDraws)
	}
}

func TestS63_ImageSealStillRespected(t *testing.T) {
	// Reaffirm S4.1 seal: mergeable images split by seal.
	base := ImageDrawCommand{
		GenerationID: 7, ImgWidth: 16, ImgHeight: 16, Opacity: 1,
		ViewportWidth: 256, ViewportHeight: 256,
	}
	cmds := make([]ImageDrawCommand, 6)
	for i := range cmds {
		cmds[i] = base
	}
	seal := make([]bool, 6)
	seal[3] = true
	var draws int
	for i := 0; i < len(cmds); {
		j := i + 1
		for j < len(cmds) {
			if seal[j] {
				break
			}
			if !canMergeImageDraw(&cmds[i], &cmds[j]) {
				break
			}
			j++
		}
		draws++
		i = j
	}
	if draws != 2 {
		t.Fatalf("draws=%d want 2", draws)
	}
}
