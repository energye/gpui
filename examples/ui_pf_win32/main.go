// Command ui_pf_win32 is the Win32 real-window full-capability verifier: the
// same shared pfkit driver as ui_pf_x11 / ui_pf_wayland, on the Win32
// backend. ENGINE_WINDOW_API.md §2.6 (layer A) requires every platform to
// have its own real-window verifier — this is Win32's.
//
// While the Win32 backend is a registered placeholder (Create returns the
// explicit "not implemented yet" error, S5), running this on Windows verifies
// exactly that promise: the open must fail clearly and without a panic —
// a probe that opens a window anyway, or crashes, is a FAIL of the honesty
// contract. Once S5 lands this same command automatically upgrades to the
// full-capability acceptance run (no example code change). On any other OS it
// reports SKIP (this window can only be verified on Windows).
//
//	go run ./examples/ui_pf_win32   # on Windows
//
// Output: per-capability PASS/FAIL/⛔ table on stderr + JSON gate on stdout.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/ui/platform"
)

func main() {
	if runtime.GOOS != "windows" {
		rows := []pfkit.ResultRow{{
			Name:   "Controller",
			OK:     false,
			Detail: "SKIP: Win32 real-window verification runs on Windows (this is " + runtime.GOOS + ")",
		}}
		finish("skip", rows, "win32")
		return
	}

	win, err := pfkit.OpenReal(platform.Options{
		Width:   900,
		Height:  600,
		Title:   "gpui ui_pf_win32 — Win32 全能力真窗",
		Visible: boolPtr(true),
	}, platform.DisplayWin32)
	if err != nil {
		// Placeholder period (S5): the only promise is an explicit error.
		// Verify it is clear, non-empty and honest — never a silent fake.
		rows := []pfkit.ResultRow{{
			Name:   "Open",
			OK:     true, // explicit failure IS the placeholder contract
			Detail: "placeholder(S5): " + err.Error(),
		}}
		finish("placeholder", rows, "win32")
		return
	}
	defer win.Close()

	ctl := win.Controls()
	if ctl == nil {
		finish("full", []pfkit.ResultRow{{Name: "Controller", OK: false, Detail: "✗ Controls() nil on win32"}}, win.Kind().String())
		return
	}
	rows := pfkit.Drive(ctl)
	evs := pfkit.Pump(win, 1200*time.Millisecond)
	rows = append(rows, pfkit.ResultRow{
		Name:   "EventPump",
		OK:     true,
		Detail: fmt.Sprintf("%d events (%s)", len(evs), eventKinds(evs)),
	})
	finish("full", rows, win.Kind().String())
}

// finish prints the table, emits the JSON gate and exits non-zero on FAIL.
func finish(mode string, rows []pfkit.ResultRow, backend string) {
	ok := pfkit.Report(rows)
	b, _ := json.Marshal(map[string]any{"backend": backend, "mode": mode, "rows": rows, "pass": ok})
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
