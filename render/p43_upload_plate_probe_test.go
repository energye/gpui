//go:build !nogpu

// P4.3 治根 · 二分定位（2026-11-10）：tier5 盖序已证绿（#5 不是根因），
// 现在对比两种离屏纹理填充方式的「贴回可呈现性」：
//
//	A. FlushGPUWithView（GPU 渲染 pass 填）  —— 已知可贴回（tier5 测试 6/6 绿）
//	B. UploadRGBAToView（CPU 数据 WriteTexture 直传，不经 render pass）
//	   —— P4.3 ②③ 缓存用的正是 B，且真窗里贴图不呈现
//
// 若 B 读回有内容、贴回却无 → 根因在「WriteTexture 直传的 view 未进入可采样
// 状态 / session registry 解析」，是通用引擎路径问题，不是 DrawShadow 特例。
package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

const upProbeW = 12

type upUploader interface {
	UploadRGBAToView(view render.TextureView, data []byte, w, h int) error
}
type upReadbacker interface {
	ReadbackViewRGBA(view render.TextureView, ww, hh int) ([]byte, error)
}

// upMakeUploadedPlate 用 UploadRGBAToView 直传纯蓝，返回 src（生命周期交调用者）。
func upMakeUploadedPlate(t *testing.T) (*render.Context, render.TextureView, func()) {
	t.Helper()
	src := render.NewContext(upProbeW, upProbeW)
	view, release := src.CreateOffscreenTexture(upProbeW, upProbeW)
	if view.IsNil() || release == nil {
		src.Close()
		t.Skip("CreateOffscreenTexture unavailable")
	}
	data := make([]byte, upProbeW*upProbeW*4)
	for i := 0; i < upProbeW*upProbeW; i++ {
		data[i*4+0] = 0
		data[i*4+1] = 0
		data[i*4+2] = 255
		data[i*4+3] = 255
	}
	u, ok := src.GPURenderContext().(upUploader)
	if !ok || u == nil {
		release()
		src.Close()
		t.Skip("UploadRGBAToView unavailable")
	}
	if err := u.UploadRGBAToView(view, data, upProbeW, upProbeW); err != nil {
		release()
		src.Close()
		t.Fatalf("UploadRGBAToView: %v", err)
	}
	return src, view, release
}

func upReadView(t *testing.T, dc *render.Context, view render.TextureView) [4]uint8 {
	t.Helper()
	rb, ok := dc.GPURenderContext().(upReadbacker)
	if !ok || rb == nil {
		t.Skip("ReadbackViewRGBA unavailable")
	}
	px, err := rb.ReadbackViewRGBA(view, upProbeW, upProbeW)
	if err != nil || len(px) < upProbeW*upProbeW*4 {
		t.Fatalf("ReadbackViewRGBA: err=%v len=%d", err, len(px))
	}
	o := (6*upProbeW + 6) * 4
	return [4]uint8{px[o], px[o+1], px[o+2], px[o+3]}
}

func upCompositeAndRead(t *testing.T, view render.TextureView) [4]uint8 {
	t.Helper()
	dst := render.NewContext(upProbeW, upProbeW)
	defer dst.Close()
	dst.ClearWithColor(render.White)
	dst.DrawGPUTexture(view, 0, 0, upProbeW, upProbeW)
	if err := dst.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	img := dst.Image()
	r, g, b, a := img.At(6, 6).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

func TestP43UploadPlateProbe(t *testing.T) {
	requireFilterOpsGPU(t)
	blue := [4]uint8{0, 0, 255, 255}

	// A 对照：GPU 渲染 pass 填充（FlushGPUWithView）
	srcA := render.NewContext(upProbeW, upProbeW)
	defer srcA.Close()
	srcA.ClearWithColor(render.RGBA{B: 1, A: 1})
	srcA.SetRGB(0, 0, 1)
	srcA.DrawRectangle(0, 0, upProbeW, upProbeW)
	_ = srcA.Fill()
	viewA, relA := srcA.CreateOffscreenTexture(upProbeW, upProbeW)
	if viewA.IsNil() || relA == nil {
		t.Skip("CreateOffscreenTexture unavailable")
	}
	defer relA()
	if err := srcA.FlushGPUWithView(viewA, uint32(upProbeW), uint32(upProbeW)); err != nil {
		t.Fatalf("A FlushGPUWithView: %v", err)
	}
	gotA := upCompositeAndRead(t, viewA)
	t.Logf("A FlushGPUWithView(填) : view=%v composite=%v want=%v", upReadView(t, srcA, viewA), gotA, blue)

	// B 被测：UploadRGBAToView 直传（P4.3 ②③ 的方式）
	srcB, viewB, relB := upMakeUploadedPlate(t)
	defer srcB.Close()
	defer relB()
	gotB := upCompositeAndRead(t, viewB)
	readB := upReadView(t, srcB, viewB)
	t.Logf("B UploadRGBAToView(填): view=%v composite=%v want=%v", readB, gotB, blue)

	if gotA != blue {
		t.Errorf("A 对照组失败：GPU 渲染填充的纹理贴回得到 %v，want %v", gotA, blue)
	}
	if gotB != blue {
		t.Errorf("B 定位结论：UploadRGBAToView 直传的纹理贴回得到 %v（读回 %v），want %v —— 根因在此路径",
			gotB, readB, blue)
	}
}
