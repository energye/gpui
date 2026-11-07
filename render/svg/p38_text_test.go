//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

// P3.8 批 3（SVG 文本）红测。字体=AHEM（每字符=满 em 高实心方块）：
// font-size 20 ⇒ 每字符 20×20 方块、advance 20px —— 锚点/间距/逐字符定位
// 全部精确断言。字体经 SetFontResolver 钩子注册（svg 不做系统扫描）。

const ahemPath = "testdata/p38/ahem.ttf"

func near(c interface{ RGBA() (r, g, b, a uint32) }, want [3]float64) bool {
	r, g, b, _ := c.RGBA()
	got := [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(b>>8) / 255}
	for i := range got {
		if absf(got[i]-want[i]) > 0.05 {
			return false
		}
	}
	return true
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func loadAhem(t *testing.T) *text.FontSource {
	t.Helper()
	data, err := os.ReadFile(ahemPath)
	if err != nil {
		t.Skipf("AHEM sample missing: %v", err)
	}
	src, err := text.NewFontSource(data)
	if err != nil {
		t.Fatalf("AHEM parse: %v", err)
	}
	return src
}

func withAhem(t *testing.T) {
	t.Helper()
	src := loadAhem(t)
	SetFontResolver(func(family string, weight text.FontWeight, italic bool) *text.FontSource {
		if family == "Ahem" {
			return src
		}
		return nil
	})
	t.Cleanup(func() { SetFontResolver(nil) })
}

// --- 基本定位与 anchor（text_basic.svg） ---

func TestP38TextBasicAnchor(t *testing.T) {
	withAhem(t)
	doc := mustParseFile(t, "text_basic.svg")
	img := doc.Render(200, 80)
	// 行 1：x=10 anchor=start（缺省）⇒ 两方块 [10,50)，基线 y=30 ⇒ 方块占 y[10,30)
	// （AHEM 基线在方块底部）。探方块内部两点 + 右端点外空。
	if c := img.RGBAAt(15, 36); c.R < 150 {
		t.Errorf("start glyph1 (15,20) = %+v, want red-ish", c)
	}
	if c := img.RGBAAt(45, 36); c.R < 150 {
		t.Errorf("start glyph2 (45,25) = %+v, want red-ish (advance 20)", c)
	}
	if c := img.RGBAAt(55, 36); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("beyond run (55,20) = %+v, want bg", c)
	}
	// 行 2：x=10 anchor=middle ⇒ 行宽 40，起点 = 10−20 = −10 ⇒ 方块 [−10,30)：可见部分 [0,30)。
	if c := img.RGBAAt(5, 60); c.B < 150 {
		t.Errorf("middle glyph1 (5,50) = %+v, want blue-ish (clipped at left)", c)
	}
	if c := img.RGBAAt(45, 60); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("middle beyond (45,50) = %+v, want bg (run ends at x=30)", c)
	}
}

// --- tspan：dx 相对偏移 / x,y 绝对定位 / 样式覆盖（text_tspan.svg） ---

func TestP38TextTspan(t *testing.T) {
	withAhem(t)
	doc := mustParseFile(t, "text_tspan.svg")
	img := doc.Render(200, 80)
	// 主行 x=10 y=30：'a' [10,30)；tspan dx=30 ⇒ 'b' 起点 = 30(a 尾) + 30 = 60 → [60,80) 绿。
	if c := img.RGBAAt(15, 36); c.R < 150 {
		t.Errorf("'a' = %+v, want red", c)
	}
	if c := img.RGBAAt(65, 36); c.G < 150 {
		t.Errorf("'b' (dx=30) = %+v, want green at [60,80)", c)
	}
	// tspan x=150 dy=20 ⇒ 绝对定位 (150, 50) 蓝。
	if c := img.RGBAAt(155, 56); c.B < 150 {
		t.Errorf("'c' (x=150,dy=20) = %+v, want blue at (150,50) baseline", c)
	}
	if c := img.RGBAAt(120, 36); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("gap between a and b = %+v, want bg", c)
	}
}

// --- paint 管线红利：渐变填充 + clipPath 裁剪（text_paint.svg） ---

func TestP38TextPaintPipeline(t *testing.T) {
	withAhem(t)
	doc := mustParseFile(t, "text_paint.svg")
	img := doc.Render(120, 60)
	// 样本叠放语义：第二个 text（绿 fill + clip [0,30)）盖在第一个（渐变）
	// 之上 —— x<30 见绿 text、x∈[30,40) 见渐变蓝段、x>40 见 bg。
	if c := img.RGBAAt(10, 36); c.G < 150 || c.R > 150 {
		t.Errorf("clip-zone 'a' = %+v, want green (top text)", c)
	}
	if c := img.RGBAAt(35, 36); c.B < 150 {
		t.Errorf("gradient 'b' tail (35,36) = %+v, want blue-ish (below-clip zone)", c)
	}
	if c := img.RGBAAt(45, 36); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("beyond glyphs (45,36) = %+v, want bg", c)
	}
}

// --- fail-closed：无匹配字体不画 + Warning（text_missing_font.svg） ---

func TestP38TextMissingFont(t *testing.T) {
	withAhem(t)
	doc := mustParseFile(t, "text_missing_font.svg")
	img := doc.Render(100, 40)
	if c := img.RGBAAt(15, 36); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("missing font text = %+v, want bg (unpainted)", c)
	}
	found := false
	for _, w := range doc.Warnings {
		if w.Reason == warnUnresolvedRef && w.Element == "text" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing font warning missing: %+v", doc.Warnings)
	}
	if got := img.RGBAAt(85, 15); got.G < 200 {
		t.Errorf("contrast rect = %+v, want green", got)
	}
}

// --- rotate 逐字符（内联样本） ---

func TestP38TextRotatePerGlyph(t *testing.T) {
	withAhem(t)
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="120" viewBox="0 0 120 120">
	  <rect x="0" y="0" width="120" height="120" fill="#171a20"/>
	  <text x="60" y="60" font-family="Ahem" font-size="20" fill="#e24a4a" rotate="90 90">ab</text>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(120, 120)
	// 'a' 原点 (60,60) 旋转 90°（绕自身原点，屏幕系顺时针）：方块 [60,60) 转到
	// 原点上方偏左（60−20..60 x，60..80 y 一侧）。'b' 原点沿旋转后 advance 移动。
	// 行为钉：旋转后墨迹不在未旋转位置 [60,80)×[40,60)，而在其下方区域。
	if c := img.RGBAAt(70, 50); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("unrotated position (70,50) = %+v, want bg (rotated away)", c)
	}
	inked := 0
	for y := 58; y < 82; y++ {
		for x := 42; x < 66; x++ {
			if img.RGBAAt(x, y).A != 0 {
				inked++
			}
		}
	}
	if inked < 200 {
		t.Errorf("rotated glyph ink = %d px, want substantial ink below-left", inked)
	}
}

// --- letter-spacing（内联样本） ---

func TestP38TextLetterSpacing(t *testing.T) {
	withAhem(t)
	src := `<svg xmlns="http://www.w3.org/2000/svg" width="120" height="40" viewBox="0 0 120 40">
	  <rect x="0" y="0" width="120" height="40" fill="#171a20"/>
	  <text x="0" y="30" font-family="Ahem" font-size="20" letter-spacing="10" fill="#e24a4a">ab</text>
	</svg>`
	doc, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	img := doc.Render(120, 40)
	// 'a' [0,20)；间距 10 ⇒ 'b' 起点 30 → [30,50)；[20,30) 空。
	if c := img.RGBAAt(10, 36); c.R < 150 {
		t.Errorf("'a' = %+v, want red", c)
	}
	if c := img.RGBAAt(25, 36); !near(c, [3]float64{0.09, 0.10, 0.125}) {
		t.Errorf("letter gap (25,20) = %+v, want bg", c)
	}
	if c := img.RGBAAt(35, 36); c.R < 150 {
		t.Errorf("'b' (spacing 10) = %+v, want red at [30,50)", c)
	}
}
