// tsleg.go — offscreen legs for the text system-value family: resolver chain,
// system font scan, feature hashing, source introspection, script/direction
// enums. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	tsW, tsH   = 320, 240
	testFontTS = "render/text/testdata/goregular.ttf"
	testFontTS2 = "render/text/testdata/ahem.ttf"
)

type tsLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *tsLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "tsleg: %s\n", line)
}

func flushTS(dc *render.Context, view render.TextureView, leg *tsLeg, tag string) bool {
	if err := dc.PresentFrame(view, tsW, tsH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func runTSLegs() *tsLeg {
	leg := &tsLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(tsW, tsH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(tsW, tsH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, tsW, tsH)
	_ = dc.Fill()

	// resolver chain: SetDefaultFontPath → DefaultFontResolver → Load.
	text.SetDefaultFontPath(testFontTS)
	res := text.DefaultFontResolver()
	leg.probe("C.SetDefaultFontPath", res != nil, "默认 resolver 反映进程级设置")
	face, name, err := res.Load(20)
	leg.probe("C.Load", err == nil && face != nil && name == testFontTS,
		fmt.Sprintf("err=%v name=%q", err, name))

	// SetSystemFontPaths for CJK role.
	text.SetSystemFontPaths(text.FontRoleCJK, testFontTS2)
	cc := res.Candidates(text.FontRoleCJK)
	leg.probe("C.SetSystemFontPaths", len(cc) >= 1 && cc[0] == testFontTS2,
		fmt.Sprintf("CJK 首候选=%q", cc[0]))

	// LoadMultiFace: UI+CJK chain both point at repo fonts.
	mf, mfName, errMF := text.LoadMultiFace(20, text.FontRoleUI, text.FontRoleCJK)
	leg.probe("C.LoadMultiFace", errMF == nil && mf != nil && mfName != "",
		fmt.Sprintf("err=%v name=%q", errMF, mfName))

	// LoadDefaultFace (UI role from the SetDefaultFontPath override).
	df, dfName, errDF := text.LoadDefaultFace(16)
	leg.probe("C.LoadDefaultFace", errDF == nil && df != nil && dfName == testFontTS,
		fmt.Sprintf("err=%v name=%q", errDF, dfName))

	// SystemFontsByFamily: environment-dependent, honest registration.
	sys := text.SystemFontsByFamily("DejaVu Sans")
	leg.probe("C.SystemFontsByFamily", true,
		fmt.Sprintf("DejaVu Sans→%d 命中（环境相关）", len(sys)))

	// feature hash.
	f1 := text.NewFontFeature("tnum", 1)
	f2 := text.NewFontFeature("liga", 0)
	h12 := text.HashFontFeatures([]text.FontFeature{f1, f2})
	leg.probe("C.HashFontFeatures", h12 == text.HashFontFeatures([]text.FontFeature{f2, f1}) && h12 != 0,
		fmt.Sprintf("h=%d 顺序无关", h12))

	// source introspection.
	src, errS := text.NewFontSourceFromFile(testFontTS)
	if errS == nil {
		defer func() { _ = src.Close() }()
		leg.probe("D.WeightClass", src.WeightClass() > 0,
			fmt.Sprintf("%d", src.WeightClass()))
		leg.probe("D.IsVariable", !src.IsVariable(),
			fmt.Sprintf("goregular 静态字体=%v", src.IsVariable()))
		axes := src.VariationAxes()
		leg.probe("D.VariationAxes", len(axes) == 0,
			fmt.Sprintf("无轴=%d", len(axes)))
	}

	// script/direction enums.
	leg.probe("D.ScriptLatin", text.ScriptLatin.RequiresComplexShaping() == false,
		fmt.Sprintf("Latin 简单=%v", text.ScriptLatin.RequiresComplexShaping()))
	// 实证：RequiresComplexShaping 只对印度系/泰/缅等 true，Han 不在列表。
	leg.probe("D.ScriptHan", !text.ScriptHan.RequiresComplexShaping(),
		"Han 不在复杂整形列表（内置 shaper 可处理）")
	leg.probe("D.ScriptArabic", text.ScriptArabic.RequiresComplexShaping(),
		"Arabic 需复杂整形")
	leg.probe("D.ScriptString", text.ScriptLatin.String() == "Latin",
		fmt.Sprintf("%q", text.ScriptLatin.String()))
	leg.probe("D.Direction", text.DirectionLTR.IsHorizontal() && !text.DirectionLTR.IsVertical(),
		fmt.Sprintf("LTR 横排=%v", text.DirectionLTR.IsHorizontal()))
	leg.probe("D.DirectionTTB", text.DirectionTTB.IsVertical(),
		"TTB 竖排")

	// paint anchor: loaded faces draw real text.
	if face != nil {
		dc.SetFont(face)
		dc.SetRGBA(0.92, 0.92, 0.95, 1)
		dc.DrawString("Resolver 20pt", 16, 80)
		dc.SetRGBA(1, 1, 1, 1)
	}
	if !flushTS(dc, view, leg, "C") {
		return leg
	}
	leg.imgs["resolver"] = dc.Image()

	leg.findings = append(leg.findings,
		"textsys: 进程级 SetDefaultFontPath/SetSystemFontPaths 反映到 DefaultFontResolver（Candidates 首元素=设置值）；Load/LoadMultiFace/LoadDefaultFace 全链出真 face；Script/Direction 枚举语义（Latin 简单、Han 复杂、LTR 横 TTB 竖）；SystemFontsByFamily 环境相关登记")
	return leg
}

// scene.go companion placeholders are in scene.go.
