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
	"image"
	"math"
)

// DrawVertices draws a triangle mesh with optional per-vertex colors.
//
// positions are in user space and transformed by the current CTM.
// When len(colors) == len(positions), Gouraud shading is used; otherwise the current
// fill solid color is used for the mesh.
//
// Preferred path: GPU convex tier with per-vertex colors (QueueColoredMesh).
// CPU fallback is true Gouraud.
func (c *Context) DrawVertices(positions []Point, colors []RGBA, mode VertexMode) {
	if c == nil || len(positions) < 3 {
		return
	}

	ctm := c.totalMatrix()
	n := len(positions)
	if cap(c.vertDevScratch) < n {
		c.vertDevScratch = make([]Point, n)
	} else {
		c.vertDevScratch = c.vertDevScratch[:n]
	}
	dev := c.vertDevScratch
	for i, p := range positions {
		dev[i] = ctm.TransformPoint(p)
	}

	solid, _ := solidColorFromPaint(c.paint)
	useVC := len(colors) == len(positions)
	meshColors := colors
	if !useVC {
		meshColors = nil
	}

	if rc := c.gpuCtxOps(); rc != nil && !CPUOnlyMode() {
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		triangleList := mode != VertexModeTriangleFan
		if meshColors == nil {
			if cap(c.meshSolidScratch) < n {
				c.meshSolidScratch = make([]RGBA, n)
			} else {
				c.meshSolidScratch = c.meshSolidScratch[:n]
			}
			solidColors := c.meshSolidScratch
			for i := range solidColors {
				solidColors[i] = solid
			}
			rc.QueueColoredMesh(target, dev, solidColors, triangleList)
		} else {
			rc.QueueColoredMesh(target, dev, meshColors, triangleList)
		}
		c.recordGPUOp()
		// Device-space AABB → layer damage so PopLayer can damage-flush
		// instead of full-surface (advanced blend mesh layers).
		c.trackDamageDevicePoints(dev)
		return
	}

	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason(VertCPUFallbackReason)
	}
	// CPU path needs its own copy if scratch will be reused later in the frame.
	devCopy := append([]Point(nil), dev...)
	c.drawVerticesCPU(devCopy, meshColors, solid, mode)
}

func (c *Context) drawVerticesCPU(positions []Point, colors []RGBA, solid RGBA, mode VertexMode) {
	// true Gouraud goes to the new per-pixel path; solid keeps AA Fill.
	if len(colors) == len(positions) {
		if uni, ok := uniformVertColor(colors); ok {
			// Identical vertex colors interpolate to a constant: keep the
			// original AA Fill path so solid arrows stay pixel-identical
			// (CPU keeps AA here; GPU mesh is SkipAA — allowed AA-edge-only diff).
			solid = uni
		} else {
			c.drawVerticesGouraudCPU(positions, colors, mode)
			return
		}
	}
	emit := func(i0, i1, i2 int) {
		col := solid
		c.SetRGBA(col.R, col.G, col.B, col.A)
		c.drawDeviceTriangle(positions[i0], positions[i1], positions[i2])
	}
	if mode == VertexModeTriangleFan {
		for i := 1; i+1 < len(positions); i++ {
			emit(0, i, i+1)
		}
		return
	}
	for i := 0; i+2 < len(positions); i += 3 {
		emit(i, i+1, i+2)
	}
}

// drawVerticesGouraudCPU rasterizes device-space triangles with true Gouraud.
// New function: old Draw signatures stay, new logic lives here.
// Premultiplied barycentric matches GPU convex mesh (SkipAA, Normal source-over).
// drawVerticesGouraudCPU rasterizes device-space triangles with true Gouraud.
// New function: old Draw signatures stay, new logic lives here.
// Premultiplied barycentric matches GPU convex mesh (SkipAA, Normal source-over).
func (c *Context) drawVerticesGouraudCPU(dev []Point, colors []RGBA, mode VertexMode) {
	if c == nil || c.pixmap == nil || len(dev) < 3 || len(colors) != len(dev) {
		return
	}
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		c.flushGPUAccelerator()
	}
	c.noteLayerCPUDraw()
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return
	}
	hasClip := c.clipStack != nil && c.clipStack.Depth() > 0
	mask := c.mask
	// Frame damage: union AABB in device space.
	minX, minY := dev[0].X, dev[0].Y
	maxX, maxY := minX, minY
	for i := 1; i < len(dev); i++ {
		if dev[i].X < minX {
			minX = dev[i].X
		} else if dev[i].X > maxX {
			maxX = dev[i].X
		}
		if dev[i].Y < minY {
			minY = dev[i].Y
		} else if dev[i].Y > maxY {
			maxY = dev[i].Y
		}
	}
	c.trackDamage(image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1))
	emitTri := func(i0, i1, i2 int) {
		p0, p1, p2 := dev[i0], dev[i1], dev[i2]
		if !isFinitePt(p0) || !isFinitePt(p1) || !isFinitePt(p2) {
			return
		}
		c0, c1, c2 := colors[i0], colors[i1], colors[i2]
		if !isFiniteRGBA(c0) || !isFiniteRGBA(c1) || !isFiniteRGBA(c2) {
			return
		}
		tri := newVertTri(p0, p1, p2)
		if tri.degen {
			return
		}
		pr0, pg0, pb0, pa0 := c0.R*c0.A, c0.G*c0.A, c0.B*c0.A, c0.A
		pr1, pg1, pb1, pa1 := c1.R*c1.A, c1.G*c1.A, c1.B*c1.A, c1.A
		pr2, pg2, pb2, pa2 := c2.R*c2.A, c2.G*c2.A, c2.B*c2.A, c2.A
		tMinX, tMinY, tMaxX, tMaxY := tri.bounds()
		x0 := int(math.Floor(tMinX))
		y0 := int(math.Floor(tMinY))
		x1 := int(math.Ceil(tMaxX))
		y1 := int(math.Ceil(tMaxY))
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		if x1 > pw {
			x1 = pw
		}
		if y1 > ph {
			y1 = ph
		}
		if x0 >= x1 || y0 >= y1 {
			return
		}
		for y := y0; y < y1; y++ {
			py := float64(y) + 0.5
			for x := x0; x < x1; x++ {
				px := float64(x) + 0.5
				w0, w1, w2, ok := tri.bary(px, py)
				if !ok {
					continue
				}
				pa := w0*pa0 + w1*pa1 + w2*pa2
				if pa <= 0 {
					continue
				}
				pr := w0*pr0 + w1*pr1 + w2*pr2
				pg := w0*pg0 + w1*pg1 + w2*pg2
				pb := w0*pb0 + w1*pb1 + w2*pb2
				k := 1.0
				if hasClip {
					cover := float64(c.clipStack.Coverage(px, py)) / 255.0
					if cover <= 0 {
						continue
					}
					k *= cover
				}
				if mask != nil {
					mc := float64(mask.At(x, y)) / 255.0
					if mc <= 0 {
						continue
					}
					k *= mc
				}
				srcA := pa * k
				if srcA <= 0 {
					continue
				}
				srcR := pr * k
				srcG := pg * k
				srcB := pb * k
				if srcA >= 1 {
					if srcR < 0 {
						srcR = 0
					}
					if srcG < 0 {
						srcG = 0
					}
					if srcB < 0 {
						srcB = 0
					}
					if srcR > 1 {
						srcR = 1
					}
					if srcG > 1 {
						srcG = 1
					}
					if srcB > 1 {
						srcB = 1
					}
					c.pixmap.SetPixelPremul(x, y, uint8(srcR*255+0.5), uint8(srcG*255+0.5), uint8(srcB*255+0.5), 255)
					continue
				}
				dstR, dstG, dstB, dstA := c.pixmap.getPremul(x, y)
				inv := 1 - srcA
				c.pixmap.setPremul(x, y,
					srcR+dstR*inv,
					srcG+dstG*inv,
					srcB+dstB*inv,
					srcA+dstA*inv,
				)
			}
		}
	}
	if mode == VertexModeTriangleFan {
		for i := 1; i+1 < len(dev); i++ {
			emitTri(0, i, i+1)
		}
		return
	}
	for i := 0; i+2 < len(dev); i += 3 {
		emitTri(i, i+1, i+2)
	}
}

// vertTri holds one triangle with hoisted barycentric factors.
type vertTri struct {
	x2, y2 float64
	a0, b0 float64
	a1, b1 float64
	den    float64
	degen  bool
	minX   float64
	minY   float64
	maxX   float64
	maxY   float64
}

func newVertTri(p0, p1, p2 Point) vertTri {
	den := (p1.Y-p2.Y)*(p0.X-p2.X) + (p2.X-p1.X)*(p0.Y-p2.Y)
	t := vertTri{
		x2: p2.X, y2: p2.Y,
		a0: p1.Y - p2.Y, b0: p2.X - p1.X,
		a1: p2.Y - p0.Y, b1: p0.X - p2.X,
		den:   den,
		degen: math.Abs(den) < 1e-12,
		minX:  math.Min(math.Min(p0.X, p1.X), p2.X),
		minY:  math.Min(math.Min(p0.Y, p1.Y), p2.Y),
		maxX:  math.Max(math.Max(p0.X, p1.X), p2.X),
		maxY:  math.Max(math.Max(p0.Y, p1.Y), p2.Y),
	}
	return t
}

func (t *vertTri) bary(px, py float64) (float64, float64, float64, bool) {
	if t == nil || t.degen {
		return 0, 0, 0, false
	}
	dx := px - t.x2
	dy := py - t.y2
	w0 := (t.a0*dx + t.b0*dy) / t.den
	w1 := (t.a1*dx + t.b1*dy) / t.den
	w2 := 1 - w0 - w1
	const eps = -1e-9
	if w0 < eps || w1 < eps || w2 < eps {
		return 0, 0, 0, false
	}
	return w0, w1, w2, true
}

func (t *vertTri) bounds() (float64, float64, float64, float64) {
	return t.minX, t.minY, t.maxX, t.maxY
}

func isFiniteRGBA(c RGBA) bool {
	return !math.IsNaN(c.R) && !math.IsNaN(c.G) && !math.IsNaN(c.B) && !math.IsNaN(c.A) &&
		!math.IsInf(c.R, 0) && !math.IsInf(c.G, 0) && !math.IsInf(c.B, 0) && !math.IsInf(c.A, 0)
}

// uniformVertColor reports whether every vertex color is identical.
// A constant interpolant keeps the original AA Fill path: pixel-identical
// to the old average fill and within the allowed AA-edge-only GPU diff.
// uniformVertColor reports whether every vertex color is identical.
// A constant interpolant keeps the original AA Fill path: pixel-identical
// to the old average fill and within the allowed AA-edge-only GPU diff.
func uniformVertColor(colors []RGBA) (RGBA, bool) {
	if len(colors) == 0 {
		return RGBA{}, false
	}
	c0 := colors[0]
	for i := 1; i < len(colors); i++ {
		if colors[i] != c0 {
			return RGBA{}, false
		}
	}
	return c0, true
}

// DrawVerticesEx validates then draws via the frozen path, returning degradation.
// Empty (<3) skips with Skipped=true and nil error; non-finite returns ErrVertsNonFinite.
// DrawVerticesEx validates then draws via the frozen path, returning degradation.
// Empty (<3) skips with Skipped=true and nil error; non-finite returns ErrVertsNonFinite.
func (c *Context) DrawVerticesEx(positions []Point, colors []RGBA, mode VertexMode, _ VertDrawOptions) (VertDrawResult, error) {
	if c == nil || len(positions) < 3 {
		return VertDrawResult{Skipped: true}, nil
	}
	for i := range positions {
		if !isFinitePt(positions[i]) {
			return VertDrawResult{}, ErrVertsNonFinite
		}
	}
	if len(colors) == len(positions) {
		for i := range colors {
			if !isFiniteRGBA(colors[i]) {
				return VertDrawResult{}, ErrVertsNonFinite
			}
		}
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	c.DrawVertices(positions, colors, mode)
	if len(positions) < 3 {
		return VertDrawResult{Skipped: true}, nil
	}
	nTri := countVertTris(len(positions), mode)
	if useGPU {
		return VertDrawResult{Degraded: false, Triangles: nTri}, nil
	}
	return VertDrawResult{Degraded: true, Reason: VertCPUFallbackReason, Triangles: nTri}, nil
}

// DrawMeshEx validates then draws via the frozen path, returning degradation.
// DrawMeshEx validates then draws via the frozen path, returning degradation.
func (c *Context) DrawMeshEx(mesh Mesh, _ VertDrawOptions) (VertDrawResult, error) {
	if c == nil || len(mesh.Positions) < 3 {
		return VertDrawResult{Skipped: true}, nil
	}
	for i := range mesh.Positions {
		if !isFinitePt(mesh.Positions[i]) {
			return VertDrawResult{}, ErrVertsNonFinite
		}
	}
	if len(mesh.Colors) != 0 && len(mesh.Colors) != len(mesh.Positions) {
		// Mismatched colors fall back to solid, same as DrawMesh.
	} else if len(mesh.Colors) == len(mesh.Positions) {
		for i := range mesh.Colors {
			if !isFiniteRGBA(mesh.Colors[i]) {
				return VertDrawResult{}, ErrVertsNonFinite
			}
		}
	}
	if len(mesh.Indices) >= 3 {
		n := len(mesh.Indices) / 3 * 3
		for i := 0; i < n; i++ {
			if int(mesh.Indices[i]) < 0 || int(mesh.Indices[i]) >= len(mesh.Positions) {
				return VertDrawResult{}, ErrVertsBadIndex
			}
		}
		useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
		c.DrawMesh(mesh)
		return VertDrawResult{Degraded: !useGPU, Reason: mapDegradedReason(!useGPU), Triangles: n / 3}, nil
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	c.DrawMesh(mesh)
	return VertDrawResult{Degraded: !useGPU, Reason: mapDegradedReason(!useGPU), Triangles: countVertTris(len(mesh.Positions), VertexModeTriangles)}, nil
}

func mapDegradedReason(degraded bool) string {
	if degraded {
		return VertCPUFallbackReason
	}
	return ""
}

func countVertTris(n int, mode VertexMode) int {
	if n < 3 {
		return 0
	}
	if mode == VertexModeTriangleFan {
		return n - 2
	}
	return n / 3
}

// drawDeviceTriangle fills a triangle specified in device pixels by mapping
// back through the inverse CTM into user space for the existing path fill path.
// drawDeviceTriangle fills a triangle specified in device pixels by mapping
// back through the inverse CTM into user space for the existing path fill path.
func (c *Context) drawDeviceTriangle(p0, p1, p2 Point) {
	inv := c.totalMatrix().Invert()
	u0 := inv.TransformPoint(p0)
	u1 := inv.TransformPoint(p1)
	u2 := inv.TransformPoint(p2)
	c.NewSubPath()
	c.MoveTo(u0.X, u0.Y)
	c.LineTo(u1.X, u1.Y)
	c.LineTo(u2.X, u2.Y)
	c.ClosePath()
	_ = c.Fill()
}

// DrawAtlas draws multiple sub-rects from a single image.
// GPU path issues one QueueImageDraw per sprite from the shared ImageBuf.
// CPU path falls back to DrawImageEx per sprite.
// DrawMesh draws an indexed (or triangle-list) colored mesh on the GPU path when available.
// This is the V.03 subset: positions + optional vertex colors + optional indices.
// Full custom fragment shaders / cubics are deferred.
//
// when Indices is set, the GPU path keeps unique verts + DrawIndexed
// (no CPU expand). CPU fallback still expands to triangle lists.
func (c *Context) DrawMesh(mesh Mesh) {
	if c == nil || len(mesh.Positions) < 3 {
		return
	}
	positions := mesh.Positions
	colors := mesh.Colors
	hasIdx := len(mesh.Indices) >= 3

	// GPU indexed path: CTM → unique device verts + indices.
	if hasIdx {
		if rc := c.gpuCtxOps(); rc != nil && !CPUOnlyMode() {
			n := len(positions)
			ctm := c.totalMatrix()
			var dev []Point
			// identity CTM — queue user-space points without a full copy/transform.
			if ctm.IsIdentity() {
				dev = positions
			} else {
				if cap(c.vertDevScratch) < n {
					c.vertDevScratch = make([]Point, n)
				} else {
					c.vertDevScratch = c.vertDevScratch[:n]
				}
				dev = c.vertDevScratch
				for i, p := range positions {
					dev[i] = ctm.TransformPoint(p)
				}
			}
			useVC := len(colors) == len(positions)
			meshColors := colors
			if !useVC {
				solid, _ := solidColorFromPaint(c.paint)
				if cap(c.meshSolidScratch) < n {
					c.meshSolidScratch = make([]RGBA, n)
				} else {
					c.meshSolidScratch = c.meshSolidScratch[:n]
				}
				for i := range c.meshSolidScratch {
					c.meshSolidScratch[i] = solid
				}
				meshColors = c.meshSolidScratch
			}
			defer c.setGPUClipRect()()
			target := c.gpuRenderTarget()
			rc.QueueColoredMeshIndexed(target, dev, meshColors, mesh.Indices)
			c.recordGPUOp()
			c.trackDamageDevicePoints(dev)
			return
		}
		// CPU / no-GPU: expand indices then triangle list.
		n := len(mesh.Indices) / 3 * 3
		useCol := len(colors) == len(positions)
		if cap(c.meshExpPosScratch) < n {
			c.meshExpPosScratch = make([]Point, 0, n)
		} else {
			c.meshExpPosScratch = c.meshExpPosScratch[:0]
		}
		if useCol {
			if cap(c.meshExpColScratch) < n {
				c.meshExpColScratch = make([]RGBA, 0, n)
			} else {
				c.meshExpColScratch = c.meshExpColScratch[:0]
			}
		}
		for i := 0; i+2 < n; i += 3 {
			i0, i1, i2 := int(mesh.Indices[i]), int(mesh.Indices[i+1]), int(mesh.Indices[i+2])
			if i0 < 0 || i1 < 0 || i2 < 0 || i0 >= len(positions) || i1 >= len(positions) || i2 >= len(positions) {
				continue
			}
			c.meshExpPosScratch = append(c.meshExpPosScratch, positions[i0], positions[i1], positions[i2])
			if useCol {
				c.meshExpColScratch = append(c.meshExpColScratch, colors[i0], colors[i1], colors[i2])
			}
		}
		if len(c.meshExpPosScratch) < 3 {
			return
		}
		positions = c.meshExpPosScratch
		if useCol {
			colors = c.meshExpColScratch
		} else {
			colors = nil
		}
	}
	c.DrawVertices(positions, colors, VertexModeTriangles)
}
