//go:build !nogpu

package gpu

import (
	"testing"
)

// TestStashGlyphQuads_SurviveStoreReuse reproduces the R18 text-vanishing bug:
// glyph batches live in rc.glyphMaskQuadStore, which is truncated by the layer
// flush and then reused by layer/sibling draws. A shallow stash (Quads slice
// pointing into the component store) would be clobbered before unstash, making
// parent text render with the wrong glyph positions. The stash must own the
// quad payload (opt22 pattern).
func TestStashGlyphQuads_SurviveStoreReuse(t *testing.T) {
	rc := &GPURenderContext{hasPendingTarget: true}

	legend := []GlyphMaskQuad{
		{X0: 16, Y0: 17, X1: 27, Y1: 29, Page: 0},
		{X0: 24, Y0: 73, X1: 33, Y1: 84, Page: 0},
		{X0: 23, Y0: 98, X1: 155, Y1: 219, Page: 0},
	}
	rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore, legend...)
	rc.pendingGlyphMaskBatches = []GlyphMaskBatch{{Quads: rc.glyphMaskQuadStore[:len(legend)]}}

	rc.stashPresentPending()
	if !rc.presentStash.active || len(rc.presentStash.glyph) != 1 {
		t.Fatalf("stash active=%v glyph=%d", rc.presentStash.active, len(rc.presentStash.glyph))
	}

	// Layer flush truncates the component store and a sibling draw reuses it
	// (the exact aliasing that corrupted R18 before the fix).
	rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
	rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore,
		GlyphMaskQuad{X0: 324, Y0: 306, X1: 336, Y1: 318, Page: 0},
		GlyphMaskQuad{X0: 675, Y0: 260, X1: 679, Y1: 270, Page: 0},
		GlyphMaskQuad{X0: 744, Y0: 260, X1: 844, Y1: 360, Page: 0},
	)

	rc.unstashPresentPending()
	got := rc.pendingGlyphMaskBatches[0].Quads
	if len(got) != len(legend) {
		t.Fatalf("restored quads=%d want %d", len(got), len(legend))
	}
	for i := range legend {
		if got[i] != legend[i] {
			t.Fatalf("quad[%d] corrupted: got %+v want %+v", i, got[i], legend[i])
		}
	}
}

// TestStashGlyphQuads_MultipleBatchesKeepsOrder covers merging stashes from
// multiple layer enter/exit in one frame: per-batch ranges must stay intact
// and in original order after rehoming into stash-owned storage.
func TestStashGlyphQuads_MultipleBatchesKeepsOrder(t *testing.T) {
	rc := &GPURenderContext{hasPendingTarget: true}

	store := []GlyphMaskQuad{
		{X0: 1, Y0: 1, X1: 2, Y1: 2, Page: 0},
		{X0: 3, Y0: 3, X1: 4, Y1: 4, Page: 0},
		{X0: 5, Y0: 5, X1: 6, Y1: 6, Page: 0},
		{X0: 7, Y0: 7, X1: 8, Y1: 8, Page: 0},
		{X0: 9, Y0: 9, X1: 10, Y1: 10, Page: 0},
	}
	rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore, store...)
	rc.pendingGlyphMaskBatches = []GlyphMaskBatch{
		{Quads: rc.glyphMaskQuadStore[0:2]},
		{Quads: rc.glyphMaskQuadStore[2:4]},
		{Quads: rc.glyphMaskQuadStore[4:5]},
	}
	rc.stashPresentPending()

	// Second stash in the same frame (another layer): append a new batch and
	// re-stash — ranges must merge without overlap.
	rc.glyphMaskQuadStore = rc.glyphMaskQuadStore[:0]
	rc.glyphMaskQuadStore = append(rc.glyphMaskQuadStore, GlyphMaskQuad{X0: 42, Y0: 42, X1: 43, Y1: 43, Page: 0})
	rc.pendingGlyphMaskBatches = []GlyphMaskBatch{{Quads: rc.glyphMaskQuadStore[0:1]}}
	rc.hasPendingTarget = true // layer paint queues on a new target
	rc.stashPresentPending()

	want := [][]GlyphMaskQuad{
		store[0:2], store[2:4], store[4:5],
		{{X0: 42, Y0: 42, X1: 43, Y1: 43, Page: 0}},
	}
	if len(rc.presentStash.glyph) != len(want) {
		t.Fatalf("stashed batches=%d want %d", len(rc.presentStash.glyph), len(want))
	}
	for bi := range want {
		q := rc.presentStash.glyph[bi].Quads
		if len(q) != len(want[bi]) {
			t.Fatalf("batch %d quads=%d want %d", bi, len(q), len(want[bi]))
		}
		for qi := range want[bi] {
			if q[qi] != want[bi][qi] {
				t.Fatalf("batch %d quad %d: got %+v want %+v", bi, qi, q[qi], want[bi][qi])
			}
		}
	}
}
