package rendering

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/scene"
)

// Retained pictures must show the same lines as direct Paint frames: one
// DrawString per display line, never a single op holding raw newlines.
func TestRecordRenderText_Multiline(t *testing.T) {
	txt := NewRenderText("alpha\nbeta gamma delta epsilon zeta eta theta\nomega")
	txt.MaxWidth = 60
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordRenderText(r, txt, 0, 0)
	})
	if pic.OpCount() == 0 {
		t.Fatal("no ops recorded")
	}
	want := 0
	for _, ln := range txt.DisplayLines() {
		if ln != "" {
			want++
		}
	}
	if pic.OpCount() != want {
		t.Fatalf("ops=%d want %d (display lines)", pic.OpCount(), want)
	}
	prevY := -1.0
	for _, op := range pic.Ops {
		if op.Kind != scene.OpDrawString {
			t.Fatalf("kind=%v want OpDrawString", op.Kind)
		}
		if strings.Contains(op.Text, "\n") {
			t.Fatalf("op holds raw newline: %q", op.Text)
		}
		if op.Y <= prevY {
			t.Fatalf("Y not increasing: %v after %v", op.Y, prevY)
		}
		prevY = op.Y
	}
}

func TestRecordRenderText_YBandCullsToVisible(t *testing.T) {
	s := "hello" + strings.Repeat("\n", 30) + "TAIL"
	txt := NewRenderText(s)
	txt.FontSize = 16
	txt.MaxWidth = 864
	lay := txt.TextLayout()
	totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
	visH := 74.0
	scrollBottom := totalH - visH
	txt.SetViewportRect(0, 864, scrollBottom, visH)
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordRenderText(r, txt, 0, 0)
	})
	foundTail, foundTop := false, false
	for _, op := range pic.Ops {
		if op.Text == "TAIL" {
			foundTail = true
		}
		if op.Text == "hello" {
			foundTop = true
		}
	}
	if !foundTail {
		t.Fatalf("tail missing in culled record ops=%d", pic.OpCount())
	}
	if foundTop {
		t.Fatalf("top should be culled at bottom scroll")
	}
	if txt.ViewportBandExpiredY(scrollBottom, visH) {
		t.Fatalf("same band should not expire")
	}
	txt.SetViewportRect(0, 864, 0, visH)
	if !txt.ViewportBandExpiredY(0, visH) {
		t.Fatalf("moved band should expire")
	}
}

func TestRecordRuns_YBandCullsToVisible(t *testing.T) {
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "TOP" + strings.Repeat("\n", 30) + "TAIL", FontSize: 14, R: 1, G: 1, B: 1, A: 1})
	txt := NewRenderText("")
	txt.MaxWidth = 864
	txt.SetRuns(pb.Runs())
	lines := txt.layoutRunLines()
	if len(lines) != 31 {
		t.Fatalf("want 31 run lines got %d", len(lines))
	}
	totalH := 0.0
	for _, ln := range lines {
		totalH += ln.Height
	}
	visH := 74.0
	txt.SetViewportRect(0, 864, totalH-visH, visH)
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordRenderText(r, txt, 0, 0)
	})
	foundTail, foundTop := false, false
	for _, op := range pic.Ops {
		if strings.Contains(op.Text, "TAIL") {
			foundTail = true
		}
		if strings.Contains(op.Text, "TOP") {
			foundTop = true
		}
	}
	if !foundTail {
		t.Fatalf("tail missing in culled run record ops=%d", pic.OpCount())
	}
	if foundTop {
		t.Fatalf("top should be culled at bottom scroll")
	}
}

// Multi-run content must keep per-span text and color in retained pictures.
func TestRecordRenderText_MultiRun(t *testing.T) {
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "red ", FontSize: 14, R: 1, G: 0, B: 0, A: 1})
	pb.AddRun(TextRun{Text: "green", FontSize: 14, R: 0, G: 1, B: 0, A: 1})
	txt := NewRenderText("")
	txt.MaxWidth = 1000
	txt.SetRuns(pb.Runs())
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordRenderText(r, txt, 5, 7)
	})
	if pic.OpCount() != 2 {
		t.Fatalf("ops=%d want 2 spans", pic.OpCount())
	}
	if pic.Ops[0].Text != "red " || pic.Ops[0].R != 1 {
		t.Fatalf("span0=%q color R=%v", pic.Ops[0].Text, pic.Ops[0].R)
	}
	if pic.Ops[1].Text != "green" || pic.Ops[1].G != 1 {
		t.Fatalf("span1=%q color G=%v", pic.Ops[1].Text, pic.Ops[1].G)
	}
	if pic.Ops[0].X != 5 {
		t.Fatalf("span0 X=%v want origin 5", pic.Ops[0].X)
	}
}

// Single-line text keeps the previous single-op shape.
func TestRecordRenderText_SingleLine(t *testing.T) {
	txt := NewRenderText("hello")
	pic := scene.RecordPicture(func(r *scene.PictureRecorder) {
		recordRenderText(r, txt, 2, 3)
	})
	if pic.OpCount() != 1 {
		t.Fatalf("ops=%d want 1", pic.OpCount())
	}
	if pic.Ops[0].Text != "hello" || pic.Ops[0].X != 2 {
		t.Fatalf("op=%+v", pic.Ops[0])
	}
}
