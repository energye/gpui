package rendering_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// M0 item 3: DisplayLines must derive from TextLayout (I7). These tests are
// written RED-first: they fail while layout ignores MaxLines / wraps on a
// different path than display.

// MaxLines+Ellipsis: layout must truncate like display (the 2v12 bug).
func TestDisplayLinesMatchLayout_MaxLinesEllipsis(t *testing.T) {
	txt := rendering.NewRenderText("one two three four five six seven eight nine ten eleven twelve")
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	txt.SetMaxWidth(40)
	txt.SetMaxLines(2)
	txt.SetOverflow(rendering.TextOverflowEllipsis)

	lay := txt.TextLayout()
	lines := txt.DisplayLines()
	if lay == nil || lay.LineCount() != 2 {
		t.Fatalf("layout lines=%d want 2 (truncated)", lay.LineCount())
	}
	if len(lines) != lay.LineCount() {
		t.Fatalf("display %d vs layout %d", len(lines), lay.LineCount())
	}
	if !lay.Truncated {
		t.Fatalf("layout Truncated=false want true")
	}
	for i := range lines {
		ls, le, _, _, _ := lay.Line(i)
		want := lay.Text[ls:le]
		got := lines[i]
		if i == len(lines)-1 {
			got = strings.TrimSuffix(got, "…")
			want = strings.TrimSuffix(want, "…")
			if !strings.Contains(lines[i], "…") {
				t.Fatalf("last display line %q want ellipsis marker", lines[i])
			}
		}
		if got != want {
			t.Fatalf("line %d display %q != layout %q", i, lines[i], want)
		}
	}
}

// MaxLines+Clip: same truncation contract without the marker.
func TestDisplayLinesMatchLayout_MaxLinesClip(t *testing.T) {
	txt := rendering.NewRenderText("alpha beta gamma delta epsilon zeta eta theta iota kappa")
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	txt.SetMaxWidth(30)
	txt.SetMaxLines(2)
	txt.SetOverflow(rendering.TextOverflowClip)

	lay := txt.TextLayout()
	lines := txt.DisplayLines()
	if lay == nil || lay.LineCount() != 2 {
		t.Fatalf("layout lines=%d want 2 (truncated)", lay.LineCount())
	}
	if len(lines) != lay.LineCount() {
		t.Fatalf("display %d vs layout %d", len(lines), lay.LineCount())
	}
	for i := range lines {
		ls, le, _, _, _ := lay.Line(i)
		want := lay.Text[ls:le]
		if lines[i] != want {
			t.Fatalf("line %d display %q != layout %q", i, lines[i], want)
		}
	}
}

// No-face estimate: display and layout must break identically (the 11v1 bug).
func TestDisplayLinesMatchLayout_NoFaceEstimate(t *testing.T) {
	txt := rendering.NewRenderText("The quick brown fox jumps over the lazy dog and then keeps running far beyond")
	txt.FontSize = 10
	txt.ApproxCharW = 1.0
	txt.SetMaxWidth(100)

	lay := txt.TextLayout()
	lines := txt.DisplayLines()
	if lay == nil || lay.LineCount() == 0 {
		t.Fatalf("no layout lines")
	}
	if len(lines) != lay.LineCount() {
		t.Fatalf("display %d vs layout %d: %q", len(lines), lay.LineCount(), lines)
	}
	for i := range lines {
		ls, le, _, _, _ := lay.Line(i)
		want := lay.Text[ls:le]
		if lines[i] != want {
			t.Fatalf("line %d display %q != layout %q", i, lines[i], want)
		}
	}
}

// Wrap with a real face: already consistent pre-fix; locks against regression.
func TestDisplayLinesMatchLayout_WrapLock(t *testing.T) {
	face, _, err := text.LoadDefaultFace(14)
	if err != nil || face == nil {
		t.Skipf("LoadDefaultFace unavailable: %v", err)
	}
	txt := rendering.NewRenderText("The quick brown fox jumps over the lazy dog")
	txt.FontSize = 14
	txt.SetFace(face)
	txt.SetMaxWidth(150)

	lay := txt.TextLayout()
	lines := txt.DisplayLines()
	if len(lines) != lay.LineCount() {
		t.Fatalf("display %d vs layout %d", len(lines), lay.LineCount())
	}
	for i := range lines {
		ls, le, _, _, _ := lay.Line(i)
		want := lay.Text[ls:le]
		if lines[i] != want {
			t.Fatalf("line %d display %q != layout %q", i, lines[i], want)
		}
	}
}

// Multi-run: display lines must match the run layout's line ranges.
func TestDisplayLinesMatchLayout_MultiRun(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.AddRun(rendering.TextRun{Text: "AAAA ", FontSize: 10, ApproxCharW: 1, R: 1, G: 0, B: 0, A: 1})
	b.AddRun(rendering.TextRun{Text: "BBBB CCCC DDDD EEEE FFFF GGGG", FontSize: 10, ApproxCharW: 1, R: 0, G: 1, B: 0, A: 1})
	rt := b.Build()
	rt.SetMaxWidth(50)
	rt.SetMaxLines(2)
	rt.SetOverflow(rendering.TextOverflowEllipsis)

	lay := rt.TextLayout()
	lines := rt.DisplayLines()
	if lay == nil || lay.LineCount() == 0 {
		t.Fatalf("no layout lines")
	}
	if len(lines) != lay.LineCount() {
		t.Fatalf("display %d vs layout %d: %q", len(lines), lay.LineCount(), lines)
	}
	if len(lines) > 2 {
		t.Fatalf("lines=%d want ≤2", len(lines))
	}
}
