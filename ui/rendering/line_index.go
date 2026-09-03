package rendering

import "sort"

// lineIndex是M1的行索引+行高前缀和:偏移→行号与y→行号走二分,
// LineTop走O(1)查表.由TextLayout构建函数填充,查询前经valid校验.
// seq是引擎patch序号:增量更新就地改索引数组时递增,快照凭序号认领.
type lineIndex struct {
	starts []int
	ends   []int
	tops   []float64
	totalH float64
	n      int
	first  int
	last   int
	seq    uint64
}

func buildLineIndex(lines []TextLayoutLine, fontSize, lineSpacing float64) *lineIndex {
	idx := &lineIndex{n: len(lines)}
	if len(lines) == 0 {
		return idx
	}
	idx.starts = make([]int, len(lines))
	idx.ends = make([]int, len(lines))
	idx.tops = make([]float64, len(lines))
	top := 0.0
	for i, ln := range lines {
		idx.starts[i] = ln.StartByte
		idx.ends[i] = ln.EndByte
		idx.tops[i] = top
		h := ln.Height
		if h <= 0 {
			h = lineHeightFor(nil, fontSize, lineSpacing)
		}
		top += h
	}
	idx.totalH = top
	idx.first = lines[0].StartByte
	idx.last = lines[len(lines)-1].StartByte
	return idx
}

// valid校验索引与当前Lines仍对应同一份构建结果(长度+首末行起点).
func (x *lineIndex) valid(lines []TextLayoutLine) bool {
	if x == nil || x.n != len(lines) {
		return false
	}
	if len(lines) == 0 {
		return true
	}
	return x.first == lines[0].StartByte && x.last == lines[len(lines)-1].StartByte
}

// rowForOffset返回首个包含off的行(复刻老代码线性扫描的"首个命中"语义).
// 行区间在回绕断行处可能点重合(上一行End==下一行Start),此时归属上一行.
func (x *lineIndex) rowForOffset(off int) int {
	j := sort.Search(x.n, func(i int) bool { return x.starts[i] > off }) - 1
	if j < 0 {
		j = 0
	}
	if j >= x.n {
		j = x.n - 1
	}
	for j > 0 && off >= x.starts[j-1] && off <= x.ends[j-1] {
		j--
	}
	return j
}

// rowRangeForSpan返回与[start,end)相交的行区间[lo,hi],无交集时lo>hi.
func (x *lineIndex) rowRangeForSpan(start, end int) (lo, hi int) {
	lo = sort.Search(x.n, func(i int) bool { return x.ends[i] > start })
	hi = sort.Search(x.n, func(i int) bool { return x.starts[i] >= end }) - 1
	return lo, hi
}

// rowForY返回y所在行(复刻"首个y<top+h"语义,越界归末行).
func (x *lineIndex) rowForY(y, fontSize, lineSpacing float64, heights func(i int) float64) int {
	lo, hi := 0, x.n-1
	for lo < hi {
		mid := (lo + hi) / 2
		h := heights(mid)
		if h <= 0 {
			h = lineHeightFor(nil, fontSize, lineSpacing)
		}
		if y < x.tops[mid]+h {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo
}

// caretXForOffset在行内二分找ByteOff对应的X;无精确命中时exact=false.
func caretXForOffset(carets []GlyphCaret, off int) (x float64, exact bool) {
	j := sort.Search(len(carets), func(i int) bool { return carets[i].ByteOff >= off })
	if j < len(carets) && carets[j].ByteOff == off {
		return carets[j].X, true
	}
	return 0, false
}

// caretXInSpan二分找ByteOff所在区间左端X(老代码mid-grapheme回退的等价实现).
// off恰为某caret时调用方已由caretXForOffset处理,此处只处理严格区间内.
func caretXInSpan(carets []GlyphCaret, off int) (float64, bool) {
	j := sort.Search(len(carets), func(i int) bool { return carets[i].ByteOff > off })
	if j > 0 && j < len(carets) && off > carets[j-1].ByteOff {
		return carets[j-1].X, true
	}
	return 0, false
}
func offsetForX(carets []GlyphCaret, x float64) (byteOff int, affinity int) {
	// 注:carets为行内相对表,返回亦为行内相对偏移,调用方加行基址转绝对.
	j := sort.Search(len(carets), func(i int) bool { return carets[i].X > x })
	if j <= 0 {
		return carets[0].ByteOff, AffinityDownstream
	}
	if j >= len(carets) {
		return carets[len(carets)-1].ByteOff, AffinityDownstream
	}
	a, b := carets[j-1], carets[j]
	if x < (a.X+b.X)*0.5 {
		return a.ByteOff, AffinityDownstream
	}
	return b.ByteOff, AffinityDownstream
}
