package textinput

import (
	"strings"
	"testing"
)

// linePen builds simple geometry helpers for tests: N display lines split at
// '\n', and a monospace pen model (each rune = width w). Mirrors the
// production contract: lineCount() from DisplayLines, penX() from
// RenderText.CaretColumn.
func linePen(display string, w float64) (lineCount func() int, penX func(int) float64, lines []string) {
	lines = strings.Split(display, "\n")
	lineCount = func() int { return len(lines) }
	penX = func(off int) float64 {
		if off > len(display) {
			off = len(display)
		}
		// Column-relative pen: count only bytes on THIS line (the newline
		// is a break, not a column) — mirrors RenderText.CaretColumn,
		// which measures the in-line prefix.
		lineStart := strings.LastIndexByte(display[:off], '\n') + 1
		return float64(off-lineStart) * w // monospace: 1 byte = 1 rune here
	}
	return
}

// TestMoveCaretVertically_BasicUpDown: two lines; caret on line 2 moves up to
// the same column, then back down.
func TestMoveCaretVertically_BasicUpDown(t *testing.T) {
	e := New()
	e.SetText("abcdef\nghijkl") // 6+6 monospace columns
	count, penX, _ := linePen(e.View().Display, 10)

	e.SetCaret(len("abcdef\ng")) // line 2, column 1 (display off 8)
	if !e.MoveCaretVertically(-1, count, penX) {
		t.Fatal("up move should succeed")
	}
	if got := e.View().Display[:e.Cursor()]; got != "a" {
		t.Fatalf("after Up caret=%q want \"a\" (same column)", got)
	}
	if !e.MoveCaretVertically(1, count, penX) {
		t.Fatal("down move should succeed")
	}
	if got := e.View().Display[:e.Cursor()]; got != "abcdef\ng" {
		t.Fatalf("after Down caret=%q want \"abcdef\\ng\"", got)
	}
}

// TestMoveCaretVertically_StickyColumn: moving up through a SHORTER line keeps
// the original column for the next move (standard desired-X behavior).
func TestMoveCaretVertically_StickyColumn(t *testing.T) {
	e := New()
	e.SetText("abc\nlonger line\nxyz") // middle line is longest
	count, penX, _ := linePen(e.View().Display, 10)

	e.SetCaret(len("abc\nlonger lin")) // line 2 (index 1), column 13
	if !e.MoveCaretVertically(-1, count, penX) {
		t.Fatal("up to line 1")
	}
	// Line 1 has only 3 columns → snap to its end ("abc").
	if e.Cursor() != len("abc") {
		t.Fatalf("on short line cursor=%d want %d", e.Cursor(), len("abc"))
	}
	// Down again must restore column 13 on line 2 (sticky), not column 3.
	if !e.MoveCaretVertically(1, count, penX) {
		t.Fatal("down to line 2")
	}
	if e.Cursor() != len("abc\nlonger lin") {
		t.Fatalf("sticky column lost: cursor=%d want %d", e.Cursor(), len("abc\nlonger lin"))
	}
}

// TestMoveCaretVertically_ClampsAtEdges: Up at top / Down at bottom stay put
// and report false.
func TestMoveCaretVertically_ClampsAtEdges(t *testing.T) {
	e := New()
	e.SetText("one\ntwo")
	count, penX, _ := linePen(e.View().Display, 10)

	e.SetCaret(0)
	if e.MoveCaretVertically(-1, count, penX) {
		t.Fatal("up at first line must be a no-op")
	}
	e.SetCaret(len("one\ntwo"))
	if e.MoveCaretVertically(1, count, penX) {
		t.Fatal("down at last line must be a no-op")
	}
}

// TestMoveCaretVertically_HorizontalResetsStickyColumn: after Left/Right the
// next vertical run re-adopts the new position's column.
func TestMoveCaretVertically_HorizontalResetsStickyColumn(t *testing.T) {
	e := New()
	e.SetText("abc\nlonger line\nxyz")
	count, penX, _ := linePen(e.View().Display, 10)

	e.SetCaret(len("abc\nlonger lin")) // column 13 on line 2
	if !e.MoveCaretVertically(-1, count, penX) {
		t.Fatal("up to line 1")
	}
	// Caret now sits ON the newline ending line 1 (display offset 3 == line
	// 1's column-3 boundary). Right moves into line 2's column 0 — the caret
	// is then VISUALLY on line 2, so Down must go to line 3.
	e.MoveCaretRunes(1)
	if !e.MoveCaretVertically(1, count, penX) {
		t.Fatal("down after horizontal move")
	}
	// Sticky re-adopted column 0 → lands on line 3 ("xyz") column 0.
	want := len("abc\nlonger line\n")
	if e.Cursor() != want {
		t.Fatalf("cursor=%d want %d (re-adopted column)", e.Cursor(), want)
	}
}

// TestMoveCaretVertically_Multibyte: CJK content — the sticky column is
// remembered in DISPLAY px, so moving up from rune column 1 of line 2 must
// land on rune column 1 of line 1 (byte offsets differ, geometry matches).
func TestMoveCaretVertically_Multibyte(t *testing.T) {
	e := New()
	e.SetText("你好世界\n第二行文本") // 4 runes per line, 3 bytes each
	count, penX, _ := linePen(e.View().Display, 20)

	// Line 2 starts at byte 13 (12 bytes line1 + '\n'); one rune = 3 bytes.
	e.SetCaret(16) // line 2, rune column 1
	if !e.MoveCaretVertically(-1, count, penX) {
		t.Fatal("up move")
	}
	if e.Cursor() != 3 { // line 1, rune column 1
		t.Fatalf("CJK up landed wrong: cursor=%d want 3", e.Cursor())
	}
}
