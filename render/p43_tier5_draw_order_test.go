//go:build !nogpu

// P4.3 治根 · 根因复现（2026-11-10）：把 P3.7 的「盖序 = 提交序」契约
// （p37_draw_order_test.go，对齐 Godot renderer_canvas_render_rd.cpp）扩展到
// tier 5（GPUTexture）。P3.7 的八组只覆盖 tier 1（路径）与 tier 4（图像），
// 从未钉过 GPUTex；P4.1 批 3 在册洞 #5「层合成统一在 gpuTex tier 末尾重放、
// 盖住后画的直接绘制」缺的正是这层钉。
//
// 本测试先红 = 真实问题结果（复现 + 逐像素数据 + 定位到 ensureDrawOrder /
// buildScissorGroups），动手修之前必须拿到。
package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

const (
	tier5Canvas = 12
	tier5ProbeX = 6
	tier5ProbeY = 6
	tier5RectX  = 2
	tier5RectY  = 2
	tier5RectW  = 8
	tier5RectH  = 8
)

// tier5Plate 造一张纯色不透明离屏纹理。src 的生命周期交调用者持有（Close
// 不能早于最后一次采样），release 同理。
func tier5Plate(t *testing.T, rgba [4]uint8) (*render.Context, render.TextureView, func()) {
	t.Helper()
	src := render.NewContext(tier5Canvas, tier5Canvas)
	src.ClearWithColor(render.RGBA{
		R: float64(rgba[0]) / 255, G: float64(rgba[1]) / 255,
		B: float64(rgba[2]) / 255, A: 1,
	})
	// ClearWithColor 只写 CPU pixmap；再压一笔全画布 GPU 填充，保证
	// FlushGPUWithView 有真实 GPU 内容可 flush（客观输入：别把空纹理
	// 误判成「引擎丢绘制」）。
	src.SetRGB(float64(rgba[0])/255, float64(rgba[1])/255, float64(rgba[2])/255)
	src.DrawRectangle(0, 0, tier5Canvas, tier5Canvas)
	if err := src.Fill(); err != nil {
		src.Close()
		t.Fatalf("plate fill: %v", err)
	}
	view, release := src.CreateOffscreenTexture(tier5Canvas, tier5Canvas)
	if view.IsNil() || release == nil {
		src.Close()
		t.Skip("CreateOffscreenTexture unavailable")
	}
	if err := src.FlushGPUWithView(view, uint32(tier5Canvas), uint32(tier5Canvas)); err != nil {
		release()
		src.Close()
		t.Fatalf("FlushGPUWithView: %v", err)
	}
	// 客观校验输入：读回 plate 中心，确认不是空/透明纹理。
	type readbacker interface {
		ReadbackViewRGBA(view render.TextureView, ww, hh int) ([]byte, error)
	}
	if rb, ok := src.GPURenderContext().(readbacker); ok && rb != nil {
		if px, err := rb.ReadbackViewRGBA(view, tier5Canvas, tier5Canvas); err == nil && len(px) >= tier5Canvas*tier5Canvas*4 {
			o := (tier5ProbeY*tier5Canvas + tier5ProbeX) * 4
			t.Logf("plate center = %v (want %v)", px[o:o+4], rgba)
		} else {
			t.Logf("plate readback failed: err=%v len=%d", err, len(px))
		}
	} else {
		t.Log("plate readback interface unavailable")
	}
	return src, view, release
}

type tier5Step struct {
	label  string
	gpuTex bool
	rgba   [4]uint8
	view   render.TextureView
}

func TestP43Tier5DrawOrder(t *testing.T) {
	requireFilterOpsGPU(t)
	red := [4]uint8{255, 0, 0, 255}
	blue := [4]uint8{0, 0, 255, 255}
	green := [4]uint8{0, 255, 0, 255}
	yellow := [4]uint8{255, 255, 0, 255}

	srcB, blueView, relB := tier5Plate(t, blue)
	defer srcB.Close()
	defer relB()
	srcG, greenView, relG := tier5Plate(t, green)
	defer srcG.Close()
	defer relG()

	path := func(label string, c [4]uint8) tier5Step {
		return tier5Step{label: label, rgba: c}
	}
	tex := func(label string, c [4]uint8, v render.TextureView) tier5Step {
		return tier5Step{label: label, gpuTex: true, rgba: c, view: v}
	}

	groups := []struct {
		name  string
		steps []tier5Step
		want  [4]uint8
		why   string
	}{
		{"T0_单GPUTex蓝", []tier5Step{tex("蓝", blue, blueView)}, blue,
			"隔离复现：一张独立离屏纹理必须能呈现（否则后面全组都是假象）"},
		{"T1_路径红→GPUTex蓝", []tier5Step{path("红", red), tex("蓝", blue, blueView)}, blue,
			"后提交的高 tier 应盖住前面的路径"},
		{"T2_GPUTex蓝→路径红", []tier5Step{tex("蓝", blue, blueView), path("红", red)}, red,
			"后提交的低 tier 须触发封段才不被 gpuTex tier 末尾重放盖住（洞 #5）"},
		{"T3_路径红→GPUTex蓝→路径黄", []tier5Step{path("红", red), tex("蓝", blue, blueView), path("黄", yellow)}, yellow,
			"两段路径夹一张贴图，最后提交的黄必须赢"},
		{"T4_GPUTex蓝→GPUTex绿", []tier5Step{tex("蓝", blue, blueView), tex("绿", green, greenView)}, green,
			"同 tier 两张贴图不得换序"},
		{"T5_GPUTex蓝→路径红→GPUTex绿", []tier5Step{tex("蓝", blue, blueView), path("红", red), tex("绿", green, greenView)}, green,
			"5→1→5 两次跨 tier 往返"},
	}

	for _, g := range groups {
		t.Run(g.name, func(t *testing.T) {
			dc := render.NewContext(tier5Canvas, tier5Canvas)
			defer dc.Close()
			dc.ClearWithColor(render.White)
			for _, s := range g.steps {
				if s.gpuTex {
					dc.DrawGPUTexture(s.view, tier5RectX, tier5RectY, tier5RectW, tier5RectH)
					continue
				}
				dc.SetRGB(float64(s.rgba[0])/255, float64(s.rgba[1])/255, float64(s.rgba[2])/255)
				dc.DrawRectangle(tier5RectX, tier5RectY, tier5RectW, tier5RectH)
				_ = dc.Fill()
			}
			if err := dc.FlushGPU(); err != nil {
				t.Fatalf("FlushGPU: %v", err)
			}
			img := dc.Image()
			r, gg, b, a := img.At(tier5ProbeX, tier5ProbeY).RGBA()
			got := [4]uint8{uint8(r >> 8), uint8(gg >> 8), uint8(b >> 8), uint8(a >> 8)}
			t.Logf("probe=(%d,%d) got=%v want=%v gpu_ops=%d | %s",
				tier5ProbeX, tier5ProbeY, got, g.want, dc.RenderPathStats().GPUOps, g.why)
			if got != g.want {
				t.Errorf("盖序≠提交序：got %v want %v（%s）", got, g.want, g.why)
			}
		})
	}
}
