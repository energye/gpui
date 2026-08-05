package hint

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
)

// M2 验证（docs/ENGINE_TEXT_HINT_LIGHT_PLAN.md §5.1 M2）：
// cf2Blues + cf2HintMap 移植，对照线 = §13.5 日 12px 8 边数值 + ftexp light
// 逐点 26.6。

func m2Font(t *testing.T) (text.ParsedFont, *cffFontData) {
	t.Helper()
	src, err := text.NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	if err != nil {
		t.Skipf("CJK font unavailable: %v", err)
	}
	f := src.Face(14).Source().Parsed()
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
	return f, cd
}

func m2Interp(cd *cffFontData, gid uint16) (*csOutline, *cffFD, error) {
	fdIdx := 0
	if cd.fdSelect != nil {
		var err error
		fdIdx, err = cd.fdSelect(gid)
		if err != nil {
			return nil, nil, err
		}
	}
	fd := cd.fds[fdIdx]
	out, err := interpretCharstring(cd.charStrings[gid], fd.subrs, cd.globalSubrs, fd.blues.nominalWidthX)
	return out, &fd, err
}

// m2HintScale：hinted 16.16 scale = (x_scale + 32) / 64（psft.c:279）。
func m2HintScale(px float64, upem int) cf2Fixed {
	xScale := ftScale(px, int64(upem))
	return cf2Fixed((int64(xScale) + 32) / 64)
}

// TestM2Ri12pxEightEdges：日 12px 逐区图 8 边 ds 对照 §13.5 表。
// 幽灵区边锁定：-120→-1.5、880→11.5；hstem 边（adjustHints 后）：
// -4→0.0、71→0.8995、352→5.0、426→5.888、697→9.1、772→10.0。
func TestM2Ri12pxEightEdges(t *testing.T) {
	f, cd := m2Font(t)
	gid := uint16(f.GlyphIndex('日'))
	out, fd, err := m2Interp(cd, gid)
	if err != nil {
		t.Fatal(err)
	}
	upem := f.UnitsPerEm()
	scale := m2HintScale(12.0, upem)
	if scale != 786 {
		t.Fatalf("hinted scale = %d, want 786", scale)
	}

	// fd12 蓝区：确认 emBox 启发式路径（dummy BlueValues）
	t.Logf("fd Blues: bv=%v lg=%d", fd.blues.blueValues, fd.blues.languageGroup)

	hStems := cf2StemSlice(out.hstems)
	var blues cf2Blues
	cf2BluesInit(&blues, fd.blues, scale, 12, true)
	if !blues.doEmBoxHints {
		t.Fatal("fd12 未走 emBox 幽灵区路径，对照线失效")
	}

	// 逐区图（全激活）——与 hintCFFLight 相同：未锁定边经 initial 图定位。
	var mask cf2HintMask
	mask.setAll(len(out.hstems) + len(out.vstems))
	initMap := &cf2HintMap{}
	initMap.build(&blues, hStems, cf2StemSlice(out.vstems), nil, scale, 0, true)
	hintMap := &cf2HintMap{initial: initMap}
	hintMap.build(&blues, hStems, cf2StemSlice(out.vstems), &mask, scale, 0, false)

	// §13.5 对照线：cs → ds（px，16.16 转 float）
	type wantEdge struct {
		cs int64
		ds float64 // px
	}
	want := []wantEdge{
		{-120, -1.5},
		{-4, 0.0},
		{71, 0.8995},
		{352, 5.0},
		{426, 5.888},
		{697, 9.1},
		{772, 10.0},
		{880, 11.5},
	}
	for _, w := range want {
		cs := cf2IntToFixed(w.cs)
		ds := hintMap.mapCS(cs)
		got := float64(ds) / 65536.0
		// §13.5 的数值是 TRACE 打印（两位小数）+反推，允许 0.002px 容差
		if d := math.Abs(got - w.ds); d > 0.002 {
			t.Errorf("cs %d → ds %.4fpx, want %.4fpx (edges=%d)",
				w.cs, got, w.ds, len(hintMap.edges))
		} else {
			t.Logf("cs %d → ds %.4fpx (want %.4fpx)", w.cs, got, w.ds)
		}
	}
}

// TestM2ContourLight：日/田/目 12px light 轮廓逐点 26.6 == ftexp contour l。
func TestM2ContourLight(t *testing.T) {
	f, cd := m2Font(t)
	upem := f.UnitsPerEm()
	px := 12.0
	scale := m2HintScale(px, upem)
	// cf2_computeDarkening 12px：darkenX = darkenY = 12（16.16，0.0002px）
	darkenX := cf2Fixed(12)
	darkenY := cf2Fixed(12)

	for _, r := range []rune{'日', '田', '目'} {
		gid := uint16(f.GlyphIndex(r))
		out, fd, err := m2Interp(cd, gid)
		if err != nil {
			t.Fatal(err)
		}
		res := hintCFFLight(out, fd, scale, darkenX, darkenY)

		ft := ftContour26Light(t, r, px)
		if len(ft) != len(res.pts) {
			t.Fatalf("%c: self %d pts vs FT %d", r, len(res.pts), len(ft))
		}
		bad := 0
		for i, p := range res.pts {
			want := ft[i]
			gotX := int64(p[0] >> 10)
			gotY := int64(p[1] >> 10)
			if gotX != want[0] || gotY != want[1] {
				bad++
				if bad <= 8 {
					t.Errorf("%c pt%d 26.6=(%d,%d) FT=(%d,%d)",
						r, i, gotX, gotY, want[0], want[1])
				}
			}
		}
		if bad > 0 {
			t.Errorf("%c: %d/%d 点不一致", r, bad, len(ft))
		} else {
			t.Logf("%c 12px: %d 点 26.6 全部与 FT-light 逐点一致", r, len(ft))
		}
	}
}

// ftContour26Light 调 ftexp 取 FT-light 轮廓 26.6（mode l）。
func ftContour26Light(t *testing.T, r rune, px float64) [][3]int64 {
	t.Helper()
	cmd := exec.Command("/tmp/opencode/ftexp/ftexp", "contour",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", string(r), strconv.Itoa(int(px)), "l")
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
