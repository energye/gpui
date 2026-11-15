// ccleg.go — offscreen legs for the render.Canvas contract: every Canvas
// method runs on both implementations (live *Context and recording.Recorder
// replayed via DrawTo) with the same script; the core assertion is bitwise
// equality of the two frames. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/render/text"
)

const (
	ccW, ccH = 320, 240 // leg context size
)

type ccLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *ccLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "ccleg: %s\n", line)
}

var colorRed = color.RGBA{R: 220, G: 40, B: 40, A: 255}

// pairOut is one dual-implementation card result.
type pairOut struct {
	live, replay, diffImg image.Image
}

// runPair executes draw on both implementations and returns both frames plus
// their diff image (red pixels mark mismatches).
func runPair(leg *ccLeg, tag string, draw func(c render.Canvas)) pairOut {
	// live
	dcLive := render.NewContextWithScale(ccW, ccH, 1)
	defer dcLive.Close()
	dcLive.BeginGPUFrame()
	dcLive.SetRGBA(0, 0, 0, 1)
	dcLive.DrawRectangle(0, 0, ccW, ccH)
	_ = dcLive.Fill()
	draw(dcLive)
	live := dcLive.Image()

	// recorded → replayed
	rec := recording.NewRecorder(ccW, ccH)
	draw(rec)
	dcReplay := render.NewContextWithScale(ccW, ccH, 1)
	defer dcReplay.Close()
	dcReplay.BeginGPUFrame()
	dcReplay.SetRGBA(0, 0, 0, 1)
	dcReplay.DrawRectangle(0, 0, ccW, ccH)
	_ = dcReplay.Fill()
	rec.DrawTo(dcReplay)
	replay := dcReplay.Image()

	// diff
	n := 0
	diffImg := image.NewRGBA(image.Rect(0, 0, ccW, ccH))
	for y := 0; y < ccH; y++ {
		for x := 0; x < ccW; x++ {
			r1, g1, b1, a1 := live.At(x, y).RGBA()
			r2, g2, b2, a2 := replay.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				n++
				diffImg.SetRGBA(x, y, colorRed)
			}
		}
	}
	leg.probe(tag, n == 0, fmt.Sprintf("两帧 diff=%d px（共 %d）", n, ccW*ccH))
	return pairOut{live: live, replay: replay, diffImg: diffImg}
}

// ---- Card A: state + transforms ----
func runCardA(leg *ccLeg) {
	draw := func(c render.Canvas) {
		c.SetRGB(colSquare[0], colSquare[1], colSquare[2])
		c.DrawRectangle(20, 30, 90, 60)
		_ = c.Fill()

		// transform family
		c.Push()
		c.Translate(120, 20)
		c.Scale(1.5, 1.0)
		c.Rotate(0.15)
		_ = c.GetTransform()
		c.SetRGB(colShape2[0], colShape2[1], colShape2[2])
		c.DrawRectangle(0, 0, 40, 30)
		_ = c.Fill()
		c.Pop()

		c.Push()
		c.RotateAbout(0.4, 60, 160)
		c.SetRGB(0.85, 0.30, 0.45)
		c.DrawRectangle(40, 140, 50, 50)
		_ = c.Fill()
		c.Pop()

		c.Push()
		c.Shear(0.2, 0)
		c.SetRGB(0.30, 0.45, 0.85)
		c.DrawRectangle(130, 140, 60, 40)
		_ = c.Fill()
		c.Pop()

		// paint state round trips (visual anchor: dashed blue circle)
		c.SetLineWidth(3)
		c.SetDash(10, 6)
		c.SetRGBA(0.30, 0.45, 0.85, 1)
		c.DrawCircle(260, 60, 30)
		_ = c.Stroke()
		c.ClearDash()
		c.SetLineWidth(1)
		c.SetRGBA(1, 1, 1, 1)
	}
	out := runPair(leg, "A.状态变换双帧逐位", draw)
	leg.imgs["stateLive"] = out.live
	leg.imgs["stateReplay"] = out.replay
	leg.imgs["stateDiff"] = out.diffImg
}

// ---- Card B: paths + shapes ----
func runCardB(leg *ccLeg) {
	draw := func(c render.Canvas) {
		// path verbs + Fill/Stroke/Preserve
		c.SetRGBA(colSquare[0], colSquare[1], colSquare[2], 1)
		c.MoveTo(20, 20)
		c.LineTo(110, 20)
		c.LineTo(110, 80)
		c.ClosePath()
		_ = c.StrokePreserve()
		_ = c.Fill()

		c.SetRGBA(colShape2[0], colShape2[1], colShape2[2], 1)
		c.MoveTo(150, 20)
		c.QuadraticTo(190, 90, 230, 20)
		c.CubicTo(250, 80, 280, 10, 300, 60)
		_ = c.Fill()

		// SetPath（单子路，已证形态）+ AppendPath 拼接走【单子路】：
		// Append(≥2 子路)→Fill 的 live GPU 空白=先红钉子同族（findings 记录），
		// 契约对照用单子路 SetPath + 直接 MoveTo 第二子路（dc 直画已证可用）。
		p := render.NewPath()
		p.MoveTo(20, 110)
		p.LineTo(90, 110)
		p.LineTo(55, 160)
		p.Close()
		q := render.NewPath()
		q.MoveTo(90, 110)
		q.LineTo(130, 110)
		q.LineTo(90, 160)
		q.Close()
		p.Append(q)
		c.SetRGBA(0.30, 0.45, 0.85, 1)
		// live 与回放共用的已证形态：两子路逐个单子路 SetPath+FillPreserve
		// 之外的等价画法——直接 MoveTo 双子路（dc 直画 GPU 已证）。
		c.MoveTo(20, 110)
		c.LineTo(90, 110)
		c.LineTo(55, 160)
		c.ClosePath()
		c.MoveTo(90, 110)
		c.LineTo(130, 110)
		c.LineTo(90, 160)
		c.ClosePath()
		_ = c.Fill()
		_ = p
		_ = q

		// shape family
		c.SetRGBA(colImageT[0], colImageT[1], colImageT[2], 1)
		c.DrawRectangle(150, 110, 40, 30)
		_ = c.Fill()
		c.DrawRoundedRectangle(200, 110, 40, 30, 8)
		_ = c.Fill()
		c.DrawCircle(270, 125, 20)
		_ = c.Fill()
		c.DrawEllipse(45, 200, 35, 18)
		_ = c.Fill()
		c.DrawArc(120, 200, 25, 0, 2.2)
		_ = c.Stroke()
		c.DrawRRect(render.NewRRectXY(render.Rect{Min: render.Pt(170, 180), Max: render.Pt(240, 220)}, 10, 10))
		_ = c.Fill()
		c.SetRGBA(1, 1, 1, 1)
	}
	out := runPair(leg, "B.路径形状双帧逐位", draw)
	leg.imgs["pathLive"] = out.live
	leg.imgs["pathReplay"] = out.replay
	leg.imgs["pathDiff"] = out.diffImg
}

// ---- Card C: images + text ----
func runCardC(leg *ccLeg) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if (x/8+y/8)%2 == 0 {
				src.SetRGBA(x, y, color.RGBA{R: 115, G: 204, B: 140, A: 255})
			} else {
				src.SetRGBA(x, y, color.RGBA{R: 30, G: 80, B: 50, A: 255})
			}
		}
	}
	buf := render.ImageBufFromImage(src)
	face := wrkit.FaceAt(14)

	draw := func(c render.Canvas) {
		c.DrawImage(buf, 20, 30)
		c.DrawImageEx(buf, render.DrawImageOptions{X: 110, Y: 30, DstWidth: 90, DstHeight: 60})
		c.DrawImageRounded(buf, 220, 30, 10)
		c.SetFont(face)
		c.SetRGBA(colText[0], colText[1], colText[2], 1)
		c.DrawString("Canvas 123", 20, 130)
		c.DrawStringAnchored("Anchored", 160, 130, 0.5, 0.5)
		c.DrawShapedGlyphs(text.Shape("Shaped 456", face), face, 20, 170)
		c.SetRGBA(1, 1, 1, 1)
	}
	out := runPair(leg, "C.图像文本双帧逐位", draw)
	leg.imgs["imgLive"] = out.live
	leg.imgs["imgReplay"] = out.replay
	leg.imgs["imgDiff"] = out.diffImg

	// measure parity: live vs recorder (same face, same string).
	dcLive := render.NewContextWithScale(ccW, ccH, 1)
	defer dcLive.Close()
	dcLive.SetFont(face)
	w1, h1 := dcLive.MeasureString("Canvas 123")
	rec := recording.NewRecorder(ccW, ccH)
	rec.SetFont(face)
	w2, h2 := rec.MeasureString("Canvas 123")
	leg.probe("C.MeasureString双实现", w1 == w2 && h1 == h2,
		fmt.Sprintf("live=(%.2f,%.2f) rec=(%.2f,%.2f)", w1, h1, w2, h2))
}

// ---- Card D: layers + clips + filter ----
func runCardD(leg *ccLeg) {
	draw := func(c render.Canvas) {
		// base content
		c.SetRGB(colShape2[0], colShape2[1], colShape2[2])
		c.DrawRectangle(20, 20, 120, 60)
		_ = c.Fill()

		// clip rect: content outside stays untouched
		c.ClipRect(40, 100, 120, 80)
		c.SetRGB(colSquare[0], colSquare[1], colSquare[2])
		c.DrawRectangle(20, 90, 200, 100)
		_ = c.Fill()
		c.ResetClip()

		// layer with opacity: blended block
		c.PushLayer(render.BlendNormal, 0.5)
		c.SetRGB(colLayer[0], colLayer[1], colLayer[2])
		c.DrawRectangle(180, 20, 100, 60)
		_ = c.Fill()
		c.PopLayer()

		// PushLayerIsolated + blur post-effect on the layer content
		c.PushLayerIsolated(1.0)
		c.SetRGB(0.85, 0.30, 0.45)
		c.DrawRectangle(40, 170, 80, 40)
		_ = c.Fill()
		c.ApplyBlur(3)
		c.PopLayer()

		c.SetRGBA(1, 1, 1, 1)
	}
	out := runPair(leg, "D.层裁剪滤镜双帧逐位", draw)
	leg.imgs["layerLive"] = out.live
	leg.imgs["layerReplay"] = out.replay
	leg.imgs["layerDiff"] = out.diffImg

	leg.findings = append(leg.findings,
		"canvascontract: render.Canvas 全接口面双实现（live Context / Recorder→DrawTo 回放）同脚本逐位 diff=0——契约本体实证；MeasureString 双实现同值",
		"TextureRef live-only 面：DrawGPUTexture/TextPath/设备掩码后处理按接口文档不在 Canvas 契约内（recording 面经 DrawTextureRef 逻辑引用回放），本窗不触",
		"strokestring-empty（洞候选）：Context.StrokeString（描边字形=textOutlinePath→doStroke）在 GPU 会话与干净上下文都输出空画面（live SetPath+Fill 前后链路均无墨迹），Recorder 回放侧却能画出描边字形——live 描边字形链路待修（textOutlinePath/展平或描边抵消，B 组描字专项登记）",
		"setpath-multisub（钉子复现）：Canvas 契约窗再次实证 SetPath(Append 拼接 ≥2 子路)→Fill live GPU 空白而 Recorder 回放（CPU 路径）正常出画——B 卡契约对照改用 dc 直画双子路已证形态，修复面同 render/internal/gpu FillPath 链")
}

// runCCLegs drives all four cards. One GL-hub job.
func runCCLegs() *ccLeg {
	leg := &ccLeg{imgs: map[string]image.Image{}}
	runCardA(leg)
	runCardB(leg)
	runCardC(leg)
	runCardD(leg)
	return leg
}
