package textinput

import (
	"fmt"
	"strings"
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/rendering"
)

// Box scenario matrix (accept window A–F shapes, headless):
// single-line viewport (A/B/C), multi-line nowrap (D/E), multi-line wrap (F).
func newMatrixBox(t *testing.T, mode string) (*Editor, *Box) {
	t.Helper()
	ed := New()
	var box *Box
	switch mode {
	case "single":
		box = NewViewportInputBox(ed, 880, 40, 16)
	case "multi":
		box = NewMultiLineInputBox(ed, 880, 90, 16)
	case "wrap":
		box = NewMultiLineInputBox(ed, 880, 90, 16)
		box.SetWrap(true)
	default:
		t.Fatalf("unknown mode %q", mode)
	}
	return ed, box
}

func caretToEnd(ed *Editor) {
	n := utf16Len(ed.GetText())
	ed.SetSelection(TextRange{Base: n, Extent: n})
}

func caretToStart(ed *Editor) {
	ed.SetSelection(TextRange{Base: 0, Extent: 0})
}

// Caret driven to the end must land scroll at the content max with the
// caret visible; Home must return scroll to zero.
func TestBoxMatrix_ScrollClampEnd(t *testing.T) {
	for _, mode := range []string{"single", "multi", "wrap"} {
		ed, box := newMatrixBox(t, mode)
		ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
		if mode == "single" {
			ed.SetTextSimple(strings.Repeat("ab", 2000))
		}
		caretToEnd(ed)
		if mode == "single" {
			if box.ScrollX() <= 0 {
				t.Fatalf("%s: scrollX=%v want >0 at end", mode, box.ScrollX())
			}
			lay := box.TextLayout()
			x, _, _, ok := lay.GetOffsetForCaret(ed.GetCursorOffset(), 0, 1.5)
			if !ok {
				t.Fatalf("%s: no caret geometry", mode)
			}
			pad := box.Padding()
			visW := box.FixedWidth - 2 - 2*pad
			if x-box.ScrollX() < 0 || x-box.ScrollX() > visW {
				t.Fatalf("%s: caret x=%.1f outside view (scroll=%.1f visW=%.1f)", mode, x, box.ScrollX(), visW)
			}
		} else {
			lay := box.TextLayout()
			totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
			pad := box.Padding()
			visH := box.FixedHeight - 2*pad
			wantMax := totalH - visH
			if wantMax < 0 {
				wantMax = 0
			}
			if box.ScrollY() != wantMax {
				t.Fatalf("%s: scrollY=%.1f want max %.1f", mode, box.ScrollY(), wantMax)
			}
			_, y, h, ok := lay.GetOffsetForCaret(ed.GetCursorOffset(), 0, 1.5)
			if !ok {
				t.Fatalf("%s: no caret geometry", mode)
			}
			if y < box.ScrollY()-0.5 || y+h > box.ScrollY()+visH+0.5 {
				t.Fatalf("%s: caret y=%.1f h=%.1f outside view [%.1f,%.1f]", mode, y, h, box.ScrollY(), box.ScrollY()+visH)
			}
		}
		caretToStart(ed)
		if box.ScrollX() != 0 || box.ScrollY() != 0 {
			t.Fatalf("%s: after home scroll=(%.1f,%.1f) want (0,0)", mode, box.ScrollX(), box.ScrollY())
		}
	}
}

// Dragging below the box must extend the selection to the end instead of
// jumping back to the start (empty-line mapping regression).
func TestBoxMatrix_DragSelectsToEnd(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
	box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 20, Y: 10})
	last := -1
	for i := 0; i < 60; i++ {
		box.OnPointer(input.PointerEvent{Kind: input.PointerMove, X: 800, Y: 500})
		got := ed.SelectionRange().End()
		if got < last {
			t.Fatalf("drag jumped back: extent %d after %d", got, last)
		}
		last = got
		if got == utf16Len(ed.GetText()) {
			break
		}
	}
	box.OnPointer(input.PointerEvent{Kind: input.PointerUp, X: 800, Y: 500})
	want := utf16Len(ed.GetText())
	if got := ed.SelectionRange().End(); got != want {
		t.Fatalf("drag to bottom extent=%d want %d (full text)", got, want)
	}
	if got := ed.SelectionRange().Start(); got == want {
		t.Fatalf("drag collapsed at end, want a real range from top")
	}
}

// Selection highlights must exist for a real range, stay inside the visible
// window, and vanish for a collapsed caret.
func TestBoxMatrix_HighlightClippedToView(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
	n := utf16Len(ed.GetText())
	ed.SetSelection(TextRange{Base: 0, Extent: n})
	if len(box.highlights) == 0 {
		t.Fatalf("select-all produced no highlight boxes")
	}
	pad := box.Padding()
	for i, hb := range box.highlights {
		off := hb.Offset()
		if off.X < 0 || off.Y < 0 || off.X+hb.Width > box.FixedWidth || off.Y+hb.Height > box.FixedHeight {
			t.Fatalf("highlight %d at (%.1f,%.1f) size %.1fx%.1f escapes %vx%v box", i, off.X, off.Y, hb.Width, hb.Height, box.FixedWidth, box.FixedHeight)
		}
		_ = pad
	}
	caretToEnd(ed)
	if len(box.highlights) != 0 {
		t.Fatalf("collapsed caret left %d highlight boxes", len(box.highlights))
	}
}

// Down arrows from the top must walk every line to the end with the caret
// kept visible and scroll pinned at the max (no overshoot blank).
func TestBoxMatrix_DownToBottomKeepsCaretVisible(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
	caretToStart(ed)
	steps := 0
	for {
		before := ed.GetCursorOffset()
		box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowDown})
		steps++
		if ed.GetCursorOffset() == before {
			break
		}
		if steps > 100 {
			t.Fatalf("down walk did not terminate")
		}
	}
	if got, want := ed.GetCursorOffset(), len("hello")+30; got != want {
		t.Fatalf("down walk ended at byte %d want last-line start %d", got, want)
	}
	lay := box.TextLayout()
	totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
	pad := box.Padding()
	visH := box.FixedHeight - 2*pad
	wantMax := totalH - visH
	if wantMax < 0 {
		wantMax = 0
	}
	if box.ScrollY() != wantMax {
		t.Fatalf("scrollY=%.1f want max %.1f", box.ScrollY(), wantMax)
	}
}

// Incremental layout behind the box must match a full rebuild after a
// scripted edit session (wrap on and off).
func TestBoxMatrix_IncrementalMatchesFull(t *testing.T) {
	for _, mode := range []string{"multi", "wrap"} {
		ed, box := newMatrixBox(t, mode)
		ed.SetTextSimple("hello")
		maxW := 0.0
		if mode == "wrap" {
			maxW = box.FixedWidth - 2*box.Padding()
		}
		check := func(step string) {
			t.Helper()
			got := box.TextLayout()
			want := rendering.BuildTextLayoutEx(ed.GetText(), nil, 16, maxW, 1.2, 0.55, 0, rendering.TextOverflowClip)
			if got.LineCount() != want.LineCount() {
				t.Fatalf("%s %s: lines=%d want %d", mode, step, got.LineCount(), want.LineCount())
			}
			for i := 0; i < got.LineCount(); i++ {
				gs, ge, gw, _, _ := got.Line(i)
				ws, we, ww, _, _ := want.Line(i)
				if gs != ws || ge != we || gw != ww {
					t.Fatalf("%s %s line %d: got [%d,%d] w=%.2f want [%d,%d] w=%.2f", mode, step, i, gs, ge, gw, ws, we, ww)
				}
			}
		}
		for i := 0; i < 10; i++ {
			caretToEnd(ed)
			ed.Insert("\n")
			check(fmt.Sprintf("enter%d", i))
		}
		caretToStart(ed)
		ed.Insert("ab")
		check("head-insert")
		mid := utf16Len(ed.GetText()) / 2
		ed.SetSelection(TextRange{Base: mid, Extent: mid})
		ed.Insert("\n\n")
		check("mid-break")
		ed.DeleteBackward()
		ed.DeleteBackward()
		check("mid-delete")
		if mode == "multi" {
			if got := strings.Join(box.txt.DisplayLines(), "\n"); got != ed.GetText() {
				t.Fatalf("%s: display join mismatch", mode)
			}
		}
	}
}

// Password boxes mask content, keep the caret at the end, and copy masked.
func TestBoxMatrix_PasswordMask(t *testing.T) {
	ed := New()
	box := NewViewportInputBox(ed, 880, 40, 16)
	ed.SetPassword(true)
	ed.AddText("secret")
	if got := box.TextLayout().Text; got != strings.Repeat("•", 6) {
		t.Fatalf("masked display=%q want 6 masks", got)
	}
	if got, want := ed.GetCursorOffset(), len("secret"); got != want {
		t.Fatalf("caret byte=%d want %d", got, want)
	}
	ed.SelectAll()
	if got := ed.Copy(); got != strings.Repeat("•", 6) {
		t.Fatalf("masked copy=%q", got)
	}
}

// Placeholder shows only while empty and unfocused.
func TestBoxMatrix_Placeholder(t *testing.T) {
	ed := New()
	box := NewMultiLineInputBox(ed, 880, 90, 16)
	box.SetPlaceholder("hint")
	if got := box.TextLayout().Text; got != "hint" {
		t.Fatalf("empty unfocused display=%q want hint", got)
	}
	box.onFocusChange(true)
	if got := box.TextLayout().Text; got != "" {
		t.Fatalf("empty focused display=%q want empty", got)
	}
	box.onFocusChange(false)
	ed.AddText("x")
	if got := box.TextLayout().Text; got != "x" {
		t.Fatalf("typed display=%q want x", got)
	}
}

// The vertical record band must follow the scroll: same band live, moved
// band expired (locks the retained-culling wiring without a GPU).
func TestBoxMatrix_YBandFollowsScroll(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
	caretToEnd(ed)
	sy := box.ScrollY()
	_ = sy
	if !box.txt.HasViewportHintY() {
		t.Fatalf("multiline box never set the Y band")
	}
	// Band expiry itself is locked in ui/rendering (TestRecordRenderText_YBandCullsToVisible).
	caretToStart(ed)
	if box.ScrollY() != 0 {
		t.Fatalf("scrollY=%.1f want 0 at home", box.ScrollY())
	}
}

// Long single line must scroll horizontally with the caret visible.
func TestBoxMatrix_SingleLineHorizontalScroll(t *testing.T) {
	ed, box := newMatrixBox(t, "single")
	ed.SetTextSimple(strings.Repeat("ab", 2000))
	caretToEnd(ed)
	if box.ScrollX() <= 0 {
		t.Fatalf("scrollX=%v want >0 at end of 4000-char line", box.ScrollX())
	}
	caretToStart(ed)
	if box.ScrollX() != 0 {
		t.Fatalf("scrollX=%.1f want 0 at home", box.ScrollX())
	}
}

// Scroll/sync cost scale per caret move (report only): documents the drag
// cost curve behind laggy scrolling on big documents.
func BenchmarkBoxSyncCaretToggle(b *testing.B) {
	for _, lines := range []int{200, 2000, 8000} {
		var sb strings.Builder
		for i := 0; i < lines; i++ {
			fmt.Fprintf(&sb, "l%04d\n", i)
		}
		ed := New()
		box := NewMultiLineInputBox(ed, 880, 90, 16)
		ed.SetTextSimple(sb.String())
		n := utf16Len(ed.GetText())
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			toEnd := true
			for i := 0; i < b.N; i++ {
				if toEnd {
					ed.SetSelection(TextRange{Base: n, Extent: n})
				} else {
					ed.SetSelection(TextRange{Base: 0, Extent: 0})
				}
				toEnd = !toEnd
			}
			_ = box
		})
	}
}

// Double click selects the word, triple click the line.
func TestBoxMatrix_ClickWordLineSelect(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello world")
	click := func() {
		box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 12, Y: 12})
		box.OnPointer(input.PointerEvent{Kind: input.PointerUp, X: 12, Y: 12})
	}
	click()
	click()
	sel := ed.GetText()[ed.SelectionRange().Start():ed.SelectionRange().End()]
	_ = sel
	start, end := byteOffsetForUtf16(ed.GetText(), ed.SelectionRange().Start()), byteOffsetForUtf16(ed.GetText(), ed.SelectionRange().End())
	if ed.GetText()[start:end] != "hello" {
		t.Fatalf("double click selected %q want hello", ed.GetText()[start:end])
	}
	click()
	start, end = byteOffsetForUtf16(ed.GetText(), ed.SelectionRange().Start()), byteOffsetForUtf16(ed.GetText(), ed.SelectionRange().End())
	if ed.GetText()[start:end] != "hello world" {
		t.Fatalf("triple click selected %q want full line", ed.GetText()[start:end])
	}
}

// Ctrl+A/C/X/V round-trip through the fallback clipboard.
func TestBoxMatrix_ClipboardRoundTrip(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("clip-me-unique-12345")
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyA, Mods: input.Modifiers{Control: true}})
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyC, Mods: input.Modifiers{Control: true}})
	caretToEnd(ed)
	ed.Insert(" ")
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyV, Mods: input.Modifiers{Control: true}})
	if got := ed.GetText(); got != "clip-me-unique-12345 clip-me-unique-12345" {
		t.Fatalf("paste result=%q", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyA, Mods: input.Modifiers{Control: true}})
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyX, Mods: input.Modifiers{Control: true}})
	if got := ed.GetText(); got != "" {
		t.Fatalf("cut left %q want empty", got)
	}
}

// Type, undo, redo through the box keep text and scroll sane.
func TestBoxMatrix_UndoRedo(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("ab")
	ed.Insert("c")
	if got := ed.GetText(); got != "abc" {
		t.Fatalf("typed=%q", got)
	}
	if !ed.Undo() || ed.GetText() != "ab" {
		t.Fatalf("after undo=%q", ed.GetText())
	}
	if !ed.Redo() || ed.GetText() != "abc" {
		t.Fatalf("after redo=%q", ed.GetText())
	}
	if box.ScrollY() != 0 {
		t.Fatalf("scrollY=%.1f want 0 for short text", box.ScrollY())
	}
}

// Read-only blocks edits but keeps caret motion and selection;
// disabled boxes ignore pointer and keys.
func TestBoxMatrix_ReadOnlyDisabled(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello")
	ed.SetReadOnly(true)
	ed.AddText("x")
	if got := ed.GetText(); got != "hello" {
		t.Fatalf("readonly typed=%q", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyEnd})
	if got := ed.GetCursorOffset(); got != len("hello") {
		t.Fatalf("readonly end byte=%d", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyA, Mods: input.Modifiers{Control: true}})
	if ed.SelectionRange().Start() != 0 || ed.SelectionRange().End() != utf16Len("hello") {
		t.Fatalf("readonly select-all broken")
	}
	ed.SetReadOnly(false)
	box.SetDisabled(true)
	box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 12, Y: 12})
	if ed.SelectionRange().End() != utf16Len("hello") {
		t.Fatalf("disabled pointer changed selection")
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyHome})
	if got := ed.GetCursorOffset(); got != len("hello") {
		t.Fatalf("disabled key moved caret to %d", got)
	}
}

// Home/End/word jumps land on boundaries.
func TestBoxMatrix_HomeEndWordJump(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello world")
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyEnd})
	if got := ed.GetCursorOffset(); got != len("hello world") {
		t.Fatalf("end byte=%d", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyHome})
	if got := ed.GetCursorOffset(); got != 0 {
		t.Fatalf("home byte=%d", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowRight, Mods: input.Modifiers{Control: true}})
	if got := ed.GetCursorOffset(); got != len("hello") {
		t.Fatalf("word jump byte=%d want %d (one word per press)", got, len("hello"))
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowRight, Mods: input.Modifiers{Control: true}})
	if got := ed.GetCursorOffset(); got != len("hello world") {
		t.Fatalf("second word jump byte=%d", got)
	}
}

// Arrow stepping never splits a surrogate pair.
func TestBoxMatrix_EmojiClusterStep(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("a😀b")
	caretToStart(ed)
	var seq []int
	for i := 0; i < 3; i++ {
		box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowRight})
		seq = append(seq, ed.GetCursorOffset())
	}
	want := []int{1, 5, 6}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("step %d byte=%d want %d", i, seq[i], want[i])
		}
	}
}

// IME preedit shows in the box, filters arrows, Escape cancels, commit keeps.
func TestBoxMatrix_IMEPreedit(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("")
	ed.BeginComposing()
	if !ed.UpdateComposingText("nihao", TextRange{Base: 5, Extent: 5}) {
		t.Fatalf("preedit update rejected")
	}
	if got := ed.GetText(); got != "nihao" {
		t.Fatalf("preedit display=%q", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyArrowDown})
	if got := ed.GetCursorOffset(); got != len("nihao") {
		t.Fatalf("composing arrow moved caret to %d", got)
	}
	box.OnKey(input.KeyEvent{Pressed: true, Key: input.KeyEscape})
	if ed.IsComposing() || ed.GetText() != "" {
		t.Fatalf("escape left composing=%v text=%q", ed.IsComposing(), ed.GetText())
	}
	ed.BeginComposing()
	if !ed.UpdateComposingText("nihao", TextRange{Base: 5, Extent: 5}) {
		t.Fatalf("preedit update rejected")
	}
	ed.CommitComposing()
	if ed.IsComposing() || ed.GetText() != "nihao" {
		t.Fatalf("commit left composing=%v text=%q", ed.IsComposing(), ed.GetText())
	}
}

// Upward drag covers from the start without flipping.
func TestBoxMatrix_DragUpSelectsFromStart(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 10) + "TAIL")
	caretToEnd(ed)
	box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 20, Y: 80})
	anchor := ed.SelectionRange().End()
	for i := 0; i < 20; i++ {
		box.OnPointer(input.PointerEvent{Kind: input.PointerMove, X: 0, Y: -200})
	}
	box.OnPointer(input.PointerEvent{Kind: input.PointerUp, X: 0, Y: -200})
	if got := ed.SelectionRange().Start(); got != 0 {
		t.Fatalf("up drag start=%d want 0", got)
	}
	if got := ed.SelectionRange().End(); got != anchor {
		t.Fatalf("up drag end=%d want anchor %d", got, anchor)
	}
}

// Edge auto-scroll ticks advance scroll and selection while dragging.
func TestBoxMatrix_AutoScrollTicks(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple("hello" + strings.Repeat("\n", 30) + "TAIL")
	box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 20, Y: 10})
	before := box.ScrollY()
	for i := 0; i < 5; i++ {
		box.dragLastX, box.dragLastY = 20, 500
		box.doAutoScroll()
	}
	box.OnPointer(input.PointerEvent{Kind: input.PointerUp, X: 20, Y: 500})
	if box.ScrollY() <= before {
		t.Fatalf("auto-scroll did not advance (%.1f)", box.ScrollY())
	}
	if ed.SelectionRange().End() <= 0 {
		t.Fatalf("auto-scroll extended nothing")
	}
}

// Empty box edges: keys, drag, highlights all stay neutral.
func TestBoxMatrix_EmptyBoxEdges(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	for _, k := range []input.Key{input.KeyArrowDown, input.KeyArrowUp, input.KeyHome, input.KeyEnd, input.KeyDelete} {
		box.OnKey(input.KeyEvent{Pressed: true, Key: k})
	}
	box.OnPointer(input.PointerEvent{Kind: input.PointerDown, X: 20, Y: 10})
	box.OnPointer(input.PointerEvent{Kind: input.PointerMove, X: 20, Y: 500})
	box.OnPointer(input.PointerEvent{Kind: input.PointerUp, X: 20, Y: 500})
	if ed.GetCursorOffset() != 0 || box.ScrollY() != 0 || len(box.highlights) != 0 {
		t.Fatalf("empty box state drifted: caret=%d scroll=%.1f hl=%d", ed.GetCursorOffset(), box.ScrollY(), len(box.highlights))
	}
}

// Long nowrap line scrolls horizontally with a visible caret.
func TestBoxMatrix_NowrapHorizontalScroll(t *testing.T) {
	ed, box := newMatrixBox(t, "multi")
	ed.SetTextSimple(strings.Repeat("ab", 2000))
	caretToEnd(ed)
	if box.ScrollX() <= 0 {
		t.Fatalf("scrollX=%v want >0", box.ScrollX())
	}
	lay := box.TextLayout()
	x, _, _, ok := lay.GetOffsetForCaret(ed.GetCursorOffset(), 0, 1.5)
	if !ok {
		t.Fatalf("no caret geometry")
	}
	pad := box.Padding()
	visW := box.FixedWidth - 2*pad
	if x-box.ScrollX() < 0 || x-box.ScrollX() > visW {
		t.Fatalf("caret x=%.1f outside view", x-box.ScrollX())
	}
}

// 2000-line document: counts exact, scroll clamps, caret visible.
func TestBoxMatrix_LongDocumentCorrectness(t *testing.T) {
	ed, box := newMatrixBox(t, "multi")
	var sb strings.Builder
	for i := 0; i < 2000; i++ {
		fmt.Fprintf(&sb, "l%04d\n", i)
	}
	ed.SetTextSimple(sb.String())
	lay := box.TextLayout()
	if lay.LineCount() != 2001 {
		t.Fatalf("lines=%d want 2001", lay.LineCount())
	}
	caretToEnd(ed)
	totalH := lay.LineTop(lay.LineCount()-1) + lay.LineHeight(lay.LineCount()-1)
	pad := box.Padding()
	visH := box.FixedHeight - 2*pad
	if box.ScrollY() != totalH-visH {
		t.Fatalf("scrollY=%.1f want max %.1f", box.ScrollY(), totalH-visH)
	}
}

// Narrowing the box rewraps on layout with no edit in between.
func TestBoxMatrix_ResizeRewrap(t *testing.T) {
	ed, box := newMatrixBox(t, "wrap")
	ed.SetTextSimple(strings.Repeat("hello world ", 40))
	before := box.TextLayout().LineCount()
	box.FixedWidth = 200
	box.Layout(rendering.Loose(1200, 800))
	after := box.TextLayout().LineCount()
	if after <= before {
		t.Fatalf("narrow lines=%d want > %d", after, before)
	}
	if mw := box.FixedWidth - 2*box.Padding(); box.txt.MaxWidth != mw {
		t.Fatalf("maxWidth=%.1f want %.1f", box.txt.MaxWidth, mw)
	}
}
