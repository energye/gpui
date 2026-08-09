package text

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

func TestDbgJing(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	f := src.Parsed().(*ownParsedFont)
	r := '静'
	gid := f.GlyphIndex(r)
	for _, px := range []float64{12, 16} {
		pts, contours, _, err := hint.LightHintVar(f.RawFontData(), f.collectionIndex, false, uint16(gid), px, nil)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Printf("==== 静 %.0fpx pts=%d contours=%v ====\n", px, len(pts), contours)
		first := 0
		for ci, n := range contours {
			fmt.Printf("轮廓%d [%d..%d]\n", ci, first, first+n-1)
			for i := first; i < first+n; i++ {
				p := pts[i]
				fmt.Printf("  %d (%d,%d) on=%v\n", i, p.X, p.Y, p.On)
			}
			first += n
		}
	}
}
