// Package textbuffer is the M3.5 piece tree: O(log n) edits and O(1)
// snapshots for large documents. Small documents (< SmallMax) stay a plain
// string; the tree activates lazily above the threshold with hysteresis on
// the way down so edits around the boundary do not flap modes.
package textbuffer

import (
	"strings"
	"sync/atomic"
)

// SmallMax documents below this byte size stay a plain string.
const SmallMax = 64 * 1024

const (
	smallLow = 32 * 1024
	maxChunk = 4096
)

var prioSeed atomic.Uint32

func nextPrio() uint32 {
	x := prioSeed.Add(0x9E3779B1)
	x ^= x >> 16
	x *= 0x21F0AAAD
	x ^= x >> 15
	if x == 0 {
		return 1
	}
	return x
}

// node is one immutable chunk in an implicit-key treap. Nodes are never
// mutated after creation; split/merge clone the path, so old roots stay
// valid and Snapshot is a pointer copy.
type node struct {
	left, right *node
	prio        uint32
	data        string
	size        int
	nl          int
	nnl         int // '\n' count of data alone; chunks are immutable so it never changes
}

func sizeOf(n *node) int {
	if n == nil {
		return 0
	}
	return n.size
}

func nlOf(n *node) int {
	if n == nil {
		return 0
	}
	return n.nl
}

func newLeaf(data string) *node {
	c := strings.Count(data, "\n")
	return &node{prio: nextPrio(), data: data, size: len(data), nl: c, nnl: c}
}

func cloneNode(n *node) *node {
	c := *n
	return &c
}

func update(n *node) {
	n.size = len(n.data) + sizeOf(n.left) + sizeOf(n.right)
	n.nl = n.nnl + nlOf(n.left) + nlOf(n.right)
}

func merge(a, b *node) *node {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.prio < b.prio {
		c := cloneNode(a)
		c.right = merge(a.right, b)
		update(c)
		return c
	}
	c := cloneNode(b)
	c.left = merge(a, b.left)
	update(c)
	return c
}

// split divides n so l holds the first k bytes. k is an exact byte
// position, so leaves may hold a split multi-byte encoding afterwards; that
// is safe because every consumer (newline scan, Slice, materialize) is
// byte-based and 0x0A never appears inside a multi-byte sequence.
func split(n *node, k int) (l, r *node) {
	if n == nil {
		return nil, nil
	}
	leftSize := sizeOf(n.left)
	if k <= leftSize {
		l, rr := split(n.left, k)
		c := cloneNode(n)
		c.left = rr
		update(c)
		return l, c
	}
	if k >= leftSize+len(n.data) {
		ll, r := split(n.right, k-leftSize-len(n.data))
		c := cloneNode(n)
		c.right = ll
		update(c)
		return c, r
	}
	cut := k - leftSize
	l = merge(n.left, newLeaf(n.data[:cut]))
	r = merge(newLeaf(n.data[cut:]), n.right)
	return l, r
}

// buildRoot chunks s at rune boundaries so freshly built leaves hold whole
// encodings. split may still cut a leaf mid-rune later (safe, see split).
// Invalid UTF-8 is kept byte-exact; the m==0 guard guarantees progress.
func buildRoot(s string) *node {
	var r *node
	for len(s) > 0 {
		m := maxChunk
		if m > len(s) {
			m = len(s)
		}
		for m > 0 && m < len(s) && (s[m]&0xC0) == 0x80 {
			m--
		}
		if m == 0 {
			m = 1
		}
		r = merge(r, newLeaf(s[:m]))
		s = s[m:]
	}
	return r
}

func collect(n *node, sb *strings.Builder) {
	if n == nil {
		return
	}
	collect(n.left, sb)
	sb.WriteString(n.data)
	collect(n.right, sb)
}

func materialize(n *node) string {
	var sb strings.Builder
	sb.Grow(sizeOf(n))
	collect(n, &sb)
	return sb.String()
}

func forEach(n *node, fn func(chunk string) bool) bool {
	if n == nil {
		return true
	}
	if !forEach(n.left, fn) {
		return false
	}
	if !fn(n.data) {
		return false
	}
	return forEach(n.right, fn)
}

func sliceInto(n *node, a, c int, sb *strings.Builder) {
	if n == nil || a >= c {
		return
	}
	leftSize := sizeOf(n.left)
	if a < leftSize {
		end := c
		if end > leftSize {
			end = leftSize
		}
		sliceInto(n.left, a, end, sb)
	}
	d0 := a - leftSize
	if d0 < 0 {
		d0 = 0
	}
	d1 := c - leftSize
	if d1 > len(n.data) {
		d1 = len(n.data)
	}
	if d1 > d0 {
		sb.WriteString(n.data[d0:d1])
	}
	rest := c - leftSize - len(n.data)
	if rest > 0 {
		ra := a - leftSize - len(n.data)
		if ra < 0 {
			ra = 0
		}
		sliceInto(n.right, ra, rest, sb)
	}
}

func clampOff(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Buffer holds document text with byte-indexed edits. Offsets are plain byte
// indexes; callers snap to rune boundaries.
type Buffer struct {
	str     string
	root    *node
	tree    bool
	pinTree bool
	pinStr  bool
}

// New returns an empty buffer.
func New() *Buffer { return &Buffer{} }

// NewFromString returns a buffer holding s.
func NewFromString(s string) *Buffer {
	b := &Buffer{}
	if len(s) >= SmallMax {
		b.root = buildRoot(s)
		b.tree = true
	} else {
		b.str = s
	}
	return b
}

// Len returns the document size in bytes.
func (b *Buffer) Len() int {
	if b == nil {
		return 0
	}
	if b.tree {
		return sizeOf(b.root)
	}
	return len(b.str)
}

// Mode reports "tree" or "string" for diagnostics and degradation checks.
func (b *Buffer) Mode() string {
	if b != nil && b.tree {
		return "tree"
	}
	return "string"
}

// ForceTree pins tree mode (benchmarks, large-document paths).
func (b *Buffer) ForceTree(v bool) {
	if b == nil {
		return
	}
	b.pinTree = v
	if v {
		b.pinStr = false
		if !b.tree {
			b.root = buildRoot(b.str)
			b.str = ""
			b.tree = true
		}
		return
	}
	b.normalize()
}

// ForceString pins plain-string mode: the degradation fallback. Content is
// preserved; only the cost model changes.
func (b *Buffer) ForceString(v bool) {
	if b == nil {
		return
	}
	b.pinStr = v
	if v {
		b.pinTree = false
		if b.tree {
			b.str = materialize(b.root)
			b.root = nil
			b.tree = false
		}
		return
	}
	// Unpinning must restore the size invariant at once: normalize only
	// downgrades, so a large string-mode buffer would otherwise stay off
	// the tree permanently despite automatic mode calling for it.
	if !b.tree && len(b.str) >= SmallMax {
		b.root = buildRoot(b.str)
		b.str = ""
		b.tree = true
	}
}

func (b *Buffer) normalize() {
	if b.pinStr {
		if b.tree {
			b.str = materialize(b.root)
			b.root = nil
			b.tree = false
		}
		return
	}
	if b.pinTree {
		if !b.tree {
			b.root = buildRoot(b.str)
			b.str = ""
			b.tree = true
		}
		return
	}
	if b.tree && sizeOf(b.root) < smallLow {
		b.str = materialize(b.root)
		b.root = nil
		b.tree = false
	}
}

// String materializes the document. Large-document hot paths should prefer
// Slice/LineAt over full materialization.
func (b *Buffer) String() string {
	if b == nil {
		return ""
	}
	if b.tree {
		return materialize(b.root)
	}
	return b.str
}

// Insert adds s at byte offset off (clamped).
func (b *Buffer) Insert(off int, s string) {
	if b == nil {
		return
	}
	if s == "" {
		b.normalize()
		return
	}
	if b.tree && !b.pinStr {
		off = clampOff(off, 0, sizeOf(b.root))
		var m *node
		for rest := s; len(rest) > 0; {
			mm := maxChunk
			if mm > len(rest) {
				mm = len(rest)
			}
			for mm > 0 && mm < len(rest) && (rest[mm]&0xC0) == 0x80 {
				mm--
			}
			if mm == 0 {
				mm = 1
			}
			m = merge(m, newLeaf(rest[:mm]))
			rest = rest[mm:]
		}
		l, r := split(b.root, off)
		b.root = merge(merge(l, m), r)
		b.normalize()
		return
	}
	off = clampOff(off, 0, len(b.str))
	b.str = b.str[:off] + s + b.str[off:]
	if !b.pinStr && (b.pinTree || len(b.str) >= SmallMax) {
		b.root = buildRoot(b.str)
		b.str = ""
		b.tree = true
	}
	b.normalize()
}

// Delete removes [a, c) (clamped, empty ranges are no-ops).
func (b *Buffer) Delete(a, c int) {
	if b == nil {
		return
	}
	if b.tree && !b.pinStr {
		n := sizeOf(b.root)
		a = clampOff(a, 0, n)
		c = clampOff(c, a, n)
		if c == a {
			b.normalize()
			return
		}
		l, t := split(b.root, a)
		_, r := split(t, c-a)
		b.root = merge(l, r)
		b.normalize()
		return
	}
	n := len(b.str)
	a = clampOff(a, 0, n)
	c = clampOff(c, a, n)
	if c > a {
		b.str = b.str[:a] + b.str[c:]
	}
	b.normalize()
}

// Slice returns [a, c) (clamped).
func (b *Buffer) Slice(a, c int) string {
	if b == nil {
		return ""
	}
	if b.tree {
		n := sizeOf(b.root)
		a = clampOff(a, 0, n)
		c = clampOff(c, a, n)
		if c == a {
			return ""
		}
		var sb strings.Builder
		sb.Grow(c - a)
		sliceInto(b.root, a, c, &sb)
		return sb.String()
	}
	n := len(b.str)
	a = clampOff(a, 0, n)
	c = clampOff(c, a, n)
	return b.str[a:c]
}

// LineCount returns the number of '\n'-separated lines; empty holds 1 line.
func (b *Buffer) LineCount() int {
	if b == nil {
		return 1
	}
	if b.tree {
		return nlOf(b.root) + 1
	}
	return strings.Count(b.str, "\n") + 1
}

// LineAt returns the k-th line without its terminator (clamped). A trailing
// '\r' of a CRLF pair is stripped even when the pair spans chunks.
func (b *Buffer) LineAt(k int) string {
	if b == nil {
		return ""
	}
	total := b.LineCount()
	k = clampOff(k, 0, total-1)
	line := 0
	var sb strings.Builder
	stop := false
	each := func(chunk string) bool {
		for len(chunk) > 0 && !stop {
			if line < k {
				i := strings.IndexByte(chunk, '\n')
				if i < 0 {
					return true
				}
				line++
				chunk = chunk[i+1:]
				continue
			}
			i := strings.IndexByte(chunk, '\n')
			if i < 0 {
				sb.WriteString(chunk)
				return true
			}
			sb.WriteString(chunk[:i])
			stop = true
			return false
		}
		return !stop
	}
	if b.tree {
		forEach(b.root, each)
	} else {
		each(b.str)
	}
	out := sb.String()
	if strings.HasSuffix(out, "\r") {
		out = out[:len(out)-1]
	}
	return out
}

// Snapshot is an O(1) capture: nodes are immutable, so the root pointer (or
// the string header) fully describes the document at capture time.
type Snapshot struct {
	str  string
	root *node
	tree bool
}

// Snapshot captures the current document in O(1).
func (b *Buffer) Snapshot() Snapshot {
	if b == nil {
		return Snapshot{}
	}
	if b.tree {
		return Snapshot{root: b.root, tree: true}
	}
	return Snapshot{str: b.str}
}

// SnapshotFromString captures a plain string in O(1) (header copy; Go
// strings are immutable).
func SnapshotFromString(s string) Snapshot { return Snapshot{str: s} }

// String materializes the snapshot content.
func (s Snapshot) String() string {
	if s.tree {
		return materialize(s.root)
	}
	return s.str
}

// Len returns the snapshot size in bytes.
func (s Snapshot) Len() int {
	if s.tree {
		return sizeOf(s.root)
	}
	return len(s.str)
}

// Restore replaces the buffer content with the snapshot in O(1).
// Degradation pins survive: a restore is a content operation like
// Insert/Delete, so it must not silently drop a production fallback
// request. A large string snapshot with no string pin activates the tree,
// matching NewFromString.
func (b *Buffer) Restore(s Snapshot) {
	if b == nil {
		return
	}
	b.str = s.str
	b.root = s.root
	b.tree = s.tree
	if !b.pinStr && !b.tree && len(b.str) >= SmallMax {
		b.root = buildRoot(b.str)
		b.str = ""
		b.tree = true
	}
	b.normalize()
}
