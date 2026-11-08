//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"math"
	"strings"
	"testing"
)

// P3.8 批 4d（showcase 全面压测窗人工对照归因）红测。四个引擎洞：
// currentColor 填充/描边、刷子填充上的元素 opacity、滤镜链在带 transform
// 元素上的泛光丢失、圆弧虚线相位缺口。语义口径：usvg 0.42 + SVG 1.1 规范，
// resvg 参照渲染逐位置对照（showcase_ref.png 复现表见 4d 证据）。

// --- 洞 1：fill/stroke="currentColor" 落到默认黑 ---
// usvg paint.rs：currentColor 解析到继承的 color 属性（无 color → 黑）。
// 修复前 applyFillAttrs 把 currentColor 交给 parseColor（显式返回 nil），
// 命中"默认填充黑"分支——showcase 两个 currentColor 色块渲染成黑块。

func TestP38C4DFillStrokeCurrentColor(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40">
	  <rect x="0" y="0" width="20" height="40" fill="currentColor" color="#e24a4a"/>
	  <rect x="20" y="0" width="20" height="40" fill="currentColor" color="#3ccf7a"/>
	  <g color="#4a90e2">
	    <rect x="14" y="14" width="12" height="12" fill="none" stroke="currentColor" stroke-width="4"/>
	  </g>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(40, 40)
	if c := img.RGBAAt(10, 20); c.R < 200 || c.G > 120 || c.B > 120 {
		t.Errorf("fill=currentColor (own color) = %+v, want #e24a4a", c)
	}
	if c := img.RGBAAt(30, 20); c.R > 120 || c.G < 180 || c.B > 180 {
		t.Errorf("fill=currentColor (own color 2) = %+v, want #3ccf7a", c)
	}
	if c := img.RGBAAt(20, 13); c.B < 180 || c.R > 120 {
		t.Errorf("stroke=currentColor (inherited color) = %+v, want #4a90e2", c)
	}
	// 无 color 属性：currentColor 回落黑（usvg 语义）。
	src2 := `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10">
	  <rect width="10" height="10" fill="currentColor"/></svg>`
	doc2, err := Parse([]byte(src2))
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	img2 := doc2.Render(10, 10)
	if c := img2.RGBAAt(5, 5); c.R != 0 || c.G != 0 || c.B != 0 {
		t.Errorf("currentColor without color = %+v, want black", c)
	}
}

// --- 洞 2：刷子填充（paint server）上的元素 opacity 被丢弃 ---
// SVG 1.1：opacity 是组不透明度，对单一填充等价于把 alpha 折进涂料。
// 实心色分支折了（setColorWithOpacity），刷子分支直接 SetFillBrush 返回
// ——showcase 底部反射带（渐变填充 + opacity 0.55）按全不透明绘制。
// 对照组：实心 fill + opacity（修复前应已通过）、stop-opacity（另一条链）。

func TestP38C4DBrushElementOpacity(t *testing.T) {
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="60" height="80">
	  <defs>
	    <linearGradient id="lg" x1="0" y1="0" x2="1" y2="0">
	      <stop offset="0" stop-color="#4a90e2"/>
	      <stop offset="1" stop-color="#4a90e2"/>
	    </linearGradient>
	    <pattern id="pt" width="20" height="20" patternUnits="userSpaceOnUse">
	      <rect width="20" height="20" fill="#e2884a"/>
	    </pattern>
	  </defs>
	  <rect x="0" y="0" width="60" height="20" fill="#4a90e2" opacity="0.5"/>
	  <rect x="0" y="20" width="60" height="20" fill="url(#lg)" opacity="0.5"/>
	  <rect x="0" y="40" width="60" height="20" fill="url(#pt)" opacity="0.5"/>
	  <rect x="0" y="60" width="60" height="20" fill="none" stroke="url(#lg)" stroke-width="20" opacity="0.5"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(60, 80)
	// 透明画布上 alpha 即涂料的等效不透明度：0.5 → 127/128（0.5·255 取整）。
	for _, tc := range []struct {
		name  string
		y     int
		wantA uint8
	}{
		{"solid fill + opacity (control)", 10, 127},
		{"gradient fill + opacity", 30, 127},
		{"pattern fill + opacity", 50, 127},
		{"gradient stroke + opacity", 70, 127},
	} {
		// pattern 行取 y=45：第 4 块的 stroke-width=20 描边带（居中于矩形
		// 轮廓）合法地覆盖 y 50..70，采样点要避开这条真实的叠涂带。
		y := tc.y
		if tc.y == 50 {
			y = 45
		}
		if c := img.RGBAAt(30, y); c.A != tc.wantA {
			t.Errorf("%s: alpha=%d, want %d (got %+v)", tc.name, c.A, tc.wantA, c)
		}
	}
	// pattern 行颜色钉死：橙 premul @0.5。
	if c := img.RGBAAt(30, 45); c.R != 113 || c.G != 68 || c.B != 37 {
		t.Errorf("pattern fill color = %+v, want premul (113,68,37)", c)
	}
}

// --- 洞 3：滤镜 + transform 组合丢泛光 ---
// showcase glowChain（feFlood→feComposite in→feOffset→feBlur→feMerge）挂在
// transform="skewY(6)" 的 rect 上：红块本体在、feFlood 泛光分支整体消失。
// 逐原语隔离：feOffset 控制组（无 transform）/ 平移 / skewY / 完整泛光链。

func TestP38C4DFilterWithTransform(t *testing.T) {
	// (a) feOffset 无 transform（控制组）与 skewY：偏移都应生效。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="60">
	  <filter id="off" x="-100%" y="-100%" width="300%" height="300%">
	    <feOffset dx="14" dy="0"/>
	  </filter>
	  <rect x="10" y="20" width="20" height="20" fill="#4a90e2" filter="url(#off)"/>
	  <rect x="60" y="20" width="20" height="20" fill="#4a90e2" filter="url(#off)" transform="skewY(6)"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(160, 60)
	// 控制组：原位 (10..30) 应空（内容被偏移走），(24..44) 有蓝。
	if c := img.RGBAAt(20, 30); c.A != 0 {
		t.Errorf("feOffset control: origin not vacated, alpha=%d", c.A)
	}
	if c := img.RGBAAt(38, 30); c.A == 0 {
		t.Errorf("feOffset control: offset content missing at x=38")
	}
	// skewY(6)：滤镜在元素局部空间先偏移（(74..94,20..40)），再随 transform
	// 映射（y + tan6°·x → 27.8..49.9），实测 bbox (74..93,26..48)。原位
	// (70,30) 应空（x<74），偏移后的 (84,37) 有蓝。
	if c := img.RGBAAt(70, 30); c.A != 0 {
		t.Errorf("feOffset + skewY: origin not vacated, alpha=%d", c.A)
	}
	if c := img.RGBAAt(84, 37); c.A == 0 {
		t.Errorf("feOffset + skewY: offset content missing at (84,37)")
	}

	// (b) 完整泛光链（showcase glowChain 同构）：flood 蓝泛光应出现在
	// 红块右上（dx=4, dy=-4 偏移 + blur）。带与不带 skewY 各一份。
	src2 := `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="80">
	  <filter id="glow" x="-40%" y="-40%" width="180%" height="180%">
	    <feFlood flood-color="#4a90e2" flood-opacity="0.9"/>
	    <feComposite in2="SourceGraphic" operator="in"/>
	    <feOffset dx="4" dy="-4"/>
	    <feGaussianBlur stdDeviation="3"/>
	    <feMerge><feMergeNode/><feMergeNode in="SourceGraphic"/></feMerge>
	  </filter>
	  <rect x="20" y="30" width="40" height="30" fill="#e24a4a" filter="url(#glow)"/>
	  <rect x="100" y="30" width="40" height="30" fill="#e24a4a" filter="url(#glow)" transform="skewY(6)"/>
	</svg>`
	doc2, err := Parse([]byte(src2))
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	img2 := doc2.Render(160, 80)
	// 泛光采样（premultiplied 画布，先还原 straight alpha 再判蓝味）：
	// 无 transform 的泛光区 (24..64,26..56)+blur，右上角外一点 (63,27)；
	// skewY 的泛光区随 transform 下移（实测 bbox 右上达 (146,35)），取
	// (143,38)。实测 bbox：无 transform (21..67,23..59)、skew (100..146,35..74)。
	hasBlueTint := func(x, y int) bool {
		c := img2.RGBAAt(x, y)
		if c.A == 0 {
			return false
		}
		r := uint32(c.R) * 255 / uint32(c.A)
		b := uint32(c.B) * 255 / uint32(c.A)
		return b > r+40 && b > 160
	}
	if !hasBlueTint(63, 27) {
		t.Errorf("glow chain (no transform): no blue tint at (63,27), got %+v", img2.RGBAAt(63, 27))
	}
	if !hasBlueTint(143, 38) {
		t.Errorf("glow chain (skewY): no blue tint at (143,38), got %+v", img2.RGBAAt(143, 38))
	}
}

// --- 洞 5：祖先级联的 font-family 到不了文字 span ---
// usvg：font-family 是 CSS 继承属性。showcase 根节点 font-family="Ahem"，
// 所有 <text> 自身不声明家族 —— 旧实现 props 无 fontFamily、flatten 只沿
// text→tspan 链，家族在两条路（渲染 + 解析期校验）都被丢，文字整棵静默
// 不渲染且无警告。

func TestP38C4DInheritedFontFamily(t *testing.T) {
	withAhem(t)
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60" font-family="Ahem">
	  <text x="10" y="25" font-size="14" fill="#c8d0da">root family</text>
	  <g>
	    <text x="10" y="50" font-size="14" fill="#c8d0da">nested</text>
	  </g>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", doc.Warnings)
	}
	img := doc.Render(120, 60)
	// Ahem 14px：墨迹在基线上方（标准排版，与 resvg 一致），基线 25 →
	// 盒 13.8..27.8，取 (12,20)；(12,45) 行基线 50 → 盒 38.8..52.8。
	if c := img.RGBAAt(12, 20); c.A != 255 {
		t.Errorf("root-family text not rendered at (12,20): %+v", c)
	}
	if c := img.RGBAAt(12, 45); c.A != 255 {
		t.Errorf("nested text (g 中间层) not rendered at (12,45): %+v", c)
	}

	// 解析期具名警告：家族注册表里没有的名字 → 一条 warning + 不渲染。
	src2 := `<svg xmlns="http://www.w3.org/2000/svg" width="60" height="30" font-family="NoSuchFace">
	  <text x="5" y="20" font-size="12">ghost</text>
	</svg>`
	doc2, err := Parse([]byte(src2))
	if err != nil {
		t.Fatalf("parse2: %v", err)
	}
	found := false
	for _, w := range doc2.Warnings {
		if w.Reason == warnUnresolvedRef && strings.Contains(w.Detail, "NoSuchFace") {
			found = true
		}
	}
	if !found {
		t.Errorf("unresolved inherited family not warned: %v", doc2.Warnings)
	}
	if c := doc2.Render(60, 30).RGBAAt(8, 16); c.A != 0 {
		t.Errorf("unresolvable text rendered: %+v", c)
	}
}

// --- 洞 4：圆弧虚线单缺口 ---
// stroke-dasharray="30 14" 的圆：理论缺口在弧长 30..44/74..88/118..132/
// 162..176/206..220/250..264（自 (cx+r,cy) 顺时针）。修复前 s≈74..88 的
// 缺口被填实（showcase 虚线圆与 resvg 参照唯一相位差）。

func TestP38C4DCircleDashPhase(t *testing.T) {
	// showcase 虚线圆带 stroke-linecap="round"（参照图泛光圆的相位差只在
	// round cap 下出现），无 cap 的裸圆做控制组。
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120">
	  <circle cx="60" cy="60" r="46" fill="none" stroke="#4a90e2" stroke-width="6" stroke-dasharray="30 14"/>
	  <circle cx="180" cy="60" r="46" fill="none" stroke="#4a90e2" stroke-width="6" stroke-dasharray="30 14" stroke-linecap="round"/>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(240, 120)
	isBlue := func(r, g, b uint8) bool { return b > 140 && b > r+40 && g > r && g < b }
	// 48 步环采样（±3 径向容差），缺口步应有透明底。round cap 下 dash 端点
	// 各延伸 width/2=3，缺口有效区间各缩 3；采样步仍以裸缺口为准，边界步
	// （±3 内）跳过以免把 cap 半圆误判成洞。
	for _, tc := range []struct {
		name string
		cx   float64
	}{
		{"butt cap", 60},
		{"round cap", 180},
	} {
		for i := 0; i < 48; i++ {
			a := 2 * math.Pi * float64(i) / 48
			s := a * 46 // 弧长
			inGap := false
			gapEdge := false
			for g0 := 30.0; g0 < 264.0; g0 += 44 {
				if s >= g0 && s <= g0+14 {
					inGap = true
				}
				// 缺口边界 2 单位内是 dash 端头抗锯齿，不作空断言。
				if s > g0-2 && s < g0+16 {
					gapEdge = true
				}
			}
			hit := false
			for o := -3; o <= 3; o++ {
				rr := 46 + float64(o)
				x := int(tc.cx + rr*math.Cos(a))
				y := int(60 + rr*math.Sin(a))
				c := img.RGBAAt(x, y)
				if isBlue(c.R, c.G, c.B) {
					hit = true
				}
			}
			if tc.name == "butt cap" {
				if inGap && !gapEdge && hit {
					t.Errorf("%s: dash gap at arc s=%.1f (step %d) should be empty, got stroke", tc.name, s, i)
				}
				if !inGap && !hit && (i < 5 || i > 44) {
					t.Errorf("%s: dash on arc s=%.1f (step %d) missing", tc.name, s, i)
				}
				continue
			}
			// round cap：缺口步且离缺口边界 >3 才必须空（cap 半圆占边界 3）。
			if inGap && hit {
				deep := true
				for g0 := 30.0; g0 < 264.0; g0 += 44 {
					if s < g0+4.5 || s > g0+14-4.5 {
						deep = false
					}
				}
				if deep {
					t.Errorf("%s: deep dash gap at arc s=%.1f (step %d) should be empty, got stroke", tc.name, s, i)
				}
			}
		}
	}
}
