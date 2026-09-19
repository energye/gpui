package skeleton

import (
	"image"
	"math"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// R2-6 snapshot quartet (button snapshot paradigm) for the whole skeleton
// family: paint follows the frozen snapshot, never live widget fields.

func skPaintToImage(t *testing.T, node rendering.RenderObject, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	node.Paint(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

func skImagesEqual(a, b image.Image) bool {
	if a == nil || b == nil || !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

func newSkeletonSnapped(t *testing.T) *Skeleton {
	t.Helper()
	s := NewSkeleton()
	s.Layout(rendering.Loose(400, 120))
	return s
}

func newSkeletonAvatarSnapped(t *testing.T) *SkeletonAvatar {
	t.Helper()
	a := NewSkeletonAvatar()
	a.Layout(rendering.Loose(48, 48))
	return a
}

func newSkeletonButtonSnapped(t *testing.T) *SkeletonButton {
	t.Helper()
	b := NewSkeletonButton()
	b.Layout(rendering.Loose(240, 120))
	return b
}

func newSkeletonInputSnapped(t *testing.T) *SkeletonInput {
	t.Helper()
	in := NewSkeletonInput()
	in.Layout(rendering.Loose(400, 120))
	return in
}

func newSkeletonImageSnapped(t *testing.T) *SkeletonImage {
	t.Helper()
	im := NewSkeletonImage()
	im.Layout(rendering.Loose(96, 96))
	return im
}

func newSkeletonNodeSnapped(t *testing.T) *SkeletonNode {
	t.Helper()
	n := NewSkeletonNode(nil)
	n.Layout(rendering.Loose(96, 96))
	return n
}

// Skeleton family — live pokes without refresh must not change dispatched
// paint; refresh picks the change up.

func TestSkeletonFamily_SnapshotIgnoresLivePoke(t *testing.T) {
	s := newSkeletonSnapped(t)
	before := skPaintToImage(t, s.Node(), 400, 120)
	s.hasAvatar = true
	s.hasTitle = false
	s.paragraphRows = 4
	s.round = true
	s.avatarSizePx = 30
	s.reduceMotion = true
	again := skPaintToImage(t, s.Node(), 400, 120)
	if !skImagesEqual(before, again) {
		t.Fatal("Skeleton dispatch paint changed after live poke without refresh")
	}
	s.refreshSnapshot()
	after := skPaintToImage(t, s.Node(), 400, 120)
	if skImagesEqual(before, after) {
		t.Fatal("Skeleton refresh did not pick up state change")
	}

	a := newSkeletonAvatarSnapped(t)
	a.SetActive(true)
	aBase := skPaintToImage(t, a.Node(), 48, 48)
	a.sizePx = 99
	a.shape = AvatarSquare
	a.reduceMotion = true
	aAgain := skPaintToImage(t, a.Node(), 48, 48)
	if !skImagesEqual(aBase, aAgain) {
		t.Fatal("Avatar dispatch paint changed after live poke without refresh")
	}
	a.refreshSnapshot()
	aAfter := skPaintToImage(t, a.Node(), 48, 48)
	if skImagesEqual(aBase, aAfter) {
		t.Fatal("Avatar refresh did not pick up state change")
	}

	b := newSkeletonButtonSnapped(t)
	b.SetActive(true)
	bBase := skPaintToImage(t, b.Node(), 240, 120)
	b.shape = ButtonCircle
	b.reduceMotion = true
	bAgain := skPaintToImage(t, b.Node(), 240, 120)
	if !skImagesEqual(bBase, bAgain) {
		t.Fatal("Button dispatch paint changed after live poke without refresh")
	}
	b.refreshSnapshot()
	bAfter := skPaintToImage(t, b.Node(), 240, 120)
	if skImagesEqual(bBase, bAfter) {
		t.Fatal("Button refresh did not pick up state change")
	}

	in := newSkeletonInputSnapped(t)
	in.SetActive(true)
	in.phase.Store(math.Float64bits(0.5))
	inBase := skPaintToImage(t, in.Node(), 400, 120)
	in.reduceMotion = true
	inAgain := skPaintToImage(t, in.Node(), 400, 120)
	if !skImagesEqual(inBase, inAgain) {
		t.Fatal("Input dispatch paint changed after live poke without refresh")
	}
	in.refreshSnapshot()
	inAfter := skPaintToImage(t, in.Node(), 400, 120)
	if skImagesEqual(inBase, inAfter) {
		t.Fatal("Input refresh did not pick up state change")
	}

	im := newSkeletonImageSnapped(t)
	im.SetActive(true)
	im.phase.Store(math.Float64bits(0.5))
	imBase := skPaintToImage(t, im.Node(), 96, 96)
	im.reduceMotion = true
	imAgain := skPaintToImage(t, im.Node(), 96, 96)
	if !skImagesEqual(imBase, imAgain) {
		t.Fatal("Image dispatch paint changed after live poke without refresh")
	}
	im.refreshSnapshot()
	imAfter := skPaintToImage(t, im.Node(), 96, 96)
	if skImagesEqual(imBase, imAfter) {
		t.Fatal("Image refresh did not pick up state change")
	}

	n := newSkeletonNodeSnapped(t)
	n.SetActive(true)
	nBase := skPaintToImage(t, n.Node(), 96, 96)
	n.child = rendering.NewRenderBox()
	n.reduceMotion = true
	nAgain := skPaintToImage(t, n.Node(), 96, 96)
	if !skImagesEqual(nBase, nAgain) {
		t.Fatal("Node dispatch paint changed after live poke without refresh")
	}
	n.refreshSnapshot()
	nAfter := skPaintToImage(t, n.Node(), 96, 96)
	if skImagesEqual(nBase, nAfter) {
		t.Fatal("Node refresh did not pick up state change")
	}
}

// TestSkeletonFamily_FrozenSnapshotPaintsDeterministic: identical snapshots
// paint identical pixels even while live widgets keep mutating.
func TestSkeletonFamily_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	s := newSkeletonSnapped(t)
	first := skPaintToImage(t, s.Node(), 400, 120)
	s.hasAvatar = true
	s.hasTitle = false
	s.paragraphRows = 4
	s.round = true
	s.avatarSizePx = 30
	s.titleWidth = 0.5
	s.paragraphWidths = []float64{0.3}
	second := skPaintToImage(t, s.Node(), 400, 120)
	if !skImagesEqual(first, second) {
		t.Fatal("Skeleton same snapshot painted different pixels after live mutations")
	}

	b := newSkeletonButtonSnapped(t)
	bBase := skPaintToImage(t, b.Node(), 240, 120)
	b.shape = ButtonCircle
	b.block = true
	bSecond := skPaintToImage(t, b.Node(), 240, 120)
	if !skImagesEqual(bBase, bSecond) {
		t.Fatal("Button same snapshot painted different pixels after live mutations")
	}

	n := newSkeletonNodeSnapped(t)
	nBase := skPaintToImage(t, n.Node(), 96, 96)
	n.child = rendering.NewRenderBox()
	nSecond := skPaintToImage(t, n.Node(), 96, 96)
	if !skImagesEqual(nBase, nSecond) {
		t.Fatal("Node same snapshot painted different pixels after live mutations")
	}
}

// TestSkeletonFamily_SnapshotConcurrentRace: single UI writer (setters,
// atomics + snapshot refresh) races against snapshot loads; the raster paint
// path is exercised serially afterward. Run with -race.
func TestSkeletonFamily_SnapshotConcurrentRace(t *testing.T) {
	s := NewSkeleton()
	s.Layout(rendering.Loose(400, 120))
	a := NewSkeletonAvatar()
	a.Layout(rendering.Loose(48, 48))
	b := NewSkeletonButton()
	b.Layout(rendering.Loose(240, 120))
	in := NewSkeletonInput()
	in.Layout(rendering.Loose(400, 120))
	im := NewSkeletonImage()
	im.Layout(rendering.Loose(96, 96))
	n := NewSkeletonNode(nil)
	n.Layout(rendering.Loose(96, 96))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		for j := 0; j < 200; j++ {
			s.SetActive(j%2 == 0)
			s.SetAvatar(j%2 == 0)
			s.SetReduceMotion(j%2 == 0)
			_ = s.loadSnapshot()
			a.SetActive(j%2 == 0)
			a.SetSizePx(float64(j%40))
			a.SetShape(AvatarCircle)
			_ = a.loadSnapshot()
			b.SetActive(j%2 == 0)
			b.SetShape(ButtonDefault)
			_ = b.loadSnapshot()
			in.SetActive(j%2 == 0)
			in.SetSize(SizeMiddle)
			_ = in.loadSnapshot()
			im.SetActive(j%2 == 0)
			_ = im.loadSnapshot()
			n.SetActive(j%2 == 0)
			_ = n.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshots — exercises the raster paint path.
	skPaintToImage(t, s.Node(), 400, 120)
	skPaintToImage(t, a.Node(), 48, 48)
	skPaintToImage(t, b.Node(), 240, 120)
	skPaintToImage(t, in.Node(), 400, 120)
	skPaintToImage(t, im.Node(), 96, 96)
	skPaintToImage(t, n.Node(), 96, 96)
}

// TestSkeletonFamily_SnapshotPurity locks the no-live-read contract: raw
// field pokes without refresh must not change the next dispatched paint.
func TestSkeletonFamily_SnapshotPurity(t *testing.T) {
	s := newSkeletonSnapped(t)
	before := skPaintToImage(t, s.Node(), 400, 120)
	s.hasAvatar = true
	s.hasTitle = false
	s.paragraphRows = 4
	s.round = true
	s.avatarSizePx = 30
	s.titleWidth = 0.5
	s.titleWidthStr = "50%"
	s.paragraphWidths = []float64{0.3}
	s.paragraphWidthStrs = []string{"61%"}
	s.reduceMotion = true
	after := skPaintToImage(t, s.Node(), 400, 120)
	if !skImagesEqual(before, after) {
		t.Fatal("Skeleton paint changed after live poke without refresh")
	}
	snap := s.loadSnapshot()
	if snap.HasAvatar || !snap.HasTitle || snap.Rows != 3 || snap.Round {
		t.Fatalf("Skeleton snapshot lost frozen structure flags: %+v", snap)
	}
	if len(snap.ParagraphWidths) != 0 || len(snap.ParagraphWidthStrs) != 0 {
		t.Fatalf("Skeleton snapshot picked up live width pokes: %+v", snap)
	}

	a := newSkeletonAvatarSnapped(t)
	aBase := skPaintToImage(t, a.Node(), 48, 48)
	a.sizePx = 99
	a.shape = AvatarSquare
	a.reduceMotion = true
	aAfter := skPaintToImage(t, a.Node(), 48, 48)
	if !skImagesEqual(aBase, aAfter) {
		t.Fatal("Avatar paint changed after live poke without refresh")
	}
	as := a.loadSnapshot()
	if as.Shape != AvatarCircle || as.ReduceMotion || as.SizePx == 99 {
		t.Fatalf("Avatar snapshot lost frozen inputs: %+v", as)
	}

	b := newSkeletonButtonSnapped(t)
	bBase := skPaintToImage(t, b.Node(), 240, 120)
	b.shape = ButtonCircle
	b.reduceMotion = true
	bAfter := skPaintToImage(t, b.Node(), 240, 120)
	if !skImagesEqual(bBase, bAfter) {
		t.Fatal("Button paint changed after live poke without refresh")
	}
	bs := b.loadSnapshot()
	if bs.Shape != ButtonDefault || bs.ReduceMotion {
		t.Fatalf("Button snapshot lost frozen inputs: %+v", bs)
	}

	n := newSkeletonNodeSnapped(t)
	nBase := skPaintToImage(t, n.Node(), 96, 96)
	n.child = rendering.NewRenderBox()
	nAfter := skPaintToImage(t, n.Node(), 96, 96)
	if !skImagesEqual(nBase, nAfter) {
		t.Fatal("Node paint changed after live poke without refresh")
	}
	ns := n.loadSnapshot()
	if ns.HasChild {
		t.Fatalf("Node snapshot picked up live child poke: %+v", ns)
	}
}