//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render/hdr"
)

// M5 P3：GPU tonemap 出口 vs CPU hdr.FloatTarget.Encode 逐像素对拍。
// 判据：每个通道 |gpu-cpu| ≤ 1（8bit 量化同式，差异只允许来自 pow/round 的末位），
// 禁 mean。所有输入值都是 half 精确值，排除了输入量化差。
func TestP3TonemapMatchesHDRCpuEncode(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	const w, h = 32, 2
	vals := []float32{0, 0.0625, 0.125, 0.25, 0.5, 0.75, 1, 2, 4, 8, 16, 0.25, 0.5, 1, 2, 4}
	halfBits := map[float32]uint16{
		0: 0x0000, 0.0625: 0x2C00, 0.125: 0x3000, 0.25: 0x3400, 0.5: 0x3800,
		0.75: 0x3A00, 1: 0x3C00, 2: 0x4000, 4: 0x4400, 8: 0x4800, 16: 0x4C00,
	}
	alpha := []float32{1, 0.5}

	data := make([]byte, w*h*8)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := vals[x%len(vals)]
			bits := halfBits[v]
			a := halfBits[alpha[y]]
			off := (y*w + x) * 8
			for c := 0; c < 3; c++ {
				data[off+c*2] = byte(bits)
				data[off+c*2+1] = byte(bits >> 8)
			}
			data[off+6] = byte(a)
			data[off+7] = byte(a >> 8)
		}
	}

	src, err := device.CreateTexture(&hal.TextureDescriptor{
		Label: "p3_src", Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA16Float,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("src texture: %v", err)
	}
	defer src.Destroy()
	if err := queue.WriteTexture(&hal.ImageCopyTexture{Texture: src, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
		data, &hal.ImageDataLayout{Offset: 0, BytesPerRow: w * 8, RowsPerImage: h},
		&hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}); err != nil {
		t.Fatalf("upload 16F: %v", err)
	}
	srcView, err := device.CreateTextureView(src, &hal.TextureViewDescriptor{
		Label: "p3_src_view", Format: types.TextureFormatRGBA16Float,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatalf("src view: %v", err)
	}
	defer srcView.Destroy()

	dst, err := device.CreateTexture(&hal.TextureDescriptor{
		Label: "p3_dst", Size: hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatBGRA8Unorm,
		Usage:  types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		t.Fatalf("dst texture: %v", err)
	}
	defer dst.Destroy()
	dstView, err := device.CreateTextureView(dst, &hal.TextureViewDescriptor{
		Label: "p3_dst_view", Format: types.TextureFormatBGRA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		t.Fatalf("dst view: %v", err)
	}
	defer dstView.Destroy()

	pipe, err := newTonemapPipeline(device, queue, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("tonemap pipeline: %v", err)
	}
	defer pipe.Destroy()

	for _, exposure := range []float64{1, 4} {
		if err := pipe.Run(srcView, dstView, exposure); err != nil {
			t.Fatalf("run exposure %v: %v", exposure, err)
		}
		got, err := downloadTestPixmap(device, queue, dst, testFmtBGRA8, w, h)
		if err != nil {
			t.Fatalf("readback: %v", err)
		}

		ref, err := hdr.NewFloatTarget(w, h)
		if err != nil {
			t.Fatalf("hdr target: %v", err)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := vals[x%len(vals)]
				if !ref.Set(x, y, [4]float32{v, v, v, alpha[y]}) {
					t.Fatalf("hdr set (%d,%d)", x, y)
				}
			}
		}
		want, err := ref.Encode(exposure)
		if err != nil {
			t.Fatalf("hdr encode: %v", err)
		}

		maxDiff, over := 0, 0
		gotData := got.Data()
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				gi := (y*w + x) * 4
				ci := y*want.Stride + x*4
				for c := 0; c < 4; c++ {
					d := int(gotData[gi+c]) - int(want.Pix[ci+c])
					if d < 0 {
						d = -d
					}
					if d > maxDiff {
						maxDiff = d
					}
					if d > 1 {
						over++
					}
				}
			}
		}
		t.Logf("exposure=%v max_channel_diff=%d over1=%d", exposure, maxDiff, over)
		if over != 0 || maxDiff > 1 {
			t.Fatalf("exposure %v: max channel diff %d (over1=%d), want <=1", exposure, maxDiff, over)
		}

		// Direction sanity: a 16x HDR value must read brighter than 1.0 after
		// the rolloff (both are mapped through the same curve).
		bright := gotData[(0*10+10)*4]
		unit := gotData[(0*10+6)*4]
		if bright <= unit {
			t.Fatalf("16.0 tonemapped channel %d <= 1.0 channel %d", bright, unit)
		}
	}
}

func TestP3TonemapRejectsBadExposure(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	pipe, err := newTonemapPipeline(device, queue, 0)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Destroy()
	// Bad exposure is rejected before touching views.
	for _, e := range []float64{0, -1, 17, math.NaN(), math.Inf(1)} {
		if err := pipe.Run(hal.TextureView(nil), hal.TextureView(nil), e); err == nil {
			t.Fatalf("exposure %v accepted", e)
		}
	}
}
