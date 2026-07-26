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

// PictureLayer holds a retained picture (or a re-record flag for P3 raster).
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
