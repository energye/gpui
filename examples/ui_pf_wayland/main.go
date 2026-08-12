// Command ui_pf_wayland is the Wayland real-window full-capability verifier:
// the exact same driver as ui_pf_x11, running on the Wayland backend. Ops
// the protocol forbids (Position/Show/Focus/AlwaysOnTop/…) must return
// ErrUnsupported — a probe that unexpectedly succeeds or fails is a FAIL
// (honesty contract, ENGINE_WINDOW_API.md §2.5.4).
//
//	export WAYLAND_DISPLAY=wayland-0
//	go run ./examples/ui_pf_wayland
//
// Output is a per-capability PASS/FAIL table on stderr (exit != 0 on FAIL).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/ui/platform"
)

func main() {
	win, err := pfkit.OpenReal(platform.Options{
		Width:   900,
		Height:  600,
		Title:   "gpui ui_pf_wayland — Wayland 全能力真窗",
		Visible: boolPtr(true),
	}, platform.DisplayWayland)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open wayland:", err)
		os.Exit(1)
	}
	defer win.Close()

	ctl := win.Controls()
	var rows []pfkit.ResultRow
	if ctl == nil {
		// Backend capability not landed: honest SKIP, not fake PASS.
		rows = append(rows, pfkit.ResultRow{Name: "Controller", OK: false, Detail: "SKIP: backend has no WindowController"})
	} else {
		rows = pfkit.Drive(ctl)
	}

	evs := pfkit.Pump(win, 1200*time.Millisecond)
	rows = append(rows, pfkit.ResultRow{
		Name:   "EventPump",
		OK:     true, // observational: steady-state windows may legitimately idle
		Detail: fmt.Sprintf("%d events (%s)", len(evs), eventKinds(evs)),
	})

	ok := pfkit.Report(rows)
	fmt.Fprintln(os.Stderr, "ui_pf_wayland backend=", win.Kind())

	b, _ := json.Marshal(map[string]any{"backend": win.Kind().String(), "rows": rows, "pass": ok})
	fmt.Fprintln(os.Stdout, string(b))
	if !ok {
		os.Exit(1)
	}
}

func boolPtr(b bool) *bool { return &b }

func eventKinds(evs []platform.Event) string {
	seen := map[platform.EventType]bool{}
	for _, e := range evs {
		seen[e.Type] = true
	}
	s := ""
	for t := range seen {
		s += t.String() + " "
	}
	return s
}
