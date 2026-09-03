package rendering

import (
	"hash/maphash"

	"github.com/energye/gpui/render/text"
)

// layoutCache是M1的双模式布局缓存(约束②/Q2):
// 不回绕按行缓存,回绕按段(\n分段)缓存.命中复用已构建的行,只重排被改的行/段.
// 调用方未接入前BuildTextLayout仍走全量构建,缓存行为由本文件单测独立锁定.
//
// key=内容哈希+face+字号+maxWidth(含回绕模式)+估算字宽,缺一项即撞车.
// 估算字宽只在无脸估算分支使用(face!=nil 的两条分支填 0,face 键已隔离).
// face 对象不可变(配置变化必换对象,见 render/text/options.go With*),
// hinting/variations 等配置由 face 身份表达,不另进键.
// 行按段内相对偏移存储,命中时拷贝并变基到全局(复用整形结果,不复用切片头;
// Glyphs只读共享,构建后不再原地修改).
// 线程封闭:只在事件循环线程使用(沿§7.4 C1),内部不加锁.
type layoutCache struct {
	rows map[cacheKey]*cachedRows
	segs map[cacheKey]*cachedRows
	hits int
	blds int
	// seed是段哈希的固定种子(同缓存实例内稳定;key只活在进程内,
	// 无需跨进程稳定,跨实例不同种子互不影响).
	seed maphash.Seed
	// live是增量更新的常驻状态(RenderText接线用,见layout_update.go).
	live layoutLive
}

type cacheKey struct {
	sum  uint64
	face text.Face
	size float64
	w    float64
	lh   float64
	aw   float64
}

type cachedRows struct {
	rows []TextLayoutLine
	gen  uint64
}

func newLayoutCache() *layoutCache {
	return &layoutCache{rows: make(map[cacheKey]*cachedRows), segs: make(map[cacheKey]*cachedRows), seed: maphash.MakeSeed()}
}

// maxCacheEntries是跨文档缓存的条目上限(纯内存护栏,正确性无关):
// 长会话不断粘贴新段落时清表重建,单次全量重建可接受,无限增长不可接受.
// 取值拍脑袋(4096 行/段约数 MB),改它只影响命中率,不影响正确性.
const maxCacheEntries = 4096

func putCached(table map[cacheKey]*cachedRows, k cacheKey, v *cachedRows) {
	if len(table) >= maxCacheEntries {
		clear(table)
	}
	table[k] = v
}

// hashStr是段缓存key的内容哈希(FNV逐字节约1ns/B,长段击键瓶颈之一;
// maphash同种子稳定,短段更快,长段约5倍速,键语义不变).
func (c *layoutCache) hashStr(s string) uint64 {
	var h maphash.Hash
	h.SetSeed(c.seed)
	_, _ = h.WriteString(s)
	return h.Sum64()
}

// buildCached用缓存构建整份布局.复用行的LineGen保留旧值,
// 新建/重建行取新Generation(全局Generation照常自增,I9).
func (c *layoutCache) buildCached(textStr string, face text.Face, fontSize, maxWidth, lineSpacing float64) *TextLayout {
	textLayoutGen++
	gen := textLayoutGen
	lines, marks := c.cachedLines(textStr, face, fontSize, maxWidth, lineSpacing, gen)
	lh := lineHeightFor(face, fontSize, lineSpacing)
	for i := range lines {
		if lines[i].Height <= 0 {
			lines[i].Height = lh
		}
	}
	l := &TextLayout{Text: textStr, lines: lines, FontSize: fontSize, LineSpacing: lineSpacing, Generation: gen, MaxWidth: maxWidth, Face: face, LineGen: marks}
	l.idx = buildLineIndex(l.lines, l.FontSize, l.LineSpacing)
	return l
}

func (c *layoutCache) cachedLines(textStr string, face text.Face, fontSize, maxWidth, lineSpacing float64, gen uint64) ([]TextLayoutLine, []uint64) {
	return c.cachedLinesFull(textStr, face, fontSize, maxWidth, lineSpacing, 0.55, gen)
}

// cachedLinesFull与cachedLines同,估算字宽由调用方传入(RenderText可用非默认 ApproxCharW).
func (c *layoutCache) cachedLinesFull(textStr string, face text.Face, fontSize, maxWidth, lineSpacing, approxCharW float64, gen uint64) ([]TextLayoutLine, []uint64) {
	if textStr == "" {
		return nil, nil
	}
	lh := lineHeightFor(face, fontSize, lineSpacing)
	if maxWidth > 0 && face != nil {
		if hasCR(textStr) {
			// 含回车:WrapText 在内部归一化换行并重映偏移,段缓存按 \n
			// 切分会对不上.该路径稀少,直接整篇构建,不进段缓存.
			var out []TextLayoutLine
			var marks []uint64
			for _, w := range wrapFaceResults(textStr, face, maxWidth) {
				out = append(out, materializeWrappedRow(w, face, lh))
				marks = append(marks, gen)
			}
			return out, marks
		}
		return c.partLines(splitHardLines(textStr), face, fontSize, maxWidth, lh, 0, gen, c.segs,
			func(seg string) []text.WrapResult { return wrapFaceResults(seg, face, maxWidth) })
	}
	if maxWidth > 0 && face == nil {
		return c.partLines(splitHardLines(textStr), face, fontSize, maxWidth, lh, approxCharW, gen, c.segs,
			func(seg string) []text.WrapResult {
				return wrapEstResults(seg, maxWidth, fontSize, approxCharW)
			})
	}
	return c.rowLines(textStr, face, fontSize, lh, gen)
}

// wrapFaceResults按字回绕一段,空结果回退整段(与partLines的空段约定一致).
func wrapFaceResults(seg string, face text.Face, maxWidth float64) []text.WrapResult {
	wrapped := text.WrapText(seg, face, maxWidth, text.WrapWordChar)
	if len(wrapped) == 0 {
		wrapped = []text.WrapResult{{Text: seg, Start: 0, End: len(seg)}}
	}
	return wrapped
}

// wrapEstResults无脸估算回绕一段,空结果回退整段.
func wrapEstResults(seg string, maxWidth, fontSize, approxCharW float64) []text.WrapResult {
	wrapped := estimateWrapResults(seg, maxWidth, fontSize, approxCharW)
	if len(wrapped) == 0 {
		wrapped = []text.WrapResult{{Text: seg, Start: 0, End: len(seg)}}
	}
	return wrapped
}

func hasCR(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			return true
		}
	}
	return false
}

// rowLines不回绕:每硬行独立key,改第K行只重建K.行高构造时填好.
func (c *layoutCache) rowLines(textStr string, face text.Face, fontSize, lh float64, gen uint64) ([]TextLayoutLine, []uint64) {
	return c.rowLinesParts(splitHardLines(textStr), face, fontSize, lh, gen)
}

func (c *layoutCache) rowLinesParts(parts []hardPart, face text.Face, fontSize, lh float64, gen uint64) ([]TextLayoutLine, []uint64) {
	out := make([]TextLayoutLine, 0, len(parts))
	marks := make([]uint64, 0, len(parts))
	for _, p := range parts {
		k := cacheKey{sum: c.hashStr(p.text), face: face, size: fontSize, lh: lh}
		if u, ok := c.rows[k]; ok && len(u.rows) == 1 {
			c.hits++
			out = append(out, rebaseLine(u.rows[0], p.start))
			marks = append(marks, u.gen)
			continue
		}
		c.blds++
		rel := materializeWrappedRow(text.WrapResult{Text: p.text, Start: 0, End: len(p.text)}, face, lh)
		putCached(c.rows, k, &cachedRows{rows: []TextLayoutLine{rel}, gen: gen})
		out = append(out, rebaseLine(rel, p.start))
		marks = append(marks, gen)
	}
	return out, marks
}

// partLines回绕/估算共用:每硬段独立key,段内回绕行整体存取.改首段不碰后续段.
// 等价性由TestLayoutCache_Equiv锁定(与BuildTextLayoutEx逐字节对照).
func (c *layoutCache) partLines(parts []hardPart, face text.Face, fontSize, maxWidth, lh, approxCharW float64, gen uint64, table map[cacheKey]*cachedRows, wrap func(seg string) []text.WrapResult) ([]TextLayoutLine, []uint64) {
	var out []TextLayoutLine
	var marks []uint64
	for _, p := range parts {
		k := cacheKey{sum: c.hashStr(p.text), face: face, size: fontSize, w: maxWidth, lh: lh, aw: approxCharW}
		if u, ok := table[k]; ok {
			c.hits++
			for _, sl := range u.rows {
				out = append(out, rebaseLine(sl, p.start))
				marks = append(marks, u.gen)
			}
			continue
		}
		c.blds++
		stored := &cachedRows{gen: gen}
		for _, w := range wrap(p.text) {
			rel := materializeWrappedRow(w, face, lh)
			stored.rows = append(stored.rows, rel)
			out = append(out, rebaseLine(rel, p.start))
			marks = append(marks, gen)
		}
		putCached(table, k, stored)
	}
	return out, marks
}

// rebaseLine把段内相对行拷贝并变基到全局:起止加基址,
// caret/字形/分区数组只读共享(行构建后无任何原地修改,分享安全;
// Glyphs此前已是只读共享,快照同样分享行内数组).
func rebaseLine(sl TextLayoutLine, base int) TextLayoutLine {
	sl.StartByte += base
	sl.EndByte += base
	return sl
}

type hardPart struct {
	text       string
	start, end int
}

func splitHardLines(s string) []hardPart {
	if s == "" {
		return []hardPart{{text: "", start: 0, end: 0}}
	}
	var out []hardPart
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, hardPart{text: s[start:i], start: start, end: i})
			start = i + 1
		}
	}
	out = append(out, hardPart{text: s[start:], start: start, end: len(s)})
	return out
}
