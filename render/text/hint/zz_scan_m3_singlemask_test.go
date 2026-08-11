package hint

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestScanM3SingleMask：C3 单 mask 扫描窗（Raster-FT-ALIGN 阶段 C · C3）。
//
// 背景：M3 的多 mask 专用窗（TestScanM3）只筛 hintmaskCount > 1 的字；
// 单 mask / 零 mask 字形（hintmaskCount ≤ 1）不在该扫描集（真源 §1.3 点名
// 的盲区）。虽然 TestScanCJK3000 全量窗已隐含覆盖绝大多数码位，但
// hintmaskCount==1（charstring 里有 hintmask 指令、但所有区共用同一 mask）
// 的字此前没有显式判红窗。本窗从 cjk3000 全字集筛出 ≤1 的字逐字对照
// ftexp light 轮廓 26.6，确认 bad=0，杜绝「单 mask 语义」回归漏检。
func TestScanM3SingleMask(t *testing.T) {
	raw, err := os.ReadFile("testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()

	// 筛选 hintmaskCount ≤ 1 的字（0=单区无 hintmask；1=单 mask 指令全字共用）。
	var sel []rune
	c1 := 0
	for _, r := range chars {
		gid := uint16(f.GlyphIndex(r))
		out, _, err := m2Interp(cd, gid)
		if err != nil {
			continue
		}
		if out.hintmaskCount <= 1 {
			sel = append(sel, r)
			if out.hintmaskCount == 1 {
				c1++
			}
		}
	}
	t.Logf("single-mask 窗：共 %d 字（其中 hintmaskCount==1 的单 mask 字 %d 个）", len(sel), c1)
	if len(sel) < 500 {
		t.Fatalf("cjk3000 单 mask 窗过小（%d < 500），解析异常？", len(sel))
	}

	for _, px := range []float64{10, 12, 14, 16, 20, 24} {
		scale := m2HintScale(px, upem)
		ft := batchContour26Light(t, sel, px)
		if len(ft) < len(sel)-100 {
			t.Fatalf("cjk3000 single-mask @%.0fpx: only %d glyphs parsed, want ≥%d", px, len(ft), len(sel)-100)
		}
		bad, npt := 0, 0
		var badR, nptR []rune
		for _, r := range sel {
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
		fmt.Printf("singlemask: bad@%.0fpx=%d (npt=%d, sel=%d)\n", px, bad, npt, len(sel))
		if len(badR) > 0 {
			s := string(badR)
			if len(s) > 60 {
				s = s[:60]
			}
			fmt.Printf("  bad: %s\n", s)
		}
		if bad > 0 {
			t.Errorf("cjk3000 single-mask @%.0fpx: bad=%d (must be 0)", px, bad)
		}
	}
}