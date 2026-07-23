package primitive_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

func TestTextEllipsisSingleLine(t *testing.T) {
	tx := primitive.NewText("HelloBeautifulWorld")
	tx.FontSize = 14
	tx.MaxWidth = 40 // ~5–6 glyphs at 0.5em
	tx.Ellipsis = true
	_ = tx.Layout(core.Loose(40, 100))
	shown := tx.DisplayedText()
	if shown == "" {
		t.Fatal("empty display")
	}
	if !strings.HasSuffix(shown, "…") && !strings.HasSuffix(shown, "...") {
		// Must use ellipsis char
		if !strings.Contains(shown, "…") {
			t.Fatalf("expected ellipsis in %q", shown)
		}
	}
	if utf8.RuneCountInString(shown) >= utf8.RuneCountInString(tx.Value) {
		t.Fatalf("truncated should be shorter: shown=%q full=%q", shown, tx.Value)
	}
	// Width must not exceed MaxWidth (within float tolerance).
	if tx.Size().Width > 40.5 {
		t.Fatalf("width=%v > MaxWidth", tx.Size().Width)
	}
}

func TestTextNoEllipsisClipsWidth(t *testing.T) {
	tx := primitive.NewText("HelloBeautifulWorld")
	tx.FontSize = 14
	tx.MaxWidth = 40
	tx.Ellipsis = false
	_ = tx.Layout(core.Loose(40, 100))
	// Full text kept for paint; box width capped.
	if tx.Size().Width > 40.5 {
		t.Fatalf("width=%v should cap to MaxWidth", tx.Size().Width)
	}
	// Displayed line is still full (clip paint only).
	if tx.DisplayedText() != "HelloBeautifulWorld" {
		t.Fatalf("without ellipsis paint text should stay full, got %q", tx.DisplayedText())
	}
}

func TestTextMaxLinesWrap(t *testing.T) {
	tx := primitive.NewText("one two three four five six")
	tx.FontSize = 14
	tx.MaxWidth = 50
	tx.MaxLines = 3
	tx.Ellipsis = false
	_ = tx.Layout(core.Loose(50, 200))
	lines := tx.DisplayedLines()
	if len(lines) < 2 {
		t.Fatalf("expected wrap to multiple lines, got %v", lines)
	}
	if len(lines) > 3 {
		t.Fatalf("MaxLines=3, got %d lines: %v", len(lines), lines)
	}
	// Height scales with line count.
	if tx.Size().Height < 20 {
		t.Fatalf("height=%v too small for multi-line", tx.Size().Height)
	}
}

func TestTextMaxLinesEllipsis(t *testing.T) {
	// Long content forced into 2 lines with ellipsis on last.
	tx := primitive.NewText("aaaaaaaa bbbbbbbb cccccccc dddddddd eeeeeeee")
	tx.FontSize = 14
	tx.MaxWidth = 45
	tx.MaxLines = 2
	tx.Ellipsis = true
	_ = tx.Layout(core.Loose(45, 200))
	lines := tx.DisplayedLines()
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %d %v", len(lines), lines)
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "…") {
		t.Fatalf("last line should ellipsize, got %q", last)
	}
}

func TestTextUnboundedIntrinsic(t *testing.T) {
	tx := primitive.NewText("Short")
	tx.FontSize = 14
	_ = tx.Layout(core.Loose(core.Unbounded, core.Unbounded))
	if tx.Size().Width <= 0 || tx.Size().Height <= 0 {
		t.Fatalf("intrinsic size=%v", tx.Size())
	}
	// No ellipsis applied.
	if tx.DisplayedText() != "Short" {
		t.Fatalf("got %q", tx.DisplayedText())
	}
}

func TestTextParentTightWidth(t *testing.T) {
	tx := primitive.NewText("0123456789ABCDEF")
	tx.FontSize = 14
	tx.Ellipsis = true
	// Parent tight width 30.
	_ = tx.Layout(core.Tight(30, 20))
	if tx.Size().Width > 30.5 {
		t.Fatalf("width=%v under tight 30", tx.Size().Width)
	}
	if !strings.Contains(tx.DisplayedText(), "…") {
		t.Fatalf("tight width should ellipsize: %q", tx.DisplayedText())
	}
}

func TestTextSetEllipsisAPI(t *testing.T) {
	tx := primitive.NewText("abcdefghij")
	tx.MaxWidth = 25
	tx.SetEllipsis(true)
	_ = tx.Layout(core.Loose(25, 40))
	if !strings.Contains(tx.DisplayedText(), "…") {
		t.Fatal("SetEllipsis(true) ineffective")
	}
	tx.SetEllipsis(false)
	_ = tx.Layout(core.Loose(25, 40))
	if strings.Contains(tx.DisplayedText(), "…") {
		t.Fatal("SetEllipsis(false) should not truncate string")
	}
}
