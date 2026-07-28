package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func TestRenderText_MeasureCacheHit(t *testing.T) {
	// Long wrap text so measureLine is called many times per layout (wrap + ellipsis paths).
	txt := rendering.NewRenderText("hello measure cache words repeated again hello measure cache words")
	txt.FontSize = 14
	txt.ApproxCharW = 0.55
	txt.SetMaxWidth(80)

	// First layout: misses populate cache.
	_ = txt.Layout(rendering.Loose(400, 400))
	hits1, miss1 := txt.MeasureCacheStats()
	if miss1 < 1 {
		t.Fatalf("first layout miss=%d want ≥1", miss1)
	}
	// Within a single layout, repeated substrings should already hit.
	if hits1 < 1 {
		// Some wrap paths may still be all-unique; force second dirty layout.
		txt.MarkNeedsLayout()
		txt.ResetMeasureCacheStats()
		_ = txt.Layout(rendering.Loose(400, 400))
		hits2, miss2 := txt.MeasureCacheStats()
		if hits2 < 1 {
			t.Fatalf("second dirty layout hits=%d miss=%d want hits≥1", hits2, miss2)
		}
		if hits2 < miss2 {
			t.Fatalf("second layout hits=%d miss=%d — cache not effective", hits2, miss2)
		}
	}

	// SetText must invalidate → cold miss again.
	txt.SetText("completely different string for measure")
	txt.ResetMeasureCacheStats()
	_ = txt.Layout(rendering.Loose(400, 400))
	hits3, miss3 := txt.MeasureCacheStats()
	if miss3 < 1 {
		t.Fatalf("after SetText miss=%d want ≥1 (cache invalidated)", miss3)
	}
	_ = hits3
}

func TestRenderText_MeasureCacheStableWidth(t *testing.T) {
	txt := rendering.NewRenderText("stable width gpui")
	txt.FontSize = 16
	txt.ApproxCharW = 0.6
	a := txt.Layout(rendering.Loose(500, 100))
	b := txt.Layout(rendering.Loose(500, 100))
	if a.Width != b.Width || a.Height != b.Height {
		t.Fatalf("size drifted %v → %v", a, b)
	}
}
