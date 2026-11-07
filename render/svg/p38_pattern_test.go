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

// P3.8 批 2e（SVG pattern）红测。语义口径：patternUnits=userSpaceOnUse（默认）、
// tile 平铺周期、patternTransform 相位、href 继承内容。

func TestP38PatternTilePeriod(t *testing.T) {
	doc := mustParseFile(t, "pattern_tile.svg")
	img := doc.Render(100, 40)
	// 周期钉：x 与 x+4 同色（tile 4px）；(1,1) 红、(3,1) 暗底、(3,3) 蓝。
	if c := img.RGBAAt(1, 1); c.R < 180 {
		t.Errorf("tile (1,1) = %+v, want red", c)
	}
	if c := img.RGBAAt(3, 1); c.R > 100 {
		t.Errorf("tile (3,1) = %+v, want dark", c)
	}
	if c := img.RGBAAt(3, 3); c.B < 150 {
		t.Errorf("tile (3,3) = %+v, want blue", c)
	}
	// 跨周期：与首个 tile 同相位。
	for _, x := range []int{41, 81} {
		if c := img.RGBAAt(x, 1); c.R < 180 {
			t.Errorf("tile period (%d,1) = %+v, want red (period 4)", x, c)
		}
	}
	// 越过 tile 起点周期 (37,1) 也红（负相位 wrap）。
	if c := img.RGBAAt(37, 1); c.R < 180 {
		t.Errorf("tile period (37,1) = %+v, want red", c)
	}
}

func TestP38PatternHrefPhase(t *testing.T) {
	doc := mustParseFile(t, "pattern_tile.svg")
	img := doc.Render(100, 40)
	// shifted = chk + translate(2 0)：采样经逆映射 q=(x−2,y) ——
	// (1,35)→tile(3,3) 蓝、(3,35)→tile(1,3) 暗底、(5,35)→tile(3,3) 蓝。
	if c := img.RGBAAt(1, 35); c.B < 150 {
		t.Errorf("shifted (1,35) = %+v, want blue (tile 3,3)", c)
	}
	if c := img.RGBAAt(3, 35); diff(float64(c.R), 23) > 12 {
		t.Errorf("shifted (3,35) = %+v, want dark #171a20", c)
	}
	if c := img.RGBAAt(5, 35); c.B < 150 {
		t.Errorf("shifted (5,35) = %+v, want blue (period)", c)
	}
}

func TestP38PatternObbUnits(t *testing.T) {
	// objectBoundingBox units：tile 相对被填元素 bbox。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="60" height="60" viewBox="0 0 60 60">
	  <defs>
	    <pattern id="ob" patternUnits="objectBoundingBox" width="0.5" height="0.5">
	      <rect x="0" y="0" width="20" height="20" fill="#3ccf7a"/>
	    </pattern>
	  </defs>
	  <rect x="10" y="10" width="40" height="40" fill="url(#ob)"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(60, 60)
	// 2×2 tiles over the rect: tile 尺寸 20×20，全绿填充。
	if c := img.RGBAAt(15, 15); c.G < 150 {
		t.Errorf("obb tile (15,15) = %+v, want green", c)
	}
	if c := img.RGBAAt(45, 45); c.G < 150 {
		t.Errorf("obb tile (45,45) = %+v, want green", c)
	}
	// rect 外透明。
	if a := img.RGBAAt(5, 5).A; a != 0 {
		t.Errorf("outside rect = %+v, want transparent", img.RGBAAt(5, 5))
	}
}

func TestP38PatternMissingRefused(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
	  <rect x="0" y="0" width="40" height="40" fill="url(#no-pattern)"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(40, 40)
	if a := img.RGBAAt(20, 20).A; a != 0 {
		t.Errorf("missing pattern = %+v, want unpainted", img.RGBAAt(20, 20))
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnresolvedPaint {
			found = true
		}
	}
	if !found {
		t.Errorf("missing pattern warning missing: %+v", doc.Warnings)
	}
}
