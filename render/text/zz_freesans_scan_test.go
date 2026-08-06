package text

import (
	"os"
	"strings"
	"testing"
)

// TestScanFreeSansLatin：M4b-4 全字集回归窗。
//
// 字表 = hint/testdata/latin_all.txt（L1 254 字 = ASCII 95 + 重音 29 +
// confusable 17 + 西里尔 66 + 希腊 48）。对照字体 = wqy-microhei-nohint.ttf
// （glyf 无字节码；FT 按 unicode 属性对 Latin 字形走 aflatin，实测 'a' light
// 59 vs nohint 57 有差异）。对照 = ftexp bcontour 批量模式取 FT-light 26.6
// 轮廓，与 ExtractOutlineHinted 逐点对照。
func TestScanFreeSansLatin(t *testing.T) {
	raw, err := os.ReadFile("hint/testdata/latin_all.txt")
	if err != nil {
		t.Skipf("latin_all.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != 254 {
		t.Fatalf("latin_all.txt = %d chars (after TrimSpace), want 254", len(chars))
	}
	fontPath := "testdata/wqy-microhei-nohint.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("wqy-microhei-nohint.ttf unavailable: %v", err)
	}
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	face := src.Parsed()
	ext := NewOutlineExtractor()

	for _, px := range []int{10, 12, 14, 16} {
		ft := batchWqyContour26(t, fontPath, chars, float64(px))
		if len(ft) < 200 {
			t.Fatalf("freesans px%d: only %d glyphs parsed, want ≥200 (parser regression)", px, len(ft))
		}
		bad, npt := 0, 0
		badR, nptR := []rune{}, []rune{}
		for _, r := range chars {
			ftFound, ok := ft[r]
			if !ok {
				continue
			}
			gid := face.GlyphIndex(r)
			out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
			if err != nil || out == nil {
				continue
			}
			gotPts := outlinePoints26(out)
			// outlinePoints26 filters (0,0) points (CJK degenerate-point hack);
			// filter the FT side the same way so real origin vertices
			// (e.g. 'A' lower-left corner at 0,0) compare symmetrically.
			ftClean := make([][2]int64, 0, len(ftFound))
			for _, q := range ftFound {
				if q[0] == 0 && q[1] == 0 {
					continue
				}
				ftClean = append(ftClean, q)
			}
			if len(ftClean) > 0 && len(gotPts) == 0 {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			matched := matchPointSets(ftClean, gotPts, 64)
			if matched < len(ftClean) && matched < len(gotPts) {
				bad++
				if len(badR) < 30 {
					badR = append(badR, r)
				}
				continue
			}
			_ = npt
		}
		t.Logf("freesans px%d: bad=%d/%d npt=%d nptR=%d first=%v", px, bad, len(chars), npt, len(nptR), runes8(nptR))
		t.Logf("freesans px%d: badR=%v", px, runes8(badR))
	}
}
