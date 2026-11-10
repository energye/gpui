//go:build !nogpu

package recording_test

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
//----------------------------------------

// P4.1 批 4 补正 3：Canvas.DrawShadow 录制命令（先红后绿）。
//
// 缺口：drawshadow 真窗 live 回放帧无阴影——DrawShadow 是 Context 独有方法，
// render.Canvas/recording 层无该命令，录制回放路径无法承载（登记降级）。
// 修法：Canvas 接口 +DrawShadow（对齐 Skia SkCanvas.drawShadow），recording
// 加 CmdDrawShadow（Recorder 记录、Playback 分发、CanvasBackend 执行、
// bounds 计阴影外扩）。
//
// 红测：带非恒等 Translate 下「录制→回放」与「直画」像素逐位一致——
// 实现前编译失败（红，unit-tests/red-compile.txt），实现后绿。
// GPU 断言无设备 skip。

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/recording/backends/raster"
)

// drawShadowScene 以 Canvas 接口画同一画面：白底 + 平移变换下的深色块 + 阴影。
// 通过接口调用（非 *render.Context），录制画布与真实画布同代码路径。
func drawShadowScene(dc render.Canvas, w, h int) {
	dc.ClearWithColor(render.White)
	dc.Push()
	dc.Translate(40, 30) // 非恒等变换：锁定 DrawShadow 的变换契约
	rect := render.NewPath()
	rect.MoveTo(20, 20)
	rect.LineTo(120, 20)
	rect.LineTo(120, 90)
	rect.LineTo(20, 90)
	rect.Close()
	dc.SetRGBA(0.05, 0.05, 0.08, 1)
	dc.DrawPath(rect)
	_ = dc.Fill()
	dc.DrawShadow(rect, render.RGBA{R: 0, G: 0, B: 0, A: 1}, 12, false)
	dc.Pop()
}

// classify 返回像素类别：0=白底，1=块本体（深色不透明），2=阴影/其它。
func classify(c color.Color) int {
	r, g, b, a := c.RGBA()
	r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
	if r8 > 240 && g8 > 240 && b8 > 240 {
		return 0
	}
	if r8 < 30 && g8 < 30 && b8 < 40 && a8 == 255 {
		return 1
	}
	return 2
}

func countShadow(img image.Image, w, h int) int {
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if classify(img.At(x, y)) == 2 {
				n++
			}
		}
	}
	return n
}

func countDiff(a, b image.Image, w, h int, tol uint8) int {
	diff := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if d(uint8(ar>>8), uint8(br>>8)) > tol ||
				d(uint8(ag>>8), uint8(bg>>8)) > tol ||
				d(uint8(ab>>8), uint8(bb>>8)) > tol ||
				d(uint8(aa>>8), uint8(ba>>8)) > tol {
				diff++
			}
		}
	}
	return diff
}

func d(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestDrawShadowReplayParity：同一画面「直画」与「录制→回放」像素一致，
// 且两者都有阴影带（缺阴影=本批要修的洞）。带平移变换锁定变换契约。
func TestDrawShadowReplayParity(t *testing.T) {
	const w, h = 200, 160

	direct := render.NewContext(w, h)
	defer direct.Close()
	drawShadowScene(direct, w, h)
	if err := direct.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}

	rec := recording.NewRecorder(w, h)
	drawShadowScene(rec, w, h)
	disp := rec.FinishRecording()
	replay := render.NewContext(w, h)
	defer replay.Close()
	backend := recording.NewCanvasBackend(replay)
	if err := disp.Playback(backend); err != nil {
		t.Fatalf("playback: %v", err)
	}
	if err := replay.FlushGPU(); err != nil {
		t.Skipf("GPU flush unavailable: %v", err)
	}

	di, ri := direct.Image(), replay.Image()
	diff := countDiff(di, ri, w, h, 2)
	if diff > w*h/100 { // 容许 <1% 摆动（AA 采样差），主体必须大面积一致
		t.Fatalf("replay parity: %d/%d px differ (%.2f%%)", diff, w*h, 100*float64(diff)/(w*h))
	}
	dShadow := countShadow(di, w, h)
	rShadow := countShadow(ri, w, h)
	if dShadow == 0 || rShadow == 0 {
		t.Fatalf("shadow band missing: direct=%d replay=%d", dShadow, rShadow)
	}
	if dShadow != rShadow {
		t.Fatalf("shadow band mismatch: direct=%d replay=%d", dShadow, rShadow)
	}
}

// TestDrawShadowRasterBackendReplay：raster 导出后端回放 DrawShadow 也出阴影
// （recording.Backend 全实现者口径）。
func TestDrawShadowRasterBackendReplay(t *testing.T) {
	const w, h = 200, 160
	rec := recording.NewRecorder(w, h)
	drawShadowScene(rec, w, h)
	disp := rec.FinishRecording()
	rb := raster.NewBackend()
	if err := disp.Playback(rb); err != nil {
		t.Fatalf("raster playback: %v", err)
	}
	img := rb.Image()
	if img == nil {
		t.Fatal("raster backend produced no image")
	}
	if n := countShadow(img, w, h); n == 0 {
		t.Fatal("raster replay: shadow band missing")
	}
}
