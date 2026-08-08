package text

import (
	"os"
	"strings"
	"testing"
)

// TestScanBytecodeLatinLight：L0/L1 tt_engine light 分派判定的回归窗。
//
// 背景（2026-08-07 实证）：FT 2.11.1 的 FT_LOAD_TARGET_LIGHT 对 glyf 字体
// （无论是否带 fpgm/prep 字节码）一律走 auto-hinter，不调用 TT 字节码解释器
// （truetype 驱动不声明 FT_MODULE_DRIVER_HINTS_LIGHTLY，ftobjs.c:940 判定
// autohint=TRUE）。ftexp bcontour 实测：DejaVuSans/FreeSans/TlwgTypo（均带
// fpgm+prep）的 LIGHT 输出 == LIGHT|FORCE_AUTOHINT 输出（全 SAME）。
//
// 因此 L0 不再给 tt_engine 加 InterpLight 字节码分支，而是把 HintingVertical
// （映射 FT light）对字节码字体的分派改为改道至 autofit light —— 与
// glyph_outline.go ExtractOutlineHinted 的修改（bytecode 拦截仅限
// HintingFull/NORMAL）对齐。本窗用带字节码的真实 Latin 系统字体对照 ftexp
// light：E/H/L/1/i/0/l × 8/12/16px 逐字，及 latin_all.txt 254 字 × 12px。
func TestScanLightBytecodeLatin(t *testing.T) {
	raw, err := os.ReadFile("hint/testdata/latin_all.txt")
	if err != nil {
		t.Skipf("latin_all.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) < 200 {
		t.Fatalf("latin_all.txt too short: %d chars", len(chars))
	}

	latin := "EHL1il0"
	sizes := []int{8, 12, 16}
	fonts := []struct {
		name string
		path string
	}{
		{"DejaVuSans", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"},
		{"FreeSans", "/usr/share/fonts/truetype/freefont/FreeSans.ttf"},
	}
	for _, f := range fonts {
		if _, err := os.Stat(f.path); err != nil {
			t.Skipf("%s unavailable: %v", f.name, err)
		}
		src, err := NewFontSourceFromFile(f.path)
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		face := src.Parsed()
		ext := NewOutlineExtractor()

		// L0 核心字形 × 3 字号
		for _, px := range sizes {
			lr := []rune(latin)
			ft := batchWqyContour26(t, f.path, lr, float64(px))
			bad := 0
			var badR []rune
			for _, r := range lr {
				ftFound, ok := ft[r]
				if !ok {
					continue
				}
				gid := face.GlyphIndex(r)
				out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
				if err != nil || out == nil {
					bad++
					badR = append(badR, r)
					continue
				}
				gotPts := outlinePoints26(out)
				if len(ftFound) > 0 && len(gotPts) == 0 {
					bad++
					badR = append(badR, r)
					continue
				}
				matched := matchPointSets(ftFound, gotPts, 64)
				if matched < len(ftFound) && matched < len(gotPts) {
					bad++
					if len(badR) < 30 {
						badR = append(badR, r)
					}
				}
			}
			t.Logf("%s L0 px%d: bad=%d/%d badR=%v", f.name, px, bad, len(lr), runes8(badR))
			if bad > 0 {
				t.Errorf("%s L0 px%d: bad=%d/%d (must be 0)", f.name, px, bad, len(lr))
			}
		}

		// L1 全字集 254 字 × 12px
		px := 12
		ft := batchWqyContour26(t, f.path, chars, float64(px))
		bad := 0
		nft := 0
		var badR []rune
		for _, r := range chars {
			ftFound, ok := ft[r]
			if !ok {
				continue
			}
			nft++
			gid := face.GlyphIndex(r)
			out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
			if err != nil || out == nil {
				bad++
				badR = append(badR, r)
				continue
			}
			gotPts := outlinePoints26(out)
			if len(ftFound) > 0 && len(gotPts) == 0 {
				bad++
				badR = append(badR, r)
				continue
			}
			matched := matchPointSets(ftFound, gotPts, 64)
			if matched < len(ftFound) && matched < len(gotPts) {
				bad++
				if len(badR) < 30 {
					badR = append(badR, r)
				}
			}
		}
		t.Logf("%s L1 12px: bad=%d/%d glyphs matched=%d badR=%v", f.name, bad, len(chars), nft, runes8(badR))
		if bad > 0 {
			t.Errorf("%s L1 12px: bad=%d/%d (must be 0)", f.name, bad, len(chars))
		}
	}
}
