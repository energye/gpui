package text

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// 阶段 A3：CFF light 26.6 直通路径（RasterizeHintedFT26）vs FT_Render_Glyph
// 位图逐字节对照。验证 glyph_mask_rasterizer.go 的直通方法（方案 A：
// hint.LightHintVar 26.6 → RasterizeFT26，跳过 float32 中转）。
func TestDbgFTGraysFT26Direct(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("font: %v", err)
	}
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatalf("font source: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	bin := ftexpLocal(t)

	badTotal := 0
	glyphs := 0
	for _, px := range []float64{8, 12, 16, 24, 32, 48} {
		for _, r := range []rune("静每合日田一二三") {
			gid := GlyphID(parsed.GlyphIndex(r))
			if gid == 0 {
				continue
			}
			rast := NewGlyphMaskRasterizer()
			res, direct, err := rast.RasterizeHintedFT26(parsed, gid, px, HintingVertical, nil)
			if err != nil {
				t.Fatalf("%c %.0fpx: %v", r, px, err)
			}
			if !direct {
				t.Fatalf("%c %.0fpx: direct path not applicable", r, px)
			}
			if res == nil {
				continue // empty glyph
			}

			// FT 位图
			ftPgm := t.TempDir() + "/ft.pgm"
			cmd := exec.Command(bin, fontPath, string(r), strconv.Itoa(int(px)), "l", ftPgm)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("ftexp: %v %s", err, out)
			}
			var left, top int
			for _, f := range strings.Fields(string(out)) {
				if v, err := strconv.Atoi(strings.TrimPrefix(f, "left=")); err == nil && strings.HasPrefix(f, "left=") {
					left = v
				}
				if v, err := strconv.Atoi(strings.TrimPrefix(f, "top=")); err == nil && strings.HasPrefix(f, "top=") {
					top = v
				}
			}
			fw, fh, fv := readPgm2(ftPgm)
			if len(fv) == 0 {
				continue
			}

			// 直通路径输出 vs FT
			bad := 0
			if int(res.BearingX) != left || int(res.BearingY) != top {
				t.Errorf("%c %.0fpx: bearing 直通=%d,%d FT=%d,%d", r, px, int(res.BearingX), int(res.BearingY), left, top)
				continue
			}
			if res.Width != fw || res.Height != fh {
				t.Errorf("%c %.0fpx: size 直通=%dx%d FT=%dx%d", r, px, res.Width, res.Height, fw, fh)
				continue
			}
			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					if int(res.Mask[y*fw+x]) != int(fv[y*fw+x]) {
						bad++
					}
				}
			}
			glyphs++
			if bad > 0 {
				badTotal++
				t.Errorf("%c %.0fpx: 直通 bad=%d/%d", r, px, bad, fw*fh)
			}
		}
	}
	if badTotal > 0 {
		t.Fatalf("直通路径 %d/%d 字不逐字节一致", badTotal, glyphs)
	}
	fmt.Printf("A3 直通: %d 字 × FT-light 逐字节一致\n", glyphs)
}
