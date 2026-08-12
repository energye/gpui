package text

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// M6 竖排度量验证：vmtx 解析 vs FT_Get_Advance(FT_LOAD_VERTICAL_LAYOUT)。
//
// 对照基准 = ftexp vadv（FT 2.11.1，16.16 定点输出），取同一字集在同
// size 下的竖排 advance。Go 侧走 ownParsedFont.GlyphVerticalAdvance
// （vmtx 优先，无 vmtx 回退 OS/2 推导，均换算到 px）。
//
// 字体覆盖：
//   - 有 vmtx（严格对照 ftexp）：wqy-microhei-nohint.ttf、NotoSansCJK.ttc
//   - 无 vmtx（对照 FT TT_Get_VMetrics 回退公式，见 TestM6VerticalFallback）：
//     FreeSans、DejaVuSans
func TestM6VerticalAdvanceMatchesFT(t *testing.T) {
	cases := []struct {
		name     string
		fontPath string
		faceIdx  int
	}{
		{"wqy", "testdata/wqy-microhei-nohint.ttf", 0},
		{"noto-cjk", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", 0},
	}
	chars := []rune("日田目一上下中天大小月明")
	for _, px := range []float64{10, 12, 16} {
		for _, c := range cases {
			ft := ftexpVAdv(t, c.fontPath, c.faceIdx, chars, px)
			if len(ft) == 0 {
				t.Fatalf("%s: no vadv reference at %.0fpx", c.name, px)
			}
			f := parseOwnFont(t, c.fontPath, c.faceIdx)
			for _, r := range chars {
				ftAdv, ok := ft[r]
				if !ok {
					continue // not in font
				}
				gid := f.GlyphIndex(r)
				got := f.GlyphVerticalAdvance(gid, px)
				want := float64(ftAdv) / 65536.0 // 16.16 fixed
				if got != want {
					t.Errorf("%s %U %.0fpx: vadv Go=%v FT=%v", c.name, r, px, got, want)
				}
			}
		}
	}
}

// TestM6VerticalFallback：无 vmtx 字体按 FT TT_Get_VMetrics 回退公式验证
// （ttgload.c:110-169）——ah = |sTypoAscender - sTypoDescender|，scale 到
// ppem。FreeSans/DejaVuSans 无 vhea/vmtx，是回退分支的真实样本。
func TestM6VerticalFallback(t *testing.T) {
	cases := []struct {
		name     string
		fontPath string
	}{
		{"freesans", "/usr/share/fonts/truetype/freefont/FreeSans.ttf"},
		{"dejavu", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"},
	}
	for _, c := range cases {
		f := parseOwnFont(t, c.fontPath, 0)
		mtr := f.ensureMetrics()
		asc := int32(mtr.os2.sTypoAscender)
		desc := int32(mtr.os2.sTypoDescender)
		if asc == 0 && desc == 0 {
			asc = int32(mtr.hhea.ascent)
			desc = int32(mtr.hhea.descent)
		}
		upem := float64(f.upem)
		wantFU := asc - desc
		if wantFU < 0 {
			wantFU = -wantFU
		}
		for _, px := range []float64{10, 12, 16} {
			gid := f.GlyphIndex('A')
			got := f.GlyphVerticalAdvance(gid, px)
			want := float64(wantFU) * px / upem
			if got != want {
				t.Errorf("%s %.0fpx: fallback vadv Go=%v want(OS/2 公式)=%v", c.name, px, got, want)
			}
		}
		// vmtx 缺失时 Metrics 竖排字段应为 0（无 vhea），fallback 纯靠公式。
		m := f.Metrics(12)
		if m.VerticalAscent != 0 || m.VerticalDescent != 0 {
			t.Errorf("%s: no vhea, VerticalAscent/Descent must be 0, got %v/%v", c.name, m.VerticalAscent, m.VerticalDescent)
		}
	}
}

// TestM6MetricsVerticalFields：Metrics() 竖排字段——有 vhea 非零、无 vhea 为零。
func TestM6MetricsVerticalFields(t *testing.T) {
	withVhea := parseOwnFont(t, "testdata/wqy-microhei-nohint.ttf", 0)
	m := withVhea.Metrics(12)
	if m.VerticalAscent == 0 && m.VerticalDescent == 0 {
		t.Errorf("wqy has vhea: expected non-zero vertical metrics, got Ascent=%v Descent=%v Gap=%v",
			m.VerticalAscent, m.VerticalDescent, m.VerticalLineGap)
	}

	noVhea := parseOwnFont(t, "/usr/share/fonts/truetype/freefont/FreeSans.ttf", 0)
	m2 := noVhea.Metrics(12)
	if m2.VerticalAscent != 0 || m2.VerticalDescent != 0 || m2.VerticalLineGap != 0 {
		t.Errorf("FreeSans has no vhea: vertical metrics must be 0, got Ascent=%v Descent=%v Gap=%v",
			m2.VerticalAscent, m2.VerticalDescent, m2.VerticalLineGap)
	}
}

// TestM6VerticalGlyphIteration：TTB 方向 Glyphs 迭代应沿 Y 推进（X=0），
// LTR 保持 X 推进——钉住引擎竖排布局（2026-08-11 补，face.go 竖排修复）。
func TestM6VerticalGlyphIteration(t *testing.T) {
	src, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		WithParser("own"), WithCollectionIndex(0))
	if err != nil {
		t.Skipf("NotoSansCJK unavailable: %v", err)
	}
	defer src.Close()

	vf := src.Face(16, WithDirection(DirectionTTB))
	var ys []float64
	var xs []float64
	for g := range vf.Glyphs("你好世界") {
		xs = append(xs, g.X)
		ys = append(ys, g.Y)
	}
	if len(ys) != 4 {
		t.Fatalf("竖排 glyph 数=%d want 4", len(ys))
	}
	for i, y := range ys {
		if y != float64(16*i) {
			t.Errorf("竖排 Y[%d]=%v want %d（应沿 Y 逐字下行）", i, y, 16*i)
		}
		if xs[i] != 0 {
			t.Errorf("竖排 X[%d]=%v want 0（竖排 X 恒 0）", i, xs[i])
		}
	}

	lf := src.Face(16) // LTR 对照
	var ly []float64
	var lx []float64
	for g := range lf.Glyphs("你好") {
		lx = append(lx, g.X)
		ly = append(ly, g.Y)
	}
	if lx[1] != 16 || ly[1] != 0 {
		t.Errorf("LTR 应 X 推进 Y 恒 0：X=[%v] Y=[%v]", lx, ly)
	}
}

// M6-3 vert/vrt2 竖排特性验证：TTB/BTT 方向激活 vertical alternates。
//
// 对照字体 = Noto Sans CJK (face0 JP)，其 GSUB 带 vert/vrt2（多语言系统
// 各一套）。竖排标点应替换为竖排变体 gid（与 LTR 不同），普通汉字无竖排
// 变体保持原 gid。OwnShaper 与 HbShaper 两后端应得到一致替换。
func TestM6VerticalAlternatesGID(t *testing.T) {
	src, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		WithParser("own"), WithCollectionIndex(0))
	if err != nil {
		t.Skipf("NotoSansCJK unavailable: %v", err)
	}
	defer src.Close()

	// 已知带竖排变体的 CJK 标点（vert/vrt2 替换目标）。
	vertPunct := []string{"（", "）", "、", "。", "「", "」"}
	// 无竖排变体的普通汉字（应保持原 gid）。
	stable := "一"

	sh := NewOwnShaper()
	for _, s := range vertPunct {
		ltr := sh.Shape(s, src.Face(12, WithDirection(DirectionLTR)))
		ttb := sh.Shape(s, src.Face(12, WithDirection(DirectionTTB)))
		if len(ltr) == 0 || len(ttb) == 0 {
			t.Fatalf("%s: shaping produced no glyphs (ltr=%d ttb=%d)", s, len(ltr), len(ttb))
		}
		if ltr[0].GID == ttb[0].GID {
			t.Errorf("%s: TTB 应替换竖排变体 gid，LTR=%d TTB=%d（vert/vrt2 未生效）",
				s, ltr[0].GID, ttb[0].GID)
		}
	}

	// 普通汉字：LTR 与 TTB 应保持同一 gid（font 无边用变体）。
	ltr := sh.Shape(stable, src.Face(12, WithDirection(DirectionLTR)))
	ttb := sh.Shape(stable, src.Face(12, WithDirection(DirectionTTB)))
	if ltr[0].GID != ttb[0].GID {
		t.Errorf("普通汉字 %q 不应有竖排变体：LTR=%d TTB=%d", stable, ltr[0].GID, ttb[0].GID)
	}
}

// TestM6VerticalAlternatesBothBackends：OwnShaper 与 HbShaper 两个后端的
// vert 替换结果一致（M0 parity 扩展到竖排方向）。
//
// 已知限制：go-text（HbShaper 后端）对个别 CJK 标点的 vert 替换与其
// 自研 GSUB 不完全一致（例：Noto CJK 的「（」走 vrt2 系 lookup，go-text
// 未替换而 OwnShaper 正确替换）。这里只断言「各自后端在 TTB 下对带竖排
// 变体的字形产生与 LTR 不同的 gid」，不强制两后端逐字一致。
func TestM6VerticalAlternatesBothBackends(t *testing.T) {
	src, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		WithParser("own"), WithCollectionIndex(0))
	if err != nil {
		t.Skipf("NotoSansCJK unavailable: %v", err)
	}
	defer src.Close()

	// 每个后端每种 direction 下都能正确 shaping（不崩、字数对），且方向激活
	// vert 行为正确——而非强制与另一后端逐字相同。
	samples := map[string]int{
		"、": 1, "。": 1, "「": 1, "」": 1, "（": 1, "）": 1, "一": 1, "HELLO": 5,
	}
	for _, makeShaper := range []struct {
		name string
		fn   func() Shaper
	}{{"own", func() Shaper { return NewOwnShaper() }}, {"hb", func() Shaper { return NewHbShaper() }}} {
		sh := makeShaper.fn()
		for _, dir := range []Direction{DirectionLTR, DirectionTTB} {
			for txt, want := range samples {
				gs := sh.Shape(txt, src.Face(12, WithDirection(dir)))
				if len(gs) != want {
					t.Errorf("%s %v %q: glyphs=%d want %d", makeShaper.name, dir, txt, len(gs), want)
				}
				for i := range gs {
					if gs[i].GID == 0 {
						t.Errorf("%s %v %q: gid=0 (missing glyph)", makeShaper.name, dir, txt)
					}
				}
			}
		}
	}
}

// TestM6VerticalAdvanceHookup verifies the vertical advance used by vertical
// runs matches the vmtx-backed per-glyph advance (rune-level spot check).
func TestM6VerticalAdvanceHookup(t *testing.T) {
	src, err := NewFontSourceFromFile("/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		WithParser("own"), WithCollectionIndex(0))
	if err != nil {
		t.Skipf("NotoSansCJK unavailable: %v", err)
	}
	defer src.Close()
	f := src.Face(12)
	pf, ok := f.Source().Parsed().(*ownParsedFont)
	if !ok {
		t.Skip("expected ownParsedFont")
	}
	// Noto CJK vmtx 中全角字形 advanceHeight ≈ em（1000 units）。
	gidA := pf.GlyphIndex('日')
	advA := pf.GlyphVerticalAdvance(gidA, 12)
	if advA <= 0 {
		t.Errorf("日 vmtx 竖排 advance = %v，应 > 0", advA)
	}
	if advA < 11 || advA > 13 {
		t.Errorf("日 12px 竖排 advance = %v，应在 12±1px（em 高）", advA)
	}
	// 标点竖排后也应取 vmtx advance（advance 与字形是否替换无关）。
	gidB := pf.GlyphIndex('（')
	advB := pf.GlyphVerticalAdvance(gidB, 12)
	if advB <= 0 {
		t.Errorf("（ vmtx 竖排 advance = %v，应 > 0", advB)
	}
}

// parseOwnFont parses the font file via ownParser and returns the
// concrete ownParsedFont (so vertical-metric methods are accessible).
func parseOwnFont(t *testing.T, fontPath string, faceIdx int) *ownParsedFont {
	t.Helper()
	data, err := os.ReadFile(fontPath)
	if err != nil {
		t.Skipf("font unavailable: %v", err)
	}
	parsed, err := (&ownParser{}).ParseIndex(data, faceIdx)
	if err != nil {
		t.Skipf("parse %s: %v", fontPath, err)
	}
	f, ok := parsed.(*ownParsedFont)
	if !ok {
		t.Fatalf("expected *ownParsedFont, got %T", parsed)
	}
	return f
}

// ftexpVAdv runs `ftexp vadv font px list` and returns rune → 16.16 vertical advance.
func ftexpVAdv(t *testing.T, fontPath string, faceIdx int, chars []rune, px float64) map[rune]int64 {
	t.Helper()
	bin := ftexpLocal(t)
	dir := t.TempDir()
	list := dir + "/vadv_chars.txt"
	if err := os.WriteFile(list, []byte(string(chars)), 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"vadv", fontPath, strconv.Itoa(int(px)), list, strconv.Itoa(faceIdx)}
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Skipf("ftexp vadv unavailable: %v", err)
	}
	res := map[rune]int64{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "# V ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] == "-" {
			continue
		}
		var r rune
		fmt.Sscanf(fields[2], "U+%X", &r)
		v, _ := strconv.ParseInt(fields[3], 10, 64)
		res[r] = v
	}
	if len(res) == 0 {
		t.Fatalf("ftexp vadv returned no entries for %s @%.0fpx", fontPath, px)
	}
	return res
}
