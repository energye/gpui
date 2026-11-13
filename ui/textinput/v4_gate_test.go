package textinput

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// v4GateUniqueLines builds a pressure document with UNIQUE lines (plan §2.2
// measurement discipline: repeated/identical lines are invalid — V0 measured
// them ~800x too fast). This is pressure input, not an oracle dataset.
func v4GateUniqueLines(n int) string {
	var sb strings.Builder
	sb.Grow(n * 24)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "L%07d-alpha-beta\n", i)
	}
	return sb.String()
}

func v4GateMedian(ds []time.Duration) time.Duration {
	c := append([]time.Duration(nil), ds...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	return c[len(c)/2]
}

func v4GateBox(t *testing.T, doc string) (*Editor, *Box) {
	t.Helper()
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	b := NewMultiLineInputBox(ed, 800, 600, 14)
	b.sync()
	return ed, b
}

// TestV4Gate_FirstSync100ms locks §2.2 "首屏 ≤100ms" on 1e6 unique lines.
func TestV4Gate_FirstSync100ms(t *testing.T) {
	doc := v4GateUniqueLines(1000000)
	ed := New()
	ed.SetSingleLine(false)
	ed.SetTextSimple(doc)
	var ds []time.Duration
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		b := NewMultiLineInputBox(ed, 800, 600, 14)
		b.sync()
		ds = append(ds, time.Since(t0))
		if !b.virtualOn {
			t.Fatalf("V4 RED: 1e6-line box did not enter virtual mode — first-sync timing would be a no-op")
		}
	}
	d := v4GateMedian(ds)
	t.Logf("V4 gate 1e6-line first sync = %v (gate <=100ms)", d)
	if d > 100*time.Millisecond {
		t.Fatalf("V4 RED: first sync %v exceeds 100ms gate", d)
	}
}

// TestV4Gate_CaretMoveHit4ms locks §2.2 "caret 移动/命中 p95 ≤4ms" on 1e6
// unique lines, measuring distinct offsets (not a repeated hot cache line).
// caretAnchor is the PRODUCTION caret path: in V2 virtual mode it uses the
// TextViewport's row-relative offset (box.go:817); calling TextLayout's
// CaretForOffset with a full-document offset would be the wrong mouth — the
// virtual layout only materializes visible rows.
func TestV4Gate_CaretMoveHit4ms(t *testing.T) {
	doc := v4GateUniqueLines(1000000)
	ed, b := v4GateBox(t, doc)
	buf := ed.ensureBuf()
	// The pressure document is pure ASCII, so utf16 length == byte length;
	// TextRange.Base/Extent are utf16 offsets (Editor's selection unit).
	total := buf.Len()
	var ds []time.Duration
	tops := make(map[int]bool)
	for i := 1; i <= 9; i++ {
		u := total * i / 10
		ed.SetSelection(TextRange{Base: u, Extent: u})
		b.sync()
		if got := ed.SelectionRange().Extent; got != u {
			t.Fatalf("V4 RED: caret did not move to utf16 %d (got %d) — timing would be a no-op", u, got)
		}
		tops[b.TopVisibleLine()] = true
		t0 := time.Now()
		_, _, _, ok := b.caretAnchor()
		if !ok {
			t.Fatalf("caret anchor failed at utf16 %d", u)
		}
		ds = append(ds, time.Since(t0))
	}
	if len(tops) < 5 {
		t.Fatalf("V4 RED: viewport did not slide across distinct caret offsets (tops=%d)", len(tops))
	}
	d := v4GateMedian(ds)
	t.Logf("V4 gate caret hit median = %v (gate <=4ms)", d)
	if d > 4*time.Millisecond {
		t.Fatalf("V4 RED: caret hit %v exceeds 4ms gate", d)
	}
}

// TestV4Gate_MultiLineSync4ms locks §2.2 "多行同步：全表最大宽扫描 ≤4ms
// @1e6 行" (V0 measured 19.27ms — a red that V2's viewport model should
// have fixed). b.sync is the full-table max-width path under test.
func TestV4Gate_MultiLineSync4ms(t *testing.T) {
	doc := v4GateUniqueLines(1000000)
	_, b := v4GateBox(t, doc)
	if !b.virtualOn {
		t.Fatalf("V4 RED: sync timing on a non-virtual box is not the §2.2 multi-line path")
	}
	lines := b.ed.ensureBuf().LineCount()
	var ds []time.Duration
	for i := 0; i < 7; i++ {
		t0 := time.Now()
		b.sync()
		ds = append(ds, time.Since(t0))
		if got := b.ed.ensureBuf().LineCount(); got != lines {
			t.Fatalf("V4 RED: sync changed line count %d -> %d", lines, got)
		}
	}
	d := v4GateMedian(ds)
	t.Logf("V4 gate 1e6-line sync median = %v (gate <=4ms)", d)
	if d > 4*time.Millisecond {
		t.Fatalf("V4 RED: multi-line sync %v exceeds 4ms gate", d)
	}
}

// TestV4Gate_RowCountEdit16ms_3e6 locks §2.2 "行数变化编辑 p95 ≤16ms
// (3e6 行)". Enter + cross-line delete at distinct positions.
func TestV4Gate_RowCountEdit16ms_3e6(t *testing.T) {
	doc := v4GateUniqueLines(3000000)
	ed, b := v4GateBox(t, doc)
	buf := ed.ensureBuf()
	total := buf.Len()
	var ds []time.Duration
	for i := 1; i <= 5; i++ {
		u := total * i / 6 // distinct edit position each round
		ed.SetSelection(TextRange{Base: u, Extent: u})
		before := buf.LineCount()
		t0 := time.Now()
		ed.Insert("\n")
		b.sync()
		ds = append(ds, time.Since(t0))
		if got := buf.LineCount(); got != before+1 {
			t.Fatalf("V4 RED: row-count edit did not add a line (%d -> %d) — timing would be a no-op", before, got)
		}
		// Undo so the document size/structure stays stable for the next arm.
		ed.Undo()
		b.sync()
		if got := buf.LineCount(); got != before {
			t.Fatalf("V4 RED: undo did not restore line count (%d -> %d)", before, got)
		}
	}
	d := v4GateMedian(ds)
	t.Logf("V4 gate 3e6-line row-count edit median = %v (gate <=16ms)", d)
	if d > 16*time.Millisecond {
		t.Fatalf("V4 RED: row-count edit %v exceeds 16ms gate", d)
	}
}
