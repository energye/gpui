package platform

import (
	"testing"
)

// TestCSDSysGlyphCJK verifies a CJK rune is rasterized from the system font
// into a real glyph (strokes present, sane advance) instead of the '?'
// placeholder. Skips when no CJK-capable system font exists (not silent
// green: the whole point of the fallback is to keep '?' in that case).
func TestCSDSysGlyphCJK(t *testing.T) {
	g, ok := csdSysGlyph('层')
	if !ok {
		t.Skipf("系统无 CJK 字体（csdSysFontCandidates 全部缺失），'层' 保持 '?' 占位 —— 跳过")
	}
	if g.width <= 0 || g.height <= 0 || g.advance <= 0 {
		t.Fatalf("'层' 字形尺寸异常: w=%d h=%d adv=%d", g.width, g.height, g.advance)
	}
	n := 0
	for _, a := range g.alpha {
		if a > 0 {
			n++
		}
	}
	if n == 0 {
		t.Fatal("'层' 栅格化后无任何笔画像素（alpha 全 0）")
	}
	// A 13px han glyph carries far more strokes than the 6x13 '?' bitmap
	// (the pre-fix placeholder); n>0 already proves rasterization, this
	// guards against a degenerate single-dot result.
	if n < 15 {
		t.Fatalf("'层' 笔画像素过少: %d（疑似栅格化退化）", n)
	}
}

// TestCSDSysGlyphCache verifies repeated lookups return identical glyphs
// (pure cache hit after the first rasterization).
func TestCSDSysGlyphCache(t *testing.T) {
	g1, ok1 := csdSysGlyph('层')
	g2, ok2 := csdSysGlyph('层')
	if ok1 != ok2 {
		t.Fatalf("缓存一致性破坏: ok1=%v ok2=%v", ok1, ok2)
	}
	if !ok1 {
		t.Skipf("系统无 CJK 字体，跳过")
	}
	if g1.advance != g2.advance || g1.width != g2.width || g1.height != g2.height {
		t.Fatalf("两次取字形不一致: %+v vs %+v", g1, g2)
	}
	for i := range g1.alpha {
		if g1.alpha[i] != g2.alpha[i] {
			t.Fatalf("字形 alpha 数据不一致 at %d: %d vs %d", i, g1.alpha[i], g2.alpha[i])
		}
	}
}

// TestBlendCSDGlyph verifies alpha blending into an opaque ARGB8888 buffer
// (independent of any system font — hand-built glyph).
func TestBlendCSDGlyph(t *testing.T) {
	buf := make([]byte, 10*10*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = 0x2B, 0x2D, 0x30, 0xFF // BGRA
	}
	g := csdGlyph{width: 2, height: 2, alpha: []byte{255, 128, 0, 255}}
	c := [4]byte{0xE5, 0xE1, 0xDF, 0xFF}
	blendCSDGlyph(buf, 10, 1, 1, g, c)

	off := (1*10 + 1) * 4 // (1,1) alpha=255 → pure foreground
	if buf[off] != 0xE5 || buf[off+1] != 0xE1 || buf[off+2] != 0xDF {
		t.Fatalf("opaque 混合错误: got %v want [E5 E1 DF]", buf[off:off+3])
	}
	off2 := (1*10 + 2) * 4 // (2,1) alpha=128 → halfway blend
	want := byte((0xE5 + 0x2B) / 2)
	if buf[off2] != want || buf[off2+1] != byte((0xE1+0x2D)/2) || buf[off2+2] != byte((0xDF+0x30)/2) {
		t.Fatalf("半透明混合错误: got %v", buf[off2:off2+3])
	}
	off3 := (2*10 + 1) * 4 // (1,2) alpha=0 → background untouched
	if buf[off3] != 0x2B || buf[off3+1] != 0x2D || buf[off3+2] != 0x30 {
		t.Fatalf("alpha=0 应保持背景: got %v", buf[off3:off3+3])
	}
	// Opaque alpha channel stays untouched.
	if buf[off+3] != 0xFF {
		t.Fatalf("alpha 通道被改动: %d", buf[off+3])
	}
}

// TestDrawTitleTextMixed renders an ASCII+CJK title into a 1200x32 title bar
// and verifies the CJK rune lands after the ASCII prefix with real strokes
// (not the '?' placeholder shape).
func TestDrawTitleTextMixed(t *testing.T) {
	g, ok := csdSysGlyph('层')
	if !ok {
		t.Skipf("系统无 CJK 字体，跳过")
	}
	const w, h = 1200, 32
	buf := make([]byte, w*h*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = 0x2B, 0x2D, 0x30, 0xFF
	}
	drawTitleText(buf, w, h, "a层b", csdColorIcon, w-3*csdButtonW)

	textW := 7 + g.advance + 7
	x := (w - textW) / 2
	// ASCII 'a' occupies x..x+6 (bitmap glyph, 7px).
	aPx := 0
	for py := 0; py < h; py++ {
		for col := 0; col < 7; col++ {
			o := (py*w + x + col) * 4
			if buf[o] == 0xE5 {
				aPx++
			}
		}
	}
	if aPx == 0 {
		t.Fatalf("ASCII 'a' 未绘制（x=%d）", x)
	}
	// CJK '层' occupies x+7 .. x+7+g.width. Glyph pixels are alpha-blended
	// (anti-aliased), so assert "clearly brighter than the background"
	// (bg B=0x2B, fg B=0xE5) instead of an exact match.
	cjkPx := 0
	for py := 0; py < h; py++ {
		for col := 0; col < g.width; col++ {
			px := x + 7 + col
			if px < 0 || px >= w {
				continue
			}
			o := (py*w + px) * 4
			if buf[o] > 0x60 {
				cjkPx++
			}
		}
	}
	if cjkPx == 0 {
		t.Fatalf("'层' 未绘制到标题栏 buffer（x=%d adv=%d）", x, g.advance)
	}
	// The trailing 'b' must start at x+7+g.advance — ensure it exists
	// beyond the CJK glyph (no overlap): count pixels in that band.
	bPx := 0
	for py := 0; py < h; py++ {
		for col := 0; col < 7; col++ {
			px := x + 7 + g.advance + col
			if px < 0 || px >= w {
				continue
			}
			o := (py*w + px) * 4
			if buf[o] == 0xE5 {
				bPx++
			}
		}
	}
	if bPx == 0 {
		t.Fatalf("尾随 ASCII 'b' 未绘制（x=%d adv=%d）", x, g.advance)
	}
}

// --- Title-bar paint benchmarks (1200x32 title bar, real title strings) ---

func benchTitleBar() []byte {
	buf := make([]byte, 1200*32*4)
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+1], buf[i+2], buf[i+3] = 0x2B, 0x2D, 0x30, 0xFF
	}
	return buf
}

// BenchmarkDrawTitleTextASCII: pure-ASCII title (the common case) — no
// system-font load, no glyph cache traffic.
func BenchmarkDrawTitleTextASCII(b *testing.B) {
	buf := benchTitleBar()
	const text = "gpui ui_wr_r4_composite - Composite Present"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawTitleText(buf, 1200, 32, text, csdColorIcon, 1200-3*csdButtonW)
	}
}

// BenchmarkDrawTitleTextMixedHot: ASCII+CJK title with the system glyphs
// already cached (steady-state repaints: hover / press / title change).
func BenchmarkDrawTitleTextMixedHot(b *testing.B) {
	buf := benchTitleBar()
	const text = "gpui ui_wr_r4_composite — 层 Composite Present"
	if _, ok := csdSysGlyph('层'); !ok {
		b.Skipf("系统无 CJK 字体，跳过")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		drawTitleText(buf, 1200, 32, text, csdColorIcon, 1200-3*csdButtonW)
	}
}

// BenchmarkDrawTitleTextMixedCold: ASCII+CJK title with uncached system
// glyphs (first paint after a new title) — rasterization included.
func BenchmarkDrawTitleTextMixedCold(b *testing.B) {
	buf := benchTitleBar()
	// Rotate through distinct CJK runes so each iteration rasterizes fresh
	// glyphs (cache grows; measures per-glyph rasterize + blend cost).
	const runes = "层序逻辑上叠云而间"
	csdSysGlyph('层') // ensure a face is loaded (candidates probed once)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := runes[i%len(runes)]
		text := "gpui 层 " + string(r) + " Composite"
		drawTitleText(buf, 1200, 32, text, csdColorIcon, 1200-3*csdButtonW)
	}
}

// BenchmarkCSDSysGlyphCold: forced-uncached rasterization of a CJK glyph
// (deletes the cache entry each iteration). First paint after a new title.
func BenchmarkCSDSysGlyphCold(b *testing.B) {
	csdSysGlyph('层') // ensure a face is loaded (candidates probed once)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := rune(0x4E00 + i%600)
		csdSysFontState.Lock()
		delete(csdSysFontState.cache, r)
		csdSysFontState.Unlock()
		csdSysGlyph(r)
	}
}

// BenchmarkBlendCSDGlyph: alpha-blend one cached 13x13 glyph into the buffer.
func BenchmarkBlendCSDGlyph(b *testing.B) {
	buf := benchTitleBar()
	g, ok := csdSysGlyph('层')
	if !ok {
		b.Skipf("系统无 CJK 字体，跳过")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blendCSDGlyph(buf, 1200, 593, 9, g, csdColorIcon)
	}
}

