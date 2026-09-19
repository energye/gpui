package progress_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/progress"
)

// TestProgress_PhaseRacesTick locks the atomic phase (R2-6).
// Run with -race.
func TestProgress_PhaseRacesTick(t *testing.T) {
	p := progress.NewProgress(50)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			p.Tick(0.016)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				ph := p.Phase()
				if ph < 0 || ph >= 1 {
					t.Errorf("phase=%v want [0,1)", ph)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// TestProgress_PercentRacesSet locks the atomic percent (R2-6): UI
// SetPercent vs concurrent readers must be race-clean. Run with -race.
func TestProgress_PercentRacesSet(t *testing.T) {
	p := progress.NewProgress(10)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			p.SetPercent(float64(j % 101))
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				v := p.Percent()
				if v < 0 || v > 100 {
					t.Errorf("percent=%v want [0,100]", v)
					return
				}
			}
		}()
	}
	wg.Wait()
}
