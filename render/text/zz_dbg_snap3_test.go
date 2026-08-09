package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"testing"
)

func TestDbgSnap3(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	ras := NewGlyphMaskRasterizer()
	ext := NewOutlineExtractor()

	for _, c := range []struct{ r rune; px float64 }{{'静', 12}, {'每', 12}, {'合', 16}} {
		fmt.Printf("\n######## %q %.0fpx ########\n", c.r, c.px)
		gid := GlyphID(parsed.GlyphIndex(c.r))

		// A: 生产 Vertical（现缺陷路径）
		oA, _ := ext.ExtractOutlineHinted(parsed, gid, c.px, HintingVertical)
		resA, _ := ras.RasterizeOutline(oA, 0, 0)

		// B: None 提取 + 只做 snap（同 Vertical 的 snap 逻辑，无 hint 引擎介入）
		oB, _ := ext.ExtractOutlineHinted(parsed, gid, c.px, HintingNone)

		// C: FT 位图
		ftPgm := t.TempDir() + "/ft.pgm"
		cmd := exec.Command(ftexpLocal(t), fontPath, string(c.r), strconv.Itoa(int(c.px)), "l", ftPgm)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("ftexp: %v %s", err, out)
		}
		fw, fh, fv := readPgm2(ftPgm)

		// 统计两两差异（bestShift）
		resB, _ := ras.RasterizeOutline(oB, 0, 0)
		uv := func(m []byte, w, h int) int { return bestShiftMism(m, w, h, fv, fw, fh) }
		fmt.Printf("生产Vertical vs FT位图 %d\n", uv(resA.Mask, resA.Width, resA.Height))
		fmt.Printf("None+不snap  vs FT位图 %d\n", uv(resB.Mask, resB.Width, resB.Height))
	}
}

func TestDbgSnap3Dump(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	px := 12.0
	r := '静'
	ext := NewOutlineExtractor()
	for _, hint := range []Hinting{HintingVertical, HintingNone} {
		o, _ := ext.ExtractOutlineHinted(parsed, GlyphID(parsed.GlyphIndex(r)), px, hint)
		ras := NewGlyphMaskRasterizer()
		res, _ := ras.RasterizeOutline(o, 0, 0)
		fmt.Printf("==== %q %.0fpx hint=%v (%dx%d) ====\n", r, px, hint, res.Width, res.Height)
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
