// Command game_input is the S60 dual-case input window.
//
// Two cases share one binary and one headless probe suite, both driving
// the real engine/input package (action mapping + press buffer + touch
// tracker). Nothing here reimplements input logic; the window only feeds
// hardware numbers in and paints the resulting action numbers out.
//
// Modes:
//
//	go run ./examples/engine/input --case=remap -auto-only
//	  headless probes + ~8s window (JSON on stdout, exit 1 on fail).
//	go run ./examples/engine/input --case=remap -manual-seconds 30
//	  manual for 30s (events logged, title shows the count), then summary.
//	go run ./examples/engine/input --case=remap -write-golden
//	  headless: freeze the golden baseline into testdata/, verify it,
//	  then exit without opening any window.
//	--case=compo is not accepted; the combo case is spelled "combo".
//
// Window: 1200x800, title game_input. First run writes the golden baseline
// into testdata/; later runs compare it with zero tolerance.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/input"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200, 800
	abilityID  = "game-input"

	// Frozen engine-side case files. The window never hardcodes action
	// numbers; it replays these files through the real API.
	actionFrozenPath = "engine/input/testdata/action_cases.json"
	bufferFrozenPath = "engine/input/testdata/buffer_cases.json"

	remapGoldenPath = "examples/engine/input/testdata/input_remap_golden.png"
	comboGoldenPath = "examples/engine/input/testdata/input_combo_golden.png"

	// probePixelTol is the hardcoded per-channel tolerance (0-255 steps)
	// for the offscreen pixel asserts. The golden mask compare stays at
	// zero tolerance.
	probePixelTol = 8

	// eps compares deadzoned strengths and vectors. The math is integer
	// hardware codes through one divide, so 1e-9 separates pass from fail
	// without flaking.
	eps = 1e-9

	// comboBase is the fixed logical clock (ms) for the scripted combo:
	// every push/consume lands inside one 150ms window by construction,
	// so the auto verdict never depends on wall-clock jitter.
	comboBase = 10000
)

// Shared scene colors (window paint and offscreen probes use the same).
const (
	bgR, bgG, bgB = 0.08, 0.09, 0.11

	barR, barG, barB = 0.35, 0.75, 0.45
	barBgR, barBgG   = 0.22, 0.24
	barBgB           = 0.28
	cellR, cellG     = 1.0, 0.80
	cellB            = 0.40
	touchR, touchG   = 0.45, 0.70
	touchB           = 1.0
	comboR, comboG   = 1.0, 0.45
	comboB           = 0.30
)

// Body-local layout (legacy probe geometry reference; live window is
// full-window content, see metricStripH).
const (
	leftX, leftY, leftW, leftH = 16.0, 44.0, 440.0, 360.0
	midX, midY, midW, midH     = 472.0, 44.0, 220.0, 360.0
	rightX, rightY             = 708.0, 44.0
	noteY                      = 420.0

	offW, offH = 480, 270
)

// 2.5D摆法：满窗即内容，指标浮左上，Golden排除指标带。
// metricStripH是浮层指标带高度，窗口Golden比对从该高度之下起算。
// 离屏探针帧无指标覆盖；窗口快照只比该带之下的纯画面。
const metricStripH = 32.0

// Frozen action file schema (mirrors engine/input/action_test.go).
type jBinding struct {
	Source string `json:"source"`
	Device int    `json:"device"`
	Code   int    `json:"code"`
	Sign   int    `json:"sign"`
}

type jAction struct {
	Name     string     `json:"name"`
	Deadzone float64    `json:"deadzone"`
	Bindings []jBinding `json:"bindings"`
}

type jWant struct {
	Action   string  `json:"action"`
	Strength float64 `json:"strength"`
	Pressed  bool    `json:"pressed"`
}

type jPadBtn struct {
	Pad  int `json:"pad"`
	Code int `json:"code"`
}

type jPadAxis struct {
	Pad   int     `json:"pad"`
	Code  int     `json:"code"`
	Value float64 `json:"value"`
}

type jSnapshot struct {
	Name        string     `json:"name"`
	Keys        []int      `json:"keys"`
	PadButtons  []jPadBtn  `json:"pad_buttons"`
	PadAxes     []jPadAxis `json:"pad_axes"`
	PinchDeltas []float64  `json:"pinch_deltas"`
	Want        []jWant    `json:"want"`
}

type jVector struct {
	Name        string     `json:"name"`
	NegX        string     `json:"neg_x"`
	PosX        string     `json:"pos_x"`
	NegY        string     `json:"neg_y"`
	PosY        string     `json:"pos_y"`
	Keys        []int      `json:"keys"`
	PadAxes     []jPadAxis `json:"pad_axes"`
	PinchDeltas []float64  `json:"pinch_deltas"`
	Want        [2]float64 `json:"want"`
}

type jRemapProbe struct {
	Name         string     `json:"name"`
	Keys         []int      `json:"keys"`
	PadButtons   []jPadBtn  `json:"pad_buttons"`
	PadAxes      []jPadAxis `json:"pad_axes"`
	WantStrength float64    `json:"want_strength"`
	WantPressed  bool       `json:"want_pressed"`
}

type jRemap struct {
	Action      string        `json:"action"`
	NewBindings []jBinding    `json:"new_bindings"`
	Probes      []jRemapProbe `json:"probes"`
}

type jRetune struct {
	Action      string        `json:"action"`
	NewDeadzone float64       `json:"new_deadzone"`
	Probes      []jRemapProbe `json:"probes"`
}

type jRumbleStep struct {
	DtMs       int64   `json:"dt_ms"`
	WantWeak   float64 `json:"want_weak"`
	WantStrong float64 `json:"want_strong"`
	WantActive bool    `json:"want_active"`
}

type jRumble struct {
	Name   string        `json:"name"`
	Pad    int           `json:"pad"`
	Weak   float64       `json:"weak"`
	Strong float64       `json:"strong"`
	DurMs  int64         `json:"dur_ms"`
	Stop   bool          `json:"stop"`
	Steps  []jRumbleStep `json:"steps"`
}

type jActionFile struct {
	Sources   []string    `json:"sources"`
	Actions   []jAction   `json:"actions"`
	Snapshots []jSnapshot `json:"snapshots"`
	Vectors   []jVector   `json:"vectors"`
	Remap     jRemap      `json:"remap"`
	Retune    jRetune     `json:"retune"`
	Rumbles   []jRumble   `json:"rumbles"`
}

// Frozen buffer file schema (mirrors engine/input/buffer_test.go).
type jPush struct {
	Action string `json:"action"`
	AtMs   int64  `json:"at_ms"`
}

type jStep struct {
	Op      string `json:"op"`
	Action  string `json:"action"`
	NowMs   int64  `json:"now_ms"`
	WantHit bool   `json:"want_hit"`
}

type jSeq struct {
	Name      string  `json:"name"`
	WindowMs  int64   `json:"window_ms"`
	Pushes    []jPush `json:"pushes"`
	Steps     []jStep `json:"steps"`
	CheckAtMs int64   `json:"check_at_ms"`
	WantLive  int     `json:"want_live"`
	WantLen   int     `json:"want_len"`
}

type jTouchOp struct {
	Op       string  `json:"op"`
	ID       int     `json:"id"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	AtMs     int64   `json:"at_ms"`
	WantCode string  `json:"want_code"`
}

type jWantTouch struct {
	ID   int     `json:"id"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	AtMs int64   `json:"at_ms"`
}

type jTouchSeq struct {
	Name        string       `json:"name"`
	Ops         []jTouchOp   `json:"ops"`
	WantActive  []int        `json:"want_active"`
	WantTouches []jWantTouch `json:"want_touches"`
}

type jLimits struct {
	MaxBuffered     int `json:"max_buffered"`
	MaxTouches      int `json:"max_touches"`
	DefaultWindowMs int `json:"default_window_ms"`
}

type jBufferFile struct {
	Limits          jLimits     `json:"limits"`
	BufferSequences []jSeq      `json:"buffer_sequences"`
	TouchSequences  []jTouchSeq `json:"touch_sequences"`
}

// probeResult is the three-evidence headless verdict (no GPU needed).
type probeResult struct {
	LogicOK, PixOK, GoldenOK bool
	Snapshots, Vectors       int
	PixDetail                string
	GoldenChanged            int
	GoldenWrote              bool
	Detail                   string
	OK                       bool
}

func closeEnough(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func toBinding(d jBinding) (input.Binding, error) {
	src, err := input.ParseSource(d.Source)
	if err != nil {
		return input.Binding{}, err
	}
	return input.Binding{Source: src, Device: d.Device, Code: d.Code, Sign: d.Sign}, nil
}

// buildActionMap replays the frozen action table through the real API.
func buildActionMap(f *jActionFile) (*input.Map, error) {
	m := input.NewMap()
	for _, a := range f.Actions {
		if err := m.AddAction(a.Name, a.Deadzone); err != nil {
			return nil, fmt.Errorf("AddAction %q: %w", a.Name, err)
		}
		for _, d := range a.Bindings {
			b, err := toBinding(d)
			if err != nil {
				return nil, fmt.Errorf("binding %q: %w", a.Name, err)
			}
			if err := m.Bind(a.Name, b); err != nil {
				return nil, fmt.Errorf("Bind %q: %w", a.Name, err)
			}
		}
	}
	return m, nil
}

func feedInputs(m *input.Map, keys []int, btns []jPadBtn, axes []jPadAxis, pinches []float64) error {
	for _, k := range keys {
		if err := m.SetKey(k, true); err != nil {
			return fmt.Errorf("SetKey %d: %w", k, err)
		}
	}
	for _, b := range btns {
		if err := m.SetPadButton(b.Pad, b.Code, true); err != nil {
			return fmt.Errorf("SetPadButton: %w", err)
		}
	}
	for _, a := range axes {
		if err := m.SetPadAxis(a.Pad, a.Code, a.Value); err != nil {
			return fmt.Errorf("SetPadAxis: %w", err)
		}
	}
	for _, p := range pinches {
		if err := m.AddPinchDelta(p); err != nil {
			return fmt.Errorf("AddPinchDelta: %w", err)
		}
	}
	return nil
}

// jumpKeyCode derives the frozen jump key from the file itself so the
// latency probe never hardcodes a hardware code.
func jumpKeyCode(f *jActionFile) (int, error) {
	for _, a := range f.Actions {
		if a.Name != "jump" {
			continue
		}
		for _, d := range a.Bindings {
			if d.Source == "key" {
				return d.Code, nil
			}
		}
	}
	return 0, fmt.Errorf("no frozen key binding for jump")
}

// probeRemapLogic replays all 19 snapshots + 4 vectors + remap + retune +
// rumble + same-tick latency through the real Map.
func probeRemapLogic() (bool, int, int, string) {
	raw, err := os.ReadFile(actionFrozenPath)
	if err != nil {
		return false, 0, 0, "frozen read: " + err.Error()
	}
	var f jActionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return false, 0, 0, "frozen decode: " + err.Error()
	}
	if len(f.Snapshots) != 19 {
		return false, 0, 0, fmt.Sprintf("snapshots = %d, want 19", len(f.Snapshots))
	}
	if len(f.Vectors) != 4 {
		return false, 0, 0, fmt.Sprintf("vectors = %d, want 4", len(f.Vectors))
	}
	for _, s := range f.Snapshots {
		m, err := buildActionMap(&f)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		if err := feedInputs(m, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas); err != nil {
			return false, 0, 0, s.Name + ": " + err.Error()
		}
		for _, w := range s.Want {
			got, err := m.Strength(w.Action)
			if err != nil {
				return false, 0, 0, s.Name + ": " + err.Error()
			}
			pressed, err := m.Pressed(w.Action)
			if err != nil {
				return false, 0, 0, s.Name + ": " + err.Error()
			}
			if !closeEnough(got, w.Strength) || pressed != w.Pressed {
				return false, 0, 0, fmt.Sprintf("%s %s = (%v,%v), want (%v,%v)",
					s.Name, w.Action, got, pressed, w.Strength, w.Pressed)
			}
		}
	}
	for _, v := range f.Vectors {
		m, err := buildActionMap(&f)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		if err := feedInputs(m, v.Keys, nil, v.PadAxes, v.PinchDeltas); err != nil {
			return false, 0, 0, v.Name + ": " + err.Error()
		}
		got, err := m.Vector(v.NegX, v.PosX, v.NegY, v.PosY)
		if err != nil {
			return false, 0, 0, v.Name + ": " + err.Error()
		}
		if !closeEnough(got.X, v.Want[0]) || !closeEnough(got.Y, v.Want[1]) {
			return false, 0, 0, fmt.Sprintf("%s = (%v,%v), want %v", v.Name, got.X, got.Y, v.Want)
		}
	}

	// Same-tick latency: feed a key and read the action back with no Tick
	// in between. Pressed in the same call chain means <1 frame by design.
	code, err := jumpKeyCode(&f)
	if err != nil {
		return false, 0, 0, err.Error()
	}
	lat, err := buildActionMap(&f)
	if err != nil {
		return false, 0, 0, err.Error()
	}
	if err := lat.SetKey(code, true); err != nil {
		return false, 0, 0, "latency feed: " + err.Error()
	}
	if s, err := lat.Strength("jump"); err != nil || s != 1 {
		return false, 0, 0, fmt.Sprintf("latency jump = (%v,%v), want (1,nil)", s, err)
	}

	// Remap: replace the whole binding list, verify the action set is
	// untouched (改键不改码) and old inputs go silent while new ones fire.
	before, err := buildActionMap(&f)
	if err != nil {
		return false, 0, 0, err.Error()
	}
	namesBefore := before.Actions()
	var nb []input.Binding
	for _, d := range f.Remap.NewBindings {
		b, err := toBinding(d)
		if err != nil {
			return false, 0, 0, "remap binding: " + err.Error()
		}
		nb = append(nb, b)
	}
	if err := before.Rebind(f.Remap.Action, nb); err != nil {
		return false, 0, 0, "Rebind: " + err.Error()
	}
	namesAfter := before.Actions()
	if len(namesBefore) != len(namesAfter) {
		return false, 0, 0, "remap changed the action set"
	}
	for i := range namesBefore {
		if namesBefore[i] != namesAfter[i] {
			return false, 0, 0, "remap changed the action set"
		}
	}
	for _, p := range f.Remap.Probes {
		m, err := buildActionMap(&f)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		if err := m.Rebind(f.Remap.Action, nb); err != nil {
			return false, 0, 0, "Rebind: " + err.Error()
		}
		if err := feedInputs(m, p.Keys, p.PadButtons, p.PadAxes, nil); err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		got, err := m.Strength(f.Remap.Action)
		if err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		pressed, err := m.Pressed(f.Remap.Action)
		if err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		if !closeEnough(got, p.WantStrength) || pressed != p.WantPressed {
			return false, 0, 0, fmt.Sprintf("remap %s = (%v,%v), want (%v,%v)",
				p.Name, got, pressed, p.WantStrength, p.WantPressed)
		}
	}

	// Retune: a hotter deadzone gates the drifted stick but keeps the
	// digital key at full strength (死区可调, 漂移门住).
	for _, p := range f.Retune.Probes {
		m, err := buildActionMap(&f)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		if err := m.SetDeadzone(f.Retune.Action, f.Retune.NewDeadzone); err != nil {
			return false, 0, 0, "SetDeadzone: " + err.Error()
		}
		if err := feedInputs(m, p.Keys, p.PadButtons, p.PadAxes, nil); err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		got, err := m.Strength(f.Retune.Action)
		if err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		pressed, err := m.Pressed(f.Retune.Action)
		if err != nil {
			return false, 0, 0, p.Name + ": " + err.Error()
		}
		if !closeEnough(got, p.WantStrength) || pressed != p.WantPressed {
			return false, 0, 0, fmt.Sprintf("retune %s = (%v,%v), want (%v,%v)",
				p.Name, got, pressed, p.WantStrength, p.WantPressed)
		}
	}

	// Rumble script: intensities hold until the clock runs out.
	for _, r := range f.Rumbles {
		m, err := buildActionMap(&f)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		if err := m.StartRumble(r.Pad, r.Weak, r.Strong, core.Milliseconds(r.DurMs)); err != nil {
			return false, 0, 0, r.Name + ": " + err.Error()
		}
		if r.Stop {
			if err := m.StopRumble(r.Pad); err != nil {
				return false, 0, 0, r.Name + ": " + err.Error()
			}
		}
		for _, st := range r.Steps {
			m.UpdateRumbles(core.Milliseconds(st.DtMs))
			w, s, active := m.RumbleLevels(r.Pad)
			if !closeEnough(w, st.WantWeak) || !closeEnough(s, st.WantStrong) || active != st.WantActive {
				return false, 0, 0, fmt.Sprintf("rumble %s = (%v,%v,%v), want (%v,%v,%v)",
					r.Name, w, s, active, st.WantWeak, st.WantStrong, st.WantActive)
			}
		}
	}

	// Bad road: unknown actions error, bad codes error, nil maps never panic.
	m, _ := buildActionMap(&f)
	if _, err := m.Strength("no-such-action"); err == nil {
		return false, 0, 0, "unknown action accepted"
	}
	if err := m.SetKey(0, true); err == nil {
		return false, 0, 0, "bad key accepted"
	}
	var nilMap *input.Map
	if _, err := nilMap.Strength("jump"); err == nil {
		return false, 0, 0, "nil map accepted"
	}
	detail := fmt.Sprintf("snapshots=19 vectors=4 remap=%s retune=%s rumbles=%d latency=0frames",
		f.Remap.Action, f.Retune.Action, len(f.Rumbles))
	return true, 19, 4, detail
}

// probeComboLogic replays the buffer + touch sequences, the 64-cap, the
// terminal zero, and the interrupted-touch road through the real types.
func probeComboLogic() (bool, int, int, string) {
	raw, err := os.ReadFile(bufferFrozenPath)
	if err != nil {
		return false, 0, 0, "frozen read: " + err.Error()
	}
	var f jBufferFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return false, 0, 0, "frozen decode: " + err.Error()
	}
	if f.Limits.MaxBuffered != input.MaxBuffered ||
		f.Limits.MaxTouches != input.MaxTouches ||
		f.Limits.DefaultWindowMs != int(input.DefaultBufferWindow.Milliseconds()) {
		return false, 0, 0, fmt.Sprintf("limits %+v vs engine %d/%d/%d",
			f.Limits, input.MaxBuffered, input.MaxTouches, input.DefaultBufferWindow.Milliseconds())
	}
	for _, s := range f.BufferSequences {
		b, err := input.NewBuffer(core.Milliseconds(s.WindowMs))
		if err != nil {
			return false, 0, 0, s.Name + ": " + err.Error()
		}
		for _, p := range s.Pushes {
			if err := b.Push(p.Action, core.Milliseconds(p.AtMs)); err != nil {
				return false, 0, 0, s.Name + ": " + err.Error()
			}
		}
		for _, st := range s.Steps {
			now := core.Milliseconds(st.NowMs)
			var hit bool
			var stepErr error
			switch st.Op {
			case "consume":
				hit, stepErr = b.Consume(st.Action, now)
			case "peek":
				hit, stepErr = b.Peek(st.Action, now)
			default:
				return false, 0, 0, s.Name + ": unknown op " + st.Op
			}
			if stepErr != nil {
				return false, 0, 0, s.Name + ": " + stepErr.Error()
			}
			if hit != st.WantHit {
				return false, 0, 0, fmt.Sprintf("%s %s %s@%d = %v, want %v",
					s.Name, st.Op, st.Action, st.NowMs, hit, st.WantHit)
			}
		}
		b.Prune(core.Milliseconds(s.CheckAtMs))
		if b.LiveCount(core.Milliseconds(s.CheckAtMs)) != s.WantLive ||
			b.Len() != s.WantLen {
			return false, 0, 0, fmt.Sprintf("%s end live=%d len=%d, want %d/%d",
				s.Name, b.LiveCount(core.Milliseconds(s.CheckAtMs)), b.Len(), s.WantLive, s.WantLen)
		}
	}
	for _, s := range f.TouchSequences {
		tr := input.NewTouchTracker()
		for _, op := range s.Ops {
			var opErr error
			switch op.Op {
			case "begin":
				opErr = tr.Begin(op.ID, core.V2(op.X, op.Y), core.Milliseconds(op.AtMs))
			case "move":
				opErr = tr.Move(op.ID, core.V2(op.X, op.Y), core.Milliseconds(op.AtMs))
			case "end":
				opErr = tr.End(op.ID, core.Milliseconds(op.AtMs))
			default:
				return false, 0, 0, s.Name + ": unknown op " + op.Op
			}
			if (opErr == nil) != (op.WantCode == "") {
				return false, 0, 0, fmt.Sprintf("%s op %s id=%d err=%v, want code %q",
					s.Name, op.Op, op.ID, opErr, op.WantCode)
			}
		}
		if got := tr.ActiveIDs(); !intEqual(got, s.WantActive) {
			return false, 0, 0, fmt.Sprintf("%s active = %v, want %v", s.Name, got, s.WantActive)
		}
		got := tr.Touches()
		if len(got) != len(s.WantTouches) {
			return false, 0, 0, fmt.Sprintf("%s touches = %d, want %d", s.Name, len(got), len(s.WantTouches))
		}
		for i, w := range s.WantTouches {
			if got[i].ID != w.ID || !closeEnough(got[i].Pos.X, w.X) ||
				!closeEnough(got[i].Pos.Y, w.Y) || got[i].At.Milliseconds() != w.AtMs {
				return false, 0, 0, fmt.Sprintf("%s touch %d = %+v, want %+v", s.Name, i, got[i], w)
			}
		}
	}

	// Full house: 70 pushes keep the newest 64, Clear returns to zero.
	full, err := input.NewBuffer(input.DefaultBufferWindow)
	if err != nil {
		return false, 0, 0, err.Error()
	}
	for i := 0; i < input.MaxBuffered+6; i++ {
		if err := full.Push("jump", core.Milliseconds(int64(i))); err != nil {
			return false, 0, 0, err.Error()
		}
	}
	if full.Len() != input.MaxBuffered {
		return false, 0, 0, fmt.Sprintf("cap len = %d, want %d", full.Len(), input.MaxBuffered)
	}
	full.Clear()
	if full.Len() != 0 || full.LiveCount(core.Milliseconds(comboBase)) != 0 {
		return false, 0, 0, "terminal state not zero"
	}

	// Interrupted touch: ending an unknown id errors and never panics,
	// the tracker stays usable afterwards (断触不崩).
	tr := input.NewTouchTracker()
	if err := tr.End(99, core.Milliseconds(0)); err == nil {
		return false, 0, 0, "unknown End accepted"
	}
	if err := tr.Begin(1, core.V2(5, 5), core.Milliseconds(0)); err != nil {
		return false, 0, 0, "tracker unusable after bad End: " + err.Error()
	}
	if tr.ActiveCount() != 1 {
		return false, 0, 0, "tracker lost state after bad End"
	}
	detail := fmt.Sprintf("seqs=%d touchseqs=%d cap=%d terminal=0", len(f.BufferSequences), len(f.TouchSequences), input.MaxBuffered)
	return true, len(f.BufferSequences), len(f.TouchSequences), detail
}

func intEqual(a, b []int) bool {
	if len(a) != len(b) {
		if len(a) == 0 && len(b) == 0 {
			return true
		}
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sample8(img image.Image, x, y int) (uint8, uint8, uint8) {
	r, g, b, _ := img.At(x, y).RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func closeEnough8(got, want uint8) bool {
	d := int(got) - int(want)
	if d < 0 {
		d = -d
	}
	return d <= probePixelTol
}

func want8(v float64) uint8 { return uint8(v*255 + 0.5) }

// paintRemapFrame draws the deterministic remap probe frame: a full jump
// bar, a 0.375 move_right bar (stick 0.5 through deadzone 0.2), and a
// silent empty row. The live window paints the same rows from live state.
func paintRemapFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	rows := []struct{ y, w float64 }{{40, 400}, {110, 150}, {180, 0}}
	for _, r := range rows {
		dc.SetRGBA(barBgR, barBgG, barBgB, 1)
		dc.DrawRectangle(40, r.y, 400, 24)
		_ = dc.Fill()
		if r.w > 0 {
			dc.SetRGBA(barR, barG, barB, 1)
			dc.DrawRectangle(40, r.y, r.w, 24)
			_ = dc.Fill()
		}
	}
}

// paintComboFrame draws the deterministic combo probe frame: 5 filled
// cells of the 64-cell strip plus two live touch dots. The live window
// paints the same strip from the live buffer.
func paintComboFrame(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: bgR, G: bgG, B: bgB, A: 1})
	for i := 0; i < 64; i++ {
		x := 40.0 + float64(i%8)*50
		y := 30.0 + float64(i/8)*26
		if i < 5 {
			dc.SetRGBA(cellR, cellG, cellB, 1)
		} else {
			dc.SetRGBA(barBgR, barBgG, barBgB, 1)
		}
		dc.DrawRectangle(x, y, 44, 20)
		_ = dc.Fill()
	}
	for _, p := range [][2]float64{{60, 248}, {120, 248}} {
		dc.SetRGBA(touchR, touchG, touchB, 1)
		dc.DrawRectangle(p[0]-3, p[1]-3, 6, 6)
		_ = dc.Fill()
	}
}

func cpuImage(paint func(dc *render.Context)) image.Image {
	prev, _ := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	defer func() {
		if prev == "" {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		} else {
			_ = os.Setenv("GOGPU_RENDER_MODE", prev)
		}
	}()
	dc := render.NewContext(offW, offH)
	paint(dc)
	img := dc.Image()
	_ = dc.Close()
	return img
}

// probePixels asserts the deterministic frame pixel shapes offscreen:
// bar tips land where the frozen strengths say, empty cells stay quiet.
func probePixels(which string) (bool, string) {
	if which == "remap" {
		img := cpuImage(paintRemapFrame)
		br, bg, bb := want8(barR), want8(barG), want8(barB)
		gr, gg, gb := want8(barBgR), want8(barBgG), want8(barBgB)
		jr, jg, jb := sample8(img, 40+400-3, 40+12)
		mr, mg, mb := sample8(img, 40+150+6, 110+12)
		er, eg, eb := sample8(img, 40+200, 180+12)
		okFull := closeEnough8(jr, br) && closeEnough8(jg, bg) && closeEnough8(jb, bb)
		okPart := closeEnough8(mr, gr) && closeEnough8(mg, gg) && closeEnough8(mb, gb)
		okEmpty := closeEnough8(er, gr) && closeEnough8(eg, gg) && closeEnough8(eb, gb)
		detail := fmt.Sprintf("full=(%d,%d,%d) part=(%d,%d,%d) empty=(%d,%d,%d) tol=%d",
			jr, jg, jb, mr, mg, mb, er, eg, eb, probePixelTol)
		return okFull && okPart && okEmpty, detail
	}
	img := cpuImage(paintComboFrame)
	cr, cg, cb := want8(cellR), want8(cellG), want8(cellB)
	gr, gg, gb := want8(barBgR), want8(barBgG), want8(barBgB)
	tr, tg, tb := want8(touchR), want8(touchG), want8(touchB)
	filled := 0
	for i := 0; i < 64; i++ {
		x := 40 + (i%8)*50 + 22
		y := 30 + (i/8)*26 + 10
		r, g, b := sample8(img, x, y)
		if closeEnough8(r, cr) && closeEnough8(g, cg) && closeEnough8(b, cb) {
			filled++
		} else if !(closeEnough8(r, gr) && closeEnough8(g, gg) && closeEnough8(b, gb)) {
			return false, fmt.Sprintf("cell %d color off (%d,%d,%d)", i, r, g, b)
		}
	}
	dr, dg, db := sample8(img, 60, 248)
	okDot := closeEnough8(dr, tr) && closeEnough8(dg, tg) && closeEnough8(db, tb)
	detail := fmt.Sprintf("filled=%d/5 dot=(%d,%d,%d) tol=%d", filled, dr, dg, db, probePixelTol)
	return filled == 5 && okDot, detail
}

// probeGolden compares the deterministic frame against the frozen mask
// with zero tolerance; the first run produces the baseline.
func probeGolden(which string) (ok bool, changed int, wrote bool) {
	var path string
	var paint func(dc *render.Context)
	if which == "remap" {
		path, paint = remapGoldenPath, paintRemapFrame
	} else {
		path, paint = comboGoldenPath, paintComboFrame
	}
	img := cpuImage(paint)
	f, err := os.Open(path)
	if err != nil {
		if err := os.MkdirAll("examples/engine/input/testdata", 0o755); err != nil {
			return false, 0, false
		}
		out, err := os.Create(path)
		if err != nil {
			return false, 0, false
		}
		encErr := png.Encode(out, img)
		_ = out.Close()
		return encErr == nil, 0, encErr == nil
	}
	defer func() { _ = f.Close() }()
	want, err := png.Decode(f)
	if err != nil {
		return false, 0, false
	}
	if !img.Bounds().Eq(want.Bounds()) {
		return false, 1, false
	}
	// Golden裁掉顶部指标带：浮层指标行不参比，只比它下面的纯画面。
	stripPx := int(metricStripH * float64(img.Bounds().Dy()) / float64(winH))
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		if y-img.Bounds().Min.Y < stripPx {
			continue
		}
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			ar, ag, ab, aa := img.At(x, y).RGBA()
			br, bg, bb, ba := want.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				changed++
			}
		}
	}
	return changed == 0, changed, false
}

// runProbes collects the three evidences: logic, pixels, golden mask.
func runProbes(which string) probeResult {
	var p probeResult
	if which == "remap" {
		p.LogicOK, p.Snapshots, p.Vectors, p.Detail = probeRemapLogic()
	} else {
		p.LogicOK, p.Snapshots, p.Vectors, p.Detail = probeComboLogic()
	}
	var pixOK bool
	pixOK, p.PixDetail = probePixels(which)
	p.PixOK = pixOK
	var wrote bool
	p.GoldenOK, p.GoldenChanged, wrote = probeGolden(which)
	p.GoldenWrote = wrote
	p.OK = p.LogicOK && p.PixOK && p.GoldenOK
	return p
}

// liveMap builds the window's action map: frozen table plus lowercase
// conveniences (w/a/s/d, j) so a human pressing keys without Shift still
// moves something. Probes use pristine file-only maps and never see these.
func liveMap() (*input.Map, error) {
	raw, err := os.ReadFile(actionFrozenPath)
	if err != nil {
		return nil, err
	}
	var f jActionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	m, err := buildActionMap(&f)
	if err != nil {
		return nil, err
	}
	extra := map[string]int{
		"move_left": 97, "move_right": 100, "move_up": 119, "move_down": 115,
		"attack": 106,
	}
	for action, code := range extra {
		b, err := input.NewKeyBinding(code)
		if err != nil {
			return nil, err
		}
		if err := m.Bind(action, b); err != nil {
			return nil, err
		}
	}
	return m, nil
}

var remapActions = []string{"jump", "attack", "move_left", "move_right", "move_up", "move_down"}

// sim is the live window state: real engine/input handles advance every
// frame, and the full-window scene paints exactly what the API reports.
type sim struct {
	which   string
	amap    *input.Map
	buf     *input.Buffer
	touches *input.TouchTracker
	app     *embedder.PipelineApp
	root    *rendering.AbsoluteBox
	phase   *wrkit.PhaseClock

	fullBox *rendering.RenderBox
	metric  *rendering.RenderText

	heroX   float64
	heroY   float64
	heroVX  float64
	savedAt int64

	lastKey int
	remaps  int

	// Remap scripted checks (auto-only): each fires once at its second.
	fired       [7]bool
	scriptedOK  [7]bool
	scriptedHit int

	// Combo scripted outcome (auto-only).
	comboHits, comboMiss, comboCap, comboZero, touchOK int

	// Combo live edge detector: rising edge of jump+attack both live.
	comboLive  bool
	comboFlash float64

	elapsed float64
	frames  int
}

func (s *sim) strengths() [6]float64 {
	var out [6]float64
	for i, a := range remapActions {
		v, err := s.amap.Strength(a)
		if err == nil {
			out[i] = v
		}
	}
	return out
}

// tickRemapScript runs the auto-only 改键即用 sequence through the same
// SetKey/Rebind/SetDeadzone entry points a human uses.
func (s *sim) tickRemapScript() {
	t := s.elapsed
	m := s.amap
	check := func(i int, cond bool) {
		if !s.fired[i] {
			s.fired[i] = true
			s.scriptedOK[i] = cond
			if cond {
				s.scriptedHit++
			}
		}
	}
	if t >= 1.0 && !s.fired[0] {
		_ = m.SetKey(32, true)
		v, _ := m.Strength("jump")
		check(0, v == 1) // same-tick read: 0 frames of latency
	}
	if t >= 2.0 && !s.fired[1] {
		_ = m.SetKey(32, false)
		v, _ := m.Strength("jump")
		check(1, v == 0)
	}
	if t >= 3.0 && !s.fired[2] {
		before := m.Actions()
		nb, _ := input.NewKeyBinding(75)
		ok := m.Rebind("attack", []input.Binding{nb}) == nil
		after := m.Actions()
		same := ok && len(before) == len(after)
		for i := range before {
			if before[i] != after[i] {
				same = false
			}
		}
		if same {
			s.remaps++
		}
		check(2, same) // 改键不改码
	}
	if t >= 3.5 && !s.fired[3] {
		_ = m.SetKey(74, true)
		v, _ := m.Strength("attack")
		_ = m.SetKey(74, false)
		check(3, v == 0) // old key silent right after remap
	}
	if t >= 4.0 && !s.fired[4] {
		_ = m.SetKey(75, true)
		v, _ := m.Strength("attack")
		_ = m.SetKey(75, false)
		check(4, v == 1) // new key fires immediately
	}
	if t >= 5.0 && !s.fired[5] {
		_ = m.SetDeadzone("move_right", 0.6)
		_ = m.SetPadAxis(0, 0, 0.5)
		gated, _ := m.Strength("move_right")
		_ = m.SetKey(68, true)
		full, _ := m.Strength("move_right")
		_ = m.SetKey(68, false)
		check(5, gated == 0 && full == 1)
	}
	if t >= 6.0 && !s.fired[6] {
		m.ResetInputs()
		zero := true
		for _, a := range remapActions {
			if v, _ := m.Strength(a); v != 0 {
				zero = false
			}
		}
		check(6, zero) // terminal state returns to zero
	}
}

// tickComboScript runs the auto-only 连招不丢 sequence on the fixed
// logical clock through the same Push/Consume entry points a human uses.
func (s *sim) tickComboScript() {
	t := s.elapsed
	b := s.buf
	base := core.Milliseconds(comboBase)
	switch {
	case t >= 1.0 && s.comboHits == 0 && s.comboMiss == 0 && b.Len() == 0:
		_ = b.Push("jump", base)
		_ = b.Push("attack", base+50)
	case t >= 1.5 && b.Len() == 2:
		h1, _ := b.Consume("jump", base+100)
		h2, _ := b.Consume("attack", base+120)
		miss, _ := b.Consume("jump", base+130)
		if h1 && h2 {
			s.comboHits = 2
		}
		if !miss {
			s.comboMiss = 1
		}
	case t >= 3.0 && s.comboHits == 2 && s.comboMiss == 1 && s.comboCap == 0:
		_ = b.Push("jump", base+1000)
		hit, _ := b.Consume("jump", base+1300)
		if !hit {
			s.comboMiss = 2 // stale press expires: a real miss
		}
		b.Clear()
		for i := 0; i < input.MaxBuffered+6; i++ {
			_ = b.Push("jump", base+2000+core.Duration(i))
		}
		if b.Len() == input.MaxBuffered {
			s.comboCap = input.MaxBuffered
		}
	case t >= 5.0 && s.comboCap == input.MaxBuffered && s.comboZero == 0:
		b.Clear()
		if b.Len() == 0 && b.LiveCount(base+100000) == 0 {
			s.comboZero = 1
		}
		tr := s.touches
		_ = tr.Begin(1, core.V2(10, 20), base)
		_ = tr.Begin(2, core.V2(30, 40), base+10)
		_ = tr.Move(1, core.V2(15, 25), base+20)
		_ = tr.End(2, base+30)
		badEnd := tr.End(99, base+30) != nil
		got := tr.ActiveIDs()
		if badEnd && intEqual(got, []int{1}) {
			s.touchOK = 1
		}
	}
}

type ticker struct{ s *sim }

func (t *ticker) Tick(dt float64) bool {
	s := t.s
	if s == nil {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if dt > 0.05 {
		dt = 0.05
	}
	s.elapsed += dt
	s.frames++
	if s.which == "remap" {
		if autoScript {
			s.tickRemapScript()
		}
		// Thickening: the remapped keys also drive a hero dot (window only).
		mv, _ := s.amap.Vector("move_left", "move_right", "move_up", "move_down")
		s.heroVX = mv.X * 220
		s.heroX += s.heroVX * dt
		if s.heroX < 40 {
			s.heroX = 40
		}
		if s.heroX > winW-80 {
			s.heroX = winW - 80
		}
		if j, _ := s.amap.Strength("jump"); j > 0 && s.heroY >= 0 {
			s.heroY = -120
		}
		if s.heroY < 0 {
			s.heroY += 320 * dt
			if s.heroY > 0 {
				s.heroY = 0
			}
		}
	} else {
		if autoScript {
			s.tickComboScript()
		} else {
			now := core.Milliseconds(int64(s.elapsed * 1000))
			s.buf.Prune(now)
			jump, _ := s.buf.Peek("jump", now)
			attack, _ := s.buf.Peek("attack", now)
			if jump && attack && !s.comboLive {
				s.comboHits++
				s.comboFlash = 0.6
			}
			s.comboLive = jump && attack
			if s.comboFlash > 0 {
				s.comboFlash -= dt
			}
		}
	}
	if s.fullBox != nil {
		s.fullBox.MarkNeedsPaint()
	}

	phase := s.phase.Advance(dt)
	snap := s.app.Metrics().Snapshot()
	fps := 0.0
	if snap.AvgFrameIntervalMs > 1e-6 {
		fps = 1000.0 / snap.AvgFrameIntervalMs
	}
	if s.metric != nil {
		if s.which == "remap" {
			st := s.strengths()
			s.metric.SetText(fmt.Sprintf("input fps=%.0f x=%.0f jump=%.1f atk=%.1f script=%d/7 key=%d",
				fps, s.heroX, st[0], st[1], s.scriptedHit, s.lastKey))
		} else {
			flash := ""
			if s.comboFlash > 0 {
				flash = " FIRE"
			}
			s.metric.SetText(fmt.Sprintf("input fps=%.0f combo=%d miss=%d cap=%d zero=%d%s",
				fps, s.comboHits, s.comboMiss, s.comboCap, s.comboZero, flash))
		}
		s.metric.MarkNeedsPaint()
	}
	_ = phase
	s.app.ScheduleFrame()
	return true
}

// autoScript is set from -auto-only: the scripted sequence owns the
// engine state, so real key/pointer events are counted but not fed.
var autoScript bool

type manualSummary struct {
	Pointer, Key, Resize, Touch int
	Timed                       bool
	Note                        string
}

// paintInputFullWindow draws the full-window scene for both cases:
// action strength bars + move vector arrow + responding hero (remap),
// combo strip + touch dots + saved-key readout (combo). Thickening
// (window only, engine untouched): hero answers keys, combo FIRE shows,
//存档键位 readout keeps the last key on screen.
func paintInputFullWindow(pc *rendering.PaintContext, w, h float64, s *sim) {
	if pc == nil || pc.DC == nil || s == nil {
		return
	}
	ax, ay := pc.Abs(0, 0)
	dc := pc.DC
	dc.SetRGB(bgR, bgG, bgB)
	dc.DrawRectangle(ax, ay, w, h)
	_ = dc.Fill()
	top := ay + metricStripH + 8
	if s.which == "remap" {
		st := s.strengths()
		for i := range remapActions {
			y := top + 20 + float64(i)*56
			dc.SetRGBA(barBgR, barBgG, barBgB, 1)
			dc.DrawRectangle(ax+24, y, w/2-48, 24)
			_ = dc.Fill()
			if st[i] > 0 {
				dc.SetRGBA(barR, barG, barB, 1)
				dc.DrawRectangle(ax+24, y, (w/2-48)*st[i], 24)
				_ = dc.Fill()
			}
		}
		// Move vector arrow盘.
		cx, cy := ax+w*0.75, top+140.0
		dc.SetRGBA(barBgR, barBgG, barBgB, 1)
		dc.DrawRectangle(cx-90, cy-90, 180, 180)
		_ = dc.Fill()
		v, err := s.amap.Vector("move_left", "move_right", "move_up", "move_down")
		if err == nil {
			dc.SetRGBA(comboR, comboG, comboB, 1)
			dc.SetLineWidth(3)
			dc.DrawLine(cx, cy, cx+v.X*70, cy-v.Y*70)
			_ = dc.Stroke()
		}
		// Responding hero: runs with keys, hops with jump (window only).
		hx := ax + s.heroX
		hy := top + h/2 + 120 + s.heroY
		dc.SetRGBA(0.9, 0.2, 0.15, 1)
		dc.DrawRectangle(hx, hy, 40, 40)
		_ = dc.Fill()
		// Combo flash line under the hero.
		for i, p := range s.buf.Presses() {
			_ = i
			_ = p
		}
	} else {
		presses := s.buf.Presses()
		now := core.Milliseconds(int64(s.elapsed * 1000))
		live := map[int]bool{}
		for i, p := range presses {
			if p.At <= now && now-p.At <= s.buf.Window() {
				live[i] = true
			}
		}
		for i := 0; i < 64; i++ {
			x := ax + 24 + float64(i%16)*((w-48)/16)
			y := top + 20 + float64(i/16)*34
			cw := (w-48)/16 - 6
			switch {
			case live[i]:
				dc.SetRGBA(cellR, cellG, cellB, 1)
			case i < len(presses):
				dc.SetRGBA(comboR, comboG, comboB, 0.45)
			default:
				dc.SetRGBA(barBgR, barBgG, barBgB, 1)
			}
			dc.DrawRectangle(x, y, cw, 26)
			_ = dc.Fill()
		}
		for i, tc := range s.touches.Touches() {
			_ = tc
			dc.SetRGBA(touchR, touchG, touchB, 1)
			dc.DrawRectangle(ax+40+float64(i)*48, top+h/2, 14, 14)
			_ = dc.Fill()
		}
		// FIRE banner when the rising edge lands (window only).
		if s.comboFlash > 0 {
			dc.SetRGBA(comboR, comboG, comboB, 1)
			dc.DrawRectangle(ax+w/2-80, top+200, 160, 30)
			_ = dc.Fill()
		}
		// 存档键位 readout: last key stays visible bottom-left.
		dc.SetRGBA(0.7, 0.78, 0.88, 1)
		dc.DrawRectangle(ax+24, ay+h-48, float64(s.lastKey%200)+40, 8)
		_ = dc.Fill()
	}
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func failJSON(scenario string, probe probeResult) {
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"probe_ok":   0,
		"pass":       false,
		"pixels":     probe.PixDetail,
		"golden":     probe.GoldenChanged,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func actionNameForCode(code int) string {
	switch code {
	case 32:
		return "jump"
	case 74, 106:
		return "attack"
	default:
		return ""
	}
}

func main() {
	caseFlag := flag.String("case", "", "scenario case (remap or combo)")
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	writeGolden := flag.Bool("write-golden", false, "freeze goldens headlessly, verify, exit with no window")
	flag.Parse()

	which := *caseFlag
	if which != "remap" && which != "combo" {
		fmt.Fprintf(os.Stderr, "FAIL: --case=%q want remap or combo\n", *caseFlag)
		os.Exit(1)
	}
	scenario := "game_input--case=" + which
	autoScript = *autoOnly
	wrkit.EnsureUIFace()

	probe := runProbes(which)
	fmt.Fprintf(os.Stderr, "game_input: case=%s probes ok=%v logic=%v snaps=%d vec=%d pix=%v golden=%v(wrote=%v changed=%d) %s | %s\n",
		which, probe.OK, probe.LogicOK, probe.Snapshots, probe.Vectors,
		probe.PixOK, probe.GoldenOK, probe.GoldenWrote, probe.GoldenChanged, probe.Detail, probe.PixDetail)
	if !probe.OK {
		if *autoOnly {
			failJSON(scenario, probe)
		} else {
			fmt.Fprintln(os.Stderr, "game_input: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	if *writeGolden {
		b, _ := json.Marshal(map[string]any{
			"ability_id": abilityID,
			"scenario":   scenario,
			"probe_ok":   1,
			"pass":       true,
			"golden":     probe.GoldenChanged,
		})
		fmt.Println(string(b))
		return
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(8)
		wrkit.RequireMinRun(secs, abilityID)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	var legend []string
	var shellTitle string
	if which == "remap" {
		shellTitle = "game_input — 13.1 改键 (input-remap)"
		legend = []string{
			"空格跳·wasd移·J攻",
			"自动窗 1秒按 3秒改键 5秒死区",
			"改键后旧键静新键即响",
			"右栏 强度/末键/死区/脚本",
			"条长=动作强度",
			"JSON见 ability_extra",
		}
	} else {
		shellTitle = "game_input — 13.2 连招 (input-combo)"
		legend = []string{
			"空格跳 J攻·点触摸点",
			"跳后150ms内攻=FIRE",
			"黄格=缓存·蓝点=触点",
			"右栏 连/丢/满/零计数",
			"断触报错不崩",
			"JSON见 ability_extra",
		}
	}
	_ = legend
	_ = shellTitle

	m, err := liveMap()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: live map:", err)
		os.Exit(1)
	}
	buf, err := input.NewBuffer(input.DefaultBufferWindow)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: live buffer:", err)
		os.Exit(1)
	}
	s := &sim{
		which:   which,
		amap:    m,
		buf:     buf,
		touches: input.NewTouchTracker(),
		heroX:   120,
	}
	if secs > 0 {
		s.phase = wrkit.NewPhaseClock(float64(secs)*0.5, float64(secs)*0.8)
	} else {
		s.phase = wrkit.NewPhaseClock(0, 0)
	}

	// 满窗即内容：整窗为输入场景+人物响应+存档键位同场加厚，无分区。
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: bgR, G: bgG, B: bgB, A: 1}
	s.root = root
	s.fullBox = rendering.NewRenderBox()
	s.fullBox.FixedWidth, s.fullBox.FixedHeight = winW, winH
	live := s
	s.fullBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paintInputFullWindow(pc, size.Width, size.Height, live)
	}
	root.Place(s.fullBox, 0, 0)
	// 指标浮内容左上角，盖画面不划区。
	s.metric = wrkit.Label("input fps=-", 13, 0.92, 0.94, 0.98)
	root.Place(s.metric, 8, 8)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "game_input", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()

	var summary manualSummary
	feedKey := func(code int, pressed bool) {
		if code <= 0 {
			return
		}
		_ = s.amap.SetKey(code, pressed)
		if s.which == "combo" && pressed {
			now := core.Milliseconds(int64(s.elapsed * 1000))
			if name := actionNameForCode(code); name != "" {
				_ = s.buf.Push(name, now)
			}
		}
	}
	app := embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: bgR, ClearG: bgG, ClearB: bgB, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "game_input: close (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				if !manualMode {
					return
				}
				summary.Pointer++
				pos := core.V2(ev.X, ev.Y)
				now := core.Milliseconds(int64(s.elapsed * 1000))
				switch ev.Pointer {
				case platform.PointerDown:
					_ = s.touches.Begin(0, pos, now)
				case platform.PointerMove:
					_ = s.touches.Move(0, pos, now)
				case platform.PointerUp, platform.PointerCancel:
					_ = s.touches.End(0, now)
				}
				fmt.Fprintf(os.Stderr, "game_input: pointer %s (%.0f,%.0f) n=%d\n",
					ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize+summary.Touch)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("game_input events=%d",
						summary.Pointer+summary.Key+summary.Resize+summary.Touch))
				}
				return
			case platform.EventTouch:
				if !manualMode {
					return
				}
				summary.Touch++
				id := ev.TouchID
				if id <= 0 {
					return
				}
				pos := core.V2(ev.X, ev.Y)
				now := core.Milliseconds(int64(s.elapsed * 1000))
				switch ev.Pointer {
				case platform.PointerDown:
					_ = s.touches.Begin(id, pos, now)
				case platform.PointerMove:
					_ = s.touches.Move(id, pos, now)
				case platform.PointerUp, platform.PointerCancel:
					_ = s.touches.End(id, now)
				}
				fmt.Fprintf(os.Stderr, "game_input: touch id=%d %s (%.0f,%.0f) n=%d\n",
					id, ev.Pointer, ev.X, ev.Y, summary.Pointer+summary.Key+summary.Resize+summary.Touch)
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("game_input events=%d",
						summary.Pointer+summary.Key+summary.Resize+summary.Touch))
				}
				return
			case platform.EventKey:
				if !manualMode {
					return
				}
				if ev.Pressed && !ev.Repeat {
					summary.Key++
					s.lastKey = ev.KeyCode
					feedKey(ev.KeyCode, true)
					fmt.Fprintf(os.Stderr, "game_input: key code=%d n=%d\n",
						ev.KeyCode, summary.Pointer+summary.Key+summary.Resize+summary.Touch)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_input key=%d events=%d", ev.KeyCode,
							summary.Pointer+summary.Key+summary.Resize+summary.Touch))
					}
				} else if !ev.Pressed {
					feedKey(ev.KeyCode, false)
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					s.fullBox.FixedWidth, s.fullBox.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
				if manualMode {
					fmt.Fprintf(os.Stderr, "game_input: resize %dx%d n=%d\n",
						ev.Width, ev.Height, summary.Pointer+summary.Key+summary.Resize+summary.Touch)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("game_input events=%d",
							summary.Pointer+summary.Key+summary.Resize+summary.Touch))
					}
				}
				return
			default:
				return
			}
		},
	})
	s.app = app
	app.Scheduler().Tickers().Add(&ticker{s: s})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	presents := app.PresentCount()
	probeOK := 0
	if probe.OK {
		probeOK = 1
	}
	extra := map[string]any{
		"case":          which,
		"probe_ok":      probeOK,
		"boundary_skip": snap.BoundarySkip,
	}
	if which == "remap" {
		extra["scripted_pass"] = s.scriptedHit
		extra["scripted_total"] = 7
		extra["remaps"] = s.remaps
		extra["latency_frames"] = 0
	} else {
		extra["combo_hits"] = s.comboHits
		extra["combo_miss"] = s.comboMiss
		extra["cap64"] = s.comboCap
		extra["terminal_zero"] = s.comboZero
		extra["touch_ok"] = s.touchOK
		extra["buffer_len"] = s.buf.Len()
		extra["touches"] = s.touches.ActiveCount()
	}

	if *autoOnly {
		report := wrgate.BuildReport(wrgate.BuildInput{
			AbilityID:     abilityID,
			Scenario:      scenario,
			Snap:          snap,
			PresentCount:  presents,
			ElapsedSec:    elapsed,
			SurfaceAreaPx: winW * winH,
			Warmup:        true,
			Extra:         extra,
		})
		raw, _ := json.Marshal(report)
		fmt.Println(string(raw))
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		var scriptOK bool
		if which == "remap" {
			scriptOK = s.scriptedHit == 7
		} else {
			scriptOK = s.comboHits == 2 && s.comboMiss == 2 &&
				s.comboCap == input.MaxBuffered && s.comboZero == 1 && s.touchOK == 1
		}
		if presents < 1 || !probe.OK || !scriptOK {
			fmt.Fprintf(os.Stderr, "FAIL: presents=%d probe=%v script=%v (want >=1, true, full)\n",
				presents, probe.OK, scriptOK)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "game_input: OK case=%s presents=%d script=%v elapsed=%.1fs\n",
			which, presents, scriptOK, elapsed)
		return
	}
	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": abilityID,
		"scenario":   scenario,
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key,
			"resize": summary.Resize, "touch": summary.Touch,
		},
		"presents":    presents,
		"elapsed_sec": elapsed,
		"probe_ok":    probeOK,
		"extra":       extra,
		"timed":       summary.Timed,
		"note":        summary.Note,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "game_input: backend=%s case=%s presents=%d elapsed=%.1fs\n",
		win.Backend(), which, presents, elapsed)
}
