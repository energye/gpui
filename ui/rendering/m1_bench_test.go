package rendering

import (
	"fmt"
	"strings"
	"testing"
)

func m1BenchDoc(nchars, perLine int) string {
	if perLine <= 0 {
		perLine = 36
	}
	var b strings.Builder
	wrote := 0
	for wrote < nchars {
		chunk := perLine
		if wrote+chunk > nchars {
			chunk = nchars - wrote
		}
		b.WriteString(strings.Repeat("a", chunk))
		wrote += chunk
		if wrote < nchars {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// BenchmarkCaretQuery偏移↔坐标查询门禁:N∈{1e3,1e5}比值≤2.
func BenchmarkCaretQuery(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		lay := BuildTextLayout(m1BenchDoc(n, 36), nil, 14, 0, 1.2)
		mid := len(lay.Text) / 2
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, x, _ := lay.CaretForOffset(mid)
				lay.GetPositionForOffset(x, lay.LineTop(1))
			}
		})
	}
}

// BenchmarkKeystrokeCached冷缓存命中组装耗时(只打日志,无门禁):同一缓存上
// 交替重建两文本,稳态全命中,量的是整表组装O(n)分配.生产热路径(零拷贝增量)
// 见BenchmarkKeystroke,复杂度门禁见TestKeystrokeRatio_M1.
func BenchmarkKeystrokeCached(b *testing.B) {
	modes := []struct {
		name string
		w    float64
	}{
		{"NoWrap", 0},
		{"Wrap", 300},
	}
	for _, m := range modes {
		for _, n := range []int{1000, 10000, 100000} {
			c := newLayoutCache()
			base := m1BenchDoc(n, 36)
			c.buildCached(base, nil, 14, m.w, 1.2)
			b.Run(fmt.Sprintf("%s/N=%d", m.name, n), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					edited := base[:len(base)/2] + "X" + base[len(base)/2+1:]
					c.buildCached(edited, nil, 14, m.w, 1.2)
				}
			})
		}
	}
}

// BenchmarkKeystroke端到端击键重排:生产路径Editor→sync→SetTextSpan→
// updateSpan(免逐字节diff,区间恒有效——两次击键间必有一次布局,与生产帧
// 节奏一致;连续未布局的多变更回退diff,见TestSpanFallback_M1).
// 三档:N∈{1e3,1e4,1e5};三模式:不回绕/回绕短段(36字/段)/回绕长段(5000字
// /段,宽300;取计划下限≥5000,长段重排代价即段长函数).本基准只输出耗时,
// 门禁T(1e5)/T(base)≤5见TestKeystrokeRatio_M1(中位数抗抖,阈值不动;
// 其WrapLong基线取5e3单个完整段,保证基线与被测同段长).
func BenchmarkKeystroke(b *testing.B) {
	modes := []struct {
		name    string
		w       float64
		perLine int
	}{
		{"NoWrap", 0, 36},
		{"WrapShort", 300, 36},
		{"WrapLong", 300, 5000},
	}
	for _, m := range modes {
		for _, n := range []int{1000, 10000, 100000} {
			base := m1BenchDoc(n, m.perLine)
			mid := len(base) / 2
			for mid < len(base) && base[mid] == '\n' {
				mid++
			}
			if mid >= len(base) {
				mid = len(base) / 2
			}
			edited := base[:mid] + "X" + base[mid+1:]
			b.Run(fmt.Sprintf("%s/N=%d", m.name, n), func(b *testing.B) {
				rt := NewRenderText(base)
				rt.MaxWidth = m.w
				_ = rt.TextLayout()
				toEdited := true
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if toEdited {
						rt.SetTextSpan(edited, mid, mid+1, mid, mid+1)
					} else {
						rt.SetTextSpan(base, mid, mid+1, mid, mid+1)
					}
					_ = rt.TextLayout()
					toEdited = !toEdited
				}
			})
		}
	}
}
