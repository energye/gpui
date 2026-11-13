package rendering

// rowShift记录"从某行起整体平移"的后缀增量,避免每次长度变化编辑逐行改写
// 行/索引里的绝对偏移。行里存base偏移,绝对偏移=base+at(i)。
// 对齐Flutter的版面语义:版面结果是不可变缓存,行度量按需派生
// (text_painter.dart:markNeedsLayout/layout/computeLineMetrics),不回写每行。
type rowShift struct {
	tree []int //1-based Fenwick,底层是差分数组
	n    int
}

func newRowShift(n int) *rowShift {
	if n <= 0 {
		return nil
	}
	return &rowShift{tree: make([]int, n+2), n: n}
}

// addSuffix对[lo,n)的所有行累加delta;lo==n表示后面没有行,忽略。
func (s *rowShift) addSuffix(lo, delta int) {
	if s == nil || delta == 0 || lo < 0 || lo >= s.n {
		return
	}
	for i := lo + 1; i <= s.n; i += i & -i {
		s.tree[i] += delta
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
