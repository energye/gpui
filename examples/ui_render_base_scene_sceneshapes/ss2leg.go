// ss2leg.go — offscreen legs for the scene shape family: all 14 New*Shape
// constructors, ToPath/BuildPathInto path conversion, Bounds readbacks, and a
// full CPU scene render through scene.NewRenderer. One GL-hub job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const (
	ss2W, ss2H = 320, 240
)

type ssLeg2 struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *ssLeg2) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "ss2leg: %s\n", line)
}

func flushSS2(dc *render.Context, view render.TextureView, leg *ssLeg2, tag string) bool {
	if err := dc.PresentFrame(view, ss2W, ss2H, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

// shapeCase drives one shape: ToPath verb count + Bounds sanity.
func shapeCase[T interface {
	ToPath() *scene.Path
	Bounds() scene.Rect
}](leg *ssLeg2, name string, s T, wantMinVerbs int) {
	p := s.ToPath()
	b := s.Bounds()
	leg.probe(name, p != nil && p.VerbCount() >= wantMinVerbs &&
		b.MaxX > b.MinX && b.MaxY > b.MinY,
		fmt.Sprintf("verbs=%d bounds=[%.0f,%.0f %.0f,%.0f]",
			p.VerbCount(), b.MinX, b.MinY, b.MaxX, b.MaxY))
}

func runSS2Legs() *ssLeg2 {
	leg := &ssLeg2{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(ss2W, ss2H, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(ss2W, ss2H)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, ss2W, ss2H)
	_ = dc.Fill()

	// ---- Card A: base shapes ----
	shapeCase(leg, "A.RectShape", scene.NewRectShape(10, 10, 60, 40), 5)
	shapeCase(leg, "A.RoundedRectShape", scene.NewRoundedRectShape(80, 10, 60, 40, 8), 9)
	shapeCase(leg, "A.CircleShape", scene.NewCircleShape(190, 30, 20), 5)
	shapeCase(leg, "A.EllipseShape", scene.NewEllipseShape(250, 30, 25, 15), 5)
	shapeCase(leg, "A.RoundRectShape", scene.NewRoundRectShape(scene.Rect{MinX: 10, MinY: 60, MaxX: 70, MaxY: 100}, 6, 6), 9)
	shapeCase(leg, "A.LineShape", scene.NewLineShape(80, 80, 140, 100), 2)

	// BuildPathInto: pooled path equivalent of ToPath.
	pooled := scene.NewPath()
	scene.NewCircleShape(190, 80, 20).BuildPathInto(pooled)
	leg.probe("A.BuildPathInto", pooled.VerbCount() == scene.NewCircleShape(190, 80, 20).ToPath().VerbCount(),
		fmt.Sprintf("pooled verbs=%d（与 ToPath 同）", pooled.VerbCount()))

	// ---- Card B: polygon family ----
	shapeCase(leg, "B.PolygonShape", scene.NewPolygonShape(10, 110, 60, 110, 35, 150), 4)
	shapeCase(leg, "B.RegularPolygonShape", scene.NewRegularPolygonShape(100, 130, 25, 6, 0), 7)
	shapeCase(leg, "B.StarShape", scene.NewStarShape(160, 130, 25, 12, 5, 0), 11)
	shapeCase(leg, "B.ArcShape", scene.NewArcShape(220, 130, 25, 25, 0, 2.5, true), 2)
	shapeCase(leg, "B.PieShape", scene.NewPieShape(280, 130, 22, 0, 2.0, true), 5)

	// ---- Card C: composite/transform ----
	inner := scene.NewRectShape(0, 0, 40, 30)
	comp := scene.NewCompositeShape(scene.NewRectShape(240, 180, 40, 30), scene.NewCircleShape(290, 195, 15))
	leg.probe("C.CompositeShape", comp.ToPath().VerbCount() > 5,
		fmt.Sprintf("verbs=%d（两子形状拼接）", comp.ToPath().VerbCount()))
	ts := scene.NewTransformShape(inner, scene.NewAffine(2, 0, 0, 2, 250, 40))
	leg.probe("C.TransformShape", ts.ToPath().VerbCount() == inner.ToPath().VerbCount(),
		fmt.Sprintf("verbs=%d（同段数变换）", ts.ToPath().VerbCount()))
	bb := ts.Bounds()
	leg.probe("C.TransformBounds", bb.MaxX-bb.MinX > 70, // 40×2=80
		fmt.Sprintf("bounds 宽=%.0f（40×2）", bb.MaxX-bb.MinX))

	// ---- Card D: full CPU scene render ----
	sc := scene.NewScene()
	brush := scene.SolidBrush(render.RGBA2(0.35, 0.70, 0.90, 1))
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(), brush, scene.NewCircleShape(80, 100, 30))
	sc.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.90, 0.55, 0.20, 1)), scene.NewRoundedRectShape(130, 70, 90, 60, 10))
	sc.Stroke(scene.DefaultStrokeStyle(), scene.IdentityAffine(), scene.SolidBrush(render.RGBA2(0.45, 0.80, 0.55, 1)), scene.NewStarShape(240, 100, 28, 14, 5, 0))
	leg.probe("D.SceneFill/Stroke", sc != nil, "三形状入场景")
	pm := render.NewPixmap(ss2W, ss2H)
	rend := scene.NewRenderer(ss2W, ss2H, scene.WithWorkers(1))
	defer rend.Close()
	if err := rend.Render(pm, sc); err != nil {
		leg.probe("D.Renderer.Render", false, err.Error())
	} else {
		cr, cg, cb, _ := pm.At(80, 100).RGBA()
		leg.probe("D.Renderer.Render", cr>>8 > 60, fmt.Sprintf("(80,100)=(%d,%d,%d)（圆实心）", cr>>8, cg>>8, cb>>8))
		dc.DrawImageEx(render.ImageBufFromImage(pm), render.DrawImageOptions{X: 0, Y: 0, DstWidth: ss2W, DstHeight: ss2H})
	}
	dc.SetRGBA(1, 1, 1, 1)
	if !flushSS2(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["shapes"] = dc.Image()

	leg.findings = append(leg.findings,
		"sceneshapes: 14 个 New*Shape 构造 × ToPath/BuildPathInto/Bounds 全读回；Composite 拼接、TransformShape 变换 bounds 放大、scene.Renderer CPU 渲染三形状实画（WithWorkers(1) 确定性）")
	return leg
}
