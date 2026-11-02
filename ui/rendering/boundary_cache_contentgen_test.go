//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering_test

import (
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// 内容世代与重绘边界指纹（P3.6 批 1d D18 的实测结论）。
//
// 动图推进一帧走的是「同一块 ImageBuf 原地换代际」：缓冲指针不变、像素变、
// 图号变，且没有人给渲染节点标脏。这里的钉锁的是两件事同时成立：
//
//  1. 画面必须是最新一帧——录制下来的图像命令存的是缓冲指针，重放时按当前像素
//     取图，所以烘焙不失效、画面也不陈旧；
//  2. 这一帧仍然走重放（skip+1、rerecord 0）——把图号并进内容指纹会让每个动画
//     帧作废重录，实测 200 子节点的边界 12.84ms/帧 → 13.66ms/帧（+6.4%），
//     换来的正确性收益为零（见 1）。
//
// 也就是说动图的失效通路是「推进帧时标脏」，不是「靠指纹发现像素变了」。
// 如果哪天录制改成存像素快照，第 1 条会先红，正好把改法顶回来。
func TestRenderImage_InPlaceFrameAdvanceReplaysFreshFrame(t *testing.T) {
	const W, H = 80, 80

	buf, err := render.NewImageBuf(40, 40, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer buf.Dispose()
	buf.Fill(220, 30, 30, 255)
	buf.NotifyPixelsChanged()
	gen1 := buf.GenerationID()
	if gen1 == 0 {
		t.Fatal("precondition: NotifyPixelsChanged must retire the generation")
	}

	child := rendering.NewRenderImage(40, 40)
	child.SetImageShared(buf)

	box := rendering.NewAbsoluteBox(60, 60)
	box.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	box.SetRepaintBoundary(true)
	box.Place(child, 10, 10)

	root := rendering.NewAbsoluteBox(W, H)
	root.Background = &rendering.Color{R: 0, G: 0, B: 0, A: 1}
	root.Place(box, 5, 5)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: W, Height: H}, true)
	cache := owner.BoundaryCache()
	if cache == nil {
		t.Fatal("PipelineOwner must carry a BoundaryCache")
	}

	// 画布坐标：box@(5,5) 60×60，child 再 +10 → (15,15) 40×40，中心 (35,35)。
	paint := func() (rerecord, skip int64, px color.RGBA) {
		t.Helper()
		dc := render.NewContext(W, H)
		dc.BeginFrame()
		dc.ClearWithColor(render.White)
		pc := rendering.NewPaintContext(dc, 1)
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
		root.Paint(pc)
		st := cache.Stats()
		r, g, b, a := dc.Image().At(35, 35).RGBA()
		return st.FrameRerecord, st.FrameSkip, rgba8From16(r, g, b, a)
	}

	// 帧 1：冷缓存 → 录制，画面为第一帧（红）。
	rr1, sk1, c1 := paint()
	if rr1 < 1 || sk1 != 0 {
		t.Fatalf("frame1 rerecord=%d skip=%d want rerecord≥1, skip=0 (cold cache)", rr1, sk1)
	}
	if !isRed(c1) {
		t.Fatalf("frame1 pixel=(35,35) %#v want the first (red) frame", c1)
	}

	// 帧 2：同一块缓冲原地推进一帧，全程不标脏。
	buf.Fill(30, 200, 40, 255)
	buf.NotifyPixelsChanged()
	if buf.GenerationID() == gen1 {
		t.Fatal("frame advance must produce a new generation")
	}
	if child.NeedsPaint() || box.NeedsPaint() {
		t.Fatal("precondition: an in-place pixel write must not dirty render objects")
	}
	rr2, sk2, c2 := paint()
	if !isGreen(c2) {
		t.Errorf("frame2 pixel=(35,35) %#v want the new (green) frame: replay must follow the live buffer", c2)
	}
	if rr2 != 0 || sk2 < 1 {
		t.Errorf("frame2 rerecord=%d skip=%d want rerecord=0, skip≥1 (the bake must survive an in-place advance)", rr2, sk2)
	}

	// 帧 3：换成另一块缓冲（SetImageShared 标脏）→ 必须重录并显示新图。
	buf2, err := render.NewImageBuf(40, 40, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	defer buf2.Dispose()
	buf2.Fill(40, 60, 230, 255)
	buf2.NotifyPixelsChanged()
	child.SetImageShared(buf2)
	if !child.NeedsPaint() {
		t.Fatal("SetImageShared must mark the node for repaint")
	}
	rr3, _, c3 := paint()
	if rr3 < 1 {
		t.Errorf("frame3 rerecord=%d want ≥1: swapping buffers invalidates the bake", rr3)
	}
	if !isBlue(c3) {
		t.Errorf("frame3 pixel=(35,35) %#v want the swapped (blue) image", c3)
	}
}

func rgba8From16(r, g, b, a uint32) color.RGBA {
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func isRed(c color.RGBA) bool   { return c.R > 150 && c.G < 90 && c.B < 90 }
func isGreen(c color.RGBA) bool { return c.G > 150 && c.R < 90 && c.B < 90 }
func isBlue(c color.RGBA) bool  { return c.B > 150 && c.R < 90 && c.G < 90 }
