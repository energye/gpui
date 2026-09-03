package rendering

import (
	"math"
	"testing"
)

// TestLineIndex_M1锁M1第5项行索引:偏移→行号/行区间二分、越界钳制、
// CRLF按字节处理、索引与行表失配自检.
func TestLineIndex_M1(t *testing.T) {
	lines := []TextLayoutLine{
		{StartByte: 0, EndByte: 1, Height: 10},
		{StartByte: 2, EndByte: 4, Height: 10},
		{StartByte: 5, EndByte: 8, Height: 10},
	}
	idx := buildLineIndex(lines, 14, 1.2)
	if !idx.valid(lines) {
		t.Fatalf("同份行表valid应为true")
	}
	if idx.valid(lines[:2]) {
		t.Fatalf("行数变化后valid应为false")
	}
	cases := []struct {
		off  int
		want int
	}{
		{-5, 0},
		{0, 0},
		{1, 0},
		{2, 1},
		{4, 1},
		{5, 2},
		{8, 2},
		{100, 2},
	}
	for _, c := range cases {
		if got := idx.rowForOffset(c.off); got != c.want {
			t.Fatalf("rowForOffset(%d)=%d want %d", c.off, got, c.want)
		}
	}
	// [1,5)与第0行[0,1)无交集(字节1是换行符本身,不属任何行),只交第1行.
	lo, hi := idx.rowRangeForSpan(1, 5)
	if lo != 1 || hi != 1 {
		t.Fatalf("rowRangeForSpan(1,5)=[%d,%d] want [1,1]", lo, hi)
	}
	// CRLF: \r只是普通字节,不得错位也不得崩.
	cr := []TextLayoutLine{
		{StartByte: 0, EndByte: 3, Height: 10}, // "a\r\n" 去\n后行内含\r
		{StartByte: 4, EndByte: 5, Height: 10},
	}
	cidx := buildLineIndex(cr, 14, 1.2)
	if got := cidx.rowForOffset(2); got != 0 {
		t.Fatalf("CRLF行内偏移应归第0行,got %d", got)
	}
	if got := cidx.rowForOffset(4); got != 1 {
		t.Fatalf("CRLF后行偏移应归第1行,got %d", got)
	}
}

// TestRowForY_RV锁收敛:RowForY(二分)必须与旧线性扫逐值一致,
// 含边界/负数/超大/NaN.旧循环在 input_box.go 逐行比LineTop.
func TestRowForY_RV(t *testing.T) {
	lay := BuildTextLayout("aaa\nbb\nc\ndddd\nee", nil, 14, 0, 1.2)
	loop := func(y float64) int {
		idx := 0
		for i := 0; i < lay.LineCount(); i++ {
			top := lay.LineTop(i)
			ht := lay.LineHeight(i)
			if y >= top-0.01 && y < top+ht-0.01 {
				idx = i
				break
			}
		}
		return idx
	}
	total := lay.LineTop(lay.LineCount() - 1)
	lastH := lay.LineHeight(lay.LineCount() - 1)
	ys := []float64{-100, -0.02, -0.01, 0, 0.5, total - 0.011, total - 0.01, total, total + lastH - 0.011, total + lastH - 0.009, total + lastH, total + 1000, math.NaN(), math.Inf(1), math.Inf(-1)}
	for d := -50.0; d < total+lastH+50; d += 0.37 {
		ys = append(ys, d)
	}
	for _, y := range ys {
		if got, want := lay.RowForY(y), loop(y); got != want {
			t.Fatalf("RowForY(%v)=%d,旧循环=%d", y, got, want)
		}
	}
}
func TestCaretPrefixSum_M1(t *testing.T) {
	doc := "hello世界\nfoo bar\n尾行 end"
	lay := BuildTextLayout(doc, nil, 14, 0, 1.2)
	for i := 0; i < lay.LineCount(); i++ {
		for _, c := range lay.LineCarets(i) {
			x, ok := lay.CaretAt(i, c.ByteOff)
			if !ok {
				t.Fatalf("行%d 偏移%d CaretAt失败", i, c.ByteOff)
			}
			if x != c.X {
				t.Fatalf("行%d 偏移%d: CaretAt=%v 表=%v", i, c.ByteOff, x, c.X)
			}
		}
	}
}
