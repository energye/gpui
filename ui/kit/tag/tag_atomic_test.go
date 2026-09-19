package tag_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/tag"
)

// TestTag_EventFlagsRace locks the atomic event flags (R2-6): UI event
// writes (close/focus) vs concurrent readers must be race-clean.
// Run with -race.
func TestTag_EventFlagsRace(t *testing.T) {
	tg := tag.NewTag("t")
	tg.SetClosable(true) // setup only: Close/Show below must actually flip
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			// Close/Show alternate so every iteration writes.
			// (Tag is closable by default in tests? ensure via Close+Show.)
			tg.Close()
			tg.Show()
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = tg.Hidden()
				_ = tg.Focused()
			}
		}()
	}
	wg.Wait()
}
