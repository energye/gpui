package rendering

// rowShift记录"从某行起整体平移"的后缀增量,避免每次长度变化编辑逐行改写
// 行/索引里的绝对偏移。行里存base偏移,绝对偏移=base+at(i)。
// 对齐Flutter的版面语义:版面结果是不可变缓存,行度量按需派生
// (text_painter.dart:markNeedsLayout/layout/computeLineMetrics),不回写每行。
type rowShift struct {
	tree []int //1-based Fenwick,底层是差分数组
	diff []int //logical row index -> suffix delta; drain可O(n)烘焙
	n    int
}

func newRowShift(n int) *rowShift {
	if n <= 0 {
		return nil
	}
	return &rowShift{tree: make([]int, n+2), diff: make([]int, n+1), n: n}
}

// addSuffix对[lo,n)的所有行累加delta;lo==n表示后面没有行,忽略。
func (s *rowShift) addSuffix(lo, delta int) {
	if s == nil || delta == 0 || lo < 0 || lo >= s.n {
		return
	}
	s.diff[lo] += delta
	for i := lo + 1; i <= s.n; i += i & -i {
		s.tree[i] += delta
	}
}

// drain把diff原地转成逐行前缀增量并返回(长度n),随后调用方应丢弃本树。
// 结构变化时用它代替逐行Fenwick查询,烘焙一遍O(n)。
func (s *rowShift) drain() []int {
	if s == nil {
		return nil
	}
	acc := 0
	for i := 0; i < s.n; i++ {
		acc += s.diff[i]
		s.diff[i] = acc
	}
	return s.diff
}

// reshape把旧树映射到新行数:删除[lo,hi+1)、插入f行(统一沿用sLo)、
// 旧tail行shift整体+delta。只搬运/重建差分数组,不改任何行。
func (s *rowShift) reshape(lo, hi, f, newN, delta int) *rowShift {
	if newN <= 0 {
		return nil
	}
	oldN := s.n
	ns := &rowShift{tree: make([]int, newN+2), diff: make([]int, newN+1), n: newN}
	copy(ns.diff, s.diff[:lo])
	if f > 0 {
		// 首条新行沿用lo处的旧事件,其余新行同shift.
		if lo < len(s.diff) {
			ns.diff[lo] = s.diff[lo]
		}
		if hi+1 < oldN {
			dst := lo + f
			ns.diff[dst] = s.diff[hi+1] + delta
			copy(ns.diff[dst+1:], s.diff[hi+2:oldN+1])
		}
	} else if lo < newN {
		// 纯删除:新行lo=旧hi+1,shift=P[hi+1]+delta;P[hi]=sLo,
		// 故差分=s.diff[lo]+s.diff[hi+1]+delta.
		ns.diff[lo] = s.diff[lo] + s.diff[hi+1] + delta
		if hi+2 <= oldN {
			copy(ns.diff[lo+1:], s.diff[hi+2:oldN+1])
		}
	}
	ns.rebuild()
	return ns
}

// rebuild从diff重建Fenwick,O(n).
func (s *rowShift) rebuild() {
	for i := 1; i <= s.n; i++ {
		s.tree[i] = s.diff[i-1]
	}
	for i := 1; i <= s.n; i++ {
		if j := i + (i & -i); j <= s.n {
			s.tree[j] += s.tree[i]
		}
	}
}

// at返回第i行的累计平移量,越界按首末行钳制。
func (s *rowShift) at(i int) int {
	if s == nil {
		return 0
	}
	if i < 0 {
		i = 0
	}
	if i >= s.n {
		i = s.n - 1
	}
	sum := 0
	for k := i + 1; k > 0; k -= k & -k {
		sum += s.tree[k]
	}
	return sum
}
