package textinput

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/energye/gpui/ui/rendering"
)

func m3Doc(t *testing.T, targetBytes int) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/cjk3000.txt")
	if err != nil || len(raw) == 0 {
		t.Fatalf("read testdata/cjk3000.txt: %v", err)
	}
	base := strings.TrimSpace(string(raw))
	if base == "" {
		t.Fatal("cjk3000.txt empty")
	}
	var sb strings.Builder
	for sb.Len() < targetBytes {
		sb.WriteString(base)
	}
	s := sb.String()
	if len(s) > targetBytes {
		s = s[:targetBytes]
		for len(s) > 0 {
			r, size := utf8.DecodeLastRuneInString(s)
			if r != utf8.RuneError || size > 1 {
				break
			}
			s = s[:len(s)-1]
		}
	}
	if !utf8.ValidString(s) {
		t.Fatal("m3Doc produced invalid UTF-8")
	}
	return s
}

func endCaret(e *Editor) {
	n := utf16Len(e.GetText())
	e.SetSelection(TextRange{Base: n, Extent: n})
}

func TestUndoDelta_Roundtrip_M3(t *testing.T) {
	e := New()
	const steps = 100
	for i := 0; i < steps; i++ {
		endCaret(e)
		if !e.AddText(fmt.Sprintf("x%d ", i)) {
			t.Fatalf("AddText step %d failed", i)
		}
	}
	final := e.GetText()
	for i := steps - 1; i >= 0; i-- {
		if !e.Undo() {
			t.Fatalf("Undo step %d failed", i)
		}
	}
	if e.GetText() != "" {
		t.Fatalf("after 100 undos want empty, got %q", e.GetText())
	}
	for i := 0; i < steps; i++ {
		if !e.Redo() {
			t.Fatalf("Redo step %d failed", i)
		}
	}
	if e.GetText() != final {
		t.Fatalf("after redo mismatch:\nwant %q\n got %q", final, e.GetText())
	}
}

func TestUndoDelta_CompositionSingleStep_M3(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.BeginComposing()
	e.UpdateComposingText("ni", TextRange{Base: 2, Extent: 4})
	e.UpdateComposingText("nihao", TextRange{Base: 2, Extent: 7})
	e.CommitComposing()
	if e.GetText() != "abnihao" {
		t.Fatalf("composed text %q", e.GetText())
	}
	if !e.Undo() {
		t.Fatal("Undo composition failed")
	}
	if e.GetText() != "ab" {
		t.Fatalf("undo composition want %q got %q", "ab", e.GetText())
	}
	if !e.Redo() {
		t.Fatal("Redo composition failed")
	}
	if e.GetText() != "abnihao" {
		t.Fatalf("redo composition want %q got %q", "abnihao", e.GetText())
	}
}

func TestUndoDelta_MemoryCeiling_M3(t *testing.T) {
	doc := m3Doc(t, 100000)
	e := New()
	e.SetTextSimple(doc)
	before := e.HistoryBytes()
	const edits = 100
	for i := 0; i < edits; i++ {
		endCaret(e)
		if !e.AddText("x") {
			t.Fatalf("AddText %d failed", i)
		}
	}
	after := e.HistoryBytes()
	growth := after - before
	if growth > edits*64 {
		t.Fatalf("history growth %d bytes for %d 1-byte edits, want <= %d (delta model)", growth, edits, edits*64)
	}
	if after > len(doc)+edits*64 {
		t.Fatalf("history total %d bytes, doc %d bytes: not incremental", after, len(doc))
	}
	for i := 0; i < 50; i++ {
		endCaret(e)
		if !e.AddText("y") {
			t.Fatalf("AddText cap %d failed", i)
		}
	}
	if got := len(e.history); got > 100 {
		t.Fatalf("history groups %d, want <= 100", got)
	}
	if after150 := e.HistoryBytes(); after150 > len(doc)+150*64 {
		t.Fatalf("history total %d bytes after 150 edits: cap eviction broken", after150)
	}
	for i := 0; i < 60; i++ {
		if !e.Undo() {
			t.Fatalf("Undo cap %d failed", i)
		}
	}
	endCaret(e)
	if !e.AddText("z") {
		t.Fatal("AddText after undo failed")
	}
	if e.Redo() {
		t.Fatal("Redo should fail after new edit cleared redo stack")
	}
}

func BenchmarkPushHistory_M3(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			e := New()
			e.SetTextSimple(doc)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.pushDelta(len(e.text), "", "x",
					e.selection, e.composingRange, e.composing,
					TextRange{Base: 1, Extent: 1}, e.composingRange, e.composing)
			}
		})
	}
}

func BenchmarkEditorSpliceShare_M3(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		b.Run(fmt.Sprintf("splice_%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				e := New()
				e.SetTextSimple(doc)
				endCaret(e)
				b.StartTimer()
				e.AddText("x")
			}
		})
		b.Run(fmt.Sprintf("layout_%d", n), func(b *testing.B) {
			doc := strings.Repeat("a", n)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = rendering.BuildTextLayout(doc, nil, 16, 0, 1.25)
			}
		})
	}
}

func m3MedianDur(ds []time.Duration) time.Duration {
	cp := append([]time.Duration(nil), ds...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

func m3TimePushDelta(t *testing.T, n int) time.Duration {
	t.Helper()
	doc := strings.Repeat("a", n)
	e := New()
	e.SetTextSimple(doc)
	endCaret(e)
	const reps = 500
	start := time.Now()
	for i := 0; i < reps; i++ {
		e.pushDelta(len(e.text), "", "x",
			e.selection, e.composingRange, e.composing,
			TextRange{Base: 1, Extent: 1}, e.composingRange, e.composing)
	}
	return time.Since(start) / reps
}

// TestPushHistory_NoQuadratic_M3 locks M3 complexity: single push cost
// independent of doc length. Old full-snapshot path measured ~34x here.
func TestPushHistory_NoQuadratic_M3(t *testing.T) {
	var a, b []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, m3TimePushDelta(t, 1000))
		b = append(b, m3TimePushDelta(t, 100000))
	}
	d1e3, d1e5 := m3MedianDur(a), m3MedianDur(b)
	t.Logf("pushDelta 1e3=%v 1e5=%v ratio=%.2f", d1e3, d1e5, float64(d1e5)/float64(d1e3))
	if r := float64(d1e5) / float64(d1e3); r > 2 {
		t.Fatalf("pushDelta scales with doc length: ratio=%.1f, gate<=2", r)
	}
}

// TestUndoDelta_Boundaries_M3 covers empty/newline/selection/long-doc edges.
func TestUndoDelta_Boundaries_M3(t *testing.T) {
	e := New()
	if e.Undo() {
		t.Fatal("Undo on empty editor should fail")
	}
	if e.Redo() {
		t.Fatal("Redo on empty editor should fail")
	}
	mustSetText(t, e, "a\nb\n")
	endCaret(e)
	if !e.AddText("c") {
		t.Fatal("AddText after newline failed")
	}
	if e.GetText() != "a\nb\nc" {
		t.Fatalf("newline text %q", e.GetText())
	}
	if !e.Undo() {
		t.Fatal("Undo newline edit failed")
	}
	if e.GetText() != "a\nb\n" {
		t.Fatalf("undo newline want %q got %q", "a\nb\n", e.GetText())
	}
	mustSetText(t, e, "hello")
	e.SetSelection(TextRange{Base: 1, Extent: 1})
	endCaret(e)
	selBefore := e.selection
	if !e.AddText("X") {
		t.Fatal("AddText for selection check failed")
	}
	if !e.Undo() {
		t.Fatal("Undo for selection check failed")
	}
	if e.selection != selBefore {
		t.Fatalf("undo selection want %+v got %+v", selBefore, e.selection)
	}
	doc := m3Doc(t, 100000)
	e2 := New()
	e2.SetTextSimple(doc)
	endCaret(e2)
	if !e2.AddText("tail") {
		t.Fatal("AddText on long doc failed")
	}
	if !e2.Undo() {
		t.Fatal("Undo on long doc failed")
	}
	if e2.GetText() != doc {
		t.Fatal("long-doc undo did not restore original")
	}
	if !e2.Redo() {
		t.Fatal("Redo on long doc failed")
	}
	if e2.GetText() != doc+"tail" {
		t.Fatal("long-doc redo did not reapply")
	}
}

// TestUndoDelta_EmptyCompositionNoHistory_M3: Begin→Commit/End with no edits
// must not leave a phantom undo step (I4): history length unchanged and the
// setup edit stays the only undoable step. Cancelling an open empty
// composition via Undo still restores pre-composition state but records nothing.
func TestUndoDelta_EmptyCompositionNoHistory_M3(t *testing.T) {
	for _, end := range []string{"commit", "end"} {
		e := New()
		mustSetText(t, e, "hello")
		endCaret(e)
		n := len(e.history)
		e.BeginComposing()
		if end == "commit" {
			e.CommitComposing()
		} else {
			e.EndComposing()
		}
		if e.GetText() != "hello" {
			t.Fatalf("%s: empty composition changed text to %q", end, e.GetText())
		}
		if got := len(e.history); got != n {
			t.Fatalf("%s: phantom group: history %d want %d", end, got, n)
		}
		if !e.Undo() {
			t.Fatalf("%s: Undo of setup text failed", end)
		}
		if e.GetText() != "" {
			t.Fatalf("%s: want empty after undo, got %q", end, e.GetText())
		}
		if !e.Redo() {
			t.Fatalf("%s: Redo of setup text failed", end)
		}
	}

	e2 := New()
	mustSetText(t, e2, "ab")
	e2.SetSelection(TextRange{Base: 2, Extent: 2})
	e2.BeginComposing()
	if !e2.Undo() {
		t.Fatal("Undo of open empty composition should cancel it")
	}
	if e2.GetText() != "ab" || e2.IsComposing() {
		t.Fatalf("cancel failed: text=%q composing=%v", e2.GetText(), e2.IsComposing())
	}
	if e2.Redo() {
		t.Fatal("Redo after cancelling empty composition should fail")
	}
}

// TestUndoDelta_NoDocRetention_M3: stored deltas must not pin whole document
// backing arrays (Go substrings share backing). 100 single-char deletions on
// a 1e5 doc would otherwise retain ~1e7 bytes while HistoryBytes reports ~100.
func TestUndoDelta_NoDocRetention_M3(t *testing.T) {
	doc := m3Doc(t, 100000)
	docBase := uintptr(unsafe.Pointer(unsafe.StringData(doc)))
	docEnd := docBase + uintptr(len(doc))
	inDoc := func(s string) bool {
		if len(s) == 0 {
			return false
		}
		p := uintptr(unsafe.Pointer(unsafe.StringData(s)))
		return p >= docBase && p < docEnd
	}
	e := New()
	e.SetTextSimple(doc)
	runtime.GC()
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 100; i++ {
		cur := e.GetText()
		r, size := utf8.DecodeLastRuneInString(cur)
		if r == utf8.RuneError && size <= 1 {
			t.Fatal("doc produced invalid tail")
		}
		e.SetTextSimple(cur[:len(cur)-size])
	}
	for gi, g := range e.history {
		for _, op := range g.ops {
			if len(op.deleted) > 0 && len(op.deleted) <= 64 && inDoc(op.deleted) {
				t.Fatalf("group %d deleted aliases full doc array", gi)
			}
			if len(op.inserted) > 0 && len(op.inserted) <= 64 && inDoc(op.inserted) {
				t.Fatalf("group %d inserted aliases full doc array", gi)
			}
		}
	}
	runtime.GC()
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	// Signed diff: GC may collect the pre-edit doc, making after < before.
	if growth := int64(after.HeapAlloc) - int64(before.HeapAlloc); growth > 2<<20 {
		t.Fatalf("heap grew %d bytes for 100 1-rune deletions, want < 2MB (delta model)", growth)
	}
	for len(e.history) > 0 {
		if !e.Undo() {
			t.Fatal("Undo during retention drain failed")
		}
	}
	if e.GetText() != doc {
		t.Fatal("undo drain did not restore original doc")
	}
}
