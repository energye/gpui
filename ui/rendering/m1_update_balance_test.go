package rendering

import (
	"testing"
)

// Pure break edits inside empty runs must keep the hard-line count exact:
// a deleted newline drops exactly one row, no phantom zero-width rows.
func TestLayoutUpdate_EmptyRunBreakBalance(t *testing.T) {
	cases := []struct {
		name      string
		old, new  string
		sp        editSpan
		maxW      float64
		wantLines int
	}{
		{"del-one", "abhello\n\n\n\n", "abhello\n\n\n", editSpan{oldA: 9, oldB: 10, newA: 9, newB: 9, ok: true}, 0, 4},
		{"del-two", "abhello\n\n\n\n", "abhello\n\n", editSpan{oldA: 9, oldB: 11, newA: 9, newB: 9, ok: true}, 0, 3},
		{"insert-two", "abhello\n\n", "abhello\n\n\n\n", editSpan{oldA: 9, oldB: 9, newA: 9, newB: 11, ok: true}, 0, 5},
		{"merge-text", "ab\ncd", "abcd", editSpan{oldA: 2, oldB: 3, newA: 2, newB: 2, ok: true}, 0, 1},
		{"split-text", "abcd", "ab\ncd", editSpan{oldA: 2, oldB: 2, newA: 2, newB: 3, ok: true}, 0, 2},
		{"wrap-del-one", "abhello\n\n\n\n", "abhello\n\n\n", editSpan{oldA: 9, oldB: 10, newA: 9, newB: 9, ok: true}, 864, 4},
	}
	for _, tc := range cases {
		c := newLayoutCache()
		c.update(tc.old, nil, 16, tc.maxW, 1.2, 0.55, 0, TextOverflowClip)
		got := c.updateSpan(tc.new, nil, 16, tc.maxW, 1.2, 0.55, 0, TextOverflowClip, tc.sp)
		if got.LineCount() != tc.wantLines {
			t.Fatalf("%s: lines=%d want %d", tc.name, got.LineCount(), tc.wantLines)
		}
		want := BuildTextLayoutEx(tc.new, nil, 16, tc.maxW, 1.2, 0.55, 0, TextOverflowClip)
		if want.LineCount() != tc.wantLines {
			t.Fatalf("%s: full lines=%d want %d", tc.name, want.LineCount(), tc.wantLines)
		}
		for i := 0; i < got.LineCount(); i++ {
			gs, ge, gw, _, _ := got.Line(i)
			ws, we, ww, _, _ := want.Line(i)
			if gs != ws || ge != we || gw != ww {
				t.Fatalf("%s line %d: got [%d,%d] w=%.2f want [%d,%d] w=%.2f", tc.name, i, gs, ge, gw, ws, we, ww)
			}
		}
		// No duplicated zero-width rows.
		for i := 1; i < got.LineCount(); i++ {
			ps, pe, _, _, _ := got.Line(i - 1)
			s, e, _, _, _ := got.Line(i)
			if ps == s && pe == e && s == e {
				t.Fatalf("%s: duplicated empty row [%d,%d] at %d", tc.name, s, e, i)
			}
		}
	}
}
