//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
)

// M5 P4（evidence/M5/p4-connection-plan.md，用户拍板方案 B「格式跟随目标视图」）：
// 公开的 float 离屏面与 tonemap 出口。标准形态对齐 Skia：
// CreateOffscreenTextureFormat 相当于 MakeRenderTarget(info)（目标自带色彩类型），
// TonemapView 是显式的输出转换（SDR 目标），不是全局 HDR 开关——避免「开关」与
// 真实目标格式成为两个真源。

// CreateOffscreenTextureFormat allocates an offscreen color target in format.
// Supported: TextureFormatBGRA8Unorm / TextureFormatRGBA8Unorm /
// TextureFormatRGBA16Float. An unsupported format (depth, compressed, unknown)
// fails closed with an error before any GPU allocation.
//
// Render into it with FlushGPUWithView (the session adapts its pipelines to the
// target format; a float target runs 1x). Composite with DrawGPUTexture.
func (c *Context) CreateOffscreenTextureFormat(width, height int, format types.TextureFormat) (gpucontext.TextureView, func(), error) {
	rc := c.gpuCtxOps()
	if rc == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("render: GPU context unavailable")
	}
	type creator interface {
		CreateOffscreenTextureFormat(w, h int, format types.TextureFormat) (gpucontext.TextureView, func(), error)
	}
	oc, ok := rc.(creator)
	if !ok {
		return gpucontext.TextureView{}, nil, fmt.Errorf("render: offscreen format not supported by this backend")
	}
	return oc.CreateOffscreenTextureFormat(width, height, format)
}

// OffscreenFormatFallbacks reports how many offscreen format requests on this
// context fell back to the 8-bit target because the requested format was not
// renderable (M5 P2 diagnostics; 0 means no silent degradation).
func (c *Context) OffscreenFormatFallbacks() uint64 {
	rc := c.gpuCtxOps()
	if rc == nil {
		return 0
	}
	type stats interface{ OffscreenFormatFallbacks() uint64 }
	if s, ok := rc.(stats); ok {
		return s.OffscreenFormatFallbacks()
	}
	return 0
}

// TonemapView converts src (must be a TextureFormatRGBA16Float view, linear)
// into a new pooled 8-bit offscreen view using exposure → Reinhard → sRGB, the
// same curve as render/hdr.FloatTarget.Encode. exposure must be in (0,16].
//
// The destination is returned with its release; composite it with
// DrawGPUTexture or hand it to the presenter. Callers own both views.
func (c *Context) TonemapView(src gpucontext.TextureView, width, height int, exposure float64) (gpucontext.TextureView, func(), error) {
	rc := c.gpuCtxOps()
	if rc == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("render: GPU context unavailable")
	}
	type toner interface {
		TonemapView(src gpucontext.TextureView, w, h int, exposure float64) (gpucontext.TextureView, func(), error)
	}
	oc, ok := rc.(toner)
	if !ok {
		return gpucontext.TextureView{}, nil, fmt.Errorf("render: tonemap not supported by this backend")
	}
	return oc.TonemapView(src, width, height, exposure)
}
