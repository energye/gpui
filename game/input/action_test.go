package input

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsInput = 1e-9

type bindingDef struct {
	Source string `json:"source"`
	Device int    `json:"device"`
	Code   int    `json:"code"`
	Sign   int    `json:"sign"`
}

type actionDef struct {
	Name     string       `json:"name"`
	Deadzone float64      `json:"deadzone"`
	Bindings []bindingDef `json:"bindings"`
}

type wantDef struct {
	Action   string  `json:"action"`
	Strength float64 `json:"strength"`
	Pressed  bool    `json:"pressed"`
}

type padButtonDef struct {
	Pad  int `json:"pad"`
	Code int `json:"code"`
}

type padAxisDef struct {
	Pad   int     `json:"pad"`
	Code  int     `json:"code"`
	Value float64 `json:"value"`
}

type snapshotDef struct {
	Name        string         `json:"name"`
	Keys        []int          `json:"keys"`
	PadButtons  []padButtonDef `json:"pad_buttons"`
	PadAxes     []padAxisDef   `json:"pad_axes"`
	PinchDeltas []float64      `json:"pinch_deltas"`
	WantPinch   float64        `json:"want_pinch"`
	Want        []wantDef      `json:"want"`
}

type vectorDef struct {
	Name        string       `json:"name"`
	NegX        string       `json:"neg_x"`
	PosX        string       `json:"pos_x"`
	NegY        string       `json:"neg_y"`
	PosY        string       `json:"pos_y"`
	Keys        []int        `json:"keys"`
	PadAxes     []padAxisDef `json:"pad_axes"`
	PinchDeltas []float64    `json:"pinch_deltas"`
	Want        [2]float64   `json:"want"`
}

type remapProbe struct {
	Name         string         `json:"name"`
	Keys         []int          `json:"keys"`
	PadButtons   []padButtonDef `json:"pad_buttons"`
	PadAxes      []padAxisDef   `json:"pad_axes"`
	WantStrength float64        `json:"want_strength"`
	WantPressed  bool           `json:"want_pressed"`
}

type remapDef struct {
	Action      string       `json:"action"`
	NewBindings []bindingDef `json:"new_bindings"`
	Probes      []remapProbe `json:"probes"`
}

type retuneProbe struct {
	Name         string       `json:"name"`
	Keys         []int        `json:"keys"`
	PadAxes      []padAxisDef `json:"pad_axes"`
	WantStrength float64      `json:"want_strength"`
	WantPressed  bool         `json:"want_pressed"`
}

type retuneDef struct {
	Action      string        `json:"action"`
	NewDeadzone float64       `json:"new_deadzone"`
	Probes      []retuneProbe `json:"probes"`
}

type rumbleStep struct {
	DtMs       int64   `json:"dt_ms"`
	WantWeak   float64 `json:"want_weak"`
	WantStrong float64 `json:"want_strong"`
	WantActive bool    `json:"want_active"`
}

type rumbleDef struct {
	Name   string       `json:"name"`
	Pad    int          `json:"pad"`
	Weak   float64      `json:"weak"`
	Strong float64      `json:"strong"`
	DurMs  int64        `json:"dur_ms"`
	Stop   bool         `json:"stop"`
	Steps  []rumbleStep `json:"steps"`
}

type badRumbleDef struct {
	Pad    int     `json:"pad"`
	Weak   float64 `json:"weak"`
	Strong float64 `json:"strong"`
	DurMs  int64   `json:"dur_ms"`
}

type actionFile struct {
	Sources     []string       `json:"sources"`
	Actions     []actionDef    `json:"actions"`
	Snapshots   []snapshotDef  `json:"snapshots"`
	Vectors     []vectorDef    `json:"vectors"`
	Remap       remapDef       `json:"remap"`
	Retune      retuneDef      `json:"retune"`
	Rumbles     []rumbleDef    `json:"rumbles"`
	BadActions  []string       `json:"bad_actions"`
	BadBindings []bindingDef   `json:"bad_bindings"`
	BadKeys     []int          `json:"bad_keys"`
	BadRumbles  []badRumbleDef `json:"bad_rumbles"`
}

func loadActionCases(t *testing.T) actionFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "action_cases.json"))
	if err != nil {
		t.Fatalf("read action_cases.json: %v", err)
	}
	var f actionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode action_cases.json: %v", err)
	}
	if len(f.Actions) == 0 || len(f.Snapshots) == 0 {
		t.Fatal("action_cases.json has no actions or snapshots")
	}
	return f
}

func bindingFromDef(t *testing.T, d bindingDef) Binding {
	t.Helper()
	src, err := ParseSource(d.Source)
	if err != nil {
		t.Fatalf("ParseSource %q: %v", d.Source, err)
	}
	b := Binding{Source: src, Device: d.Device, Code: d.Code, Sign: d.Sign}
	if err := b.validate(); err != nil {
		t.Fatalf("binding %+v invalid: %v", d, err)
	}
	return b
}

func buildMap(t *testing.T, f actionFile) *Map {
	t.Helper()
	m := NewMap()
	for _, a := range f.Actions {
		if err := m.AddAction(a.Name, a.Deadzone); err != nil {
			t.Fatalf("AddAction %q: %v", a.Name, err)
		}
		for _, bd := range a.Bindings {
			if err := m.Bind(a.Name, bindingFromDef(t, bd)); err != nil {
				t.Fatalf("Bind %q %+v: %v", a.Name, bd, err)
			}
		}
	}
	if got := m.ActionCount(); got != len(f.Actions) {
		t.Fatalf("ActionCount = %d, want %d", got, len(f.Actions))
	}
	return m
}

func applySnapshot(t *testing.T, m *Map, keys []int, buttons []padButtonDef, axes []padAxisDef, deltas []float64) {
	t.Helper()
	for _, k := range keys {
		if err := m.SetKey(k, true); err != nil {
			t.Fatalf("SetKey %d: %v", k, err)
		}
	}
	for _, b := range buttons {
		if err := m.SetPadButton(b.Pad, b.Code, true); err != nil {
			t.Fatalf("SetPadButton %+v: %v", b, err)
		}
	}
	for _, a := range axes {
		if err := m.SetPadAxis(a.Pad, a.Code, a.Value); err != nil {
			t.Fatalf("SetPadAxis %+v: %v", a, err)
		}
	}
	for _, d := range deltas {
		if err := m.AddPinchDelta(d); err != nil {
			t.Fatalf("AddPinchDelta %v: %v", d, err)
		}
	}
}

func mustFindSnapshot(t *testing.T, f actionFile, name string) snapshotDef {
	t.Helper()
	for _, s := range f.Snapshots {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("action_cases.json has no snapshot %q", name)
	return snapshotDef{}
}

func closeStrength(got, want float64) bool { return math.Abs(got-want) < epsInput }

// A: multi-key, deadzone, rumble, and pinch gesture all land on the frozen numbers.
func TestActionValuesFromCases(t *testing.T) {
	f := loadActionCases(t)
	if got := AllSources(); len(got) != len(f.Sources) {
		t.Fatalf("sources = %d, want %d", len(got), len(f.Sources))
	}
	for i, s := range AllSources() {
		if s.String() != f.Sources[i] {
			t.Errorf("source[%d] = %q, want %q", i, s.String(), f.Sources[i])
		}
		back, err := ParseSource(f.Sources[i])
		if err != nil || back != s {
			t.Errorf("ParseSource %q = %v/%v, want %v/nil", f.Sources[i], back, err, s)
		}
		if !Valid(s) {
			t.Errorf("Valid(%v) = false, want true", s)
		}
	}
	for _, s := range f.Snapshots {
		m := buildMap(t, f)
		applySnapshot(t, m, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
		if got := m.Pinch(); !closeStrength(got, s.WantPinch) {
			t.Errorf("%s: pinch = %.17g, want %.17g", s.Name, got, s.WantPinch)
		}
		for _, w := range s.Want {
			got, err := m.Strength(w.Action)
			if err != nil {
				t.Errorf("%s: Strength %q: %v", s.Name, w.Action, err)
				continue
			}
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Errorf("%s: %q strength = %v, want finite", s.Name, w.Action, got)
				continue
			}
			if got < 0 || got > 1 {
				t.Errorf("%s: %q strength = %v, want within [0,1]", s.Name, w.Action, got)
			}
			if !closeStrength(got, w.Strength) {
				t.Errorf("%s: %q strength = %.17g, want %.17g", s.Name, w.Action, got, w.Strength)
			}
			pressed, err := m.Pressed(w.Action)
			if err != nil {
				t.Errorf("%s: Pressed %q: %v", s.Name, w.Action, err)
				continue
			}
			if pressed != w.Pressed {
				t.Errorf("%s: %q pressed = %v, want %v", s.Name, w.Action, pressed, w.Pressed)
			}
		}
	}
	for _, v := range f.Vectors {
		m := buildMap(t, f)
		applySnapshot(t, m, v.Keys, nil, v.PadAxes, v.PinchDeltas)
		got, err := m.Vector(v.NegX, v.PosX, v.NegY, v.PosY)
		if err != nil {
			t.Errorf("%s: Vector: %v", v.Name, err)
			continue
		}
		if math.Abs(got.X-v.Want[0]) >= epsInput || math.Abs(got.Y-v.Want[1]) >= epsInput {
			t.Errorf("%s: vector = %v, want %v", v.Name, got, v.Want)
		}
	}
	// Remap: old inputs die, new inputs live. One remapped map serves all
	// probes; ResetInputs clears the hardware between probes.
	m := buildMap(t, f)
	var newBindings []Binding
	for _, bd := range f.Remap.NewBindings {
		newBindings = append(newBindings, bindingFromDef(t, bd))
	}
	if err := m.Rebind(f.Remap.Action, newBindings); err != nil {
		t.Fatalf("Rebind %q: %v", f.Remap.Action, err)
	}
	for _, p := range f.Remap.Probes {
		m.ResetInputs()
		applySnapshot(t, m, p.Keys, p.PadButtons, p.PadAxes, nil)
		got, err := m.Strength(f.Remap.Action)
		if err != nil {
			t.Errorf("remap %s: Strength: %v", p.Name, err)
			continue
		}
		if !closeStrength(got, p.WantStrength) {
			t.Errorf("remap %s: strength = %.17g, want %.17g", p.Name, got, p.WantStrength)
		}
		if pressed, _ := m.Pressed(f.Remap.Action); pressed != p.WantPressed {
			t.Errorf("remap %s: pressed = %v, want %v", p.Name, pressed, p.WantPressed)
		}
	}
	// Retune: a higher deadzone gates the same stick without killing keys.
	// One retuned map serves both probes.
	m = buildMap(t, f)
	if err := m.SetDeadzone(f.Retune.Action, f.Retune.NewDeadzone); err != nil {
		t.Fatalf("SetDeadzone: %v", err)
	}
	if got, ok := m.Deadzone(f.Retune.Action); !ok || math.Abs(got-f.Retune.NewDeadzone) >= epsInput {
		t.Fatalf("Deadzone = %v/%v, want %v/true", got, ok, f.Retune.NewDeadzone)
	}
	for _, p := range f.Retune.Probes {
		m.ResetInputs()
		applySnapshot(t, m, p.Keys, nil, p.PadAxes, nil)
		got, err := m.Strength(f.Retune.Action)
		if err != nil {
			t.Errorf("retune %s: Strength: %v", p.Name, err)
			continue
		}
		if !closeStrength(got, p.WantStrength) {
			t.Errorf("retune %s: strength = %.17g, want %.17g", p.Name, got, p.WantStrength)
		}
		if pressed, _ := m.Pressed(f.Retune.Action); pressed != p.WantPressed {
			t.Errorf("retune %s: pressed = %v, want %v", p.Name, pressed, p.WantPressed)
		}
	}
	// Rumble: levels hold until the clock runs out, stop clears at once.
	for _, r := range f.Rumbles {
		m := NewMap()
		if err := m.StartRumble(r.Pad, r.Weak, r.Strong, core.Milliseconds(r.DurMs)); err != nil {
			t.Errorf("%s: StartRumble: %v", r.Name, err)
			continue
		}
		if r.Stop {
			if err := m.StopRumble(r.Pad); err != nil {
				t.Errorf("%s: StopRumble: %v", r.Name, err)
			}
		}
		for i, st := range r.Steps {
			m.UpdateRumbles(core.Milliseconds(st.DtMs))
			w, s, active := m.RumbleLevels(r.Pad)
			if active != st.WantActive {
				t.Errorf("%s step %d: active = %v, want %v", r.Name, i, active, st.WantActive)
			}
			if math.Abs(w-st.WantWeak) >= epsInput || math.Abs(s-st.WantStrong) >= epsInput {
				t.Errorf("%s step %d: levels = (%v,%v), want (%v,%v)", r.Name, i, w, s, st.WantWeak, st.WantStrong)
			}
			if m.RumbleActive(r.Pad) != st.WantActive {
				t.Errorf("%s step %d: RumbleActive mismatch", r.Name, i)
			}
		}
	}
	// Registry matches the file: sorted names, deadzones, binding counts.
	m = buildMap(t, f)
	names := m.Actions()
	if len(names) != len(f.Actions) {
		t.Fatalf("Actions = %d, want %d", len(names), len(f.Actions))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Errorf("Actions not sorted: %v", names)
			break
		}
	}
	for _, a := range f.Actions {
		if !m.Has(a.Name) {
			t.Errorf("Has %q = false, want true", a.Name)
		}
		dz, ok := m.Deadzone(a.Name)
		if !ok || math.Abs(dz-a.Deadzone) >= epsInput {
			t.Errorf("Deadzone %q = %v, want %v", a.Name, dz, a.Deadzone)
		}
		bs, err := m.Bindings(a.Name)
		if err != nil || len(bs) != len(a.Bindings) {
			t.Errorf("Bindings %q = %d/%v, want %d/nil", a.Name, len(bs), err, len(a.Bindings))
		}
	}
}

// B: empty, zero, oversized, and corrupt inputs error without crashing.
func TestActionEdgesNoCrash(t *testing.T) {
	f := loadActionCases(t)
	m := buildMap(t, f)

	// Empty binding lists stay silent, never pressed.
	if s, err := m.Strength("empty"); err != nil || s != 0 {
		t.Errorf("empty Strength = %v/%v, want 0/nil", s, err)
	}
	if p, err := m.Pressed("empty"); err != nil || p {
		t.Errorf("empty Pressed = %v/%v, want false/nil", p, err)
	}
	if err := m.ClearBindings("empty"); err != nil {
		t.Errorf("ClearBindings empty: %v", err)
	}

	// Unknown and empty action names fail closed with the right codes.
	for _, name := range f.BadActions {
		if _, err := m.Strength(name); err == nil {
			t.Errorf("Strength %q: want error", name)
		} else if name == "" {
			if core.CodeOf(err) != core.CodeInvalidArg {
				t.Errorf("Strength %q code = %v, want invalid-arg", name, core.CodeOf(err))
			}
		} else if core.CodeOf(err) != core.CodeNotFound {
			t.Errorf("Strength %q code = %v, want not-found", name, core.CodeOf(err))
		}
		if _, err := m.Pressed(name); err == nil {
			t.Errorf("Pressed %q: want error", name)
		}
		if _, err := m.Bindings(name); err == nil {
			t.Errorf("Bindings %q: want error", name)
		}
		if err := m.SetDeadzone(name, 0.1); err == nil {
			t.Errorf("SetDeadzone %q: want error", name)
		}
		if err := m.ClearBindings(name); err == nil {
			t.Errorf("ClearBindings %q: want error", name)
		}
	}
	if _, err := m.Vector("", "move_right", "move_up", "move_down"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Vector empty name err = %v, want invalid-arg", err)
	}
	if _, err := m.Vector("move_left", "no-such", "move_up", "move_down"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("Vector unknown name err = %v, want not-found", err)
	}
	if v, err := m.Vector("", "", "", ""); err == nil || v != (core.Vec2{}) {
		t.Errorf("Vector all empty = %v/%v, want zero + error", v, err)
	}

	// Bad bindings never store: Bind rejects every frozen bad shape and the
	// old list stays untouched. Constructors cover their own slice below.
	before, _ := m.Bindings("jump")
	for i, bd := range f.BadBindings {
		src, perr := ParseSource(bd.Source)
		var b Binding
		if perr == nil {
			b = Binding{Source: src, Device: bd.Device, Code: bd.Code, Sign: bd.Sign}
		} else {
			b = Binding{Source: Source(99), Device: bd.Device, Code: bd.Code, Sign: bd.Sign}
		}
		if err := m.Bind("jump", b); err == nil {
			t.Errorf("bad binding[%d] Bind: want error", i)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad binding[%d] Bind code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	// Constructors reject their own bad arguments.
	for _, err := range []error{
		func() error { _, e := NewKeyBinding(0); return e }(),
		func() error { _, e := NewPadButtonBinding(-2, 0); return e }(),
		func() error { _, e := NewPadAxisBinding(0, 0, 0); return e }(),
		func() error { _, e := NewPinchBinding(0); return e }(),
	} {
		if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad constructor err = %v, want invalid-arg", err)
		}
	}
	if after, _ := m.Bindings("jump"); len(after) != len(before) {
		t.Error("bad Bind changed the binding list, want untouched")
	}
	// Exact duplicates and duplicates inside Rebind are rejected.
	dup, _ := NewKeyBinding(32)
	if err := m.Bind("jump", dup); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duplicate Bind err = %v, want invalid-arg", err)
	}
	if _, err := NewKeyBinding(33); err != nil {
		t.Errorf("NewKeyBinding 33: unexpected %v", err)
	}
	if err := m.Rebind("jump", []Binding{dup, dup}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("duplicate Rebind err = %v, want invalid-arg", err)
	}
	if err := m.Rebind("jump", []Binding{{Source: Source(99)}}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad Rebind err = %v, want invalid-arg", err)
	}
	if after, _ := m.Bindings("jump"); len(after) != len(before) {
		t.Error("bad Rebind changed the binding list, want untouched")
	}
	// Unbind of an absent binding reports not-found and changes nothing.
	absent, _ := NewKeyBinding(9999)
	if err := m.Unbind("jump", absent); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("Unbind absent err = %v, want not-found", err)
	}
	if err := m.Unbind("no-such", dup); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("Unbind unknown action err = %v, want not-found", err)
	}

	// Bad hardware ids change nothing.
	for _, code := range f.BadKeys {
		if err := m.SetKey(code, true); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("SetKey %d err = %v, want invalid-arg", code, err)
		}
	}
	if err := m.SetPadButton(-1, 0, true); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("SetPadButton negative pad err = %v, want invalid-arg", err)
	}
	if err := m.SetPadAxis(-1, 0, 0.5); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("SetPadAxis negative pad err = %v, want invalid-arg", err)
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if err := m.SetPadAxis(0, 0, v); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("SetPadAxis %v err = %v, want invalid-arg", v, err)
		}
	}
	for _, d := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		pinchBefore := m.Pinch()
		if err := m.AddPinchDelta(d); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("AddPinchDelta %v err = %v, want invalid-arg", d, err)
		}
		if m.Pinch() != pinchBefore {
			t.Errorf("AddPinchDelta %v changed pinch, want untouched", d)
		}
	}
	// Bad deadzones and duplicate/empty action adds change nothing.
	countBefore := m.ActionCount()
	for _, dz := range []float64{math.NaN(), math.Inf(1), -0.1, 1, 2} {
		if err := m.AddAction("bad_dz", dz); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("AddAction dz %v err = %v, want invalid-arg", dz, err)
		}
		if err := m.SetDeadzone("jump", dz); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("SetDeadzone %v err = %v, want invalid-arg", dz, err)
		}
	}
	if err := m.AddAction("", 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("AddAction empty err = %v, want invalid-arg", err)
	}
	if err := m.AddAction("jump", 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("AddAction duplicate err = %v, want invalid-arg", err)
	}
	if m.ActionCount() != countBefore || m.Has("bad_dz") {
		t.Error("bad AddAction changed the registry, want untouched")
	}
	if err := m.RemoveAction(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("RemoveAction empty err = %v, want invalid-arg", err)
	}
	if err := m.RemoveAction("no-such"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("RemoveAction missing err = %v, want not-found", err)
	}

	// Bad rumbles never arm.
	for i, r := range f.BadRumbles {
		if err := m.StartRumble(r.Pad, r.Weak, r.Strong, core.Milliseconds(r.DurMs)); err == nil {
			t.Errorf("bad rumble[%d]: want error", i)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad rumble[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if err := m.StopRumble(-1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("StopRumble -1 err = %v, want invalid-arg", err)
	}
	if w, s, active := m.RumbleLevels(-1); active || w != 0 || s != 0 {
		t.Errorf("RumbleLevels(-1) = %v/%v/%v, want 0/0/false", w, s, active)
	}
	// Zero and negative dt advance nothing.
	m2 := NewMap()
	if err := m2.StartRumble(0, 0.5, 0.5, core.Milliseconds(100)); err != nil {
		t.Fatalf("StartRumble: %v", err)
	}
	m2.UpdateRumbles(0)
	m2.UpdateRumbles(core.Milliseconds(-10))
	if w, s, active := m2.RumbleLevels(0); !active || w != 0.5 || s != 0.5 {
		t.Errorf("zero-dt rumble = %v/%v/%v, want 0.5/0.5/true", w, s, active)
	}

	// Huge-but-finite inputs clamp, never NaN.
	if err := m.SetPadAxis(0, 0, 1e308); err != nil {
		t.Errorf("huge axis: %v", err)
	}
	if s, _ := m.Strength("move_right"); math.IsNaN(s) || math.IsInf(s, 0) || s != 1 {
		t.Errorf("huge axis strength = %v, want 1", s)
	}
	if err := m.SetKey(1<<30, true); err != nil {
		t.Errorf("huge key: %v", err)
	}

	// Nil receivers never panic.
	var nilMap *Map
	if nilMap.Has("jump") || nilMap.ActionCount() != 0 || len(nilMap.Actions()) != 0 {
		t.Error("nil accessors returned live values")
	}
	if _, ok := nilMap.Deadzone("jump"); ok {
		t.Error("nil Deadzone ok=true, want false")
	}
	if _, err := nilMap.Strength("jump"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Strength err = %v, want invalid-arg", err)
	}
	if _, err := nilMap.Pressed("jump"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Pressed err = %v, want invalid-arg", err)
	}
	if _, err := nilMap.Vector("a", "b", "c", "d"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Vector err = %v, want invalid-arg", err)
	}
	if nilMap.Pinch() != 0 {
		t.Error("nil Pinch != 0")
	}
	if _, _, active := nilMap.RumbleLevels(0); active {
		t.Error("nil RumbleLevels active=true, want false")
	}
	if nilMap.RumbleActive(0) {
		t.Error("nil RumbleActive = true, want false")
	}
	for _, err := range []error{
		nilMap.AddAction("x", 0), nilMap.RemoveAction("x"), nilMap.SetDeadzone("x", 0),
		nilMap.Bind("x", dup), nilMap.Unbind("x", dup), nilMap.Rebind("x", nil),
		nilMap.ClearBindings("x"), nilMap.SetKey(1, true), nilMap.SetPadButton(0, 0, true),
		nilMap.SetPadAxis(0, 0, 0), nilMap.AddPinchDelta(1.1),
		nilMap.StartRumble(0, 0, 0, 1), nilMap.StopRumble(0),
	} {
		if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("nil mutator err = %v, want invalid-arg", err)
		}
	}
	if _, err := nilMap.Bindings("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Bindings err = %v, want invalid-arg", err)
	}
	nilMap.ClearPinch()
	nilMap.ResetInputs()
	nilMap.UpdateRumbles(10)
	if _, err := ParseSource(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("ParseSource empty err = %v, want invalid-arg", err)
	}
	if got := Source(99).String(); got != "unknown" {
		t.Errorf("Source(99) = %q, want unknown", got)
	}
	if Valid(Source(99)) {
		t.Error("Valid(99) = true, want false")
	}
}

// C does not need pixels (pure math, draws nothing): the number path must
// be lossless and replay bitwise identical instead.
func TestActionBoundaryIdentical(t *testing.T) {
	f := loadActionCases(t)
	for _, s := range f.Snapshots {
		a, b := buildMap(t, f), buildMap(t, f)
		applySnapshot(t, a, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
		applySnapshot(t, b, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
		if a.Pinch() != b.Pinch() {
			t.Fatalf("%s: pinch replay diverged: %v vs %v", s.Name, a.Pinch(), b.Pinch())
		}
		for _, w := range s.Want {
			sa, _ := a.Strength(w.Action)
			sb, _ := b.Strength(w.Action)
			if sa != sb {
				t.Fatalf("%s: %q replay diverged: %.17g vs %.17g", s.Name, w.Action, sa, sb)
			}
			pa, _ := a.Pressed(w.Action)
			pb, _ := b.Pressed(w.Action)
			if pa != pb {
				t.Fatalf("%s: %q pressed replay diverged", s.Name, w.Action)
			}
		}
	}
	// Returned binding slices are copies.
	m := buildMap(t, f)
	bs, err := m.Bindings("jump")
	if err != nil || len(bs) == 0 {
		t.Fatalf("Bindings jump: %v/%d", err, len(bs))
	}
	bs[0] = Binding{}
	again, _ := m.Bindings("jump")
	if again[0] == (Binding{}) {
		t.Error("Bindings aliases the map, want a copy")
	}
	// Rebind copies the input slice.
	src := []Binding{mustSampleBinding(t)}
	if err := m.Rebind("empty", src); err != nil {
		t.Fatalf("Rebind empty: %v", err)
	}
	src[0] = Binding{}
	kept, _ := m.Bindings("empty")
	if kept[0] == (Binding{}) {
		t.Error("Rebind aliases the caller slice, want a copy")
	}
	// Source names round-trip losslessly.
	for _, s := range AllSources() {
		back, err := ParseSource(s.String())
		if err != nil || back != s {
			t.Errorf("source %v round trip = %v/%v", s, back, err)
		}
	}
}

func mustSampleBinding(t *testing.T) Binding {
	t.Helper()
	b, err := NewKeyBinding(99)
	if err != nil {
		t.Fatalf("NewKeyBinding: %v", err)
	}
	return b
}

// D: a hundred inputs evaluate with a measured cost.
func TestActionPerfHundred(t *testing.T) {
	f := loadActionCases(t)
	m := buildMap(t, f)
	// Synthetic load only (no golden): golden strengths stay in
	// action_cases.json. One frozen snapshot drives every evaluation.
	s := mustFindSnapshot(t, f, "s_multi_inputs")
	applySnapshot(t, m, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
	names := m.Actions()
	const reps = 20000
	var acc float64
	var accVec core.Vec2
	start := time.Now()
	for i := 0; i < reps; i++ {
		for _, n := range names {
			v, err := m.Strength(n)
			if err != nil {
				t.Fatalf("Strength %q: %v", n, err)
			}
			acc += v
		}
		v, err := m.Vector("move_left", "move_right", "move_up", "move_down")
		if err != nil {
			t.Fatalf("Vector: %v", err)
		}
		accVec = accVec.Add(v)
	}
	el := time.Since(start)
	total := int64(reps * (len(names) + 1))
	t.Logf("input-100: %d reps x %d actions (+vector) (%d evals) in %v (%.1f ns/op)",
		reps, len(names), total, el, float64(el.Nanoseconds())/float64(total))
	if acc == 0 || (accVec.IsZero() && acc == 0) {
		t.Error("perf loop folded to zero, benchmark invalid")
	}
}

// E: long runs neither drift nor diverge between replays.
func TestActionLongRunNoDrift(t *testing.T) {
	f := loadActionCases(t)
	mk := func() *Map { return buildMap(t, f) }
	a, b := mk(), mk()
	s := mustFindSnapshot(t, f, "s_multi_inputs")
	applySnapshot(t, a, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
	applySnapshot(t, b, s.Keys, s.PadButtons, s.PadAxes, s.PinchDeltas)
	for _, n := range a.Actions() {
		sa, _ := a.Strength(n)
		sb, _ := b.Strength(n)
		if sa != sb {
			t.Fatalf("replay diverged on %q: %.17g vs %.17g", n, sa, sb)
		}
	}
	// Small stick drift stays gated across 10k identical evaluations.
	drift := mustFindSnapshot(t, f, "s_stick_drift")
	da, db := mk(), mk()
	applySnapshot(t, da, drift.Keys, drift.PadButtons, drift.PadAxes, drift.PinchDeltas)
	applySnapshot(t, db, drift.Keys, drift.PadButtons, drift.PadAxes, drift.PinchDeltas)
	for i := 0; i < 10000; i++ {
		sa, _ := da.Strength("move_right")
		sb, _ := db.Strength("move_right")
		if sa != 0 || sb != 0 || sa != sb {
			t.Fatalf("drift rep %d: %v vs %v, want 0/0", i, sa, sb)
		}
	}
	// Retuning the deadzone to 0 releases the same stick deterministically.
	if err := da.SetDeadzone("move_right", 0); err != nil {
		t.Fatalf("SetDeadzone 0: %v", err)
	}
	if s, _ := da.Strength("move_right"); math.Abs(s-0.1) >= epsInput {
		t.Errorf("ungated drift = %.17g, want 0.1", s)
	}
	if err := da.SetDeadzone("move_right", 0.2); err != nil {
		t.Fatalf("restore deadzone: %v", err)
	}
	if s, _ := da.Strength("move_right"); s != 0 {
		t.Errorf("restored drift = %v, want 0", s)
	}
	// Pinch accumulation clamps and replays: 10k tiny spreads park at 1.
	pa, pb := mk(), mk()
	for i := 0; i < 10000; i++ {
		if err := pa.AddPinchDelta(1.001); err != nil {
			t.Fatalf("pinch a step %d: %v", i, err)
		}
		if err := pb.AddPinchDelta(1.001); err != nil {
			t.Fatalf("pinch b step %d: %v", i, err)
		}
	}
	if pa.Pinch() != 1 || pb.Pinch() != 1 {
		t.Errorf("pinch soak = %v/%v, want 1/1", pa.Pinch(), pb.Pinch())
	}
	if sa, _ := pa.Strength("zoom_in"); sa != 1 {
		t.Errorf("soaked zoom_in = %v, want 1", sa)
	}
	// Rumble clocks tick down identically on both replays.
	ra, rb := mk(), mk()
	if err := ra.StartRumble(0, 0.4, 0.6, core.Milliseconds(1000)); err != nil {
		t.Fatalf("StartRumble a: %v", err)
	}
	if err := rb.StartRumble(0, 0.4, 0.6, core.Milliseconds(1000)); err != nil {
		t.Fatalf("StartRumble b: %v", err)
	}
	for i := 0; i < 100; i++ {
		ra.UpdateRumbles(core.Milliseconds(10))
		rb.UpdateRumbles(core.Milliseconds(10))
		wa, sa, aa := ra.RumbleLevels(0)
		wb, sb, ab := rb.RumbleLevels(0)
		if wa != wb || sa != sb || aa != ab {
			t.Fatalf("rumble rep %d diverged", i)
		}
	}
	if _, _, active := ra.RumbleLevels(0); active {
		t.Error("100x10ms rumble still active, want expired")
	}
	// Reset clears live inputs but keeps the registry.
	a.ResetInputs()
	a.ClearPinch()
	for _, n := range a.Actions() {
		if s, _ := a.Strength(n); s != 0 {
			t.Errorf("after reset %q = %v, want 0", n, s)
		}
	}
	if a.Pinch() != 0 {
		t.Error("after reset pinch != 0")
	}
	if a.ActionCount() != len(f.Actions) {
		t.Error("reset changed the registry, want it kept")
	}
}

// F: offscreen golden stands in for the window (window-exempt pure math).
// The frozen strengths in action_cases.json are the evidence both backends
// share; shape assertions below pin the meaning, not just the numbers.
func TestActionOffscreenGolden(t *testing.T) {
	f := loadActionCases(t)
	// Digital fires full, analog fires partial through the same gate.
	jump := mustFindSnapshot(t, f, "s_keys_jump")
	for _, w := range jump.Want {
		if w.Action == "jump" && w.Strength != 1 {
			t.Errorf("digital jump golden = %v, want 1", w.Strength)
		}
	}
	half := mustFindSnapshot(t, f, "s_stick_right_half")
	for _, w := range half.Want {
		if w.Action == "move_right" && (w.Strength <= 0 || w.Strength >= 1) {
			t.Errorf("analog half golden = %v, want inside (0,1)", w.Strength)
		}
	}
	// Drift gate: the same stick below deadzone stays silent.
	drift := mustFindSnapshot(t, f, "s_stick_drift")
	for _, w := range drift.Want {
		if w.Strength != 0 || w.Pressed {
			t.Errorf("drift golden %+v, want 0/false", w)
		}
	}
	// Sides never fire together on one axis snapshot.
	for _, name := range []string{"s_stick_right_half", "s_stick_left_full"} {
		s := mustFindSnapshot(t, f, name)
		var left, right *wantDef
		for i := range s.Want {
			switch s.Want[i].Action {
			case "move_left":
				left = &s.Want[i]
			case "move_right":
				right = &s.Want[i]
			}
		}
		if left != nil && right != nil && left.Pressed && right.Pressed {
			t.Errorf("%s: both sides pressed, want one side only", name)
		}
	}
	// Pinch directions split by sign.
	spread := mustFindSnapshot(t, f, "s_pinch_spread")
	closeSnap := mustFindSnapshot(t, f, "s_pinch_close")
	strengthOf := func(s snapshotDef, action string) wantDef {
		for _, w := range s.Want {
			if w.Action == action {
				return w
			}
		}
		return wantDef{}
	}
	if got := strengthOf(spread, "zoom_in"); !got.Pressed || got.Strength <= 0 {
		t.Errorf("spread zoom_in = %+v, want pressed partial", got)
	}
	if got := strengthOf(spread, "zoom_out"); got.Pressed {
		t.Errorf("spread zoom_out = %+v, want silent", got)
	}
	if got := strengthOf(closeSnap, "zoom_out"); !got.Pressed {
		t.Errorf("close zoom_out = %+v, want pressed", got)
	}
	if spread.WantPinch <= 0 || closeSnap.WantPinch >= 0 {
		t.Errorf("pinch signs = %v/%v, want +spread/-close", spread.WantPinch, closeSnap.WantPinch)
	}
	// Vectors: diagonals normalize to length 1, small drift stays zero.
	byVec := map[string]vectorDef{}
	for _, v := range f.Vectors {
		byVec[v.Name] = v
	}
	diag := byVec["v_keys_diag"]
	if l := math.Hypot(diag.Want[0], diag.Want[1]); math.Abs(l-1) >= 1e-9 {
		t.Errorf("diag golden length = %v, want 1", l)
	}
	if byVec["v_drift"].Want != [2]float64{0, 0} {
		t.Errorf("drift vector golden = %v, want [0 0]", byVec["v_drift"].Want)
	}
	// Remap kills the old key and the old button, wakes the new key.
	if len(f.Remap.Probes) != 3 {
		t.Fatalf("remap probes = %d, want 3", len(f.Remap.Probes))
	}
	if f.Remap.Probes[0].WantPressed || f.Remap.Probes[2].WantPressed {
		t.Error("remap golden: old inputs must be silent after Rebind")
	}
	if !f.Remap.Probes[1].WantPressed || f.Remap.Probes[1].WantStrength != 1 {
		t.Errorf("remap golden new = %+v, want 1/true", f.Remap.Probes[1])
	}
	// Retune gates the same stick without muting keys.
	if len(f.Retune.Probes) != 2 {
		t.Fatalf("retune probes = %d, want 2", len(f.Retune.Probes))
	}
	if f.Retune.Probes[0].WantPressed || f.Retune.Probes[1].WantStrength != 1 {
		t.Errorf("retune golden = %+v/%+v, want gated stick + full key",
			f.Retune.Probes[0], f.Retune.Probes[1])
	}
	// Source order is frozen for files and logs.
	if len(f.Sources) != len(AllSources()) {
		t.Fatalf("sources = %d, want %d", len(f.Sources), len(AllSources()))
	}
	for i, s := range AllSources() {
		if f.Sources[i] != s.String() {
			t.Errorf("source[%d] = %q, want %q", i, f.Sources[i], s.String())
		}
	}
}
