package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// snapshot captures the handed-off packet content for "mutate tree" comparison.
type packetSnapshot struct {
	ops   int
	kinds []string
	dirty []uint64
	bound [4]int // first picture bounds Dx/Dy
}

func snapshotPacket(t *testing.T, pkt *scene.FramePacket) packetSnapshot {
	t.Helper()
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	s := packetSnapshot{ops: scene.CountPictureOps(pkt)}
	scene.Walk(pkt.Root, func(l scene.Layer) {
		s.kinds = append(s.kinds, l.Kind())
		if pl, ok := l.(*scene.PictureLayer); ok && s.bound == ([4]int{}) {
			s.bound[0] = pl.Picture.Bounds.Dx()
			s.bound[1] = pl.Picture.Bounds.Dy()
		}
	})
	s.dirty = append([]uint64(nil), pkt.DirtyLayerIDs...)
	return s
}

func checkSnapshotUnchanged(t *testing.T, pkt *scene.FramePacket, s packetSnapshot) {
	t.Helper()
	if got := scene.CountPictureOps(pkt); got != s.ops {
		t.Fatalf("handed-off packet ops changed: was %d now %d", s.ops, got)
	}
	var kinds []string
	scene.Walk(pkt.Root, func(l scene.Layer) { kinds = append(kinds, l.Kind()) })
	if len(kinds) != len(s.kinds) {
		t.Fatalf("handed-off packet layers changed: was %d now %d", len(s.kinds), len(kinds))
	}
	for i := range kinds {
		if kinds[i] != s.kinds[i] {
			t.Fatalf("layer %d kind changed: was %s now %s", i, s.kinds[i], kinds[i])
		}
	}
	if len(pkt.DirtyLayerIDs) != len(s.dirty) {
		t.Fatalf("handed-off dirty set changed: was %v now %v", s.dirty, pkt.DirtyLayerIDs)
	}
	for i := range s.dirty {
		if pkt.DirtyLayerIDs[i] != s.dirty[i] {
			t.Fatalf("dirty %d changed: was %v now %v", i, s.dirty, pkt.DirtyLayerIDs)
		}
	}
}

// TestBuildFramePacket_SealedReadOnly: the builder seals at EndFrame (G3),
// tags the UI producer (G1), and stamps the build pair (G10).
func TestBuildFramePacket_SealedReadOnly(t *testing.T) {
	box := rendering.NewRenderColorBox(60, 40, 1, 0, 0, 1)
	box.MarkNeedsPaint()
	pkt := rendering.BuildFramePacket(box, 1, 1, 100, 80)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet")
	}
	if !pkt.IsSealed() {
		t.Fatal("built packet must be sealed (EndFrame)")
	}
	if pkt.Producer != scene.ProducerUI {
		t.Fatalf("producer=%q want %q", pkt.Producer, scene.ProducerUI)
	}
	if pkt.BuildBeginNs == 0 || pkt.BuildEndNs == 0 {
		t.Fatal("build stamps must be set")
	}
	if pkt.BuildEndNs < pkt.BuildBeginNs {
		t.Fatal("build end must not precede begin")
	}
	if pkt.RasterBeginNs != 0 || pkt.RasterEndNs != 0 {
		t.Fatal("raster stamps must stay zero until T2 wires the raster thread")
	}
	if pkt.RegenerateFrom != 0 {
		t.Fatalf("fresh build RegenerateFrom=%d want 0 (G2)", pkt.RegenerateFrom)
	}
}

// TestBuildFramePacket_MutateTreeKeepsHandedOffPacket: changing the RO tree
// after the handoff must not rewrite the already-built packet (T1 core).
// Scope: geometry/color/structure snapshot. RasterExtra live reads (button
// hover etc.) are the known D1 exception fixed by T2 build snapshots.
func TestBuildFramePacket_MutateTreeKeepsHandedOffPacket(t *testing.T) {
	scene.ResetLayerIDGen()
	root := rendering.NewRenderBox()
	root.FixedWidth, root.FixedHeight = 200, 120
	leaf := rendering.NewRenderColorBox(60, 40, 1, 0, 0, 1)
	leaf.SetRepaintBoundary(true)
	root.AddChild(leaf)
	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 120}, true)
	owner.ConsumeNeedsPaint()
	leaf.MarkNeedsPaint()

	pkt := rendering.BuildFramePacket(root, 11, 1, 200, 120)
	before := snapshotPacket(t, pkt)
	if len(before.dirty) == 0 {
		t.Fatal("expected a dirty set to lock")
	}

	// Mutate the live tree after the handoff: move, recolor, resize, add child.
	leaf.MoveTo(30, 25)
	leaf.SetAlpha(0.5)
	leaf.Width, leaf.Height = 80, 50
	extra := rendering.NewRenderColorBox(10, 10, 0, 1, 0, 1)
	root.AddChild(extra)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 120}, true)

	checkSnapshotUnchanged(t, pkt, before)

	// A fresh build after the mutation must differ (new tree, new dirty set).
	scene.ResetLayerIDGen()
	pkt2 := rendering.BuildFramePacket(root, 12, 1, 200, 120)
	if scene.CountPictureOps(pkt2) == 0 {
		t.Fatal("rebuilt packet records nothing")
	}
	if !pkt2.IsSealed() {
		t.Fatal("rebuilt packet must also be sealed")
	}
}

// TestBuildFramePacketWithSaveLayerStats_Sealed: the stats path seals too.
func TestBuildFramePacketWithSaveLayerStats_Sealed(t *testing.T) {
	box := rendering.NewRenderColorBox(20, 10, 0, 1, 0, 1)
	pkt, st := rendering.BuildFramePacketWithSaveLayerStats(box, 3, 1, 100, 80, nil, nil)
	if pkt == nil || !pkt.IsSealed() {
		t.Fatal("stats-path packet must be sealed")
	}
	if st.PictureOpCount != scene.CountPictureOps(pkt) {
		t.Fatalf("op count %d != packet %d", st.PictureOpCount, scene.CountPictureOps(pkt))
	}
}
