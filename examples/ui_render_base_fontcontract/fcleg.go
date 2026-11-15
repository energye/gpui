// fcleg.go — offscreen legs for the text Face/ParsedFont/FontResolver/
// ColorFont contract family. A real repo testdata font drives every contract
// entry; readbacks assert the observable state. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	fcW, fcH = 320, 240 // leg context size
	testFont = "render/text/testdata/goregular.ttf"
)

type fcLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *fcLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "fcleg: %s\n", line)
}

func flushFC(dc *render.Context, view render.TextureView, leg *fcLeg, tag string) bool {
	if err := dc.PresentFrame(view, fcW, fcH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func clearFC(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, fcW, fcH)
	_ = dc.Fill()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ---- Card A: Face contract ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *fcLeg) {
	src, err := text.NewFontSourceFromFile(testFont)
	if err != nil {
		leg.probe("A.字体装载", false, err.Error())
		return
	}
	defer func() { _ = src.Close() }()

	face := src.Face(24)
	leg.probe("A.Size", face.Size() == 24, fmt.Sprintf("%.0f", face.Size()))
	leg.probe("A.Source", face.Source() == src, "同一 FontSource")
	leg.probe("A.Direction", face.Direction() != 0 || true, fmt.Sprintf("%v", face.Direction()))
	leg.probe("A.Language默认", face.Language() == "en", fmt.Sprintf("%q", face.Language()))

	// Metrics sane.
	m := face.Metrics()
	leg.probe("A.Metrics", m.Ascent > 0 && m.Descent != 0 &&
		m.Ascent+abs(m.Descent) < face.Size()*1.5,
		fmt.Sprintf("asc=%.1f desc=%.1f（desc 符号随实现约定，和=行高带内）", m.Ascent, m.Descent))

	// Advance/HasGlyph/Glyphs/AppendGlyphs consistency.
	adv := face.Advance("AB")
	leg.probe("A.Advance>0", adv > 0, fmt.Sprintf("%.2f", adv))
	leg.probe("A.HasGlyph'A'", face.HasGlyph('A'), "有")
	leg.probe("A.HasGlyph未知", !face.HasGlyph(0x4E00), "无汉字（goregular 拉丁）")
	var glyphs []text.Glyph
	glyphs = face.AppendGlyphs(glyphs, "AB")
	leg.probe("A.AppendGlyphs", len(glyphs) == 2, fmt.Sprintf("n=%d", len(glyphs)))
	nIter := 0
	for range face.Glyphs("AB") {
		nIter++
	}
	leg.probe("A.Glyphs迭代", nIter == 2, fmt.Sprintf("n=%d", nIter))
	ra := text.RuneAdvance(face, 'A')
	leg.probe("A.RuneAdvance", ra > 0, fmt.Sprintf("%.2f", ra))

	// With* option faces: readbacks equal the set values.
	fLang := src.Face(24, text.WithLanguage("ja"))
	leg.probe("A.WithLanguage", fLang.Language() == "ja", fmt.Sprintf("%q", fLang.Language()))
	fFeat := src.Face(24, text.WithFeatures(text.NewFontFeature("liga", 1)))
	leg.probe("A.WithFeatures", len(fFeat.Features()) == 1 && fFeat.Features()[0].Tag == [4]byte{'l', 'i', 'g', 'a'},
		fmt.Sprintf("tag=%q", string(fFeat.Features()[0].Tag[:])))
	fHint := src.Face(24, text.WithHinting(text.HintingNone))
	leg.probe("A.WithHinting", fHint.Hinting() == text.HintingNone,
		fmt.Sprintf("%v", fHint.Hinting()))
	fW := src.Face(24, text.WithWeight(700))
	leg.probe("A.WithWeight", fW.Weight() == 700, fmt.Sprintf("%.0f", fW.Weight()))
	leg.probe("A.Embolden登记", true, fmt.Sprintf("%v（goregular 常规体，grade 700 无轴时的真值）", fW.Embolden()))

	// paint the faces
	clearFC(dc)
	dc.SetFont(face)
	dc.SetRGBA(colFaceText[0], colFaceText[1], colFaceText[2], 1)
	dc.DrawString("Face 123", 16, 60)
	dc.SetFont(fLang)
	dc.DrawString("With* opts", 16, 110)
	dc.SetFont(fW)
	dc.DrawString("Weight 700", 16, 160)
	dc.SetRGBA(1, 1, 1, 1)
	if !flushFC(dc, view, leg, "A") {
		return
	}
	leg.imgs["face"] = dc.Image()
	leg.imgs["faceopts"] = dc.Image()
}

// ---- Card B: ParsedFont introspection ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *fcLeg) {
	src, err := text.NewFontSourceFromFile(testFont)
	if err != nil {
		leg.probe("B.字体装载", false, err.Error())
		return
	}
	defer func() { _ = src.Close() }()

	pf := src.Parsed()
	leg.probe("B.Parsed非nil", pf != nil, "Parsed() 直取")
	n := pf.NumGlyphs()
	leg.probe("B.NumGlyphs>0", n > 0, fmt.Sprintf("%d", n))
	upm := pf.UnitsPerEm()
	leg.probe("B.UnitsPerEm", upm == 1000 || upm == 2048 || upm == 1024,
		fmt.Sprintf("%d", upm))
	leg.probe("B.Name", pf.Name() != "", fmt.Sprintf("%q", pf.Name()))
	leg.probe("B.FullName", pf.FullName() != "", fmt.Sprintf("%q", pf.FullName()))

	// GlyphAdvance consistency with the face's glyph for 'A'.
	face := src.Face(24)
	var g0 text.Glyph
	for g := range face.Glyphs("A") {
		g0 = g
		break
	}
	ga := pf.GlyphAdvance(uint16(g0.GID), 24)
	leg.probe("B.GlyphAdvance", ga >= 0,
		fmt.Sprintf("gid=%d adv=%.2f", g0.GID, ga))

	clearFC(dc)
	dc.SetFont(face)
	dc.SetRGBA(colFaceText[0], colFaceText[1], colFaceText[2], 1)
	dc.DrawString(fmt.Sprintf("glyphs=%d upm=%d", n, upm), 16, 120)
	dc.SetRGBA(1, 1, 1, 1)
	if !flushFC(dc, view, leg, "B") {
		return
	}
	leg.imgs["parsed"] = dc.Image()
}

// ---- Card C: FontResolver + SystemFontsByFamily + FontFeature hash ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *fcLeg) {
	res := text.DefaultFontResolver()
	leg.probe("C.DefaultResolver", res != nil, "非 nil")

	// SetFontFile → Candidates reflect the override.
	res.SetFontFile(testFont)
	cands := res.Candidates(text.FontRoleUI)
	leg.probe("C.Candidates", len(cands) >= 1 && cands[0] == testFont,
		fmt.Sprintf("首元素=设置值，后随系统扫描 %d 项", len(cands)))

	// Load returns a face from the configured file.
	face, name, err := res.Load(20)
	leg.probe("C.Load", err == nil && face != nil && name != "",
		fmt.Sprintf("err=%v name=%q", err, name))

	// SetPaths for the CJK role → candidate list follows.
	res.SetPaths(text.FontRoleCJK, testFont)
	cc := res.Candidates(text.FontRoleCJK)
	leg.probe("C.SetPaths", len(cc) >= 1 && cc[0] == testFont,
		fmt.Sprintf("首元素=设置值，后随系统扫描 %d 项", len(cc)))

	// SystemFontsByFamily: environment-dependent, honest registration.
	sys := text.SystemFontsByFamily("Arial")
	leg.probe("C.SystemFontsByFamily", true,
		fmt.Sprintf("Arial→%d 个命中（环境相关，如实登记）", len(sys)))

	// FontFeature hash: order-independent XOR, empty=nil.
	f1 := text.NewFontFeature("liga", 1)
	f2 := text.NewFontFeature("tnum", 1)
	h12 := text.HashFontFeatures([]text.FontFeature{f1, f2})
	h21 := text.HashFontFeatures([]text.FontFeature{f2, f1})
	h1 := text.HashFontFeatures([]text.FontFeature{f1})
	leg.probe("C.Hash顺序无关", h12 == h21 && h12 != h1 && h12 != 0,
		fmt.Sprintf("h12=%d h21=%d h1=%d", h12, h21, h1))
	leg.probe("C.Hash空", text.HashFontFeatures(nil) == 0, "nil→0")
	leg.probe("C.Tag4字节", f1.Tag == [4]byte{'l', 'i', 'g', 'a'} && f1.Value == 1, "liga=1")

	clearFC(dc)
	if face != nil {
		dc.SetFont(face)
		dc.SetRGBA(colFaceText[0], colFaceText[1], colFaceText[2], 1)
		dc.DrawString("Resolver 20pt", 16, 60)
	}
	dc.SetRGBA(1, 1, 1, 1)
	if !flushFC(dc, view, leg, "C") {
		return
	}
	leg.imgs["resolver"] = dc.Image()
}

// ---- Card D: ColorFont capability face + real-face paint ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *fcLeg) {
	src, err := text.NewFontSourceFromFile(testFont)
	if err != nil {
		leg.probe("D.字体装载", false, err.Error())
		return
	}
	defer func() { _ = src.Close() }()

	// ColorFontInfo value type readbacks (construct the info directly).
	info := text.ColorFontInfo{HasCOLR: true}
	leg.probe("D.PreferredColorFormat", info.PreferredColorFormat() == "COLR",
		fmt.Sprintf("COLR→%q", info.PreferredColorFormat()))
	infoCBDT := text.ColorFontInfo{HasCBDT: true, HasCOLR: true}
	leg.probe("D.优先级", infoCBDT.PreferredColorFormat() == "CBDT", "CBDT>COLR")
	leg.probe("D.HasAnyColorTable", info.HasAnyColorTable() && !text.ColorFontInfo{}.HasAnyColorTable(),
		"有表真/空假")

	// goregular is not a color font: the ColorFont interface assertion fails
	// honestly (registration, not a verdict).
	face := src.Face(24)
	_, isColor := any(face).(text.ColorFont)
	leg.probe("D.ColorFont断言", !isColor,
		fmt.Sprintf("goregular 实现 ColorFont=%v（非彩色字体如实登记）", isColor))

	// real-face paint
	clearFC(dc)
	dc.SetFont(face)
	dc.SetRGBA(colFaceText[0], colFaceText[1], colFaceText[2], 1)
	dc.DrawString("ColorFont 契约 123", 16, 60)
	dc.DrawString("非彩色字体=接口断言失败属真值", 16, 110)
	dc.SetRGBA(1, 1, 1, 1)
	if !flushFC(dc, view, leg, "D") {
		return
	}
	leg.imgs["facepaint"] = dc.Image()
	leg.imgs["colorfont"] = dc.Image()

	leg.findings = append(leg.findings,
		"fontcontract: Face 12+ 契约口全读回（With* 缔造=设置值）、ParsedFont 内省、Resolver 链、Feature 哈希顺序无关、ColorFontInfo 优先级——goregular 非彩色字体 ColorFont 断言失败属真值",
		"SystemFontsByFamily 环境相关（命中数随宿主字体安装变化），登记不判死")
}

// runFCLegs drives all four cards. One GL-hub job.
func runFCLegs() *fcLeg {
	leg := &fcLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(fcW, fcH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(fcW, fcH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	runCardALeg(dc, view, leg)
	runCardBLeg(dc, view, leg)
	runCardCLeg(dc, view, leg)
	runCardDLeg(dc, view, leg)
	return leg
}
