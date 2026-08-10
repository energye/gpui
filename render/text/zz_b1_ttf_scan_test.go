package text

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// zz_b1_ttf_scan_test.go —— 阶段 B1b：TTF 自动加框 26.6 直通链四维度对照
// （docs ENGINE_TEXT_RASTER_FT_ALIGN_PLAN.md §阶段B B1b）。
//
// 引擎侧：ParseGlyfContours → autoHintContourPoints（26.6 定点 Y-up，
// 不平移）→ hintedContoursToFT26 直通出口（不经过 contoursToOutline 的
// float32 段重建）。
// FT 侧：ftexp bcontour（FT-light autofit 26.6 轮廓 + tag + ends）。
// 对照维度同 B1a：①点数 ②坐标 ③on/off（TTF：tag1=on，tag0=conic off）
// ④contours 分组（末点索引）。

// b1TTFGlyph 是 ft26 bcontour 单个字形的完整对照数据。
type b1TTFGlyph struct {
	pts  [][3]int64 // x, y（26.6 Y-up）, tag（0=conic 1=on）
	ends []int64    // 每轮廓末点索引 inclusive
}

// hintedContoursToFT26 把 autoHintContourPoints 的 26.6 定点 Y-up 结果
// 直通为 FT 轮廓对照数组（pts/tags/ends），不经 contoursToOutline 的
// float32 段重建。
func hintedContoursToFT26(hinted *GlyfContours) ([]ftVec26, []ftOutlineTag, []int32) {
	pts := make([]ftVec26, len(hinted.Points))
	tags := make([]ftOutlineTag, len(hinted.Points))
	for i, p := range hinted.Points {
		pts[i] = ftVec26{x: int64(p.X), y: int64(p.Y)}
		if p.OnCurve {
			tags[i] = ftTagOn
		} else {
			tags[i] = ftTagConic
		}
	}
	ends := make([]int32, len(hinted.EndPts))
	for i, e := range hinted.EndPts {
		ends[i] = int32(e)
	}
	return pts, tags, ends
}

// batchB1TTFContour 分块批量取 FT-light 26.6 轮廓（含 tag 与分组），
// 空块字小批量重试（同 hint 包 B1a 语义，禁止静默假绿）。
func batchB1TTFContour(t *testing.T, fontPath string, chars []rune, px float64) map[rune]*b1TTFGlyph {
	t.Helper()
	res := map[rune]*b1TTFGlyph{}
	retry := map[rune]bool{}
	const chunk = 512
	for start := 0; start < len(chars); start += chunk {
		end := start + chunk
		if end > len(chars) {
			end = len(chars)
		}
		parseB1TTFBatch(t, fontPath, chars[start:end], px, res, retry)
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
			parseB1TTFBatch(t, fontPath, rs[s:e], px, res, next)
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

func parseB1TTFBatch(t *testing.T, fontPath string, chars []rune, px float64,
	res map[rune]*b1TTFGlyph, retry map[rune]bool) {
	t.Helper()
	list := t.TempDir() + "/b1ttf_runes.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(wqyFTExpBin(t), "bcontour",
		fontPath, strconv.Itoa(int(px)), "l", list)
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
		g := &b1TTFGlyph{}
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

// b1TTFRunScan 对一种 TTF 字体跑四维度对照（chars 全字集）。
func b1TTFRunScan(t *testing.T, name, fontPath, listPath string, wantCount int, pxs []float64) int {
	t.Helper()
	raw, err := os.ReadFile(listPath)
	if err != nil {
		t.Skipf("%s: %v", name, err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != wantCount {
		t.Fatalf("%s: %s = %d chars, want %d", name, listPath, len(chars), wantCount)
	}
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("%s: open font: %v", name, err)
	}
	defer src.Close()
	face := src.Parsed()
	own, ok := face.(*ownParsedFont)
	if !ok {
		t.Skipf("%s: 非 ownParsedFont", name)
	}
	fontData := own.RawFontData()

	totalBad := 0
	for _, px := range pxs {
		ft := batchB1TTFContour(t, fontPath, chars, px)
		if len(ft) < wantCount-200 {
			t.Fatalf("%s @%.0fpx: only %d glyphs parsed, want ≥%d (parser regression)",
				name, px, len(ft), wantCount-200)
		}
		bad, npt, noff, ngrp := 0, 0, 0, 0
		var badR, nptR, noffR, ngrpR []rune
		for _, r := range chars {
			ftG, ok := ft[r]
			if !ok {
				continue
			}
			gid := GlyphID(face.GlyphIndex(r))
			contours, err := ParseGlyfContours(fontData, gid)
			if err != nil || contours == nil {
				continue
			}
			hinted, _ := autoHintContourPoints(contours, face, gid, px, HintingVertical)
			if hinted == nil || len(hinted.Points) == 0 {
				continue
			}
			pts, tags, ends := hintedContoursToFT26(hinted)
			// ① 点数
			if len(ftG.pts) != len(pts) {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			// ② 坐标 + ③ on/off（TTF：FT tag1=on，tag0=conic off）
			offBad := false
			dbgN := 0
			for i, p := range pts {
				if p.x != ftG.pts[i][0] || p.y != ftG.pts[i][1] {
					bad++
					badR = append(badR, r)
					offBad = true
					if dbgN < 3 {
						dbgN++
						fmt.Printf("  DBGC %U @%.0fpx pt%d: engine(%d,%d) ft(%d,%d) tag %d vs %d\n",
							r, px, i, p.x, p.y, ftG.pts[i][0], ftG.pts[i][1],
							int(tags[i]), ftG.pts[i][2])
					}
					if r == 0x513F {
						fmt.Printf("  DBGFULL %U @%.0fpx:\n", r, px)
						for j := range pts {
							fmt.Printf("    pt%d engine(%d,%d) ft(%d,%d)\n",
								j, pts[j].x, pts[j].y, ftG.pts[j][0], ftG.pts[j][1])
						}
						for j := len(pts); j < len(ftG.pts); j++ {
							fmt.Printf("    pt%d engine(ABSENT) ft(%d,%d)\n",
								j, ftG.pts[j][0], ftG.pts[j][1])
						}
					}
					break
				}
				on := tags[i] == ftTagOn
				if (ftG.pts[i][2] == 1) != on {
					bad++
					noff++
					noffR = append(noffR, r)
					offBad = true
					break
				}
			}
			if offBad {
				continue
			}
			// ④ 分组
			if len(ends) != len(ftG.ends) {
				bad++
				ngrp++
				ngrpR = append(ngrpR, r)
				continue
			}
			grpBad := false
			for ci, e := range ends {
				if int64(e) != ftG.ends[ci] {
					bad++
					ngrp++
					ngrpR = append(ngrpR, r)
					grpBad = true
					break
				}
			}
			if grpBad {
				continue
			}
		}
		if bad > 0 {
			totalBad += bad
			fmt.Printf("B1 %s @%.0fpx: bad=%d (npt=%d off=%d grp=%d)\n",
				name, px, bad, npt, noff, ngrp)
			if len(badR) > 0 {
				fmt.Printf("  coord: %s\n", string(badR[:min(40, len(badR))]))
			}
			if len(noffR) > 0 {
				fmt.Printf("  off:   %s\n", string(noffR[:min(40, len(noffR))]))
			}
			if len(ngrpR) > 0 {
				fmt.Printf("  grp:   %s\n", string(ngrpR[:min(40, len(ngrpR))]))
			}
		}
	}
	if totalBad > 0 {
		t.Errorf("B1 %s: 累计 bad=%d (must be 0)", name, totalBad)
	} else {
		fmt.Printf("B1 %s: %d 档 × %d 字四维度全部一致\n", name, len(pxs), wantCount)
	}
	return totalBad
}

// TestB1TTFWqyCJK：wqy-microhei-nohint（glyf 无字节码，FT light→autofit）
// × cjk3000 × 档位（B1b 先 10/12/16/24 验证直通链）。
func TestB1TTFWqyCJK(t *testing.T) {
	path := "testdata/wqy-microhei-nohint.ttf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("%s unavailable: %v", path, err)
	}
	b1TTFRunScan(t, "wqy-nohint", path, "hint/testdata/cjk3000.txt", 3000,
		[]float64{10, 12, 16, 24})
}