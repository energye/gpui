package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestDbgYRange(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	for _, px := range []float64{16, 24} {
		r := '合'
		gid := GlyphID(parsed.GlyphIndex(r))
		ext := NewOutlineExtractor()
		o, _ := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
		minY, maxY := float32(1e9), float32(-1e9)
		for _, s := range o.Segments {
			for _, p := range s.Points {
				if p.X != 0 || p.Y != 0 {
					if p.Y < minY {
						minY = p.Y
					}
					if p.Y > maxY {
						maxY = p.Y
					}
				}
			}
		}
		fmt.Printf("生产 %.0fpx: Y∈[%.2f,%.2f] 高%.1f\n", px, minY, maxY, maxY-minY)

		cmd := exec.Command(ftexpLocal(t), "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
		out, _ := cmd.CombinedOutput()
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var np, nc, adv int
		fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
		fmin, fmax := float64(1e9), float64(-1e9)
		for i := 0; i < np; i++ {
			var x, y, tag int64
			fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag)
			yy := -float64(y) / 64.0
			if yy < fmin {
				fmin = yy
			}
			if yy > fmax {
				fmax = yy
			}
		}
		fmt.Printf("FT-26.6 %.0fpx: Y范围=[%.2f,%.2f] 高%.1f\n", px, fmin, fmax, fmax-fmin)
	}
}
