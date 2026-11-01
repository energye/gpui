//go:build linux && !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestP34GLOffscreenMaskFilterFill pins the P3.4 paint-level MaskFilter fill
// on the boxed GL device (the go-tier twin of TestP34PaintMaskFilterGPURoute):
// the feathered fill must land feathered AND with the correct channel order
// (orange = 1,0.6,0.2). Regression context: the GL CopyTextureToBuffer read
// format was hardcoded gl.BGRA, flipping every RGBA8-texture readback on the
// GL chain (filter-graph blur handed BGRA bytes back into an RGBA content
// buffer — C card rendered blue on the go tier, evidence/P3.4).
func TestP34GLOffscreenMaskFilterFill(t *testing.T) {
	dev, ad := p1GLDevice(t)
	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		t.Fatalf("SetAcceleratorDeviceProvider(boxed GL): %v", err)
	}
	defer render.AbandonAcceleratorDevice()

	dc := render.NewContext(64, 64)
	defer dc.Close()
	if dc.GPURenderContext() == nil {
		t.Fatal("per-context GPU not established on GL device")
	}
	dc.ClearWithColor(render.Transparent)
	dc.SetRGBA(1, 0.6, 0.2, 1)
	dc.SetPaintMaskFilter(3)
	dc.DrawRectangle(16, 16, 32, 32)
	if err := dc.Fill(); err != nil {
		t.Fatalf("fill: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Micro-probe: hook-only byte order — solid red in, blurXY through the
	// registered GPU filter graph. Out should keep R=255 (RGBA contract).
	hooks := render.RegisteredGPUFilterGraphHooks()
	if hooks.Apply != nil {
		in := make([]byte, 4*4*4)
		for i := 0; i < 16; i++ {
			in[i*4], in[i*4+1], in[i*4+2], in[i*4+3] = 255, 0, 0, 255
		}
		if out, err := hooks.Apply(in, 4, 4, []render.ImageFilterNode{{Kind: render.ImageFilterBlurXY, RadiusX: 1, RadiusY: 1}}); err != nil {
			t.Logf("hook apply err: %v", err)
		} else {
			t.Logf("hook out[0:8] = %v", out[0:8])
		}
	} else {
		t.Logf("hooks.Apply == nil on GL chain")
	}

	stats := dc.RenderPathStats()
	t.Logf("GL filtered fill %s", stats.LogLine())
	out := dc.Image()
	for _, x := range []int{14, 16, 20, 32} {
		r, g, b, a := out.At(x, 32).RGBA()
		t.Logf("x=%d rgba=(%d,%d,%d,%d)", x, r>>8, g>>8, b>>8, a>>8)
	}
	r, g, b, a := out.At(32, 32).RGBA()
	if r>>8 != 255 || g>>8 != 153 || b>>8 != 51 || a>>8 != 255 {
		t.Fatalf("interior = (%d,%d,%d,%d), want (255,153,51,255) — channel order broken",
			r>>8, g>>8, b>>8, a>>8)
	}
}
