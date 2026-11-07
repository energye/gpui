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
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
)

// P3.8 批 1（SVG 文档语义地基）红测。语义口径：Skia modules/svg（SkSVGDOM）行为，
// 依据见 evidence/P3.8/design.md §1。样本一律 testdata/p38/。

// --- pAR：viewportTransform 九值 × {meet, slice} 值级钉（手算矩阵，依据 design §1.1-F1） ---

func TestP38ViewportTransformGrid(t *testing.T) {
	// viewport 100×50，viewBox 0 0 40 40：meet s=1.25（内容 50×50），slice s=2.5（内容 100×100）。
	const vw, vh = 100.0, 50.0
	vb := ViewBox{0, 0, 40, 40}
	const sMeet, sSlice = 1.25, 2.5

	type cell struct {
		align       ParAlign
		meet, slice render.Matrix
	}
	// C=tx（x 平移），F=ty（y 平移）；meet 横向 tx∈{0,25,50}，纵向 ty∈{0,0,0}（内容高=视口高）；
	// slice 横向 tx=0（内容宽=视口宽），纵向 ty∈{0,-25,-25?}——逐格手算见下。
	grid := []cell{
		{ParAlignXMinYMin, m(sMeet, 0, 0), m(sSlice, 0, 0)},
		{ParAlignXMidYMin, m(sMeet, 25, 0), m(sSlice, 0, 0)},
		{ParAlignXMaxYMin, m(sMeet, 50, 0), m(sSlice, 0, 0)},
		{ParAlignXMinYMid, m(sMeet, 0, 0), m(sSlice, 0, -25)},
		{ParAlignXMidYMid, m(sMeet, 25, 0), m(sSlice, 0, -25)},
		{ParAlignXMaxYMid, m(sMeet, 50, 0), m(sSlice, 0, -25)},
		{ParAlignXMinYMax, m(sMeet, 0, 0), m(sSlice, 0, -50)},
		{ParAlignXMidYMax, m(sMeet, 25, 0), m(sSlice, 0, -50)},
		{ParAlignXMaxYMax, m(sMeet, 50, 0), m(sSlice, 0, -50)},
	}
	for _, c := range grid {
		got := viewportTransform(vw, vh, vb, PreserveAspectRatio{Align: c.align, Slice: false})
		want := c.meet
		if got != want {
			t.Errorf("pAR %v meet: viewportTransform = %+v, want %+v", c.align, got, want)
		}
		got = viewportTransform(vw, vh, vb, PreserveAspectRatio{Align: c.align, Slice: true})
		want = c.slice
		if got != want {
			t.Errorf("pAR %v slice: viewportTransform = %+v, want %+v", c.align, got, want)
		}
	}
}

func m(s, c, f float64) render.Matrix { return render.Matrix{A: s, B: 0, C: c, D: 0, E: s, F: f} }

// 非 0 原点 viewBox：C = tx − s·minX，F = ty − s·minY。
func TestP38ViewportTransformNonZeroOrigin(t *testing.T) {
	got := viewportTransform(100, 50, ViewBox{10, 20, 40, 40},
		PreserveAspectRatio{Align: ParAlignXMidYMid, Slice: false})
	want := render.Matrix{A: 1.25, B: 0, C: 25 - 1.25*10, D: 0, E: 1.25, F: 0 - 1.25*20}
	if got != want {
		t.Errorf("viewportTransform(non-zero origin) = %+v, want %+v", got, want)
	}
}

// pAR 属性解析：缺省 xMidYMid meet；九值+两模式全收；非法值回缺省并 Warning。
func TestP38PreserveAspectRatioParse(t *testing.T) {
	if got := parsePreserveAspectRatio(""); got != (PreserveAspectRatio{}) {
		t.Errorf("empty pAR = %+v, want zero value = xMidYMid meet", got)
	}
	if got := parsePreserveAspectRatio("slice"); got.Align != ParAlignXMidYMid || !got.Slice {
		t.Errorf("pAR \"slice\" = %+v, want xMidYMid+slice", got)
	}
	if got := parsePreserveAspectRatio("xMaxYMax"); got.Align != ParAlignXMaxYMax || got.Slice {
		t.Errorf("pAR \"xMaxYMax\" = %+v, want xMaxYMax+meet", got)
	}
	bad := parsePreserveAspectRatio("nonsense")
	if bad.Align != ParAlignXMidYMid || bad.Slice {
		t.Errorf("pAR nonsense = %+v, want default fallback", bad)
	}
}

// --- 根视口裁剪：slice 出界内容必须被裁（样本 par_slice_overflow.svg） ---

func TestP38RootViewportClips(t *testing.T) {
	doc := mustParseFile(t, "par_slice_overflow.svg")
	// 视口 100×50，slice：缩放 2.5，内容 100×100，纵向溢出 50 被裁。
	img := doc.Render(100, 50)
	// 视口内必红（样本底 rect #ff0000）；视口外（整图高 50 即视口高，此处验证换算：
	// y=40 用户单位在 slice 下映射到 0+2.5*40=100 > 50 ⇒ 视口外，出图为空由宽高即视口兜住）。
	// 取视口内两点：用户 (5,5) → 屏幕 (12.5,12.5) 红；用户 (35,5) → (87.5,12.5) 绿。
	if got := img.RGBAAt(12, 12); got.R < 200 {
		t.Errorf("inside viewport (12,12) = %+v, want red-ish", got)
	}
	if got := img.RGBAAt(87, 12); got.G < 200 {
		t.Errorf("inside viewport (87,12) = %+v, want green-ish", got)
	}

	// RenderTo 大画布：视口外（y>50）必须保持透明——根视口裁剪的承重点
	// （slice 下内容 y'∈[−25,75]，y=70 处无裁剪会漏红）。
	big := render.NewContext(200, 100)
	doc.RenderTo(big, 0, 0, 100, 50)
	bigImg := big.Image().(*image.RGBA)
	if a := bigImg.RGBAAt(50, 70).A; a != 0 {
		t.Errorf("outside viewport (50,70) alpha = %d, want 0 (viewport clip)", a)
	}
	if a := bigImg.RGBAAt(150, 70).A; a != 0 {
		t.Errorf("outside viewport x (150,70) alpha = %d, want 0", a)
	}
}

// --- 单位系统（样本 units_absolute.svg / units_relative.svg） ---

func TestP38UnitsAbsolute(t *testing.T) {
	doc := mustParseFile(t, "units_absolute.svg")
	rects := []*RectElement{doc.Elements[0].(*RectElement), doc.Elements[1].(*RectElement),
		doc.Elements[2].(*RectElement), doc.Elements[3].(*RectElement)}
	// CSS 96dpi：pt=96/72, pc=16, in=96, cm=96/2.54, mm=96/25.4, q=96/254。
	r0 := rects[0]
	if r0.W.Unit != LengthUnitPt || r0.W.Value != 24 {
		t.Errorf("rect[0].W = %+v, want 24pt", r0.W)
	}
	if got := r0.W.Resolve(refSpace(doc, 1, 1), LengthAxisX); got != 24*96.0/72.0 {
		t.Errorf("24pt resolve = %v, want %v", got, 24*96.0/72.0)
	}
	if got := rects[1].W.Resolve(refSpace(doc, 1, 1), LengthAxisX); got != 16 {
		t.Errorf("1pc resolve = %v, want 16", got)
	}
	if got := rects[2].X.Resolve(refSpace(doc, 1, 1), LengthAxisX); got != 0.5*96 {
		t.Errorf("0.5in resolve = %v, want 48", got)
	}
	// 运行时浮点与无类型常量的末位差：单位换算用 1e-12 容差钉。
	if got := rects[2].H.Resolve(refSpace(doc, 1, 1), LengthAxisY); diff(got, 96/25.4) > 1e-12 {
		t.Errorf("1mm resolve = %v, want %v", got, 96/25.4)
	}
	if got := rects[3].X.Resolve(refSpace(doc, 1, 1), LengthAxisX); diff(got, 2*96/2.54) > 1e-12 {
		t.Errorf("2cm resolve = %v, want %v", got, 2*96/2.54)
	}
	if got := rects[3].W.Resolve(refSpace(doc, 1, 1), LengthAxisX); diff(got, 10*96/254.0) > 1e-12 {
		t.Errorf("10q resolve = %v, want %v", got, 10*96/254.0)
	}
}

func TestP38UnitsFontRelativeAndPercent(t *testing.T) {
	doc := mustParseFile(t, "units_relative.svg")
	g1 := doc.Elements[0].(*GroupElement)
	r1 := g1.Children[0].(*RectElement)
	// font-size=20 继承：1em=20，1ex=10。
	fs := 20.0
	rs := RefSpace{ViewportW: 200, ViewportH: 100, FontSize: fs}
	if got := r1.X.Resolve(rs, LengthAxisX); got != fs {
		t.Errorf("1em resolve = %v, want %v", got, fs)
	}
	if got := r1.Y.Resolve(rs, LengthAxisX); got != fs*0.5 {
		t.Errorf("1ex resolve = %v, want %v", got, fs*0.5)
	}
	g2 := doc.Elements[1].(*GroupElement)
	r2 := g2.Children[0].(*RectElement)
	// %：x/width 相对视口宽 200，y/height 相对视口高 100。
	if got := r2.X.Resolve(rs, LengthAxisX); got != 10 {
		t.Errorf("5%% of 200 = %v, want 10", got)
	}
	if got := r2.Y.Resolve(rs, LengthAxisY); got != 10 {
		t.Errorf("10%% of 100 = %v, want 10", got)
	}
	c := g2.Children[1].(*CircleElement)
	// r 相对归一化对角（SVG 1.1 §7.10）：sqrt((200²+100²)/2)=158.113883…，10% ⇒ 15.811…
	wantDiag := 15.811388300841898
	if got := c.R.Resolve(rs, LengthAxisDiagonal); diff(got, wantDiag) > 1e-9 {
		t.Errorf("10%% diagonal = %v, want %v", got, wantDiag)
	}
}

// --- 继承模型（样本 inherit_tree.svg） ---

func TestP38InheritanceTree(t *testing.T) {
	doc := mustParseFile(t, "inherit_tree.svg")
	g1 := doc.Elements[0].(*GroupElement)
	g2 := g1.Children[0].(*GroupElement)
	r0 := g2.Children[0].(*RectElement)
	r1 := g2.Children[1].(*RectElement)
	r2 := g2.Children[2].(*RectElement)

	root := rootProps()
	p1 := evalProps(root, g1.Attrs, doc)
	p2 := evalProps(p1, g2.Attrs, doc)
	p0 := evalProps(p2, r0.Attrs, doc)
	// fill #ff0000、fill-opacity 0.5 继承、stroke-width 被 g2 覆盖为 2、dasharray 继承。
	if p0.fill != "#ff0000" || p0.fillOpacity != 0.8 || p0.strokeWidth != 2 {
		t.Errorf("rect[0] props: fill=%q fillOpacity=%v strokeWidth=%v", p0.fill, p0.fillOpacity, p0.strokeWidth)
	}
	if p0.stroke != "#0000ff" {
		t.Errorf("rect[0] stroke = %q, want inherited #0000ff", p0.stroke)
	}
	if len(p0.dash) != 2 || p0.dash[0] != 6 || p0.dash[1] != 2 || p0.strokeMiter != 4 {
		t.Errorf("rect[0] dash=%v miter=%v, want [6 2] miter 4 (inherited)", p0.dash, p0.strokeMiter)
	}
	// currentColor：color=#00ff00 继承，fill=currentColor ⇒ #00ff00。
	pa := evalProps(p2, r1.Attrs, doc)
	if pa.fill != "#00ff00" {
		t.Errorf("currentColor fill = %q, want #00ff00", pa.fill)
	}
	// style 属性覆盖表现属性：fill:#ffff00 覆盖 fill="#000000"。
	pb := evalProps(p2, r2.Attrs, doc)
	if pb.fill != "#ffff00" {
		t.Errorf("style-override fill = %q, want #ffff00", pb.fill)
	}
	// opacity 不继承：父 g 未设 opacity，子元素 props.opacity 恒 1。
	if p0.opacity != 1 || pb.opacity != 1 {
		t.Errorf("opacity must not inherit, got %v/%v", p0.opacity, pb.opacity)
	}
}

// <style> 元素：单类选择器 + 标签选择器（样本 style_element.svg）。

func TestP38StyleElementSelectors(t *testing.T) {
	doc := mustParseFile(t, "style_element.svg")
	if len(doc.Styles) != 1 || len(doc.Styles[0].Rules) != 2 {
		t.Fatalf("style rules = %d blocks/%d rules, want 1/2", len(doc.Styles), len(doc.Styles[0].Rules))
	}
	// <style> 节点占 Elements[0]（入树不渲染），rect/circle 顺延。
	// rect class="warm"：class 规则优先于标签规则（fill 来自 .warm，stroke 来自 rect 标签规则）。
	rect := doc.Elements[1].(*RectElement)
	p := evalProps(rootProps(), rect.Attrs, doc)
	if p.fill != "#ff6600" {
		t.Errorf("class-rule fill = %q, want #ff6600", p.fill)
	}
	if p.stroke != "#cc00cc" || p.strokeWidth != 2 {
		t.Errorf("tag-rule stroke = %q w=%v, want #cc00cc w=2", p.stroke, p.strokeWidth)
	}
	// circle 无 class：标签规则 rect 不命中，fill 用自己的表现属性。
	circle := doc.Elements[2].(*CircleElement)
	pc := evalProps(rootProps(), circle.Attrs, doc)
	if pc.fill != "#0066ff" || pc.stroke != "" {
		t.Errorf("circle fill=%q stroke=%q, want #0066ff/空", pc.fill, pc.stroke)
	}
}

// --- 引用机制（样本 use_symbol.svg / ref_dangling.svg） ---

func TestP38UseExpandAndSymbol(t *testing.T) {
	doc := mustParseFile(t, "use_symbol.svg")
	if doc.ElementByID("chip") == nil || doc.ElementByID("box") == nil {
		t.Fatalf("byID lookup failed: chip=%v box=%v", doc.ElementByID("chip") != nil, doc.ElementByID("box") != nil)
	}
	use1 := doc.Elements[1].(*UseElement)
	if use1.HRef != "#box" || use1.X.Value != 10 || use1.W.Value != 40 {
		t.Errorf("use[0] = href %q x %+v w %+v", use1.HRef, use1.X, use1.W)
	}
	if use1.RefKind() != RefKindSymbol {
		t.Errorf("use[0] should resolve to symbol, got kind %v", use1.RefKind())
	}
	sym := doc.ElementByID("box").(*SymbolElement)
	if sym.ViewBox != (ViewBox{0, 0, 20, 10}) {
		t.Errorf("symbol viewBox = %+v", sym.ViewBox)
	}
	// symbol 内嵌 use（#chip）解析成功（引用收集含 defs 内元素）；
	// symbol 体 = [rect, use]，use 在 Children[1]。
	inner := sym.Children[1].(*UseElement)
	if inner.HRef != "#chip" || inner.RefKind() != RefKindElement {
		t.Errorf("inner use href=%q kind=%v", inner.HRef, inner.RefKind())
	}
	use2 := doc.Elements[2].(*UseElement)
	if use2.HRef != "#chip" || use2.RefKind() != RefKindElement {
		t.Errorf("use[1] href=%q kind=%v", use2.HRef, use2.RefKind())
	}
	// 像素探针（负向对照 N4 的红面）：symbol 视口 (10,10)-(50,30) 红底、
	// chip 阴影树实例 (20,14)-(40,34) 芯片色、独立 chip (80,30)-(90,40)。
	img := doc.Render(120, 60)
	if c := img.RGBAAt(12, 12); c.R < 150 {
		t.Errorf("symbol body (12,12) = %+v, want red-ish #aa0000", c)
	}
	if c := img.RGBAAt(30, 20); c.B < 80 || c.R > 40 {
		t.Errorf("shadow-tree chip (30,20) = %+v, want blue-ish #123456", c)
	}
	if c := img.RGBAAt(85, 35); c.B < 80 || c.R > 40 {
		t.Errorf("plain use chip (85,35) = %+v, want blue-ish #123456", c)
	}
}

func TestP38UseCycleRejected(t *testing.T) {
	// 自引用 use：解析成功、渲染不挂、具名 Warning（深度/环护栏）。样本 use_cycle.svg。
	doc := mustParseFile(t, "use_cycle.svg")
	img := doc.Render(10, 10) // 不得栈溢出/死循环
	_ = img
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUseCycle {
			found = true
		}
	}
	if !found {
		t.Errorf("use cycle: want warning %q, got %+v", warnUseCycle, doc.Warnings)
	}
}

func TestP38DanglingRefFailClosed(t *testing.T) {
	doc := mustParseFile(t, "ref_dangling.svg")
	// 悬空 url() 填充：不画（非黑）、具名 Warning。
	if len(doc.Warnings) < 2 {
		t.Fatalf("want >=2 warnings (dangling paint + dangling use), got %+v", doc.Warnings)
	}
	sawPaint, sawUse := false, false
	for _, w := range doc.Warnings {
		switch w.Reason {
		case warnUnresolvedPaint:
			sawPaint = true
			if w.Element != "rect" || w.Attribute != "fill" {
				t.Errorf("paint warning fields = %+v, want rect/fill", w)
			}
		case warnUnresolvedRef:
			sawUse = true
		}
	}
	if !sawPaint || !sawUse {
		t.Errorf("missing warnings: paint=%v use=%v (%+v)", sawPaint, sawUse, doc.Warnings)
	}
	// 画面：两处引用位置均为透明（不是黑），第三个自绘 rect 仍绿。
	img := doc.Render(100, 100)
	if a := img.RGBAAt(10, 10).A; a != 0 {
		t.Errorf("dangling paint pixel alpha = %d, want 0 (not black)", a)
	}
	if g := img.RGBAAt(50, 50).G; g < 200 {
		t.Errorf("self rect (50,50) = %+v, want green-ish", img.RGBAAt(50, 50))
	}
}

// RenderWithColor 主题色：描边也要被换色（批 1 重写曾丢该分支，存量真窗探针咬住后补钉）。
func TestP38OverrideRecolorsStroke(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="none">
	  <circle cx="8" cy="8" r="6" stroke="#4A90E2" stroke-width="2"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	img := doc.RenderWithColor(64, 64, color.NRGBA{R: 200, G: 80, B: 40, A: 255})
	if a := img.RGBAAt(32, 32).A; a != 0 {
		t.Errorf("center alpha = %d, want 0 (fill none survives override)", a)
	}
	r, g, b, a := img.RGBAAt(32, 8).RGBA()
	if a == 0 || diff(float64(r>>8), 200) > 8 || diff(float64(g>>8), 80) > 8 || diff(float64(b>>8), 40) > 8 {
		t.Errorf("ring = (%d,%d,%d,%d), want override (200,80,40)", r>>8, g>>8, b>>8, a>>8)
	}
}

// --- 嵌套 svg 视口（样本 nested_viewport.svg） ---

func TestP38NestedSVGViewport(t *testing.T) {
	doc := mustParseFile(t, "nested_viewport.svg")
	nested, ok := doc.Elements[1].(*SVGElement)
	if !ok {
		t.Fatalf("elements[1] = %T, want *SVGElement", doc.Elements[1])
	}
	if nested.X.Value != 20 || nested.W.Value != 60 || nested.ViewBox != (ViewBox{0, 0, 20, 10}) {
		t.Errorf("nested svg = x %+v w %+v vb %+v", nested.X, nested.W, nested.ViewBox)
	}
	if nested.Par.Align != ParAlignXMaxYMax {
		t.Errorf("nested pAR = %+v, want xMaxYMax", nested.Par)
	}
	// 20×10 内容等比 s=2 ⇒ 40×20，xMaxYMax ⇒ tx=60−40=20：内容占屏幕 [40,80]×[20,40]，
	// 子视口近角 (21,21) 留给底色（灰 R=240），远角 (79,39) 是蓝内容。
	img := doc.Render(100, 100)
	if b := img.RGBAAt(79, 39).B; b < 150 {
		t.Errorf("nested content (79,39) = %+v, want blue-ish at sub-viewport far corner", img.RGBAAt(79, 39))
	}
	if r := img.RGBAAt(21, 21).R; r < 150 {
		t.Errorf("xMaxYMax should push content to far corner, (21,21) = %+v", img.RGBAAt(21, 21))
	}
}

// --- 行为保持：方形 viewBox + meet（缺省）≡ 旧独立拉伸 ---

func TestP38SquareMeetEqualsLegacyStretch(t *testing.T) {
	doc := mustParseFile(t, "basic_square.svg")
	img := doc.Render(64, 64)
	if img == nil {
		t.Fatal("nil image")
	}
	// 存量语义守护：16×16 viewBox → 64×64 方形目标，meet s=4 与旧 sx=sy=4 逐点同变换。
	// 具体画面由存量 svg_test 覆盖；此处防回归：非空即过（详细逐位由真窗金图承担）。
	nonZero := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] != 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("empty render")
	}
}

// --- helpers ---

func mustParseFile(t *testing.T, name string) *Document {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "p38", name))
	if err != nil {
		t.Fatalf("read sample %s: %v", name, err)
	}
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

func refSpace(doc *Document, w, h float64) RefSpace {
	return RefSpace{ViewportW: w, ViewportH: h, FontSize: 16}
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}
