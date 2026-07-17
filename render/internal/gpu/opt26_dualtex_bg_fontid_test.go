//go:build !nogpu

package gpu

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render/text"
	"golang.org/x/image/font/gofont/goregular"
)

func TestOpt26_ComputeGlyphMaskFontID_MatchesLegacyFmt(t *testing.T) {
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	got := computeGlyphMaskFontID(src)
	// Legacy identity: FNV64a of fmt.Sprintf("%s:%d", name, numGlyphs)
	parsed := src.Parsed()
	fullName := parsed.FullName()
	if fullName == "" {
		fullName = src.Name()
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%d", fullName, parsed.NumGlyphs())
	want := h.Sum64()
	if got != want {
		t.Fatalf("fontID mismatch: got %x want %x name=%q n=%d", got, want, fullName, parsed.NumGlyphs())
	}
	eng := NewGlyphMaskEngine()
	if eng.fontID(src) != got || eng.fontID(src) != got {
		t.Fatal("fontID cache inconsistent")
	}
	if len(eng.fontIDCache) != 1 {
		t.Fatalf("cache entries=%d", len(eng.fontIDCache))
	}
}

func TestOpt26_IndexBytesFingerprint_Stable(t *testing.T) {
	b := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	if indexBytesFingerprint(b) != indexBytesFingerprint(append([]byte(nil), b...)) {
		t.Fatal("fingerprint not stable")
	}
	if indexBytesFingerprint(b) == indexBytesFingerprint(b[:8]) {
		t.Fatal("len should affect fingerprint")
	}
}

func TestOpt26_DualTexMultiBindGroup_ReusesSlot(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	var cache dualTexBlendCache
	t.Cleanup(func() { cache.release() })
	if err := cache.ensure(device); err != nil {
		t.Fatal(err)
	}
	// Minimal 1x1 BGRA textures + views for bind group entries.
	mkView := func(label string) (*webgpu.Texture, *webgpu.TextureView) {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
			Label: label + "_v", Format: types.TextureFormatBGRA8Unorm,
			Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		return tex, view
	}
	dstTex, dstView := mkView("dst")
	srcTex, srcView := mkView("src")
	t.Cleanup(func() {
		dstView.Release()
		dstTex.Release()
		srcView.Release()
		srcTex.Release()
	})
	if err := cache.acquireUniformRing(device, 1); err != nil {
		t.Fatal(err)
	}
	ubuf := cache.uniformRing[0]
	if err := dualTexWriteParams(queue, ubuf, 1, 0, 0, 1, 1, 1, false); err != nil {
		t.Fatal(err)
	}
	bg1, err := cache.multiBindGroup(device, cache.bgl, cache.sampler, dstView, srcView, ubuf, 0)
	if err != nil || bg1 == nil {
		t.Fatalf("bg1: %v", err)
	}
	bg2, err := cache.multiBindGroup(device, cache.bgl, cache.sampler, dstView, srcView, ubuf, 0)
	if err != nil || bg2 != bg1 {
		t.Fatalf("expected slot reuse, bg2=%v err=%v", bg2, err)
	}
}
