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

// 阶段 A2 全字集验证：Go 移植光栅器（ftgrays.go RasterizeFT26）
// vs FT_Render_Glyph 位图逐字节一致，覆盖 cjk3000 全部 3000 常用字。
//
// 与 TestDbgFTGraysByteExact 同源对照，但用 ftexp 批量模式
// （bcontour + bpgm 各一次进程取全字集），避免每字一次 exec。
// 字表 = hint/testdata/cjk3000.txt（AGENTS.md 规定的标准字表文件）。
func TestDbgFTGraysScanCJK3000(t *testing.T) {
	raw, err := os.ReadFile("hint/testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != 3000 {
		t.Fatalf("cjk3000.txt = %d chars, want 3000", len(chars))
	}
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("font: %v", err)
	}
	bin := ftexpLocal(t)

	dir := t.TempDir()
	listFile := filepath.Join(dir, "chars.txt")
	if err := os.WriteFile(listFile, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, px := range []float64{8, 12, 16, 24, 32} {
		// 1) 批量轮廓（一次进程）
		contours := map[rune][][3]int64{}
		contourLines := map[rune][]string{}
		out, err := exec.Command(bin, "bcontour", fontPath, strconv.Itoa(int(px)), "l", listFile).Output()
		if err != nil {
			t.Fatalf("bcontour %.0fpx: %v", px, err)
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

		// 2) 批量位图（一次进程）
		out, err = exec.Command(bin, "bpgm", fontPath, strconv.Itoa(int(px)), "l", listFile).Output()
		if err != nil {
			t.Fatalf("bpgm %.0fpx: %v", px, err)
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

		// 3) 逐字对照
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
			mask, gLeft, gTop, _, _, err := RasterizeFT26(tagPts, tags, ends, false)
			if err != nil {
				t.Fatalf("%c %.0fpx: RasterizeFT26: %v", r, px, err)
			}
			w, h, left, top := b[0], b[1], b[2], b[3]
			if int(gLeft) != left || int(gTop) != top {
				totalBad++
				badRunes = append(badRunes, r)
				if len(badRunes) <= 10 {
					t.Errorf("%c %.0fpx: left/top 自研=%d,%d FT=%d,%d", r, px, gLeft, gTop, left, top)
				}
				continue
			}
			if len(mask) != w*h {
				totalBad++
				badRunes = append(badRunes, r)
				if len(badRunes) <= 10 {
					t.Errorf("%c %.0fpx: 位图尺寸 自研=%d FT=%dx%d", r, px, len(mask), w, h)
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
					t.Errorf("%c %.0fpx: bad=%d/%d", r, px, bad, w*h)
				}
			}
		}
		if totalBad > 0 {
			t.Errorf("%.0fpx: %d/%d 字不逐字节一致，前 10: %c", px, totalBad, totalGlyphs, badRunes[:min(10, len(badRunes))])
		} else {
			fmt.Printf("%.0fpx: %d/%d 字逐字节一致 (FT light)\n", px, totalGlyphs, totalGlyphs)
		}
	}
}
