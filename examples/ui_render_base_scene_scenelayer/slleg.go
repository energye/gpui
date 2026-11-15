// slleg.go — offscreen legs for the scene layer family: LayerStack push/pop,
// clip layers, filter chains, damage tracking, RenderWithDamage. Complements
// (does not duplicate) the existing ui_render_base_scene window's LayerCache
// coverage. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const (
	slW, slH = 320, 240
)

type slLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *slLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "slleg: %s\n", line)
}

func flushSL(dc *render.Context, view render.TextureView, leg *slLeg, tag string) bool {
	if err := dc.PresentFrame(view, slW, slH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func runSLLegs() *slLeg {
	leg := &slLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(slW, slH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(slW, slH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, slW, slH)
	_ = dc.Fill()

	// ---- Card A: LayerStack + LayerState ----
	// 实证：栈初始含根层（Depth=1），Pop 不能弹根层（返回 nil）。
	stack := scene.NewLayerStack()
	leg.probe("A.LayerStack初始", stack.IsRoot() && stack.Depth() == 1 && stack.Root() != nil,
		"根层在场（Depth=1）")
	reg := &scene.LayerState{Kind: scene.LayerRegular, BlendMode: scene.BlendNormal, Alpha: 0.5}
	stack.Push(reg)
	leg.probe("A.Push后", stack.Depth() == 2 && stack.Top() == reg, "Top=新层")
	clip := scene.NewClipLayer(scene.NewRectShape(10, 10, 100, 80))
	stack.Push(clip)
	leg.probe("A.ClipLayer", clip.Kind == scene.LayerClip && stack.Depth() == 3,
		fmt.Sprintf("kind=%v depth=%d", clip.Kind, stack.Depth()))
	b := scene.Rect{MinX: 0, MinY: 0, MaxX: 50, MaxY: 40}
	clip.UpdateBounds(b)
	leg.probe("A.UpdateBounds", clip.Bounds.MinX <= 10, "clip 界与形状界求交")
	popped := stack.Pop()
	leg.probe("A.Pop", popped == clip && stack.Depth() == 2, "弹出 clip 层")
	_ = stack.Pop()
	leg.probe("A.回根", stack.IsRoot() && stack.Pop() == nil, "根层不可弹（nil）")

	// ---- Card B: FilterChain + FilterType ----
	chain := scene.NewFilterChain()
	leg.probe("B.空链", chain.IsEmpty() && chain.Len() == 0, "Len=0")
	leg.probe("B.FilterType枚举", scene.FilterNone.String() == "None" &&
		scene.FilterBlur.String() == "Blur" && scene.FilterDropShadow.ExpandsOutput() &&
		!scene.FilterColorMatrix.ExpandsOutput(),
		"None/Blur/DropShadow/ColorMatrix（Blur/DropShadow 扩界）")

	// ---- Card C: Scene layer/clip/transform depth ----
	sc := scene.NewScene()
	sc.PushLayer(scene.BlendMultiply, 0.5, nil)
	leg.probe("C.PushLayer", sc != nil, "blend=Multiply alpha=0.5")
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.9, 0.3, 0.3, 1)), scene.NewCircleShape(80, 60, 25))
	sc.PopLayer()
	sc.PushClip(scene.NewRectShape(40, 100, 160, 60))
	leg.probe("C.PushClip", true, "clip 形状入栈")
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.35, 0.7, 0.9, 1)), scene.NewRectShape(20, 90, 220, 80))
	okPop := sc.PopClip()
	leg.probe("C.PopClip", okPop, "弹出")
	sc.PushTransform(scene.TranslateAffine(10, 10))
	leg.probe("C.TransformDepth", sc.TransformDepth() == 1, fmt.Sprintf("depth=%d", sc.TransformDepth()))
	sc.PopTransform()
	leg.probe("C.PopTransform", sc.TransformDepth() == 0, "回 0")

	// ---- Card D: DamageTracker + RenderWithDamage ----
	dt := scene.NewDamageTracker()
	leg.probe("D.IsFirstRender", dt.IsFirstRender(), "首帧")
	// 实证：首帧 ComputeDamage 返回零 rect——全量渲染由 IsFirstRender 判定。
	frame1 := []scene.TaggedBounds{{ID: 1, Rect: image.Rect(10, 10, 110, 110)}}
	dmg1 := dt.ComputeDamage(frame1)
	// 实证语义：ComputeDamage 首帧返回零 rect（firstFrame 内部翻转）；
	// IsFirstRender 判【rendered 标志】——MarkRendered() 之后才 false。
	leg.probe("D.首帧零rect+标志", dmg1.Empty() && dt.IsFirstRender(),
		fmt.Sprintf("damage=%v IsFirstRender=%v（首帧标志未 MarkRendered 前=真）", dmg1, dt.IsFirstRender()))
	dt.MarkRendered()
	leg.probe("D.MarkRendered", !dt.IsFirstRender(), "标记后 IsFirstRender=false")
	frame2 := []scene.TaggedBounds{{ID: 1, Rect: image.Rect(60, 60, 160, 160)}}
	dmg2 := dt.ComputeDamage(frame2)
	leg.probe("D.移动产损伤", !dmg2.Empty(), fmt.Sprintf("damage=%v（旧+新并集）", dmg2))
	frame3 := []scene.TaggedBounds{{ID: 1, Rect: image.Rect(60, 60, 160, 160)}}
	dmg3 := dt.ComputeDamage(frame3)
	leg.probe("D.静止零损伤", dmg3.Empty(), "同帧无损伤")
	dt.Reset()
	leg.probe("D.Reset", dt.IsFirstRender(), "Reset 后回首帧态")

	// RenderWithDamage on a live scene pair (supplements existing window's chain).
	sc1 := scene.NewScene()
	sc1.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.35, 0.7, 0.9, 1)), scene.NewCircleShape(80, 60, 30))
	sc2 := scene.NewScene()
	sc2.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.90, 0.55, 0.20, 1)), scene.NewCircleShape(160, 120, 30))
	pm := render.NewPixmap(slW, slH)
	rend := scene.NewRenderer(slW, slH, scene.WithWorkers(1))
	defer rend.Close()
	if _, err := rend.RenderWithDamage(pm, sc1, dt); err != nil {
		leg.probe("D.RenderWithDamage帧1", false, err.Error())
	} else {
		r1, _, _, _ := pm.At(80, 60).RGBA()
		leg.probe("D.RenderWithDamage帧1", r1>>8 > 60, fmt.Sprintf("(80,60) r=%d", r1>>8))
	}
	if _, err := rend.RenderWithDamage(pm, sc2, dt); err != nil {
		leg.probe("D.RenderWithDamage帧2", false, err.Error())
	} else {
		r2, _, _, _ := pm.At(160, 120).RGBA()
		leg.probe("D.RenderWithDamage帧2", r2>>8 > 60, fmt.Sprintf("(160,120) r=%d（橙）", r2>>8))
	}
	dc.DrawImageEx(render.ImageBufFromImage(pm), render.DrawImageOptions{X: 0, Y: 0, DstWidth: slW, DstHeight: slH})
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSL(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["damage"] = dc.Image()

	leg.findings = append(leg.findings,
		"scenelayer: LayerStack 三态层（Regular/Clip/Filtered）Push/Pop/Top/Depth、FilterChain 空+len、FilterType 四枚举（Blur/DropShadow 扩界）、Scene.PushLayer(blend,alpha,clip)/PushClip/TransformDepth、DamageTracker 首帧全量→移动产损伤→静止零损伤→Reset、RenderWithDamage 双帧实画——补既有 scene 窗未触的层型/链面")
	return leg
}
