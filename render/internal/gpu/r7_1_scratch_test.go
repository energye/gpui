//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestR71_DualTexParamsPool_NoAllocHot verifies the R7.1 48-byte params pool
// is alloc-free on the warm path (Get/clear/Put).
func TestR71_DualTexParamsPool_NoAllocHot(t *testing.T) {
	// Warm the pool once.
	p := dualTexParamsPool.Get().(*[]byte)
	if cap(*p) < 48 {
		*p = make([]byte, 48)
	}
	dualTexParamsPool.Put(p)

	allocs := testing.AllocsPerRun(500, func() {
		p := dualTexParamsPool.Get().(*[]byte)
		data := (*p)[:48]
		clear(data)
		dualTexParamsPool.Put(p)
	})
	if allocs != 0 {
		t.Fatalf("dualTexParamsPool warm allocs=%v want 0", allocs)
	}
}

// TestR71_GlyphMaskLCDUniformInto_NoAllocHot warm scratch reuses buffer.
func TestR71_GlyphMaskLCDUniformInto_NoAllocHot(t *testing.T) {
	var scratch []byte
	tr := render.Identity()
	col := [4]float32{1, 1, 1, 1}
	scratch = makeGlyphMaskLCDUniformInto(scratch, tr, col, 512, 512)
	if len(scratch) != int(glyphMaskLCDUniformSize) {
		t.Fatalf("len=%d want %d", len(scratch), glyphMaskLCDUniformSize)
	}
	allocs := testing.AllocsPerRun(200, func() {
		scratch = makeGlyphMaskLCDUniformInto(scratch, tr, col, 512, 512)
	})
	if allocs != 0 {
		t.Fatalf("LCD Into warm allocs=%v want 0", allocs)
	}
	// Pixel-identical to non-Into.
	a := makeGlyphMaskLCDUniform(tr, col, 256, 128)
	b := makeGlyphMaskLCDUniformInto(nil, tr, col, 256, 128)
	if len(a) != len(b) {
		t.Fatalf("len a=%d b=%d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("byte %d: %d vs %d", i, a[i], b[i])
		}
	}
}

// TestR71_ImageStagingAcquireRelease_RoundTrip ensures pool returns usable slices.
func TestR71_ImageStagingAcquireRelease_RoundTrip(t *testing.T) {
	p := acquireImageStaging(4096)
	if len(*p) != 4096 {
		t.Fatalf("len=%d", len(*p))
	}
	(*p)[0] = 0xab
	releaseImageStaging(p)
	p2 := acquireImageStaging(2048)
	if len(*p2) != 2048 {
		t.Fatalf("len2=%d", len(*p2))
	}
	releaseImageStaging(p2)
}
