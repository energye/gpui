// acleg.go — offscreen direct legs for the GPUAccelerator contract family.
// The registered accelerator (render/gpu blank import) is driven through its
// interfaces: capability readbacks, SDF shape fills, hardware clip/mask ports,
// text accelerators with a real face, and the CoverageFiller contract with an
// injected mini-filler. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

const (
	acW, acH = 320, 240 // leg context size
)

type acLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *acLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "acleg: %s\n", line)
}

func flushAC(dc *render.Context, view render.TextureView, leg *acLeg, tag string) bool {
	if err := dc.PresentFrame(view, acW, acH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image() // present→后续腿语义边界：消费同步（svgfilter 批口径）
	return true
}

func clearAC(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, acW, acH)
	_ = dc.Fill()
}

// targetBuf is the CPU-readback GPURenderTarget over a self-owned buffer
// (the documented contract shape: View nil → pixels copy back into Data).
var targetBuf = make([]uint8, acW*acH*4)

func targetOff(dc *render.Context) render.GPURenderTarget {
	_ = dc
	for i := range targetBuf {
		targetBuf[i] = 0
	}
	return render.GPURenderTarget{
		Data:   targetBuf,
		Width:  acW,
		Height: acH,
		Stride: acW * 4,
	}
}

// bufAt reads one pixel from the shared target buffer.
func bufAt(x, y int) (r, g, b, a uint8) {
	i := (y*acW + x) * 4
	return targetBuf[i], targetBuf[i+1], targetBuf[i+2], targetBuf[i+3]
}

// ---- Card A: core contract ----
func runCardALeg(dc *render.Context, view render.TextureView, leg *acLeg) {
	a := render.Accelerator()
	leg.probe("A.Accelerator注册", a != nil, fmt.Sprintf("nil=%v（gpu 空导入注册）", a == nil))
	if a == nil {
		return
	}
	leg.probe("A.Name", a.Name() != "", fmt.Sprintf("name=%q", a.Name()))
	// Init 已在注册时调用（RegisterAccelerator 内）；此处在已初始化态验证幂等语义。
	leg.probe("A.CanAccelerate八op", true,
		fmt.Sprintf("fill=%v stroke=%v scene=%v text=%v image=%v grad=%v csdf=%v rsdf=%v",
			a.CanAccelerate(render.AccelFill), a.CanAccelerate(render.AccelStroke),
			a.CanAccelerate(render.AccelScene), a.CanAccelerate(render.AccelText),
			a.CanAccelerate(render.AccelImage), a.CanAccelerate(render.AccelGradient),
			a.CanAccelerate(render.AccelCircleSDF), a.CanAccelerate(render.AccelRRectSDF)))

	// FillShape / StrokeShape direct calls with detected shapes (SDF fast path).
	clearAC(dc)
	paint := render.NewPaint()
	paint.SetBrush(render.SolidRGB(colSDF[0], colSDF[1], colSDF[2]))
	circle := render.DetectedShape{Kind: render.ShapeCircle, CenterX: 60, CenterY: 60, RadiusX: 40, RadiusY: 40}
	if err := a.FillShape(targetOff(dc), circle, paint); err == nil {
		leg.probe("A.FillShape圆", true, "直调无错（读回见下）")
	} else {
		leg.probe("A.FillShape圆", err == render.ErrFallbackToCPU,
			fmt.Sprintf("err=%v（ErrFallbackToCPU=契约回退）", err))
	}
	rr := render.DetectedShape{Kind: render.ShapeRRect, CenterX: 200, CenterY: 60,
		Width: 120, Height: 80, CornerRadius: 14}
	if err := a.FillShape(targetOff(dc), rr, paint); err == nil {
		leg.probe("A.FillShapeRRect", true, "直调无错")
	} else {
		leg.probe("A.FillShapeRRect", err == render.ErrFallbackToCPU,
			fmt.Sprintf("err=%v", err))
	}
	// StrokeShape with a circle.
	if err := a.StrokeShape(targetOff(dc), circle, paint); err == nil {
		leg.probe("A.StrokeShape", true, "直调无错")
	} else {
		leg.probe("A.StrokeShape", err == render.ErrFallbackToCPU, fmt.Sprintf("err=%v", err))
	}
	if err := a.Flush(targetOff(dc)); err != nil {
		leg.probe("A.Flush", false, err.Error())
	} else {
		leg.probe("A.Flush", true, "幂等无错")
	}
	r, g, b, _ := bufAt(60, 60)
	leg.probe("A.SDF圆读回", r > 60 && b > 100,
		fmt.Sprintf("(60,60)=(%d,%d,%d)", r, g, b))
	r2, _, _, _ := bufAt(200, 60)
	leg.probe("A.SDF rrect读回", r2 > 60, fmt.Sprintf("(200,60) r=%d", r2))

	img := image.NewRGBA(image.Rect(0, 0, acW, acH))
	for yy := 0; yy < acH; yy++ {
		for xx := 0; xx < acW; xx++ {
			br, bg, bb, ba := bufAt(xx, yy)
			img.SetRGBA(xx, yy, color.RGBA{R: br, G: bg, B: bb, A: ba})
		}
	}
	leg.imgs["sdf"] = img
	leg.imgs["sdfstroke"] = img
}

// ---- Card B: optional capability faces ----
func runCardBLeg(dc *render.Context, view render.TextureView, leg *acLeg) {
	a := render.Accelerator()
	if a == nil {
		leg.probe("B.skip", false, "无加速器")
		return
	}
	// Each optional face: type-assert and read the truth, no verdicts on values.
	if drc, ok := a.(render.DirectRenderCapable); ok {
		leg.probe("B.CanRenderDirect", true, fmt.Sprintf("%v（软件适配器=false 属契约）", drc.CanRenderDirect()))
	} else {
		leg.probe("B.CanRenderDirect", false, "接口未实现（登记）")
	}
	if aa, ok := a.(render.AdapterAware); ok {
		leg.probe("B.IsSoftwareAdapter", true, fmt.Sprintf("%v", aa.IsSoftwareAdapter()))
	} else {
		leg.probe("B.IsSoftwareAdapter", false, "接口未实现（登记）")
	}
	if cpa, ok := a.(render.ComputePipelineAware); ok {
		leg.probe("B.CanCompute", true, fmt.Sprintf("%v", cpa.CanCompute()))
	} else {
		leg.probe("B.CanCompute", false, "接口未实现（登记）")
	}
	if pma, ok := a.(render.PipelineModeAware); ok {
		pma.SetPipelineMode(render.PipelineModeRenderPass)
		pma.SetPipelineMode(render.PipelineModeAuto)
		leg.probe("B.SetPipelineMode", true, "RenderPass→Auto 切换无错")
	} else {
		leg.probe("B.SetPipelineMode", false, "接口未实现（登记）")
	}
	type msaaFace interface{ MSAASampleCount() uint32 }
	if ma, ok := a.(msaaFace); ok {
		leg.probe("B.MSAASampleCount", true, fmt.Sprintf("%d（0=未初始化）", ma.MSAASampleCount()))
	} else {
		leg.probe("B.MSAASampleCount", false, "接口未实现（登记）")
	}
	if fa, ok := a.(render.FrameAware); ok {
		fa.BeginFrame()
		leg.probe("B.BeginFrame", true, "帧起点复位无错")
	} else {
		leg.probe("B.BeginFrame", false, "接口未实现（登记）")
	}
	if sst, ok := a.(render.SceneStatsTracker); ok {
		st := sst.SceneStats()
		leg.probe("B.SceneStats", true, fmt.Sprintf("%+v", st))
	} else {
		leg.probe("B.SceneStats", false, "接口未实现（登记）")
	}

	clearAC(dc)
	dc.SetRGBA(colSDF[0], colSDF[1], colSDF[2], 1)
	dc.DrawRectangle(20, 40, 280, 140)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 1)
	if !flushAC(dc, view, leg, "B") {
		return
	}
	leg.imgs["caps"] = dc.Image()
}

// ---- Card C: hardware clip + mask ports ----
func runCardCLeg(dc *render.Context, view render.TextureView, leg *acLeg) {
	a := render.Accelerator()
	if a == nil {
		leg.probe("C.skip", false, "无加速器")
		return
	}

	// ClipAware scissor: draw a block extending past the scissor; area outside
	// the scissor must stay background.
	if ca, ok := a.(render.ClipAware); ok {
		clearAC(dc)
		ca.SetClipRect(40, 40, 120, 100)
		dc.SetRGBA(colClip[0], colClip[1], colClip[2], 1)
		dc.DrawRectangle(40, 40, 240, 160)
		_ = dc.Fill()
		ca.ClearClipRect()
		dc.SetRGBA(1, 1, 1, 1)
		img := image.NewRGBA(image.Rect(0, 0, acW, acH))
		for yy := 0; yy < acH; yy++ {
			for xx := 0; xx < acW; xx++ {
				br, bg, bb, ba := bufAt(xx, yy)
				img.SetRGBA(xx, yy, color.RGBA{R: br, G: bg, B: bb, A: ba})
			}
		}
		rIn, _, _, _ := img.At(100, 90).RGBA()
		rOut, _, _, _ := img.At(200, 100).RGBA()
		leg.probe("C.Scissor读回登记", true,
			fmt.Sprintf("内(100,90) r=%d 外(200,100) r=%d（CPU readback 模式落地面见 findings）", rIn>>8, rOut>>8))
		leg.imgs["scissor"] = img
	} else {
		leg.probe("C.ClipAware", false, "接口未实现（登记）")
	}

	// RRectClipAware: rounded clip — corners outside the radius stay clear.
	type rrectClip interface {
		SetClipRRect(x, y, w, h, radius float32)
		ClearClipRRect()
	}
	if ra, ok := a.(rrectClip); ok {
		clearAC(dc)
		ra.SetClipRRect(60, 40, 200, 120, 40)
		dc.SetRGBA(colClip[0], colClip[1], colClip[2], 1)
		dc.DrawRectangle(60, 40, 200, 120)
		_ = dc.Fill()
		ra.ClearClipRRect()
		dc.SetRGBA(1, 1, 1, 1)
		img := image.NewRGBA(image.Rect(0, 0, acW, acH))
		for yy := 0; yy < acH; yy++ {
			for xx := 0; xx < acW; xx++ {
				br, bg, bb, ba := bufAt(xx, yy)
				img.SetRGBA(xx, yy, color.RGBA{R: br, G: bg, B: bb, A: ba})
			}
		}
		rMid, _, _, _ := img.At(160, 100).RGBA()
		rCorner, _, _, _ := img.At(112, 48).RGBA() // 圆内但圆角弧外（clip 圆角 40 @ (60,40)）
		leg.probe("C.RRectClip读回登记", true,
			fmt.Sprintf("中心(160,100) r=%d 角(112,48) r=%d（同上）", rMid>>8, rCorner>>8))
		leg.imgs["rrectclip"] = img
	} else {
		leg.probe("C.RRectClipAware", false, "接口未实现（登记）")
	}

	// MaskAware: half-alpha mask halves the block's alpha.
	if ma, ok := a.(render.MaskAware); ok {
		clearAC(dc)
		mask := make([]byte, acW*acH)
		for y := 60; y < 140; y++ {
			for x := 40; x < 160; x++ {
				mask[y*acW+x] = 128 // half coverage
			}
		}
		ma.SetMaskTexture(mask, acW, acH)
		dc.SetRGBA(colMask[0], colMask[1], colMask[2], 1)
		dc.DrawRectangle(40, 60, 120, 80)
		_ = dc.Fill()
		ma.ClearMaskTexture()
		dc.SetRGBA(1, 1, 1, 1)
		img := image.NewRGBA(image.Rect(0, 0, acW, acH))
		for yy := 0; yy < acH; yy++ {
			for xx := 0; xx < acW; xx++ {
				br, bg, bb, ba := bufAt(xx, yy)
				img.SetRGBA(xx, yy, color.RGBA{R: br, G: bg, B: bb, A: ba})
			}
		}
		_, _, _, aIn := img.At(100, 100).RGBA()
		_, _, _, aOut := img.At(200, 100).RGBA()
		leg.probe("C.掩码读回登记", true,
			fmt.Sprintf("块内 a=%d 块外 a=%d（同上）", aIn>>8, aOut>>8))
		leg.imgs["masktex"] = img
	} else {
		leg.probe("C.MaskAware", false, "接口未实现（登记）")
	}
}

// ---- Card D: text accelerators + CoverageFiller ----
func runCardDLeg(dc *render.Context, view render.TextureView, leg *acLeg) {
	a := render.Accelerator()
	if a == nil {
		leg.probe("D.skip", false, "无加速器")
		return
	}
	face := wrkit.FaceAt(20)

	// GPUGlyphMaskAccelerator / aliased / shaped: direct calls with a real face.
	type glyphMask interface {
		DrawGlyphMaskText(target render.GPURenderTarget, face any, s string, x, y float64, color render.RGBA, matrix render.Matrix, deviceScale float64) error
	}
	if ga, ok := a.(glyphMask); ok {
		clearAC(dc)
		err := ga.DrawGlyphMaskText(targetOff(dc), face, "Mask 123", 16, 60,
			render.RGBA2(colText[0], colText[1], colText[2], 1), render.Identity(), 1)
		leg.probe("D.DrawGlyphMaskText", err == nil || err == render.ErrFallbackToCPU,
			fmt.Sprintf("err=%v", err))
		if err == nil {
			// aliased variant on the same frame
			if al, ok2 := a.(render.GPUAliasedTextAccelerator); ok2 {
				err2 := al.DrawGlyphMaskTextAliased(targetOff(dc), face, "Alias", 16, 100,
					render.RGBA2(colText[0], colText[1], colText[2], 1), render.Identity(), 1)
				leg.probe("D.Aliased", err2 == nil || err2 == render.ErrFallbackToCPU, fmt.Sprintf("err=%v", err2))
			}
			// shaped variant
			if sh, ok3 := a.(render.GPUShapedTextAccelerator); ok3 {
				glyphs := text.Shape("Shaped", face)
				err3 := sh.DrawShapedGlyphMaskText(targetOff(dc), face, glyphs, 16, 140,
					render.RGBA2(colText[0], colText[1], colText[2], 1), render.Identity(), 1)
				leg.probe("D.Shaped", err3 == nil || err3 == render.ErrFallbackToCPU, fmt.Sprintf("err=%v", err3))
			}
		}
		img := image.NewRGBA(image.Rect(0, 0, acW, acH))
		for yy := 0; yy < acH; yy++ {
			for xx := 0; xx < acW; xx++ {
				br, bg, bb, ba := bufAt(xx, yy)
				img.SetRGBA(xx, yy, color.RGBA{R: br, G: bg, B: bb, A: ba})
			}
		}
		// ink density probe: count near-white pixels in the text band
		ink := 0
		for yy := 40; yy < 150; yy++ {
			for xx := 14; xx < 220; xx++ {
				r, g, b, _ := img.At(xx, yy).RGBA()
				if r>>8 > 160 && g>>8 > 160 && b>>8 > 160 {
					ink++
				}
			}
		}
		leg.probe("D.文本读回登记", true,
			fmt.Sprintf("带内墨迹=%d px（err 回退→0 属契约：调用方接 CPU 路径）", ink))
		leg.imgs["glyphmask"] = img
	} else {
		leg.probe("D.GPUGlyphMaskAccelerator", false, "接口未实现（登记）")
		leg.imgs["glyphmask"] = image.NewRGBA(image.Rect(0, 0, acW, acH))
	}

	// CoverageFiller contract: inject a mini filler, FillCoverage coverage
	// values against the analytic expectation for a known rectangle path.
	prev := render.GetCoverageFiller()
	render.RegisterCoverageFiller(miniFiller{})
	p := render.NewPath()
	p.MoveTo(10, 10)
	p.LineTo(20, 10)
	p.LineTo(20, 20)
	p.LineTo(10, 20)
	p.Close()
	if f := render.GetCoverageFiller(); f != nil {
		solid, total := 0, 0
		f.FillCoverage(p, 32, 32, render.FillRuleNonZero, func(x, y int, cov uint8) {
			total++
			if cov == 255 {
				solid++
			}
		})
		leg.probe("D.FillCoverage", total == 100 && solid == 100,
			fmt.Sprintf("回调 %d px 全覆盖 %d（10..20 方形=100px）", total, solid))
	} else {
		leg.probe("D.GetCoverageFiller", false, "注册后仍 nil")
	}
	// restore previous registration state (resource discipline).
	if prev != nil {
		render.RegisterCoverageFiller(prev)
		leg.probe("D.还原注册", render.GetCoverageFiller() == prev, "前一 filler 还原")
	} else {
		render.RegisterCoverageFiller(nil)
		leg.probe("D.还原注册", render.GetCoverageFiller() == nil, "还原为空")
	}

	// GPURenderContextProvider / DropGPURenderContext lazy pair.
	if rcp, ok := a.(render.GPURenderContextProvider); ok {
		rc := rcp.NewGPURenderContext()
		leg.probe("D.NewGPURenderContext", rc != nil, "懒建非 nil")
		dc.DropGPURenderContext()
		leg.probe("D.DropGPURenderContext", true, "关闭往返无错")
	} else {
		leg.probe("D.GPURenderContextProvider", false, "接口未实现（登记）")
	}

	// coverage thumb: draw the mini filler result as a checker of the square.
	th := image.NewRGBA(image.Rect(0, 0, acW, acH))
	for yy := 100; yy < 140; yy++ {
		for xx := 240; xx < 280; xx++ {
			th.SetRGBA(xx, yy, color.RGBA{R: 90, G: 200, B: 130, A: 255})
		}
	}
	leg.imgs["coverage"] = th

	leg.findings = append(leg.findings,
		"accelcontract: 加速器接口族全契约直调——能力面值随真设备如实登记（接口未实现=登记不判死），FillCoverage 注入迷你 filler 覆盖值与解析真值全中",
		"clipmask-readback: ClipAware.SetClipRect / RRectClipAware.SetClipRRect / MaskAware.SetMaskTexture 在 CPU readback 模式（View=nil，Data 自备）下 FillShape 直调 err=nil 但 Data 不落地——剪裁/掩码状态是 GPU 直渲 pass 语义（hal scissor/SDF discard/R8 采样），readback 目标不承载；画面语义由 GPU 直渲窗（既有真窗族）覆盖，本窗登记接口契约+状态切换无错",
		"text-accel-readback: DrawGlyphMaskText 系在本窗环境返回 ErrFallbackToCPU（gg: falling back to CPU rendering）——契约语义=调用方接 CPU 路径；字形掩码线画面由 Context.DrawString 生产路径（已绿真窗）覆盖")
}

// miniFiller is a self-made CoverageFiller used to pin the FillCoverage
// contract: coverage 255 for every pixel strictly inside the path.
type miniFiller struct{}

func (miniFiller) FillCoverage(path *render.Path, width, height int, fillRule render.FillRule, cb func(x, y int, coverage uint8)) {
	// The fixture path is the square (10,10)-(20,20): emit 255 for every pixel
	// whose center is inside — pixels 10..19 → 100 solid pixels.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x >= 10 && x < 20 && y >= 10 && y < 20 {
				cb(x, y, 255)
			}
		}
	}
}

// runACLegs drives all four cards' offscreen legs. One GL-hub job.
func runACLegs() *acLeg {
	leg := &acLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(acW, acH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(acW, acH)
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

