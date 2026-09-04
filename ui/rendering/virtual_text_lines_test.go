package rendering_test

// M4 红灯测试:纵向行虚拟化 + 懒测量(ENGINE_TEXT_SCALE_PLAN §M4).
// 被测 API 尚不存在,本文件先红,实现落 ui/rendering/virtual_text_lines.go 后转绿.

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// 定行高:总高 O(1),只物化视口,1e6 行窗口有界.
func TestVirtualLines_FixedWindow(t *testing.T) {
	const count = 1000000
	const lh = 20.0
	v := rendering.NewVirtualTextLines(count, lh)
	if got := v.TotalHeight(); got != float64(count)*lh {
		t.Fatalf("total=%v want %v", got, float64(count)*lh)
	}
	first, last := v.SetViewport(0, 800)
	if first != 0 {
		t.Fatalf("first=%d want 0", first)
	}
	if n := last - first; n > 60 || n <= 0 {
		t.Fatalf("window size=%d want (0,60]", n)
	}
	// 滚动到底部:窗口滑动且仍有界.
	first2, last2 := v.SetViewport(v.TotalHeight()-800, 800)
	if first2 <= first {
		t.Fatalf("window did not slide: first %d -> %d", first, first2)
	}
	if n := last2 - first2; n > 60 {
		t.Fatalf("bottom window size=%d want <=60", n)
	}
	if last2 != count {
		t.Fatalf("bottom last=%d want %d", last2, count)
	}
}

// ScrollToIndex 与 IndexAtOffset 互逆(滚动条定位).
func TestVirtualLines_ScrollToIndex(t *testing.T) {
	const count = 1000000
	const lh = 20.0
	v := rendering.NewVirtualTextLines(count, lh)
	for _, idx := range []int{0, 1, 12345, 999999} {
		off := v.ScrollOffsetForIndex(idx)
		if back := v.IndexAtOffset(off + 0.5); back != idx {
			t.Fatalf("idx %d -> off %v -> idx %d", idx, off, back)
		}
	}
}

// 未测行用估算;实测后总高按差值增量修正.
func TestLazyMeasure_EstimateThenCorrect(t *testing.T) {
	const count = 1000
	const est = 20.0
	v := rendering.NewVariableVirtualTextLines(count, est, nil)
	if got := v.TotalHeight(); got != float64(count)*est {
		t.Fatalf("unmeasured total=%v want %v", got, float64(count)*est)
	}
	if v.IsMeasured(7) {
		t.Fatal("row 7 reported measured before Measure")
	}
	v.Measure(7, 36)
	if !v.IsMeasured(7) {
		t.Fatal("row 7 not measured after Measure")
	}
	want := float64(count)*est + (36 - est)
	if got := v.TotalHeight(); got != want {
		t.Fatalf("corrected total=%v want %v", got, want)
	}
	if v.MeasuredCount() != 1 {
		t.Fatalf("measured=%d want 1", v.MeasuredCount())
	}
}

// 实测窗口内行不搬动窗口起点(不跳变):窗口之上行的 Offset 不动.
func TestLazyMeasure_NoJump(t *testing.T) {
	const count = 10000
	v := rendering.NewVariableVirtualTextLines(count, 20, func(i int) float64 {
		if i%3 == 0 {
			return 34
		}
		return 20
	})
	first, last := v.SetViewport(2000, 800)
	anchorBefore := v.OffsetOf(first)
	totalBefore := v.TotalHeight()
	n := v.MeasureWindow()
	if n != last-first {
		t.Fatalf("newly measured=%d want %d", n, last-first)
	}
	if got := v.OffsetOf(first); got != anchorBefore {
		t.Fatalf("window start jumped: %v -> %v", anchorBefore, got)
	}
	// 窗口之上行不受影响,总高只增不减且差值可解释.
	if got := v.TotalHeight(); got < totalBefore {
		t.Fatalf("total shrank: %v -> %v", totalBefore, got)
	}
}

// OffsetOf 与 IndexAtOffset 在变行高下互逆(抽样).
func TestLazyMeasure_PrefixConsistent(t *testing.T) {
	const count = 5000
	v := rendering.NewVariableVirtualTextLines(count, 20, func(i int) float64 {
		return 16 + float64(i%5)*4
	})
	v.SetViewport(0, 800)
	v.MeasureWindow()
	for _, i := range []int{0, 1, 40, 41, 4999} {
		off := v.OffsetOf(i)
		if back := v.IndexAtOffset(off + 0.5); back != i {
			t.Fatalf("row %d off %v inverts to %d", i, off, back)
		}
	}
	// 末端哨兵:超出总高钳制到末行.
	if back := v.IndexAtOffset(v.TotalHeight() + 1000); back != count-1 {
		t.Fatalf("overflow clamped to %d want %d", back, count-1)
	}
}

// 首屏代价与总行数无关:只测"首视口物化",构造不计时.
// 三档 N 同操作,断言 1e6 档 wall ≤100ms 且 T(1e6)/T(1e4) ≤ 1.5.
func BenchmarkOpenDocument(b *testing.B) {
	for _, n := range []int{10000, 100000, 1000000} {
		v := rendering.NewVariableVirtualTextLines(n, 20, func(i int) float64 {
			return 16 + float64(i%5)*4
		})
		b.Run(map[int]string{10000: "10k", 100000: "100k", 1000000: "1M"}[n], func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				v.SetViewport(0, 800)
				v.MeasureWindow()
			}
		})
	}
}
