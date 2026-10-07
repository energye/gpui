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

	gpucontext "github.com/energye/gpui/gpu/context"
)

// newLayerStack creates a new layer stack with a pool for memory reuse.
func newLayerStack() *layerStack {
	return &layerStack{
		layers: make([]*Layer, 0, 4),
		pool:   newPixmapPool(8),
	}
}

func (c *Context) LayerPoolStats() (gets, puts, hits, misses int) {
	if c == nil || c.layerStack == nil || c.layerStack.pool == nil {
		return 0, 0, 0, 0
	}
	return c.layerStack.pool.Stats()
}

// ResetLayerPoolStats clears layer pool counters (tests).
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
//	dc.PushLayer(render.BlendMultiply, 0.5)
//	dc.SetRGB(1, 0, 0)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.PopLayer() // Composite circle onto canvas with multiply blend at 50% opacity
//
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
//	dc.PushLayer(render.BlendMultiply, 0.5)
//	dc.SetRGB(1, 0, 0)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.PopLayer() // Composite circle onto canvas with multiply blend at 50% opacity
func (c *Context) PushLayer(blendMode BlendMode, opacity float64) {

	c.pushLayerSurface(blendMode, opacity, true, false)
}

// PushLayerIsolated is like PushLayer(BlendNormal, opacity) but always allocates
// an offscreen isolation surface (Flutter Canvas.saveLayer semantics).
// Use for overlapping draws within a group; prefer PushLayer(Normal) for cheap
// non-overlapping opacity cards.
// PushLayerIsolated is like PushLayer(BlendNormal, opacity) but always allocates
// an offscreen isolation surface (Flutter Canvas.saveLayer semantics).
// Use for overlapping draws within a group; prefer PushLayer(Normal) for cheap
// non-overlapping opacity cards.
func (c *Context) PushLayerIsolated(opacity float64) {
	c.pushLayerSurface(BlendNormal, opacity, true, true)
}

// pushLayerSurface is the shared PushLayer implementation.
// clear=true for normal layers; false when the caller will fully overwrite (backdrop).
// forceIsolated skips the F1 opacity-group fast path (true saveLayer).
// pushLayerSurface is the shared PushLayer implementation.
// clear=true for normal layers; false when the caller will fully overwrite (backdrop).
// forceIsolated skips the F1 opacity-group fast path (true saveLayer).
func (c *Context) pushLayerSurface(blendMode BlendMode, opacity float64, clear, forceIsolated bool) {
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

	// Create layer shell.
	layer := &Layer{
		blendMode: blendMode,
		opacity:   opacity,
	}

	// F1 Normal/Copy opacity-group: skip isolation RT and multiply paint alpha
	// (layerOpacityMul). Matches CSS-style group opacity for non-overlapping
	// SourceOver UI cards (PKS MULTI_LAYER). Overlapping multi-draw groups that
	// need Skia SaveLayer isolation use PushLayerIsolated (forceIsolated).
	//
	// Backdrop layers (clear=false) always need an isolation surface so the
	// parent snapshot can be copied; opacity-group would skip the pool and break
	// PushBackdropLayer.
	if clear && !forceIsolated && (blendMode == BlendNormal || blendMode == BlendCopy) {
		layer.opacityGroup = true
		c.layerStack.layers = append(c.layerStack.layers, layer)
		return
	}

	// Advanced blend / mask path: isolation RT (GPU preferred).
	pw, ph := c.width, c.height
	if pw > 0 && ph > 0 {
		view, release := c.CreateOffscreenTexture(pw, ph)
		if !view.IsNil() && release != nil {
			layer.gpuView = view
			layer.gpuRelease = release
			layer.gpuW = pw
			layer.gpuH = ph
		}
	}

	// Acquire layer surface from pool.
	var layerPixmap *Pixmap
	if layer.gpuView.IsNil() {
		if clear {
			layerPixmap = c.layerStack.pool.Get(c.width, c.height)
		} else {
			layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
		}
	} else {
		layerPixmap = c.layerStack.pool.GetForOverwrite(c.width, c.height)
	}
	layer.pixmap = layerPixmap

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
//	dc.PushLayer(render.BlendScreen, 1.0)
//	// ... draw operations ...
//	dc.PopLayer() // Composite layer onto parent
//
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
//	dc.PushLayer(render.BlendScreen, 1.0)
//	// ... draw operations ...
//	dc.PopLayer() // Composite layer onto parent
func (c *Context) PopLayer() {

	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		return
	}

	// Pop the current layer (keep fields for composite/release).
	layers := c.layerStack.layers
	layer := layers[len(layers)-1]
	c.layerStack.layers = layers[:len(layers)-1]

	// F1 opacity-group: nothing to composite — draws already hit the parent
	// with multiplied alpha.
	if layer.opacityGroup {
		if len(c.layerStack.layers) == 0 {
			c.basePixmap = nil
		}
		return
	}

	// Finish any pending draws that targeted this layer's GPU RT first.
	// Must happen before restoring the parent target so gpuRenderTarget still
	// points at the layer view if FlushGPU is used as a fallback.
	if !layer.gpuView.IsNil() {
		if rc := c.gpuCtxOps(); rc != nil && rc.PendingCount() > 0 {
			// F1: damage-aware flush when dirty rect is a real subset of the layer.
			// Near-full damage (e.g. particle mesh covering the stage) falls back to
			// full flush — damage path has setup cost without reducing encode work.
			if !layer.fullComposite && !layer.damage.Empty() && layer.gpuW > 0 && layer.gpuH > 0 {
				const pad = 2
				full := image.Rect(0, 0, layer.gpuW, layer.gpuH)
				d := layer.damage.Inset(-pad).Intersect(full)
				useDamage := !d.Empty()
				if useDamage {
					// area ratio; int math avoids float on hot path
					fullA := layer.gpuW * layer.gpuH
					if fullA > 0 && d.Dx()*d.Dy()*10 >= fullA*7 { // ≥70%
						useDamage = false
					}
				}
				if useDamage {
					_ = c.FlushGPUWithViewDamage(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH), d) //nolint:gosec
				} else {
					_ = c.FlushGPUWithView(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH)) //nolint:gosec
				}
			} else {
				_ = c.FlushGPUWithView(layer.gpuView, uint32(layer.gpuW), uint32(layer.gpuH)) //nolint:gosec // bounded
			}
		}
	}
	// F1: CPU Normal layers already wrote into layer.pixmap via forceCPULayer.
	// Do NOT FlushGPU here — parent/stage pending ops would be resolved into the
	// layer pixmap and force full-surface submits every Pop (MULTI_LAYER ~10fps).

	// Get parent pixmap (previous isolation layer or base). Opacity-group
	// ancestors keep pixmap==nil — walk past them to the real surface.
	var parentPixmap *Pixmap
	if len(c.layerStack.layers) > 0 {
		for i := len(c.layerStack.layers) - 1; i >= 0; i-- {
			L := c.layerStack.layers[i]
			if L == nil || L.opacityGroup {
				continue
			}
			if L.pixmap != nil {
				parentPixmap = L.pixmap
				break
			}
		}
		if parentPixmap == nil {
			parentPixmap = c.basePixmap
			if parentPixmap == nil {
				parentPixmap = c.pixmap
			}
		}
	} else {
		// Restore base pixmap
		parentPixmap = c.basePixmap
		c.basePixmap = nil
	}

	// Restore parent as the active drawing target BEFORE compositing so
	// DrawGPUTexture* / composite write into the parent, not the child.
	if parentPixmap != nil {
		c.pixmap = parentPixmap
	}

	// Apply mask to layer content before compositing (PushMaskLayer).
	// Prefer GPU R8 modulate composite when layer content is on a GPU RT.
	maskedGPUDone := false
	if layer.mask != nil {
		layer.fullComposite = true
		// Parent draws may still be present-stashed (queued before PushMaskLayer).
		// CompositeMaskedLayer / applyMask write into parent.Data — materialize the
		// stashed base first so final Image/Flush does not paint base over the mask.
		if parentPixmap != nil {
			_ = c.FlushGPU()
		}
		if !layer.gpuView.IsNil() && !layer.cpuDrew && parentPixmap != nil {
			if c.compositeLayerMaskedGPU(layer, parentPixmap) {
				maskedGPUDone = true
				// CompositeMaskedLayer writes parent.Data (CPU pixmap), not a parent
				// GPU RT. Nested PushMaskLayer parents still hold an empty gpuView —
				// mark them cpuDrew so the next Pop uses pixmap + mask path.
				if c.layerStack != nil {
					for i := len(c.layerStack.layers) - 1; i >= 0; i-- {
						L := c.layerStack.layers[i]
						if L == nil || L.opacityGroup {
							continue
						}
						if L.pixmap == parentPixmap {
							L.cpuDrew = true
							break
						}
					}
				}
				if layer.gpuRelease != nil {
					layer.gpuRelease()
					layer.gpuRelease = nil
				}
				layer.gpuView = gpucontext.TextureView{}
			}
		}
		if !maskedGPUDone {
			// Materialize GPU content into pixmap, then CPU DestinationIn mask.
			if !layer.gpuView.IsNil() && !layer.cpuDrew {
				_ = c.materializeLayerGPUToPixmap(layer)
			}
			c.applyMaskToPixmap(layer.pixmap, layer.mask)
		}
	}

	// GPU composite path: no mask, no CPU writes into the layer pixmap.
	// Content lives on layer.gpuView.
	canGPUTexture := !layer.gpuView.IsNil() && !layer.cpuDrew && layer.mask == nil && !maskedGPUDone
	canGPUComposite := canGPUTexture &&
		(layer.blendMode == BlendNormal || layer.blendMode == BlendCopy)
	canGPUAdvanced := canGPUTexture && IsAdvancedBlendMode(layer.blendMode)

	if canGPUAdvanced {
		// P0-3: defer dual-tex to Present Flush (keep layer RT until resolve).
		if c.queueLayerAdvancedGPU(layer, parentPixmap) {
			layer.gpuRelease = nil
			layer.gpuView = gpucontext.TextureView{}
		} else {
			// Fallback: release GPU RT and composite empty/CPU pixmap.
			if layer.gpuRelease != nil {
				layer.gpuRelease()
				layer.gpuRelease = nil
			}
			layer.gpuView = gpucontext.TextureView{}
			c.compositeLayer(layer, parentPixmap)
		}
	} else if canGPUComposite {
		// Layer RT is already in device/pixel space. Temporarily clear CTM so
		// the full-surface blit is not re-transformed by the user matrix.
		savedMat := c.matrix
		savedDev := c.deviceMatrix
		c.matrix = Identity()
		c.deviceMatrix = Identity()
		op := float32(layer.opacity)
		if op < 0 {
			op = 0
		}
		if op > 1 {
			op = 1
		}
		// F1: damage-tight GPU composite — only blit the dirty UV rect instead
		// of the full layer RT (800x600 full-surface was the MULTI_LAYER tax).
		if !layer.fullComposite && !layer.damage.Empty() && layer.gpuW > 0 && layer.gpuH > 0 {
			const pad = 2
			d := layer.damage.Inset(-pad)
			d = d.Intersect(image.Rect(0, 0, layer.gpuW, layer.gpuH))
			if !d.Empty() {
				c.DrawGPUTextureWithOpacityUV(layer.gpuView,
					float64(d.Min.X), float64(d.Min.Y), d.Dx(), d.Dy(), op,
					float32(d.Min.X)/float32(layer.gpuW),
					float32(d.Min.Y)/float32(layer.gpuH),
					float32(d.Max.X)/float32(layer.gpuW),
					float32(d.Max.Y)/float32(layer.gpuH),
				)
			}
		} else {
			c.DrawGPUTextureWithOpacity(layer.gpuView, 0, 0, layer.gpuW, layer.gpuH, op)
		}
		c.matrix = savedMat
		c.deviceMatrix = savedDev
		// Keep the texture alive until the composite command is flushed.
		if layer.gpuRelease != nil {
			c.layerGPUReleases = append(c.layerGPUReleases, layer.gpuRelease)
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}

		// (or CPU parent). The GPU texture blit stays queued and materializes on
		// the next FlushGPU / Image / PresentFrame — single submit per frame.
		// Callers that sample pixmap.GetPixel without Flush must FlushGPU first
		// (Image() already flushes). Nested GPU parents already batch.
	} else if !maskedGPUDone {
		// CPU composite (advanced blend, mask, CPU-drawn content, or no GPU).
		if layer.gpuRelease != nil {
			layer.gpuRelease()
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}
		c.compositeLayer(layer, parentPixmap)
	} else {
		// Masked GPU composite already wrote parent; just drop any residual RT.
		if layer.gpuRelease != nil {
			layer.gpuRelease()
			layer.gpuRelease = nil
		}
		layer.gpuView = gpucontext.TextureView{}
	}

	// Return layer surface to pool.
	if c.layerStack.pool != nil {
		c.layerStack.pool.Put(layer.pixmap)
	}
	layer.pixmap = nil
	layer.mask = nil
}

// PushMaskLayer creates an isolated layer with an associated alpha mask.
// All subsequent drawing operations render to this layer normally (without masking).
// When PopLayer is called, the ENTIRE layer is masked by the mask and then
// composited onto the parent using source-over blending with full opacity.
//
// This produces different results from SetMask: PushMaskLayer masks the
// composited group, while SetMask masks each shape individually.
//
// Example:
//
//	mask := render.NewMaskFromAlpha(maskImage)
//	dc.PushMaskLayer(mask)
//	dc.DrawCircle(100, 100, 50)
//	dc.Fill()
//	dc.DrawRect(80, 80, 40, 40)
//	dc.Fill()
//	dc.PopLayer() // entire layer content masked, then composited
//
// PushMaskLayer creates an isolated layer with an associated alpha mask.
// All subsequent drawing operations render to this layer normally (without masking).
// When PopLayer is called, the ENTIRE layer is masked by the mask and then
// composited onto the parent using source-over blending with full opacity.
//
// This produces different results from SetMask: PushMaskLayer masks the
// composited group, while SetMask masks each shape individually.
//
// Example:
//
//	mask := render.NewMaskFromAlpha(maskImage)
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

	// Acquire layer surface from pool.
	layerPixmap := c.layerStack.pool.Get(c.width, c.height)

	// Create layer with mask.
	layer := &Layer{
		pixmap:    layerPixmap,
		blendMode: BlendNormal,
		opacity:   1.0,
		mask:      mask,
	}

	// R1 residual fix: mask layers also get a GPU RT so in-layer Fill/Stroke
	// stay on GPU. Pop uses CompositeMaskedLayer or CPU mask.
	pw, ph := layerPixmap.Width(), layerPixmap.Height()
	if pw > 0 && ph > 0 {
		view, release := c.CreateOffscreenTexture(pw, ph)
		if !view.IsNil() && release != nil {
			layer.gpuView = view
			layer.gpuRelease = release
			layer.gpuW = pw
			layer.gpuH = ph
		}
	}

	// Switch to layer pixmap.
	c.layerStack.layers = append(c.layerStack.layers, layer)
	c.pixmap = layerPixmap
}

// applyMaskToPixmap applies a DestinationIn mask to a pixmap's pixel data.
// For each pixel: all channels are scaled by mask.At(x,y) / 255.
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
//	dc.SetBlendMode(render.BlendMultiply)
//	dc.Fill() // Future: will use multiply blend mode
