package textbuffer

import (
	"strings"
	"testing"
	"time"
)

func lineAtTime(buf *Buffer, lines, reps int) time.Duration {
	start := time.Now()
	for i := 0; i < reps; i++ {
		k := lines - 1 - (i*7)%(lines/4)
		_ = buf.LineAt(k)
	}
	return time.Since(start) / time.Duration(reps)
}

// TestLineAt_NoLinear锁行定位复杂度:1e6行与1e4行的单次LineAt应同量级.
// 当前实现按块顺序遍历(近似O(bytes)),该用例先红后绿.
func TestLineAt_NoLinear(t *testing.T) {
	const reps = 200
	small := NewFromString(strings.Repeat("line content here\n", 10000))
	small.ForceTree(true)
	big := NewFromString(strings.Repeat("line content here\n", 1000000))
	big.ForceTree(true)
	lineAtTime(small, 10000, 20)
	lineAtTime(big, 1000000, 20)
	ds := lineAtTime(small, 10000, reps)
	db := lineAtTime(big, 1000000, reps)
	ratio := float64(db) / float64(ds)
	t.Logf("LineAt 1e4=%v 1e6=%v ratio=%.2f", ds, db, ratio)
	if ratio > 8 {
		t.Fatalf("LineAt疑似线性:1e6/1e4=%.1f,门禁≤8", ratio)
	}
}

func BenchmarkLineAt(b *testing.B) {
	for _, n := range []int{10000, 100000, 1000000} {
		buf := NewFromString(strings.Repeat("line content here\n", n))
		buf.ForceTree(true)
		b.Run(strings.TrimSpace(strings.ReplaceAll(map[int]string{10000: "1e4", 100000: "1e5", 1000000: "1e6"}[n], " ", "")), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = buf.LineAt(n - 1 - i%(n/4))
			}
		})
	}
}

func lineStartRef(lines []string) []int {
	out := make([]int, len(lines))
	off := 0
	for i, ln := range lines {
		out[i] = off
		off += len(ln) + 1
	}
	return out
}

func checkLineLookup(t *testing.T, buf *Buffer, model string, forceTree bool) {
	t.Helper()
	lines := strings.Split(model, "\n")
	starts := lineStartRef(lines)
	for k := range lines {
		if got, want := buf.LineStart(k), starts[k]; got != want {
			t.Fatalf("tree=%v k=%d LineStart=%d want %d", forceTree, k, got, want)
		}
		if got, want := buf.LineAt(k), strings.TrimSuffix(lines[k], "\r"); got != want {
			t.Fatalf("tree=%v k=%d LineAt=%q want %q", forceTree, k, got, want)
		}
	}
	if got, want := buf.LineStart(-1), 0; got != want {
		t.Fatalf("tree=%v LineStart(-1)=%d want %d", forceTree, got, want)
	}
	if got, want := buf.LineStart(1<<30), starts[len(starts)-1]; got != want {
		t.Fatalf("tree=%v LineStart(huge)=%d want %d", forceTree, got, want)
	}
	for off := 0; off <= len(model); off++ {
		want := strings.Count(model[:off], "\n")
		if got := buf.LineIndex(off); got != want {
			t.Fatalf("tree=%v off=%d LineIndex=%d want %d", forceTree, off, got, want)
		}
	}
}

func TestLineStart_LineIndex(t *testing.T) {
	docs := []string{
		"",
		"hello",
		"a\nb\nc",
		"a\n",
		"\n",
		"a\n\nb",
		"a\r\nb\r\n",
		"世\n界\n👨‍👩‍👧",
		strings.Repeat("line content here\n", 1000),
	}
	for _, doc := range docs {
		for _, forceTree := range []bool{false, true} {
			buf := NewFromString(doc)
			if forceTree {
				buf.ForceTree(true)
				buf.Insert(0, "")
			}
			checkLineLookup(t, buf, doc, forceTree)
		}
	}
}

func TestLineLookup_AfterEdits(t *testing.T) {
	model := strings.Repeat("alpha\n\nbeta\r\ngamma\n", 50)
	buf := NewFromString(model)
	buf.ForceTree(true)
	buf.Insert(0, "")
	buf.Insert(0, "PREFIX\n")
	model = "PREFIX\n" + model
	mid := len(model) / 2
	buf.Insert(mid, "XYZ")
	model = model[:mid] + "XYZ" + model[mid:]
	buf.Delete(20, 40)
	model = model[:20] + model[40:]
	big := strings.Repeat("bulk line\n", 1000)
	buf.Insert(len(model), big)
	model += big
	checkLineLookup(t, buf, model, true)
}
