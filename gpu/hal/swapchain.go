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
	"unsafe"

	gputypes "github.com/energye/gpui/gpu/types"
)

// SwapchainFrame is one acquired swapchain image ready for rendering.
// ViewHandle carries the opaque gpucontext.TextureView word (single
// unsafe.Pointer): backends store h.Pointer(), render rebuilds the typed
// handle with gpucontext.NewTextureView. Words (not interface values)
// cross hal so hal stays free of the context import cycle.
//
// The word must stay an unsafe.Pointer, never a uintptr round trip: checkptr
// (enabled by -race) rejects uintptr→unsafe.Pointer conversions, which used to
// abort every GPU window under -race at render/present_frame.go.
type SwapchainFrame struct {
	ViewHandle unsafe.Pointer
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

// PresentStats is the cumulative OS-present accounting of one swapchain.
// PartialCount counts presents that used OS-level damage/partial present
// (EGL SwapBuffersWithDamage on the Go GL backend); FullCount counts full
// swaps. Backends without an OS damage-present protocol (wgpu-native
// deliberately ignores damage; Windows WGL has no damage present) report
// PartialCount=0 — the zero is honest, not missing data.
type PresentStats struct {
	PartialCount uint64
	FullCount    uint64
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
	// PresentStats reports the cumulative OS-present counters (partial vs
	// full swaps) of this swapchain. Counters reset when the swapchain is
	// reconfigured/recreated — render accumulates deltas. The same thread
	// that presented reads the counters, so no extra synchronization is
	// required beyond the backend's own present serialization.
	PresentStats() PresentStats
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
