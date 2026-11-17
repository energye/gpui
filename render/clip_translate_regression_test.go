//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "testing"

// 轴对齐判定必须只看线性部分（B/D 斜切项），平移（C/F）不得触发降级。
// Skia 标准：SkMatrix::isScaleTranslate() = type 里除 scale|translate 外无
// 其它位（skew/rotate/persp），平移是允许的。曾经的检查写成 B||C，把
// x 平移当成斜切，导致视口（画布 Translate + 内容本地原点）里的圆角裁剪
// 被降级成直角 quad 路径 —— 圆角半径整块丢失（ui_wr_record_coverage 实测
// 圆角块角落仍是填充色）。本测试用最小画布平移固定该语义。

// TestClipRect_TranslateStaysRect: a translated rect clip must stay a
// rectangular clip (hardware scissor), not degrade to a path clip.
func TestClipRect_TranslateStaysRect(t *testing.T) {
	dc := NewContext(200, 200)
	defer dc.Close()
	dc.BeginFrame()
	dc.Translate(40, 30)
	dc.ClipRect(0, 0, 100, 100)
	defer dc.Pop()
	if dc.clipStack == nil || !dc.clipStack.IsRectOnly() {
		t.Fatal("translated rect clip degraded to a non-rect clip (path/mask)")
	}
}

// TestClipRoundRect_TranslateKeepsRadius: a translated rounded-rect clip must
// keep its radius (analytic RRect + scissor), and the corner pixels must stay
// outside the fill. Red before the fix: translated rrect became a quad path,
// the top-left corner painted solid.
func TestClipRoundRect_TranslateKeepsRadius(t *testing.T) {
	dc := NewContext(200, 200)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(White)
	dc.Translate(40, 30)
	dc.ClipRoundRect(0, 0, 100, 100, 24)
	defer dc.Pop()
	if dc.clipStack == nil || !dc.clipStack.IsRRectOnly() {
		t.Fatal("translated rrect clip degraded to a path clip (radius would be lost)")
	}
	if _, r, ok := dc.clipStack.RRectBounds(); !ok || r != 24 {
		t.Fatalf("RRectBounds radius=%v ok=%v want 24/true", r, ok)
	}
	dc.SetRGBA(0.9, 0.15, 0.15, 1)
	dc.DrawRectangle(0, 0, 100, 100)
	_ = dc.Fill()
	img := dc.Image()
	// Corner (device 42,32) is outside the radius-24 arc -> white background.
	if r, g, b, _ := img.At(42, 32).RGBA(); r>>8 < 0xF0 || g>>8 < 0xF0 || b>>8 < 0xF0 {
		t.Fatalf("translated rrect corner=(%d,%d,%d) want white (radius applied)", r>>8, g>>8, b>>8)
	}
	// Center (device 90,80) is inside -> fill color.
	if r, g, b, _ := img.At(90, 80).RGBA(); r>>8 < 0xC0 || g>>8 > 0x40 || b>>8 > 0x40 {
		t.Fatalf("translated rrect center=(%d,%d,%d) want red", r>>8, g>>8, b>>8)
	}
}
