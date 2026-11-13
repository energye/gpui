package rendering

// V4 capBytes 防回归（2026-11-14 L3 真窗 burst 画像发现）：
// 旧实现的守卫 `nChars >= utf8.RuneCountInString(s)` 为了判断「上限是否超过
// 全文」每次把整行 rune 数完 —— longLineIndex.Window 每次窗口查询都调它，
// 5e6 字节单行实测击键 28.8ms（线性于行长，L1 1e6=3.5ms / L2 3e6=17.5ms /
// L3 5e6=28.8ms，≈5.7µs/KB），超 §2.2 击键 ≤16ms 门禁。修法语义等价：
// 步进循环走满 nChars 步或到行尾即停，O(L) → O(min(cap, L))。
// 本文件钉两件事：逐案语义不变 + 计时不随行长增长。

import (
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// capBytesRef is the pre-fix reference semantics (guard + bounded walk) used
// as the oracle for the equivalence lock.
func capBytesRef(s string, nChars int) int {
	if nChars <= 0 || nChars >= utf8.RuneCountInString(s) {
		return len(s)
	}
	i, count := 0, 0
	for i < len(s) && count < nChars {
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
		count++
	}
	return i
}

func capBytesSeed(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile("testdata/v4_longline_seed.txt")
	if err != nil || len(b) == 0 {
		t.Skipf("v4 seed missing: %v", err)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		t.Skipf("v4 seed is empty")
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}

// TestV4CapBytes_Equivalence locks the semantics across case classes: whole
// line (cap 0/negative/beyond end), partial caps, and mid-cluster outcomes.
func TestV4CapBytes_Equivalence(t *testing.T) {
	s := capBytesSeed(t, 200000)
	for _, nChars := range []int{-1, 0, 1, 7, 100, 9999, 10000, 20000, 30000, 1 << 30} {
		got := capBytes(s, nChars)
		want := capBytesRef(s, nChars)
		if got != want {
			t.Fatalf("capBytes(nChars=%d) = %d, want %d (reference semantics)", nChars, got, want)
		}
	}
	// Result must sit on a rune boundary.
	if got := capBytes(s, 12345); got < len(s) && !utf8.RuneStart(s[got]) {
		t.Fatalf("capBytes cut at %d is not a rune boundary", got)
	}
}

// TestV4CapBytes_CappedWalk is the timing red light: with a 10000-char cap on
// a 5e6-byte line the walk must stay bounded by the cap, never scan the whole
// line. The old guard measured ~2-4ms (full rune count); the capped walk is
// ~50µs — the 0.5ms bound keeps a 4x red margin and an 8x green margin.
func TestV4CapBytes_CappedWalk(t *testing.T) {
	s := capBytesSeed(t, 5000000)
	if utf8.RuneCountInString(s) <= 10000 {
		t.Fatalf("seed repeat did not exceed the 10000-char cap; probe invalid")
	}
	var ds []time.Duration
	for i := 0; i < 50; i++ {
		t0 := time.Now()
		limit := capBytes(s, 10000)
		ds = append(ds, time.Since(t0))
		if limit <= 0 || limit > len(s) {
			t.Fatalf("capBytes limit out of range: %d", limit)
		}
	}
	median := ds[0]
	for i := 1; i < len(ds); i++ { // insertion sort for a stable median
		for j := i; j > 0 && ds[j] < ds[j-1]; j-- {
			ds[j], ds[j-1] = ds[j-1], ds[j]
		}
	}
	median = ds[len(ds)/2]
	t.Logf("V4 capBytes(5e6 text, cap 10000) median = %v (bound <=0.5ms)", median)
	if median > 500*time.Microsecond {
		t.Fatalf("V4 RED: capBytes median %v — the walk is scanning past the cap (O(L) per window query)", median)
	}
}
