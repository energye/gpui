package render

import (
	"errors"
	"image"
	"math"

	intImage "github.com/energye/gpui/render/internal/image"
)

// R2 CPU渐变接口冻结（S16/W2，前置 S01 R1）。
//
// 一句话：显卡对三角逐像素插预乘颜色（Gouraud），
// CPU 必须按同样重心权重逐像素插，不许再取平均填纯色；
// 两边只差抗锯齿（当前网格管线 SkipAA，两边都是硬边）。
//
// 约定：
//   - 老 DrawVertices/DrawMesh 签名不动；纯色（无逐点色）仍走原 Fill 抗锯齿路。
//   - 新 DrawVerticesEx/DrawMeshEx 带显式校验加降级标记：
//     空 mesh 跳过（Skipped），非有限点与越界索引返回哨兵错，
//     CPU 真渐变时 Degraded=true（仅差抗锯齿），GPU 时 Degraded=false。
//   - CPU 采样与显卡一致：预乘（R*A/G*A/B*A/A）线性插，
//     再乘夹子/遮罩覆盖做源覆盖混合；混合恒按 Normal（与显卡凸包管线一致）。
//   - GOGPU_RENDER_MODE=cpu 强制走 CPU 真渐变，供 C 两边齐离屏对比取 CPU 真值。

// R2 哨兵错与降级标记：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrVertsNonFinite 非有限顶点（NaN/Inf）。
	ErrVertsNonFinite = errors.New("render: non-finite vertex")
	// ErrVertsBadIndex 索引越界（DrawMeshEx）。
	ErrVertsBadIndex = errors.New("render: vertex index out of range")
)

// VertCPUFallbackReason 是 CPU 真渐变回退的降级标记（与 RenderPathStats 对齐）。
// CPU 与 GPU 只差抗锯齿时仍记这一条，调用方凭它判定降级。
const VertCPUFallbackReason = "verts:DrawVertices"

// VertDrawOptions 是 DrawVerticesEx/DrawMeshEx 的选项（R2 预留，零值即默认）。
type VertDrawOptions struct{}

// VertDrawResult 是 Ex 新函数的降级标记与诊断。
type VertDrawResult struct {
	// Degraded 为 true 表示走了 CPU 真渐变（与 GPU 只差抗锯齿）。
	Degraded bool
	// Reason 为降级原因（Degraded 时为 VertCPUFallbackReason，否则为空）。
	Reason string
	// Skipped 为 true 表示空 mesh/全退化跳过，什么都没画。
	Skipped bool
	// Triangles 为实际光栅化的三角数（D/E 诊断用）。
	Triangles int
}

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
//
// R4 图集扩展冻结（S30/W4，2.2 的 render 底，前置 S24 R3）。
// 老 DrawAtlas 签名不动，只读老字段；新字段零值即老路。
//   - Rot 弧度，与 Rotate 一致（Y 朝下时正角顺时针），绕轴心转。
//   - FlipX/FlipY 先于旋转绕轴心镜像（几何翻转，UV 不动）。
//   - PivotX/PivotY 为 Dst 原点起的偏移（用户单位），零值即左上角。
//   - Tint 零结构体即白不透明（不染色）；其余值按直射相乘（含 A）。
//   - Filter 零值即 Bilinear（老路）；单图选 Nearest/Bilinear/Bicubic。
type AtlasSprite struct {
	SrcX, SrcY, SrcW, SrcH float64
	DstX, DstY, DstW, DstH float64
	// Opacity is 0..1; values <= 0 default to 1.
	Opacity float64
	// Rot rotates the Dst rect about the pivot (radians, R4 new branch).
	Rot float64
	// FlipX/FlipY mirror the Dst rect about the pivot before Rot (R4).
	FlipX, FlipY bool
	// PivotX/PivotY is the pivot offset from (DstX,DstY) in Dst units (R4).
	PivotX, PivotY float64
	// Tint multiplies straight source texels (R4); zero struct means white.
	Tint RGBA
	// Filter selects per-sprite sampling (R4); zero means Bilinear.
	Filter InterpolationMode
}

// R4 哨兵错与降级标记：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrAtlasNonFinite 非有限图集精灵（NaN/Inf，含 Tint）。
	ErrAtlasNonFinite = errors.New("render: non-finite atlas sprite")
	// ErrAtlasUnsupportedFilter 不支持的单图过滤（R4 仅 Nearest/Bilinear/Bicubic）。
	ErrAtlasUnsupportedFilter = errors.New("render: unsupported atlas filter (R4 only Nearest/Bilinear/Bicubic)")
)

// AtlasCPUFallbackReason 是图集 CPU 回退的降级标记（与 RenderPathStats 对齐）。
// tint 染色暂无显卡着色器，整批走 CPU 真采样时记这一条。
const AtlasCPUFallbackReason = "verts:DrawAtlas"

// AtlasDrawOptions 是 DrawAtlasEx 的选项（R4 预留，零值即默认）。
type AtlasDrawOptions struct{}

// AtlasDrawResult 是 Ex 新函数的降级标记与诊断。
type AtlasDrawResult struct {
	// Degraded 为 true 表示走了 CPU 真采样（含 tint 整批回退）。
	Degraded bool
	// Reason 为降级原因（Degraded 时为 AtlasCPUFallbackReason，否则为空）。
	Reason string
	// Skipped 为 true 表示空批或全跳过，什么都没画。
	Skipped bool
	// Drawn 为实际提交的精灵数（D/E 诊断用）。
	Drawn int
}

// DrawVertices draws a triangle mesh with optional per-vertex colors (Skia drawVertices / V.01).
//
// positions are in user space and transformed by the current CTM.
// When len(colors) == len(positions), Gouraud shading is used; otherwise the current
// fill solid color is used for the mesh.
//
// Preferred path: GPU convex tier with per-vertex colors (QueueColoredMesh).
// CPU fallback is true Gouraud (R2, per-pixel barycentric on premultiplied colors).
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
	// R2: true Gouraud goes to the new per-pixel path; solid keeps AA Fill.
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

// drawVerticesGouraudCPU rasterizes device-space triangles with true Gouraud (R2).
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
	// Frame damage: union AABB in device space (scale=1 tests; matches R1 quad path).
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

// vertTri holds one triangle with hoisted barycentric factors (R2).
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

// uniformVertColor reports whether every vertex color is identical (R2).
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

// DrawVerticesEx validates then draws via the frozen path, returning degradation (R2 new function).
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

// DrawMeshEx validates then draws via the frozen path, returning degradation (R2 new function).
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

// atlasTintIsIdentity 报告 Tint 是否为零结构体（白不透明，不染色）。
// 只有全零才算无染色，{1,1,1,0} 这类显式零 A 仍按透明处理。
func atlasTintIsIdentity(t RGBA) bool {
	return t == (RGBA{})
}

// atlasTintFactors 将 Tint 折成 0..1 系数，零结构体返回全 1。
func atlasTintFactors(t RGBA) (r, g, b, a float64) {
	if atlasTintIsIdentity(t) {
		return 1, 1, 1, 1
	}
	return t.R, t.G, t.B, t.A
}

// normalizeAtlasFilter 将零值映射为 Bilinear，其余仅收三种模式。
func normalizeAtlasFilter(m InterpolationMode) (InterpolationMode, error) {
	if m == 0 {
		return InterpBilinear, nil
	}
	if m == InterpNearest || m == InterpBilinear || m == InterpBicubic {
		return m, nil
	}
	return InterpBilinear, ErrAtlasUnsupportedFilter
}

// normalizeAtlasOpacity 沿用老路惯例：<=0 按 1（默认不透明），>1 钳 1。
func normalizeAtlasOpacity(o float64) float64 {
	if o <= 0 {
		return 1
	}
	if o > 1 {
		return 1
	}
	return o
}

// isFiniteAtlasSprite 检查精灵全部浮点字段（含 Tint）有限。
func isFiniteAtlasSprite(sp AtlasSprite) bool {
	for _, v := range []float64{sp.SrcX, sp.SrcY, sp.SrcW, sp.SrcH, sp.DstX, sp.DstY, sp.DstW, sp.DstH, sp.Opacity, sp.Rot, sp.PivotX, sp.PivotY} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return isFiniteRGBA(sp.Tint)
}

// atlasDrawable 报告精灵是否可画（与老 DrawAtlas 跳过规则一致）。
func atlasDrawable(sp AtlasSprite) bool {
	return sp.SrcW > 0 && sp.SrcH > 0 && sp.DstW != 0 && sp.DstH != 0
}

// atlasSpriteCorners 算出设备四角（TL/TR/BR/BL），先绕轴心翻转再旋转。
// 老路（Rot==0 且无翻转）走原直接式，保证与老 DrawAtlas 逐位一致。
func atlasSpriteCorners(sp AtlasSprite, ctm Matrix) (tl, tr, br, bl Point) {
	if sp.Rot == 0 && !sp.FlipX && !sp.FlipY {
		tl = ctm.TransformPoint(Pt(sp.DstX, sp.DstY))
		tr = ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY))
		br = ctm.TransformPoint(Pt(sp.DstX+sp.DstW, sp.DstY+sp.DstH))
		bl = ctm.TransformPoint(Pt(sp.DstX, sp.DstY+sp.DstH))
		return tl, tr, br, bl
	}
	px := sp.DstX + sp.PivotX
	py := sp.DstY + sp.PivotY
	cos := math.Cos(sp.Rot)
	sin := math.Sin(sp.Rot)
	xform := func(x, y float64) Point {
		if sp.FlipX {
			x = 2*px - x
		}
		if sp.FlipY {
			y = 2*py - y
		}
		dx := x - px
		dy := y - py
		ux := px + dx*cos - dy*sin
		uy := py + dx*sin + dy*cos
		return ctm.TransformPoint(Pt(ux, uy))
	}
	tl = xform(sp.DstX, sp.DstY)
	tr = xform(sp.DstX+sp.DstW, sp.DstY)
	br = xform(sp.DstX+sp.DstW, sp.DstY+sp.DstH)
	bl = xform(sp.DstX, sp.DstY+sp.DstH)
	return tl, tr, br, bl
}

// DrawAtlasEx 校验后按 R4 新分支绘制并返回降级标记（新函数，老路不动）。
// 空批或全跳过返回 Skipped；非有限数与未知过滤返回哨兵错且什么都不画。
// tint 染色暂无显卡着色器，含染色整批走 CPU 真采样（Degraded）。
func (c *Context) DrawAtlasEx(img *ImageBuf, sprites []AtlasSprite, _ AtlasDrawOptions) (AtlasDrawResult, error) {
	if c == nil || img == nil || len(sprites) == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	for i := range sprites {
		if !isFiniteAtlasSprite(sprites[i]) {
			return AtlasDrawResult{}, ErrAtlasNonFinite
		}
		if _, err := normalizeAtlasFilter(sprites[i].Filter); err != nil {
			return AtlasDrawResult{}, err
		}
	}
	drawIdx := make([]int, 0, len(sprites))
	for i := range sprites {
		if atlasDrawable(sprites[i]) {
			drawIdx = append(drawIdx, i)
		}
	}
	if len(drawIdx) == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	hasTint := false
	for _, i := range drawIdx {
		if !atlasTintIsIdentity(sprites[i].Tint) {
			hasTint = true
			break
		}
	}
	useGPU := !CPUOnlyMode() && c.gpuCtxOps() != nil
	var pixelData []byte
	var genID uint64
	var stride int
	if useGPU {
		pixelData = img.PremultipliedData()
		if len(pixelData) == 0 {
			useGPU = false
		} else {
			genID = img.GenerationID()
			stride = img.Stride()
		}
		// 染色无着色器：整批回 CPU，保证两边同一真值。
		if useGPU && hasTint {
			useGPU = false
		}
	}
	if useGPU {
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
		vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32
		ctm := c.totalMatrix()
		queued := 0
		for _, i := range drawIdx {
			sp := sprites[i]
			filt, _ := normalizeAtlasFilter(sp.Filter)
			op := normalizeAtlasOpacity(sp.Opacity)
			tl, tr, br, bl := atlasSpriteCorners(sp, ctm)
			u0 := float32(sp.SrcX) / float32(imgW)
			v0 := float32(sp.SrcY) / float32(imgH)
			u1 := float32(sp.SrcX+sp.SrcW) / float32(imgW)
			v1 := float32(sp.SrcY+sp.SrcH) / float32(imgH)
			nearest := filt == InterpNearest
			bicubic := filt == InterpBicubic
			c.gpuCtxOps().QueueImageDraw(target, pixelData, genID, imgW, imgH, stride,
				float32(tl.X), float32(tl.Y),
				float32(tr.X), float32(tr.Y),
				float32(br.X), float32(br.Y),
				float32(bl.X), float32(bl.Y),
				float32(op), vpW, vpH, u0, v0, u1, v1, nearest, false, bicubic)
			queued++
		}
		if queued > 0 {
			c.recordGPUOp()
			return AtlasDrawResult{Drawn: queued}, nil
		}
		return AtlasDrawResult{Skipped: true}, nil
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason(AtlasCPUFallbackReason)
	}
	n := c.drawAtlasExCPU(img, sprites, drawIdx)
	if n == 0 {
		return AtlasDrawResult{Skipped: true}, nil
	}
	return AtlasDrawResult{Degraded: true, Reason: AtlasCPUFallbackReason, Drawn: n}, nil
}

// drawAtlasExCPU 用显卡同拆法逐像素真采样（R4 新路，支持 rot/flip/tint/filter）。
// 与 GPU 同为 TL-TR-BL + TR-BR-BL，对角线归首三角；采样钳边与显卡一致。
func (c *Context) drawAtlasExCPU(img *ImageBuf, sprites []AtlasSprite, drawIdx []int) int {
	if c == nil || c.pixmap == nil || img == nil || len(drawIdx) == 0 {
		return 0
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return 0
	}
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		c.flushGPUAccelerator()
	}
	c.noteLayerCPUDraw()
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return 0
	}
	hasClip := c.clipStack != nil && c.clipStack.Depth() > 0
	mask := c.mask
	premulSrc := img.Format().IsPremultiplied()
	ctm := c.totalMatrix()
	drawn := 0
	for _, si := range drawIdx {
		sp := sprites[si]
		filt, _ := normalizeAtlasFilter(sp.Filter)
		op := normalizeAtlasOpacity(sp.Opacity)
		if op <= 0 {
			continue
		}
		tr, tg, tb, ta := atlasTintFactors(sp.Tint)
		tl, trPt, br, bl := atlasSpriteCorners(sp, ctm)
		dev := [4]Point{tl, trPt, br, bl}
		minX, minY, maxX, maxY := quadBounds(dev)
		c.trackDamage(image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1))
		u0 := sp.SrcX / float64(imgW)
		v0 := sp.SrcY / float64(imgH)
		u1 := (sp.SrcX + sp.SrcW) / float64(imgW)
		v1 := (sp.SrcY + sp.SrcH) / float64(imgH)
		t1 := newTriSampler(dev[0], dev[1], dev[3], [3][2]float64{{u0, v0}, {u1, v0}, {u0, v1}})
		t2 := newTriSampler(dev[1], dev[2], dev[3], [3][2]float64{{u1, v0}, {u1, v1}, {u0, v1}})
		x0 := int(math.Floor(minX))
		y0 := int(math.Floor(minY))
		x1 := int(math.Ceil(maxX))
		y1 := int(math.Ceil(maxY))
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
			continue
		}
		mode := intImage.InterpolationMode(filt)
		touched := false
		for y := y0; y < y1; y++ {
			py := float64(y) + 0.5
			for x := x0; x < x1; x++ {
				px := float64(x) + 0.5
				u, v, ok := t1.uv(px, py)
				if !ok {
					u, v, ok = t2.uv(px, py)
					if !ok {
						continue
					}
				}
				sr, sg, sb, sa := intImage.Sample(img, u, v, mode)
				if sa == 0 {
					continue
				}
				k := op
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
				if k <= 0 {
					continue
				}
				var srcR, srcG, srcB, srcA float64
				if premulSrc {
					srcA = float64(sa) / 255.0 * ta * k
					srcR = float64(sr) / 255.0 * tr * ta * k
					srcG = float64(sg) / 255.0 * tg * ta * k
					srcB = float64(sb) / 255.0 * tb * ta * k
				} else {
					baseA := float64(sa) / 255.0
					srcA = baseA * ta * k
					if srcA <= 0 {
						continue
					}
					srcR = float64(sr) / 255.0 * tr * srcA
					srcG = float64(sg) / 255.0 * tg * srcA
					srcB = float64(sb) / 255.0 * tb * srcA
				}
				if srcA <= 0 {
					continue
				}
				touched = true
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
		_ = touched
		drawn++
	}
	return drawn
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
//
// opt22: when Indices is set, the GPU path keeps unique verts + DrawIndexed
// (no CPU expand). CPU fallback still expands to triangle lists.
func (c *Context) DrawMesh(mesh Mesh) {
	if c == nil || len(mesh.Positions) < 3 {
		return
	}
	positions := mesh.Positions
	colors := mesh.Colors
	hasIdx := len(mesh.Indices) >= 3

	// GPU indexed path: CTM → unique device verts + indices (opt22).
	if hasIdx {
		if rc := c.gpuCtxOps(); rc != nil && !CPUOnlyMode() {
			n := len(positions)
			ctm := c.totalMatrix()
			var dev []Point
			// opt23: identity CTM — queue user-space points without a full copy/transform.
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
