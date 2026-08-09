package text

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 阶段 A4 矩阵补充：wqy-microhei（TTF/glyf + conic 二次曲线）抽样对照。
// Noto Sans CJK 是 CFF（cubic），全字集只覆盖 renderCubic 路径；
// wqy 是 TrueType（conic），覆盖 renderConicDDA 路径的 CJK 结构。
// 与 TestDbgFTGraysByteExact 同源对照（同一 FT 字体/字号/light）。
func TestDbgFTGraysByteExactWqy(t *testing.T) {
	fontPath := "testdata/wqy-microhei.ttf"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("font: %v", err)
	}
	bin := ftexpLocal(t)

	for _, px := range []float64{8, 12, 16, 24, 32, 48, 72} {
		for _, r := range []rune("静每合日田一二三上下大小山川土木人") {
			// 1) FT 位图 + left/top
			ftPgm := filepath.Join(t.TempDir(), "ft.pgm")
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
				continue // 该字在该字号无位图（比如缺字形）
			}

			// 2) FT 26.6 轮廓（同 ftexp）
			pts, tags, contours := ftContour26(t, bin, fontPath, r, px)

			// 3) Go 移植光栅器
			mask, gLeft, gTop, err := RasterizeFT26(pts, tags, contours, false)
			if err != nil {
				t.Fatalf("%c %.0fpx: RasterizeFT26: %v", r, px, err)
			}

			// 4) 逐字节对照（含尺寸/left/top）
			bad := 0
			total := 0
			if gLeft != int32(left) || gTop != int32(top) {
				t.Errorf("%c %.0fpx: left/top 自研=%d,%d FT=%d,%d", r, px, gLeft, gTop, left, top)
				continue
			}
			if len(mask) != fw*fh {
				t.Errorf("%c %.0fpx: 位图尺寸 自研=%d FT=%dx%d", r, px, len(mask), fw, fh)
				continue
			}
			for y := 0; y < fh; y++ {
				for x := 0; x < fw; x++ {
					total++
					a := int(fv[y*fw+x])
					b := int(mask[y*int(fw)+x])
					if a != b {
						bad++
					}
				}
			}
			if bad > 0 {
				t.Errorf("%c %.0fpx: bad=%d/%d", r, px, bad, total)
			} else {
				fmt.Printf("%c %.0fpx: wqy ftgrays-go bad=0/%d\n", r, px, total)
			}
		}
	}
}
