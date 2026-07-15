//go:build !(js && wasm)

package webgpu

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
)

// Swapchain manages Configure → GetCurrentTexture → Present for a platform Surface (S.03).
// It is the production path for window presentation; offscreen textures remain a headless stand-in.
type Swapchain struct {
	Surface     *Surface
	Device      *Device
	Width       uint32
	Height      uint32
	Format      TextureFormat
	Usage       TextureUsage
	PresentMode PresentMode
	AlphaMode   CompositeAlphaMode

	configured bool
}

// Frame is one acquired swapchain image ready for rendering.
type Frame struct {
	SurfaceTexture *SurfaceTexture
	View           *TextureView
	// Handle is the gpucontext-facing view for render.FlushGPUWithView.
	Handle     gpucontext.TextureView
	Suboptimal bool
	Width      uint32
	Height     uint32
}

// NewSwapchain builds a swapchain for an existing surface + device.
// Call Configure before BeginFrame.
func NewSwapchain(surface *Surface, device *Device, width, height uint32) *Swapchain {
	return &Swapchain{
		Surface:     surface,
		Device:      device,
		Width:       width,
		Height:      height,
		Format:      types.TextureFormatBGRA8Unorm,
		Usage:       types.TextureUsageRenderAttachment,
		PresentMode: PresentModeFifo,
		AlphaMode:   types.CompositeAlphaModeOpaque,
	}
}

// Configure applies SurfaceConfiguration to the underlying surface.
// Prefer choosing Format/PresentMode from Adapter.GetSurfaceCapabilities when available.
func (sc *Swapchain) Configure() error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if sc.Surface == nil {
		return fmt.Errorf("wgpu: swapchain surface is nil")
	}
	if sc.Device == nil {
		return fmt.Errorf("wgpu: swapchain device is nil")
	}
	if sc.Width == 0 || sc.Height == 0 {
		return fmt.Errorf("wgpu: swapchain extent must be non-zero")
	}
	if sc.Usage == 0 {
		sc.Usage = types.TextureUsageRenderAttachment
	}
	if sc.Format == 0 {
		sc.Format = types.TextureFormatBGRA8Unorm
	}
	if sc.PresentMode == 0 {
		sc.PresentMode = PresentModeFifo
	}
	if sc.AlphaMode == 0 {
		sc.AlphaMode = types.CompositeAlphaModeOpaque
	}
	cfg := &SurfaceConfiguration{
		Width:       sc.Width,
		Height:      sc.Height,
		Format:      sc.Format,
		Usage:       sc.Usage,
		PresentMode: sc.PresentMode,
		AlphaMode:   sc.AlphaMode,
	}
	if err := sc.Surface.Configure(sc.Device, cfg); err != nil {
		return err
	}
	sc.configured = true
	return nil
}

// ConfigureFromCapabilities picks a supported format/present/alpha mode then configures.
func (sc *Swapchain) ConfigureFromCapabilities(adapter *Adapter) error {
	if sc == nil || adapter == nil {
		return fmt.Errorf("wgpu: swapchain/adapter nil")
	}
	caps := adapter.GetSurfaceCapabilities(sc.Surface)
	if caps != nil {
		if len(caps.Formats) > 0 {
			sc.Format = caps.Formats[0]
			// Prefer BGRA8Unorm when listed (common on Windows/Linux).
			for _, f := range caps.Formats {
				if f == types.TextureFormatBGRA8Unorm || f == types.TextureFormatRGBA8Unorm {
					sc.Format = f
					break
				}
			}
		}
		if len(caps.PresentModes) > 0 {
			sc.PresentMode = caps.PresentModes[0]
			for _, pm := range caps.PresentModes {
				if pm == PresentModeFifo {
					sc.PresentMode = pm
					break
				}
			}
		}
		if len(caps.AlphaModes) > 0 {
			sc.AlphaMode = caps.AlphaModes[0]
			for _, am := range caps.AlphaModes {
				if am == types.CompositeAlphaModeOpaque {
					sc.AlphaMode = am
					break
				}
			}
		}
	}
	return sc.Configure()
}

// Resize updates extent and reconfigures.
func (sc *Swapchain) Resize(width, height uint32) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	sc.Width = width
	sc.Height = height
	return sc.Configure()
}

// BeginFrame acquires the next surface texture and creates a render view.
// Caller must EndFrame (or DiscardFrame) exactly once per successful BeginFrame.
func (sc *Swapchain) BeginFrame() (*Frame, error) {
	if sc == nil {
		return nil, fmt.Errorf("wgpu: swapchain is nil")
	}
	if !sc.configured {
		if err := sc.Configure(); err != nil {
			return nil, err
		}
	}
	st, suboptimal, err := sc.Surface.GetCurrentTexture()
	if err != nil {
		return nil, err
	}
	view, err := st.CreateView(nil)
	if err != nil {
		return nil, fmt.Errorf("wgpu: surface texture CreateView: %w", err)
	}
	return &Frame{
		SurfaceTexture: st,
		View:           view,
		Handle:         TextureViewToHandle(view),
		Suboptimal:     suboptimal,
		Width:          sc.Width,
		Height:         sc.Height,
	}, nil
}

// EndFrame presents the frame to the platform surface.
func (sc *Swapchain) EndFrame(frame *Frame) error {
	if sc == nil {
		return fmt.Errorf("wgpu: swapchain is nil")
	}
	if frame == nil {
		return fmt.Errorf("wgpu: frame is nil")
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	err := sc.Surface.Present(frame.SurfaceTexture)
	frame.SurfaceTexture = nil
	return err
}

// DiscardFrame drops an acquired frame without presenting (best-effort).
func (sc *Swapchain) DiscardFrame(frame *Frame) {
	if frame == nil {
		return
	}
	if frame.View != nil {
		frame.View.Release()
		frame.View = nil
	}
	if sc != nil && sc.Surface != nil {
		sc.Surface.DiscardTexture()
	}
	frame.SurfaceTexture = nil
}

// Release unconfigures the surface; does not release Surface/Device ownership.
func (sc *Swapchain) Release() {
	if sc == nil || sc.Surface == nil {
		return
	}
	sc.Surface.Unconfigure()
	sc.configured = false
}
