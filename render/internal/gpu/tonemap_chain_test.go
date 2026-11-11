//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/hdr"
)

// M5 P4：端到端链路（真实绘制 → 16F 面 → tonemap → 8bit）在内部层的单点证明。
// 真窗 ui_wr_m5_hdr 负责双后端与金图；本测保证链路语义与 hdr.Encode 对齐。
func TestP4HDRDrawTonemapChain(t *testing.T) {
	shared := NewGPUShared()
	shared.mu.Lock()
	err := shared.initGPU()
	shared.mu.Unlock()
	if err != nil {
		t.Skipf("GPU not available: %v", err)
	}
	defer shared.Close()
	rc := shared.NewRenderContext()
	defer rc.Close()
	device, queue := shared.Device(), shared.Queue()

	const w, h = 64, 64
	view, release, err := rc.CreateOffscreenTextureFormat(w, h, types.TextureFormatRGBA16Float)
	if err != nil {
		t.Fatalf("create 16F target: %v", err)
	}
	defer release()
	target := render.GPURenderTarget{View: view, ViewWidth: w, ViewHeight: h}

	paint := render.NewPaint()
	paint.SetBrush(render.Solid(render.RGBA{R: 2, G: 0.5, B: 0.25, A: 1}))
	shape := render.DetectedShape{
		Kind: render.ShapeCircle, CenterX: w / 2, CenterY: h / 2, RadiusX: 24, RadiusY: 24,
	}
	if err := rc.QueueShape(target, shape, paint, false); err != nil {
		t.Fatalf("queue shape: %v", err)
	}
	if err := rc.Flush(target); err != nil {
		t.Fatalf("flush into 16F target: %v", err)
	}

	const exposure = 1.0
	toned, releaseTone, err := rc.TonemapView(view, w, h, exposure)
	if err != nil {
		t.Fatalf("tonemap: %v", err)
	}
	defer releaseTone()
	pm, err := downloadTestPixmap(device, queue, unpackView(toned).Texture(), testFmtBGRA8, w, h)
	if err != nil {
		t.Fatalf("readback tonemapped: %v", err)
	}

	ref, err := hdr.NewFloatTarget(w, h)
	if err != nil {
		t.Fatalf("hdr target: %v", err)
	}
	if !ref.Set(w/2, h/2, [4]float32{2, 0.5, 0.25, 1}) {
		t.Fatal("hdr set")
	}
	want, err := ref.Encode(exposure)
	if err != nil {
		t.Fatalf("hdr encode: %v", err)
	}
	got := pm.Data()
	gi := ((h/2)*w + w/2) * 4
	ci := (h/2)*want.Stride + (w/2)*4
	maxDiff := 0
	for c := 0; c < 4; c++ {
		d := int(got[gi+c]) - int(want.Pix[ci+c])
		if d < 0 {
			d = -d
		}
		if d > maxDiff {
			maxDiff = d
		}
	}
	t.Logf("center gpu=%d/%d/%d cpu=%d/%d/%d max_diff=%d",
		got[gi], got[gi+1], got[gi+2], want.Pix[ci], want.Pix[ci+1], want.Pix[ci+2], maxDiff)
	if maxDiff > 1 {
		t.Fatalf("draw→tonemap chain max channel diff %d > 1", maxDiff)
	}
	// The >1 red must survive into the tonemapped result: 2.0 maps well above
	// the 1.0 clamp (188).
	if got[gi] <= 195 {
		t.Fatalf("red channel %d suggests the float target clamped the HDR value", got[gi])
	}
}
