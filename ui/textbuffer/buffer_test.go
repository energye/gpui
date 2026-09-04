package textbuffer

import (
	"math/rand"
	"strings"
	"testing"
	"time"
)

// snapDown moves off back to a rune boundary (model and buffer get the same
// snapped offsets, so byte-exact comparison holds by construction).
func snapDown(s string, off int) int {
	if off < 0 {
		return 0
	}
	if off > len(s) {
		return len(s)
	}
	for off > 0 && off < len(s) && (s[off]&0xC0) == 0x80 {
		off--
	}
	return off
}

var fuzzChunks = []string{"a", "b", "世", "界", "hello", " ", "\n", "\r\n", "👨\u200d👩\u200d👧", "é", "xyz\nabc", "0123456789"}

// runFuzz drives buf and a plain-string model with identical ops; any
// divergence stops the test immediately (M3.5 fuse: stop on first mismatch).
func runFuzz(t *testing.T, forceTree bool) {
	t.Helper()
	rng := rand.New(rand.NewSource(20260904))
	buf := New()
	if forceTree {
		buf.ForceTree(true)
	}
	model := ""
	for i := 0; i < 400; i++ {
		c := fuzzChunks[rng.Intn(len(fuzzChunks))]
		pos := 0
		if len(model) > 0 {
			pos = snapDown(model, rng.Intn(len(model)+1))
		}
		model = model[:pos] + c + model[pos:]
		buf.Insert(pos, c)
	}
	big := strings.Repeat("a世", 20000) // ~60KB bulk to force mode transitions in auto mode
	for i := 0; i < 10000; i++ {
		if i == 5000 {
			pos := snapDown(model, len(model)/2)
			model = model[:pos] + big + model[pos:]
			buf.Insert(pos, big)
		}
		if i == 6000 && len(model) > len(big) {
			pos := strings.Index(model, big)
			if pos >= 0 {
				model = model[:pos] + model[pos+len(big):]
				buf.Delete(pos, pos+len(big))
			}
		}
		if rng.Intn(100) < 60 {
			c := fuzzChunks[rng.Intn(len(fuzzChunks))]
			pos := snapDown(model, rng.Intn(len(model)+1))
			model = model[:pos] + c + model[pos:]
			buf.Insert(pos, c)
		} else if len(model) > 0 {
			a := snapDown(model, rng.Intn(len(model)+1))
			b := snapDown(model, a+rng.Intn(13))
			model = model[:a] + model[b:]
			buf.Delete(a, b)
		}
		if i%50 == 0 {
			if got := buf.String(); got != model {
				t.Fatalf("mode forceTree=%v op %d: divergence len=%d want %d", forceTree, i, len(got), len(model))
			}
		}
		if i%500 == 0 && len(model) > 0 {
			a := snapDown(model, rng.Intn(len(model)+1))
			b := snapDown(model, a+rng.Intn(25))
			if got, want := buf.Slice(a, b), model[a:b]; got != want {
				t.Fatalf("mode forceTree=%v op %d: slice divergence", forceTree, i)
			}
		}
	}
	if got := buf.String(); got != model {
		t.Fatalf("mode forceTree=%v: final divergence len=%d want %d", forceTree, len(got), len(model))
	}
	if want := strings.Count(model, "\n") + 1; buf.LineCount() != want {
		t.Fatalf("mode forceTree=%v: LineCount=%d want %d", forceTree, buf.LineCount(), want)
	}
}

func TestPieceTree_RandomAgainstString(t *testing.T) {
	t.Run("auto", func(t *testing.T) { runFuzz(t, false) })
	t.Run("forced-tree", func(t *testing.T) { runFuzz(t, true) })
}

func TestBufferLineAt_Boundaries(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		count int
		lines []string
	}{
		{"empty", "", 1, []string{""}},
		{"single", "hello", 1, []string{"hello"}},
		{"three", "a\nb\nc", 3, []string{"a", "b", "c"}},
		{"trailing-nl", "a\n", 2, []string{"a", ""}},
		{"lone-nl", "\n", 2, []string{"", ""}},
		{"empty-mid", "a\n\nb", 3, []string{"a", "", "b"}},
		{"crlf", "a\r\nb\r\n", 3, []string{"a", "b", ""}},
		{"crlf-single", "a\r\n", 2, []string{"a", ""}},
		{"cr-only", "a\rb", 1, []string{"a\rb"}},
		{"unicode", "世\n界", 2, []string{"世", "界"}},
	}
	for _, c := range cases {
		for _, forceTree := range []bool{false, true} {
			buf := NewFromString(c.text)
			if forceTree {
				buf.ForceTree(true)
				buf.Insert(0, "")
			}
			if got := buf.LineCount(); got != c.count {
				t.Fatalf("%s tree=%v: LineCount=%d want %d", c.name, forceTree, got, c.count)
			}
			for k, want := range c.lines {
				if got := buf.LineAt(k); got != want {
					t.Fatalf("%s tree=%v: LineAt(%d)=%q want %q", c.name, forceTree, k, got, want)
				}
			}
			if got := buf.LineAt(-1); got != c.lines[0] {
				t.Fatalf("%s tree=%v: LineAt(-1)=%q want %q", c.name, forceTree, got, c.lines[0])
			}
			if got := buf.LineAt(1 << 30); got != c.lines[len(c.lines)-1] {
				t.Fatalf("%s tree=%v: LineAt(huge)=%q want %q", c.name, forceTree, got, c.lines[len(c.lines)-1])
			}
		}
	}
}

func TestBufferLineAt_SplitCRLF(t *testing.T) {
	// \r and \n land in different chunks; the pair must still count once.
	buf := New()
	buf.ForceTree(true)
	buf.Insert(0, "x\r")
	buf.Insert(3, "\ny")
	if got := buf.LineCount(); got != 2 {
		t.Fatalf("LineCount=%d want 2", got)
	}
	if got := buf.LineAt(0); got != "x" {
		t.Fatalf("LineAt(0)=%q want %q", got, "x")
	}
	if got := buf.LineAt(1); got != "y" {
		t.Fatalf("LineAt(1)=%q want %q", got, "y")
	}
}

func TestBufferMode_Switch(t *testing.T) {
	buf := New()
	if got := buf.Mode(); got != "string" {
		t.Fatalf("fresh Mode=%q want string", got)
	}
	buf.Insert(0, strings.Repeat("a", 100))
	if got := buf.Mode(); got != "string" {
		t.Fatalf("small Mode=%q want string", got)
	}
	buf.Insert(0, strings.Repeat("b", 70*1024))
	if got := buf.Mode(); got != "tree" {
		t.Fatalf("large Mode=%q want tree", got)
	}
	buf.Delete(0, buf.Len()-100)
	if got := buf.Mode(); got != "string" {
		t.Fatalf("shrunk Mode=%q want string", got)
	}
	fb := New()
	fb.ForceString(true)
	fb.Insert(0, strings.Repeat("c", 1024*1024))
	if got := fb.Mode(); got != "string" {
		t.Fatalf("forced-string Mode=%q want string", got)
	}
	if got := fb.String(); got != strings.Repeat("c", 1024*1024) {
		t.Fatalf("forced-string content mismatch len=%d", len(got))
	}
	tb := NewFromString("tiny")
	tb.ForceTree(true)
	if got := tb.Mode(); got != "tree" {
		t.Fatalf("forced-tree Mode=%q want tree", got)
	}
	if got := tb.String(); got != "tiny" {
		t.Fatalf("forced-tree content=%q want tiny", got)
	}
}

func TestBufferSnapshot_Restore(t *testing.T) {
	for _, forceTree := range []bool{false, true} {
		buf := NewFromString(strings.Repeat("a世\nb", 2000))
		if forceTree {
			buf.ForceTree(true)
		}
		t0 := time.Now()
		snap := buf.Snapshot()
		if el := time.Since(t0); el > 5*time.Second {
			t.Fatalf("tree=%v: snapshot took %v, not O(1)", forceTree, el)
		}
		buf.Insert(3, "INSERTED")
		buf.Delete(0, 2)
		mid := buf.String()
		buf.Restore(snap)
		if got, want := buf.String(), snap.String(); got != want {
			t.Fatalf("tree=%v: restore mismatch", forceTree)
		}
		buf.Insert(0, "x")
		if got := buf.String(); got == mid {
			t.Fatalf("tree=%v: post-restore edit had no effect", forceTree)
		}
	}
}

func TestBufferSnapshot_LargeIsO1(t *testing.T) {
	buf := NewFromString(strings.Repeat("a世b", 400000)) // ~2MB
	buf.ForceTree(true)
	t0 := time.Now()
	snap := buf.Snapshot()
	if el := time.Since(t0); el > 5*time.Second {
		t.Fatalf("2MB snapshot took %v, not O(1)", el)
	}
	if got := snap.String(); len(got) != buf.Len() {
		t.Fatalf("snapshot len=%d want %d", len(got), buf.Len())
	}
}

func TestBufferRestore_SizeInvariant_M35(t *testing.T) {
	large := strings.Repeat("a世b\n", 20000) // ~140KB, above SmallMax
	b := New()
	b.Restore(SnapshotFromString(large))
	if got := b.Mode(); got != "tree" {
		t.Fatalf("large string snapshot Mode=%q want tree (NewFromString parity)", got)
	}
	if got := b.String(); got != large {
		t.Fatalf("large restore content mismatch len=%d", len(got))
	}
	s := New()
	s.Restore(SnapshotFromString("small"))
	if got := s.Mode(); got != "string" {
		t.Fatalf("small string snapshot Mode=%q want string", got)
	}
}

// TestBufferRestore_KeepsPins_M35 locks that Restore is a content operation
// like Insert/Delete: degradation pins survive instead of being silently
// dropped (a dropped string pin would spike memory back to tree mode).
func TestBufferRestore_KeepsPins_M35(t *testing.T) {
	doc := strings.Repeat("a世b\n", 20000) // ~140KB
	treeSnap := NewFromString(doc).Snapshot()
	b := NewFromString(doc)
	b.ForceString(true)
	b.Restore(treeSnap)
	if got := b.Mode(); got != "string" {
		t.Fatalf("string pin lost on restore: Mode=%q want string", got)
	}
	if got := b.String(); got != doc {
		t.Fatalf("pinned restore content mismatch len=%d", len(got))
	}
	c := New()
	c.ForceTree(true)
	c.Restore(SnapshotFromString(doc))
	if got := c.Mode(); got != "tree" {
		t.Fatalf("tree pin lost on restore: Mode=%q want tree", got)
	}
	if got := c.String(); got != doc {
		t.Fatalf("tree-pinned restore content mismatch len=%d", len(got))
	}
}

func TestInsertComplexityRatio(t *testing.T) {
	sizes := []int{10000, 100000, 1000000}
	const k = 500
	perOp := make([]time.Duration, len(sizes))
	for i, n := range sizes {
		buf := NewFromString(strings.Repeat("a世b", n/3+1))
		buf.ForceTree(true)
		for len([]rune(buf.String())) > n {
			s := buf.String()
			buf.Delete(len(s)-8, len(s))
		}
		mid := buf.Len() / 2
		for off := mid; off > 0 && off < buf.Len() && (buf.Slice(off, off+1)[0]&0xC0) == 0x80; off-- {
			mid = off
		}
		t0 := time.Now()
		for j := 0; j < k; j++ {
			buf.Insert(mid, "x")
			mid++
		}
		perOp[i] = time.Since(t0) / k
		t.Logf("N=%d per-insert=%v", n, perOp[i])
	}
	if r := float64(perOp[2]) / float64(perOp[0]); r > 3 {
		t.Fatalf("T(1e6)/T(1e4)=%.2f > 3", r)
	}
	if r := float64(perOp[1]) / float64(perOp[0]); r > 3 {
		t.Fatalf("T(1e5)/T(1e4)=%.2f > 3", r)
	}
}

// TestBufferInvalidUTF8Progress locks the progress guard: inputs that are
// not valid UTF-8 must still build, insert, and round-trip byte-exact
// instead of spinning forever on an empty chunk.
func TestBufferInvalidUTF8Progress(t *testing.T) {
	bad := strings.Repeat("a世\x80\x80", 20000) // ~140KB, crosses chunk bounds
	for _, forceTree := range []bool{false, true} {
		b := NewFromString(bad)
		if forceTree {
			b.ForceTree(true)
		}
		if got := b.String(); got != bad {
			t.Fatalf("tree=%v: round-trip len=%d want %d", forceTree, len(got), len(bad))
		}
		lead := strings.Repeat("\x80", 70000) // pure continuation bytes
		b2 := New()
		if forceTree {
			b2.ForceTree(true)
		}
		b2.Insert(0, lead)
		if got := b2.String(); got != lead {
			t.Fatalf("tree=%v: lead-continuation round-trip len=%d want %d", forceTree, len(got), len(lead))
		}
		b2.Delete(0, 100)
		if got, want := b2.String(), lead[100:]; got != want {
			t.Fatalf("tree=%v: post-delete len=%d want %d", forceTree, len(got), len(want))
		}
	}
}

// TestForceStringUnpinRestoresTree locks the unpin path: releasing the
// plain-string pin on a large document must rebuild the tree at once
// (normalize only downgrades, so without this the buffer would stay off
// the tree permanently in supposed automatic mode).
func TestForceStringUnpinRestoresTree(t *testing.T) {
	doc := strings.Repeat("a世b\n", 20000) // ~140KB, above SmallMax
	b := NewFromString(doc)
	b.ForceString(true)
	if got := b.Mode(); got != "string" {
		t.Fatalf("pinned Mode=%q want string", got)
	}
	b.Insert(b.Len(), "tail")
	b.ForceString(false)
	if got := b.Mode(); got != "tree" {
		t.Fatalf("unpinned large Mode=%q want tree", got)
	}
	if got := b.String(); got != doc+"tail" {
		t.Fatalf("unpinned content len=%d want %d", len(got), len(doc)+4)
	}
	s := NewFromString("tiny")
	s.ForceString(true)
	s.ForceString(false)
	if got := s.Mode(); got != "string" {
		t.Fatalf("unpinned small Mode=%q want string", got)
	}
}
