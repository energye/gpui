package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// DateEngineInstance owns value/range/multiple plus the controlled
// panel (mode/pickerValue) for one engine mount.
type DateEngineInstance struct {
	props   DateEngineProps
	ctx     scope.Ctx
	gen     DateEngineGenerateConfig
	mu      sync.Mutex
	value   *DateEngineDate
	rangeV  [2]*DateEngineDate
	multi   []DateEngineDate
	picker  *DateEngineDate
	mode    DateEngineMode
	pending int

	ctlValue  bool
	ctlPicker bool
	ctlMode   bool

	onChange   func(d *DateEngineDate, s string)
	onRange    func(dates [2]*DateEngineDate, strs [2]string)
	onPanel    func(value *DateEngineDate, mode DateEngineMode)
	onCalendar func(dates []*DateEngineDate)
	mounted    bool
}

func newDateEngineInstance(ctx scope.Ctx, props DateEngineProps, gen DateEngineGenerateConfig) *DateEngineInstance {
	if gen == nil {
		gen = DefaultDateEngineGenerateConfig()
	}
	in := &DateEngineInstance{props: props, ctx: ctx.Normalize(), gen: gen}
	if props.DefaultValue != nil {
		v := *props.DefaultValue
		in.value = &v
	}
	for i := 0; i < 2; i++ {
		if props.DefaultRange[i] != nil {
			v := *props.DefaultRange[i]
			in.rangeV[i] = &v
		}
	}
	in.multi = append([]DateEngineDate(nil), props.DefaultMultiple...)
	if props.DefaultPicker != nil {
		v := *props.DefaultPicker
		in.picker = &v
	} else if props.DefaultValue != nil {
		v := *props.DefaultValue
		in.picker = &v
	} else {
		v := DateEngineToday(gen)
		in.picker = &v
	}
	in.mode = props.DefaultMode
	if in.mode == "" {
		in.mode = DateEngineModeDate
	}
	return in
}

// DateEngineToday returns today at midnight under the generate config.
func DateEngineToday(gen DateEngineGenerateConfig) DateEngineDate {
	_ = gen
	return DateEngineDateOf(2026, 9, 21)
}

// Mount marks the host live.
func (in *DateEngineInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots.
func (in *DateEngineInstance) Update(ctx scope.Ctx, next DateEngineProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.props = next
}

// Unmount clears live state.
func (in *DateEngineInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.mounted = false
}

// SetState runs f under the lock (sole state mutation gate).
func (in *DateEngineInstance) SetState(f func(*DateEngineInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Generate returns the bound implementation.
func (in *DateEngineInstance) Generate() DateEngineGenerateConfig {
	if in == nil {
		return DefaultDateEngineGenerateConfig()
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.gen
}

// Locale resolves props locale over Ctx locale.
func (in *DateEngineInstance) Locale() string {
	if in == nil {
		return "zh-CN"
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.props.Locale != "" {
		return in.props.Locale
	}
	if in.ctx.Locale != "" {
		return in.ctx.Locale
	}
	return "zh-CN"
}

// WeekStart resolves the week start for this host.
func (in *DateEngineInstance) WeekStart() int {
	if in == nil {
		return 0
	}
	return ResolveDateEngineWeekStart(in.props, in.Locale())
}

// SetOnChange registers the value callback.
func (in *DateEngineInstance) SetOnChange(fn func(d *DateEngineDate, s string)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onChange = fn
}

// SetOnRangeChange registers the range callback.
func (in *DateEngineInstance) SetOnRangeChange(fn func(dates [2]*DateEngineDate, strs [2]string)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onRange = fn
}

// SetOnPanelChange registers the panel callback.
func (in *DateEngineInstance) SetOnPanelChange(fn func(value *DateEngineDate, mode DateEngineMode)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onPanel = fn
}

// SetOnCalendarChange registers the pending-date callback.
func (in *DateEngineInstance) SetOnCalendarChange(fn func(dates []*DateEngineDate)) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.onCalendar = fn
}

// SetValue drives controlled value from outside.
func (in *DateEngineInstance) SetValue(d *DateEngineDate) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctlValue = true
	in.value = d
}

// Value returns a copy of the single value.
func (in *DateEngineInstance) Value() *DateEngineDate {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.value == nil {
		return nil
	}
	v := *in.value
	return &v
}

// Range returns copies of the range ends.
func (in *DateEngineInstance) Range() [2]*DateEngineDate {
	if in == nil {
		return [2]*DateEngineDate{}
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	var out [2]*DateEngineDate
	for i := 0; i < 2; i++ {
		if in.rangeV[i] != nil {
			v := *in.rangeV[i]
			out[i] = &v
		}
	}
	return out
}

// Multiple returns a copy of the multi list.
func (in *DateEngineInstance) Multiple() []DateEngineDate {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return append([]DateEngineDate(nil), in.multi...)
}

// PickerValue returns a copy of the panel date.
func (in *DateEngineInstance) PickerValue() *DateEngineDate {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.picker == nil {
		return nil
	}
	v := *in.picker
	return &v
}

// Mode returns the visible panel mode.
func (in *DateEngineInstance) Mode() DateEngineMode {
	if in == nil {
		return DateEngineModeDate
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.mode
}

// SetPickerValue drives controlled panel date from outside.
func (in *DateEngineInstance) SetPickerValue(d *DateEngineDate) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctlPicker = true
	in.picker = d
}

// SetMode drives controlled mode from outside.
func (in *DateEngineInstance) SetMode(m DateEngineMode) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctlMode = true
	in.mode = m
}

// IsDisabledDate reports min/max plus DisabledDate for one day.
func (in *DateEngineInstance) IsDisabledDate(d DateEngineDate) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.disabledLocked(d)
}

func (in *DateEngineInstance) disabledLocked(d DateEngineDate) bool {
	day := in.gen.StartOfDay(d)
	if in.props.MinDateSet && in.props.MinDate != nil {
		if in.gen.Compare(day, in.gen.StartOfDay(*in.props.MinDate)) < 0 {
			return true
		}
	}
	if in.props.MaxDateSet && in.props.MaxDate != nil {
		if in.gen.Compare(day, in.gen.StartOfDay(*in.props.MaxDate)) > 0 {
			return true
		}
	}
	if in.props.DisabledDate != nil {
		return in.props.DisabledDate(d, nil, in.props.Picker)
	}
	return false
}

// DisabledClock returns disabled clock units for one datetime.
func (in *DateEngineInstance) DisabledClock(d DateEngineDate, part string) DateEngineDisabledTime {
	if in == nil {
		return DateEngineDisabledTime{}
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if in.props.DisabledTime != nil {
		return in.props.DisabledTime(d, part)
	}
	return DateEngineDisabledTime{}
}

// formatLocked renders with the display format.
func (in *DateEngineInstance) formatLocked(d *DateEngineDate) string {
	if d == nil {
		return ""
	}
	loc := ResolveDateEngineLocale(in.localeLocked())
	return FormatDateEngineDate(*d, DisplayDateEngineFormat(in.props), in.props.BuddhistEra, loc, ResolveDateEngineWeekStart(in.props, in.localeLocked()))
}

func (in *DateEngineInstance) localeLocked() string {
	if in.props.Locale != "" {
		return in.props.Locale
	}
	if in.ctx.Locale != "" {
		return in.ctx.Locale
	}
	return "zh-CN"
}

// Select picks one date through single/multiple/range semantics.
func (in *DateEngineInstance) Select(d DateEngineDate) bool {
	if in == nil || !d.IsValid() {
		return false
	}
	in.mu.Lock()
	if in.disabledLocked(d) {
		in.mu.Unlock()
		return false
	}
	if in.props.Multiple {
		in.toggleMultiLocked(d)
		cals := in.calendarLocked()
		cb := in.onCalendar
		in.mu.Unlock()
		if cb != nil {
			cb(cals)
		}
		return true
	}
	if in.props.Range {
		ok := in.stepRangeLocked(d)
		in.mu.Unlock()
		return ok
	}
	str := in.formatLocked(&d)
	if !in.ctlValue {
		v := d
		in.value = &v
	}
	cb := in.onChange
	in.mu.Unlock()
	if cb != nil {
		var out *DateEngineDate
		if !in.ctlValue {
			out = &d
		} else {
			out = &d
		}
		cb(out, str)
	}
	return true
}

func (in *DateEngineInstance) toggleMultiLocked(d DateEngineDate) {
	for i, m := range in.multi {
		if m.SameDay(d) {
			in.multi = append(in.multi[:i], in.multi[i+1:]...)
			return
		}
	}
	in.multi = append(in.multi, d)
}

func (in *DateEngineInstance) calendarLocked() []*DateEngineDate {
	out := make([]*DateEngineDate, 0, len(in.multi))
	for _, m := range in.multi {
		v := m
		out = append(out, &v)
	}
	return out
}

// stepRangeLocked consumes one click for range mode.
func (in *DateEngineInstance) stepRangeLocked(d DateEngineDate) bool {
	order := ResolveDateEngineOrder(in.props)
	if in.rangeV[0] != nil && in.rangeV[1] != nil {
		// Completed range: a new click starts the next range.
		v := d
		in.rangeV[0], in.rangeV[1] = &v, nil
		in.pending = 1
		cals := []*DateEngineDate{in.rangeV[0]}
		cb := in.onCalendar
		in.mu.Unlock()
		if cb != nil {
			cb(cals)
		}
		in.mu.Lock()
		return true
	}
	if in.rangeV[0] == nil {
		v := d
		in.rangeV[0] = &v
		in.pending = 1
		cals := []*DateEngineDate{in.rangeV[0]}
		cb := in.onCalendar
		in.mu.Unlock()
		if cb != nil {
			cb(cals)
		}
		in.mu.Lock()
		return true
	}
	v := d
	in.rangeV[1] = &v
	a, b := in.rangeV[0], in.rangeV[1]
	if order && a != nil && b != nil && in.gen.Compare(*a, *b) > 0 {
		in.rangeV[0], in.rangeV[1] = in.rangeV[1], in.rangeV[0]
	}
	if !in.props.AllowEmpty[0] && in.rangeV[0] == nil {
		in.rangeV[0] = in.rangeV[1]
	}
	dates := in.rangeV
	strs := [2]string{in.formatLocked(dates[0]), in.formatLocked(dates[1])}
	in.pending = 0
	cb := in.onRange
	cal := in.onCalendar
	cals := []*DateEngineDate{}
	if dates[0] != nil {
		cals = append(cals, dates[0])
	}
	if dates[1] != nil {
		cals = append(cals, dates[1])
	}
	in.mu.Unlock()
	if cal != nil {
		cal(cals)
	}
	if cb != nil {
		cb(dates, strs)
	}
	in.mu.Lock()
	return true
}

// ClearRange clears one or both ends honoring allowEmpty.
func (in *DateEngineInstance) ClearRange(end int) {
	if in == nil {
		return
	}
	in.mu.Lock()
	if end < 0 {
		in.rangeV = [2]*DateEngineDate{}
		in.pending = 0
	} else if end < 2 {
		in.rangeV[end] = nil
		if end == 0 {
			in.pending = 0
		}
	}
	dates := in.rangeV
	strs := [2]string{in.formatLocked(dates[0]), in.formatLocked(dates[1])}
	cb := in.onRange
	in.mu.Unlock()
	if cb != nil {
		cb(dates, strs)
	}
}

// SelectPreset resolves one preset (func values supported) and applies it.
func (in *DateEngineInstance) SelectPreset(i int) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	if i < 0 || i >= len(in.props.Presets) {
		in.mu.Unlock()
		return false
	}
	vals := in.props.Presets[i].Resolve()
	if len(vals) == 0 {
		in.mu.Unlock()
		return false
	}
	if len(vals) == 1 {
		d := vals[0]
		if in.disabledLocked(d) {
			in.mu.Unlock()
			return false
		}
		str := in.formatLocked(&d)
		if !in.ctlValue {
			in.value = &d
		}
		cb := in.onChange
		in.mu.Unlock()
		if cb != nil {
			cb(&d, str)
		}
		return true
	}
	a, b := vals[0], vals[1]
	if ResolveDateEngineOrder(in.props) && in.gen.Compare(a, b) > 0 {
		a, b = b, a
	}
	in.rangeV[0], in.rangeV[1] = &a, &b
	in.pending = 0
	dates := in.rangeV
	strs := [2]string{in.formatLocked(dates[0]), in.formatLocked(dates[1])}
	cb := in.onRange
	in.mu.Unlock()
	if cb != nil {
		cb(dates, strs)
	}
	return true
}

// GoPanel steps the panel date by one unit of the current mode.
func (in *DateEngineInstance) GoPanel(delta int) {
	if in == nil {
		return
	}
	in.mu.Lock()
	if in.picker == nil {
		v := DateEngineToday(in.gen)
		in.picker = &v
	}
	switch in.mode {
	case DateEngineModeYear:
		*in.picker = in.gen.AddYears(*in.picker, delta)
	case DateEngineModeDecade:
		*in.picker = in.gen.AddYears(*in.picker, delta*10)
	case DateEngineModeMonth:
		*in.picker = in.gen.AddYears(*in.picker, delta)
	default:
		*in.picker = in.gen.AddMonths(*in.picker, delta)
	}
	if !in.ctlPicker {
		// Uncontrolled keeps its own panel date; controlled waits
		// for the outside to push it back.
	}
	pv := *in.picker
	m := in.mode
	cb := in.onPanel
	in.mu.Unlock()
	if cb != nil {
		cb(&pv, m)
	}
}

// DrillDown moves date->month->year->decade on header click.
func (in *DateEngineInstance) DrillDown() {
	if in == nil {
		return
	}
	in.mu.Lock()
	next := in.mode
	switch in.mode {
	case DateEngineModeDate:
		next = DateEngineModeMonth
	case DateEngineModeMonth:
		next = DateEngineModeYear
	case DateEngineModeYear:
		next = DateEngineModeDecade
	}
	if !in.ctlMode {
		in.mode = next
	}
	m := next
	var pv *DateEngineDate
	if in.picker != nil {
		v := *in.picker
		pv = &v
	}
	cb := in.onPanel
	in.mu.Unlock()
	if cb != nil {
		cb(pv, m)
	}
}

// HolderContent renders static-call holder copy through Ctx.
func (in *DateEngineInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "date holder"
	}
	return ctx.HolderRender("date")
}
