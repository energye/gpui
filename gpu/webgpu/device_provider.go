//go:build !(js && wasm)

package webgpu

import (
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// SimpleDeviceProvider adapts hal.Device/hal.Adapter into gpucontext.DeviceProvider
// so apps can inject a window/swapchain device into render (GPUShared).
// Without this, render creates a second device and MSAA resolve into a
// foreign surface texture fails validation.
// Dev/Adpt store hal interfaces (7f Device→hal.Device); concrete access is a
// direct internal assert to *Device/*Adapter (same package unpack pattern).
type SimpleDeviceProvider struct {
	Dev    hal.Device
	Adpt   hal.Adapter
	Format TextureFormat
}

// Device implements gpucontext.DeviceProvider.
func (p *SimpleDeviceProvider) Device() gpucontext.Device {
	if p == nil || p.Dev == nil {
		return gpucontext.Device{}
	}
	wd, ok := p.Dev.(*Device)
	if !ok || wd == nil {
		return gpucontext.Device{}
	}
	return DeviceToHandle(wd)
}

// Queue implements gpucontext.DeviceProvider.
func (p *SimpleDeviceProvider) Queue() gpucontext.Queue {
	if p == nil || p.Dev == nil {
		return gpucontext.Queue{}
	}
	hq := p.Dev.Queue()
	wq, ok := hq.(*Queue)
	if !ok || wq == nil {
		return gpucontext.Queue{}
	}
	return QueueToHandle(wq)
}

// SurfaceFormat implements gpucontext.DeviceProvider.
func (p *SimpleDeviceProvider) SurfaceFormat() types.TextureFormat {
	if p == nil {
		return types.TextureFormatUndefined
	}
	return p.Format
}

// Adapter implements gpucontext.DeviceProvider.
func (p *SimpleDeviceProvider) Adapter() gpucontext.Adapter {
	if p == nil || p.Adpt == nil {
		return gpucontext.Adapter{}
	}
	wa, ok := p.Adpt.(*Adapter)
	if !ok || wa == nil {
		return gpucontext.Adapter{}
	}
	return AdapterToHandle(wa)
}

// AdapterInfo implements gpucontext.DeviceProvider.
func (p *SimpleDeviceProvider) AdapterInfo() gpucontext.AdapterInfo {
	if p == nil || p.Adpt == nil {
		return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
	}
	info := p.Adpt.Info()
	ai := gpucontext.AdapterInfo{Name: info.Name}
	switch info.DeviceType {
	case types.DeviceTypeDiscreteGPU:
		ai.Type = gpucontext.AdapterTypeDiscrete
	case types.DeviceTypeIntegratedGPU:
		ai.Type = gpucontext.AdapterTypeIntegrated
	case types.DeviceTypeCPU:
		ai.Type = gpucontext.AdapterTypeSoftware
	default:
		ai.Type = gpucontext.AdapterTypeUnknown
	}
	return ai
}

// Ensure interface compliance.
var _ gpucontext.DeviceProvider = (*SimpleDeviceProvider)(nil)
