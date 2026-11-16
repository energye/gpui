// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//---------------------------------------------------------------------------
//
// P6 阶段 0 客观门禁（层消失 damage）：cached 层从树里消失（或换 cache key）
// 时，它最后一帧的绘制 footprint 必须上报为 damage，否则持久合成面的剪刀
// 永远不重画那块旧位置 → 残影/叠印（P1 icon 标签乱码根因）。
//
// Flutter `DiffContext` 对比 prev/current 层树天然覆盖层消失；gpui 逐层事件
// 累加模型必须在 BeginFrame 对"本帧不再 live 的 entry"补报旧 footprint。

package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// setupBlittedEntry 建一个已经 blit 过（lastBlit 有效）的 cached entry。
func setupBlittedEntry(t *testing.T, dc *render.Context, tex *PictureTextureCache, id uint64, b image.Rectangle) {
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
		t.Fatalf("entry %d missing", id)
	}
	e.slots[e.contentSlot].view = fakeLiveView()
	tex.mu.Unlock()
	tex.SetLiveKeys([]uint64{id})
	// 一次 blit 建立 lastBlit（无 CTM 变换时 == b）。
	dc.ResetFrameDamage()
	if !tex.blit(id) {
		t.Fatalf("blit(%d) failed", id)
	}
}

// TestDisappearedLayerDamagesOldFootprint：层上一帧还在（blit 过），本帧
// 从 liveKeys 消失。BeginFrame 必须把它的 lastBlit 上报为 damage。
func TestDisappearedLayerDamagesOldFootprint(t *testing.T) {
	dc := render.NewContext(200, 100)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	b := image.Rect(20, 10, 60, 30)
	setupBlittedEntry(t, dc, tex, 77, b)

	// 本帧 key=77 离开树（其它层仍在，liveKeys 非空——生产中的真实场景）。
	tex.SetLiveKeys([]uint64{999})
	dc.ResetFrameDamage()
	tex.BeginFrame()

	damage := dc.FrameDamage()
	// lastBlit 是实际绘制 quad（含 1px AA 外扩），断言 damage 覆盖旧 bounds b。
	covered := false
	for _, r := range damage {
		if r.Min.X <= b.Min.X && r.Min.Y <= b.Min.Y && r.Max.X >= b.Max.X && r.Max.Y >= b.Max.Y {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("disappeared layer old footprint %v not covered by damage %v: stale pixels survive scissor", b, damage)
	}
}

// TestDisappearedLayerDamageOnce：上报一次后就不再重复上报（层已不在树里，
// 后续帧不该永久带着这块 damage），但层若回来仍能重新 blit 建立新的 footprint。
func TestDisappearedLayerDamageOnce(t *testing.T) {
	dc := render.NewContext(200, 100)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	b := image.Rect(20, 10, 60, 30)
	setupBlittedEntry(t, dc, tex, 88, b)

	tex.SetLiveKeys([]uint64{999})
	dc.ResetFrameDamage()
	tex.BeginFrame()
	if len(dc.FrameDamage()) == 0 {
		t.Fatal("first disappeared frame must report old footprint")
	}

	dc.ResetFrameDamage()
	tex.BeginFrame()
	if len(dc.FrameDamage()) != 0 {
		t.Fatalf("second frame must not re-report disappeared footprint, got %v", dc.FrameDamage())
	}
}
