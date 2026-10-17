package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// Worth-caching policy tests (Flutter raster cache alignment):
//   - will_change (self-dirty) content is never cached,
//   - the naive complexity gate is score > 5 draw ops,
//   - a boundary needs accessThreshold+1 unchanged live paints before entry,
//   - at most maxNewPerFrame new caches are admitted per frame,
//   - accessThreshold 0 disables caching entirely.

// complexFills is the child fill count a test boundary needs so that its
// own-content score (1 background + fills) beats the naive complexity gate.
const complexFills = 6

// complexBoundary builds a RepaintBoundary AbsoluteBox whose own content is
// worth caching: one background fill + fills child fills (7 ops for
// complexFills, > 5 — Flutter dl_complexity.h naive ShouldBeCached). The fills
// sit in the bottom strip so pixel probes at the top stay background.
func complexBoundary(w, h float64, fills int) *rendering.AbsoluteBox {
	a := rendering.NewAbsoluteBox(w, h)
	a.Background = &rendering.Color{R: 0.2, G: 0.2, B: 0.25, A: 1}
	a.SetRepaintBoundary(true)
	for i := 0; i < fills; i++ {
		c := rendering.NewRenderColorBox(2, 2, 0.8, 0.4, 0.1, 1)
		a.Place(c, 4+float64(i)*3, h-6)
	}
	return a
}

// newPolicyTree builds a plain (non-boundary) root holding bs left to right.
func newPolicyTree(w, h float64, bs ...rendering.RenderObject) *rendering.AbsoluteBox {
	root := rendering.NewAbsoluteBox(w, h)
	for i, b := range bs {
		root.Place(b, 10+float64(i)*140, 10)
	}
	return root
}

// settle paints frames clean frames — the caller has already painted the frame
// that changed the content, so the content generation can build up its access
// count and reach the cache.
func settle(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache, w, h, frames int) {
	t.Helper()
	for i := 0; i < frames; i++ {
		paintWithCache(t, root, cache, w, h)
	}
}

// warmBoundaryCache paints the dirty first frame plus accessThreshold+1 settle
// frames: the dirty frame is will_change (not cached), and the following
// unchanged frames carry the boundary past the access threshold into the cache.
func warmBoundaryCache(t *testing.T, root rendering.RenderObject, cache *rendering.BoundaryCache, w, h int) {
	t.Helper()
	paintWithCache(t, root, cache, w, h)
	settle(t, root, cache, w, h, cache.AccessThreshold()+1)
}

// TestBoundaryCache_AccessThresholdAdmitsThenReplays pins the Flutter
// raster-cache access rule: content that changed is not cached, and unchanged
// content enters the cache only after access threshold + 1 prepared frames.
func TestBoundaryCache_AccessThresholdAdmitsThenReplays(t *testing.T) {
	b := complexBoundary(120, 60, complexFills) // score 7 > 5
	root := newPolicyTree(200, 200, b)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()
	if !b.NeedsPaint() {
		t.Fatal("fresh boundary must start paint-dirty")
	}

	// Frame 1: dirty live paint → will_change, no entry.
	if rr, _ := paintWithCache(t, root, cache, 200, 200); rr != 0 {
		t.Fatalf("frame1 rerecord=%d want 0 (changing content is never cached)", rr)
	}
	if cache.Len() != 0 {
		t.Fatalf("frame1 entries=%d want 0", cache.Len())
	}
	// Frames 2..4: clean live paints, held back by the access threshold.
	for i := 0; i < 3; i++ {
		rr, sk := paintWithCache(t, root, cache, 200, 200)
		if rr != 0 || sk != 0 {
			t.Fatalf("hold-back frame %d rr=%d sk=%d want 0/0 (no entry yet)", i, rr, sk)
		}
	}
	waits, willChange, complexity, limit, maxScore := cache.PolicyCounts()
	if waits != 3 {
		t.Fatalf("admission_waits=%d want 3 (threshold 3 = 3 held-back frames)", waits)
	}
	if willChange != 1 || complexity != 0 || limit != 0 {
		t.Fatalf("unexpected rejects: will_change=%d (want 1, the dirty frame) complexity=%d limit=%d",
			willChange, complexity, limit)
	}
	if maxScore != complexFills+1 {
		t.Fatalf("max_score=%d want %d", maxScore, complexFills+1)
	}
	if cache.Len() != 0 {
		t.Fatal("nothing may enter the cache before the access threshold is reached")
	}

	// Frame 5: access count 4 > threshold 3 → admitted.
	if rr, _ := paintWithCache(t, root, cache, 200, 200); rr != 1 {
		t.Fatalf("admit frame rerecord=%d want 1", rr)
	}
	if !cache.HasValid(b) {
		t.Fatal("boundary must be cached once the access threshold is passed")
	}
	// Frame 6: clean + valid entry → Replay.
	if rr, sk := paintWithCache(t, root, cache, 200, 200); rr != 0 || sk != 1 {
		t.Fatalf("replay frame rr=%d sk=%d want 0/1", rr, sk)
	}
}

// TestBoundaryCache_ComplexityGateRejectsTrivialContent: content at or below
// the naive complexity gate stays live every frame (never an entry), while a
// complex sibling in the same tree is cached normally.
func TestBoundaryCache_ComplexityGateRejectsTrivialContent(t *testing.T) {
	trivial := rendering.NewRenderColorBox(40, 40, 0.1, 0.8, 0.2, 1) // score 1
	trivial.SetRepaintBoundary(true)
	complex := complexBoundary(120, 60, complexFills) // score 7
	root := newPolicyTree(300, 200, trivial, complex)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 300, Height: 200}, true)
	cache := owner.BoundaryCache()

	for i := 0; i < 8; i++ {
		paintWithCache(t, root, cache, 300, 200)
		if cache.HasValid(trivial) {
			t.Fatalf("frame %d: score-1 content must never be cached", i)
		}
	}
	if !cache.HasValid(complex) {
		t.Fatal("complex boundary must be cached")
	}
	_, _, complexity, _, maxScore := cache.PolicyCounts()
	if complexity < 8 {
		t.Fatalf("complexity_rejects=%d want >=8 (one per live frame of the trivial box)", complexity)
	}
	if maxScore != complexFills+1 {
		t.Fatalf("max_score=%d want %d (trivial score must not raise it)", maxScore, complexFills+1)
	}
}

// TestBoundaryCache_PerFrameNewCacheLimit: at most maxNewPerFrame new caches
// per frame; the rest are deferred to later frames (Flutter
// GenerateNewCacheInThisFrame).
func TestBoundaryCache_PerFrameNewCacheLimit(t *testing.T) {
	b1 := complexBoundary(120, 60, complexFills)
	b2 := complexBoundary(120, 60, complexFills)
	b3 := complexBoundary(120, 60, complexFills)
	root := newPolicyTree(460, 200, b1, b2, b3)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 460, Height: 200}, true)
	cache := owner.BoundaryCache()
	cache.SetMaxNewPerFrame(1)
	if cache.MaxNewPerFrame() != 1 {
		t.Fatalf("MaxNewPerFrame=%d want 1", cache.MaxNewPerFrame())
	}

	// F1 dirty (will_change) + F2..F4 held back: no entries yet.
	for i := 0; i < 4; i++ {
		paintWithCache(t, root, cache, 460, 200)
	}
	if got := cache.Len(); got != 0 {
		t.Fatalf("entries before the limit frame=%d want 0", got)
	}
	// F5: all three are past the access threshold, but only one may be admitted.
	paintWithCache(t, root, cache, 460, 200)
	if got := cache.Len(); got != 1 {
		t.Fatalf("entries after first limit frame=%d want 1", got)
	}
	_, _, _, limit, _ := cache.PolicyCounts()
	if limit != 2 {
		t.Fatalf("limit_rejects=%d want 2 (two boundaries deferred)", limit)
	}
	// F6 and F7 admit the remaining two, one per frame.
	paintWithCache(t, root, cache, 460, 200)
	if got := cache.Len(); got != 2 {
		t.Fatalf("entries after second limit frame=%d want 2", got)
	}
	paintWithCache(t, root, cache, 460, 200)
	if got := cache.Len(); got != 3 {
		t.Fatalf("entries after third limit frame=%d want 3", got)
	}
	// F8: all three replay, no rerecord.
	rr, sk := paintWithCache(t, root, cache, 460, 200)
	if rr != 0 || sk != 3 {
		t.Fatalf("steady frame rr=%d sk=%d want 0/3", rr, sk)
	}
}

// TestBoundaryCache_AccessThresholdZeroDisablesCaching: Flutter's
// access_threshold 0 disables the cache entirely ("If the number is 0, then no
// picture will ever be cached").
func TestBoundaryCache_AccessThresholdZeroDisablesCaching(t *testing.T) {
	b := complexBoundary(120, 60, complexFills)
	root := newPolicyTree(200, 200, b)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	cache := owner.BoundaryCache()
	cache.SetAccessThreshold(0)
	if cache.AccessThreshold() != 0 {
		t.Fatalf("AccessThreshold=%d want 0", cache.AccessThreshold())
	}

	for i := 0; i < 6; i++ {
		rr, sk := paintWithCache(t, root, cache, 200, 200)
		if rr != 0 || sk != 0 {
			t.Fatalf("frame %d rr=%d sk=%d want 0/0 (caching disabled)", i, rr, sk)
		}
	}
	if cache.Len() != 0 || cache.HasValid(b) {
		t.Fatalf("disabled cache must stay empty (len=%d)", cache.Len())
	}
}

// TestBoundaryCache_PictureBoxWillChangeThenAdmit: the picture box path scores
// the built Picture (>5 ops here) and carries the box's own dirty flag as the
// will_change signal — a box repainted every frame never enters the cache, a
// box that stops changing is admitted after the access threshold.
func TestBoundaryCache_PictureBoxWillChangeThenAdmit(t *testing.T) {
	pb := rendering.NewRenderPictureBox(40, 40, func(r *scene.PictureRecorder, ox, oy float64) {
		for i := 0; i < 7; i++ {
			r.FillRect(ox+float64(i*5), oy, 4, 4, 0.3, 0.6, 0.9, 1)
		}
	})
	pb.SetRepaintBoundary(true)
	root := newPolicyTree(120, 120, pb)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 120, Height: 120}, true)
	cache := owner.BoundaryCache()

	for i := 0; i < 3; i++ {
		pb.MarkNeedsPaint()
		paintWithCache(t, root, cache, 120, 120)
		if cache.HasValid(pb) {
			t.Fatalf("dirty frame %d: repainted content must not be cached", i)
		}
	}
	_, willChange, _, _, _ := cache.PolicyCounts()
	if willChange != 3 {
		t.Fatalf("will_change_rejects=%d want 3", willChange)
	}

	// Clean frames: three held back, the fourth admits.
	for i := 0; i < 3; i++ {
		paintWithCache(t, root, cache, 120, 120)
	}
	if cache.HasValid(pb) {
		t.Fatal("picture box must be held back until the access threshold")
	}
	paintWithCache(t, root, cache, 120, 120)
	if !cache.HasValid(pb) {
		t.Fatal("picture box must be cached after four unchanged paints")
	}
	if rr, sk := paintWithCache(t, root, cache, 120, 120); rr != 0 || sk != 1 {
		t.Fatalf("picture box replay rr=%d sk=%d want 0/1", rr, sk)
	}
}
