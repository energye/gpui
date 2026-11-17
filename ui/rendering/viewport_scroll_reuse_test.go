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
	"fmt"
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// 视口内容边界 + 滚动位移复用（ENGINE_NODE_RECORD_COVERAGE_PLAN §6 步 3）。
// 断言口径：
//  1. 滚动帧零重录（FrameRerecord=0，内容层 R7b 命中 skip），像素与同偏移活画逐位一致；
//  2. 内容脏重录后，换任意滚动偏移回放都正确——录制锚点必须与滚动无关
//     （录制时把 scroll 烤进去会让其他偏移整体错位，divider 会话踩过）；
//  3. 超长内容（>8192 逻辑高）回退逐帧活画，不留巨型 picture。
const (
	vrW = 100
	vrH = 60
)

func vrBuild(contentH float64) (*rendering.AbsoluteBox, *rendering.RenderViewport, *rendering.AbsoluteBox) {
	content := rendering.NewAbsoluteBox(vrW, contentH)
	content.Place(rendering.NewRenderColorBox(vrW, 30, 0.9, 0.15, 0.15, 1), 0, 0)
	content.Place(rendering.NewRenderColorBox(vrW, 30, 0.15, 0.35, 0.9, 1), 0, contentH/2)
	content.Place(rendering.NewRenderColorBox(vrW, 30, 0.15, 0.8, 0.25, 1), 0, contentH-30)
	vp := rendering.NewRenderViewport(content)
	vp.FixedWidth, vp.FixedHeight = vrW, vrH
	root := rendering.NewAbsoluteBox(vrW, vrH)
	root.Background = &rendering.Color{R: 1, G: 1, B: 1, A: 1}
	root.Place(vp, 0, 0)
	return root, vp, content
}

// vrPaint renders one frame; useCache=false is the pure live reference.
func vrPaint(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache, useCache bool) image.Image {
	t.Helper()
	dc := render.NewContext(vrW, vrH)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	if useCache {
		pc.BoundaryCache, pc.UseBoundaryCache = cache, true
		cache.BeginFrame()
	}
	root.Paint(pc)
	return dc.Image()
}

func vrStats(cache *rendering.BoundaryCache) (rerecord, skip int64) {
	st := cache.Stats()
	return st.FrameRerecord, st.FrameSkip
}

func TestViewport_ScrollReuse(t *testing.T) {
	root, vp, content := vrBuild(300)
	if !content.IsRepaintBoundary() {
		t.Fatal("viewport content must be a repaint boundary (cache unit)")
	}
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: vrW, Height: vrH}, true)
	cache := owner.BoundaryCache()

	// Frame 1 (live reference) + frame 2 (records once because no entry yet).
	live0 := vrPaint(t, root, nil, false)
	_ = vrPaint(t, root, cache, true)
	if rr, sk := vrStats(cache); rr != 1 || sk != 0 {
		t.Fatalf("first cached frame rerecord=%d skip=%d want 1/0", rr, sk)
	}
	// Frame 3: clean content replays, zero re-record.
	again := vrPaint(t, root, cache, true)
	if rr, sk := vrStats(cache); rr != 0 || sk != 1 {
		t.Fatalf("clean frame rerecord=%d skip=%d want 0/1", rr, sk)
	}
	rpDiff(t, "Viewport scroll-0 replay", live0, again)

	// Scroll: viewport dirty, content clean -> zero re-record + pixel-correct.
	vp.SetScrollOffset(0, 80)
	scrolled := vrPaint(t, root, cache, true)
	if rr, sk := vrStats(cache); rr != 0 || sk != 1 {
		t.Fatalf("scroll frame rerecord=%d skip=%d want 0/1 (content replay)", rr, sk)
	}
	rpDiff(t, "Viewport scroll-80 replay", vrPaint(t, root, nil, false), scrolled)

	// Dirty content at scroll 80: exactly one re-record, content-local anchor.
	content.MarkNeedsPaint()
	_ = vrPaint(t, root, cache, true)
	if rr, sk := vrStats(cache); rr != 1 || sk != 0 {
		t.Fatalf("dirty content rerecord=%d skip=%d want 1/0", rr, sk)
	}
	// Replay at other scroll offsets: a scroll-baked anchor would shift the
	// whole picture here, not just the expected band.
	for _, off := range []float64{0, 40, 120} {
		vp.SetScrollOffset(0, off)
		got := vrPaint(t, root, cache, true)
		if rr, _ := vrStats(cache); rr != 0 {
			t.Fatalf("scroll %v after re-record: rerecord=%d want 0 (scroll-independent anchor)", off, rr)
		}
		rpDiff(t, fmt.Sprintf("Viewport replay at scroll %v", off), vrPaint(t, root, nil, false), got)
	}
}

func TestViewport_OversizeContentFallsBackLive(t *testing.T) {
	root, _, content := vrBuild(9000)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: vrW, Height: vrH}, true)
	cache := owner.BoundaryCache()

	live := vrPaint(t, root, nil, false)
	first := vrPaint(t, root, cache, true)
	if rr, _ := vrStats(cache); rr != 0 {
		t.Fatalf("oversize content rerecord=%d want 0 (live fallback)", rr)
	}
	if cache.HasValid(content) {
		t.Fatal("oversize content must not hold a recording")
	}
	second := vrPaint(t, root, cache, true)
	rpDiff(t, "Oversize live frame 1", live, first)
	rpDiff(t, "Oversize live frame 2", live, second)
}
