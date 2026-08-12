// Package pfkit is the shared real-window driver for platform capability
// verification (examples/ui_pf_x11, examples/ui_pf_wayland and their test
// twins). It drives the full Window / WindowController / Host surface
// through the unified L0 API only — zero platform-specific code — so the
// same driver runs unchanged on X11, Wayland, Win32 and AppKit.
//
// Layout:
//
//	OpenReal(opts, backend)        — force a specific display backend
//	Drive(ctl)                     — walk every controller capability; log all
//	                                  results; tolerate ErrUnsupported (⛔)
//	Report(...)                    — PASS/FAIL summary + structured result rows
//
// The upper-layer interface test lives in pfkit_test.go: it drives a fake
// controller (StubHost + recording ctl) through the exact same Drive()
// sequence, proving the driver is deterministic without any GPU window.
package pfkit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/energye/gpui/ui/platform"
)

// ResultRow is one capability probe: name → outcome + observed value.
type ResultRow struct {
	Name   string
	OK     bool
	Detail string
}

// OpenReal opens a real native window on the requested backend (no GPU
// surface — this is the platform layer, pixels are owned by the renderer).
func OpenReal(opts platform.Options, backend platform.DisplayBackend) (*platform.Window, error) {
	opts.Backend = backend
	return platform.Open(opts)
}

// Drive walks the full controller surface once. Every probe is recorded;
// ErrUnsupported is expected on protocol-impossible ops (Wayland
// Position/Show/Focus/…) and counts as OK with a ⛔ note — the contract from
// ENGINE_WINDOW_API.md §2.5.4 is that non-nil errors are either
// ErrUnsupported or a native failure; native failures are FAIL.
//
// When ctl is nil (backend capability not yet landed), a single SKIP row is
// reported — never a fake PASS.
func Drive(ctl platform.WindowController) []ResultRow {
	if ctl == nil {
		return []ResultRow{{Name: "Controller", OK: false, Detail: "SKIP: backend has no WindowController yet"}}
	}
	var rows []ResultRow
	probe := func(name string, run func() (string, error)) {
		detail, err := run()
		ok := true
		switch {
		case err == nil:
		case errors.Is(err, platform.ErrUnsupported):
			detail = "⛔ unsupported: " + detail
		default:
			ok = false
			detail = "✗ " + err.Error()
		}
		rows = append(rows, ResultRow{Name: name, OK: ok, Detail: detail})
	}

	probe("Title", func() (string, error) {
		ctl.SetTitle("pfkit-title-v1")
		return "got=" + ctl.Title(), nil
	})
	probe("Size", func() (string, error) {
		ctl.SetSize(900, 600)
		w, h := ctl.Size()
		return fmt.Sprintf("set(900x600) got=(%d,%d)", w, h), nil
	})
	probe("MinSize", func() (string, error) {
		ctl.SetMinSize(300, 200)
		return "ok", nil
	})
	probe("MaxSize", func() (string, error) {
		ctl.SetMaxSize(1600, 1200)
		return "ok", nil
	})
	probe("Resizable", func() (string, error) {
		ctl.SetResizable(true)
		return fmt.Sprintf("is=%v", ctl.IsResizable()), nil
	})
	probe("Decorations", func() (string, error) {
		return fmt.Sprintf("dec=%v", ctl.IsDecorated()), ctl.SetDecorations(true)
	})
	probe("Position", func() (string, error) {
		x, y, ok := ctl.Position()
		return fmt.Sprintf("pos=(%d,%d) ok=%v", x, y, ok), ctl.SetPosition(120, 100)
	})
	probe("Minimize", func() (string, error) {
		ctl.Minimize()
		return fmt.Sprintf("is=%v", ctl.IsMinimized()), nil
	})
	probe("Maximize", func() (string, error) {
		ctl.Maximize()
		return fmt.Sprintf("is=%v", ctl.IsMaximized()), nil
	})
	probe("Unmaximize", func() (string, error) {
		ctl.Unmaximize()
		return fmt.Sprintf("is=%v", ctl.IsMaximized()), nil
	})
	probe("Fullscreen", func() (string, error) {
		ctl.SetFullscreen(true)
		on := ctl.IsFullscreen()
		ctl.SetFullscreen(false)
		return fmt.Sprintf("on=%v then off", on), nil
	})
	probe("Show/Hide", func() (string, error) {
		if err := ctl.Show(); err != nil {
			return "show failed", err
		}
		vis := ctl.IsVisible()
		err := ctl.Hide()
		// Restore visibility: later probes (Focus on X11) fail with BadMatch
		// on unmapped windows.
		_ = ctl.Show()
		return fmt.Sprintf("shown=%v then hidden+reshown", vis), err
	})
	probe("Focus", func() (string, error) {
		// XSetInputFocus requires a viewable window; map is async, so let
		// the WM finish remapping after the Show/Hide probe.
		time.Sleep(300 * time.Millisecond)
		return fmt.Sprintf("focused=%v", ctl.IsFocused()), ctl.Focus()
	})
	probe("AlwaysOnTop", func() (string, error) {
		return "ok", ctl.SetAlwaysOnTop(true)
	})
	probe("Cursor", func() (string, error) {
		ctl.SetCursor(platform.CursorText)
		return "text", nil
	})
	probe("RequestMove", func() (string, error) {
		return "ok", ctl.RequestMove()
	})
	probe("RequestResize", func() (string, error) {
		return "edge=right", ctl.RequestResize(platform.WindowEdgeRight)
	})
	probe("IgnoreCursorEvents", func() (string, error) {
		return "ok", ctl.SetIgnoreCursorEvents(true)
	})
	return rows
}

// Report prints the result rows and returns false when any probe failed
// (unsupported rows are counted as OK by design; SKIP rows are not FAIL
// either — they report missing capability, not broken code).
func Report(rows []ResultRow) bool {
	return ReportTo(os.Stderr, rows)
}

// ReportTo is Report with an injectable writer (tests pass io.Discard; gate
// tooling can redirect). Same PASS/SKIP/FAIL counting semantics.
func ReportTo(w io.Writer, rows []ResultRow) bool {
	fail := 0
	skip := 0
	for _, r := range rows {
		mark := "PASS"
		switch {
		case !r.OK && (r.Detail == "" || r.Detail[0] == 'S'): // SKIP rows keep OK=false
			mark = "SKIP"
			skip++
		case !r.OK:
			mark = "FAIL"
			fail++
		}
		fmt.Fprintf(w, "  [%s] %-18s %s\n", mark, r.Name, r.Detail)
	}
	fmt.Fprintf(w, "pfkit: %d ok, %d skip, %d fail (total %d)\n",
		len(rows)-fail-skip, skip, fail, len(rows))
	return fail == 0
}

// Pump drains events for d and returns what arrived. A steady-state real
// window (no input, no state change) legitimately produces zero events
// (Wayland consumes configure during create); strict event assertions live
// in the native tests (x11/wayland_window_linux_test.go), Pump is
// observational.
func Pump(w *platform.Window, d time.Duration) []platform.Event {
	h := w.Host()
	if h == nil {
		return nil
	}
	deadline := time.Now().Add(d)
	var out []platform.Event
	for time.Now().Before(deadline) {
		evs := h.WaitEvents(120 * time.Millisecond)
		out = append(out, evs...)
	}
	return out
}
