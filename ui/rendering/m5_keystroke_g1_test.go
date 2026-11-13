package rendering

import (
	"runtime"
	"sort"
	"testing"
	"time"
)

type m5EditKind int

const (
	m5EditReplace m5EditKind = iota
	m5EditInsertDelete
)

// m5TimeKeystroke复刻生产路径单次击键:RenderText.SetTextSpan+TextLayout.
// 计时前先跑200次预热并在预热后回收一次:门禁在1e6档首建后直接计时,
// 冷启动/GC会被算进均值(2026-11-13复核:不预热比值1.3-2.7,预热+回收后0.60-1.22).
func m5TimeKeystroke(t *testing.T, n, perLine int, w float64, kind m5EditKind) time.Duration {
	t.Helper()
	base := m1BenchDoc(n, perLine)
	mid := len(base) / 2
	for mid < len(base) && base[mid] == '\n' {
		mid++
	}
	var edited string
	if kind == m5EditInsertDelete {
		edited = base[:mid] + "X" + base[mid:]
	} else {
		edited = base[:mid] + "X" + base[mid+1:]
	}
	rt := NewRenderText(base)
	rt.MaxWidth = w
	_ = rt.TextLayout()
	apply := func(toEdited bool) {
		switch {
		case kind == m5EditInsertDelete && toEdited:
			rt.SetTextSpan(edited, mid, mid, mid, mid+1)
		case kind == m5EditInsertDelete:
			rt.SetTextSpan(base, mid, mid+1, mid, mid)
		case toEdited:
			rt.SetTextSpan(edited, mid, mid+1, mid, mid+1)
		default:
			rt.SetTextSpan(base, mid, mid+1, mid, mid+1)
		}
		_ = rt.TextLayout()
	}
	const warm = 200
	for i := 0; i < warm; i++ {
		apply(i%2 == 0)
	}
	// 1e6档首建后堆顶就在回收线附近,预热后先回收一次,避免GC落进计时窗口.
	runtime.GC()
	hits0, miss0 := rt.lcache.live.spansHit, rt.lcache.live.spansMiss
	const reps = 1000
	start := time.Now()
	for i := 0; i < reps; i++ {
		apply(i%2 == 0)
	}
	if got := rt.lcache.live.spansHit - hits0; got != reps {
		t.Fatalf("击键应全走span增量,实际命中%d/%d", got, reps)
	}
	if rt.lcache.live.spansMiss != miss0 {
		t.Fatalf("击键不应回退,miss=%d", rt.lcache.live.spansMiss-miss0)
	}
	return time.Since(start) / reps
}

func TestKeystrokeRatio_M5(t *testing.T) {
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
	arms := []struct {
		name string
		kind m5EditKind
	}{
		{"等长替换", m5EditReplace},
		{"插入删除", m5EditInsertDelete},
	}
	for _, m := range modes {
		for _, arm := range arms {
			var a, c []time.Duration
			var ratios []float64
			for i := 0; i < 7; i++ {
				// 基线与1e6交替并成对取比值,抵消测量期间的机器漂移;
				// 门禁取下限用比值的中位数(亚微秒档靠配对比值抗单轮抖动).
				da := m5TimeKeystroke(t, m.baseN, m.perLine, m.w, arm.kind)
				dc := m5TimeKeystroke(t, 1000000, m.perLine, m.w, arm.kind)
				a = append(a, da)
				c = append(c, dc)
				ratios = append(ratios, float64(dc)/float64(da))
			}
			sort.Float64s(ratios)
			ratio := ratios[len(ratios)/2]
			dBase, d1e6 := m1Median(a), m1Median(c)
			t.Logf("G1击键%s/%s: base=%v 1e6=%v 比值1e6/base=%.2f", m.name, arm.name, dBase, d1e6, ratio)
			if ratio > 1.5 {
				t.Errorf("G1未达标%s/%s:文本×%d耗时×%.1f,门禁≤1.5", m.name, arm.name, 1000000/m.baseN, ratio)
			}
		}
	}
}

// m5TimeRowCountKeystroke测行数变化编辑(插入换行再删回):Flutter对文本变化
// 整段重建版面(O(n)),本引擎同口径,不逐行插入;这里只锁常量上限.
func m5TimeRowCountKeystroke(t *testing.T, n, reps int) time.Duration {
	t.Helper()
	base := m1BenchDoc(n, 36)
	mid := len(base) / 2
	for mid < len(base) && base[mid] == '\n' {
		mid++
	}
	withNL := base[:mid] + "\n" + base[mid:]
	rt := NewRenderText(base)
	_ = rt.TextLayout()
	apply := func(toNL bool) {
		if toNL {
			rt.SetTextSpan(withNL, mid, mid, mid, mid+1)
		} else {
			rt.SetTextSpan(base, mid, mid+1, mid, mid)
		}
		_ = rt.TextLayout()
	}
	for i := 0; i < 200; i++ {
		apply(i%2 == 0)
	}
	runtime.GC()
	h0, m0 := rt.lcache.live.spansHit, rt.lcache.live.spansMiss
	start := time.Now()
	for i := 0; i < reps; i++ {
		apply(i%2 == 0)
	}
	if got := rt.lcache.live.spansHit - h0; got != reps {
		t.Fatalf("行数编辑应全走span增量,实际命中%d/%d", got, reps)
	}
	if rt.lcache.live.spansMiss != m0 {
		t.Fatalf("行数编辑不应回退,miss=%d", rt.lcache.live.spansMiss-m0)
	}
	return time.Since(start) / time.Duration(reps)
}

// TestRowCountEditBound_M5:O(n)整段重建的绝对上限(1e6档≤2ms,1e5档≤500µs).
// 比值门禁不适用:行数变化天然是线性重建,Flutter同样整段重排.
func TestRowCountEditBound_M5(t *testing.T) {
	var big, mid []time.Duration
	for i := 0; i < 5; i++ {
		big = append(big, m5TimeRowCountKeystroke(t, 1000000, 100))
		mid = append(mid, m5TimeRowCountKeystroke(t, 100000, 200))
	}
	dBig, dMid := m1Median(big), m1Median(mid)
	t.Logf("回车/跨行删: 1e6=%v 1e5=%v", dBig, dMid)
	if dBig > 2*time.Millisecond {
		t.Fatalf("1e6行数变化编辑%v超上限2ms", dBig)
	}
	if dMid > 500*time.Microsecond {
		t.Fatalf("1e5行数变化编辑%v超上限500µs", dMid)
	}
}
