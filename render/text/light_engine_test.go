package text

import (
	"testing"
)

// reproLightFont 加载系统 CJK+Latin 对照字体（与本轮验证链路一致）。
func reproLightFont(t *testing.T) (cjk ParsedFont, latin ParsedFont) {
	t.Helper()
	cjkSrc, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	if err != nil {
		t.Skipf("CJK font unavailable: %v", err)
	}
	cjk = cjkSrc.Face(14).Source().Parsed()
	latSrc, err := NewFontSourceFromFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skipf("latin font unavailable: %v", err)
	}
	latin = latSrc.Face(14).Source().Parsed()
	return
}

// TestLightNew 验证引擎可创建。
func TestLightNew(t *testing.T) {
	e := New()
	if e == nil || e.extractor == nil {
		t.Fatal("New() returned bad engine")
	}
}

// TestLightHintNilFont 验证空字体报错。
func TestLightHintNilFont(t *testing.T) {
	e := New()
	for _, m := range []Mode{ModeLightCJK, ModeLightLatin} {
		if _, err := e.Hint(nil, 0, 14, m); err == nil {
			t.Fatalf("mode %v: expected error for nil font", m)
		}
	}
}

// TestLightHintSkeleton 骨架阶段：合法字体返回非空轮廓（未拟合 = FT-nohint 等价）。
func TestLightHintSkeleton(t *testing.T) {
	cjk, latin := reproLightFont(t)
	e := New()
	cases := []struct {
		name string
		font ParsedFont
		rune rune
		mode Mode
	}{
		{"cjk-niu", cjk, '钮', ModeLightCJK},
		{"latin-H", latin, 'H', ModeLightLatin},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			gid := tt.font.GlyphIndex(tt.rune)
			if gid == 0 {
				t.Skipf("glyph %q not in font", tt.rune)
			}
			out, err := e.Hint(tt.font, GlyphID(gid), 14, tt.mode)
			if err != nil {
				t.Fatalf("Hint: %v", err)
			}
			if out == nil {
				t.Fatal("Hint returned nil outline for inked glyph")
			}
			if len(out.Segments) == 0 {
				t.Fatal("outline has zero segments")
			}
		})
	}
}

// outlineLightTop 返回字形最高点（Y-up 像素）。
func outlineLightTop(t *testing.T, out *GlyphOutline) float64 {
	t.Helper()
	top := -1e9
	for _, s := range out.Segments {
		for _, p := range s.Points {
			up := -float64(p.Y)
			if up > top {
				top = up
			}
		}
	}
	return top
}

// TestLightM0TopBlueAnchor M0：有顶横字（日/田/目）顶横锚定蓝线。
// 覆盖 12–16px（§M0：主蓝字 12–16px 88–96% 归零；anchor=round(816×px/upem)
// 经验式，NotoSansCJK upem=1000 → 12px=10.0 / 14px=11.0 / 16px=13.0）。
func TestLightM0TopBlueAnchor(t *testing.T) {
	cjk, _ := reproLightFont(t)
	e := New()
	for _, tc := range []struct {
		px   float64
		want float64
	}{{12, 10.0}, {14, 11.0}, {16, 13.0}} {
		for _, r := range []rune{'日', '田', '目'} {
			gid := cjk.GlyphIndex(r)
			out, err := e.Hint(cjk, GlyphID(gid), tc.px, ModeLightCJK)
			if err != nil {
				t.Fatalf("Hint %q @%.0fpx: %v", r, tc.px, err)
			}
			if got := outlineLightTop(t, out); got != tc.want {
				t.Errorf("%q @%.0fpx M0 top = %.3f, want %.1f", r, tc.px, got, tc.want)
			}
		}
	}
}

// TestLightM0UnanchoredTop 无直线顶横的字（钮，曲线顶部）M0 不锚（保持原轮廓 top）。
func TestLightM0UnanchoredTop(t *testing.T) {
	cjk, _ := reproLightFont(t)
	e := New()
	base, err := NewOutlineExtractor().ExtractOutline(cjk, GlyphID(cjk.GlyphIndex('钮')), 12)
	if err != nil {
		t.Fatal(err)
	}
	hinted, err := e.Hint(cjk, GlyphID(cjk.GlyphIndex('钮')), 12, ModeLightCJK)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := outlineLightTop(t, hinted), outlineLightTop(t, base); got != want {
		t.Errorf("钮 top changed by light: %v → %v (want unchanged)", want, got)
	}
}

// TestLightUnknownMode 未知模式报错。
func TestLightUnknownMode(t *testing.T) {
	e := New()
	if _, err := e.Hint(nil, 0, 14, Mode(99)); err == nil {
		t.Fatal("unknown mode should error")
	}
}

// TestCFFLightBridgeTrailingOff 回归：CFF charstring 轮廓允许以 off 点结束
// （闭合段以轮廓起点 on 为隐式终点）。此前 rebuildSegmentsFromLightPts 遇
// 尾部 off 直接 return nil，导致 cffLightHintOutline 静默失败 → 生产路径
// 落 auto-hint/gridFit 兜底（整数网格），「合」16px 底横线整条消失。
func TestCFFLightBridgeTrailingOff(t *testing.T) {
	cjk, _ := reproLightFont(t)
	f, ok := cjk.(*ownParsedFont)
	if !ok || !f.hasPostScriptOutlines() {
		t.Skip("font not CFF, bridge path not used")
	}
	ext := NewOutlineExtractor()
	for _, tc := range []struct {
		r  rune
		px float64
	}{
		{'合', 12}, {'合', 16}, {'合', 24}, {'合', 48},
		{'静', 12}, {'静', 16}, {'每', 12}, {'每', 16},
	} {
		gid := f.GlyphIndex(tc.r)
		lo, ok := ext.cffLightHintOutline(f, GlyphID(gid), tc.px)
		if !ok {
			t.Fatalf("%q %.0fpx: cffLightHintOutline failed (trailing-off contour rejected)", tc.r, tc.px)
		}
		if len(lo.Segments) == 0 {
			t.Fatalf("%q %.0fpx: zero segments", tc.r, tc.px)
		}
	}
}

// TestCFFLightBridgeKeepsSubpixel 回归：light 引擎输出的亚像素 Y
// 不得被兜底路径吸成整数。此前「合」16px 底边 Y 从 1.83 变成 1.00，
// 底横线落在像素边界零覆盖消失。断言 ExtractOutlineHinted(HintVertical)
// 保持亚像素底边（与 FT-light 26.6 一致）。
func TestCFFLightBridgeKeepsSubpixel(t *testing.T) {
	cjk, _ := reproLightFont(t)
	f, ok := cjk.(*ownParsedFont)
	if !ok || !f.hasPostScriptOutlines() {
		t.Skip("font not CFF, bridge path not used")
	}
	ext := NewOutlineExtractor()
	r := '合'
	gid := f.GlyphIndex(r)
	o, err := ext.ExtractOutlineHinted(f, GlyphID(gid), 16, HintingVertical)
	if err != nil || o == nil {
		t.Fatalf("ExtractOutlineHinted: %v", err)
	}
	maxY := float32(-1e9)
	for _, s := range o.Segments {
		for _, p := range s.Points {
			if p.X != 0 || p.Y != 0 {
				if p.Y > maxY {
					maxY = p.Y
				}
			}
		}
	}
	if maxY < 1.5 {
		t.Errorf("合 16px bottom Y = %.2f, want ≈1.83 (subpixel preserved, FT-light match)", maxY)
	}
}