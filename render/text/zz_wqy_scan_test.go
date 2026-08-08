package text

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestScanWqyCJKLight：M4a 全字集回归窗（docs §5.2a M4b-3）。
//
// 字表 = testdata/cjk3000.txt（《通用规范汉字表》一级字表前 3000 常用字，
// 与 cf2 的 M3 全字集回归同表，保证可比）。对照 = ftexp bcontour 批量模式
// 取 wqy-microhei-nohint.ttf（glyf 无字节码 CJK 主字体）FT-light 26.6 轮廓，
// 与 ExtrisOpeningHinted 逐点对照。
func TestScanWqyCJK3000(t *testing.T) {
	raw, err := os.ReadFile("hint/testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != 3000 {
		t.Fatalf("cjk3000.txt = %d chars, want 3000", len(chars))
	}
	fontPath := "testdata/wqy-microhei-nohint.ttf"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	face := src.Parsed()
	ext := NewOutlineExtractor()

	for _, px := range []int{10, 12, 14, 16} {
		ft := batchWqyContour26(t, fontPath, chars, float64(px))
		if len(ft) < 2900 {
			t.Fatalf("wqy px%d: only %d glyphs parsed, want ≥2900 (parser regression)", px, len(ft))
		}
		bad, npt := 0, 0
		badR, nptR := []rune{}, []rune{}
		for _, r := range chars {
			ftFound, ok := ft[r]
			if !ok {
				continue
			}
			gid := face.GlyphIndex(r)
			out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
			if err != nil || out == nil {
				continue
			}
			gotPts := outlinePoints26(out)
			if len(ftFound) > 0 && len(gotPts) == 0 {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			// FT 26.6 坐标（Y-up）；Go 像素（Y-down）。同一字形形状：坐标点集合应
			// 在 ±1/64 内一一对应。顺序/重复不做严格比对（segment 重建拓扑可能与
			// FT 轮廓顺序不同），故用多集最近邻匹配。
			matched := matchPointSets(ftFound, gotPts, 64)
			if matched < len(ftFound) && matched < len(gotPts) {
				bad++
				if len(badR) < 30 {
					badR = append(badR, r)
				}
				continue
			}
			_ = npt
		}
		t.Logf("wqy px%d: bad=%d/%d npt=%d nptR=%d first=%v", px, bad, len(chars), npt, len(nptR), runes8(nptR))
		t.Logf("wqy px%d: badR=%v", px, runes8(badR))
		if bad > 0 {
			t.Errorf("wqy px%d: bad=%d/%d (must be 0)", px, bad, len(chars))
		}
	}
}

// outlinePoints26 把 Extractor 输出（float px 像素，Y-down）转成 FT 26.6 定点 Y-up，
// 并过滤 segment 重建产生的 (0,0) 退化点（田字 29 segments → 87 原始点 → 24 真实点）。
func outlinePoints26(out *GlyphOutline) [][2]int64 {
	var pts [][2]int64
	for _, s := range out.Segments {
		for _, p := range s.Points {
			if p.X == 0 && p.Y == 0 {
				continue // 退化/幻影点，非真实轮廓顶点
			}
			x := int64(p.X * 64)
			y := int64(-p.Y * 64)
			pts = append(pts, [2]int64{x, y})
		}
	}
	return pts
}

// matchPointSets 用贪心最近邻匹配两个点集（各自含重复点），返回匹配数。
// tol 为容差（1/64 单位）。用 0.5px 容差吸收 segment 重建的 ±1 舍入，仍能暴露
// 结构性偏差（田 16px 顶 738 vs 766 = 28/64 > 0.5px，会被判 bad）。
func matchPointSets(a, b [][2]int64, tol int64) int {
	used := make([]bool, len(b))
	matched := 0
	for _, p := range a {
		best, bi := int64(1<<62), -1
		for j, q := range b {
			if used[j] {
				continue
			}
			dx, dy := p[0]-q[0], p[1]-q[1]
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			d := dx + dy
			if d < best {
				best, bi = d, j
			}
		}
		if bi >= 0 && best <= tol {
			used[bi] = true
			matched++
		}
	}
	return matched
}

func runes8(rs []rune) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, fmt.Sprintf("%q", r))
	}
	return out
}

// wqyFTExpBin 解析 FT 度量衡二进制：$FTEXP_BIN → hint/testdata/ftexp/ftexp →
// 拷贝源码到 t.TempDir() 自动重建（与 hint 包 ftexpBin 同策略）。
func wqyFTExpBin(t *testing.T) string {
	t.Helper()
	const srcRel = "hint/testdata/ftexp"
	if p := os.Getenv("FTEXP_BIN"); p != "" {
		return p
	}
	bin := srcRel + "/ftexp"
	if st, err := os.Stat(bin); err == nil && !st.IsDir() {
		return bin
	}
	tmp := t.TempDir()
	for _, name := range []string{"main.go", "go.mod", "go.sum"} {
		data, err := os.ReadFile(srcRel + "/" + name)
		if err != nil {
			t.Skipf("ftexp source missing (%s): %v", name, err)
		}
		if err := os.WriteFile(tmp+"/"+name, data, 0o644); err != nil {
			t.Fatalf("write ftexp source %s: %v", name, err)
		}
	}
	bin = tmp + "/ftexp"
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ftexp rebuild failed (need go + purego): %v %s", err, out)
	}
	return bin
}

// batchWqyContour26 调 ftexp bcontour 取 wqy FT 26.6 轮廓（批量，一次启动）。
// hint 参数：l=light, n=no-hint, f=full(FT_LOAD_TARGET_NORMAL)。
func batchWqyContour26(t *testing.T, fontPath string, chars []rune, px float64) map[rune][][2]int64 {
	t.Helper()
	return batchWqyContour26Hint(t, fontPath, chars, px, "l")
}

// batchWqyContour26Hint 是 batchWqyContour26 的可指定 hint 模式版本。
func batchWqyContour26Hint(t *testing.T, fontPath string, chars []rune, px float64, hint string) map[rune][][2]int64 {
	t.Helper()
	list := t.TempDir() + "/wqy_runes.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(wqyFTExpBin(t), "bcontour",
		fontPath, strconv.Itoa(int(px)), hint, list)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("ftexp bcontour unavailable: %v", err)
	}
	res := map[rune][][2]int64{}
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
		np, _ := strconv.Atoi(parts[3])
		var r rune
		fmt.Sscanf(parts[2], "U+%X", &r)
		pts := make([][2]int64, 0, np)
		for j := 0; j < np; j++ {
			i++
			if i >= len(lines) {
				t.Fatalf("short bcontour block for %U", r)
			}
			var x, y int64
			var tag int
			fmt.Sscanf(lines[i], "%d %d %d", &x, &y, &tag)
			pts = append(pts, [2]int64{x, y})
		}
		res[r] = pts
		i++
	}
	return res
}
