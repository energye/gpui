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
		{Kind: scene.MutSetTransform, LayerID: 4, Rotation: 0.1, SX: 1, SY: 1},
	}
	r, c := scene.ClassifyDirty(muts)
	if len(r) != 1 || r[0] != 1 {
		t.Fatalf("raster=%v", r)
	}
	if len(c) != 3 {
		t.Fatalf("compositor=%v", c)
	}
}

func TestTransformLayer_Builder(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	tr := b.PushTransform(5, 6, 0.25, 1.5, 1.5)
	b.AddPicture(true)
	b.Pop()
	if tr.Kind() != "transform" {
		t.Fatalf("kind=%s", tr.Kind())
	}
	sx, sy := tr.EffectiveScale()
	if sx != 1.5 || sy != 1.5 {
		t.Fatalf("scale %v %v", sx, sy)
	}
	var found bool
	scene.Walk(b.Root(), func(l scene.Layer) {
		if l.Kind() == "transform" {
			found = true
		}
	})
	if !found {
		t.Fatal("transform not in tree")
	}
}

func TestColorFilterLayer_Builder(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	cf := b.PushGrayscaleFilter()
	b.AddPicture(true)
	b.Pop()
	if cf.Kind() != "color_filter" {
		t.Fatalf("kind=%s want color_filter", cf.Kind())
	}
	// Grayscale matrix: R row luma coeffs non-zero and equal across RGB rows for R/G/B out.
	if cf.Matrix[0] < 0.2 || cf.Matrix[1] < 0.4 || cf.Matrix[2] < 0.05 {
		t.Fatalf("grayscale matrix unexpected: %+v", cf.Matrix)
	}
	// Not identity (identity has 1 on diagonal R/G/B).
	if cf.Matrix[0] == 1 && cf.Matrix[6] == 1 && cf.Matrix[12] == 1 {
		t.Fatal("grayscale matrix must not be identity")
	}
	var found bool
	var nested bool
	scene.Walk(b.Root(), func(l scene.Layer) {
		if l.Kind() == "color_filter" {
			found = true
			if len(l.Children()) > 0 {
				nested = true
			}
		}
	})
	if !found {
		t.Fatal("color_filter not in tree after PushGrayscaleFilter")
	}
	if !nested {
		t.Fatal("color_filter must nest children (picture under it)")
	}
}

func TestImageFilterLayer_Builder(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	im := b.PushImageFilter(4.5)
	b.AddPicture(false)
	b.Pop()
	if im.Kind() != "image_filter" {
		t.Fatalf("kind=%s want image_filter", im.Kind())
	}
	if im.BlurRadius != 4.5 {
		t.Fatalf("BlurRadius=%v want 4.5", im.BlurRadius)
	}
	var found bool
	scene.Walk(b.Root(), func(l scene.Layer) {
		if f, ok := l.(*scene.ImageFilterLayer); ok {
			found = true
			if f.BlurRadius != 4.5 {
				t.Fatalf("walk BlurRadius=%v", f.BlurRadius)
			}
		}
	})
	if !found {
		t.Fatal("image_filter not in tree")
	}
	// Nested with color filter stack integrity.
	b2 := scene.NewLayerBuilder()
	b2.PushColorFilter([20]float32{1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 1, 0})
	inner := b2.PushImageFilter(2)
	b2.AddPicture(true)
	b2.Pop()
	b2.Pop()
	if inner.Kind() != "image_filter" {
		t.Fatalf("inner kind=%s", inner.Kind())
	}
	n := scene.Walk(b2.Root(), nil)
	if n < 3 {
		t.Fatalf("walk count=%d want >=3", n)
	}
}

func TestClipRRectLayer_Builder(t *testing.T) {
	scene.ResetLayerIDGen()
	b := scene.NewLayerBuilder()
	cr := b.PushClipRRect(2, 4, 100, 80, 12)
	b.AddPicture(true)
	b.Pop()
	if cr.Kind() != "clip_rrect" {
		t.Fatalf("kind=%s want clip_rrect (distinct from clip_rect)", cr.Kind())
	}
	if cr.X != 2 || cr.Y != 4 || cr.W != 100 || cr.H != 80 || cr.Radius != 12 {
		t.Fatalf("fields %+v", cr)
	}
	// Kind must differ from plain clip_rect.
	plain := scene.NewClipRectLayer(0, 0, 10, 10)
	if plain.Kind() == cr.Kind() {
		t.Fatalf("clip_rrect kind must not equal clip_rect (%q)", plain.Kind())
	}
	var foundRRect, foundRect bool
	scene.Walk(b.Root(), func(l scene.Layer) {
		switch l.Kind() {
		case "clip_rrect":
			foundRRect = true
			if rr, ok := l.(*scene.ClipRRectLayer); !ok || rr.Radius != 12 {
				t.Fatalf("walk type/radius: %T %+v", l, l)
			}
		case "clip_rect":
			foundRect = true
		}
	})
	if !foundRRect {
		t.Fatal("clip_rrect not in tree after PushClipRRect")
	}
	if foundRect {
		t.Fatal("unexpected clip_rect in rrect-only tree")
	}
	// Nested push/pop stack integrity.
	b2 := scene.NewLayerBuilder()
	b2.PushClipRect(0, 0, 50, 50)
	inner := b2.PushClipRRect(5, 5, 40, 40, 8)
	b2.AddPicture(false)
	b2.Pop() // rrect
	b2.Pop() // rect
	if inner.Kind() != "clip_rrect" {
		t.Fatalf("inner kind=%s", inner.Kind())
	}
	n := scene.Walk(b2.Root(), nil)
	if n < 3 {
		t.Fatalf("walk count=%d want >=3 (container+rect+rrect+pic)", n)
	}
}

func TestEnsureOverlayBand(t *testing.T) {
	o := scene.EnsureOverlayBand(nil)
	if o == nil || o.Kind() != "container" {
		t.Fatalf("%v", o)
	}
}
