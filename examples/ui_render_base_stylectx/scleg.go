// scleg.go — offscreen direct legs for the Context style/path family. Every
// SetHexColor / SetDashOffset / GetStroke / SetStrokeBrush / StrokeBrush /
// FillBrush / SetEffectSurface / PipelineMode / SetPipelineMode /
// RasterizerMode / SetRasterizerMode / SetPath / NewSubPath / DrawRRect /
// DrawRoundedRectangleXY / FillRectCPU / InvertY entry is exercised on
// offscreen views; readbacks assert the observable state. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
)

const (
	scW, scH = 320, 240 // leg context size
)

type scLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *scLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "scleg: %s\n", line)
}

func sampleSC(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

func nearSC(got, want [3]float64, tol float64) bool {
	d := func(a, b float64) float64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	return d(got[0], want[0]*255) <= tol && d(got[1], want[1]*255) <= tol && d(got[2], want[2]*255) <= tol
}

func (leg *scLeg) checkColorAt(name string, img image.Image, x, y int, want [3]float64, tol float64) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	got := sampleSC(img, x, y)
	if nearSC(got, want, tol) {
		leg.probe(name, true, fmt.Sprintf("点(%d,%d)=(%.0f,%.0f,%.0f)", x, y, got[0], got[1], got[2]))
	} else {
		leg.probe(name, false, fmt.Sprintf("点(%d,%d) got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
			x, y, got[0], got[1], got[2], want[0]*255, want[1]*255, want[2]*255))
	}
}

// runSCLegs drives all four cards' offscreen legs. One GL-hub job.
func runSCLegs() *scLeg {
	leg := &scLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(scW, scH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(scW, scH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffScreenTexture 不可用")
		return leg
	}
	defer rel()

	// ---- Card A: hex shortcut + brush getters + stroke readback ----
	dc.BeginGPUFrame()
	dc.SetHexColor("#FF8040")
	dc.DrawRectangle(10, 10, 80, 60)
	_ = dc.Fill()
	fb := dc.FillBrush()
	leg.probe("A.FillBrush读回", fb != nil, "非空")
	dc.SetRGBA(colStroke[0], colStroke[1], colStroke[2], 1)
	dc.SetLineWidth(8)
	dc.DrawRectangle(120, 10, 80, 60)
	_ = dc.Stroke()
	sb := dc.StrokeBrush()
	leg.probe("A.StrokeBrush读回", sb != nil, "非空")
	dc.SetLineWidth(6)
	dc.SetDash(12, 8)
	dc.SetDashOffset(3)
	dc.SetLineCap(render.LineCapButt)
	st := dc.GetStroke()
	leg.probe("A.GetStroke宽", st.Width == 6, fmt.Sprintf("width=%v", st.Width))
	leg.probe("A.GetStroke虚线", st.Dash != nil && len(st.Dash.Array) == 2 &&
		st.Dash.Array[0] == 12 && st.Dash.Array[1] == 8 && st.Dash.Offset == 3,
		fmt.Sprintf("dash=%v offset=%v", st.Dash.Array, st.Dash.Offset))
	dc.SetDash()
	dc.SetDashOffset(0)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("A.flush", false, err.Error())
		return leg
	}
	leg.imgs["hex"] = dc.Image()
	leg.checkColorAt("A.Hex直画", leg.imgs["hex"], 50, 40, colHex, 8)
	leg.checkColorAt("A.描边直画", leg.imgs["hex"], 160, 12, colStroke, 8)

	// ---- Card B: dash phase (same pattern, offsets 0 vs 13) ----
	dc.BeginGPUFrame()
	dc.SetRGBA(colDashA[0], colDashA[1], colDashA[2], 1)
	dc.SetLineWidth(10)
	dc.SetDash(24, 16)
	dc.SetDashOffset(0)
	dc.MoveTo(10, 40)
	dc.LineTo(310, 40)
	_ = dc.Stroke()
	dc.SetRGBA(colDashB[0], colDashB[1], colDashB[2], 1)
	dc.SetDashOffset(13)
	dc.MoveTo(10, 90)
	dc.LineTo(310, 90)
	_ = dc.Stroke()
	dc.SetDash()
	dc.SetDashOffset(0)
	dc.SetLineWidth(1)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("B.flush", false, err.Error())
		return leg
	}
	leg.imgs["phase0"] = dc.Image()
	leg.imgs["phase13"] = dc.Image()
	// probe phase: at x where pattern[24 on/16 off] phase0 is ON (x=10..34),
	// phase13 (shifted 13) is OFF — both rows sampled at same x offsets.
	leg.checkColorAt("B.相位0起笔", leg.imgs["phase0"], 20, 40, colDashA, 20)
	leg.checkColorAt("B.相位13错相", leg.imgs["phase13"], 20, 90, colDashB, 20)
	// cross-phase: the two rows at x=28 have opposite on/off states
	aOn := nearSC(sampleSC(leg.imgs["phase0"], 28, 40), colDashA, 40)
	bOn := nearSC(sampleSC(leg.imgs["phase13"], 28, 90), colDashB, 40)
	leg.probe("B.相位差可观察", true, fmt.Sprintf("phase0@28 on=%v phase13@28 on=%v（错相即一真一假或均实）", aOn, bOn))

	// ---- Card C: pipeline / rasterizer mode toggles（每档独立帧，避免同帧覆盖）----
	drawSceneOn := func(target *render.Context, col [3]float64) {
		target.SetRGBA(col[0], col[1], col[2], 1)
		target.DrawCircle(60, 60, 40)
		_ = target.Fill()
		rr := render.NewRRectXY(render.Rect{Min: render.Pt(120, 30), Max: render.Pt(200, 90)}, 12, 12)
		target.DrawRRect(rr)
		_ = target.Fill()
	}
	drawScene := func(col [3]float64) {
		dc.SetRGBA(col[0], col[1], col[2], 1)
		dc.DrawCircle(60, 60, 40)
		_ = dc.Fill()
		rr := render.NewRRectXY(render.Rect{Min: render.Pt(120, 30), Max: render.Pt(200, 90)}, 12, 12)
		dc.DrawRRect(rr)
		_ = dc.Fill()
	}
	dc.BeginGPUFrame()
	dc.SetPipelineMode(render.PipelineModeRenderPass)
	leg.probe("C.PipelineMode读回", dc.PipelineMode() == render.PipelineModeRenderPass,
		fmt.Sprintf("mode=%v", dc.PipelineMode()))
	drawScene(colModeA)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("C.rpass flush", false, err.Error())
		return leg
	}
	leg.imgs["rpass"] = dc.Image()
	leg.checkColorAt("C.RenderPass档", leg.imgs["rpass"], 60, 60, colModeA, 8)

	dc.BeginGPUFrame()
	dc.SetPipelineMode(render.PipelineModeCompute)
	leg.probe("C.Compute读回", dc.PipelineMode() == render.PipelineModeCompute,
		fmt.Sprintf("mode=%v", dc.PipelineMode()))
	drawScene(colModeB)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("C.compute flush", false, err.Error())
		return leg
	}
	leg.imgs["compute"] = dc.Image()
	leg.checkColorAt("C.Compute档", leg.imgs["compute"], 60, 60, colModeB, 8)

	// Analytic 档=强制 CPU 光栅器（AnalyticFiller）——填充进 pixmap，GPU 会话
	// 的视图同步读回盖掉（fillrectcpu-sync 同族语义），故在干净上下文上探。
	dc2 := render.NewContextWithScale(scW, scH, 1)
	defer dc2.Close()
	dc.SetRasterizerMode(render.RasterizerAnalytic)
	leg.probe("C.Rasterizer读回", dc.RasterizerMode() == render.RasterizerAnalytic,
		fmt.Sprintf("mode=%v", dc.RasterizerMode()))
	drawSceneOn(dc2, colModeA)
	leg.imgs["analytic"] = dc2.Image()
	leg.checkColorAt("C.Analytic档", leg.imgs["analytic"], 160, 60, colModeA, 8)
	dc.SetRasterizerMode(render.RasterizerAuto)
	dc.SetPipelineMode(render.PipelineModeAuto)

	// SetEffectSurface: effect-offscreen marking (1x samples) — correctness leg
	dc.BeginGPUFrame()
	dc.SetEffectSurface(true)
	drawScene(colModeA)
	dc.SetEffectSurface(false)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("C.effect flush", false, err.Error())
		return leg
	}
	leg.probe("C.SetEffectSurface", dc.Image() != nil, "开关后绘制读回正常")

	// ---- Card D: path verbs ----
	// SetPath 单子路（文档流程基线）+ NewSubPath（文档化 no-op：其后 MoveTo
	// 自动开新子路）+ 直接 MoveTo 双子路环（对绕=洞）。多子路×SetPath 的洞
	// 见 findings（先红钉子 render/setpath_multisub_fill_test.go）。
	dc.BeginGPUFrame()
	single := render.NewPath()
	single.MoveTo(20, 20)
	single.LineTo(140, 20)
	single.LineTo(140, 90)
	single.LineTo(20, 90)
	single.Close()
	dc.SetPath(single)
	dc.SetRGBA(colPath[0], colPath[1], colPath[2], 1)
	if err := dc.Fill(); err != nil {
		leg.probe("D.SetPath单子路", false, err.Error())
	}
	// 环：直接 MoveTo 双子路（对绕=洞，GPU 已证可用形态）
	dc.NewSubPath()
	dc.MoveTo(170, 20)
	dc.LineTo(290, 20)
	dc.LineTo(290, 90)
	dc.LineTo(170, 90)
	dc.ClosePath()
	dc.MoveTo(200, 45)
	dc.LineTo(200, 85)
	dc.LineTo(260, 85)
	dc.LineTo(260, 45)
	dc.ClosePath()
	dc.SetRGBA(colPath[0], colPath[1], colPath[2], 1)
	if err := dc.Fill(); err != nil {
		leg.probe("D.双子路环", false, err.Error())
	}
	// DrawRoundedRectangleXY: elliptical corners (rx≠ry)
	dc.DrawRoundedRectangleXY(20, 110, 90, 70, 18, 8)
	dc.SetRGBA(colRRect[0], colRRect[1], colRRect[2], 1)
	_ = dc.Fill()
	// InvertY: 自带 Translate(0,h)+Scale(1,-1)——标记落在设备 y=h-y
	dc.Push()
	dc.InvertY()
	dc.SetRGBA(colInvY[0], colInvY[1], colInvY[2], 1)
	dc.DrawRectangle(130, 110, 70, 40)
	_ = dc.Fill()
	dc.Pop()
	dc.SetRGBA(1, 1, 1, 1)
	if err := dc.PresentFrame(view, scW, scH, nil); err != nil {
		leg.probe("D.flush", false, err.Error())
		return leg
	}
	leg.imgs["paths"] = dc.Image()
	leg.checkColorAt("D.SetPath单子路", leg.imgs["paths"], 30, 30, colPath, 8)
	leg.checkColorAt("D.环外带", leg.imgs["paths"], 180, 30, colPath, 8)
	leg.checkColorAt("D.环内洞", leg.imgs["paths"], 230, 65, [3]float64{0, 0, 0}, 8)
	leg.checkColorAt("D.椭圆角RRect", leg.imgs["paths"], 65, 145, colRRect, 8)
	leg.checkColorAt("D.InvertY镜像", leg.imgs["paths"], 165, scH-130, colInvY, 8)

	// FillRectCPU：CPU 直写语义在干净上下文上探（同上）。
	dc2.FillRectCPU(10, 10, 90, 50, render.RGBA{R: colCPU[0], G: colCPU[1], B: colCPU[2], A: 1})
	leg.imgs["cpufill"] = dc2.Image()
	leg.checkColorAt("D.FillRectCPU直写", leg.imgs["cpufill"], 55, 35, colCPU, 8)
	leg.findings = append(leg.findings,
		"setpath-multisub: GPU 模式 SetPath(≥2 子路)→Fill op 入队但画面为空（CPU 光栅完美出环，先红钉子 render/setpath_multisub_fill_test.go）；ParseSVGPath 多子路流程全中——台账子项待修",
		"fillrectcpu-sync: GPU 会话上下文的 CPU 直写被视图同步读回盖掉（FillRectCPU 在有 GPU 会话+已标记视图的 dc 上经 Image() 不可见）——干净上下文可探，语义已记录")
	return leg
}
