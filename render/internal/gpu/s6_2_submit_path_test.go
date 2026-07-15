//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

func TestS62_ClipParams_BytesInto_NoAllocHot(t *testing.T) {
	p := &ClipParams{RectX1: 1, RectY1: 2, RectX2: 3, RectY2: 4, Radius: 5, Enabled: 1}
	buf := make([]byte, clipParamsSize)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = p.BytesInto(buf)
	})
	if allocs != 0 {
		t.Fatalf("BytesInto allocs/op = %v want 0", allocs)
	}
	out := p.BytesInto(buf)
	if len(out) != clipParamsSize {
		t.Fatalf("len=%d", len(out))
	}
	// Sanity: Enabled bit present.
	if out[20] == 0 && out[21] == 0 && out[22] == 0 && out[23] == 0 {
		t.Fatal("enabled float bits zero")
	}
}

func TestS62_SDFUniformInto_NoAllocHot(t *testing.T) {
	buf := make([]byte, sdfRenderUniformSize)
	allocs := testing.AllocsPerRun(1000, func() {
		_ = makeSDFRenderUniformInto(buf, 800, 600, true)
	})
	if allocs != 0 {
		t.Fatalf("makeSDFRenderUniformInto allocs/op = %v want 0", allocs)
	}
}

func TestS62_RenderSession_StatsAfterGrouped(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	defer shared.Close()
	if err := shared.ensureGPU(); err != nil {
		t.Skipf("GPU unavailable: %v", err)
	}
	shared.ensurePipelines()

	rc := shared.NewRenderContext()
	const w, h = 200, 120
	target := render.GPURenderTarget{Width: w, Height: h, Stride: w * 4, Data: make([]byte, w*h*4)}
	shape := render.DetectedShape{Kind: render.ShapeRect, CenterX: 40, CenterY: 30, Width: 40, Height: 30}
	paint := render.NewPaint()
	paint.SetBrush(render.Solid(render.RGBA{R: 0.2, G: 0.5, B: 0.9, A: 1}))
	if err := rc.QueueShape(target, shape, paint, false); err != nil {
		t.Fatalf("QueueShape: %v", err)
	}
	if err := rc.Flush(target); err != nil {
		// Offscreen may still succeed; hard fail only on unexpected fallback-less errors.
		if err == render.ErrFallbackToCPU {
			t.Skip("CPU fallback")
		}
		t.Fatalf("Flush: %v", err)
	}
	st := rc.LastSubmitPathStats()
	t.Logf("S6.2 stats=%+v", st)
	if st.Groups < 1 {
		t.Fatalf("groups=%d want >=1", st.Groups)
	}
	if !st.SingleGroupFast {
		t.Fatal("expected SingleGroupFast for one scissor group")
	}
	if st.WriteBuffers < 1 && st.EncodersCreated < 1 {
		t.Fatalf("expected write/encoder activity: %+v", st)
	}

	// Second flush on same context (retained) must keep SingleGroupFast.
	if err := rc.QueueShape(target, shape, paint, false); err != nil {
		t.Fatalf("QueueShape2: %v", err)
	}
	if err := rc.Flush(target); err != nil {
		t.Fatalf("Flush2: %v", err)
	}
	st2 := rc.LastSubmitPathStats()
	t.Logf("S6.2 stats2=%+v", st2)
	if !st2.SingleGroupFast || st2.WriteBuffers < 1 {
		t.Fatalf("retained second flush stats=%+v", st2)
	}
}
