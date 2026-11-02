//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"github.com/energye/gpui/render"
)

// ImageState is the async image load state.
type ImageState int

const (
	ImageIdle ImageState = iota
	ImageLoading
	ImageReady
	ImageError
)

// RenderImage shows a placeholder while loading, then a decoded ImageBuf.
// Decoding must not run inside Layout/Paint — use ui/io helpers.
//
// Ownership (FImg-DISPOSE):
//   - SetImage takes ownership of the buffer; the previous owned buffer is Dispose'd.
//   - SetImageShared does not take ownership (caller remains responsible for Dispose).
//   - Clear / SetError / replace paths release only buffers this node owns.
//   - Paint never draws a Disposed buffer (falls back to placeholder/error chrome).
type RenderImage struct {
	Base
	Width, Height float64
	State         ImageState
	Img           *render.ImageBuf
	// ownsImg is true when Img was installed via SetImage (node will Dispose it).
	ownsImg bool
	// Placeholder color while loading.
	PR, PG, PB float64
	// anim 是本节点接管的动图解码器（nil = 静态图模式）。解码器的 Dispose 归本节点；
	// 当前帧缓冲只共享显示（buf 属于解码器，本节点不得单独释放）。
	anim *render.AnimatedImage
	// animPending 是 Paint 期要推进到的目标帧；-1 表示没有待推进帧。
	animPending int
	// animApplied 是已解码显示的帧号；装上时置 0，首个 Paint 期推进第 0 帧。
	animApplied int
	// animPaused 是外部暂停标志，只表达意图：节奏归组件层，Paint 仍会画出 pending 帧。
	animPaused bool
}

// NewRenderImage creates an image node with logical size.
func NewRenderImage(w, h float64) *RenderImage {
	im := &RenderImage{
		Width: w, Height: h,
		State:       ImageIdle,
		PR:          0.2, PG: 0.2, PB: 0.25,
		animPending: -1,
	}
	im.Init(im)
	return im
}

// SetLoading marks loading state (paint only). Does not release the current image.
func (im *RenderImage) SetLoading() {
	if im == nil {
		return
	}
	im.State = ImageLoading
	im.MarkNeedsPaint()
}

// SetImage applies a decoded buffer and **takes ownership**.
// The previous owned buffer is Dispose'd (unless it is the same pointer).
// Call on the UI thread after IO completes.
// Passing nil is equivalent to SetError (releases owned image).
// Re-SetImage with the same live buffer is a no-op ownership-wise (stays Ready, not disposed).
func (im *RenderImage) SetImage(img *render.ImageBuf) {
	if im == nil {
		return
	}
	// 动图让位：显式装静态缓冲即退出动图模式（持有解码器连带释放）。
	im.releaseAnimated()
	if img != nil && img.Disposed() {
		// Reject already-freed buffers; treat as error without adopting.
		im.releaseOwnedExcept(img)
		im.Img = nil
		im.ownsImg = false
		im.State = ImageError
		im.MarkNeedsPaint()
		return
	}
	// Same pointer: do not Dispose then re-adopt a dead buffer.
	if img != nil && img == im.Img {
		im.ownsImg = true
		im.State = ImageReady
		im.MarkNeedsPaint()
		return
	}
	im.releaseOwnedExcept(img)
	im.Img = img
	im.ownsImg = img != nil
	if img != nil {
		im.State = ImageReady
	} else {
		im.State = ImageError
	}
	im.MarkNeedsPaint()
}

// SetImageShared installs img without taking ownership (caller must Dispose).
// Any previously owned buffer is still released unless it is the same pointer
// (then ownership is only dropped; the buffer is not Dispose'd).
func (im *RenderImage) SetImageShared(img *render.ImageBuf) {
	if im == nil {
		return
	}
	// 动图让位：显式装静态缓冲即退出动图模式（持有解码器连带释放）。
	im.releaseAnimated()
	if img != nil && img.Disposed() {
		im.releaseOwnedExcept(img)
		im.Img = nil
		im.ownsImg = false
		im.State = ImageError
		im.MarkNeedsPaint()
		return
	}
	if img != nil && img == im.Img {
		im.ownsImg = false
		im.State = ImageReady
		im.MarkNeedsPaint()
		return
	}
	im.releaseOwnedExcept(img)
	im.Img = img
	im.ownsImg = false
	if img != nil {
		im.State = ImageReady
	} else {
		im.State = ImageError
	}
	im.MarkNeedsPaint()
}

// Clear releases an owned image and returns to Idle placeholder (paint only).
func (im *RenderImage) Clear() {
	if im == nil {
		return
	}
	im.releaseAnimated()
	im.releaseOwned()
	im.State = ImageIdle
	im.MarkNeedsPaint()
}

// SetError marks error state and releases any owned image.
func (im *RenderImage) SetError() {
	if im == nil {
		return
	}
	im.releaseAnimated()
	im.releaseOwned()
	im.State = ImageError
	im.MarkNeedsPaint()
}

// OwnsImage reports whether the node will Dispose the current Img on replace/clear.
func (im *RenderImage) OwnsImage() bool {
	return im != nil && im.ownsImg && im.Img != nil
}

func (im *RenderImage) releaseOwned() {
	im.releaseOwnedExcept(nil)
}

// releaseOwnedExcept Disposes the currently owned buffer unless it is keep
// (same-pointer reinstall / demote paths must not free the live buffer).
func (im *RenderImage) releaseOwnedExcept(keep *render.ImageBuf) {
	if im == nil {
		return
	}
	if im.ownsImg && im.Img != nil && im.Img != keep {
		im.Img.Dispose()
	}
	if im.Img != keep {
		im.Img = nil
		im.ownsImg = false
	}
}

// Layout implements RenderObject.
func (im *RenderImage) Layout(c Constraints) Size {
	if sz, ok := im.LayoutSkipIfClean(c); ok {
		return sz
	}
	out := c.Tighten(Size{Width: im.Width, Height: im.Height})
	im.setSize(out)
	im.RememberConstraints(c)
	im.clearLayoutDirty()
	return out
}

// Paint implements RenderObject — never decodes images here.
func (im *RenderImage) Paint(pc *PaintContext) {
	if pc == nil {
		return
	}
	if pc.CompositeOnly && !im.NeedsPaint() {
		return
	}
	pc.NotePaintVisit()
	sz := im.size
	// D19：待推进帧只在 Paint 期内解码——写像素与光栅线程读像素天然错开（先例
	// video_bridge/anim.go 同一形状）；DecodeFrame 的 publish 会 NotifyPixelsChanged
	// 换代际，图号=内容版本的消费链（批 1c）自动重传并逐出旧纹理。
	// 解码失败保留旧帧画面并停止本次推进（推进失败不等于图像坏，状态不转 Error）。
	if im.animPending >= 0 && im.anim != nil && !im.anim.Disposed() {
		if err := im.anim.DecodeFrame(im.animPending); err == nil {
			im.animApplied = im.animPending
		}
		im.animPending = -1
	}
	switch im.State {
	case ImageReady:
		if im.Img != nil && !im.Img.Disposed() {
			drawImageBuf(pc, im.Img, 0, 0, sz.Width, sz.Height)
		} else {
			// Disposed or missing buffer under Ready → error chrome (fail closed).
			fillRect(pc, 0, 0, sz.Width, sz.Height, 0.8, 0.2, 0.2, 1)
		}
	case ImageError:
		fillRect(pc, 0, 0, sz.Width, sz.Height, 0.6, 0.15, 0.15, 1)
	default:
		fillRect(pc, 0, 0, sz.Width, sz.Height, im.PR, im.PG, im.PB, 1)
	}
	im.clearPaintDirty()
}

// HitTest implements RenderObject.
func (im *RenderImage) HitTest(p Point) RenderObject {
	sz := im.size
	if p.X >= 0 && p.Y >= 0 && p.X < sz.Width && p.Y < sz.Height {
		return im
	}
	return nil
}
