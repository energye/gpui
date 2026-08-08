package hint

import (
	"os"
	"strings"
	"testing"
)

// TestLightHintMatchesFT：公开入口 LightHint 对照 ftexp light 26.6
// （与 zz_scan_3000 同一对照链，验证接线层不破坏对齐）。
func TestLightHintMatchesFT(t *testing.T) {
	path := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no CJK: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sample := []rune("日田目钮我一中")
	for _, px := range []float64{12, 14} {
		ft := batchContour26LightFont(t, path, sample, px)
		for _, r := range sample {
			ftPts, ok := ft[r]
			if !ok {
				continue
			}
			gid := uint16(openTestFont(t, path).GlyphIndex(r))
			pts, contours, _, err := LightHint(raw, 0, false, gid, px)
			if err != nil {
				t.Fatalf("%c @%.0fpx: %v", r, px, err)
			}
			_ = contours
			if len(pts) != len(ftPts) {
				t.Errorf("%c @%.0fpx: LightHint %d pts vs FT %d", r, px, len(pts), len(ftPts))
				continue
			}
			bad := 0
			for i, p := range pts {
				if p.X != ftPts[i][0] || p.Y != ftPts[i][1] {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%c @%.0fpx: %d/%d pts mismatch", r, px, bad, len(pts))
			}
		}
	}
}

// TestLightHintContours：轮廓分组与点数非负（结构 sanity）。
func TestLightHintContours(t *testing.T) {
	path := "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no CJK: %v", err)
	}
	f := openTestFont(t, path)
	gid := uint16(f.GlyphIndex('日'))
	pts, contours, _, err := LightHint(raw, 0, false, gid, 12)
	if err != nil {
		t.Fatal(err)
	}
	tot := 0
	for _, n := range contours {
		if n <= 0 {
			t.Fatalf("contour size %d", n)
		}
		tot += n
	}
	if tot != len(pts) {
		t.Fatalf("contours sum %d != pts %d", tot, len(pts))
	}
	if strings.ContainsRune(strings.Join(nil, ""), 0) {
	}
}

// TestLightHintCFF2：公开入口 CFF2 分支对照 ftexp light
//（SourceSans3 VF 默认实例）。
func TestLightHintCFF2(t *testing.T) {
	path := "../testdata/source-sans/VF/SourceSans3VF-Upright.otf"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("no CFF2 VF: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sample := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	for _, px := range []float64{12, 14} {
		ft := batchContour26LightFont(t, path, sample, px)
		for _, r := range sample {
			ftPts, ok := ft[r]
			if !ok {
				continue
			}
			gid := uint16(openTestFont(t, path).GlyphIndex(r))
			pts, _, _, err := LightHint(raw, 0, true, gid, px)
			if err != nil {
				t.Fatalf("%c @%.0fpx: %v", r, px, err)
			}
			if len(pts) != len(ftPts) {
				t.Errorf("%c @%.0fpx: LightHint %d pts vs FT %d", r, px, len(pts), len(ftPts))
				continue
			}
			bad := 0
			for i, p := range pts {
				if p.X != ftPts[i][0] || p.Y != ftPts[i][1] {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%c @%.0fpx: %d/%d pts mismatch", r, px, bad, len(pts))
			}
		}
	}
}
