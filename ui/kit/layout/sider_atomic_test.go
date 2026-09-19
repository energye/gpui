package layout_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/layout"
)

// TestSider_EventFlagsRace locks the atomic event flags (R2-6): UI event
// writes (collapse/hover/focus) vs concurrent readers must be race-clean.
// Run with -race.
func TestSider_EventFlagsRace(t *testing.T) {
	s := layout.NewSider()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			s.SetCollapsed(j%2 == 0)
			s.SetTriggerHovered(j%2 == 0)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = s.CollapsedState()
				_ = s.TriggerHovered()
				_ = s.TriggerHasFocus()
			}
		}()
	}
	wg.Wait()
}
