package render

import (
	intImage "github.com/energye/gpui/render/internal/image"
)

// Layer represents a drawing layer with blend mode and opacity.
// Layers allow isolating drawing operations and compositing them with
// different blend modes and opacity values, similar to layers in Photoshop
// or SVG group opacity.
type Layer struct {
	pixmap    *Pixmap
	blendMode BlendMode
	opacity   float64
	mask      *Mask // optional alpha mask, applied on PopLayer (nil = no mask)
}

// layerStack manages the layer hierarchy for the context.
type layerStack struct {
	layers []*Layer
	// pool reuses full-surface layer Pixmaps (S6.4). intImage.Pool is ImageBuf-only.
	pool *pixmapPool
}

// newLayerStack creates a new layer stack with a pool for memory reuse.
func newLayerStack() *layerStack {
	return &layerStack{
		layers: make([]*Layer, 0, 4),
		pool:   newPixmapPool(8),
	}
}

// LayerPoolStats returns S6.4 layer pixmap pool counters for the context.
func (c *Context) LayerPoolStats() (gets, puts, hits, misses int) {
	if c == nil || c.layerStack == nil || c.layerStack.pool == nil {
		return 0, 0, 0, 0
	}
	return c.layerStack.pool.Stats()
}

// ResetLayerPoolStats clears layer pool counters (tests).
func (c *Context) ResetLayerPoolStats() {
	if c == nil || c.layerStack == nil || c.layerStack.pool == nil {
		return
	}
	c.layerStack.pool.ResetStats()
}

// PushLayer creates a new layer and makes it the active drawing target.
// All subsequent drawing operations will render to this layer until PopLayer is called.
//
// The layer will be composited onto the parent layer/canvas when PopLayer is called,
// using the specified blend mode and opacity.
//
// Parameters:
//   - blendMode: How to composite this layer onto the parent (e.g., BlendMultiply, BlendScreen)
//   - opacity: Layer opacity in range [0.0, 1.0] where 0 is fully transparent and 1 is fully opaque
//
// Example:
//
//	dc.PushLayer(gg.BlendMultiply, 0.5)
//	dc.SetRGB(1, 0, 0)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.PopLayer() // Composite circle onto canvas with multiply blend at 50% opacity
func (c *Context) PushLayer(blendMode BlendMode, opacity float64) {
	c.pushLayerSurface(blendMode, opacity, true)
}

// pushLayerSurface is the shared PushLayer implementation.
// clear=true for normal layers; false when the caller will fully overwrite (backdrop).
func (c *Context) pushLayerSurface(blendMode BlendMode, opacity float64, clear bool) {
	// Clamp opacity to valid range
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}

	// Initialize layer stack if needed
	if c.layerStack == nil {
		c.layerStack = newLayerStack()
	}

	// Save base pixmap on first push
	if len(c.layerStack.layers) == 0 && c.basePixmap == nil {
		c.basePixmap = c.pixmap
	}

	// Acquire layer surface from pool (S6.4: avoid per-push NewPixmap).
	var layerPixmap *Pixmap
	if clear {
		layerPixmap = c.layerStack.pool.Get(c.width, c.height)
	} else {
		layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
	}

	// Create layer
	layer := &Layer{
		pixmap:    layerPixmap,
		blendMode: blendMode,
		opacity:   opacity,
	}

	// Save current pixmap and switch to layer pixmap
	c.layerStack.layers = append(c.layerStack.layers, layer)
	c.pixmap = layerPixmap
}

// PopLayer composites the current layer onto the parent layer/canvas.
// Uses the blend mode and opacity specified in the corresponding PushLayer call.
//
// The layer is composited using the specified blend mode and opacity.
// After compositing, the layer's memory is returned to the pool for reuse.
//
// If there are no layers to pop, this function does nothing.
//
// Example:
//
//	dc.PushLayer(gg.BlendScreen, 1.0)
//	// ... draw operations ...
//	dc.PopLayer() // Composite layer onto parent
func (c *Context) PopLayer() {
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}

	// Pop the current layer
	layers := c.layerStack.layers
	layer := layers[len(layers)-1]
	c.layerStack.layers = layers[:len(layers)-1]

	// Get parent pixmap (either previous layer or base)
	var parentPixmap *Pixmap
	if len(c.layerStack.layers) > 0 {
		parentPixmap = c.layerStack.layers[len(c.layerStack.layers)-1].pixmap
	} else {
		// Restore base pixmap
		parentPixmap = c.basePixmap
		c.basePixmap = nil
	}

	// Apply mask to layer content before compositing (PushMaskLayer).
	if layer.mask != nil {
		c.applyMaskToPixmap(layer.pixmap, layer.mask)
	}

	// Composite layer onto parent
	c.compositeLayer(layer, parentPixmap)

	// Return layer surface to pool (S6.4).
	if c.layerStack.pool != nil {
		c.layerStack.pool.Put(layer.pixmap)
	}
	layer.pixmap = nil
	layer.mask = nil

	// Restore parent pixmap as current drawing target
	c.pixmap = parentPixmap
}

// PushMaskLayer creates an isolated layer with an associated alpha mask.
// All subsequent drawing operations render to this layer normally (without masking).
// When PopLayer is called, the ENTIRE layer is masked by the mask and then
// composited onto the parent using source-over blending with full opacity.
//
// This produces different results from SetMask: PushMaskLayer masks the
// composited group, while SetMask masks each shape individually.
//
// Matches Vello push_mask_layer() semantics (research §4).
//
// Example:
//
//	mask := gg.NewMaskFromAlpha(maskImage)
//	dc.PushMaskLayer(mask)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.DrawRect(80, 80, 40, 40)
//	dc.Fill()
//	dc.PopLayer() // entire layer content masked, then composited
func (c *Context) PushMaskLayer(mask *Mask) {
	// Clamp: nil mask means no masking (equivalent to PushLayer).
	if mask == nil {
		c.PushLayer(BlendNormal, 1.0)
		return
	}

	// Initialize layer stack if needed.
	if c.layerStack == nil {
		c.layerStack = newLayerStack()
	}

	// Save base pixmap on first push.
	if len(c.layerStack.layers) == 0 && c.basePixmap == nil {
		c.basePixmap = c.pixmap
	}

	// Acquire layer surface from pool (S6.4).
	layerPixmap := c.layerStack.pool.Get(c.width, c.height)

	// Create layer with mask.
	layer := &Layer{
		pixmap:    layerPixmap,
		blendMode: BlendNormal,
		opacity:   1.0,
		mask:      mask,
	}

	// Switch to layer pixmap.
	c.layerStack.layers = append(c.layerStack.layers, layer)
	c.pixmap = layerPixmap
}

// applyMaskToPixmap applies a DestinationIn mask to a pixmap's pixel data.
// For each pixel: all channels are scaled by mask.At(x,y) / 255.
func (c *Context) applyMaskToPixmap(pm *Pixmap, mask *Mask) {
	applyMaskToPixmapData(pm, mask)
}

// SetBlendMode sets the blend mode for subsequent fill and stroke operations.
//
// GPU fixed-function modes (B.02): BlendNormal (SourceOver), BlendCopy, BlendClear,
// BlendPlus, DstOut/SrcAtop/Xor, DstOver/SrcIn/SrcOut/DstIn/DstAtop.
// Advanced modes (Multiply/Screen/…) use resolve+CPU composite+GPU blit or CPU.
//
// Example:
//
//	dc.SetBlendMode(gg.BlendMultiply)
//	dc.Fill() // Future: will use multiply blend mode
func (c *Context) SetBlendMode(mode BlendMode) {
	c.paint.BlendMode = mode
}

// compositeLayer composites a layer onto a parent pixmap using the layer's
// blend mode and opacity.
func (c *Context) compositeLayer(layer *Layer, parent *Pixmap) {
	// Convert pixmaps to ImageBuf for blending
	srcImg := c.pixmapToImageBuf(layer.pixmap)
	dstImg := c.pixmapToImageBuf(parent)

	// Use DrawImage to composite with blend mode and opacity
	srcW, srcH := srcImg.Bounds()

	params := intImage.DrawParams{
		DstRect: intImage.Rect{
			X:      0,
			Y:      0,
			Width:  srcW,
			Height: srcH,
		},
		Interp:    intImage.InterpNearest, // No scaling, so nearest is fine
		Opacity:   layer.opacity,
		BlendMode: layer.blendMode,
	}

	intImage.DrawImage(dstImg, srcImg, params)
}
