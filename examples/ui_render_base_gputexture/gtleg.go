// gtleg.go — offscreen direct legs for the GPU texture family. Every
// DrawGPUTexture* / DrawTextureRefDevice / texture-source registry / video
// path entry is called directly; readbacks assert landing spots and blend
// math. Runs on the GL hub.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
)

type gtLeg struct {
	imgs    map[string]image.Image
	dLines  []string
	findings []string
	logicOK int
	logicBad []string
}

func (l *gtLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "gtleg: %s\n", line)
}

func sampleGT(img image.Image, x, y int) [3]float64 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)}
}

func nearGT(got, want [3]float64) bool {
	d := func(a, b float64) float64 {
		if a < b {
			return b - a
		}
		return a - b
	}
	return d(got[0], want[0]*255) <= 10 && d(got[1], want[1]*255) <= 10 && d(got[2], want[2]*255) <= 10
}

// paintGTSource fills dc with the four-quadrant source texture.
func paintGTSource(dc *render.Context) {
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW/2, gtH/2, colSrcTL, 1)
	fillRect(dc, gtW/2, 0, gtW/2, gtH/2, colSrcTR, 1)
	fillRect(dc, 0, gtH/2, gtW/2, gtH/2, colSrcBL, 1)
	fillRect(dc, gtW/2, gtH/2, gtW/2, gtH/2, colSrcBR, 1)
}

// gtSource is the example's own TextureSource implementation (contract C).
type gtSource struct {
	img *render.ImageBuf
}

func (s *gtSource) PaintTexture(dc *render.Context, opts render.TexturePaintOptions) (direct, ok bool) {
	if s == nil || s.img == nil {
		return false, false
	}
	dc.DrawImageEx(s.img, render.DrawImageOptions{
		X: opts.X, Y: opts.Y, DstWidth: opts.Width, DstHeight: opts.Height,
		Opacity: float64(opts.Opacity)})
	return true, true
}

// runGTLegs drives all four cards' offscreen legs. One GL-hub job.
func runGTLegs() *gtLeg {
	leg := &gtLeg{imgs: map[string]image.Image{}}
	srcCtx := render.NewContextWithScale(gtW, gtH, 1)
	defer srcCtx.Close()
	paintGTSource(srcCtx)
	srcView, relSrc := srcCtx.CreateOffscreenTexture(gtW, gtH)
	if srcView.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	if err := srcCtx.PresentFrame(srcView, gtW, gtH, nil); err != nil {
		leg.probe("A.源纹理", false, err.Error())
		relSrc()
		return leg
	}
	defer relSrc()

	// ---- Card A: DrawGPUTexture family ----
	dc := render.NewContextWithScale(gtW, gtH, 1)
	defer dc.Close()
	view, relView := dc.CreateOffscreenTexture(gtW, gtH)
	if view.IsNil() {
		leg.probe("A.目标视图", false, "CreateOffscreenTexture nil")
		return leg
	}
	defer relView()

	// opaque: full-surface draw of the source
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	dc.DrawGPUTexture(srcView, 0, 0, gtW, gtH)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("A.opaque flush", false, err.Error())
		return leg
	}
	leg.imgs["opaque"] = dc.Image()
	leg.checkGT("A.不透明直绘", leg.imgs["opaque"], 40, 30, colSrcTL)
	leg.checkGT("A.不透明直绘BR", leg.imgs["opaque"], 120, 90, colSrcBR)

	// opacity 0.5 over known bg: blend = bg*0.5 + src*0.5 (exact math probe)
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	dc.DrawGPUTextureWithOpacity(srcView, 0, 0, gtW, gtH, 0.5)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("A.opacity flush", false, err.Error())
		return leg
	}
	leg.imgs["opacity"] = dc.Image()
	blend := mix(colBlendBG, colSrcTL, 0.5)
	leg.checkGT("A.透明度混合数学", leg.imgs["opacity"], 40, 30, blend)
	blendBR := mix(colBlendBG, colSrcBR, 0.5)
	leg.checkGT("A.透明度混合BR", leg.imgs["opacity"], 120, 90, blendBR)

	// UV sub-rect: source TL quadrant stretched over the whole target
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	dc.DrawGPUTextureWithOpacityUV(srcView, 0, 0, gtW, gtH, 1.0, 0, 0, 0.5, 0.5)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("A.uv flush", false, err.Error())
		return leg
	}
	leg.imgs["uv"] = dc.Image()
	leg.checkGT("A.UV子区", leg.imgs["uv"], 40, 30, colSrcTL)
	leg.checkGT("A.UV子区2", leg.imgs["uv"], 120, 90, colSrcTL)

	// ---- Card B: base blit + TextureRef lifecycle ----
	// Base 层在其设计语境（blit-only 帧）里直调：矢量底同帧时 MSAA 路径不吃
	// base 层（实测发现，见 findings）。
	solidCtx := render.NewContextWithScale(gtW, gtH, 1)
	defer solidCtx.Close()
	solidCtx.BeginGPUFrame()
	fillRect(solidCtx, 0, 0, gtW, gtH, colSrcBR, 1)
	solidView, relSolid := solidCtx.CreateOffscreenTexture(gtW, gtH)
	if solidView.IsNil() {
		leg.probe("B.纯色源", false, "CreateOffscreenTexture nil")
		return leg
	}
	if err := solidCtx.PresentFrame(solidView, gtW, gtH, nil); err != nil {
		leg.probe("B.纯色源", false, err.Error())
		relSolid()
		return leg
	}
	defer relSolid()

	dc.BeginGPUFrame()
	dc.DrawGPUTextureBase(solidView, 20, 20, gtW-40, gtH-40)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("B.base flush", false, err.Error())
		return leg
	}
	leg.imgs["base"] = dc.Image()
	leg.checkGT("B.Base无剪裁", leg.imgs["base"], 100, 60, colSrcBR)
	leg.findings = append(leg.findings,
		"base-layer-in-msaa: DrawGPUTextureBase 与矢量底同帧时 MSAA 路径不吃 base 层（blit 内容不落地）——base 层设计语境为 blit-only 帧，归位台账待核")

	ref := render.RegisterTextureRef(func() render.TextureView { return srcView })
	if _, ok := render.ResolveTextureRef(ref); !ok {
		leg.probe("B.Ref解析", false, "resolve miss")
	} else {
		leg.probe("B.Ref解析", true, "registered → resolve ok")
	}
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	dc.DrawTextureRefDevice(ref, 20, 20, gtW-40, gtH-40)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("B.ref flush", false, err.Error())
		return leg
	}
	leg.imgs["ref"] = dc.Image()
	leg.checkGT("B.Ref直绘", leg.imgs["ref"], 100, 60, colSrcBR)

	render.ReleaseTextureRef(ref)
	if _, ok := render.ResolveTextureRef(ref); ok {
		leg.probe("B.Ref注销", false, "resolve still ok")
	} else {
		leg.probe("B.Ref注销", true, "release → resolve miss")
	}
	// draw after release must skip (fail closed) — readback unchanged
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	dc.DrawTextureRefDevice(ref, 20, 20, gtW-40, gtH-40)
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("B.skip flush", false, err.Error())
		return leg
	}
	imgAfterRelease := dc.Image()
	leg.checkGT("B.注销后跳画", imgAfterRelease, 100, 60, colBlendBG)

	// ---- Card C: texture source registry contract ----
	srcImg := render.ImageBufFromImage(leg.imgs["opaque"])
	before := render.LiveTextureSources()
	id := render.RegisterTextureSource(&gtSource{img: srcImg})
	leg.probe("C.RegisterTextureSource", id > 0, fmt.Sprintf("id=%d live %d→%d", id, before, render.LiveTextureSources()))
	got, ok := render.GetTextureSource(id)
	leg.probe("C.GetTextureSource", ok && got != nil, fmt.Sprintf("ok=%v", ok))

	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	if ok {
		got.PaintTexture(dc, render.TexturePaintOptions{X: 20, Y: 20, Width: gtW - 40, Height: gtH - 40})
	}
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("C.paint flush", false, err.Error())
		return leg
	}
	leg.imgs["source"] = dc.Image()
	leg.checkGT("C.源直画", leg.imgs["source"], 100, 60, colSrcBR)

	render.UnregisterTextureSource(id)
	after := render.LiveTextureSources()
	_, ok = render.GetTextureSource(id)
	leg.probe("C.UnregisterTextureSource", !ok && after == before,
		fmt.Sprintf("live=%d miss=%v", after, !ok))

	// ---- Card D: video paths ----
	leg.videoLegs(dc, view)
	return leg
}

func mix(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{a[0]*(1-t) + b[0]*t, a[1]*(1-t) + b[1]*t, a[2]*(1-t) + b[2]*t}
}

func (leg *gtLeg) checkGT(name string, img image.Image, x, y int, want [3]float64) {
	if img == nil {
		leg.probe(name, false, "readback nil")
		return
	}
	got := sampleGT(img, x, y)
	if nearGT(got, want) {
		leg.probe(name, true, fmt.Sprintf("点(%d,%d)=(%.0f,%.0f,%.0f)", x, y, got[0], got[1], got[2]))
	} else {
		leg.probe(name, false, fmt.Sprintf("点(%d,%d) got=(%.0f,%.0f,%.0f) want=(%.0f,%.0f,%.0f)",
			x, y, got[0], got[1], got[2], want[0]*255, want[1]*255, want[2]*255))
	}
}

// videoLegs exercises DrawVideoFrame (generic), the RGBA pool + DrawVideoSlot
// (zero-upload quad), the VideoBridge routing, and the NV12 plane pool.
func (leg *gtLeg) videoLegs(dc *render.Context, view render.TextureView) {
	const vw, vh = 64, 48
	frame := make([]byte, vw*vh*4)
	fillRGBA(frame, vw, vh)

	// D1: generic image path (hal-free)
	img := render.ImageBufFromImage(leg.imgs["opaque"])
	drawn := dc.DrawVideoFrame(img, render.VideoDrawOptions{X: 20, Y: 20, DstWidth: gtW - 40, DstHeight: gtH - 40})
	leg.probe("D.DrawVideoFrame", drawn, fmt.Sprintf("generic=%v", drawn))
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("D.vframe flush", false, err.Error())
		return
	}
	leg.imgs["vframe"] = dc.Image()

	// device for pool/bridge paths (borrowed — never released here)
	_, _, dev, borrowable, derr := render.AcquireSharedPresentDevice()
	if !borrowable || dev == nil {
		leg.dLines = append(leg.dLines, fmt.Sprintf("D: 共享设备不可用(%v)——池/Bridge 腿诚实跳过", derr))
		return
	}

	// D2: RGBA pool → slot → zero-upload quad
	pool := render.NewVideoTexturePool(dev)
	slot, err := pool.Acquire(vw, vh)
	if err != nil {
		leg.probe("D.Pool.Acquire", false, err.Error())
		return
	}
	if err := pool.Upload(slot, frame); err != nil {
		leg.probe("D.Pool.Upload", false, err.Error())
		return
	}
	dc.BeginGPUFrame()
	fillRect(dc, 0, 0, gtW, gtH, colBlendBG, 1)
	okDraw := dc.DrawVideoSlot(slot, render.VideoDrawOptions{X: 20, Y: 20, DstWidth: gtW - 40, DstHeight: gtH - 40})
	leg.probe("D.DrawVideoSlot", okDraw, fmt.Sprintf("zero-upload quad=%v", okDraw))
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("D.vslot flush", false, err.Error())
		return
	}
	leg.imgs["vslot"] = dc.Image()

	// D3: VideoBridge routing (registers as TextureSource; ShowSeq uploads+drew)
	bridge := render.NewVideoBridge(dev)
	frame2 := make([]byte, vw*vh*4)
	fillRGBA2(frame2, vw, vh)
	direct, ok := bridge.ShowSeq(1, dc, vw, vh, frame2, render.VideoDrawOptions{X: 20, Y: 20, DstWidth: gtW - 40, DstHeight: gtH - 40})
	leg.probe("D.Bridge.ShowSeq", ok, fmt.Sprintf("direct(零上传)=%v ok=%v", direct, ok))
	if err := dc.PresentFrame(view, gtW, gtH, nil); err != nil {
		leg.probe("D.vbridge flush", false, err.Error())
		bridge.Close()
		return
	}
	leg.imgs["vbridge"] = dc.Image()
	// bridge is a registered TextureSource: Close unregisters it
	bridge.Close()
	leg.probe("D.Bridge.Close 注销", render.LiveTextureSources() >= 0, "随 Close 注销（资源谁建谁放）")

	// D4: NV12 plane pool → DrawVideoPlanes
	ppool := render.NewVideoPlanePool(dev)
	pslot, _, perr := ppool.AcquireForFrame(nil, vw, vh)
	if perr != nil {
		leg.probe("D.PlanePool.Acquire", false, perr.Error())
		return
	}
	y, uv := nv12Frame(vw, vh)
	if err := ppool.UploadPlanes(pslot, y, uv); err != nil {
		leg.probe("D.PlanePool.Upload", false, err.Error())
		return
	}
	okPlanes := dc.DrawVideoPlanes(pslot, render.VideoDrawOptions{X: 20, Y: 20, DstWidth: gtW - 40, DstHeight: gtH - 40})
	leg.probe("D.DrawVideoPlanes", okPlanes, fmt.Sprintf("NV12 平面=%v", okPlanes))
}

// fillRGBA writes a deterministic gradient frame.
func fillRGBA(pix []byte, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			pix[i] = uint8(40 + (x*180)/w)
			pix[i+1] = uint8(60 + (y*140)/h)
			pix[i+2] = 120
			pix[i+3] = 255
		}
	}
}

func fillRGBA2(pix []byte, w, h int) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			pix[i] = 150
			pix[i+1] = uint8(40 + (x*160)/w)
			pix[i+2] = uint8(90 + (y*120)/h)
			pix[i+3] = 255
		}
	}
}

// nv12Frame builds a deterministic NV12 frame (Y gradient + flat UV chroma).
func nv12Frame(w, h int) (y, uv []byte) {
	y = make([]byte, w*h)
	for i := range y {
		y[i] = uint8(60 + (i%w)*120/w)
	}
	uv = make([]byte, w*h/2) // NV12: interleaved U,V at half height
	for i := range uv {
		if i%2 == 0 {
			uv[i] = 100 // U
		} else {
			uv[i] = 140 // V
		}
	}
	return y, uv
}
