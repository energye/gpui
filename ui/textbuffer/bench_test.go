package textbuffer

import (
	"strings"
	"testing"
)

// BenchmarkInsertAtOffset measures steady-state single insert cost on a
// constant-size document (insert then remove at the same spot).
// Three tiers; the ratio gate lives in TestInsertComplexityRatio.
func BenchmarkInsertAtOffset(b *testing.B) {
	for _, n := range []int{10000, 100000, 1000000} {
		b.Run(strings.ReplaceAll(map[int]string{10000: "1e4", 100000: "1e5", 1000000: "1e6"}[n], " ", ""), func(b *testing.B) {
			buf := NewFromString(strings.Repeat("a世b", n/3+1))
			buf.ForceTree(true)
			mid := buf.Len() / 2
			for off := mid; off > 0 && off < buf.Len() && (buf.Slice(off, off+1)[0]&0xC0) == 0x80; off-- {
				mid = off
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				buf.Insert(mid, "x")
				buf.Delete(mid, mid+1)
			}
		})
	}
}

// BenchmarkSnapshot reports O(1) capture cost on a large document.
func BenchmarkSnapshot(b *testing.B) {
	buf := NewFromString(strings.Repeat("a世b", 400000))
	buf.ForceTree(true)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buf.Snapshot()
	}
}
