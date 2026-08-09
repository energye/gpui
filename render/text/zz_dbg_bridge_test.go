package text

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

// TestDbgBridgeFidelity 验证 cff_light_bridge 的 rebuildSegmentsFromLightPts
// 是否忠实于 hint.LightPt 序列（M3 已对该序列全绿）。探针比对：
//  1. hint.LightHintVar 原始 pts（X 26.6 / Y 26.6 Y-up）
//  2. 生产 ExtractOutlineHinted → segs 重建后的点列（X px, Y-down px 反 26.6）
//
// 每段只取「终点」；off 点（QuadTo 控制点）保留；比较时把 segs 串回
// (X*64, -Y*64) 与 pts 逐点对。
func TestDbgBridgesFidelity(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	own := parsed.(*ownParsedFont)

	for _, px := range []float64{8, 12, 16, 24, 48} {
		for _, r := range []rune("静每合") {
			gid := GlyphID(parsed.GlyphIndex(r))
			raw := own.RawFontData()
			isCFF2 := own.hasCFF2Table() && !own.hasCFFTable()
			pts, _, _, err := hint.LightHintVar(raw, own.collectionIndex, isCFF2, uint16(gid), px, nil)
			if err != nil {
				t.Fatalf("hint direct %c %.0fpx: %v", r, px, err)
			}
			ext := NewOutlineExtractor()
			o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
			if err != nil || o == nil {
				t.Fatalf("prod %c %.0fpx: %v", r, px, err)
			}

			// 串起 segs：LineTo/QuadTo/CubicTo 的终点 + 控制点，转 26.6
			var got []struct{ x, y int64 }
			appendPt := func(p OutlinePoint) {
				got = append(got, struct{ x, y int64 }{int64(p.X * 64), int64(-p.Y * 64)})
			}
			for _, s := range o.Segments {
				switch s.Op {
				case OutlineOpMoveTo:
					appendPt(s.Points[0])
				case OutlineOpLineTo:
					appendPt(s.Points[0])
				case OutlineOpQuadTo:
					appendPt(s.Points[0])
					appendPt(s.Points[1])
				case OutlineOpCubicTo:
					appendPt(s.Points[0])
					appendPt(s.Points[1])
					appendPt(s.Points[2])
				}
			}
			// 直接 pts 序列（每点含 X,Y 26.6）
			want := pts
			if len(got) != len(want) {
				fmt.Printf("Npts %c %.0fpx: prodSegs=%d hintPts=%d\n", r, px, len(got), len(want))
				continue
			}
			bad := 0
			for i, p := range got {
				if p.x != want[i].X || p.y != want[i].Y {
					if bad < 8 {
						fmt.Printf("pt%d %c %.0fpx: segs=(%d,%d) pts=(%d,%d)\n", i, r, px, p.x, p.y, want[i].X, want[i].Y)
					}
					bad++
				}
			}
			if bad > 0 {
				fmt.Printf("BAD %c %.0fpx: %d/%d pts differ\n", r, px, bad, len(want))
			} else {
				fmt.Printf("OK  %c %.0fpx: %d pts 桥接忠实\n", r, px, len(want))
			}
		}
	}
}