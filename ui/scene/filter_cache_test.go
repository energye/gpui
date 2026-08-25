package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

func testImageBuf(t *testing.T) *render.ImageBuf {
	t.Helper()
	buf, err := render.NewImageBuf(64, 64, render.FormatRGBAPremul)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	return buf
}

func grayMatrixTest() [20]float32 {
	return [20]float32{
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0.299, 0.587, 0.114, 0, 0,
		0, 0, 0, 1, 0,
	}
}

func filterTestTree() *ColorFilterLayer {
	pic := NewPictureLayer()
	pic.Picture = Picture{Bounds: image.Rect(10, 20, 110, 120)}
	off := NewOffsetLayer(5, 7)
	off.Add(pic)
	cf := NewColorFilterLayer(grayMatrixTest())
	cf.Add(off)
	return cf
}

func emptyDirty() map[uint64]struct{} { return map[uint64]struct{}{} }

func TestFilterCache_HitThenInvalidateOnParams(t *testing.T) {
	fc := NewFilterResultCache()
	cf := filterTestTree()
	key := uint64(42)

	fp1 := filterFingerprint(cf, emptyDirty(), fc.seed)
	buf := testImageBuf(t)
	fc.put(key, fp1, buf, 1, 2, 100, 100)
	if e := fc.get(key, fp1); e == nil {
		t.Fatal("same fingerprint must hit")
	}

	cf.Matrix = identityColorMatrixTest()
	fp2 := filterFingerprint(cf, emptyDirty(), fc.seed)
	if fp1 == fp2 {
		t.Fatal("matrix change must change the fingerprint")
	}
	if e := fc.get(key, fp2); e != nil {
		t.Fatal("changed params must miss")
	}
}

func TestFilterCache_DirtySubtreeInvalidates(t *testing.T) {
	fc := NewFilterResultCache()
	cf := filterTestTree()
	key := uint64(7)

	fpClean := filterFingerprint(cf, emptyDirty(), fc.seed)
	fc.put(key, fpClean, testImageBuf(t), 0, 0, 50, 50)

	var picID uint64
	subtreeIDs(cf, func(id uint64) { picID = id })
	dirty := map[uint64]struct{}{picID: {}}
	fpDirty := filterFingerprint(cf, dirty, fc.seed)

	if fpClean == fpDirty {
		t.Fatal("dirty subtree id must change the fingerprint")
	}
	if e := fc.get(key, fpDirty); e != nil {
		t.Fatal("dirty subtree must miss")
	}
	if e := fc.get(key, fpClean); e == nil {
		t.Fatal("clean fingerprint must still hit")
	}
}

func TestFilterCache_AgingDropsStaleEntries(t *testing.T) {
	fc := NewFilterResultCache()
	cf := filterTestTree()
	fp := filterFingerprint(cf, emptyDirty(), fc.seed)
	fc.put(1, fp, testImageBuf(t), 0, 0, 10, 10)
	for i := 0; i < 300; i++ {
		fc.BeginFrame()
	}
	if e := fc.get(1, fp); e != nil {
		t.Fatal("stale entry must be aged out")
	}
}

func TestFilterSubtreeBounds_OffsetChain(t *testing.T) {
	cf := filterTestTree()
	b, ok := filterSubtreeBounds(cf)
	if !ok {
		t.Fatal("picture-only subtree must be bounds-collectable")
	}
	// picture (10,20)-(110,120) inside offset (5,7) ⇒ (15,27)-(115,127).
	want := image.Rect(15, 27, 115, 127)
	if b != want {
		t.Fatalf("bounds=%v want %v", b, want)
	}
}

func TestSubtreeHasTransform(t *testing.T) {
	cf := filterTestTree()
	if subtreeHasTransform(cf) {
		t.Fatal("no transform in tree — must report false")
	}
	pic := NewPictureLayer()
	pic.Picture = Picture{Bounds: image.Rect(0, 0, 10, 10)}
	tr := NewTransformLayer(0, 0, 0.5, 1, 1)
	tr.Add(pic)
	cf2 := NewColorFilterLayer(grayMatrixTest())
	cf2.Add(tr)
	if !subtreeHasTransform(cf2) {
		t.Fatal("transform in subtree must be detected")
	}
}

func identityColorMatrixTest() [20]float32 {
	var m [20]float32
	m[0], m[6], m[12], m[18] = 1, 1, 1, 1
	return m
}

type renderImageBufStub struct{}
