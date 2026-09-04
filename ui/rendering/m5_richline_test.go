package rendering

import (
	"testing"
)

// TestRichLineMaxHeight_M5 locks M5-2 (Flutter SkParagraph semantics):
// a line mixing 8pt and 24pt runs opens its box by the row's maximum
// ascent/descent, never by the first run's metrics.
func TestRichLineMaxHeight_M5(t *testing.T) {
	rt := NewRenderText("")
	rt.MaxWidth = 0
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "small ", FontSize: 8, R: 1, G: 1, B: 1, A: 1})
	pb.AddRun(TextRun{Text: "BIG", FontSize: 24, R: 1, G: 1, B: 1, A: 1})
	rt.SetRuns(pb.Runs())

	lines := rt.layoutRunLines()
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1 (no wrap at MaxWidth=0)", len(lines))
	}
	smallH := rt.runLineHeight(TextRun{FontSize: 8})
	bigH := rt.runLineHeight(TextRun{FontSize: 24})
	if bigH <= smallH {
		t.Fatalf("fixture invalid: bigH=%v <= smallH=%v", bigH, smallH)
	}
	if lines[0].Height != bigH {
		t.Fatalf("line height = %v, want max run height %v", lines[0].Height, bigH)
	}
	if len(lines[0].Spans) != 2 {
		t.Fatalf("spans = %d, want 2", len(lines[0].Spans))
	}
}
