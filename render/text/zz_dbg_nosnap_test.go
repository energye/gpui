package text

import (
	"fmt"
	"testing"
)

func TestDbgNoSnap(t *testing.T) {
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
	oNo := &GlyphOutline{Segments: append([]OutlineSegment(nil), o.Segments...), Type: o.Type}
	refreshOutlineBounds(oNo)
	applyGridFit(oNo, map[float32]float32{}, HintingVertical)

	ras := NewGlyphMaskRasterizer()
	for _, oo := range []*GlyphOutline{oNo, o} {
		res, err := ras.RasterizeOutline(oo, 0, 0)
		if err != nil || res == nil {
			t.Fatalf("raster: %v", err)
		}
		fmt.Printf("==== %c %.0fpx mask %dx%d ====\n", r, px, res.Width, res.Height)
		for y := 0; y < res.Height; y++ {
			line := ""
			for x := 0; x < res.Width; x++ {
				v := res.Mask[y*res.Width+x]
				switch {
				case v >= 200:
					line += "#"
				case v >= 128:
					line += "O"
				case v >= 40:
					line += "."
				default:
					line += " "
				}
			}
			fmt.Println(line)
		}
	}
}
