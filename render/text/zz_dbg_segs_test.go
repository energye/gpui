package text

import (
	"fmt"
	"testing"
)

func TestDbgDumpSegs(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	for _, px := range []float64{16, 24, 48} {
		r := '合'
		gid := GlyphID(parsed.GlyphIndex(r))
		ext := NewOutlineExtractor()
		o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
		if err != nil || o == nil {
			t.Fatalf("%c %.0fpx outline: %v", r, px, err)
		}
		fmt.Printf("\n==== %c %.0fpx 生产轮廓 segs=%v bounds=(%v..%v,%v..%v) 单位26.6? 直接打印点 ====\n",
			r, px, len(o.Segments), o.Bounds.MinX, o.Bounds.MaxX, o.Bounds.MinY, o.Bounds.MaxY)
		minY, maxY := float32(1<<20), float32(-1<<20)
		for _, s := range o.Segments {
			for _, p := range s.Points {
				if p.Y == 0 && p.X == 0 && s.Op != OutlineOpMoveTo {
					continue
				}
				if p.X != 0 || p.Y != 0 {
					if p.Y < minY {
						minY = p.Y
					}
					if p.Y > maxY {
						maxY = p.Y
					}
				}
			}
		}
		fmt.Printf("minY=%.1f maxY=%.1f (预期如合高≈%.0f, 底部横线应在 maxY 附近)\n", minY, maxY, px)
		// 找出底横线：y 在 maxY-0.75..maxY 范围内的线段
		count := 0
		for _, s := range o.Segments {
			var ys [3]float32
			valid := 0
			for i, p := range s.Points {
				if i == 0 && s.Op == OutlineOpMoveTo {
					continue
				}
				if p.X != 0 || p.Y != 0 {
					ys[valid] = p.Y
					valid++
				}
			}
			for i := 0; i < valid; i++ {
				if ys[i] >= maxY-1.5 && ys[i] <= maxY {
					count++
					break
				}
			}
		}
		fmt.Printf("底部 1.5px 内线段数: %v\n", count)
	}
}
