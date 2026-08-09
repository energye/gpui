package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"testing"
)

func TestDbgDumpASCII(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	for _, px := range []float64{16} {
		for _, r := range []rune("合") {
			gid := GlyphID(parsed.GlyphIndex(r))
			ext := NewOutlineExtractor()
			o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
			if err != nil || o == nil {
				t.Fatalf("%c %.0fpx outline: %v", r, px, err)
			}
			ras := NewGlyphMaskRasterizer()
			res, err := ras.RasterizeOutline(o, 0, 0)
			if err != nil || res == nil {
				t.Fatalf("%c %.0fpx raster: %v", r, px, err)
			}
			fmt.Printf("\n==== %c %.0fpx 自研光栅 (%dx%d) ====\n", r, px, res.Width, res.Height)
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
						line += "o"
					case v > 0:
						line += "."
					default:
						line += " "
					}
				}
				fmt.Println(line)
			}

			ftPgm := t.TempDir() + "/ft.pgm"
			cmd := exec.Command(ftexpLocal(t), fontPath, string(r), strconv.Itoa(int(px)), "l", ftPgm)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("ftexp: %v %s", err, out)
			}
			fw, fh, fv := readPgm2(ftPgm)
			fmt.Printf("==== %c %.0fpx FT-light 位图 (%dx%d) ====\n", r, px, fw, fh)
			for y := 0; y < fh; y++ {
				line := ""
				for x := 0; x < fw; x++ {
					v := fv[y*fw+x]
					switch {
					case v >= 200:
						line += "#"
					case v >= 128:
						line += "O"
					case v >= 40:
						line += "."
					case v > 0:
						line += " "
					default:
						line += " "
					}
				}
				fmt.Println(line)
			}
		}
	}
}

func TestDbgDumpFTContourASCII(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	for _, px := range []float64{16} {
		for _, r := range []rune("合") {
			ras := NewGlyphMaskRasterizer()
			ftOutline := ftOutlineToGlyphSimpler(t, fontPath, r, px)
			res, err := ras.RasterizeOutline(ftOutline, 0, 0)
			if err != nil || res == nil {
				t.Fatalf("%c %.0fpx raster: %v", r, px, err)
			}
			fmt.Printf("==== %c %.0fpx FT轮廓→自研光栅 (%dx%d) ====\n", r, px, res.Width, res.Height)
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
}
