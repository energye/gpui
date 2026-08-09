package text

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

// zz_b1_vf_test.go —— 阶段 B1a：CFF2 可变字体轮廓级四维度矩阵。
//
// 对照维度同 B1a CFF（① 点数 ② 坐标 ③ on/off ④ contours 分组）。
// hint 侧用 hint.LightHintVar（CFF2 + coords），FT 侧用 ftexp bcontour
// （可选 wght 参数 → FT_Set_Var_Design_Coordinates）。
//
// 矩阵：65 档（8–72px）× 4 wght × 字集（cjk3000 前 300 常用 + latin_all
// 254）。字体：NotoSansSC-VF.otf / SourceSans3VF-Upright.otf。

// b1VFGlyph 是 ftexp bcontour 单个字形完整对照数据。
type b1VFGlyph struct {
	pts  [][3]int64 // x, y（26.6 Y-up）, tag（1=on 2=off）
	ends []int64    // 每轮廓末点索引 inclusive
}

// batchContourFull26 分块批量取 FT-light 26.6 轮廓（含 tag 与分组），
// 可选 wght 设计坐标（0 = 默认实例）。分块 + 空块重试逻辑同 hint 包
// batchContourB1（ftexp 长跑空块是工具缺陷，小批量 exec 稳定）。
func batchContourFull26(t *testing.T, fontPath string, chars []rune, px float64, wght float64) map[rune]*b1VFGlyph {
	t.Helper()
	res := map[rune]*b1VFGlyph{}
	retry := map[rune]bool{}
	const chunk = 512
	for start := 0; start < len(chars); start += chunk {
		end := start + chunk
		if end > len(chars) {
			end = len(chars)
		}
		parseB1VFBatch(t, fontPath, chars[start:end], px, wght, res, retry)
	}
	for len(retry) > 0 {
		rs := make([]rune, 0, len(retry))
		for r := range retry {
			rs = append(rs, r)
		}
		next := map[rune]bool{}
		for s := 0; s < len(rs); s += 100 {
			e := s + 100
			if e > len(rs) {
				e = len(rs)
			}
			parseB1VFBatch(t, fontPath, rs[s:e], px, wght, res, next)
		}
		retry = next
		if len(next) > 0 && len(next) >= len(rs) {
			t.Errorf("ftexp 空块重试无效（ftexp 工具缺陷，非引擎回归），字: %s",
				string(rs[:min(20, len(rs))]))
			break
		}
	}
	return res
}

func parseB1VFBatch(t *testing.T, fontPath string, chars []rune, px, wght float64,
	res map[rune]*b1VFGlyph, retry map[rune]bool) {
	t.Helper()
	list := t.TempDir() + "/b1vf.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"bcontour", fontPath, strconv.Itoa(int(px)), "l", list}
	if wght != 0 {
		args = append(args, fmt.Sprintf("%g", wght))
	}
	cmd := exec.Command(wqyFTExpBin(t), args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("ftexp bcontour unavailable: %v", err)
		return
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := 0; i < len(lines); {
		line := lines[i]
		if !strings.HasPrefix(line, "# R ") {
			i++
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) < 6 || parts[3] == "MISSING" || parts[3] == "FAIL" ||
			parts[3] == "LOADFAIL" || parts[3] == "SLOTFAIL" {
			i++
			continue
		}
		np, _ := strconv.Atoi(parts[3])
		nc, _ := strconv.Atoi(parts[4])
		var r rune
		fmt.Sscanf(parts[2], "U+%X", &r)
		if np <= 0 || nc <= 0 {
			i++
			continue
		}
		if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" ||
			strings.HasPrefix(lines[i+1], "# R ") {
			retry[r] = true
			i++
			continue
		}
		g := &b1VFGlyph{}
		broken := false
		for j := 0; j < np; j++ {
			i++
			if i >= len(lines) || strings.TrimSpace(lines[i]) == "" ||
				strings.HasPrefix(lines[i], "# R ") {
				broken = true
				break
			}
			var x, y, tag int64
			fmt.Sscanf(lines[i], "%d %d %d", &x, &y, &tag)
			g.pts = append(g.pts, [3]int64{x, y, tag})
		}
		if broken {
			retry[r] = true
			i++
			continue
		}
		i++
		if i >= len(lines) {
			retry[r] = true
			continue
		}
		parts = strings.Fields(lines[i])
		if len(parts) < nc {
			retry[r] = true
			continue
		}
		badEnds := false
		for c := 0; c < nc; c++ {
			e, err := strconv.ParseInt(parts[c], 10, 64)
			if err != nil {
				badEnds = true
				break
			}
			g.ends = append(g.ends, e)
		}
		if badEnds {
			retry[r] = true
			continue
		}
		res[r] = g
		i++
	}
}

// b1VFRun 对一种 CFF2 VF 字体跑四维度矩阵。
func b1VFRun(t *testing.T, name, fontPath string, chars []rune, wghts []float64) {
	t.Helper()
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("%s: %v", name, err)
	}
	defer src.Close()
	own, ok := src.Parsed().(*ownParsedFont)
	if !ok {
		t.Skipf("%s: 非 ownParsedFont", name)
	}
	raw := own.RawFontData()
	isCFF2 := own.hasCFF2Table() && !own.hasCFFTable()
	if !isCFF2 {
		t.Fatalf("%s: 不是 CFF2 字体", name)
	}

	totalBad := 0
	for _, w := range wghts {
		variations := []FontVariation{NewFontVariation("wght", float32(w))}
		fc := own.cff2VariationCoords(variations)
		var coords []float32
		for _, c := range fc {
			coords = append(coords, float32(c)/16384.0)
		}
		// FT 侧用同一归一化 blend（ftexp 第 6 参）。默认实例（coords 空/
		// 全 0）不传参。注意：不能传 FT 的 design 坐标——FT 对 CFF2 的
		// design→blend 转换与本引擎不一致（实测溢出 1，见 B1a 记录）。
		blendArg := 0.0
		if len(coords) > 0 && coords[0] != 0 {
			blendArg = float64(coords[0])
		}
		for px := 8.0; px <= 72.0; px++ {
			ft := batchContourFull26(t, fontPath, chars, px, blendArg)
			bad, npt, noff, ngrp := 0, 0, 0, 0
			for _, r := range chars {
				ftG, ok := ft[r]
				if !ok {
					continue
				}
				gid := own.GlyphIndex(r)
				pts, contours, _, herr := hint.LightHintVar(raw, own.collectionIndex, true, gid, px, coords)
				if herr != nil || len(pts) == 0 {
					continue
				}
				// ① 点数
				if len(ftG.pts) != len(pts) {
					bad++
					npt++
					continue
				}
				// ② 坐标 + ③ on/off
			ok = true
			for i, p := range pts {
				if int64(p.X) != ftG.pts[i][0] || int64(p.Y) != ftG.pts[i][1] {
					bad++
					ok = false
					break
				}
					if p.On != (ftG.pts[i][2] == 1) {
						bad++
						noff++
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				// ④ 分组
				if len(contours) != len(ftG.ends) {
					bad++
					ngrp++
					continue
				}
				acc := 0
				for ci, n := range contours {
					acc += n
					if int64(acc-1) != ftG.ends[ci] {
						bad++
						ngrp++
						break
					}
				}
			}
			if bad > 0 {
				totalBad += bad
				fmt.Printf("B1 %s wght=%.0f @%.0fpx: bad=%d (npt=%d off=%d grp=%d)\n",
					name, w, px, bad, npt, noff, ngrp)
			}
		}
	}
	if totalBad > 0 {
		t.Errorf("B1 %s: 累计 bad=%d (must be 0)", name, totalBad)
	} else {
		fmt.Printf("B1 %s: 65 档 × %d wght 四维度全部一致\n", name, len(wghts))
	}
}

// b1VFRunes 返回 VF 字集：cjk3000 前 cjkN 常用 + latin_all。
func b1VFRunes(t *testing.T, cjkN int) []rune {
	t.Helper()
	raw, err := os.ReadFile("hint/testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000: %v", err)
	}
	cjk := []rune(strings.TrimSpace(string(raw)))
	if len(cjk) > cjkN {
		cjk = cjk[:cjkN]
	}
	lraw, err := os.ReadFile("hint/testdata/latin_all.txt")
	if err != nil {
		t.Skipf("latin_all: %v", err)
	}
	return append(cjk, []rune(strings.TrimSpace(string(lraw)))...)
}

// TestB1VFNotoSansSC：NotoSansSC-VF × 4 wght（200/400/600/800）。
func TestB1VFNotoSansSC(t *testing.T) {
	path := "testdata/NotoSansSC-VF.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("NotoSansSC-VF unavailable: %v", err)
	}
	b1VFRun(t, "notosc_vf", path, b1VFRunes(t, 300), []float64{200, 400, 600, 800})
}

// TestB1VFSourceSans：SourceSans3VF × 4 wght（200/400/600/800）。
func TestB1VFSourceSans(t *testing.T) {
	path := "testdata/source-sans/VF/SourceSans3VF-Upright.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("SourceSans3VF unavailable: %v", err)
	}
	b1VFRun(t, "sourcesans_vf", path, b1VFRunes(t, 300), []float64{200, 400, 600, 800})
}
