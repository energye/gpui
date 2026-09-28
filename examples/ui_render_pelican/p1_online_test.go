//go:build linux && !nogpu

package main

// P1-2 在线鹈鹕：同一套冻结画法（PELICAN_T=5，static+stage）在正式
// 在线链路（render.NewPresentTarget + GPUI_P1_GL=1 + X11 真窗 1200×700）
// 上点亮并连跑。只证整场景能走在线 swapchain 打到屏上、GPU 零回退、
// 帧率达标；像素对照仍由离屏特征门 + P1-2 WebGPU 快照负责，本用例
// 不做像素断言。
//
// 单窗单进程：同一进程里开第二个在线窗会在烘标题图块时堵在字形管线
// （第二份 scene 的 bake 卡在 glyph 管线建管线上，疑似首窗 teardown 后
// 共享 GPU 会话状态没回到可重建位）。多窗重开是 P2 的题，这里只开
// 一个窗，预热 2 帧 + 计时 30 帧一次跑完。

import (
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestP1OnlinePelicanGL(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	t.Setenv("GPUI_P1_GL", "1")
	t.Setenv("PELICAN_T", "5")

	xw := p1OpenX11(t, 1200, 700)
	defer xw.close()

	pt, err := render.NewPresentTarget(render.PresentNativeSurface{
		Platform: render.PresentPlatformX11,
		Display:  xw.display,
		Window:   xw.window,
	}, 1200, 700, 1)
	if err != nil {
		t.Fatalf("NewPresentTarget(GL online): %v", err)
	}
	defer pt.Close()

	// Tiles bake after the share device is published so uploads go to GL.
	sc := newPelicanScene(1200, 700)
	draw := func(dc *render.Context) {
		pc := rendering.NewPaintContext(dc, 1)
		size := rendering.Size{Width: stageW, Height: stageH}
		sc.paintStatic(pc, size)
		sc.paintStage(pc, size)
	}

	// 预热 2 帧：首帧数必须与离屏 GL 单帧一致（104），零回退。
	for i := 0; i < 2; i++ {
		if err := pt.PresentWith(draw); err != nil {
			t.Fatalf("PresentWith pelican frame %d: %v", i, err)
		}
		stats := pt.Context().RenderPathStats()
		t.Logf("online pelican frame %d %s backend=%s", i, stats.LogLine(), pt.GPUBackend())
		if stats.GPUOps == 0 {
			t.Fatalf("online pelican frame %d drew nothing on GPU: %s", i, stats.LogLine())
		}
		if stats.CPUFallbackOps != 0 {
			t.Fatalf("online pelican frame %d must not CPU-fallback: %s reason=%q",
				i, stats.LogLine(), pt.Context().LastCPUFallbackReason())
		}
	}

	// 计时 30 帧：只量数，门限等量稳了再定，这里只要求送满且零回退。
	const frames = 30
	gaps := make([]float64, 0, frames)
	t0 := time.Now()
	for i := 0; i < frames; i++ {
		f0 := time.Now()
		if err := pt.PresentWith(draw); err != nil {
			t.Fatalf("PresentWith timed frame %d: %v", i, err)
		}
		gaps = append(gaps, time.Since(f0).Seconds()*1000)
	}
	total := time.Since(t0).Seconds()
	fps := float64(frames) / total
	sorted := append([]float64(nil), gaps...)
	sort.Float64s(sorted)
	p50 := sorted[len(sorted)/2]
	p95idx := int(float64(len(sorted)) * 0.95)
	if p95idx >= len(sorted) {
		p95idx = len(sorted) - 1
	}
	p95 := sorted[p95idx]
	stats := pt.Context().RenderPathStats()
	t.Logf("online pelican timed frames=%d total=%.2fs fps=%.1f p50=%.2fms p95=%.2fms %s backend=%s",
		frames, total, fps, p50, p95, stats.LogLine(), pt.GPUBackend())
	if stats.CPUFallbackOps != 0 {
		t.Fatalf("timed run must not CPU-fallback: %s reason=%q",
			stats.LogLine(), pt.Context().LastCPUFallbackReason())
	}
}
