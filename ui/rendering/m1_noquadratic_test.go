package rendering

import (
	"sort"
	"strings"
	"testing"
	"time"
)

// M1-a red-light tests:面2查询去O(n²).
// CaretForOffset/BoxesForRange当前走全表遍历+行内线性扫,行数×10耗时×100.
// 本文件先红后绿:下面的比值门禁在改动前必须红.
// medianOf5取中位数抗负载抖动(同仓TestCaretBuild_NoQuadratic做法),阈值不动.

func m1ManyLinesDoc(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line content here"
	}
	return strings.Join(lines, "\n")
}

func m1TimeCaretForOffsetEnd(t *testing.T, n int) time.Duration {
	t.Helper()
	lay := BuildTextLayout(m1ManyLinesDoc(n), nil, 14, 0, 1.2)
	if lay.LineCount() != n {
		t.Fatalf("want %d lines got %d", n, lay.LineCount())
	}
	off := len(lay.Text)
	start := time.Now()
	reps := 5
	for i := 0; i < reps; i++ {
		if _, _, ok := lay.CaretForOffset(off); !ok {
			t.Fatalf("CaretForOffset failed")
		}
	}
	return time.Since(start) / time.Duration(reps)
}

func m1Median(ds []time.Duration) time.Duration {
	cp := append([]time.Duration(nil), ds...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	return cp[len(cp)/2]
}

func TestCaretForOffset_NoQuadratic_M1(t *testing.T) {
	var a, b []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, m1TimeCaretForOffsetEnd(t, 200))
		b = append(b, m1TimeCaretForOffsetEnd(t, 2000))
	}
	d200, d2000 := m1Median(a), m1Median(b)
	t.Logf("CaretForOffset 200行=%v 2000行=%v 比值=%.2f", d200, d2000, float64(d2000)/float64(d200))
	if r := float64(d2000) / float64(d200); r > 3 {
		t.Fatalf("CaretForOffset疑似O(n²):行数×10耗时×%.1f,门禁≤3", r)
	}
}

func m1TimeBoxesForRangeFull(t *testing.T, n int) time.Duration {
	t.Helper()
	lay := BuildTextLayout(m1ManyLinesDoc(n), nil, 14, 0, 1.2)
	start := time.Now()
	reps := 3
	for i := 0; i < reps; i++ {
		if boxes := lay.BoxesForRange(0, len(lay.Text)); len(boxes) != n {
			t.Fatalf("want %d boxes got %d", n, len(boxes))
		}
	}
	return time.Since(start) / time.Duration(reps)
}

func m1TimeBoxesForRangeTail(t *testing.T, n int) time.Duration {
	t.Helper()
	lay := BuildTextLayout(m1ManyLinesDoc(n), nil, 14, 0, 1.2)
	tail, _, _, _, _ := lay.Line(n - 2)
	end := len(lay.Text)
	start := time.Now()
	reps := 20
	for i := 0; i < reps; i++ {
		if boxes := lay.BoxesForRange(tail, end); len(boxes) != 2 {
			t.Fatalf("want 2 boxes got %d", len(boxes))
		}
	}
	return time.Since(start) / time.Duration(reps)
}

func TestBoxesForRange_NoQuadratic_M1(t *testing.T) {
	var a, b []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, m1TimeBoxesForRangeTail(t, 200))
		b = append(b, m1TimeBoxesForRangeTail(t, 2000))
	}
	d200, d2000 := m1Median(a), m1Median(b)
	t.Logf("BoxesForRange末小段 200行=%v 2000行=%v 比值=%.2f", d200, d2000, float64(d2000)/float64(d200))
	if r := float64(d2000) / float64(d200); r > 3 {
		t.Fatalf("BoxesForRange疑似O(n²):行数×10耗时×%.1f,门禁≤3", r)
	}
}

func TestBoxesForRange_FullLinear_M1(t *testing.T) {
	// 用大N对(2000/20000)避开小样本固定开销抖动,中位数抗负载.
	var a, b []time.Duration
	for i := 0; i < 5; i++ {
		a = append(a, m1TimeBoxesForRangeFull(t, 2000))
		b = append(b, m1TimeBoxesForRangeFull(t, 20000))
	}
	d2000, d20000 := m1Median(a), m1Median(b)
	t.Logf("BoxesForRange全文 2000行=%v 20000行=%v 比值=%.2f(输出本身O(n),线性即达标)", d2000, d20000, float64(d20000)/float64(d2000))
	if r := float64(d20000) / float64(d2000); r > 15 {
		t.Fatalf("BoxesForRange全文查询超线性:行数×10耗时×%.1f,门禁≤15", r)
	}
}

func TestLineTop_PrefixSum_M1(t *testing.T) {
	lay := BuildTextLayout("a\nbb\nccc", nil, 14, 0, 1.2)
	if lay.LineCount() != 3 {
		t.Fatalf("want 3 lines got %d", lay.LineCount())
	}
	h0 := lay.LineHeight(0)
	if top1, top2 := lay.LineTop(1), lay.LineTop(2); top1 != h0 || top2 != 2*h0 {
		t.Fatalf("LineTop应为行高前缀和:top1=%v top2=%v h=%v", top1, top2, h0)
	}
}

// m1TimeKeystroke测生产路径单次改字重排:RenderText.SetTextSpan+TextLayout
// (与InputBox.sync同路).每次击键后必布局,区间恒有效;同时断言50次全部走
// span增量(零回退),锁住测的是快径而非全量重建.
func m1TimeKeystroke(t *testing.T, n, perLine int, w float64) time.Duration {
	t.Helper()
	base := m1BenchDoc(n, perLine)
	mid := len(base) / 2
	for mid < len(base) && base[mid] == '\n' {
		mid++
	}
	edited := base[:mid] + "X" + base[mid+1:]
	rt := NewRenderText(base)
	rt.MaxWidth = w
	_ = rt.TextLayout()
	hits0, miss0 := rt.lcache.live.spansHit, rt.lcache.live.spansMiss
	toEdited := true
	const reps = 50
	start := time.Now()
	for i := 0; i < reps; i++ {
		if toEdited {
			rt.SetTextSpan(edited, mid, mid+1, mid, mid+1)
		} else {
			rt.SetTextSpan(base, mid, mid+1, mid, mid+1)
		}
		_ = rt.TextLayout()
		toEdited = !toEdited
	}
	if got := rt.lcache.live.spansHit - hits0; got != reps {
		t.Fatalf("击键应全走span增量,实际命中%d/%d", got, reps)
	}
	if rt.lcache.live.spansMiss != miss0 {
		t.Fatalf("击键不应回退,miss=%d", rt.lcache.live.spansMiss-miss0)
	}
	return time.Since(start) / reps
}

// TestKeystrokeRatio_M1端到端击键门禁:单次改字耗时比值≤5(阈值不动).
// 三模式:不回绕/回绕短段(36字/段)/回绕长段(5000字/段,取计划下限).
// 基线档必须与被测档同段长,否则比值混入段长缩放:1e3字装不下5000字段,
// 故WrapLong基线取5e3(单个完整段),测的是段数×20下耗时是否持平(段隔离),
// 而非段长×5的O(段长)缩放(那是已签约的复杂度,非门禁对象).长段另
// 断言T(1e5)/T(1e4)≤2(同段长,段隔离的纯净证据).
func TestKeystrokeRatio_M1(t *testing.T) {
	modes := []struct {
		name    string
		w       float64
		perLine int
		baseN   int
	}{
		{"NoWrap", 0, 36, 1000},
		{"WrapShort", 300, 36, 1000},
		{"WrapLong", 300, 5000, 5000},
	}
	for _, m := range modes {
		var a, b, c []time.Duration
		for i := 0; i < 5; i++ {
			a = append(a, m1TimeKeystroke(t, m.baseN, m.perLine, m.w))
			b = append(b, m1TimeKeystroke(t, 10000, m.perLine, m.w))
			c = append(c, m1TimeKeystroke(t, 100000, m.perLine, m.w))
		}
		dBase, d1e4, d1e5 := m1Median(a), m1Median(b), m1Median(c)
		t.Logf("击键%s: base=%v 1e4=%v 1e5=%v 比值1e5/base=%.2f", m.name, dBase, d1e4, d1e5, float64(d1e5)/float64(dBase))
		if r := float64(d1e5) / float64(dBase); r > 5 {
			t.Fatalf("击键%s疑似O(n):文本×%d耗时×%.1f,门禁≤5", m.name, 100000/m.baseN, r)
		}
		if m.name == "WrapLong" {
			if r := float64(d1e5) / float64(d1e4); r > 2 {
				t.Fatalf("击键长段隔离失效:同段长下段数×8耗时×%.1f,门禁≤2", r)
			}
		}
	}
}
