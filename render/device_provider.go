// Copyright 2026 The gogpu Authors
// SPDX-License-Identifier: MIT

package render

import (
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// halDeviceProvider adapts any hal.Device/hal.Adapter into
// gpucontext.DeviceProvider via the boxed encoding (PackDevice/PackQueue/
// PackAdapter) so every backend travels without concrete asserts. It
// replaces the webgpu-only provider on the online path: that one asserts
// *webgpu.Device and returns empty for other backends, which callers
// silently ignore, leaving the accelerator unbound.
type halDeviceProvider struct {
	Dev    hal.Device
	Adpt   hal.Adapter
	Format types.TextureFormat
}

func (p *halDeviceProvider) Device() gpucontext.Device {
	if p == nil || p.Dev == nil {
		return gpucontext.Device{}
	}
	return gpucontext.PackDevice(p.Dev)
}

func (p *halDeviceProvider) Queue() gpucontext.Queue {
	if p == nil || p.Dev == nil {
		return gpucontext.Queue{}
	}
	return gpucontext.PackQueue(p.Dev.Queue())
}

func (p *halDeviceProvider) SurfaceFormat() types.TextureFormat {
	if p == nil {
		return types.TextureFormatUndefined
	}
	return p.Format
}

func (p *halDeviceProvider) Adapter() gpucontext.Adapter {
	if p == nil || p.Adpt == nil {
		return gpucontext.Adapter{}
	}
	return gpucontext.PackAdapter(p.Adpt)
}

func (p *halDeviceProvider) AdapterInfo() gpucontext.AdapterInfo {
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

var _ gpucontext.DeviceProvider = (*halDeviceProvider)(nil)
