//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scene

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
)

// TestCPUTileLayerOpacity pins the CPU tile renderer's layer opacity.
//
// Regression: executeEncodingOnTile ignored TagPushLayer alpha (`_ = alpha`)
// so a 50% layer composited as fully opaque while the GPU path applied the
// opacity. Same scene, two backends, different pixels.
func TestCPUTileLayerOpacity(t *testing.T) {
	b := NewSceneBuilder()
	b.FillRect(0, 0, 64, 64, SolidBrush(render.RGBA{R: 1, A: 1}))
	b.Layer(BlendNormal, 0.5, nil, func(lb *SceneBuilder) {
		lb.FillRect(0, 0, 32, 32, SolidBrush(render.RGBA{B: 1, A: 1}))
	})

	target := render.NewPixmap(64, 64)
	r := NewRenderer(64, 64, WithWorkers(1))
	if err := r.Render(target, b.Build()); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// 50% blue over opaque red: premultiplied bytes are (127,0,128,255).
	got := target.At(10, 10)
	gr, gg, gb, _ := got.RGBA()
	r8, g8, b8 := float64(gr>>8), float64(gg>>8), float64(gb>>8)
	if math.Abs(r8-127) > 2 || math.Abs(g8-0) > 2 || math.Abs(b8-128) > 2 {
		t.Fatalf("layer opacity lost: got (%.0f,%.0f,%.0f) want ~(127,0,128)", r8, g8, b8)
	}
	// Outside the layer rect the opaque red base must stay untouched.
	or, og, ob, _ := target.At(48, 48).RGBA()
	if or>>8 != 255 || og>>8 != 0 || ob>>8 != 0 {
		t.Fatalf("outside layer: got (%d,%d,%d) want (255,0,0)", or>>8, og>>8, ob>>8)
	}
}

// TestCPUTileRoundRectRespectsClip pins rounded-rect fills against the active
// clip.
//
// Regression: TagFillRoundRect rendered into the tile pixmap (`pm`) instead of
// the active (clipped) pixmap, so a rounded rect ignored every enclosing clip.
func TestCPUTileRoundRectRespectsClip(t *testing.T) {
	const (
		bgR, bgG, bgB = 0.10, 0.20, 0.10
	)
	b := NewSceneBuilder()
	b.FillRect(0, 0, 64, 64, SolidBrush(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1}))
	b.Clip(NewRectShape(0, 0, 32, 64), func(cb *SceneBuilder) {
		cb.FillRoundRect(0, 0, 64, 64, 8, 8, SolidBrush(render.RGBA{R: 1, G: 1, B: 0, A: 1}))
	})

	target := render.NewPixmap(64, 64)
	r := NewRenderer(64, 64, WithWorkers(1))
	if err := r.Render(target, b.Build()); err != nil {
		t.Fatalf("Render: %v", err)
	}

	if r8, g8, b8, _ := target.At(10, 10).RGBA(); r8>>8 < 250 || g8>>8 < 250 || b8>>8 > 5 {
		t.Fatalf("inside clip: got (%d,%d,%d) want yellow", r8>>8, g8>>8, b8>>8)
	}
	// The rounded rect spans the full tile, but the right half is clipped away.
	if r8, g8, b8, _ := target.At(48, 32).RGBA(); math.Abs(float64(r8>>8)-25.5) > 2 ||
		math.Abs(float64(g8>>8)-51) > 2 || math.Abs(float64(b8>>8)-25.5) > 2 {
		t.Fatalf("outside clip: got (%d,%d,%d) want background (25,51,25)", r8>>8, g8>>8, b8>>8)
	}
}

// TestCPUTileRoundRectMultiTile pins rounded-rect fills across tile
// boundaries. Path.Transform returns a new path; ignoring that return left the
// rect in scene coordinates, so every tile but the first painted a shifted
// fragment (visible as vertical holes in the fill).
func TestCPUTileRoundRectMultiTile(t *testing.T) {
	b := NewSceneBuilder()
	b.FillRect(0, 0, 176, 150, SolidBrush(render.RGBA{R: 0.07, G: 0.08, B: 0.11, A: 1}))
	b.FillRoundRect(18, 18, 140, 114, 18, 18, SolidBrush(render.RGBA{R: 0.86, G: 0.20, B: 0.22, A: 1}))

	pm := render.NewPixmap(176, 150)
	r := NewRenderer(176, 150, WithWorkers(1))
	r.MarkAllDirty()
	if err := r.RenderDirty(pm, b.Build(), nil); err != nil {
		t.Fatalf("RenderDirty: %v", err)
	}
	// Centre (tile 1,1) and a point near the right edge (tile 2) must both be
	// the fill colour, not the background.
	for _, p := range [][2]int{{88, 75}, {120, 75}, {40, 40}, {150, 75}} {
		r8, g8, b8, _ := pm.At(p[0], p[1]).RGBA()
		if r8>>8 < 200 || g8>>8 > 80 || b8>>8 > 80 {
			t.Fatalf("pixel %v got (%d,%d,%d), want rounded-rect red", p, r8>>8, g8>>8, b8>>8)
		}
	}
	if r8, g8, b8, _ := pm.At(170, 140).RGBA(); r8>>8 > 40 || g8>>8 > 40 || b8>>8 > 40 {
		t.Fatalf("outside pixel got (%d,%d,%d), want background", r8>>8, g8>>8, b8>>8)
	}
}

// TestSceneDrawTransformResetsAfterTransformedDraw pins the decoder's
// stateful transform contract.
//
// Regression: Scene.Fill/Stroke/clip only emitted TagTransform when the draw
// transform was non-identity. After one transformed draw, every following
// identity draw (including clip masks and their content) inherited the stale
// transform, so the circle/round-rect clips rendered off-position while the
// GPU and CPU backends agreed with each other — and both were wrong.
func TestSceneDrawTransformResetsAfterTransformedDraw(t *testing.T) {
	bg := render.RGBA{R: 0.10, G: 0.12, B: 0.16, A: 1}
	red := render.RGBA{R: 0.90, G: 0.25, B: 0.28, A: 1}
	blue := render.RGBA{R: 0.25, G: 0.45, B: 0.95, A: 1}

	b := NewSceneBuilder()
	b.FillRect(0, 0, 64, 64, SolidBrush(bg))
	// Translated draw leaves a non-identity transform in the stream.
	b.WithTransform(TranslateAffine(32, 32), func(tb *SceneBuilder) {
		tb.FillRect(-8, -8, 16, 16, SolidBrush(red))
	})
	// Identity clip + identity fill must land back at absolute coordinates.
	b.Clip(NewRectShape(40, 40, 20, 20), func(cb *SceneBuilder) {
		cb.FillRect(0, 0, 64, 64, SolidBrush(blue))
	})
	sc := b.Build()

	target := render.NewPixmap(64, 64)
	r := NewRenderer(64, 64, WithWorkers(1))
	if err := r.Render(target, sc); err != nil {
		t.Fatalf("Render: %v", err)
	}
	px := func(x, y int) [3]int {
		c := target.At(x, y)
		cr, cg, cb, _ := c.RGBA()
		return [3]int{int(cr >> 8), int(cg >> 8), int(cb >> 8)}
	}
	// Translated red square centre.
	if got := px(32, 32); got[0] < 200 || got[1] > 90 || got[2] > 90 {
		t.Fatalf("translated square at (32,32) = %v, want red", got)
	}
	// Clip content must be blue inside (40,40)-(60,60)...
	if got := px(50, 50); got[2] < 200 || got[0] > 90 {
		t.Fatalf("clip content at (50,50) = %v, want blue", got)
	}
	// ...and background outside it (inside the fill rect but outside the clip).
	if got := px(20, 50); got[0] > 60 || got[2] > 70 {
		t.Fatalf("outside clip at (20,50) = %v, want background", got)
	}
	// Nothing should have been drawn at the translated clip position (72..92).
	if got := px(10, 10); got[0] > 60 || got[2] > 70 {
		t.Fatalf("stale-transform artifact at (10,10) = %v, want background", got)
	}
}

// TestEncodingAppendResetsStaleTransform pins the append boundary. A child
// encoding whose first draw is identity must not inherit the parent's last
// non-identity transform.
func TestEncodingAppendResetsStaleTransform(t *testing.T) {
	red := render.RGBA{R: 0.90, G: 0.25, B: 0.28, A: 1}
	blue := render.RGBA{R: 0.25, G: 0.45, B: 0.95, A: 1}

	parent := NewScene()
	parent.Fill(FillNonZero, TranslateAffine(32, 32), SolidBrush(red), NewRectShape(-8, -8, 16, 16))
	child := NewScene()
	child.Fill(FillNonZero, IdentityAffine(), SolidBrush(blue), NewRectShape(4, 4, 8, 8))
	parent.Append(child)

	target := render.NewPixmap(64, 64)
	r := NewRenderer(64, 64, WithWorkers(1))
	if err := r.Render(target, parent); err != nil {
		t.Fatalf("Render: %v", err)
	}
	if _, _, b8, _ := target.At(8, 8).RGBA(); b8>>8 < 200 {
		t.Fatalf("appended child at (8,8) missing: %v", target.At(8, 8))
	}
	if _, _, b8, _ := target.At(40, 40).RGBA(); b8>>8 > 90 {
		t.Fatalf("appended child shifted to (40,40): %v", target.At(40, 40))
	}
}

// TestSceneImageDrawResetsStaleTransform pins DrawImage's reset call site.
// TagImage carries its own transform inline, so the decoder CTM must be
// identity when it is reached; otherwise the base matrix shifts the image by
// whatever transform the previous draw left active.
func TestSceneImageDrawResetsStaleTransform(t *testing.T) {
	sc := NewScene()
	sc.Fill(FillNonZero, TranslateAffine(17, 9), SolidBrush(render.RGBA{R: 1, A: 1}),
		NewRectShape(0, 0, 4, 4))
	sc.DrawImage(NewImage(2, 2), IdentityAffine())

	enc := sc.Encoding()
	tags := enc.Tags()
	trs := enc.Transforms()
	ti := 0
	last := IdentityAffine()
	seenImage := false
	for _, tag := range tags {
		switch tag {
		case TagTransform:
			if ti >= len(trs) {
				t.Fatalf("transform stream shorter than tags")
			}
			last = trs[ti]
			ti++
		case TagImage:
			if ti >= len(trs) {
				t.Fatalf("image transform missing")
			}
			ti++ // TagImage stores its own transform inline.
			seenImage = true
			if !last.IsIdentity() {
				t.Fatalf("image inherits stale transform %+v, want identity", last)
			}
		}
	}
	if !seenImage {
		t.Fatalf("encoding has no TagImage")
	}
}

// TestEncodingCommandBoundsSurviveFlattenAppendClone pins per-command bounds
// through the paths DamageTracker depends on.
//
// Regression: SceneBuilder records into the root LayerState encoding, and
// TaggedBounds read s.encoding without flattening — a freshly built scene
// reported zero tracked objects, so RenderWithDamage never saw a moved shape.
// Encoding.Append / AppendWithTranslation / Clone also dropped commandBounds.
func TestEncodingCommandBoundsSurviveFlattenAppendClone(t *testing.T) {
	b := NewSceneBuilder()
	b.FillRect(0, 0, 10, 10, SolidBrush(render.RGBA{R: 1, A: 1}))
	b.Layer(BlendNormal, 0.5, nil, func(lb *SceneBuilder) {
		lb.FillRect(20, 20, 10, 10, SolidBrush(render.RGBA{B: 1, A: 1}))
	})
	sc := b.Build()
	bounds := sc.TaggedBounds()
	if len(bounds) != 2 {
		t.Fatalf("TaggedBounds after Build = %d, want 2", len(bounds))
	}
	if bounds[0].Rect != image.Rect(0, 0, 10, 10) {
		t.Fatalf("command 0 bounds = %v", bounds[0].Rect)
	}
	if bounds[1].Rect != image.Rect(20, 20, 30, 30) {
		t.Fatalf("command 1 bounds = %v", bounds[1].Rect)
	}

	enc := sc.Encoding()
	if got := len(enc.CommandBounds()); got != 2 {
		t.Fatalf("Encoding.CommandBounds = %d, want 2", got)
	}
	if got := len(sc.Flatten().CommandBounds()); got != 2 {
		t.Fatalf("Flatten CommandBounds = %d, want 2", got)
	}
	if got := len(enc.Clone().CommandBounds()); got != 2 {
		t.Fatalf("Clone CommandBounds = %d, want 2", got)
	}

	dst := NewScene()
	dst.AppendWithTranslation(sc, 5, 7)
	appended := dst.TaggedBounds()
	if len(appended) != 2 {
		t.Fatalf("appended TaggedBounds = %d, want 2", len(appended))
	}
	if appended[1].Rect != image.Rect(25, 27, 35, 37) {
		t.Fatalf("appended command 1 bounds = %v, want (25,27)-(35,37)", appended[1].Rect)
	}
}

// TestEncodingHashIncludesBrushes pins the cache key against brush-only edits.
//
// Regression: Encoding.Hash hashed tags/pathData/drawData/textData/transforms
// but not the brushes stream. Two scenes with identical geometry and different
// colours produced the same hash, so LayerCache could return stale pixels.
func TestEncodingHashIncludesBrushes(t *testing.T) {
	build := func(c render.RGBA) uint64 {
		b := NewSceneBuilder()
		b.FillRect(8, 8, 48, 48, SolidBrush(c))
		return b.Build().Encoding().Hash()
	}
	red := build(render.RGBA{R: 1, A: 1})
	blue := build(render.RGBA{B: 1, A: 1})
	if red == blue {
		t.Fatalf("hash collision: brush-only change kept hash %#x", red)
	}
	// Identical input must stay stable.
	if again := build(render.RGBA{R: 1, A: 1}); again != red {
		t.Fatalf("hash not stable: %#x vs %#x", red, again)
	}
}
