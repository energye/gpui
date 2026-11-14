package rendering

// 挂账清偿（2026-11-14 #1）：SetViewportHint / longLineWindow 的单行判定
// 每次 strings.IndexByte(t.Text,'\n') 全串扫描（5e6 字节 ≈1ms/次），横向
// 滚动与每次击键布局重建都付一遍。修法：RenderText 维护 hasNewline 增量
// 标记——SetText/SetRuns 整串入口扫一次 O(n)（本就是重操作），SetTextSpan
// 击键入口只扫插入段 O(击键长度)；删除不能创造换行，无需处理。判定结果
// 与扫描完全一致（既有长行/V4 测试锁行为）。
//
// 红灯：5e6 无换行单行上连续 SetViewportHint（scrollX 每次变化），旧实现
// 每次 ~1ms；本测试锁 median ≤100µs（10× 修后余量）。

import (
	"sort"
	"strings"
	"testing"
	"time"
)

func TestV5NewlineGuard_HintNoFullScan(t *testing.T) {
	face := m5DejaVuFace(t)
	rt := NewRenderText("")
	rt.SetFace(face)
	rt.SetFontSize(14)
	var b strings.Builder
	b.Grow(5000000)
	for b.Len() < 5000000 {
		b.WriteString("56789 abcdefghij single line content no newline here ")
	}
	long := strings.TrimRight(b.String(), " ")
	if strings.IndexByte(long, '\n') >= 0 {
		t.Fatalf("seed must be single-line")
	}
	rt.SetText(long) // 整串入口：一次性扫描属于 SetText 语义，不计时
	if got := strings.Count(long, ""); got == 0 {
		t.Fatalf("empty seed")
	}
	var ds []time.Duration
	for i := 0; i < 60; i++ {
		t0 := time.Now()
		rt.SetViewportHint(float64(i*1000), 904)
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	median := ds[len(ds)/2]
	t.Logf("SetViewportHint on 5e6-byte single line: median %v (bound <=100µs, old ~1ms full scan)", median)
	if median > 100*time.Microsecond {
		t.Fatalf("RED: SetViewportHint median %v — per-call full-string newline scan still present", median)
	}
	// 判定结果必须与扫描一致：无换行 → 单行语义成立（横向视口 hint 生效）。
	rt.SetViewportHint(1000, 904)
	lay := rt.TextLayout()
	if lay == nil || lay.LongLine == nil {
		t.Fatalf("single-line 5e6 doc must take the long-line window path (truncated layout)")
	}
}
