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
	im.releaseOwned()
	im.State = ImageIdle
	im.MarkNeedsPaint()
}

// SetError marks error state and releases any owned image.
func (im *RenderImage) SetError() {
	if im == nil {
		return
	}
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
