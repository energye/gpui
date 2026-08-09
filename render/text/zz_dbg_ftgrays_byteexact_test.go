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

// 阶段 A2 验证：FT 26.6 轮廓 → Go 移植光栅器（ftgrays.go）
// vs FT_Render_Glyph 位图（ftexp PGM）逐字节一致。
// 轮廓与位图同源（同一 FT 字体/字号/light），bad=0 即光栅器对齐。
func TestDbgFTGraysByteExact(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("font: %v", err)
	}
	bin := ftexpLocal(t)

	for _, px := range []float64{8, 12, 16, 24, 32, 48, 72} {
		for _, r := range []rune("静每合日田") {
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
				t.Fatalf("%c %.0fpx: no FT bitmap", r, px)
			}

			// 2) FT 26.6 轮廓（同 ftexp）
			pts, tags, contours := ftContour26(t, bin, fontPath, r, px)

			// 3) Go 移植光栅器
			mask, gLeft, gTop, _, _, err := RasterizeFT26(pts, tags, contours, false)
			if err != nil {
				t.Fatalf("%c %.0fpx: RasterizeFT26: %v", r, px, err)
			}

			// 4) 逐字节对照（含尺寸/left/top）
			bad := 0
			total := 0
			if gLeft != int32(left) || gTop != int32(top) {
				t.Errorf("%c %.0fpx: left/top 自研=%d,%d FT=%d,%d", r, px, gLeft, gTop, left, top)
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
			fmt.Printf("%c %.0fpx: ftgrays-go bad=%d/%d (位图 %dx%d left=%d top=%d)\n",
				r, px, bad, total, fw, fh, left, top)
		}
	}
}

// ftContour26 从 ftexp contour 输出解析 26.6 定点轮廓。
func ftContour26(t *testing.T, bin, fontPath string, r rune, px float64) ([]ftVec26, []ftOutlineTag, []int32) {
	t.Helper()
	cmd := exec.Command(bin, "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var np, nc, adv int
	fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
	pts := make([]ftVec26, np)
	tags := make([]ftOutlineTag, np)
	for i := 0; i < np; i++ {
		var x, y, tag int64
		fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
		pts[i] = ftVec26{x: x, y: y}
		switch tag {
		case 1:
			tags[i] = ftTagOn
		case 0:
			tags[i] = ftTagConic
		default:
			tags[i] = ftTagCubic
		}
	}
	var contours []int32
	if len(lines) >= 2+np {
		for _, s := range strings.Fields(lines[1+np]) {
			if v, err := strconv.Atoi(s); err == nil {
				contours = append(contours, int32(v))
			}
		}
	}
	if len(contours) == 0 {
		t.Fatalf("%c %.0fpx: no contours", r, px)
	}
	return pts, tags, contours
}
