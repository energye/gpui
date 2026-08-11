//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
)

// P5: bind-group caches key on the RESOLVED native view identity. A texture
// rebuild resolves to a different view → cache miss → new BG, and the old BG
// goes to pendingBindGroupRelease (post-submit release). Same view → reuse.
//
// This is verified for glyph mask bind groups (materialize path); the image
// (imageBindGroups: texView+nearest) and filter (filterBGKey: view/buffer
// pointers) caches follow the same resolved-identity semantics and were
// audited against the code directly.

func TestP5_GlyphMaskBindGroup_KeySemantics(t *testing.T) {
	device, queue, cleanup := createNativeTestDevice(t)
	if device == nil {
		t.Skip("no native wgpu device (createNativeTestDevice unavailable)")
	}
	t.Cleanup(cleanup)

	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensureGlyphMaskPipeline(false); err != nil {
		t.Skipf("glyph mask pipeline unavailable: %v", err)
	}

	mkView := func(label string) *webgpu.TextureView {
		tex, err := device.CreateTexture(&webgpu.TextureDescriptor{
			Label: label, Size: webgpu.Extent3D{Width: 8, Height: 8, DepthOrArrayLayers: 1},
			MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
			Format: types.TextureFormatR8Unorm,
			Usage:  types.TextureUsageTextureBinding,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { tex.Release() })
		v, err := device.CreateTextureView(tex, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { v.Release() })
		return v
	}
	v1, v2 := mkView("p5_glyph_v1"), mkView("p5_glyph_v2")

	s.glyphMaskBindGroups = make([]*webgpu.BindGroup, 1)
	s.glyphMaskBGViews = make([]*webgpu.TextureView, 1)
	s.glyphMaskBGIsLCD = make([]bool, 1)
	s.ensureGlyphMaskBatchPools(1)

	// Same atlas view → reuse the same BG, pendingRelease untouched.
	s.glyphMaskPendingViews = []glyphMaskPendingView{{batchIndex: 0, atlasView: v1}}
	s.materializeGlyphMaskBindGroups()
	bg1 := s.glyphMaskBindGroups[0]
	if bg1 == nil {
		t.Fatal("bg1 must be created")
	}
	pendingBefore := len(s.pendingBindGroupRelease)
	s.glyphMaskPendingViews = []glyphMaskPendingView{{batchIndex: 0, atlasView: v1}}
	s.materializeGlyphMaskBindGroups()
	if s.glyphMaskBindGroups[0] != bg1 {
		t.Fatal("same atlas view must reuse the BG")
	}
	if len(s.pendingBindGroupRelease) != pendingBefore {
		t.Fatalf("reuse must not retire a BG, pending %d→%d", pendingBefore, len(s.pendingBindGroupRelease))
	}

	// Atlas texture replaced → resolves to a new view → BG rebuilt; the old
	// BG is deferred to pendingBindGroupRelease (post-submit release), not
	// released mid-frame.
	s.glyphMaskPendingViews = []glyphMaskPendingView{{batchIndex: 0, atlasView: v2}}
	s.materializeGlyphMaskBindGroups()
	if s.glyphMaskBindGroups[0] == bg1 {
		t.Fatal("new atlas view must rebuild the BG")
	}
	found := false
	for _, bg := range s.pendingBindGroupRelease {
		if bg == bg1 {
			found = true
		}
	}
	if !found {
		t.Fatal("old BG must be deferred to pendingBindGroupRelease")
	}
}