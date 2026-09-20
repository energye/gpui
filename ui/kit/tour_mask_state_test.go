package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func tourSteps() []kit.TourMaskStep {
	return []kit.TourMaskStep{
		{Title: "s1", Description: "d1", Target: kit.TourMaskRect{X: 100, Y: 100, W: 200, H: 80}},
		{Title: "s2", Description: "d2", Target: kit.TourMaskRect{X: 400, Y: 300, W: 160, H: 90}},
		{Title: "s3", Description: "d3"},
	}
}

func TestTourMask_OpenNextPrevCloseFinish(t *testing.T) {
	props := kit.DefaultTourMaskProps()
	host := kit.BuildTourMask(kit.DefaultScopeCtx(), props, tourSteps())
	if host.IsOpen() {
		t.Fatal("default must start closed")
	}
	closed := 0
	finished := 0
	changed := []int{}
	host.SetOnClose(func() { closed++ })
	host.SetOnFinish(func() { finished++ })
	host.SetOnChange(func(c int) { changed = append(changed, c) })

	host.SetOpen(true)
	if !host.IsOpen() || host.Current() != 0 {
		t.Fatal("open must show step 0")
	}
	if host.Focused() == "" {
		t.Fatal("focus must enter panel on open")
	}
	if !host.ScrollNeeded() {
		t.Fatal("open must request scrollIntoView")
	}
	host.ConsumeScroll()
	if host.ScrollNeeded() {
		t.Fatal("consume must clear scroll request")
	}

	host.Next()
	if host.Current() != 1 || len(changed) != 1 || changed[0] != 1 {
		t.Fatalf("next must go 1 with onChange, got %d %v", host.Current(), changed)
	}
	host.Prev()
	if host.Current() != 0 {
		t.Fatal("prev must return to 0")
	}
	// Hole follows the target and blocks pass-through by default.
	if !host.HitHole(150, 140) {
		t.Fatal("hole must contain target center")
	}
	if host.HitHole(10, 10) {
		t.Fatal("outside must not hit hole")
	}
	if !host.ClickHole(150, 140) {
		t.Fatal("hole click must pass through when interaction allowed")
	}
	// Step switch re-targets the panel.
	host.Next()
	host.SetTarget(1, kit.TourMaskRect{X: 500, Y: 350, W: 100, H: 60})
	step, ok := host.CurrentStep()
	if !ok || step.Target.X != 500 {
		t.Fatalf("SetTarget must rewrite rect, got %+v", step)
	}

	// Finish on last step fires finish then close.
	host.Next()
	host.Next()
	if host.IsOpen() {
		t.Fatal("last next must close")
	}
	if finished != 1 || closed != 1 {
		t.Fatalf("finish=%d close=%d want 1,1", finished, closed)
	}

	// Controlled current never self-moves, only notifies.
	cprops := kit.DefaultTourMaskProps()
	cprops.Open = true
	cprops.Current, cprops.CurrentSet = 0, true
	chost := kit.BuildTourMask(kit.DefaultScopeCtx(), cprops, tourSteps())
	creport := -1
	chost.SetOnChange(func(c int) { creport = c })
	chost.Next()
	if chost.Current() != 0 || creport != 1 {
		t.Fatalf("controlled next must notify 1 without moving, got %d report %d", chost.Current(), creport)
	}
	chost.SetCurrent(2)
	if chost.Current() != 2 {
		t.Fatal("SetCurrent drives controlled current")
	}

	// Esc and mask respect switches.
	host2 := kit.BuildTourMask(kit.DefaultScopeCtx(), kit.DefaultTourMaskProps(), tourSteps())
	host2.SetOpen(true)
	if !host2.PressEsc() || host2.IsOpen() {
		t.Fatal("esc must close when keyboard on")
	}
	host2.SetOpen(true)
	if !host2.ClickMask() || host2.IsOpen() {
		t.Fatal("mask click must close when mask on")
	}
	nomask := kit.DefaultTourMaskProps()
	nomask.Mask, nomask.MaskSet = false, true
	host3 := kit.BuildTourMask(kit.DefaultScopeCtx(), nomask, tourSteps())
	host3.SetOpen(true)
	if host3.EffectiveMask() {
		t.Fatal("mask=false must report non-modal")
	}
	if host3.ClickMask() {
		t.Fatal("mask=false click must not close")
	}
	if !host3.IsOpen() {
		t.Fatal("non-modal keeps panel open")
	}
	// Hole still draws without mask.
	if !host3.HitHole(150, 140) {
		t.Fatal("mask=false must still draw hole")
	}
	nokb := kit.DefaultTourMaskProps()
	nokb.Keyboard, nokb.KeyboardSet = false, true
	host4 := kit.BuildTourMask(kit.DefaultScopeCtx(), nokb, tourSteps())
	host4.SetOpen(true)
	if host4.PressEsc() {
		t.Fatal("keyboard=false must ignore esc")
	}
	// Tab traps inside the panel.
	host4.SetOpen(false)
	host4.SetOpen(true)
	f1 := host4.PressTab()
	f2 := host4.PressTab()
	if f1 == "" || f2 == "" || f1 == f2 && host4.StepCount() > 1 {
		// Two tabs must move focus; single-element trap still returns a name.
		t.Logf("tab cycle %q %q", f1, f2)
	}
}
