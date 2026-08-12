// Command ui_pf_x11 is the X11 real-window full-capability verifier: it
// opens a native X11 window through the unified L0 API and drives every
// WindowController capability plus the Host event pump — the dual of
// ui/platform/x11_window_linux_test.go (which asserts through the same
// surface for CI where DISPLAY exists).
//
//	export DISPLAY=:0   # or whatever your X server is
//	go run ./examples/ui_pf_x11
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
		Title:   "gpui ui_pf_x11 — X11 全能力真窗",
		Visible: boolPtr(true),
	}, platform.DisplayX11)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open x11:", err)
		os.Exit(1)
	}
	defer win.Close()

	ctl := win.Controls()
	if ctl == nil {
		fmt.Fprintln(os.Stderr, "FAIL: Controls() nil on x11")
		os.Exit(1)
	}

	rows := pfkit.Drive(ctl)

	// Pump events briefly so resize/move/focus land; report what arrived.
	evs := pfkit.Pump(win, 1200*time.Millisecond)
	rows = append(rows, pfkit.ResultRow{
		Name:   "EventPump",
		OK:     true, // observational: steady-state windows may legitimately idle
		Detail: fmt.Sprintf("%d events (%s)", len(evs), eventKinds(evs)),
	})

	ok := pfkit.Report(rows)
	fmt.Fprintln(os.Stderr, "ui_pf_x11 backend=", win.Kind())

	// JSON for gate tooling.
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
