package scene_test

// P3.7 批 3 收尾：scene 离屏渲染的「走了哪条腿 + 这一腿真排了多少 op」必须有真数可读。
//
// 起因（示例层报假数，实证与负向留档见 evidence/P3.7/batch3/wrapup.md）：
//   - `examples/render_visual_scene_gpu` 硬印一行 `gpu_ops=0 cpu_fallback_ops=0`；
//   - 两扇 _gpu 示例的判据写成「TilesRendered == 0 ⇒ 走了 GPU」，而 GPU 腿早退时
//     **根本不重置 stats**，上一帧 CPU 的瓦片数会残留进 GPU 帧的 Stats() 里。
//     0 块瓦片既可能是「走了 GPU 腿」也可能是「上一帧的数被抹了/这帧没画」。
// 修法在引擎侧：renderGPU 把 render.Context 自己的 RenderPathStats 取出来上报，
// 两条腿都从 resetStats() 起账。这几枚钉就是把这个可观测性钉死。

import (
	"sync/atomic"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

// legProbe 是一枚只记账不出图的加速器替身：每个形状都回 ErrFallbackToCPU，
// 于是「GPU 腿被采纳」与「逐形状回落 CPU」两件事能被分开量到（正是真机上
// sdf-gpu direct=false 的形态）。
type legProbe struct {
	shapes atomic.Int32
}

func (p *legProbe) Name() string { return "leg-probe" }
func (p *legProbe) Init() error  { return nil }
func (p *legProbe) Close()       {}
func (p *legProbe) CanAccelerate(op render.AcceleratedOp) bool {
	return op&(render.AccelCircleSDF|render.AccelRRectSDF|render.AccelFill) != 0
}

func (p *legProbe) FillShape(_ render.GPURenderTarget, _ render.DetectedShape, _ *render.Paint) error {
	p.shapes.Add(1)
	return render.ErrFallbackToCPU
}

func (p *legProbe) StrokeShape(_ render.GPURenderTarget, _ render.DetectedShape, _ *render.Paint) error {
	return render.ErrFallbackToCPU
}

func (p *legProbe) FillPath(_ render.GPURenderTarget, _ *render.Path, _ *render.Paint) error {
	p.shapes.Add(1)
	return render.ErrFallbackToCPU
}

func (p *legProbe) StrokePath(_ render.GPURenderTarget, _ *render.Path, _ *render.Paint) error {
	return render.ErrFallbackToCPU
}

func (p *legProbe) Flush(_ render.GPURenderTarget) error { return nil }

func legScene() *scene.Scene {
	s := scene.NewScene()
	b := scene.NewSceneBuilderFrom(s)
	b.FillRect(10, 10, 100, 100, scene.SolidBrush(render.Blue))
	b.FillCircle(150, 60, 40, scene.SolidBrush(render.Red))
	return s
}

// 收尾钉 A：瓦片腿的账必须干净——TilesRendered>0，且不许谎报走了 GPU 腿。
func TestP37StatsReportsTileLeg(t *testing.T) {
	render.CloseAccelerator() // 确保没有残留替身
	r := scene.NewRenderer(200, 200)
	defer r.Close()
	pm := render.NewPixmap(200, 200)
	if err := r.Render(pm, legScene()); err != nil {
		t.Fatalf("Render: %v", err)
	}
	st := r.Stats()
	if st.GPULegTaken {
		t.Fatal("没注册加速器却报 GPULegTaken=true")
	}
	if st.TilesRendered <= 0 {
		t.Fatalf("瓦片腿没记账：TilesRendered=%d", st.TilesRendered)
	}
	if st.GPUOps != 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("纯 CPU 帧却有 GPU 计数：%d/%d", st.GPUOps, st.CPUFallbackOps)
	}
}

// 收尾钉 B（残留洞）：同一 Renderer 先瓦片帧后 GPU 帧，GPU 帧不许带着上一帧的瓦片数；
// 再回到瓦片帧，GPU 那一路的计数也必须清零。
func TestP37StatsDoNotLeakAcrossLegs(t *testing.T) {
	r := scene.NewRenderer(200, 200)
	defer r.Close()
	pm := render.NewPixmap(200, 200)

	if err := r.Render(pm, legScene()); err != nil {
		t.Fatalf("tile frame: %v", err)
	}
	tileStats := r.Stats()
	if tileStats.TilesRendered <= 0 || tileStats.GPULegTaken {
		t.Fatalf("第一帧应为干净的瓦片帧：%+v", tileStats)
	}

	probe := &legProbe{}
	if err := render.RegisterAccelerator(probe); err != nil {
		t.Fatalf("RegisterAccelerator: %v", err)
	}
	if err := r.Render(pm, legScene()); err != nil {
		t.Fatalf("gpu frame: %v", err)
	}
	gpuStats := r.Stats()
	render.CloseAccelerator()

	if !gpuStats.GPULegTaken {
		t.Fatal("注册了加速器却没报 GPULegTaken：示例只能继续靠「0 块瓦片」猜腿")
	}
	if gpuStats.TilesRendered != 0 {
		t.Fatalf("GPU 帧残留了上一帧的瓦片数：%d（want 0）——Stats 不可信，「0 块瓦片⇒GPU」的判据就是这么来的",
			gpuStats.TilesRendered)
	}
	if probe.shapes.Load() == 0 {
		t.Fatal("GPU 腿一个形状都没收到：这帧其实没走 GPU")
	}
	// 每个形状都回 ErrFallbackToCPU ⇒ Context 的回落计数必须非零，
	// 这就是「GPU 腿被采纳 ≠ 真 GPU 求值」的实测形态（真机 sdf-gpu direct=false 同构）。
	if gpuStats.CPUFallbackOps == 0 {
		t.Fatalf("Context 的回落计数没转交上来：%+v（示例当年硬印 0 就是因为没这个口）", gpuStats)
	}

	// 撤掉加速器再回瓦片腿：GPU 那一路的计数不许留在本帧。
	if err := r.Render(pm, legScene()); err != nil {
		t.Fatalf("second tile frame: %v", err)
	}
	back := r.Stats()
	if back.GPULegTaken || back.GPUOps != 0 || back.CPUFallbackOps != 0 {
		t.Fatalf("回到瓦片腿却带着 GPU 计数：GPULegTaken=%v ops=%d/%d",
			back.GPULegTaken, back.GPUOps, back.CPUFallbackOps)
	}
	if back.TilesRendered <= 0 {
		t.Fatalf("第二帧瓦片腿没记账：%d", back.TilesRendered)
	}
}

// 收尾钉 C：脏区入口（RenderDirty / RenderWithDamage）同样从干净一帧起账。
func TestP37StatsResetOnDirtyEntry(t *testing.T) {
	r := scene.NewRenderer(200, 200)
	defer r.Close()
	pm := render.NewPixmap(200, 200)

	probe := &legProbe{}
	if err := render.RegisterAccelerator(probe); err != nil {
		t.Fatalf("RegisterAccelerator: %v", err)
	}
	if err := r.Render(pm, legScene()); err != nil {
		t.Fatalf("gpu frame: %v", err)
	}
	if gpu := r.Stats(); !gpu.GPULegTaken {
		render.CloseAccelerator()
		t.Fatal("GPU 帧没标记腿")
	}
	render.CloseAccelerator()

	r.MarkAllDirty()
	if err := r.RenderDirty(pm, legScene(), nil); err != nil {
		t.Fatalf("RenderDirty: %v", err)
	}
	st := r.Stats()
	if st.GPULegTaken || st.CPUFallbackOps != 0 || st.GPUOps != 0 {
		t.Fatalf("脏区帧带着上一帧 GPU 计数：GPULegTaken=%v ops=%d/%d",
			st.GPULegTaken, st.GPUOps, st.CPUFallbackOps)
	}
	if st.TilesRendered <= 0 {
		t.Fatalf("脏区帧没记瓦片数：%d", st.TilesRendered)
	}
}
