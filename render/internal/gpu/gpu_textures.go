//go:build !nogpu

package gpu

import (
	"fmt"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// textureSet holds a set of MSAA color, depth/stencil, and resolve textures
// for offscreen rendering. This is shared by GPURenderSession and
// StencilRenderer to avoid code duplication.
//
// The texture set supports stencil-then-cover and SDF rendering:
//   - MSAA color: 4x samples, BGRA8Unorm, RenderAttachment
//   - Depth/stencil: 4x samples, Depth24PlusStencil8, RenderAttachment
//   - Resolve: 1x sample, BGRA8Unorm, RenderAttachment | CopySrc
type textureSet struct {
	msaaTex     *webgpu.Texture
	msaaView    *webgpu.TextureView
	stencilTex  *webgpu.Texture
	stencilView *webgpu.TextureView
	resolveTex  *webgpu.Texture
	resolveView *webgpu.TextureView
	width       uint32
	height      uint32
}

// ensureTextures creates or recreates textures if the requested dimensions
// differ from the current size. If dimensions match and textures exist,
// this is a no-op. The labelPrefix parameter distinguishes GPU debug labels
// between different owners (e.g., "session" vs "stencil").
//
// The samples parameter sets the MSAA sample count for color and depth/stencil
// textures (typically 4 for MSAA, 1 for non-MSAA fallback).
func (ts *textureSet) ensureTextures(device *webgpu.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	if device == nil {
		return fmt.Errorf("ensureTextures: device is nil")
	}
	sc := uint32(4) // default MSAA sample count
	if len(samples) > 0 && samples[0] > 0 {
		sc = samples[0]
	}

	// Offscreen mode requires a resolve texture for readback. After a surface-mode
	// pass (ensureSurfaceTextures), msaa/stencil may exist at the same size with
	// resolveTex == nil. Do not early-return in that state or encodeSubmitReadback
	// fails with "offscreen textures destroyed (concurrent resize?)".
	// sc==1 does not allocate msaa color (colorAttachment draws to resolve directly).
	needMSAA := sc > 1
	haveMSAA := ts.msaaTex != nil && ts.msaaView != nil
	if ts.width == w && ts.height == h && ts.resolveTex != nil && ts.resolveView != nil &&
		ts.stencilView != nil && (!needMSAA || haveMSAA) {
		return nil
	}
	ts.destroyTextures()

	size := webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}

	if needMSAA {
		msaaTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         labelPrefix + "_msaa_color",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   sc,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			return fmt.Errorf("create MSAA color texture: %w", err)
		}
		ts.msaaTex = msaaTex

		msaaView, err := device.CreateTextureView(msaaTex, &webgpu.TextureViewDescriptor{
			Label:         labelPrefix + "_msaa_color_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create MSAA color view: %w", err)
		}
		ts.msaaView = msaaView
	}

	// Depth/stencil texture (sc samples, Depth24PlusStencil8).
	stencilTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil texture: %w", err)
	}
	ts.stencilTex = stencilTex

	stencilView, err := device.CreateTextureView(stencilTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil view: %w", err)
	}
	ts.stencilView = stencilView

	// Single-sample resolve target (CopySrc for readback). For sc==1 this is
	// also the color attachment (no MSAA resolve).
	resolveTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         labelPrefix + "_resolve",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create resolve texture: %w", err)
	}
	ts.resolveTex = resolveTex

	resolveView, err := device.CreateTextureView(resolveTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_resolve_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create resolve view: %w", err)
	}
	ts.resolveView = resolveView

	ts.width = w
	ts.height = h
	slogger().Info("created offscreen textures",
		"label", labelPrefix,
		"width", w, "height", h,
		"msaa_samples", sc,
		"msaa_color", needMSAA,
	)
	return nil
}

func (ts *textureSet) ensureSurfaceTextures(device *webgpu.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	sc := uint32(4) // default MSAA sample count
	if len(samples) > 0 && samples[0] > 0 {
		sc = samples[0]
	}
	needMSAA := sc > 1
	haveMSAA := ts.msaaTex != nil && ts.msaaView != nil

	// Surface mode does not use resolveTex. sc==1 draws directly to the surface
	// view, so only depth/stencil is required (positive VRAM save on low-GPU hosts).
	if ts.width == w && ts.height == h && ts.stencilView != nil && (!needMSAA || haveMSAA) {
		if ts.resolveView != nil {
			ts.resolveView.Release()
			ts.resolveView = nil
		}
		if ts.resolveTex != nil {
			ts.resolveTex.Release()
			ts.resolveTex = nil
		}
		// Drop leftover MSAA when switching into sc==1.
		if !needMSAA && haveMSAA {
			if ts.msaaView != nil {
				ts.msaaView.Release()
				ts.msaaView = nil
			}
			if ts.msaaTex != nil {
				ts.msaaTex.Release()
				ts.msaaTex = nil
			}
		}
		return nil
	}
	ts.destroyTextures()

	size := webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}

	if needMSAA {
		msaaTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label:         labelPrefix + "_msaa_color",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   sc,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			return fmt.Errorf("create MSAA color texture: %w", err)
		}
		ts.msaaTex = msaaTex

		msaaView, err := device.CreateTextureView(msaaTex, &webgpu.TextureViewDescriptor{
			Label:         labelPrefix + "_msaa_color_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create MSAA color view: %w", err)
		}
		ts.msaaView = msaaView
	}

	stencilTex, err := device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil texture: %w", err)
	}
	ts.stencilTex = stencilTex

	stencilView, err := device.CreateTextureView(stencilTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil view: %w", err)
	}
	ts.stencilView = stencilView

	// No resolve texture -- surface view is the resolve target.
	ts.width = w
	ts.height = h
	slogger().Info("created surface textures",
		"label", labelPrefix,
		"width", w, "height", h,
		"msaa_samples", sc,
		"msaa_color", needMSAA,
	)
	return nil
}

func (ts *textureSet) destroyTextures() {
	if ts.resolveView != nil {
		ts.resolveView.Release()
		ts.resolveView = nil
	}
	if ts.resolveTex != nil {
		ts.resolveTex.Release()
		ts.resolveTex = nil
	}
	if ts.stencilView != nil {
		ts.stencilView.Release()
		ts.stencilView = nil
	}
	if ts.stencilTex != nil {
		ts.stencilTex.Release()
		ts.stencilTex = nil
	}
	if ts.msaaView != nil {
		ts.msaaView.Release()
		ts.msaaView = nil
	}
	if ts.msaaTex != nil {
		ts.msaaTex.Release()
		ts.msaaTex = nil
	}
	ts.width = 0
	ts.height = 0
}
