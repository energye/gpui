package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// P6 P2 — the scene-level damage prediction must stay a SUPERSET of phase 2's
// damage emission (under-damage leaves stale pixels on the P1 persistent
// surface; over-damage only costs cull coverage). These tests pin the pure
// math (transform mirror, accumulator, clip gate) and the cull decisions
// without a GPU; the end-to-end superset property is continuously verified on
// real windows by WR_P2CHK=1 (icon sweep: zero violations).

func TestTransformRectByMatrix_Translate(t *testing.T) {
	m := render.Translate(30, 20)
	got := transformRectByMatrix(m, image.Rect(10, 10, 50, 40))
	// Min corner (10,10) → (40,30); max (50,40) → (80,60); floor/ceil + 1px
	// outset on every side.
	if got.Min.X != 39 || got.Min.Y != 29 || got.Max.X != 81 || got.Max.Y != 61 {
		t.Fatalf("translate transform: got %v want (39,29)-(81,61)", got)
	}
}

func TestTransformRectByMatrix_RotateCoversQuad(t *testing.T) {
	// A 45° rotation around the rect center: the four-corner AABB of the
	// rotated quad must cover the original footprint (superset property).
	theta := 45 * 3.141592653589793 / 180
	m := render.Translate(50, 50).
		Multiply(render.Rotate(theta)).
		Multiply(render.Translate(-50, -50))
	b := image.Rect(40, 40, 60, 60)
	got := transformRectByMatrix(m, b)
	// Rotated square spans ±10√2 ≈ ±14.14 from center (50,50) →
	// (35.86,35.86)-(64.14,64.14) → ceil (36,36)-(65,65) → outset ±1.
	if got.Min.X > 36 || got.Min.Y > 36 || got.Max.X < 64 || got.Max.Y < 64 {
		t.Fatalf("rotate transform under-covers: got %v", got)
	}
	if got.Min.X < 33 || got.Max.X > 67 || got.Min.Y < 33 || got.Max.Y > 67 {
		t.Fatalf("rotate transform grossly over-covers: got %v", got)
	}
}

func TestDamageAccumulator_UnionAndSeeds(t *testing.T) {
	var acc damageAccumulator
	if !acc.union.Empty() {
		t.Fatal("zero accumulator union must be empty")
	}
	acc.add(image.Rect(10, 10, 20, 20))
	acc.add(image.Rect(30, 30, 40, 40))
	if len(acc.rects) != 2 {
		t.Fatalf("rects must be kept individually for coverage math, got %d", len(acc.rects))
	}
	if acc.union != image.Rect(10, 10, 40, 40) {
		t.Fatalf("union=%v", acc.union)
	}
	// Seeds arrive in physical px and must land in logical coords with
	// conservative outward rounding (floor min / ceil max after /scale).
	acc.addSeed([]image.Rectangle{image.Rect(30, 45, 60, 90)}, 1.5)
	// (20,30)-(40,60) logical, outset by outward rounding only.
	if acc.union.Max.X < 40 || acc.union.Max.Y < 60 || acc.union.Min.X > 20 || acc.union.Min.Y > 30 {
		t.Fatalf("seed must extend the union into logical coords: %v", acc.union)
	}
	acc.markUnbounded()
	if !acc.unbounded {
		t.Fatal("markUnbounded must stick")
	}
}

func TestPlanCompositeClip_Gates(t *testing.T) {
	dc := render.NewContext(200, 100)
	defer dc.Close()

	// Kill switch: GPUI_COMPOSITE_CULL=0 forces the full walk.
	t.Setenv("GPUI_COMPOSITE_CULL", "0")
	acc := &damageAccumulator{}
	acc.add(image.Rect(10, 10, 20, 20))
	if g := planCompositeClip(dc, acc); g.active {
		t.Fatal("kill switch must disable clipping")
	}
	t.Setenv("GPUI_COMPOSITE_CULL", "1")

	// Unbounded effect (backdrop / unknown layer) → full walk.
	accU := &damageAccumulator{unbounded: true}
	accU.add(image.Rect(10, 10, 20, 20))
	if g := planCompositeClip(dc, accU); g.active {
		t.Fatal("unbounded damage must disable clipping")
	}

	// Coverage ≥ DamageFullCoverageThreshold (Σ rect area / surface) → full.
	accFull := &damageAccumulator{}
	accFull.add(image.Rect(0, 0, 200, 100)) // 100% coverage
	if g := planCompositeClip(dc, accFull); g.active {
		t.Fatal("full-coverage damage must disable clipping (present plan will be full anyway)")
	}

	// A plain CPU context is not a persistent surface: the clip gate must
	// stay off (P1 carries the validity contract; wgpu/first-frame/force
	// frames composite full).
	accS := &damageAccumulator{}
	accS.add(image.Rect(10, 10, 20, 20))
	if g := planCompositeClip(dc, accS); g.active {
		t.Fatal("non-persistent surface must disable clipping (no previous pixels outside damage)")
	}
}

func TestCompositeCullable_PictureAndClip(t *testing.T) {
	dc := render.NewContext(200, 100)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	defer tex.Close()
	union := image.Rect(100, 50, 150, 80)

	// A picture whose recording footprint misses the union culls.
	miss := &PictureLayer{Recording: fillRecording(image.Rect(0, 0, 10, 10)), CacheKey: 1}
	if !compositeCullable(miss, dc, union, tex, nil) {
		t.Fatal("picture far from the damage union must cull")
	}
	// A picture overlapping the union stays live.
	hit := &PictureLayer{Recording: fillRecording(image.Rect(120, 60, 140, 70)), CacheKey: 2}
	if compositeCullable(hit, dc, union, tex, nil) {
		t.Fatal("picture overlapping the damage union must stay live")
	}
	// A clip layer whose rect misses the union culls its whole subtree.
	farClip := &ClipRectLayer{X: 0, Y: 0, W: 10, H: 10}
	if !compositeCullable(farClip, dc, union, tex, nil) {
		t.Fatal("clip far from the damage union must cull the subtree")
	}
	// A clip layer covering the union stays live.
	coverClip := &ClipRectLayer{X: 90, Y: 40, W: 70, H: 50}
	if compositeCullable(coverClip, dc, union, tex, nil) {
		t.Fatal("clip covering the damage union must stay live")
	}
	// An empty union culls everything (idle frames skip the whole walk).
	if !compositeCullable(hit, dc, image.Rectangle{}, tex, nil) {
		t.Fatal("empty damage union must cull everything")
	}
	// Containers without bounds never cull (children decide individually).
	if compositeCullable(&ContainerLayer{}, dc, union, tex, nil) {
		t.Fatal("unbounded container must not cull")
	}
}

func TestCompositeCullable_RerecordedShellNeverCulls(t *testing.T) {
	dc := render.NewContext(200, 100)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	defer tex.Close()
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc
	tex.BeginFrame()

	// A layer re-recorded this frame whose recording went EMPTY keeps a
	// viewless shell owing the prev∪current damage emission in phase 2 —
	// the cull check must refuse it or the cleared tail goes stale.
	b := image.Rect(10, 10, 40, 30)
	rec := fillRecording(b)
	if _, ok := tex.recordLocal(31, rec, b); !ok {
		t.Fatal("establishing record must succeed")
	}
	// Headless has no device: fake a live view so the transparent re-record
	// keeps a viewless shell (same fixture as TestAdmission_RefuseEvictsAndDamages).
	tex.mu.Lock()
	if e := tex.entries[31]; e != nil && e.contentSlot >= 0 {
		e.slots[e.contentSlot].view = fakeLiveView()
	}
	tex.mu.Unlock()
	empty := recording.NewRecorder(10, 10).FinishRecording()
	// Transparent re-record drops the texture but keeps the shell + prevBounds.
	if _, ok := tex.recordLocal(31, empty, b); ok {
		t.Fatal("transparent re-record must not allocate a texture")
	}
	if !tex.RecordedThisFrame(31) {
		t.Fatal("shell re-record must count as recorded this frame")
	}
	pl := &PictureLayer{Recording: empty, CacheKey: 31}
	union := image.Rect(100, 50, 150, 80) // shell far from the damage
	if compositeCullable(pl, dc, union, tex, nil) {
		t.Fatal("re-recorded shell must never cull — it owes the prev∪current damage emission")
	}
}

func TestCompositeLayerMatrix_MatchesPushOrder(t *testing.T) {
	// The mirror must compose exactly like pushCompositeCTM's canvas calls:
	// each dc op post-multiplies (the new op applies FIRST to points).
	m := render.Identity()
	m = compositeLayerMatrix(m, &OffsetLayer{DX: 10, DY: 5})
	m = compositeLayerMatrix(m, &TransformLayer{
		CX: 50, CY: 50, Rotation: 0.5, SX: 2, SY: 2,
	})
	got := m.TransformPoint(render.Pt(50, 50))
	// Local (50,50) → pivot translate/rotate/scale → (rotated about 50,50
	// scaled 2 → still (50,50) as it IS the pivot) → offset → (60,55).
	if got.X != 60 || got.Y != 55 {
		t.Fatalf("mirror matrix diverged from pushCompositeCTM semantics: %v", got)
	}
	// A point off the pivot must scale then rotate around (50,50).
	got2 := m.TransformPoint(render.Pt(60, 50))
	// Local (60,50): pivot-relative (10,0) → scale 2 → (20,0) → rotate 0.5
	// → (20cos.5, 20sin.5) ≈ (17.55, 9.59) → back to (67.55, 59.59) →
	// offset → (77.55, 64.59).
	wantX := 60 + 20*0.8775825618903728
	wantY := 55 + 20*0.4794255386042031
	if got2.X < wantX-1e-9 || got2.X > wantX+1e-9 || got2.Y < wantY-1e-9 || got2.Y > wantY+1e-9 {
		t.Fatalf("pivot rotate/scale mirror: got (%g,%g) want (%g,%g)", got2.X, got2.Y, wantX, wantY)
	}
}
