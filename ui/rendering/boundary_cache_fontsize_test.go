package rendering

import (
	"testing"

	"github.com/energye/gpui/render/recording"
)

// TestRecordOwnContent_TextFaceFollowsFontSize guards against the R21 regression
// where boundary recordings stored the raw SetFace face (wrkit FaceAt falls back
// to the base 14pt MultiFace because MultiFace.Source() is nil). Replay then drew
// every label at 14pt regardless of FontSize, and live-paint frames (16pt)
// alternated with replay frames (14pt) while dragging → title text toggled size.
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

	// recordedTextFace returns the pooled face of the recording's first text
	// command (the old Picture op.Face probe on the new carrier).
	recordedTextFace := func(rec *recording.Recording, wantOps int) (any, bool) {
		if rec.PaintOpCount() != wantOps {
			t.Fatalf("want %d ops, got ops=%d", wantOps, rec.PaintOpCount())
		}
		for _, cmd := range rec.Commands() {
			switch c := cmd.(type) {
			case recording.DrawTextCommand:
				return rec.Resources().GetFont(c.Font), true
			case recording.StrokeTextCommand:
				return rec.Resources().GetFont(c.Font), true
			case recording.DrawGlyphsCommand:
				return rec.Resources().GetFont(c.Face), true
			}
		}
		return nil, false
	}

	// Path 1: boundary node itself is a RenderText (recordOwnContent RenderText case).
	labelSelf := mkLabel()
	recSelf := recording.NewRecorder(120, 40)
	recordOwnContent(recSelf, labelSelf, 0, 0)
	faceSelf, ok := recordedTextFace(recSelf.FinishRecording(), 1)
	if !ok {
		t.Fatal("self: want text command, got none")
	}
	if size := faceSize(faceSelf); size != 16 {
		t.Fatalf("self: recorded face size=%v want 16 (FontSize), Face=%v", size, faceSelf)
	}

	// Path 2: non-RB child of an AbsoluteBox (recordAbsoluteOwnContent case).
	box := NewAbsoluteBox(120, 40)
	labelChild := mkLabel()
	box.Place(labelChild, 4, 4)
	recChild := recording.NewRecorder(120, 40)
	recordAbsoluteOwnContent(recChild, box, 0, 0)
	built := recChild.FinishRecording()
	if built.PaintOpCount() == 0 {
		t.Fatal("child: no text command recorded")
	}
	var faceChild any
	for _, cmd := range built.Commands() {
		switch c := cmd.(type) {
		case recording.DrawTextCommand:
			faceChild = built.Resources().GetFont(c.Font)
		case recording.StrokeTextCommand:
			faceChild = built.Resources().GetFont(c.Font)
		case recording.DrawGlyphsCommand:
			faceChild = built.Resources().GetFont(c.Face)
		}
		if faceChild != nil {
			break
		}
	}
	if faceChild == nil {
		t.Fatal("child: no text command recorded")
	}
	if size := faceSize(faceChild); size != 16 {
		t.Fatalf("child: recorded face size=%v want 16 (FontSize)", size)
	}
}

func faceSize(f any) float64 {
	type sized interface{ Size() float64 }
	if s, ok := f.(sized); ok {
		return s.Size()
	}
	return -1
}
