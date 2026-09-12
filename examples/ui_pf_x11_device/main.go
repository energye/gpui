// Command ui_pf_x11_device is the X11 device hot-plug true window.
//
// Unlike ui_pf_x11 (full-capability prober that exits after ~1s), this window
// stays open so a human can verify the real effect:
//
//   - SELFTEST (automatic, no human needed): maps synthetic platform device
//     events through input.FromPlatform and asserts the unified Kind/payload.
//   - MANUAL (human): plug/unplug keyboards, mice, touchscreens, pens.
//     Every DeviceAdded/Removed is logged to stderr (raw platform.Event
//     plus unified input.Event) and the window title mirrors the last
//     change, so the effect is visible without judging pixels.
//
// Usage:
//
//	go run ./examples/ui_pf_x11_device                 # selftest, then manual until close
//	go run ./examples/ui_pf_x11_device -auto-only      # selftest only (gate, JSON on stdout)
//	go run ./examples/ui_pf_x11_device -manual-seconds 30  # manual for 30s, then summary
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
		Title: "gpui ui_pf_x11_device — plug/unplug devices here",
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
		fmt.Fprintln(os.Stderr, "ui_pf_x11_device: selftest done, entering manual phase")
		fmt.Fprintln(os.Stderr, "  plug/unplug keyboards, mice, touchscreens, pens;")
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
	fmt.Fprintln(os.Stderr, "ui_pf_x11_device backend=", win.Kind())
	if !ok {
		os.Exit(1)
	}
}

type manualSummary struct {
	Added   int
	Removed int
	ByClass map[string]int
	Names   []string
	Timed   bool
}

func (s manualSummary) rows() []pfkit.ResultRow {
	detail := fmt.Sprintf("added=%d removed=%d byclass=%v names=%q timed=%v",
		s.Added, s.Removed, s.ByClass, strings.Join(s.Names, ","), s.Timed)
	return []pfkit.ResultRow{{Name: "ManualDevice", OK: true, Detail: detail}}
}

func emitJSON(win *platform.Window, rows []pfkit.ResultRow, pass bool, m manualSummary) {
	b, _ := json.Marshal(map[string]any{
		"backend": win.Kind().String(),
		"rows":    rows,
		"manual": map[string]any{
			"added": m.Added, "removed": m.Removed,
			"byclass": m.ByClass, "names": m.Names, "timed": m.Timed,
		},
		"pass": pass,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// selftest maps synthetic platform device events through the unified input
// layer. It proves the mapping contract without needing hardware; the real
// XI hierarchy wire path is covered by the go test
// (ui/platform/x11_device_linux_test.go) plus manual plug/unplug below.
func selftest() []pfkit.ResultRow {
	var rows []pfkit.ResultRow
	check := func(name string, ok bool, detail string) {
		rows = append(rows, pfkit.ResultRow{Name: name, OK: ok, Detail: detail})
	}

	added := input.FromPlatform(platform.Event{
		Type: platform.EventDeviceAdded, DeviceClass: platform.DeviceKeyboard, DeviceName: "AT keyboard",
	}, input.Modifiers{})
	check("MapDeviceAdded",
		added.Kind == input.KindDeviceAdded && added.Device.Class == input.DeviceKeyboard && added.Device.Name == "AT keyboard",
		fmt.Sprintf("kind=%s device=%+v", added.Kind, added.Device))

	removed := input.FromPlatform(platform.Event{
		Type: platform.EventDeviceRemoved, DeviceClass: platform.DeviceTouch, DeviceName: "touchscreen",
	}, input.Modifiers{})
	check("MapDeviceRemoved",
		removed.Kind == input.KindDeviceRemoved && removed.Device.Class == input.DeviceTouch && removed.Device.Name == "touchscreen",
		fmt.Sprintf("kind=%s device=%+v", removed.Kind, removed.Device))

	pen := input.FromPlatform(platform.Event{
		Type: platform.EventDeviceAdded, DeviceClass: platform.DevicePen, DeviceName: "Wacom Pen",
	}, input.Modifiers{})
	check("MapDevicePen",
		pen.Kind == input.KindDeviceAdded && pen.Device.Class == input.DevicePen,
		fmt.Sprintf("kind=%s device=%+v", pen.Kind, pen.Device))

	unknown := input.FromPlatform(platform.Event{
		Type: platform.EventDeviceAdded, DeviceClass: platform.DeviceClass(99),
	}, input.Modifiers{})
	check("MapDeviceUnknown",
		unknown.Kind == input.KindDeviceAdded && unknown.Device.Class == input.DeviceUnknown,
		fmt.Sprintf("kind=%s device=%+v", unknown.Kind, unknown.Device))
	return rows
}

// manualLoop pumps real window events until close or timeout, logging every
// device event and mirroring the state in the window title.
func manualLoop(win *platform.Window, ctl platform.WindowController, host platform.Host, seconds int) manualSummary {
	s := manualSummary{ByClass: make(map[string]int)}
	deadline := time.Time{}
	if seconds > 0 {
		deadline = time.Now().Add(time.Duration(seconds) * time.Second)
	}
	for {
		if !deadline.IsZero() && time.Now().After(deadline) {
			s.Timed = true
			fmt.Fprintln(os.Stderr, "ui_pf_x11_device: manual timeout, summary above")
			return s
		}
		for _, ev := range host.WaitEvents(200 * time.Millisecond) {
			switch ev.Type {
			case platform.EventCloseRequested, platform.EventClose:
				fmt.Fprintln(os.Stderr, "ui_pf_x11_device: close event, summary above")
				return s
			case platform.EventDeviceAdded, platform.EventDeviceRemoved:
				u := input.FromPlatform(ev, input.Modifiers{})
				fmt.Fprintf(os.Stderr, "device %s raw=%s %+v unified=%s %+v\n",
					ev.Type, ev.DeviceClass, ev, u.Kind, u.Device)
				if ev.Type == platform.EventDeviceAdded {
					s.Added++
				} else {
					s.Removed++
				}
				s.ByClass[ev.DeviceClass.String()]++
				if ev.DeviceName != "" {
					s.Names = append(s.Names, ev.DeviceName)
				}
				ctl.SetTitle(fmt.Sprintf("device %s: %s %q (added=%d removed=%d)",
					ev.Type, ev.DeviceClass, ev.DeviceName, s.Added, s.Removed))
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
		if s.ByClass == nil {
			s.ByClass = make(map[string]int)
		}
	}
}
