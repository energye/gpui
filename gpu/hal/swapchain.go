//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package hal

import (
	"image"

	gputypes "github.com/energye/gpui/gpu/types"
)

// SwapchainFrame is one acquired swapchain image ready for rendering.
// ViewHandle carries the opaque gpucontext.TextureView word (single
// unsafe.Pointer): backends store h.Pointer(), render rebuilds the typed
// handle with gpucontext.NewTextureView. Words (not interface values)
// cross hal so hal stays free of the context import cycle.
type SwapchainFrame struct {
	ViewHandle uintptr
	Width      uint32
	Height     uint32
	// Suboptimal reports the surface config is usable but not optimal.
	Suboptimal bool
	// DamageRects optionally records dirty regions (backend may ignore).
	DamageRects []image.Rectangle
	// Payload keeps the backend frame alive until EndFrame/DiscardFrame.
	// render must not inspect it; only the owning backend unwraps it.
	Payload any
}

// Swapchain is the backend-neutral online present surface.
// webgpu.Swapchain and gwgpu/gles.Swapchain both implement it so render
// stays single-path: creation picks a backend once, afterwards render only
// talks this interface. No backend-specific types leak into render.
type Swapchain interface {
	Configure() error
	ConfigureFromCapabilities(adapter Adapter) error
	Resize(w, h uint32) error
	BeginFrame() (*SwapchainFrame, error)
	EndFrame(frame *SwapchainFrame) error
	DiscardFrame(frame *SwapchainFrame)
	// EndFrameWithDamage forwards damage rects when the backend supports
	// partial present; backends without support present full.
	EndFrameWithDamage(frame *SwapchainFrame, rects []image.Rectangle) error
	// EndFrameMerged submits command buffers then presents in one backend
	// call (gles holds one GL binding across both). Backends without a
	// merged path implement it as Submit + EndFrameWithDamage. Returns the
	// submission index for fence alignment / diagnostics (same as Submit).
	EndFrameMerged(frame *SwapchainFrame, rects []image.Rectangle, commandBuffers ...CommandBuffer) (uint64, error)
	// MarkNeedsReconfigure schedules a reconfigure on the next BeginFrame.
	MarkNeedsReconfigure()
	Release()
	SetPreferVSync()
	SetPreferFifoRelaxed()
	PresentModeForVsync(on bool) gputypes.PresentMode
	SetPresentModeForce(mode gputypes.PresentMode) error
	PresentModeName() string
	GetPresentMode() gputypes.PresentMode
	GetFormat() gputypes.TextureFormat
	SetUsage(u gputypes.TextureUsage)
	GetDevice() Device
	SetDevice(dev Device)
	GetOnDeviceAbandon() func(Device)
	GetOnDeviceRecreated() func(Device)
	FireOnDeviceRecreated(dev Device)
}
