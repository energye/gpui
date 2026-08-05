package text

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// zz_m4_probe_test.go — M4 探针：FT afcjk light vs Go autohint（CJK 脚本）差量。
// 用 wqy-microhei-nohint.ttf（剥离字节码）测：ftexp contour light vs
// ExtractOutlineHinted(Vertical)。跑完删除。

func zzM4FTContour(t *testing.T, fontPath string, r rune, px int) (xs, ys []float64) {
	cmd := exec.Command("/tmp/opencode/ftexp/ftexp", "contour", fontPath, string(r), strconv.Itoa(px), "l")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ftexp: %v %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		t.Fatalf("short output: %q", out)
	}
	var np, nc, adv int
	if _, err := fmt.Sscanf(lines[0], "%d %d %d", &np, &nc, &adv); err != nil {
		t.Fatalf("header: %v", err)
	}
	for i := 0; i < np; i++ {
		var x, y int64
		var tg int
		if _, err := fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tg); err != nil {
			t.Fatalf("pt %d: %v", i, err)
		}
		xs = append(xs, float64(x)/64)
		ys = append(ys, float64(y)/64)
	}
	return
}

func TestZZM4Probe(t *testing.T) {
	fontPath := "testdata/wqy-microhei-nohint.ttf"
	src, err := NewFontSourceFromFile(fontPath)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	face := src.Parsed()
	runes := []rune{'一', '二', '田', '目', '日', '好', '永'}
	for _, px := range []int{10, 12, 14, 16} {
		ext := NewOutlineExtractor()
		for _, r := range runes {
			gid := face.GlyphIndex(r)
			if gid == 0 {
				t.Logf("skip %q no glyph", r)
				continue
			}
			_, fty := zzM4FTContour(t, fontPath, r, px)
			out, err := ext.ExtractOutlineHinted(src.Parsed(), GlyphID(gid), float64(px), HintingVertical)
			if err != nil {
				t.Fatalf("go extract %q: %v", r, err)
			}
			ftMinY, ftMaxY := 0.0, 0.0
			for _, y := range fty {
				if y < ftMinY {
					ftMinY = y
				}
				if y > ftMaxY {
					ftMaxY = y
				}
			}
			goMinY, goMaxY := 0.0, 0.0
			for _, s := range out.Segments {
				for _, p := range s.Points {
					y := float64(p.Y)
					if y < goMinY {
						goMinY = y
					}
					if y > goMaxY {
						goMaxY = y
					}
				}
			}
			// FT y 是 Y-up；Go 是 Y-down。比较：FT top(up 最大) vs Go top(down 最小)
			ftTop64 := int64(-ftMaxY * 64) // top 在 baseline 上方 → Y-down 负 → 转为正
			ftBot64 := int64(-ftMinY * 64)
			goTop64 := int64(-goMinY * 64)
			goBot64 := int64(-goMaxY * 64)
			t.Logf("px=%d %q: FT top=%d/64 bot=%d/64 | Go top=%d/64 bot=%d/64 | dTop=%+d dBot=%+d",
				px, r, ftTop64, ftBot64, goTop64, goBot64, goTop64-ftTop64, goBot64-ftBot64)
		}
	}
}
