package rendering

// HeightMap is the interval/gap height model for text lines:
// every unmeasured line uses one estimate, measured lines are sparse override
// nodes kept in an order-statistic treap. Offset/Total are O(log measured);
// IndexAtOffset is O(log lines * log measured). Memory is O(measured), not
// O(lines): 3M lines with the default 32K measured cap stays under 3MB.
type HeightMap struct {
	count       int
	estimate    float64
	total       float64
	root        *heightNode
	measured    int
	maxMeasured int
	// order is a FIFO of measured line indices for cap eviction. Stale entries
	// (line updated back to the estimate or already evicted) are skipped.
	order []int
	head  int
}

type heightNode struct {
	left, right *heightNode
	prio        uint32
	line        int
	delta       float64 // measured height - estimate
	count       int     // subtree node count
	sum         float64 // subtree delta sum
}

func hsize(n *heightNode) int {
	if n == nil {
		return 0
	}
	return n.count
}

func hsum(n *heightNode) float64 {
	if n == nil {
		return 0
	}
	return n.sum
}

func hupdate(n *heightNode) {
	if n == nil {
		return
	}
	n.count = 1 + hsize(n.left) + hsize(n.right)
	n.sum = n.delta + hsum(n.left) + hsum(n.right)
}

func hprio(line int) uint32 {
	x := uint32(line)*0x9E3779B1 + 0x85EBCA6B
	x ^= x >> 16
	x *= 0x7FEB352D
	x ^= x >> 15
	if x == 0 {
		return 1
	}
	return x
}

func hsplit(root *heightNode, key int) (l, r *heightNode) {
	if root == nil {
		return nil, nil
	}
	if root.line < key {
		l2, r2 := hsplit(root.right, key)
		root.right = l2
		hupdate(root)
		return root, r2
	}
	l2, r2 := hsplit(root.left, key)
	root.left = r2
	hupdate(root)
	return l2, root
}

func hmerge(a, b *heightNode) *heightNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.prio > b.prio {
		a.right = hmerge(a.right, b)
		hupdate(a)
		return a
	}
	b.left = hmerge(a, b.left)
	hupdate(b)
	return b
}

func hinsert(root, n *heightNode) *heightNode {
	if n == nil {
		return root
	}
	n.count = 1
	n.sum = n.delta
	l, r := hsplit(root, n.line)
	return hmerge(hmerge(l, n), r)
}

func herase(root *heightNode, line int) *heightNode {
	l, r := hsplit(root, line)
	_, r = hsplit(r, line+1)
	return hmerge(l, r)
}

func hfind(root *heightNode, line int) *heightNode {
	for root != nil {
		switch {
		case line < root.line:
			root = root.left
		case line > root.line:
			root = root.right
		default:
			return root
		}
	}
	return nil
}

// hprefix sums deltas for keys < line.
func hprefix(root *heightNode, line int) float64 {
	s := 0.0
	for root != nil {
		if line <= root.line {
			root = root.left
			continue
		}
		s += hsum(root.left) + root.delta
		root = root.right
	}
	return s
}

// NewHeightMap builds an estimation map. maxMeasured caps the sparse measured
// nodes (0 uses 32768); evicted lines fall back to the estimate.
func NewHeightMap(count int, estimate float64, maxMeasured int) *HeightMap {
	if count < 0 {
		count = 0
	}
	if estimate <= 0 {
		estimate = 20
	}
	if maxMeasured <= 0 {
		maxMeasured = 32768
	}
	return &HeightMap{
		count:       count,
		estimate:    estimate,
		total:       float64(count) * estimate,
		maxMeasured: maxMeasured,
	}
}

func (m *HeightMap) LineCount() int {
	if m == nil {
		return 0
	}
	return m.count
}

func (m *HeightMap) Estimate() float64 {
	if m == nil {
		return 0
	}
	return m.estimate
}

func (m *HeightMap) MeasuredCount() int {
	if m == nil {
		return 0
	}
	return m.measured
}

// TotalHeight returns the current total (estimates + measured deltas).
func (m *HeightMap) TotalHeight() float64 {
	if m == nil {
		return 0
	}
	return m.total
}

// OffsetOf returns the content Y of line i's top.
func (m *HeightMap) OffsetOf(i int) float64 {
	if m == nil {
		return 0
	}
	if i <= 0 {
		return 0
	}
	if i >= m.count {
		return m.total
	}
	return float64(i)*m.estimate + hprefix(m.root, i)
}

// IndexAtOffset returns the line containing content Y (clamped).
func (m *HeightMap) IndexAtOffset(y float64) int {
	if m == nil || m.count == 0 {
		return 0
	}
	if y <= 0 {
		return 0
	}
	if y >= m.total {
		return m.count - 1
	}
	lo, hi := 0, m.count-1
	for lo < hi {
		mid := (lo + hi) / 2
		if y < m.OffsetOf(mid+1) {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

func (m *HeightMap) ScrollOffsetForIndex(i int) float64 { return m.OffsetOf(i) }

// Measure records line i's height (estimates for non-positive input). A
// measured height equal to the estimate removes the override.
func (m *HeightMap) Measure(i int, h float64) bool {
	if m == nil || i < 0 || i >= m.count {
		return false
	}
	if h <= 0 {
		h = m.estimate
	}
	if h == m.estimate {
		if n := hfind(m.root, i); n != nil {
			m.total -= n.delta
			m.root = herase(m.root, i)
			m.measured--
		}
		return true
	}
	delta := h - m.estimate
	if n := hfind(m.root, i); n != nil {
		if n.delta == delta {
			return true
		}
		m.total += delta - n.delta
		m.root = herase(m.root, i)
		m.root = hinsert(m.root, &heightNode{line: i, delta: delta, prio: hprio(i)})
		return true
	}
	m.root = hinsert(m.root, &heightNode{line: i, delta: delta, prio: hprio(i)})
	m.total += delta
	m.measured++
	m.order = append(m.order, i)
	for m.measured > m.maxMeasured {
		if !m.evictOldest() {
			break
		}
	}
	if m.head > len(m.order)/2 && m.head > 1024 {
		m.order = append([]int(nil), m.order[m.head:]...)
		m.head = 0
	}
	return true
}

// Reset drops line i's measured override so it falls back to the estimate.
// Used when a patch edit changes one line's wrapped height.
func (m *HeightMap) Reset(i int) bool {
	if m == nil || i < 0 || i >= m.count {
		return false
	}
	n := hfind(m.root, i)
	if n == nil {
		return true
	}
	m.total -= n.delta
	m.root = herase(m.root, i)
	m.measured--
	return true
}

// Measured returns the stored measured height for line i.
func (m *HeightMap) Measured(i int) (float64, bool) {
	if m == nil {
		return 0, false
	}
	n := hfind(m.root, i)
	if n == nil {
		return 0, false
	}
	return m.estimate + n.delta, true
}

func (m *HeightMap) evictOldest() bool {
	for m.head < len(m.order) {
		line := m.order[m.head]
		m.head++
		n := hfind(m.root, line)
		if n == nil {
			continue
		}
		m.total -= n.delta
		m.root = herase(m.root, line)
		m.measured--
		return true
	}
	return false
}

// EachMeasured visits measured lines in ascending order (tests/observability).
func (m *HeightMap) EachMeasured(fn func(i int, h float64) bool) {
	if m == nil || fn == nil {
		return
	}
	var walk func(n *heightNode) bool
	walk = func(n *heightNode) bool {
		if n == nil {
			return true
		}
		if !walk(n.left) {
			return false
		}
		if !fn(n.line, m.estimate+n.delta) {
			return false
		}
		return walk(n.right)
	}
	walk(m.root)
}

// MeasuredCap reports the configured eviction cap.
func (m *HeightMap) MeasuredCap() int {
	if m == nil {
		return 0
	}
	return m.maxMeasured
}
