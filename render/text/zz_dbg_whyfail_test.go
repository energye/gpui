package text

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

func TestDbgWhyFail(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	f := src.Parsed().(*ownParsedFont)
	r := '合'
	gid := f.GlyphIndex(r)
	fmt.Println("collectionIndex:", f.collectionIndex, "gid:", gid, "rawlen:", len(f.RawFontData()))
	pts, contours, w, err := hint.LightHintVar(f.RawFontData(), f.collectionIndex, false, uint16(gid), 16, nil)
	fmt.Println("LightHintVar err:", err, "pts:", len(pts), "contours:", len(contours), "w:", w)
	if err != nil {
		fmt.Println("raw head bytes:", f.RawFontData()[:32])
	}
}
