package rendering

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestTextViewport_WrappedMatchesFullLayout locks the wrapped visual-row
// mapping: a logical line's rows and heights must match the monolithic
// TextLayout row for row, and the viewport's measured offsets/heights must
// reproduce the full layout's prefix geometry.
func TestTextViewport_WrappedMatchesFullLayout(t *testing.T) {
	var lines []string
	for i := 0; i < 240; i++ {
		switch i % 6 {
		case 0:
			lines = append(lines, "")
		case 1:
			lines = append(lines, strings.Repeat("wrap ", i%9))
		case 2:
			lines = append(lines, fmt.Sprintf("第 %d 行：回绕文本，中文内容用来撑宽每一行。", i))
		case 3:
			lines = append(lines, "👨‍👩‍👧 family emoji line")
		case 4:
			lines = append(lines, "short")
		default:
			lines = append(lines, strings.Repeat("a", 80+i%30))
		}
	}
	const maxW = 120.0
	doc := strings.Join(lines, "\n")
	full := BuildTextLayout(doc, nil, 14, maxW, 1.2)
	if full == nil || full.LineCount() == 0 {
		t.Fatal("full layout empty")
	}
	src := newSliceLineSource(lines)
	view := NewWrappedTextViewport(src, nil, 14, 1.2, maxW, 0, 0)
	if view.LineCount() != len(lines) {
		t.Fatalf("LineCount=%d want %d", view.LineCount(), len(lines))
	}
	if view.MaxWidth() != maxW {
		t.Fatalf("MaxWidth=%.1f want %.1f", view.MaxWidth(), maxW)
	}

	row := 0
	for i, s := range lines {
		vl, ok := view.Line(i)
		if !ok {
			t.Fatalf("Line(%d) not ok", i)
		}
		if vl.Start != src.starts[i] || vl.End != src.starts[i]+len(s) {
			t.Fatalf("line %d range [%d,%d) want [%d,%d)", i, vl.Start, vl.End, src.starts[i], src.starts[i]+len(s))
		}
		per := BuildTextLayout(s, nil, 14, maxW, 1.2)
		rows := per.LineCount()
		if vl.Rows != rows {
			t.Fatalf("line %d Rows=%d want %d", i, vl.Rows, rows)
		}
		wantTop := full.LineTop(row)
		if vlTop := view.LineTop(i); vlTop != wantTop {
			t.Fatalf("line %d top=%.3f want %.3f (rows above=%d)", i, vlTop, wantTop, row)
		}
		maxRowW := 0.0
		for r := 0; r < rows; r++ {
			fs, fe, fw, fh, ok := full.Line(row + r)
			ps, pe, pw, _, ok2 := per.Line(r)
			if !ok || !ok2 {
				t.Fatalf("line %d row %d missing (full rows=%d)", i, r, full.LineCount())
			}
			if fs != src.starts[i]+ps || fe != src.starts[i]+pe {
				t.Fatalf("line %d row %d range [%d,%d) want [%d,%d)", i, r, fs, fe, src.starts[i]+ps, src.starts[i]+pe)
			}
			if full.LineTop(row+r) != view.LineTop(i)+per.LineTop(r) {
				t.Fatalf("line %d row %d top=%.3f want %.3f", i, r, full.LineTop(row+r), view.LineTop(i)+per.LineTop(r))
			}
			if fh != per.LineHeight(r) {
				t.Fatalf("line %d row %d h=%.3f want %.3f", i, r, fh, per.LineHeight(r))
			}
			if fw != pw {
				t.Fatalf("line %d row %d w=%.3f want %.3f", i, r, fw, pw)
			}
			if fw > maxRowW {
				maxRowW = fw
			}
		}
		if vl.Width != maxRowW {
			t.Fatalf("line %d width=%.3f want max row width %.3f", i, vl.Width, maxRowW)
		}
		if rows == 0 {
			// Empty logical line: still one row in the full layout.
			if vl.Height != full.LineHeight(row) {
				t.Fatalf("empty line %d h=%.3f want %.3f", i, vl.Height, full.LineHeight(row))
			}
			row++
			continue
		}
		wantH := per.LineTop(rows-1) + per.LineHeight(rows-1)
		if vl.Height != wantH {
			t.Fatalf("line %d h=%.3f want %.3f", i, vl.Height, wantH)
		}
		row += rows
	}
	if row != full.LineCount() {
		t.Fatalf("mapped %d rows, full layout has %d", row, full.LineCount())
	}
	wantTotal := full.LineTop(full.LineCount()-1) + full.LineHeight(full.LineCount()-1)
	if d := view.TotalHeight() - wantTotal; d > 1e-6 || d < -1e-6 {
		t.Fatalf("TotalHeight=%.4f want %.4f", view.TotalHeight(), wantTotal)
	}
}

// TestTextViewport_PrepareCoversWrappedBand locks the measure→cover loop:
// after measuring, the returned window must cover [scrollY, scrollY+visH]
// even when the configured estimate disagrees with the measured wrapped
// heights.
func TestTextViewport_PrepareCoversWrappedBand(t *testing.T) {
	lh := lineHeightFor(nil, 14, 1.2)
	cases := []struct {
		name     string
		text     string
		maxW     float64
		estimate float64
	}{
		// Estimate says 5 rows, content is one short row: the estimated window
		// (visH/estimate lines) covers only a fraction of the band.
		{"估计偏大", "x", 160, 5 * lh},
		// Estimate says one row, content wraps into several: the measured
		// window overshoots the band (must stay bounded, not O(n)).
		{"估计偏小", strings.Repeat("wrap words ", 8), 160, lh},
	}
	const visH = 600.0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &constLineSource{n: 200000, text: c.text, width: len(c.text) + 1}
			view := NewWrappedTextViewport(src, nil, 14, 1.2, c.maxW, c.estimate, 128)
			scrollY := view.TotalHeight() * 0.5
			first, last := view.Prepare(scrollY, visH)
			if last < first || first < 0 {
				t.Fatalf("Prepare -> [%d,%d]", first, last)
			}
			if top := view.LineTop(first); first > 0 && top > scrollY+0.01 {
				t.Fatalf("window top=%.2f starts below scrollY=%.2f (blank band)", top, scrollY)
			}
			end, ok := view.Line(last)
			if !ok {
				t.Fatalf("Line(%d) not ok", last)
			}
			bottom := view.LineTop(last) + end.Height
			if last < view.LineCount()-1 && bottom < scrollY+visH-0.01 {
				t.Fatalf("window bottom=%.2f short of band end %.2f (blank band)", bottom, scrollY+visH)
			}
			if end.Height != lh && c.name == "估计偏大" {
				t.Fatalf("measured short line h=%.2f want %.2f", end.Height, lh)
			}
			if view.CacheLen() > 3*(last-first+1)+2 {
				t.Fatalf("cache=%d for window %d", view.CacheLen(), last-first+1)
			}
			if view.MeasuredCount() > 400 {
				t.Fatalf("measured %d lines for a %0.fpx viewport (O(n) window)", view.MeasuredCount(), visH)
			}
		})
	}
}

// TestTextViewport_LineCountTracksSource locks live line-count edits: the
// height map was built at construction, so growth/shrink must rebuild it
// instead of keeping the stale count (which hid the new last line).
func TestTextViewport_LineCountTracksSource(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	src := newSliceLineSource(lines)
	view := NewTextViewport(src, nil, 14, 1.2, 0)
	if view.LineCount() != 100 {
		t.Fatalf("LineCount=%d want 100", view.LineCount())
	}
	grown := make([]string, 150)
	copy(grown, lines)
	for i := 100; i < 150; i++ {
		grown[i] = "added"
	}
	*src = *newSliceLineSource(grown)
	if got := view.LineCount(); got != 150 {
		t.Fatalf("after growth LineCount=%d want 150", got)
	}
	if _, ok := view.Line(149); !ok {
		t.Fatalf("Line(149) hidden after growth")
	}
	if want := 150 * view.estimate; view.TotalHeight() != want {
		t.Fatalf("after growth TotalHeight=%.2f want %.2f", view.TotalHeight(), want)
	}
	*src = *newSliceLineSource(grown[:40])
	if got := view.LineCount(); got != 40 {
		t.Fatalf("after shrink LineCount=%d want 40", got)
	}
	if _, ok := view.Line(40); ok {
		t.Fatalf("Line(40) still reachable after shrink")
	}
	if _, ok := view.Line(39); !ok {
		t.Fatalf("Line(39) missing after shrink")
	}
}

// TestTextViewport_InvalidateDropsMeasuredHeights locks edit invalidation:
// wrapped heights depend on content, so Invalidate/InvalidateLine must drop
// stale measured overrides instead of keeping pre-edit geometry.
func TestTextViewport_InvalidateDropsMeasuredHeights(t *testing.T) {
	src := newSliceLineSource([]string{strings.Repeat("wrap ", 40), "short"})
	view := NewWrappedTextViewport(src, nil, 14, 1.2, 100, 0, 0)
	vl, ok := view.Line(0)
	if !ok || vl.Height == view.estimate {
		t.Fatalf("line 0 did not measure a wrapped height (h=%.2f est=%.2f)", vl.Height, view.estimate)
	}
	if view.MeasuredCount() != 1 {
		t.Fatalf("MeasuredCount=%d want 1", view.MeasuredCount())
	}
	view.InvalidateLine(0)
	if view.MeasuredCount() != 0 {
		t.Fatalf("InvalidateLine kept %d measured heights", view.MeasuredCount())
	}
	if view.TotalHeight() != 2*view.estimate {
		t.Fatalf("TotalHeight=%.2f want %.2f", view.TotalHeight(), 2*view.estimate)
	}
	if _, ok := view.Line(0); !ok || view.MeasuredCount() != 1 {
		t.Fatalf("re-measure after InvalidateLine failed")
	}
	view.Invalidate()
	if view.MeasuredCount() != 0 {
		t.Fatalf("Invalidate kept %d measured heights", view.MeasuredCount())
	}
	*src = *newSliceLineSource([]string{"a", "b", "c", "d", "e"})
	view.Invalidate()
	if view.LineCount() != 5 {
		t.Fatalf("Invalidate after growth LineCount=%d want 5", view.LineCount())
	}
}

// TestTextViewport_WrappedFirstScreenFlat locks scale-independence of the
// wrapped window: the first screen must not grow with total line count.
func TestTextViewport_WrappedFirstScreenFlat(t *testing.T) {
	text := strings.Repeat("wrapped line content ", 6)
	measure := func(src LineSource) time.Duration {
		var ds []time.Duration
		for i := 0; i < 20; i++ {
			view := NewWrappedTextViewport(src, nil, 14, 1.2, 180, 0, 0)
			t0 := time.Now()
			view.Prepare(0, 600)
			ds = append(ds, time.Since(t0))
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		return ds[len(ds)/2]
	}
	small := &constLineSource{n: 10000, text: text, width: len(text) + 1}
	big := &constLineSource{n: 1000000, text: text, width: len(text) + 1}
	dSmall, dBig := measure(small), measure(big)
	ratio := float64(dBig) / float64(dSmall)
	t.Logf("wrapped first screen 1e4=%v 1e6=%v ratio=%.2f", dSmall, dBig, ratio)
	if ratio > 3 {
		t.Fatalf("回绕首屏随总行数增长:1e6/1e4=%.1f,门禁≤3", ratio)
	}
	view := NewWrappedTextViewport(big, nil, 14, 1.2, 180, 0, 0)
	view.Prepare(0, 600)
	if view.MeasuredCount() > 200 {
		t.Fatalf("measured %d lines for first screen", view.MeasuredCount())
	}
	if view.CacheLen() > 3*200 {
		t.Fatalf("cache=%d for first screen", view.CacheLen())
	}
}

// TestTextViewport_WrappedScrollMeasuresBounded walks a large wrapped
// document: the measured override set and cache stay O(visible), never O(n).
func TestTextViewport_WrappedScrollMeasuresBounded(t *testing.T) {
	text := strings.Repeat("scroll wrap ", 8)
	src := &constLineSource{n: 1000000, text: text, width: len(text) + 1}
	view := NewWrappedTextViewport(src, nil, 14, 1.2, 150, 0, 128)
	const visH = 600.0
	y := 0.0
	for step := 0; step < 200; step++ {
		first, last := view.Prepare(y, visH)
		if last < first || first < 0 {
			t.Fatalf("step %d Prepare(%.1f) -> [%d,%d]", step, y, first, last)
		}
		if view.CacheLen() > 3*(last-first+1)+2 {
			t.Fatalf("step %d cache=%d window=%d", step, view.CacheLen(), last-first+1)
		}
		if view.MeasuredCount() > view.heights.MeasuredCap() {
			t.Fatalf("step %d measured=%d over cap %d", step, view.MeasuredCount(), view.heights.MeasuredCap())
		}
		y += 2 * visH
		if y >= view.TotalHeight()-visH {
			y = 0
		}
	}
}
