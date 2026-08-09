package text

import (
	"fmt"
	"testing"
)

func TestDbgBoxSegs(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	px := 16.0
	r := '合'
	gid := GlyphID(parsed.GlyphIndex(r))
	ext := NewOutlineExtractor()
	o, _ := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
	for i, s := range o.Segments {
		var xs, ys []float32
		for _, p := range s.Points {
			if p.X != 0 || p.Y != 0 || s.Op == OutlineOpMoveTo {
				xs = append(xs, p.X)
				ys = append(ys, p.Y)
			}
		}
		for j := 0; j < len(xs); j++ {
			fmt.Printf("[%d] %s (%.1f,%.1f)", i, s.Op, xs[j], ys[j])
		}
		fmt.Println()
	}
}
