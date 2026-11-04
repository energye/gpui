//go:build !nogpu

package render_test

// P3.7 批 2 步 2e · 盖序不变式常驻钉（探针 p1-draw-order.txt 转正常驻测试）。
//
// 判据三条（照 batch2-design.md §5，按 Godot 标准写）：
//  1. 屏幕盖序 = 提交序（画家算法前提，容差 0）；
//  2. tier 切换只 flush 不重排——跨 tier 提交（路径 tier=1 ↔ 图像 tier=4）与同 tier
//     合批都不许换序（Godot 也是遇 screen-texture/canvas-group/SDF 切换就地 flush，
//     renderer_canvas_render_rd.cpp:820,852,876,906）；
//  3. 适用边界=单容器（一张 Context 的一帧内），跨窗跨层不在此断言范围。
//
// 八组形态照探针原样搬进来（A/B 跨 tier 双向、C/D 同 tier 双向、E/F 三段往返、
// G 两次跨 tier 往返、H 同 tier 路径对照），GPUOps 数一并钉住，防"只画了一层却
// 碰巧对"。双后端由门禁分别在 GPUI_BACKEND=native / go 下跑本文件；本文件内部
// 再各跑一遍 CPU 腿（GOGPU_RENDER_MODE=cpu），两条光栅器同判据。

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

const (
	drawOrderCanvas = 12
	// 交点 (6,6) 同时落在红矩形(2,2,8,8)、蓝图(4,4,4,4)、绿图(5,5,4,4)、
	// 黄矩形(3,3,6,6) 之内：谁最后提交，谁就是这里的颜色。
	drawOrderProbeX = 6
	drawOrderProbeY = 6
)

type drawOrderStep struct {
	label string
	path  bool // true=路径矩形（tier 1），false=图像（tier 4）
	x     float64
	y     float64
	w     float64
	h     float64
	rgb   [3]float64
}

type drawOrderGroup struct {
	name      string
	steps     []drawOrderStep
	wantProbe [4]uint8
	wantGPU   int
	reason    string
}

var (
	drawOrderRed    = [3]float64{1, 0, 0}
	drawOrderYellow = [3]float64{1, 1, 0}
	drawOrderBlue   = [4]uint8{0, 0, 255, 255}
	drawOrderGreen  = [4]uint8{0, 255, 0, 255}
)

func rectStep(label string, x, y, w, h float64, rgb [3]float64) drawOrderStep {
	return drawOrderStep{label: label, path: true, x: x, y: y, w: w, h: h, rgb: rgb}
}

func imageStep(label string, x, y float64, rgba [4]uint8) drawOrderStep {
	return drawOrderStep{label: label, path: false, x: x, y: y, w: 4, h: 4, rgb: [3]float64{
		float64(rgba[0]) / 255, float64(rgba[1]) / 255, float64(rgba[2]) / 255,
	}}
}

var drawOrderGroups = []drawOrderGroup{
	{"A_路径(先)->图像(后)", []drawOrderStep{
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
		imageStep("蓝图", 4, 4, drawOrderBlue),
	}, drawOrderBlue, 2, "低 tier 后提交，须触发封段才不丢序"},
	{"B_图像(先)->路径(后)", []drawOrderStep{
		imageStep("蓝图", 4, 4, drawOrderBlue),
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
	}, [4]uint8{255, 0, 0, 255}, 2, "与 A 反向，判 tier 是否压过提交序"},
	{"C_同_tier_两图：绿后", []drawOrderStep{
		imageStep("蓝图", 4, 4, drawOrderBlue),
		imageStep("绿图", 5, 5, drawOrderGreen),
	}, drawOrderGreen, 2, "同 tier 合批不得换序"},
	{"D_同_tier_两图：蓝后", []drawOrderStep{
		imageStep("绿图", 5, 5, drawOrderGreen),
		imageStep("蓝图", 4, 4, drawOrderBlue),
	}, drawOrderBlue, 2, "与 C 反向"},
	{"E_三段混排：路径->图像->路径", []drawOrderStep{
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
		imageStep("蓝图", 4, 4, drawOrderBlue),
		rectStep("黄矩形", 3, 3, 6, 6, drawOrderYellow),
	}, [4]uint8{255, 255, 0, 255}, 3, "2.5D 地面(路径)+精灵(图像)+前景(路径) 的真实混排"},
	{"F_三段反序：路径->图像->路径", []drawOrderStep{
		rectStep("黄矩形", 3, 3, 6, 6, drawOrderYellow),
		imageStep("蓝图", 4, 4, drawOrderBlue),
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
	}, [4]uint8{255, 0, 0, 255}, 3, "与 E 反向"},
	{"G_图像被路径压过再由图像盖回", []drawOrderStep{
		imageStep("蓝图", 4, 4, drawOrderBlue),
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
		imageStep("绿图", 5, 5, drawOrderGreen),
	}, drawOrderGreen, 3, "两次跨 tier 往返（4->1->4）"},
	{"H_纯路径互叠：红先黄后", []drawOrderStep{
		rectStep("红矩形", 2, 2, 8, 8, drawOrderRed),
		rectStep("黄矩形", 3, 3, 6, 6, drawOrderYellow),
	}, [4]uint8{255, 255, 0, 255}, 2, "同 tier=1 内的对照（路径链上画家算法必须成立）"},
}

func drawOrderPlate(t *testing.T, rgba [4]uint8) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(4, 4, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if err := img.SetRGBA(x, y, rgba[0], rgba[1], rgba[2], rgba[3]); err != nil {
				t.Fatalf("SetRGBA: %v", err)
			}
		}
	}
	return img
}

// runDrawOrderGroup 画一组并提交，返回交点颜色、角落颜色与 GPUOps。
func runDrawOrderGroup(t *testing.T, g drawOrderGroup, gpu bool) ([4]uint8, [4]uint8, int) {
	t.Helper()
	plates := map[[3]float64]*render.ImageBuf{}
	for _, s := range g.steps {
		if s.path {
			continue
		}
		if _, ok := plates[s.rgb]; ok {
			continue
		}
		var rgba [4]uint8
		rgba[0] = uint8(s.rgb[0]*255 + 0.5)
		rgba[1] = uint8(s.rgb[1]*255 + 0.5)
		rgba[2] = uint8(s.rgb[2]*255 + 0.5)
		rgba[3] = 255
		plates[s.rgb] = drawOrderPlate(t, rgba)
	}
	dc := render.NewContext(drawOrderCanvas, drawOrderCanvas)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	for _, s := range g.steps {
		if s.path {
			dc.SetRGB(s.rgb[0], s.rgb[1], s.rgb[2])
			dc.DrawRectangle(s.x, s.y, s.w, s.h)
			if err := dc.Fill(); err != nil {
				t.Fatalf("%s fill: %v", g.name, err)
			}
			continue
		}
		dc.SetRGB(1, 1, 1)
		dc.DrawImage(plates[s.rgb], s.x, s.y)
	}
	if gpu {
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("%s FlushGPU: %v", g.name, err)
		}
	}
	probe, corner := [4]uint8{}, [4]uint8{}
	img := dc.Image()
	read := func(x, y int, into *[4]uint8) {
		r, gg, b, a := img.At(x, y).RGBA()
		into[0], into[1], into[2], into[3] = uint8(r>>8), uint8(gg>>8), uint8(b>>8), uint8(a>>8)
	}
	read(drawOrderProbeX, drawOrderProbeY, &probe)
	read(0, 0, &corner)
	ops := 0
	if gpu {
		ops = dc.RenderPathStats().GPUOps
	}
	return probe, corner, ops
}

// 判据逐组核对：交点颜色、角落必须是白（没被越界污染）、GPUOps 数（GPU 腿）。
func checkDrawOrderGroup(t *testing.T, g drawOrderGroup, gpu bool) {
	t.Helper()
	probe, corner, ops := runDrawOrderGroup(t, g, gpu)
	if probe != g.wantProbe {
		t.Errorf("%s 交点=(%d,%d) got %v want %v：提交序=%v（%s）",
			g.name, drawOrderProbeX, drawOrderProbeY, probe, g.wantProbe, labelsOf(g.steps), g.reason)
	}
	if corner != [4]uint8{255, 255, 255, 255} {
		t.Errorf("%s 角落被污染：%v", g.name, corner)
	}
	if gpu && ops != g.wantGPU {
		t.Errorf("%s GPUOps=%d want %d（批次数变了说明提交被重排或合并）", g.name, ops, g.wantGPU)
	}
}

func labelsOf(steps []drawOrderStep) []string {
	out := make([]string, len(steps))
	for i, s := range steps {
		out[i] = s.label
	}
	return out
}

func TestP37DrawOrderIsSubmissionOrder(t *testing.T) {
	t.Run("CPU腿", func(t *testing.T) {
		t.Setenv("GOGPU_RENDER_MODE", "cpu")
		for _, g := range drawOrderGroups {
			checkDrawOrderGroup(t, g, false)
		}
	})
	t.Run("GPU腿", func(t *testing.T) {
		if render.Accelerator() == nil {
			t.Skipf("no GPU accelerator registered（不算假绿：门禁在双后端进程各跑一次）")
		}
		orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
		t.Cleanup(func() {
			if had {
				_ = os.Setenv("GOGPU_RENDER_MODE", orig)
			} else {
				_ = os.Unsetenv("GOGPU_RENDER_MODE")
			}
		})
		probe := render.NewContext(8, 8)
		probe.ClearWithColor(render.White)
		probe.SetRGB(1, 0, 0)
		probe.DrawRectangle(0, 0, 8, 8)
		_ = probe.Fill()
		if err := probe.FlushGPU(); err != nil {
			probe.Close()
			t.Skipf("GPU flush unavailable: %v", err)
		}
		if probe.RenderPathStats().GPUOps == 0 {
			line := probe.RenderPathStats().LogLine()
			probe.Close()
			t.Skipf("no GPU ops（本环境没有原生 GPU）: %s", line)
		}
		probe.Close()
		for _, g := range drawOrderGroups {
			t.Run(g.name, func(t *testing.T) {
				checkDrawOrderGroup(t, g, true)
			})
		}
	})
}

// 整幅画面逐位对照：同一段提交在 CPU 腿与 GPU 腿上必须画出一样的盖序
// （采样舍入不许影响颜色索引，容差 0）。
func TestP37DrawOrderCPUGPUMapsMatch(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skipf("no GPU accelerator registered")
	}
	for _, g := range drawOrderGroups {
		t.Setenv("GOGPU_RENDER_MODE", "cpu")
		probeCPU, _, _ := runDrawOrderGroup(t, g, false)
		orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
		_ = os.Unsetenv("GOGPU_RENDER_MODE")
		probeGPU, _, _ := runDrawOrderGroup(t, g, true)
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
		if probeCPU != probeGPU {
			t.Fatalf("%s CPU 腿交点 %v ≠ GPU 腿 %v", g.name, probeCPU, probeGPU)
		}
	}
}
