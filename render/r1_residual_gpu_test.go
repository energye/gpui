//go:build !nogpu

package render_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func TestR1_PushMaskLayerGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 48, 48
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	mask := render.NewMask(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				mask.Set(x, y, 255)
			} else {
				mask.Set(x, y, 0)
			}
		}
	}
	dc.PushMaskLayer(mask)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 1)
	dc.DrawCircle(24, 24, 14)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R1 mask %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("expected GPUOps>0 for mask-layer content")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	lr, lg, lb, _ := sampleRGBA(dc, 10, 24)
	rr, rg, rb, _ := sampleRGBA(dc, 40, 24)
	t.Logf("left=%d,%d,%d right=%d,%d,%d", lr, lg, lb, rr, rg, rb)
	if lr > 240 && lg > 240 && lb > 240 {
		t.Fatalf("left should show masked content")
	}
	if rr < 240 || rg < 240 || rb < 240 {
		t.Fatalf("right should stay white, got %d,%d,%d", rr, rg, rb)
	}
}

func TestR1_BackdropSeedGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.15, 0.35, 0.9)
	dc.DrawRectangle(8, 8, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("parent flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// Backdrop + partial dim (not full cover) so snapshot seed is required.
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(16, 16, 32, 32)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("backdrop flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R1 backdrop %s base=%d", st.LogLine(), base)
	if st.GPUOps <= base {
		t.Fatalf("expected additional GPUOps after backdrop draw")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	// Center of dimmed region should keep some blue from seeded backdrop.
	r, g, b, _ := sampleRGBA(dc, 32, 32)
	t.Logf("center=%d,%d,%d", r, g, b)
	if b < 40 {
		t.Fatalf("lost backdrop blue: %d,%d,%d", r, g, b)
	}
	// Outside dim rect but inside blue parent should stay bright blue-ish.
	or, og, ob, _ := sampleRGBA(dc, 12, 12)
	t.Logf("outer blue=%d,%d,%d", or, og, ob)
	if ob < 100 {
		t.Fatalf("outer blue destroyed: %d,%d,%d", or, og, ob)
	}
}

// TestR1_ImageMipmapUsesGPU 是 P3.6 批 2 把「反向钉」改正向钉的那一条。
//
// 旧版本只断言「这口被忽略也算走 GPU」，等于把假语义当契约。现在断言档位真
// 驱动多级采样：8× 缩小时 `MipmapNone` 保留双线性抽样（读到的是个别噪声纹素，
// 离中性远），`MipmapLinear` 必须先预滤波（贴近中性灰），两档结果必须不同。
//
// 像素一律走 ExportImageBuf 从显存读回——headless Context 的 Image() 返回的是
// CPU 面，用它判 GPU 行为本身就是错的（我第一次就这么错过，两档「相同」是
// 读取路径的假象，不是 GPU 的）。
func TestR1_ImageMipmapUsesGPU(t *testing.T) {
	requireNativeGPU(t)
	const side, dst = 64, 8

	noise := r1NoiseImage(t, side)

	gpuMeanDev := func(mode render.MipmapMode) float64 {
		dc := render.NewContext(side, side)
		defer dc.Close()
		dc.SetEffectSurface(true)
		dc.ResetRenderPathStats()
		dc.ClearWithColor(render.White)
		dc.DrawImageEx(noise, render.DrawImageOptions{
			X: 24, Y: 24, DstWidth: dst, DstHeight: dst,
			Interpolation: render.InterpBilinear,
			Mipmap:        mode,
			Opacity:       1,
			BlendMode:     render.BlendNormal,
		})
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU(mode=%v): %v", mode, err)
		}
		st := dc.RenderPathStats()
		if st.GPUOps == 0 {
			t.Fatalf("mode=%v 未走 GPU（last=%s）", mode, st.LastCPUFallbackReason)
		}
		if st.CPUFallbackOps != 0 {
			t.Fatalf("mode=%v 掉回 CPU：cpu_fb=%d last=%s（mip 档不得强制 CPU）", mode, st.CPUFallbackOps, st.LastCPUFallbackReason)
		}
		var img *render.ImageBuf
		if !dc.ExportImageBuf(&img) || img == nil {
			t.Fatalf("mode=%v 显存读回失败", mode)
		}
		defer img.Dispose()
		data := img.Data()
		stride := img.Stride()
		var sum float64
		for y := 24; y < 24+dst; y++ {
			for x := 24; x < 24+dst; x++ {
				off := y*stride + x*4
				sum += math.Abs(float64((int(data[off])+int(data[off+1])+int(data[off+2]))/3) - 127.5)
			}
		}
		return sum / float64(dst*dst)
	}

	noneDev := gpuMeanDev(render.MipmapNone)
	linDev := gpuMeanDev(render.MipmapLinear)
	t.Logf("显存读回离中性均值：None=%.1f Linear=%.1f", noneDev, linDev)
	if noneDev == linDev {
		t.Fatalf("两档像素完全相同：Mipmap 档位没有驱动多级采样")
	}
	if linDev >= noneDev {
		t.Fatalf("MipmapLinear(%.1f) 未比 MipmapNone(%.1f) 更贴近预滤波均值", linDev, noneDev)
	}
}

// TestR1_MipLevelBlendSelect 钉三档互相可辨：缩小到 2.67× 时 LOD 落在两级之间，
// `MipmapNearest` 只取一级、`MipmapLinear` 混两级，两者像素必不同；`MipmapNone`
// 又与二者都不同（只吃 level0）。少了这条，"采样器的级间选择"就是没人守的开关
// ——负向对照 N3（摘掉按档绑采样器）当时照样全绿，就是这么暴露出来的。
func TestR1_MipLevelBlendSelect(t *testing.T) {
	requireNativeGPU(t)
	const side, dst = 64, 24 // LOD ≈ log2(64/24) = 1.415，非整数级
	src := r1NoiseImage(t, side)
	defer src.Dispose()

	sums := map[render.MipmapMode][]uint8{}
	for _, mode := range []render.MipmapMode{render.MipmapNone, render.MipmapNearest, render.MipmapLinear} {
		dc := render.NewContext(side, side)
		dc.SetEffectSurface(true)
		dc.ClearWithColor(render.White)
		dc.DrawImageEx(src, render.DrawImageOptions{
			X: 20, Y: 20, DstWidth: dst, DstHeight: dst,
			Interpolation: render.InterpBilinear, Mipmap: mode, Opacity: 1,
		})
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("mode=%v: %v", mode, err)
		}
		var img *render.ImageBuf
		if !dc.ExportImageBuf(&img) || img == nil {
			t.Fatalf("mode=%v 读回失败", mode)
		}
		pix := make([]uint8, 0, dst*dst)
		d, st := img.Data(), img.Stride()
		for y := 20; y < 20+dst; y++ {
			for x := 20; x < 20+dst; x++ {
				pix = append(pix, d[y*st+x*4])
			}
		}
		img.Dispose()
		dc.Close()
		sums[mode] = pix
	}
	diff := func(a, b render.MipmapMode) int {
		n := 0
		for i := range sums[a] {
			if sums[a][i] != sums[b][i] {
				n++
			}
		}
		return n
	}
	dNL, dNNe, dNn := diff(render.MipmapNone, render.MipmapLinear), diff(render.MipmapNone, render.MipmapNearest), diff(render.MipmapNearest, render.MipmapLinear)
	t.Logf("差异像素数（共 %d）：None↔Linear=%d None↔Nearest=%d Nearest↔Linear=%d", dst*dst, dNL, dNNe, dNn)
	if dNL == 0 {
		t.Fatalf("None 与 Linear 逐位相同：多级采样没生效")
	}
	if dNn == 0 {
		t.Fatalf("Nearest 与 Linear 逐位相同：级间选择（mipmap_filter）没被消费，档位是个空开关")
	}
}

// r1NoiseImage 造一张确定性的黑白噪声图（LCG 固定种子，非标准数据集，只是
// 用来把「缩小抽样相位」的混叠暴露出来：mipmap 的存在意义就是这个）。
func r1NoiseImage(t *testing.T, side int) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(side, side, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	data := img.Data()
	state := uint32(0x1234ABCD)
	for i := 0; i+3 < len(data); i += 4 {
		state = state*1664525 + 1013904223
		v := byte(0)
		if state&(1<<7) != 0 {
			v = 255
		}
		data[i+0], data[i+1], data[i+2], data[i+3] = v, v, v, 255
	}
	img.NotifyPixelsChanged()
	t.Cleanup(img.Dispose)
	return img
}


func TestR2_GradientStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(128, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	grad := render.NewLinearGradientBrush(0, 0, 128, 0).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(1, render.RGB(0, 0, 1))
	dc.SetStrokeBrush(grad)
	dc.SetLineWidth(8)
	dc.MoveTo(10, 32)
	dc.LineTo(118, 32)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R2 grad stroke %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("gradient stroke must produce GPUOps")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	// Mid-line should not be pure white.
	r, g, b, _ := sampleRGBA(dc, 64, 32)
	t.Logf("mid=%d,%d,%d", r, g, b)
	if r > 250 && g > 250 && b > 250 {
		t.Fatalf("expected inked gradient stroke")
	}
}

func TestR3_DashedCircleStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.1, 0.2, 0.9)
	dc.SetLineWidth(3)
	dc.SetDash(6, 4)
	dc.DrawCircle(40, 40, 28)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R3 dash circle %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("dashed circle stroke must be GPU")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
}

func TestR3_ThinStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1) // thin: was hard CPU via StrokeShape SDF reject
	dc.DrawRectangle(8, 8, 48, 48)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R3 thin %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("thin stroke must be GPU via expand")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
}
