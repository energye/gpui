package rendering

import (
	"testing"

	"github.com/energye/gpui/ui/scene"
)

// TestRecordOwnContent_TextFaceFollowsFontSize guards against the R21 regression
// where boundary Pictures recorded the raw SetFace face (wrkit FaceAt falls back
// to the base 14pt MultiFace because MultiFace.Source() is nil). Replay then drew
// every label at 14pt regardless of FontSize, and live-paint frames (16pt)
// alternated with Replay frames (14pt) while dragging → title text toggled size.
func TestRecordOwnContent_TextFaceFollowsFontSize(t *testing.T) {
	base, _, err := LoadFaceByFamily("sans-serif", 14)
	if err != nil || base == nil {
		t.Skipf("no system face: %v", err)
	}
	mkLabel := func() *RenderText {
		l := NewRenderText("TITLE")
		l.FontSize = 16
		l.SetFace(base)
		return l
	}

	// Path 1: boundary node itself is a RenderText (recordOwnContent RenderText case).
	labelSelf := mkLabel()
	picSelf := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordOwnContent(r, labelSelf, 0, 0)
	})
	if picSelf.OpCount() != 1 || picSelf.Ops[0].Kind != scene.OpDrawString {
		t.Fatalf("self: want OpDrawString, got ops=%d", picSelf.OpCount())
	}
	if picSelf.Ops[0].Face == nil || picSelf.Ops[0].Face.Size() != 16 {
		t.Fatalf("self: recorded face size=%v want 16 (FontSize), Face=%v",
			faceSize(picSelf.Ops[0].Face), picSelf.Ops[0].Face)
	}

	// Path 2: non-RB child of an AbsoluteBox (recordAbsoluteOwnContent case).
	box := NewAbsoluteBox(120, 40)
	labelChild := mkLabel()
	box.Place(labelChild, 4, 4)
	picChild := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordAbsoluteOwnContent(r, box, 0, 0)
	})
	var found bool
	for _, op := range picChild.Ops {
		if op.Kind == scene.OpDrawString && op.Face != nil {
			found = true
			if op.Face.Size() != 16 {
				t.Fatalf("child: recorded face size=%v want 16 (FontSize)", faceSize(op.Face))
			}
		}
	}
	if !found {
		t.Fatal("child: no OpDrawString recorded")
	}
}

func faceSize(f any) float64 {
	type sized interface{ Size() float64 }
	if s, ok := f.(sized); ok {
		return s.Size()
	}
	return -1
}
