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

// 阶段 A2 跨脚本验证：Go 光栅器（RasterizeFT26）vs FT_Render_Glyph 逐字节一致，
// 覆盖拉丁（latin_all.txt 254 字）、泰文（th_all.txt 128 码位）、
// 韩文（kr_all.txt 11172 音节），各用对应标准字表文件（AGENTS.md）。
func TestDbgFTGraysScanScripts(t *testing.T) {
	bin := ftexpLocal(t)
	cases := []struct {
		name  string
		list  string
		font  string
		px    float64
		minOk int
	}{
		{"latin", "hint/testdata/latin_all.txt", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 12, 200},
		{"thai", "hint/testdata/th_all.txt", "hint/testdata/NotoSansThai-Regular.otf", 12, 100},
		{"korean", "hint/testdata/kr_all.txt", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", 12, 11000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := os.ReadFile(c.list)
			if err != nil {
				t.Skipf("list unavailable: %v", err)
			}
			chars := []rune(strings.TrimSpace(string(raw)))
			if len(chars) < c.minOk {
				t.Fatalf("%s = %d chars, want ≥%d", c.name, len(chars), c.minOk)
			}
			if _, err := os.Stat(c.font); err != nil {
				t.Skipf("font: %v", err)
			}
			dir := t.TempDir()
			listFile := filepath.Join(dir, "chars.txt")
			if err := os.WriteFile(listFile, []byte(string(chars)), 0o644); err != nil {
				t.Fatal(err)
			}
			px := c.px

			contours := map[rune][][3]int64{}
			contourLines := map[rune][]string{}
			out, err := exec.Command(bin, "bcontour", c.font, strconv.Itoa(int(px)), "l", listFile).Output()
			if err != nil {
				t.Fatalf("bcontour: %v", err)
			}
			lines := strings.Split(string(out), "\n")
			for i := 0; i < len(lines); i++ {
				if !strings.HasPrefix(lines[i], "# R ") {
					continue
				}
				fields := strings.Fields(lines[i])
				if len(fields) < 5 || fields[2] == "MISSING" || fields[2] == "FAIL" {
					continue
				}
				cp, err := strconv.ParseUint(fields[2][2:], 16, 32)
				if err != nil {
					continue
				}
				r := rune(cp)
				np, _ := strconv.Atoi(fields[3])
				pts := [][3]int64{}
				for j := 0; j < np; j++ {
					var x, y, tag int64
					fmt.Sscanf(lines[i+1+j], "%d %d %d", &x, &y, &tag)
					pts = append(pts, [3]int64{x, y, tag})
				}
				contours[r] = pts
				contourLines[r] = lines[i : i+1+np+1]
			}

			out, err = exec.Command(bin, "bpgm", c.font, strconv.Itoa(int(px)), "l", listFile).Output()
			if err != nil {
				t.Fatalf("bpgm: %v", err)
			}
			pgm := map[rune][4]int{}
			pgmPixels := map[rune][]byte{}
			lines = strings.Split(string(out), "\n")
			for i := 0; i < len(lines); i++ {
				if !strings.HasPrefix(lines[i], "# B ") {
					continue
				}
				fields := strings.Fields(lines[i])
				cp, err := strconv.ParseUint(fields[2][2:], 16, 32)
				if err != nil {
					continue
				}
				r := rune(cp)
				w, _ := strconv.Atoi(fields[3])
				h, _ := strconv.Atoi(fields[4])
				left, _ := strconv.Atoi(fields[5])
				top, _ := strconv.Atoi(fields[6])
				if w <= 0 || h <= 0 {
					continue
				}
				vals := []byte{}
				for row := 0; row < h; row++ {
					for _, f := range strings.Fields(lines[i+1+row]) {
						v, _ := strconv.Atoi(f)
						vals = append(vals, byte(v))
					}
				}
				pgm[r] = [4]int{w, h, left, top}
				pgmPixels[r] = vals
			}

			totalBad := 0
			totalGlyphs := 0
			var badRunes []rune
			for _, r := range chars {
				pts, ok := contours[r]
				if !ok {
					continue
				}
				b, ok := pgm[r]
				if !ok {
					continue
				}
				totalGlyphs++
				tagPts := make([]ftVec26, len(pts))
				tags := make([]ftOutlineTag, len(pts))
				for i, p := range pts {
					tagPts[i] = ftVec26{x: p[0], y: p[1]}
					switch p[2] {
					case 1:
						tags[i] = ftTagOn
					case 0:
						tags[i] = ftTagConic
					default:
						tags[i] = ftTagCubic
					}
				}
				var ends []int32
				for _, s := range strings.Fields(contourLines[r][1+len(pts)]) {
					if v, err := strconv.Atoi(s); err == nil {
						ends = append(ends, int32(v))
					}
				}
				mask, gLeft, gTop, err := RasterizeFT26(tagPts, tags, ends, false)
				if err != nil {
					t.Fatalf("%U: RasterizeFT26: %v", r, err)
				}
				w, h, left, top := b[0], b[1], b[2], b[3]
				if int(gLeft) != left || int(gTop) != top {
					totalBad++
					badRunes = append(badRunes, r)
					if len(badRunes) <= 10 {
						t.Errorf("%U: left/top 自研=%d,%d FT=%d,%d", r, gLeft, gTop, left, top)
					}
					continue
				}
				if len(mask) != w*h {
					totalBad++
					badRunes = append(badRunes, r)
					if len(badRunes) <= 10 {
						t.Errorf("%U: 位图尺寸 自研=%d FT=%dx%d", r, len(mask), w, h)
					}
					continue
				}
				bad := 0
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						if mask[y*w+x] != pgmPixels[r][y*w+x] {
							bad++
						}
					}
				}
				if bad > 0 {
					totalBad++
					badRunes = append(badRunes, r)
					if len(badRunes) <= 10 {
						t.Errorf("%U: bad=%d/%d", r, bad, w*h)
					}
				}
			}
			if totalBad > 0 {
				t.Errorf("%s: %d/%d 字不逐字节一致, 前 10: %c", c.name, totalBad, totalGlyphs, badRunes[:min(10, len(badRunes))])
			} else {
				fmt.Printf("%s: %d/%d 字逐字节一致 (FT light)\n", c.name, totalGlyphs, totalGlyphs)
			}
		})
	}
}
