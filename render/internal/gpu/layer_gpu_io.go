//go:build !nogpu

package gpu

import (
	"fmt"
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// ReadbackViewRGBA copies a full BGRA8 offscreen texture view into tight RGBA8.
// Used to materialize layer GPU RTs before mask modulation / CPU composite (R1 residual).
func (rc *GPURenderContext) ReadbackViewRGBA(view gpucontext.TextureView, w, h int) ([]byte, error) {
	if rc == nil || view.IsNil() || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("ReadbackViewRGBA: bad args")
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return nil, fmt.Errorf("ReadbackViewRGBA: GPU not ready")
		}
	}
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return nil, fmt.Errorf("ReadbackViewRGBA: nil device/queue")
	}
	return readTextureViewRegionRGBA(device, queue, view, image.Rect(0, 0, w, h), w, h)
}

// UploadRGBAToView writes tight RGBA8 into a BGRA8 offscreen texture view (seed layer RT).
// Used by PushBackdropLayer / post-filter to keep GPU RT coherent with pixmap.
// ReadbackViewStraightRGBA copies an RGBA8Unorm texture view into tight RGBA8
// without BGRA channel swizzle. Used for F.03 filter publish textures.
func (rc *GPURenderContext) ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error) {
	if rc == nil || view.IsNil() || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("ReadbackViewStraightRGBA: bad args")
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return nil, fmt.Errorf("ReadbackViewStraightRGBA: GPU not ready")
		}
	}
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return nil, fmt.Errorf("ReadbackViewStraightRGBA: nil device/queue")
	}
	return readTextureViewRegionStraightRGBA(device, queue, view, image.Rect(0, 0, w, h), w, h)
}

func (rc *GPURenderContext) UploadRGBAToView(view gpucontext.TextureView, data []byte, w, h int) error {
	if rc == nil || view.IsNil() || w <= 0 || h <= 0 {
		return fmt.Errorf("UploadRGBAToView: bad args")
	}
	need := w * h * 4
	if len(data) < need {
		return fmt.Errorf("UploadRGBAToView: short data")
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return fmt.Errorf("UploadRGBAToView: GPU not ready")
		}
	}
	rc.shared.mu.Lock()
	queue := rc.shared.queue
	rc.shared.mu.Unlock()
	if queue == nil {
		return fmt.Errorf("UploadRGBAToView: nil queue")
	}
	wgpuView := (*webgpu.TextureView)(view.Pointer())
	if wgpuView == nil {
		return fmt.Errorf("UploadRGBAToView: nil view ptr")
	}
	tex := wgpuView.Texture()
	if tex == nil {
		return fmt.Errorf("UploadRGBAToView: nil texture")
	}
	// Pixmap is RGBA, offscreen texture is BGRA8Unorm — swizzle R↔B.
	// R7.1: stage via imageStagingPool (WriteTexture copies before return).
	bgraScratch := acquireImageStaging(need)
	bgra := *bgraScratch
	for i := 0; i < need; i += 4 {
		bgra[i+0] = data[i+2]
		bgra[i+1] = data[i+1]
		bgra[i+2] = data[i+0]
		bgra[i+3] = data[i+3]
	}
	uw, uh := uint32(w), uint32(h) //nolint:gosec // bounded by surface
	tight := uw * 4
	aligned := alignTextureBytesPerRow(tight)
	upload := bgra
	var padScratch *[]byte
	if aligned != tight && h > 1 {
		padNeed := int(aligned) * h
		padScratch = acquireImageStaging(padNeed)
		padded := *padScratch
		for y := 0; y < h; y++ {
			copy(padded[y*int(aligned):y*int(aligned)+w*4], bgra[y*w*4:(y+1)*w*4])
		}
		upload = padded
	}
	err := queue.WriteTexture(
		&webgpu.ImageCopyTexture{Texture: tex, MipLevel: 0},
		upload,
		&webgpu.ImageDataLayout{BytesPerRow: aligned, RowsPerImage: uh},
		&webgpu.Extent3D{Width: uw, Height: uh, DepthOrArrayLayers: 1},
	)
	releaseImageStaging(padScratch)
	releaseImageStaging(bgraScratch)
	return err
}

// CompositeMaskedLayer materializes a GPU layer RT, modulates by R8 mask on GPU,
// and SourceOver-composites onto parent RGBA (PushMaskLayer Pop path, R1).
func (rc *GPURenderContext) CompositeMaskedLayer(
	parentData []byte, parentW, parentH int,
	srcView gpucontext.TextureView, srcW, srcH int,
	mask *render.Mask,
	opacity float64,
) error {
	if rc == nil || parentData == nil || srcView.IsNil() || mask == nil ||
		parentW <= 0 || parentH <= 0 || srcW <= 0 || srcH <= 0 {
		return render.ErrFallbackToCPU
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	bw, bh := srcW, srcH
	if mask.Width() < bw {
		bw = mask.Width()
	}
	if mask.Height() < bh {
		bh = mask.Height()
	}
	if bw <= 0 || bh <= 0 {
		return nil
	}
	srcRGBA, err := rc.ReadbackViewRGBA(srcView, srcW, srcH)
	if err != nil {
		return err
	}
	// Tight R8 from mask (region 0,0,bw,bh).
	maskR8 := make([]byte, bw*bh)
	for y := 0; y < bh; y++ {
		for x := 0; x < bw; x++ {
			maskR8[y*bw+x] = mask.At(x, y)
		}
	}
	// Crop src to mask region if needed.
	srcRegion := srcRGBA
	if bw != srcW || bh != srcH {
		srcRegion = make([]byte, bw*bh*4)
		for y := 0; y < bh; y++ {
			copy(srcRegion[y*bw*4:(y+1)*bw*4], srcRGBA[y*srcW*4:y*srcW*4+bw*4])
		}
	}
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	if opacity < 1.0-1e-6 {
		op := opacity
		for i := 0; i < len(srcRegion); i += 4 {
			srcRegion[i+0] = byte(float64(srcRegion[i+0]) * op)
			srcRegion[i+1] = byte(float64(srcRegion[i+1]) * op)
			srcRegion[i+2] = byte(float64(srcRegion[i+2]) * op)
			srcRegion[i+3] = byte(float64(srcRegion[i+3]) * op)
		}
	}

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.maskR8
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return render.ErrFallbackToCPU
	}
	modulated, err := maskR8Modulate(device, queue, cache, srcRegion, maskR8, bw, bh)
	if err != nil {
		return err
	}
	// Source-over onto parent pixmap (premul).
	stride := parentW * 4
	maxW, maxH := bw, bh
	if parentW < maxW {
		maxW = parentW
	}
	if parentH < maxH {
		maxH = parentH
	}
	for y := 0; y < maxH; y++ {
		for x := 0; x < maxW; x++ {
			si := (y*bw + x) * 4
			di := y*stride + x*4
			sr, sg, sb, sa := modulated[si+0], modulated[si+1], modulated[si+2], modulated[si+3]
			if sa == 0 {
				continue
			}
			if sa == 255 {
				parentData[di+0] = sr
				parentData[di+1] = sg
				parentData[di+2] = sb
				parentData[di+3] = 255
				continue
			}
			// premul SO: out = src + dst * (1 - sa/255)
			inv := uint16(255 - sa)
			parentData[di+0] = uint8(uint16(sr) + uint16(parentData[di+0])*inv/255)
			parentData[di+1] = uint8(uint16(sg) + uint16(parentData[di+1])*inv/255)
			parentData[di+2] = uint8(uint16(sb) + uint16(parentData[di+2])*inv/255)
			parentData[di+3] = uint8(uint16(sa) + uint16(parentData[di+3])*inv/255)
		}
	}
	return nil
}
