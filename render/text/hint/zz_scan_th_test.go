package hint

import (
	"fmt"
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

// TestScanTH：泰文 CFF 扩集（docs §5.1 M3 收尾项）。
//
// 字体 = Noto Sans Thai Regular（CFF OTF，testdata/ 下）；
// 验证 U+0E00–U+0E7F 全泰文块（87 个有字形码位）对照 ftexp bcontour
// 批量模式（FT-light 26.6）。
func TestScanTH(t *testing.T) {
	thaiFont := "testdata/NotoSansThai-Regular.otf"
	src, err := text.NewFontSourceFromFile(thaiFont)
	if err != nil {
		t.Skipf("thai font unavailable: %v", err)
	}
	f := src.Face(0).Source().Parsed()
	provider, ok := f.(text.RawFontDataProvider)
	if !ok {
		t.Fatal("font lacks RawFontDataProvider")
	}
	raw := provider.RawFontData()
	start, ln, err := cffTableData(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	cd, err := cffParseAll(raw[start:start+ln], f.UnitsPerEm())
	if err != nil {
		t.Fatal(err)
	}
	upem := f.UnitsPerEm()
	var chars []rune
	for cp := 0x0E00; cp < 0x0E80; cp++ {
		chars = append(chars, rune(cp))
	}
	list := "/tmp/opencode/th_all.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, px := range []float64{10, 12, 14, 16, 20, 24} {
		scale := m2HintScale(px, upem)
		ft := batchContour26LightFont(t, thaiFont, chars, px)
		if len(ft) < 85 {
			t.Fatalf("th: only %d glyphs parsed, want ≥85", len(ft))
		}
		bad, npt := 0, 0
		var badR, nptR []rune
		for _, r := range chars {
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
		fmt.Printf("th: bad@%.0fpx=%d (npt=%d)\n", px, bad, npt)
		for i := 0; i < len(badR); i += 30 {
			end := i + 30
			if end > len(badR) {
				end = len(badR)
			}
			fmt.Printf("  bad: %s\n", string(badR[i:end]))
		}
		if len(nptR) > 0 {
			fmt.Printf("  npt: %s\n", string(nptR))
		}
	}
}
