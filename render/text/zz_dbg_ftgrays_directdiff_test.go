package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render/text/hint"
)

// 诊断：hint.LightHintVar 26.6 输出 vs ftexp contour（FT_Outline）26.6 点列，
// 定位 A3 直通 bad 的源头（点数/坐标/tag/分组差异）。
func TestDbgFT26DirectDiff(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()

	for _, px := range []float64{12} {
		for _, r := range []rune("静每合") {
			gid := GlyphID(parsed.GlyphIndex(r))
			own := parsed.(*ownParsedFont)
			raw := own.RawFontData()
			isCFF2 := own.hasCFF2Table() && !own.hasCFFTable()
			pts, contours, _, err := hint.LightHintVar(raw, own.collectionIndex, isCFF2, uint16(gid), px, nil)
			if err != nil {
				t.Fatalf("%c LightHint: %v", r, err)
			}

			// ftexp FT_Outline
			cmd := exec.Command(ftexpLocal(t), "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("contour: %v", err)
			}
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			var np, nc int
			fmt.Sscanf(lines[0], "%d %d", &np, &nc)
			ftPts := make([][3]int64, np)
			for i := 0; i < np; i++ {
				var x, y, tag int64
				fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
				ftPts[i] = [3]int64{x, y, tag}
			}
			ftEnds := []int{}
			for _, s := range strings.Fields(lines[1+np]) {
				if v, err := strconv.Atoi(s); err == nil {
					ftEnds = append(ftEnds, v)
				}
			}

			fmt.Printf("%c %.0fpx: LightPts=%d contours=%d | FT_Outline np=%d nc=%d ends=%v\n",
				r, px, len(pts), len(contours), np, nc, ftEnds)
			n := len(pts)
			if n > np {
				n = np
			}
			diffPts := 0
			for i := 0; i < n; i++ {
				ftX, ftY := ftPts[i][0], ftPts[i][1]
				ftTag := ftPts[i][2]
				lp := pts[i]
				lpTag := int64(0)
				if lp.On {
					lpTag = 1
				}
				if lp.X != ftX || lp.Y != ftY || lpTag != ftTag {
					if diffPts < 8 {
						fmt.Printf("  pt %d: light=(%d,%d,tag%d) ft=(%d,%d,tag%d)\n", i, lp.X, lp.Y, lpTag, ftX, ftY, ftTag)
					}
					diffPts++
				}
			}
			fmt.Printf("  点差异: %d/%d (FT=%d)\n", diffPts, n, np)
		}
	}
}
