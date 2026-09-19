package popover_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/popover"
)

// TestPopover_EventFlagsRace locks the atomic event flags (R2-6): UI event
// writes (hover/press/focus/disabled) vs concurrent readers must be
// race-clean. Run with -race.
func TestPopover_EventFlagsRace(t *testing.T) {
	p := popover.NewPopover("trigger")
	p.SetTitle("t")
	p.SetContent("c")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			p.PointerMove(float64(j%50), 5)
			p.PointerDown(float64(j%50), 5)
			p.PointerUp(float64(j%50), 5)
			p.SetDisabled(j%2 == 0)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = p.Hovered()
				_ = p.Focused()
				_ = p.Disabled()
			}
		}()
	}
	wg.Wait()
}
