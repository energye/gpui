package anim

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

const epsMachine = 1e-9

type smStateDef struct {
	Name    string   `json:"name"`
	CanTo   []string `json:"can_to"`
	BlendMs int64    `json:"blend_ms"`
}

type smStepDef struct {
	DtMs        int64   `json:"dt_ms"`
	Request     string  `json:"request"`
	WantCurrent string  `json:"want_current"`
	WantFrom    string  `json:"want_from"`
	WantTo      string  `json:"want_to"`
	WantBlend   bool    `json:"want_blending"`
	WantProg    float64 `json:"want_progress"`
	WantFromW   float64 `json:"want_from_w"`
	WantToW     float64 `json:"want_to_w"`
	WantErr     string  `json:"want_err"`
}

type smSeqDef struct {
	Name  string      `json:"name"`
	Start string      `json:"start"`
	Steps []smStepDef `json:"steps"`
}

type smFile struct {
	States    []smStateDef `json:"states"`
	Sequences []smSeqDef   `json:"sequences"`
}

func loadMachineCases(t *testing.T) smFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "statemachine_cases.json"))
	if err != nil {
		t.Fatalf("read statemachine_cases.json: %v", err)
	}
	var f smFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode statemachine_cases.json: %v", err)
	}
	if len(f.States) == 0 || len(f.Sequences) == 0 {
		t.Fatal("statemachine_cases.json has no states or sequences")
	}
	return f
}

func buildMachine(t *testing.T, f smFile) *Machine {
	t.Helper()
	m := NewMachine()
	for _, s := range f.States {
		if err := m.AddState(State{
			Name:  s.Name,
			CanTo: append([]string(nil), s.CanTo...),
			Blend: core.Milliseconds(s.BlendMs),
		}); err != nil {
			t.Fatalf("AddState %q: %v", s.Name, err)
		}
	}
	return m
}

func mustFindMachineSeq(t *testing.T, f smFile, name string) smSeqDef {
	t.Helper()
	for _, s := range f.Sequences {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("statemachine_cases.json has no sequence %q", name)
	return smSeqDef{}
}

func checkMachineStep(t *testing.T, m *Machine, seq string, i int, st smStepDef) {
	t.Helper()
	if got := m.Current(); got != st.WantCurrent {
		t.Errorf("%s step %d: current = %q, want %q", seq, i, got, st.WantCurrent)
	}
	if got := m.From(); got != st.WantFrom {
		t.Errorf("%s step %d: from = %q, want %q", seq, i, got, st.WantFrom)
	}
	if got := m.To(); got != st.WantTo {
		t.Errorf("%s step %d: to = %q, want %q", seq, i, got, st.WantTo)
	}
	if got := m.Blending(); got != st.WantBlend {
		t.Errorf("%s step %d: blending = %v, want %v", seq, i, got, st.WantBlend)
	}
	if got := m.Progress(); math.Abs(got-st.WantProg) >= epsMachine {
		t.Errorf("%s step %d: progress = %.17g, want %.17g", seq, i, got, st.WantProg)
	}
	fw, tw := m.Weights()
	if math.Abs(fw-st.WantFromW) >= epsMachine || math.Abs(tw-st.WantToW) >= epsMachine {
		t.Errorf("%s step %d: weights = (%.17g,%.17g), want (%.17g,%.17g)",
			seq, i, fw, tw, st.WantFromW, st.WantToW)
	}
}

// A: switches blend with the frozen durations and weights.
func TestStateMachineSwitchFromCases(t *testing.T) {
	f := loadMachineCases(t)
	for _, seq := range f.Sequences {
		m := buildMachine(t, f)
		if err := m.Start(seq.Start); err != nil {
			t.Fatalf("%s Start: %v", seq.Name, err)
		}
		for i, st := range seq.Steps {
			if st.Request != "" {
				err := m.Request(st.Request)
				if st.WantErr == "" {
					if err != nil {
						t.Errorf("%s step %d: Request(%q) = %v, want nil",
							seq.Name, i, st.Request, err)
					}
				} else if core.CodeOf(err).String() != st.WantErr {
					t.Errorf("%s step %d: Request(%q) code = %v, want %q",
						seq.Name, i, st.Request, core.CodeOf(err), st.WantErr)
				}
			}
			if st.DtMs != 0 || st.Request == "" {
				m.Update(core.Milliseconds(st.DtMs))
			}
			checkMachineStep(t, m, seq.Name, i, st)
		}
	}
}

// B: empty, unknown, and illegal switches error without crashing.
func TestStateMachineEdgesNoCrash(t *testing.T) {
	f := loadMachineCases(t)

	var nilM *Machine
	if err := nilM.AddState(State{Name: "x"}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddState code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilM.Start("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Start code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilM.Request("x"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Request code = %v, want invalid-arg", core.CodeOf(err))
	}
	if nilM.Current() != "" || nilM.From() != "" || nilM.To() != "" || nilM.Blending() {
		t.Error("nil machine reads non-zero")
	}
	if fw, tw := nilM.Weights(); fw != 1 || tw != 0 {
		t.Errorf("nil Weights = (%v,%v), want (1,0)", fw, tw)
	}
	if nilM.Update(16) {
		t.Error("nil Update reported blending")
	}

	m := NewMachine()
	badStates := []State{
		{Name: ""},
		{Name: "idle", CanTo: []string{""}},
		{Name: "idle", Blend: -1},
	}
	for i, s := range badStates {
		if err := m.AddState(s); err == nil {
			t.Errorf("bad state[%d] %+v: want error", i, s)
		} else if core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("bad state[%d] code = %v, want invalid-arg", i, core.CodeOf(err))
		}
	}
	if m.StateCount() != 0 {
		t.Errorf("bad AddState stored %d states, want 0", m.StateCount())
	}
	if err := m.Request("run"); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("Request before Start code = %v, want invalid-arg", core.CodeOf(err))
	}

	m = buildMachine(t, f)
	if err := m.AddState(State{Name: "idle"}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("dup AddState code = %v, want invalid-arg", core.CodeOf(err))
	}
	if m.StateCount() != len(f.States) {
		t.Errorf("dup AddState changed count to %d", m.StateCount())
	}
	if err := m.Start(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Start code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := m.Start("ghost"); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Start code = %v, want not-found", core.CodeOf(err))
	}
	if err := m.Start("jump"); err != nil {
		t.Fatalf("Start jump: %v", err)
	}
	before := m.Current()
	illegal := []struct {
		to   string
		code core.Code
	}{
		{"run", core.CodeInvalidArg},
		{"ghost", core.CodeNotFound},
		{"", core.CodeInvalidArg},
	}
	for _, c := range illegal {
		if err := m.Request(c.to); core.CodeOf(err) != c.code {
			t.Errorf("Request(%q) code = %v, want %v", c.to, core.CodeOf(err), c.code)
		}
		if m.Current() != before || m.Blending() || m.From() != before {
			t.Errorf("Request(%q) moved the machine off %q", c.to, before)
		}
	}
	if m.Update(0) || m.Update(-16) {
		t.Error("settled Update(0/-16) reported blending")
	}

	// Same-state requests are silent no-ops, even mid-blend.
	if err := m.Start("idle"); err != nil {
		t.Fatalf("Start idle: %v", err)
	}
	if err := m.Request("idle"); err != nil {
		t.Errorf("same-state Request = %v, want nil", err)
	}
	if err := m.Request("run"); err != nil {
		t.Fatalf("Request run: %v", err)
	}
	if err := m.Request("run"); err != nil {
		t.Errorf("mid-blend same-target Request = %v, want nil", err)
	}
	if !m.Blending() {
		t.Error("mid-blend no-op Request settled the blend")
	}
}

// C: the same script replays bitwise identically on two machines, and the
// library never aliases caller memory. Bitwise replay is the both-sides
// evidence for this pure-math package: two backends fed the same numbers
// must combine the same weights.
func TestStateMachineBoundaryIdentical(t *testing.T) {
	f := loadMachineCases(t)
	snap := func(m *Machine) []any {
		fw, tw := m.Weights()
		return []any{m.Current(), m.From(), m.To(), m.Blending(), m.Progress(), fw, tw}
	}
	same := func(a, b []any) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	for _, seq := range f.Sequences {
		a, b := buildMachine(t, f), buildMachine(t, f)
		if err := a.Start(seq.Start); err != nil {
			t.Fatalf("%s Start a: %v", seq.Name, err)
		}
		if err := b.Start(seq.Start); err != nil {
			t.Fatalf("%s Start b: %v", seq.Name, err)
		}
		for i, st := range seq.Steps {
			var ea, eb error
			if st.Request != "" {
				ea, eb = a.Request(st.Request), b.Request(st.Request)
				if (ea == nil) != (eb == nil) || core.CodeOf(ea) != core.CodeOf(eb) {
					t.Fatalf("%s step %d: error diverged: %v vs %v", seq.Name, i, ea, eb)
				}
			}
			if st.DtMs != 0 || st.Request == "" {
				ra, rb := a.Update(core.Milliseconds(st.DtMs)), b.Update(core.Milliseconds(st.DtMs))
				if ra != rb {
					t.Fatalf("%s step %d: Update diverged: %v vs %v", seq.Name, i, ra, rb)
				}
			}
			if !same(snap(a), snap(b)) {
				t.Fatalf("%s step %d: replay diverged: %+v vs %+v", seq.Name, i, snap(a), snap(b))
			}
		}
	}

	// The library copies CanTo; later caller writes change nothing.
	m := NewMachine()
	can := []string{"run", "jump"}
	if err := m.AddState(State{Name: "idle", CanTo: can, Blend: 200}); err != nil {
		t.Fatalf("AddState: %v", err)
	}
	can[0] = "ghost"
	if !m.Has("idle") || m.Has("ghost") {
		t.Error("CanTo alias leaked caller mutation into the machine")
	}
	got := m.States()
	got[0] = "ghost"
	if !m.Has("idle") {
		t.Error("States reused its buffer across calls")
	}
	// Boundary crossings are lossless both ways.
	v := core.V2(12.5, -7.25)
	if back := core.Vec2FromRenderPoint(v.ToRenderPoint()); back != v {
		t.Errorf("Vec2 boundary round trip = %+v, want %+v", back, v)
	}
	mm := core.Mat2D{A: 1, B: 2, C: 3, D: 4, E: 5, F: 6}
	if back := core.Mat2DFromRenderMatrix(mm.ToRenderMatrix()); back != mm {
		t.Errorf("Mat2D boundary round trip = %+v, want %+v", back, mm)
	}
}

// D: a hundred switches with a measured cost.
func TestStateMachinePerfHundred(t *testing.T) {
	// Synthetic load only (no golden): frozen blends stay in
	// statemachine_cases.json.
	f := loadMachineCases(t)
	m := buildMachine(t, f)
	if err := m.Start("idle"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	next := map[string]string{"idle": "run", "run": "jump", "jump": "idle"}
	const switches = 100
	start := time.Now()
	for i := 0; i < switches; i++ {
		to := next[m.Current()]
		if err := m.Request(to); err != nil {
			t.Fatalf("switch %d to %q: %v", i, to, err)
		}
		for guard := 0; m.Blending() && guard < 100; guard++ {
			m.Update(core.Milliseconds(16))
		}
		if m.Blending() {
			t.Fatalf("switch %d to %q never settled", i, to)
		}
	}
	el := time.Since(start)
	t.Logf("statemachine-100: %d settled switches in %v (%.1f ns/switch)", switches, el, float64(el.Nanoseconds())/float64(switches))
	if m.Current() != "run" {
		t.Errorf("100 switches from idle land on %q, want run", m.Current())
	}
}

// E: long runs never wedge and replays stay identical.
func TestStateMachineLongRunStable(t *testing.T) {
	f := loadMachineCases(t)
	mk := func(t *testing.T) *Machine {
		t.Helper()
		m := buildMachine(t, f)
		if err := m.Start("idle"); err != nil {
			t.Fatalf("Start: %v", err)
		}
		return m
	}
	// Same request plus time stream replays bitwise identically.
	const steps = 200000
	a, b := mk(t), mk(t)
	order := []string{"run", "jump", "idle"}
	oi := 0
	for i := 0; i < steps; i++ {
		if i%50 == 0 {
			want := order[oi%len(order)]
			oi++
			ea, eb := a.Request(want), b.Request(want)
			if core.CodeOf(ea) != core.CodeOf(eb) {
				t.Fatalf("rep %d Request(%q) diverged: %v vs %v", i, want, ea, eb)
			}
			if ea != nil {
				// Only the jump->run hop is illegal; anything else must go.
				if !(a.Current() == "jump" && want == "run") {
					t.Fatalf("rep %d Request(%q) = %v from %q", i, want, ea, a.Current())
				}
			}
		}
		ra, rb := a.Update(core.Milliseconds(16)), b.Update(core.Milliseconds(16))
		if ra != rb || a.Current() != b.Current() || a.Progress() != b.Progress() {
			t.Fatalf("rep %d diverged: %q/%v vs %q/%v", i, a.Current(), ra, b.Current(), rb)
		}
		fa, ta := a.Weights()
		fb, tb := b.Weights()
		if fa != fb || ta != tb {
			t.Fatalf("rep %d weights diverged", i)
		}
	}
	// Illegal storms never wedge the machine or move the error codes.
	for i := 0; i < 500; i++ {
		if err := a.Request("run"); a.Current() == "jump" {
			if core.CodeOf(err) != core.CodeInvalidArg {
				t.Fatalf("bad %d code = %v", i, core.CodeOf(err))
			}
		}
		if err := a.Request("ghost"); core.CodeOf(err) != core.CodeNotFound {
			t.Fatalf("bad %d ghost code = %v", i, core.CodeOf(err))
		}
		a.Update(core.Milliseconds(16))
	}
	// Start parks both runs back on the same action exactly.
	if err := a.Start("idle"); err != nil {
		t.Fatalf("re-Start a: %v", err)
	}
	if err := b.Start("idle"); err != nil {
		t.Fatalf("re-Start b: %v", err)
	}
	if a.Current() != b.Current() || a.Blending() || b.Blending() {
		t.Fatalf("re-Start diverged: %+v vs %+v", a.Current(), b.Current())
	}
	if p := a.Progress(); p != 1 {
		t.Fatalf("re-Start progress = %v, want 1", p)
	}
}

// F: the frozen sequences plus shape assertions stand in until the
// game_anim --case=fsm window verdict lands below. Weights always
// partition one blend unit, progress stays in [0,1], settled means
// (1,0), and zero-blend cuts synchronously.
func TestStateMachineOffscreenGolden(t *testing.T) {
	f := loadMachineCases(t)
	if len(f.States) != 3 {
		t.Fatalf("states = %d, want 3 (idle/run/jump)", len(f.States))
	}
	byName := map[string]smStateDef{}
	for _, s := range f.States {
		byName[s.Name] = s
	}
	if byName["idle"].BlendMs != 200 || byName["run"].BlendMs != 100 || byName["jump"].BlendMs != 0 {
		t.Errorf("blend durations = %d/%d/%d, want 200/100/0",
			byName["idle"].BlendMs, byName["run"].BlendMs, byName["jump"].BlendMs)
	}
	idle, run, jump := byName["idle"], byName["run"], byName["jump"]
	has := func(s smStateDef, want string) bool {
		for _, n := range s.CanTo {
			if n == want {
				return true
			}
		}
		return false
	}
	if !(has(idle, "run") && has(idle, "jump") && has(run, "idle") && has(run, "jump") && has(jump, "idle")) {
		t.Errorf("transitions moved: idle=%q run=%q jump=%q", idle.CanTo, run.CanTo, jump.CanTo)
	}
	if has(jump, "run") {
		t.Error("jump can reach run, want terminal except idle")
	}
	for _, seq := range f.Sequences {
		m := buildMachine(t, f)
		if err := m.Start(seq.Start); err != nil {
			t.Fatalf("%s Start: %v", seq.Name, err)
		}
		var lastProg float64
		wasBlending := false
		for i, st := range seq.Steps {
			if st.Request != "" {
				_ = m.Request(st.Request)
			}
			if st.DtMs != 0 || st.Request == "" {
				m.Update(core.Milliseconds(st.DtMs))
			}
			fw, tw := m.Weights()
			if math.Abs(fw+tw-1) >= epsMachine {
				t.Errorf("%s step %d: weights sum %v, want 1", seq.Name, i, fw+tw)
			}
			if p := m.Progress(); p < 0 || p > 1 {
				t.Errorf("%s step %d: progress %v outside [0,1]", seq.Name, i, p)
			}
			if !m.Blending() {
				if fw != 1 || tw != 0 || m.Progress() != 1 || m.To() != "" {
					t.Errorf("%s step %d: settled state not (1,0)/p1/empty-to", seq.Name, i)
				}
			} else if m.To() != m.Current() || m.From() == m.To() {
				t.Errorf("%s step %d: blending from=%q to=%q current=%q inconsistent",
					seq.Name, i, m.From(), m.To(), m.Current())
			}
			if wasBlending && m.Blending() && m.To() == m.From() {
				t.Errorf("%s step %d: blend target equals origin", seq.Name, i)
			}
			if m.Blending() && st.Request == "" && st.DtMs > 0 && m.Progress() < lastProg {
				t.Errorf("%s step %d: progress went back %.4g -> %.4g",
					seq.Name, i, lastProg, m.Progress())
			}
			lastProg, wasBlending = m.Progress(), m.Blending()
		}
	}
	// Zero-blend cuts synchronously: jump->idle never reports blending.
	m := buildMachine(t, f)
	if err := m.Start("jump"); err != nil {
		t.Fatalf("Start jump: %v", err)
	}
	if err := m.Request("idle"); err != nil {
		t.Fatalf("Request idle: %v", err)
	}
	if m.Blending() || m.Current() != "idle" {
		t.Errorf("zero-blend cut left blending=%v current=%q", m.Blending(), m.Current())
	}
	// Window intent: the real window game_anim --case=fsm (switch crisp,
	// blend bar live) lands with W7; the frozen sequences above are the
	// numbers both backends share.
	if _, err := os.Stat(filepath.Join("..", "..", "examples", "game_anim")); err == nil {
		t.Log("game_anim window exists; W7 should wire --case=fsm to this machine")
	} else {
		t.Log("game_anim window absent; offscreen statemachine_cases.json stands in (pure math, W7 intent game_anim --case=fsm)")
	}
}
