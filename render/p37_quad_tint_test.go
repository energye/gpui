//go:build !nogpu

package render_test

import (
	"image"
	"math"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// P3.7 批 1b 钉：quad 染色的 CPU/GPU 一致性与零值不变。
//
// 透视腿的 36B 顶点布局与 textured_quad_persp.wgsl 的 @location(2) tint 通道本来就存在，
// 只是 `buildPerspVerticesInto` 把它写死成白 ⇒ 「按深度投影的精灵」与「被灯光染色」在过去
// 只能选一个（仿射批绘腿有 tint、透视腿没有）。本批把两侧打通，并显式声明容差。

func drawTintedQuad(t *testing.T, img *render.ImageBuf, persp bool, tint render.RGBA, cpu bool) image.Image {
	t.Helper()
	if cpu {
		t.Setenv("GOGPU_RENDER_MODE", "cpu")
	} else {
		os.Unsetenv("GOGPU_RENDER_MODE")
	}
	dc := render.NewContext(241, 121)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	if err := dc.DrawImageQuad(img, perspGPUCorners, render.QuadDrawOptions{
		Perspective:   persp,
		Weights:       perspGPUWeights,
		Interpolation: render.InterpNearest,
		Opacity:       1,
		Tint:          tint,
	}); err != nil {
		t.Fatalf("DrawImageQuad persp=%v cpu=%v: %v", persp, cpu, err)
	}
	if ferr := dc.FlushGPU(); ferr != nil {
		t.Fatalf("FlushGPU: %v", ferr)
	}
	if !cpu {
		if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("染色透视 quad 没走 GPU 或掉回 CPU：%s", st.LogLine())
		}
	}
	out := image.NewNRGBA(dc.Image().Bounds())
	src := dc.Image()
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			out.Set(x, y, src.At(x, y))
		}
	}
	return out
}

// maxChannelDiff 只在「纯色稳定区」（远离 homography 色带边界与 quad 边缘 AA）取样，
// 容差口径沿用 P3.5 批 3 的声明：稳定像素逐通道 ≤1/255（GPU f32 clip-w vs CPU f64）。
func maxChannelDiff(a, b image.Image, pts [][2]int) (int, [2]int) {
	worst, at := 0, [2]int{}
	for _, p := range pts {
		ra, ga, ba, _ := a.At(p[0], p[1]).RGBA()
		rb, gb, bb, _ := b.At(p[0], p[1]).RGBA()
		for _, pair := range [][2]uint32{{ra >> 8, rb >> 8}, {ga >> 8, gb >> 8}, {ba >> 8, bb >> 8}} {
			d := int(pair[0]) - int(pair[1])
			if d < 0 {
				d = -d
			}
			if d > worst {
				worst = d
				at = p
			}
		}
	}
	return worst, at
}

func max8(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

var tintSettledPoints = [][2]int{{30, 30}, {60, 60}, {100, 30}, {140, 90}, {200, 30}, {220, 90}}

func TestP37QuadTintPerspCPUGPUParity(t *testing.T) {
	requireNativeGPU(t)
	img := newPerspBandImage(t)
	defer img.Dispose()

	tints := []render.RGBA{
		{R: 0.5, G: 0.25, B: 0.75, A: 1},
		{R: 1, G: 0, B: 0, A: 1},
		{R: 0.25, G: 0.25, B: 0.25, A: 1},
		{R: 0.8, G: 0.8, B: 0.8, A: 0.5},
	}
	for _, tint := range tints {
		gpu := drawTintedQuad(t, img, true, tint, false)
		cpu := drawTintedQuad(t, img, true, tint, true)
		if diff, at := maxChannelDiff(gpu, cpu, tintSettledPoints); diff > 1 {
			t.Errorf("透视染色 GPU/CPU 分叉 tint=%v：最大通道差 %d @%v", tint, diff, at)
		}
	}
}

func TestP37QuadTintAffineCPUGPUParity(t *testing.T) {
	requireNativeGPU(t)
	img := newPerspBandImage(t)
	defer img.Dispose()

	for _, tint := range []render.RGBA{{R: 0.5, G: 0.25, B: 0.75, A: 1}, {R: 0.8, G: 0.8, B: 0.8, A: 0.5}} {
		gpu := drawTintedQuad(t, img, false, tint, false)
		cpu := drawTintedQuad(t, img, false, tint, true)
		if diff, at := maxChannelDiff(gpu, cpu, tintSettledPoints); diff > 1 {
			t.Errorf("仿射染色 GPU/CPU 分叉 tint=%v：最大通道差 %d @%v", tint, diff, at)
		}
	}
}

// TestP37QuadTintActuallyDarkens 证明染色不是空转：同图同几何，染色后必须比不染色暗，
// 且透视与仿射两腿都被 tint 影响（否则「通道打通」只是自我声明）。
func TestP37QuadTintActuallyDarkens(t *testing.T) {
	requireNativeGPU(t)
	img := newPerspBandImage(t)
	defer img.Dispose()

	gray := render.RGBA{R: 0.5, G: 0.5, B: 0.5, A: 1}
	for _, persp := range []bool{false, true} {
		label := "仿射"
		if persp {
			label = "透视"
		}
		plain := drawTintedQuad(t, img, persp, render.RGBA{}, false)
		dyed := drawTintedQuad(t, img, persp, gray, false)
		for _, p := range tintSettledPoints {
			r0, _, b0, _ := plain.At(p[0], p[1]).RGBA()
			r1, _, b1, _ := dyed.At(p[0], p[1]).RGBA()
			// 红通道或蓝通道之一在该点必有非零亮度（色带图），半灰染色必须把它压下去。
			if max8(r0>>8, b0>>8) <= 40 {
				continue // 该点是白底，不预期变化
			}
			if r1>>8 >= r0>>8 && b1>>8 >= b0>>8 {
				t.Errorf("%s腿 tint=0.5 灰未压暗 @(%d,%d)：plain=(%d,%d) dyed=(%d,%d)",
					label, p[0], p[1], r0>>8, b0>>8, r1>>8, b1>>8)
			}
		}
	}
}

// TestP37QuadTintZeroIsIdentity 钉存量零变化：零结构体与显式白不透明必须逐位同画面
// （两者都走 identity 因子），且零结构体画面与本批之前的行为一致。
func TestP37QuadTintZeroIsIdentity(t *testing.T) {
	requireNativeGPU(t)
	img := newPerspBandImage(t)
	defer img.Dispose()

	white := render.RGBA{R: 1, G: 1, B: 1, A: 1}
	for _, persp := range []bool{false, true} {
		a := drawTintedQuad(t, img, persp, render.RGBA{}, false)
		b := drawTintedQuad(t, img, persp, white, false)
		if diff, at := maxChannelDiff(a, b, tintSettledPoints); diff != 0 {
			t.Errorf("零结构体与显式白不逐位相同 persp=%v：差 %d @%v", persp, diff, at)
		}
	}
}

// TestP37QuadTintRejectsNonFinite 钉非有限染色的具名拒绝（不静默画错）。
func TestP37QuadTintRejectsNonFinite(t *testing.T) {
	img := newPerspBandImage(t)
	defer img.Dispose()

	dc := render.NewContext(241, 121)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	bad := []render.RGBA{
		{R: math.NaN(), G: 0, B: 0, A: 1},
		{R: 0, G: math.Inf(1), B: 0, A: 1},
		{R: 0, G: 0, B: 0, A: math.NaN()},
	}
	for _, tint := range bad {
		if err := dc.DrawImageQuad(img, perspGPUCorners, render.QuadDrawOptions{
			Perspective: true, Weights: perspGPUWeights, Interpolation: render.InterpNearest, Tint: tint,
		}); err == nil {
			t.Fatalf("非有限 tint=%v 未被拒绝", tint)
		}
	}
}
