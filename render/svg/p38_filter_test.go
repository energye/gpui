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
	// 不支持原语（feTurbulence）→ filter 不应用 = 原样绘制（CSS 口径）。
	if c := img.RGBAAt(35, 30); c.R < 150 {
		t.Errorf("unsupported filter = %+v, want unfiltered red", c)
	}
	// 悬空引用 → 同原样绘制。
	if c := img.RGBAAt(105, 30); c.B < 150 {
		t.Errorf("dangling filter = %+v, want unfiltered blue", c)
	}
	foundT, foundD := false, false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnsupportedElement && w.Element == "feTurbulence" {
			foundT = true
		}
		if w.Reason == warnUnresolvedRef && w.Element == "rect" {
			foundD = true
		}
	}
	if !foundT {
		t.Errorf("feTurbulence warning missing (element=rect): %+v", doc.Warnings)
	}
	if !foundD {
		t.Errorf("dangling filter warning missing: %+v", doc.Warnings)
	}
}
