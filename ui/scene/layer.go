//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene

import (
	"sync/atomic"

	"github.com/energye/gpui/render/recording"
)

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

// layerIDGen allocates monotonic ids for boundary / picture layers. It is
// process-global because multiple window Run loops build layer trees
// concurrently (multi-window); a plain uint64 here is a data race and can
// hand out duplicate ids, which corrupts retained dirty tracking.
var layerIDGen atomic.Uint64

// NextLayerID returns a new non-zero layer id.
func NextLayerID() uint64 {
	for {
		id := layerIDGen.Add(1)
		if id != 0 {
			return id
		}
	}
}

// ResetLayerIDGen is for tests only.
func ResetLayerIDGen() { layerIDGen.Store(0) }

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
	// CX, CY is the rotation/scale pivot in layer-local coordinates (the
	// transformed node's content center). Zero values mean the layer origin —
	// callers that know the subtree size should always set the center so the
	// vector paint path and the retained composite path rotate identically.
	CX, CY float64
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
	// CacheKey is the stable cross-frame identity of the originating render
	// object (EnsureCacheID) — per-tree layer ids cannot key result caches.
	CacheKey uint64
}

// SetCacheKey binds the stable render-object identity (layer_build wiring).
func (c *ColorFilterLayer) SetCacheKey(k uint64) { c.CacheKey = k }

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
	// CacheKey is the stable cross-frame identity of the originating render
	// object (EnsureCacheID) — per-tree layer ids cannot key result caches.
	CacheKey uint64
}

// SetCacheKey binds the stable render-object identity (layer_build wiring).
func (i *ImageFilterLayer) SetCacheKey(k uint64) { i.CacheKey = k }

// NewImageFilterLayer creates an image-filter layer with uniform blur radius.
func NewImageFilterLayer(blurRadius float64) *ImageFilterLayer {
	i := &ImageFilterLayer{BlurRadius: blurRadius}
	i.id = NextLayerID()
	return i
}

func (i *ImageFilterLayer) Kind() string { return "image_filter" }

// BackdropFilterLayer snapshots the parent canvas, optionally blurs it, then
// paints children on top (Flutter BackdropFilterLayer subset / FF-BACKDROP).
// Not a clip-local product backdrop; composite uses full-surface PushBackdropLayer.
type BackdropFilterLayer struct {
	ContainerLayer
	// BlurRadius is Gaussian blur applied to the backdrop snapshot (0 = no blur).
	BlurRadius float64
	// Opacity is layer composite opacity (0..1; ≤0 treated as 1 at apply time).
	Opacity float64
}

// NewBackdropFilterLayer creates a backdrop filter layer.
func NewBackdropFilterLayer(blurRadius, opacity float64) *BackdropFilterLayer {
	if opacity <= 0 {
		opacity = 1
	}
	b := &BackdropFilterLayer{BlurRadius: blurRadius, Opacity: opacity}
	b.id = NextLayerID()
	return b
}

func (b *BackdropFilterLayer) Kind() string { return "backdrop_filter" }

// PictureLayer holds a retained display list.
//
// The content is a recording the UI thread produced and sealed: replayable onto
// a context without re-walking the render-object tree, and carrying no live
// object, callback or widget reference. That is what keeps the raster thread
// out of UI state — a layer hands over immutable data, never a function that
// reads shared objects when it runs.
//
// NeedsRaster still tracks dirty vs static reuse in RasterizeDirty
// (flag/stats path — not dirty-rect Present).
type PictureLayer struct {
	id uint64
	// Recording is this layer's display list in layer-local coordinates at
	// scale 1; the raster thread's offset and device scale compose onto it at
	// playback. nil = no content was recorded.
	Recording *recording.Recording
	// NeedsRaster is true when the recording content must be re-drawn (dirty).
	NeedsRaster bool
	// CacheKey is a stable cross-frame identity for retained texture caching
	// (boundary cacheID / RO identity). LayerID is per-tree (rebuilt every
	// frame) so texture caches must key on CacheKey. 0 = not cacheable.
	CacheKey uint64
}

// NewPictureLayer creates a picture layer.
func NewPictureLayer() *PictureLayer {
	return &PictureLayer{id: NextLayerID(), NeedsRaster: true}
}

// SetCacheKey binds a stable cross-frame identity (see CacheKey).
func (p *PictureLayer) SetCacheKey(k uint64) {
	if p == nil {
		return
	}
	p.CacheKey = k
}

func (p *PictureLayer) LayerID() uint64   { return p.id }
func (p *PictureLayer) Children() []Layer { return nil }
func (p *PictureLayer) Kind() string      { return "picture" }

// SetRecording installs this layer's display list. It is a pure content
// install and does not touch NeedsRaster: the builder decides dirt when it
// creates the layer (LayerBuilder.AddPicture(needsRaster), driven by the node's
// NeedsPaint), which is what lets a frame that re-records every node still
// report only the changed ones as dirty. A caller installing content outside a
// builder must mark NeedsRaster itself.
func (p *PictureLayer) SetRecording(rec *recording.Recording) {
	if p == nil {
		return
	}
	p.Recording = rec
}

// TextureLayer paints GPU texture content that no display list can describe
// (Flutter TextureLayer subset). The layer carries only an id: the producer
// (a video route, registered in render's texture registry) owns the texture and
// rewrites it in place, and the raster thread resolves id → source at paint
// time. A layer whose id no longer resolves paints nothing and reports a miss.
//
// Like Flutter's, a texture layer is never raster-cached and repaints every
// frame it is in: its content can change without the UI tree changing at all,
// so the honest damage model is "paint, then damage what was painted" — a cached
// blit of a texture the producer already rewrote would show a stale frame.
type TextureLayer struct {
	id uint64
	// TextureID is render.RegisterTextureSource's id for the source to paint
	// (0 = no source: paints nothing).
	TextureID uint64
	// X, Y, W, H place the quad in the parent's coordinates, like a picture's
	// geometry — the ancestor offset/clip/transform chain still applies.
	X, Y, W, H float64
	// DrawIndex tells the source which of its views this layer shows, in paint
	// order (0 = first). One route feeding a main view plus a picture-in-picture
	// overlay emits one layer per view so "did the second view draw" stays
	// observable per view instead of collapsed into one counter.
	DrawIndex int
	// painted records whether this layer's source has ever drawn through it.
	// A later miss only damages the quad when an earlier frame painted here;
	// a layer whose producer was already gone never dirtied the surface.
	painted bool
}

// NewTextureLayer creates a texture layer for the given source id and rect.
func NewTextureLayer(textureID uint64, x, y, w, h float64) *TextureLayer {
	return &TextureLayer{id: NextLayerID(), TextureID: textureID, X: x, Y: y, W: w, H: h}
}

func (t *TextureLayer) LayerID() uint64   { return t.id }
func (t *TextureLayer) Children() []Layer { return nil }
func (t *TextureLayer) Kind() string      { return "texture" }

// Rect returns the layer's quad in parent coordinates (logical px).
func (t *TextureLayer) Rect() (x, y, w, h float64) {
	if t == nil {
		return 0, 0, 0, 0
	}
	return t.X, t.Y, t.W, t.H
}

// BoundaryLayer is a repaint-boundary root: independent dirty / raster unit.
type BoundaryLayer struct {
	OffsetLayer
	// Source is an optional debug tag (e.g. "spinner").
	Source string
	// Shell marks a window-shell boundary. The
	// retained textured composite partitions its per-frame skip/rerecord by
	// this flag so a scrolling body proves the shell texture never re-records.
	Shell bool
}

// NewBoundaryLayer creates a repaint boundary layer at offset.
func NewBoundaryLayer(dx, dy float64, source string) *BoundaryLayer {
	b := &BoundaryLayer{Source: source}
	b.id = NextLayerID()
	b.DX, b.DY = dx, dy
	return b
}

func (b *BoundaryLayer) Kind() string { return "boundary" }

// SetPivot sets the rotation/scale pivot (subtree content center, layer-local).
func (t *TransformLayer) SetPivot(cx, cy float64) *TransformLayer {
	if t == nil {
		return nil
	}
	t.CX, t.CY = cx, cy
	return t
}
