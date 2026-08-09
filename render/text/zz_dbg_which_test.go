package text

import (
	"fmt"
	"testing"
)

func TestDbgWhichPath(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	f := src.Parsed().(*ownParsedFont)
	r := '合'
	gid := f.GlyphIndex(r)
	fmt.Println("hasPostScriptOutlines:", f.hasPostScriptOutlines())
	fmt.Println("hasCFF:", f.hasCFFTable(), "hasCFF2:", f.hasCFF2Table(), "hasGlyf:", false)

	// 路径1：直接 cffLightHintOutline
	ext := NewOutlineExtractor()
	lo, ok := ext.cffLightHintOutline(f, GlyphID(gid), 16)
	fmt.Println("cffLightHintOutline ok:", ok)
	if ok {
		minY, maxY := float32(1e9), float32(-1e9)
		for _, s := range lo.Segments {
			for _, p := range s.Points {
				if p.X != 0 || p.Y != 0 || s.Op == OutlineOpMoveTo {
					if p.Y < minY {
						minY = p.Y
					}
					if p.Y > maxY {
						maxY = p.Y
					}
				}
			}
		}
		fmt.Printf("cffLightHint 输出 Y∈[%.2f,%.2f]\n", minY, maxY)
	}

	// 路径2：完整 ExtractOutlineHinted
	o, err := ext.ExtractOutlineHinted(f, GlyphID(gid), 16, HintingVertical)
	fmt.Println("ExtractOutlineHinted err:", err)
	if o != nil {
		minY, maxY := float32(1e9), float32(-1e9)
		for _, s := range o.Segments {
			for _, p := range s.Points {
				if p.X != 0 || p.Y != 0 || s.Op == OutlineOpMoveTo {
					if p.Y < minY {
						minY = p.Y
					}
					if p.Y > maxY {
						maxY = p.Y
					}
				}
			}
		}
		fmt.Printf("ExtractOutlineHinted Y∈[%.2f,%.2f]\n", minY, maxY)
	}
}
