package hint

import (
	"math"
	"path/filepath"
	"testing"

	gtextcff "github.com/go-text/typesetting/font/cff"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// TestCFF2VSIndexBlendMatchGT：M5 vsindex 语义门禁。
//
// 与 TestCFF2VarBlendMatchGT（62 字 × 4 点位，隐式含 vsindex 字形）不同，
// 本窗**显式**扫描全部含 vsindex(15) 指令的字形（SourceSans3VF 实测 694
// 字形），每个 × wght=0/0.5/1/-1 四点位，对照 go-text 解释器逐点一致。
//
// vsindex 是 CFF2 的核心语义：字形中途切换 ItemVariationData 下标，后续
// blend 用新下标取 region 标量（cffcs.go case 15 → setScalars）。对照
// go-text cff2CharstringHandler.setVSIndex，坐标量化 F26.6 后 ±1 容差
// （float32/float64 blend 中间舍入，见 cff2_gate_test.go 注释）。
func TestCFF2VSIndexBlendMatchGT(t *testing.T) {
	path := filepath.Join("..", "testdata", "source-sans", "VF", "SourceSans3VF-Upright.otf")
	if _, err := osStat(path); err != nil {
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
	if len(cd.charStrings) != 2478 {
		t.Fatalf("charStrings = %d, want 2478 (regression guard)", len(cd.charStrings))
	}
	fd := cd.fds[0]

	scanned, withVS := 0, 0
	for gid := uint16(0); gid < uint16(len(cd.charStrings)); gid++ {
		cs := cd.charStrings[gid]
		scanned++
		if !hasOp15(cs) {
			continue
		}
		withVS++
		for _, wght := range []float64{0, 0.5, 1.0, -1.0} {
			gtSeq, _, err := gtcf.LoadGlyph(gid, []tables.Coord{f2dot14(wght)})
			if err != nil {
				t.Fatalf("gid %d gt load wght=%v: %v", gid, wght, err)
			}
			gtPts := expandGTSegs(gtSeq)
			blend := &cff2BlendData{vstore: cd.vstore, coords: []cff2Coord{{axis: 0, value: wght}}}
			my, err := interpretCharstring2(cs, fd.subrs, cd.globalSubrs, 0, 0, blend)
			if err != nil {
				t.Fatalf("gid %d my load wght=%v: %v", gid, wght, err)
			}
			myPts := dropPerContour(my, gid)
			if len(gtPts) != len(myPts) {
				t.Errorf("gid %d wght=%v: gt %d pts vs my %d", gid, wght, len(gtPts), len(myPts))
				continue
			}
			bad := 0
			for i := range gtPts {
				dx := int32(math.Round(gtPts[i][0]*64)) - int32(math.Round(myPts[i][0]*64))
				dy := int32(math.Round(gtPts[i][1]*64)) - int32(math.Round(myPts[i][1]*64))
				if abs32(dx) > 1 || abs32(dy) > 1 {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("gid %d wght=%v: %d/%d pts mismatch vs go-text", gid, wght, bad, len(gtPts))
			}
		}
	}
	if scanned != 2478 {
		t.Fatalf("scanned = %d, want 2478", scanned)
	}
	if withVS < 100 {
		t.Fatalf("vsindex glyphs = %d, want >=100 (semantic coverage guard)", withVS)
	}
	t.Logf("M5 vsindex 门禁: %d/%d 字形含 vsindex × 4 wght 全对照 go-text", withVS, scanned)
}
