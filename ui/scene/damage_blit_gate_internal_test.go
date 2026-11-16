// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//---------------------------------------------------------------------------
//
// P6 阶段 0 客观门禁（blit damage 几何）：cached 层 blit 时上报的 damage
// 必须覆盖**实际绘制的纹理 quad**，而不是收紧的记录 bounds。
// Flutter `DiffContext::AddDamage(previous_paint_region)` 用绘制 footprint；
// gpui 若用 e.bounds，旋转 quad 的四角会漏报 → 持久面剪刀留残影（P1 spin）。

package scene

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// makeBlitEntry 用 fake slot 建一个可 blit 的 cached entry（headless 无 GPU
// 也能走完整 blit damage 逻辑，避免"离线探针不 import 真实路径"的假绿）。
func makeBlitEntry(t *testing.T, dc *render.Context, tex *PictureTextureCache, id uint64, b image.Rectangle) {
	t.Helper()
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc
	rec := fillRecording(b)
	tex.BeginFrame()
	if _, ok := tex.recordLocal(id, rec, b); !ok {
		t.Fatalf("recordLocal(%d) failed", id)
	}
	tex.mu.Lock()
	e := tex.entries[id]
	if e == nil || e.contentSlot < 0 {
		tex.mu.Unlock()
		t.Fatalf("entry %d missing after record", id)
	}
	e.slots[e.contentSlot].view = fakeLiveView()
	tex.mu.Unlock()
	if !tex.Has(id) {
		t.Fatalf("Has(%d) false after setup", id)
	}
}

// blitQuadCorners 按 blit() 的真实公式算出绘制 quad 的四角 surface-logical
// 坐标（image.Rect(floor(ox),floor(oy),+s.w,+s.h) 过的 CTM）。
func blitQuadCorners(dc *render.Context, off image.Point, w, h int) []struct{ x, y float64 } {
	q := image.Rect(off.X, off.Y, off.X+w, off.Y+h)
	return transformedCorners(dc, q)
}

// TestBlitDamageCoversDrawnQuadRotated：旋转帧的 blit damage 必须覆盖绘制
// quad 四角。P0 用 e.bounds（收紧 2px）+ int 截断会漏；改成 quad + floor/ceil
// 后转绿。逐角断言，非 mean。
func TestBlitDamageCoversDrawnQuadRotated(t *testing.T) {
	dc := render.NewContext(200, 200)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	b := image.Rect(-18, -18, 18, 18)
	makeBlitEntry(t, dc, tex, 41, b)

	tex.mu.Lock()
	e := tex.entries[41]
	off, sw, sh := e.off, e.slots[e.contentSlot].w, e.slots[e.contentSlot].h
	tex.mu.Unlock()

	// 第一帧：无旋转建立 lastBlit（offset-only 首帧本就不报 damage）。
	dc.ResetFrameDamage()
	tex.blit(41)

	// 第二帧：旋转 45°，重新取 damage。
	dc.BeginFrame()
	dc.Push()
	dc.Translate(100, 100)
	dc.Rotate(math.Pi / 4)
	dc.ResetFrameDamage()
	if !tex.blit(41) {
		t.Fatal("rotated blit must succeed on cached entry")
	}
	damage := dc.FrameDamage()
	dc.Pop()

	if len(damage) == 0 {
		t.Fatal("rotated blit reported no damage; stale pixels would survive scissor")
	}
	// 合并所有 damage 矩形，逐角断言覆盖旋转 quad。
	union := damage[0]
	for _, r := range damage[1:] {
		union = union.Union(r)
	}
	corners := blitQuadCorners(dc, off, sw, sh)
	// Pop 后 CTM 已恢复，需在 Push 状态算——重算：
	dc.Push()
	dc.Translate(100, 100)
	dc.Rotate(math.Pi / 4)
	corners = blitQuadCorners(dc, off, sw, sh)
	dc.Pop()
	assertCoversAllCorners(t, union, corners)
}
