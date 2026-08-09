package text

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

func TestDbgHintPts(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	f := src.Parsed().(*ownParsedFont)
	r := '合'
	gid := f.GlyphIndex(r)
	for _, px := range []float64{16, 24} {
		pts, contours, w, err := hint.LightHint(f.RawFontData(), f.collectionIndex, false, uint16(gid), px)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("==== 合 %.0fpx LightHint pts=%d contours=%v w=%.1f ====\n", px, len(pts), contours, w)
		for i, p := range pts {
			fmt.Printf("  %d (%d,%d) on=%v\n", i, p.X, p.Y, p.On)
		}
	}
}
