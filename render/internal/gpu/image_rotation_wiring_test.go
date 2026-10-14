//go:build !nogpu

package gpu

import (
	"math"
	"testing"

	"github.com/energye/gpui/render/internal/gpu/res"
)

// rigidGPUCmd builds a GPU texture overlay command with baked rotated
// corners around (cx,cy), driving the real shipped quad path.
func rigidGPUCmd(view res.View, cx, cy, w, h, angle float64) GPUTextureDrawCommand {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	mk := func(lx, ly float64) (float32, float32) {
		return float32(cos*(lx-cx) - sin*(ly-cy) + cx),
			float32(sin*(lx-cx) + cos*(ly-cy) + cy)
	}
	x0, y0 := cx-w/2, cy-h/2
	x1, y1 := cx+w/2, cy+h/2
	tlx, tly := mk(x0, y0)
	trx, try_ := mk(x1, y0)
	brx, bry := mk(x1, y1)
	blx, bly := mk(x0, y1)
	minX := math.Min(math.Min(float64(tlx), float64(trx)), math.Min(float64(brx), float64(blx)))
	minY := math.Min(math.Min(float64(tly), float64(try_)), math.Min(float64(bry), float64(bly)))
	maxX := math.Max(math.Max(float64(tlx), float64(trx)), math.Max(float64(brx), float64(blx)))
	maxY := math.Max(math.Max(float64(tly), float64(try_)), math.Max(float64(bry), float64(bly)))
	return GPUTextureDrawCommand{
		View: view, Opacity: 1.0, ViewportWidth: 480, ViewportHeight: 320,
		DstX: float32(minX), DstY: float32(minY),
		DstW: float32(maxX - minX), DstH: float32(maxY - minY),
		TLX: tlx, TLY: tly, TRX: trx, TRY: try_,
		BRX: brx, BRY: bry, BLX: blx, BLY: bly,
		U0: 0, V0: 0, U1: 1, V1: 1,
	}
}

func TestWiring_RotatedAnglesDoNotMerge(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 11})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.3)
	b := rigidGPUCmd(v, 240, 160, 64, 64, 0.9)
	if canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("different rotation angles must NOT share one uniform slot (angles would freeze)")
	}
}

func TestWiring_SameAngleTripleMerges(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 12})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b := a // bit-identical quad: same center + size + angle
	if !canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("identical rotation key should still merge (shared uniform valid)")
	}
}

func TestWiring_SameAngleDifferentCenterDoNotMerge(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 18})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b := rigidGPUCmd(v, 100, 100, 64, 64, 0.5)
	if canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("same angle at different centers must NOT merge (slot keeps only head rotation, overlay would misplace)")
	}
}

func TestWiring_SameAngleDifferentSizeDoNotMerge(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 19})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b := rigidGPUCmd(v, 240, 160, 32, 32, 0.5)
	if canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("same angle with different sizes must NOT merge (base vertices differ, head rotation misplaces)")
	}
}

func TestWiring_RotatedVsBakedDoNotMerge(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 13})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b := GPUTextureDrawCommand{
		View: v, Opacity: 1.0, ViewportWidth: 480, ViewportHeight: 320,
		DstX: 208, DstY: 128, DstW: 64, DstH: 64,
		U0: 0, V0: 0, U1: 1, V1: 1,
	}
	if canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("rotated quad must NOT merge with baked/axis quad (different uniform terms)")
	}
}

func TestWiring_AxisQuadsStillMerge(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 14})
	a := GPUTextureDrawCommand{
		View: v, Opacity: 1.0, ViewportWidth: 480, ViewportHeight: 320,
		DstX: 0, DstY: 0, DstW: 8, DstH: 8, U0: 0, V0: 0, U1: 1, V1: 1,
	}
	b := GPUTextureDrawCommand{
		View: v, Opacity: 1.0, ViewportWidth: 480, ViewportHeight: 320,
		DstX: 8, DstY: 0, DstW: 8, DstH: 8, U0: 0, V0: 0, U1: 1, V1: 1,
	}
	if !canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("axis-aligned overlays must keep merging (bit-identical baked path)")
	}
}

func TestWiring_YUVNeverRotates(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 15})
	uv := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 16})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	a.UVView, a.IsYUV = uv, true
	if _, ok := gpuTexHasRotation(&a); ok {
		t.Fatal("YUV draws must stay baked (own pipeline, never rotation-uniform)")
	}
	b := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b.UVView, b.IsYUV = uv, true
	if canMergeGPUTextureDraw(&a, &b) {
		t.Fatal("YUV draws must never merge")
	}
}

// TestWiring_BaseStableAcrossAngles drives the shipped vertex path:
// same center/size at different angles must emit byte-identical base
// vertices (hash reuse), while slot uniforms differ per angle.
func TestWiring_BaseStableAcrossAngles(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 17})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.3)
	b := rigidGPUCmd(v, 240, 160, 64, 64, 0.9)
	ax0, ay0, ax1, ay1, ax2, ay2, ax3, ay3 := gpuTexBaseCorners(&a)
	bx0, by0, bx1, by1, bx2, by2, bx3, by3 := gpuTexBaseCorners(&b)
	if ax0 != bx0 || ay0 != by0 || ax1 != bx1 || ay1 != by1 ||
		ax2 != bx2 || ay2 != by2 || ax3 != bx3 || ay3 != by3 {
		t.Fatalf("base must be angle-independent, got %v vs %v bits %08x/%08x/%08x/%08x/%08x/%08x/%08x/%08x vs %08x/%08x/%08x/%08x/%08x/%08x/%08x/%08x",
			[8]float32{ax0, ay0, ax1, ay1, ax2, ay2, ax3, ay3},
			[8]float32{bx0, by0, bx1, by1, bx2, by2, bx3, by3},
			math.Float32bits(ax0), math.Float32bits(ay0),
			math.Float32bits(ax1), math.Float32bits(ay1),
			math.Float32bits(ax2), math.Float32bits(ay2),
			math.Float32bits(ax3), math.Float32bits(ay3),
			math.Float32bits(bx0), math.Float32bits(by0),
			math.Float32bits(bx1), math.Float32bits(by1),
			math.Float32bits(bx2), math.Float32bits(by2),
			math.Float32bits(bx3), math.Float32bits(by3))
	}
	// Same base through the shipped emitter must be byte-identical.
	mkCmd := func(tlx, tly, trx, try_, brx, bry, blx, bly float32) *ImageDrawCommand {
		return &ImageDrawCommand{
			TLX: tlx, TLY: tly, TRX: trx, TRY: try_,
			BRX: brx, BRY: bry, BLX: blx, BLY: bly,
			U0: 0, V0: 0, U1: 1, V1: 1,
		}
	}
	va := buildImageVertices(mkCmd(ax0, ay0, ax1, ay1, ax2, ay2, ax3, ay3))
	vb := buildImageVertices(mkCmd(bx0, by0, bx1, by1, bx2, by2, bx3, by3))
	for i := range va {
		if va[i] != vb[i] {
			t.Fatalf("base vertex byte %d differs across angles: reuse would miss", i)
		}
	}
	// Uniforms must differ (angle term), or the second frame would freeze.
	arot, aok := gpuTexHasRotation(&a)
	brot, bok := gpuTexHasRotation(&b)
	if !aok || !bok {
		t.Fatal("setup: both quads must take the rotation path")
	}
	ua := make([]byte, imageUniformSize)
	ub := make([]byte, imageUniformSize)
	writeGpuTexUniform(ua, 480, 320, 1.0, arot, aok)
	writeGpuTexUniform(ub, 480, 320, 1.0, brot, bok)
	same := true
	for i := range ua {
		if ua[i] != ub[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("uniforms must differ across angles (per-draw rotation term)")
	}
	// Key must track the angle triple.
	var ka, kb imageUniformLastKey
	storeImageUniformKey(&ka, 480, 320, 1.0, arot, aok)
	storeImageUniformKey(&kb, 480, 320, 1.0, brot, bok)
	if imageUniformKeyMatches(&ka, 480, 320, 1.0, brot, bok) {
		t.Fatal("uniform key must not match across angles")
	}
	if !imageUniformKeyMatches(&ka, 480, 320, 1.0, arot, aok) {
		t.Fatal("uniform key must match identical angle triple")
	}
}

func TestWiring_UniformKeyTracksSize(t *testing.T) {
	v := res.ViewFromKey(res.SourceKey{Kind: res.KindTextureView, Role: res.RoleCoverResult, Index: 20})
	a := rigidGPUCmd(v, 240, 160, 64, 64, 0.5)
	b := rigidGPUCmd(v, 240, 160, 32, 32, 0.5)
	arot, aok := gpuTexHasRotation(&a)
	brot, bok := gpuTexHasRotation(&b)
	if !aok || !bok {
		t.Fatal("setup: both quads must take the rotation path")
	}
	var ka imageUniformLastKey
	storeImageUniformKey(&ka, 480, 320, 1.0, arot, aok)
	if imageUniformKeyMatches(&ka, 480, 320, 1.0, brot, bok) {
		t.Fatal("uniform key must not match across sizes (base vertices differ)")
	}
}
