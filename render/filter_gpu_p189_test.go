//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// P3.4 交付后审查 P1-8/P1-9 钉：GPU 图的 tile 非零原点与 lighting 负 z 必须
// 对齐 CPU 正本（lighting.go 的 modF 平移与「负 z 保留」语义）。

// Tile 非零原点：旧状 WGSL 把像素值 x0/y0 直接加进归一化 uv，采样越界被
// ClampToEdge 拖成边缘色带，与 CPU 分叉。
func TestFilterOpsGPUTileNonZeroOriginMatchesCPU(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 48
	draw := func(dc *render.Context) {
		dc.Clear()
		dc.SetRGBA(1, 0, 0, 1)
		dc.DrawRectangle(6, 6, 18, 12)
		_ = dc.Fill()
		dc.SetRGBA(0, 0, 1, 1)
		dc.DrawRectangle(40, 30, 10, 10)
		_ = dc.Fill()
	}
	rect := [4]float32{6, 6, 24, 18} // 非零原点 tile 区

	gpu := render.NewContext(w, h)
	defer gpu.Close()
	draw(gpu)
	gpu.ResetRenderPathStats()
	gpu.ApplyTile(rect)
	if stats := gpu.RenderPathStats(); stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("ApplyTile did not run on the GPU graph: %s", stats.LogLine())
	}
	gpuImg := gpu.Image()

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	draw(cpu)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyTile(rect)
	restore()
	cpuImg := cpu.Image()

	if d := filterMaxDiff(gpuImg, cpuImg); d > 3 {
		t.Fatalf("GPU tile(non-zero origin) vs CPU max diff = %d, want <=3", d)
	}
}

// Lighting 负 z：CPU 正本保留负 z（lighting.go:73-75 的守卫只抬小正零值），
// GPU 旧状 max(z,0.001) 把灯翻到平面上方——分叉在凹形高度场的缝壁法线上
// （凸块两侧都黑测不出，实测坐实后按缝壁场景钉）。
func TestFilterOpsGPULightingNegativeZMatchesCPU(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 64, 48
	draw := func(dc *render.Context) {
		dc.Clear()
		dc.SetRGBA(1, 1, 1, 1)
		// 两条不透明带留 2px 缝：缝壁是凹形高度场，负 z 分叉在壁上显形。
		dc.DrawRectangle(8, 8, 16, 32)
		_ = dc.Fill()
		dc.DrawRectangle(26, 8, 16, 32)
		_ = dc.Fill()
	}
	lightBelow := [3]float32{10, 24, -20}
	color := render.RGBA{R: 1, G: 1, B: 1, A: 1}

	gpu := render.NewContext(w, h)
	defer gpu.Close()
	draw(gpu)
	gpu.ResetRenderPathStats()
	gpu.ApplyLighting(lightBelow, color, 1, 2)
	if stats := gpu.RenderPathStats(); stats.GPUOps == 0 || stats.CPUFallbackOps != 0 {
		t.Fatalf("ApplyLighting did not run on the GPU graph: %s", stats.LogLine())
	}

	cpu := render.NewContext(w, h)
	defer cpu.Close()
	draw(cpu)
	restore := render.DisableGPUFilterGraphForTest()
	cpu.ApplyLighting(lightBelow, color, 1, 2)
	restore()

	// 容差声明：既有 GPU==CPU 钉为 ≤3；本场景的缝壁是 1px alpha 阶跃，
	// 法线投影把 GPU FP32 归一化/采样的微差放大到恒定 4/255（修复前为
	// 166，语义分叉；修复后三轮恒 4，纯数值残差），故放 5。台面近黑锚
	// 在下面单独钉死，排除「双双变黑」的假对齐。
	if d := filterMaxDiff(gpu.Image(), cpu.Image()); d > 5 {
		t.Fatalf("GPU lighting(negative z) vs CPU max diff = %d, want <=5 (seam-wall FP32 residual, pre-fix was 166)", d)
	}
	// 独立证据：灯在下方时平坦台面必须近黑（两侧都要黑才算对齐，防双错同绿）。
	r, g, b, _ := filterRawPixel(cpu, 34, 24)
	if r+g+b > 30 {
		t.Fatalf("CPU reference plateau lit by below-plane light: (%d,%d,%d)", r, g, b)
	}
	rg, gg, bg, _ := filterRawPixel(gpu, 34, 24)
	if rg+gg+bg > 30 {
		t.Fatalf("GPU plateau lit under a below-plane light: (%d,%d,%d), want near black", rg, gg, bg)
	}
}

// P1-7 钉：大 sigma 的 GPU blur 必须与 CPU 正本一致（核半径钳 ±24 的截断
// 分叉实测 σ=12 diff=5、破既有 ≤3 容差；按 Impeller 动态半径口径去掉钳位）。
func TestFilterOpsGPUBlurLargeSigmaMatchesCPU(t *testing.T) {
	requireFilterOpsGPU(t)
	const w, h = 96, 64
	draw := func(dc *render.Context) {
		dc.Clear()
		dc.SetRGBA(1, 1, 1, 1)
		dc.DrawRectangle(40, 25, 16, 14)
		_ = dc.Fill()
	}
	for _, sigma := range []float64{9, 12, 20} {
		gpu := render.NewContext(w, h)
		draw(gpu)
		gpu.ResetRenderPathStats()
		gpu.ApplyBlur(sigma)
		st := gpu.RenderPathStats()
		if st.GPUOps == 0 || st.CPUFallbackOps != 0 {
			t.Fatalf("sigma=%v: blur did not run on the GPU graph: %s", sigma, st.LogLine())
		}
		cpu := render.NewContext(w, h)
		draw(cpu)
		restore := render.DisableGPUFilterGraphForTest()
		cpu.ApplyBlur(sigma)
		restore()
		if d := filterMaxDiff(gpu.Image(), cpu.Image()); d > 3 {
			t.Fatalf("sigma=%v GPU blur vs CPU max diff = %d, want <=3（核半径截断残留）", sigma, d)
		}
		gpu.Close()
		cpu.Close()
	}
}
