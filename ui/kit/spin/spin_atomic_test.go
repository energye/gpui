package spin

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// TestSpin_IndSizeRacesLayout locks the R2-6 write-back elimination: the
// custom-indicator size is stored by Layout (UI) and read on both threads
// (measure helpers on UI, paintCustom on raster) through an atomic value.
// Paint never lays out nor writes back. Run with -race. Red on the old
// bare fields (raster write vs UI read/write).
func TestSpin_IndSizeRacesLayout(t *testing.T) {
	s := NewSpin(nil)
	s.SetIndicator(rendering.NewRenderColorBox(40, 30, 1, 0, 0, 1))
	s.Layout(rendering.Loose(1000, 1000))
	if got := s.indicatorHeight(); got <= 0 {
		t.Fatalf("indicatorHeight=%v want >0 after layout", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = s.indicatorHeight()
				if v, ok := s.indSize.Load().(rendering.Size); ok {
					_, _ = v.Width, v.Height
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			s.Layout(rendering.Loose(1000, 1000))
		}
	}()
	wg.Wait()
}
