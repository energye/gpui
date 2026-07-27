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
type RenderImage struct {
	Base
	Width, Height float64
	State         ImageState
	Img           *render.ImageBuf
	// Placeholder color while loading.
	PR, PG, PB float64
}

// NewRenderImage creates an image node with logical size.
func NewRenderImage(w, h float64) *RenderImage {
	im := &RenderImage{
		Width: w, Height: h,
		State: ImageIdle,
		PR:    0.2, PG: 0.2, PB: 0.25,
	}
	im.Init(im)
	return im
}

// SetLoading marks loading state (paint only).
func (im *RenderImage) SetLoading() {
	if im == nil {
		return
	}
	im.State = ImageLoading
	im.MarkNeedsPaint()
}

// SetImage applies a decoded buffer (call on UI thread after IO completes).
func (im *RenderImage) SetImage(img *render.ImageBuf) {
	if im == nil {
		return
	}
	im.Img = img
	if img != nil {
		im.State = ImageReady
	} else {
		im.State = ImageError
	}
	im.MarkNeedsPaint()
}

// SetError marks error state.
func (im *RenderImage) SetError() {
	if im == nil {
		return
	}
	im.State = ImageError
	im.Img = nil
	im.MarkNeedsPaint()
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
	switch im.State {
	case ImageReady:
		if im.Img != nil {
			drawImageBuf(pc, im.Img, 0, 0, sz.Width, sz.Height)
		} else {
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
