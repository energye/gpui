// Command ui_pf_x11_dnd is the X11 XDND drag-and-drop true window.
//
// Unlike ui_pf_x11 (full-capability prober that exits after ~1s), this window
// stays open so a human can verify the real effect:
//
//   - SELFTEST (automatic, no human needed): maps synthetic platform drag
//     events through input.FromPlatform and asserts the unified Kind/payload.
//   - MANUAL (human): drag files from the file manager into the window.
//     Every DragEnter/Over/Leave/Drop is logged to stderr (raw platform.Event
//     plus unified input.Event) and the window title mirrors the drag state,
//     so the effect is visible without judging pixels.
//
// Usage:
//
//	go run ./examples/ui_pf_x11_dnd                 # selftest, then manual until close
//	go run ./examples/ui_pf_x11_dnd -auto-only      # selftest only (gate, JSON on stdout)
//	go run ./examples/ui_pf_x11_dnd -manual-seconds 30  # manual for 30s, then summary
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "run selftest only and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	flag.Parse()

	win, err := pfkit.OpenReal(platform.Options{
		Width: 900, Height: 600,
		Title: "gpui ui_pf_x11_dnd — drag files here",
		// Standard frame is required for manual testing: frameless windows
		// have no title bar to move/close with.
		Decorations: true,
		Resizable:   true,
	}, platform.DisplayX11)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open x11:", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()
	host := win.Host()
	if ctl == nil || host == nil {
		fmt.Fprintln(os.Stderr, "FAIL: no controls/host on x11")
		os.Exit(1)
	}

	rows := selftest()
	ok := pfkit.Report(rows)
	if !*autoOnly {
		fmt.Fprintln(os.Stderr, "ui_pf_x11_dnd: selftest done, entering manual phase")
		fmt.Fprintln(os.Stderr, "  drag files from the file manager into the window;")
		fmt.Fprintln(os.Stderr, "  close the window (X) to finish, or wait for -manual-seconds.")
	}
	if *autoOnly {
		emitJSON(win, rows, ok, manualSummary{})
		if !ok {
			os.Exit(1)
		}
		return
	}

	summary := manualLoop(win, ctl, host, *manualSeconds)
	rows = append(rows, summary.rows()...)
	ok = pfkit.Report(rows)
	emitJSON(win, rows, ok, summary)
	fmt.Fprintln(os.Stderr, "ui_pf_x11_dnd backend=", win.Kind())
	if !ok {
		os.Exit(1)
	}
}

type manualSummary struct {
	Enter int
	Over  int
	Leave int
	Drop  int
	Files []string
	Timed bool
}

func (s manualSummary) rows() []pfkit.ResultRow {
	detail := fmt.Sprintf("enter=%d over=%d leave=%d drop=%d files=%q timed=%v",
		s.Enter, s.Over, s.Leave, s.Drop, strings.Join(s.Files, ","), s.Timed)
	return []pfkit.ResultRow{{Name: "ManualDrag", OK: true, Detail: detail}}
}

func emitJSON(win *platform.Window, rows []pfkit.ResultRow, pass bool, m manualSummary) {
	b, _ := json.Marshal(map[string]any{
		"backend": win.Kind().String(),
		"rows":    rows,
		"manual": map[string]any{
			"enter": m.Enter, "over": m.Over, "leave": m.Leave,
			"drop": m.Drop, "files": m.Files, "timed": m.Timed,
		},
		"pass": pass,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// selftest maps synthetic platform drag events through the unified input
// layer. It proves the mapping+routing contract without needing a drag
// source; the real XDND wire path is covered by the two-window go test
// (ui/platform/x11_dnd_linux_test.go).
func selftest() []pfkit.ResultRow {
	var rows []pfkit.ResultRow
	check := func(name string, ok bool, detail string) {
		rows = append(rows, pfkit.ResultRow{Name: name, OK: ok, Detail: detail})
	}

	enter := input.FromPlatform(platform.Event{
		Type: platform.EventDragEnter, X: 10, Y: 20,
		MIMETypes: []string{"text/uri-list"},
	}, input.Modifiers{})
	check("MapDragEnter",
		enter.Kind == input.KindDragEnter && enter.Drag.X == 10 && enter.Drag.Y == 20 &&
			len(enter.Drag.MIMETypes) == 1 && enter.Drag.MIMETypes[0] == "text/uri-list",
		fmt.Sprintf("kind=%s drag=%+v", enter.Kind, enter.Drag))

	over := input.FromPlatform(platform.Event{
		Type: platform.EventDragOver, X: 30, Y: 40,
		MIMETypes: []string{"text/uri-list", "text/plain"},
	}, input.Modifiers{})
	check("MapDragOver",
		over.Kind == input.KindDragOver && over.Drag.X == 30 && len(over.Drag.MIMETypes) == 2,
		fmt.Sprintf("kind=%s drag=%+v", over.Kind, over.Drag))

	leave := input.FromPlatform(platform.Event{Type: platform.EventDragLeave}, input.Modifiers{})
	check("MapDragLeave",
		leave.Kind == input.KindDragLeave && len(leave.Drag.MIMETypes) == 0 && len(leave.Drag.Files) == 0,
		fmt.Sprintf("kind=%s", leave.Kind))

	drop := input.FromPlatform(platform.Event{
		Type: platform.EventDrop, X: 50, Y: 60, Files: []string{"/tmp/a"},
	}, input.Modifiers{})
	check("MapDrop",
		drop.Kind == input.KindDrop && drop.Drag.X == 50 && len(drop.Drag.Files) == 1,
		fmt.Sprintf("kind=%s drag=%+v", drop.Kind, drop.Drag))
	return rows
}

// manualLoop pumps real window events until close or timeout, logging every
// drag event and mirroring the state in the window title.
func manualLoop(win *platform.Window, ctl platform.WindowController, host platform.Host, seconds int) manualSummary {
	var s manualSummary
	idleTitle := "gpui ui_pf_x11_dnd — drag files here"
	deadline := time.Time{}
	if seconds > 0 {
		deadline = time.Now().Add(time.Duration(seconds) * time.Second)
	}
	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			s.Timed = true
			fmt.Fprintln(os.Stderr, "ui_pf_x11_dnd: manual timeout, summary above")
			return s
		}
		for _, ev := range host.WaitEvents(200 * time.Millisecond) {
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				fmt.Fprintln(os.Stderr, "ui_pf_x11_dnd: close event, summary above")
				return s
			case platform.EventDragEnter:
				s.Enter++
				u := input.FromPlatform(ev, input.Modifiers{})
				fmt.Fprintf(os.Stderr, "drag-enter raw=%+v unified=%s %+v\n", ev, u.Kind, u.Drag)
				ctl.SetTitle(fmt.Sprintf("drag enter: types=%s", strings.Join(ev.MIMETypes, ",")))
			case platform.EventDragOver:
				s.Over++
				u := input.FromPlatform(ev, input.Modifiers{})
				if s.Over == 1 || s.Over%20 == 0 {
					fmt.Fprintf(os.Stderr, "drag-over raw=(%.0f,%.0f) types=%q unified=%s\n",
						ev.X, ev.Y, ev.MIMETypes, u.Kind)
				}
				ctl.SetTitle(fmt.Sprintf("drag over: (%.0f,%.0f) types=%d", ev.X, ev.Y, len(ev.MIMETypes)))
			case platform.EventDragLeave:
				s.Leave++
				fmt.Fprintln(os.Stderr, "drag-leave")
				ctl.SetTitle(idleTitle)
			case platform.EventDrop:
				s.Drop++
				s.Files = append([]string(nil), ev.Files...)
				u := input.FromPlatform(ev, input.Modifiers{})
				fmt.Fprintf(os.Stderr, "drop raw=(%.0f,%.0f) files=%q unified=%s %+v\n",
					ev.X, ev.Y, ev.Files, u.Kind, u.Drag)
				ctl.SetTitle(fmt.Sprintf("drop: %d files: %s", len(ev.Files), strings.Join(ev.Files, ",")))
			default:
				// Keep the pump honest: surface anything unexpected while
				// manual testing (focus/move/resize), but stay quiet on
				// steady-state idle.
				switch ev.Type {
				case platform.EventFocus, platform.EventMove, platform.EventResize:
					fmt.Fprintf(os.Stderr, "win event: %s %+v\n", ev.Type, ev)
				}
			}
		}
	}
}
