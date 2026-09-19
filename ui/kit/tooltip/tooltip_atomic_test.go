package tooltip_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/tooltip"
)

// TestTooltip_EventFlagsRace locks the atomic event flags (R2-6): UI event
// writes (hover/focus/disabled) vs concurrent readers must be race-clean.
// Run with -race.
func TestTooltip_EventFlagsRace(t *testing.T) {
	tp := tooltip.NewTooltip("tip")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			tp.HoverEnter()
			tp.HoverLeave()
			tp.SetDisabled(j%2 == 0)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = tp.Hovered()
				_ = tp.Focused()
				_ = tp.Disabled()
			}
		}()
	}
	wg.Wait()
}
