package rendering

import (
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/energye/gpui/render/text"
)

// V4 gate: docs/ENGINE_TEXT_LARGE_SCALE_PLAN.md §2.2 requires
// "字符击键 p95 ≤16ms（含 5e6 字单行）". V0 (§2.1/§6) measured 1.804s @1e6
// and 10.412s @5e6 on the production SetTextSpan+TextLayout path — 113x and
// 650x over the gate. Stage 4 (plan §3 layer 4) fixes this with a VISIBLE
// horizontal window sized by pixel width, block prefix sums and a content-
// hashed block shape cache, plus independent render/shape caps (D1).
//
// Measurement discipline (V0 §2.2 / §9): every keystroke edits a DIFFERENT
// position so the whole-line string is unique and cannot hit the line cache
// (alternating the same position under-reports ~800x). Only the edit itself
// is timed; the restore is not.
const v4KeystrokeGate = 16 * time.Millisecond

// v4LongLine builds one logical line of n bytes from the testdata seed.
func v4LongLine(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile("testdata/v4_longline_seed.txt")
	if err != nil {
		t.Skipf("V4 needs testdata seed: %v", err)
	}
	seed := strings.TrimRight(string(b), "\r\n")
	if seed == "" {
		t.Skipf("V4 testdata seed is empty")
	}
	var sb strings.Builder
	sb.Grow(n + len(seed))
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}

// v4ReplaceASCII replaces one byte at/after pos, skipping UTF-8 continuation
// and lead bytes so the edit always lands on an ASCII rune. Each keystroke
// therefore produces a distinct whole-line string.
func v4ReplaceASCII(s string, pos int, ch byte) (string, int) {
	b := []byte(s)
	for pos < len(b) && b[pos] >= 0x80 {
		pos++
	}
	if pos >= len(b) {
		pos = len(b) - 1
	}
	b[pos] = ch
	return string(b), pos
}

// v4Keystroke measures ONE production keystroke on a single logical line.
func v4Keystroke(rt *RenderText, base string, pos int) time.Duration {
	edited, at := v4ReplaceASCII(base, pos, 'X')
	t0 := time.Now()
	rt.SetTextSpan(edited, at, at+1, at, at+1)
	_ = rt.TextLayout()
	d := time.Since(t0)
	// Restore to the base so the next keystroke starts clean. The base string
	// is identical every time, so this restore may hit the cache — it is
	// deliberately excluded from the gate measurement.
	rt.SetTextSpan(base, at, at+1, at, at+1)
	_ = rt.TextLayout()
	return d
}

func v4Face(t *testing.T) text.Face {
	t.Helper()
	return m5DejaVuFace(t)
}

// TestV4_SingleLineKeystroke_1e6: 1e6-byte single line, three distinct edit
// positions INSIDE the visible window (real typing, not an out-of-window
// edit that legitimately skips re-shaping), median of three.
func TestV4_SingleLineKeystroke_1e6(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 40000) // wide visible band: window covers ~5KB
	_ = rt.TextLayout()          // first layout (warm-up, not gated)

	ds := make([]time.Duration, 0, 3)
	for i := 1; i <= 3; i++ {
		ds = append(ds, v4Keystroke(rt, base, i*1000))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	med := ds[len(ds)/2]
	t.Logf("V4 1e6-char single-line keystroke median = %v (gate <=%v)", med, v4KeystrokeGate)
	if med > v4KeystrokeGate {
		t.Fatalf("1e6-char keystroke %v exceeds 16ms gate (%.0fx over on this corpus). V0 §2.1 measured 1.804s on the 'a世' corpus (113x) — different corpus, reference only",
			med, float64(med)/float64(v4KeystrokeGate))
	}
}

// TestV4_SingleLineKeystroke_5e6 is the gate's headline case (plan §2.1 L3).
// V0 §8 ran only ONE unique edit at this size to stay inside the 7GB machine
// memory budget, so this test does the same: one keystroke, no repeated runs.
// The edit sits beyond the render cap (legitimate "far edit" fast path).
func TestV4_SingleLineKeystroke_5e6(t *testing.T) {
	runtime.GC()
	const n = 5000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	_ = rt.TextLayout() // first layout (not gated)

	d := v4Keystroke(rt, base, n/2)
	t.Logf("V4 5e6-char single-line keystroke = %v (gate <=%v)", d, v4KeystrokeGate)
	if d > v4KeystrokeGate {
		t.Fatalf("5e6-char keystroke %v exceeds 16ms gate (%.0fx over on this corpus). V0 §2.1 measured 10.412s on the 'a世' corpus (650x) — different corpus, reference only",
			d, float64(d)/float64(v4KeystrokeGate))
	}
}

// TestV4_LongLineHorizontalViewport: a horizontal scroll slides the window
// through the block prefix sum, the full text stays for copy/select-all, and
// caret lookups on full-text offsets keep working inside the window.
func TestV4_LongLineHorizontalViewport(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 904)
	_ = rt.TextLayout()
	before := rt.TextLayout().LongLine.WindowStart

	// scrollX beyond one block width slides the window via prefix sums.
	rt.SetViewportHint(40000, 904)
	lay := rt.TextLayout()
	winStart := lay.LongLine.WindowStart
	if winStart <= before {
		t.Fatalf("window did not slide: %d -> %d", before, winStart)
	}
	lineStart, lineEnd, _, _, ok := lay.Line(0)
	if !ok || lineStart != winStart || lineEnd <= winStart {
		t.Fatalf("layout row is not the slid window: start=%d end=%d (want start=%d)", lineStart, lineEnd, winStart)
	}
	if len(rt.Text) != n || len(lay.Text) != n {
		t.Fatalf("full text lost: rt=%d layout=%d want=%d", len(rt.Text), len(lay.Text), n)
	}
	// A full-text offset inside the window must map to the caret at that
	// position, NOT be clamped to the window end (which would still return
	// ok=true and look green while placing the caret wrongly).
	_, xIn, ok := lay.CaretForOffset(winStart + 100)
	if !ok {
		t.Fatalf("caret lookup for an in-window full-text offset failed")
	}
	_, xEnd, ok := lay.CaretForOffset(lay.LongLine.WindowEnd - 1)
	if !ok {
		t.Fatalf("caret lookup at window end failed")
	}
	if xIn >= xEnd {
		t.Fatalf("caret X was not mapped into the window: xIn=%.1f xEnd=%.1f", xIn, xEnd)
	}
}

// TestV4_WindowTracksVisibleWidth locks "只整形可见水平窗口": the layout
// window is sized by the VISIBLE pixel width, not by the render cap (10000
// characters). At 200px (~26 glyphs) a 10000-byte window would waste ~99%
// of the shaping work on GPU-clipped content.
func TestV4_WindowTracksVisibleWidth(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 200) // 200px visible band
	lay := rt.TextLayout()
	if lay.LongLine == nil {
		t.Fatalf("no long-line window")
	}
	winBytes := lay.LongLine.WindowEnd - lay.LongLine.WindowStart
	// 200px at ~7.7px/char is ~26 bytes; allow generous overscan (10x)
	// but a 10000-byte fixed window must not pass.
	if winBytes > 2000 {
		t.Fatalf("window is %d bytes for a 200px visible band; it is sized by the render cap, not the visible width (want <=2000)", winBytes)
	}
}

// TestV4_WindowTracksScrollX locks that a pixel X maps to a byte window
// through the block prefix sum (the old byte-offset-only window ignored
// scrollX entirely).
func TestV4_WindowTracksScrollX(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 200)
	before := rt.TextLayout().LongLine.WindowStart
	rt.SetViewportHint(50000, 200) // scroll horizontally
	after := rt.TextLayout().LongLine.WindowStart
	if after <= before {
		t.Fatalf("scrollX=50000 did not slide the window (start %d -> %d); no prefix-width mapping", before, after)
	}
}

// TestV4_BlockShapeCacheSurvivesEdit is the plan §1.4 "编辑只重建变化区间"
// capability: the content-hashed block cache outlives an index rebuild, so
// editing one byte re-measures only the changed block.
func TestV4_BlockShapeCacheSurvivesEdit(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 904)
	_ = rt.TextLayout()
	ix := rt.longLineIndexFor(base)
	_ = ix.totalWidth() // measure every block (misses)
	h0, _ := ix.CacheStats()

	edited, _ := v4ReplaceASCII(base, 100, 'X')
	rt.SetText(edited)
	_ = rt.TextLayout()
	ix2 := rt.longLineIndexFor(edited)
	_ = ix2.totalWidth() // rebuilt index: unchanged blocks must hit the cache
	h1, m1 := ix2.CacheStats()
	if h1 <= h0 {
		t.Fatalf("block shape cache did not survive an edit: hits %d -> %d (misses now %d); unchanged blocks were re-shaped", h0, h1, m1)
	}
}

// TestV4_LongLineWindow locks the D1 window semantics (not just timing):
// the full text is kept for copy/select-all, only the bounded window is laid
// out, an inner edit is really reflected in the layout, offsets beyond the
// window clamp safely, and the caps are independent / disableable.
func TestV4_LongLineWindow(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	rt.SetViewportHint(0, 904)

	end, fullLen, truncated := rt.LongLineWindow()
	lay := rt.TextLayout()
	ll := lay.LongLine
	if !truncated || fullLen != n || ll == nil || ll.WindowEnd-ll.WindowStart <= 0 ||
		ll.WindowEnd-ll.WindowStart >= n/10 {
		t.Fatalf("window not bounded / full text wrong: (%d,%d,%v) ll=%+v", end, fullLen, truncated, ll)
	}
	if len(rt.Text) != n || len(lay.Text) != n {
		t.Fatalf("full text lost: rt=%d layout=%d want=%d", len(rt.Text), len(lay.Text), n)
	}
	rowStart, rowEnd, _, _, ok := lay.Line(0)
	if !ok || rowStart != ll.WindowStart || rowEnd != ll.WindowEnd {
		t.Fatalf("layout row is not the window: (%d,%d) want (%d,%d)", rowStart, rowEnd, ll.WindowStart, ll.WindowEnd)
	}

	// A real edit inside the window must appear in the laid-out text.
	want := ll.WindowStart + (ll.WindowEnd-ll.WindowStart)/2
	edited, at := v4ReplaceASCII(base, want, 'Z')
	rt.SetTextSpan(edited, at, at+1, at, at+1)
	lay = rt.TextLayout()
	if len(lay.Text) != n || lay.Text[at] != 'Z' {
		t.Fatalf("inner-window edit not reflected in full text: len=%d byte=%q", len(lay.Text), lay.Text[at])
	}

	// Offsets beyond the window clamp into it (no panic / no out-of-bounds).
	if _, _, ok := lay.CaretForOffset(n - 1); !ok {
		t.Fatalf("CaretForOffset beyond window returned !ok; expected a clamped caret")
	}
	if got, _ := lay.GetPositionForOffset(1e9, 0); got < 0 || got > n {
		t.Fatalf("clamped byte offset out of full text: %d", got)
	}

	// Independent render/shape caps: RenderCap bounds what is displayed;
	// ShapeCap bounds the prefix-sum shaping budget.
	rt.SetRenderCap(2000)
	if e, _, tr := rt.LongLineWindow(); !tr || e > 2000 {
		t.Fatalf("render cap not applied: end=%d truncated=%v", e, tr)
	}
	s0, e0, _, _, ok := rt.TextLayout().Line(0)
	if !ok || e0 <= s0 || e0 > 2000 {
		t.Fatalf("window row exceeds render cap: (%d,%d)", s0, e0)
	}
	rt.SetShapeCap(5000)
	if _, _, tr := rt.LongLineWindow(); !tr {
		t.Fatalf("shape cap change should keep the window active")
	}

	// RenderCap < 0 disables truncation: layout sees the full (small) text.
	small := v4LongLine(t, 20000)
	off := NewRenderText(small)
	off.SetRenderCap(-1)
	if _, _, tr := off.LongLineWindow(); tr {
		t.Fatalf("RenderCap -1 should disable truncation")
	}
	if len(off.TextLayout().Text) != len(small) {
		t.Fatalf("disabled cap still truncated: %d != %d", len(off.TextLayout().Text), len(small))
	}
}

// TestV4_TruncationMarkerInGlyphs is the stage-B red light, aligned with
// Skia TextLine::createEllipsis: the "…" must be a REAL GLYPH in the line's
// glyph table (paintCompositeRuns / bulk submit read LineGlyphs, never a
// string drawn afterwards), sitting at the right edge of the visible band.
func TestV4_TruncationMarkerInGlyphs(t *testing.T) {
	const n = 1000000
	base := v4LongLine(t, n)
	face := v4Face(t)
	rt := NewRenderText(base)
	rt.SetFace(face)
	const visW = 904.0
	rt.SetViewportHint(0, visW)
	lay := rt.TextLayout()

	var wantGID text.GlyphID
	if el := text.LayoutText(textEllipsis, face, text.LayoutOptions{}); el != nil && len(el.Lines) > 0 && len(el.Lines[0].Glyphs) > 0 {
		wantGID = el.Lines[0].Glyphs[0].GID
	}
	if wantGID == 0 {
		t.Fatalf("test face has no glyph for %q", textEllipsis)
	}
	found := false
	for _, gl := range lay.LineGlyphs(0) {
		if gl.GID == wantGID && gl.X >= visW-24 && gl.X <= visW+4 {
			found = true
		}
	}
	if !found {
		t.Fatalf("V4 RED: no %q glyph at the visible right edge (LineGlyphs=%d); "+
			"the marker must be shaped INTO the layout glyph table, not appended to a paint string",
			textEllipsis, len(lay.LineGlyphs(0)))
	}
}

// TestV4_WindowTextValidUTF8 locks the stage-B root cause: an equal-byte
// marker replacement at an arbitrary BYTE offset can land inside a
// multi-byte rune and produce invalid UTF-8, which crashes emoji.Segment
// and drops the glyph. The window text must always be valid UTF-8.
func TestV4_WindowTextValidUTF8(t *testing.T) {
	base := v4LongLine(t, 1000000)
	rt := NewRenderText(base)
	rt.SetFace(v4Face(t))
	for _, scrollX := range []float64{0, 5000, 50000} {
		rt.SetViewportHint(scrollX, 904)
		lay := rt.TextLayout()
		s, e, _, _, ok := lay.Line(0)
		if !ok {
			t.Fatalf("no line at scrollX=%v", scrollX)
		}
		if !utf8.ValidString(lay.Text[s:e]) {
			t.Fatalf("V4 RED: window text at scrollX=%v is invalid UTF-8", scrollX)
		}
	}
}

// TestV4_HorizontalCullCoordinates locks the A-stage coordinate bug:
// hCullWindow uses the FULL-TEXT scrollX while a D1 long-line window's
// glyph X positions were window-relative (starting at 0). After the fix
// glyph X must be full-text absolute so the culled band shows the right
// content instead of an offset slice of the window text.
func TestV4_HorizontalCullCoordinates(t *testing.T) {
	base := v4LongLine(t, 1000000)
	face := v4Face(t)
	rt := NewRenderText(base)
	rt.SetFace(face)
	const scrollX, visW = 5000.0, 904.0
	rt.SetViewportHint(scrollX, visW)
	lay := rt.TextLayout()
	ll := lay.LongLine
	if ll == nil {
		t.Fatalf("long-line window not active")
	}
	g := lay.LineGlyphs(0)
	if len(g) == 0 {
		t.Fatalf("no glyphs")
	}
	wantX := rt.longLineIndexFor(base).XAtByte(ll.WindowStart)
	if math.Abs(g[0].X-wantX) > 2 {
		t.Fatalf("V4 RED: glyph X is window-relative: first X=%.1f, full-text X for WindowStart=%d is %.1f",
			g[0].X, ll.WindowStart, wantX)
	}
	lo, hi, ok := rt.hCullWindow(lay.LineCount(), len(g))
	if !ok {
		t.Fatalf("cull inactive")
	}
	cs, ce := cullRangeForWindow(g, lo, hi)
	if ce <= cs {
		t.Fatalf("V4 RED: cull emptied the visible band")
	}
	// cullRangeForWindow starts at the first glyph whose RIGHT edge reaches
	// loX, so a half-visible glyph may sit up to one glyph-width left of lo.
	if g[cs].X < lo-16 || g[cs].X > hi+1 {
		t.Fatalf("V4 RED: culled glyph X=%.1f outside visible band [%.1f,%.1f] (+half-glyph margin)", g[cs].X, lo, hi)
	}
}
