package painting

import (
	"github.com/energye/gpui/render"
)

// PushClipRect clips subsequent draws to a local logical rect (Y-down).
// Pair with PopClip. Uses render.Push + ClipRect.
func (c *Context) PushClipRect(x, y, w, h float64) {
	if c == nil || c.DC == nil || w <= 0 || h <= 0 {
		return
	}
	c.DC.Push()
	c.DC.ClipRect(c.OriginX+x, c.OriginY+y, w, h)
}

// PopClip restores clip/transform state from PushClipRect.
func (c *Context) PopClip() {
	if c == nil || c.DC == nil {
		return
	}
	c.DC.Pop()
}

// DrawText draws s at local logical (x,y) baseline-ish top-left via render.DrawString.
func (c *Context) DrawText(s string, x, y float64) {
	if c == nil || c.DC == nil || s == "" {
		return
	}
	c.DC.DrawString(s, c.OriginX+x, c.OriginY+y)
}

// DrawImageBuf draws a render.ImageBuf at local logical (x,y) with optional dest size.
// If dstW/dstH <= 0, uses intrinsic image size.
func (c *Context) DrawImageBuf(img *render.ImageBuf, x, y, dstW, dstH float64) {
	if c == nil || c.DC == nil || img == nil {
		return
	}
	ax, ay := c.OriginX+x, c.OriginY+y
	if dstW <= 0 || dstH <= 0 {
		c.DC.DrawImage(img, ax, ay)
		return
	}
	c.DC.DrawImageEx(img, render.DrawImageOptions{
		X: ax, Y: ay,
		DstWidth:  dstW,
		DstHeight: dstH,
	})
}

// SaveLayerBudget limits expensive saveLayer-style ops (F16 starter).
type SaveLayerBudget struct {
	// MaxOps is max PushLayer calls per frame (0 = unlimited).
	MaxOps int
	// MaxArea is max total logical area (0 = unlimited).
	MaxArea float64

	ops  int
	area float64
}

// Reset clears per-frame counters.
func (b *SaveLayerBudget) Reset() {
	if b == nil {
		return
	}
	b.ops = 0
	b.area = 0
}

// Allow reports whether another saveLayer of the given logical area is within budget.
func (b *SaveLayerBudget) Allow(w, h float64) bool {
	if b == nil {
		return true
	}
	a := w * h
	if b.MaxOps > 0 && b.ops+1 > b.MaxOps {
		return false
	}
	if b.MaxArea > 0 && b.area+a > b.MaxArea {
		return false
	}
	b.ops++
	b.area += a
	return true
}
