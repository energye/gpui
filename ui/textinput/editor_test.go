package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
)

func TestInsertBasics(t *testing.T) {
	e := New()
	e.Insert("a")
	e.Insert("你")
	if e.Text() != "a你" || e.Cursor() != 4 {
		t.Fatalf("text=%q cursor=%d", e.Text(), e.Cursor())
	}
	if e.LenRunes() != 2 {
		t.Fatalf("runes=%d", e.LenRunes())
	}
}

func TestEpochMonotonic(t *testing.T) {
	e := New()
	e0 := e.Epoch()
	e.Insert("x")
	e1 := e.Epoch()
	e.SetText("xy")
	e2 := e.Epoch()
	if !(e0 < e1 && e1 < e2) {
		t.Fatalf("epoch not monotonic: %d %d %d", e0, e1, e2)
	}
}

func TestDeleteRuneBoundary(t *testing.T) {
	e := New()
	e.SetText("a你b")
	e.SetCaret(len("a你b"))
	e.DeleteBackward()
	e.DeleteBackward()
	if e.Text() != "a" {
		t.Fatalf("text=%q", e.Text())
	}
	e.SetCaret(0)
	e.DeleteForward()
	if e.Text() != "" {
		t.Fatalf("after forward: %q", e.Text())
	}
}

func TestDeleteSurroundingSnaps(t *testing.T) {
	e := New()
	e.SetText("你好")
	e.SetCaret(3)
	if !e.DeleteSurrounding(1, 0) {
		t.Fatal("should report change")
	}
	if e.Text() != "好" || e.Cursor() != 0 {
		t.Fatalf("snap: text=%q cursor=%d", e.Text(), e.Cursor())
	}
	if e.DeleteSurrounding(0, 0) {
		t.Fatal("no-op should return false")
	}
}

func TestMoveCaretRunes(t *testing.T) {
	e := New()
	e.SetText("你好w")
	e.SetCaret(0)
	e.MoveCaretRunes(2)
	if e.Cursor() != 6 {
		t.Fatalf("+2 = %d", e.Cursor())
	}
	e.MoveCaretRunes(-2)
	if e.Cursor() != 0 {
		t.Fatalf("-2 = %d", e.Cursor())
	}
}

// pure four-tuple: text includes preedit
func TestComposeIncludesInText(t *testing.T) {
	e := New()
	e.SetText("hi")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"})
	if !e.ComposeActive() || e.CompositionText() != "ni" {
		t.Fatalf("compose not active: %q", e.CompositionText())
	}
	if e.Text() != "hini" {
		t.Fatalf("text should include preedit: %q", e.Text())
	}
	v := e.View()
	if v.Display != "hini" || v.CompStart != 2 || v.CompEnd != 4 {
		t.Fatalf("view = %+v", v)
	}
}

func TestEmptyPreeditOnlyTerminates(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	if e.ComposeActive() {
		t.Fatal("empty preedit started a session")
	}
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "cd"})
	if e.Text() != "abcd" {
		t.Fatalf("after cd text=%q", e.Text())
	}
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: ""})
	if e.ComposeActive() || e.Text() != "ab" {
		t.Fatalf("clear left residue: active=%v text=%q", e.ComposeActive(), e.Text())
	}
}

func TestCommitReplacesOverlayAtomically(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihao"})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECommit, Text: "你好"})
	if e.Text() != "ab你好" || e.ComposeActive() {
		t.Fatalf("text=%q active=%v", e.Text(), e.ComposeActive())
	}
	if e.Cursor() != len("ab你好") {
		t.Fatalf("cursor after commit = %d", e.Cursor())
	}
}

func TestComposedViewMappings(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "xyz"})
	v := e.View()
	cases := []struct{ view, buf int }{
		{0, 0}, {2, 2},
		{3, 2}, {5, 2},
		{6, 3}, {8, 5},
	}
	for _, c := range cases {
		if got := v.MapViewToBuf(c.view); got != c.buf {
			t.Fatalf("MapViewToBuf(%d) = %d, want %d", c.view, got, c.buf)
		}
	}
	if got := v.MapBufToViewExact(2, 1); got != 3 {
		t.Fatalf("MapBufToViewExact = %d", got)
	}
	bcases := []struct{ buf, view int }{
		{0, 0}, {2, 2},
		{3, 6}, {5, 8},
	}
	for _, c := range bcases {
		if got := v.MapBufToView(c.buf); got != c.view {
			t.Fatalf("MapBufToView(%d) = %d, want %d", c.buf, got, c.view)
		}
	}
	e2 := New()
	e2.SetText("abc")
	v2 := e2.View()
	if v2.MapViewToBuf(1) != 1 || v2.CompStart != -1 {
		t.Fatalf("identity mapping broken: %+v", v2)
	}
}

func TestByteOffsetAtSnapsOutOfSpan(t *testing.T) {
	e := New()
	e.SetText("ab")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "xyz"})
	widths := map[string]float64{}
	w := func(s string) float64 { widths[s]++; return float64(len(s)) }
	if got := e.ByteOffsetAt(4, w); got != 2 {
		t.Fatalf("click in span → %d, want 2", got)
	}
	if got := e.ByteOffsetAt(999, w); got != 5-3 {
		t.Fatalf("click past end → %d, want 2", got)
	}
}

func TestSnapshotIncludesComposition(t *testing.T) {
	e := New()
	e.SetText("hi")
	e.SetCaret(2)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "ni"})
	text, cur := e.Snapshot()
	if text != "hini" || cur != 2 {
		t.Fatalf("snapshot = (%q,%d), want (hini,2)", text, cur)
	}
}

func TestDeleteDuringCompositionDeletesInside(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetCaret(5)
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "f", Start: 1})
	e.DeleteBackward()
	if e.Text() != "hello" {
		t.Fatalf("after backspace in composing text=%q want hello", e.Text())
	}
	if e.CompositionText() != "" {
		t.Fatalf("composition should be empty after delete: %q", e.CompositionText())
	}
}

func TestClipboardRoundtrip(t *testing.T) {
	e := New()
	e.SetText("hello")
	e.SetSelectionBytes(1, 4)
	if e.Copy() != "ell" {
		t.Fatalf("copy = %q", e.Copy())
	}
	if e.Cut() != "ell" || e.Text() != "ho" {
		t.Fatalf("cut state = %q", e.Text())
	}
	if !e.Paste("ELL") || e.Text() != "hELLo" {
		t.Fatalf("paste state = %q", e.Text())
	}
}

func TestOnChangeFiresPerMutation(t *testing.T) {
	e := New()
	n := 0
	e.OnChange = func() { n++ }
	e.Insert("a")
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "b"})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECommit, Text: "c"})
	if n != 3 {
		t.Fatalf("change fires = %d, want 3", n)
	}
}
