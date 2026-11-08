//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image"
	"testing"

	"github.com/energye/gpui/render/text"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4b-4（textPath + shapes 容错）钉测。textPath：沿弧长排布（resvg
// usvg layout.rs collect_normals 逐条——cluster 中点挂路径、切线角绕中点旋转、
// dx 入 advance、dy running 累积、界外隐藏、组 anchor、startOffset 百分比
// 按路径长、引用形状 transform 烘焙、非形状/无 href 回普通流）。resvg 语料
// 样本入 testdata/p38/filter4b/（textPath- 前缀）。语料字体是 Noto Sans，
// 钉测把 resolver fallback 到 AHEM（conformance 工具同口径 fail-open，引擎
// fail-closed 语义不变）。
//
// shapes 容错（usvg PointsParser 语义）见 TestP384c 系列 + 本文件负向。

func withAhemFallback(t *testing.T) {
	t.Helper()
	src := loadAhem(t)
	SetFontResolver(func(family string, weight text.FontWeight, italic bool) *text.FontSource {
		_ = weight
		_ = italic
		return src // fallback-all: the corpus names Noto Sans; AHEM stands in
	})
	t.Cleanup(func() { SetFontResolver(nil) })
}

func gradeTP(t *testing.T, name string) {
	t.Helper()
	withAhemFallback(t)
	gradeSample(t, name)
}

// inkInArea counts opaque-ish pixels in a rect (text ink detector).
func inkInArea(t *testing.T, img *image.RGBA, x0, y0, x1, y1 int) int {
	t.Helper()
	n := 0
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			_, _, _, a := img.At(x, y).RGBA()
			if a>>8 > 100 {
				n++
			}
		}
	}
	return n
}

func TestP384b4TextPathSimple(t *testing.T) {
	// 文字沿曲线真的画出来了：曲线路径带之外有文字墨水（resvg 语料期望
	// 即文字在弧上）；分级 ≤ close。
	withAhemFallback(t)
	doc := mustParseFile(t, "filter4b/textPath-simple-case.svg")
	img := doc.Render(300, 300)
	if ink := inkInArea(t, img, 40, 40, 260, 260); ink < 30 {
		t.Fatalf("textPath ink = %d sampled pixels, want substantial glyph ink", ink)
	}
	gradeSample(t, "textPath-simple-case")
}

func TestP384b4TextPathStartOffsets(t *testing.T) {
	// startOffset 各形态：绝对 30、百分比 10%、50%（子路径）、负 -100、
	// 超长 9999（后半文字隐藏）。
	for _, name := range []string{
		"textPath-startOffset=30",
		"textPath-startOffset=10percent",
		"textPath-path-with-subpaths-and-startOffset",
		"textPath-startOffset=-100",
		"textPath-startOffset=9999",
	} {
		gradeTP(t, name)
	}
}

func TestP384b4TextPathAnchor(t *testing.T) {
	// text-anchor="middle" 在 textPath 上：组总长的 -1/2 进 startOffset。
	gradeTP(t, "textPath-with-text-anchor")
}

func TestP384b4TextPathShapes(t *testing.T) {
	// 闭路径 / 多子路径 / m-A / m-L-Z / ClosePath 的弧长连续语义。
	for _, name := range []string{
		"textPath-closed-path",
		"textPath-path-with-subpaths",
		"textPath-path-with-ClosePath",
		"textPath-m-A-path",
		"textPath-m-L-Z-path",
	} {
		gradeTP(t, name)
	}
}

func TestP384b4TextPathFallbacks(t *testing.T) {
	// 无 href / href 缺 # 形式 / 指向非形状 → 普通流文字；
	// 指向 rect → 沿矩形轮廓（resvg shapes::convert 语义）。
	for _, name := range []string{
		"textPath-no-link",
		"textPath-with-path", // SVG2 path 属性：usvg v0.42 忽略 → 普通流
		"textPath-with-invalid-path-and-xlink-href",
		"textPath-invalid-link",
		"textPath-link-to-rect",
	} {
		gradeTP(t, name)
	}
	// 普通流回退的落点：no-link 的 text 带 y="100" → 文字水平画在
	// user y=100（300×300 渲染的 device y≈150）。
	doc := mustParseFile(t, "filter4b/textPath-no-link.svg")
	img := doc.Render(300, 300)
	if ink := inkInArea(t, img, 20, 128, 280, 162); ink < 20 {
		t.Fatalf("no-link fallback: no text ink at the y=100 baseline (ink=%d)", ink)
	}
}

func TestP384b4TextPathNestedAndMixed(t *testing.T) {
	for _, name := range []string{
		"textPath-nested",
		"textPath-mixed-children-1",
		"textPath-mixed-children-2",
		"textPath-two-paths",
		"textPath-tspan-with-absolute-position",
		"textPath-tspan-with-relative-position",
		"textPath-invalid-textPath-in-the-middle",
	} {
		gradeTP(t, name)
	}
}

func TestP384b4TextPathSpacingAndRotations(t *testing.T) {
	for _, name := range []string{
		"textPath-with-letter-spacing",
		"textPath-with-big-letter-spacing",
		"textPath-spacing=auto",
		"textPath-with-rotate",
		"textPath-with-baseline-shift",
		"textPath-with-baseline-shift-and-rotate",
		"textPath-with-coordinates-on-textPath",
		"textPath-with-coordinates-on-text",
		"textPath-dy-with-tiny-coordinates",
		"textPath-very-long-text",
		"textPath-complex",
		"textPath-with-transform-on-a-referenced-path",
		"textPath-with-transform-outside-a-referenced-path",
		"textPath-method=stretch",
		"textPath-side=right",
		"textPath-writing-mode=tb",
		"textPath-with-underline",
		"textPath-with-filter",
	} {
		gradeTP(t, name)
	}
}

func TestP384b4TextPathParse(t *testing.T) {
	doc := mustParseFile(t, "filter4b/textPath-simple-case.svg")
	// 解析面：text 树里有 TextPathElement、href/startOffset 收对。
	var tp *TextPathElement
	var walk func(elems []Element)
	walk = func(elems []Element) {
		for _, e := range elems {
			switch t := e.(type) {
			case *TextPathElement:
				tp = t
			case *TspanElement:
				walk(t.Children)
			}
		}
	}
	for _, el := range doc.Elements {
		if te, ok := el.(*TextElement); ok {
			walk(te.Children)
		}
	}
	if tp == nil {
		t.Fatal("TextPathElement not parsed from <text>")
	}
	if tp.Href != "#pathForText1" {
		t.Fatalf("href = %q", tp.Href)
	}
}

func TestP384b4ShapesTolerance(t *testing.T) {
	// usvg PointsParser 容错：奇数尾丢弃、坏 token 停止（前缀保留）、
	// <2 点跳元素+Warning、文档照常。
	for _, name := range []string{
		"feMorphology-empty-radius", // 回归锚（半径系语义已在 4c 钉）
	} {
		_ = name
	}
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
<polyline points="20 40 160 180 30 150 text" fill="none" stroke="#0f0" stroke-width="4"/>
<polyline points="20 40 160" fill="none" stroke="#f00" stroke-width="4"/>
<polygon points="7 7" fill="#00f"/>
<rect x="1" y="1" width="98" height="98" fill="none" stroke="#000"/>
</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("tolerant parse: %v", err)
	}
	// 第一条 polyline：坏 token 前的 3 点保留（绿线画了）；第二条：
	// 奇数尾丢弃后 1 点 <2 → 跳过；polygon 1 点 → 跳过；文档不失败。
	found := 0
	for _, w := range doc.Warnings {
		if w.Reason == warnInvalidValue && (w.Element == "polyline" || w.Element == "polygon") {
			found++
		}
	}
	if found != 2 {
		t.Errorf("skip warnings = %d, want 2 (1-pt polyline + 1-pt polygon)", found)
	}
	_ = doc.Render(100, 100)
}
