package rendering

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// 行内整形复用(遗留#1,M5§9):单行文档击键只重整变更 run,前后缀拼接。
// 后缀 X 为整体平移(旧值+位移),与全量重累加有末位 ulp 差,测试按 1e-9 断言。

// reuseMinLineBytes是复用门槛:短行走全量(输出与此前逐位一致),
// 只在长行启用复用。
const reuseMinLineBytes = 4096

func runeBoundary(s string, i int) bool {
	return i == 0 || i == len(s) || (i >= 0 && i < len(s) && utf8.RuneStart(s[i]))
}

// tryReuseShapedRow重建单个非回绕整形行。oldRow 数组永不改动,
// 结果与 oldRow 无共享。reused=false 时调用方全量重建。
func tryReuseShapedRow(oldRow TextLayoutLine, oldLine, newLine string, face text.Face, lh float64) (TextLayoutLine, bool) {
	row, _, ok := tryReuseShapedRowSegs(oldRow, oldLine, nil, newLine, face, lh)
	return row, ok
}

// tryReuseShapedRowSegs同上，但用调用方缓存的上次分段增量得到新分段
// （`render/text.SegmentReuse` 前后缀复用），免整行 SegmentText。
// oldSegs 为空时退化为全量分段；返回的 newSegs 与本次 newLine 对应，
// 调用方缓存后供下次击键复用。
func tryReuseShapedRowSegs(oldRow TextLayoutLine, oldLine string, oldSegs []text.Segment, newLine string, face text.Face, lh float64) (TextLayoutLine, []text.Segment, bool) {
	fail := func() (TextLayoutLine, []text.Segment, bool) { return TextLayoutLine{}, nil, false }
	if face == nil || strings.IndexByte(oldLine, '\n') >= 0 || strings.IndexByte(newLine, '\n') >= 0 {
		return fail()
	}
	gruns := oldRow.GlyphRuns
	if len(gruns) == 0 || len(gruns) != len(oldRow.runRTL) || gruns[0].Start != 0 {
		return fail()
	}
	for i, g := range gruns {
		if g.Start > g.End || g.End > len(oldRow.Glyphs) {
			return fail()
		}
		if i > 0 && g.Start != gruns[i-1].End {
			return fail()
		}
	}
	oldA, oldB, newA, newB := diffSpan(oldLine, newLine)
	var newSegs []text.Segment
	if len(oldSegs) == 0 {
		newSegs = text.SegmentText(newLine)
	} else {
		newSegs = text.SegmentReuse(oldLine, oldSegs, newLine, oldA, oldB, newA, newB)
	}
	newRuns := itemizeRunsWithSegs(newLine, face, newSegs)
	if len(newRuns) == 0 {
		return fail()
	}
	// 前缀 run:成对一致且完全落在公共前缀内。
	pre := 0
	for pre < len(gruns) && pre < len(newRuns) {
		g := gruns[pre]
		r := newRuns[pre]
		if g.TextEnd > newA || r.end > newA {
			break
		}
		if g.Face != r.face || g.TextStart != r.start || g.TextEnd != r.end || oldRow.runRTL[pre] != r.rtl {
			break
		}
		pre++
	}
	// 后缀 run:成对一致(模全局字节平移)且完全落在公共后缀内。
	shiftBytes := len(newLine) - len(oldLine)
	suf := 0
	for suf < len(gruns)-pre && suf < len(newRuns)-pre {
		gi := len(gruns) - 1 - suf
		ri := len(newRuns) - 1 - suf
		g := gruns[gi]
		r := newRuns[ri]
		if g.TextStart < oldB || r.start < newB {
			break
		}
		if g.Face != r.face || g.TextStart+shiftBytes != r.start ||
			g.TextEnd+shiftBytes != r.end || oldRow.runRTL[gi] != r.rtl {
			break
		}
		suf++
	}
	oldEnd := len(gruns) - suf
	newEnd := len(newRuns) - suf
	var winA, winB, oldWinA, oldWinB int
	if pre < newEnd {
		winA, winB = newRuns[pre].start, newRuns[newEnd-1].end
	} else {
		winA, winB = newA, newA
	}
	if pre < oldEnd {
		oldWinA, oldWinB = gruns[pre].TextStart, gruns[oldEnd-1].TextEnd
	} else {
		oldWinA, oldWinB = oldA, oldA
	}
	if !runeBoundary(newLine, winA) || !runeBoundary(newLine, winB) ||
		!runeBoundary(oldLine, oldWinA) || !runeBoundary(oldLine, oldWinB) {
		return fail()
	}
	// 字形前后缀分界。
	preGlyphs := 0
	if pre > 0 {
		preGlyphs = gruns[pre-1].End
	}
	sufGlyph := len(oldRow.Glyphs)
	if oldEnd < len(gruns) {
		sufGlyph = gruns[oldEnd].Start
	}
	if preGlyphs > sufGlyph {
		return fail()
	}
	// 旧 caret 须首尾齐且含窗口边界。
	oldCarets := oldRow.Carets
	if len(oldCarets) == 0 || oldCarets[0].ByteOff != 0 ||
		oldCarets[len(oldCarets)-1].ByteOff != len(oldLine) {
		return fail()
	}
	preCarets := sort.Search(len(oldCarets), func(i int) bool { return oldCarets[i].ByteOff >= winA })
	if preCarets >= len(oldCarets) || oldCarets[preCarets].ByteOff != winA {
		return fail()
	}
	oldWinCaret := sort.Search(len(oldCarets), func(i int) bool { return oldCarets[i].ByteOff >= oldWinB })
	if oldWinCaret >= len(oldCarets) || oldCarets[oldWinCaret].ByteOff != oldWinB {
		return fail()
	}
	// 前缀 caret 数须等于前缀 rune 数(交叉验证行完整性)。
	if preCarets != utf8.RuneCountInString(newLine[:winA]) {
		return fail()
	}
	preX := oldCarets[preCarets].X
	// 前缀笔位:前缀字形最右沿(与原构建的累加终值逐位一致)。
	prePen := 0.0
	for _, g := range oldRow.Glyphs[:preGlyphs] {
		if e := g.X + g.XAdvance; e > prePen {
			prePen = e
		}
	}
	var midCarets []GlyphCaret
	var midGlyphs []text.ShapedGlyph
	var midGruns []LineGlyphRun
	newPen := prePen
	if pre < newEnd {
		winRuns := make([]itemizedRun, 0, newEnd-pre)
		for _, r := range newRuns[pre:newEnd] {
			winRuns = append(winRuns, itemizedRun{face: r.face, start: r.start - winA, end: r.end - winA, rtl: r.rtl})
		}
		mc, mw, mg, mgr, ok := buildShapedCarets(newLine[winA:winB], winRuns, prePen, preX)
		if !ok || len(mgr) != newEnd-pre {
			return fail()
		}
		for i := range mc {
			mc[i].ByteOff += winA
		}
		for i := range mg {
			mg[i].Cluster += preCarets
		}
		for i := range mgr {
			mgr[i].Start += preGlyphs
			mgr[i].End += preGlyphs
			mgr[i].TextStart += winA
			mgr[i].TextEnd += winA
		}
		midCarets, midGlyphs, midGruns, newPen = mc, mg, mgr, mw
	} else {
		midCarets = []GlyphCaret{{ByteOff: winA, X: preX}}
	}
	// 旧窗口笔位终值与位移量。
	oldPen := prePen
	for _, g := range oldRow.Glyphs[preGlyphs:sufGlyph] {
		if e := g.X + g.XAdvance; e > oldPen {
			oldPen = e
		}
	}
	shiftX := newPen - oldPen
	oldWinRunes := utf8.RuneCountInString(oldLine[oldWinA:oldWinB])
	shiftRunes := (len(midCarets) - 1) - oldWinRunes
	shiftGlyphs := preGlyphs + len(midGlyphs) - sufGlyph
	newCarets := make([]GlyphCaret, 0, preCarets+len(midCarets)+len(oldCarets)-oldWinCaret-1)
	for _, c := range oldCarets[:preCarets] {
		if c.ByteOff >= winA {
			return fail()
		}
		newCarets = append(newCarets, c)
	}
	newCarets = append(newCarets, midCarets...)
	for _, c := range oldCarets[oldWinCaret+1:] {
		if c.ByteOff <= oldWinB {
			return fail()
		}
		c.ByteOff += shiftBytes
		c.X += shiftX
		newCarets = append(newCarets, c)
	}
	// 新行 caret 首尾须完整。
	if len(newCarets) == 0 || newCarets[0].ByteOff != 0 ||
		newCarets[len(newCarets)-1].ByteOff != len(newLine) {
		return fail()
	}
	newGlyphs := make([]text.ShapedGlyph, 0, preGlyphs+len(midGlyphs)+len(oldRow.Glyphs)-sufGlyph)
	newGlyphs = append(newGlyphs, oldRow.Glyphs[:preGlyphs]...)
	newGlyphs = append(newGlyphs, midGlyphs...)
	for _, g := range oldRow.Glyphs[sufGlyph:] {
		g.X += shiftX
		g.Cluster += shiftRunes
		newGlyphs = append(newGlyphs, g)
	}
	newGruns := make([]LineGlyphRun, 0, pre+len(midGruns)+len(gruns)-oldEnd)
	newGruns = append(newGruns, gruns[:pre]...)
	newGruns = append(newGruns, midGruns...)
	for _, r := range gruns[oldEnd:] {
		r.Start += shiftGlyphs
		r.End += shiftGlyphs
		r.TextStart += shiftBytes
		r.TextEnd += shiftBytes
		newGruns = append(newGruns, r)
	}
	newRTL := make([]bool, len(newRuns))
	for i, r := range newRuns {
		newRTL[i] = r.rtl
	}
	return TextLayoutLine{
		StartByte: 0, EndByte: len(newLine),
		Carets: newCarets, Width: oldRow.Width + shiftX, Height: lh,
		Glyphs: newGlyphs, GlyphRuns: newGruns, runRTL: newRTL,
	}, newSegs, true
}

// tryReuseCachedRow是单行快径的分段缓存外壳:segSegs 恒对应旧行文本,
// 对不上(首建/回退后)现场全量一种子,命中后增量分段并滚动缓存。
func (c *layoutCache) tryReuseCachedRow(oldRow TextLayoutLine, oldLine, newLine string, face text.Face, lh float64) (TextLayoutLine, []text.Segment, bool) {
	fail := func() (TextLayoutLine, []text.Segment, bool) { return TextLayoutLine{}, nil, false }
	if c.segLine != oldLine || len(c.segSegs) == 0 {
		c.segSegs = text.SegmentText(oldLine)
		c.segLine = oldLine
		if len(c.segSegs) == 0 {
			return fail()
		}
	}
	row, newSegs, ok := tryReuseShapedRowSegs(oldRow, oldLine, c.segSegs, newLine, face, lh)
	if !ok || len(newSegs) == 0 {
		c.segLine = ""
		c.segSegs = nil
		return fail()
	}
	return row, newSegs, true
}

// reuseSingleRow是 patchRows 单行快径:整篇单行且够长时走行内复用,
// 否则 ok=false 由调用方走 rowSpan 全量。
func (c *layoutCache) reuseSingleRow(textStr string, parts []hardPart, lo, hi int, face text.Face, fontSize, lh float64, gen uint64) ([]TextLayoutLine, []uint64, bool) {
	fail := func() ([]TextLayoutLine, []uint64, bool) { return nil, nil, false }
	lv := &c.live
	if len(parts) != 1 || len(lv.rows) != 1 || lo != 0 || hi != 0 {
		return fail()
	}
	p0 := parts[0]
	if p0.start != 0 || p0.end != len(textStr) || len(textStr) < reuseMinLineBytes {
		return fail()
	}
	oldRow := lv.rows[0]
	if oldRow.StartByte != 0 || oldRow.EndByte != len(lv.text) {
		return fail()
	}
	row, newSegs, ok := c.tryReuseCachedRow(oldRow, lv.text, textStr, face, lh)
	if !ok {
		return fail()
	}
	c.segSegs, c.segLine = newSegs, textStr
	row = rebaseLine(row, p0.start)
	k := cacheKey{sum: c.hashStr(p0.text), face: face, size: fontSize, lh: lh}
	putCached(c.rows, k, &cachedRows{rows: []TextLayoutLine{row}, gen: gen})
	return []TextLayoutLine{row}, []uint64{gen}, true
}
