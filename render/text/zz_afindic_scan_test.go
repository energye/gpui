package text

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestScanM4b5Indic(t *testing.T) {
	fonts := map[string]string{
		"deva": "Samyak-Devanagari.ttf",
		"beng": "Mukti.ttf",
		"taml": "Samyak-Tamil.ttf",
	}
	for name, fn := range fonts {
		fp := findSystemFont(t, fn)
		if fp == "" {
			t.Logf("%s: font not installed", name)
			continue
		}
		raw, err := os.ReadFile("hint/testdata/" + name + "_sample.txt")
		if err != nil {
			t.Skipf("%s: %v", name, err)
		}
		chars := []rune(strings.TrimSpace(string(raw)))
		src, _ := NewFontSourceFromFile(fp)
		face := src.Parsed()
		ext := NewOutlineExtractor()
		for _, px := range []int{12, 16} {
			ft := batchWqyContour26(t, fp, chars, float64(px))
			bad := []string{}
			for _, r := range chars {
				ftPts, ok := ft[r]
				if !ok {
					continue
				}
				out, err := ext.ExtractOutlineHinted(face, GlyphID(face.GlyphIndex(r)), float64(px), HintingVertical)
				if err != nil || out == nil {
					bad = append(bad, fmt.Sprintf("%q:extract-err", r))
					continue
				}
				g := outlinePoints26(out)
				if n := len(g); n > 1 && g[0] == g[n-1] {
					g = g[:n-1]
				}
				ftClean := make([][2]int64, 0, len(ftPts))
				for _, q := range ftPts {
					if q[0] == 0 && q[1] == 0 {
						continue
					}
					ftClean = append(ftClean, q)
				}
				if len(ftClean) == 0 || len(g) == 0 {
					continue
				}
				m := matchPointSets(ftClean, g, 64)
				if m < len(ftClean) && m < len(g) {
					bad = append(bad, fmt.Sprintf("%q(%d/%d)", r, m, len(ftClean)))
				}
			}
			t.Logf("%s px%d: bad=%d/%d %v", name, px, len(bad), len(chars), bad)
			if len(bad) > 0 {
				t.Errorf("%s px%d: bad=%d/%d (must be 0)", name, px, len(bad), len(chars))
			}
		}
		src.Close()
	}
}
