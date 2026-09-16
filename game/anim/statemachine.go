package anim

import (
	"sort"

	"github.com/energye/gpui/game/core"
)

// State is one named action in the blend state machine: idle, run, jump
// each own one. Name is caller-owned; the engine never hardcodes user
// text. CanTo lists the legal switch targets (empty means terminal).
// Blend is how long leaving this state blends toward the next one;
// zero cuts instantly. Only core numbers are used.
type State struct {
	Name  string
	CanTo []string
	Blend core.Duration
}

// Machine blends between named actions with a linear crossfade weight.
// It only computes numbers; the caller feeds Weights into the frozen
// skeleton blend entry (BlendReplace) or the flipbook Play picker and
// draws with the existing render draws. Time is integer milliseconds,
// so long runs never drift.
type Machine struct {
	states map[string]State
	cur    string
	hasCur bool
	from   string
	to     string
	blend  bool
	el     core.Duration
	dur    core.Duration
}

// NewMachine builds an empty state library. States arrive via AddState,
// the start state via Start.
func NewMachine() *Machine {
	return &Machine{states: map[string]State{}}
}

func containsName(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

// AddState stores one action. Name must be non-empty and unique, Blend
// must not be negative, every CanTo entry must be non-empty. Bad states
// return core InvalidArg and store nothing; the CanTo list is copied so
// later writes by the caller cannot change the library. Forward targets
// (added later) are allowed here; an unknown Request target still
// reports NotFound.
func (m *Machine) AddState(s State) error {
	if m == nil {
		return core.InvalidArg("anim.AddState", "machine")
	}
	if s.Name == "" {
		return core.InvalidArg("anim.AddState", "name")
	}
	if _, dup := m.states[s.Name]; dup {
		return core.InvalidArg("anim.AddState", "name")
	}
	if s.Blend < 0 {
		return core.InvalidArg("anim.AddState", "blend")
	}
	for _, n := range s.CanTo {
		if n == "" {
			return core.InvalidArg("anim.AddState", "canTo")
		}
	}
	cp := make([]string, len(s.CanTo))
	copy(cp, s.CanTo)
	m.states[s.Name] = State{Name: s.Name, CanTo: cp, Blend: s.Blend}
	return nil
}

// Has reports whether the named action exists.
func (m *Machine) Has(name string) bool {
	if m == nil {
		return false
	}
	_, ok := m.states[name]
	return ok
}

// StateCount returns how many actions the library holds.
func (m *Machine) StateCount() int {
	if m == nil {
		return 0
	}
	return len(m.states)
}

// States returns every action name, sorted. The slice is a fresh copy.
func (m *Machine) States() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.states))
	for name := range m.states {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Start parks the machine on the named action with no blend in flight.
// Empty names return core InvalidArg, unknown names core NotFound;
// either way the previous current (if any) stays untouched.
func (m *Machine) Start(name string) error {
	if m == nil {
		return core.InvalidArg("anim.Start", "machine")
	}
	if name == "" {
		return core.InvalidArg("anim.Start", "name")
	}
	if _, ok := m.states[name]; !ok {
		return core.NotFound("anim.Start", name)
	}
	m.cur = name
	m.hasCur = true
	m.from = name
	m.to = ""
	m.blend = false
	m.el = 0
	m.dur = 0
	return nil
}

// Current returns the action the machine is heading to (the blend target
// while blending, the settled action otherwise), or "" before Start.
func (m *Machine) Current() string {
	if m == nil || !m.hasCur {
		return ""
	}
	return m.cur
}

// From returns the blend origin (the settled action when idle).
// It returns "" before Start.
func (m *Machine) From() string {
	if m == nil || !m.hasCur {
		return ""
	}
	return m.from
}

// To returns the blend target, or "" when no blend is in flight.
func (m *Machine) To() string {
	if m == nil || !m.hasCur {
		return ""
	}
	if !m.blend {
		return ""
	}
	return m.to
}

// Blending reports whether a crossfade is in flight.
func (m *Machine) Blending() bool {
	if m == nil {
		return false
	}
	return m.hasCur && m.blend
}

// Progress returns the blend progress in [0,1]: 0 at request time,
// 1 when settled. A settled machine reports 1; before Start it is 0.
func (m *Machine) Progress() float64 {
	if m == nil || !m.hasCur || !m.blend {
		if m != nil && m.hasCur {
			return 1
		}
		return 0
	}
	if m.dur <= 0 {
		return 1
	}
	p := float64(m.el) / float64(m.dur)
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// Weights returns the linear crossfade pair (fromW, toW) with fromW+toW==1.
// Settled it is (1,0); mid-blend it is (1-p,p). Feed toW into the
// skeleton BlendReplace weight or an alpha lerp.
func (m *Machine) Weights() (fromW, toW float64) {
	if m == nil || !m.hasCur || !m.blend {
		return 1, 0
	}
	p := m.Progress()
	return 1 - p, p
}

// BlendDur returns the in-flight blend length, or 0 when settled.
func (m *Machine) BlendDur() core.Duration {
	if m == nil || !m.blend {
		return 0
	}
	return m.dur
}

// Elapsed returns how far the in-flight blend has advanced, or 0.
func (m *Machine) Elapsed() core.Duration {
	if m == nil || !m.blend {
		return 0
	}
	return m.el
}

// Request asks for the named action and starts (or retargets) the blend.
// Requesting the settled current (or, mid-blend, the blend target) is a
// silent no-op success. Requesting the blend origin mid-blend cancels the
// blend and snaps back. A zero outgoing Blend cuts instantly instead of
// blending. Errors never move the machine: empty names and switches the
// origin state forbids report core InvalidArg, unknown targets report
// core NotFound, and requesting before Start reports core InvalidArg.
func (m *Machine) Request(to string) error {
	if m == nil {
		return core.InvalidArg("anim.Request", "machine")
	}
	if !m.hasCur {
		return core.InvalidArg("anim.Request", "start")
	}
	if to == "" {
		return core.InvalidArg("anim.Request", "name")
	}
	dst, ok := m.states[to]
	_ = dst
	if !ok {
		return core.NotFound("anim.Request", to)
	}
	if !m.blend {
		if to == m.cur {
			return nil
		}
		org := m.states[m.cur]
		if !containsName(org.CanTo, to) {
			return core.InvalidArg("anim.Request", "transition")
		}
		if org.Blend == 0 {
			m.cur = to
			m.from = to
			return nil
		}
		m.from = m.cur
		m.to = to
		m.cur = to
		m.blend = true
		m.el = 0
		m.dur = org.Blend
		return nil
	}
	if to == m.to {
		return nil
	}
	if to == m.from {
		m.cur = m.from
		m.to = ""
		m.blend = false
		m.el = 0
		m.dur = 0
		return nil
	}
	org := m.states[m.to]
	if !containsName(org.CanTo, to) {
		return core.InvalidArg("anim.Request", "transition")
	}
	if org.Blend == 0 {
		m.cur = to
		m.from = to
		m.to = ""
		m.blend = false
		m.el = 0
		m.dur = 0
		return nil
	}
	m.from = m.to
	m.to = to
	m.cur = to
	m.el = 0
	m.dur = org.Blend
	return nil
}

// Update advances the in-flight blend by dt and reports whether a blend
// is still running. dt at or below zero advances nothing. A settled
// machine stays settled and reports false. Reaching (or passing) the
// blend length snaps to the target exactly.
func (m *Machine) Update(dt core.Duration) bool {
	if m == nil || !m.hasCur || !m.blend {
		return false
	}
	if dt <= 0 {
		return true
	}
	m.el += dt
	if m.el >= m.dur {
		m.cur = m.to
		m.from = m.to
		m.to = ""
		m.blend = false
		m.el = 0
		m.dur = 0
		return false
	}
	return true
}
