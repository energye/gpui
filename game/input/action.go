package input

import (
	"math"
	"sort"

	"github.com/energye/gpui/game/core"
)

// Source names which hardware feeds one binding.
type Source int

const (
	// SourceKey is one digital key code (Code > 0).
	SourceKey Source = 0
	// SourcePadButton is one pad button (pad >= 0 or DeviceAny, Code >= 0).
	SourcePadButton Source = 1
	// SourcePadAxis is one side of one stick axis (side +1/-1).
	SourcePadAxis Source = 2
	// SourcePinch is one pinch direction (spread +1, close -1).
	SourcePinch Source = 3
)

// DeviceAny matches any pad. Specific pads use their index (>= 0).
const DeviceAny = -1

var sourceNames = []string{"key", "pad_button", "pad_axis", "pinch"}

// Valid reports whether s names a frozen source.
func Valid(s Source) bool { return s >= SourceKey && int(s) < len(sourceNames) }

// String returns the frozen name of s, or "unknown" for bad sources.
func (s Source) String() string {
	if !Valid(s) {
		return "unknown"
	}
	return sourceNames[int(s)]
}

// ParseSource looks s up by frozen name. Empty and unknown names return a
// core InvalidArg error and never guess a source.
func ParseSource(name string) (Source, error) {
	for i, n := range sourceNames {
		if n == name {
			return Source(i), nil
		}
	}
	return SourceKey, core.InvalidArg("input.ParseSource", name)
}

// AllSources returns every frozen source in declaration order.
func AllSources() []Source {
	out := make([]Source, len(sourceNames))
	for i := range sourceNames {
		out[i] = Source(i)
	}
	return out
}

// Binding ties one hardware input to an action. Device is DeviceAny or a
// pad index; key and pinch require DeviceAny. Code is the key code
// (> 0), button/axis index (>= 0), or 0 for pinch (ignored). Sign is 0
// for digital sources and +1/-1 for axis/pinch sides.
type Binding struct {
	Source Source
	Device int
	Code   int
	Sign   int
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// clampUnit pins v to [-1,1]. Stick values and the pinch span share it.
func clampUnit(v float64) float64 {
	if v > 1 {
		return 1
	}
	if v < -1 {
		return -1
	}
	return v
}

// makeBinding validates a literal built by the four constructors.
func makeBinding(b Binding) (Binding, error) {
	if err := b.validate(); err != nil {
		return Binding{}, err
	}
	return b, nil
}

func (b Binding) validate() error {
	if !Valid(b.Source) {
		return core.InvalidArg("input.Binding", "source")
	}
	if b.Device < DeviceAny {
		return core.InvalidArg("input.Binding", "device")
	}
	switch b.Source {
	case SourceKey:
		if b.Device != DeviceAny {
			return core.InvalidArg("input.Binding", "device")
		}
		if b.Code <= 0 {
			return core.InvalidArg("input.Binding", "code")
		}
		if b.Sign != 0 {
			return core.InvalidArg("input.Binding", "sign")
		}
	case SourcePadButton:
		if b.Code < 0 {
			return core.InvalidArg("input.Binding", "code")
		}
		if b.Sign != 0 {
			return core.InvalidArg("input.Binding", "sign")
		}
	case SourcePadAxis:
		if b.Code < 0 {
			return core.InvalidArg("input.Binding", "code")
		}
		if b.Sign != 1 && b.Sign != -1 {
			return core.InvalidArg("input.Binding", "sign")
		}
	case SourcePinch:
		if b.Device != DeviceAny {
			return core.InvalidArg("input.Binding", "device")
		}
		if b.Code != 0 {
			return core.InvalidArg("input.Binding", "code")
		}
		if b.Sign != 1 && b.Sign != -1 {
			return core.InvalidArg("input.Binding", "sign")
		}
	}
	return nil
}

// NewKeyBinding builds a digital key binding. Code must be > 0.
func NewKeyBinding(code int) (Binding, error) {
	return makeBinding(Binding{Source: SourceKey, Device: DeviceAny, Code: code})
}

// NewPadButtonBinding builds a pad button binding. Device is DeviceAny or
// a pad index (>= 0); button index must be >= 0.
func NewPadButtonBinding(device, button int) (Binding, error) {
	return makeBinding(Binding{Source: SourcePadButton, Device: device, Code: button})
}

// NewPadAxisBinding builds one side of a stick axis. Device is DeviceAny
// or a pad index; axis index must be >= 0; sign must be +1 (positive side)
// or -1 (negative side).
func NewPadAxisBinding(device, axis, sign int) (Binding, error) {
	return makeBinding(Binding{Source: SourcePadAxis, Device: device, Code: axis, Sign: sign})
}

// NewPinchBinding builds one pinch direction: +1 spread (fingers apart),
// -1 close (fingers together).
func NewPinchBinding(sign int) (Binding, error) {
	return makeBinding(Binding{Source: SourcePinch, Device: DeviceAny, Sign: sign})
}

func validDeadzone(dz float64) bool { return finite(dz) && dz >= 0 && dz < 1 }

func validIntensity(v float64) bool { return finite(v) && v >= 0 && v <= 1 }

type action struct {
	deadzone float64
	bindings []Binding
}

type padButton struct {
	pad  int
	code int
}

type padAxis struct {
	pad  int
	code int
}

type rumbleEntry struct {
	weak      float64
	strong    float64
	remaining core.Duration
}

// Map owns action names, their bindings, live hardware state, pinch
// accumulation, and rumble requests. It draws nothing and makes no sound.
type Map struct {
	actions    map[string]*action
	keys       map[int]bool
	padButtons map[padButton]bool
	padAxes    map[padAxis]float64
	pinch      float64
	rumbles    map[int]*rumbleEntry
}

// lookup resolves a registered action. Nil maps and empty names are
// InvalidArg, missing names are NotFound; the map stays untouched.
func (m *Map) lookup(name, op string) (*action, error) {
	if m == nil {
		return nil, core.InvalidArg(op, "map")
	}
	if name == "" {
		return nil, core.InvalidArg(op, "name")
	}
	a, ok := m.actions[name]
	if !ok {
		return nil, core.NotFound(op, name)
	}
	return a, nil
}

// NewMap builds an empty action map.
func NewMap() *Map {
	return &Map{
		actions:    map[string]*action{},
		keys:       map[int]bool{},
		padButtons: map[padButton]bool{},
		padAxes:    map[padAxis]float64{},
		rumbles:    map[int]*rumbleEntry{},
	}
}

// AddAction registers name with deadzone in [0,1). Empty names, duplicate
// names, and deadzones outside [0,1) return core InvalidArg and store
// nothing.
func (m *Map) AddAction(name string, deadzone float64) error {
	if m == nil {
		return core.InvalidArg("input.AddAction", "map")
	}
	if name == "" {
		return core.InvalidArg("input.AddAction", "name")
	}
	if !validDeadzone(deadzone) {
		return core.InvalidArg("input.AddAction", "deadzone")
	}
	if _, dup := m.actions[name]; dup {
		return core.InvalidArg("input.AddAction", "name")
	}
	m.actions[name] = &action{deadzone: deadzone}
	return nil
}

// Has reports whether name is registered. Nil maps and empty names report
// false.
func (m *Map) Has(name string) bool {
	if m == nil || name == "" {
		return false
	}
	_, ok := m.actions[name]
	return ok
}

// RemoveAction deletes name. Empty names are InvalidArg, missing names are
// NotFound; either way the remaining actions stay untouched.
func (m *Map) RemoveAction(name string) error {
	if _, err := m.lookup(name, "input.RemoveAction"); err != nil {
		return err
	}
	delete(m.actions, name)
	return nil
}

// Actions returns every registered name in sorted order. Nil maps return
// nil.
func (m *Map) Actions() []string {
	if m == nil {
		return nil
	}
	out := make([]string, 0, len(m.actions))
	for n := range m.actions {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ActionCount returns how many actions are registered. Nil maps return 0.
func (m *Map) ActionCount() int {
	if m == nil {
		return 0
	}
	return len(m.actions)
}

// SetDeadzone retunes the stick drift gate for name. Bad deadzones are
// core InvalidArg and leave the old value untouched.
func (m *Map) SetDeadzone(name string, dz float64) error {
	a, err := m.lookup(name, "input.SetDeadzone")
	if err != nil {
		return err
	}
	if !validDeadzone(dz) {
		return core.InvalidArg("input.SetDeadzone", "deadzone")
	}
	a.deadzone = dz
	return nil
}

// Deadzone returns the deadzone for name. Missing names and nil maps
// report ok=false.
func (m *Map) Deadzone(name string) (float64, bool) {
	if m == nil || name == "" {
		return 0, false
	}
	a, ok := m.actions[name]
	if !ok {
		return 0, false
	}
	return a.deadzone, true
}

// Bind adds one hardware input to name (remap adds, never replaces).
// Exact duplicate bindings return core InvalidArg and store nothing.
func (m *Map) Bind(name string, b Binding) error {
	a, err := m.lookup(name, "input.Bind")
	if err != nil {
		return err
	}
	if err := b.validate(); err != nil {
		return err
	}
	for _, have := range a.bindings {
		if have == b {
			return core.InvalidArg("input.Bind", "binding")
		}
	}
	a.bindings = append(a.bindings, b)
	return nil
}

// Unbind removes one hardware input from name. A binding that is not
// present returns core NotFound and changes nothing.
func (m *Map) Unbind(name string, b Binding) error {
	a, err := m.lookup(name, "input.Unbind")
	if err != nil {
		return err
	}
	if err := b.validate(); err != nil {
		return err
	}
	for i, have := range a.bindings {
		if have == b {
			a.bindings = append(a.bindings[:i], a.bindings[i+1:]...)
			return nil
		}
	}
	return core.NotFound("input.Unbind", "binding")
}

// Rebind replaces the whole binding list of name (remap). An empty or nil
// list clears the action and stays silent. Bad entries and duplicates
// inside the new list return core InvalidArg and leave the old list
// untouched.
func (m *Map) Rebind(name string, bindings []Binding) error {
	a, err := m.lookup(name, "input.Rebind")
	if err != nil {
		return err
	}
	seen := map[Binding]bool{}
	for _, b := range bindings {
		if err := b.validate(); err != nil {
			return err
		}
		if seen[b] {
			return core.InvalidArg("input.Rebind", "binding")
		}
		seen[b] = true
	}
	cp := make([]Binding, len(bindings))
	copy(cp, bindings)
	a.bindings = cp
	return nil
}

// ClearBindings removes every hardware input from name. The action stays
// registered and silent.
func (m *Map) ClearBindings(name string) error {
	a, err := m.lookup(name, "input.ClearBindings")
	if err != nil {
		return err
	}
	a.bindings = nil
	return nil
}

// Bindings returns a copy of the binding list for name. Writing the result
// cannot change the map.
func (m *Map) Bindings(name string) ([]Binding, error) {
	a, err := m.lookup(name, "input.Bindings")
	if err != nil {
		return nil, err
	}
	out := make([]Binding, len(a.bindings))
	copy(out, a.bindings)
	return out, nil
}

// SetKey feeds one digital key state. Codes must be > 0.
func (m *Map) SetKey(code int, pressed bool) error {
	if m == nil {
		return core.InvalidArg("input.SetKey", "map")
	}
	if code <= 0 {
		return core.InvalidArg("input.SetKey", "code")
	}
	if pressed {
		m.keys[code] = true
	} else {
		delete(m.keys, code)
	}
	return nil
}

// SetPadButton feeds one pad button state. Pad and button must be >= 0.
func (m *Map) SetPadButton(pad, button int, pressed bool) error {
	if m == nil {
		return core.InvalidArg("input.SetPadButton", "map")
	}
	if pad < 0 || button < 0 {
		return core.InvalidArg("input.SetPadButton", "pad")
	}
	k := padButton{pad: pad, code: button}
	if pressed {
		m.padButtons[k] = true
	} else {
		delete(m.padButtons, k)
	}
	return nil
}

// SetPadAxis feeds one stick axis value in [-1,1]. Out-of-range finite
// values clamp to the ends; non-finite values return core InvalidArg and
// leave the old value untouched.
func (m *Map) SetPadAxis(pad, axis int, value float64) error {
	if m == nil {
		return core.InvalidArg("input.SetPadAxis", "map")
	}
	if pad < 0 || axis < 0 {
		return core.InvalidArg("input.SetPadAxis", "pad")
	}
	if !finite(value) {
		return core.InvalidArg("input.SetPadAxis", "value")
	}
	m.padAxes[padAxis{pad: pad, code: axis}] = clampUnit(value)
	return nil
}

// AddPinchDelta feeds one two-finger zoom step: the multiplicative factor
// since the last step (1.2 spread apart, 0.8 closed together). The map
// accumulates (scaleDelta-1) clamped to [-1,1]; spread drives +1 bindings,
// close drives -1 bindings. Non-finite and non-positive deltas return core
// InvalidArg and change nothing.
func (m *Map) AddPinchDelta(scaleDelta float64) error {
	if m == nil {
		return core.InvalidArg("input.AddPinchDelta", "map")
	}
	if !finite(scaleDelta) || scaleDelta <= 0 {
		return core.InvalidArg("input.AddPinchDelta", "scale")
	}
	m.pinch = clampUnit(m.pinch + scaleDelta - 1)
	return nil
}

// Pinch returns the accumulated pinch span in [-1,1]: positive spread,
// negative close, 0 no gesture. Nil maps return 0.
func (m *Map) Pinch() float64 {
	if m == nil {
		return 0
	}
	return m.pinch
}

// ClearPinch resets the accumulated pinch span to 0. Nil maps do nothing.
func (m *Map) ClearPinch() {
	if m == nil {
		return
	}
	m.pinch = 0
}

// ResetInputs clears every live hardware state (keys, pad buttons, pad
// axes, pinch). Bindings, deadzones, and rumble requests are kept.
func (m *Map) ResetInputs() {
	if m == nil {
		return
	}
	m.keys = map[int]bool{}
	m.padButtons = map[padButton]bool{}
	m.padAxes = map[padAxis]float64{}
	m.pinch = 0
}

func sideRaw(v float64, sign int) float64 {
	if sign <= 0 {
		v = -v
	}
	if v <= 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// applyDeadzone remaps raw through dz. Callers guarantee raw in [0,1]
// and dz in [0,1), so the result stays in [0,1] with no extra guard.
func applyDeadzone(raw, dz float64) float64 {
	if raw <= dz {
		return 0
	}
	return (raw - dz) / (1 - dz)
}

func (m *Map) bindingRaw(b Binding) float64 {
	switch b.Source {
	case SourceKey:
		if m.keys[b.Code] {
			return 1
		}
		return 0
	case SourcePadButton:
		if b.Device == DeviceAny {
			// The live map only holds pressed buttons; release deletes.
			for k := range m.padButtons {
				if k.code == b.Code {
					return 1
				}
			}
			return 0
		}
		if m.padButtons[padButton{pad: b.Device, code: b.Code}] {
			return 1
		}
		return 0
	case SourcePadAxis:
		if b.Device == DeviceAny {
			best := 0.0
			for k, v := range m.padAxes {
				if k.code == b.Code {
					if c := sideRaw(v, b.Sign); c > best {
						best = c
					}
				}
			}
			return best
		}
		return sideRaw(m.padAxes[padAxis{pad: b.Device, code: b.Code}], b.Sign)
	case SourcePinch:
		return sideRaw(m.pinch, b.Sign)
	}
	return 0
}

// Strength returns the deadzoned action strength in [0,1]: the maximum
// over all bindings. Unbound actions return 0 with no error. The result
// is always finite, never NaN.
func (m *Map) Strength(name string) (float64, error) {
	a, err := m.lookup(name, "input.Strength")
	if err != nil {
		return 0, err
	}
	best := 0.0
	for _, b := range a.bindings {
		if v := applyDeadzone(m.bindingRaw(b), a.deadzone); v > best {
			best = v
		}
	}
	return best, nil
}

// Pressed reports whether the deadzoned strength is above 0.
func (m *Map) Pressed(name string) (bool, error) {
	s, err := m.Strength(name)
	if err != nil {
		return false, err
	}
	return s > 0, nil
}

// Vector combines four actions into one stick vector: x grows with posX
// and shrinks with negX, y grows with posY and shrinks with negY. Lengths
// above 1 normalize to 1 so diagonals never run faster. Missing names
// return core NotFound with a zero vector, never a guessed direction.
func (m *Map) Vector(negX, posX, negY, posY string) (core.Vec2, error) {
	if m == nil {
		return core.Vec2{}, core.InvalidArg("input.Vector", "map")
	}
	names := []string{negX, posX, negY, posY}
	for _, n := range names {
		if n == "" {
			return core.Vec2{}, core.InvalidArg("input.Vector", "name")
		}
	}
	got := make([]float64, 4)
	for i, n := range names {
		v, err := m.Strength(n)
		if err != nil {
			return core.Vec2{}, err
		}
		got[i] = v
	}
	v := core.V2(got[1]-got[0], got[3]-got[2])
	if l := v.Length(); l > 1 {
		v = v.Div(l)
	}
	return v, nil
}

// StartRumble requests pad vibration: weak (high-frequency) and strong
// (low-frequency) intensities in [0,1] for dur. It only records the
// request; the caller drives the hardware. Overwrites any running request
// on the same pad.
func (m *Map) StartRumble(pad int, weak, strong float64, dur core.Duration) error {
	if m == nil {
		return core.InvalidArg("input.StartRumble", "map")
	}
	if pad < 0 {
		return core.InvalidArg("input.StartRumble", "pad")
	}
	if !validIntensity(weak) {
		return core.InvalidArg("input.StartRumble", "weak")
	}
	if !validIntensity(strong) {
		return core.InvalidArg("input.StartRumble", "strong")
	}
	if dur <= 0 {
		return core.InvalidArg("input.StartRumble", "dur")
	}
	m.rumbles[pad] = &rumbleEntry{weak: weak, strong: strong, remaining: dur}
	return nil
}

// StopRumble cancels the rumble request on pad. Unknown pads are a silent
// no-op.
func (m *Map) StopRumble(pad int) error {
	if m == nil {
		return core.InvalidArg("input.StopRumble", "map")
	}
	if pad < 0 {
		return core.InvalidArg("input.StopRumble", "pad")
	}
	delete(m.rumbles, pad)
	return nil
}

// UpdateRumbles advances every rumble clock by dt and expires finished
// requests. dt at or below zero advances nothing. Nil maps do nothing.
func (m *Map) UpdateRumbles(dt core.Duration) {
	if m == nil || dt <= 0 {
		return
	}
	for pad, e := range m.rumbles {
		e.remaining = e.remaining.Sub(dt)
		if e.remaining <= 0 {
			delete(m.rumbles, pad)
		}
	}
}

// RumbleLevels returns the requested intensities for pad. Inactive pads
// (and nil maps, negative pads) return 0,0,false and never panic.
func (m *Map) RumbleLevels(pad int) (weak, strong float64, active bool) {
	if m == nil || pad < 0 {
		return 0, 0, false
	}
	e, ok := m.rumbles[pad]
	if !ok {
		return 0, 0, false
	}
	return e.weak, e.strong, true
}

// RumbleActive reports whether pad has a live rumble request.
func (m *Map) RumbleActive(pad int) bool {
	_, _, ok := m.RumbleLevels(pad)
	return ok
}
