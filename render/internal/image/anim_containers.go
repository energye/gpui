//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"time"

	twebp "github.com/SeriousBug/webp-go-pure"
	kapng "github.com/kettek/apng"
)

// 三容器的接入策略（P3.6 批 1b，用户拍板：优先成熟纯 Go 库）：
//
//	GIF        标准库 image/gif（已是成熟实现，透明/延迟/处置语义本包只做映射）
//	APNG       github.com/kettek/apng 负责容器与帧元数据解析，合成仍走本包唯一的
//	           合成器（anim.go），三容器共用一套语义
//	动图 WebP  github.com/SeriousBug/webp-go-pure 直接给出**已合成的整画布帧**
//
// 动图 WebP 为什么必须交给库：容器规范（libwebp doc/webp-container-spec.txt）里
// Blending 位 `0`=用 alpha 合成、`1`=不合成（覆盖矩形），Disposal=`1` 是填 ANIM 的
// 背景色，非零 Loop Count 表示重复 Loop Count-1 次。本包先前手写实现把这三条都
// 记反了，是「参照帧逐字节对拍」抓到的；该库对同一批样本的输出与 Pillow/libwebp
// 参照帧零字节差异，因此把这段算法交出去，不重复造。

// ───────────────────────────── GIF ─────────────────────────────

// gifAnim adapts the stdlib's fully-decoded GIF to the animSource shape.
//
// Measured against GOROOT/src/image/gif: Delay is in 1/100 s, Disposal is the
// raw GCE value (0 unspecified, 1 keep, 2 restore background, 3 restore
// previous), transparency shows up as a palette entry whose alpha is 0
// (reader.go:396-408) rather than as an index field, and LoopCount means
// 0 = forever, -1 = play once, N = play N+1 times (reader.go:577-582).
type gifAnim struct {
	g               *gif.GIF
	infos           []FrameInfo
	bg              color.RGBA
	repetitionCount int
	w, h            int
}

func openGIFAnim(data []byte) (*AnimatedImage, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: decode GIF: %w", err)
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("%w: GIF has no frames", ErrUnsupportedFormat)
	}
	w, h := g.Config.Width, g.Config.Height
	if w <= 0 || h <= 0 {
		b := g.Image[0].Bounds()
		w, h = b.Dx(), b.Dy()
	}
	a := &gifAnim{g: g, w: w, h: h}
	switch {
	case g.LoopCount < 0:
		a.repetitionCount = 0 // 只播一次
	case g.LoopCount == 0:
		a.repetitionCount = -1 // 无限
	default:
		a.repetitionCount = g.LoopCount
	}
	if pal, ok := g.Config.ColorModel.(color.Palette); ok && int(g.BackgroundIndex) < len(pal) {
		a.bg = color.RGBAModel.Convert(pal[g.BackgroundIndex]).(color.RGBA)
	}
	canvas := image.Rect(0, 0, w, h)
	a.infos = make([]FrameInfo, len(g.Image))
	for i, fr := range g.Image {
		rect := fr.Bounds()
		disposal := FrameDisposalKeep
		if i < len(g.Disposal) {
			switch g.Disposal[i] {
			case 2:
				disposal = FrameDisposalRestoreBGColor
			case 3:
				disposal = FrameDisposalRestorePrevious
			}
		}
		a.infos[i] = FrameInfo{
			Duration: time.Duration(g.Delay[i]) * 10 * time.Millisecond,
			// GIF 的透明索引等价于「透明处留住画布」，所以用 SrcOver 表达。
			RequiredFrame: requiredFor(i, rect, canvas, FrameBlendSrcOver),
			Blend:         FrameBlendSrcOver,
			Disposal:      disposal,
			DisposalRect:  rect,
		}
	}
	return newAnimatedImage(a, w, h)
}

func (s *gifAnim) frameCount() int             { return len(s.g.Image) }
func (s *gifAnim) repetition() int             { return s.repetitionCount }
func (s *gifAnim) frameInfo(i int) FrameInfo   { return s.infos[i] }
func (s *gifAnim) backgroundColor() color.RGBA { return s.bg }

func (s *gifAnim) rawFrame(i int) (image.Image, error) {
	if i < 0 || i >= len(s.g.Image) {
		return nil, fmt.Errorf("image: GIF frame %d out of range", i)
	}
	return s.g.Image[i], nil
}

// requiredFor says which earlier frame must be composited first: a frame that
// covers the whole canvas and replaces pixels stands alone; anything else needs
// the frame before it.
func requiredFor(i int, rect, canvas image.Rectangle, blend FrameBlend) int {
	if i == 0 {
		return NoRequiredFrame
	}
	if blend == FrameBlendSrc && rect.Eq(canvas) {
		return NoRequiredFrame
	}
	return i - 1
}

// ───────────────────────────── APNG ─────────────────────────────

// apngAnim wraps kettek/apng's parsed frames. The library owns container
// parsing (fcTL field widths, fdAT sequence handling, the default-image
// nuance); compositing stays in anim.go so all three containers share one set
// of semantics.
type apngAnim struct {
	a               kapng.APNG
	infos           []FrameInfo
	repetitionCount int
	w, h            int
}

func openAPNGAnim(data []byte) (*AnimatedImage, error) {
	a, err := kapng.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("image: decode APNG: %w", err)
	}
	if len(a.Frames) < 2 {
		return nil, fmt.Errorf("image: PNG is not animated")
	}
	p := &apngAnim{a: a}
	// APNG 的 num_plays 是**总播放次数**（0=无限），和 GIF 的 LoopCount、WebP 的
	// Loop Count 那种「额外重复次数」差一次，所以这里减一。
	if a.LoopCount == 0 {
		p.repetitionCount = -1
	} else {
		p.repetitionCount = int(a.LoopCount) - 1
	}
	for _, fr := range a.Frames {
		if r := fr.XOffset + fr.Image.Bounds().Dx(); r > p.w {
			p.w = r
		}
		if b := fr.YOffset + fr.Image.Bounds().Dy(); b > p.h {
			p.h = b
		}
	}
	if p.w <= 0 || p.h <= 0 {
		return nil, fmt.Errorf("image: APNG canvas %dx%d", p.w, p.h)
	}
	canvas := image.Rect(0, 0, p.w, p.h)
	p.infos = make([]FrameInfo, len(a.Frames))
	for i, fr := range a.Frames {
		den := fr.DelayDenominator
		if den == 0 {
			den = 100 // 上游口径：分母为 0 按 100 处理（测得参照实现同为 120/100 秒）
		}
		rect := image.Rect(fr.XOffset, fr.YOffset,
			fr.XOffset+fr.Image.Bounds().Dx(), fr.YOffset+fr.Image.Bounds().Dy())
		blend := FrameBlendSrc
		if fr.BlendOp == 1 {
			blend = FrameBlendSrcOver
		}
		disposal := FrameDisposalKeep
		switch fr.DisposeOp {
		case 1:
			disposal = FrameDisposalRestoreBGColor
		case 2:
			disposal = FrameDisposalRestorePrevious
		}
		p.infos[i] = FrameInfo{
			Duration:      time.Duration(fr.DelayNumerator) * time.Second / time.Duration(den),
			RequiredFrame: requiredFor(i, rect, canvas, blend),
			Blend:         blend,
			Disposal:      disposal,
			DisposalRect:  rect,
		}
	}
	return newAnimatedImage(p, p.w, p.h)
}

func (s *apngAnim) frameCount() int           { return len(s.a.Frames) }
func (s *apngAnim) repetition() int           { return s.repetitionCount }
func (s *apngAnim) frameInfo(i int) FrameInfo { return s.infos[i] }

// backgroundColor: APNG 的 dispose-to-background 是清成透明（与 GIF 回到逻辑屏
// 底色不同，本机两个独立读端都如此）。
func (s *apngAnim) backgroundColor() color.RGBA { return color.RGBA{} }

func (s *apngAnim) rawFrame(i int) (image.Image, error) {
	if i < 0 || i >= len(s.a.Frames) {
		return nil, fmt.Errorf("image: APNG frame %d out of range", i)
	}
	f := s.a.Frames[i]
	return offsetImage(f.Image, image.Pt(f.XOffset, f.YOffset)), nil
}

// offsetImage re-anchors a decoded sub-image so its bounds say where on the
// canvas it belongs.
func offsetImage(img image.Image, min image.Point) image.Image {
	if img.Bounds().Min == min {
		return img
	}
	return &translatedImage{img: img, min: min, src: img.Bounds()}
}

type translatedImage struct {
	img image.Image
	min image.Point
	src image.Rectangle
}

func (t *translatedImage) ColorModel() color.Model { return t.img.ColorModel() }

func (t *translatedImage) Bounds() image.Rectangle {
	return image.Rectangle{Min: t.min, Max: t.min.Add(t.src.Size())}
}

func (t *translatedImage) At(x, y int) color.Color {
	return t.img.At(x-t.min.X+t.src.Min.X, y-t.min.Y+t.src.Min.Y)
}

// ───────────────────────────── 动图 WebP ─────────────────────
//
// webpAnim is a thin adapter: the library hands back fully composited canvas
// frames, so this package does not composite them a second time — the frame is
// presented as one opaque-to-the-compositor full-canvas Src paint, which keeps
// a single code path for publishing frames.
type webpAnim struct {
	an              twebp.Animation
	infos           []FrameInfo
	repetitionCount int
	w, h            int
}

func openWebPAnim(data []byte) (*AnimatedImage, error) {
	an, err := twebp.DecodeAnimation(data)
	if err != nil {
		return nil, fmt.Errorf("image: decode animated WebP: %w", err)
	}
	if len(an.Frames) == 0 {
		return nil, fmt.Errorf("image: animated WebP has no frames")
	}
	if an.Width <= 0 || an.Height <= 0 {
		return nil, fmt.Errorf("image: animated WebP canvas %dx%d", an.Width, an.Height)
	}
	p := &webpAnim{an: an, w: an.Width, h: an.Height}
	// 容器规范：Loop Count 非零表示重复 Loop Count-1 次；0=无限。
	if an.LoopCount == 0 {
		p.repetitionCount = -1
	} else {
		p.repetitionCount = int(an.LoopCount) - 1
	}
	canvas := image.Rect(0, 0, an.Width, an.Height)
	p.infos = make([]FrameInfo, len(an.Frames))
	for i, fr := range an.Frames {
		p.infos[i] = FrameInfo{
			Duration: time.Duration(fr.Duration) * time.Millisecond,
			// 库给的已是完整画布，帧不依赖调用方保留的合成历史。
			RequiredFrame: NoRequiredFrame,
			Blend:         FrameBlendSrc,
			Disposal:      FrameDisposalKeep,
			DisposalRect:  canvas,
		}
	}
	return newAnimatedImage(p, an.Width, an.Height)
}

func (s *webpAnim) frameCount() int           { return len(s.an.Frames) }
func (s *webpAnim) repetition() int           { return s.repetitionCount }
func (s *webpAnim) frameInfo(i int) FrameInfo { return s.infos[i] }
func (s *webpAnim) backgroundColor() color.RGBA {
	return color.RGBA{} // 背景色已由库在合成时填入
}

func (s *webpAnim) rawFrame(i int) (image.Image, error) {
	if i < 0 || i >= len(s.an.Frames) {
		return nil, fmt.Errorf("image: WebP frame %d out of range", i)
	}
	f := s.an.Frames[i]
	stride := s.w * 4
	if len(f.RGBA) != stride*s.h {
		return nil, fmt.Errorf("image: WebP frame %d is %d bytes, want %d", i, len(f.RGBA), stride*s.h)
	}
	// 库的字节序是 R,G,B,A 且**非预乘**（与参照帧逐字节相等已验），所以按 NRGBA
	// 读；按 image.RGBA 读会被当成预乘值，半透明画布会被再解一次预乘而画错。
	return &image.NRGBA{Pix: f.RGBA, Stride: stride, Rect: image.Rect(0, 0, s.w, s.h)}, nil
}
