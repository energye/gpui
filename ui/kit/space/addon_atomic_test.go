package space_test

import (
	"sync"
	"testing"

	"github.com/energye/gpui/ui/kit/space"
)

// TestSpaceAddon_CompactEdgesRace locks the atomic edge flags (R2-6):
// SetCompactEdges writes (UI/layout), paint-time readers (CompactEdges /
// EffectiveRadius) load concurrently. Run with -race.
func TestSpaceAddon_CompactEdgesRace(t *testing.T) {
	a := space.NewSpaceAddon()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			a.SetCompactEdges(j%2 == 0, j%3 == 0)
		}
	}()
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			_, _, _ = a.CompactEdges()
			_ = a.EffectiveRadius()
		}
	}()
	wg.Wait()
}
