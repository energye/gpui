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
// B1: with a rotated/sheared CTM the transformed rect is a QUADRILATERAL —
// clipping to the AABB of two transformed corners would leave the four
// corner regions visible (Skia clips via the exact transformed quad). The
// axis-aligned fast path is unchanged.
func (c *Context) ClipRectOp(x, y, w, h float64, op ClipOp) {
	if c.clipStack == nil {
		c.initClipStack()
	}
	tm := c.totalMatrix()
	if tm.B != 0 || tm.C != 0 {
		// Non-axis-aligned: clip to the transformed quad as a path.
		c.deviceQuadClipPath(
			tm.TransformPoint(Pt(x, y)),
			tm.TransformPoint(Pt(x+w, y)),
			tm.TransformPoint(Pt(x+w, y+h)),
			tm.TransformPoint(Pt(x, y+h)),
			op,
		)
		return
	}
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

// deviceQuadClipPath pushes an exact clip of the transformed quadrilateral
// (corners given in device space) — the B1 non-axis-aligned branch shared by
// ClipRectOp and ClipRoundRect. Mirrors ClipPathOp's device-space handling:
// the GPU clip stores the device-space path as-is.
func (c *Context) deviceQuadClipPath(p0, p1, p2, p3 Point, op ClipOp) {
	quad := NewPath()
	quad.MoveTo(p0.X, p0.Y)
	quad.LineTo(p1.X, p1.Y)
	quad.LineTo(p2.X, p2.Y)
	quad.LineTo(p3.X, p3.Y)
	quad.Close()

	verbs, coords := convertPathToClipVerbs(quad)
	canvas := clip.NewRect(0, 0, float64(c.pixmap.Width()), float64(c.pixmap.Height()))
	switch op {
	case ClipOpReplace:
		_ = c.clipStack.ReplacePath(canvas, verbs, coords, c.antiAlias, clipFillRule(FillRuleNonZero))
		c.gpuClipPath = quad.Clone()
		c.gpuClipFillRule = FillRuleNonZero
	case ClipOpDifference:
		_ = c.clipStack.PushPathDifference(verbs, coords, c.antiAlias, clipFillRule(FillRuleNonZero))
		c.gpuClipPath = nil
		c.gpuClipFillRule = FillRuleNonZero
	default:
		_ = c.clipStack.PushPath(verbs, coords, c.antiAlias, clipFillRule(FillRuleNonZero))
		c.gpuClipPath = quad.Clone()
		c.gpuClipFillRule = FillRuleNonZero
	}
	c.bumpClipMaskGPUGen()
}
