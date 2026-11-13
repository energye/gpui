package textinput

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/ui/rendering"
)

func TestEditorLineSource_Viewport(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", 1000000))
	src := editorLineSource{ed: e}
	v := rendering.NewTextViewport(src, nil, 14, 1.2, 0)
	if got, want := v.LineCount(), e.ensureBuf().LineCount(); got != want {
		t.Fatalf("LineCount=%d want %d", got, want)
	}
	for _, i := range []int{0, 1, 543210, v.LineCount() - 1} {
		got, ok := v.Line(i)
		if !ok {
			t.Fatalf("Line(%d) not ok", i)
		}
		wantText := e.ensureBuf().LineAt(i)
		wantStart := e.ensureBuf().LineStart(i)
		if got.Layout == nil || got.Layout.Text != wantText {
			t.Fatalf("line %d text mismatch", i)
		}
		if got.Start != wantStart || got.End != wantStart+len(wantText) {
			t.Fatalf("line %d span=[%d,%d) want [%d,%d)", i, got.Start, got.End, wantStart, wantStart+len(wantText))
		}
	}
	v.Invalidate()
	if v.CacheLen() != 0 {
		t.Fatalf("Invalidate left %d lines", v.CacheLen())
	}
}

func editorViewFirstScreen(t *testing.T, lines, reps int) time.Duration {
	t.Helper()
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("line content here\n", lines))
	src := editorLineSource{ed: e}
	var ds []time.Duration
	for i := 0; i < reps; i++ {
		v := rendering.NewTextViewport(src, nil, 14, 1.2, 0)
		t0 := time.Now()
		v.Prepare(0, 42*14*1.2)
		ds = append(ds, time.Since(t0))
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

func TestEditorLineSource_FirstScreenFlat(t *testing.T) {
	small := editorViewFirstScreen(t, 10000, 10)
	big := editorViewFirstScreen(t, 1000000, 10)
	ratio := float64(big) / float64(small)
	t.Logf("editor line source first screen 1e4=%v 1e6=%v ratio=%.2f", small, big, ratio)
	if ratio > 3 {
		t.Fatalf("首屏随总行数增长:1e6/1e4=%.1f,门禁≤3", ratio)
	}
}
