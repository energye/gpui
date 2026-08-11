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

// 阶段 B2 位图级矩阵：Go 光栅器（RasterizeFT26）vs FT_Render_Glyph
// **逐字节（含 0 值）** 一致，15 档（8/10/12/14/16/18/20/24/28/32/40/48/56/64/72px）
// × 全字集。输入 = FT light 26.6 轮廓（ftexp bcontour），输出对照 FT 灰度位图
// （ftexp bpgm），bitmap_left/top 对齐，逐像素 bad 计数，bad>0 判 FAIL。
//
// 矩阵范围（docs/ENGINE_TEXT_RASTER_FT_ALIGN_PLAN.md §B2）：
//   - CFF：cjk3000 × NotoSansCJK-Regular.ttc face0(JP)；kr_all × face1(KR)；
//     th_all × hint/testdata/NotoSansThai-Regular.otf
//   - TTF：wqy-microhei-nohint × cjk3000 抽样 300（B2 全量 TTF 位图时长不可控）；
//     latin_all × FreeSans/DejaVuSans 全量
func TestB2BitmapCJK(t *testing.T) {
	b2BitmapRunScan(t, "noto-cjk-jp", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", 0,
		"hint/testdata/cjk3000.txt", 3000, 0, b2c15pxs())
}

func TestB2BitmapKorean(t *testing.T) {
	b2BitmapRunScan(t, "noto-cjk-kr", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", 1,
		"hint/testdata/kr_all.txt", 11172, 0, b2c15pxs())
}

func TestB2BitmapThai(t *testing.T) {
	b2BitmapRunScan(t, "noto-thai", "hint/testdata/NotoSansThai-Regular.otf", 0,
		"hint/testdata/th_all.txt", 128, 0, b2c15pxs())
}

func TestB2BitmapTTF(t *testing.T) {
	b2BitmapRunScan(t, "wqy-nohint", "testdata/wqy-microhei-nohint.ttf", 0,
		"hint/testdata/cjk3000.txt", 3000, 300, b2c15pxs())
	b2BitmapRunScan(t, "freesans", "/usr/share/fonts/truetype/freefont/FreeSans.ttf", 0,
		"hint/testdata/latin_all.txt", 254, 0, b2c15pxs())
	b2BitmapRunScan(t, "dejavu", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 0,
		"hint/testdata/latin_all.txt", 254, 0, b2c15pxs())
}

// b2c15pxs 是 B2 档位：8/10/12/14/16/18/20/24/28/32/40/48/56/64/72px（15 档）。
func b2c15pxs() []float64 {
	return []float64{8, 10, 12, 14, 16, 18, 20, 24, 28, 32, 40, 48, 56, 64, 72}
}

// b2BitmapRunScan 跑一个字体×字集×档位矩阵：每档一次 ftexp bcontour 取
// FT light 26.6 轮廓、一次 bpgm 取 FT 灰度位图，逐字 RasterizeFT26 后
// 逐字节（含 0 值）对照。maxN>0 时截断字集（TTF 全量位图时长不可控，抽样）。
func b2BitmapRunScan(t *testing.T, name, fontPath string, faceIdx int, listPath string, wantCount, maxN int, pxs []float64) {
	t.Helper()
	raw, err := os.ReadFile(listPath)
	if err != nil {
		t.Skipf("%s: list unavailable: %v", name, err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if wantCount > 0 && len(chars) < wantCount-200 {
		t.Fatalf("%s: char list too short (got %d)", name, len(chars))
	}
	if _, err := os.Stat(fontPath); err != nil {
		t.Skipf("%s: font unavailable: %v", name, err)
	}
	if maxN > 0 && len(chars) > maxN {
		chars = chars[:maxN]
	}
	bin := ftexpLocal(t)
	dir := t.TempDir()
	listFile := filepath.Join(dir, "chars.txt")
	if err := os.WriteFile(listFile, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	faceStr := strconv.Itoa(faceIdx)
	faceArgs := []string{"0", "0", faceStr} // bcontour: [blend=0][skip][faceIdx]

	for _, px := range pxs {
		pxStr := strconv.Itoa(int(px))
		contours := map[rune][][3]int64{}
		contourLines := map[rune][]string{}
		args := append([]string{"bcontour", fontPath, pxStr, "l", listFile}, faceArgs...)
		out, err := exec.Command(bin, args...).Output()
		if err != nil {
			t.Fatalf("%s bcontour %.0fpx: %v", name, px, err)
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

		pgm := map[rune][4]int{}
		pgmPixels := map[rune][]byte{}
		args = []string{"bpgm", fontPath, pxStr, "l", listFile, faceStr}
		out, err = exec.Command(bin, args...).Output()
		if err != nil {
			t.Fatalf("%s bpgm %.0fpx: %v", name, px, err)
		}
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

		totalBad, totalGlyphs := 0, 0
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
				t.Fatalf("%s %U %.0fpx: RasterizeFT26: %v", name, r, px, err)
			}
			w, h, left, top := b[0], b[1], b[2], b[3]
			if int(gLeft) != left || int(gTop) != top {
				totalBad++
				badRunes = append(badRunes, r)
				if len(badRunes) <= 10 {
					t.Errorf("%s %U %.0fpx: left/top 自研=%d,%d FT=%d,%d", name, r, px, gLeft, gTop, left, top)
				}
				continue
			}
			if len(mask) != w*h {
				totalBad++
				badRunes = append(badRunes, r)
				if len(badRunes) <= 10 {
					t.Errorf("%s %U %.0fpx: 位图尺寸 自研=%d FT=%dx%d", name, r, px, len(mask), w, h)
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
					t.Errorf("%s %U %.0fpx: bad=%d/%d", name, r, px, bad, w*h)
				}
			}
		}
		if totalBad > 0 {
			t.Fatalf("%s %.0fpx: %d/%d 字不逐字节一致，前 10: %c", name, px, totalBad, totalGlyphs, badRunes[:min(10, len(badRunes))])
		}
		fmt.Printf("B2 %s %.0fpx: %d/%d 字逐字节一致 (FT light)\n", name, px, totalGlyphs, totalGlyphs)
	}
}
