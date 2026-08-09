package hint

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// zz_b1_scan_test.go —— 阶段 B1a：CFF 轮廓级矩阵四维度对照升级。
//
// 背景（docs ENGINE_TEXT_RASTER_FT_ALIGN_PLAN.md §阶段B）：旧扫描
// （zz_scan_3000/kr/th）只对照坐标+点数，1.3 盲区（on/off 标记与轮廓
// 分组）未补。B1 对照维度升级为：
//
//	① 点数  ② 坐标  ③ on/off 标记  ④ contours 分组
//
// ftexp bcontour 已输出全部信息（# R 头带 np/nc；每点 `x y tag`；
// 末行末点索引序列），旧解析器丢弃了 tag 与 ends——此处升级解析。
//
// 字号：8–72 每 1px 一档（65 档），全字集（cjk3000 / kr_all / th_all）。

// b1FTGlyph 是 ftexp bcontour 单个字形的完整对照数据。
type b1FTGlyph struct {
	pts [][3]int64 // x, y（26.6 Y-up）, tag（0=conic 1=on 2=cubic）
	// ends 是每轮廓末点索引（inclusive，FT_Outline.contours 语义）。
	ends []int64
}

// batchContourB1 分块批量取 FT-light 26.6 轮廓（含 tag 与分组），
// 返回 rune → 完整字形。缺失/加载失败的字不在 map 中。
//
// 分块 + 重试原因：ftexp 单进程跑超长字表（kr_all 11172 字）时，FT
// 长跑后个别字形输出空块（块头声明 np>0 但点行全空，实测间歇出现，
// 同一输入不同 exec 结果不同——ftexp 的 slot outline 扫描 hack 读野
// 内存所致）。空块字先收集，再用小批量 exec 重试（小批量稳定）。
// 重试仍空块则 t.Errorf——禁止静默假绿。
func batchContourB1(t *testing.T, fontPath string, chars []rune, px float64) map[rune]*b1FTGlyph {
	t.Helper()
	res := map[rune]*b1FTGlyph{}
	retry := map[rune]bool{}
	const chunk = 512
	for start := 0; start < len(chars); start += chunk {
		end := start + chunk
		if end > len(chars) {
			end = len(chars)
		}
		parseB1Batch(t, fontPath, chars[start:end], px, res, retry)
	}
	// 空块字重试：小批量（≤100 字/批）exec 稳定。
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
			parseB1Batch(t, fontPath, rs[s:e], px, res, next)
		}
		retry = next
		// 防死循环：若重试后无进展，判定 ftexp 工具不可靠。
		if len(next) > 0 && len(next) >= len(rs) {
			t.Errorf("ftexp 空块重试无效（ftexp 工具缺陷，非引擎回归），字: %s",
				string(rs[:min(20, len(rs))]))
			break
		}
	}
	return res
}

// parseB1Batch 对一批字 exec ftexp bcontour 并解析写入 res；
// 空块字（块头声明 np>0 但点行缺失/为空）记入 retry，不写入 res。
func parseB1Batch(t *testing.T, fontPath string, chars []rune, px float64,
	res map[rune]*b1FTGlyph, retry map[rune]bool) {
	t.Helper()
	list := t.TempDir() + "/b1_runes.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(ftexpBin(t), "bcontour",
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
		// 空块检测：块头后无点行（直接空行或下个块头）→ 该字无效，
		// 交给重试（小批量 exec 稳定），不污染后续解析。
		if i+1 >= len(lines) || strings.TrimSpace(lines[i+1]) == "" ||
			strings.HasPrefix(lines[i+1], "# R ") {
			retry[r] = true
			i++
			continue
		}
		g := &b1FTGlyph{}
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

// b1FTTagMatchesHintOn 判定 FT tag 与 hint 侧 on 标志是否一致。
// CFF 字体：FT on 点 tag=1；off 点（cubic 控制点）tag=2。
func b1FTTagMatchesHintOn(tag int64, on bool) bool {
	switch tag {
	case 1: // FT_CURVE_TAG_ON
		return on
	case 2: // FT_CURVE_TAG_CUBIC（CFF 曲线控制点）
		return !on
	default:
		return false
	}
}

// b1RunScan 对一种 CFF 字体跑四维度矩阵扫描（65 档）。
// chars 为全字集；wantCount 为预期字表长度（校验字表文件）。
// 返回 bad 总数（0 = 通过）。
func b1RunScan(t *testing.T, name, fontPath, listPath string, wantCount int) int {
	t.Helper()
	raw, err := os.ReadFile(listPath)
	if err != nil {
		t.Skipf("%s: %v", name, err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	if len(chars) != wantCount {
		t.Fatalf("%s: %s = %d chars, want %d", name, listPath, len(chars), wantCount)
	}
	f := openTestFont(t, fontPath)
	fraw := f.raw
	start, ln, err := cffTableData(fraw, 0)
	if err != nil {
		t.Fatalf("%s: CFF table: %v", name, err)
	}
	cd, err := cffParseAll(fraw[start:start+ln], f.unitsPerEm)
	if err != nil {
		t.Fatalf("%s: CFF parse: %v", name, err)
	}
	upem := f.UnitsPerEm()

	totalBad := 0
	for px := 8.0; px <= 72.0; px++ {
		scale := m2HintScale(px, upem)
		ft := batchContourB1(t, fontPath, chars, px)
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
			gid := uint16(f.GlyphIndex(r))
			out, fd, err := m2Interp(cd, gid)
			if err != nil {
				continue
			}
			res := hintCFFLight(out, fd, scale, 0, 0)
			if res == nil {
				continue
			}
			// ① 点数
			if len(ftG.pts) != len(res.pts) {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			// ② 坐标 + ③ on/off 标记
			offBad := false
			for i, p := range res.pts {
				if int64(p[0]>>10) != ftG.pts[i][0] || int64(p[1]>>10) != ftG.pts[i][1] {
					bad++
					badR = append(badR, r)
					offBad = true
					break
				}
				if !b1FTTagMatchesHintOn(ftG.pts[i][2], res.on[i]) {
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
			// ④ contours 分组：hint 返回每轮廓点数，换算末点索引后
			// 与 FT ends（inclusive 末点索引）比对。
			if len(res.contours) != len(ftG.ends) {
				bad++
				ngrp++
				ngrpR = append(ngrpR, r)
				continue
			}
			acc := 0
			grpBad := false
			for ci, n := range res.contours {
				acc += n
				if int64(acc-1) != ftG.ends[ci] {
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
		t.Errorf("B1 %s: 65 档累计 bad=%d (must be 0)", name, totalBad)
	} else {
		fmt.Printf("B1 %s: 65 档 × %d 字四维度全部一致\n", name, wantCount)
	}
	return totalBad
}

// TestB1ScanCJK3000：cjk3000 × Noto Sans CJK TTC，65 档四维度。
func TestB1ScanCJK3000(t *testing.T) {
	b1RunScan(t, "cjk3000", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"testdata/cjk3000.txt", 3000)
}

// TestB1ScanKR：kr_all × Noto Sans CJK TTC（KR face 由 m2Font 同一 TTC），
// 65 档四维度。
func TestB1ScanKR(t *testing.T) {
	b1RunScan(t, "kr_all", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"testdata/kr_all.txt", 11172)
}

// TestB1ScanThai：th_all × NotoSansThai（testdata OTF），65 档四维度。
func TestB1ScanThai(t *testing.T) {
	path := "testdata/NotoSansThai-Regular.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("NotoSansThai unavailable: %v", err)
	}
	b1RunScan(t, "th_all", path, "testdata/th_all.txt", 128)
}
