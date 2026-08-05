package hint

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestScanCJK3000：3000 常用字全量回归（docs §5.1 M3 收尾项）。
//
// 字表 = testdata/cjk3000.txt（《通用规范汉字表》一级字表前 3000 常用字，
// 国标 2013，docs §4.3 规定来源）。对照 = ftexp bcontour 批量模式
// （一次进程取全部字 FT-light 26.6 轮廓，避免每字一次 exec）。
func TestScanCJK3000(t *testing.T) {
	raw, err := os.ReadFile("testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != 3000 {
		t.Fatalf("cjk3000.txt = %d chars, want 3000", len(chars))
	}
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()

	for _, px := range []float64{10, 12, 14, 16, 20, 24} {
		scale := m2HintScale(px, upem)
		ft := batchContour26Light(t, chars, px)
		if len(ft) < 2900 {
			t.Fatalf("cjk3000: only %d glyphs parsed, want ≥2900 (parser regression)", len(ft))
		}
		bad := 0
		npt := 0
		var badR, nptR []rune
		for _, r := range chars {
			ftPts, ok := ft[r]
			if !ok {
				continue
			}
			gid := uint16(f.GlyphIndex(r))
			out, fd, err := m2Interp(cd, gid)
			if err != nil {
				continue
			}
			res := hintCFFLight(out, fd, scale, 0, 0)
			if len(ftPts) != len(res.pts) {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			for i, p := range res.pts {
				if int64(p[0]>>10) != ftPts[i][0] || int64(p[1]>>10) != ftPts[i][1] {
					bad++
					badR = append(badR, r)
					break
				}
			}
		}
		fmt.Printf("cjk3000: bad@%.0fpx=%d (npt=%d)\n", px, bad, npt)
		for i := 0; i < len(badR); i += 30 {
			end := i + 30
			if end > len(badR) {
				end = len(badR)
			}
			fmt.Printf("  bad: %s\n", string(badR[i:end]))
		}
		if len(nptR) > 0 {
			fmt.Printf("  npt: %s\n", string(nptR))
		}
	}
}

// batchContour26Light 一次 exec 批量取 FT-light 26.6 轮廓（mode l），
// 返回 rune → 点表。缺失/加载失败的字不在 map 中。
func batchContour26Light(t *testing.T, chars []rune, px float64) map[rune][][3]int64 {
	return batchContour26LightFont(t, "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", chars, px)
}

// batchContour26LightFont 同 batchContour26Light，但可指定字体文件。
func batchContour26LightFont(t *testing.T, fontPath string, chars []rune, px float64) map[rune][][3]int64 {
	t.Helper()
	list := "/tmp/opencode/cjk3000_runes.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/tmp/opencode/ftexp/ftexp", "bcontour",
		fontPath, strconv.Itoa(int(px)), "l", list)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("ftexp bcontour unavailable: %v", err)
	}
	res := map[rune][][3]int64{}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := 0; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "# R ") {
			i++
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) < 6 || parts[3] == "MISSING" {
			i++
			continue
		}
		// header: "# R U+XXXX <np> <nc> <adv>"——np 是 parts[3]。
		np, _ := strconv.Atoi(parts[3])
		var r rune
		fmt.Sscanf(parts[2], "U+%X", &r)
		pts := make([][3]int64, 0, np)
		for j := 0; j < np; j++ {
			i++
			if i >= len(lines) {
				t.Fatalf("short bcontour block for %U", r)
			}
			var x, y, tag int64
			fmt.Sscanf(lines[i], "%d %d %d", &x, &y, &tag)
			pts = append(pts, [3]int64{x, y, tag})
		}
		res[r] = pts
		i++
	}
	return res
}
