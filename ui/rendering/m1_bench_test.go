//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
