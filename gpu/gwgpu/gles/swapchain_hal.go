// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"image"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// halSwapchainAdapter exposes *Swapchain as hal.Swapchain so render talks
// hal only. Only wraps the H4-e proven Acquire/Present path.
type halSwapchainAdapter struct {
	sc *Swapchain
}

func (a *halSwapchainAdapter) Configure() error { return a.sc.Configure() }
func (a *halSwapchainAdapter) ConfigureFromCapabilities(ad hal.Adapter) error {
	return a.sc.ConfigureFromCapabilities(ad)
}
func (a *halSwapchainAdapter) Resize(w, h uint32) error { return a.sc.Resize(w, h) }
func (a *halSwapchainAdapter) BeginFrame() (*hal.SwapchainFrame, error) {
	f, err := a.sc.BeginFrame()
	if err != nil {
		return nil, err
	}
	return &hal.SwapchainFrame{
		ViewHandle: uintptr(f.Handle.Pointer()),
		Width:      f.Width,
		Height:     f.Height,
		Payload:    f,
	}, nil
}
func (a *halSwapchainAdapter) EndFrame(f *hal.SwapchainFrame) error {
	if f == nil {
		return a.sc.EndFrame(nil)
	}
	gf, _ := f.Payload.(*Frame)
	if gf == nil {
		return a.sc.EndFrame(nil)
	}
	return a.sc.EndFrame(gf)
}
func (a *halSwapchainAdapter) DiscardFrame(f *hal.SwapchainFrame) {
	if f == nil {
		return
	}
	gf, _ := f.Payload.(*Frame)
	if gf == nil {
		return
	}
	a.sc.DiscardFrame(gf)
}
func (a *halSwapchainAdapter) EndFrameWithDamage(f *hal.SwapchainFrame, _ []image.Rectangle) error {
	// GL has no partial present: damage is accepted and presented full
	// (same content contract as the webgpu adapter's full fallback).
	return a.EndFrame(f)
}
func (a *halSwapchainAdapter) MarkNeedsReconfigure() {}
func (a *halSwapchainAdapter) Release()              { a.sc.Release() }
func (a *halSwapchainAdapter) SetPreferVSync()       { a.sc.SetPreferVSync() }
func (a *halSwapchainAdapter) SetPreferFifoRelaxed() { a.sc.SetPreferFifoRelaxed() }
func (a *halSwapchainAdapter) PresentModeForVsync(on bool) gputypes.PresentMode {
	return a.sc.PresentModeForVsync(on)
}
func (a *halSwapchainAdapter) SetPresentModeForce(m gputypes.PresentMode) error {
	return a.sc.SetPresentModeForce(m)
}
func (a *halSwapchainAdapter) PresentModeName() string { return a.sc.PresentModeName() }
func (a *halSwapchainAdapter) GetPresentMode() gputypes.PresentMode {
	return a.sc.GetPresentMode()
}
func (a *halSwapchainAdapter) GetFormat() gputypes.TextureFormat { return a.sc.GetFormat() }
func (a *halSwapchainAdapter) SetUsage(u gputypes.TextureUsage)  { a.sc.SetUsage(u) }
func (a *halSwapchainAdapter) GetDevice() hal.Device             { return a.sc.GetDevice() }
func (a *halSwapchainAdapter) SetDevice(d hal.Device)            { a.sc.SetDevice(d) }
func (a *halSwapchainAdapter) GetOnDeviceAbandon() func(hal.Device) {
	return a.sc.OnDeviceAbandon
}
func (a *halSwapchainAdapter) GetOnDeviceRecreated() func(hal.Device) {
	return a.sc.OnDeviceRecreated
}
func (a *halSwapchainAdapter) FireOnDeviceRecreated(d hal.Device) {
	a.sc.FireOnDeviceRecreated(d)
}
