package hint

import (
	"os"
	"strings"
	"fmt"
	"testing"
)

// TestDbg8pxCJK 8px 全字集对照（M3 只验 10-24px，8px 是盲区）。
func TestDbg8pxCJK(t *testing.T) {
	raw, err := os.ReadFile("testdata/cjk3000.txt")
	if err != nil {
		t.Skipf("cjk3000.txt unavailable: %v", err)
	}
	chars := []rune(strings.TrimSpace(string(raw)))
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()

	for _, px := range []float64{8, 9} {
		scale := m2HintScale(px, upem)
		ft := batchContour26Light(t, chars, px)
		bad := 0
		npt := 0
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
		fmt.Printf("dbg8px: bad@%.0fpx=%d (npt=%d) of %d\n", px, bad, npt, len(chars))
		if len(badR) > 0 {
			for i := 0; i < len(badR) && i < 40; i += 20 {
				end := i + 20
				if end > len(badR) {
					end = len(badR)
				}
				fmt.Printf("  bad: %s\n", string(badR[i:end]))
			}
		}
		if len(nptR) > 0 {
			fmt.Printf("  npt: %s\n", string(nptR))
		}
	}
}