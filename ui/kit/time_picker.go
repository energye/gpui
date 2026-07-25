package kit

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design TimePicker defaults — docs/antd/time-picker.md §6.2 / §6.10
// https://ant.design/components/time-picker
const (
	DefaultTimePickerWidth        = 200.0
	DefaultTimePickerMinWidth     = 140.0
	DefaultTimePickerRangeWidth   = 280.0
	DefaultTimePickerGap          = 4.0
	DefaultTimePickerPanelPad     = 4.0
	DefaultTimePickerPanelRadius  = 8.0
	DefaultTimePickerColWidth     = 56.0
	DefaultTimePickerCellH        = 28.0
	DefaultTimePickerListMaxH     = 224.0 // ~8 cells
	DefaultTimePickerFocusOutset  = 1.5
	DefaultTimePickerPlaceholder  = "Select time"
	DefaultTimePickerRangeStartPH = "Start time"
	DefaultTimePickerRangeEndPH   = "End time"
	DefaultTimePickerFormat       = "HH:mm:ss"
	DefaultTimePickerFormat12     = "h:mm:ss a"
)

// TimeValue is a clock time aligned with antd/dayjs time semantics.
// Hour is always stored in 24h form (0..23). Valid=false means empty / cleared.
type TimeValue struct {
	Hour, Minute, Second int
	Valid                bool
}

// TimeOf builds a valid time (clamped).
func TimeOf(hour, minute, second int) TimeValue {
	if hour < 0 {
		hour = 0
	}
	if hour > 23 {
		hour = 23
	}
	if minute < 0 {
		minute = 0
	}
	if minute > 59 {
		minute = 59
	}
	if second < 0 {
		second = 0
	}
	if second > 59 {
		second = 59
	}
	return TimeValue{Hour: hour, Minute: minute, Second: second, Valid: true}
}

// NowTime returns local clock now.
func NowTime() TimeValue {
	n := time.Now()
	return TimeOf(n.Hour(), n.Minute(), n.Second())
}

// TimeValueFromTime converts time.Time (zero → invalid; date ignored).
func TimeValueFromTime(t time.Time) TimeValue {
	if t.IsZero() {
		return TimeValue{}
	}
	return TimeOf(t.Hour(), t.Minute(), t.Second())
}

// IsZero reports empty value.
func (v TimeValue) IsZero() bool { return !v.Valid }

// Equal reports same H:M:S.
func (v TimeValue) Equal(o TimeValue) bool {
	if !v.Valid || !o.Valid {
		return v.Valid == o.Valid
	}
	return v.Hour == o.Hour && v.Minute == o.Minute && v.Second == o.Second
}

// Before reports chronological order within a day.
func (v TimeValue) Before(o TimeValue) bool {
	if !v.Valid || !o.Valid {
		return false
	}
	if v.Hour != o.Hour {
		return v.Hour < o.Hour
	}
	if v.Minute != o.Minute {
		return v.Minute < o.Minute
	}
	return v.Second < o.Second
}

// TotalSeconds is seconds since 00:00:00 (invalid → -1).
func (v TimeValue) TotalSeconds() int {
	if !v.Valid {
		return -1
	}
	return v.Hour*3600 + v.Minute*60 + v.Second
}

// TimeDisabled lists disabled options for the current draft (kit form of antd disabledTime).
type TimeDisabled struct {
	Hours   []int // 0..23
	Minutes []int // 0..59 for current hour
	Seconds []int // 0..59 for current hour+minute
}

// TimePickerPlacement is popup placement (antd placement).
type TimePickerPlacement int

const (
	TimePickerBottomLeft TimePickerPlacement = iota // default
	TimePickerBottomRight
	TimePickerTopLeft
	TimePickerTopRight
)

// TimePicker is Ant Design TimePicker / Time RangePicker — field trigger + column panel.
//
//	Column (Wrap)
//	  ├─ Pressable trigger
//	  └─ AnchoredPopup → panel (H/M/S[/AM·PM] columns + footer)
//
// Product contract: docs/antd/time-picker.md §6 (P0 DoD).
type TimePicker struct {
	Wrap     *primitive.Flex
	Root     *primitive.Pressable
	decor    *primitive.Decorated
	display  *primitive.Text
	clearBtn *primitive.Pressable
	suffix   *primitive.Icon
	spinner  *primitive.Canvas
	popup    *primitive.AnchoredPopup
	panel    *primitive.Decorated
	body     *primitive.Flex

	// Product fields (§6.10).
	Value        TimeValue
	RangeStart   TimeValue
	RangeEnd     TimeValue
	DefaultValue TimeValue
	DefaultStart TimeValue
	DefaultEnd   TimeValue
	Format       string
	HourStep     int
	MinuteStep   int
	SecondStep   int
	NeedConfirm  bool
	ShowNow      bool
	Use12Hours   bool
	Range        bool
	Order        bool
	Disabled     bool
	Loading      bool
	Size         InputSize
	Variant      InputVariant
	Status       InputStatus
	Open         bool
	AllowClear   bool
	Placement    TimePickerPlacement
	Placeholder  string
	RangeStartPH string
	RangeEndPH   string
	AriaLabel    string
	Face         text.Face
	Theme        *core.Theme
	Viewport     core.Size
	FixedWidth   float64

	// DisabledTime returns disabled units for the current draft.
	DisabledTime func(draft TimeValue) TimeDisabled
	// RenderExtraFooter is panel bottom addon (antd renderExtraFooter).
	RenderExtraFooter func() core.Node

	OnChange      func(time TimeValue, timeString string)
	OnChangeRange func(start, end TimeValue, timeStrings [2]string)
	OnOpenChange  func(open bool)
	OnOk          func(time TimeValue, timeString string)
	OnClear       func()

	// Draft / pending selection while open.
	draft        TimeValue
	pending      TimeValue
	pendingStart TimeValue
	pendingEnd   TimeValue
	hasPending   bool
	rangePicking bool // true after start picked, waiting for end
	meridiemPM   bool // draft AM/PM when Use12Hours (false=AM)

	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool

	// Loading spinner.
	spinPhase float64
	boundTree *core.Tree

	Nav *core.KeyboardNav
}

// NewTimePicker creates a single TimePicker.
// Defaults (§6.10): middle/outlined, closed, AllowClear=true, ShowNow=true, steps=1.
func NewTimePicker() *TimePicker {
	tp := &TimePicker{
		Size:       InputMiddle,
		Variant:    InputOutlined,
		Status:     InputStatusNone,
		AllowClear: true,
		ShowNow:    true,
		Order:      true,
		Placement:  TimePickerBottomLeft,
		HourStep:   1,
		MinuteStep: 1,
		SecondStep: 1,
	}
	tp.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	tp.rebuild()
	return tp
}

// NewTimeRangePicker creates a range TimePicker (Range=true).
func NewTimeRangePicker() *TimePicker {
	tp := NewTimePicker()
	tp.Range = true
	tp.rebuild()
	return tp
}

// Node returns the composition root.
func (tp *TimePicker) Node() core.Node {
	if tp == nil {
		return nil
	}
	if tp.Wrap == nil {
		tp.rebuild()
	}
	return tp.Wrap
}

// Popup returns the anchored popup.
func (tp *TimePicker) Popup() *primitive.AnchoredPopup {
	if tp == nil {
		return nil
	}
	return tp.popup
}

// Panel returns the dropdown panel chrome.
func (tp *TimePicker) Panel() *primitive.Decorated {
	if tp == nil {
		return nil
	}
	return tp.panel
}

// TriggerShell returns the trigger pressable.
func (tp *TimePicker) TriggerShell() *primitive.Pressable {
	if tp == nil {
		return nil
	}
	return tp.Root
}

// IsOpen reports panel visibility.
func (tp *TimePicker) IsOpen() bool { return tp != nil && tp.Open }

// GetValue returns the single value.
func (tp *TimePicker) GetValue() TimeValue {
	if tp == nil {
		return TimeValue{}
	}
	return tp.Value
}

// GetRangeValue returns range ends.
func (tp *TimePicker) GetRangeValue() (start, end TimeValue) {
	if tp == nil {
		return TimeValue{}, TimeValue{}
	}
	return tp.RangeStart, tp.RangeEnd
}

// DisplayText returns the current field display string.
func (tp *TimePicker) DisplayText() string {
	if tp == nil {
		return ""
	}
	return tp.computeDisplay()
}

// FormatValue formats one value with current Format.
func (tp *TimePicker) FormatValue(v TimeValue) string {
	if tp == nil {
		return ""
	}
	return tp.formatOne(v)
}

// Draft returns the in-panel draft selection.
func (tp *TimePicker) Draft() TimeValue {
	if tp == nil {
		return TimeValue{}
	}
	return tp.draft
}

// HourOptions returns the hour column values (24h internal, stepped).
func (tp *TimePicker) HourOptions() []int {
	if tp == nil {
		return nil
	}
	return tp.hourOptions()
}

// MinuteOptions returns minute column values.
func (tp *TimePicker) MinuteOptions() []int {
	if tp == nil {
		return nil
	}
	return tp.minuteOptions()
}

// SecondOptions returns second column values.
func (tp *TimePicker) SecondOptions() []int {
	if tp == nil {
		return nil
	}
	return tp.secondOptions()
}

// ShowSecondColumn reports whether the seconds column is visible (format).
func (tp *TimePicker) ShowSecondColumn() bool {
	if tp == nil {
		return true
	}
	return tp.showSecond()
}

// ShowMinuteColumn reports whether the minutes column is visible.
func (tp *TimePicker) ShowMinuteColumn() bool {
	if tp == nil {
		return true
	}
	return tp.showMinute()
}

// ---------------------------------------------------------------------------
// Setters
// ---------------------------------------------------------------------------

// SetValue sets single value (API write; no OnChange).
func (tp *TimePicker) SetValue(v TimeValue) {
	if tp == nil {
		return
	}
	tp.Value = v
	if v.Valid {
		tp.syncDraftFrom(v)
	}
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetDefaultValue seeds uncontrolled value when still empty.
func (tp *TimePicker) SetDefaultValue(v TimeValue) {
	if tp == nil {
		return
	}
	tp.DefaultValue = v
	if !tp.appliedDefVal && !tp.Value.Valid && !tp.Range && v.Valid {
		tp.appliedDefVal = true
		tp.Value = v
		tp.syncDraftFrom(v)
		tp.refreshDisplay()
	}
}

// SetRangeValue sets range ends (no OnChange).
func (tp *TimePicker) SetRangeValue(start, end TimeValue) {
	if tp == nil {
		return
	}
	if tp.Order && start.Valid && end.Valid && end.Before(start) {
		start, end = end, start
	}
	tp.RangeStart, tp.RangeEnd = start, end
	if start.Valid {
		tp.syncDraftFrom(start)
	}
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetDefaultRangeValue seeds uncontrolled range.
func (tp *TimePicker) SetDefaultRangeValue(start, end TimeValue) {
	if tp == nil {
		return
	}
	tp.DefaultStart, tp.DefaultEnd = start, end
	if !tp.appliedDefVal && tp.Range && !tp.RangeStart.Valid && !tp.RangeEnd.Valid {
		tp.appliedDefVal = true
		tp.SetRangeValue(start, end)
	}
}

// Clear resets selection and fires OnChange / OnClear.
func (tp *TimePicker) Clear() {
	if tp == nil || tp.Disabled {
		return
	}
	tp.hasPending = false
	tp.rangePicking = false
	if tp.Range {
		tp.RangeStart, tp.RangeEnd = TimeValue{}, TimeValue{}
		tp.fireRangeChange(TimeValue{}, TimeValue{})
	} else {
		tp.Value = TimeValue{}
		tp.fireChange(TimeValue{})
	}
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
	if tp.OnClear != nil {
		tp.OnClear()
	}
}

// SetFormat sets display format (empty → default by Use12Hours).
func (tp *TimePicker) SetFormat(f string) {
	if tp == nil {
		return
	}
	tp.Format = f
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetHourStep sets hour column step (≥1).
func (tp *TimePicker) SetHourStep(n int) {
	if tp == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	tp.HourStep = n
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetMinuteStep sets minute column step (≥1).
func (tp *TimePicker) SetMinuteStep(n int) {
	if tp == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	tp.MinuteStep = n
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetSecondStep sets second column step (≥1).
func (tp *TimePicker) SetSecondStep(n int) {
	if tp == nil {
		return
	}
	if n < 1 {
		n = 1
	}
	tp.SecondStep = n
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetNeedConfirm toggles confirm-before-commit.
func (tp *TimePicker) SetNeedConfirm(v bool) {
	if tp == nil {
		return
	}
	tp.NeedConfirm = v
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetShowNow toggles the Now button.
func (tp *TimePicker) SetShowNow(v bool) {
	if tp == nil {
		return
	}
	tp.ShowNow = v
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetUse12Hours toggles 12-hour mode (+ AM/PM column).
func (tp *TimePicker) SetUse12Hours(v bool) {
	if tp == nil {
		return
	}
	tp.Use12Hours = v
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetRange enables range mode.
func (tp *TimePicker) SetRange(v bool) {
	if tp == nil {
		return
	}
	tp.Range = v
	tp.refreshDisplay()
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetOrder toggles auto-sort for range (default true).
func (tp *TimePicker) SetOrder(v bool) {
	if tp == nil {
		return
	}
	tp.Order = v
}

// SetDisabledTime sets the disable predicate.
func (tp *TimePicker) SetDisabledTime(fn func(draft TimeValue) TimeDisabled) {
	if tp == nil {
		return
	}
	tp.DisabledTime = fn
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetRenderExtraFooter sets panel footer addon.
func (tp *TimePicker) SetRenderExtraFooter(fn func() core.Node) {
	if tp == nil {
		return
	}
	tp.RenderExtraFooter = fn
	if tp.Open {
		tp.rebuildPanelBody()
	}
}

// SetDisabled toggles whole control.
func (tp *TimePicker) SetDisabled(v bool) {
	if tp == nil {
		return
	}
	tp.Disabled = v
	if v && tp.Open {
		tp.applyOpen(false, false)
	}
	if tp.Root != nil {
		tp.Root.SetDisabled(v)
	}
	tp.applyChrome()
	tp.applyA11y()
	// clear button may need to disappear
	if tp.AllowClear {
		tp.rebuild()
	}
}

// SetLoading toggles loading spinner on the suffix (Ticker).
func (tp *TimePicker) SetLoading(v bool) {
	if tp == nil {
		return
	}
	tp.Loading = v
	if tp.boundTree != nil {
		if v {
			tp.boundTree.AddTicker(tp)
		} else {
			tp.boundTree.RemoveTicker(tp)
		}
	}
	tp.rebuild()
}

// SetSize sets control size.
func (tp *TimePicker) SetSize(sz InputSize) {
	if tp == nil {
		return
	}
	tp.Size = sz
	tp.rebuild()
}

// SetVariant sets visual variant.
func (tp *TimePicker) SetVariant(v InputVariant) {
	if tp == nil {
		return
	}
	tp.Variant = v
	tp.applyChrome()
}

// SetStatus sets validation chrome.
func (tp *TimePicker) SetStatus(st InputStatus) {
	if tp == nil {
		return
	}
	tp.Status = st
	tp.applyChrome()
	tp.applyA11y()
}

// SetOpen sets open state. Marks open as controlled (antd open prop).
func (tp *TimePicker) SetOpen(open bool) {
	if tp == nil {
		return
	}
	tp.openControlled = true
	tp.applyOpen(open, false)
}

// SetDefaultOpen seeds uncontrolled open once.
func (tp *TimePicker) SetDefaultOpen(open bool) {
	if tp == nil {
		return
	}
	tp.defaultOpen = open
	tp.defaultOpenSet = true
	if !tp.openControlled && !tp.appliedDefault {
		tp.appliedDefault = true
		tp.applyOpen(open, false)
	}
}

// SetPlacement sets popup placement.
func (tp *TimePicker) SetPlacement(p TimePickerPlacement) {
	if tp == nil {
		return
	}
	tp.Placement = p
	if tp.popup != nil {
		tp.popup.Placement = mapTimePickerPlacement(p)
	}
}

// SetPlaceholder sets empty-field placeholder.
func (tp *TimePicker) SetPlaceholder(s string) {
	if tp == nil {
		return
	}
	tp.Placeholder = s
	tp.refreshDisplay()
}

// SetAriaLabel sets accessible name.
func (tp *TimePicker) SetAriaLabel(s string) {
	if tp == nil {
		return
	}
	tp.AriaLabel = s
	tp.applyA11y()
}

// SetFace sets font face.
func (tp *TimePicker) SetFace(face text.Face) {
	if tp == nil {
		return
	}
	tp.Face = face
	tp.rebuild()
}

// SetTheme sets explicit theme override.
func (tp *TimePicker) SetTheme(th *core.Theme) {
	if tp == nil {
		return
	}
	tp.Theme = th
	tp.rebuild()
}

// SetOnChange sets single-value change callback.
func (tp *TimePicker) SetOnChange(fn func(time TimeValue, timeString string)) {
	if tp == nil {
		return
	}
	tp.OnChange = fn
}

// SetOnChangeRange sets range change callback.
func (tp *TimePicker) SetOnChangeRange(fn func(start, end TimeValue, timeStrings [2]string)) {
	if tp == nil {
		return
	}
	tp.OnChangeRange = fn
}

// SetOnOpenChange sets open change callback.
func (tp *TimePicker) SetOnOpenChange(fn func(open bool)) {
	if tp == nil {
		return
	}
	tp.OnOpenChange = fn
}

// SetOnOk sets confirm callback (needConfirm OK / Now commit).
func (tp *TimePicker) SetOnOk(fn func(time TimeValue, timeString string)) {
	if tp == nil {
		return
	}
	tp.OnOk = fn
}

// SetOnClear sets clear callback.
func (tp *TimePicker) SetOnClear(fn func()) {
	if tp == nil {
		return
	}
	tp.OnClear = fn
}

// ---------------------------------------------------------------------------
// Selection API (tests / headless)
// ---------------------------------------------------------------------------

// SelectTime selects a full time (respects disabledTime / needConfirm / range).
func (tp *TimePicker) SelectTime(v TimeValue) {
	if tp == nil || tp.Disabled || !v.Valid {
		return
	}
	if tp.isTimeDisabled(v) {
		return
	}
	tp.syncDraftFrom(v)
	if tp.Range {
		tp.pickRange(v)
		return
	}
	if tp.NeedConfirm {
		tp.pending = v
		tp.hasPending = true
		tp.rebuildPanelBody()
		return
	}
	tp.commitSingle(v, true)
}

// SelectHour sets draft hour (24h internal) and may commit.
func (tp *TimePicker) SelectHour(h int) {
	if tp == nil || tp.Disabled {
		return
	}
	if !tp.draft.Valid {
		tp.draft = TimeOf(0, 0, 0)
	}
	if tp.Use12Hours {
		// h is display hour 1..12 → convert with meridiem
		h24 := displayHourTo24(h, tp.meridiemPM)
		if tp.isHourDisabled(h24) {
			return
		}
		tp.draft.Hour = h24
	} else {
		if tp.isHourDisabled(h) {
			return
		}
		tp.draft.Hour = clampInt(h, 0, 23)
	}
	tp.draft.Valid = true
	tp.afterUnitPick()
}

// SelectMinute sets draft minute and may commit.
func (tp *TimePicker) SelectMinute(m int) {
	if tp == nil || tp.Disabled {
		return
	}
	if !tp.draft.Valid {
		tp.draft = TimeOf(0, 0, 0)
	}
	if tp.isMinuteDisabled(m) {
		return
	}
	tp.draft.Minute = clampInt(m, 0, 59)
	tp.draft.Valid = true
	tp.afterUnitPick()
}

// SelectSecond sets draft second and may commit.
func (tp *TimePicker) SelectSecond(s int) {
	if tp == nil || tp.Disabled {
		return
	}
	if !tp.draft.Valid {
		tp.draft = TimeOf(0, 0, 0)
	}
	if tp.isSecondDisabled(s) {
		return
	}
	tp.draft.Second = clampInt(s, 0, 59)
	tp.draft.Valid = true
	tp.afterUnitPick()
}

// SelectMeridiem sets AM/PM (use12Hours); pm=true → PM.
func (tp *TimePicker) SelectMeridiem(pm bool) {
	if tp == nil || tp.Disabled || !tp.Use12Hours {
		return
	}
	if !tp.draft.Valid {
		tp.draft = TimeOf(0, 0, 0)
	}
	tp.meridiemPM = pm
	// remap hour into the chosen half-day
	h12 := hour24ToDisplay(tp.draft.Hour)
	tp.draft.Hour = displayHourTo24(h12, pm)
	tp.draft.Valid = true
	tp.afterUnitPick()
}

// SelectRange sets both ends (orders when Order=true).
func (tp *TimePicker) SelectRange(start, end TimeValue) {
	if tp == nil || tp.Disabled {
		return
	}
	if start.Valid && tp.isTimeDisabled(start) {
		return
	}
	if end.Valid && tp.isTimeDisabled(end) {
		return
	}
	if tp.Order && start.Valid && end.Valid && end.Before(start) {
		start, end = end, start
	}
	if tp.NeedConfirm {
		tp.pendingStart, tp.pendingEnd = start, end
		tp.hasPending = true
		tp.rebuildPanelBody()
		return
	}
	tp.RangeStart, tp.RangeEnd = start, end
	tp.fireRangeChange(start, end)
	tp.refreshDisplay()
	tp.applyOpen(false, false)
}

// Confirm commits pending (needConfirm) or current draft.
func (tp *TimePicker) Confirm() {
	if tp == nil || tp.Disabled {
		return
	}
	if tp.Range {
		start, end := tp.pendingStart, tp.pendingEnd
		if !tp.hasPending {
			start, end = tp.RangeStart, tp.RangeEnd
		}
		if tp.Order && start.Valid && end.Valid && end.Before(start) {
			start, end = end, start
		}
		tp.hasPending = false
		tp.RangeStart, tp.RangeEnd = start, end
		tp.fireRangeChange(start, end)
		if tp.OnOk != nil {
			tp.OnOk(start, tp.formatOne(start))
		}
		tp.refreshDisplay()
		tp.applyOpen(false, false)
		return
	}
	v := tp.pending
	if !tp.hasPending {
		v = tp.draft
	}
	if !v.Valid {
		v = tp.ensureDraft()
	}
	if tp.isTimeDisabled(v) {
		return
	}
	tp.hasPending = false
	tp.commitSingle(v, true)
	if tp.OnOk != nil {
		tp.OnOk(v, tp.formatOne(v))
	}
}

// Now sets the current clock time.
func (tp *TimePicker) Now() {
	if tp == nil || tp.Disabled {
		return
	}
	v := NowTime()
	if tp.isTimeDisabled(v) {
		return
	}
	tp.syncDraftFrom(v)
	if tp.NeedConfirm {
		tp.pending = v
		tp.hasPending = true
		tp.rebuildPanelBody()
		return
	}
	if tp.Range {
		// fill the active end
		tp.pickRange(v)
		return
	}
	tp.commitSingle(v, true)
}

// CancelPending drops uncommitted selection.
func (tp *TimePicker) CancelPending() {
	if tp == nil {
		return
	}
	tp.hasPending = false
	tp.rangePicking = false
	if tp.Value.Valid {
		tp.syncDraftFrom(tp.Value)
	} else if tp.Range && tp.RangeStart.Valid {
		tp.syncDraftFrom(tp.RangeStart)
	} else {
		tp.draft = TimeValue{}
	}
}

// HandleKey processes open / escape / enter when focused (tests / hosts).
func (tp *TimePicker) HandleKey(ev *core.KeyEvent) {
	if tp == nil || tp.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !tp.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" || ev.Key == "Down" {
			tp.requestOpen(true)
			ev.Handled = true
		}
		return
	}
	if ev.Key == "Escape" || ev.Key == "Esc" {
		tp.CancelPending()
		tp.requestOpen(false)
		ev.Handled = true
		return
	}
	if (ev.Key == "Enter" || ev.Key == " ") && tp.NeedConfirm {
		tp.Confirm()
		ev.Handled = true
	}
}

// ---------------------------------------------------------------------------
// Ticker (loading)
// ---------------------------------------------------------------------------

// AttachTicker registers loading spinner animation.
func (tp *TimePicker) AttachTicker(t *core.Tree) {
	if tp == nil || t == nil {
		return
	}
	tp.boundTree = t
	t.BindTicker(tp, tp.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (tp *TimePicker) Tick(dt float64) bool {
	if tp == nil || !tp.Loading {
		return false
	}
	tp.spinPhase += dt * 1.4
	if tp.spinPhase > 1 {
		tp.spinPhase -= 1
	}
	if tp.spinner != nil {
		tp.spinner.MarkNeedsPaint()
	} else if tp.decor != nil {
		tp.decor.MarkNeedsPaint()
	}
	return tp.Loading
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (tp *TimePicker) theme() *core.Theme {
	var n core.Node
	if tp.Wrap != nil {
		n = tp.Wrap
	}
	return themeOf(tp.Theme, n)
}

func (tp *TimePicker) controlHeight() float64 {
	th := tp.theme()
	switch tp.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (tp *TimePicker) fontSize() float64 {
	th := tp.theme()
	switch tp.Size {
	case InputSmall:
		return th.SizeOr(core.TokenFontSizeSM, 12)
	case InputLarge:
		return th.SizeOr(core.TokenFontSizeLG, 16)
	default:
		return th.SizeOr(core.TokenFontSize, 14)
	}
}

func (tp *TimePicker) hasValue() bool {
	if tp.Range {
		return tp.RangeStart.Valid || tp.RangeEnd.Valid
	}
	return tp.Value.Valid
}

func (tp *TimePicker) activeFormat() string {
	if tp.Format != "" {
		return tp.Format
	}
	if tp.Use12Hours {
		return DefaultTimePickerFormat12
	}
	return DefaultTimePickerFormat
}

func (tp *TimePicker) showHour() bool {
	f := tp.activeFormat()
	// always show hour unless format explicitly has no hour token
	if f == "" {
		return true
	}
	return strings.Contains(f, "H") || strings.Contains(f, "h") ||
		(!strings.Contains(f, "m") && !strings.Contains(f, "s"))
}

func (tp *TimePicker) showMinute() bool {
	f := tp.activeFormat()
	if f == "" {
		return true
	}
	// antd: "HH" alone hides minutes; "HH:mm" shows minutes
	return strings.Contains(f, "m")
}

func (tp *TimePicker) showSecond() bool {
	f := tp.activeFormat()
	if f == "" {
		return true
	}
	return strings.Contains(f, "s") || strings.Contains(f, "S")
}

func (tp *TimePicker) formatOne(v TimeValue) string {
	if !v.Valid {
		return ""
	}
	f := tp.activeFormat()
	h24 := v.Hour
	h12 := hour24ToDisplay(h24)
	pm := h24 >= 12
	ampm := "am"
	AMPM := "AM"
	if pm {
		ampm = "pm"
		AMPM = "PM"
	}
	// Replace longer tokens first.
	repl := []struct{ tok, val string }{
		{"HH", fmt.Sprintf("%02d", h24)},
		{"hh", fmt.Sprintf("%02d", h12)},
		{"mm", fmt.Sprintf("%02d", v.Minute)},
		{"ss", fmt.Sprintf("%02d", v.Second)},
		{"H", fmt.Sprintf("%d", h24)},
		{"h", fmt.Sprintf("%d", h12)},
		{"m", fmt.Sprintf("%d", v.Minute)},
		{"s", fmt.Sprintf("%d", v.Second)},
		{"A", AMPM},
		{"a", ampm},
	}
	out := f
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.tok, r.val)
	}
	return out
}

func (tp *TimePicker) computeDisplay() string {
	if tp.Range {
		if !tp.RangeStart.Valid && !tp.RangeEnd.Valid {
			return ""
		}
		a, b := tp.formatOne(tp.RangeStart), tp.formatOne(tp.RangeEnd)
		if a == "" {
			a = "…"
		}
		if b == "" {
			b = "…"
		}
		return a + " ~ " + b
	}
	if !tp.Value.Valid {
		return ""
	}
	return tp.formatOne(tp.Value)
}

func (tp *TimePicker) placeholderText() string {
	if tp.Range {
		s := tp.RangeStartPH
		if s == "" {
			s = DefaultTimePickerRangeStartPH
		}
		e := tp.RangeEndPH
		if e == "" {
			e = DefaultTimePickerRangeEndPH
		}
		return s + " ~ " + e
	}
	if tp.Placeholder != "" {
		return tp.Placeholder
	}
	return DefaultTimePickerPlaceholder
}

func (tp *TimePicker) refreshDisplay() {
	if tp.display == nil {
		return
	}
	text := tp.computeDisplay()
	ph := false
	if text == "" {
		text = tp.placeholderText()
		ph = true
	}
	tp.display.SetValue(text)
	th := tp.theme()
	if ph {
		tp.display.Color = tp.placeholderColor(th)
	} else {
		tp.display.Color = tp.valueColor(th)
	}
	if tp.AllowClear {
		wantClear := tp.hasValue() && !tp.Disabled
		haveClear := tp.clearBtn != nil
		if wantClear != haveClear {
			tp.rebuild()
			return
		}
	}
	if tp.decor != nil {
		tp.decor.MarkNeedsPaint()
	}
}

func (tp *TimePicker) placeholderColor(th *core.Theme) render.RGBA {
	col := th.Color(core.TokenColorTextSecondary)
	if col.A < 0.2 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	return col
}

func (tp *TimePicker) valueColor(th *core.Theme) render.RGBA {
	if tp.Disabled {
		col := th.Color(core.TokenColorDisabledText)
		if col.A < 0.2 {
			col = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		return col
	}
	col := th.Color(core.TokenColorText)
	if col.A < 0.2 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
	}
	return col
}

func (tp *TimePicker) syncDraftFrom(v TimeValue) {
	tp.draft = v
	if v.Valid {
		tp.meridiemPM = v.Hour >= 12
	}
}

func (tp *TimePicker) ensureDraft() TimeValue {
	if tp.draft.Valid {
		return tp.draft
	}
	// seed from value / now
	if tp.Value.Valid {
		tp.syncDraftFrom(tp.Value)
		return tp.draft
	}
	if tp.Range && tp.RangeStart.Valid {
		tp.syncDraftFrom(tp.RangeStart)
		return tp.draft
	}
	tp.draft = TimeOf(0, 0, 0)
	tp.meridiemPM = false
	return tp.draft
}

func (tp *TimePicker) afterUnitPick() {
	v := tp.draft
	if !v.Valid {
		return
	}
	// zero unused units when columns hidden
	if !tp.showMinute() {
		v.Minute = 0
	}
	if !tp.showSecond() {
		v.Second = 0
	}
	tp.draft = v
	if tp.Range {
		// in range mode, unit picks update draft; commit via SelectTime-like double pick
		// For simplicity: each full draft pick via Confirm or explicit SelectTime.
		// Clicking a unit updates draft and, if !needConfirm, advances range picking.
		if !tp.NeedConfirm {
			tp.pickRange(v)
		} else {
			if !tp.rangePicking {
				tp.pendingStart = v
				tp.hasPending = true
			} else {
				tp.pendingEnd = v
				tp.hasPending = true
			}
			tp.rebuildPanelBody()
		}
		return
	}
	if tp.NeedConfirm {
		tp.pending = v
		tp.hasPending = true
		tp.rebuildPanelBody()
		return
	}
	tp.commitSingle(v, false)
	tp.rebuildPanelBody()
}

func (tp *TimePicker) commitSingle(v TimeValue, close bool) {
	tp.Value = v
	tp.syncDraftFrom(v)
	tp.fireChange(v)
	tp.refreshDisplay()
	if close {
		tp.applyOpen(false, false)
	}
}

func (tp *TimePicker) pickRange(v TimeValue) {
	if !tp.rangePicking || !tp.RangeStart.Valid {
		tp.RangeStart = v
		tp.RangeEnd = TimeValue{}
		tp.rangePicking = true
		tp.syncDraftFrom(v)
		tp.refreshDisplay()
		tp.rebuildPanelBody()
		return
	}
	start, end := tp.RangeStart, v
	if tp.Order && end.Valid && start.Valid && end.Before(start) {
		start, end = end, start
	}
	tp.RangeStart, tp.RangeEnd = start, end
	tp.rangePicking = false
	tp.fireRangeChange(start, end)
	tp.refreshDisplay()
	tp.applyOpen(false, false)
}

func (tp *TimePicker) fireChange(v TimeValue) {
	if tp.OnChange != nil {
		tp.OnChange(v, tp.formatOne(v))
	}
}

func (tp *TimePicker) fireRangeChange(start, end TimeValue) {
	if tp.OnChangeRange != nil {
		tp.OnChangeRange(start, end, [2]string{tp.formatOne(start), tp.formatOne(end)})
	}
	// also fire OnChange with start for simple listeners
	if tp.OnChange != nil {
		tp.OnChange(start, tp.formatOne(start))
	}
}

func (tp *TimePicker) disabledSpec() TimeDisabled {
	if tp.DisabledTime == nil {
		return TimeDisabled{}
	}
	d := tp.ensureDraft()
	return tp.DisabledTime(d)
}

func (tp *TimePicker) isHourDisabled(h int) bool {
	spec := tp.disabledSpec()
	for _, x := range spec.Hours {
		if x == h {
			return true
		}
	}
	return false
}

func (tp *TimePicker) isMinuteDisabled(m int) bool {
	spec := tp.disabledSpec()
	for _, x := range spec.Minutes {
		if x == m {
			return true
		}
	}
	return false
}

func (tp *TimePicker) isSecondDisabled(s int) bool {
	spec := tp.disabledSpec()
	for _, x := range spec.Seconds {
		if x == s {
			return true
		}
	}
	return false
}

func (tp *TimePicker) isTimeDisabled(v TimeValue) bool {
	if !v.Valid {
		return false
	}
	if tp.isHourDisabled(v.Hour) {
		return true
	}
	if tp.isMinuteDisabled(v.Minute) {
		return true
	}
	if tp.isSecondDisabled(v.Second) {
		return true
	}
	return false
}

func (tp *TimePicker) hourOptions() []int {
	step := tp.HourStep
	if step < 1 {
		step = 1
	}
	if tp.Use12Hours {
		// display hours 12,1,2,...,11 — stored as display values for column labels
		out := make([]int, 0, 12/step+1)
		// generate 0..11 then map to display
		for h := 0; h < 12; h += step {
			out = append(out, hour24ToDisplay(h)) // 12,1,2,...
		}
		return out
	}
	out := make([]int, 0, 24/step+1)
	for h := 0; h < 24; h += step {
		out = append(out, h)
	}
	return out
}

func (tp *TimePicker) minuteOptions() []int {
	step := tp.MinuteStep
	if step < 1 {
		step = 1
	}
	out := make([]int, 0, 60/step+1)
	for m := 0; m < 60; m += step {
		out = append(out, m)
	}
	return out
}

func (tp *TimePicker) secondOptions() []int {
	step := tp.SecondStep
	if step < 1 {
		step = 1
	}
	out := make([]int, 0, 60/step+1)
	for s := 0; s < 60; s += step {
		out = append(out, s)
	}
	return out
}

func (tp *TimePicker) applyOpen(open bool, fromDismiss bool) {
	if tp == nil || (tp.Disabled && open) {
		return
	}
	prev := tp.Open
	tp.Open = open
	if !open {
		if fromDismiss {
			tp.CancelPending()
		}
	} else {
		if tp.Range && tp.RangeStart.Valid {
			tp.syncDraftFrom(tp.RangeStart)
		} else if tp.Value.Valid {
			tp.syncDraftFrom(tp.Value)
		} else {
			tp.ensureDraft()
		}
		tp.rebuildPanelBody()
	}
	if tp.popup != nil {
		if open {
			tp.syncPopupGeometry()
		}
		tp.popup.SetOpen(open)
	}
	tp.applyChrome()
	if prev != open && tp.OnOpenChange != nil {
		tp.OnOpenChange(open)
	}
}

func (tp *TimePicker) requestOpen(open bool) {
	if tp == nil || tp.Disabled {
		return
	}
	if tp.openControlled {
		if tp.OnOpenChange != nil && tp.Open != open {
			tp.OnOpenChange(open)
		}
		return
	}
	tp.applyOpen(open, true)
}

func (tp *TimePicker) syncPopupGeometry() {
	if tp.popup == nil || tp.Root == nil {
		return
	}
	tp.popup.UpdateAnchorFromNode(tp.Root)
	if tp.Viewport.Width > 0 {
		tp.popup.Viewport = tp.Viewport
	}
}

func (tp *TimePicker) rebuild() {
	if tp == nil {
		return
	}
	// apply default value once
	if !tp.appliedDefVal {
		if tp.Range && !tp.RangeStart.Valid && !tp.RangeEnd.Valid && (tp.DefaultStart.Valid || tp.DefaultEnd.Valid) {
			tp.appliedDefVal = true
			tp.RangeStart, tp.RangeEnd = tp.DefaultStart, tp.DefaultEnd
			if tp.Order && tp.RangeStart.Valid && tp.RangeEnd.Valid && tp.RangeEnd.Before(tp.RangeStart) {
				tp.RangeStart, tp.RangeEnd = tp.RangeEnd, tp.RangeStart
			}
		} else if !tp.Range && !tp.Value.Valid && tp.DefaultValue.Valid {
			tp.appliedDefVal = true
			tp.Value = tp.DefaultValue
			tp.syncDraftFrom(tp.Value)
		}
	}

	th := tp.theme()
	h := tp.controlHeight()
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	fontSz := tp.fontSize()
	wasOpen := tp.Open

	disp := tp.computeDisplay()
	ph := false
	if disp == "" {
		disp = tp.placeholderText()
		ph = true
	}
	tp.display = primitive.NewText(disp)
	tp.display.FontSize = fontSz
	tp.display.Face = tp.Face
	if ph {
		tp.display.Color = tp.placeholderColor(th)
	} else {
		tp.display.Color = tp.valueColor(th)
	}

	var clearNode core.Node
	if tp.AllowClear && tp.hasValue() && !tp.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		tp.clearBtn = primitive.NewPressable(x)
		tp.clearBtn.Focusable = false
		tp.clearBtn.ShowFocusRing = false
		tp.clearBtn.Base().Role = "button"
		tp.clearBtn.Base().Label = "clear"
		tp.clearBtn.Click = func() { tp.Clear() }
		clearNode = tp.clearBtn
	} else {
		tp.clearBtn = nil
	}

	// suffix: loading spinner or clock-ish chevron
	var suffixNode core.Node
	tp.spinner = nil
	if tp.Loading {
		spin := primitive.NewCanvas(14, 14, tp.paintSpinner)
		tp.spinner = spin
		suffixNode = spin
	} else {
		tp.suffix = primitive.NewIcon("chevron-down")
		tp.suffix.Size = 12
		sec := th.Color(core.TokenColorTextSecondary)
		if sec.A < 0.3 {
			sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
		}
		tp.suffix.Color = sec
		suffixNode = tp.suffix
	}

	rowKids := []core.Node{tp.display, primitive.Spacer()}
	if clearNode != nil {
		rowKids = append(rowKids, clearNode)
	}
	if suffixNode != nil {
		rowKids = append(rowKids, suffixNode)
	}
	row := primitive.Row(rowKids...)
	row.CrossAlign = core.CrossCenter
	row.Gap = 8

	tp.decor = primitive.NewDecorated(row)
	tp.decor.Padding = primitive.Symmetric(padH, 0)
	tp.decor.Radius = radius
	tp.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	tp.decor.MinHeight = h
	tp.decor.Height = h
	w := tp.FixedWidth
	if w <= 0 {
		w = DefaultTimePickerWidth
		if tp.Range {
			w = DefaultTimePickerRangeWidth
		}
	}
	tp.decor.MinWidth = DefaultTimePickerMinWidth
	tp.decor.Width = w
	tp.decor.SetCenterContent(true)
	tp.decor.StretchChild = true
	tp.applyChrome()

	// panel body
	tp.body = primitive.Column()
	tp.body.Gap = 4
	tp.body.CrossAlign = core.CrossStart
	tp.body.Base().Role = "listbox"

	tp.panel = primitive.NewDecorated(tp.body)
	tp.panel.Padding = primitive.All(DefaultTimePickerPanelPad)
	tp.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultTimePickerPanelRadius)
	tp.panel.Background = th.Color(core.TokenColorBgContainer)
	tp.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	tp.panel.BorderColor = th.Color(core.TokenColorBorder)
	tp.panel.MinWidth = DefaultTimePickerMinWidth
	tp.panel.Base().Role = "listbox"

	if tp.popup == nil {
		tp.popup = primitive.NewAnchoredPopup(tp.panel)
	} else {
		tp.popup.Content = tp.panel
	}
	tp.popup.Placement = mapTimePickerPlacement(tp.Placement)
	tp.popup.Gap = DefaultTimePickerGap
	tp.popup.DismissOnOutside = true
	tp.popup.OnDismiss = func() {
		tp.Open = false
		tp.CancelPending()
		if tp.OnOpenChange != nil {
			tp.OnOpenChange(false)
		}
		tp.applyChrome()
		tp.applyA11y()
	}

	// trigger shell — keep identity
	if tp.Root == nil {
		tp.Root = primitive.NewPressable(tp.decor)
	} else {
		tp.Root.ClearChildren()
		tp.Root.AddChild(tp.decor)
	}
	tp.Root.Focusable = true
	tp.Root.ShowFocusRing = true
	tp.Root.FocusRingRadius = tp.decor.Radius
	tp.Root.FocusRingOutset = DefaultTimePickerFocusOutset
	tp.Root.SetDisabled(tp.Disabled)
	tp.Root.OnStateChange = func() { tp.applyChrome() }
	tp.Root.Click = func() {
		if tp.Disabled {
			return
		}
		tp.requestOpen(!tp.Open)
	}

	if tp.Wrap == nil {
		tp.Wrap = primitive.Column(tp.Root, tp.popup)
	} else {
		tp.Wrap.ClearChildren()
		tp.Wrap.AddChild(tp.Root)
		tp.Wrap.AddChild(tp.popup)
	}
	tp.Wrap.CrossAlign = core.CrossStart
	tp.Wrap.Gap = 0
	tp.Wrap.SetThemeHook(func(*core.Theme) { tp.rebuild() })
	tp.Wrap.MarkNeedsLayout()
	tp.Wrap.MarkNeedsPaint()

	tp.applyA11y()
	tp.rebuildPanelBody()

	if wasOpen {
		tp.applyOpen(true, false)
	} else if tp.defaultOpenSet && !tp.openControlled && !tp.appliedDefault {
		tp.appliedDefault = true
		tp.applyOpen(tp.defaultOpen, false)
	}
}

func (tp *TimePicker) rebuildPanelBody() {
	if tp == nil || tp.body == nil {
		return
	}
	tp.body.ClearChildren()
	th := tp.theme()
	_ = tp.ensureDraft()

	cols := primitive.Row()
	cols.Gap = 0
	cols.CrossAlign = core.CrossStart

	if tp.showHour() {
		cols.AddChild(tp.buildUnitColumn(th, "hour"))
	}
	if tp.showMinute() {
		cols.AddChild(tp.buildUnitColumn(th, "minute"))
	}
	if tp.showSecond() {
		cols.AddChild(tp.buildUnitColumn(th, "second"))
	}
	if tp.Use12Hours {
		cols.AddChild(tp.buildMeridiemColumn(th))
	}
	tp.body.AddChild(cols)

	// footer: Now / OK / extra
	footerKids := []core.Node{}
	if tp.ShowNow {
		footerKids = append(footerKids, tp.buildTextBtn(th, "Now", func() { tp.Now() }))
	}
	if tp.NeedConfirm {
		footerKids = append(footerKids, primitive.Spacer())
		footerKids = append(footerKids, tp.buildPrimaryBtn(th, "OK", func() { tp.Confirm() }))
	}
	if len(footerKids) > 0 {
		foot := primitive.Row(footerKids...)
		foot.CrossAlign = core.CrossCenter
		foot.Gap = 8
		// pad top separator-ish
		wrap := primitive.Column(foot)
		wrap.Padding = primitive.EdgeInsets{Top: 4}
		tp.body.AddChild(wrap)
	}
	if tp.RenderExtraFooter != nil {
		if n := tp.RenderExtraFooter(); n != nil {
			tp.body.AddChild(n)
		}
	}
	tp.body.MarkNeedsLayout()
	tp.body.MarkNeedsPaint()
}

func (tp *TimePicker) buildUnitColumn(th *core.Theme, kind string) core.Node {
	var opts []int
	var cur int
	var disabled func(int) bool
	var onPick func(int)
	labelW := DefaultTimePickerColWidth

	switch kind {
	case "hour":
		opts = tp.hourOptions()
		if tp.Use12Hours {
			cur = hour24ToDisplay(tp.draft.Hour)
			disabled = func(h int) bool {
				return tp.isHourDisabled(displayHourTo24(h, tp.meridiemPM))
			}
			onPick = func(h int) { tp.SelectHour(h) }
		} else {
			cur = tp.draft.Hour
			disabled = tp.isHourDisabled
			onPick = func(h int) { tp.SelectHour(h) }
		}
	case "minute":
		opts = tp.minuteOptions()
		cur = tp.draft.Minute
		disabled = tp.isMinuteDisabled
		onPick = func(m int) { tp.SelectMinute(m) }
	case "second":
		opts = tp.secondOptions()
		cur = tp.draft.Second
		disabled = tp.isSecondDisabled
		onPick = func(s int) { tp.SelectSecond(s) }
	}

	col := primitive.Column()
	col.Gap = 0
	col.CrossAlign = core.CrossStretch
	col.Base().Role = "listbox"
	// cap height for scroll-like viewport (no virtual scroll in P0 — show all stepped opts)
	for _, v := range opts {
		v := v
		lab := fmt.Sprintf("%02d", v)
		if kind == "hour" && tp.Use12Hours {
			lab = fmt.Sprintf("%d", v) // 12,1,2… unpadded ok; pad if prefer
			lab = fmt.Sprintf("%02d", v)
		}
		t := primitive.NewText(lab)
		t.FontSize = 13
		t.Face = tp.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width = labelW
		dec.Height = DefaultTimePickerCellH
		dec.MinHeight = DefaultTimePickerCellH
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		dis := disabled != nil && disabled(v)
		selected := v == cur
		if selected && !dis {
			dec.Background = th.Color(core.TokenColorPrimary)
			t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		if dis {
			t.Color = th.Color(core.TokenColorDisabledText)
			if t.Color.A < 0.2 {
				t.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		p.Focusable = !dis
		p.Base().Role = "option"
		p.Base().Label = lab
		if dis {
			p.SetDisabled(true)
		} else {
			p.ColorHovered = antItemHoverFill(th)
			vv := v
			p.Click = func() { onPick(vv) }
		}
		col.AddChild(p)
	}
	// wrap in decorated for column border-right
	shell := primitive.NewDecorated(col)
	shell.BorderWidth = 0
	shell.Width = labelW
	// P0: full stepped list (no virtual scroll). Height follows options.
	_ = DefaultTimePickerListMaxH
	return shell
}

func (tp *TimePicker) buildMeridiemColumn(th *core.Theme) core.Node {
	col := primitive.Column()
	col.Gap = 0
	for _, pm := range []bool{false, true} {
		pm := pm
		lab := "am"
		if pm {
			lab = "pm"
		}
		t := primitive.NewText(lab)
		t.FontSize = 13
		t.Face = tp.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width = DefaultTimePickerColWidth
		dec.Height = DefaultTimePickerCellH
		dec.MinHeight = DefaultTimePickerCellH
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		if tp.meridiemPM == pm {
			dec.Background = th.Color(core.TokenColorPrimary)
			t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		p.Base().Role = "option"
		p.Base().Label = lab
		p.ColorHovered = antItemHoverFill(th)
		p.Click = func() { tp.SelectMeridiem(pm) }
		col.AddChild(p)
	}
	shell := primitive.NewDecorated(col)
	shell.Width = DefaultTimePickerColWidth
	return shell
}

func (tp *TimePicker) buildTextBtn(th *core.Theme, label string, click func()) core.Node {
	t := primitive.NewText(label)
	t.FontSize = 13
	t.Face = tp.Face
	t.Color = th.Color(core.TokenColorPrimary)
	dec := primitive.NewDecorated(t)
	dec.Padding = primitive.Symmetric(8, 4)
	dec.Radius = 4
	dec.BorderWidth = 0
	dec.SetCenterContent(true)
	dec.StretchChild = true
	p := primitive.NewPressable(dec)
	p.ShowFocusRing = false
	p.Base().Role = "button"
	p.Base().Label = label
	p.ColorHovered = antItemHoverFill(th)
	p.Click = click
	return p
}

func (tp *TimePicker) buildPrimaryBtn(th *core.Theme, label string, click func()) core.Node {
	t := primitive.NewText(label)
	t.FontSize = 13
	t.Face = tp.Face
	t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	dec := primitive.NewDecorated(t)
	dec.Padding = primitive.Symmetric(12, 4)
	dec.Radius = 4
	dec.Background = th.Color(core.TokenColorPrimary)
	dec.BorderWidth = 0
	dec.SetCenterContent(true)
	dec.StretchChild = true
	p := primitive.NewPressable(dec)
	p.ShowFocusRing = false
	p.Base().Role = "button"
	p.Base().Label = label
	p.Click = click
	return p
}

func (tp *TimePicker) applyChrome() {
	if tp == nil || tp.decor == nil {
		return
	}
	th := tp.theme()
	h := tp.controlHeight()
	tp.decor.MinHeight = h
	tp.decor.Height = h
	tp.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	tp.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	switch tp.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		tp.decor.BorderWidth = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		tp.decor.BorderWidth = 0
	case InputUnderlined:
		bg = render.RGBA{}
		tp.decor.Radius = 0
	}

	if tp.Disabled {
		dbg := th.Color(core.TokenColorDisabledBg)
		if dbg.A < 0.05 {
			dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bg = dbg
		bd = th.Color(core.TokenColorBorder)
		tp.decor.Background = bg
		tp.decor.BorderColor = bd
		if tp.display != nil {
			tp.display.Color = tp.valueColor(th)
		}
		tp.decor.MarkNeedsPaint()
		return
	}

	switch tp.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
	default:
		if tp.Root != nil && (tp.Root.State.Focused || tp.Open) {
			bd = th.Color(core.TokenColorPrimary)
		} else if tp.Root != nil && tp.Root.State.Hovered {
			hb := th.Color(core.TokenColorBorderHover)
			if hb.A < 0.5 {
				hb = th.Color(core.TokenColorPrimaryHover)
			}
			bd = hb
		}
	}
	if tp.Variant == InputUnderlined {
		tp.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	}
	tp.decor.Background = bg
	tp.decor.BorderColor = bd
	if tp.display != nil {
		if tp.computeDisplay() == "" {
			tp.display.Color = tp.placeholderColor(th)
		} else {
			tp.display.Color = tp.valueColor(th)
		}
	}
	tp.decor.MarkNeedsPaint()
}

func (tp *TimePicker) applyA11y() {
	if tp == nil || tp.Root == nil {
		return
	}
	tp.Root.Base().Role = "combobox"
	label := tp.AriaLabel
	if label == "" {
		label = tp.Placeholder
	}
	if label == "" {
		if tp.Range {
			label = "TimeRangePicker"
		} else {
			label = "TimePicker"
		}
	}
	if tp.Status == InputStatusError {
		label = label + " invalid"
	}
	tp.Root.Base().Label = label
}

func (tp *TimePicker) paintSpinner(pc *core.PaintContext, size core.Size) {
	if pc == nil {
		return
	}
	th := tp.theme()
	col := th.Color(core.TokenColorPrimary)
	if col.A < 0.1 {
		col = render.Hex("#1677FF")
	}
	cx, cy := size.Width/2, size.Height/2
	r := math.Min(size.Width, size.Height)/2 - 1
	if r < 2 {
		r = 2
	}
	stroke := 1.5
	steps := 12
	start := -math.Pi/2 + tp.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func mapTimePickerPlacement(p TimePickerPlacement) primitive.Placement {
	switch p {
	case TimePickerBottomRight:
		return primitive.PlaceBottomEnd
	case TimePickerTopLeft:
		return primitive.PlaceTopStart
	case TimePickerTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}

func hour24ToDisplay(h24 int) int {
	h := h24 % 12
	if h == 0 {
		return 12
	}
	return h
}

func displayHourTo24(h12 int, pm bool) int {
	if h12 < 1 {
		h12 = 12
	}
	if h12 > 12 {
		h12 = 12
	}
	if pm {
		if h12 == 12 {
			return 12
		}
		return h12 + 12
	}
	if h12 == 12 {
		return 0
	}
	return h12
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
