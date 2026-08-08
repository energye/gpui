package hint

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	gtextcff "github.com/go-text/typesetting/font/cff"
	"github.com/go-text/typesetting/font/opentype"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// TestCFF2VarBlendMatchGT 对照 go-text 的 CFF2 变体 blend：同一 glyph
// 在 wght=0/0.5/1/-1 坐标下解释出的轮廓必须逐点一致（对照器 = 门禁）。
//
// 字集 = a-z/A-Z/0-9 全 62 字（与 TestLightHintCFF2 同一 sample），
// 4 个 wght 点位全覆盖。
//
// go-text 的段序列含闭轮廓回到起点的冗余段，量化（F26.6）后先去掉末尾
// 与首点重合的闭合点再逐点比较。
func TestCFF2VarBlendMatchGT(t *testing.T) {
	path := filepath.Join("..", "testdata", "source-sans", "VF", "SourceSans3VF-Upright.otf")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no CFF2 VF: %v", err)
	}
	f := openTestFont(t, path)
	raw := f.raw
	start, ln, err := cff2TableData(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	cd, err := cff2ParseAll(raw[start:start+ln], f.unitsPerEm)
	if err != nil {
		t.Fatal(err)
	}
	gtcf, err := gtextcff.ParseCFF2(raw[start : start+ln])
	if err != nil {
		t.Fatal(err)
	}
	fd := cd.fds[0]
	sample := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	if len(sample) != 62 {
		t.Fatalf("sample = %d chars, want 62", len(sample))
	}
	for _, r := range sample {
		gid := uint16(f.GlyphIndex(r))
		if gid == 0 {
			t.Errorf("%c: not mapped", r)
			continue
		}
		for _, wght := range []float64{0, 0.5, 1.0, -1.0} {
			gtSeq, _, err := gtcf.LoadGlyph(gid, []tables.Coord{f2dot14(wght)})
			if err != nil {
				t.Fatalf("%c gt load wght=%v: %v", r, wght, err)
			}
			gtPts := expandGTSegs(gtSeq)
			blend := &cff2BlendData{vstore: cd.vstore, coords: []cff2Coord{{axis: 0, value: wght}}}
			cs, err := interpretCharstring2(cd.charStrings[gid], fd.subrs, cd.globalSubrs, 0, 0, blend)
			if err != nil {
				t.Fatalf("%c my load wght=%v: %v", r, wght, err)
			}
			var myPts [][]float64
			for _, p := range cs.pts {
				myPts = append(myPts, []float64{float64(p.x), float64(p.y)})
			}
			myPts = dropClosePoints(myPts)
			if len(gtPts) != len(myPts) {
				t.Errorf("%c wght=%v: gt %d pts vs my %d", r, wght, len(gtPts), len(myPts))
				continue
			}
			bad := 0
			for i := range gtPts {
				gt26 := int32(math.Round(gtPts[i][0] * 64))
				gt26y := int32(math.Round(gtPts[i][1] * 64))
				my26 := int32(math.Round(myPts[i][0] * 64))
				my26y := int32(math.Round(myPts[i][1] * 64))
				// go-text 全程 float32、本实现 float64：blend 中间舍入在 1/64
				// 边界处可能差 1 个量化单位（实测 'w' wght=0.5），±1 容差吸收。
				if abs32(gt26-my26) > 1 || abs32(gt26y-my26y) > 1 {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%c wght=%v: %d/%d pts mismatch vs go-text", r, wght, bad, len(gtPts))
			}
		}
	}
}

func dropClosePoints(pts [][]float64) [][]float64 {
	for len(pts) > 1 {
		last := pts[len(pts)-1]
		if q26(last[0]) == q26(pts[0][0]) && q26(last[1]) == q26(pts[0][1]) {
			pts = pts[:len(pts)-1]
			continue
		}
		break
	}
	return pts
}

func q26(v float64) int32 { return int32(v * 64) }

func f2dot14(v float64) tables.Coord {
	if v > 1 {
		v = 1
	}
	if v < -1 {
		v = -1
	}
	return tables.Coord(int32(v * 16384))
}

func q32(v float64) int64 { return int64(math.Round(v * 64)) }

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// expandGTSegs 把 go-text 段序列展开为完整点列：MoveTo 起点 + 各段全部点
// （quad/cubic 控制点不可丢），并按轮廓删除闭合冗余点（闭合段终点 = 轮廓
// 起点；blend 后 float32 两次运算有微差，量化 1/64 判定）。
//
// 另按 FT 语义合并零长直线段：go-text 保留 charstring 中的 zero-length
// lineto（如 CJK 字形的「回笔」），FT cf2 的 lineTo 忽略零长段
// （cf2_glyphpath_lineTo，pshints.c:1743-1765，我们 cffcs.go lineTo 同义）
// 且量化相等才判零长——段内层合并避免跨轮廓误并；删除闭合冗余点后
// 结束。
func expandGTSegs(gtSeq []opentype.Segment) [][]float64 {
	var gtPts [][]float64
	var curStart [2]float64
	for i, s := range gtSeq {
		if s.Op == opentype.SegmentOpMoveTo {
			p := s.Args[0]
			curStart = [2]float64{float64(p.X), float64(p.Y)}
			gtPts = append(gtPts, []float64{curStart[0], curStart[1]})
			continue
		}
		args := s.ArgsSlice()
		isContourLast := i == len(gtSeq)-1 || (i+1 < len(gtSeq) && gtSeq[i+1].Op == opentype.SegmentOpMoveTo)
		for j, p := range args {
			isEnd := j == len(args)-1
			if isContourLast && isEnd && q32(float64(p.X)) == q32(curStart[0]) && q32(float64(p.Y)) == q32(curStart[1]) {
				continue
			}
			// FT 零长段：直线段（单参数段）终点与前一点量化相等则忽略
			// （曲线段控制点即使与起点重合也必须保留，否则形变）。
			if len(args) == 1 && len(gtPts) > 0 {
				prev := gtPts[len(gtPts)-1]
				if q32(float64(p.X)) == q32(prev[0]) && q32(float64(p.Y)) == q32(prev[1]) {
					continue
				}
			}
			gtPts = append(gtPts, []float64{float64(p.X), float64(p.Y)})
		}
	}
	return dropClosePoints(gtPts)
}

// hasOp15 判断 charstring 是否含 vsindex(15) 指令（M5 vsindex 语义扫描）。
func hasOp15(b []byte) bool {
	for _, x := range b {
		if x == 15 {
			return true
		}
	}
	return false
}

// osStat 包装 os.Stat（供各扫描窗统一字体存在性判定）。
func osStat(p string) (os.FileInfo, error) { return os.Stat(p) }
