package splitter_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// TestSplitter_EventStateRace locks the atomic drag/focus/hover state
// (R2-6): UI event writes (drag, focus, hover) vs concurrent readers must
// be race-clean. Run with -race.
func TestSplitter_EventStateRace(t *testing.T) {
	s := newTwoPanelSplitter(600, 400)
	layoutSplitter(t, s, rendering.Tight(600, 400))
	if s.BarCount() != 1 {
		t.Fatalf("bars=%d want 1", s.BarCount())
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			if s.BeginDrag(0) {
				s.UpdateDrag(float64(j%10) - 5)
				s.EndDrag()
			}
			s.FocusBar(0)
			s.BlurBar()
			s.SetBarHover(0, j%2 == 0)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = s.DraggingBar()
				_ = s.BarFocused(0)
				_ = s.Focused()
				_ = s.BarHovered(0)
			}
		}()
	}
	wg.Wait()
}
