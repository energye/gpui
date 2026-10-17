package rendering

import (
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/scene"
)

// TestBoundaryScore_MatchesRecordedOps pins the invariant behind the naive
// complexity gate: boundaryScore(n) must equal the number of ops
// recordOwnContent actually emits for n, so the gate can decide "worth
// caching" without recording first (Flutter dl_complexity.h naive Compute =
// op count; ShouldBeCached = score > 5).
func TestBoundaryScore_MatchesRecordedOps(t *testing.T) {
	bg := &Color{R: 0.2, G: 0.2, B: 0.25, A: 1}
	box := NewRenderColorBox(30, 20, 0.9, 0.1, 0.1, 1)
	bgOnly := NewAbsoluteBox(50, 40)
	bgOnly.Background = bg
	withChildren := NewAbsoluteBox(50, 40)
	withChildren.Background = bg
	for i := 0; i < 3; i++ {
		withChildren.Place(NewRenderColorBox(6, 6, 0.1, 0.9, 0.1, 1), float64(i*8), 4)
	}
	noBg := NewAbsoluteBox(50, 40)
	noBg.Place(NewRenderColorBox(6, 6, 0.1, 0.9, 0.1, 1), 0, 0)

	// Lay the boxes out: an unfixed AbsoluteBox has no size until layout, and a
	// zero-area fill emits no op (the score is state-derived and would then
	// count a background the recorder drops).
	root := NewAbsoluteBox(120, 90)
	root.Place(box, 0, 0)
	root.Place(bgOnly, 60, 0)
	root.Place(withChildren, 0, 50)
	root.Place(noBg, 60, 50)
	owner := NewPipelineOwner(root)
	owner.FlushLayout(Size{Width: 120, Height: 90}, true)

	cases := []struct {
		name string
		n    RenderObject
	}{
		{"colorbox", box},
		{"absolute-bg-only", bgOnly},
		{"absolute-bg+3-children", withChildren},
		{"absolute-no-bg+1-child", noBg},
	}
	for _, c := range cases {
		pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
			recordOwnContent(r, c.n, 0, 0)
		})
		if got, want := boundaryScore(c.n), pic.OpCount(); got != want {
			t.Errorf("%s: boundaryScore=%d recorded ops=%d", c.name, got, want)
		}
	}

	// PictureBox: the Record closure is opaque, so boundaryScore is 0 and the
	// StorePicture path scores the built Picture instead.
	pb := NewRenderPictureBox(20, 20, func(r *scene.PictureRecorder, ox, oy float64) {
		r.FillRect(ox, oy, 8, 8, 1, 0, 0, 1)
	})
	if got := boundaryScore(pb); got != 0 {
		t.Errorf("picture box boundaryScore=%d want 0 (scored from the built Picture)", got)
	}
	pbPic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordOwnContent(r, pb, 0, 0)
	})
	if pbPic.OpCount() != 1 {
		t.Errorf("picture box recorded ops=%d want 1", pbPic.OpCount())
	}

	// Text: the score is the visible row count and each row emits one op.
	face, _, err := LoadFaceByFamily("sans-serif", 14)
	if err != nil || face == nil {
		t.Skipf("no system face: %v", err)
	}
	txt := NewRenderText("hello")
	txt.FontSize = 14
	txt.SetFace(face)
	txtPic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordOwnContent(r, txt, 0, 0)
	})
	if got, want := boundaryScore(txt), txtPic.OpCount(); got != want {
		t.Errorf("text: boundaryScore=%d recorded ops=%d", got, want)
	}
}

// TestTextContentKey_DistinguishesState pins the fingerprint axes the
// parent-picture cache relies on: weight and spacing changes must yield a
// different key (Replay must miss) and repeated reads must be stable. The key
// is also the invalidation signal on machines whose installed face has a
// single grade — SetFontWeight then resolves the same file, so a weight change
// has no pixel axis to assert on (weight_test.go documents that).
func TestTextContentKey_DistinguishesState(t *testing.T) {
	face, _, err := LoadFaceByFamily("sans-serif", 14)
	if err != nil || face == nil {
		t.Skipf("no system face: %v", err)
	}
	txt := NewRenderText("AB")
	txt.FontSize = 14
	txt.SetFace(face)

	base := textContentKey(txt, 20, 16)
	if base == 0 {
		t.Fatal("empty fingerprint")
	}
	if got := textContentKey(txt, 20, 16); got != base {
		t.Fatalf("fingerprint unstable: %#x vs %#x", got, base)
	}
	txt.SetFontWeight(text.WeightBold)
	bold := textContentKey(txt, 20, 16)
	if bold == base {
		t.Error("weight change must change the fingerprint")
	}
	txt.SetLetterSpacing(6)
	if got := textContentKey(txt, 20, 16); got == bold {
		t.Error("spacing change must change the fingerprint")
	}
}
