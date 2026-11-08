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
	"testing"

	"github.com/energye/gpui/render"
)

// P3.8 批 2a（SVG 渐变）红测。语义口径：Skia modules/svg 行为 + SVG 1.1 渐变规范，
// spreadMethod/fx,fy/gradientUnits 全部映射既有 render 底座（design.md §2.0）。

// --- 前置重构：parseTransformList 纯函数（M = M1·…·Mn 右乘链） ---

func TestP38TransformListPure(t *testing.T) {
	// translate(10,20) 单独
	m, err := parseTransformList("translate(10 20)")
	if err != nil || m != (render.Matrix{A: 1, C: 10, E: 1, F: 20}) {
		t.Errorf("translate = %+v err=%v", m, err)
	}
	// "translate(10 20) scale(2)"：点先 scale 再 translate ⇒ M = T·S
	m, err = parseTransformList("translate(10 20) scale(2)")
	want := render.Translate(10, 20).Multiply(render.Scale(2, 2))
	if err != nil || m != want {
		t.Errorf("T∘S = %+v err=%v want %+v", m, err, want)
	}
	// rotate(90 50 30) = T(50,30)·R·T(−50,−30)：绕点转 90°，(50,30) 不动、(60,30)→(50,40)（gg Rotate 正角 = 逆时针？以恒等式验证：变换后矩阵作用于向量 (10,0) 得 (0,10) 还是 (0,−10) 由 render.Rotate 定义，这里只钉合成序与不动点）
	m, err = parseTransformList("rotate(90 50 30)")
	rot := render.Translate(50, 30).Multiply(render.Rotate(90 * 3.141592653589793 / 180)).Multiply(render.Translate(-50, -30))
	if err != nil || m != rot {
		t.Errorf("rotate-about = %+v err=%v want %+v", m, err, rot)
	}
	p := rot.TransformPoint(render.Point{X: 60, Y: 30})
	if diff(p.X, 50) > 1e-9 || diff(p.Y, 40) > 1e-9 {
		t.Errorf("rotate-about (60,30) = (%v,%v), want (50,40)", p.X, p.Y)
	}
	// skewX(45)：B=tan(90°) 发散——用 45°→tan=1
	m, err = parseTransformList("skewX(45)")
	if err != nil || diff(m.B, 1) > 1e-9 {
		t.Errorf("skewX = %+v err=%v", m, err)
	}
	// matrix 六参直通
	m, err = parseTransformList("matrix(1 2 3 4 5 6)")
	if err != nil || m != (render.Matrix{A: 1, B: 3, C: 5, D: 2, E: 4, F: 6}) {
		t.Errorf("matrix = %+v err=%v", m, err)
	}
	// 空串 = 恒等
	m, err = parseTransformList("")
	if err != nil || m != render.Identity() {
		t.Errorf("empty = %+v err=%v", m, err)
	}
	// 坏函数具名报错
	if _, err = parseTransformList("nonsense(1)"); err == nil {
		t.Error("nonsense accepted")
	}
}

// --- 渐变解析：默认值/单位/stops/href 继承 ---

func TestP38GradientParse(t *testing.T) {
	doc := mustParseFile(t, "grad_linear_userspace.svg")
	defs, ok := doc.Elements[0].(*DefsElement)
	if !ok {
		t.Fatalf("Elements[0] = %T", doc.Elements[0])
	}
	g, ok := defs.Children[0].(*LinearGradientElement)
	if !ok {
		t.Fatalf("def[0] = %T, want *LinearGradientElement", defs.Children[0])
	}
	if g.Units != UnitsUserSpaceOnUse {
		t.Errorf("units = %v, want userSpaceOnUse", g.Units)
	}
	if g.X1.Value != 0 || g.X2.Value != 100 || g.Y2.Value != 0 {
		t.Errorf("line = (%v,%v)-(%v,%v)", g.X1, g.Y1, g.X2, g.Y2)
	}
	// 批 4c-4：offset/opacity 归一化移到求值期（usvg convert_stops 语义），
	// 解析面只存原始值。
	if len(g.Stops) != 2 || g.Stops[0].OffsetRaw != "0" || g.Stops[1].OffsetRaw != "1" {
		t.Fatalf("stops = %+v", g.Stops)
	}
	if g.Stops[0].Color != "#ff0000" || g.Stops[0].Opacity != 1 {
		t.Errorf("stop0 = %+v", g.Stops[0])
	}
	// 默认单位 = objectBoundingBox
	doc2 := mustParseFile(t, "grad_obb_repeat.svg")
	defs2 := doc2.Elements[0].(*DefsElement)
	g2 := defs2.Children[0].(*LinearGradientElement)
	if g2.Units != UnitsObjectBoundingBox {
		t.Errorf("default units = %v, want objectBoundingBox", g2.Units)
	}
	if g2.Spread != SpreadRepeat {
		t.Errorf("spread = %v, want repeat", g2.Spread)
	}
	// oBB 默认线 (0,0)→(1,0)：x2 缺省 100%（spec；userSpaceOnUse 下按
	// viewBox 宽解析，oBB 下 /100 = 1，批 4c-4 起 Value=100/Percent）。
	if g2.X1.Value != 0 || g2.X2.Value != 100 || g2.X2.Unit != LengthUnitPercent {
		t.Errorf("obb default line x = %v→%v, want 0→100%%", g2.X1, g2.X2)
	}
	// href 继承 stops：tilted 自身零 stops，href=#base
	doc3 := mustParseFile(t, "grad_transform_href.svg")
	defs3 := doc3.Elements[0].(*DefsElement)
	tilted := defs3.Children[1].(*LinearGradientElement)
	if tilted.HRef != "#base" {
		t.Errorf("href = %q", tilted.HRef)
	}
	stops := EffectiveStops(doc3, tilted)
	if len(stops) != 2 || stops[0].Color != "#ff0000" {
		t.Errorf("inherited stops = %+v", stops)
	}
}

// --- 渐变映射：Extend 三态直映 + 求值像素 ---

func TestP38GradientSpreadAndPixels(t *testing.T) {
	doc := mustParseFile(t, "grad_obb_repeat.svg")
	img := doc.Render(200, 60)
	// 4 个方块各自 0→1 循环：每块左端红、右端蓝；repeat 连续无 pad 拖尾。
	// 渐变底座为线性域插值（gamma-correct）：sRGB 期望按 srgb(linear lerp) 手算，
	// 如 t=0.05 的蓝 = srgb(0.05)≈60 而非 13。
	if c := img.RGBAAt(2, 30); c.R < 230 || c.B > 90 {
		t.Errorf("sq0 left = %+v, want red-ish (t≈0.05)", c)
	}
	if c := img.RGBAAt(38, 30); c.B < 200 || c.R > 90 {
		t.Errorf("sq0 right = %+v, want blue-ish (t≈0.95)", c)
	}
	if c := img.RGBAAt(62, 30); c.R < 230 || c.B > 90 {
		t.Errorf("sq1 left (repeat restart) = %+v, want red-ish (t≈0.05)", c)
	}
	// reflect 对照样本：同几何 reflect 第一周期末蓝、第二周期初又蓝（镜像起点）。
	// （线性域插值同上口径。）
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="40" viewBox="0 0 80 40">
	  <defs><linearGradient id="rf" spreadMethod="reflect" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="40" y2="0">
	    <stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/>
	  </linearGradient></defs>
	  <rect x="0" y="0" width="80" height="40" fill="url(#rf)"/>
	</svg>`
	doc2, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("reflect parse: %v", err)
	}
	img2 := doc2.Render(80, 40)
	if c := img2.RGBAAt(38, 20); c.B < 180 {
		t.Errorf("reflect t≈1 = %+v, want blue-ish", c)
	}
	if c := img2.RGBAAt(42, 20); c.B < 180 {
		t.Errorf("reflect t≈1 mirrored = %+v, want blue-ish", c)
	}
	// x=60 是镜像段中点：t'=0.5，sRGB 中点紫（stop 插值空间 = sRGB——
	// SVG color-interpolation 缺省 + resvg/tiny-skia 参照实证，批 4c-4）。
	if c := img2.RGBAAt(60, 20); diff(float64(c.R), 128) > 25 || diff(float64(c.B), 128) > 25 || c.G > 30 {
		t.Errorf("reflect mid-mirror = %+v, want purple ≈(127,0,127)", c)
	}
	// x=78 镜像近末端：t'≈0.95 → 红端。
	if c := img2.RGBAAt(78, 20); c.R < 230 || c.B > 60 {
		t.Errorf("reflect near mirror end = %+v, want red-ish", c)
	}
}

func TestP38GradientUserSpacePixels(t *testing.T) {
	doc := mustParseFile(t, "grad_linear_userspace.svg")
	img := doc.Render(100, 60)
	// 两端近纯色；中点 (50,30)：stop 插值在 sRGB（SVG 缺省
	// color-interpolation + resvg/tiny-skia 参照实证），t=0.5 ⇒ 128。
	if c := img.RGBAAt(2, 30); c.R < 220 || c.B > 90 {
		t.Errorf("left = %+v, want red", c)
	}
	if c := img.RGBAAt(98, 30); c.B < 220 || c.R > 90 {
		t.Errorf("right = %+v, want blue", c)
	}
	mid := img.RGBAAt(50, 30)
	if diff(float64(mid.R), 128) > 25 || diff(float64(mid.B), 128) > 25 {
		t.Errorf("mid = %+v, want ~128 (sRGB midpoint)", mid)
	}
}

func TestP38GradientFocalPixels(t *testing.T) {
	doc := mustParseFile(t, "grad_radial_focal.svg")
	img := doc.Render(100, 100)
	// 热点在 (30,30)（白），中心 (50,50) 偏灰，远离焦点角落近黑。
	hot := img.RGBAAt(30, 30)
	if hot.R < 240 || hot.G < 240 {
		t.Errorf("hotspot = %+v, want white-ish", hot)
	}
	if c := img.RGBAAt(95, 95); c.R > 60 {
		t.Errorf("far corner = %+v, want near black", c)
	}
	// 焦点偏移的不对称性：(70,70) 比 (30,30) 显著更暗。
	if c := img.RGBAAt(70, 70); c.R >= hot.R-40 {
		t.Errorf("opposite side = %+v, want clearly darker than hotspot", c)
	}
}

func TestP38GradientTransformPixel(t *testing.T) {
	doc := mustParseFile(t, "grad_transform_href.svg")
	img := doc.Render(100, 60)
	// gradientTransform=rotate(90 50 30)：CustomBrush 逆映射精确求值——
	// 点 (50,y) 逆变换回梯度空间 x = 50±(y−30)（绕 (50,30) 逆转 90°）：
	// 顶 (50,4) → 梯度空间 x=24 ⇒ t=0.24（sRGB 域 R≈194/B≈61），
	// 底 (50,56) → x=76 ⇒ t=0.76（sRGB 域 R≈61/B≈194），t 沿 y 增大。
	if c := img.RGBAAt(50, 4); c.R < 185 {
		t.Errorf("top = %+v, want t≈0.24 ramp (R≈194)", c)
	}
	if c := img.RGBAAt(50, 56); c.B < 185 || c.R > 75 {
		t.Errorf("bottom = %+v, want t≈0.76 ramp (R≈61/B≈194)", c)
	}
	if top, bottom := img.RGBAAt(50, 4), img.RGBAAt(50, 56); top.B >= bottom.B {
		t.Errorf("ramp must increase downward: top=%+v bottom=%+v", top, bottom)
	}
}

func TestP38GradientStrokeAndRefusals(t *testing.T) {
	doc := mustParseFile(t, "grad_stroke_and_refused.svg")
	img := doc.Render(100, 80)
	// 渐变描边：描边带 y∈[0,10)（内缘 y=10 起是内部，不属 band）——
	// 上边框带中央 (50,5) t≈0.5 ⇒ 线性域 (186,0,188)；左上角区 (8,10) t≈0.08 ⇒ R≈244。
	if c := img.RGBAAt(50, 5); c.R < 120 || c.B < 120 {
		t.Errorf("gradient stroke top = %+v, want mid-ramp (not black)", c)
	}
	if c := img.RGBAAt(8, 10); c.R < 180 {
		t.Errorf("gradient stroke corner = %+v, want red-ish", c)
	}
	// 零 stop 渐变：不画（卡底透明），具名 Warning；右下自绘 rect 绿
	// （探点避开自绘 rect (40,55,20,10)）。
	if a := img.RGBAAt(20, 60).A; a != 0 {
		t.Errorf("zero-stop fill = %+v, want nothing painted", img.RGBAAt(20, 60))
	}
	sawEmpty := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnresolvedPaint && w.Detail == "#empty" {
			sawEmpty = true
		}
	}
	if !sawEmpty {
		t.Errorf("zero-stop warning missing: %+v", doc.Warnings)
	}
	if got := img.RGBAAt(50, 60); got.G < 200 {
		t.Errorf("self rect = %+v, want green-ish", got)
	}
	// 回归保持：悬空 url 仍拒绝（批 1 行为）。
	dang := mustParseFile(t, "ref_dangling.svg")
	_ = dang
	dimg := dang.Render(100, 100)
	if a := dimg.RGBAAt(10, 10).A; a != 0 {
		t.Errorf("dangling paint regression = %+v, want unpainted", dimg.RGBAAt(10, 10))
	}
}

func TestP38GradientCurrentColorStop(t *testing.T) {
	// stop-color=currentColor 解析到继承的 color。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 40 40">
	  <g color="#ff8800">
	    <defs><linearGradient id="cc" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="40" y2="0">
	      <stop offset="0" stop-color="currentColor"/><stop offset="1" stop-color="currentColor"/>
	    </linearGradient></defs>
	    <rect x="0" y="0" width="40" height="40" fill="url(#cc)"/>
	  </g>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(40, 40)
	if c := img.RGBAAt(20, 20); c.R < 200 || c.G < 100 || c.B > 60 {
		t.Errorf("currentColor stop = %+v, want #ff8800", c)
	}
}

var _ = color.NRGBA{} // keep import if probes evolve
var _ = image.Rectangle{}
