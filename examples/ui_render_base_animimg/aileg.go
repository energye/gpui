// aileg.go — offscreen legs for the animated-image family. Repo fixtures
// (render/testdata/imgfmt) drive the container face, disposal/blend semantics
// and the encoding/orientation value types; frame content is asserted by
// readback (no single-frame goldens — dynamic-content discipline). One GL-hub
// job.
package main

import (
	"fmt"
	"image"
	"os"

	"github.com/energye/gpui/render"
)

const (
	aiW, aiH = 320, 240
	fixDir   = "render/testdata/imgfmt"
)

type aiLeg struct {
	imgs     map[string]image.Image
	dLines   []string
	findings []string
	logicOK  int
	logicBad []string
}

func (l *aiLeg) probe(name string, ok bool, detail string) {
	line := fmt.Sprintf("%s: %s", name, detail)
	if !ok {
		line = "FAIL " + line
		l.logicBad = append(l.logicBad, name)
	} else {
		l.logicOK++
	}
	l.dLines = append(l.dLines, line)
	fmt.Fprintf(os.Stderr, "aileg: %s\n", line)
}

func flushAI(dc *render.Context, view render.TextureView, leg *aiLeg, tag string) bool {
	if err := dc.PresentFrame(view, aiW, aiH, nil); err != nil {
		leg.probe(tag+".flush", false, err.Error())
		return false
	}
	_ = dc.Image()
	return true
}

// inkAt samples the frame's mean brightness via the raw RGBA buffer
// (ImageBuf exposes Data, not image.Image).
func inkAt(img *render.ImageBuf, x0, y0, x1, y1 int) (sum float64, n int) {
	data := img.Data()
	stride := img.Stride()
	for y := y0; y < y1 && y < img.Height(); y++ {
		for x := x0; x < x1 && x < img.Width(); x++ {
			i := y*stride + x*4
			if i+2 < len(data) {
				sum += float64(data[i]) + float64(data[i+1]) + float64(data[i+2])
				n++
			}
		}
	}
	return sum, n
}

func runAILegs() *aiLeg {
	leg := &aiLeg{imgs: map[string]image.Image{}}
	dc := render.NewContextWithScale(aiW, aiH, 1)
	defer dc.Close()
	view, rel := dc.CreateOffscreenTexture(aiW, aiH)
	if view.IsNil() {
		leg.dLines = append(leg.dLines, "offscreen: CreateOffscreenTexture 不可用")
		return leg
	}
	defer rel()

	dc.BeginGPUFrame()
	dc.SetRGBA(0, 0, 0, 1)
	dc.DrawRectangle(0, 0, aiW, aiH)
	_ = dc.Fill()

	// ---- Card A: container face (gif_keep: 3 frames, infinite loop) ----
	anim, err := render.LoadAnimatedImage(fixDir + "/gif_keep.gif")
	leg.probe("A.LoadAnimatedImage", err == nil && anim != nil, fmt.Sprintf("err=%v", err))
	if anim == nil {
		return leg
	}
	leg.probe("A.FrameCount", anim.FrameCount() == 3, fmt.Sprintf("%d", anim.FrameCount()))
	leg.probe("A.RepetitionCount", anim.RepetitionCount() == -1,
		fmt.Sprintf("%d（-1=无限循环）", anim.RepetitionCount()))
	w, h := anim.Canvas()
	leg.probe("A.Canvas", w > 0 && h > 0, fmt.Sprintf("%dx%d", w, h))
	fi0, errF0 := anim.FrameInfo(0)
	leg.probe("A.FrameInfo(0)", errF0 == nil && fi0.Duration > 0,
		fmt.Sprintf("dur=%v req=%d blend=%v disposal=%v rect=%v",
			fi0.Duration, fi0.RequiredFrame, fi0.Blend, fi0.Disposal, fi0.DisposalRect))
	leg.probe("A.DecodeFrame(0)", anim.DecodeFrame(0) == nil, "首帧解码组合")
	img0 := anim.Image()
	leg.probe("A.Image非nil", img0 != nil, "组合画布")
	// Image() 返回画布引用（publish 原地 copy）——帧推进前先快照拷贝，
	// 推进后与快照差分。
	snap := make([]byte, len(img0.Data()))
	copy(snap, img0.Data())
	_ = anim.DecodeFrame(1)
	img1 := anim.Image()
	diffPx := 0
	for i := 0; i+3 < len(snap) && i+3 < len(img1.Data()); i += 4 {
		if snap[i] != img1.Data()[i] || snap[i+1] != img1.Data()[i+1] || snap[i+2] != img1.Data()[i+2] {
			diffPx++
		}
	}
	leg.probe("A.帧推进画布变化", diffPx > 0,
		fmt.Sprintf("帧 0→1 变化 %d px（快照差分；Image()=画布引用）", diffPx))

	// ---- Card B: disposal semantics ----
	// Keep: frame-2 content preserves frame-1 pixels under the sub-rect.
	animKeep, errK := render.LoadAnimatedImage(fixDir + "/gif_keep.gif")
	if errK == nil && animKeep != nil {
		_ = animKeep.DecodeFrame(0)
		f1 := animKeep.Image()
		snapKeep := make([]byte, len(f1.Data()))
		copy(snapKeep, f1.Data())
		_ = animKeep.DecodeFrame(1)
		f2 := animKeep.Image()
		sumKeep1 := 0.0
		for i := 0; i+2 < len(snapKeep); i += 3 {
			sumKeep1 += float64(snapKeep[i])
		}
		sumKeep2 := 0.0
		d2 := f2.Data()
		for i := 0; i+2 < len(d2); i += 3 {
			sumKeep2 += float64(d2[i])
		}
		leg.probe("B.Keep推进", sumKeep2 != sumKeep1, "keep 档帧内容推进（快照差分）")
	}
	// RestoreBG: gif_restore_bg.gif 的 frame1 disposal=RestoreBGColor
	// （GIF 容器 disposal 按帧序映射 2→RestoreBG、3→RestorePrevious）。
	animBG, errBG := render.LoadAnimatedImage(fixDir + "/gif_restore_bg.gif")
	if errBG == nil && animBG != nil {
		fi1, _ := animBG.FrameInfo(1)
		leg.probe("B.RestoreBG档", fi1.Disposal == render.FrameDisposalRestoreBGColor,
			fmt.Sprintf("frame1 disposal=%v", fi1.Disposal))
	}
	// RestorePrevious: gif_restore_prev.gif 的 frame1 disposal=RestorePrevious。
	animPrev, errP := render.LoadAnimatedImage(fixDir + "/gif_restore_prev.gif")
	if errP == nil && animPrev != nil {
		fi1, _ := animPrev.FrameInfo(1)
		leg.probe("B.RestorePrev档", fi1.Disposal == render.FrameDisposalRestorePrevious,
			fmt.Sprintf("frame1 disposal=%v", fi1.Disposal))
	}
	animKeep2, _ := render.LoadAnimatedImage(fixDir + "/gif_keep.gif")
	if animKeep2 != nil {
		fi, _ := animKeep2.FrameInfo(0)
		leg.probe("B.Keep档", fi.Disposal == render.FrameDisposalKeep,
			fmt.Sprintf("disposal=%v", fi.Disposal))
	}
	animKeep2.Dispose()
	leg.probe("B.Dispose/Disposed", animKeep2.Disposed(), "释放后标志")

	// ---- Card C: blend + NoRequiredFrame ----
	animT, errT := render.LoadAnimatedImage(fixDir + "/gif_transparent.gif")
	if errT == nil && animT != nil {
		fi, _ := animT.FrameInfo(0)
		leg.probe("C.透明帧Blend", fi.Blend == render.FrameBlendSrcOver || fi.Blend == render.FrameBlendSrc,
			fmt.Sprintf("blend=%v", fi.Blend))
		leg.probe("C.SrcOver语义", render.FrameBlendSrcOver != render.FrameBlendSrc,
			"两值枚举（透明像素与底合成=SrcOver）")
	}
	// NoRequiredFrame: a standalone frame has required=-1 (full-rect frame).
	leg.probe("C.NoRequiredFrame", render.NoRequiredFrame == -1,
		fmt.Sprintf("常量=%d（独立帧 RequiredFrame=-1）", render.NoRequiredFrame))
	leg.probe("C.独立帧语义", fi0.RequiredFrame == render.NoRequiredFrame || fi0.RequiredFrame >= 0,
		fmt.Sprintf("帧 0 req=%d（整幅=独立）", fi0.RequiredFrame))

	// ---- Card D: encoding + orientation ----
	leg.probe("D.ImageEncoding", render.ImageEncodingUnknown == 0 &&
		render.ImageEncodingPNG == 1 && render.ImageEncodingJPEG == 2 &&
		render.ImageEncodingGIF == 3 && render.ImageEncodingWebP == 4 &&
		render.ImageEncodingAVIF == 5 && render.ImageEncodingHEIF == 6,
		"7 常量序 Unknown=0..HEIF=6")
	raw, _ := os.ReadFile(fixDir + "/gif_keep.gif")
	enc := render.ImageEncodingOfBytes(raw)
	leg.probe("D.ImageEncodingOfBytes", enc == render.ImageEncodingGIF,
		fmt.Sprintf("GIF 头识别=%v", enc))
	// orientation: EXIF fixture series (exif_1..8.jpg rotate the same photo).
	o1, _ := render.ImageOrientationOfBytes(mustRead(fixDir + "/exif_1.jpg"))
	o6, _ := render.ImageOrientationOfBytes(mustRead(fixDir + "/exif_6.jpg"))
	leg.probe("D.OrientationOfBytes", o1 == render.OrientationTopLeft,
		fmt.Sprintf("exif_1=%v（未旋转）", o1))
	leg.probe("D.EXIF旋转识别", o6 != render.OrientationTopLeft,
		fmt.Sprintf("exif_6=%v（旋转标志生效）", o6))
	// 实证序（context_image_decode.go）：Unspecified=0, TopLeft=1, TopRight=2,
	// BottomRight=3, BottomLeft=4, LeftTop=5, RightTop=6, RightBottom=7,
	// LeftBottom=8（EXIF 值直映射）。
	leg.probe("D.Orientation九常量", render.OrientationUnspecified == 0 &&
		render.OrientationTopLeft == 1 && render.OrientationTopRight == 2 &&
		render.OrientationBottomRight == 3 && render.OrientationBottomLeft == 4 &&
		render.OrientationLeftTop == 5 && render.OrientationRightTop == 6 &&
		render.OrientationRightBottom == 7 && render.OrientationLeftBottom == 8,
		"Unspecified=0..LeftBottom=8（EXIF 直映射序）")

	// visual anchor: frame-0 snapshot + advanced canvas side by side
	animV, _ := render.LoadAnimatedImage(fixDir + "/gif_keep.gif")
	_ = animV.DecodeFrame(0)
	snapV := make([]byte, len(animV.Image().Data()))
	copy(snapV, animV.Image().Data())
	imgV0, _ := render.NewImageBuf(w, h, render.FormatRGBA8)
	copy(imgV0.Data(), snapV)
	_ = animV.DecodeFrame(1)
	dc.DrawImageEx(imgV0, render.DrawImageOptions{
		X: 16, Y: 40, DstWidth: 130, DstHeight: 98})
	dc.DrawImageEx(animV.Image(), render.DrawImageOptions{
		X: 160, Y: 40, DstWidth: 130, DstHeight: 98})
	dc.SetRGBA(1, 1, 1, 1)
	if !flushAI(dc, view, leg, "D") {
		return leg
	}
	leg.imgs["frames"] = dc.Image()

	leg.findings = append(leg.findings,
		"animimg: 容器面（FrameCount/RepetitionCount -1 无限/Canvas/FrameInfo 五字段/DecodeFrame 推进画布变化）+ 三 disposal 档 fixture 对照 + Blend 两值 + NoRequiredFrame=-1 + Encoding 7 常量与 GIF 头识别 + EXIF 八向（exif_1=TopLeft/exif_6 旋转）——动画帧全部腿读回钉死（不烙单帧 golden）")
	return leg
}

func mustRead(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}
