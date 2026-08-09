package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// TestDbgProdContour 生产路径（ExtractOutlineHinted）轮廓 vs ftexp FT-light
// 26.6 逐点对照「静每合」各字号。检查 M 对照（hintCFFLight）与生产桥
// （rebuildSegmentsFromLightPts）在轮廓级是否一致。
func TestDbgProdContour(t *testing.T) {
	fontPath := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Skipf("font: %v", err)
	}
	defer src.Close()
	parsed := src.Parsed()
	// 注意：hint 包对照的是 hint.LightHint；生产 ExtractOutlineHinted →
	// cffLightHintOutline → hint.LightHintVar(…, nil)。同一引擎。
	for _, px := range []float64{8, 10, 12, 14, 16, 20, 24, 28, 32, 48} {
		for _, r := range []rune("静每合") {
			gid := GlyphID(parsed.GlyphIndex(r))
			ext := NewOutlineExtractor()
			o, err := ext.ExtractOutlineHinted(parsed, gid, px, HintingVertical)
			if err != nil || o == nil {
				t.Fatalf("%c %.0fpx: %v", r, px, err)
			}
			// 自研：每段取点（MoveTo/LineTo 1 点，QuadTo 2 点，CubicTo 3 点）
			// 转 26.6：X*64, Y*64（Y-down → FT Y-up 翻 -Y）
			var selfPts [][2]int64
			for _, s := range o.Segments {
				n := 1
				switch s.Op {
				case OutlineOpQuadTo:
					n = 2
				case OutlineOpCubicTo:
					n = 3
				}
				pts := s.Points[:n]
				for _, p := range pts {
					selfPts = append(selfPts, [2]int64{int64(p.X * 64), int64(-p.Y * 64)})
				}
			}
			ftPts := ftp26(t, fontPath, r, px)
			if len(ftPts) != len(selfPts) {
				fmt.Printf("%q %.0fpx: N pts self=%d ft=%d\n", r, px, len(selfPts), len(ftPts))
				continue
			}
			bad := 0
			for i := range selfPts {
				if selfPts[i][0] != ftPts[i][0] || selfPts[i][1] != ftPts[i][1] {
					if bad < 6 {
						fmt.Printf("%q %.0fpx pt%d self=(%d,%d) ft=(%d,%d)\n", r, px, i,
							selfPts[i][0], selfPts[i][1], ftPts[i][0], ftPts[i][1])
					}
					bad++
				}
			}
			if bad > 0 {
				fmt.Printf("%q %.0fpx: %d/%d pts differ (vs fc m3 expect 0)\n", r, px, bad, len(ftPts))
			} else {
				fmt.Printf("%q %.0fpx: %d pts 全部一致\n", r, px, len(ftPts))
			}
		}
	}
}

func ftp26(t *testing.T, fontPath string, r rune, px float64) [][2]int64 {
	t.Helper()
	bin := ftexpLocal(t)
	cmd := exec.Command(bin, "contour", fontPath, string(r), strconv.Itoa(int(px)), "l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("ftexp: %v %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		t.Fatalf("short ftexp: %s", out)
	}
	var np, nc, adv int
	fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv)
	pts := make([][2]int64, 0, np)
	for i := 1; i <= np && i < len(lines); i++ {
		var x, y, tag int64
		fmt.Sscanf(lines[i], "%d %d %d", &x, &y, &tag)
		pts = append(pts, [2]int64{x, y})
	}
	return pts
}