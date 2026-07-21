//go:build !nogpu

package gpu

import (
	"fmt"
	"log"
	"os"
	"strings"

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
		msaaTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
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
	stencilTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		// Post-TDR / AutoRecover: full-size depth may OOM while device heap is
		// still reclaiming. Fall back to 1x1 depth (stencil Always/Keep still valid).
		log.Printf("depth %dx%d samples=%d OOM, falling back to 1x1: %v", size.Width, size.Height, sc, err)
		stencilTex, err = createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_depth_stencil_1x1",
			Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatDepth24PlusStencil8,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create depth/stencil texture: %w", err)
		}
		// Force recreate next frame when heap has reclaimed (size mismatch path).
		ts.width, ts.height = 0, 0
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
	resolveTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
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
		msaaTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
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

	stencilTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		// Post-TDR / AutoRecover: full-size depth may OOM while device heap is
		// still reclaiming. Fall back to 1x1 depth (stencil Always/Keep still valid).
		log.Printf("depth %dx%d samples=%d OOM, falling back to 1x1: %v", size.Width, size.Height, sc, err)
		stencilTex, err = createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_depth_stencil_1x1",
			Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatDepth24PlusStencil8,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create depth/stencil texture: %w", err)
		}
		// Force recreate next frame when heap has reclaimed (size mismatch path).
		ts.width, ts.height = 0, 0
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

// createTextureRetryOOM creates a texture; on OOM-like errors flushes and retries.
// Second try forces SampleCount=1 when the failed desc used MSAA (post-TDR reclaim).
func createTextureRetryOOM(device *webgpu.Device, desc *webgpu.TextureDescriptor) (*webgpu.Texture, error) {
	if device == nil || desc == nil {
		return nil, fmt.Errorf("createTextureRetryOOM: nil device/desc")
	}
	if os.Getenv("GPUI_LOG_TEXTURE") == "1" {
		sc := desc.SampleCount
		if sc == 0 {
			sc = 1
		}
		// Rough VRAM estimate: BGRA8=4B, depth24+stencil≈4B per sample.
		bpp := uint64(4)
		est := uint64(desc.Size.Width) * uint64(desc.Size.Height) * uint64(sc) * bpp
		log.Printf("TEX_CREATE label=%q %dx%d samples=%d est_mib=%.2f usage=%d",
			desc.Label, desc.Size.Width, desc.Size.Height, sc, float64(est)/(1024*1024), desc.Usage)
	}
	tex, err := device.CreateTexture(desc)
	if err == nil {
		return tex, nil
	}
	low := strings.ToLower(err.Error())
	if !strings.Contains(low, "not enough memory") && !strings.Contains(low, "out of memory") {
		return nil, err
	}
	log.Printf("CreateTexture OOM label=%s %dx%d samples=%d", desc.Label, desc.Size.Width, desc.Size.Height, desc.SampleCount)
	noteTextureOOM()
	device.FlushCallbacks()
	_ = device.WaitIdle()
	// Retry original once after flush.
	tex, err2 := device.CreateTexture(desc)
	if err2 == nil {
		return tex, nil
	}
	// Last resort: drop MSAA for this allocation.
	if desc.SampleCount > 1 {
		d2 := *desc
		d2.SampleCount = 1
		return device.CreateTexture(&d2)
	}
	return nil, err2
}
