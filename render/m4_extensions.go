package render

import (
	"errors"
	"image"
	"math"

	intImage "github.com/energye/gpui/render/internal/image"
)

// R1 梯形贴图接口冻结（S01/W1，前置 0.0）。
//
// 一句话：显卡把四边形拆成 TL-TR-BL 与 TR-BR-BL 两个三角分别贴图，
// CPU 必须按同样拆法逐像素采样，不许再拉成方块，也不许静默画错。
//
// 约定：
//   - 老 DrawImageQuad 签名不动，内部改走正确梯形；退化四边形直接跳过不崩。
//   - 新 DrawImageQuadEx 带选项加显式报错：退化、自交、非有限数、
//     非 Normal 混合都返回哨兵错，调用方能判错不背锅。
//   - 源图恒取整张（u/v 0..1），与显卡 QueueImageDraw 一致；
//     采样按 Interpolation 选项走既有 Sample 实现。

// QuadKind 是四边形分类：只有 QuadOK 能画，其余画不得。
type QuadKind int

const (
	// QuadOK 可画（凸凹皆可，按显卡同拆法）。
	QuadOK QuadKind = iota
	// QuadDegenerate 面积为零（点线重合或被矩阵压扁）。
	QuadDegenerate
	// QuadBowTie 自交蝴蝶结（边与边交叉）。
	QuadBowTie
	// QuadNonFinite 角点有 NaN 或 Inf。
	QuadNonFinite
)

// String 便于日志与单测打印。
func (k QuadKind) String() string {
	switch k {
	case QuadOK:
		return "ok"
	case QuadDegenerate:
		return "degenerate"
	case QuadBowTie:
		return "bowtie"
	case QuadNonFinite:
		return "nonfinite"
	default:
		return "unknown"
	}
}

// QuadDrawOptions 是 DrawImageQuadEx 的选项。
type QuadDrawOptions struct {
	// Interpolation 采样方式，零值按 Bilinear。
	Interpolation InterpolationMode
	// Opacity 0..1，零值按 1（沿用 DrawImageEx 惯例：0 表默认不透明）。
	Opacity float64
	// BlendMode 混合方式，R1 只支持 Normal，其余显式报错。
	BlendMode BlendMode
}

// R1 哨兵错：调用方用 errors.Is 判定，不许静默画错。
var (
	// ErrQuadDegenerate 退化四边形（零面积）。
	ErrQuadDegenerate = errors.New("render: degenerate quad (zero area)")
	// ErrQuadBowTie 自交四边形。
	ErrQuadBowTie = errors.New("render: self-intersecting quad")
	// ErrQuadNonFinite 非有限角点。
	ErrQuadNonFinite = errors.New("render: non-finite quad corner")
	// ErrQuadUnsupportedBlend R1 不支持的混合模式。
	ErrQuadUnsupportedBlend = errors.New("render: unsupported quad blend mode (R1 only Normal)")
	// ErrQuadUnsupportedInterp 不支持的采样方式。
	ErrQuadUnsupportedInterp = errors.New("render: unsupported quad interpolation")
)

// SetDither enables ordered dithering of soft fills after GPU resolve (P.09).
// When enabled, subsequent Image()/FlushGPU results for this context receive a
// Bayer 4x4 ordered dither on the pixmap (reduces gradient banding).
func (c *Context) SetDither(enabled bool) {
	if c == nil {
		return
	}
	c.dither = enabled
}

// Dither reports whether ordered dithering is enabled.
func (c *Context) Dither() bool {
	if c == nil {
		return false
	}
	return c.dither
}

func (c *Context) applyDitherIfEnabled() {
	if c == nil || !c.dither || c.pixmap == nil {
		return
	}
	applyBayerDither4(c.pixmap)
	c.pixmap.NotifyPixelsChanged()
}

// Bayer 4x4 thresholds in 0..15, scaled into low-bit noise for 8-bit channels.
var bayer4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func applyBayerDither4(pm *Pixmap) {
	if pm == nil {
		return
	}
	w, h := pm.Width(), pm.Height()
	data := pm.Data()
	stride := pm.Width() * 4
	if len(data) < h*stride {
		return
	}
	for y := 0; y < h; y++ {
		row := y * stride
		for x := 0; x < w; x++ {
			thr := bayer4[y&3][x&3] // 0..15
			// Bias each channel by ±2 based on threshold vs low bits.
			off := row + x*4
			for c := 0; c < 3; c++ {
				v := int(data[off+c])
				// ordered: if (v & 15) > thr then bump else leave; mild banding break
				if (v & 15) > thr {
					if v < 253 {
						v += 2
					}
				} else if v > 2 {
					v -= 1
				}
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				data[off+c] = byte(v)
			}
		}
	}
}

// DrawImageQuad draws an image into a free-form destination quad (T.04 non-affine subset).
// corners are user-space points in order: top-left, top-right, bottom-right, bottom-left.
// GPU path uses QueueImageDraw with arbitrary corner mapping (perspective-like trapezoids).
// CPU path uses the same TL-TR-BL + TR-BR-BL split for pixel parity (R1).
// Compat wrapper: defaults Bilinear + opaque + Normal, degenerate quads no-op.
func (c *Context) DrawImageQuad(img *ImageBuf, corners [4]Point) {
	_ = c.DrawImageQuadEx(img, corners, QuadDrawOptions{
		Interpolation: InterpBilinear, Opacity: 1, BlendMode: BlendNormal,
	})
}

// DrawImageQuadEx draws the full source image into corners with explicit options.
// Returns a sentinel error for degenerate/self-intersecting/non-finite quads
// or unsupported blend/interp instead of silently drawing wrong.
func (c *Context) DrawImageQuadEx(img *ImageBuf, corners [4]Point, opts QuadDrawOptions) error {
	if c == nil || img == nil {
		return nil
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return nil
	}
	interp, err := normalizeQuadInterp(opts.Interpolation)
	if err != nil {
		return err
	}
	if opts.BlendMode != BlendNormal {
		return ErrQuadUnsupportedBlend
	}
	opacity := normalizeQuadOpacity(opts.Opacity)
	if kind := ClassifyQuad(corners); kind != QuadOK {
		return quadKindErr(kind)
	}
	ctm := c.totalMatrix()
	dev := [4]Point{}
	for i := 0; i < 4; i++ {
		dev[i] = ctm.TransformPoint(corners[i])
	}
	if quadHasNonFinite(dev) || quadAreaSum(dev) < 1e-9 {
		return ErrQuadDegenerate
	}
	// Damage bounds (device space).
	minX, minY, maxX, maxY := quadBounds(dev)
	c.trackDamage(image.Rect(int(minX), int(minY), int(maxX)+1, int(maxY)+1))

	// GOGPU_RENDER_MODE=cpu 强制走 CPU 双三角，供 C 两边齐离屏对比取 CPU 真值。
	if !CPUOnlyMode() {
		if rc := c.gpuCtxOps(); rc != nil {
			pixelData := img.PremultipliedData()
			if len(pixelData) == 0 {
				if c.gpuPathAvailable() {
					c.recordCPUFallbackReason("image:DrawImageQuad")
				}
				c.drawImageQuadCPUSplit(img, dev, interp, opacity)
				return nil
			}
			defer c.setGPUClipRect()()
			target := c.gpuRenderTarget()
			vpW := uint32(target.Width)  //nolint:gosec
			vpH := uint32(target.Height) //nolint:gosec
			nearest := interp == InterpNearest
			bicubic := interp == InterpBicubic
			rc.QueueImageDraw(target, pixelData, img.GenerationID(), imgW, imgH, img.Stride(),
				float32(dev[0].X), float32(dev[0].Y),
				float32(dev[1].X), float32(dev[1].Y),
				float32(dev[2].X), float32(dev[2].Y),
				float32(dev[3].X), float32(dev[3].Y),
				float32(opacity), vpW, vpH,
				0, 0, 1, 1,
				nearest, false, bicubic)
			c.recordGPUOp()
			return nil
		}
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("image:DrawImageQuad")
	}
	c.drawImageQuadCPUSplit(img, dev, interp, opacity)
	return nil
}

// normalizeQuadInterp maps zero to Bilinear and rejects unknown modes.
func normalizeQuadInterp(m InterpolationMode) (InterpolationMode, error) {
	if m == 0 {
		m = InterpBilinear
	}
	if m != InterpNearest && m != InterpBilinear && m != InterpBicubic {
		return InterpBilinear, ErrQuadUnsupportedInterp
	}
	return m, nil
}

// normalizeQuadOpacity maps zero to opaque (DrawImageEx convention) and clamps 0..1.
func normalizeQuadOpacity(o float64) float64 {
	if o == 0 {
		return 1
	}
	if o < 0 {
		return 0
	}
	if o > 1 {
		return 1
	}
	return o
}

// isFinitePt reports finite coordinates (NaN/Inf safe for ClassifyQuad).
func isFinitePt(p Point) bool {
	return !math.IsNaN(p.X) && !math.IsNaN(p.Y) &&
		!math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0)
}

// quadHasNonFinite reports any non-finite corner.
func quadHasNonFinite(q [4]Point) bool {
	for i := 0; i < 4; i++ {
		if !isFinitePt(q[i]) {
			return true
		}
	}
	return false
}

// quadAreaSum is the twin-split area (GPU TL-TR-BL + TR-BR-BL), degenerate near zero.
func quadAreaSum(q [4]Point) float64 {
	return math.Abs(triArea(q[0], q[1], q[3])) + math.Abs(triArea(q[1], q[2], q[3]))
}

// quadBounds returns device-space AABB for damage and raster clamping.
func quadBounds(dev [4]Point) (minX, minY, maxX, maxY float64) {
	minX = math.Min(math.Min(dev[0].X, dev[1].X), math.Min(dev[2].X, dev[3].X))
	minY = math.Min(math.Min(dev[0].Y, dev[1].Y), math.Min(dev[2].Y, dev[3].Y))
	maxX = math.Max(math.Max(dev[0].X, dev[1].X), math.Max(dev[2].X, dev[3].X))
	maxY = math.Max(math.Max(dev[0].Y, dev[1].Y), math.Max(dev[2].Y, dev[3].Y))
	return minX, minY, maxX, maxY
}

func quadKindErr(k QuadKind) error {
	switch k {
	case QuadDegenerate:
		return ErrQuadDegenerate
	case QuadBowTie:
		return ErrQuadBowTie
	case QuadNonFinite:
		return ErrQuadNonFinite
	default:
		return nil
	}
}

// ClassifyQuad classifies user-space corners without drawing.
func ClassifyQuad(corners [4]Point) QuadKind {
	if quadHasNonFinite(corners) {
		return QuadNonFinite
	}
	if quadBowTie(corners) {
		return QuadBowTie
	}
	if quadAreaSum(corners) < 1e-9 {
		return QuadDegenerate
	}
	return QuadOK
}

func triArea(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (c.X-a.X)*(b.Y-a.Y)
}

// quadBowTie reports edge crossing of 0-1 vs 2-3 or 1-2 vs 3-0.
func quadBowTie(q [4]Point) bool {
	if segsCross(q[0], q[1], q[2], q[3]) {
		return true
	}
	if segsCross(q[1], q[2], q[3], q[0]) {
		return true
	}
	return false
}

func orient(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

func segsCross(a, b, c, d Point) bool {
	o1 := orient(a, b, c)
	o2 := orient(a, b, d)
	o3 := orient(c, d, a)
	o4 := orient(c, d, b)
	// Shared endpoints do not count as crossing.
	if (a == c) || (a == d) || (b == c) || (b == d) {
		return false
	}
	return (o1*o2 < 0) && (o3*o4 < 0)
}

func (c *Context) drawImageQuadCPU(img *ImageBuf, corners [4]Point) {
	// Compat: user-space corners in, device split inside Ex.
	_ = c.DrawImageQuadEx(img, corners, QuadDrawOptions{
		Interpolation: InterpBilinear, Opacity: 1, BlendMode: BlendNormal,
	})
}

// drawImageQuadCPUSplit rasterizes device-space quad with GPU twin split.
func (c *Context) drawImageQuadCPUSplit(img *ImageBuf, dev [4]Point, interp InterpolationMode, opacity float64) {
	if c == nil || c.pixmap == nil || img == nil {
		return
	}
	opacity = normalizeQuadOpacity(opacity)
	if opacity <= 0 {
		return
	}
	// Keep z-order with pending GPU when drawing on base canvas.
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		c.flushGPUAccelerator()
	}
	c.noteLayerCPUDraw()
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return
	}
	minXF, minYF, maxXF, maxYF := quadBounds(dev)
	x0 := int(math.Floor(minXF))
	y0 := int(math.Floor(minYF))
	x1 := int(math.Ceil(maxXF))
	y1 := int(math.Ceil(maxYF))
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
	premulSrc := img.Format().IsPremultiplied()
	hasClip := c.clipStack != nil && c.clipStack.Depth() > 0
	mask := c.mask
	// GPU twin UVs: TL(0,0) TR(1,0) BR(1,1) BL(0,1), denominators hoisted per quad.
	t1 := newTriSampler(dev[0], dev[1], dev[3], [3][2]float64{{0, 0}, {1, 0}, {0, 1}})
	t2 := newTriSampler(dev[1], dev[2], dev[3], [3][2]float64{{1, 0}, {1, 1}, {0, 1}})
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
			if u < 0 || u > 1 || v < 0 || v > 1 {
				continue
			}
			sr, sg, sb, sa := intImage.Sample(img, u, v, intImage.InterpolationMode(interp))
			if sa == 0 {
				continue
			}
			cover := 255
			if hasClip {
				cover = int(c.clipStack.Coverage(px, py))
				if cover == 0 {
					continue
				}
			}
			mcov := 255
			if mask != nil {
				mcov = int(mask.At(x, y))
				if mcov == 0 {
					continue
				}
			}
			k := opacity * float64(cover) / 255.0 * float64(mcov) / 255.0
			if k <= 0 {
				continue
			}
			var srcR, srcG, srcB, srcA float64
			if premulSrc {
				srcA = float64(sa) / 255.0 * k
				srcR = float64(sr) / 255.0 * k
				srcG = float64(sg) / 255.0 * k
				srcB = float64(sb) / 255.0 * k
			} else {
				baseA := float64(sa) / 255.0
				srcA = baseA * k
				srcR = float64(sr) / 255.0 * srcA
				srcG = float64(sg) / 255.0 * srcA
				srcB = float64(sb) / 255.0 * srcA
			}
			if srcA <= 0 {
				continue
			}
			if srcA >= 1 {
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

// triSampler holds one split triangle with hoisted barycentric factors.
// Same math as the per-pixel formula, denominators computed once per quad.
type triSampler struct {
	x2, y2 float64
	a0, b0 float64
	a1, b1 float64
	den    float64
	uvs    [3][2]float64
	degen  bool
}

func newTriSampler(p0, p1, p2 Point, uv [3][2]float64) triSampler {
	den := (p1.Y-p2.Y)*(p0.X-p2.X) + (p2.X-p1.X)*(p0.Y-p2.Y)
	return triSampler{
		x2: p2.X, y2: p2.Y,
		a0: p1.Y - p2.Y, b0: p2.X - p1.X,
		a1: p2.Y - p0.Y, b1: p0.X - p2.X,
		den: den, uvs: uv,
		degen: math.Abs(den) < 1e-12,
	}
}

// uv interpolates UV with barycentric weights; shared diagonal goes to tri1.
func (t *triSampler) uv(px, py float64) (float64, float64, bool) {
	if t == nil || t.degen {
		return 0, 0, false
	}
	dx := px - t.x2
	dy := py - t.y2
	w0 := (t.a0*dx + t.b0*dy) / t.den
	w1 := (t.a1*dx + t.b1*dy) / t.den
	w2 := 1 - w0 - w1
	const eps = -1e-9
	if w0 < eps || w1 < eps || w2 < eps {
		return 0, 0, false
	}
	return w0*t.uvs[0][0] + w1*t.uvs[1][0] + w2*t.uvs[2][0],
		w0*t.uvs[0][1] + w1*t.uvs[1][1] + w2*t.uvs[2][1], true
}

// PushBackdropLayer creates a layer pre-filled with a snapshot of the current
// parent canvas (L.05 backdrop subset). Subsequent drawing/filters operate over
// that backdrop; PopLayer composites with the given blend/opacity.
func (c *Context) PushBackdropLayer(blendMode BlendMode, opacity float64) {
	if c == nil {
		return
	}
	// Ensure GPU content is resolved into the current pixmap before snapshot.
	_ = c.FlushGPU()
	c.applyDitherIfEnabled()

	parent := c.pixmap
	// S6.4: push layer without Clear — full backdrop copy overwrites every pixel.
	c.pushLayerSurface(blendMode, opacity, false, false)
	if parent != nil && c.pixmap != nil {
		dst := c.pixmap.Data()
		src := parent.Data()
		n := len(dst)
		if len(src) < n {
			n = len(src)
		}
		copy(dst[:n], src[:n])
		c.pixmap.NotifyPixelsChanged()
	}
	// R1 L.05: seed layer GPU RT from snapshot so subsequent GPU draws composite
	// over real backdrop content (not an empty RT).
	if !c.seedTopLayerGPUFromPixmap() {
		// If seed fails, mark CPU drew so Pop uses pixmap composite path.
		c.noteLayerCPUDraw()
	}
	// Backdrop is a full-surface snapshot; Pop must blend the whole layer.
	c.markLayerFullComposite()
}
