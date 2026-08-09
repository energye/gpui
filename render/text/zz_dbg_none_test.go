package text

import (
	"fmt"
	"testing"
)

func TestDbgNoneHint(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	for _, hint := range []Hinting{HintingNone, HintingVertical} {
		for _, px := range []float64{16, 24} {
			r := '合'
			gid := GlyphID(parsed.GlyphIndex(r))
			ext := NewOutlineExtractor()
			o, _ := ext.ExtractOutlineHinted(parsed, gid, px, hint)
			minY, maxY := float32(1e9), float32(-1e9)
			for _, s := range o.Segments {
				for _, p := range s.Points {
					if p.X != 0 || p.Y != 0 || (s.Op == OutlineOpMoveTo && len(s.Points) > 0) {
						if p.Y < minY {
							minY = p.Y
						}
						if p.Y > maxY {
							maxY = p.Y
						}
					}
				}
			}
			fmt.Printf("合 %.0fpx hint=%v: Y∈[%.2f,%.2f] 高%.1f\n", px, hint, minY, maxY, maxY-minY)
		}
	}
}
