package rendering

import (
	"os"
	"testing"

	"github.com/energye/gpui/render/text"
)

// m5ColorFace builds a DejaVu + NotoColorEmoji MultiFace, skipping when the
// system fonts are absent (same Skip discipline as render/text hint tests).
func m5ColorFace(t *testing.T) text.Face {
	t.Helper()
	open := func(p string) text.Face {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		src, err := text.NewFontSource(b)
		if err != nil {
			return nil
		}
		t.Cleanup(func() { _ = src.Close() })
		return src.Face(16)
	}
	latin := open("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf")
	emoji := open("/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf")
	if latin == nil || emoji == nil {
		t.Skip("need DejaVuSans + NotoColorEmoji system fonts")
	}
	mf, err := text.NewMultiFace(latin, emoji)
	if err != nil {
		t.Fatalf("NewMultiFace: %v", err)
	}
	return mf
}

// TestIsColorText_M5 locks the M5-1 detector: emoji-presentation sequences
// are color, plain text is not.
func TestIsColorText_M5(t *testing.T) {
	for _, s := range []string{"🎉", "👨\u200d👩\u200d👧", "hello 🎉", "👋🏽"} {
		if !isColorText(s) {
			t.Errorf("isColorText(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"hello", "你好世界", "مرحبا", "", "123"} {
		if isColorText(s) {
			t.Errorf("isColorText(%q) = true, want false", s)
		}
	}
}

// TestColorRunMarked_M5 locks M5-1 marking: the emoji run of a mixed line
// carries IsColor with its byte range; the latin run does not.
func TestColorRunMarked_M5(t *testing.T) {
	mf := m5ColorFace(t)
	line := "hi 🎉"
	lay := BuildTextLayout(line, mf, 16, 0, 1.2)
	if lay == nil || lay.LineCount() == 0 {
		t.Fatal("no layout")
	}
	runs := lay.LineGlyphRuns(0)
	if len(runs) == 0 {
		t.Fatal("no glyph runs")
	}
	found := false
	for _, r := range runs {
		if !r.IsColor {
			continue
		}
		found = true
		if r.TextStart < 0 || r.TextEnd > len(line) || r.TextStart >= r.TextEnd {
			t.Fatalf("color run byte range [%d,%d) invalid for %q", r.TextStart, r.TextEnd, line)
		}
		if got := line[r.TextStart:r.TextEnd]; !isColorText(got) {
			t.Fatalf("color run text %q is not color", got)
		}
	}
	if !found {
		t.Fatalf("no IsColor run in %q (%d runs)", line, len(runs))
	}
	for _, r := range runs {
		if r.IsColor {
			continue
		}
		if got := line[r.TextStart:r.TextEnd]; isColorText(got) {
			t.Fatalf("non-color run text %q should be color", got)
		}
	}
}

// TestColorRunBypassesMask_M5 locks M5-1 routing: mask load excludes color
// runs while total submissions still count them (separate channel, I3 holds:
// coordinates untouched).
func TestColorRunBypassesMask_M5(t *testing.T) {
	mf := m5ColorFace(t)
	rt := NewRenderText("hi 🎉 bye")
	rt.SetFace(mf)
	total := rt.SubmittedGlyphEstimate()
	mask := rt.SubmittedMaskGlyphEstimate()
	if total <= 0 {
		t.Fatalf("total estimate = %d, want > 0", total)
	}
	if mask < 0 || mask >= total {
		t.Fatalf("mask estimate = %d, want in [0,%d)", mask, total)
	}
}

// TestRichColorRunMarked_M5 locks TextRun auto-marking: emoji runs added via
// the builder carry IsColor into spans; plain runs do not.
func TestRichColorRunMarked_M5(t *testing.T) {
	pb := NewParagraphBuilder()
	pb.AddRun(TextRun{Text: "hello ", FontSize: 14})
	pb.AddRun(TextRun{Text: "🎉", FontSize: 14})
	rt := NewRenderText("")
	rt.SetRuns(pb.Runs())
	if !rt.Runs[1].IsColor {
		t.Fatal("emoji TextRun not marked IsColor")
	}
	if rt.Runs[0].IsColor {
		t.Fatal("plain TextRun wrongly marked IsColor")
	}
	lines := rt.layoutRunLines()
	if len(lines) != 1 || len(lines[0].Spans) != 2 {
		t.Fatalf("want 1 line / 2 spans, got %d / %d", len(lines), len(lines[0].Spans))
	}
	if !lines[0].Spans[1].IsColor || lines[0].Spans[0].IsColor {
		t.Fatal("span IsColor not carried from runs")
	}
}
