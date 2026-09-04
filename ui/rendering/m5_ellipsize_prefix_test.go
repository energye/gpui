package rendering

import (
	"strings"
	"testing"
)

// TestEllipsizePrefixMatches_M5 locks M5-5: the prefix-sum O(log n) ellipsis
// locator returns byte-identical results to the binary-search implementation
// on short, long, CJK and already-ellipsized inputs.
func TestEllipsizePrefixMatches_M5(t *testing.T) {
	mk := func() *RenderText {
		rt := NewRenderText("")
		rt.MaxWidth = 300
		rt.MaxLines = 1
		rt.Overflow = TextOverflowEllipsis
		return rt
	}
	fixtures := []string{
		"Hello world this is a long line that should be ellipsized",
		strings.Repeat("a世b ", 500),
		"你好世界混排测试文本需要被省略显示的部分",
		"short",
		"already…",
		"",
	}
	for _, s := range fixtures {
		rt := mk()
		want := ellipsizeToWidth(s, 120, rt)
		got := ellipsizeWithPrefix(trimEllipsis(s), 120, rt)
		if got != want {
			t.Fatalf("ellipsizeWithPrefix(%q) = %q, want %q", s, got, want)
		}
	}

	// Estimate path is hand-computable: 10px/rune, ellipsis 10px, maxW=50
	// fits exactly 4 runes + marker.
	rt := NewRenderText("")
	rt.FontSize = 10
	rt.ApproxCharW = 1.0
	if got := ellipsizeToWidth("abcdefghijklmnopqrstuvwxyz", 50, rt); got != "abcd…" {
		t.Fatalf("estimate ellipsis = %q, want %q", got, "abcd…")
	}
}

// trimEllipsis mirrors ellipsizeToWidth's preamble (strip trailing markers
// before re-fitting) so the test feeds the helper exactly what production
// passes it.
func trimEllipsis(s string) []rune {
	runes := []rune(s)
	for strings.HasSuffix(string(runes), textEllipsis) {
		runes = runes[:len(runes)-len([]rune(textEllipsis))]
	}
	return runes
}

// BenchmarkEllipsizePrefix reports ellipsis-fit cost at two magnitudes
// (M5-5 complexity evidence, run alone: timing-sensitive).
func BenchmarkEllipsizePrefix(b *testing.B) {
	for _, n := range []int{1000, 100000} {
		s := strings.Repeat("a世", n/2)
		b.Run(strings.ReplaceAll(map[int]string{1000: "N=1k", 100000: "N=100k"}[n], " ", ""), func(b *testing.B) {
			rt := NewRenderText("")
			rt.MaxWidth = 300
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = ellipsizeToWidth(s, 120, rt)
			}
		})
	}
}
