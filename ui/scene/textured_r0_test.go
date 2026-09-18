package scene_test

import (
	"testing"

	"github.com/energye/gpui/ui/scene"
)

// R0-5 wiring: with no share published (unit-test env) a lone window keeps
// historic behavior — EnsureCapacity grows to the working set and records
// no budget refusals.
func TestPictureCache_EnsureCapacity_SingleWindowUncapped(t *testing.T) {
	c := scene.NewPictureTextureCache(nil, 0)
	c.EnsureCapacity(200)
	if got := c.BudgetRefusals(); got != 0 {
		t.Fatalf("single-window refusals=%d want 0", got)
	}
	// Shrink requests never move the cap (historic behavior).
	c.EnsureCapacity(10)
	if got := c.BudgetRefusals(); got != 0 {
		t.Fatalf("shrink refusals=%d want 0", got)
	}
}
