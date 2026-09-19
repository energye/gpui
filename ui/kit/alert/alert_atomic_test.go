package alert_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/alert"
)

// TestAlert_EventFlagsRace locks the atomic event flags (R2-6).
// Run with -race.
func TestAlert_EventFlagsRace(t *testing.T) {
	a := alert.NewAlert("t")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			a.SetClosable(j%2 == 0)
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = a.Closable()
				_ = a.Hidden()
				_ = a.Visible()
			}
		}()
	}
	wg.Wait()
}
