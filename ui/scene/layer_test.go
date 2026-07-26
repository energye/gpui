package scene_test

import (
	"testing"

	"github.com/energye/gpui/ui/scene"
)

func TestLayer_WalkAndIDs(t *testing.T) {
	scene.ResetLayerIDGen()
	root := scene.NewContainerLayer()
	off := scene.NewOffsetLayer(10, 20)
	pic := scene.NewPictureLayer()
	off.Add(pic)
	root.Add(off)

	n := scene.Walk(root, nil)
	if n != 3 {
		t.Fatalf("walk count=%d", n)
	}
	ids := scene.CollectLayerIDs(root)
	if len(ids) != 3 {
		t.Fatalf("ids=%v", ids)
	}
}

func TestFramePacket_CloneShallowSharesRoot(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	b.PushBoundary(0, 0, "spin", true)
	b.AddPicture(true)
	b.Pop()
	p1 := b.BuildPacket(1, 1, 100, 100)
	p2 := p1.CloneShallow()
	p2.FrameID = 2
	if !scene.ShareRoot(p1, p2) {
		t.Fatal("CloneShallow must share Root pointer (no deep copy)")
	}
	if p1.FrameID == p2.FrameID {
		t.Fatal("frame ids should differ after edit")
	}
	// Mutating dirty slice on p2 must not clear p1's underlying if we copied slice header with same array —
	// CloneShallow appends a new slice copy.
	p2.DirtyLayerIDs[0] = 999
	if p1.DirtyLayerIDs[0] == 999 {
		t.Fatal("dirty slice should be copied")
	}
}

func TestClassifyDirty(t *testing.T) {
	muts := []scene.Mutation{
		{Kind: scene.MutReplacePicture, LayerID: 1},
		{Kind: scene.MutSetOpacity, LayerID: 2, Opacity: 0.5},
		{Kind: scene.MutSetOffset, LayerID: 3, DX: 1},
	}
	r, c := scene.ClassifyDirty(muts)
	if len(r) != 1 || r[0] != 1 {
		t.Fatalf("raster=%v", r)
	}
	if len(c) != 2 {
		t.Fatalf("compositor=%v", c)
	}
}

func TestEnsureOverlayBand(t *testing.T) {
	o := scene.EnsureOverlayBand(nil)
	if o == nil || o.Kind() != "container" {
		t.Fatalf("%v", o)
	}
}
