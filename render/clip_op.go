//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"math"

	"github.com/energye/gpui/render/internal/clip"
)

// ClipOp selects how a clip region combines with the current clip (C.06 / SkClipOp).
type ClipOp int

const (
	// ClipOpIntersect keeps the intersection.
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
		c.gpuClipFillRule = FillRuleNonZero
	case ClipOpDifference:
		_ = c.clipStack.PushRectDifference(rect)
		// Difference uses mask coverage; GPU via R8 MaskAware.
		c.gpuClipPath = nil
		c.gpuClipFillRule = FillRuleNonZero
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
	// Path verbs are already baked by the user matrix at MoveTo/LineTo time, so
	// the device-space form needs only the device scale — the same conversion
	// Clip() uses. Transforming by totalMatrix here multiplied the user CTM a
	// second time and moved the clip off the geometry it was meant to keep.
	dev := c.deviceSpacePath()
	verbs, coords := convertPathToClipVerbs(dev)
	canvas := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))
	fillRule := clipFillRule(c.paint.FillRule)
	switch op {
	case ClipOpReplace:
		_ = c.clipStack.ReplacePath(canvas, verbs, coords, c.antiAlias, fillRule)
		c.gpuClipPath = dev.Clone()
		c.gpuClipFillRule = c.paint.FillRule
	case ClipOpDifference:
		_ = c.clipStack.PushPathDifference(verbs, coords, c.antiAlias, fillRule)
		c.gpuClipPath = nil
		c.gpuClipFillRule = FillRuleNonZero
	default:
		_ = c.clipStack.PushPath(verbs, coords, c.antiAlias, fillRule)
		c.gpuClipPath = dev.Clone()
		c.gpuClipFillRule = c.paint.FillRule
	}
	c.bumpClipMaskGPUGen()
	c.path.Clear()
}
