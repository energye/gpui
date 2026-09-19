package spin_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/spin"
)

// TestSpin_PhaseRacesTick locks the atomic spinner phase (R2-6): UI Tick
// vs concurrent readers must be race-clean. Run with -race.
func TestSpin_PhaseRacesTick(t *testing.T) {
	s := spin.NewSpin(nil)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			s.Tick(0.016)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				ph := s.Phase()
				if ph < 0 || ph >= 1 {
					t.Errorf("phase=%v want [0,1)", ph)
					return
				}
			}
		}()
	}
	wg.Wait()
}
