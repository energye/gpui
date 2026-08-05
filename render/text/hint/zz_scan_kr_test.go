package hint

import (
	"fmt"
	"os"
	"testing"
)

// TestScanKR：韩文 CFF 扩集（docs §5.1 M3 收尾项）。
//
// 全 11172 个 Hangul 音节（U+AC00–U+D7A3）对照 ftexp bcontour 批量模式
// （FT-light 26.6，face 0 与 m2Font Face(14) 同一 TTC 的 CFF 数据）。
func TestScanKR(t *testing.T) {
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()
	var chars []rune
	for cp := 0xAC00; cp < 0xD7A4; cp++ {
		chars = append(chars, rune(cp))
	}
	list := "/tmp/opencode/kr_all.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, px := range []float64{10, 12, 14, 16, 20, 24} {
		scale := m2HintScale(px, upem)
		ft := batchContour26Light(t, chars, px)
		if len(ft) < 11000 {
			t.Fatalf("kr: only %d glyphs parsed, want ≥11000", len(ft))
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
		fmt.Printf("kr: bad@%.0fpx=%d (npt=%d)\n", px, bad, npt)
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
