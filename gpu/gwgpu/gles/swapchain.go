//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"
	"sync"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Swapchain presents through hal.Surface Acquire +
// Device.CreateTextureView(isSurface) + Queue.Present (swapchainFBO blit +
// Y-flip + SwapBuffers).
type Swapchain struct {
	mu          sync.Mutex
	surf        hal.Surface
	dev         hal.Device
	Width       uint32
	Height      uint32
	Format      gputypes.TextureFormat
	Usage       gputypes.TextureUsage
	PresentMode gputypes.PresentMode
	AlphaMode   gputypes.CompositeAlphaMode

	PreferPresentModes []gputypes.PresentMode

	OnDeviceAbandon   func(oldDevice hal.Device)
	OnDeviceRecreated func(newDevice hal.Device)

	configured bool
	frameOpen  bool
	curTex     hal.SurfaceTexture
	curView    hal.TextureView

	acquires uint64
	presents uint64
	discards uint64
}

// Frame is one acquired GL surface image.
type Frame struct {
	SurfaceTexture hal.SurfaceTexture
	View           hal.TextureView
	Handle         gpucontext.TextureView
	Width          uint32
	Height         uint32
}

// NewHalSwapchain exposes this swapchain as hal.Swapchain so render only
// names the backend in the one creation sentence.
func (sc *Swapchain) NewHalSwapchain() hal.Swapchain {
	return &halSwapchainAdapter{sc: sc}
}

// NewSwapchain builds a GL swapchain for an existing surface + device.
func NewSwapchain(surface hal.Surface, device hal.Device, width, height uint32) *Swapchain {
	return &Swapchain{
		surf:        surface,
		dev:         device,
		Width:       width,
		Height:      height,
		Format:      gputypes.TextureFormatBGRA8Unorm,
		Usage:       gputypes.TextureUsageRenderAttachment,
		PresentMode: hal.PresentModeFifo,
		AlphaMode:   gputypes.CompositeAlphaModeOpaque,
	}
}

// SetPreferVSync selects Fifo when available (steady UI path).
func (sc *Swapchain) SetPreferVSync() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []gputypes.PresentMode{hal.PresentModeFifo, hal.PresentModeImmediate}
}

// SetPreferFifoRelaxed keeps Fifo steady (no relaxed mode on GL).
func (sc *Swapchain) SetPreferFifoRelaxed() {
	if sc == nil {
		return
	}
	sc.PreferPresentModes = []gputypes.PresentMode{hal.PresentModeFifo, hal.PresentModeImmediate}
}

// Configure applies the surface configuration.
func (sc *Swapchain) Configure() error {
	if sc == nil {
		return fmt.Errorf("gles: swapchain is nil")
	}
	if sc.surf == nil {
		return fmt.Errorf("gles: swapchain surface is nil")
	}
	if sc.dev == nil {
		return fmt.Errorf("gles: swapchain device is nil")
	}
	if sc.Width == 0 || sc.Height == 0 {
		return fmt.Errorf("gles: swapchain extent must be non-zero")
	}
	if sc.Usage == 0 {
		sc.Usage = gputypes.TextureUsageRenderAttachment
	}
	if sc.Format == 0 {
		sc.Format = gputypes.TextureFormatBGRA8Unorm
	}
	if sc.PresentMode == 0 {
		sc.PresentMode = hal.PresentModeFifo
	}
	if sc.AlphaMode == 0 {
		sc.AlphaMode = gputypes.CompositeAlphaModeOpaque
	}
	cfg := &hal.SurfaceConfiguration{
		Width:       sc.Width,
		Height:      sc.Height,
		Format:      sc.Format,
		Usage:       sc.Usage,
		PresentMode: sc.PresentMode,
		AlphaMode:   sc.AlphaMode,
	}
	if err := sc.surf.Configure(sc.dev, cfg); err != nil {
		return err
	}
	sc.configured = true
	return nil
}

type surfaceCapsProvider interface {
	GetSurfaceCapabilities(hal.Surface) *hal.SurfaceCapabilities
}

// ConfigureFromCapabilities picks a supported format then configures.
// GL only advertises Fifo/Immediate, so present-mode preference is a no-op
// beyond keeping Fifo steady.
func (sc *Swapchain) ConfigureFromCapabilities(adapter hal.Adapter) error {
	if sc == nil {
		return fmt.Errorf("gles: swapchain is nil")
	}
	if adapter != nil {
		if p, ok := adapter.(surfaceCapsProvider); ok {
			if caps := p.GetSurfaceCapabilities(sc.surf); caps != nil {
				if len(caps.Formats) > 0 {
					sc.Format = caps.Formats[0]
					for _, f := range caps.Formats {
						if f == gputypes.TextureFormatBGRA8Unorm || f == gputypes.TextureFormatRGBA8Unorm {
							sc.Format = f
							break
						}
					}
				}
			}
		}
	}
	sc.PresentMode = hal.PresentModeFifo
	return sc.Configure()
}

// applyOrRevert runs apply, then Configure; on failure it runs revert and
// returns the error. Caller must hold sc.mu. Resize and SetPresentModeForce
// share it so a failed Configure never leaves a size/mode whose FBO was
// just destroyed (empty shell blitting nothing until the storm settles).
func (sc *Swapchain) applyOrRevert(apply, revert func()) error {
	apply()
	if err := sc.Configure(); err != nil {
		revert()
		return err
	}
	return nil
}

// Resize updates extent and reconfigures on change. The new size is only
// committed when Configure succeeds (via applyOrRevert).
func (sc *Swapchain) Resize(width, height uint32) error {
	if sc == nil {
		return fmt.Errorf("gles: swapchain is nil")
	}
	if width == 0 || height == 0 {
		return fmt.Errorf("gles: swapchain extent must be non-zero")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.Width == width && sc.Height == height && sc.configured {
		return nil
	}
	oldW, oldH := sc.Width, sc.Height
	return sc.applyOrRevert(
		func() { sc.Width, sc.Height = width, height },
		func() { sc.Width, sc.Height = oldW, oldH },
	)
}

// PresentModeForVsync picks Fifo steady vs Immediate low-latency.
func (sc *Swapchain) PresentModeForVsync(on bool) gputypes.PresentMode {
	if on {
		return hal.PresentModeFifo
	}
	return hal.PresentModeImmediate
}

// SetPresentModeForce reconfigures with an explicit mode. The mode is only
// committed when Configure succeeds (via applyOrRevert, same as Resize).
func (sc *Swapchain) SetPresentModeForce(mode gputypes.PresentMode) error {
	if sc == nil {
		return fmt.Errorf("gles: swapchain is nil")
	}
	if mode == 0 {
		mode = hal.PresentModeFifo
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.PresentMode == mode && sc.configured {
		return nil
	}
	old := sc.PresentMode
	return sc.applyOrRevert(
		func() { sc.PresentMode = mode },
		func() { sc.PresentMode = old },
	)
}

// BeginFrame acquires the next surface texture and boxes its view.
func (sc *Swapchain) BeginFrame() (*Frame, error) {
	if sc == nil {
		return nil, fmt.Errorf("gles: swapchain is nil")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if sc.surf == nil {
		return nil, fmt.Errorf("gles: swapchain surface is nil")
	}
	if sc.frameOpen {
		return nil, fmt.Errorf("gles: frame already in flight")
	}
	if sc.Width == 0 || sc.Height == 0 {
		return nil, fmt.Errorf("gles: swapchain extent must be non-zero")
	}
	if !sc.configured {
		if err := sc.Configure(); err != nil {
			return nil, err
		}
	}
	// Borrowed-frame convergence: a storm step may have borrowed the old FBO
	// (fbo size != requested size) to survive transient driver pressure.
	// Retry the rebuild on every later present until it converges; failures
	// keep borrowing (Configure returns nil on borrow), so this never fails
	// the frame. All inside the GL backend; render never sees a failure and
	// never re-arms swapchainPending, so one transient stall cannot turn
	// into a per-present reconfig loop.
	if sc.configured {
		if s, ok := sc.surf.(*Surface); ok && s.borrowedSize(sc.Width, sc.Height) {
			_ = sc.surf.Configure(sc.dev, &hal.SurfaceConfiguration{
				Width:       sc.Width,
				Height:      sc.Height,
				Format:      sc.Format,
				Usage:       sc.Usage,
				PresentMode: sc.PresentMode,
				AlphaMode:   sc.AlphaMode,
			})
		}
	}
	acq, err := sc.surf.AcquireTexture(nil)
	if err != nil {
		return nil, err
	}
	if acq == nil || acq.Texture == nil {
		return nil, fmt.Errorf("gles: AcquireTexture returned nil texture")
	}
	view, err := sc.dev.CreateTextureView(acq.Texture, nil)
	if err != nil {
		sc.surf.DiscardTexture(acq.Texture)
		return nil, fmt.Errorf("gles: CreateTextureView(surface): %w", err)
	}
	sc.curTex = acq.Texture
	sc.curView = view
	sc.frameOpen = true
	sc.acquires++
	return &Frame{
		SurfaceTexture: acq.Texture,
		View:           view,
		Handle:         gpucontext.PackTextureView(view),
		Width:          sc.Width,
		Height:         sc.Height,
	}, nil
}

// EndFrame presents the frame via Queue.Present (blit + SwapBuffers).
func (sc *Swapchain) EndFrame(frame *Frame) error {
	if sc == nil {
		return fmt.Errorf("gles: swapchain is nil")
	}
	if frame == nil {
		return fmt.Errorf("gles: frame is nil")
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !sc.frameOpen {
		return fmt.Errorf("gles: no frame in flight")
	}
	q := sc.dev.Queue()
	if q == nil {
		sc.surf.DiscardTexture(frame.SurfaceTexture)
		sc.curTex = nil
		sc.curView = nil
		sc.frameOpen = false
		return fmt.Errorf("gles: device queue is nil")
	}
	err := q.Present(sc.surf, frame.SurfaceTexture, nil)
	sc.curTex = nil
	sc.curView = nil
	sc.frameOpen = false
	if err == nil {
		sc.presents++
		if sc.dev != nil && sc.dev.IsLost() {
			return hal.ErrDeviceLost
		}
	}
	return err
}

// DiscardFrame drops an acquired frame without presenting.
func (sc *Swapchain) DiscardFrame(frame *Frame) {
	if sc == nil || frame == nil {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if frame.SurfaceTexture != nil {
		sc.surf.DiscardTexture(frame.SurfaceTexture)
	}
	sc.curTex = nil
	sc.curView = nil
	if sc.frameOpen {
		sc.discards++
		sc.frameOpen = false
	}
}

// Release unconfigures the surface; Surface/Device ownership stays out.
func (sc *Swapchain) Release() {
	if sc == nil || sc.surf == nil {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.surf.Unconfigure(sc.dev)
	sc.configured = false
	sc.frameOpen = false
	sc.curTex = nil
	sc.curView = nil
}

// PresentModeName labels the active mode for logs.
func (sc *Swapchain) PresentModeName() string {
	if sc == nil {
		return "nil"
	}
	switch sc.PresentMode {
	case hal.PresentModeFifo:
		return "fifo"
	case hal.PresentModeImmediate:
		return "immediate"
	default:
		return "gl-present"
	}
}

// GetDevice reports the bound device for the render interface.
func (sc *Swapchain) GetDevice() hal.Device {
	if sc == nil {
		return nil
	}
	return sc.dev
}

// SetDevice rebinds to a new device (shared recovery path).
func (sc *Swapchain) SetDevice(dev hal.Device) {
	if sc == nil {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.dev = dev
}

// GetFormat reports the configured format (render interface accessor).
func (sc *Swapchain) GetFormat() gputypes.TextureFormat {
	if sc == nil {
		return gputypes.TextureFormatUndefined
	}
	return sc.Format
}

// SetUsage updates the surface usage before Configure.
func (sc *Swapchain) SetUsage(u gputypes.TextureUsage) {
	if sc == nil {
		return
	}
	sc.Usage = u
}

// GetPresentMode reports the active present mode.
func (sc *Swapchain) GetPresentMode() gputypes.PresentMode {
	if sc == nil {
		return hal.PresentModeFifo
	}
	return sc.PresentMode
}

// FireOnDeviceRecreated invokes the recreation hook when armed.
func (sc *Swapchain) FireOnDeviceRecreated(dev hal.Device) {
	if sc == nil || sc.OnDeviceRecreated == nil {
		return
	}
	sc.OnDeviceRecreated(dev)
}
