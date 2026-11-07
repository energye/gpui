//go:build !nogpu

package gpu

import (
	"fmt"
	"hash/fnv"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render/text"
	"golang.org/x/image/font/gofont/goregular"
)

func TestOpt26_ComputeGlyphMaskFontID_MatchesLegacyFmt(t *testing.T) {
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	got := computeGlyphMaskFontID(src)
	// Identity since the P3.8 batch-5 same-name atlas fix: FNV64a of
	// "name:numGlyphs" + ":" + raw font bytes (RawFontDataProvider). The
	// raw-byte tail disambiguates sibling builds of the same family, whose
	// color rasters must never share an atlas key.
	parsed := src.Parsed()
	fullName := parsed.FullName()
	if fullName == "" {
		fullName = src.Name()
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%d", fullName, parsed.NumGlyphs())
	_, _ = h.Write([]byte{':'})
	if p, ok := parsed.(text.RawFontDataProvider); ok {
		_, _ = h.Write(p.RawFontData())
	}
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

// TestOpt26_FontID_DistinguishesSameNameSiblings pins the batch-5 fix: two
// fonts with identical FullName+NumGlyphs but different raw bytes must get
// different atlas keys (the color-table-spliced derivatives used to collide
// and swap rasters across cards).
func TestOpt26_FontID_DistinguishesSameNameSiblings(t *testing.T) {
	src, err := text.NewFontSource(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	// Splice one extra byte into the font data: same parsed identity
	// (name/glyph count), different content.
	spliced := append([]byte(nil), goregular.TTF...)
	spliced[len(spliced)-1] ^= 0xFF
	src2, err := text.NewFontSource(spliced)
	if err != nil {
		t.Fatal(err)
	}
	id1 := computeGlyphMaskFontID(src)
	id2 := computeGlyphMaskFontID(src2)
	if id1 == id2 {
		t.Fatalf("same-name sibling fonts share fontID %x — atlas key collision", id1)
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
	mkView := func(label string) (hal.Texture, hal.TextureView) {
		tex, err := device.CreateTexture(&hal.TextureDescriptor{
			Label: label, Size: hal.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatBGRA8Unorm,
			Usage:  types.TextureUsageTextureBinding | types.TextureUsageRenderAttachment,
		})
		if err != nil {
			t.Fatal(err)
		}
		view, err := device.CreateTextureView(tex, &hal.TextureViewDescriptor{
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
		dstView.Destroy()
		dstTex.Destroy()
		srcView.Destroy()
		srcTex.Destroy()
	})
	slab, err := cache.ensureUniformSlab(device, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := dualTexWriteParams(queue, slab, 1, 0, 0, 1, 1, 1, false, false); err != nil {
		t.Fatal(err)
	}
	bg1, err := cache.multiBindGroup(device, cache.bgl, cache.sampler, dstView, srcView, slab, 0, 0)
	if err != nil || bg1 == nil {
		t.Fatalf("bg1: %v", err)
	}
	bg2, err := cache.multiBindGroup(device, cache.bgl, cache.sampler, dstView, srcView, slab, 0, 0)
	if err != nil || bg2 != bg1 {
		t.Fatalf("expected slot reuse, bg2=%v err=%v", bg2, err)
	}
}
