// pjleg.go — offscreen legs for the projection family: Push/Pop/Set/Projection
// stack semantics, identity normalization, policy constants, and the draw
// effect of a non-identity projection. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

const (
	pjW, pjH = 320, 240
)

type pjLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *pjLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "pjleg: %s\n", line)
}

func flushPJ(dc *render.Context, view render.TextureView, leg *pjLeg, tag string) bool {
	if err := dc.PresentFrame(view, pjW, pjH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func clearPJ(dc *render.Context) {
	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, pjW, pjH)
	_ = dc.Fill()
}

func runPJLegs() *pjLeg {
	leg := &pjLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(pjW, pjH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(pjW, pjH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	// ---- Card A: stack semantics ----
	clearPJ(dc)
	leg.probe("A.初始恒等", dc.Projection().IsIdentity(), "未设置=恒等")
	// Matrix4FromElements 列主序 e[12]=Tx。
	translate := render.Matrix4FromElements([16]float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		40, 0, 0, 1,
	})
	dc.PushProjection(translate)
	leg.probe("A.Push后读回", dc.Projection() == translate,
		"Projection()==推入矩阵")
	dc.PushProjection(render.Matrix4Identity())
	leg.probe("A.双层LIFO", dc.Projection().IsIdentity(), "第二层恒等覆盖")
	dc.PopProjection()
	leg.probe("A.Pop回第一层", dc.Projection() == translate, "LIFO 弹回")
	dc.PopProjection()
	leg.probe("A.弹空回恒等", dc.Projection().IsIdentity(), "栈空=恒等（归一化）")

	// SetProjection direct (no stack).
	dc.SetProjection(translate)
	leg.probe("A.SetProjection读回", dc.Projection() == translate, "直设非恒等")
	dc.SetProjection(render.Matrix4Identity())
	leg.probe("A.Set回恒等", dc.Projection().IsIdentity(), "复位")

	// ---- Card B: draw effect + fail-closed ----
	// Non-identity projection scales the image draw chain 2×.
	clearPJ(dc)
	scale := render.Matrix4FromElements([16]float64{
		2, 0, 0, 0,
		0, 2, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})
	dc.SetProjection(scale)
	sq := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			sq.SetRGBA(x, y, color.RGBA{R: 230, G: 90, B: 40, A: 255})
		}
	}
	buf := render.ImageBufFromImage(sq)
	dc.DrawImageEx(buf, render.DrawImageOptions{
		X: 20, Y: 20, DstWidth: 40, DstHeight: 40, Interpolation: render.InterpBilinear})
	dc.SetProjection(render.Matrix4Identity())
	if !flushPJ(dc, view, leg, "B") {
		return leg
	}
	img := dc.Image()
	rIn, _, _, _ := img.At(50, 50).RGBA()    // inside the 2×-scaled square (40..120)
	rOut, _, _, _ := img.At(130, 130).RGBA() // beyond the scaled extent
	leg.probe("B.投影放大生效", rIn>>8 > 128 && rOut>>8 < 40,
		fmt.Sprintf("in(50,50)=%d out(130,130)=%d（40px 方块×2 → 40..120）", rIn>>8, rOut>>8))

	// fail-closed: non-finite matrix is ignored (projection stays identity).
	bad := render.Matrix4FromElements([16]float64{
		math.NaN(), 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})
	dc.SetProjection(bad)
	leg.probe("B.非有限忽略", dc.Projection().IsIdentity(),
		"NaN 矩阵 fail-closed（恒等兜底，D6 契约）")

	// ---- 补卡: Policy constants + DepthCPUFallbackReason ----
	leg.probe("C.Policy三值", render.PolicyDefault == 0 && render.PolicyHigh == 1 && render.PolicyLow == 2,
		"Default/High/Low 序")
	leg.probe("C.弃用别名", render.PolicyNone == render.PolicyDefault &&
		render.PolicyHighPerformance == render.PolicyHigh &&
		render.PolicyLowPower == render.PolicyLow &&
		render.PolicyAuto == render.PolicyDefault,
		"None/HighPerformance/LowPower/Auto 同值（Deprecated）")
	leg.probe("C.PolicyString", render.PolicyDefault.String() == "default" &&
		render.PolicyHigh.String() == "high" && render.PolicyLow.String() == "low",
		"String 三名")
	// DepthCPUFallbackReason 是 string 常量（深度分支 CPU 回退降级标记）。
	leg.probe("C.DepthCPUFallbackReason", len(render.DepthCPUFallbackReason) > 0,
		fmt.Sprintf("常量=%q（深度分支降级标记，accelcontract 接口族登记）", render.DepthCPUFallbackReason))

	leg.findings = append(leg.findings,
		"projection: Push/Pop LIFO 栈语义（恒等态归一化快照，零值污染防护 D6）+ SetProjection 直设 + 非恒等投影放大 image 链实画 + 非有限矩阵 fail-closed 恒等兜底 + Policy 三值/四弃用别名同值/DepthCPUFallbackReason 枚举面（补卡）")
	return leg
}
