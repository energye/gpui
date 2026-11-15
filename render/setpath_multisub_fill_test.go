//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build !nogpu

package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// TestSetPathMultiSubpathFill_GPUPixels pins the documented SetPath flow with
// a multi-subpath path (ParseSVGPath results are multi-subpath: any SVG
// `<path d="M…M…">`): SetPath + Fill must render the outer band and keep the
// opposite-wound inner subpath as a hole, pixel-equal to the same geometry
// built through direct Context.MoveTo verbs (CPU rasterizer parity already
// proven — see TestSetPathMultiSubpathFill_CPUPixels).
//
// 2026-11-15 O13 批 1b 建窗期发现（先红钉子）：GPU 模式下该填充 op 入队
// （PendingGPUCount 0→1、Fill 无错误）但画面为空；CPU 光栅同一路径完美出环。
// 修复后本测试转绿；修复面在 render/internal/gpu 的 FillPath 链。
func TestSetPathMultiSubpathFill_GPUPixels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()

	// 先铺白底（对照洞的「空」与填充的「紫」都建立在白底上）。
	dc.ClearWithColor(render.White)
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("base present: %v", err)
	}

	// 多子路路径（外矩形 CW + 内矩形 CCW = 环）按文档流程走 SetPath + Fill。
	multi := render.NewPath()
	multi.MoveTo(20, 20)
	multi.LineTo(140, 20)
	multi.LineTo(140, 90)
	multi.LineTo(20, 90)
	multi.Close()
	multi.MoveTo(50, 45)
	multi.LineTo(50, 85)
	multi.LineTo(110, 85)
	multi.LineTo(110, 45)
	multi.Close()
	dc.BeginGPUFrame()
	dc.SetPath(multi)
	dc.SetRGBA(0.6, 0.3, 0.85, 1)
	if err := dc.Fill(); err != nil {
		t.Fatalf("SetPath multi-subpath Fill: %v", err)
	}
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("present: %v", err)
	}
	img := dc.Image()
	if img == nil {
		t.Fatal("readback nil")
	}
	// 外带必须涂上紫（当前洞：alpha 0 / 白底）。
	r, g, b, a := img.At(30, 50).RGBA()
	if a>>8 == 0 || r>>8 < 120 || b>>8 < 160 {
		t.Fatalf("SetPath 多子路填充外带未落地：got=(%d,%d,%d,a%d) want≈紫(153,76,217)",
			r>>8, g>>8, b>>8, a>>8)
	}
	// 内子路对绕 = 洞（保持白底）。
	r2, g2, b2, a2 := img.At(80, 65).RGBA()
	isWhite := r2>>8 == 255 && g2>>8 == 255 && b2>>8 == 255
	if a2>>8 != 0 && !isWhite {
		t.Fatalf("对绕内子路应成洞（白底）: got=(%d,%d,%d,a%d)", r2>>8, g2>>8, b2>>8, a2>>8)
	}

	// 对照腿：同一几何走 Context.MoveTo 直接建（GPU 已证可用形态），外带同色。
	dc.BeginGPUFrame()
	dc.ClearWithColor(render.White)
	dc.MoveTo(20, 20)
	dc.LineTo(140, 20)
	dc.LineTo(140, 90)
	dc.LineTo(20, 90)
	dc.ClosePath()
	dc.MoveTo(50, 45)
	dc.LineTo(50, 85)
	dc.LineTo(110, 85)
	dc.LineTo(110, 45)
	dc.ClosePath()
	dc.SetRGBA(0.6, 0.3, 0.85, 1)
	if err := dc.Fill(); err != nil {
		t.Fatalf("direct MoveTo multi-subpath Fill: %v", err)
	}
	if err := dc.PresentFrame(view, w, h, nil); err != nil {
		t.Fatalf("present2: %v", err)
	}
	img2 := dc.Image()
	r3, g3, b3, _ := img2.At(30, 50).RGBA()
	if r3>>8 < 120 || b3>>8 < 160 {
		t.Fatalf("对照腿（直接 MoveTo）外带未落地：got=(%d,%d,%d)", r3>>8, g3>>8, b3>>8)
	}
}

// TestSetPathMultiSubpathFill_CPUPixels pins CPU-rasterizer parity for the
// same geometry: the CPU path renders the ring correctly, which is the oracle
// the GPU leg must match.
func TestSetPathMultiSubpathFill_CPUPixels(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	const w, h = 320, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	multi := render.NewPath()
	multi.MoveTo(20, 20)
	multi.LineTo(140, 20)
	multi.LineTo(140, 90)
	multi.LineTo(20, 90)
	multi.Close()
	multi.MoveTo(50, 45)
	multi.LineTo(50, 85)
	multi.LineTo(110, 85)
	multi.LineTo(110, 45)
	multi.Close()
	dc.SetPath(multi)
	dc.SetRGBA(0.6, 0.3, 0.85, 1)
	if err := dc.Fill(); err != nil {
		t.Fatalf("CPU Fill: %v", err)
	}
	img := dc.Image()
	r, g, b, _ := img.At(30, 50).RGBA()
	if r>>8 < 120 || b>>8 < 160 {
		t.Fatalf("CPU 外带未落地：got=(%d,%d,%d)", r>>8, g>>8, b>>8)
	}
	r2, g2, b2, a2 := img.At(80, 65).RGBA()
	isWhite := r2>>8 == 255 && g2>>8 == 255 && b2>>8 == 255
	if a2>>8 != 0 && !isWhite {
		t.Fatalf("CPU 对绕内子路应成洞: got=(%d,%d,%d,a%d)", r2>>8, g2>>8, b2>>8, a2>>8)
	}
	_ = image.Rect
}
