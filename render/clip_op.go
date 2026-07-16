package render

import (
	"math"

	"github.com/energye/gpui/render/internal/clip"
)

// ClipOp selects how a clip region combines with the current clip (C.06 / SkClipOp).
type ClipOp int

const (
	// ClipOpIntersect keeps the intersection (default Skia behavior).
	ClipOpIntersect ClipOp = iota
	// ClipOpDifference subtracts the new region from the current clip.
	ClipOpDifference
	// ClipOpReplace replaces the entire clip with the new region.
	ClipOpReplace
)

// ClipRectOp clips to a rectangle using the given clip operation.
func (c *Context) ClipRectOp(x, y, w, h float64, op ClipOp) {
	if c.clipStack == nil {
		c.initClipStack()
	}
	tm := c.totalMatrix()
	p1 := tm.TransformPoint(Pt(x, y))
	p2 := tm.TransformPoint(Pt(x+w, y+h))
	devX := math.Min(p1.X, p2.X)
	devY := math.Min(p1.Y, p2.Y)
	devW := math.Abs(p2.X - p1.X)
	devH := math.Abs(p2.Y - p1.Y)
	rect := clip.NewRect(devX, devY, devW, devH)
	canvas := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))

	switch op {
	case ClipOpReplace:
		c.clipStack.ReplaceRect(canvas, rect)
		c.gpuClipPath = nil
	case ClipOpDifference:
		_ = c.clipStack.PushRectDifference(rect)
		// Difference uses mask coverage; GPU via R8 MaskAware (P1-2).
		c.gpuClipPath = nil
	default:
		c.clipStack.PushRect(rect)
	}
	c.bumpClipMaskGPUGen()
}

// ClipPathOp clips using the current path with the given operation, then clears the path.
func (c *Context) ClipPathOp(op ClipOp) {
	if c.path == nil || c.path.isEmpty() {
		return
	}
	if c.clipStack == nil {
		c.initClipStack()
	}
	// Transform path to device space.
	dev := c.path.Transform(c.totalMatrix())
	verbs, coords := convertPathToClipVerbs(dev)
	canvas := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))
	switch op {
	case ClipOpReplace:
		_ = c.clipStack.ReplacePath(canvas, verbs, coords, c.antiAlias)
		c.gpuClipPath = dev
	case ClipOpDifference:
		_ = c.clipStack.PushPathDifference(verbs, coords, c.antiAlias)
		c.gpuClipPath = nil
	default:
		_ = c.clipStack.PushPath(verbs, coords, c.antiAlias)
		c.gpuClipPath = dev
	}
	c.bumpClipMaskGPUGen()
	c.path.Clear()
}
