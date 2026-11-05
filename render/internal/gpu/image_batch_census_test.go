//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// P3.7 批 6 ①：合批账本的算术侧（不需设备）。透视/材质两条臂各自记
// 「draw 次数」与「最长一段吃几张」，累计到读口复位为止。

func call(cmds int, persp, material bool) imageDrawCall {
	c := imageDrawCall{vertexCount: uint32(cmds) * 6}
	c.cmdCount = uint32(cmds)
	c.perspective = persp
	c.material = material
	return c
}

func TestP376ImageBatchCensusPerArm(t *testing.T) {
	var c imageBatchCensus
	// 一帧：透视段 3 张合成 1 次 draw、普通 1 张、材质段 2 张
	c.note([]imageDrawCall{call(3, true, false), call(1, false, false), call(2, false, true)}, 6)
	got := c.snapshot()
	if got.PerspDraws != 1 || got.MaxPerspRun != 3 {
		t.Errorf("透视臂 = %d 次 / 最长 %d 段，want 1/3", got.PerspDraws, got.MaxPerspRun)
	}
	if got.MaterialDraws != 1 || got.MaxMaterialRun != 2 {
		t.Errorf("材质臂 = %d 次 / 最长 %d 段，want 1/2", got.MaterialDraws, got.MaxMaterialRun)
	}
	if got.Draws != 3 || got.Quads != 6 || got.MaxRun != 3 {
		t.Errorf("总账 = %+v，want Draws 3 / Quads 6 / MaxRun 3", got)
	}

	// 累计：第二帧只有一段更长的普通合并，最大段跟着走、draws 继续加
	c.note([]imageDrawCall{call(5, false, false)}, 5)
	got = c.snapshot()
	if got.Draws != 4 || got.Quads != 11 || got.MaxRun != 5 {
		t.Errorf("两帧累计 = %+v，want Draws 4 / Quads 11 / MaxRun 5", got)
	}
	if got.MaxPerspRun != 3 || got.PerspDraws != 1 {
		t.Errorf("透视臂不该被别的臂污染：%+v", got)
	}
}

func TestP376ImageBatchCensusReset(t *testing.T) {
	var c imageBatchCensus
	c.note([]imageDrawCall{call(4, true, false)}, 4)
	prev := c.reset()
	if prev.MaxPerspRun != 4 || prev.PerspDraws != 1 {
		t.Fatalf("复位必须交回旧值：%+v", prev)
	}
	after := c.snapshot()
	if after != (render.ImageBatchStats{}) {
		t.Fatalf("复位后必须是零值账本：%+v", after)
	}
}
