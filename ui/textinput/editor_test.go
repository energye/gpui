package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
)

func TestNewEmpty(t *testing.T) {
	e := New()
	if !e.IsEmpty() || e.Text() != "" || e.LenRunes() != 0 {
		t.Fatalf("new editor not empty: %q", e.Text())
	}
	if s, en := e.Selection(); s != 0 || en != 0 {
		t.Fatalf("selection = (%d,%d)", s, en)
	}
}

func TestInsertASCII(t *testing.T) {
	e := New()
	e.Insert("a")
	e.Insert("b")
	if e.Text() != "ab" {
		t.Fatalf("text = %q", e.Text())
	}
	if e.Cursor() != 2 {
		t.Fatalf("cursor = %d", e.Cursor())
	}
}

func TestInsertCJK(t *testing.T) {
	e := New()
	e.Insert("你好")
	if e.Text() != "你好" {
		t.Fatalf("text = %q", e.Text())
	}
	// CJK 3 bytes per rune.
	if e.Cursor() != 6 {
		t.Fatalf("cursor = %d (want 6 bytes)", e.Cursor())
	}
	if e.LenRunes() != 2 {
		t.Fatalf("runes = %d", e.LenRunes())
	}
}

func TestInsertReplacesSelection(t *testing.T) {
	e := New()
	e.SetText("hello world")
	e.SetSelection(0, 5) // select "hello"
	e.Insert("hi")
	if e.Text() != "hi world" {
		t.Fatalf("text = %q", e.Text())
	}
	if e.Cursor() != 2 {
		t.Fatalf("cursor = %d", e.Cursor())
	}
}

func TestDeleteBackward(t *testing.T) {
	e := New()
	e.SetText("abc")
	e.SetCaret(3)
	e.DeleteBackward()
	if e.Text() != "ab" || e.Cursor() != 2 {
		t.Fatalf("text=%q cursor=%d", e.Text(), e.Cursor())
	}
	// CJK rune deletion.
	e.SetText("a你b")
	e.SetCaret(len("a你b"))
	e.DeleteBackward() // deletes 'b'
	if e.Text() != "a你" {
		t.Fatalf("text = %q", e.Text())
	}
	e.DeleteBackward() // deletes 你 (3 bytes)
	if e.Text() != "a" {
		t.Fatalf("text = %q", e.Text())
	}
}

func TestDeleteSelection(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetSelection(1, 4) // "ell"
	e.DeleteBackward()
	if e.Text() != "ho" {
		t.Fatalf("text = %q", e.Text())
	}
}

func TestDeleteForward(t *testing.T) {
	e := New()
	e.SetText("abc")
	e.SetCaret(0)
	e.DeleteForward()
	if e.Text() != "bc" || e.Cursor() != 0 {
		t.Fatalf("text=%q cursor=%d", e.Text(), e.Cursor())
	}
}

func TestMoveCaretRunes(t *testing.T) {
	e := New()
	e.SetText("你好world")
	e.SetCaret(0)
	e.MoveCaretRunes(1)
	if e.Cursor() != 3 { // 你 = 3 bytes
		t.Fatalf("cursor after +1 = %d", e.Cursor())
	}
	e.MoveCaretRunes(2) // 好 + w
	if e.Cursor() != 3+3+1 {
		t.Fatalf("cursor after +2 = %d", e.Cursor())
	}
	e.MoveCaretRunes(-3)
	if e.Cursor() != 0 {
		t.Fatalf("cursor after -3 = %d", e.Cursor())
	}
}

func TestOnChangeFires(t *testing.T) {
	e := New()
	n := 0
	e.OnChange = func() { n++ }
	e.Insert("a")      // 1
	e.Insert("b")      // 2
	e.DeleteBackward() // 3
	if n != 3 {
		t.Fatalf("change fires = %d, want 3", n)
	}
}

func TestComposeLifecycle(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetCaret(5)

	// Begin compose: pre-edit "ni" inserted at caret.
	e.BeginCompose("ni")
	if e.Text() != "helloni" {
		t.Fatalf("compose begin text = %q", e.Text())
	}
	if !e.ComposeActive() {
		t.Fatal("compose should be active")
	}
	if s, en, ok := e.ComposeRange(); !ok || s != 5 || en != 7 {
		t.Fatalf("compose range = (%d,%d,%v)", s, en, ok)
	}
	// Update compose.
	e.UpdateCompose("nihao", 5)
	if e.Text() != "hellonihao" {
		t.Fatalf("compose update text = %q", e.Text())
	}
	// Commit: pre-edit stays as committed text.
	e.CommitCompose()
	if e.Text() != "hellonihao" {
		t.Fatalf("compose commit text = %q", e.Text())
	}
	if e.ComposeActive() {
		t.Fatal("compose should be inactive after commit")
	}
	if e.Cursor() != len("hellonihao") {
		t.Fatalf("cursor after commit = %d", e.Cursor())
	}
}

func TestCancelComposeRollsBack(t *testing.T) {
	e := New()
	e.SetText("abc")
	e.SetCaret(3)
	e.BeginCompose("xyz")
	if e.Text() != "abcxyz" {
		t.Fatalf("before cancel: %q", e.Text())
	}
	e.CancelCompose()
	if e.Text() != "abc" {
		t.Fatalf("after cancel: %q", e.Text())
	}
	if e.ComposeActive() {
		t.Fatal("compose should be inactive")
	}
}

func TestInsertCancelsCompose(t *testing.T) {
	e := New()
	e.SetText("abc")
	e.SetCaret(3)
	e.BeginCompose("xyz")
	e.Insert("!")
	// Insert cancels compose and replaces caret position text.
	if e.Text() != "abc!" {
		t.Fatalf("text = %q", e.Text())
	}
	if e.ComposeActive() {
		t.Fatal("compose should be cancelled by insert")
	}
}

func TestApplyTextEvent(t *testing.T) {
	e := New()
	if e.ApplyText(input.TextEvent{Text: "你好"}) != true {
		t.Fatal("ApplyText should report change")
	}
	if e.Text() != "你好" {
		t.Fatalf("text = %q", e.Text())
	}
	if e.ApplyText(input.TextEvent{Text: ""}) != false {
		t.Fatal("empty ApplyText should not change")
	}
}

func TestApplyIMEComposeAndCommit(t *testing.T) {
	e := New()
	// Compose event starts pre-edit.
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"})
	if !e.ComposeActive() {
		t.Fatal("compose not started")
	}
	// Commit event with text replaces pre-edit.
	e.ApplyIME(input.IMEEvent{Kind: input.IMECommit, Text: "你"})
	if e.Text() != "你" {
		t.Fatalf("text = %q", e.Text())
	}
	if e.ComposeActive() {
		t.Fatal("compose should be done after commit")
	}
}

func TestApplyIMEEmptyComposeResets(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	// Compose starts, then the compositor sends an empty preedit (= reset).
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"})
	if !e.ComposeActive() || e.Text() != "abni" {
		t.Fatalf("compose active: text=%q", e.Text())
	}
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	if e.ComposeActive() {
		t.Fatal("empty preedit must end the compose session")
	}
	if e.Text() != "ab" {
		t.Fatalf("reset should roll back pre-edit, text=%q", e.Text())
	}
	// An empty preedit with no active session must NOT start one (this is
	// what latched compose state after a commit and blocked plain typing).
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	if e.ComposeActive() {
		t.Fatal("empty preedit on inactive session must stay inactive")
	}
}

func TestCutCopyPaste(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetSelection(1, 4)
	if e.Copy() != "ell" {
		t.Fatalf("copy = %q", e.Copy())
	}
	// Copy does not mutate.
	if e.Text() != "hello" {
		t.Fatalf("copy mutated: %q", e.Text())
	}
	if e.Cut() != "ell" {
		t.Fatalf("cut = %q", e.Cut())
	}
	if e.Text() != "ho" {
		t.Fatalf("after cut: %q", e.Text())
	}
	e.Paste("ELL")
	if e.Text() != "hELLo" {
		t.Fatalf("after paste: %q", e.Text())
	}
}

func TestEditorStringAndTrim(t *testing.T) {
	e := New()
	e.SetText("  hi  ")
	if e.String() != "  hi  " {
		t.Fatalf("string = %q", e.String())
	}
	if e.Trimmed() != "hi" {
		t.Fatalf("trimmed = %q", e.Trimmed())
	}
}

func TestApplyIMEComposeCaretIndex(t *testing.T) {
	e := New()
	e.SetText("hi")
	e.SetCaret(2)
	// First preedit event carries the caret index (end of "ni").
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni", Start: 2, End: 2})
	if e.Text() != "hini" || e.Cursor() != len("hini") {
		t.Fatalf("compose begin: text=%q cursor=%d", e.Text(), e.Cursor())
	}
	// Follow-up preedit with index -1 → caret at the end of the pre-edit.
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihao", Start: -1, End: -1})
	if e.Cursor() != len("hinihao") {
		t.Fatalf("caret at end: %d", e.Cursor())
	}
	// Caret inside the pre-edit (byte offset 3 = after "nih").
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihaoni", Start: 3, End: 3})
	if e.Cursor() != len("hi")+3 {
		t.Fatalf("caret mid pre-edit: %d", e.Cursor())
	}
}

func TestApplyIMEDeleteSurrounding(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetCaret(5)
	// Delete one byte before the caret: "hello" → "hell".
	e.ApplyIME(input.IMEEvent{Kind: input.IMEDeleteSurrounding, Start: -1, End: 0})
	if e.Text() != "hell" || e.Cursor() != 4 {
		t.Fatalf("delete before: text=%q cursor=%d", e.Text(), e.Cursor())
	}
	// Delete one byte on each side of the caret ("el"); caret lands on the
	// deletion start.
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMEDeleteSurrounding, Start: -1, End: 1})
	if e.Text() != "hl" || e.Cursor() != 1 {
		t.Fatalf("delete around: text=%q cursor=%d", e.Text(), e.Cursor())
	}
}

func TestDeleteSurroundingCJKSnapsRuneBoundary(t *testing.T) {
	e := New()
	e.SetText("你好")
	e.SetCaret(3) // boundary between 你 and 好
	// A misaligned byte count (1) snaps back to the full 3-byte 你.
	e.ApplyIME(input.IMEEvent{Kind: input.IMEDeleteSurrounding, Start: -1, End: 0})
	if e.Text() != "好" || e.Cursor() != 0 {
		t.Fatalf("cjk snap: text=%q cursor=%d", e.Text(), e.Cursor())
	}
}

func TestDeleteSurroundingCancelsOverlappingCompose(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.BeginCompose("cd") // "abcd", compose [2,4)
	// Deletion [0..3) crosses the pre-edit region → session cancelled.
	e.ApplyIME(input.IMEEvent{Kind: input.IMEDeleteSurrounding, Start: -2, End: 1})
	if e.ComposeActive() {
		t.Fatal("overlapping deletion should cancel compose")
	}
	if e.Text() != "d" || e.Cursor() != 0 {
		t.Fatalf("text=%q cursor=%d", e.Text(), e.Cursor())
	}
}

func TestDeleteSurroundingShiftsComposeRegion(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.BeginCompose("xy") // "abxy", compose [2,4)
	// Delete the two bytes before the pre-edit region → region shifts left.
	e.ApplyIME(input.IMEEvent{Kind: input.IMEDeleteSurrounding, Start: -2, End: 0})
	if s, en, ok := e.ComposeRange(); !ok || s != 0 || en != 2 {
		t.Fatalf("compose range after shift = (%d,%d,%v)", s, en, ok)
	}
	if e.Text() != "xy" {
		t.Fatalf("text = %q", e.Text())
	}
}
