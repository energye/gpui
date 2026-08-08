package hint

import (
	"os"
	"path/filepath"
	"testing"

	gtextcff "github.com/go-text/typesetting/font/cff"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// TestCFF2VarBlendMatchGT 对照 go-text 的 CFF2 变体 blend：同一 glyph
// 在 wght=0/0.5/1/-1 坐标下解释出的轮廓必须逐点一致（对照器 = 门禁）。
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
	gid := uint16(f.GlyphIndex('H'))
	fd := cd.fds[0]
	for _, wght := range []float64{0, 0.5, 1.0, -1.0} {
		gtSeq, _, err := gtcf.LoadGlyph(gid, []tables.Coord{f2dot14(wght)})
		if err != nil {
			t.Fatalf("gt load wght=%v: %v", wght, err)
		}
		var gtPts [][]float64
		for _, s := range gtSeq {
			a := s.ArgsSlice()
			gtPts = append(gtPts, []float64{float64(a[len(a)-1].X), float64(a[len(a)-1].Y)})
		}
		gtPts = dropClosePoints(gtPts)
		blend := &cff2BlendData{vstore: cd.vstore, coords: []cff2Coord{{axis: 0, value: wght}}}
		cs, err := interpretCharstring2(cd.charStrings[gid], fd.subrs, cd.globalSubrs, 0, 0, blend)
		if err != nil {
			t.Fatalf("my load wght=%v: %v", wght, err)
		}
		var myPts [][]float64
		for _, p := range cs.pts {
			myPts = append(myPts, []float64{float64(p.x), float64(p.y)})
		}
		myPts = dropClosePoints(myPts)
		if len(gtPts) != len(myPts) {
			t.Errorf("wght=%v: gt %d pts vs my %d", wght, len(gtPts), len(myPts))
			continue
		}
		bad := 0
		for i := range gtPts {
			gt26 := int32(gtPts[i][0] * 64)
			gt26y := int32(gtPts[i][1] * 64)
			my26 := int32(myPts[i][0] * 64)
			my26y := int32(myPts[i][1] * 64)
			if gt26 != my26 || gt26y != my26y {
				bad++
			}
		}
		if bad > 0 {
			t.Errorf("wght=%v: %d/%d pts mismatch vs go-text", wght, bad, len(gtPts))
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
