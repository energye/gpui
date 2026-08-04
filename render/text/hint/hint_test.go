package hint

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
)

// reproFont 加载系统 CJK+Latin 对照字体（与本轮验证链路一致）。
func reproFont(t *testing.T) (cjk text.ParsedFont, latin text.ParsedFont) {
	t.Helper()
	cjkSrc, err := text.NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	if err != nil {
		t.Skipf("CJK font unavailable: %v", err)
	}
	cjk = cjkSrc.Face(14).Source().Parsed()
	latSrc, err := text.NewFontSourceFromFile("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	if err != nil {
		t.Skipf("latin font unavailable: %v", err)
	}
	latin = latSrc.Face(14).Source().Parsed()
	return
}

// TestNew 验证引擎可创建。
func TestNew(t *testing.T) {
	e := New()
	if e == nil || e.extractor == nil {
		t.Fatal("New() returned bad engine")
	}
}

// TestHintNilFont 验证空字体报错。
func TestHintNilFont(t *testing.T) {
	e := New()
	for _, m := range []Mode{ModeLightCJK, ModeLightLatin} {
		if _, err := e.Hint(nil, 0, 14, m); err == nil {
			t.Fatalf("mode %v: expected error for nil font", m)
		}
	}
}

// TestHintSkeleton 骨架阶段：合法字体返回非空轮廓（未拟合 = FT-nohint 等价）。
func TestHintSkeleton(t *testing.T) {
	cjk, latin := reproFont(t)
	e := New()
	cases := []struct {
		name string
		font text.ParsedFont
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
			out, err := e.Hint(tt.font, text.GlyphID(gid), 14, tt.mode)
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

// outlineTop 返回字形最高点（Y-up 像素）。
func outlineTop(t *testing.T, out *text.GlyphOutline) float64 {
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

// TestM0TopBlueAnchor M0：有顶横字（日/田/目）顶横锚定蓝线（12px→10.0px）。
func TestM0TopBlueAnchor(t *testing.T) {
	cjk, _ := reproFont(t)
	e := New()
	for _, r := range []rune{'日', '田', '目'} {
		gid := cjk.GlyphIndex(r)
		out, err := e.Hint(cjk, text.GlyphID(gid), 12, ModeLightCJK)
		if err != nil {
			t.Fatalf("Hint %q: %v", r, err)
		}
		if got := outlineTop(t, out); got != 10.0 {
			t.Errorf("%q M0 top = %.3f, want 10.0", r, got)
		}
	}
}

// TestM0UnanchoredTop 无直线顶横的字（钮，曲线顶部）M0 不锚（保持原轮廓 top）。
func TestM0UnanchoredTop(t *testing.T) {
	cjk, _ := reproFont(t)
	e := New()
	base, err := text.NewOutlineExtractor().ExtractOutline(cjk, text.GlyphID(cjk.GlyphIndex('钮')), 12)
	if err != nil {
		t.Fatal(err)
	}
	hinted, err := e.Hint(cjk, text.GlyphID(cjk.GlyphIndex('钮')), 12, ModeLightCJK)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := outlineTop(t, hinted), outlineTop(t, base); got != want {
		t.Errorf("钮 top changed by M0: %v → %v (want unchanged)", want, got)
	}
}

// TestUnknownMode 未知模式报错。
func TestUnknownMode(t *testing.T) {
	e := New()
	if _, err := e.Hint(nil, 0, 14, Mode(99)); err == nil {
		t.Fatal("unknown mode should error")
	}
}

// TestVerifierSet 字集完备性：非空、分组结构、字符统计。
func TestVerifierSet(t *testing.T) {
	gs := VerifierSet()
	if len(gs) == 0 {
		t.Fatal("verifier set empty")
	}
	total := 0
	seen := map[string]bool{}
	for _, g := range gs {
		if g.Name == "" || g.Chars == "" {
			t.Fatalf("group has empty name or chars: %+v", g)
		}
		if seen[g.Name] {
			t.Fatalf("duplicate group name %q", g.Name)
		}
		seen[g.Name] = true
		n := len([]rune(g.Chars))
		total += n
		t.Logf("%-22s runes=%3d  %s", g.Name, n, g.Desc)
	}
	t.Logf("TOTAL  runes=%d", total)
	// 覆盖语言族齐全
	for _, want := range []string{"cjk-", "latin-", "thai-", "arabic-", "cyrillic", "greek"} {
		has := false
		for _, g := range gs {
			if strings.HasPrefix(g.Name, want) {
				has = true
			}
		}
		if !has {
			t.Errorf("missing script group prefix %q", want)
		}
	}
}
