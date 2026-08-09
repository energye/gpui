package text

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

func TestDbgWhyFail2(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	f := src.Parsed().(*ownParsedFont)
	r := '合'
	gid := f.GlyphIndex(r)
	raw := f.RawFontData()
	isCFF2 := f.hasCFF2Table() && !f.hasCFFTable()
	fmt.Println("isCFF2:", isCFF2)
	pts, contours, _, err := hint.LightHintVar(raw, f.collectionIndex, isCFF2, uint16(gid), 16, nil)
	fmt.Println("内部调用: err:", err, "pts:", len(pts), "contours:", len(contours))

	segments := rebuildSegmentsFromLightPts(pts, contours)
	fmt.Println("segments:", len(segments))
	if len(segments) == 0 {
		t.Fatal("no segments")
	}
	outline := &GlyphOutline{Segments: segments, GID: GlyphID(gid), Type: GlyphTypeOutline, Advance: float32(f.GlyphAdvance(uint16(gid), 16))}
	refreshOutlineBounds(outline)
	fmt.Println("bounds:", outline.Bounds)
}
