package rendering

// VirtualTextLines是M4的纵向行虚拟化+懒测量(ENGINE_TEXT_SCALE_PLAN §M4).
// 窗口数学直接复用 VirtualList 范式(含变高行前缀和),不加新概念:
// 定行高总高 O(1);变行高用 Fenwick 树做前缀和,Offset/Index/Total 均为
// O(log n)且实测只做增量修正.未显示过的行高度是估算值(I5):依赖精确总高
// 的逻辑不得直接使用 TotalHeight,须先 MeasureWindow 物化视口.
//
// 控件层接入即退回全量排版:不接入本类型,沿用既有 TextLayout 全量路径.
// 已接入 VirtualList 的控件可用 ExtentFunc 适配(变高模式),实测后调
// InvalidateExtents 刷新其前缀缓存.
type VirtualTextLines struct {
	count int
	fixed bool
	fixedH float64

	estimate  float64
	heights   []float64 // 变高模式每行当前高度,未测=估算
	measured  []bool
	bit       []float64 // Fenwick,bit[1..count],变高模式
	total     float64
	measuredN int
	measureFn func(i int) float64 // 懒实测回调,nil 则只走显式 Measure

	scrollY   float64
	viewportH float64
	cachePx   float64
	first     int
	last      int
}

func clampLineCount(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// NewVirtualTextLines创建定行高虚拟化:零按行内存,总高=行数×行高.
func NewVirtualTextLines(count int, lineHeight float64) *VirtualTextLines {
	if lineHeight <= 0 {
		lineHeight = 20
	}
	return &VirtualTextLines{
		count:     clampLineCount(count),
		fixed:     true,
		fixedH:    lineHeight,
		estimate:  lineHeight,
		total:     float64(clampLineCount(count)) * lineHeight,
		cachePx:   lineHeight * 2,
	}
}

// NewVariableVirtualTextLines创建变行高虚拟化:未测行先按 estimate 估算,
// 滚动到视口后经 MeasureWindow/EnsureMeasured 懒实测并增量修正总高.
func NewVariableVirtualTextLines(count int, estimate float64, measure func(i int) float64) *VirtualTextLines {
	if estimate <= 0 {
		estimate = 20
	}
	n := clampLineCount(count)
	v := &VirtualTextLines{
		count:     n,
		estimate:  estimate,
		measureFn: measure,
		cachePx:   estimate * 2,
		last:      0,
	}
	if n == 0 {
		return v
	}
	// 一次性 memset 约 16MB/1e6行(约10ms,首屏100ms预算内);首屏物化本身 O(可见行).
	v.heights = make([]float64, n)
	v.measured = make([]bool, n)
	v.bit = make([]float64, n+1)
	for i := range v.heights {
		v.heights[i] = estimate
		v.bit[i+1] = estimate
	}
	for i := 1; i <= n; i++ {
		if j := i + (i & -i); j <= n {
			v.bit[j] += v.bit[i]
		}
	}
	v.total = float64(n) * estimate
	return v
}

// TotalHeight返回内容总高.定行高 O(1);变行高为缓存的运行总和 O(1).
// 变高下含估算成分(I5),精确总高须先 MeasureWindow.
func (v *VirtualTextLines) TotalHeight() float64 {
	if v == nil {
		return 0
	}
	return v.total
}

// LineCount返回总行数.
func (v *VirtualTextLines) LineCount() int {
	if v == nil {
		return 0
	}
	return v.count
}

func (v *VirtualTextLines) bitAdd(i int, d float64) {
	for k := i + 1; k <= v.count; k += k & -k {
		v.bit[k] += d
	}
}

// bitSum返回前 k 行高度和 [0,k).
func (v *VirtualTextLines) bitSum(k int) float64 {
	s := 0.0
	for ; k > 0; k -= k & -k {
		s += v.bit[k]
	}
	return s
}

// OffsetOf返回第 i 行行首的内容区 Y.越界钳制: <0→0, ≥count→总高.
func (v *VirtualTextLines) OffsetOf(i int) float64 {
	if v == nil {
		return 0
	}
	if i <= 0 {
		return 0
	}
	if i >= v.count {
		return v.total
	}
	if v.fixed {
		return float64(i) * v.fixedH
	}
	return v.bitSum(i)
}

// IndexAtOffset返回包含内容区 Y 的行.钳制到 [0,count-1].
func (v *VirtualTextLines) IndexAtOffset(y float64) int {
	if v == nil || v.count <= 0 {
		return 0
	}
	if y <= 0 {
		return 0
	}
	if y >= v.total {
		return v.count - 1
	}
	if v.fixed {
		idx := int(y / v.fixedH)
		if idx >= v.count {
			return v.count - 1
		}
		return idx
	}
	return v.rowAt(y)
}

// rowAt找首个 prefix[i+1] > y 的行(与 VirtualList.indexContaining 同语义).
func (v *VirtualTextLines) rowAt(y float64) int {
	idx := 0
	step := 1
	for step<<1 <= v.count {
		step <<= 1
	}
	for ; step > 0; step >>= 1 {
		if nxt := idx + step; nxt <= v.count && v.bit[nxt] <= y {
			y -= v.bit[nxt]
			idx = nxt
		}
	}
	if idx >= v.count {
		return v.count - 1
	}
	return idx
}

// atOrAfter找首个行首偏移 ≥ y 的行(与 VirtualList.indexAtOrAfter 同语义).
func (v *VirtualTextLines) atOrAfter(y float64) int {
	if y <= 0 {
		return 0
	}
	if y >= v.total {
		return v.count
	}
	idx := 0
	step := 1
	for step<<1 <= v.count {
		step <<= 1
	}
	for ; step > 0; step >>= 1 {
		if nxt := idx + step; nxt <= v.count && v.bit[nxt] < y {
			y -= v.bit[nxt]
			idx = nxt
		}
	}
	return idx + 1
}

// ScrollOffsetForIndex返回把第 i 行置于视口顶部的 scrollY(滚动条定位).
func (v *VirtualTextLines) ScrollOffsetForIndex(i int) float64 {
	if v == nil || v.count <= 0 {
		return 0
	}
	if i < 0 {
		i = 0
	}
	if i >= v.count {
		i = v.count - 1
	}
	return v.OffsetOf(i)
}

// Measure记录第 i 行实测高度(>0,否则回退估算),总高按差值增量修正.
// 重复实测按新差值再次修正;定行高模式为无操作.
func (v *VirtualTextLines) Measure(i int, h float64) {
	if v == nil || v.fixed || i < 0 || i >= v.count {
		return
	}
	if h <= 0 {
		h = v.estimate
	}
	d := h - v.heights[i]
	if d == 0 && v.measured[i] {
		return
	}
	v.heights[i] = h
	v.bitAdd(i, d)
	v.total += d
	if !v.measured[i] {
		v.measured[i] = true
		v.measuredN++
	}
}

// EnsureMeasured确保第 i 行已实测(调 measureFn),返回当前高度.
func (v *VirtualTextLines) EnsureMeasured(i int) float64 {
	if v == nil || v.count <= 0 {
		return 0
	}
	if i < 0 {
		i = 0
	}
	if i >= v.count {
		i = v.count - 1
	}
	if v.fixed {
		return v.fixedH
	}
	if !v.measured[i] && v.measureFn != nil {
		v.Measure(i, v.measureFn(i))
	}
	return v.heights[i]
}

// SetViewport更新滚动窗口,返回待物化的行区间 [first,last).
func (v *VirtualTextLines) SetViewport(scrollY, viewportH float64) (first, last int) {
	if v == nil {
		return 0, 0
	}
	if v.count == 0 {
		v.first, v.last = 0, 0
		return 0, 0
	}
	if scrollY < 0 {
		scrollY = 0
	}
	if viewportH < 0 {
		viewportH = 0
	}
	v.scrollY, v.viewportH = scrollY, viewportH
	cache := v.cachePx
	if cache < 0 {
		cache = 0
	}
	startY := scrollY - cache
	if startY < 0 {
		startY = 0
	}
	endY := scrollY + viewportH + cache
	if v.fixed {
		first = int(startY / v.fixedH)
		last = int(endY/v.fixedH) + 1
	} else {
		first = v.rowAt(startY)
		last = v.atOrAfter(endY)
	}
	if first < 0 {
		first = 0
	}
	if last > v.count {
		last = v.count
	}
	if first > last {
		first = last
	}
	v.first, v.last = first, last
	return first, last
}

// Window返回当前待物化区间 [first,lastExclusive).
func (v *VirtualTextLines) Window() (first, lastExclusive int) {
	if v == nil {
		return 0, 0
	}
	return v.first, v.last
}

// MeasureWindow懒实测当前窗口内所有行,返回新增实测数.
// 只动窗口内行,窗口起点偏移不动(不跳变).
func (v *VirtualTextLines) MeasureWindow() int {
	if v == nil || v.fixed || v.measureFn == nil {
		return 0
	}
	n := 0
	for i := v.first; i < v.last; i++ {
		if !v.measured[i] {
			v.Measure(i, v.measureFn(i))
			n++
		}
	}
	return n
}

// IsMeasured报告第 i 行是否已实测(定行高恒 true).
func (v *VirtualTextLines) IsMeasured(i int) bool {
	if v == nil || i < 0 || v.count <= 0 {
		return false
	}
	if v.fixed {
		return true
	}
	if i >= v.count {
		return false
	}
	return v.measured[i]
}

// MeasuredCount返回已实测行数(定行高返回全行数).
func (v *VirtualTextLines) MeasuredCount() int {
	if v == nil {
		return 0
	}
	if v.fixed {
		return v.count
	}
	return v.measuredN
}

// ResetEstimate更换估算高度:未测行回到新估算并 O(n) 重建前缀和,
// 已测行保持.字体/字号变化时调用.
func (v *VirtualTextLines) ResetEstimate(h float64) {
	if v == nil || v.fixed || v.count == 0 {
		return
	}
	if h <= 0 {
		h = 20
	}
	v.estimate = h
	v.total = 0
	for i := 0; i < v.count; i++ {
		if !v.measured[i] {
			v.heights[i] = h
		}
		v.bit[i+1] = v.heights[i]
	}
	for i := 1; i <= v.count; i++ {
		if j := i + (i & -i); j <= v.count {
			v.bit[j] += v.bit[i]
		}
		v.total += v.heights[i-1]
	}
	v.cachePx = h * 2
}

// ExtentFunc把当前高度表导出为 VirtualList 的 ItemExtentAt,
// 供已接入 VirtualList 的控件复用.实测推进后须调 InvalidateExtents.
func (v *VirtualTextLines) ExtentFunc() ItemExtentFunc {
	return func(i int) float64 {
		if v == nil || v.count <= 0 {
			return 0
		}
		if i < 0 {
			i = 0
		}
		if i >= v.count {
			i = v.count - 1
		}
		if v.fixed {
			return v.fixedH
		}
		if h := v.heights[i]; h > 0 {
			return h
		}
		return v.estimate
	}
}
