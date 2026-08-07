package text

import (
	"fmt"
	"os"
	"testing"
)

// cff2VFSourceSans 返回 SourceSans 3 VF OTTO（CFF2 轮廓，无 glyf）。
// 该字体是 CFF2 VF 唯一路径测试样本：有 fvar + CFF2 表，glyf 缺失，
// 能保证 extractCFF2Outline 而不是 CFF1/glyf 分支被走到。
const cff2VFSourceSans = "testdata/source-sans/VF/SourceSans3VF-Upright.otf"

// cff2FTChars 是全字集对照样本：大小写拉丁 + 数字（SourceSans 覆盖段）。
// bcontour 批量模式一次启动 FT 遍历，避免逐字起进程。
var cff2FTChars = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

// TestCFF2VFNoHintMatchesFT 是 CFF2 VF 的正式对照窗（M5 前置，探针升级）：
// Go 管线提取的 CFF2 轮廓（default master，无 hint）逐点与 FT nohint.
// contour 批量对照，覆盖率必须 100%（容差 <=0.1px 对应 26.6 定点舍入差）。
// 缺字体 -> Skipf（禁止静默假绿，见 AGENTS.md）。
func TestCFF2VFNoHintMatchesFT(t *testing.T) {
path := cff2VFSourceSans
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no CFF2 VF font: %v", err)
	}
	source, err := NewFontSourceFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	parsed := source.Parsed()
	own, ok := parsed.(*ownParsedFont)
	if !ok {
		t.Fatal("not ownParsedFont")
	}
	_, hasCFF2 := own.tables["CFF2"]
	if !hasCFF2 {
		t.Fatal("CFF2 table missing; wrong sample")
	}

	extractor := NewOutlineExtractor()
	ft := batchWqyContour26Hint(t, path, cff2FTChars, 16, "n") // hint=n: nohint baseline
	bad := 0
	var badLogs []string
	for _, r := range cff2FTChars {
		ftPts, ok := ft[r]
		if !ok {
			bad++
			badLogs = append(badLogs, fmt.Sprintf("%c FT missing", r))
			continue
		}
		if len(ftPts) == 0 {
			bad++
			badLogs = append(badLogs, fmt.Sprintf("%c FT empty", r))
			continue
		}
		gid := GlyphID(parsed.GlyphIndex(r))
		o, err := extractor.ExtractOutlineHinted(parsed, gid, 16, HintingNone)
		if err != nil || o == nil || len(o.Segments) == 0 {
			bad++
			badLogs = append(badLogs, fmt.Sprintf("%c go outline err %v", r, err))
			continue
		}
		gPts := goSegments(o.Segments)
		coverage, worst := cover26(ftPts, gPts)
		if coverage != 100 {
			bad++
			badLogs = append(badLogs, fmt.Sprintf("%c cov=%d%% worst=%d(26.6)", r, coverage, worst))
		}
	}
	if bad > 0 {
		t.Errorf("CFF2 VS FT: %d/%d characters not fully covered", bad, len(cff2FTChars))
		for _, l := range badLogs {
			t.Logf("  %s", l)
		}
	}
}

// goSegments 把提取的 OutlineSegment（px, Y-down）展开为 26.6 fixed Y-up 点集，
// Move/Line/Quad/Cubic 全部计入（含控制点），用于与 FT contour 的点集匹配。
func goSegments(segs []OutlineSegment) [][2]int64 {
	out := make([][2]int64, 0, len(segs)*2)
	for _, s := range segs {
		n := segPointCount(s.Op)
		for j := 0; j < n; j++ {
			out = append(out, [2]int64{int64(s.Points[j].X*64), int64(-s.Points[j].Y * 64)})
		}
	}
	return out
}

// cover26 计算 FT 点在 go 点集内的覆盖率(%)与最远距离（26.6 fixed）。
// 每个 FT 点找最近 go 点，距离 <= tol(6=0.1px) 记为 covered。
func cover26(ft, goPts [][2]int64) (coverage, worst int) {
	const tol = 6
	matched := 0
	for _, f := range ft {
		best := int64(1 << 30)
		for _, g := range goPts {
			dx := f[0] - g[0]
			if dx < 0 {
				dx = -dx
			}
			dy := f[1] - g[1]
			if dy < 0 {
				dy = -dy
			}
			d := dx
			if dy > d {
				d = dy
			}
			if d < best {
				best = d
			}
		}
		if best <= tol {
			matched++
		}
		if int(best) > worst {
			worst = int(best)
		}
	}
	return 100 * matched / len(ft), worst
}