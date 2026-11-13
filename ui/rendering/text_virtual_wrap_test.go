package rendering

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/render/recording"
	"github.com/energye/gpui/ui/scene"
)

func collectRecordedRowYs(l scene.Layer, ys map[int]bool) {
	if l == nil {
		return
	}
	if pl, ok := l.(*scene.PictureLayer); ok && pl != nil {
		for _, cmd := range pl.Recording.Commands() {
			switch c := cmd.(type) {
			case recording.DrawGlyphsCommand:
				if len(c.Glyphs) > 0 {
					ys[int(math.Round(c.Y*10))] = true
				}
			case recording.DrawTextCommand:
				if c.Text != "" {
					ys[int(math.Round(c.Y*10))] = true
				}
			}
		}
	}
	for _, c := range l.Children() {
		collectRecordedRowYs(c, ys)
	}
}

// TestRenderText_VirtualWrappedRowsRecorded locks the V3b paint path: a
// wrapped virtual line records one command per visual row at that row's top,
// not one command per logical line. Before the change only row 0 was
// recorded, so every continuation row was missing from the retained picture
// while the live path drew the whole line.
func TestRenderText_VirtualWrappedRowsRecorded(t *testing.T) {
	face, _, err := TryLoadDefaultFace(16)
	if err != nil {
		t.Skip(err)
	}
	lines := make([]string, 5000)
	for i := range lines {
		lines[i] = fmt.Sprintf("row %d: %s", i, strings.Repeat("abcdef ghijk ", 12))
	}
	src := newSliceLineSource(lines)
	const maxW = 240.0
	view := NewWrappedTextViewport(src, face, 16, 1.2, maxW, 0, 128)

	rt := NewRenderText("")
	rt.FontSize = 16
	rt.SetFace(face)
	rt.SetVirtualView(view)
	rt.Layout(Tight(maxW, 400))
	const scrollY, visH = 2000.0, 400.0
	rt.SetViewportRect(0, maxW, scrollY, visH)

	root := NewRenderBox(rt)
	root.Layout(Tight(maxW, 400))
	pkt := BuildLayerTree(root).BuildPacket(1, 1, maxW, 400)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	got := map[int]bool{}
	collectRecordedRowYs(pkt.Root, got)
	if len(got) == 0 {
		t.Fatal("virtual wrapped paint recorded no rows")
	}

	first, last := view.Prepare(scrollY, visH)
	ascent := rt.fontSize() * 0.8
	if m, ok := rt.Metrics(); ok && m.Ascent > 0 {
		ascent = m.Ascent
	}
	want := map[int]bool{}
	rows := 0
	for i := first; i <= last; i++ {
		vl, ok := view.Line(i)
		if !ok || vl.Layout == nil {
			t.Fatalf("line %d not materialized", i)
		}
		for r := 0; r < vl.Layout.LineCount(); r++ {
			if LineRowText(vl.Layout, r) == "" {
				continue
			}
			y := ascent + view.LineTop(i) + vl.Layout.LineTop(r)
			want[int(math.Round(y*10))] = true
			rows++
		}
	}
	if rows <= last-first+1 {
		t.Fatalf("fixture did not wrap: %d rows over %d lines", rows, last-first+1)
	}
	for y := range want {
		if !got[y] {
			t.Fatalf("visual row y=%.1f missing from recording (recorded %d rows, want %d)", float64(y)/10, len(got), len(want))
		}
	}
	if len(got) != len(want) {
		t.Fatalf("recorded %d rows, want %d", len(got), len(want))
	}
}
