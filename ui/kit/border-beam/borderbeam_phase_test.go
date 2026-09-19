package border_beam_test

import (
	"sync"
	"testing"

	border_beam "github.com/energye/gpui/ui/kit/border-beam"
	"github.com/energye/gpui/ui/rendering"
)

// TestBorderBeam_PhaseRacesTick locks the atomic phase (R2-6).
// Run with -race.
func TestBorderBeam_PhaseRacesTick(t *testing.T) {
	b := border_beam.NewBorderBeam(rendering.NewRenderBox())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			b.Tick(0.016)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				ph := b.Phase()
				if ph < 0 || ph >= 1 {
					t.Errorf("phase=%v want [0,1)", ph)
					return
				}
			}
		}()
	}
	wg.Wait()
}
