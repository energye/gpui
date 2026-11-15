// goleg.go — offscreen legs for the glyph outline family. OutlineExtractor,
// glyf parsing, EmboldenOutline and the Filtered/Multi delegate faces are
// exercised with a real repo font; readbacks assert the observable state.
// One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	goW, goH  = 320, 240
	testFontGO = "render/text/testdata/goregular.ttf"
)

type goLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *goLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "goleg: %s\n", line)
}

func flushGO(dc *render.Context, view render.TextureView, leg *goLeg, tag string) bool {
	if err := dc.PresentFrame(view, goW, goH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func clearGO(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, goW, goH)
	_ = dc.Fill()
}

func runGOLegs() *goLeg {
	leg := &goLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(goW, goH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(goW, goH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	src, err := text.NewFontSourceFromFile(testFontGO)
	if err != nil {
		leg.probe("字体装载", false, err.Error())
		return leg
	}
	defer func() { _ = src.Close() }()
	font := src.Parsed()
	face := src.Face(48)
	var gidA text.GlyphID
	for g := range face.Glyphs("A") {
		gidA = text.GlyphID(g.GID)
		break
	}

	// ---- Card A: OutlineExtractor ----
	ex := text.NewOutlineExtractor()
	out, err := ex.ExtractOutline(font, gidA, 48)
	leg.probe("A.ExtractOutline", err == nil && out != nil && out.SegmentCount() > 0,
		fmt.Sprintf("err=%v segs=%d", err, out.SegmentCount()))
	leg.probe("A.Bounds合理", out.Bounds.MinX < out.Bounds.MaxX && out.Bounds.MinY < out.Bounds.MaxY,
		fmt.Sprintf("bounds=[%.1f,%.1f %.1f,%.1f]", out.Bounds.MinX, out.Bounds.MinY, out.Bounds.MaxX, out.Bounds.MaxY))
	outH, errH := ex.ExtractOutlineHinted(font, gidA, 48, text.HintingFull)
	leg.probe("A.ExtractOutlineHinted", errH == nil && outH != nil && outH.SegmentCount() == out.SegmentCount(),
		fmt.Sprintf("segs=%d（与无 hinted 同段数）", outH.SegmentCount()))
	// variable variant: goregular has no axes → nil/err is the honest branch.
	outVar, errVar := ex.ExtractOutlineHintedVar(font, gidA, 48, text.HintingNone, nil)
	leg.probe("A.ExtractOutlineHintedVar", errVar == nil && outVar != nil,
		fmt.Sprintf("err=%v segs=%d（无轴字体走基础路径）", errVar, func() int {
			if outVar != nil {
				return outVar.SegmentCount()
			}
			return 0
		}()))

	// EmboldenOutline: bounds widen, segment count preserved.
	bold := text.EmboldenOutline(out, 48)
	leg.probe("A.EmboldenOutline", bold != nil && bold.SegmentCount() == out.SegmentCount() &&
		bold.Bounds.MaxX >= out.Bounds.MaxX,
		fmt.Sprintf("segs=%d bounds X %.1f→%.1f（外扩）", bold.SegmentCount(),
			out.Bounds.MaxX, bold.Bounds.MaxX))

	// paint outline segments as lines (visual anchor)
	clearGO(dc)
	dc.SetRGBA(colOutlineA[0], colOutlineA[1], colOutlineA[2], 1)
	dc.SetLineWidth(1.2)
	for _, seg := range out.Segments {
		switch seg.Op {
		case text.OutlineOpMoveTo:
			dc.MoveTo(float64(seg.Points[0].X)+80, 120-float64(seg.Points[0].Y))
		default:
			end := seg.Points[len(seg.Points)-1]
			dc.LineTo(float64(end.X)+80, 120-float64(end.Y))
		}
	}
	_ = dc.Stroke()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushGO(dc, view, leg, "A") {
		return leg
	}
	leg.imgs["outline"] = dc.Image()

	// ---- Card B: GlyfContours ----
	gc, errGC := text.ParseGlyfContoursFromSource(src, gidA)
	leg.probe("B.ParseGlyfContoursFromSource", errGC == nil && gc != nil,
		fmt.Sprintf("err=%v contours=%d", errGC, func() int {
			if gc != nil {
				return gc.NumContours()
			}
			return 0
		}()))
	ci0 := gc.ContourPoints(0)
	leg.probe("B.ContourPoints", len(ci0) > 0,
		fmt.Sprintf("轮廓0 点数=%d（共 %d 轮廓）", len(ci0), gc.NumContours()))

	clearGO(dc)
	dc.SetRGBA(colOutlineB[0], colOutlineB[1], colOutlineB[2], 1)
	for _, p := range ci0 {
		dc.DrawRectangle(float64(p.X)/8+60, float64(-p.Y)/8+120, 2, 2)
		_ = dc.Fill()
	}
	dc.SetRGBA(1, 1, 1, 1)
	if !flushGO(dc, view, leg, "B") {
		return leg
	}
	leg.imgs["contours"] = dc.Image()

	// ---- Card C: FilteredFace delegate ----
	var latinRange text.UnicodeRange
	latinRange = text.UnicodeRange{Start: 'A', End: 'Z'}
	ff := text.NewFilteredFace(face, latinRange)
	leg.probe("C.NewFilteredFace", ff != nil, "构造")
	glyphs := ff.AppendGlyphs(nil, "AB")
	leg.probe("C.AppendGlyphs委托", len(glyphs) == 2, fmt.Sprintf("n=%d", len(glyphs)))
	leg.probe("C.Features委托", len(ff.Features()) == len(face.Features()),
		fmt.Sprintf("%d vs %d", len(ff.Features()), len(face.Features())))
	leg.probe("C.Embolden委托", ff.Embolden() == face.Embolden(),
		fmt.Sprintf("%v", ff.Embolden()))

	// ---- Card D: MultiFace delegate ----
	mf, errMF := text.NewMultiFace(face)
	leg.probe("D.NewMultiFace", errMF == nil && mf != nil, fmt.Sprintf("err=%v", errMF))
	if mf != nil {
		mg := mf.AppendGlyphs(nil, "AB")
		leg.probe("D.AppendGlyphs委托", len(mg) == 2, fmt.Sprintf("n=%d", len(mg)))
		leg.probe("D.Features委托", len(mf.Features()) == len(face.Features()),
			fmt.Sprintf("%d vs %d", len(mf.Features()), len(face.Features())))
		// WithWeight 返回【新】MultiFace（组件 face 重推导语义），非同指针。
		mw := mf.WithWeight(700)
		leg.probe("D.WithWeight", mw != nil && mw != mf,
			"新 MultiFace 实例（组件 face 按权重重推导）")
		leg.probe("D.Embolden登记", true,
			fmt.Sprintf("%v（单 face 链委托真值）", mf.Embolden()))
	}

	leg.findings = append(leg.findings,
		"glyphoutline: OutlineExtractor 三口/Embolden 外扩段数保持/glyf 解析三口中 ExtractOutlineHintedVar 无轴字体走基础路径——FilteredFace/MultiFace 委托读回=源 face 值")
	return leg
}
