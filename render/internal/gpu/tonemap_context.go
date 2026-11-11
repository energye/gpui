//go:build !nogpu

package gpu

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
)

// M5 P4：把 P2 的 float 离屏面与 P3 的 tonemap pass 接成一条可调用的出口。
//
// TonemapView 只接受 RGBA16Float 源（HDR 线性面），输出 8bit 离屏视图
// （池化，调用方 release）。曲线与 hdr.FloatTarget.Encode 同式；语义见 tonemap.go。

// TonemapView tonemaps src (RGBA16Float) into a new pooled 8-bit offscreen view.
func (rc *GPURenderContext) TonemapView(src gpucontext.TextureView, w, h int, exposure float64) (gpucontext.TextureView, func(), error) {
	if rc == nil || rc.shared == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("tonemap: GPU context unavailable")
	}
	if src.IsNil() {
		return gpucontext.TextureView{}, nil, fmt.Errorf("tonemap: nil source view")
	}
	sv := unpackView(src)
	if sv == nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("tonemap: foreign source view")
	}
	if tex := sv.Texture(); tex != nil {
		if f := tex.Format(); f != types.TextureFormatRGBA16Float {
			return gpucontext.TextureView{}, nil, fmt.Errorf("tonemap: source format %v, want RGBA16Float", f)
		}
	}
	pipe, err := rc.ensureTonemapPipeline()
	if err != nil {
		return gpucontext.TextureView{}, nil, err
	}
	dst, release, err := rc.CreateOffscreenTextureFormat(w, h, defaultColorFormat)
	if err != nil {
		return gpucontext.TextureView{}, nil, fmt.Errorf("tonemap: destination: %w", err)
	}
	if err := pipe.Run(sv, unpackView(dst), exposure); err != nil {
		release()
		return gpucontext.TextureView{}, nil, err
	}
	return dst, release, nil
}

// ensureTonemapPipeline builds the pass once per context (device formats are
// stable for a context's lifetime; a device swap recreates the context).
func (rc *GPURenderContext) ensureTonemapPipeline() (*tonemapPipeline, error) {
	rc.tonemapMu.Lock()
	defer rc.tonemapMu.Unlock()
	if rc.tonemapPipe != nil {
		return rc.tonemapPipe, nil
	}
	if err := rc.ensureDeviceReady(); err != nil {
		return nil, err
	}
	device := rc.shared.Device()
	queue := rc.shared.Queue()
	p, err := newTonemapPipeline(device, queue, defaultColorFormat)
	if err != nil {
		return nil, err
	}
	rc.tonemapPipe = p
	return p, nil
}

// ensureDeviceReady mirrors the readiness chain of CreateOffscreenTextureFormat.
func (rc *GPURenderContext) ensureDeviceReady() error {
	if rc.shared == nil {
		return fmt.Errorf("tonemap: shared is nil")
	}
	if !rc.shared.deviceReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil {
			return fmt.Errorf("tonemap: ensureGPU: %w", err)
		}
		if !rc.shared.deviceReady {
			return fmt.Errorf("tonemap: device not ready after ensureGPU")
		}
	}
	if rc.shared.Device() == nil {
		return fmt.Errorf("tonemap: device is nil")
	}
	return nil
}

// destroyTonemap releases the pass (called from Close).
func (rc *GPURenderContext) destroyTonemap() {
	rc.tonemapMu.Lock()
	defer rc.tonemapMu.Unlock()
	if rc.tonemapPipe != nil {
		rc.tonemapPipe.Destroy()
		rc.tonemapPipe = nil
	}
}
