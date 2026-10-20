//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"fmt"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
)

// writeRGBAToView uploads premultiplied RGBA pixels into a BGRA8 texture view.
//
// Pixmap data is RGBA; offscreen color targets are BGRA8Unorm, so swap R↔B.
// WebGPU's copy pitch must be 256-byte aligned, so unaligned rows are re-packed.
// The write is a queue operation: it is ordered before the next Queue.Submit,
// which the caller must perform before presenting or sampling the view.
func (rc *GPURenderContext) writeRGBAToView(view gpucontext.TextureView, data []byte, w, h int) error {
	if rc == nil || rc.shared == nil || view.IsNil() || w <= 0 || h <= 0 {
		return fmt.Errorf("gpu: write RGBA to view: bad args (w=%d h=%d)", w, h)
	}
	need := w * h * 4
	if len(data) < need {
		return fmt.Errorf("gpu: write RGBA to view: short data %d < %d", len(data), need)
	}
	queue := rc.shared.Queue()
	if queue == nil {
		return fmt.Errorf("gpu: write RGBA to view: no queue")
	}
	halView := unpackView(view)
	if halView == nil {
		return fmt.Errorf("gpu: write RGBA to view: nil native view")
	}
	tex := halView.Texture()
	if tex == nil {
		return fmt.Errorf("gpu: write RGBA to view: nil texture")
	}

	bgra := make([]byte, need)
	for i := 0; i < need; i += 4 {
		bgra[i+0] = data[i+2]
		bgra[i+1] = data[i+1]
		bgra[i+2] = data[i+0]
		bgra[i+3] = data[i+3]
	}

	const copyPitchAlignment = 256
	bpr := w * 4
	aligned := (bpr + copyPitchAlignment - 1) &^ (copyPitchAlignment - 1)
	if aligned != bpr {
		packed := make([]byte, aligned*h)
		for row := 0; row < h; row++ {
			copy(packed[row*aligned:row*aligned+bpr], bgra[row*bpr:(row+1)*bpr])
		}
		bgra = packed
	}
	return queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: tex, MipLevel: 0},
		bgra,
		&hal.ImageDataLayout{BytesPerRow: uint32(aligned), RowsPerImage: uint32(h)}, //nolint:gosec // view-bounded
		&hal.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1},   //nolint:gosec // view-bounded
	)
}

// seedViewFromRGBA queues an upload of premultiplied RGBA surface bytes into an
// offscreen view and marks the view as color content, so the render pass this
// flush encodes loads the seed instead of clearing it away.
//
// Skia semantics: an image filter / effect layer sees the whole surface,
// including pixels rasterized on the CPU. Without this seed the filter RT
// starts transparent and drops a CPU-painted background.
//
// No submit happens here: writeRGBAToView is a queue operation and the caller
// (GPURenderContext.Flush) submits the command buffer that consumes it.
func (rc *GPURenderContext) seedViewFromRGBA(view gpucontext.TextureView, data []byte, w, h int) error {
	if err := rc.writeRGBAToView(view, data, w, h); err != nil {
		return err
	}
	halView := extractTextureView(view)
	if halView == nil {
		return fmt.Errorf("gpu: seed view: nil native view")
	}
	if rc.session != nil {
		rc.session.MarkColorContent(halView)
	}
	rc.frameRendered = true
	rc.lastView = halView
	return nil
}
