//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// P3.1b-2a 能力钉：opt-in sRGB 目标上固定功能 13 模式走硬件线性混合，
// shader 输出经 encode_out 线性化；默认 Unorm 目标仍按 sRGB 域并保持降级计数。
func TestP31bLinearTargetFixedBlend(t *testing.T) {
	shared, rc := newP2Shared(t)

	const size = 64
	const rowBytes = size * 4
	fill := func(format types.TextureFormat) ([4]byte, int64) {
		view, release, err := rc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			t.Fatalf("CreateOffscreenTextureFormat(%v): %v", format, err)
		}
		defer release()
		target := render.GPURenderTarget{
			View: view, ViewWidth: size, ViewHeight: size,
			Width: size, Height: size, Stride: rowBytes,
		}
		path := render.NewPath()
		path.Rectangle(0, 0, size, size)
		base := render.NewPaint()
		base.SetBrush(render.Solid(render.RGBA{R: 0.5, G: 0.25, B: 0.1, A: 1}))
		if err := rc.FillPath(target, path, base, render.Identity()); err != nil {
			t.Fatalf("FillPath base (%v): %v", format, err)
		}
		before := shared.linearFixedFunctionFallbacks.Load()
		plus := render.NewPaint()
		plus.SetBrush(render.Solid(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1}))
		plus.BlendMode = render.BlendPlus
		plus.BlendSpace = render.BlendSpaceLinear
		if err := rc.FillPath(target, path, plus, render.Identity()); err != nil {
			t.Fatalf("FillPath plus (%v): %v", format, err)
		}
		if err := rc.Flush(target); err != nil {
			t.Fatalf("Flush (%v): %v", format, err)
		}
		delta := shared.linearFixedFunctionFallbacks.Load() - before

		tex := unpackView(view).Texture()
		device, queue := shared.Device(), shared.Queue()
		staging, err := device.CreateBuffer(&hal.BufferDescriptor{
			Label: "p31b_linear_target_staging", Size: rowBytes * size,
			Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
		})
		if err != nil {
			t.Fatalf("staging: %v", err)
		}
		defer staging.Destroy()
		enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_linear_target_copy"})
		if err != nil {
			t.Fatalf("encoder: %v", err)
		}
		enc.CopyTextureToBuffer(tex, staging, []hal.BufferTextureCopy{{
			BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: rowBytes, RowsPerImage: size},
			TextureBase:  hal.ImageCopyTexture{Texture: tex, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
			Size:         hal.Extent3D{Width: size, Height: size, DepthOrArrayLayers: 1},
		}})
		cmd, err := enc.Finish()
		if err != nil {
			t.Fatalf("finish copy: %v", err)
		}
		defer device.FreeCommandBuffer(cmd)
		if _, err := queue.Submit(cmd); err != nil {
			t.Fatalf("submit copy: %v", err)
		}
		mapping, err := device.MapBuffer(staging, 0, rowBytes*size)
		if err != nil {
			t.Fatalf("map: %v", err)
		}
		raw := make([]byte, rowBytes*size)
		copy(raw, unsafe.Slice((*byte)(mapping.Ptr), rowBytes*size))
		if err := device.UnmapBuffer(staging); err != nil {
			t.Fatalf("unmap: %v", err)
		}
		center := size/2*rowBytes + size/2*4
		var out [4]byte
		copy(out[:], raw[center:center+4])
		return out, delta
	}

	srgb, srgbFallback := fill(types.TextureFormatRGBA8UnormSrgb)
	unorm, unormFallback := fill(types.TextureFormatRGBA8Unorm)
	t.Logf("P3.1b-2a sRGB target=%v fallback=%d; Unorm target=%v fallback=%d", srgb, srgbFallback, unorm, unormFallback)

	// sRGB target: Plus runs in linear light, shader output linearized.
	// linear(0.5)+linear(0.25)=0.2649 -> sRGB 141; G 0.1018 -> 89; B 0.0609 -> 70.
	wantSrgb := [3]int{141, 89, 70}
	for i := 0; i < 3; i++ {
		if d := int(srgb[i]) - wantSrgb[i]; d < -3 || d > 3 {
			t.Fatalf("sRGB target channel %d = %d, want %d±3 (linear Plus); bytes=%v", i, srgb[i], wantSrgb[i], srgb)
		}
	}
	if srgbFallback != 0 {
		t.Fatalf("sRGB target fixed-function linear fallback = %d, want 0", srgbFallback)
	}
	// Unorm target: Plus stays in encoded domain; linear request is counted as
	// a deliberate degradation.
	wantUnorm := [3]int{192, 128, 90}
	for i := 0; i < 3; i++ {
		if d := int(unorm[i]) - wantUnorm[i]; d < -3 || d > 3 {
			t.Fatalf("Unorm target channel %d = %d, want %d±3 (encoded Plus); bytes=%v", i, unorm[i], wantUnorm[i], unorm)
		}
	}
	if unormFallback != 1 {
		t.Fatalf("Unorm target fixed-function linear fallback = %d, want 1", unormFallback)
	}
}

// TestP31bPublicFlowLinearTelemetry pins the deferred degradation counter.
// Public Context draws queue before FlushGPUWithView binds the target, so a
// fixed-function linear request must be counted against the format Flush
// actually renders to: 0 for an opt-in sRGB target, 1 for a Unorm target.
func TestP31bPublicFlowLinearTelemetry(t *testing.T) {
	shared, rc := newP2Shared(t)
	const size = 64
	data := make([]byte, size*size*4)
	queued := render.GPURenderTarget{Data: data, Width: size, Height: size, Stride: size * 4}
	path := render.NewPath()
	path.Rectangle(0, 0, size, size)
	plus := render.NewPaint()
	plus.SetBrush(render.Solid(render.RGBA{R: 0.25, G: 0.25, B: 0.25, A: 1}))
	plus.BlendMode = render.BlendPlus
	plus.BlendSpace = render.BlendSpaceLinear

	count := func(format types.TextureFormat) int64 {
		before := shared.linearFixedFunctionFallbacks.Load()
		if err := rc.FillPath(queued, path, plus, render.Identity()); err != nil {
			t.Fatalf("queue FillPath (%v): %v", format, err)
		}
		if got := shared.linearFixedFunctionFallbacks.Load() - before; got != 0 {
			t.Fatalf("counted %d before the target was known (%v), want 0", got, format)
		}
		view, release, err := rc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			t.Fatalf("create offscreen (%v): %v", format, err)
		}
		target := render.GPURenderTarget{
			View: view, ViewWidth: size, ViewHeight: size,
			Data: data, Width: size, Height: size, Stride: size * 4,
		}
		if err := rc.Flush(target); err != nil {
			release()
			t.Fatalf("Flush (%v): %v", format, err)
		}
		release()
		return shared.linearFixedFunctionFallbacks.Load() - before
	}

	if got := count(types.TextureFormatRGBA8UnormSrgb); got != 0 {
		t.Fatalf("sRGB target counted %d fixed-function linear fallbacks, want 0 (hardware blends in linear light)", got)
	}
	if got := count(types.TextureFormatRGBA8Unorm); got != 1 {
		t.Fatalf("Unorm target counted %d fixed-function linear fallbacks, want 1", got)
	}
}

// p31bFixedBlendModes is the complete fixed-function set: every mode that
// gpuBlendStateForPaint maps to hardware blend factors.
var p31bFixedBlendModes = []render.BlendMode{
	render.BlendNormal,
	render.BlendCopy,
	render.BlendClear,
	render.BlendPlus,
	render.BlendModulate,
	render.BlendDestinationOut,
	render.BlendSourceAtop,
	render.BlendXor,
	render.BlendDestinationOver,
	render.BlendSourceIn,
	render.BlendSourceOut,
	render.BlendDestinationIn,
	render.BlendDestinationAtop,
}

// TestP31bLinearTargetAllFixedBlendModes pins the full fixed-function matrix
// on an opt-in sRGB target. The expected value is computed in linear light
// (premultiplied) and sRGB-encoded exactly like the hardware attachment does;
// the Unorm leg is the control: it stays in the encoded domain and every mode
// is counted as a deliberate linear-request degradation.
func TestP31bLinearTargetAllFixedBlendModes(t *testing.T) {
	shared, rc := newP2Shared(t)
	const size = 64
	dstC := [3]float64{0.5, 0.25, 0.1}
	srcC := [3]float64{0.25, 0.5, 0.75}
	const srcA = 0.5

	run := func(format types.TextureFormat, mode render.BlendMode) ([4]byte, int64) {
		view, release, err := rc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			t.Fatalf("CreateOffscreenTextureFormat(%v): %v", format, err)
		}
		defer release()
		target := render.GPURenderTarget{
			View: view, ViewWidth: size, ViewHeight: size,
			Width: size, Height: size, Stride: size * 4,
		}
		path := render.NewPath()
		path.Rectangle(0, 0, size, size)
		base := render.NewPaint()
		base.SetBrush(render.Solid(render.RGBA{R: dstC[0], G: dstC[1], B: dstC[2], A: 1}))
		if err := rc.FillPath(target, path, base, render.Identity()); err != nil {
			t.Fatalf("FillPath base (%v/%v): %v", format, mode, err)
		}
		before := shared.linearFixedFunctionFallbacks.Load()
		src := render.NewPaint()
		src.SetBrush(render.Solid(render.RGBA{R: srcC[0], G: srcC[1], B: srcC[2], A: srcA}))
		src.BlendMode = mode
		src.BlendSpace = render.BlendSpaceLinear
		if err := rc.FillPath(target, path, src, render.Identity()); err != nil {
			t.Fatalf("FillPath src (%v/%v): %v", format, mode, err)
		}
		if err := rc.Flush(target); err != nil {
			t.Fatalf("Flush (%v/%v): %v", format, mode, err)
		}
		delta := shared.linearFixedFunctionFallbacks.Load() - before
		return p31bReadCenter(t, shared, view, size), delta
	}

	modeName := map[render.BlendMode]string{
		render.BlendNormal: "normal", render.BlendCopy: "copy", render.BlendClear: "clear",
		render.BlendPlus: "plus", render.BlendModulate: "modulate",
		render.BlendDestinationOut: "destination-out", render.BlendSourceAtop: "source-atop",
		render.BlendXor: "xor", render.BlendDestinationOver: "destination-over",
		render.BlendSourceIn: "source-in", render.BlendSourceOut: "source-out",
		render.BlendDestinationIn: "destination-in", render.BlendDestinationAtop: "destination-atop",
	}

	for _, mode := range p31bFixedBlendModes {
		got, fallback := run(types.TextureFormatRGBA8UnormSrgb, mode)
		want := p31bFixedBlendExpected(mode, dstC, srcC, srcA)
		for i := 0; i < 4; i++ {
			if d := int(got[i]) - int(want[i]); d < -3 || d > 3 {
				t.Fatalf("%s sRGB center=%v, want %v±3", modeName[mode], got, want)
			}
		}
		if fallback != 0 {
			t.Fatalf("%s sRGB target counted %d fixed-function linear fallbacks, want 0", modeName[mode], fallback)
		}

		_, fallback = run(types.TextureFormatRGBA8Unorm, mode)
		if fallback != 1 {
			t.Fatalf("%s Unorm target counted %d fixed-function linear fallbacks, want 1", modeName[mode], fallback)
		}
	}
}

// p31bFixedBlendExpected mirrors the fixed-function blend equations in linear
// light for an opaque destination and a straight-alpha source. RGB is
// premultiplied before the sRGB attachment encoding (what the hardware does
// for an *-srgb format); alpha is never gamma-encoded.
func p31bFixedBlendExpected(mode render.BlendMode, dstC, srcC [3]float64, srcA float64) [4]byte {
	d := [3]float64{float64(srgbToLinear(float32(dstC[0]))), float64(srgbToLinear(float32(dstC[1]))), float64(srgbToLinear(float32(dstC[2])))}
	s := [3]float64{float64(srgbToLinear(float32(srcC[0]))), float64(srgbToLinear(float32(srcC[1]))), float64(srgbToLinear(float32(srcC[2])))}
	for i := range s {
		s[i] *= srcA
	}
	var o [3]float64
	var ao float64
	// dst alpha is 1 for every leg (base fill is opaque).
	switch mode {
	case render.BlendNormal:
		for i := range o {
			o[i] = s[i] + d[i]*(1-srcA)
		}
		ao = 1
	case render.BlendCopy:
		o = s
		ao = srcA
	case render.BlendClear:
		ao = 0
	case render.BlendPlus:
		for i := range o {
			o[i] = s[i] + d[i]
		}
		ao = 1.5
	case render.BlendModulate:
		for i := range o {
			o[i] = s[i] * d[i]
		}
		ao = srcA
	case render.BlendDestinationOut:
		for i := range o {
			o[i] = d[i] * (1 - srcA)
		}
		ao = 1 - srcA
	case render.BlendSourceAtop:
		for i := range o {
			o[i] = s[i] + d[i]*(1-srcA)
		}
		ao = 1
	case render.BlendXor:
		for i := range o {
			o[i] = d[i] * (1 - srcA)
		}
		ao = 1 - srcA
	case render.BlendDestinationOver:
		o = d
		ao = 1
	case render.BlendSourceIn:
		o = s
		ao = srcA
	case render.BlendSourceOut:
		ao = 0
	case render.BlendDestinationIn:
		for i := range o {
			o[i] = d[i] * srcA
		}
		ao = srcA
	case render.BlendDestinationAtop:
		for i := range o {
			o[i] = d[i] * srcA
		}
		ao = srcA
	default:
		panic("unhandled fixed blend mode")
	}
	var out [4]byte
	for i := 0; i < 3; i++ {
		out[i] = uint8(mathRound(clamp01(float64(linearToSRGB(float32(o[i])))) * 255))
	}
	out[3] = uint8(mathRound(clamp01(ao) * 255))
	return out
}

func mathRound(v float64) float64 {
	if v < 0 {
		return 0
	}
	return float64(int(v + 0.5))
}

// p31bReadCenter copies the center pixel of an offscreen view into RGBA bytes.
func p31bReadCenter(t *testing.T, shared *GPUShared, view gpucontext.TextureView, size int) [4]byte {
	t.Helper()
	tex := unpackView(view).Texture()
	device, queue := shared.Device(), shared.Queue()
	rowBytes := size * 4
	staging, err := device.CreateBuffer(&hal.BufferDescriptor{
		Label: "p31b_blend_table_staging", Size: uint64(rowBytes * size),
		Usage: types.BufferUsageMapRead | types.BufferUsageCopyDst,
	})
	if err != nil {
		t.Fatalf("staging: %v", err)
	}
	defer staging.Destroy()
	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p31b_blend_table_copy"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	enc.CopyTextureToBuffer(tex, staging, []hal.BufferTextureCopy{{
		BufferLayout: hal.ImageDataLayout{Offset: 0, BytesPerRow: uint32(rowBytes), RowsPerImage: uint32(size)},
		TextureBase:  hal.ImageCopyTexture{Texture: tex, MipLevel: 0, Origin: hal.Origin3D{}, Aspect: types.TextureAspectAll},
		Size:         hal.Extent3D{Width: uint32(size), Height: uint32(size), DepthOrArrayLayers: 1},
	}})
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish copy: %v", err)
	}
	defer device.FreeCommandBuffer(cmd)
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit copy: %v", err)
	}
	mapping, err := device.MapBuffer(staging, 0, uint64(rowBytes*size))
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	raw := make([]byte, rowBytes*size)
	copy(raw, unsafe.Slice((*byte)(mapping.Ptr), rowBytes*size))
	if err := device.UnmapBuffer(staging); err != nil {
		t.Fatalf("unmap: %v", err)
	}
	center := size/2*rowBytes + size/2*4
	var out [4]byte
	copy(out[:], raw[center:center+4])
	return out
}
