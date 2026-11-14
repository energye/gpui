package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// O11 ① 层录制复用：retained 构建路径按 cacheKey 复用干净层的 Recording。
// 断言基准 = 指针恒等（复用的真凭据：同一份不可变显示列表，零重录），
// 佐证 = LayerCache 复用/入库计数。失效门（NeedsPaint / 指纹 / 尺寸 /
// Prune）各有独立用例；信任契约与纹理腿一致（scene/textured.go
// changed := dirtyID || NeedsRaster），指纹只是无标脏变更的自愈安全网。

// collectPictureLayers maps every keyed PictureLayer of a packet to its
// recording pointer so reuse assertions can compare identities across builds.
func collectPictureLayers(t *testing.T, pkt *scene.FramePacket) map[uint64]*recording.Recording {
	t.Helper()
	m := map[uint64]*recording.Recording{}
	scene.Walk(pkt.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok && pl.CacheKey != 0 {
			m[pl.CacheKey] = pl.Recording
		}
	})
	return m
}

// newReuseTree builds root(200×200) + two color-box leaves and flushes layout
// once, mirroring the production attach → FlushLayout sequence.
func newReuseTree(t *testing.T) (*rendering.AbsoluteBox, *rendering.RenderColorBox, *rendering.RenderColorBox, *rendering.PipelineOwner) {
	t.Helper()
	root := rendering.NewAbsoluteBox(200, 200)
	a := rendering.NewRenderColorBox(60, 40, 1, 0, 0, 1)
	b := rendering.NewRenderColorBox(30, 30, 0, 1, 0, 1)
	root.Place(a, 10, 10)
	root.Place(b, 100, 100)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	return root, a, b, owner
}

// TestBuildReuse_PointerIdentitySteadyFrames: after the cold build, every
// clean build reuses the SAME recording pointers (zero re-record).
func TestBuildReuse_PointerIdentitySteadyFrames(t *testing.T) {
	root, _, _, owner := newReuseTree(t)
	lc := rendering.NewLayerCache()

	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, lc)
	m1 := collectPictureLayers(t, p1)
	if len(m1) != 2 {
		t.Fatalf("layers=%d want 2 (two leaves)", len(m1))
	}
	reuse0, store0 := lc.Totals()
	if store0 != 2 {
		t.Fatalf("cold build stores=%d want 2", store0)
	}

	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, lc)
	m2 := collectPictureLayers(t, p2)
	for k, rec := range m1 {
		if m2[k] != rec {
			t.Fatalf("clean build re-recorded layer key=%d", k)
		}
	}
	if lc.ReusedPics != 2 {
		t.Fatalf("reused=%d want 2", lc.ReusedPics)
	}
	reuse1, store1 := lc.Totals()
	if reuse1-reuse0 != 2 || store1 != store0 {
		t.Fatalf("totals after clean build: reuseΔ=%d storeΔ=%d, want 2/0", reuse1-reuse0, store1-store0)
	}

	owner.ConsumeNeedsPaint()
	p3 := rendering.BuildFramePacketWithSaveLayer(root, 3, 1, 200, 200, nil, nil, lc)
	m3 := collectPictureLayers(t, p3)
	for k, rec := range m1 {
		if m3[k] != rec {
			t.Fatalf("third clean build re-recorded layer key=%d", k)
		}
	}
}

// TestBuildReuse_DirtyRerecordThenResume: a leaf whose content actually
// changed re-records exactly once; its untouched sibling is untouched; the
// next clean build resumes reuse.
func TestBuildReuse_DirtyRerecordThenResume(t *testing.T) {
	root, a, _, owner := newReuseTree(t)
	lc := rendering.NewLayerCache()

	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, lc)
	_ = collectPictureLayers(t, p1)
	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, lc)
	m2 := collectPictureLayers(t, p2)

	a.R = 0.5
	a.MarkNeedsPaint()
	p3 := rendering.BuildFramePacketWithSaveLayer(root, 3, 1, 200, 200, nil, nil, lc)
	m3 := collectPictureLayers(t, p3)
	keyA := keyOf(t, m2, a)
	if m3[keyA] == m2[keyA] {
		t.Fatal("changed leaf must re-record (fresh pointer)")
	}
	keyB := otherKey(t, m2, keyA)
	if m3[keyB] != m2[keyB] {
		t.Fatal("untouched sibling must reuse (leaf-level isolation)")
	}

	owner.ConsumeNeedsPaint()
	p4 := rendering.BuildFramePacketWithSaveLayer(root, 4, 1, 200, 200, nil, nil, lc)
	m4 := collectPictureLayers(t, p4)
	if m4[keyA] != m3[keyA] {
		t.Fatal("clean build after dirty frame must resume reuse")
	}
}

// TestBuildReuse_FingerprintDriftInvalidates: content mutated without
// NeedsPaint must not serve a stale recording — the fingerprint net
// re-records and re-stores, so the mutation self-heals within one frame.
func TestBuildReuse_FingerprintDriftInvalidates(t *testing.T) {
	root, a, _, owner := newReuseTree(t)
	lc := rendering.NewLayerCache()

	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, lc)
	_ = collectPictureLayers(t, p1)
	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, lc)
	m2 := collectPictureLayers(t, p2)
	keyA := keyOf(t, m2, a)

	a.R = 0.9 // silent mutation: no MarkNeedsPaint
	p3 := rendering.BuildFramePacketWithSaveLayer(root, 3, 1, 200, 200, nil, nil, lc)
	m3 := collectPictureLayers(t, p3)
	if m3[keyA] == m2[keyA] {
		t.Fatal("fingerprint drift must invalidate the stale recording")
	}

	owner.ConsumeNeedsPaint()
	p4 := rendering.BuildFramePacketWithSaveLayer(root, 4, 1, 200, 200, nil, nil, lc)
	m4 := collectPictureLayers(t, p4)
	if m4[keyA] != m3[keyA] {
		t.Fatal("re-stored fingerprint must stabilize the next build")
	}
}

// TestBuildReuse_DirtyPropagationReusesUnchangedLayers: a child move dirties
// the child AND propagates up (MarkNeedsPaint marks ancestors up to the
// boundary), but neither layer's recording content changed — the parent's
// background layer and the moved child's own layer (a pure function of
// fingerprinted inputs) must both reuse. This pins the bg-only own-layer
// fingerprint AND complete-fingerprint authority over propagated dirty flags.
func TestBuildReuse_DirtyPropagationReusesUnchangedLayers(t *testing.T) {
	root := rendering.NewAbsoluteBox(200, 200)
	root.Background = &rendering.Color{R: 0.2, G: 0.3, B: 0.4, A: 1}
	child := rendering.NewRenderColorBox(30, 30, 1, 0, 0, 1)
	root.Place(child, 20, 20)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 200}, true)
	lc := rendering.NewLayerCache()

	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, lc)
	m1 := collectPictureLayers(t, p1)
	if len(m1) != 2 {
		t.Fatalf("layers=%d want 2 (bg own layer + child leaf)", len(m1))
	}
	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, lc)
	m2 := collectPictureLayers(t, p2)

	child.MoveTo(60, 60) // dirties child + propagates to root
	p3 := rendering.BuildFramePacketWithSaveLayer(root, 3, 1, 200, 200, nil, nil, lc)
	m3 := collectPictureLayers(t, p3)
	keyBg := keyOf(t, m2, root)
	keyChild := keyOf(t, m2, child)
	if m3[keyBg] != m2[keyBg] {
		t.Fatal("child move must not re-record the parent's background layer")
	}
	if m3[keyChild] != m2[keyChild] {
		t.Fatal("moved child's own recording is unchanged (offset is layer state) — must reuse")
	}
}

// TestBuildReuse_NilCacheRecordsFreshEveryBuild: lc == nil keeps the legacy
// always-record behavior (every build produces distinct recordings).
func TestBuildReuse_NilCacheRecordsFreshEveryBuild(t *testing.T) {
	root, _, _, owner := newReuseTree(t)
	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, nil)
	m1 := collectPictureLayers(t, p1)
	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, nil)
	m2 := collectPictureLayers(t, p2)
	for k, rec := range m1 {
		if m2[k] == rec {
			t.Fatalf("nil cache must record fresh every build (key=%d)", k)
		}
	}
}

// TestBuildReuse_PruneDropsUnmounted: a detached node's entry is dropped at
// the next build (alive-set liveness); re-attaching records fresh again
// instead of resurrecting a stale display list.
func TestBuildReuse_PruneDropsUnmounted(t *testing.T) {
	root, a, b, owner := newReuseTree(t)
	lc := rendering.NewLayerCache()

	p1 := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 200, 200, nil, nil, lc)
	m1 := collectPictureLayers(t, p1)
	owner.ConsumeNeedsPaint()
	root.RemoveChild(b)
	owner.ConsumeNeedsPaint()
	p2 := rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 200, 200, nil, nil, lc)
	m2 := collectPictureLayers(t, p2)
	keyA := keyOf(t, m2, a)
	if m2[keyA] != m1[keyOf(t, m1, a)] {
		t.Fatal("mounted sibling must survive the prune")
	}
	if len(m2) != 1 {
		t.Fatalf("detached node must leave the packet (layers=%d want 1)", len(m2))
	}

	owner.ConsumeNeedsPaint()
	root.Place(b, 100, 100)
	owner.ConsumeNeedsPaint()
	p3 := rendering.BuildFramePacketWithSaveLayer(root, 3, 1, 200, 200, nil, nil, lc)
	m3 := collectPictureLayers(t, p3)
	keyB := keyOf(t, m3, b)
	if m3[keyB] == m1[keyOf(t, m1, b)] {
		t.Fatal("pruned entry must not resurrect: re-mounted node re-records")
	}
}

// TestLayerCache_ReuseGates: the store-side gate contract at unit level —
// size mismatch and fingerprint mismatch refuse; a zero fingerprint (types
// without one) skips the fingerprint gate; Totals count only successes.
func TestLayerCache_ReuseGates(t *testing.T) {
	lc := rendering.NewLayerCache()
	rec := recording.NewRecorder(60, 40).FinishRecording()
	const key = 42
	const fp = 7

	lc.StoreRecording(key, fp, rec, 60, 40)
	if got := lc.ReuseRecording(key, fp, 60, 40); got != rec {
		t.Fatal("matching size+fingerprint must reuse")
	}
	if got := lc.ReuseRecording(key, fp, 61, 40); got != nil {
		t.Fatal("size mismatch must refuse")
	}
	if got := lc.ReuseRecording(key, fp+1, 60, 40); got != nil {
		t.Fatal("fingerprint mismatch must refuse")
	}
	if got := lc.ReuseRecording(key, 0, 60, 40); got != rec {
		t.Fatal("zero caller fingerprint skips the gate (NeedsPaint-only types)")
	}
	lc.StoreRecording(key, 0, rec, 60, 40)
	if got := lc.ReuseRecording(key, fp, 60, 40); got != rec {
		t.Fatal("zero stored fingerprint skips the gate")
	}
	if got := lc.ReuseRecording(0, fp, 60, 40); got != nil {
		t.Fatal("zero key must refuse (key-less nodes never cache)")
	}
	reuse, store := lc.Totals()
	if reuse != 3 || store != 2 {
		t.Fatalf("totals reuse=%d store=%d, want 3/2", reuse, store)
	}
}

// keyOf finds the layer key owning node n (tests build identity-free trees,
// so ownership is resolved through the node's cache ID contract via a probe
// build: the layer whose key maps to n's EnsureCacheID).
func keyOf(t *testing.T, m map[uint64]*recording.Recording, n rendering.RenderObject) uint64 {
	t.Helper()
	if b, ok := n.(interface{ EnsureCacheID() uint64 }); ok {
		if k := b.EnsureCacheID(); k != 0 {
			if _, hit := m[k]; hit {
				return k
			}
		}
	}
	t.Fatal("node has no layer key in the collected map")
	return 0
}

func otherKey(t *testing.T, m map[uint64]*recording.Recording, not uint64) uint64 {
	t.Helper()
	for k := range m {
		if k != not {
			return k
		}
	}
	t.Fatal("no other key")
	return 0
}
