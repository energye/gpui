// Upper-layer interface test — C layer of ENGINE_WINDOW_API.md §2.6.
//
// The exact same Drive()/Pump() sequence the real-window verifiers run
// (ui_pf_x11 / ui_pf_wayland / ui_pf_win32 / ui_pf_appkit) is driven here
// against a recording fake controller + StubHost: no GPU, no display, no
// native calls. This proves the driver itself is deterministic — the call
// sequence is fixed, ErrUnsupported is tolerated as ⛔ (never FAIL), real
// errors are FAIL, and Report counting is honest. It does NOT prove any
// platform backend works (that is A/B layer work — green here ≠ green there).
package pfkit

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/ui/platform"
)

// recCtl is a recording fake WindowController: every call is appended to
// calls, and queries reflect the state the previous setters established —
// the same behavioural contract a real backend promises.
type recCtl struct {
	calls []string
	errs  map[string]error // method name → error to return (nil = success)

	title      string
	w, h       int
	minW, minH int
	maxW, maxH int
	resizable  bool
	decorated  bool
	posX, posY int
	posOK      bool
	minimized  bool
	maximized  bool
	fullscreen bool
	visible    bool
	focused    bool
	cursor     platform.Cursor
}

func (c *recCtl) rec(name string) error {
	c.calls = append(c.calls, name)
	return c.errs[name]
}

func (c *recCtl) Title() string { c.rec("Title"); return c.title }
func (c *recCtl) SetTitle(t string) {
	if c.rec("SetTitle") == nil {
		c.title = t
	}
}
func (c *recCtl) Size() (int, int) { c.rec("Size"); return c.w, c.h }
func (c *recCtl) SetSize(w, h int) {
	if c.rec("SetSize") == nil {
		c.w, c.h = w, h
	}
}
func (c *recCtl) SetMinSize(w, h int) {
	if c.rec("SetMinSize") == nil {
		c.minW, c.minH = w, h
	}
}
func (c *recCtl) SetMaxSize(w, h int) {
	if c.rec("SetMaxSize") == nil {
		c.maxW, c.maxH = w, h
	}
}
func (c *recCtl) SetResizable(r bool) {
	if c.rec("SetResizable") == nil {
		c.resizable = r
	}
}
func (c *recCtl) IsResizable() bool                  { c.rec("IsResizable"); return c.resizable }
func (c *recCtl) SetDecorations(d bool) error        { return c.rec("SetDecorations") }
func (c *recCtl) IsDecorated() bool                  { c.rec("IsDecorated"); return c.decorated }
func (c *recCtl) SetIgnoreCursorEvents(b bool) error { return c.rec("SetIgnoreCursorEvents") }
func (c *recCtl) Position() (int, int, bool)         { c.rec("Position"); return c.posX, c.posY, c.posOK }
func (c *recCtl) SetPosition(x, y int) error {
	if err := c.rec("SetPosition"); err != nil {
		return err
	}
	c.posX, c.posY, c.posOK = x, y, true
	return nil
}
func (c *recCtl) Minimize() {
	if c.rec("Minimize") == nil {
		c.minimized = true
	}
}
func (c *recCtl) IsMinimized() bool { c.rec("IsMinimized"); return c.minimized }
func (c *recCtl) Maximize() {
	if c.rec("Maximize") == nil {
		c.maximized = true
	}
}
func (c *recCtl) Unmaximize() {
	if c.rec("Unmaximize") == nil {
		c.maximized = false
	}
}
func (c *recCtl) IsMaximized() bool     { c.rec("IsMaximized"); return c.maximized }
func (c *recCtl) SetFullscreen(fs bool) { c.rec("SetFullscreen") }
func (c *recCtl) IsFullscreen() bool    { c.rec("IsFullscreen"); return c.fullscreen }
func (c *recCtl) Show() error {
	if err := c.rec("Show"); err != nil {
		return err
	}
	c.visible = true
	return nil
}
func (c *recCtl) Hide() error {
	if err := c.rec("Hide"); err != nil {
		return err
	}
	c.visible = false
	return nil
}
func (c *recCtl) IsVisible() bool              { c.rec("IsVisible"); return c.visible }
func (c *recCtl) Focus() error                 { c.rec("Focus"); return nil }
func (c *recCtl) IsFocused() bool              { c.rec("IsFocused"); return c.focused }
func (c *recCtl) SetAlwaysOnTop(on bool) error { return c.rec("SetAlwaysOnTop") }
func (c *recCtl) SetCursor(cur platform.Cursor) {
	if c.rec("SetCursor") == nil {
		c.cursor = cur
	}
}
func (c *recCtl) RequestMove() error                           { return c.rec("RequestMove") }
func (c *recCtl) RequestResize(edge platform.WindowEdge) error { c.rec("RequestResize"); return nil }

func newRecCtl(rowErr map[string]error) *recCtl {
	return &recCtl{
		w: 640, h: 480, // Open() defaults
		minW: 0, minH: 0, // unconstrained
		maxW: 0, maxH: 0,
		decorated: true,
		visible:   true,
		posOK:     true,
		posX:      50, posY: 60,
		errs: rowErr,
	}
}

// TestDriveNilCtl: no controller (backend capability not landed) yields a
// single honest SKIP row — never a fake PASS.
func TestDriveNilCtl(t *testing.T) {
	rows := Drive(nil)
	if len(rows) != 1 {
		t.Fatalf("Drive(nil) rows = %d, want 1", len(rows))
	}
	if rows[0].OK {
		t.Fatal("Drive(nil) must not be PASS")
	}
	if !strings.HasPrefix(rows[0].Detail, "SKIP") {
		t.Fatalf("Drive(nil) detail = %q, want SKIP", rows[0].Detail)
	}
	if !ReportTo(io.Discard, rows) {
		t.Fatal("Report of SKIP-only rows must pass (SKIP is an honest gap, not a FAIL)")
	}
}

// TestDriveDeterministicCallSequence pins the exact probe sequence: if a
// future edit reorders/renames Drive() probes, this test fails and forces an
// intentional review — the sequence is the cross-platform contract every
// real-window verifier depends on.
func TestDriveDeterministicCallSequence(t *testing.T) {
	ctl := newRecCtl(nil)
	rows := Drive(ctl)

	want := []string{
		"SetTitle", "Title",
		"SetSize", "Size",
		"SetMinSize",
		"SetMaxSize",
		"SetResizable", "IsResizable",
		"IsDecorated", "SetDecorations",
		"Position", "SetPosition",
		"Minimize", "IsMinimized",
		"Maximize", "IsMaximized",
		"Unmaximize", "IsMaximized",
		"SetFullscreen", "IsFullscreen", "SetFullscreen",
		"Show", "IsVisible", "Hide", "Show",
		"IsFocused", "Focus",
		"SetAlwaysOnTop",
		"SetCursor",
		"RequestMove",
		"RequestResize",
		"SetIgnoreCursorEvents",
	}
	if len(ctl.calls) != len(want) {
		t.Fatalf("call sequence length = %d, want %d:\n  got:  %v\n  want: %v",
			len(ctl.calls), len(want), ctl.calls, want)
	}
	for i, w := range want {
		if ctl.calls[i] != w {
			t.Fatalf("call[%d] = %q, want %q (full: %v)", i, ctl.calls[i], w, ctl.calls)
		}
	}

	// Deterministic outcomes: every probe PASSES with sane detail strings
	// (query reflects prior set — same contract a real backend keeps).
	// 18 probes (calls = 32); probe count is the report surface.
	wantProbes := []string{
		"Title", "Size", "MinSize", "MaxSize", "Resizable", "Decorations",
		"Position", "Minimize", "Maximize", "Unmaximize", "Fullscreen",
		"Show/Hide", "Focus", "AlwaysOnTop", "Cursor", "RequestMove",
		"RequestResize", "IgnoreCursorEvents",
	}
	if len(rows) != len(wantProbes) {
		t.Fatalf("rows = %d, want %d probes", len(rows), len(wantProbes))
	}
	for i, r := range rows {
		if r.Name != wantProbes[i] {
			t.Fatalf("row[%d].Name = %q, want %q", i, r.Name, wantProbes[i])
		}
		if !r.OK {
			t.Errorf("probe %q failed on a healthy fake: %s", r.Name, r.Detail)
		}
	}
	if !ReportTo(io.Discard, rows) {
		t.Fatal("healthy fake must Report pass")
	}

	// State round-trips visible in the detail strings.
	if ctl.title != "pfkit-title-v1" {
		t.Errorf("title = %q, want pfkit-title-v1", ctl.title)
	}
	if ctl.w != 900 || ctl.h != 600 {
		t.Errorf("size = %d x %d, want 900x600", ctl.w, ctl.h)
	}
	if ctl.minW != 300 || ctl.minH != 200 {
		t.Errorf("min = %d x %d, want 300x200", ctl.minW, ctl.minH)
	}
	if ctl.posX != 120 || ctl.posY != 100 || !ctl.posOK {
		t.Errorf("pos = (%d,%d,%v), want (120,100,true)", ctl.posX, ctl.posY, ctl.posOK)
	}
	if ctl.visible != true {
		t.Errorf("visible = %v after Show/Hide/Show, want true", ctl.visible)
	}
	if ctl.cursor != platform.CursorText {
		t.Errorf("cursor = %v, want text", ctl.cursor)
	}
}

// TestDriveUnsupportedTolerated: operations the platform protocol forbids
// must surface ErrUnsupported and be marked OK (⛔) — never FAIL, never a
// silent ignore. This is the §2.5.4 error contract driving the honesty rule.
func TestDriveUnsupportedTolerated(t *testing.T) {
	un := platform.ErrUnsupported
	ctl := newRecCtl(map[string]error{
		"SetPosition":           un, // Wayland: no client positioning
		"SetAlwaysOnTop":        un, // Wayland: no above state
		"Show":                  un,
		"Hide":                  un,
		"Focus":                 un,
		"SetDecorations":        un,
		"SetIgnoreCursorEvents": un, // X11: XShape not landed
	})
	rows := Drive(ctl)
	if !ReportTo(io.Discard, rows) {
		t.Fatal("ErrUnsupported rows must not fail the report")
	}
	for _, r := range rows {
		if !r.OK {
			t.Errorf("probe %q failed: %s", r.Name, r.Detail)
		}
		if r.Name == "Position" && !strings.Contains(r.Detail, "⛔") {
			t.Errorf("Position detail = %q, want ⛔ note", r.Detail)
		}
		if r.Name == "Show/Hide" && !strings.Contains(r.Detail, "⛔") {
			t.Errorf("Show/Hide detail = %q, want ⛔ note", r.Detail)
		}
	}
	// Calls still happen (probe drove through the errors).
	if len(ctl.calls) == 0 {
		t.Fatal("no calls recorded — probes must run even when errors are returned")
	}
}

// TestDriveNativeErrorFails: a real native failure (non-ErrUnsupported) must
// FAIL the probe and the report.
func TestDriveNativeErrorFails(t *testing.T) {
	boom := errors.New("BadWindow (invalid X Window)") // a plausible native error
	ctl := newRecCtl(map[string]error{"RequestMove": boom})
	rows := Drive(ctl)
	if ReportTo(io.Discard, rows) {
		t.Fatal("native failure must fail the report")
	}
	for _, r := range rows {
		if r.Name == "RequestMove" {
			if r.OK {
				t.Error("RequestMove must be FAIL on native error")
			}
			if !strings.HasPrefix(r.Detail, "✗") {
				t.Errorf("RequestMove detail = %q, want ✗ prefix", r.Detail)
			}
		} else if !r.OK {
			t.Errorf("probe %q unexpectedly failed: %s", r.Name, r.Detail)
		}
	}
}

// TestReportCounts checks skip/fail/pass arithmetic honestly.
func TestReportCounts(t *testing.T) {
	rows := []ResultRow{
		{Name: "a", OK: true, Detail: "ok"},
		{Name: "b", OK: false, Detail: "SKIP: not landed"},
		{Name: "c", OK: false, Detail: "✗ boom"},
	}
	var sb strings.Builder
	if ReportTo(&sb, rows) {
		t.Fatal("report with a FAIL row must return false")
	}
	if !strings.Contains(sb.String(), "1 ok, 1 skip, 1 fail") {
		t.Fatalf("summary = %q, want '1 ok, 1 skip, 1 fail'", sb.String())
	}
	if !strings.Contains(sb.String(), "[SKIP]") || !strings.Contains(sb.String(), "[FAIL]") {
		t.Fatalf("marks missing:\n%s", sb.String())
	}
}

// TestPumpStubHost: Pump() drains the Host event pump. With StubHost the
// events are injected through the same Host interface real backends use —
// proving the pump loop contract (drain until deadline, wake included).
func TestPumpStubHost(t *testing.T) {
	h := platform.NewStubHost(100, 100)
	w := platform.WrapHost(h)
	if w == nil || w.Host() == nil {
		t.Fatal("WrapHost must produce a window with a host")
	}

	h.Push(
		platform.Event{Type: platform.EventResize, Width: 800, Height: 600, Scale: 2},
		platform.Event{Type: platform.EventFocus, Focused: true},
		platform.Event{Type: platform.EventPointer, Pointer: platform.PointerEnter, X: 10, Y: 20},
	)
	evs := Pump(w, 300*time.Millisecond)
	if len(evs) < 3 {
		t.Fatalf("Pump returned %d events, want >= 3 (resize+focus+enter+wake): %+v", len(evs), evs)
	}
	seen := map[platform.EventType]bool{}
	for _, e := range evs {
		seen[e.Type] = true
	}
	if !seen[platform.EventResize] || !seen[platform.EventFocus] || !seen[platform.EventPointer] {
		t.Fatalf("pumped events missing injected types: %v", seen)
	}
	if !seen[platform.EventWake] {
		t.Logf("note: no EventWake (deadline race); not a failure — wake is timing-dependent")
	}
}
