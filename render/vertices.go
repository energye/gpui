package render

import "image"

// VertexMode selects how DrawVertices interprets the position list (V.01).
type VertexMode int

const (
	// VertexModeTriangles groups positions as independent triangles (0,1,2), (3,4,5), ...
	VertexModeTriangles VertexMode = iota
	// VertexModeTriangleFan fans triangles from the first vertex: (0,i,i+1).
	VertexModeTriangleFan
)

// AtlasSprite describes one sub-rect of an atlas image drawn to a destination rect (V.02).
// Source coordinates are in image pixels; destination is in user space (CTM applied).
type AtlasSprite struct {
	SrcX, SrcY, SrcW, SrcH float64
	DstX, DstY, DstW, DstH float64
	// Opacity is 0..1; values <= 0 default to 1.
	Opacity float64
}

// DrawVertices draws a triangle mesh with optional per-vertex colors (Skia drawVertices / V.01).
//
// positions are in user space and transformed by the current CTM.
// When len(colors) == len(positions), Gouraud shading is used; otherwise the current
// fill solid color is used for the mesh.
//
// Preferred path: GPU convex tier with per-vertex colors (QueueColoredMesh).
// CPU fallback fills each triangle with a solid average color (no true Gouraud on CPU).
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

	if rc := c.gpuCtxOps(); rc != nil {
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
		c.recordCPUFallbackReason("verts:DrawVertices")
	}
	// CPU path needs its own copy if scratch will be reused later in the frame.
	devCopy := append([]Point(nil), dev...)
	c.drawVerticesCPU(devCopy, meshColors, solid, mode)
}

func (c *Context) drawVerticesCPU(positions []Point, colors []RGBA, solid RGBA, mode VertexMode) {
	emit := func(i0, i1, i2 int) {
		col := solid
		if len(colors) == len(positions) {
			c0, c1, c2 := colors[i0], colors[i1], colors[i2]
			col = RGBA{
				R: (c0.R + c1.R + c2.R) / 3,
				G: (c0.G + c1.G + c2.G) / 3,
				B: (c0.B + c1.B + c2.B) / 3,
				A: (c0.A + c1.A + c2.A) / 3,
			}
		}
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

// DrawAtlas draws multiple sub-rects from a single image (Skia drawAtlas / V.02).
// GPU path issues one QueueImageDraw per sprite from the shared ImageBuf.
// CPU path falls back to DrawImageEx per sprite.
func (c *Context) DrawAtlas(img *ImageBuf, sprites []AtlasSprite) {
	if c == nil || img == nil || len(sprites) == 0 {
		return
	}

	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return
	}

	if rc := c.gpuCtxOps(); rc != nil {
		pixelData := img.PremultipliedData()
		if len(pixelData) == 0 {
			if c.gpuPathAvailable() {
				c.recordCPUFallbackReason("verts:DrawAtlas")
			}
			c.drawAtlasCPU(img, sprites)
			return
		}
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
		vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32
		ctm := c.totalMatrix()
		genID := img.GenerationID()
		stride := img.Stride()
		queued := 0
		for _, sp := range sprites {
			if sp.SrcW <= 0 || sp.SrcH <= 0 || sp.DstW == 0 || sp.DstH == 0 {
				continue
			}
			op := sp.Opacity
			if op <= 0 {
				op = 1
			}
			if op > 1 {
				op = 1
			}
			tl := ctm.TransformPoint(Pt(sp.DstX, sp.DstY))
			tr := ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY))
			br := ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY+sp.DstH))
			bl := ctm.TransformPoint(Pt(sp.DstX, sp.DstY+sp.DstH))
			u0 := float32(sp.SrcX) / float32(imgW)
			v0 := float32(sp.SrcY) / float32(imgH)
			u1 := float32(sp.SrcX+sp.SrcW) / float32(imgW)
			v1 := float32(sp.SrcY+sp.SrcH) / float32(imgH)
			rc.QueueImageDraw(target, pixelData, genID, imgW, imgH, stride,
				float32(tl.X), float32(tl.Y),
				float32(tr.X), float32(tr.Y),
				float32(br.X), float32(br.Y),
				float32(bl.X), float32(bl.Y),
				float32(op), vpW, vpH, u0, v0, u1, v1, false, false)
			queued++
		}
		if queued > 0 {
			c.recordGPUOp()
			return
		}
	}

	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("verts:DrawAtlas")
	}
	c.drawAtlasCPU(img, sprites)
}

func (c *Context) drawAtlasCPU(img *ImageBuf, sprites []AtlasSprite) {
	for _, sp := range sprites {
		if sp.SrcW <= 0 || sp.SrcH <= 0 {
			continue
		}
		op := sp.Opacity
		if op <= 0 {
			op = 1
		}
		src := image.Rect(
			int(sp.SrcX), int(sp.SrcY),
			int(sp.SrcX+sp.SrcW), int(sp.SrcY+sp.SrcH),
		)
		c.DrawImageEx(img, DrawImageOptions{
			X:             sp.DstX,
			Y:             sp.DstY,
			DstWidth:      sp.DstW,
			DstHeight:     sp.DstH,
			SrcRect:       &src,
			Interpolation: InterpBilinear,
			Opacity:       op,
			BlendMode:     BlendNormal,
		})
	}
}

// Mesh describes an indexed triangle mesh for DrawMesh (Skia drawMesh / V.03 subset).
// Positions are user-space points (CTM applied). When Indices is non-empty, triangles
// are formed as (i0,i1,i2) groups of 3 indices; otherwise positions are a triangle list.
// When len(Colors)==len(Positions), Gouraud shading is used.
type Mesh struct {
	Positions []Point
	Colors    []RGBA
	Indices   []uint16
}

// DrawMesh draws an indexed (or triangle-list) colored mesh on the GPU path when available.
// This is the V.03 subset: positions + optional vertex colors + optional indices.
// Full custom fragment shaders / cubics are deferred.
func (c *Context) DrawMesh(mesh Mesh) {
	if c == nil || len(mesh.Positions) < 3 {
		return
	}
	positions := mesh.Positions
	colors := mesh.Colors
	if len(mesh.Indices) >= 3 {
		n := len(mesh.Indices) / 3 * 3
		useCol := len(colors) == len(positions)
		// Expand indices into reusable scratch (no per-call make when capacity ok).
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
