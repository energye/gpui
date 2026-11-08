//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image/color"
	"testing"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4c-0（conformance 真实口径暴露的既有洞修复）。12 far 归因三修：
// ①feMorphology 半径语义（usvg convert_morphology：缺省=primitiveUnits scale、
// 零值回落 1 的 Chrome/Safari quirk、负值整体保持缺省——旧实现「无效→0/0→透明」
// 错）；②feFlood 缺省黑（spec；旧实现回退白）+ flood-color="inherit" 关键字取
// 直接父（filter 元素）值（usvg parse.rs resolve_inherit 非继承属性一层语义）；
// ③hsl()/hsla() 函数式颜色（usvg svgtypes 子集）。resvg 语料样本入 testdata。

func TestP384cMorphologyRadiusSemantics(t *testing.T) {
	// usvg convert_morphology 逐字：len1→both、len2→xy、其他→0；零值→1；
	// 负值任一→双双缺省；结果 × scale（userSpace=1）。
	cases := []struct {
		nums      []float64
		defX, dY  float64
		rx, ry    float64
	}{
		{nil, 1, 1, 1, 1},                // 缺省 = scale
		{[]float64{}, 1, 1, 1, 1},        // 空串 → 回落
		{[]float64{0}, 1, 1, 1, 1},       // 零值回落（Chrome/Safari quirk）
		{[]float64{-5}, 1, 1, 1, 1},      // 负值 → 缺省
		{[]float64{10, 20, 30}, 1, 1, 1, 1}, // 3+ 值 → 0 → 回落
		{[]float64{10}, 1, 1, 10, 10},    // 单值 both
		{[]float64{4, 2}, 1, 1, 4, 2},    // 双值 xy
		{[]float64{0, 5}, 1, 1, 1, 5},    // 单零回落
		{[]float64{4, -2}, 1, 1, 1, 1},   // 负值任一 → 双双缺省
		{[]float64{2}, 30, 20, 60, 40},   // oBB: 值 × bbox
		{nil, 30, 20, 30, 20},            // oBB 缺省 = bbox
	}
	for i, c := range cases {
		rx, ry := morphologyRadii(c.nums, c.defX, c.dY)
		if rx != c.rx || ry != c.ry {
			t.Errorf("case %d: morphologyRadii(%v) = (%v,%v), want (%v,%v)", i, c.nums, rx, ry, c.rx, c.ry)
		}
	}
}

func TestP384cMorphologyRadiusCorpus(t *testing.T) {
	// 五枚无效/零/负 radius 全部落到 erode 半径 1（期望像素互证一致），
	// 不再是旧实现的透明输出。
	for _, name := range []string{
		"feMorphology-empty-radius",
		"feMorphology-negative-radius",
		"feMorphology-no-radius",
		"feMorphology-radius-with-too-many-values",
		"feMorphology-zero-radius",
	} {
		gradeSample(t, name)
	}
	// 语义互锁：empty 与 zero 的输出逐位一致（同为 erode 1）。
	a := mustParseFile(t, "filter4b/feMorphology-empty-radius.svg").Render(200, 200)
	b := mustParseFile(t, "filter4b/feMorphology-zero-radius.svg").Render(200, 200)
	for _, p := range [][2]int{{100, 100}, {50, 50}, {150, 150}, {30, 170}} {
		if a.RGBAAt(p[0], p[1]) != b.RGBAAt(p[0], p[1]) {
			t.Fatalf("empty vs zero radius diverge at %v: %v vs %v", p, a.RGBAAt(p[0], p[1]), b.RGBAAt(p[0], p[1]))
		}
	}
}

func TestP384cFloodDefaultsAndInheritance(t *testing.T) {
	// feFlood 缺省 = 黑（spec default flood-color=black；旧实现回退白）。
	// flood-color 不可继承：filter/g 元素上的值到不了 feFlood。
	for _, name := range []string{
		"feFlood-default-values",
		"flood-color-inheritance-1",
		"flood-color-inheritance-2",
		"flood-color-inheritance-4",
		"flood-color-inheritance-5",
	} {
		doc := mustParseFile(t, "filter4b/"+name+".svg")
		img := doc.Render(200, 200)
		if c := img.RGBAAt(100, 100); c.R != 0 || c.G != 0 || c.B != 0 || c.A != 255 {
			t.Errorf("%s: center = %v, want opaque black", name, c)
		}
		gradeSample(t, name)
	}
	// 显式 flood-color="inherit" 取直接父（filter 元素）的 flood-color。
	doc3 := mustParseFile(t, "filter4b/flood-color-inheritance-3.svg")
	img3 := doc3.Render(200, 200)
	if c := img3.RGBAAt(100, 100); c.G != 128 || c.R != 0 {
		t.Errorf("inheritance-3: center = %v, want green (inherited from filter)", c)
	}
	gradeSample(t, "flood-color-inheritance-3")
}

func TestP384cHSLColor(t *testing.T) {
	// CSS Color 4 hsl()/hsla() 函数式颜色（usvg svgtypes 子集）。
	c, err := parseColor("hsla(225, 100%, 50%, 0.75)")
	if err != nil || c == nil {
		t.Fatalf("parse hsla: %v", err)
	}
	// NRGBA is straight-alpha: assert the decoded fields, not RGBA() (which
	// premultiplies).
	n, ok := c.(color.NRGBA)
	if !ok {
		t.Fatalf("hsla type = %T", c)
	}
	if n.R != 0 || n.G != 64 || n.B != 255 || n.A != 191 {
		t.Fatalf("hsla(225,100%%,50%%,0.75) = (%d,%d,%d,%d), want (0,64,255,191)", n.R, n.G, n.B, n.A)
	}
	// 纯 hsl（无 alpha）与色相归一（负角/超 360）。
	if c2, err2 := parseColor("hsl(120, 100%, 50%)"); err2 != nil {
		t.Fatalf("parse hsl: %v", err2)
	} else if n2, ok2 := c2.(color.NRGBA); !ok2 || n2.R != 0 || n2.G != 255 || n2.B != 0 {
		t.Fatalf("hsl(120,100%%,50%%) = %+v, want (0,255,0)", c2)
	}
	gradeSample(t, "feDropShadow-hsla-color")
}
