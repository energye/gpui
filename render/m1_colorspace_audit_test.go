//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// M1 撤销标定钉（2026-11-03）：曾登记「fillMaskedAsImage 把 premul 栅格喂进
// straight 合成链 → GPU/CPU 分叉、正确应 (128,255,255)」。实测收回该结论——
// 那条推演把 Pixmap 的 premul 存储约定错误地套到了 fill 源数据上；引擎的
// image-合成链（QueueImageDraw/DrawImage/遮罩口/层 pop）端到端按 straight 解释
// pixelData，GPU 与 CPU **逐位一致**。本钉把四链等值锁死：色空间口径若再分叉，
// 这里先红。coverage 取 100/255 是刻意的——128≈√0.5 会让 straight/premul 两式
// 简并成同值，失去判别力（M1 误判的路径依赖之一）。

func m1px(dc *render.Context, x, y int) [4]int {
	r, g, b, a := dc.Image().At(x, y).RGBA()
	return [4]int{int(r >> 8), int(g >> 8), int(b >> 8), int(a >> 8)}
}

func m1QuadEqual(t *testing.T, name string, setup func(dc *render.Context), x, y, tol int) {
	t.Helper()
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator")
	}
	var gpuPix, cpuPix [4]int
	for _, probe := range []struct {
		mode string
		out  *[4]int
	}{{"gpu", &gpuPix}, {"cpu", &cpuPix}} {
		t.Setenv("GOGPU_RENDER_MODE", probe.mode)
		dc := render.NewContext(32, 32)
		dc.ClearWithColor(render.White)
		setup(dc)
		if err := dc.Fill(); err != nil {
			t.Fatalf("%s %s fill: %v", name, probe.mode, err)
		}
		if probe.mode == "gpu" {
			if err := dc.FlushGPU(); err != nil {
				t.Fatalf("flush: %v", err)
			}
		}
		*probe.out = m1px(dc, x, y)
		_ = dc.Close()
	}
	for i := range gpuPix {
		d := gpuPix[i] - cpuPix[i]
		if d < 0 {
			d = -d
		}
		if d > tol {
			t.Errorf("%s: GPU=%v CPU=%v 通道差 %d > %d（色空间/链路分叉）", name, gpuPix, cpuPix, d, tol)
			return
		}
	}
	t.Logf("%s: GPU=%v CPU=%v 等值（tol %d）", name, gpuPix, cpuPix, tol)
}

func TestM1ColorSpaceChainEquivalence(t *testing.T) {
	// 纯色半透明（分析填充路）
	m1QuadEqual(t, "solid", func(dc *render.Context) {
		dc.SetRGBA(1, 0, 0, 0.4)
		dc.DrawRectangle(4, 4, 24, 24)
	}, 16, 16, 0)
	// 渐变刷（brush→mask 链）
	m1QuadEqual(t, "gradient", func(dc *render.Context) {
		brush := render.NewLinearGradientBrush(0, 0, 32, 0).
			AddColorStop(0, render.RGBA{R: 1, A: 0.4}).
			AddColorStop(1, render.RGBA{G: 1, A: 0.4})
		dc.SetFillBrush(brush)
		dc.DrawRectangle(4, 4, 24, 24)
	}, 16, 16, 1)
	// 非对称 coverage 遮罩（fillMaskedAsImage——M1 点名的链）
	m1QuadEqual(t, "mask-cov100", func(dc *render.Context) {
		data := make([]byte, 32*32)
		for i := range data {
			data[i] = 100
		}
		dc.SetMask(render.NewMaskFromData(data, 32, 32))
		dc.SetRGBA(1, 0, 0, 0.4)
		dc.DrawRectangle(4, 4, 24, 24)
	}, 16, 16, 0)
	// 层离屏合成（resolve 腿）
	t.Run("layer", func(t *testing.T) {
		if render.Accelerator() == nil {
			t.Skip("no GPU accelerator")
		}
		var gpuPix, cpuPix [4]int
		for _, probe := range []struct {
			mode string
			out  *[4]int
		}{{"gpu", &gpuPix}, {"cpu", &cpuPix}} {
			t.Setenv("GOGPU_RENDER_MODE", probe.mode)
			dc := render.NewContext(32, 32)
			dc.ClearWithColor(render.White)
			dc.PushLayerIsolated(1)
			dc.SetRGBA(1, 0, 0, 0.4)
			dc.DrawRectangle(4, 4, 24, 24)
			if err := dc.Fill(); err != nil {
				t.Fatal(err)
			}
			dc.PopLayer()
			if probe.mode == "gpu" {
				if err := dc.FlushGPU(); err != nil {
					t.Fatal(err)
				}
			}
			*probe.out = m1px(dc, 16, 16)
			_ = dc.Close()
		}
		if gpuPix != cpuPix {
			t.Errorf("layer: GPU=%v CPU=%v 层合成色空间分叉", gpuPix, cpuPix)
		} else {
			t.Logf("layer: %v 等值", gpuPix)
		}
	})
}
