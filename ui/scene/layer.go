package scene

// Layer is a retained scene-graph node (Flutter Layer subset).
// Layers are shared across frames when unchanged (COW / pointer reuse).
type Layer interface {
	// LayerID is a stable identity for dirty sets (0 = anonymous / no dirty tracking).
	LayerID() uint64
	// Children returns child layers in paint order.
	Children() []Layer
	// Kind is a debug/type tag.
	Kind() string
}

// layerIDGen allocates monotonic ids for boundary / picture layers.
var layerIDGen uint64

// NextLayerID returns a new non-zero layer id (not concurrency-hardened beyond atomic-ish P2 use).
func NextLayerID() uint64 {
	layerIDGen++
	if layerIDGen == 0 {
		layerIDGen = 1
	}
	return layerIDGen
}

// ResetLayerIDGen is for tests only.
func ResetLayerIDGen() { layerIDGen = 0 }

// ContainerLayer holds ordered children.
type ContainerLayer struct {
	id   uint64
	kids []Layer
}

// NewContainerLayer creates an empty container.
func NewContainerLayer() *ContainerLayer {
	return &ContainerLayer{id: NextLayerID()}
}

func (c *ContainerLayer) LayerID() uint64   { return c.id }
func (c *ContainerLayer) Children() []Layer { return c.kids }
func (c *ContainerLayer) Kind() string      { return "container" }

// Add appends a child layer.
func (c *ContainerLayer) Add(child Layer) {
	if c == nil || child == nil {
		return
	}
	c.kids = append(c.kids, child)
}

// Walk visits this layer then children (pre-order). Returns visit count.
func Walk(l Layer, fn func(Layer)) int {
	if l == nil {
		return 0
	}
	n := 1
	if fn != nil {
		fn(l)
	}
	for _, ch := range l.Children() {
		n += Walk(ch, fn)
	}
	return n
}

// OffsetLayer applies a logical pixel offset to children (Y-down).
type OffsetLayer struct {
	ContainerLayer
	DX, DY float64
}

// NewOffsetLayer creates an offset layer.
func NewOffsetLayer(dx, dy float64) *OffsetLayer {
	o := &OffsetLayer{DX: dx, DY: dy}
	o.id = NextLayerID()
	return o
}

func (o *OffsetLayer) Kind() string { return "offset" }

// OpacityLayer multiplies opacity for descendants (compositor-only candidate).
type OpacityLayer struct {
	ContainerLayer
	Opacity float64 // 0..1
}

// NewOpacityLayer creates an opacity layer.
func NewOpacityLayer(opacity float64) *OpacityLayer {
	o := &OpacityLayer{Opacity: opacity}
	o.id = NextLayerID()
	return o
}

func (o *OpacityLayer) Kind() string { return "opacity" }

// ClipRectLayer clips children to a logical rect (local to parent).
type ClipRectLayer struct {
	ContainerLayer
	X, Y, W, H float64
}

// NewClipRectLayer creates a clip layer.
func NewClipRectLayer(x, y, w, h float64) *ClipRectLayer {
	c := &ClipRectLayer{X: x, Y: y, W: w, H: h}
	c.id = NextLayerID()
	return c
}

func (c *ClipRectLayer) Kind() string { return "clip_rect" }

// ClipRRectLayer clips children to a rounded rect (uniform corner radius, logical px).
// Radius <= 0 is treated as a hard rect at composite/paint apply time.
type ClipRRectLayer struct {
	ContainerLayer
	X, Y, W, H float64
	Radius     float64
}

// NewClipRRectLayer creates a rounded-rect clip layer.
func NewClipRRectLayer(x, y, w, h, radius float64) *ClipRRectLayer {
	c := &ClipRRectLayer{X: x, Y: y, W: w, H: h, Radius: radius}
	c.id = NextLayerID()
	return c
}

func (c *ClipRRectLayer) Kind() string { return "clip_rrect" }

// TransformLayer applies a 2D similarity transform to children (Flutter TransformLayer subset).
// Order when applied at paint/composite: translate (TX,TY) → rotate about local origin → scale.
// SX/SY default to 1 when zero is stored as "unset" — callers should set SX=SY=1 explicitly.
type TransformLayer struct {
	ContainerLayer
	TX, TY   float64 // translation (logical px, Y-down)
	Rotation float64 // radians, clockwise in Y-down canvas matches render.Rotate
	SX, SY   float64 // scale; treat 0 as 1 when applying
}

// NewTransformLayer creates a transform layer. Pass sx,sy=1 for pure rotate/translate.
func NewTransformLayer(tx, ty, rotation, sx, sy float64) *TransformLayer {
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	t := &TransformLayer{TX: tx, TY: ty, Rotation: rotation, SX: sx, SY: sy}
	t.id = NextLayerID()
	return t
}

func (t *TransformLayer) Kind() string { return "transform" }

// EffectiveScale returns SX,SY with 0 treated as 1.
func (t *TransformLayer) EffectiveScale() (sx, sy float64) {
	if t == nil {
		return 1, 1
	}
	sx, sy = t.SX, t.SY
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	return sx, sy
}

// ColorFilterLayer applies a 4×5 color matrix to descendants at composite time
// (Flutter ColorFilterLayer subset). Matrix is row-major [20]float32 like
// render.ApplyColorMatrix. Identity leaves colors unchanged.
// Present-time application is still limited (full-frame Present); this type
// retains the filter intent in the scene tree.
type ColorFilterLayer struct {
	ContainerLayer
	Matrix [20]float32
}

// NewColorFilterLayer creates a color-filter layer. Pass grayscale/sepia/etc. matrix.
func NewColorFilterLayer(matrix [20]float32) *ColorFilterLayer {
	c := &ColorFilterLayer{Matrix: matrix}
	c.id = NextLayerID()
	return c
}

// NewGrayscaleColorFilterLayer builds a standard luminance grayscale matrix layer.
func NewGrayscaleColorFilterLayer() *ColorFilterLayer {
	// ITU-R BT.601 luma → R=G=B.
	const (
		lr = float32(0.299)
		lg = float32(0.587)
		lb = float32(0.114)
	)
	return NewColorFilterLayer([20]float32{
		lr, lg, lb, 0, 0,
		lr, lg, lb, 0, 0,
		lr, lg, lb, 0, 0,
		0, 0, 0, 1, 0,
	})
}

func (c *ColorFilterLayer) Kind() string { return "color_filter" }

// ImageFilterLayer applies a blur (or future image filter) to descendants
// (Flutter ImageFilterLayer subset — blur radius first). Radius <= 0 is a no-op
// at apply time. Not a full BackdropFilterLayer (no live background sampling).
type ImageFilterLayer struct {
	ContainerLayer
	// BlurRadius is the Gaussian blur sigma/radius in logical px (uniform).
	BlurRadius float64
}

// NewImageFilterLayer creates an image-filter layer with uniform blur radius.
func NewImageFilterLayer(blurRadius float64) *ImageFilterLayer {
	i := &ImageFilterLayer{BlurRadius: blurRadius}
	i.id = NextLayerID()
	return i
}

func (i *ImageFilterLayer) Kind() string { return "image_filter" }

// PictureLayer holds a retained picture (or a re-record flag for P3 raster).
// When Picture.Ops is non-empty, the display list can be Replay'd onto a Context
// without re-walking the RO tree. NeedsRaster still tracks dirty vs static reuse
// in RasterizeDirty (flag/stats path — not dirty-rect Present).
type PictureLayer struct {
	id      uint64
	Picture Picture
	// NeedsRaster is true when the picture content must be re-drawn (dirty).
	NeedsRaster bool
}

// NewPictureLayer creates a picture layer.
func NewPictureLayer() *PictureLayer {
	return &PictureLayer{id: NextLayerID(), NeedsRaster: true}
}

func (p *PictureLayer) LayerID() uint64   { return p.id }
func (p *PictureLayer) Children() []Layer { return nil }
func (p *PictureLayer) Kind() string      { return "picture" }

// SetPicture installs a recorded Picture. Any new display list (or empty/invalid
// install) marks NeedsRaster so the next RasterizeDirty / RasterizeDirtyToContext
// will apply or re-apply content. NeedsRaster is cleared only by RasterizeDirty*
// after a successful dirty pass — not here — so AddPicture→Record→BuildPacket→
// RasterizeDirtyToContext replays without a manual dirty workaround.
func (p *PictureLayer) SetPicture(pic Picture) {
	if p == nil {
		return
	}
	p.Picture = pic
	p.NeedsRaster = true
}

// Record replaces the layer's Picture via a recorder callback and dirties
// NeedsRaster (see SetPicture).
func (p *PictureLayer) Record(fn func(*PictureRecorder)) {
	if p == nil {
		return
	}
	p.SetPicture(RecordPicture(fn))
}

// BoundaryLayer is a repaint-boundary root: independent dirty / raster unit.
type BoundaryLayer struct {
	OffsetLayer
	// Source is an optional debug tag (e.g. "spinner").
	Source string
}

// NewBoundaryLayer creates a repaint boundary layer at offset.
func NewBoundaryLayer(dx, dy float64, source string) *BoundaryLayer {
	b := &BoundaryLayer{Source: source}
	b.id = NextLayerID()
	b.DX, b.DY = dx, dy
	return b
}

func (b *BoundaryLayer) Kind() string { return "boundary" }
