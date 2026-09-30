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

package webgpu

import (
	"image"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// halSwapchainAdapter exposes *Swapchain as hal.Swapchain so render talks
// hal only.
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
		ViewHandle:  uintptr(f.Handle.Pointer()),
		Width:       f.Width,
		Height:      f.Height,
		Suboptimal:  f.Suboptimal,
		DamageRects: f.DamageRects,
		Payload:     f,
	}, nil
}
func (a *halSwapchainAdapter) EndFrame(f *hal.SwapchainFrame) error {
	if f == nil {
		return a.sc.EndFrame(nil)
	}
	wf, _ := f.Payload.(*Frame)
	if wf == nil {
		return a.sc.EndFrame(nil)
	}
	return a.sc.EndFrame(wf)
}
func (a *halSwapchainAdapter) DiscardFrame(f *hal.SwapchainFrame) {
	if f == nil {
		return
	}
	wf, _ := f.Payload.(*Frame)
	if wf == nil {
		return
	}
	a.sc.DiscardFrame(wf)
}
func (a *halSwapchainAdapter) EndFrameWithDamage(f *hal.SwapchainFrame, rects []image.Rectangle) error {
	if f == nil {
		return a.sc.EndFrame(nil)
	}
	wf, _ := f.Payload.(*Frame)
	if wf == nil {
		return a.sc.EndFrame(nil)
	}
	return a.sc.EndFrameWithDamage(wf, rects)
}
func (a *halSwapchainAdapter) MarkNeedsReconfigure() { a.sc.MarkNeedsReconfigure() }
func (a *halSwapchainAdapter) Release()              { a.sc.Release() }
func (a *halSwapchainAdapter) SetPreferVSync()       { a.sc.SetPreferVSync() }
func (a *halSwapchainAdapter) SetPreferFifoRelaxed() { a.sc.SetPreferFifoRelaxed() }
func (a *halSwapchainAdapter) PresentModeForVsync(on bool) types.PresentMode {
	return a.sc.PresentModeForVsync(on)
}
func (a *halSwapchainAdapter) SetPresentModeForce(m types.PresentMode) error {
	return a.sc.SetPresentModeForce(m)
}
func (a *halSwapchainAdapter) PresentModeName() string { return a.sc.PresentModeName() }
func (a *halSwapchainAdapter) GetPresentMode() types.PresentMode {
	return a.sc.PresentMode
}
func (a *halSwapchainAdapter) GetFormat() types.TextureFormat { return a.sc.Format }
func (a *halSwapchainAdapter) SetUsage(u types.TextureUsage)  { a.sc.Usage = u }
func (a *halSwapchainAdapter) GetDevice() hal.Device          { return a.sc.Device }
func (a *halSwapchainAdapter) SetDevice(d hal.Device)         { a.sc.Device = d }
func (a *halSwapchainAdapter) GetOnDeviceAbandon() func(hal.Device) {
	return a.sc.OnDeviceAbandon
}
func (a *halSwapchainAdapter) GetOnDeviceRecreated() func(hal.Device) {
	return a.sc.OnDeviceRecreated
}
func (a *halSwapchainAdapter) FireOnDeviceRecreated(d hal.Device) {
	if a.sc.OnDeviceRecreated != nil {
		a.sc.OnDeviceRecreated(d)
	}
}
