package hint

import (
	"fmt"
	"testing"
)

func TestScanM3(t *testing.T) {
	f, cd := m2Font(t)
	var all string
	for _, g := range VerifierSet() {
		if len(g.Name) < 3 || g.Name[:3] != "cjk" {
			continue
		}
		all += g.Chars
	}
	seen := map[rune]bool{}
	var multi2 []rune
	for _, r := range all {
		if seen[r] {
			continue
		}
		seen[r] = true
		gid := uint16(f.GlyphIndex(r))
		out, _, err := m2Interp(cd, gid)
		if err != nil {
			continue
		}
		if out.hintmaskCount > 1 {
			multi2 = append(multi2, r)
		}
	}
	for _, px := range []float64{10, 12, 14, 16, 20, 24} {
		upem := f.UnitsPerEm()
		scale := m2HintScale(px, upem)
		bad := 0
		badR := []rune{}
		npt := 0
		nptR := []rune{}
		for _, r := range multi2 {
			gid := uint16(f.GlyphIndex(r))
			out, fd, err := m2Interp(cd, gid)
			if err != nil {
				continue
			}
			res := hintCFFLight(out, fd, scale, 0, 0)
			ft := ftContour26Light(t, r, px)
			if len(ft) != len(res.pts) {
				bad++
				npt++
				nptR = append(nptR, r)
				continue
			}
			for i, p := range res.pts {
				if int64(p[0]>>10) != ft[i][0] || int64(p[1]>>10) != ft[i][1] {
					bad++
					badR = append(badR, r)
					break
				}
			}
		}
		fmt.Printf("scan: multi2=%d bad@%.0fpx=%d (npt=%d)\n", len(multi2), px, bad, npt)
		for i := 0; i < len(badR); i += 20 {
			end := i + 20
			if end > len(badR) {
				end = len(badR)
			}
			fmt.Printf("  bad: %s\n", string(badR[i:end]))
		}
		if len(nptR) > 0 {
			fmt.Printf("  npt: %s\n", string(nptR))
		}
		if bad > 0 {
			t.Errorf("m3 multi-mask @%.0fpx: bad=%d (must be 0)", px, bad)
		}
	}
}
