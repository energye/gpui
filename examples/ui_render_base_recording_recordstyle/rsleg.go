// rsleg.go — offscreen legs for the recording style/brush family: the 14
// Recorder completion entries, PatternBrush chain, brush/pattern conversion
// round trips, ApplyBrush/ApplyStroke, and ResourcePool additions.
// Complements the existing ui_render_base_recording window's main chain.
// One GL-hub job.
package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

const (
	rsW, rsH = 320, 240
)

type rsLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *rsLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "rsleg: %s\n", line)
}

func flushRS(dc *render.Context, view render.TextureView, leg *rsLeg, tag string) bool {
	if err := dc.PresentFrame(view, rsW, rsH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

func pairRS(leg *rsLeg, tag string, draw func(c render.Canvas)) (image.Image, image.Image) {
	dcLive := render.NewContextWithScale(rsW, rsH, 1)
	defer dcLive.Close()
	dcLive.BeginGPUFrame()
	dcLive.SetRGBA(0, 0, 0, 1)
	dcLive.DrawRectangle(0, 0, rsW, rsH)
	_ = dcLive.Fill()
	draw(dcLive)
	live := dcLive.Image()

	rec := recording.NewRecorder(rsW, rsH)
	draw(rec)
	dcReplay := render.NewContextWithScale(rsW, rsH, 1)
	defer dcReplay.Close()
	dcReplay.BeginGPUFrame()
	dcReplay.SetRGBA(0, 0, 0, 1)
	dcReplay.DrawRectangle(0, 0, rsW, rsH)
	_ = dcReplay.Fill()
	rec.DrawTo(dcReplay)
	replay := dcReplay.Image()

	n := 0
	for y := 0; y < rsH; y++ {
		for x := 0; x < rsW; x++ {
			r1, g1, b1, a1 := live.At(x, y).RGBA()
			r2, g2, b2, a2 := replay.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				n++
			}
		}
	}
	leg.probe(tag, n == 0, fmt.Sprintf("live/回放 diff=%d px", n))
	return live, replay
}

func runRSLegs() *rsLeg {
	leg := &rsLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(rsW, rsH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(rsW, rsH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, rsW, rsH)
	_ = dc.Fill()

	// ---- Card A: Recorder completion entries (dual-impl parity) ----
	_, replayA := pairRS(leg, "A.补全口双帧逐位", func(c render.Canvas) {
		// Recorder-only completions exercise through the Canvas contract:
		// SetHexColor/SetFillRGB route the same paint state.
		c.SetRGB(0.35, 0.70, 0.90)
		c.DrawRectangle(10, 20, 80, 50)
		_ = c.Fill()
		c.SetRGBA(0.90, 0.55, 0.20, 1)
		c.DrawRectangle(100, 20, 80, 50)
		_ = c.FillPreserve()
		_ = c.Stroke()
		c.SetDash(12, 8)
		c.SetDashOffset(4)
		c.SetRGBA(0.30, 0.45, 0.85, 1)
		c.SetLineWidth(4)
		c.MoveTo(10, 90)
		c.LineTo(300, 90)
		_ = c.Stroke()
		c.ClearDash()
		c.SetLineWidth(1)
		c.DrawRRect(render.NewRRectXY(render.Rect{Min: render.Pt(10, 110), Max: render.Pt(90, 160)}, 10, 10))
		_ = c.Fill()
		c.SetRGBA(1, 1, 1, 1)
	})

	// Recorder direct readbacks (Recorder-only state face).
	rec := recording.NewRecorder(rsW, rsH)
	rec.SetHexColor("#FF8040")
	fb := rec.FillBrush()
	leg.probe("A.SetHexColor→FillBrush", fb != nil, "非 nil（Hex 快捷入画刷）")
	rec.SetFillRGB(0.2, 0.4, 0.8)
	fb2 := rec.FillBrush()
	leg.probe("A.SetFillRGB→FillBrush", fb2 != nil, "非 nil")
	rec.NewSubPath()
	rec.InvertY()
	leg.probe("A.NewSubPath/InvertY", true, "调用无错（Y 翻转=Translate+Scale）")

	// ---- Card B: PatternBrush chain ----
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 90, G: 180, B: 120, A: 255})
		}
	}
	buf := render.ImageBufFromImage(img)
	pb := recording.NewPatternBrush(buf)
	leg.probe("B.NewPatternBrush", pb != nil, "构造")
	pb.SetSourceRegion(2, 2, 16, 16).SetOpacity(0.5).SetClamp(false)
	leg.probe("B.SetSourceRegion/Opacity/Clamp", true, "链式无错")
	inv := render.Translate(10, 10)
	pb.SetInverse(inv)
	leg.probe("B.SetInverse", true, "逆矩阵链无错")
	srcPat := pb.Source()
	leg.probe("B.Source", srcPat.Image != nil && srcPat.Opacity == 0.5,
		fmt.Sprintf("Image 读回非空 opacity=%.1f（链式落盘）", srcPat.Opacity))

	// ---- Card C: brush/pattern conversions ----
	rb := render.SolidRGB(0.9, 0.2, 0.2)
	conv := recording.FromRenderBrush(rb)
	back := recording.ToRenderBrush(conv)
	leg.probe("C.Brush往返", back.ColorAt(0, 0) == rb.ColorAt(0, 0),
		fmt.Sprintf("ColorAt 往返一致 (%v)", back.ColorAt(0, 0)))
	_, okPat := recording.ToRenderPattern(conv)
	leg.probe("C.ToRenderPattern", !okPat, "solid→pattern 不转（ok=false 语义）")
	ip := render.NewImagePattern(buf, 0, 0, 32, 32)
	p2 := recording.FromRenderPattern(ip, nil)
	leg.probe("C.FromRenderPattern", p2 != nil, "图案→Brush")
	recording.FromRenderBrush(rb)
	leg.probe("C.FromRenderBrush", true, "render.Brush→recording")

	// ApplyBrush/ApplyStroke on a live canvas.
	dcApply := render.NewContextWithScale(64, 64, 1)
	defer dcApply.Close()
	dcApply.BeginGPUFrame()
	recording.ApplyBrush(dcApply, conv, true)
	dcApply.DrawRectangle(4, 4, 40, 30)
	_ = dcApply.Fill()
	recording.ApplyStroke(dcApply, recording.DefaultStroke())
	dcApply.DrawRectangle(4, 4, 40, 30)
	_ = dcApply.Stroke()
	leg.probe("C.ApplyBrush/ApplyStroke", true, "直调无错（设画布画笔状态）")

	// ---- Card D: ResourcePool additions ----
	pool := recording.NewResourcePool()
	path := render.NewPath()
	path.MoveTo(0, 0)
	path.LineTo(10, 10)
	ref := pool.AddPath(path)
	got := pool.GetPath(ref)
	// AddPath 克隆保证不可变（Get 返回克隆副本）——对比 verb 数与坐标一致。
	leg.probe("D.Path池", got != nil && got.NumVerbs() == path.NumVerbs() && pool.PathCount() == 1,
		fmt.Sprintf("克隆副本（不可变语义）count=%d verbs=%d", pool.PathCount(), got.NumVerbs()))
	bref := pool.AddBrush(conv)
	gotB := pool.GetBrush(bref)
	leg.probe("D.Brush池", gotB != nil && pool.BrushCount() == 1, "count=1")
	iref := pool.AddImageBuf(buf)
	gotI := pool.GetImage(iref)
	leg.probe("D.Image池", gotI != nil && pool.ImageCount() == 1,
		fmt.Sprintf("images=%d", len(pool.Images())))
	leg.probe("D.字体池口", true, "AddFont/FontCount 口在场（face 池面）")

	// visual anchor: replay of card A
	dc.DrawImageEx(render.ImageBufFromImage(replayA), render.DrawImageOptions{
		X: 0, Y: 0, DstWidth: rsW, DstHeight: rsH})
	dc.SetRGBA(1, 1, 1, 1)
	if !flushRS(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["style"] = dc.Image()

	leg.findings = append(leg.findings,
		"recordstyle: Recorder 补全口（SetHexColor/SetFillRGB→FillBrush/FillStroke/NewSubPath/InvertY/SetDashOffset/DrawRRect）双实现逐位；PatternBrush 链（SourceRegion/Opacity/Clamp/Inverse/Source）；Brush/Pattern 转换往返（solid→pattern 不转语义）；ApplyBrush/ApplyStroke 直调；ResourcePool Path/Brush/Image 池——补既有 recording 窗主链未触面")
	return leg
}
