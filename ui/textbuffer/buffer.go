//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// snapshots for large documents. Small documents (< SmallMax) stay a plain
// string; the tree activates lazily above the threshold with hysteresis on
// the way down so edits around the boundary do not flap modes.
package textbuffer

import (
	"strings"
	"sync/atomic"
	"unicode/utf8"
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
	u16         int
	nu16        int // UTF-16 units of data alone; chunks are immutable so it never changes
	nr          int
	nnr         int // rune count of data alone; chunks are immutable so it never changes
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
	u := utf16Count(data)
	r := runeCount(data)
	return &node{prio: nextPrio(), data: data, size: len(data), nl: c, nnl: c, u16: u, nu16: u, nr: r, nnr: r}
}

// u16Of reports UTF-16 units of a subtree. Counts assume chunk boundaries sit
// on rune boundaries (editor edits/selections guarantee this); byte APIs stay
// exact even if a caller splits inside a rune.
func u16Of(n *node) int {
	if n == nil {
		return 0
	}
	return n.u16
}

func nrOf(n *node) int {
	if n == nil {
		return 0
	}
	return n.nr
}

func cloneNode(n *node) *node {
	c := *n
	return &c
}

func update(n *node) {
	n.size = len(n.data) + sizeOf(n.left) + sizeOf(n.right)
	n.nl = n.nnl + nlOf(n.left) + nlOf(n.right)
	n.u16 = n.nu16 + u16Of(n.left) + u16Of(n.right)
	n.nr = n.nnr + nrOf(n.left) + nrOf(n.right)
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

// LineStart returns the byte offset of line k's first byte (clamped).
func (b *Buffer) LineStart(k int) int {
	if b == nil {
		return 0
	}
	total := b.LineCount()
	k = clampOff(k, 0, total-1)
	if k == 0 {
		return 0
	}
	pos, ok := b.newlineAt(k - 1)
	if !ok {
		return 0
	}
	return pos + 1
}

// newlineAt returns the byte offset of the n-th '\n' in document order.
// Tree mode descends by subtree newline counts: O(log chunks + one chunk).
func (b *Buffer) newlineAt(n int) (int, bool) {
	if b == nil || n < 0 {
		return 0, false
	}
	if !b.tree {
		off := -1
		for i := 0; i <= n; i++ {
			j := strings.IndexByte(b.str[off+1:], '\n')
			if j < 0 {
				return 0, false
			}
			off += j + 1
		}
		return off, true
	}
	base := 0
	cur := b.root
	for cur != nil {
		left := nlOf(cur.left)
		if n < left {
			cur = cur.left
			continue
		}
		n -= left
		if n < cur.nnl {
			off := -1
			for i := 0; i <= n; i++ {
				j := strings.IndexByte(cur.data[off+1:], '\n')
				off += j + 1
			}
			return base + sizeOf(cur.left) + off, true
		}
		n -= cur.nnl
		base += sizeOf(cur.left) + len(cur.data)
		cur = cur.right
	}
	return 0, false
}

// LineIndex returns the number of '\n' before off (the line containing off).
// Tree mode descends by subtree sizes/newline counts: O(log chunks + one chunk).
func (b *Buffer) LineIndex(off int) int {
	if b == nil {
		return 0
	}
	if !b.tree {
		off = clampOff(off, 0, len(b.str))
		return strings.Count(b.str[:off], "\n")
	}
	n := 0
	off = clampOff(off, 0, sizeOf(b.root))
	cur := b.root
	for cur != nil {
		left := sizeOf(cur.left)
		if off < left {
			cur = cur.left
			continue
		}
		n += nlOf(cur.left)
		d := off - left
		if d <= len(cur.data) {
			return n + strings.Count(cur.data[:d], "\n")
		}
		n += cur.nnl
		off = d - len(cur.data)
		cur = cur.right
	}
	return n
}

// LineAt returns the k-th line without its terminator (clamped). A trailing
// '\r' of a CRLF pair is stripped even when the pair spans chunks.
func (b *Buffer) LineAt(k int) string {
	if b == nil {
		return ""
	}
	total := b.LineCount()
	k = clampOff(k, 0, total-1)
	start := b.LineStart(k)
	end := b.Len()
	if k < total-1 {
		end = b.LineStart(k+1) - 1
	}
	if end < start {
		end = start
	}
	out := b.Slice(start, end)
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

// utf16Count reports the number of UTF-16 code units in s (surrogate pairs
// count 2), matching the editor's UTF-16 selection offsets.
func utf16Count(s string) int {
	n := 0
	for _, r := range s {
		if r <= 0xFFFF {
			n++
		} else {
			n += 2
		}
	}
	return n
}

// byteOffsetInString maps a UTF-16 code-unit offset to a byte offset,
// snapping to the rune boundary at or before the target. Returns -1 when
// the target falls inside a surrogate pair (callers map that to document
// end, matching the editor's existing byteOffsetForUtf16 semantics).
func byteOffsetInString(s string, utf16Off int) int {
	if utf16Off <= 0 {
		return 0
	}
	off := 0
	cu := 0
	for _, r := range s {
		sz := utf8.RuneLen(r)
		cuLen := 1
		if r > 0xFFFF {
			cuLen = 2
		}
		if cu+cuLen > utf16Off {
			if cu < utf16Off {
				return -1
			}
			break
		}
		cu += cuLen
		off += sz
		if cu >= utf16Off {
			break
		}
	}
	if cu < utf16Off {
		return len(s)
	}
	return off
}

// UTF16Len returns the document length in UTF-16 code units (O(1) in tree mode).
func (b *Buffer) UTF16Len() int {
	if b == nil {
		return 0
	}
	if b.tree {
		return u16Of(b.root)
	}
	return utf16Count(b.str)
}

// ByteOffsetForUTF16 maps a UTF-16 offset to a byte offset (clamped), snapping
// to the rune boundary at or before the target. Tree mode descends by subtree
// UTF-16 counts: O(log chunks + one chunk).
func (b *Buffer) ByteOffsetForUTF16(pos int) int {
	if b == nil || pos <= 0 {
		return 0
	}
	if !b.tree {
		if in := byteOffsetInString(b.str, pos); in >= 0 {
			return in
		}
		return len(b.str)
	}
	if pos >= u16Of(b.root) {
		return sizeOf(b.root)
	}
	base := 0
	rem := pos
	cur := b.root
	for cur != nil {
		left := u16Of(cur.left)
		if rem < left {
			cur = cur.left
			continue
		}
		rem -= left
		if rem <= cur.nu16 {
			if in := byteOffsetInString(cur.data, rem); in >= 0 {
				return base + sizeOf(cur.left) + in
			}
			return sizeOf(b.root)
		}
		rem -= cur.nu16
		base += sizeOf(cur.left) + len(cur.data)
		cur = cur.right
	}
	return base
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func utf16ToRuneIndexString(s string, utf16Off int) int {
	off := 0
	idx := 0
	for _, r := range s {
		if off >= utf16Off {
			break
		}
		sz := 1
		if r > 0xFFFF {
			sz = 2
		}
		off += sz
		idx++
	}
	return idx
}

func runeIndexToUTF16String(s string, idx int) int {
	if idx <= 0 {
		return 0
	}
	n := 0
	for _, r := range s {
		if idx == 0 {
			break
		}
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
		idx--
	}
	return n
}

// RuneCount returns the document length in runes (O(1) in tree mode).
func (b *Buffer) RuneCount() int {
	if b == nil {
		return 0
	}
	if b.tree {
		return nrOf(b.root)
	}
	return runeCount(b.str)
}

// UTF16ToRuneIndex maps a UTF-16 offset to a rune index (editor semantics:
// a surrogate pair counts as one rune; a target inside the pair lands after it).
func (b *Buffer) UTF16ToRuneIndex(pos int) int {
	if b == nil || pos <= 0 {
		return 0
	}
	if !b.tree {
		return utf16ToRuneIndexString(b.str, pos)
	}
	if pos >= u16Of(b.root) {
		return nrOf(b.root)
	}
	runes := 0
	rem := pos
	cur := b.root
	for cur != nil {
		left := u16Of(cur.left)
		if rem < left {
			cur = cur.left
			continue
		}
		rem -= left
		runes += nrOf(cur.left)
		if rem <= cur.nu16 {
			cu := 0
			for _, r := range cur.data {
				if cu >= rem {
					break
				}
				w := 1
				if r > 0xFFFF {
					w = 2
				}
				cu += w
				runes++
			}
			return runes
		}
		rem -= cur.nu16
		runes += cur.nnr
		cur = cur.right
	}
	return runes
}

// RuneIndexToUTF16 maps a rune index to a UTF-16 offset (clamped).
func (b *Buffer) RuneIndexToUTF16(k int) int {
	if b == nil || k <= 0 {
		return 0
	}
	if !b.tree {
		return runeIndexToUTF16String(b.str, k)
	}
	if k >= nrOf(b.root) {
		return u16Of(b.root)
	}
	rem := k
	cu := 0
	cur := b.root
	for cur != nil {
		left := nrOf(cur.left)
		if rem < left {
			cur = cur.left
			continue
		}
		rem -= left
		cu += u16Of(cur.left)
		if rem <= cur.nnr {
			for _, r := range cur.data {
				if rem == 0 {
					break
				}
				if r > 0xFFFF {
					cu += 2
				} else {
					cu++
				}
				rem--
			}
			return cu
		}
		rem -= cur.nnr
		cu += cur.nu16
		cur = cur.right
	}
	return cu
}

// UTF16OffsetAt maps a byte offset to a UTF-16 offset (clamped).
func (b *Buffer) UTF16OffsetAt(byteOff int) int {
	if b == nil || byteOff <= 0 {
		return 0
	}
	if !b.tree {
		if byteOff > len(b.str) {
			byteOff = len(b.str)
		}
		return utf16Count(b.str[:byteOff])
	}
	if byteOff >= sizeOf(b.root) {
		return u16Of(b.root)
	}
	n := 0
	cur := b.root
	for cur != nil {
		left := sizeOf(cur.left)
		if byteOff < left {
			cur = cur.left
			continue
		}
		n += u16Of(cur.left)
		d := byteOff - left
		if d <= len(cur.data) {
			return n + utf16Count(cur.data[:d])
		}
		n += cur.nu16
		byteOff = d - len(cur.data)
		cur = cur.right
	}
	return n
}

// ByteAt returns the byte at off (0 when out of range).
func (b *Buffer) ByteAt(off int) byte {
	if b == nil || off < 0 || off >= b.Len() {
		return 0
	}
	s := b.Slice(off, off+1)
	if len(s) == 0 {
		return 0
	}
	return s[0]
}

// StepUTF16 moves one rune forward/backward from a UTF-16 offset, matching
// the editor's stepRuneUTF16 semantics (surrogate pairs count 2 units).
func (b *Buffer) StepUTF16(pos, dir int) (int, bool) {
	if b == nil || dir == 0 {
		return pos, false
	}
	total := b.UTF16Len()
	if pos < 0 || pos > total {
		return pos, false
	}
	byteOff := b.ByteOffsetForUTF16(pos)
	if dir < 0 {
		if byteOff <= 0 {
			return 0, false
		}
		lo := byteOff - 4
		if lo < 0 {
			lo = 0
		}
		r, _ := utf8.DecodeLastRuneInString(b.Slice(lo, byteOff))
		n := pos - 1
		if r > 0xFFFF {
			n = pos - 2
		}
		if n < 0 {
			n = 0
		}
		return n, true
	}
	if pos >= total {
		return total, false
	}
	if byteOff >= b.Len() {
		// 目标落在代理对中间时 ByteOffsetForUTF16 返回串尾；编辑器语义是 +1 单位.
		return pos + 1, true
	}
	hi := byteOff + 4
	if hi > b.Len() {
		hi = b.Len()
	}
	r, _ := utf8.DecodeRuneInString(b.Slice(byteOff, hi))
	n := pos + 1
	if r > 0xFFFF {
		n = pos + 2
	}
	if n > total {
		n = total
	}
	return n, true
}

// RuneAtUTF16 returns the rune starting at a UTF-16 offset (0 at end).
func (b *Buffer) RuneAtUTF16(pos int) rune {
	if b == nil {
		return 0
	}
	byteOff := b.ByteOffsetForUTF16(pos)
	if byteOff >= b.Len() {
		return 0
	}
	hi := byteOff + 4
	if hi > b.Len() {
		hi = b.Len()
	}
	r, _ := utf8.DecodeRuneInString(b.Slice(byteOff, hi))
	return r
}
