package rendering

import (
	"hash/fnv"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// blockHash is a stable content hash: it must survive index rebuilds so an
// edit only re-measures the changed block (maphash.MakeSeed would change the
// seed on every rebuild and defeat the cache).
func blockHash(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// V4 / D1 stage-4: an over-long logical line is split into grapheme-ish
// blocks (2-4KB). Each block's shaped width is cached by content hash, the
// widths form a prefix sum, and a pixel X (horizontal scroll) maps back to a
// byte offset through that prefix sum (D1 "far-end estimate"). Only the
// blocks touched by the visible band are handed to the incremental layout
// engine — the render cap bounds how much of the line can be shown at all.
const (
	// DefaultRenderCapChars is VS Code stopRenderingLineAfter: characters
	// beyond this are truncated and marked, not laid out for display.
	DefaultRenderCapChars = 10000
	// DefaultShapeCapChars is VS Code maxTokenizationLineLength: characters
	// up to this are measured into the prefix sum (scroll range estimate).
	DefaultShapeCapChars = 20000
	// longLineBlockBytes is the 2-4KB block size from plan §3 layer 4:
	// blocks are the shaping/prefix-sum unit, NOT the layout window size.
	longLineBlockBytes = 4096
	// longLineWindowOverscan bounds the visible window in bytes. The window
	// is sized by the visible pixel band (plan §3 "只整形可见水平窗口"), not
	// by the block size — otherwise a 200px band would shape 8192 bytes.
	longLineWindowOverscan = 512
)

// longLineBlock is one lazily measured block (full-text byte range).
type longLineBlock struct {
	start, end int
	width      float64
	measured   bool
}

type blockCacheKey struct {
	face text.Face
	hash uint64
}

// longLineIndex owns the block model for one long-line text+face pair.
type longLineIndex struct {
	text       string
	face       text.Face
	renderCap  int // characters, <=0 = unlimited
	shapeCap   int // characters, <=0 = unlimited
	blocks     []longLineBlock
	prefix     []float64 // prefix[k] = width of blocks [0,k)
	blockCache map[blockCacheKey]float64
	hits       int
	misses     int
}

// capBytes returns the byte offset after nChars runes (nChars<=0 = whole).
func capBytes(s string, nChars int) int {
	if nChars <= 0 || nChars >= utf8.RuneCountInString(s) {
		return len(s)
	}
	i, count := 0, 0
	for i < len(s) && count < nChars {
		_, sz := utf8.DecodeRuneInString(s[i:])
		i += sz
		count++
	}
	return i
}

func newLongLineIndex(s string, face text.Face, renderCap, shapeCap int, cache map[blockCacheKey]float64) *longLineIndex {
	ix := &longLineIndex{
		text:       s,
		face:       face,
		renderCap:  renderCap,
		shapeCap:   shapeCap,
		blockCache: cache,
	}
	limit := capBytes(s, shapeCap)
	for start := 0; start < limit; {
		end := start + longLineBlockBytes
		if end > limit {
			end = limit
		}
		// Pull the cut back to a UTF-8 rune boundary.
		for end < limit && !utf8.RuneStart(s[end]) {
			end--
		}
		if end <= start {
			_, sz := utf8.DecodeRuneInString(s[start:])
			end = start + sz
		}
		ix.blocks = append(ix.blocks, longLineBlock{start: start, end: end})
		start = end
	}
	return ix
}

// blockWidth measures (and caches) one block's shaped width. Widths come
// from text.LayoutText — the same shaped builder the layout uses — so
// kerning/ligatures are not lost as they would be with isolated advances.
func (ix *longLineIndex) blockWidth(k int) float64 {
	b := &ix.blocks[k]
	if b.measured {
		return b.width
	}
	key := blockCacheKey{face: ix.face, hash: blockHash(ix.text[b.start:b.end])}
	if w, ok := ix.blockCache[key]; ok {
		ix.hits++
		b.width, b.measured = w, true
		return w
	}
	ix.misses++
	var w float64
	if ix.face != nil {
		if l := text.LayoutText(ix.text[b.start:b.end], ix.face, text.LayoutOptions{}); l != nil {
			w = l.Width
		}
	}
	if w <= 0 {
		w = float64(b.end-b.start) * 7.7 // estimate fallback (D1 far-end estimate)
	}
	ix.blockCache[key] = w
	b.width, b.measured = w, true
	return w
}

// ensurePrefix fills prefix up to block k (inclusive).
func (ix *longLineIndex) ensurePrefix(k int) {
	if k > len(ix.blocks) {
		k = len(ix.blocks)
	}
	for len(ix.prefix) <= k {
		i := len(ix.prefix) - 1
		prev := 0.0
		if i >= 0 {
			prev = ix.prefix[i]
		}
		if i < 0 {
			ix.prefix = append(ix.prefix, 0)
			continue
		}
		ix.prefix = append(ix.prefix, prev+ix.blockWidth(i))
	}
}

// totalWidth is the estimated width of the whole shaped region.
func (ix *longLineIndex) totalWidth() float64 {
	ix.ensurePrefix(len(ix.blocks))
	return ix.prefix[len(ix.blocks)]
}

// ByteAtX maps a pixel X to a full-text byte offset via the block prefix
// sum; within a block it interpolates by average advance (far-end estimate).
func (ix *longLineIndex) ByteAtX(x float64) int {
	if len(ix.blocks) == 0 {
		return 0
	}
	ix.ensurePrefix(len(ix.blocks))
	if x <= 0 {
		return 0
	}
	lo, hi := 0, len(ix.blocks)
	for lo < hi {
		mid := (lo + hi) / 2
		if ix.prefix[mid+1] <= x {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo >= len(ix.blocks) {
		return ix.blocks[len(ix.blocks)-1].end
	}
	b := ix.blocks[lo]
	bw := ix.prefix[lo+1] - ix.prefix[lo]
	frac := 0.0
	if bw > 0 {
		frac = (x - ix.prefix[lo]) / bw
	}
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return b.start + int(frac*float64(b.end-b.start))
}

// XAtByte is the inverse estimate (prefix width + intra-block fraction).
func (ix *longLineIndex) XAtByte(off int) float64 {
	if len(ix.blocks) == 0 || off <= 0 {
		return 0
	}
	ix.ensurePrefix(len(ix.blocks))
	lo, hi := 0, len(ix.blocks)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if ix.blocks[mid].start <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	b := ix.blocks[lo]
	bw := ix.prefix[lo+1] - ix.prefix[lo]
	frac := float64(off-b.start) / float64(b.end-b.start)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return ix.prefix[lo] + bw*frac
}

func (ix *longLineIndex) blockOf(off int) int {
	lo, hi := 0, len(ix.blocks)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if ix.blocks[mid].start <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

// Window returns the full-text byte range to lay out for a visible band
// [scrollX, scrollX+visW]. scrollX maps to a byte offset through the block
// prefix sum; the window is that visible byte range plus a small overscan,
// clamped to rune boundaries and to the render cap. Characters beyond the
// cap are truncated, not scrolled into view (D1 / VS Code).
func (ix *longLineIndex) Window(scrollX, visW float64) (start, end int) {
	limit := capBytes(ix.text, ix.renderCap)
	if limit > len(ix.text) {
		limit = len(ix.text)
	}
	if limit <= 0 || len(ix.blocks) == 0 {
		return 0, 0
	}
	if visW <= 0 {
		visW = 904 // conservative default visible width
	}
	if scrollX < 0 {
		scrollX = 0
	}
	firstByte := ix.ByteAtX(scrollX)
	lastByte := ix.ByteAtX(scrollX + visW)
	start = firstByte - longLineWindowOverscan
	end = lastByte + longLineWindowOverscan
	if start < 0 {
		start = 0
	}
	if end > limit {
		end = limit
	}
	if start > limit {
		start = limit
	}
	// Keep both cuts on UTF-8 rune boundaries.
	for start > 0 && !utf8.RuneStart(ix.text[start]) {
		start--
	}
	for end < limit && !utf8.RuneStart(ix.text[end]) {
		end++
	}
	if start > end {
		start = end
	}
	return start, end
}

// CacheStats reports block shape-cache hits/misses (observability/tests).
func (ix *longLineIndex) CacheStats() (hits, misses int) { return ix.hits, ix.misses }

// adaptLongLineLayout converts a layout built from the bounded D1 window
// text into full-text coordinates and attaches the truncation marker.
//
// Two independent problems are fixed here:
//   - The incremental engine only sees the window text, so its glyph/caret
//     X positions start at 0 (window-relative), while hCullWindow and every
//     query mouth use FULL-text X / byte offsets. Rebasing X by the estimated
//     width before the window (XAtByte) makes the two agree — otherwise a
//     scrolled window shows content from the wrong part of the line.
//   - Skia TextLine::createEllipsis hangs "…" on the TextLine as an
//     INDEPENDENT RUN (shapeEllipsis), not as a string drawn afterwards.
//     paintCompositeRuns / bulk submit read LineGlyphs only, so the marker
//     must live in the glyph table. We drop the glyphs past the visible right
//     edge and append the marker run — the window text itself is never
//     rewritten, so UTF-8 and cluster/byte offsets stay intact.
func adaptLongLineLayout(t *RenderText, l *TextLayout, start, end, fullLen int) {
	if t == nil || l == nil {
		return
	}
	ix := t.longLineIndexFor(t.Text)
	xOff := ix.XAtByte(start)
	visW := t.viewportWidth
	if visW <= 0 {
		visW = 904
	}
	rightX := t.viewportScrollX + visW

	rows := make([]TextLayoutLine, len(l.lines))
	copy(rows, l.lines)
	for i := range rows {
		rows[i].StartByte += start
		rows[i].EndByte += start
		// Fresh copies: the shared cache snapshot is read-only.
		if len(rows[i].Glyphs) > 0 {
			g := make([]text.ShapedGlyph, len(rows[i].Glyphs))
			copy(g, rows[i].Glyphs)
			for j := range g {
				g[j].X += xOff
			}
			rows[i].Glyphs = g
		}
		if len(rows[i].Carets) > 0 {
			c := make([]GlyphCaret, len(rows[i].Carets))
			copy(c, rows[i].Carets)
			for j := range c {
				c[j].X += xOff
			}
			rows[i].Carets = c
		}
		rows[i].clusters = nil
	}
	l.lines = rows
	l.Text = t.Text
	l.shift = nil
	l.clusters = nil
	l.idx = buildLineIndex(rows, l.FontSize, l.LineSpacing)
	l.idxSeq = l.idx.seq
	l.LongLine = &LongLineInfo{FullLen: fullLen, WindowStart: start, WindowEnd: end}

	if end >= fullLen || len(rows) == 0 {
		return // window reaches the tail: nothing is truncated
	}
	row := &l.lines[0]
	// The marker must be shaped AND submitted with a concrete face, not the
	// composite MultiFace: paintCompositeRuns calls DrawShapedGlyphs with
	// r.Face, and a MultiFace cannot submit glyphs directly. Reuse the last
	// window run's face (the face of the text right before the cut).
	mFace := l.Face
	if len(row.GlyphRuns) > 0 {
		if f := row.GlyphRuns[len(row.GlyphRuns)-1].Face; f != nil {
			mFace = f
		}
	}
	if mFace == nil {
		return
	}
	ml := BuildTextLayoutExWithSpacing(textEllipsis, mFace, t.fontSize(), 0,
		t.lineSpacing(), t.approxCharW(), 0, TextOverflowClip, 0, 0)
	if ml == nil || ml.LineCount() == 0 {
		return
	}
	mg := ml.LineGlyphs(0)
	if len(mg) == 0 {
		return
	}
	ellW := ml.lines[0].Width
	if ellW <= 0 {
		ellW = 16
	}
	markerX := rightX - ellW
	if markerX < xOff {
		markerX = xOff
	}
	cut := len(row.Glyphs)
	for i, gl := range row.Glyphs {
		if gl.X >= markerX {
			cut = i
			break
		}
	}
	// Keep the visible prefix. With the marker disabled the tail is still
	// dropped (so a side-by-side diff isolates exactly the "…" ink).
	ng := make([]text.ShapedGlyph, 0, cut+len(mg))
	ng = append(ng, row.Glyphs[:cut]...)
	var nr []LineGlyphRun
	for _, r := range row.GlyphRuns {
		if r.Start >= cut {
			break
		}
		if r.End > cut {
			r.End = cut
		}
		nr = append(nr, r)
	}
	// row.Width stays the window text's full width (as in stage A): it feeds
	// m.maxW / mouse-edge horizontal scroll limits. Shrinking it to the
	// visible band would clamp those limits and regress dragging.
	if !t.DisableLongLineMarker {
		baseX := mg[0].X
		for _, gl := range mg {
			mgl := gl
			mgl.X = markerX + (mgl.X - baseX)
			mgl.Cluster = cut
			ng = append(ng, mgl)
		}
		nr = append(nr, LineGlyphRun{
			Face: mFace, Start: cut, End: cut + len(mg),
			TextStart: end - start, TextEnd: end - start,
		})
		l.LongLine.MarkerX = markerX
	}
	row.Glyphs = ng
	row.GlyphRuns = nr
}
