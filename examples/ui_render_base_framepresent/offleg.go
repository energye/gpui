// offleg.go — card D: the offscreen direct-drive leg. Every Context.PresentFrame*
// entry point is called DIRECTLY with nil present on offscreen GPU views
// (view stays readable → Image() readback), giving pixel evidence for the
// manual-present family without the swapchain.
//
// 实证语义基线（2026-11-15 探针矩阵，见 README「损伤语义实证」）：
//   - LoadOpLoad 保留：损伤呈现后损伤外像素保留上一帧 ✓（一致）
//   - 矢量帧损伤：MSAA 全清（文档限界）✓
//   - 画笔剪刀：blit 是否被裁进损伤矩形与会话历史有关（同一调用一剪一漏）
//     → 已知洞，探针只摆在损伤矩形内的绘制（不依赖剪刀），漏剪以
//     findings 记录上报台账，不在本窗门禁内判死。
package main

import (
	"fmt"
	"image"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
)

// offLeg carries the readback images and plan-probe lines painted into card D.
type offLeg struct {
	imgV1     image.Image
	imgSingle image.Image
	imgMulti  image.Image
	imgFull   image.Image

	planLines []string // plan probes + live-call evidence lines
	findings  []string // engine findings (不判死，随证据上报)
	logicOK   int
	logicBad  []string
}

func (l *offLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.planLines = append(l.planLines, line)
}

// quadrant palette of the blit-source texture (distinct colors so each blit's
// landing spot is identifiable in the readback)
var (
	qTL = [3]float64{0.20, 0.70, 0.40}
	qTR = [3]float64{0.75, 0.25, 0.45}
	qBL = [3]float64{0.25, 0.35, 0.80}
	qBR = [3]float64{0.90, 0.60, 0.15}
)

// runOffscreenLeg drives every Context.PresentFrame* entry on offscreen views.
// Must run on the GL hub (RunGLSync) like every GPU-touching leg.
func runOffscreenLeg() *offLeg {
	leg := &offLeg{}

	// Pure damage-plan probes first (no GPU involvement): PlanFramePresent
	// is the single multi-vs-union-vs-full decision (B4/P2.4e).
	runPlanProbes(leg)

	dc := render.NewContextWithScale(offW, offH, 1)
	defer dc.Close()

	v1View, relV1 := dc.CreateOffscreenTexture(offW, offH)
	if v1View.IsNil() {
		leg.notesUnavailable()
		return leg
	}
	defer relV1()

	// v1 base frame: full flush, nil present → view readable.
	paintOffV1(dc)
	fd := dc.FrameDamage()
	leg.probe("FrameDamage", len(fd) > 0, fmt.Sprintf("rects=%d", len(fd)))
	if err := dc.PresentFrame(v1View, offW, offH, nil); err != nil {
		leg.probe("PresentFrame", false, err.Error())
		return leg
	}
	leg.probe("PresentFrame", true, "nil present → 视图可读回")
	leg.imgV1 = dc.Image()

	// blit-source texture: four distinct quadrants.
	varView, relVar := dc.CreateOffscreenTexture(offW, offH)
	if varView.IsNil() {
		leg.notesUnavailable()
		return leg
	}
	defer relVar()
	paintOffVar(dc)
	if err := dc.PresentFrame(varView, offW, offH, nil); err != nil {
		leg.probe("PresentFrame(var)", false, err.Error())
		return leg
	}

	// single-damage preserve probe: blit the TR quadrant (u[0.5,1] v[0,0.5] —
	// v=0 is the TOP edge in texture space) into the bottom-right (AWAY from
	// the damage rect L) — L and the corner must keep v1.
	dc.DrawGPUTextureWithOpacityUV(varView, 160, 100, 160, 100, 1.0, 0.5, 0, 1, 0.5)
	if err := dc.PresentFrameDamage(v1View, offW, offH, offRectL, nil); err != nil {
		leg.probe("PresentFrameDamage", false, err.Error())
		return leg
	}
	leg.imgSingle = dc.Image()
	leg.checkReadback("PresentFrameDamage", leg.imgSingle, map[string]checkPt{
		"L保留":  {76, 80, v1Base},
		"角保留":  {5, 5, v1Base},
		"TR落点": {240, 150, qTR},
	})

	// multi-damage: two quadrant blits INSIDE their rects (independent of the
	// scissor question), damage rects L+R. The mid check point (40,170) sits
	// outside every blit dst AND outside L/R — the single leg's TR blit already
	// painted the bottom-right, so (160,100) is no longer pristine.
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectL.Min.X), float64(offRectL.Min.Y),
		offRectL.Dx(), offRectL.Dy(), 1.0, 0, 0, 0.5, 0.5)
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectR.Min.X), float64(offRectR.Min.Y),
		offRectR.Dx(), offRectR.Dy(), 1.0, 0.5, 0.5, 1, 1)
	if err := dc.PresentFrameDamageRects(v1View, offW, offH,
		[]image.Rectangle{offRectL, offRectR}, nil); err != nil {
		leg.probe("PresentFrameDamageRects", false, err.Error())
		return leg
	}
	leg.imgMulti = dc.Image()
	leg.checkReadback("PresentFrameDamageRects", leg.imgMulti, map[string]checkPt{
		"L落点": {76, 80, qTL},
		"R落点": {248, 136, qBR},
	})
	// 未触点 (40,170)（不在任何 blit dst、不在 L/R）应保留 v1；实测出现会话
	// 状态相关的变暗——与 scissor-leak 同族发现，不判死，上报台账。
	if img := leg.imgMulti; img != nil {
		got := sampleAt(img, 40, 170)
		if !near3(got, v1Base) {
			leg.findings = append(leg.findings,
				fmt.Sprintf("multi-untouched: 损伤外未触点 (40,170) got=(%.0f,%.0f,%.0f) want=保留(76,87,140) — 会话状态相关，见 README/台账",
					got[0], got[1], got[2]))
		}
	}

	// scissor-leak finding: whole-view blit (source BR quadrant stretched over
	// the full surface) + damage L — outside-L pixels leak varView content when
	// the session scissor is not engaged.
	dc.DrawGPUTextureWithOpacityUV(varView, 0, 0, offW, offH, 1.0, 0.5, 0.5, 1, 1)
	if err := dc.PresentFrameDamage(v1View, offW, offH, offRectL, nil); err != nil {
		leg.findings = append(leg.findings, "scissor-leak probe flush err: "+err.Error())
	} else {
		img := dc.Image()
		got := sampleAt(img, 240, 150)
		if !near3(got, v1Base) {
			leg.findings = append(leg.findings,
				fmt.Sprintf("scissor-leak: 损伤外 (240,150) got=(%.0f,%.0f,%.0f) want=保留(76,87,140) — blit 漏剪，见 README/台账",
					got[0], got[1], got[2]))
		} else {
			leg.findings = append(leg.findings, "scissor-leak: 本轮未复现（会话状态相关）")
		}
	}

	// precomputed plan dispatch: damage_union over L only.
	plan := render.FramePresentPlan{
		Mode:  render.PresentModeDamageUnion,
		Rects: []image.Rectangle{offRectL},
		Union: offRectL,
	}
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectL.Min.X), float64(offRectL.Min.Y),
		offRectL.Dx(), offRectL.Dy(), 1.0, 0, 0, 0.5, 0.5)
	if _, err := dc.PresentFramePlan(v1View, offW, offH, plan, nil); err != nil {
		leg.probe("PresentFramePlan", false, err.Error())
	} else {
		leg.probe("PresentFramePlan", true, "union=L 直派")
	}

	// explicit full: rewrites everything (TL quadrant scaled over the surface).
	dc.DrawGPUTextureWithOpacityUV(varView, 0, 0, offW, offH, 1.0, 0, 0, 0.5, 0.5)
	if err := dc.PresentFrameFull(v1View, offW, offH, nil); err != nil {
		leg.probe("PresentFrameFull", false, err.Error())
		return leg
	}
	leg.probe("PresentFrameFull", true, "全帧重写")
	leg.imgFull = dc.Image()
	leg.checkReadback("PresentFrameFull", leg.imgFull, map[string]checkPt{
		"全帧TL":  {160, 100, qTL},
		"全帧TL2": {5, 5, qTL},
	})

	// auto: blit-only draws carry no damage bookkeeping → plan Idle.
	dc.BeginFrame()
	dc.DrawGPUTextureWithOpacityUV(varView, float64(offRectL.Min.X), float64(offRectL.Min.Y),
		offRectL.Dx(), offRectL.Dy(), 1.0, 0, 0, 0.5, 0.5)
	out, err := dc.PresentFrameAuto(v1View, offW, offH, nil)
	leg.probe("PresentFrameAuto(blit)", err == nil && out.Idle,
		fmt.Sprintf("mode=%v idle=%v err=%v", out.Mode, out.Idle, err))

	// auto with vector damage: two fill rects track damage → plan dispatches
	// multi; the MSAA vector path full-clears (documented limitation — pinned
	// here as the known behavior, see README 限界).
	dc.BeginGPUFrame()
	fillOffRect(dc, offRectL, qTL)
	fillOffRect(dc, offRectR, qBR)
	fdAuto := dc.FrameDamage()
	if len(fdAuto) == 0 {
		leg.probe("auto(vector damage)", false, "FrameDamage 空")
	} else {
		outV, errV := dc.PresentFrameAuto(v1View, offW, offH, nil)
		leg.probe("auto(vector damage)", errV == nil && !outV.Idle,
			fmt.Sprintf("damage=%d → mode=%v err=%v", len(fdAuto), outV.Mode, errV))
		img := dc.Image()
		// MSAA path clears the whole target: the untouched mid must be black.
		leg.checkReadback("vector_msaa_clear", img, map[string]checkPt{
			"L落点": {76, 80, qTL},
			"清屏":  {160, 100, [3]float64{0, 0, 0}},
		})
	}
	return leg
}

func (l *offLeg) notesUnavailable() {
	l.planLines = append(l.planLines, "offscreen: CreateOffscreenTexture 不可用")
}

type checkPt struct {
	x, y int
	want [3]float64
}

func sampleAt(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

// checkReadback asserts sampled points (tolerance ±6 per channel) on a readback.
func (l *offLeg) checkReadback(name string, img image.Image, pts map[string]checkPt) {
	if img == nil {
		l.probe(name, false, "readback nil")
		return
	}
	bad := ""
	for n, p := range pts {
		got := sampleAt(img, p.x, p.y)
		w := p.want
		if absf(got[0]-w[0]*255) > 6 || absf(got[1]-w[1]*255) > 6 || absf(got[2]-w[2]*255) > 6 {
			bad += fmt.Sprintf(" %s got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
				n, got[0], got[1], got[2], w[0]*255, w[1]*255, w[2]*255)
		}
	}
	if bad == "" {
		l.probe(name, true, fmt.Sprintf("%d 点读回全中", len(pts)))
	} else {
		l.probe(name, false, "读回不符:"+bad)
	}
}

func absf(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func near3(got, want [3]float64) bool {
	return absf(got[0]-want[0]*255) <= 6 && absf(got[1]-want[1]*255) <= 6 && absf(got[2]-want[2]*255) <= 6
}

// runPlanProbes pins the pure PlanFramePresent decisions (no GPU, no context).
func runPlanProbes(leg *offLeg) {
	cases := []struct {
		name  string
		rects []image.Rectangle
		want  render.PresentMode
	}{
		{"plan:空", nil, render.PresentModeIdle},
		{"plan:单小块", []image.Rectangle{image.Rect(10, 10, 60, 50)}, render.PresentModeDamageUnion},
		{"plan:远距双块", []image.Rectangle{image.Rect(0, 0, 100, 80), image.Rect(240, 120, 320, 200)},
			render.PresentModeDamageMulti},
		{"plan:近邻双块", []image.Rectangle{image.Rect(10, 10, 80, 60), image.Rect(70, 40, 140, 90)},
			render.PresentModeDamageUnion},
		{"plan:高覆盖", []image.Rectangle{image.Rect(0, 0, 300, 190)}, render.PresentModeFull},
	}
	for _, c := range cases {
		got := render.PlanFramePresent(c.rects, offW, offH)
		leg.probe(c.name, got.Mode == c.want, fmt.Sprintf("mode=%v want=%v", got.Mode, c.want))
	}
}

// paintOffV1 draws the v1 base content (vector fills track damage).
func paintOffV1(dc *render.Context) {
	dc.BeginGPUFrame()
	wrkit.DirectFillRect(dc, 0, 0, offW, offH, v1Base, 1)
	wrkit.DirectFillRect(dc, 0, 20, offW, 14, v1Bar, 1)
	wrkit.DirectFillRect(dc, 0, 168, offW, 14, v1Bar, 1)
}

// paintOffVar draws the blit-source texture: four distinct quadrants.
func paintOffVar(dc *render.Context) {
	dc.BeginGPUFrame()
	wrkit.DirectFillRect(dc, 0, 0, offW/2, offH/2, qTL, 1)
	wrkit.DirectFillRect(dc, offW/2, 0, offW/2, offH/2, qTR, 1)
	wrkit.DirectFillRect(dc, 0, offH/2, offW/2, offH/2, qBL, 1)
	wrkit.DirectFillRect(dc, offW/2, offH/2, offW/2, offH/2, qBR, 1)
}

// fillOffRect draws one vector fill (damage-tracking leg of the auto probe).
func fillOffRect(dc *render.Context, r image.Rectangle, c [3]float64) {
	wrkit.DirectFillRect(dc, float64(r.Min.X), float64(r.Min.Y), float64(r.Dx()), float64(r.Dy()), c, 1)
}
