//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"errors"
	"fmt"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

// M5 HDR 全链 P2（evidence/M5/assessment-design.md §3 G3）：
// float 离屏 RT 的格式门与能力探测。
//
// render 层的离屏目标历史上只有 BGRA8；float 面（RGBA16Float）要能建、能池化、
// 能记账，并且「设备/后端不支持」必须是一条具名错 + 可观测回退，不能静默黑屏。
//
// 能力探测为什么不用 Adapter.TextureFormatCapabilities：webgpu（native）实现是
// 空壳（gpu/webgpu/adapter.go:114 返回零值），GLES 才有真实能力位。这里统一用
// 「1×1 目标真的跑一遍 clear pass」作跨后端判据，并按设备代次缓存。
var ErrOffscreenFormatUnsupported = errors.New("gpu: offscreen render-target format unsupported")

// offscreenEngineFormats is the closed set the engine can create, view, bind and
// blend today. Anything else fails closed with ErrOffscreenFormatUnsupported
// instead of relying on backend-specific validation text.
func offscreenEngineFormats(f types.TextureFormat) bool {
	switch f {
	case types.TextureFormatBGRA8Unorm, types.TextureFormatRGBA8Unorm, types.TextureFormatRGBA16Float:
		return true
	}
	return false
}

// normalizeOffscreenFormat maps the zero value to the historical BGRA8 target.
func normalizeOffscreenFormat(f types.TextureFormat) types.TextureFormat {
	if f == types.TextureFormatUndefined {
		return defaultColorFormat
	}
	return f
}

// probeOffscreenFormatRenderable creates a 1×1 attachment and runs a clear pass
// on it. Creation alone is not enough on GLES (storage allocates before FBO
// completeness is tested), so the pass is the real answer. The probe texture is
// destroyed immediately; nothing is pooled or charged to the VRAM ledger beyond
// its short lifetime.
func probeOffscreenFormatRenderable(device hal.Device, queue hal.Queue, f types.TextureFormat) bool {
	if device == nil {
		return false
	}
	tex, err := device.CreateTexture(&hal.TextureDescriptor{
		Label:         "offscreen_format_probe",
		Size:          hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        f,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		return false
	}
	defer tex.Destroy()
	view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
		Label: "offscreen_format_probe_view", Format: f,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		return false
	}
	defer view.Destroy()
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "offscreen_format_probe_enc"})
	if err != nil {
		return false
	}
	rp, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "offscreen_format_probe_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return false
	}
	if err := rp.End(); err != nil {
		return false
	}
	cmd, err := enc.Finish()
	if err != nil {
		return false
	}
	defer device.FreeCommandBuffer(cmd)
	if queue == nil {
		return true
	}
	if _, err := queue.Submit(cmd); err != nil {
		return false
	}
	return true
}

// offscreenFormatRenderable is the context-level gate: engine format whitelist
// first (fail closed before touching the device), then the cached device probe.
func (rc *GPURenderContext) offscreenFormatRenderable(f types.TextureFormat) bool {
	if !offscreenEngineFormats(f) {
		return false
	}
	if rc.shared == nil {
		return false
	}
	return rc.shared.offscreenFormatRenderable(f)
}

// offscreenFormatRenderable probes once per (device generation, format) and
// caches the verdict. Called without s.mu held; the GPU probe runs outside the
// lock so a slow backend never blocks other windows' shared state.
func (s *GPUShared) offscreenFormatRenderable(f types.TextureFormat) bool {
	s.mu.Lock()
	if s.offscreenFormatProbeGen != s.deviceGen {
		s.offscreenFormatProbe = nil
		s.offscreenFormatProbeGen = s.deviceGen
	}
	if v, ok := s.offscreenFormatProbe[f]; ok {
		s.mu.Unlock()
		return v
	}
	device, queue := s.device, s.queue
	s.mu.Unlock()
	if device == nil {
		// Not initialized yet: do not cache a false negative for this
		// device generation (H20: a probe before ensureGPU poisoned the
		// cache and CreateOffscreenTexture stayed unavailable).
		return false
	}

	ok := probeOffscreenFormatRenderable(device, queue, f)

	s.mu.Lock()
	if s.offscreenFormatProbe == nil {
		s.offscreenFormatProbe = make(map[types.TextureFormat]bool, 3)
	}
	s.offscreenFormatProbe[f] = ok
	s.mu.Unlock()
	return ok
}

// unsupportedOffscreenFormat builds the named error (format is not user text).
func unsupportedOffscreenFormat(f types.TextureFormat) error {
	return fmt.Errorf("%w: %v", ErrOffscreenFormatUnsupported, f)
}
