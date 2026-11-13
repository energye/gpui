package textinput

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/rendering"
)

// v3cWrappedDoc builds a large document whose logical lines wrap into
// several visual rows at the test box width. Same pressure-data style as
// box_virtual_test.go (strings.Repeat); no golden/standard data is involved.
func v3cWrappedDoc(lines int) string {
	row := strings.Repeat("wrap-line-alpha-beta-", 3) + "\n"
	return strings.Repeat(row, lines)
}

func v3cWrappedBox(t *testing.T) (*Editor, *Box) {
	t.Helper()
	ed := New()
	ed.SetSingleLine(false)
	// 1600 行 × 64B ≈ 100KB > virtualTextMinBytes (64KB).
	ed.SetTextSimple(v3cWrappedDoc(1600))
	b := NewMultiLineInputBox(ed, 160, 600, 14)
	b.SetWrap(true)
	b.sync()
	return ed, b
}

// TestBoxV3c_WrapEntersVirtual locks V3c requirement 1: a large wrapped
// document must still use the viewport model, with the content width as the
// wrap width. Before V3c canVirtual rejects every wrapped box.
func TestBoxV3c_WrapEntersVirtual(t *testing.T) {
	ed, b := v3cWrappedBox(t)
	if !b.virtualOn || !b.txt.Virtual() {
		t.Fatalf("wrapped large doc did not use the virtual viewport: virtualOn=%v virtual=%v", b.virtualOn, b.txt.Virtual())
	}
	// §2.2: the full-layout fallback would materialize the whole document
	// via Editor.GetText/Text; virtual mode must not.
	if ed.text != "" {
		t.Fatalf("wrapped large doc materialized the full editor text (len=%d)", len(ed.text))
	}
	want := b.FixedWidth - 2*b.Padding()
	if b.vtext == nil || math.Abs(b.vtext.MaxWidth()-want) > 0.01 {
		t.Fatalf("wrapped viewport width=%v want=%v", b.vtext.MaxWidth(), want)
	}
	v := b.vtext
	visH := b.FixedHeight - 2*b.Padding()
	first, _ := v.VisibleRange(b.scrollY, visH)
	if vl, ok := v.Line(first); !ok || vl.Rows < 2 {
		t.Fatalf("visible line %d did not wrap: rows=%d (need >1 to exercise variable heights)", first, vl.Rows)
	}
}

// TestBoxV3c_WrapWidthChangeRebuildsView locks the "rebuild the shared view
// by width" part of V3c: after a resize the wrapped viewport must carry the
// new content width, otherwise rows wrap at the old width forever.
func TestBoxV3c_WrapWidthChangeRebuildsView(t *testing.T) {
	_, b := v3cWrappedBox(t)
	if !b.virtualOn {
		t.Fatalf("V3c prerequisite: wrapped large doc must be virtual (virtualOn=%v)", b.virtualOn)
	}
	want0 := b.FixedWidth - 2*b.Padding()
	if math.Abs(b.vtext.MaxWidth()-want0) > 0.01 {
		t.Fatalf("initial wrap width=%v want=%v", b.vtext.MaxWidth(), want0)
	}

	b.FixedWidth = 320
	b.Layout(rendering.Constraints{})

	if !b.virtualOn {
		t.Fatalf("resize dropped virtual mode")
	}
	want1 := b.FixedWidth - 2*b.Padding()
	if math.Abs(b.vtext.MaxWidth()-want1) > 0.01 {
		t.Fatalf("wrap width not rebuilt after resize: got=%v want=%v", b.vtext.MaxWidth(), want1)
	}
	v := b.vtext
	visH := b.FixedHeight - 2*b.Padding()
	first, _ := v.VisibleRange(b.scrollY, visH)
	if _, ok := v.Line(first); !ok {
		t.Fatalf("visible line %d did not materialize after rebuild", first)
	}
}

// TestBoxV3c_AnchorStableAcrossLineCountEdit locks V3c requirement 2: after
// a line-count-changing edit (Enter), the line at the top of the viewport
// must stay put. Before V3c the height map is rebuilt at estimate height
// while scrollY keeps its measured-height value, so the view jumps.
func TestBoxV3c_AnchorStableAcrossLineCountEdit(t *testing.T) {
	ed, b := v3cWrappedBox(t)
	if !b.virtualOn {
		t.Fatalf("V3c prerequisite: wrapped large doc must be virtual (virtualOn=%v)", b.virtualOn)
	}
	buf := ed.ensureBuf()
	v := b.vtext
	visH := b.FixedHeight - 2*b.Padding()

	// Park the caret mid-document so sync scrolls the viewport there.
	k := 800
	u := buf.UTF16OffsetAt(buf.LineStart(k))
	ed.SetSelection(TextRange{Base: u, Extent: u})
	b.sync()

	// Materialize the visible window so the height map holds measured
	// (wrapped, multi-row) heights rather than the one-row estimate.
	v.Prepare(b.scrollY, visH)
	topLine, _ := v.VisibleRange(b.scrollY, visH)
	if vl, ok := v.Line(topLine); !ok || vl.Rows < 2 {
		t.Fatalf("top line %d rows=%d, need a wrapped (>1 row) line to expose an anchor jump", topLine, vl.Rows)
	}

	// Insert a newline one logical line below the top: the edit changes the
	// line count but happens after the anchor, so the top line must not move.
	editLine := topLine + 1
	ue := buf.UTF16OffsetAt(buf.LineStart(editLine))
	ed.SetSelection(TextRange{Base: ue, Extent: ue})
	b.sync()
	topBefore, _ := v.VisibleRange(b.scrollY, visH)
	topBeforeAPI := b.TopVisibleLine()
	linesBefore := buf.LineCount()
	scrollBefore := b.scrollY

	ed.Insert("\n")
	b.sync()

	if got := buf.LineCount(); got != linesBefore+1 {
		t.Fatalf("edit did not add a line: %d -> %d", linesBefore, got)
	}
	topAfter, _ := v.VisibleRange(b.scrollY, visH)
	if topAfter != topBefore {
		t.Fatalf("anchor jumped: top line %d -> %d after Enter at line %d (scrollY %.1f -> %.1f)",
			topBefore, topAfter, editLine, scrollBefore, b.scrollY)
	}
	if got := b.TopVisibleLine(); got != topBeforeAPI {
		t.Fatalf("public TopVisibleLine jumped: %d -> %d after Enter", topBeforeAPI, got)
	}
}

// TestBoxV3c_RebaseKeepsMeasuredHeights locks the P1 root cause found by the
// real window: a line-count edit must only rebuild the changed interval
// (CodeMirror §1.4), not Invalidate() the whole height map. Dropping the
// measured heights changes the Y scale, so the next Prepare re-measures and
// the top logical line drifts by one row (real window: 499963 -> 499964).
func TestBoxV3c_RebaseKeepsMeasuredHeights(t *testing.T) {
	ed, b := v3cWrappedBox(t)
	if !b.virtualOn {
		t.Fatalf("V3c prerequisite: wrapped large doc must be virtual (virtualOn=%v)", b.virtualOn)
	}
	buf := ed.ensureBuf()
	v := b.vtext
	visH := b.FixedHeight - 2*b.Padding()
	k := 800
	u := buf.UTF16OffsetAt(buf.LineStart(k))
	ed.SetSelection(TextRange{Base: u, Extent: u})
	b.sync()
	v.Prepare(b.scrollY, visH)
	measuredBefore := v.MeasuredCount()
	if measuredBefore < 5 {
		t.Fatalf("need >=5 measured lines before the edit, got %d", measuredBefore)
	}
	topBefore := b.TopVisibleLine()

	ed.Insert("\n")
	b.sync()

	if got := v.MeasuredCount(); got < measuredBefore/2 {
		t.Fatalf("line-count edit dropped measured heights: %d -> %d (want >=%d). Rebase must keep measured heights and shift their line numbers, not Invalidate() the whole map.",
			measuredBefore, got, measuredBefore/2)
	}
	if got := b.TopVisibleLine(); got != topBefore {
		t.Fatalf("top line drifted: %d -> %d", topBefore, got)
	}
}

// TestBoxV3c_VisualRowArrowKeys locks V3c requirement 3: in a wrapped
// virtual box, Up/Down must step visual rows inside one logical line, not
// hop to the next logical line. Before V3c the virtual branch calls
// MoveCursorUp/Down (logical lines).
func TestBoxV3c_VisualRowArrowKeys(t *testing.T) {
	ed, b := v3cWrappedBox(t)
	if !b.virtualOn {
		t.Fatalf("V3c prerequisite: wrapped large doc must be virtual (virtualOn=%v)", b.virtualOn)
	}
	buf := ed.ensureBuf()
	v := b.vtext
	k := 800
	u := buf.UTF16OffsetAt(buf.LineStart(k))
	ed.SetSelection(TextRange{Base: u, Extent: u})
	b.sync()
	vl, ok := v.Line(k)
	if !ok || vl.Rows < 2 {
		t.Fatalf("line %d rows=%d, need >=2 visual rows", k, vl.Rows)
	}

	before := ed.GetCursorOffset()
	b.OnKey(input.KeyEvent{Key: input.KeyArrowDown, Pressed: true})
	after := ed.GetCursorOffset()
	if after <= before {
		t.Fatalf("ArrowDown did not move the caret: %d -> %d", before, after)
	}
	if got := buf.LineIndex(after); got != k {
		t.Fatalf("ArrowDown jumped to logical line %d (want %d): visual-row movement must stay inside the wrapped logical line", got, k)
	}

	// Shift+ArrowDown extends the selection by a visual row, same boundary.
	base := ed.SelectionRange()
	b.OnKey(input.KeyEvent{Key: input.KeyArrowDown, Pressed: true, Mods: input.Modifiers{Shift: true}})
	ext := ed.SelectionRange()
	if ext.Base != base.Base {
		t.Fatalf("Shift+ArrowDown changed selection base %d -> %d", base.Base, ext.Base)
	}
	extByte := buf.ByteOffsetForUTF16(ext.Extent)
	if got := buf.LineIndex(extByte); got != k {
		t.Fatalf("Shift+ArrowDown extended to logical line %d (want %d)", got, k)
	}
	// §2.2 hard constraint: the visual-row path must not fall back to
	// Editor.Text() and materialize the whole document.
	if ed.text != "" {
		t.Fatalf("visual-row movement materialized the full editor text (len=%d)", len(ed.text))
	}
}
