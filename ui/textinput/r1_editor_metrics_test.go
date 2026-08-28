package textinput

import (
	"strings"
	"testing"
	"time"
)

// Shared source strings — same values used by ui_wr_ime_r1_editor window (G1).
const (
	r1MixedCJK      = "你好Hello世界Flutter输入法IME测试abcdefghijklmnopqrstuvwxyz0123456789"
	r1SentinelText  = "hello"
	r1SurrogateText = "a😀b𝄞c" // 2 surrogates (U+1F600, U+1D11E) => 2 units each
	r1LongBase      = "你好Hello世界"
)

func TestR1_SentinelDoubleSetText(t *testing.T) {
	e := New()
	// First SetText with -1/-1 should normalize to 0,0 per Flutter.
	ok := e.SetText(r1SentinelText, TextRange{Base: -1, Extent: -1}, TextRange{Base: -1, Extent: -1}, AffinityDownstream)
	if !ok {
		t.Fatalf("first SetText should report changed")
	}
	if e.GetText() != r1SentinelText || e.selection.Base != 0 || e.selection.Extent != 0 {
		t.Fatalf("sentinel normalized failed text=%q sel=%v", e.GetText(), e.selection)
	}
	// Second SetText with same content but explicit 0,0 should dedup via ShouldSkip?
	// Editor always updates; framework side dedup is ShouldSkipFrameworkUpdate.
	e.lastFrameworkText = r1SentinelText
	e.lastFrameworkSel = TextRange{Base: 0, Extent: 0}
	e.lastFrameworkComp = TextRange{}
	if !e.ShouldSkipFrameworkUpdate(r1SentinelText, TextRange{Base: 0, Extent: 0}, TextRange{}) {
		t.Fatalf("ShouldSkipFrameworkUpdate should be true for identical")
	}
}

func TestR1_MixedCJK200(t *testing.T) {
	// 50+ chars混排，模拟 cjk3000.txt 抽200字（同源于窗口的 cjkMixed 长串）
	long := strings.Repeat(r1MixedCJK, 4) // ~200 chars
	e := New()
	n := utf16Len(long)
	e.SetText(long, TextRange{Base: n, Extent: n}, TextRange{}, 0)
	if e.GetText() != long {
		t.Fatalf("mixed set failed")
	}
	// paste middle: insert at 10
	e.SetSelection(TextRange{Base: 10, Extent: 10})
	e.AddText("你好Hello")
	if !strings.Contains(e.GetText(), "你好Hello") {
		t.Fatalf("paste not found")
	}
	// verify utf16 roundtrip for GetCursorOffset
	off := e.GetCursorOffset()
	back := byteOffsetForUtf16(e.GetText(), e.selection.Extent)
	if off != back {
		t.Fatalf("cursor byte mismatch %d vs %d", off, back)
	}
}

func TestR1_SurrogateCount1(t *testing.T) {
	e := New()
	mustSetText(t, e, r1SurrogateText)
	// DeleteSurrounding(-1,1) should delete one codepoint (surrogate counted as 1 via utf16 2 units)
	// Position caret after 'a' (1 unit)
	e.SetSelection(TextRange{Base: 1, Extent: 1})
	// Delete forward one codepoint (😀 计 1，内部按 rune→utf16 换算 2 units)
	if !e.DeleteSurrounding(0, 1) {
		t.Fatalf("DeleteSurrounding forward surrogate failed")
	}
	if e.GetText() != "ab𝄞c" {
		t.Fatalf("after delete got %q want ab𝄞c", e.GetText())
	}
	// Backspace surrogate at end
	mustSetText(t, e, "a𝄞")
	e.SetSelection(TextRange{Base: 3, Extent: 3}) // a=1, 𝄞=2 => 3
	e.Backspace()
	if e.GetText() != "a" {
		t.Fatalf("backspace surrogate %q", e.GetText())
	}
}

func TestR1_SurroundingCenter4(t *testing.T) {
	// 4000 center 4 anchors: 0, 1/4, 3/4, end — same as window's 4 probes.
	base := strings.Repeat("a", 5000)
	anchors := []int{0, 1250, 3750, 5000}
	for _, cur := range anchors {
		tr, newCur := TruncateSurrounding(base, cur)
		if len(tr) > 3999 {
			t.Fatalf("anchor %d len %d >3999", cur, len(tr))
		}
		if newCur < 0 || newCur > len(tr) {
			t.Fatalf("anchor %d newCur %d out of range", cur, newCur)
		}
		if cur > 0 && cur < 5000 && tr[newCur] != 'a' {
			t.Fatalf("anchor %d cursor not centered", cur)
		}
	}
	// Multibyte centered: cursor inside CJK
	mb := strings.Repeat("你", 2500) // each 3 bytes => 7500 bytes >4000
	curByte := len("你") * 1250
	tr, newCur := TruncateSurrounding(mb, curByte)
	if len(tr) > 3999 {
		t.Fatalf("mb truncate len %d", len(tr))
	}
	if newCur < 0 || newCur > len(tr) {
		t.Fatalf("mb newCur %d", newCur)
	}
}

func TestR1_BatchNest3(t *testing.T) {
	e := New()
	calls := 0
	e.OnChange = func() { calls++ }
	e.BeginBatchEdit()
	e.BeginBatchEdit()
	e.BeginBatchEdit()
	e.AddText("a")
	e.SetComposingRange(TextRange{Base: 0, Extent: 1}, 1) // should be allowed even in batch depth>0? BeginComposing sets true
	// Actually composing requires not password; batchDepth>0 still allows state changes but suppress OnChange
	e.AddText("b")
	if calls != 0 {
		t.Fatalf("batch should suppress, calls=%d", calls)
	}
	e.EndBatchEdit()
	if calls != 0 {
		t.Fatalf("still nested, calls=%d", calls)
	}
	e.EndBatchEdit()
	if calls != 0 {
		t.Fatalf("still nested 1, calls=%d", calls)
	}
	e.EndBatchEdit()
	if calls != 1 {
		t.Fatalf("only final EndBatch should fire once, calls=%d", calls)
	}
	epoch1 := e.Epoch()
	e.BeginBatchEdit()
	e.AddText("c")
	e.EndBatchEdit()
	if e.Epoch() <= epoch1 {
		t.Fatalf("epoch not bumped after batch")
	}
}

func TestR1_Utf16Roundtrip(t *testing.T) {
	cases := []string{
		"你好",
		"Hello",
		"a😀b𝄞c",
		r1MixedCJK,
		strings.Repeat("한", 10), // Hangul
	}
	for _, s := range cases {
		n := utf16Len(s)
		for _, off := range []int{0, n / 2, n} {
			b := byteOffsetForUtf16(s, off)
			// roundtrip via utf16 length of prefix
			prefix := s[:b]
			if utf16Len(prefix) != off && off != n {
				// Allow surrogate boundary: if off lands inside surrogate, we snapped to start
				// Check that decoding prefix and re-encoding gives consistent
				t.Fatalf("roundtrip %q off %d prefix %q len %d", s, off, prefix, utf16Len(prefix))
			}
		}
		// GetCursorOffset roundtrip via Editor
		e := New()
		e.SetText(s, TextRange{Base: n, Extent: n}, TextRange{}, 0)
		if e.GetCursorOffset() != len(s) {
			t.Fatalf("GetCursorOffset %q got %d want %d", s, e.GetCursorOffset(), len(s))
		}
	}
}

func TestR1_UpdateNoop(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.BeginComposing()
	// collapsed composing + empty text + collapsed sel should be no-op
	before := e.GetText()
	ok := e.UpdateComposingText("", TextRange{Base: 2, Extent: 2})
	if ok {
		t.Fatalf("empty collapsed UpdateComposingText should be no-op")
	}
	if e.GetText() != before {
		t.Fatalf("text changed on no-op")
	}
	// EndComposing should clear
	e.UpdateComposingText("xy", TextRange{Base: 2, Extent: 4})
	e.EndComposing()
	if e.ComposeActive() {
		t.Fatalf("EndComposing should clear")
	}
	if e.GetText() != "ab" {
		t.Fatalf("EndComposing deleted preedit, got %q", e.GetText())
	}
}

func TestR1_EditableReject(t *testing.T) {
	e := New()
	mustSetText(t, e, "abcd")
	e.BeginComposing()
	e.UpdateComposingText("xy", TextRange{Base: 4, Extent: 6})
	// composing range is [4,6), selection at 6
	if e.EditableRange().Start() != 4 || e.EditableRange().End() != 6 {
		t.Fatalf("EditableRange %v", e.EditableRange())
	}
	// SetSelection non-collapsed outside composing should fail
	if e.SetSelection(TextRange{Base: 0, Extent: 2}) {
		t.Fatalf("SetSelection outside editable should be clamped or fail")
	}
	// Actually spec: SetSelection non-collapsed while composing => false
	if e.SetSelection(TextRange{Base: 4, Extent: 5}) {
		// This is inside composing but non-collapsed => should fail per spec
		t.Fatalf("non-collapsed while composing should fail")
	}
	// Move should stay inside
	if !e.SetSelection(TextRange{Base: 4, Extent: 4}) {
		t.Fatalf("collapsed inside should succeed")
	}
	// AddText outside editableRange should fail
	e.SetSelection(TextRange{Base: 4, Extent: 4})
	// Try to move to 0 and AddText — SetSelection will clamp to 2, so AddText will succeed inside
	// To test reject, set selection to 2 (inside) then try AddText with composing active — it replaces composing range, so allowed.
	// Instead test readOnly path
	e.SetReadOnly(true)
	if e.AddText("z") {
		t.Fatalf("readOnly AddText should fail")
	}
	if e.UpdateComposingText("z", TextRange{Base: 2, Extent: 3}) {
		t.Fatalf("readOnly UpdateComposingText should fail")
	}
}

func TestR1_AddCodePoint(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	if !e.AddCodePoint('😀') {
		t.Fatalf("AddCodePoint failed")
	}
	if e.GetText() != "ab😀" {
		t.Fatalf("AddCodePoint text %q", e.GetText())
	}
}

func TestR1_BatchShouldSkipDedup(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello")
	e.lastFrameworkText = "hello"
	e.lastFrameworkSel = TextRange{Base: 5, Extent: 5}
	e.lastFrameworkComp = TextRange{}
	if !e.ShouldSkipFrameworkUpdate("hello", TextRange{Base: 5, Extent: 5}, TextRange{}) {
		t.Fatalf("should skip identical")
	}
	if e.ShouldSkipFrameworkUpdate("hello!", TextRange{Base: 5, Extent: 5}, TextRange{}) {
		t.Fatalf("should not skip different text")
	}
}

func TestR1_Long5000Perf(t *testing.T) {
	long := strings.Repeat(r1LongBase, 500) // ~5000 runes (actually ~12*500=6000)
	e := New()
	start := time.Now()
	e.SetText(long, TextRange{Base: utf16Len(long), Extent: utf16Len(long)}, TextRange{}, 0)
	if time.Since(start) > 100*time.Millisecond {
		t.Fatalf("SetText 5000 took too long")
	}
	start = time.Now()
	for i := 0; i < 100; i++ {
		e.MoveCursorBack()
	}
	if time.Since(start) > 50*time.Millisecond {
		t.Fatalf("MoveCursorBack 100 steps too slow")
	}
}

func TestR1_AffinityCarry(t *testing.T) {
	e := New()
	e.SetText("a\nb", TextRange{Base: 1, Extent: 1, Affinity: AffinityDownstream}, TextRange{}, AffinityDownstream)
	if e.selection.Affinity != AffinityDownstream {
		t.Fatalf("affinity not carried")
	}
	e.SetText("a\nb", TextRange{Base: 1, Extent: 1, Affinity: AffinityUpstream}, TextRange{}, AffinityUpstream)
	if e.selection.Affinity != AffinityUpstream {
		t.Fatalf("affinity upstream not carried")
	}
}
