package hint

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gtextcff "github.com/go-text/typesetting/font/cff"
	"github.com/go-text/typesetting/font/opentype/tables"
)

// TestCFF2CJKVFBlendMatchGT：M5 CJK 可变实例化门禁。
//
// 对照源 = render/text/testdata/NotoSansSC-VF.otf（Google Fonts 官方
// Noto Sans SC Variable，CFF2 + wght 100–900，含大量 vsindex 指令字形）。
// 与拉丁 SourceSans3 互为补充：Latin VF 验证 blend 语义非拉丁字形、CJK
// VF 验证 3000 常用字实例化的坐标一致性（go-text 解释器同源对照，±1/64）。
//
// 采样：testdata/cjk3000.txt 里的 3000 常用字（与 M0–M3 同源采样），
// 每个 × wght=0/0.5/1（-1 在 Noto SC 为轴外值，go-text 会 clamp 到 min，
// 也一并对照）。
func TestCFF2CJKVFBlendMatchGT(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "cjk3000.txt"))
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	runes := []rune(strings.TrimSpace(string(raw)))
	if len(runes) != 3000 {
		t.Fatalf("cjk3000.txt = %d chars, want 3000", len(runes))
	}
	path := filepath.Join("..", "testdata", "NotoSansSC-VF.otf")
	if _, err := osStat(path); err != nil {
		t.Skipf("no CJK VF: %v", err)
	}
	f := openTestFont(t, path)
	rawf := f.raw
	start, ln, err := cff2TableData(rawf, 0)
	if err != nil {
		t.Fatal(err)
	}
	cd, err := cff2ParseAll(rawf[start:start+ln], f.unitsPerEm)
	if err != nil {
		t.Fatal(err)
	}
	gtcf, err := gtextcff.ParseCFF2(rawf[start : start+ln])
	if err != nil {
		t.Fatal(err)
	}
	fd := cd.fds[0]

	// 覆盖守卫：CJK VF 必须有 vsindex 字形（语义真实性的前提）
	withVS := 0
	for gid := uint16(0); gid < uint16(len(cd.charStrings)); gid++ {
		if hasOp15(cd.charStrings[gid]) {
			withVS++
		}
	}
	if withVS < 1000 {
		t.Fatalf("CJK VF vsindex glyphs = %d, want >=1000 (semantic coverage guard)", withVS)
	}
	t.Logf("CJK VF: %d charStrings, %d 含 vsindex", len(cd.charStrings), withVS)

	scanned, skipped, bad := 0, 0, 0
	for _, r := range runes {
		gid := uint16(f.GlyphIndex(r))
		if gid == 0 {
			skipped++
			continue
		}
		scanned++
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
			if len(gtPts) != len(cs.pts) {
				t.Errorf("%c wght=%v: gt %d pts vs my %d", r, wght, len(gtPts), len(cs.pts))
				bad++
				continue
			}
			for i := range gtPts {
				dx := int32(math.Round(gtPts[i][0]*64)) - int32(math.Round(cs.pts[i].x*64))
				dy := int32(math.Round(gtPts[i][1]*64)) - int32(math.Round(cs.pts[i].y*64))
				if abs32(dx) > 1 || abs32(dy) > 1 {
					bad++
				}
			}
		}
	}
	if skipped > 0 {
		t.Logf("skipped %d chars not mapped in SC VF", skipped)
	}
	if scanned < 2000 {
		t.Fatalf("scanned = %d, want >=2000 (CJK coverage guard)", scanned)
	}
	if bad > 0 {
		t.Fatalf("%d CJK VF point mismatches vs go-text", bad)
	}
	t.Logf("M5 CJK VF 门禁: %d 汉字 × 4 wght 全对照 go-text 一致（vsindex 字形 %d 个）", scanned, withVS)
}