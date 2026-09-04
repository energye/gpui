package text

import (
	"strings"
	"testing"
)

// 红灯单测：单行击键增量分段必须与全量 SegmentText 逐字节一致。
// 先写失败（SegmentReuse 未实现），再做引擎层。
func TestSegmentReuse_KeystrokeEquiv(t *testing.T) {
	const n = 50000
	base := strings.Repeat("a世", n/2)
	mid := len(base) / 2
	edited := base[:mid] + "X" + base[mid+1:]
	oldSegs := SegmentText(base)
	if len(oldSegs) == 0 {
		t.Fatalf("old segs empty")
	}
	oldA, oldB, newA, newB := mid, mid+1, mid, mid+1
	got := SegmentReuse(base, oldSegs, edited, oldA, oldB, newA, newB)
	want := SegmentText(edited)
	if len(got) != len(want) {
		t.Fatalf("reuse segs=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i].Start != want[i].Start || got[i].End != want[i].End ||
			got[i].Direction != want[i].Direction || got[i].Script != want[i].Script ||
			got[i].Level != want[i].Level || got[i].Text != want[i].Text {
			t.Fatalf("seg %d mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestSegmentReuse_ScriptChangeFallbackEquiv(t *testing.T) {
	base := "aaa世世世bbb"
	oldSegs := SegmentText(base)
	// 把中间 latin 改成 Han，触发合并：增量允许回退，但结果必须与全量一致。
	edited := "aaa世世世世bbb"
	_ = edited
	edited2 := "aaa世世世bbb"
	// 构造脚本变化：a->世
	b2 := "a" + "世" + "aa世世世bbb"
	_ = b2
	_ = oldSegs
	_ = edited2
	newText := strings.Replace(base, "aaa", "世世世", 1)
	oldA, oldB, newA, newB := 0, 3, 0, 9
	got := SegmentReuse(base, SegmentText(base), newText, oldA, oldB, newA, newB)
	want := SegmentText(newText)
	if len(got) != len(want) {
		t.Fatalf("reuse segs=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seg %d mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestSegmentReuse_RTLFallbackEquiv(t *testing.T) {
	base := "hello world hello"
	oldSegs := SegmentText(base)
	newText := "hello مرحبا hello"
	got := SegmentReuse(base, oldSegs, newText, 6, 11, 6, 11)
	want := SegmentText(newText)
	if len(got) != len(want) {
		t.Fatalf("reuse segs=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("seg %d mismatch: got %+v want %+v", i, got[i], want[i])
		}
	}
}
