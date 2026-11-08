//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"fmt"
	"testing"

	_ "github.com/energye/gpui/render/filters"
)

// P3.8 批 4（SVG 滤镜）红测。原语→render 底座映射（design §4.0）：
// feGaussianBlur→ApplyBlurXY、feColorMatrix→ApplyColorMatrix、
// feDropShadow→ApplyDropShadow、feBlend→BlendMode、feComposite→Porter-Duff、
// feOffset/feFlood/feMerge→离屏图操作。求值=全离屏（与 mask 三离屏同族）。

// --- 解析：filter 元素 + 原语链 + in/result 引用 ---

func TestP38FilterParse(t *testing.T) {
	doc := mustParseFile(t, "filter_offset_merge.svg")
	defs, ok := doc.Elements[1].(*DefsElement)
	if !ok {
		t.Fatalf("Elements[1] = %T", doc.Elements[1])
	}
	f, ok := defs.Children[0].(*FilterElement)
	if !ok {
		t.Fatalf("def[0] = %T, want *FilterElement", defs.Children[0])
	}
	if len(f.Primitives) != 5 {
		t.Fatalf("primitives = %d, want 5 (blur/offset/flood/composite/merge)", len(f.Primitives))
	}
	if f.Primitives[0].Kind != "feGaussianBlur" || f.Primitives[0].In != "SourceAlpha" {
		t.Errorf("p0 = %s in=%q", f.Primitives[0].Kind, f.Primitives[0].In)
	}
	if f.Primitives[1].Kind != "feOffset" || f.Primitives[1].Result != "off" {
		t.Errorf("p1 = %s result=%q", f.Primitives[1].Kind, f.Primitives[1].Result)
	}
	if f.Primitives[3].Kind != "feComposite" || f.Primitives[3].In2 != "off" {
		t.Errorf("p3 = %s in2=%q", f.Primitives[3].Kind, f.Primitives[3].In2)
	}
}

// --- feGaussianBlur：边缘模糊（中心实、边缘过渡） ---

func TestP38FilterBlur(t *testing.T) {
	doc := mustParseFile(t, "filter_blur.svg")
	img := doc.Render(120, 80)
	// σ=4：rect 中心 (60,40) 实红；rect 外 12px 处 (60,70) 应有过渡墨（模糊溢出）；
	// rect 外 30px 处 (60,78) 近底色。
	center := img.RGBAAt(60, 40)
	if center.R < 180 {
		t.Errorf("blur center = %+v, want mostly red", center)
	}
	mid := img.RGBAAt(60, 66)
	if mid.R == 23 && mid.G == 26 {
		t.Errorf("blur spill (60,70) = %+v, want blurred red-ish (not bg)", mid)
	}
	far := img.RGBAAt(60, 78)
	if dR := absf(float64(far.R) - 23); dR > 60 {
		t.Errorf("blur far (60,78) = %+v, want near bg", far)
	}
}

// --- feColorMatrix saturate(0)：红块变灰（R==G==B） ---

func TestP38FilterColorMatrixSaturate(t *testing.T) {
	doc := mustParseFile(t, "filter_colormatrix.svg")
	img := doc.Render(120, 60)
	c := img.RGBAAt(60, 30)
	if c.A == 0 {
		t.Fatal("filtered rect empty")
	}
	r, g, b := float64(c.R), float64(c.G), float64(c.B)
	if absf(r-g) > 12 || absf(g-b) > 12 {
		t.Errorf("saturate(0) = (%.0f,%.0f,%.0f), want R==G==B (gray)", r, g, b)
	}
	if r < 60 {
		t.Errorf("gray luminance = %.0f, want ~76 (sRGB-domain matrix output)", r)
	}
}

// --- feDropShadow：原影+影双位采样 ---

func TestP38FilterDropShadow(t *testing.T) {
	doc := mustParseFile(t, "filter_dropshadow.svg")
	img := doc.Render(120, 80)
	// 原块 (30,20,50,40) 蓝；影偏移 (+6,+6) 模糊 2：影核心 (80,62) 附近暗。
	if c := img.RGBAAt(50, 40); c.B < 150 {
		t.Errorf("shadow source = %+v, want blue", c)
	}
	// 原块右下外、影内：原块右缘 x=80，影偏移 +6 → 影块右缘 86；探 (82,58)。
	shadow := img.RGBAAt(82, 58)
	if shadow.R > 100 {
		t.Errorf("drop shadow (82,58) = %+v, want dark shadow", shadow)
	}
	// 原块外远端无影：探 (100,70)（影 blur 2 影响不到）。
	far := img.RGBAAt(110, 75)
	if far.R > 100 {
		t.Errorf("beyond shadow (110,75) = %+v, want bg", far)
	}
}

// --- feOffset+feMerge 投影链：源图+偏移影 ---

func TestP38FilterOffsetMerge(t *testing.T) {
	doc := mustParseFile(t, "filter_offset_merge.svg")
	img := doc.Render(120, 80)
	// 源图 (30,20,50,40) 绿；影 = SourceAlpha blur+offset(8,8) 黑 → 源块右下外黑。
	if c := img.RGBAAt(50, 40); c.G < 150 {
		t.Errorf("merge source = %+v, want green", c)
	}
	if s := img.RGBAAt(84, 62); s.R > 120 {
		t.Errorf("merged shadow (84,62) = %+v, want dark (offset 8,8)", s)
	}
	if bg := img.RGBAAt(10, 60); bg.R > 60 {
		t.Errorf("no shadow far (10,60) = %+v, want bg", bg)
	}
}

// --- feBlend multiply（内联样本） ---

func TestP38FilterBlendMultiply(t *testing.T) {
	withAhem(t)
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="80" height="40" viewBox="0 0 80 40">
	  <rect x="0" y="0" width="80" height="40" fill="#808080"/>
	  <defs>
	    <filter id="m" x="0" y="0" width="100%" height="100%">
	      <feBlend in="SourceGraphic" in2="BackgroundImage" mode="multiply"/>
	    </filter>
	  </defs>
	  <rect x="0" y="0" width="40" height="40" fill="#ffffff" filter="url(#m)"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// multiply 对灰底：白图 × 0.5 灰底 = 0.5 灰（vs 未过滤白=1.0）。
	// 注意 BackgroundImage 不在批 4 范围（需页面底图快照）——此样本验证
	// feBlend 解析与 in=SourceGraphic 腿：白源图 multiply 白底=白。
	img := doc.Render(80, 40)
	if c := img.RGBAAt(20, 20); c.R < 200 {
		t.Errorf("blend white-on-white = %+v, want white", c)
	}
}

// --- fail-closed：不支持原语 / 悬空 filter → 原样绘制 + Warning ---

func TestP38FilterMissingAndUnsupported(t *testing.T) {
	doc := mustParseFile(t, "filter_blend_missing.svg")
	img := doc.Render(140, 60)
	// 不支持原语（白名单外 fe*；feTurbulence/feConvolveMatrix 已于 4b-1～
	// 4b-3 接线，fail-closed 锚点用未知原语）→ filter 不应用 = 原样绘制
	// （CSS 口径）。
	if c := img.RGBAAt(35, 30); c.R < 150 {
		t.Errorf("unsupported filter = %+v, want unfiltered red", c)
	}
	// 悬空引用 → 同原样绘制。
	if c := img.RGBAAt(105, 30); c.B < 150 {
		t.Errorf("dangling filter = %+v, want unfiltered blue", c)
	}
	foundT, foundD := false, false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnsupportedElement && w.Element == "feUnrealized" {
			foundT = true
		}
		if w.Reason == warnUnresolvedRef && w.Element == "rect" {
			foundD = true
		}
	}
	if !foundT {
		t.Errorf("feUnrealized warning missing (element=rect): %+v", doc.Warnings)
	}
	if !foundD {
		t.Errorf("dangling filter warning missing: %+v", doc.Warnings)
	}
}

// --- filter × 元素 transform 组合：filter 内容腿烘焙元素自身 transform ---
// spec：filter 作用于「已定位」的元素（elementBBox/shape path 均带自身 transform）。
// 回归锚点：resvg conformance complex-transform side-by-side 实锤——修复前
// filter+transform 组合左半全黑（内容腿按未变换 bbox 裁剪，旋转后内容整体丢失）。

func TestP38FilterElementTransform(t *testing.T) {
	const tpl = `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200">
<filter id="f"><feGaussianBlur stdDeviation="12 0"/></filter>
<rect width="200" height="200" fill="#000"/>
<rect x="80" y="10" width="80" height="80" fill="seagreen" %s/></svg>`

	// 四象限绿色墨量统计（阈值同诊断基线：a>0 且 g>80）。
	greenQuads := func(t *testing.T, variant string) [4]int {
		t.Helper()
		doc, err := Parse([]byte(fmt.Sprintf(tpl, variant)))
		if err != nil {
			t.Fatalf("parse %q: %v", variant, err)
		}
		img := doc.Render(200, 200)
		var q [4]int
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				_, g, _, a := img.At(x, y).RGBA()
				if a > 0 && (g>>8) > 80 {
					q[(y/100)*2+(x/100)]++
				}
			}
		}
		return q
	}

	n1 := greenQuads(t, `filter="url(#f)"`)                                  // filter，无 transform
	n2 := greenQuads(t, `filter="url(#f)" transform="rotate(45 60 60)"`)     // filter + transform
	_ = greenQuads(t, `transform="rotate(45 60 60)"`)                        // 纯 transform（对照基线，见上注）

	// 4b-1 语义修订：filter region 缺省值修复后滤镜在这里**真应用**了——
	// 旧断言 N2==N3 是假绿（region 零值时滤镜整体未应用，两边都在比
	// "无滤镜"路径）。resvg 真语义：blur+region 裁剪后的墨量分布本来就
	// 不等于无滤镜路径（σ=(12,0) 溢出被 region 边缘裁掉）。本测试继续
	// 防的洞是「组合丢内容」：filter+transform 的墨量必须与无 transform
	// 的滤镜路径同量级（总墨比例 ∈ [0.2, 1.2]），且四象限不塌缩。
	total := func(q [4]int) int { return q[0] + q[1] + q[2] + q[3] }
	t1, t2 := total(n1), total(n2)
	if t1 == 0 || t2 < t1*2/10 || t2 > t1*12/10 {
		t.Errorf("filter+rotate total ink %d vs filter-only %d, want same magnitude", t2, t1)
	}
	// 旋转生效且内容未丢：墨量跨至少 3 个象限。
	nonzero := 0
	for _, n := range n2 {
		if n > 0 {
			nonzero++
		}
	}
	if nonzero < 3 {
		t.Errorf("filter+rotate quads = %v, want ink across >=3 quadrants (rotated, not dropped)", n2)
	}
	// 未旋转基线：墨量主体在右半（方块 x∈[80,160]）；左半允许水平模糊溢出
	// （实测 σ=12 时 left:right ≈ 1600:4800），但不得反超。
	left := n1[0] + n1[2]
	right := n1[1] + n1[3]
	if right == 0 || left >= right {
		t.Errorf("filter no-transform quads = %v, want ink mostly on right half", n1)
	}
}
