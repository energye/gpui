package rendering_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// recordedBand aggregates recorded text coverage: glyph-op pens stay absolute
// (bulk keeps glyph.X at pen ox; partitions rebase to ox+off with shifted
// glyphs), so op.X+g.X reconstructs the text-origin band the texture covers.
type recordedBand struct {
	glyphs int
	strs   []string
	minX   float64
	maxX   float64
	init   bool
}

func collectRecordedBand(l scene.Layer, b *recordedBand) {
	if l == nil || b == nil {
		return
	}
	if pl, ok := l.(*scene.PictureLayer); ok && pl != nil {
		for _, op := range pl.Picture.Ops {
			switch op.Kind {
			case scene.OpDrawShapedGlyphs:
				for _, g := range op.Glyphs {
					x0, x1 := op.X+g.X, op.X+g.X+g.XAdvance
					if !b.init || x0 < b.minX {
						b.minX = x0
					}
					if !b.init || x1 > b.maxX {
						b.maxX = x1
					}
					b.init = true
				}
				b.glyphs += len(op.Glyphs)
			case scene.OpDrawString:
				if op.Text != "" {
					b.strs = append(b.strs, op.Text)
				}
			}
		}
	}
	for _, c := range l.Children() {
		collectRecordedBand(c, b)
	}
}

// TestRecordViewportCullsLongLine locks the B/C fix: a long single line with
// a tail viewport hint must record only the visible band (tail), not the
// whole line head, so the texture covers what Paint submits.
func TestRecordViewportCullsLongLine(t *testing.T) {
	face, _, err := rendering.TryLoadDefaultFace(16)
	if err != nil {
		t.Skip(err)
	}
	txt := strings.Repeat("abcdefghij", 500) // 5000 chars, B-like magnitude
	rt := rendering.NewRenderText(txt)
	rt.FontSize = 16
	rt.SetFace(face)
	rt.Layout(rendering.Tight(1200, 40))
	lay := rt.TextLayout()
	if lay == nil || lay.LineCount() != 1 {
		t.Fatalf("want 1 layout line, got %+v", lay)
	}
	_, _, maxW, _, _ := lay.Line(0)
	if maxW < 5000 {
		t.Skipf("line too narrow for cull test (%.0fpx, need shaped face)", maxW)
	}
	const visW = 800.0
	rt.SetViewportHint(maxW-visW+4, visW) // scrolled to tail, like B preload

	root := rendering.NewRenderBox(rt)
	root.Layout(rendering.Tight(1200, 600))
	pkt := rendering.BuildLayerTree(root).BuildPacket(1, 1, 1200, 600)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	var band recordedBand
	collectRecordedBand(pkt.Root, &band)
	full := len(lay.LineGlyphs(0))
	assertCulledBand(t, &band, full, maxW, visW)
}

func assertCulledBand(t *testing.T, band *recordedBand, full int, maxW, visW float64) {
	t.Helper()
	recorded := band.glyphs
	for _, s := range band.strs {
		recorded += len([]rune(s))
	}
	if recorded == 0 {
		t.Fatal("recorded nothing")
	}
	if recorded >= full {
		t.Fatalf("recorded %d glyphs, want culled band << %d", recorded, full)
	}
	scrollX := maxW - visW + 4
	if band.init {
		if band.maxX < maxW-200 {
			t.Fatalf("recorded band maxX=%.0f misses tail (line maxW=%.0f)", band.maxX, maxW)
		}
		if band.minX < scrollX-600 {
			t.Fatalf("recorded band minX=%.0f still carries line head (scrollX=%.0f)", band.minX, scrollX)
		}
		return
	}
	joined := strings.Join(band.strs, "")
	if len([]rune(joined)) >= full {
		t.Fatalf("recorded %d runes, want culled band << %d", len([]rune(joined)), full)
	}
}

// TestRecordViewportCullsCompositeFace covers the B/C runtime route: mixed
// CJK+Latin text on a MultiFace paints per-face partitions, so the record
// must cull each partition to the visible band as well.
func TestRecordViewportCullsCompositeFace(t *testing.T) {
	face, _, err := text.LoadMultiFace(16)
	if err != nil {
		t.Skip(err)
	}
	txt := strings.Repeat("a世界bHello你好", 400) // B-like mixed line
	rt := rendering.NewRenderText(txt)
	rt.FontSize = 16
	rt.SetFace(face)
	rt.Layout(rendering.Tight(1200, 40))
	lay := rt.TextLayout()
	if lay == nil || lay.LineCount() != 1 {
		t.Fatalf("want 1 layout line, got %+v", lay)
	}
	_, _, maxW, _, _ := lay.Line(0)
	if maxW < 5000 {
		t.Skipf("line too narrow for cull test (%.0fpx, need shaped face)", maxW)
	}
	const visW = 800.0
	rt.SetViewportHint(maxW-visW+4, visW) // scrolled to tail, like B preload

	root := rendering.NewRenderBox(rt)
	root.Layout(rendering.Tight(1200, 600))
	pkt := rendering.BuildLayerTree(root).BuildPacket(1, 1, 1200, 600)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	var band recordedBand
	collectRecordedBand(pkt.Root, &band)
	assertCulledBand(t, &band, len(lay.LineGlyphs(0)), maxW, visW)
}
