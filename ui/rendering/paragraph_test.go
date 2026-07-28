package rendering_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func TestParagraphBuilder_TwoRunsDifferentColor(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.SetDefaultStyle(nil, 12, 1, 0, 0, 1, 1.0) // red, 1em/char
	b.AddText("Hello")
	b.SetDefaultStyle(nil, 12, 0, 0, 1, 1, 1.0) // blue
	b.AddText("World")

	rt := b.Build()
	if rt.RunCount() != 2 {
		t.Fatalf("runs=%d want 2", rt.RunCount())
	}
	if rt.Text != "HelloWorld" {
		t.Fatalf("concat Text=%q", rt.Text)
	}
	// Colors must differ across runs (not collapsed to one style).
	if rt.Runs[0].R == rt.Runs[1].R && rt.Runs[0].B == rt.Runs[1].B {
		t.Fatalf("runs collapsed to same color: %+v %+v", rt.Runs[0], rt.Runs[1])
	}
	if rt.Runs[0].Text != "Hello" || rt.Runs[1].Text != "World" {
		t.Fatalf("run texts=%q %q", rt.Runs[0].Text, rt.Runs[1].Text)
	}

	sz := rt.Layout(rendering.Loose(400, 100))
	// 10 chars * 12 * 1.0 ≈ 120
	if sz.Width < 100 || sz.Width > 140 {
		t.Fatalf("width=%v want ~120", sz.Width)
	}

	disp := rt.DisplayText()
	if disp != "HelloWorld" {
		t.Fatalf("display=%q", disp)
	}

	// Paint must not panic; uses per-span colors.
	dc := render.NewContext(200, 40)
	defer dc.Close()
	dc.BeginFrame()
	pc := rendering.NewPaintContext(dc, 1)
	rt.Paint(pc)
}

func TestParagraph_MaxWidthWrap_MultiRun(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.AddRun(rendering.TextRun{Text: "AAAA ", FontSize: 10, ApproxCharW: 1, R: 1, G: 0, B: 0, A: 1})
	b.AddRun(rendering.TextRun{Text: "BBBB CCCC DDDD", FontSize: 10, ApproxCharW: 1, R: 0, G: 1, B: 0, A: 1})
	rt := b.Build()
	rt.SetMaxWidth(50) // ~5 chars/line at 10px
	sz := rt.Layout(rendering.Loose(400, 400))
	if sz.Width > 50.1 {
		t.Fatalf("width=%v want ≤50", sz.Width)
	}
	lines := rt.DisplayLines()
	if len(lines) < 2 {
		t.Fatalf("expected wrap to multi-line, got %v", lines)
	}
	joined := strings.Join(lines, "")
	// Content preserved across runs (spaces may vary at wrap boundaries).
	if !strings.Contains(joined, "AAAA") || !strings.Contains(joined, "BBBB") {
		t.Fatalf("display lost run content: %v", lines)
	}
}

func TestParagraph_MaxLines_Ellipsis(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.AddRun(rendering.TextRun{
		Text:     "one two three four five six seven eight nine ten",
		FontSize: 10, ApproxCharW: 1, R: 0.9, G: 0.9, B: 0.9, A: 1,
	})
	b.AddRun(rendering.TextRun{
		Text:     " EXTRA",
		FontSize: 10, ApproxCharW: 1, R: 1, G: 0.2, B: 0.2, A: 1,
	})
	rt := b.Build()
	rt.SetMaxWidth(40)
	rt.SetMaxLines(2)
	rt.SetOverflow(rendering.TextOverflowEllipsis)
	sz := rt.Layout(rendering.Loose(400, 400))
	if sz.Width > 40.1 {
		t.Fatalf("width=%v", sz.Width)
	}
	lines := rt.DisplayLines()
	if len(lines) > 2 {
		t.Fatalf("lines=%d %v", len(lines), lines)
	}
	if !strings.Contains(rt.DisplayText(), "…") {
		t.Fatalf("want ellipsis in %q", rt.DisplayText())
	}
}

func TestParagraph_SetTextClearsRuns(t *testing.T) {
	b := rendering.NewParagraphBuilder()
	b.AddText("A")
	b.AddText("B")
	rt := b.Build()
	if rt.RunCount() != 2 {
		t.Fatal(rt.RunCount())
	}
	rt.SetText("plain")
	if rt.RunCount() != 0 {
		t.Fatalf("SetText should clear runs, got %d", rt.RunCount())
	}
	if rt.Text != "plain" {
		t.Fatal(rt.Text)
	}
}

func TestParagraph_SingleStringPathUnchanged(t *testing.T) {
	// Regression: plain NewRenderText still works without Runs.
	rt := rendering.NewRenderText("hello")
	rt.FontSize = 10
	rt.ApproxCharW = 1
	if rt.RunCount() != 0 {
		t.Fatal("default must be single-string mode")
	}
	sz := rt.Layout(rendering.Loose(200, 50))
	if sz.Width < 40 || sz.Width > 60 {
		t.Fatalf("width=%v", sz.Width)
	}
}
