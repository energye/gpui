//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"testing"
)

// P3.8 批 2b（SVG clipPath）红测。语义口径：CSS Masking Level 1 / SkSVGDOM——
// clipPath 子形状并集、单形状保 winding（挖洞成立）、clip-rule 归位到 clipPath
// 内容、objectBoundingBox 相对引用元素 bbox、悬空引用 fail-closed（元素不画）。

func TestP38ClipPathUnion(t *testing.T) {
	doc := mustParseFile(t, "clip_union.svg")
	img := doc.Render(120, 60)
	// 并集区域 = [0,90)：重叠区 [40,50) 仍在并集内（union 不是交集）。
	for _, x := range []int{10, 45, 85} {
		if c := img.RGBAAt(x, 30); c.R < 150 {
			t.Errorf("union (%d,30) = %+v, want painted red", x, c)
		}
	}
	// 并集外两侧透明。
	for _, x := range []int{95, 110} {
		if a := img.RGBAAt(x, 30).A; a != 0 {
			t.Errorf("outside (%d,30) = %+v, want transparent", x, img.RGBAAt(x, 30))
		}
	}
	// 重叠带 [40,50) 是单次绘制（并集裁剪，不是两次重叠叠加——alpha 不叠深）。
	// （同一元素裁剪一次，天然满足；此点由 clip 语义保证。）
}

func TestP38ClipPathWindingHole(t *testing.T) {
	doc := mustParseFile(t, "clip_hole.svg")
	img := doc.Render(100, 100)
	// 环带内可见（单形状双 subpath，非零绕组挖洞）。
	if c := img.RGBAAt(20, 50); c.B < 150 {
		t.Errorf("ring band (20,50) = %+v, want blue", c)
	}
	// 洞内透明。
	if a := img.RGBAAt(50, 50).A; a != 0 {
		t.Errorf("hole (50,50) = %+v, want transparent", img.RGBAAt(50, 50))
	}
}

func TestP38ClipPathObjectBoundingBox(t *testing.T) {
	doc := mustParseFile(t, "clip_obb.svg")
	img := doc.Render(200, 100)
	// 两个不同尺寸 rect，同一 0..0.5 oBB 裁剪：各自左半可见。
	for _, pt := range [][2]int{{30, 50}, {135, 50}} {
		if c := img.RGBAAt(pt[0], pt[1]); c.R < 100 && c.G < 100 {
			t.Errorf("obb inside (%d,%d) = %+v, want painted", pt[0], pt[1], c)
		}
	}
	// 右半透明：rect0 右半 (50,50)、rect1 右半 (170,50)（离开 x=140 裁剪 AA 边）。
	for _, pt := range []int{50, 170} {
		if a := img.RGBAAt(pt, 50).A; a != 0 {
			t.Errorf("obb outside (%d,50) = %+v, want transparent", pt, img.RGBAAt(pt, 50))
		}
	}
}

func TestP38ClipPathOnGroupAndMissing(t *testing.T) {
	doc := mustParseFile(t, "clip_group_missing.svg")
	img := doc.Render(100, 80)
	// g 整体裁剪：上半可见、下半（rect 自身范围）透明。
	if c := img.RGBAAt(50, 20); c.B < 100 {
		t.Errorf("group clip top = %+v, want purple", c)
	}
	if a := img.RGBAAt(50, 50).A; a != 0 {
		t.Errorf("group clip bottom = %+v, want transparent", img.RGBAAt(50, 50))
	}
	// 悬空 clip 引用：元素不画 + Warning（fail-closed）。
	if a := img.RGBAAt(30, 65).A; a != 0 {
		t.Errorf("missing clip ref = %+v, want unpainted", img.RGBAAt(30, 65))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnresolvedRef {
			found = true
		}
	}
	if !found {
		t.Errorf("missing clip warning missing: %+v", doc.Warnings)
	}
	// 对照 rect 仍绿。
	if got := img.RGBAAt(75, 65); got.G < 200 {
		t.Errorf("contrast rect = %+v, want green", got)
	}
}

func TestP38ClipRuleUsedNotFillRule(t *testing.T) {
	// clip-rule 归位：clipPath 内形状的 clip-rule 决定裁剪绕组，
	// 被裁元素自身的 fill-rule 不再被借用（批 1 的借用行为作废）。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="60" height="60" viewBox="0 0 60 60">
	  <defs>
	    <clipPath id="c" clipPathUnits="userSpaceOnUse">
	      <path clip-rule="evenodd" d="M5,5 H55 V55 H5 Z M20,20 H40 V40 H20 Z"/>
	    </clipPath>
	  </defs>
	  <rect x="0" y="0" width="60" height="60" fill="#e24a4a" clip-path="url(#c)"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(60, 60)
	// evenodd 挖洞：环带可见、洞透明。
	if c := img.RGBAAt(10, 30); c.R < 150 {
		t.Errorf("evenodd band = %+v, want red", c)
	}
	if a := img.RGBAAt(30, 30).A; a != 0 {
		t.Errorf("evenodd hole = %+v, want transparent", img.RGBAAt(30, 30))
	}
}
