package hint

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// M1 验证（docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §5.1 M1）：
// Type 2 charstring 解释器（cffcs.go）产出精确 cs hstem 对 + cs 轮廓，
// 消除从 26.6 像素反推的 ±1.3FU 误差（§13.4）。

// ftMulFix 复刻 freetype-2.14.3 ftcalc.h 内联 FT_MulFix（FT_INT64 版）：
// ab = a*b; ab += 0x8000 + (ab >> 63); return ab >> 16（C 算术右移，负向下取整）。
func ftMulFix(a, b int64) int64 {
	ab := a * b
	ab += 0x8000
	if ab < 0 {
		ab--
	}
	return ab >> 16
}

// ftDivFix 复刻 FT_DivFix（ftcalc.h）：q = (|a|<<16 + |b|/2) / |b|。
func ftDivFix(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	q := (a<<16 + b/2) / b
	return q
}

// ftScale 返回 FT size->metrics.x_scale（16.16）：FT_DivFix(charWidth26_6, upem)。
func ftScale(px float64, upem int64) int64 {
	cw := int64(px*64 + 0.5)
	return ftDivFix(cw, upem)
}

// csTo26_6 将解释器 cs（字体单位）转为 FT 26.6 定点：MulFix(cs, x_scale)。
// cf2 unhinted 以 unity 渲染（26.6 = cs），cff_slot_load 后乘 x_scale（cffgload.c:707-713）。
func csTo26_6(cs float64, scale int64) int64 {
	return ftMulFix(int64(math.Round(cs)), scale)
}

// m1Font 加载 CJK TTC（face 0）并解析 CFF。
func m1Font(t *testing.T) (*testFont, *cffFontData, []byte) {
	t.Helper()
	f := openTestFont(t, "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	raw := f.raw
	start, ln, err := cffTableData(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	cd, err := cffParseAll(raw[start:start+ln], f.unitsPerEm)
	if err != nil {
		t.Fatal(err)
	}
	return f, cd, raw
}

// m1Interp 解释 gid 的 charstring（FD Select + subrs + nominalWidthX）。
func m1Interp(cd *cffFontData, gid uint16) (*csOutline, int, error) {
	fdIdx := 0
	if cd.fdSelect != nil {
		var err error
		fdIdx, err = cd.fdSelect(gid)
		if err != nil {
			return nil, 0, err
		}
	}
	fd := cd.fds[fdIdx]
	out, err := interpretCharstring(cd.charStrings[gid], fd.subrs, cd.globalSubrs, fd.blues.nominalWidthX)
	return out, fdIdx, err
}

// ftContour26 调 ftexp 取 FT-nohint 轮廓 26.6 定点（face 0）。
// 返回逐点 (x26, y26, on)，point 顺序 = FT 解释顺序。
func ftContour26(t *testing.T, r rune, px float64) [][3]int64 {
	t.Helper()
	cmd := exec.Command(ftexpBin(t), "contour",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", string(r), strconv.Itoa(int(px)), "n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("ftexp unavailable: %v %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		t.Fatalf("short ftexp contour output: %s", out)
	}
	var np int
	if _, err := fmt.Sscanf(lines[0], "%d", &np); err != nil {
		t.Fatalf("bad ftexp header: %v", err)
	}
	pts := make([][3]int64, 0, np)
	for i := 0; i < np && i+1 < len(lines); i++ {
		var x, y, tag int64
		if _, err := fmt.Sscanf(lines[1+i], "%d %d %d", &x, &y, &tag); err != nil {
			t.Fatalf("bad ftexp point %d: %v", i, err)
		}
		pts = append(pts, [3]int64{x, y, tag})
	}
	if len(pts) != np {
		t.Fatalf("ftexp points %d != header %d", len(pts), np)
	}
	return pts
}

// TestM1HStemsTRACE：日 12px hstem/vstem 对 = TRACE 数值一致
// （§13.5 对照线：-4/71/352/426/697/772；vstem 176/77/499/80）。
func TestM1HStemsTRACE(t *testing.T) {
	f, cd, _ := m1Font(t)
	gid := uint16(f.GlyphIndex('日'))
	out, fdIdx, err := m1Interp(cd, gid)
	if err != nil {
		t.Fatal(err)
	}
	if fdIdx != 12 {
		t.Fatalf("日 gid=%d fd=%d, want 12", gid, fdIdx)
	}
	wantH := []csStem{{-4, 71}, {352, 426}, {697, 772}}
	if len(out.hstems) != len(wantH) {
		t.Fatalf("hstems %v, want %v", out.hstems, wantH)
	}
	for i, w := range wantH {
		if out.hstems[i] != w {
			t.Errorf("hstem[%d] = (%.1f,%.1f), want (%.1f,%.1f)",
				i, out.hstems[i].lo, out.hstems[i].hi, w.lo, w.hi)
		}
	}
	wantV := []csStem{{176, 253}, {752, 832}}
	if len(out.vstems) != len(wantV) {
		t.Fatalf("vstems %v, want %v", out.vstems, wantV)
	}
	for i, w := range wantV {
		if out.vstems[i] != w {
			t.Errorf("vstem[%d] = (%.1f,%.1f), want (%.1f,%.1f)",
				i, out.vstems[i].lo, out.vstems[i].hi, w.lo, w.hi)
		}
	}
	// hintmask 位序：hstem 先、vstem 后（pshints.c:870 "hStem hints first"）。
	if len(out.hints) != len(out.hstems)+len(out.vstems) {
		t.Errorf("hints 位序长度 %d, want %d", len(out.hints), len(out.hstems)+len(out.vstems))
	}
	for i, s := range out.hints {
		_ = s
		if i < len(out.hstems) {
			if out.hints[i] != out.hstems[i] {
				t.Errorf("hints[%d] != hstems[%d]", i, i)
			}
		}
	}
}

// TestM1Outline26_6：日/田/目 12px——解释器 cs 轮廓换算 26.6 与 FT-nohint 逐点相等
// （±0 误差；FT 转换链 = MulFix(cs, FT_DivFix(768,upem))，见 csTo26_6）。
func TestM1Outline26_6(t *testing.T) {
	f, cd, _ := m1Font(t)
	upem := int64(f.unitsPerEm)
	px := 12.0
	scale := ftScale(px, upem)
	if scale != 50332 {
		t.Fatalf("scale = %d, want 50332 (FT_DivFix(768,1000))", scale)
	}
	for _, r := range []rune{'日', '田', '目'} {
		gid := uint16(f.GlyphIndex(r))
		out, _, err := m1Interp(cd, gid)
		if err != nil {
			t.Fatal(err)
		}
		ft := ftContour26(t, r, px)
		if len(ft) != len(out.pts) {
			t.Fatalf("%c: self %d pts vs FT %d", r, len(out.pts), len(ft))
		}
		bad := 0
		for i := range out.pts {
			p := out.pts[i]
			want := ft[i]
			gotX := csTo26_6(p.x, scale)
			gotY := csTo26_6(p.y, scale)
			gotOn := int64(0)
			if p.on {
				gotOn = 1
			}
			if gotX != want[0] || gotY != want[1] || gotOn != want[2]&1 {
				bad++
				if bad <= 8 {
					t.Errorf("%c pt%d cs=(%.0f,%.0f) on=%v → 26.6=(%d,%d) FT=(%d,%d) tag=%d",
						r, i, p.x, p.y, p.on, gotX, gotY, want[0], want[1], want[2])
				}
			}
		}
		if bad > 0 {
			t.Errorf("%c: %d/%d 点不一致", r, bad, len(ft))
		} else {
			t.Logf("%c 12px: %d 点 26.6 全部与 FT-nohint 逐点一致", r, len(ft))
		}
	}
}

// TestM1Width：日 advance = defaultWidthX(1000 FU) × scale → 12px（±1/64）。
func TestM1Width(t *testing.T) {
	f, cd, _ := m1Font(t)
	gid := uint16(f.GlyphIndex('日'))
	out, fdIdx, err := m1Interp(cd, gid)
	if err != nil {
		t.Fatal(err)
	}
	fd := cd.fds[fdIdx]
	// 日 charstring 无 width 参数（解释器 hasWidth=false）→ FT 用 defaultWidthX。
	if out.hasWidth {
		t.Logf("日 hasWidth=true width=%.1f（charstring 带 width）", out.width)
	}
	want := fd.blues.defaultWidthX
	if want == 0 {
		t.Skip("fd defaultWidthX = 0，无法验证宽度")
	}
	gotPx := out.width * 12.0 / float64(f.unitsPerEm)
	if !out.hasWidth {
		gotPx = want * 12.0 / float64(f.unitsPerEm)
	}
	if d := gotPx - 12.0; d < -0.02 || d > 0.02 {
		t.Errorf("advance = %.3fpx, want 12.0 (±1/64)", gotPx)
	}
	t.Logf("日 advance = %.3fpx (defaultWidthX=%.0f FU)", gotPx, want)
}

// TestM1TianMuStems：田/目 stem 数（无 TRACE，验证结构合理性）。
func TestM1TianMuStems(t *testing.T) {
	f, cd, _ := m1Font(t)
	want := map[rune]int{'田': 3, '目': 4} // hstem 数量
	for r, n := range want {
		out, _, err := m1Interp(cd, uint16(f.GlyphIndex(r)))
		if err != nil {
			t.Fatal(err)
		}
		if len(out.hstems) != n {
			t.Errorf("%c hstems = %d, want %d", r, len(out.hstems), n)
		}
		if len(out.hints) != len(out.hstems)+len(out.vstems) {
			t.Errorf("%c hints 长度 %d != %d+%d", r, len(out.hints), len(out.hstems), len(out.vstems))
		}
	}
}

var _ = os.Getenv
