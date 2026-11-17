//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package scene

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/render"
)

// P6 P2 — scene-level damage prediction and culling for the retained textured
// composite (ENGINE_PHASE_P6_DAMAGE_COMPOSITE.md §4.1B / §4.2). Phase 1
// predicts this frame's damage union while it makes the record/blit/refuse
// decisions; phase 2 clips the composite to that union and skips subtrees
// whose painted footprint provably misses it (Flutter compositor_context.cc
// Preroll(clip_rect) + canvas.ClipRect parity). Every prediction rule mirrors
// a phase-2 damage emission site one-to-one so the prediction stays a
// SUPERSET: under-damage would leave stale pixels on the persistent surface
// (P1 LoadOpLoad), over-damage only costs cull coverage.
//
// Culling is gated on render.Context.CompositeClipSafe (P6 P1 persistent
// surface + validity) — pixels outside the damage only have defined previous
// content when the back buffer carries the previous complete frame.

// compositeCullEnabled gates P6 P2. Default ON (pixel-gated: hover sweep
// P2-on vs P2-off surface diff 0px, icon golden 0.0000%); kill switch
// GPUI_COMPOSITE_CULL=0 restores the P1-only full-walk composite.
func compositeCullEnabled() bool {
	return os.Getenv("GPUI_COMPOSITE_CULL") != "0"
}

// damageAccumulator collects the phase-1 predicted damage in logical surface
// coordinates. union is the clip candidate; unbounded marks a frame whose
// damage cannot be proven bounded (backdrop, projection, unknown-bound
// effects) — the composite then runs the full unclipped walk.
type damageAccumulator struct {
	rects     []image.Rectangle
	union     image.Rectangle
	unbounded bool
}

func (a *damageAccumulator) add(r image.Rectangle) {
	if a == nil || r.Empty() {
		return
	}
	a.rects = append(a.rects, r)
	if a.union.Empty() {
		a.union = r
	} else {
		a.union = a.union.Union(r)
	}
}

// addSeed folds pre-composite damage into the logical union. Seeds arrive in
// physical pixels (trackDamage scales logical→physical on store): the full
// rects of a MarkFullRedraw force frame, the gone/evict lastBlit sweeps from
// PictureTextureCache.BeginFrame, and any direct draws the embedder made
// before the composite.
func (a *damageAccumulator) addSeed(physical []image.Rectangle, scale float64) {
	if a == nil || len(physical) == 0 {
		return
	}
	if scale <= 0 {
		scale = 1
	}
	for _, r := range physical {
		a.add(image.Rect(
			int(math.Floor(float64(r.Min.X)/scale)),
			int(math.Floor(float64(r.Min.Y)/scale)),
			int(math.Ceil(float64(r.Max.X)/scale)),
			int(math.Ceil(float64(r.Max.Y)/scale)),
		))
	}
}

func (a *damageAccumulator) markUnbounded() {
	if a != nil {
		a.unbounded = true
	}
}

// BlitFootprint returns an entry's previous blit footprint (the logical
// surface rect of the texture quad as last drawn). False when the entry has
// never blitted — nothing of it lives on the surface yet, so a first blit
// owes no move damage.
func (c *PictureTextureCache) BlitFootprint(id uint64) (image.Rectangle, bool) {
	if c == nil {
		return image.Rectangle{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	if e == nil || !e.hasBlit || e.lastBlit.Empty() {
		return image.Rectangle{}, false
	}
	return e.lastBlit, true
}

// BlitQuad returns the quad blit() will draw for this entry THIS frame: the
// recorded slot extents at the entry offset. Full-surface records (record())
// sit at the origin with window extents; bounds-sized records (recordLocal)
// sit at bounds.Min with the +2 AA pad baked into the slot dims. This — not
// the recording bounds — is the painted footprint damage must cover.
func (c *PictureTextureCache) BlitQuad(id uint64) (image.Rectangle, bool) {
	if c == nil {
		return image.Rectangle{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[id]
	if e == nil || e.contentSlot < 0 || e.contentSlot >= len(e.slots) {
		return image.Rectangle{}, false
	}
	s := &e.slots[e.contentSlot]
	if s.w <= 0 || s.h <= 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(e.off.X, e.off.Y, e.off.X+s.w, e.off.Y+s.h), true
}

// SnappedBlitQuad mirrors blit()'s device-pixel snap for a translate-only
// composite CTM: returns the EXACT logical quad blit will draw this frame,
// so phase-1 move-damage equality against BlitFootprint is stable instead of
// rounding-sensitive (an unsnapped quad differs from the stored one by ≤1px
// on nearly every edge and would report motion on every frozen frame). Only
// valid under a pure-translate chain (rotDepth==0, where the record CTM the
// caller's dc carries equals the composite CTM); rotating callers use the
// always-damage rule instead.
func (c *PictureTextureCache) SnappedBlitQuad(dc *render.Context, id uint64) (image.Rectangle, bool) {
	if c == nil || dc == nil {
		return image.Rectangle{}, false
	}
	c.mu.Lock()
	e := c.entries[id]
	if e == nil || e.contentSlot < 0 || e.contentSlot >= len(e.slots) {
		c.mu.Unlock()
		return image.Rectangle{}, false
	}
	off := e.off
	w, h := e.slots[e.contentSlot].w, e.slots[e.contentSlot].h
	c.mu.Unlock()
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, false
	}
	ox, oy := float64(off.X), float64(off.Y)
	dx, dy := dc.TransformPoint(ox, oy)
	xx, _ := dc.TransformPoint(ox+1, oy)
	_, yy := dc.TransformPoint(ox, oy+1)
	kx, ky := xx-dx, yy-dy
	const eps = 1e-9
	if math.Abs(kx) > eps && math.Abs(kx-ky) < eps*math.Max(1, math.Abs(kx)) {
		// translate-only uniform scale: snap the origin to whole device px
		// (identical math to blit's device-pixel snap).
		ox += (math.Round(dx) - dx) / kx
		oy += (math.Round(dy) - dy) / ky
	}
	q := image.Rect(int(math.Floor(ox)), int(math.Floor(oy)),
		int(math.Floor(ox))+w, int(math.Floor(oy))+h)
	return transformBounds(dc, q), true
}

// expandRect outset a rect by n on every side (AA fringe slack).
func expandRect(b image.Rectangle, n int) image.Rectangle {
	return image.Rect(b.Min.X-n, b.Min.Y-n, b.Max.X+n, b.Max.Y+n)
}

// transformRectByMatrix maps a layer-local rect through a logical composite
// transform (four corners, conservative outward rounding +1px) — the matrix
// twin of transformBounds for phase-1 prediction, where the dc carries the
// record CTM instead of the composite CTM. The +1 outset absorbs the
// rounding slack against blit's device-snapped quad: the snap pulls the far
// edge in by up to 1 logical px and the float-associativity difference can
// push it out again — an under-covered edge would leave a stale line under
// LoadOpLoad, while over-coverage only costs cull precision.
func transformRectByMatrix(m render.Matrix, b image.Rectangle) image.Rectangle {
	if b.Empty() {
		return b
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range [4]render.Point{
		{X: float64(b.Min.X), Y: float64(b.Min.Y)},
		{X: float64(b.Max.X), Y: float64(b.Min.Y)},
		{X: float64(b.Max.X), Y: float64(b.Max.Y)},
		{X: float64(b.Min.X), Y: float64(b.Max.Y)},
	} {
		p := m.TransformPoint(c)
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	return expandRect(image.Rect(int(math.Floor(minX)), int(math.Floor(minY)),
		int(math.Ceil(maxX)), int(math.Ceil(maxY))), 1)
}

// compositeLayerMatrix mirrors pushCompositeCTM's canvas-state effect as a
// pure 2D affine: Offset/Boundary translate, clip layers are matrix-neutral,
// TransformLayer applies translate + pivot rotate/scale exactly like the
// dc call sequence (canvas post-multiply: render.Matrix.Multiply(other)
// applies other first). Phase 1 pushes the record CTM on the dc, so the
// phase-2 damage quads under rotation/scale are predicted through this.
func compositeLayerMatrix(compM render.Matrix, l Layer) render.Matrix {
	switch t := l.(type) {
	case *OffsetLayer:
		return compM.Multiply(render.Translate(t.DX, t.DY))
	case *BoundaryLayer:
		return compM.Multiply(render.Translate(t.DX, t.DY))
	case *TransformLayer:
		sx, sy := t.EffectiveScale()
		m := compM
		if t.TX != 0 || t.TY != 0 {
			m = m.Multiply(render.Translate(t.TX, t.TY))
		}
		if t.Rotation != 0 || sx != 1 || sy != 1 {
			m = m.Multiply(render.Translate(t.CX, t.CY)).
				Multiply(render.Rotate(t.Rotation)).
				Multiply(render.Scale(sx, sy)).
				Multiply(render.Translate(-t.CX, -t.CY))
		}
		return m
	}
	return compM
}

// compositeClipGate is the phase-2 clipping decision.
type compositeClipGate struct {
	active bool
	union  image.Rectangle
}

// planCompositeClip applies the P2 clip gate. Culling activates only when
// every condition holds:
//   - kill switch off (GPUI_COMPOSITE_CULL=0 forces the full walk),
//   - no unbounded effect this frame (backdrop / projection / unknown bounds),
//   - the back buffer carries the previous complete frame
//     (render.Context.CompositeClipSafe — P1 persistent surface + validity),
//   - predicted damage covers less than DamageFullCoverageThreshold (0.85) of
//     the surface, with the same Σ-rect-area口径 as PlanFramePresent —
//     frames the present plan will promote to full anyway must not clip.
func planCompositeClip(dc *render.Context, acc *damageAccumulator) compositeClipGate {
	if acc == nil || acc.unbounded || !compositeCullEnabled() {
		return compositeClipGate{}
	}
	if !dc.CompositeClipSafe() {
		return compositeClipGate{}
	}
	// Projection layers bend coordinates outside the 2D affine chain —
	// four-corner math no longer bounds a footprint; composite full.
	if !dc.Projection().IsIdentity() {
		return compositeClipGate{}
	}
	w, h := dc.Width(), dc.Height()
	if w <= 0 || h <= 0 {
		return compositeClipGate{}
	}
	surface := int64(w) * int64(h)
	var sum int64
	for _, r := range acc.rects {
		r = r.Intersect(image.Rect(0, 0, w, h))
		if !r.Empty() {
			sum += int64(r.Dx()) * int64(r.Dy())
		}
	}
	if surface > 0 && float64(sum) >= render.DamageFullCoverageThreshold*float64(surface) {
		return compositeClipGate{}
	}
	return compositeClipGate{active: true, union: acc.union}
}

// compositeCullable reports whether phase 2 may skip the layer entirely: its
// painted footprint provably misses the damage union, so the persistent
// surface already holds the correct pixels there. A culled container skips
// its whole subtree. Every "never" branch below exists because the layer
// still owes a phase-2 damage emission or paints outside its bounds.
func compositeCullable(l Layer, dc *render.Context, union image.Rectangle, tex *PictureTextureCache, dirty map[uint64]struct{}) bool {
	if union.Empty() {
		// Nothing changes this frame: skip everything drawable. (The
		// composite output is discarded by an idle present plan anyway.)
		return true
	}
	switch t := l.(type) {
	case *PictureLayer:
		// A re-recorded shell (empty recording, viewless entry) still owes
		// the prev∪current damage emission in compositeWalk — never skip it
		// or the cleared tail goes stale. Non-empty re-records are inside
		// the union and fail the intersect test on their own.
		if t.CacheKey != 0 && tex != nil && tex.RecordedThisFrame(t.CacheKey) {
			return false
		}
		b := t.Recording.Bounds()
		if b.Empty() {
			return true
		}
		return !transformBounds(dc, b).Overlaps(union)
	case *TextureLayer:
		// The prediction damages every texture layer's rect, so a live one
		// always intersects; this only fires for never-painted sources.
		r := textureLayerDamage(dc, t)
		return r.Empty() || !r.Overlaps(union)
	case *ClipRectLayer:
		if t.W <= 0 || t.H <= 0 {
			return false
		}
		clip := image.Rect(int(t.X), int(t.Y), int(t.X+t.W), int(t.Y+t.H))
		// Current-frame child bounds sit inside the clip, so any child
		// damage intersects clip∩union; an empty intersection means the
		// whole subtree provably contributes nothing inside the damage.
		return !transformBounds(dc, clip).Overlaps(union)
	case *ClipRRectLayer:
		if t.W <= 0 || t.H <= 0 {
			return false
		}
		clip := image.Rect(int(t.X), int(t.Y), int(t.X+t.W), int(t.Y+t.H))
		// Rect bounds are a superset of the rounded clip region (outward
		// conservative — a rounded corner never extends past its bbox).
		return !transformBounds(dc, clip).Overlaps(union)
	case *ColorFilterLayer, *ImageFilterLayer:
		return compositeFilterCullable(l, dc, union, tex, dirty)
	}
	return false
}

// compositeFilterCullable skips a filter layer whose cached result is clean
// AND whose output quad misses the damage union — the surface already holds
// those pixels. A cache miss means the filter re-renders this frame and its
// damage was predicted into the union, so it never culls here.
func compositeFilterCullable(l Layer, dc *render.Context, union image.Rectangle, tex *PictureTextureCache, dirty map[uint64]struct{}) bool {
	fc := tex.filterCache
	if fc == nil || l.LayerID() == 0 {
		return false
	}
	var key uint64
	switch t := l.(type) {
	case *ColorFilterLayer:
		if t.CacheKey == 0 || subtreeHasTransform(l) {
			return false
		}
		key = t.CacheKey
	case *ImageFilterLayer:
		if t.CacheKey == 0 || subtreeHasTransform(l) {
			return false
		}
		key = t.CacheKey
	default:
		return false
	}
	fp := filterFingerprint(l, dirty, fc.seed)
	if fc.get(key, fp) == nil {
		return false // miss → re-render → predicted damage → keep
	}
	b, okB := filterSubtreeBounds(l)
	if !okB {
		return false
	}
	const pad = 16 // same outset the filtered buffer is drawn with
	out := image.Rect(b.Min.X-pad, b.Min.Y-pad, b.Max.X+pad, b.Max.Y+pad)
	return !transformBounds(dc, out).Overlaps(union)
}

// filterSubtreeDirty reports whether any descendant layer id is in this
// frame's dirty set — the raw predicate behind filterFingerprint's dirty
// byte, needed when the fingerprint itself is not enough (unsupported filter
// paths whose damage must be predicted conservatively).
func filterSubtreeDirty(l Layer, dirty map[uint64]struct{}) bool {
	found := false
	walkFilterSubtree(l, func(id uint64, _ Layer) {
		if _, d := dirty[id]; d {
			found = true
		}
	})
	return found
}

// checkP2CHK verifies the superset property after phase 2: every damage rect
// the composite emitted (st.DamageRects plus blit()'s direct TrackDamageRect
// calls, diffed from dc.FrameDamage) must sit inside the predicted union.
// WR_P2CHK=1 enables the trace; violations mean a prediction rule drifted
// from its phase-2 counterpart — a stale-pixel risk on the persistent
// surface, not a cosmetic offset.
func checkP2CHK(dc *render.Context, gate compositeClipGate, pre []image.Rectangle, st *TexturedStats, acc *damageAccumulator) {
	if !gate.active || os.Getenv("WR_P2CHK") != "1" {
		return
	}
	viol := 0
	report := func(r image.Rectangle, src string) {
		if r.Empty() || r.In(gate.union) {
			return
		}
		viol++
		if viol <= 6 {
			fmt.Fprintf(os.Stderr, "P2CHK viol %s r=%v union=%v\n", src, r, gate.union)
		}
	}
	post := dc.FrameDamage()
	if len(post) > len(pre) {
		for _, r := range post[len(pre):] {
			report(r, "blit-dc")
		}
	}
	for _, r := range st.DamageRects {
		report(r, "st")
	}
	if viol > 0 {
		fmt.Fprintf(os.Stderr, "P2CHK violations=%d (phase-2 damage outside predicted union — prediction rule drift)\n", viol)
		for i, r := range acc.rects {
			fmt.Fprintf(os.Stderr, "P2CHK acc[%d]=%v\n", i, r)
		}
	}
}

// predictFilterDamage mirrors phase 2's filter branches (compositeWalk): a
// cache hit draws the unchanged filtered result (no damage); a miss
// re-renders — damage the output quad (subtree bounds + the pad the buffer
// is drawn with) when the subtree bounds are provable, else mark the frame
// unbounded. Unsupported filters (no cache key, transform subtree) take the
// isolated compositeLayer path: damage the subtree extent when provable AND
// the subtree actually changed, else unbounded.
func predictFilterDamage(l Layer, acc *damageAccumulator, tex *PictureTextureCache, dirty map[uint64]struct{}, compM render.Matrix) {
	fc := tex.filterCache
	if fc == nil || l.LayerID() == 0 {
		if filterSubtreeDirty(l, dirty) {
			if b, okB := filterSubtreeBounds(l); okB {
				acc.add(transformRectByMatrix(compM, expandRect(b, 2)))
			} else {
				acc.markUnbounded()
			}
		}
		return
	}
	var key uint64
	supported := false
	switch t := l.(type) {
	case *ColorFilterLayer:
		if t.CacheKey != 0 && !subtreeHasTransform(l) {
			key, supported = t.CacheKey, true
		}
	case *ImageFilterLayer:
		if t.CacheKey != 0 && !subtreeHasTransform(l) {
			key, supported = t.CacheKey, true
		}
	}
	if supported {
		fp := filterFingerprint(l, dirty, fc.seed)
		if fc.get(key, fp) != nil {
			return // hit: the unchanged filtered result is already on the surface
		}
		if b, okB := filterSubtreeBounds(l); okB {
			const pad = 16 // same outset the filtered buffer is drawn with
			acc.add(transformRectByMatrix(compM, expandRect(b, pad)))
			return
		}
		acc.markUnbounded() // transform inside: bounds unprovable
		return
	}
	// Unsupported (CacheKey==0 / transform subtree): isolated redraw —
	// damage only when the subtree actually changed.
	if filterSubtreeDirty(l, dirty) {
		if b, okB := filterSubtreeBounds(l); okB {
			acc.add(transformRectByMatrix(compM, expandRect(b, 2)))
		} else {
			acc.markUnbounded()
		}
	}
}
