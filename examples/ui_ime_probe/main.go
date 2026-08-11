//go:build linux

// Command-free probe: opens a Wayland window through ui/platform and checks
// whether the compositor advertises zwp_text_input_v3 and the IME binding
// succeeds. Used to verify the IME protocol chain without a GPU present.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/ui/platform"
)

func main() {
	backend := platform.DetectDisplayBackend()
	fmt.Fprintf(os.Stderr, "probe: backend=%s\n", backend)
	if backend != platform.DisplayWayland {
		fmt.Fprintf(os.Stderr, "probe: not wayland (got %s), nothing to probe\n", backend)
		return
	}
	win, err := platform.Open(platform.Options{
		Width: 480, Height: 320, Title: "ime-probe", Backend: platform.DisplayWayland,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe: open failed: %v\n", err)
		os.Exit(1)
	}
	defer win.Close()

	ime := win.IME()
	if ime == nil {
		fmt.Fprintf(os.Stderr, "probe: IME capability NOT available (compositor lacks zwp_text_input_v3?)\n")
	} else {
		fmt.Fprintf(os.Stderr, "probe: IME capability AVAILABLE\n")
		ime.EnableIME(platform.Rect{X: 10, Y: 10, W: 200, H: 30})
		fmt.Fprintf(os.Stderr, "probe: EnableIME sent\n")
	}
	fmt.Fprintf(os.Stderr, "probe: kind=%s surface=%x/%x\n", win.Kind(),
		win.Host().NativeSurface().Display, win.Host().NativeSurface().Window)

	// Drain events for a moment; if the compositor sends IME events they'd
	// surface here. Close request would show as EventClose.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		evs := win.Host().WaitEvents(200 * time.Millisecond)
		for _, ev := range evs {
			switch ev.Type {
			case platform.EventIME:
				fmt.Fprintf(os.Stderr, "probe: IME event kind=%d text=%q start=%d end=%d\n",
					ev.IMEKind, ev.IMEText, ev.IMEStart, ev.IMEEnd)
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "probe: close\n")
				return
			}
		}
	}
	fmt.Fprintf(os.Stderr, "probe: done (no protocol error)\n")
}
