//go:build !nogpu

package render_test

import (
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"

	_ "github.com/energye/gpui/render/gpu"
)

// P3.1b-3: compositing a GPU texture view must convert between the source
// texture's color space and the destination target's color space exactly once.
//
// The four combinations of {Unorm, sRGB} source x {Unorm, sRGB} destination
// must all reproduce the same visible sRGB color. A missing source-side flag
// shows up as a too-dark sRGB source composited into a Unorm window target
// (hardware sRGB decode followed by a raw write), or a double-decoded sRGB
// source on an sRGB destination.
func TestP31bCompositeSourceColorSpace(t *testing.T) {
	const size = 64

	makeSource := func(format types.TextureFormat) (gpucontext.TextureView, func()) {
		dc := render.NewContext(size, size)
		dc.SetRGBA(0.4, 0.2, 0.1, 1)
		dc.DrawRectangle(0, 0, size, size)
		if err := dc.Fill(); err != nil {
			dc.Close()
			t.Fatalf("source fill (%v): %v", format, err)
		}
		view, release, err := dc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			dc.Close()
			t.Fatalf("source offscreen (%v): %v", format, err)
		}
		if err := dc.FlushGPUWithView(view, size, size); err != nil {
			release()
			dc.Close()
			t.Fatalf("source flush (%v): %v", format, err)
		}
		// Keep the source context (and its device resources) alive while the
		// view is composited by another context.
		return view, func() { release(); dc.Close() }
	}

	composite := func(src gpucontext.TextureView, dstFormat types.TextureFormat) [4]byte {
		dc := render.NewContext(size, size)
		defer dc.Close()
		dc.DrawGPUTexture(src, 0, 0, size, size)
		view, release, err := dc.CreateOffscreenTextureFormat(size, size, dstFormat)
		if err != nil {
			t.Fatalf("dst offscreen (%v): %v", dstFormat, err)
		}
		defer release()
		if err := dc.FlushGPUWithView(view, size, size); err != nil {
			t.Fatalf("dst flush (%v): %v", dstFormat, err)
		}
		rb := dc.GPURenderContext().(interface {
			ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
		})
		rgba, err := rb.ReadbackViewStraightRGBA(view, size, size)
		if err != nil {
			t.Fatalf("readback (%v): %v", dstFormat, err)
		}
		center := (size/2*size + size/2) * 4
		var got [4]byte
		copy(got[:], rgba[center:center+4])
		return got
	}

	// sRGB-encoded input 0.4/0.2/0.1 -> 102/51/26. A correct round trip keeps
	// those bytes visible regardless of which color space the intermediate
	// surface uses.
	want := [3]int{102, 51, 26}
	sources := []struct {
		name   string
		format types.TextureFormat
	}{
		{"unorm", types.TextureFormatRGBA8Unorm},
		{"srgb", types.TextureFormatRGBA8UnormSrgb},
	}
	dests := []struct {
		name   string
		format types.TextureFormat
	}{
		{"unorm", types.TextureFormatRGBA8Unorm},
		{"srgb", types.TextureFormatRGBA8UnormSrgb},
	}
	for _, src := range sources {
		view, closeSrc := makeSource(src.format)
		for _, dst := range dests {
			got := composite(view, dst.format)
			for i := 0; i < 3; i++ {
				if d := int(got[i]) - want[i]; d < -3 || d > 3 {
					closeSrc()
					t.Fatalf("src=%s dst=%s center=%v, want %v±3", src.name, dst.name, got, want)
				}
			}
			t.Logf("P3.1b composite src=%s dst=%s center=%v", src.name, dst.name, got)
		}
		closeSrc()
	}
}

// P3.1b-3: semi-transparent targets store premultiplied values per target
// convention: a Unorm target stores premultiplied sRGB-encoded bytes (64),
// an sRGB target stores premultiplied linear light encoded per channel (93).
// Both represent the same visible color; compositing must preserve the
// convention of the destination instead of leaking one into the other.
func TestP31bSemiTransparentSrgbTarget(t *testing.T) {
	const size = 64

	fill := func(format types.TextureFormat) ([4]byte, gpucontext.TextureView, func()) {
		dc := render.NewContext(size, size)
		dc.SetRGBA(0.5, 0.5, 0.5, 0.5)
		dc.DrawRectangle(0, 0, size, size)
		if err := dc.Fill(); err != nil {
			dc.Close()
			t.Fatalf("fill (%v): %v", format, err)
		}
		view, release, err := dc.CreateOffscreenTextureFormat(size, size, format)
		if err != nil {
			dc.Close()
			t.Fatalf("offscreen (%v): %v", format, err)
		}
		if err := dc.FlushGPUWithView(view, size, size); err != nil {
			release()
			dc.Close()
			t.Fatalf("flush (%v): %v", format, err)
		}
		rb := dc.GPURenderContext().(interface {
			ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
		})
		rgba, err := rb.ReadbackViewStraightRGBA(view, size, size)
		if err != nil {
			release()
			dc.Close()
			t.Fatalf("readback (%v): %v", format, err)
		}
		center := (size/2*size + size/2) * 4
		var got [4]byte
		copy(got[:], rgba[center:center+4])
		return got, view, func() { release(); dc.Close() }
	}

	composite := func(src gpucontext.TextureView, dstFormat types.TextureFormat) [4]byte {
		dc := render.NewContext(size, size)
		defer dc.Close()
		dc.DrawGPUTexture(src, 0, 0, size, size)
		view, release, err := dc.CreateOffscreenTextureFormat(size, size, dstFormat)
		if err != nil {
			t.Fatalf("dst offscreen (%v): %v", dstFormat, err)
		}
		defer release()
		if err := dc.FlushGPUWithView(view, size, size); err != nil {
			t.Fatalf("dst flush (%v): %v", dstFormat, err)
		}
		rb := dc.GPURenderContext().(interface {
			ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
		})
		rgba, err := rb.ReadbackViewStraightRGBA(view, size, size)
		if err != nil {
			t.Fatalf("dst readback (%v): %v", dstFormat, err)
		}
		center := (size/2*size + size/2) * 4
		var got [4]byte
		copy(got[:], rgba[center:center+4])
		return got
	}

	// Direct fills pin the two storage conventions.
	srgbDirect, srgbView, closeSrgb := fill(types.TextureFormatRGBA8UnormSrgb)
	if d := int(srgbDirect[0]) - 93; d < -3 || d > 3 {
		closeSrgb()
		t.Fatalf("semi-transparent sRGB target = %v, want premul-light 93±3", srgbDirect)
	}
	if d := int(srgbDirect[3]) - 128; d < -3 || d > 3 {
		closeSrgb()
		t.Fatalf("semi-transparent sRGB alpha = %d, want 128±3", srgbDirect[3])
	}
	unormDirect, unormView, closeUnorm := fill(types.TextureFormatRGBA8Unorm)
	if d := int(unormDirect[0]) - 64; d < -3 || d > 3 {
		closeSrgb()
		closeUnorm()
		t.Fatalf("semi-transparent Unorm target = %v, want premul-encoded 64±3", unormDirect)
	}
	if d := int(unormDirect[3]) - 128; d < -3 || d > 3 {
		closeSrgb()
		closeUnorm()
		t.Fatalf("semi-transparent Unorm alpha = %d, want 128±3", unormDirect[3])
	}

	// Four combinations must convert once into the destination's convention.
	// The composite pass clears its target to opaque black (window-surface
	// semantics). Over black, premultiplied SourceOver keeps the source's
	// premultiplied value: Unorm stores the encoded premul 0.25 (64), while
	// the sRGB target stores the linear premul 0.107 re-encoded (93). Alpha
	// becomes opaque because the destination surface is opaque.
	cases := []struct {
		name string
		src  gpucontext.TextureView
		dst  types.TextureFormat
		want int
	}{
		{"srgb-src→unorm-dst", srgbView, types.TextureFormatRGBA8Unorm, 64},
		{"srgb-src→srgb-dst", srgbView, types.TextureFormatRGBA8UnormSrgb, 93},
		{"unorm-src→unorm-dst", unormView, types.TextureFormatRGBA8Unorm, 64},
		{"unorm-src→srgb-dst", unormView, types.TextureFormatRGBA8UnormSrgb, 93},
	}
	for _, c := range cases {
		got := composite(c.src, c.dst)
		if d := int(got[0]) - c.want; d < -3 || d > 3 {
			closeSrgb()
			closeUnorm()
			t.Fatalf("%s = %v, want %d±3", c.name, got, c.want)
		}
		if got[3] != 255 {
			closeSrgb()
			closeUnorm()
			t.Fatalf("%s alpha = %d, want 255 (opaque composite target)", c.name, got[3])
		}
	}
	closeSrgb()
	closeUnorm()
}
