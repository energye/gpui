package rendering

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

type sliceLineSource struct {
	lines  []string
	starts []int
}

func newSliceLineSource(lines []string) *sliceLineSource {
	starts := make([]int, len(lines))
	off := 0
	for i, ln := range lines {
		starts[i] = off
		off += len(ln) + 1
	}
	return &sliceLineSource{lines: lines, starts: starts}
}

func (s *sliceLineSource) LineCount() int      { return len(s.lines) }
func (s *sliceLineSource) LineAt(k int) string { return s.lines[k] }
func (s *sliceLineSource) LineStart(k int) int { return s.starts[k] }

type constLineSource struct {
	n     int
	text  string
	width int
}

func (s *constLineSource) LineCount() int      { return s.n }
func (s *constLineSource) LineAt(k int) string { return s.text }
func (s *constLineSource) LineStart(k int) int { return k * s.width }

func TestTextViewport_MatchesFullLayout(t *testing.T) {
	var lines []string
	for i := 0; i < 300; i++ {
		switch i % 5 {
		case 0:
			lines = append(lines, "")
		case 1:
			lines = append(lines, strings.Repeat("a", i%37))
		case 2:
			lines = append(lines, fmt.Sprintf("line %d content 世界", i))
		case 3:
			lines = append(lines, "👨‍👩‍👧 emoji")
		default:
			lines = append(lines, "hello")
		}
	}
	doc := strings.Join(lines, "\n")
	full := BuildTextLayout(doc, nil, 14, 0, 1.2)
	src := newSliceLineSource(lines)
	view := NewTextViewport(src, nil, 14, 1.2, 0)
	if view.LineCount() != full.LineCount() {
		t.Fatalf("LineCount=%d want %d", view.LineCount(), full.LineCount())
	}
	for i := 0; i < full.LineCount(); i++ {
		wl, ok := view.Line(i)
		if !ok {
			t.Fatalf("Line(%d) not ok", i)
		}
		fs, fe, fw, fh, _ := full.Line(i)
		if wl.Start != fs || wl.End != fe || wl.Width != fw || wl.Height != fh {
			t.Fatalf("line %d got [%d,%d) w=%.2f h=%.2f want [%d,%d) w=%.2f h=%.2f",
				i, wl.Start, wl.End, wl.Width, wl.Height, fs, fe, fw, fh)
		}
		if view.LineTop(i) != full.LineTop(i) {
			t.Fatalf("line %d top=%.2f want %.2f", i, view.LineTop(i), full.LineTop(i))
		}
	}
}

func firstScreenTime(src LineSource, reps int) time.Duration {
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		view := NewTextViewport(src, nil, 14, 1.2, 0)
		lh := view.lineH
		t0 := time.Now()
		view.Prepare(0, 42*lh)
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func TestTextViewport_FirstScreenFlat(t *testing.T) {
	lineText := "line content here"
	small := &constLineSource{n: 10000, text: lineText, width: len(lineText) + 1}
	big := &constLineSource{n: 1000000, text: lineText, width: len(lineText) + 1}
	ds := firstScreenTime(small, 20)
	db := firstScreenTime(big, 20)
	ratio := float64(db) / float64(ds)
	t.Logf("first screen 1e4=%v 1e6=%v ratio=%.2f", ds, db, ratio)
	if ratio > 3 {
		t.Fatalf("首屏疑似随总行数增长:1e6/1e4=%.1f,门禁≤3", ratio)
	}
	view := NewTextViewport(big, nil, 14, 1.2, 0)
	if view.LineCount() != 1000000 {
		t.Fatalf("LineCount=%d", view.LineCount())
	}
	if view.CacheLen() > 3*42 {
		t.Fatalf("cache=%d 未保持 O(可见行)", view.CacheLen())
	}
	wantTotal := float64(1000000) * view.lineH
	if view.TotalHeight() != wantTotal {
		t.Fatalf("TotalHeight=%.2f want %.2f", view.TotalHeight(), wantTotal)
	}
}

func TestTextViewport_ScrollKeepsCacheBounded(t *testing.T) {
	src := &constLineSource{n: 10000, text: "line content here", width: 18}
	view := NewTextViewport(src, nil, 14, 1.2, 128)
	h := 42 * view.lineH
	for y := 0.0; y < view.TotalHeight()-h; y += 4 * h {
		first, last := view.Prepare(y, h)
		if first < 0 || last < first {
			t.Fatalf("window y=%.0f -> [%d,%d]", y, first, last)
		}
		if view.CacheLen() > 3*42+2 {
			t.Fatalf("y=%.0f cache=%d 超窗", y, view.CacheLen())
		}
	}
}

func TestTextViewport_VisibleRange(t *testing.T) {
	src := &constLineSource{n: 1000, text: "x", width: 2}
	view := NewTextViewport(src, nil, 14, 1.2, 0)
	lh := view.lineH
	cases := []struct {
		y     float64
		h     float64
		first int
		last  int
	}{
		{0, 10 * lh, 0, 10},
		{5 * lh, 10 * lh, 5, 15},
		{995 * lh, 10 * lh, 995, 999},
		{0, 0, 0, -1},
	}
	for _, c := range cases {
		f, l := view.VisibleRange(c.y, c.h)
		if f != c.first || l != c.last {
			t.Fatalf("VisibleRange(%.1f,%.1f)=[%d,%d] want [%d,%d]", c.y, c.h, f, l, c.first, c.last)
		}
	}
}
