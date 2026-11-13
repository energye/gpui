//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"sort"
	"strings"

	"github.com/energye/gpui/render/text"
)

// layoutUpdate是M1-d的增量更新:首尾diff定界,只重排被改的行/段,
// 后续行只做偏移平移(不重整形、不重分配caret数组).
//
// 快照语义:每次Update返回与引擎共享行表与行标记存储的快照(零拷贝
// 切片头)+全局Generation自增;caret/glyph数组与索引同样分享.同一引擎上
// 再次Update后,旧快照即过期——生产链无跨击键留存(单线程事件循环,构建即
// 换指针),单测不得在二次Update后重读旧快照(含LineGen;同文更新不改标记,
// 跨同文快照比值仍成立,见TestLayoutUpdate_Generation).
type layoutLive struct {
	text      string
	face      text.Face
	size      float64
	maxW      float64
	spacing   float64
	approx    float64
	maxLines  int
	overflow  TextOverflow
	rows      []TextLayoutLine
	marks     []uint64
	idx       *lineIndex
	seq       uint64
	ok        bool
	hasCR     bool
	spansHit  int
	spansMiss int
	// shift是行偏移的后缀增量(绝对偏移=行base+shift.at(i)).
	// 长度变化编辑不再逐行改写tail偏移(2026-11-13 G1:1e6档逐行平移约270µs).
	shift *rowShift
	// lastRow/lastOK缓存上次替换的单行区间:击键集中于光标附近,
	// 命中时免两次O(log n)二分(G1 1e6档剩余成本的主项).
	lastRow int
	lastOK  bool
}

func (lv *layoutLive) shiftAt(i int) int { return lv.shift.at(i) }

// matches报告常驻引擎是否为这组参数构建的.文本/区间/回车判定留给调用方,
// 此处只合并8项参数比较,两处入口语义逐项一致.
func (lv *layoutLive) matches(face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, maxLines int, overflow TextOverflow) bool {
	return lv.ok && lv.face == face && lv.size == fontSize && lv.maxW == maxWidth &&
		lv.spacing == lineSpacing && lv.approx == approxCharW &&
		lv.maxLines == maxLines && lv.overflow == overflow
}

// update增量构建.首建或参数变化走全量;否则diff patch.
// 每次调用全局Generation自增一次(与BuildTextLayoutEx语义一致).
func (c *layoutCache) update(textStr string, face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, maxLines int, overflow TextOverflow) *TextLayout {
	c.mu.Lock()
	defer c.mu.Unlock()
	gen := textLayoutGen.Add(1)
	lv := &c.live
	if !lv.matches(face, fontSize, maxWidth, lineSpacing, approxCharW, maxLines, overflow) ||
		hasCR(textStr) || hasCR(lv.text) {
		hit, miss := lv.spansHit, lv.spansMiss
		lines, marks := c.cachedLinesFull(textStr, face, fontSize, maxWidth, lineSpacing, approxCharW, gen)
		*lv = layoutLive{text: textStr, face: face, size: fontSize, maxW: maxWidth,
			spacing: lineSpacing, approx: approxCharW, maxLines: maxLines, overflow: overflow,
			rows: lines, marks: marks, ok: true, hasCR: hasCR(textStr)}
		lv.spansHit, lv.spansMiss = hit, miss
		lv.idx = buildLineIndex(lv.rows, fontSize, lineSpacing)
		lv.seq = 0
		lv.idx.seq = 0
		return c.snapshot(textStr, face, fontSize, maxWidth, lineSpacing, maxLines, overflow, gen)
	}
	if textStr == lv.text {
		return c.snapshot(textStr, face, fontSize, maxWidth, lineSpacing, maxLines, overflow, gen)
	}
	oldA, oldB, newA, newB := diffSpan(lv.text, textStr)
	return c.applyPatch(textStr, oldA, oldB, newA, newB, face, fontSize, maxWidth, lineSpacing, approxCharW, maxLines, overflow, gen)
}

// updateSpan同update,但变更区间由调用方(Editor→sync)给出,免逐字节diff.
// 区间经边界+长度方程+两侧抽查三重校验,任一不过回退diff(保正确).
// \r:live有\r走全量;否则只扫新区段(插入的\r只可能在新区段).
func (c *layoutCache) updateSpan(textStr string, face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, maxLines int, overflow TextOverflow, sp editSpan) *TextLayout {
	c.mu.Lock()
	defer c.mu.Unlock()
	gen := textLayoutGen.Add(1)
	lv := &c.live
	if !lv.matches(face, fontSize, maxWidth, lineSpacing, approxCharW, maxLines, overflow) ||
		lv.hasCR || !checkSpan(lv.text, textStr, sp) || spanHasCR(textStr, sp) {
		hit, miss := lv.spansHit, lv.spansMiss+1
		lines, marks := c.cachedLinesFull(textStr, face, fontSize, maxWidth, lineSpacing, approxCharW, gen)
		*lv = layoutLive{text: textStr, face: face, size: fontSize, maxW: maxWidth,
			spacing: lineSpacing, approx: approxCharW, maxLines: maxLines, overflow: overflow,
			rows: lines, marks: marks, ok: true, hasCR: hasCR(textStr)}
		lv.spansHit, lv.spansMiss = hit, miss
		lv.idx = buildLineIndex(lv.rows, fontSize, lineSpacing)
		lv.seq = 0
		lv.idx.seq = 0
		return c.snapshot(textStr, face, fontSize, maxWidth, lineSpacing, maxLines, overflow, gen)
	}
	if sp.oldA == sp.oldB && sp.newA == sp.newB && textStr == lv.text {
		// 空区间才需整串确认同文;非空区间下串必然已变(等长改字时整串
		// ==是O(n)全量比较,正是击键瓶颈),直接走patch(等文patch结果一致).
		return c.snapshot(textStr, face, fontSize, maxWidth, lineSpacing, maxLines, overflow, gen)
	}
	lv.spansHit++
	return c.applyPatch(textStr, sp.oldA, sp.oldB, sp.newA, sp.newB, face, fontSize, maxWidth, lineSpacing, approxCharW, maxLines, overflow, gen)
}

// checkSpan三重校验:边界合法+长度方程+新旧串在区间两侧各抽查32字节一致.
func checkSpan(oldText, newText string, sp editSpan) bool {
	if sp.oldA < 0 || sp.oldB < sp.oldA || sp.oldB > len(oldText) ||
		sp.newA < 0 || sp.newB < sp.newA || sp.newB > len(newText) ||
		(sp.newB-sp.newA)-(sp.oldB-sp.oldA) != len(newText)-len(oldText) ||
		sp.newA != sp.oldA {
		return false
	}
	const probe = 32
	pre := sp.newA
	if pre > probe {
		pre = probe
	}
	if newText[sp.newA-pre:sp.newA] != oldText[sp.oldA-pre:sp.oldA] {
		return false
	}
	oldSuf, newSuf := len(oldText)-sp.oldB, len(newText)-sp.newB
	if oldSuf != newSuf {
		return false
	}
	suf := oldSuf
	if suf > probe {
		suf = probe
	}
	if newText[sp.newB:sp.newB+suf] != oldText[sp.oldB:sp.oldB+suf] {
		return false
	}
	return true
}

// spanHasCR只扫新区段(插入的\r只可能在此).
func spanHasCR(textStr string, sp editSpan) bool {
	for i := sp.newA; i < sp.newB && i < len(textStr); i++ {
		if textStr[i] == '\r' {
			return true
		}
	}
	return false
}

// applyPatch是update/updateSpan的公共patch尾:定界→patch→快照.
// patchRows报告索引是否变化;纯改字快径下索引逐项全同,不碰序号
// (新旧快照共享同一索引对象,查询依然有效).
func (c *layoutCache) applyPatch(textStr string, oldA, oldB, newA, newB int, face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, maxLines int, overflow TextOverflow, gen uint64) *TextLayout {
	lv := &c.live
	delta := (newB - newA) - (oldB - oldA)
	if c.patchRows(textStr, oldA, oldB, newA, newB, delta, face, fontSize, maxWidth, lineSpacing, approxCharW, gen) {
		lv.seq++
		lv.idx.seq = lv.seq
	}
	lv.text = textStr
	return c.snapshot(textStr, face, fontSize, maxWidth, lineSpacing, maxLines, overflow, gen)
}

// diffSpan返回首尾公共区间的变化范围:[oldA,oldB)→[newA,newB).
func diffSpan(old, new string) (oldA, oldB, newA, newB int) {
	maxPre := len(old)
	if len(new) < maxPre {
		maxPre = len(new)
	}
	pre := 0
	for pre < maxPre && old[pre] == new[pre] {
		pre++
	}
	oldSuf, newSuf := len(old), len(new)
	for oldSuf > pre && newSuf > pre && old[oldSuf-1] == new[newSuf-1] {
		oldSuf--
		newSuf--
	}
	return pre, oldSuf, pre, newSuf
}

// countBreaks统计区间内的硬换行数(越界钳制).
func countBreaks(s string, a, b int) int {
	if a < 0 {
		a = 0
	}
	if b > len(s) {
		b = len(s)
	}
	if a >= b {
		return 0
	}
	return strings.Count(s[a:b], "\n")
}

// patchRows重建落入变化区的硬行/段,其余行只平移偏移.
// lo=首个End>=oldA的行(含插入点所在行);hi=max(lo,末个Start<oldB的行).
// 行表按字节有序,lo/hi二分定位(O(log n),原线性扫是击键主瓶颈).
// 返回索引是否变化:等长改字且行数/偏移/行高全同时原地换行内容,
// 索引逐项全同,直接返回false(零O(n)工作,尾部簇缓存继续有效).
func (c *layoutCache) patchRows(textStr string, oldA, oldB, newA, newB, delta int, face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, gen uint64) bool {
	lv := &c.live
	if textStr == "" {
		// 与BuildTextLayoutEx空串早返一致:零行(首建空串走cachedLinesFull同分支).
		lv.rows = nil
		lv.marks = nil
		lv.idx = &lineIndex{}
		lv.shift = nil
		lv.lastOK = false
		return true
	}
	// 变化区向硬行边界扩展(只扫局部,不扫全文;memchr级查找,长段约10倍速).
	segA := 0
	if i := strings.LastIndexByte(textStr[:newA], '\n'); i >= 0 {
		segA = i + 1
	}
	segB := len(textStr)
	if i := strings.IndexByte(textStr[newB:], '\n'); i >= 0 {
		segB = newB + i
	}
	lo, hi := len(lv.rows), -1
	fastHit := false
	fastShiftVal := 0
	if len(lv.rows) > 0 {
		// 旧串变化区同样向硬行边界扩展(删换行会吞并两侧行;
		// 首判用>=以吞下正好落在改动点上的零宽行).
		oldSegA := 0
		if i := strings.LastIndexByte(lv.text[:oldA], '\n'); i >= 0 {
			oldSegA = i + 1
		}
		oldSegB := len(lv.text)
		if i := strings.IndexByte(lv.text[oldB:], '\n'); i >= 0 {
			oldSegB = oldB + i
		}
		// 上次替换的单行仍覆盖本次变化区时免二分(击键局部性).
		if lv.lastOK && lv.lastRow >= 0 && lv.lastRow < len(lv.rows) {
			r := lv.lastRow
			rs := lv.shiftAt(r)
			okLo := lv.rows[r].EndByte+rs >= oldSegA &&
				(r == 0 || lv.rows[r-1].EndByte+lv.shiftAt(r-1) < oldSegA)
			okHi := r == len(lv.rows)-1 || lv.rows[r+1].StartByte+lv.shiftAt(r+1) >= oldSegB
			if okLo && okHi {
				lo, hi = r, r
				fastShiftVal = rs
				fastHit = true
			}
		}
		if hi < 0 {
			// 行起止单调,二分等价于原线性首/末命中(带后缀增量派生绝对偏移).
			lo = sort.Search(len(lv.rows), func(i int) bool { return lv.rows[i].EndByte+lv.shiftAt(i) >= oldSegA })
			hi = sort.Search(len(lv.rows), func(i int) bool { return lv.rows[i].StartByte+lv.shiftAt(i) >= oldSegB }) - 1
			if hi < lo {
				hi = lo
			}
		}
	}
	var fresh []TextLayoutLine
	var freshMarks []uint64
	span := textStr[segA:segB]
	// 单硬行判定:两侧由边界扩展保证无换行,只需查本次新区段.
	// 击键新区段通常1字符,此处O(1);多行粘贴回退全量切分(语义不变).
	single := true
	for i := newA; i < newB && i < len(textStr); i++ {
		if textStr[i] == '\n' {
			single = false
			break
		}
	}
	var parts []hardPart
	if single {
		parts = []hardPart{{text: span, start: segA, end: segB}}
	} else {
		parts = splitHardLines(span)
		for i := range parts {
			parts[i].start += segA
			parts[i].end += segA
		}
	}
	lh := lineHeightFor(face, fontSize, lineSpacing)
	if maxWidth > 0 && face != nil {
		fresh, freshMarks = c.wrapSpan(parts, face, fontSize, maxWidth, lh, gen)
	} else if maxWidth > 0 {
		fresh, freshMarks = c.estSpan(parts, fontSize, maxWidth, approxCharW, lh, gen)
	} else {
		if row, marks, ok := c.reuseSingleRow(textStr, parts, lo, hi, face, fontSize, lh, gen); ok {
			// 单行快径命中(遗留#1):只重整变更 run,前后缀复用。
			fresh, freshMarks = row, marks
		} else {
			fresh, freshMarks = c.rowSpan(parts, face, fontSize, lh, gen)
		}
	}
	removedH := 0.0
	// Hard-break balance: pure break edits inside empty runs change the
	// hard-line count by (nNew - nOld); replaced old rows must outnumber
	// fresh rows by exactly that, or phantom zero-width rows survive.
	// Empty rows map 1:1 to hard lines in every mode, non-empty regions
	// keep the existing boundaries.
	if nOld, nNew := countBreaks(lv.text, oldA, oldB), countBreaks(textStr, newA, newB); nOld != nNew {
		empty := len(fresh) > 0
		for _, r := range fresh {
			if r.StartByte != r.EndByte {
				empty = false
				break
			}
		}
		if empty {
			for i := lo; i <= hi && i < len(lv.rows); i++ {
				if lv.rows[i].StartByte != lv.rows[i].EndByte {
					empty = false
					break
				}
			}
		}
		if empty {
			if want := len(fresh) + (nOld - nNew); want > hi-lo+1 {
				hi = lo + want - 1
				if hi > len(lv.rows)-1 {
					hi = len(lv.rows) - 1
				}
			}
		}
	}
	for i := lo; i <= hi && i < len(lv.rows); i++ {
		removedH += lv.rows[i].Height
	}
	sameCount := len(fresh) == hi-lo+1
	sLo := fastShiftVal
	if !fastHit {
		sLo = lv.shiftAt(lo)
		if len(lv.rows) > 0 && lv.shiftAt(hi) != sLo {
			// 旧后缀增量事件落在本次替换区间内,先烘焙成绝对偏移再patch.
			c.compactOffsets()
			sLo = 0
		}
	}
	// 纯改字快径:行数相同且总长不变时,若新区段每行起止/行高与旧行
	// 逐项一致,索引数组逐项全同——原地换行内容即可,索引不动.
	if sameCount && delta == 0 && len(lv.rows) > 0 {
		same := true
		for i := range fresh {
			o := &lv.rows[lo+i]
			if fresh[i].StartByte != o.StartByte+sLo || fresh[i].EndByte != o.EndByte+sLo || fresh[i].Height != o.Height {
				same = false
				break
			}
		}
		if same {
			subRowShift(fresh, sLo)
			copy(lv.rows[lo:], fresh)
			copy(lv.marks[lo:], freshMarks)
			lv.lastRow, lv.lastOK = lo, lo == hi
			return false
		}
	}
	if sameCount && len(lv.rows) > 0 {
		// 行数不变:只换本段行内容;尾部偏移记一笔后缀增量,不再逐行平移.
		subRowShift(fresh, sLo)
		copy(lv.rows[lo:], fresh)
		copy(lv.marks[lo:], freshMarks)
		if delta != 0 {
			if lv.shift == nil {
				lv.shift = newRowShift(len(lv.rows))
			}
			lv.shift.addSuffix(hi+1, delta)
		}
		c.patchIndex(lo, hi, fresh, removedH, delta)
		lv.lastRow, lv.lastOK = lo, lo == hi
		return true
	}
	// 行数变化:Flutter式整段重建(O(n));就地splice,偏移由增量树映射,
	// 不烘焙整表、不逐行平移.
	oldN := len(lv.rows)
	f := len(fresh)
	remove := hi - lo + 1
	newN := oldN - remove + f
	subRowShift(fresh, sLo)
	if newN > oldN {
		grow := newN - oldN
		lv.rows = append(lv.rows, make([]TextLayoutLine, grow)...)
		lv.marks = append(lv.marks, make([]uint64, grow)...)
		copy(lv.rows[hi+1+grow:], lv.rows[hi+1:oldN])
		copy(lv.marks[hi+1+grow:], lv.marks[hi+1:oldN])
	} else if newN < oldN {
		copy(lv.rows[lo+f:], lv.rows[hi+1:oldN])
		copy(lv.marks[lo+f:], lv.marks[hi+1:oldN])
		clear(lv.rows[newN:oldN])
		lv.rows = lv.rows[:newN]
		lv.marks = lv.marks[:newN]
	}
	copy(lv.rows[lo:lo+f], fresh)
	copy(lv.marks[lo:lo+f], freshMarks)
	if lv.shift != nil {
		lv.shift = lv.shift.reshape(lo, hi, f, newN, delta)
	} else if delta != 0 && newN > 0 {
		lv.shift = newRowShift(newN)
		lv.shift.addSuffix(lo+f, delta)
	}
	c.patchIndex(lo, hi, fresh, removedH, delta)
	lv.lastOK = false
	return true
}

// subRowShift把绝对偏移行转回base(减掉该区间统一的累计后缀增量).
func subRowShift(rows []TextLayoutLine, s int) {
	if s == 0 {
		return
	}
	for i := range rows {
		rows[i].StartByte -= s
		rows[i].EndByte -= s
	}
}

// compactOffsets把行/索引里的base偏移烘焙成绝对偏移并清空增量树.
// 结构性编辑(行数变化)或替换区间内含旧增量事件时使用,代价O(rows).
func (c *layoutCache) compactOffsets() {
	lv := &c.live
	if lv.shift == nil {
		return
	}
	pre := lv.shift.drain()
	n := len(lv.rows)
	if len(pre) < n {
		n = len(pre)
	}
	for i := 0; i < n; i++ {
		if s := pre[i]; s != 0 {
			lv.rows[i].StartByte += s
			lv.rows[i].EndByte += s
		}
	}
	if lv.idx != nil {
		m := lv.idx.n
		if m > n {
			m = n
		}
		for i := 0; i < m; i++ {
			if s := pre[i]; s != 0 {
				lv.idx.starts[i] += s
				lv.idx.ends[i] += s
			}
		}
	}
	lv.shift = nil
}

// patchIndex同步行索引:影响区替换,后续区起止平移,行顶按新增高度顺延
// (影响区新高度与旧高度之差累加到后续行顶;行高均匀时行数不变则差为0).
// removedH为被替换行的原高度和(调用方在splice前算好传入).
// 行数不变时原地改数组(零分配,值与重建逐项一致);行数变才重建.
func (c *layoutCache) patchIndex(lo, hi int, fresh []TextLayoutLine, removedH float64, delta int) {
	lv := &c.live
	old := lv.idx
	n := old.n - (hi - lo + 1) + len(fresh)
	if n == 0 {
		lv.idx = &lineIndex{}
		return
	}
	if len(fresh) == hi-lo+1 && n == old.n && old.n > 0 {
		for k, r := range fresh {
			i := lo + k
			old.starts[i] = r.StartByte
			old.ends[i] = r.EndByte
		}
		top := 0.0
		if lo > 0 {
			top = old.tops[lo]
		}
		freshH := 0.0
		for _, r := range fresh {
			freshH += r.Height
		}
		shift := freshH - removedH
		old.totalH -= removedH
		for k, r := range fresh {
			i := lo + k
			old.tops[i] = top
			top += r.Height
			old.totalH += r.Height
		}
		if shift != 0 {
			// 行偏移走rowShift后缀增量,这里只处理行高变化对行顶的影响.
			for i := hi + 1; i < old.n; i++ {
				old.tops[i] += shift
			}
		}
		old.first = old.starts[0]
		old.last = old.starts[old.n-1]
		return
	}
	// 行数变化:就地插入/删除索引项;tail字符偏移交给rowShift,
	// 只有行高变化才平移tail行顶(整段重建,O(n)一次).
	f := len(fresh)
	oldN := old.n
	if n > oldN {
		grow := n - oldN
		old.starts = append(old.starts, make([]int, grow)...)
		old.ends = append(old.ends, make([]int, grow)...)
		old.tops = append(old.tops, make([]float64, grow)...)
		copy(old.starts[hi+1+grow:], old.starts[hi+1:oldN])
		copy(old.ends[hi+1+grow:], old.ends[hi+1:oldN])
		copy(old.tops[hi+1+grow:], old.tops[hi+1:oldN])
	} else if n < oldN {
		copy(old.starts[lo+f:], old.starts[hi+1:oldN])
		copy(old.ends[lo+f:], old.ends[hi+1:oldN])
		copy(old.tops[lo+f:], old.tops[hi+1:oldN])
		old.starts = old.starts[:n]
		old.ends = old.ends[:n]
		old.tops = old.tops[:n]
	}
	top := 0.0
	if lo > 0 && lo < oldN {
		top = old.tops[lo]
	}
	freshH := 0.0
	for k, r := range fresh {
		old.starts[lo+k] = r.StartByte
		old.ends[lo+k] = r.EndByte
		old.tops[lo+k] = top
		top += r.Height
		freshH += r.Height
	}
	if hshift := freshH - removedH; hshift != 0 {
		for i := lo + f; i < n; i++ {
			old.tops[i] += hshift
		}
	}
	old.n = n
	old.totalH = old.totalH - removedH + freshH
	if n > 0 {
		old.first = old.starts[0]
		old.last = old.starts[n-1]
	}
	lv.idx = old
}

// rowSpan/wrapSpan/estSpan为变化区局部构建(调用方已切分为硬段并变基到
// 全局,单段时免全量换行扫描),命中跨文档缓存.
func (c *layoutCache) rowSpan(parts []hardPart, face text.Face, fontSize, lh float64, gen uint64) ([]TextLayoutLine, []uint64) {
	return c.rowLinesParts(parts, face, fontSize, lh, gen)
}

func (c *layoutCache) wrapSpan(parts []hardPart, face text.Face, fontSize, maxWidth, lh float64, gen uint64) ([]TextLayoutLine, []uint64) {
	return c.partLines(parts, face, fontSize, maxWidth, lh, 0, gen, c.segs,
		func(seg string) []text.WrapResult { return wrapFaceResults(seg, face, maxWidth) })
}

func (c *layoutCache) estSpan(parts []hardPart, fontSize, maxWidth, approxCharW, lh float64, gen uint64) ([]TextLayoutLine, []uint64) {
	return c.partLines(parts, nil, fontSize, maxWidth, lh, approxCharW, gen, c.segs,
		func(seg string) []text.WrapResult {
			return wrapEstResults(seg, maxWidth, fontSize, approxCharW)
		})
}

// snapshot从live状态装配快照:行表与行标记切片头均与引擎共享(零拷贝,
// 击键零O(n)分配),全局Generation照常自增;caret/glyph数组与索引分享
// (序号一致时).快照随下一次Update过期(见文件头快照语义).外部访问器均为
// 值拷贝或只读(Line/LineCarets/LineGlyphs),调用方无法经快照改引擎状态.
func (c *layoutCache) snapshot(textStr string, face text.Face, fontSize, maxWidth, lineSpacing float64, maxLines int, overflow TextOverflow, gen uint64) *TextLayout {
	lv := &c.live
	rows := lv.rows
	marks := lv.marks
	truncated := false
	if maxLines > 0 && len(rows) > maxLines {
		rows = rows[:maxLines]
		marks = marks[:maxLines]
		truncated = true
	}
	// 行高由构建器保证(物化行必填正行高,查询侧另有兜底),此处不再逐行扫描.
	l := &TextLayout{Text: textStr, lines: rows, FontSize: fontSize, LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxWidth, Face: face, MaxLines: maxLines, Overflow: overflow, Truncated: truncated, LineGen: marks, shift: lv.shift}
	if len(rows) == len(lv.rows) {
		l.idx = lv.idx
		l.idxSeq = lv.seq
	} else {
		l.idx = buildLineIndex(rows, fontSize, lineSpacing)
		l.idxSeq = l.idx.seq
	}
	return l
}
