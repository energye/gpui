//go:build !nogpu

package render_test

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// P3.7 批 1b 附带修复的钉：bicubic 必须真的生效。
//
// 洞的形态（与 P3.6 批 2 记过的「variadic 吞参数」同族）：QueueImageDraw /
// QueueImageDrawTint 的签名结尾是 `nearest bool, bicubic ...bool`，实现读
// bicubic[0]；两处调用按位置多塞了一个 false（`nearest, false, bicubic`），
// 于是 bicubic[0] 永远是那个 false ⇒ DrawImageQuad 的仿射腿与图集「带染色」分支
// 指定 bicubic 时实际走双线性，而且与非染色分支（:296 写法正确）行为不一致。
// 主图像路 DrawImageEx（context_image_gpu.go:126）写法正确，不受影响。
//
// 判据设计成「两侧都不生效也会红」：只断言 bicubic≠bilinear 的差异存在，
// 不断言差异方向；GPU 像素一律 ExportImageBuf 从显存读回后 GetRGBA。

func bicubicPlate(t *testing.T) *render.ImageBuf {
	t.Helper()
	// 硬边棋盘 + 明暗斜阶：放大绘制时 4×4 核与双线性在边缘处必然分叉。
	const w, h = 16, 16
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8(20)
			if (x/4+y/4)%2 == 0 {
				v = 235
			}
			if err := img.SetRGBA(x, y, v, uint8(x*16), uint8(y*16), 255); err != nil {
				t.Fatal(err)
			}
		}
	}
	img.NotifyPixelsChanged()
	return img
}

// drawQuadWithInterp 用指定采样方式把整张图铺进矩形 quad，显存读回。
// 返回的缓冲由调用方 Dispose（helper 不代放，数据读完才算完）。
func drawQuadWithInterp(t *testing.T, img *render.ImageBuf, interp render.InterpolationMode, tint render.RGBA) *render.ImageBuf {
	t.Helper()
	os.Unsetenv("GOGPU_RENDER_MODE")
	dc := render.NewContext(200, 120)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	corners := [4]render.Point{{X: 10, Y: 10}, {X: 150, Y: 10}, {X: 150, Y: 100}, {X: 10, Y: 100}}
	if err := dc.DrawImageQuad(img, corners, render.QuadDrawOptions{
		Interpolation: interp, Opacity: 1, Tint: tint,
	}); err != nil {
		t.Fatalf("DrawImageQuad interp=%v: %v", interp, err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("没走 GPU 或掉回 CPU：%s", st.LogLine())
	}
	var out *render.ImageBuf
	if !dc.ExportImageBuf(&out) || out == nil {
		t.Fatal("显存读回失败")
	}
	return out
}

func diffStats(a, b *render.ImageBuf) (changed int, maxDelta int) {
	w, h := a.Bounds()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r1, g1, b1, _ := a.GetRGBA(x, y)
			r2, g2, b2, _ := b.GetRGBA(x, y)
			for _, pair := range [][2]uint8{{r1, r2}, {g1, g2}, {b1, b2}} {
				d := int(pair[0]) - int(pair[1])
				if d < 0 {
					d = -d
				}
				if d > maxDelta {
					maxDelta = d
				}
				if d > 0 {
					changed++
					break
				}
			}
		}
	}
	return changed, maxDelta
}

// TestP37QuadBicubicActuallyEngages：仿射腿 bicubic 必须与 bilinear 出不同画面。
func TestP37QuadBicubicActuallyEngages(t *testing.T) {
	requireNativeGPU(t)
	img := bicubicPlate(t)
	defer img.Dispose()

	bilinear := drawQuadWithInterp(t, img, render.InterpBilinear, render.RGBA{})
	defer bilinear.Dispose()
	bicubic := drawQuadWithInterp(t, img, render.InterpBicubic, render.RGBA{})
	defer bicubic.Dispose()
	changed, _ := diffStats(bilinear, bicubic)
	if changed == 0 {
		t.Fatal("bicubic 与 bilinear 逐像素相同：4×4 核没接上（variadic 尾参被 false 吞掉的形态）")
	}
	t.Logf("P37 bicubic 生效：差异像素=%d（两档必须分叉）", changed)

	// bicubic 自己也要与 CPU 同式（容差显式声明：稳定区逐通道 ≤8/255，
	// GPU f32 4×4 核 vs CPU f64 的既有量级，P3.5 批 3 同族口径）。
	os.Setenv("GOGPU_RENDER_MODE", "cpu")
	dcCPU := render.NewContext(200, 120)
	defer func() { _ = dcCPU.Close() }()
	dcCPU.ClearWithColor(render.White)
	corners := [4]render.Point{{X: 10, Y: 10}, {X: 150, Y: 10}, {X: 150, Y: 100}, {X: 10, Y: 100}}
	if err := dcCPU.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic, Opacity: 1}); err != nil {
		t.Fatal(err)
	}
	_ = dcCPU.FlushGPU()
	cpuImg := dcCPU.Image()
	os.Unsetenv("GOGPU_RENDER_MODE")
	dcGPU := render.NewContext(200, 120)
	defer func() { _ = dcGPU.Close() }()
	dcGPU.ClearWithColor(render.White)
	if err := dcGPU.DrawImageQuad(img, corners, render.QuadDrawOptions{Interpolation: render.InterpBicubic, Opacity: 1}); err != nil {
		t.Fatal(err)
	}
	if err := dcGPU.FlushGPU(); err != nil {
		t.Fatal(err)
	}
	if !dcGPU.ExportImageBuf(&bicubic) || bicubic == nil {
		t.Fatal("显存读回失败")
	}
	defer bicubic.Dispose()
	worst := 0
	for y := 20; y < 90; y += 7 {
		for x := 20; x < 140; x += 7 {
			r1, g1, b1, _ := cpuImg.At(x, y).RGBA()
			r2, g2, b2, _ := bicubic.GetRGBA(x, y)
			for _, d := range []int{abs8(int(r1>>8), int(r2)), abs8(int(g1>>8), int(g2)), abs8(int(b1>>8), int(b2))} {
				if d > worst {
					worst = d
				}
			}
		}
	}
	t.Logf("P37 bicubic GPU/CPU 稳定区最大通道差=%d", worst)
	if worst > 8 {
		t.Errorf("bicubic GPU/CPU 分叉 %d > 8（显式声明的容差）", worst)
	}
}

func abs8(a, b int) int {
	if d := a - b; d < 0 {
		return -d
	} else {
		return d
	}
}

// TestP37AtlasTintBicubicEngages：图集腿（DrawAtlasEx）GPU 路恒走 QueueImageDrawTint
// （vertices_atlas.go tintIface=useGPU），该分支的 bicubic 尾参修前被吞 ⇒ 指定 bicubic
// 实走双线性。本钉保证图集腿 bicubic 真生效、且白染色不改变采样核。
func TestP37AtlasTintBicubicEngages(t *testing.T) {
	requireNativeGPU(t)
	img := bicubicPlate(t)
	defer img.Dispose()

	draw := func(filter render.InterpolationMode, tint render.RGBA) *render.ImageBuf {
		t.Helper()
		os.Unsetenv("GOGPU_RENDER_MODE")
		dc := render.NewContext(200, 120)
		defer func() { _ = dc.Close() }()
		dc.ClearWithColor(render.White)
		res, err := dc.DrawAtlasEx(img, []render.AtlasSprite{{
			SrcX: 0, SrcY: 0, SrcW: 16, SrcH: 16,
			DstX: 10, DstY: 10, DstW: 140, DstH: 90,
			Filter: filter, Tint: tint, Opacity: 1,
		}}, render.AtlasDrawOptions{})
		if err != nil {
			t.Fatalf("DrawAtlasEx filter=%v: %v", filter, err)
		}
		if res.Skipped || res.Drawn != 1 || res.Degraded {
			t.Fatalf("图集未走 GPU 批绘：res=%+v", res)
		}
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("FlushGPU: %v", err)
		}
		if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("图集没走 GPU 或掉回 CPU：%s", st.LogLine())
		}
		var out *render.ImageBuf
		if !dc.ExportImageBuf(&out) || out == nil {
			t.Fatal("显存读回失败")
		}
		return out
	}

	bicubic := draw(render.InterpBicubic, render.RGBA{})
	defer bicubic.Dispose()
	bilinear := draw(render.InterpBilinear, render.RGBA{})
	defer bilinear.Dispose()
	changedAtlas, _ := diffStats(bilinear, bicubic)
	if changedAtlas == 0 {
		t.Fatal("图集腿 bicubic 与 bilinear 逐像素相同：染色分支的 bicubic 尾参被吞")
	}
	t.Logf("P37 图集 bicubic 生效：差异像素=%d", changedAtlas)

	bicWhite := draw(render.InterpBicubic, render.RGBA{R: 1, G: 1, B: 1, A: 1})
	defer bicWhite.Dispose()
	if changed, maxDelta := diffStats(bicubic, bicWhite); changed != 0 || maxDelta != 0 {
		t.Errorf("图集腿显式白染色与零结构体在 bicubic 下画面不同（changed=%d max=%d）", changed, maxDelta)
	}
}

// TestP37QuadTintDoesNotChangeSampling：同一 bicubic 请求，带不带染色只该差在染色上，
// 采样核不能因为走了另一条队列口而变档（修前正是这样：tint 分支 bicubic 被吞）。
func TestP37QuadTintDoesNotChangeSampling(t *testing.T) {
	requireNativeGPU(t)
	img := bicubicPlate(t)
	defer img.Dispose()

	bicWhite := drawQuadWithInterp(t, img, render.InterpBicubic, render.RGBA{R: 1, G: 1, B: 1, A: 1})
	defer bicWhite.Dispose()
	bicNone := drawQuadWithInterp(t, img, render.InterpBicubic, render.RGBA{})
	defer bicNone.Dispose()
	if changed, maxDelta := diffStats(bicWhite, bicNone); changed != 0 || maxDelta != 0 {
		t.Errorf("显式白染色与零结构体在 bicubic 下画面不同（changed=%d max=%d）：两分支采样档不一致", changed, maxDelta)
	}
	// 且两者都要与 bilinear 分叉（否则两分支都退回双线性）。
	bilinear := drawQuadWithInterp(t, img, render.InterpBilinear, render.RGBA{R: 1, G: 1, B: 1, A: 1})
	defer bilinear.Dispose()
	if changed, _ := diffStats(bilinear, bicWhite); changed == 0 {
		t.Error("带染色的 bicubic 与 bilinear 相同：tint 分支的采样核没接上")
	}
}
