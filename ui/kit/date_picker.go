package kit

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design DatePicker defaults — docs/antd/date-picker.md §6.2 / §6.10
// https://ant.design/components/date-picker
const (
	DefaultDatePickerWidth         = 280.0
	DefaultDatePickerMinWidth      = 200.0
	DefaultDatePickerGap           = 4.0
	DefaultDatePickerPanelPad      = 8.0
	DefaultDatePickerPanelRadius   = 8.0
	DefaultDatePickerCell          = 36.0
	DefaultDatePickerFocusOutset   = 1.5
	DefaultDatePickerPlaceholder   = "Select date"
	DefaultDatePickerRangeStartPH  = "Start date"
	DefaultDatePickerRangeEndPH    = "End date"
	DefaultDatePickerMultiMaxShown = 3
)

// DateValue is a calendar value aligned with antd/dayjs semantics.
// Valid=false means empty / cleared.
type DateValue struct {
	Year, Month, Day     int // Month 1..12
	Hour, Minute, Second int
	Valid                bool
	HasTime              bool
}

// DateOf builds a valid date (no time).
func DateOf(year, month, day int) DateValue {
	if year == 0 || month < 1 || month > 12 || day < 1 {
		return DateValue{}
	}
	max := daysInMonth(year, time.Month(month))
	if day > max {
		day = max
	}
	return DateValue{Year: year, Month: month, Day: day, Valid: true}
}

// DateTimeOf builds a valid date+time.
func DateTimeOf(year, month, day, hour, min, sec int) DateValue {
	v := DateOf(year, month, day)
	if !v.Valid {
		return v
	}
	if hour < 0 {
		hour = 0
	}
	if hour > 23 {
		hour = 23
	}
	if min < 0 {
		min = 0
	}
	if min > 59 {
		min = 59
	}
	if sec < 0 {
		sec = 0
	}
	if sec > 59 {
		sec = 59
	}
	v.Hour, v.Minute, v.Second = hour, min, sec
	v.HasTime = true
	return v
}

// DateValueFromTime converts time.Time (zero → invalid).
func DateValueFromTime(t time.Time) DateValue {
	if t.IsZero() {
		return DateValue{}
	}
	return DateTimeOf(t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// Today returns local-calendar today (date only).
func Today() DateValue {
	n := time.Now()
	return DateOf(n.Year(), int(n.Month()), n.Day())
}

// IsZero reports empty value.
func (v DateValue) IsZero() bool { return !v.Valid }

// Time converts to time.Time in local zone (invalid → zero).
func (v DateValue) Time() time.Time {
	if !v.Valid {
		return time.Time{}
	}
	h, m, s := 0, 0, 0
	if v.HasTime {
		h, m, s = v.Hour, v.Minute, v.Second
	}
	return time.Date(v.Year, time.Month(v.Month), v.Day, h, m, s, 0, time.Local)
}

// EqualDate reports same Y-M-D (ignores time).
func (v DateValue) EqualDate(o DateValue) bool {
	if !v.Valid || !o.Valid {
		return v.Valid == o.Valid
	}
	return v.Year == o.Year && v.Month == o.Month && v.Day == o.Day
}

// Before reports chronological order (date+time when HasTime).
func (v DateValue) Before(o DateValue) bool {
	if !v.Valid || !o.Valid {
		return false
	}
	return v.Time().Before(o.Time())
}

// DatePickerPicker is antd picker: date | week | month | quarter | year.
type DatePickerPicker int

const (
	DatePickerDate DatePickerPicker = iota // default
	DatePickerWeek
	DatePickerMonth
	DatePickerQuarter
	DatePickerYear
)

func (p DatePickerPicker) String() string {
	switch p {
	case DatePickerWeek:
		return "week"
	case DatePickerMonth:
		return "month"
	case DatePickerQuarter:
		return "quarter"
	case DatePickerYear:
		return "year"
	default:
		return "date"
	}
}

// DatePickerPlacement is popup placement (antd placement).
type DatePickerPlacement int

const (
	DatePickerBottomLeft DatePickerPlacement = iota // default
	DatePickerBottomRight
	DatePickerTopLeft
	DatePickerTopRight
)

// DatePicker is Ant Design DatePicker / RangePicker — field trigger + panel popup.
//
//	Column (Wrap)
//	  ├─ Pressable trigger
//	  └─ AnchoredPopup → panel (header / body / time / confirm)
//
// Product contract: docs/antd/date-picker.md §6 (P0 DoD).
type DatePicker struct {
	Wrap     *primitive.Flex
	Root     *primitive.Pressable
	decor    *primitive.Decorated
	display  *primitive.Text
	clearBtn *primitive.Pressable
	suffix   *primitive.Icon
	popup    *primitive.AnchoredPopup
	panel    *primitive.Decorated
	body     *primitive.Flex

	// Product fields (§6.10).
	Value        DateValue
	RangeStart   DateValue
	RangeEnd     DateValue
	MultiValue   []DateValue
	DefaultValue DateValue
	DefaultStart DateValue
	DefaultEnd   DateValue
	DefaultMulti []DateValue
	Picker       DatePickerPicker
	Format       string
	FormatMask   bool
	ShowTime     bool
	NeedConfirm  bool
	Multiple     bool
	Range        bool
	Order        bool
	Disabled     bool
	Size         InputSize
	Variant      InputVariant
	Status       InputStatus
	Open         bool
	AllowClear   bool
	Placement    DatePickerPlacement
	Placeholder  string
	RangeStartPH string
	RangeEndPH   string
	AriaLabel    string
	Face         text.Face
	Theme        *core.Theme
	Viewport     core.Size
	FixedWidth   float64
	DisabledDate func(DateValue) bool

	OnChange      func(date DateValue, dateString string)
	OnChangeRange func(start, end DateValue, dateStrings [2]string)
	OnChangeMulti func(dates []DateValue, dateStrings []string)
	OnOpenChange  func(open bool)
	OnOk          func(date DateValue, dateString string)
	OnClear       func()

	panelYear  int
	panelMonth time.Month

	pending      DateValue
	pendingStart DateValue
	pendingEnd   DateValue
	hasPending   bool
	rangePicking bool

	draftHour, draftMin, draftSec int

	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool

	Nav *core.KeyboardNav
}

// dayCell is one grid cell.
type dayCell struct {
	v     DateValue
	label string
	inMon bool
}

// NewDatePicker creates a single DatePicker (picker=date).
func NewDatePicker() *DatePicker {
	now := time.Now()
	d := &DatePicker{
		Size:       InputMiddle,
		Variant:    InputOutlined,
		Status:     InputStatusNone,
		AllowClear: true,
		Order:      true,
		Placement:  DatePickerBottomLeft,
		Picker:     DatePickerDate,
		panelYear:  now.Year(),
		panelMonth: now.Month(),
	}
	d.Nav = core.NewKeyboardNav(core.NavHorizontal, 0)
	d.rebuild()
	return d
}

// NewRangePicker creates a RangePicker (Range=true).
func NewRangePicker() *DatePicker {
	d := NewDatePicker()
	d.Range = true
	d.rebuild()
	return d
}

// Node returns the composition root.
func (d *DatePicker) Node() core.Node {
	if d == nil {
		return nil
	}
	if d.Wrap == nil {
		d.rebuild()
	}
	return d.Wrap
}

// Popup returns the anchored popup.
func (d *DatePicker) Popup() *primitive.AnchoredPopup {
	if d == nil {
		return nil
	}
	return d.popup
}

// Panel returns the dropdown panel chrome.
func (d *DatePicker) Panel() *primitive.Decorated {
	if d == nil {
		return nil
	}
	return d.panel
}

// TriggerShell returns the trigger pressable.
func (d *DatePicker) TriggerShell() *primitive.Pressable {
	if d == nil {
		return nil
	}
	return d.Root
}

// IsOpen reports panel visibility.
func (d *DatePicker) IsOpen() bool { return d != nil && d.Open }

// GetValue returns the single value.
func (d *DatePicker) GetValue() DateValue {
	if d == nil {
		return DateValue{}
	}
	return d.Value
}

// GetRangeValue returns range ends.
func (d *DatePicker) GetRangeValue() (start, end DateValue) {
	if d == nil {
		return DateValue{}, DateValue{}
	}
	return d.RangeStart, d.RangeEnd
}

// GetMultiValue returns a copy of multi selection.
func (d *DatePicker) GetMultiValue() []DateValue {
	if d == nil {
		return nil
	}
	return append([]DateValue(nil), d.MultiValue...)
}

// DisplayText returns the current field display string.
func (d *DatePicker) DisplayText() string {
	if d == nil {
		return ""
	}
	return d.computeDisplay()
}

// FormatValue formats one value with current Format / picker defaults.
func (d *DatePicker) FormatValue(v DateValue) string {
	if d == nil {
		return ""
	}
	return d.formatOne(v)
}

// PanelYearMonth returns the visible panel year/month.
func (d *DatePicker) PanelYearMonth() (year int, month time.Month) {
	if d == nil {
		return 0, 0
	}
	return d.panelYear, d.panelMonth
}

// YearMonth returns panel year and month as ints (compat).
func (d *DatePicker) YearMonth() (year int, month int) {
	y, m := d.PanelYearMonth()
	return y, int(m)
}

// SelectedDay returns day-of-month of Value (0 if empty).
func (d *DatePicker) SelectedDay() int {
	if d == nil || !d.Value.Valid {
		return 0
	}
	return d.Value.Day
}

// ---------------------------------------------------------------------------
// Setters
// ---------------------------------------------------------------------------

// SetValue sets single value (API write; no OnChange).
func (d *DatePicker) SetValue(v DateValue) {
	if d == nil {
		return
	}
	d.Value = v
	if v.Valid {
		d.panelYear, d.panelMonth = v.Year, time.Month(v.Month)
		if v.HasTime {
			d.draftHour, d.draftMin, d.draftSec = v.Hour, v.Minute, v.Second
		}
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetDefaultValue seeds uncontrolled value when still empty.
func (d *DatePicker) SetDefaultValue(v DateValue) {
	if d == nil {
		return
	}
	d.DefaultValue = v
	if !d.appliedDefVal && !d.Value.Valid && !d.Range && !d.Multiple && v.Valid {
		d.appliedDefVal = true
		d.Value = v
		d.panelYear, d.panelMonth = v.Year, time.Month(v.Month)
		d.refreshDisplay()
	}
}

// SetRangeValue sets range ends (no OnChange).
func (d *DatePicker) SetRangeValue(start, end DateValue) {
	if d == nil {
		return
	}
	if d.Order && start.Valid && end.Valid && end.Before(start) {
		start, end = end, start
	}
	d.RangeStart, d.RangeEnd = start, end
	if start.Valid {
		d.panelYear, d.panelMonth = start.Year, time.Month(start.Month)
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetDefaultRangeValue seeds uncontrolled range.
func (d *DatePicker) SetDefaultRangeValue(start, end DateValue) {
	if d == nil {
		return
	}
	d.DefaultStart, d.DefaultEnd = start, end
	if !d.appliedDefVal && d.Range && !d.RangeStart.Valid && !d.RangeEnd.Valid {
		d.appliedDefVal = true
		d.SetRangeValue(start, end)
	}
}

// SetMultiValue sets multi selection (no OnChange).
func (d *DatePicker) SetMultiValue(vs []DateValue) {
	if d == nil {
		return
	}
	d.MultiValue = append([]DateValue(nil), vs...)
	if d.Order {
		d.sortMulti()
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetDefaultMultiValue seeds uncontrolled multi.
func (d *DatePicker) SetDefaultMultiValue(vs []DateValue) {
	if d == nil {
		return
	}
	d.DefaultMulti = append([]DateValue(nil), vs...)
	if !d.appliedDefVal && d.Multiple && len(d.MultiValue) == 0 && len(vs) > 0 {
		d.appliedDefVal = true
		d.SetMultiValue(vs)
	}
}

// Clear resets selection and fires OnChange / OnClear.
func (d *DatePicker) Clear() {
	if d == nil || d.Disabled {
		return
	}
	d.hasPending = false
	d.rangePicking = false
	if d.Range {
		d.RangeStart, d.RangeEnd = DateValue{}, DateValue{}
		d.fireRangeChange(DateValue{}, DateValue{})
	} else if d.Multiple {
		d.MultiValue = nil
		d.fireMultiChange()
	} else {
		d.Value = DateValue{}
		d.fireChange(DateValue{})
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
	if d.OnClear != nil {
		d.OnClear()
	}
}

// SetPicker sets picker type.
func (d *DatePicker) SetPicker(p DatePickerPicker) {
	if d == nil {
		return
	}
	d.Picker = p
	if d.Open {
		d.rebuildPanelBody()
	}
	d.refreshDisplay()
}

// SetFormat sets display format (empty → picker default).
func (d *DatePicker) SetFormat(f string) {
	if d == nil {
		return
	}
	d.Format = f
	d.refreshDisplay()
}

// SetFormatMask toggles format type=mask.
func (d *DatePicker) SetFormatMask(v bool) {
	if d == nil {
		return
	}
	d.FormatMask = v
	d.refreshDisplay()
}

// SetShowTime toggles time columns.
func (d *DatePicker) SetShowTime(v bool) {
	if d == nil {
		return
	}
	d.ShowTime = v
	if d.Open {
		d.rebuildPanelBody()
	}
	d.refreshDisplay()
}

// SetNeedConfirm toggles confirm-before-commit.
func (d *DatePicker) SetNeedConfirm(v bool) {
	if d == nil {
		return
	}
	d.NeedConfirm = v
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetMultiple enables multi-date selection.
func (d *DatePicker) SetMultiple(v bool) {
	if d == nil {
		return
	}
	d.Multiple = v
	if v {
		d.Range = false
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetRange enables RangePicker mode.
func (d *DatePicker) SetRange(v bool) {
	if d == nil {
		return
	}
	d.Range = v
	if v {
		d.Multiple = false
	}
	d.refreshDisplay()
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetOrder toggles auto-sort for range/multi (default true).
func (d *DatePicker) SetOrder(v bool) {
	if d == nil {
		return
	}
	d.Order = v
}

// SetDisabledDate sets day disable predicate.
func (d *DatePicker) SetDisabledDate(fn func(DateValue) bool) {
	if d == nil {
		return
	}
	d.DisabledDate = fn
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SetDisabled toggles whole control.
func (d *DatePicker) SetDisabled(v bool) {
	if d == nil {
		return
	}
	d.Disabled = v
	if d.Root != nil {
		d.Root.SetDisabled(v)
	}
	if v && d.Open {
		d.applyOpen(false, false)
	}
	d.applyChrome()
	d.applyA11y()
	d.rebuild()
}

// SetSize updates control height via Token.
func (d *DatePicker) SetSize(s InputSize) {
	if d == nil {
		return
	}
	d.Size = s
	d.applyChrome()
}

// SetVariant updates visual variant.
func (d *DatePicker) SetVariant(v InputVariant) {
	if d == nil {
		return
	}
	d.Variant = v
	d.applyChrome()
}

// SetStatus updates validation chrome.
func (d *DatePicker) SetStatus(s InputStatus) {
	if d == nil {
		return
	}
	d.Status = s
	d.applyChrome()
	d.applyA11y()
}

// SetAllowClear toggles clear affordance.
func (d *DatePicker) SetAllowClear(v bool) {
	if d == nil {
		return
	}
	d.AllowClear = v
	d.rebuild()
}

// SetOpen sets visibility and marks controlled.
func (d *DatePicker) SetOpen(open bool) {
	if d == nil {
		return
	}
	d.openControlled = true
	d.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (d *DatePicker) SetDefaultOpen(open bool) {
	if d == nil || d.openControlled {
		return
	}
	d.defaultOpen = open
	d.defaultOpenSet = true
	if !d.appliedDefault {
		d.appliedDefault = true
		d.applyOpen(open, false)
	}
}

// SetPlacement sets popup placement.
func (d *DatePicker) SetPlacement(p DatePickerPlacement) {
	if d == nil {
		return
	}
	d.Placement = p
	if d.popup != nil {
		d.popup.Placement = mapDatePickerPlacement(p)
		if d.Open {
			d.syncPopupGeometry()
		}
	}
}

// SetPlaceholder sets single-mode placeholder.
func (d *DatePicker) SetPlaceholder(s string) {
	if d == nil {
		return
	}
	d.Placeholder = s
	d.refreshDisplay()
}

// SetRangePlaceholder sets RangePicker placeholders.
func (d *DatePicker) SetRangePlaceholder(start, end string) {
	if d == nil {
		return
	}
	d.RangeStartPH, d.RangeEndPH = start, end
	d.refreshDisplay()
}

// SetTheme sets an explicit theme override.
func (d *DatePicker) SetTheme(th *core.Theme) {
	if d == nil {
		return
	}
	d.Theme = th
	d.rebuild()
}

// SetFace sets the font face.
func (d *DatePicker) SetFace(face text.Face) {
	if d == nil {
		return
	}
	d.Face = face
	d.rebuild()
}

// SetAriaLabel sets the accessible name.
func (d *DatePicker) SetAriaLabel(s string) {
	if d == nil {
		return
	}
	d.AriaLabel = s
	d.applyA11y()
}

// SetOnChange sets single-select callback.
func (d *DatePicker) SetOnChange(fn func(date DateValue, dateString string)) {
	if d == nil {
		return
	}
	d.OnChange = fn
}

// SetOnChangeRange sets range callback.
func (d *DatePicker) SetOnChangeRange(fn func(start, end DateValue, dateStrings [2]string)) {
	if d == nil {
		return
	}
	d.OnChangeRange = fn
}

// SetOnChangeMulti sets multi callback.
func (d *DatePicker) SetOnChangeMulti(fn func(dates []DateValue, dateStrings []string)) {
	if d == nil {
		return
	}
	d.OnChangeMulti = fn
}

// SetOnOpenChange sets open-change callback.
func (d *DatePicker) SetOnOpenChange(fn func(open bool)) {
	if d == nil {
		return
	}
	d.OnOpenChange = fn
}

// SetOnOk sets needConfirm / showTime OK callback.
func (d *DatePicker) SetOnOk(fn func(date DateValue, dateString string)) {
	if d == nil {
		return
	}
	d.OnOk = fn
}

// SetOnClear sets clear callback.
func (d *DatePicker) SetOnClear(fn func()) {
	if d == nil {
		return
	}
	d.OnClear = fn
}

// SetPanelMonth sets visible panel year/month.
func (d *DatePicker) SetPanelMonth(year int, month time.Month) {
	if d == nil {
		return
	}
	if year == 0 {
		year = d.panelYear
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	d.panelYear, d.panelMonth = year, month
	if d.Open {
		d.rebuildPanelBody()
	}
}

// SelectDate selects a value programmatically.
func (d *DatePicker) SelectDate(v DateValue) {
	if d == nil || d.Disabled || !v.Valid {
		return
	}
	if d.isDisabled(v) {
		return
	}
	if !d.Open {
		d.applyOpen(true, false)
	}
	d.pickValue(v)
}

// SelectDay selects day-of-month in the current panel month.
func (d *DatePicker) SelectDay(day int) {
	if d == nil || day < 1 {
		return
	}
	max := daysInMonth(d.panelYear, d.panelMonth)
	if day > max {
		day = max
	}
	v := DateOf(d.panelYear, int(d.panelMonth), day)
	if d.ShowTime {
		v = DateTimeOf(v.Year, v.Month, v.Day, d.draftHour, d.draftMin, d.draftSec)
	}
	d.SelectDate(v)
}

// SelectRange selects a closed range.
func (d *DatePicker) SelectRange(start, end DateValue) {
	if d == nil || d.Disabled {
		return
	}
	if !d.Range {
		d.Range = true
	}
	if d.Order && start.Valid && end.Valid && end.Before(start) {
		start, end = end, start
	}
	if start.Valid && d.isDisabled(start) {
		return
	}
	if end.Valid && d.isDisabled(end) {
		return
	}
	if d.NeedConfirm {
		d.pendingStart, d.pendingEnd = start, end
		d.hasPending = true
		d.rebuildPanelBody()
		return
	}
	d.RangeStart, d.RangeEnd = start, end
	d.rangePicking = false
	d.fireRangeChange(start, end)
	d.refreshDisplay()
	if !d.openControlled {
		d.applyOpen(false, false)
	}
	if d.Open {
		d.rebuildPanelBody()
	}
}

// Confirm commits pending selection (needConfirm).
func (d *DatePicker) Confirm() {
	if d == nil || d.Disabled {
		return
	}
	if d.Range {
		start, end := d.pendingStart, d.pendingEnd
		if !d.hasPending {
			start, end = d.RangeStart, d.RangeEnd
		}
		if d.Order && start.Valid && end.Valid && end.Before(start) {
			start, end = end, start
		}
		d.RangeStart, d.RangeEnd = start, end
		d.hasPending = false
		d.rangePicking = false
		d.fireRangeChange(start, end)
		if d.OnOk != nil {
			d.OnOk(end, d.formatOne(end))
		}
	} else if d.Multiple {
		d.hasPending = false
		d.fireMultiChange()
		if d.OnOk != nil && len(d.MultiValue) > 0 {
			last := d.MultiValue[len(d.MultiValue)-1]
			d.OnOk(last, d.formatOne(last))
		}
	} else {
		v := d.pending
		if !d.hasPending {
			v = d.Value
		}
		if d.ShowTime && v.Valid {
			v.Hour, v.Minute, v.Second = d.draftHour, d.draftMin, d.draftSec
			v.HasTime = true
		}
		d.Value = v
		d.hasPending = false
		d.fireChange(v)
		if d.OnOk != nil {
			d.OnOk(v, d.formatOne(v))
		}
	}
	d.refreshDisplay()
	if !d.openControlled {
		d.applyOpen(false, false)
	} else if d.Open {
		d.rebuildPanelBody()
	}
}

// CancelPending discards needConfirm preview.
func (d *DatePicker) CancelPending() {
	if d == nil {
		return
	}
	d.hasPending = false
	d.rangePicking = false
	d.pending = DateValue{}
	d.pendingStart, d.pendingEnd = DateValue{}, DateValue{}
	if d.Open {
		d.rebuildPanelBody()
	}
}

// HandleKey processes open / escape / enter when focused.
func (d *DatePicker) HandleKey(ev *core.KeyEvent) {
	if d == nil || d.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !d.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" || ev.Key == "Down" {
			d.applyOpen(true, false)
			ev.Handled = true
		}
		return
	}
	if ev.Key == "Escape" || ev.Key == "Esc" {
		d.CancelPending()
		d.applyOpen(false, false)
		ev.Handled = true
		return
	}
	if (ev.Key == "Enter" || ev.Key == " ") && d.NeedConfirm {
		d.Confirm()
		ev.Handled = true
	}
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (d *DatePicker) theme() *core.Theme {
	var n core.Node
	if d.Wrap != nil {
		n = d.Wrap
	}
	return themeOf(d.Theme, n)
}

func (d *DatePicker) controlHeight() float64 {
	th := d.theme()
	switch d.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (d *DatePicker) isDisabled(v DateValue) bool {
	if !v.Valid || d.DisabledDate == nil {
		return false
	}
	return d.DisabledDate(v)
}

func (d *DatePicker) hasValue() bool {
	if d.Range {
		return d.RangeStart.Valid || d.RangeEnd.Valid
	}
	if d.Multiple {
		return len(d.MultiValue) > 0
	}
	return d.Value.Valid
}

func (d *DatePicker) defaultFormat() string {
	base := "YYYY-MM-DD"
	switch d.Picker {
	case DatePickerWeek:
		base = "YYYY-wo"
	case DatePickerMonth:
		base = "YYYY-MM"
	case DatePickerQuarter:
		base = "YYYY-[Q]Q"
	case DatePickerYear:
		base = "YYYY"
	}
	if d.ShowTime && d.Picker == DatePickerDate {
		base += " HH:mm:ss"
	}
	return base
}

func (d *DatePicker) activeFormat() string {
	if d.Format != "" {
		return d.Format
	}
	return d.defaultFormat()
}

func (d *DatePicker) formatOne(v DateValue) string {
	if !v.Valid {
		return ""
	}
	f := d.activeFormat()
	repl := []struct{ tok, val string }{
		{"YYYY", fmt.Sprintf("%04d", v.Year)},
		{"MM", fmt.Sprintf("%02d", v.Month)},
		{"DD", fmt.Sprintf("%02d", v.Day)},
		{"HH", fmt.Sprintf("%02d", v.Hour)},
		{"mm", fmt.Sprintf("%02d", v.Minute)},
		{"ss", fmt.Sprintf("%02d", v.Second)},
		{"[Q]Q", fmt.Sprintf("Q%d", quarterOf(v.Month))},
		{"wo", fmt.Sprintf("%02dw", weekOfYear(v))},
	}
	out := f
	for _, r := range repl {
		out = strings.ReplaceAll(out, r.tok, r.val)
	}
	return out
}

func quarterOf(month int) int {
	if month < 1 {
		return 1
	}
	return (month-1)/3 + 1
}

func weekOfYear(v DateValue) int {
	if !v.Valid {
		return 0
	}
	t := time.Date(v.Year, time.Month(v.Month), v.Day, 0, 0, 0, 0, time.UTC)
	_, w := t.ISOWeek()
	return w
}

func (d *DatePicker) computeDisplay() string {
	if d.Range {
		if !d.RangeStart.Valid && !d.RangeEnd.Valid {
			return ""
		}
		a, b := d.formatOne(d.RangeStart), d.formatOne(d.RangeEnd)
		if a == "" {
			a = "…"
		}
		if b == "" {
			b = "…"
		}
		return a + " ~ " + b
	}
	if d.Multiple {
		if len(d.MultiValue) == 0 {
			return ""
		}
		parts := make([]string, 0, len(d.MultiValue))
		for i, v := range d.MultiValue {
			if i >= DefaultDatePickerMultiMaxShown {
				parts = append(parts, fmt.Sprintf("+%d…", len(d.MultiValue)-DefaultDatePickerMultiMaxShown))
				break
			}
			parts = append(parts, d.formatOne(v))
		}
		return strings.Join(parts, ", ")
	}
	if !d.Value.Valid {
		return ""
	}
	return d.formatOne(d.Value)
}

func (d *DatePicker) placeholderText() string {
	if d.Range {
		s := d.RangeStartPH
		if s == "" {
			s = DefaultDatePickerRangeStartPH
		}
		e := d.RangeEndPH
		if e == "" {
			e = DefaultDatePickerRangeEndPH
		}
		return s + " ~ " + e
	}
	if d.Placeholder != "" {
		return d.Placeholder
	}
	if d.FormatMask {
		return d.activeFormat()
	}
	return DefaultDatePickerPlaceholder
}

func (d *DatePicker) refreshDisplay() {
	if d.display == nil {
		return
	}
	text := d.computeDisplay()
	ph := false
	if text == "" {
		text = d.placeholderText()
		ph = true
	}
	d.display.SetValue(text)
	th := d.theme()
	if ph {
		d.display.Color = d.placeholderColor(th)
	} else {
		d.display.Color = d.valueColor(th)
	}
	if d.AllowClear {
		wantClear := d.hasValue() && !d.Disabled
		haveClear := d.clearBtn != nil
		if wantClear != haveClear {
			d.rebuild()
			return
		}
	}
	if d.decor != nil {
		d.decor.MarkNeedsPaint()
	}
}

func (d *DatePicker) placeholderColor(th *core.Theme) render.RGBA {
	col := th.Color(core.TokenColorTextSecondary)
	if col.A < 0.2 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	return col
}

func (d *DatePicker) valueColor(th *core.Theme) render.RGBA {
	if d.Disabled {
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

func (d *DatePicker) applyOpen(open bool, fromDismiss bool) {
	if d == nil || (d.Disabled && open) {
		return
	}
	prev := d.Open
	d.Open = open
	if !open {
		if fromDismiss {
			d.CancelPending()
		}
	} else {
		if d.Range && d.RangeStart.Valid {
			d.panelYear, d.panelMonth = d.RangeStart.Year, time.Month(d.RangeStart.Month)
		} else if d.Value.Valid {
			d.panelYear, d.panelMonth = d.Value.Year, time.Month(d.Value.Month)
		} else if d.panelYear == 0 {
			n := time.Now()
			d.panelYear, d.panelMonth = n.Year(), n.Month()
		}
		if d.Value.Valid && d.Value.HasTime {
			d.draftHour, d.draftMin, d.draftSec = d.Value.Hour, d.Value.Minute, d.Value.Second
		}
		d.rebuildPanelBody()
	}
	if d.popup != nil {
		if open {
			d.syncPopupGeometry()
		}
		d.popup.SetOpen(open)
	}
	d.applyChrome()
	if prev != open && d.OnOpenChange != nil {
		d.OnOpenChange(open)
	}
}

func (d *DatePicker) syncPopupGeometry() {
	if d.popup == nil || d.Root == nil {
		return
	}
	d.popup.UpdateAnchorFromNode(d.Root)
	if d.Viewport.Width > 0 {
		d.popup.Viewport = d.Viewport
	}
}

func (d *DatePicker) rebuild() {
	if d == nil {
		return
	}
	if !d.appliedDefVal {
		if d.Range && !d.RangeStart.Valid && !d.RangeEnd.Valid && (d.DefaultStart.Valid || d.DefaultEnd.Valid) {
			d.appliedDefVal = true
			d.RangeStart, d.RangeEnd = d.DefaultStart, d.DefaultEnd
		} else if d.Multiple && len(d.MultiValue) == 0 && len(d.DefaultMulti) > 0 {
			d.appliedDefVal = true
			d.MultiValue = append([]DateValue(nil), d.DefaultMulti...)
		} else if !d.Range && !d.Multiple && !d.Value.Valid && d.DefaultValue.Valid {
			d.appliedDefVal = true
			d.Value = d.DefaultValue
			d.panelYear, d.panelMonth = d.Value.Year, time.Month(d.Value.Month)
		}
	}
	if d.panelYear == 0 {
		n := time.Now()
		d.panelYear, d.panelMonth = n.Year(), n.Month()
	}

	th := d.theme()
	h := d.controlHeight()
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	fontSz := th.SizeOr(core.TokenFontSize, 14)
	if d.Size == InputSmall {
		fontSz = th.SizeOr(core.TokenFontSizeSM, 12)
	} else if d.Size == InputLarge {
		fontSz = th.SizeOr(core.TokenFontSizeLG, 16)
	}

	disp := d.computeDisplay()
	ph := false
	if disp == "" {
		disp = d.placeholderText()
		ph = true
	}
	d.display = primitive.NewText(disp)
	d.display.FontSize = fontSz
	d.display.Face = d.Face
	if ph {
		d.display.Color = d.placeholderColor(th)
	} else {
		d.display.Color = d.valueColor(th)
	}

	var clearNode core.Node
	if d.AllowClear && d.hasValue() && !d.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		d.clearBtn = primitive.NewPressable(x)
		d.clearBtn.Focusable = false
		d.clearBtn.ShowFocusRing = false
		d.clearBtn.Base().Role = "button"
		d.clearBtn.Base().Label = "clear"
		d.clearBtn.Click = func() { d.Clear() }
		clearNode = d.clearBtn
	} else {
		d.clearBtn = nil
	}

	d.suffix = primitive.NewIcon("calendar")
	d.suffix.Size = 14
	sec := th.Color(core.TokenColorTextSecondary)
	if sec.A < 0.3 {
		sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	d.suffix.Color = sec

	rowKids := []core.Node{d.display, primitive.Spacer()}
	if clearNode != nil {
		rowKids = append(rowKids, clearNode)
	}
	rowKids = append(rowKids, d.suffix)
	row := primitive.Row(rowKids...)
	row.CrossAlign = core.CrossCenter
	row.Gap = 8

	d.decor = primitive.NewDecorated(row)
	d.decor.Padding = primitive.Symmetric(padH, 0)
	d.decor.Radius = radius
	d.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	d.decor.MinHeight = h
	d.decor.Height = h
	w := d.FixedWidth
	if w <= 0 {
		w = DefaultDatePickerWidth
		if d.Range || d.Multiple {
			w = 320
		}
	}
	d.decor.MinWidth = DefaultDatePickerMinWidth
	d.decor.Width = w
	d.decor.SetCenterContent(true)
	d.decor.StretchChild = true
	d.applyChrome()

	d.body = primitive.Column()
	d.body.Gap = 8
	d.body.CrossAlign = core.CrossStart
	d.panel = primitive.NewDecorated(d.body)
	d.panel.Padding = primitive.All(DefaultDatePickerPanelPad)
	d.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultDatePickerPanelRadius)
	d.panel.Background = th.Color(core.TokenColorBgContainer)
	d.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	d.panel.BorderColor = th.Color(core.TokenColorBorder)
	d.panel.MinWidth = 280
	d.panel.Base().Role = "dialog"
	d.panel.Base().Label = "DatePicker panel"

	if d.popup == nil {
		d.popup = primitive.NewAnchoredPopup(d.panel)
	} else {
		d.popup.Content = d.panel
	}
	d.popup.Placement = mapDatePickerPlacement(d.Placement)
	d.popup.Gap = DefaultDatePickerGap
	d.popup.DismissOnOutside = true
	d.popup.OnDismiss = func() {
		d.Open = false
		d.CancelPending()
		if d.OnOpenChange != nil {
			d.OnOpenChange(false)
		}
		d.applyChrome()
	}

	if d.Root == nil {
		d.Root = primitive.NewPressable(d.decor)
	} else {
		d.Root.ClearChildren()
		d.Root.AddChild(d.decor)
	}
	d.Root.Focusable = true
	d.Root.ShowFocusRing = true
	d.Root.FocusRingRadius = radius
	d.Root.FocusRingOutset = DefaultDatePickerFocusOutset
	d.Root.SetDisabled(d.Disabled)
	d.Root.OnStateChange = func() { d.applyChrome() }
	d.Root.Click = func() {
		if d.Disabled {
			return
		}
		d.applyOpen(!d.Open, false)
	}
	d.applyA11y()

	if d.Wrap == nil {
		d.Wrap = primitive.Column(d.Root, d.popup)
	} else {
		d.Wrap.ClearChildren()
		d.Wrap.AddChild(d.Root)
		d.Wrap.AddChild(d.popup)
	}
	d.Wrap.CrossAlign = core.CrossStart
	d.Wrap.SetThemeHook(func(*core.Theme) { d.rebuild() })

	if d.defaultOpenSet && !d.appliedDefault && !d.openControlled {
		d.appliedDefault = true
		if d.defaultOpen {
			d.Open = true
		}
	}

	d.rebuildPanelBody()
	if d.Open {
		d.syncPopupGeometry()
		if d.popup != nil {
			d.popup.SetOpen(true)
		}
	}
	d.Wrap.MarkNeedsLayout()
	d.Wrap.MarkNeedsPaint()
}

func (d *DatePicker) applyChrome() {
	if d == nil || d.decor == nil {
		return
	}
	th := d.theme()
	h := d.controlHeight()
	d.decor.MinHeight = h
	d.decor.Height = h
	d.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	d.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	switch d.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		d.decor.BorderWidth = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		d.decor.BorderWidth = 0
	case InputUnderlined:
		bg = render.RGBA{}
		d.decor.Radius = 0
	}

	if d.Disabled {
		dbg := th.Color(core.TokenColorDisabledBg)
		if dbg.A < 0.05 {
			dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bg = dbg
		bd = th.Color(core.TokenColorBorder)
		d.decor.Background = bg
		d.decor.BorderColor = bd
		if d.display != nil {
			d.display.Color = d.valueColor(th)
		}
		d.decor.MarkNeedsPaint()
		return
	}

	switch d.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
	default:
		if d.Root != nil && (d.Root.State.Focused || d.Open) {
			bd = th.Color(core.TokenColorPrimary)
		} else if d.Root != nil && d.Root.State.Hovered {
			hb := th.Color(core.TokenColorBorderHover)
			if hb.A < 0.5 {
				hb = th.Color(core.TokenColorPrimaryHover)
			}
			bd = hb
		}
	}
	if d.Variant == InputUnderlined {
		d.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	}
	d.decor.Background = bg
	d.decor.BorderColor = bd
	if d.display != nil {
		if d.computeDisplay() == "" {
			d.display.Color = d.placeholderColor(th)
		} else {
			d.display.Color = d.valueColor(th)
		}
	}
	d.decor.MarkNeedsPaint()
}

func (d *DatePicker) applyA11y() {
	if d == nil || d.Root == nil {
		return
	}
	d.Root.Base().Role = "combobox"
	label := d.AriaLabel
	if label == "" {
		label = d.Placeholder
	}
	if label == "" {
		if d.Range {
			label = "RangePicker"
		} else {
			label = "DatePicker"
		}
	}
	// status=error exposed via label suffix for screen-reader-ish hosts.
	if d.Status == InputStatusError {
		label = label + " invalid"
	}
	d.Root.Base().Label = label
}

func (d *DatePicker) rebuildPanelBody() {
	if d == nil || d.body == nil {
		return
	}
	d.body.ClearChildren()
	th := d.theme()
	d.body.AddChild(d.buildHeader(th))
	switch d.Picker {
	case DatePickerMonth:
		d.body.AddChild(d.buildMonthGrid(th))
	case DatePickerQuarter:
		d.body.AddChild(d.buildQuarterGrid(th))
	case DatePickerYear:
		d.body.AddChild(d.buildYearGrid(th))
	case DatePickerWeek:
		d.body.AddChild(d.buildDayGrid(th, true))
	default:
		d.body.AddChild(d.buildDayGrid(th, false))
	}
	if d.ShowTime && d.Picker == DatePickerDate {
		d.body.AddChild(d.buildTimeRow(th))
	}
	if d.NeedConfirm {
		d.body.AddChild(d.buildConfirmFooter(th))
	}
	d.body.MarkNeedsLayout()
	d.body.MarkNeedsPaint()
	if d.panel != nil {
		d.panel.MarkNeedsLayout()
		d.panel.MarkNeedsPaint()
	}
}

func (d *DatePicker) buildHeader(th *core.Theme) core.Node {
	title := ""
	switch d.Picker {
	case DatePickerYear:
		base := (d.panelYear / 10) * 10
		title = fmt.Sprintf("%d – %d", base, base+9)
	case DatePickerQuarter, DatePickerMonth:
		title = fmt.Sprintf("%d", d.panelYear)
	default:
		title = fmt.Sprintf("%d-%02d", d.panelYear, int(d.panelMonth))
	}
	lab := primitive.NewText(title)
	lab.FontSize = 14
	lab.Face = d.Face
	lab.Color = th.Color(core.TokenColorText)

	mkNav := func(label string, fn func()) *primitive.Pressable {
		t := primitive.NewText(label)
		t.FontSize = 14
		t.Face = d.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width, dec.Height = 28, 28
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		p.ColorHovered = antItemHoverFill(th)
		p.Click = fn
		return p
	}

	prev := mkNav("‹", func() { d.shiftPanel(-1) })
	next := mkNav("›", func() { d.shiftPanel(1) })
	prevY := mkNav("«", func() { d.shiftPanelYear(-1) })
	nextY := mkNav("»", func() { d.shiftPanelYear(1) })

	row := primitive.Row(prevY, prev, primitive.Spacer(), lab, primitive.Spacer(), next, nextY)
	row.CrossAlign = core.CrossCenter
	row.Gap = 4
	return row
}

func (d *DatePicker) shiftPanel(delta int) {
	switch d.Picker {
	case DatePickerYear:
		d.panelYear += delta * 10
	case DatePickerMonth, DatePickerQuarter:
		d.panelYear += delta
	default:
		m := int(d.panelMonth) + delta
		y := d.panelYear
		for m < 1 {
			m += 12
			y--
		}
		for m > 12 {
			m -= 12
			y++
		}
		d.panelYear, d.panelMonth = y, time.Month(m)
	}
	d.rebuildPanelBody()
}

func (d *DatePicker) shiftPanelYear(delta int) {
	switch d.Picker {
	case DatePickerYear:
		d.panelYear += delta * 10
	default:
		d.panelYear += delta
	}
	d.rebuildPanelBody()
}

func (d *DatePicker) buildDayGrid(th *core.Theme, weekMode bool) core.Node {
	col := primitive.Column()
	col.Gap = 2
	col.CrossAlign = core.CrossStart

	head := primitive.Row()
	head.Gap = 0
	for _, name := range []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"} {
		lab := primitive.NewText(name)
		lab.FontSize = 12
		lab.Face = d.Face
		lab.Color = th.Color(core.TokenColorTextSecondary)
		box := primitive.NewDecorated(lab)
		box.Width, box.Height = DefaultDatePickerCell, 24
		box.SetCenterContent(true)
		box.StretchChild = true
		box.BorderWidth = 0
		head.AddChild(box)
	}
	col.AddChild(head)

	first := time.Date(d.panelYear, d.panelMonth, 1, 0, 0, 0, 0, time.UTC)
	startPad := int(first.Weekday())
	daysIn := daysInMonth(d.panelYear, d.panelMonth)

	cells := make([]dayCell, 0, 42)
	prevM := d.panelMonth - 1
	prevY := d.panelYear
	if prevM < 1 {
		prevM = 12
		prevY--
	}
	prevDays := daysInMonth(prevY, prevM)
	for i := 0; i < startPad; i++ {
		day := prevDays - startPad + 1 + i
		cells = append(cells, dayCell{v: DateOf(prevY, int(prevM), day), label: fmt.Sprintf("%d", day), inMon: false})
	}
	for day := 1; day <= daysIn; day++ {
		cells = append(cells, dayCell{v: DateOf(d.panelYear, int(d.panelMonth), day), label: fmt.Sprintf("%d", day), inMon: true})
	}
	nextM := d.panelMonth + 1
	nextY := d.panelYear
	if nextM > 12 {
		nextM = 1
		nextY++
	}
	for len(cells) < 42 {
		day := len(cells) - startPad - daysIn + 1
		cells = append(cells, dayCell{v: DateOf(nextY, int(nextM), day), label: fmt.Sprintf("%d", day), inMon: false})
	}

	for w := 0; w < 6; w++ {
		weekCells := cells[w*7 : w*7+7]
		if weekMode {
			wv := weekCells[0].v
			selected := d.isWeekSelected(wv)
			disabled := d.isDisabled(wv)
			rowInner := primitive.Row()
			rowInner.Gap = 0
			for _, c := range weekCells {
				t := primitive.NewText(c.label)
				t.FontSize = 13
				t.Face = d.Face
				if c.inMon {
					t.Color = th.Color(core.TokenColorText)
				} else {
					t.Color = th.Color(core.TokenColorTextSecondary)
				}
				if selected {
					t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
				}
				box := primitive.NewDecorated(t)
				box.Width, box.Height = DefaultDatePickerCell, DefaultDatePickerCell
				box.Radius = 4
				box.SetCenterContent(true)
				box.StretchChild = true
				box.BorderWidth = 0
				if selected {
					box.Background = th.Color(core.TokenColorPrimary)
				}
				rowInner.AddChild(box)
			}
			p := primitive.NewPressable(rowInner)
			p.ShowFocusRing = false
			if disabled {
				p.SetDisabled(true)
			} else {
				p.ColorHovered = antItemHoverFill(th)
				pick := wv
				p.Click = func() { d.pickValue(pick) }
			}
			col.AddChild(p)
			continue
		}
		row := primitive.Row()
		row.Gap = 0
		for _, c := range weekCells {
			c := c
			selected := d.isSelected(c.v)
			inRange := d.inRangeHighlight(c.v)
			disabled := d.isDisabled(c.v)
			t := primitive.NewText(c.label)
			t.FontSize = 13
			t.Face = d.Face
			if disabled {
				t.Color = th.Color(core.TokenColorDisabledText)
			} else if !c.inMon {
				t.Color = th.Color(core.TokenColorTextSecondary)
			} else {
				t.Color = th.Color(core.TokenColorText)
			}
			dec := primitive.NewDecorated(t)
			dec.Width, dec.Height = DefaultDatePickerCell, DefaultDatePickerCell
			dec.Radius = 4
			dec.SetCenterContent(true)
			dec.StretchChild = true
			dec.BorderWidth = 0
			if selected {
				dec.Background = th.Color(core.TokenColorPrimary)
				t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			} else if inRange {
				dec.Background = antItemSelectedFill(th)
			}
			if c.inMon && c.v.EqualDate(Today()) && !selected {
				dec.BorderWidth = 1
				dec.BorderColor = th.Color(core.TokenColorPrimary)
			}
			p := primitive.NewPressable(dec)
			p.ShowFocusRing = false
			if disabled {
				p.SetDisabled(true)
			} else {
				p.ColorHovered = antItemHoverFill(th)
				p.Click = func() {
					v := c.v
					if d.ShowTime {
						v = DateTimeOf(v.Year, v.Month, v.Day, d.draftHour, d.draftMin, d.draftSec)
					}
					d.pickValue(v)
				}
			}
			row.AddChild(p)
		}
		col.AddChild(row)
	}
	return col
}

func (d *DatePicker) buildMonthGrid(th *core.Theme) core.Node {
	names := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	col := primitive.Column()
	col.Gap = 4
	var row *primitive.Flex
	for i, name := range names {
		if i%3 == 0 {
			row = primitive.Row()
			row.Gap = 4
			col.AddChild(row)
		}
		m := i + 1
		v := DateOf(d.panelYear, m, 1)
		selected := d.isSelected(v)
		disabled := d.isDisabled(v)
		t := primitive.NewText(name)
		t.FontSize = 13
		t.Face = d.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width, dec.Height = 80, 40
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		if selected {
			dec.Background = th.Color(core.TokenColorPrimary)
			t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		if disabled {
			p.SetDisabled(true)
			t.Color = th.Color(core.TokenColorDisabledText)
		} else {
			p.ColorHovered = antItemHoverFill(th)
			pick := v
			p.Click = func() { d.pickValue(pick) }
		}
		row.AddChild(p)
	}
	return col
}

func (d *DatePicker) buildQuarterGrid(th *core.Theme) core.Node {
	row := primitive.Row()
	row.Gap = 8
	for q := 1; q <= 4; q++ {
		month := (q-1)*3 + 1
		v := DateOf(d.panelYear, month, 1)
		selected := d.isSelected(v)
		disabled := d.isDisabled(v)
		t := primitive.NewText(fmt.Sprintf("Q%d", q))
		t.FontSize = 14
		t.Face = d.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width, dec.Height = 64, 40
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		if selected {
			dec.Background = th.Color(core.TokenColorPrimary)
			t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		if disabled {
			p.SetDisabled(true)
			t.Color = th.Color(core.TokenColorDisabledText)
		} else {
			p.ColorHovered = antItemHoverFill(th)
			pick := v
			p.Click = func() { d.pickValue(pick) }
		}
		row.AddChild(p)
	}
	return row
}

func (d *DatePicker) buildYearGrid(th *core.Theme) core.Node {
	base := (d.panelYear / 10) * 10
	col := primitive.Column()
	col.Gap = 4
	var row *primitive.Flex
	for i := 0; i < 12; i++ {
		if i%3 == 0 {
			row = primitive.Row()
			row.Gap = 4
			col.AddChild(row)
		}
		y := base + i - 1 // show one year before decade for antd-like
		v := DateOf(y, 1, 1)
		selected := d.isSelected(v)
		disabled := d.isDisabled(v)
		t := primitive.NewText(fmt.Sprintf("%d", y))
		t.FontSize = 13
		t.Face = d.Face
		t.Color = th.Color(core.TokenColorText)
		dec := primitive.NewDecorated(t)
		dec.Width, dec.Height = 80, 40
		dec.Radius = 4
		dec.SetCenterContent(true)
		dec.StretchChild = true
		dec.BorderWidth = 0
		if selected {
			dec.Background = th.Color(core.TokenColorPrimary)
			t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		p := primitive.NewPressable(dec)
		p.ShowFocusRing = false
		if disabled {
			p.SetDisabled(true)
			t.Color = th.Color(core.TokenColorDisabledText)
		} else {
			p.ColorHovered = antItemHoverFill(th)
			pick := v
			p.Click = func() { d.pickValue(pick) }
		}
		row.AddChild(p)
	}
	return col
}

func (d *DatePicker) buildTimeRow(th *core.Theme) core.Node {
	mkCol := func(label string, max int, cur int, set func(int)) core.Node {
		col := primitive.Column()
		col.Gap = 2
		cap := primitive.NewText(label)
		cap.FontSize = 11
		cap.Face = d.Face
		cap.Color = th.Color(core.TokenColorTextSecondary)
		col.AddChild(cap)
		// show a compact set of options around current for P0
		start := cur - 2
		if start < 0 {
			start = 0
		}
		end := start + 5
		if end > max {
			end = max
			start = end - 5
			if start < 0 {
				start = 0
			}
		}
		for i := start; i < end; i++ {
			i := i
			t := primitive.NewText(fmt.Sprintf("%02d", i))
			t.FontSize = 12
			t.Face = d.Face
			t.Color = th.Color(core.TokenColorText)
			dec := primitive.NewDecorated(t)
			dec.Width, dec.Height = 40, 24
			dec.Radius = 4
			dec.SetCenterContent(true)
			dec.StretchChild = true
			dec.BorderWidth = 0
			if i == cur {
				dec.Background = th.Color(core.TokenColorPrimary)
				t.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			}
			p := primitive.NewPressable(dec)
			p.ShowFocusRing = false
			p.ColorHovered = antItemHoverFill(th)
			p.Click = func() {
				set(i)
				// update pending/value time
				if d.hasPending && d.pending.Valid {
					d.pending.Hour, d.pending.Minute, d.pending.Second = d.draftHour, d.draftMin, d.draftSec
					d.pending.HasTime = true
				} else if d.Value.Valid && !d.NeedConfirm {
					d.Value.Hour, d.Value.Minute, d.Value.Second = d.draftHour, d.draftMin, d.draftSec
					d.Value.HasTime = true
					d.fireChange(d.Value)
					d.refreshDisplay()
				}
				d.rebuildPanelBody()
			}
			col.AddChild(p)
		}
		return col
	}
	row := primitive.Row(
		mkCol("HH", 24, d.draftHour, func(v int) { d.draftHour = v }),
		mkCol("mm", 60, d.draftMin, func(v int) { d.draftMin = v }),
		mkCol("ss", 60, d.draftSec, func(v int) { d.draftSec = v }),
	)
	row.Gap = 8
	row.CrossAlign = core.CrossStart
	return row
}

func (d *DatePicker) buildConfirmFooter(th *core.Theme) core.Node {
	lab := primitive.NewText("OK")
	lab.FontSize = 13
	lab.Face = d.Face
	lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	dec := primitive.NewDecorated(lab)
	dec.Padding = primitive.Symmetric(12, 4)
	dec.Radius = 4
	dec.Background = th.Color(core.TokenColorPrimary)
	dec.SetCenterContent(true)
	dec.StretchChild = true
	dec.BorderWidth = 0
	btn := primitive.NewPressable(dec)
	btn.ShowFocusRing = false
	btn.Base().Role = "button"
	btn.Base().Label = "OK"
	btn.Click = func() { d.Confirm() }
	row := primitive.Row(primitive.Spacer(), btn)
	row.CrossAlign = core.CrossCenter
	return row
}

func (d *DatePicker) isSelected(v DateValue) bool {
	if !v.Valid {
		return false
	}
	if d.Range {
		start, end := d.effectiveRange()
		return (start.Valid && start.EqualDate(v)) || (end.Valid && end.EqualDate(v))
	}
	if d.Multiple {
		for _, m := range d.MultiValue {
			if m.EqualDate(v) {
				return true
			}
		}
		return false
	}
	cur := d.Value
	if d.hasPending && d.NeedConfirm {
		cur = d.pending
	}
	switch d.Picker {
	case DatePickerMonth:
		return cur.Valid && cur.Year == v.Year && cur.Month == v.Month
	case DatePickerQuarter:
		return cur.Valid && cur.Year == v.Year && quarterOf(cur.Month) == quarterOf(v.Month)
	case DatePickerYear:
		return cur.Valid && cur.Year == v.Year
	case DatePickerWeek:
		return d.isWeekSelected(v)
	default:
		return cur.Valid && cur.EqualDate(v)
	}
}

func (d *DatePicker) isWeekSelected(v DateValue) bool {
	cur := d.Value
	if d.hasPending && d.NeedConfirm {
		cur = d.pending
	}
	if !cur.Valid || !v.Valid {
		return false
	}
	return weekOfYear(cur) == weekOfYear(v) && cur.Year == v.Year
}

func (d *DatePicker) effectiveRange() (start, end DateValue) {
	if d.hasPending && d.NeedConfirm {
		return d.pendingStart, d.pendingEnd
	}
	return d.RangeStart, d.RangeEnd
}

func (d *DatePicker) inRangeHighlight(v DateValue) bool {
	if !d.Range || !v.Valid {
		return false
	}
	start, end := d.effectiveRange()
	if !start.Valid || !end.Valid {
		return false
	}
	if end.Before(start) {
		start, end = end, start
	}
	// exclusive of ends (ends already selected)
	if v.EqualDate(start) || v.EqualDate(end) {
		return false
	}
	return !v.Before(start) && v.Before(end) || (start.Before(v) && v.Before(end)) || (!v.Time().Before(start.Time()) && !v.Time().After(end.Time()) && !v.EqualDate(start) && !v.EqualDate(end))
}

func (d *DatePicker) pickValue(v DateValue) {
	if d == nil || d.Disabled || !v.Valid || d.isDisabled(v) {
		return
	}
	if d.ShowTime && d.Picker == DatePickerDate {
		if v.HasTime {
			d.draftHour, d.draftMin, d.draftSec = v.Hour, v.Minute, v.Second
		} else {
			v.Hour, v.Minute, v.Second = d.draftHour, d.draftMin, d.draftSec
			v.HasTime = true
		}
	}

	if d.Range {
		d.pickRange(v)
		return
	}
	if d.Multiple {
		d.toggleMulti(v)
		return
	}
	if d.NeedConfirm {
		d.pending = v
		d.hasPending = true
		d.rebuildPanelBody()
		d.refreshDisplay() // preview optional — keep committed display until OK
		return
	}
	d.Value = v
	d.fireChange(v)
	d.refreshDisplay()
	if !d.openControlled {
		d.applyOpen(false, false)
	} else {
		d.rebuildPanelBody()
	}
}

func (d *DatePicker) pickRange(v DateValue) {
	if !d.rangePicking || !d.RangeStart.Valid {
		// first click
		if d.NeedConfirm {
			d.pendingStart, d.pendingEnd = v, DateValue{}
			d.hasPending = true
		} else {
			d.RangeStart, d.RangeEnd = v, DateValue{}
		}
		d.rangePicking = true
		d.refreshDisplay()
		d.rebuildPanelBody()
		return
	}
	// second click
	start := d.RangeStart
	if d.NeedConfirm {
		start = d.pendingStart
	}
	end := v
	if d.Order && end.Before(start) {
		start, end = end, start
	}
	if d.NeedConfirm {
		d.pendingStart, d.pendingEnd = start, end
		d.hasPending = true
		d.rangePicking = false
		d.rebuildPanelBody()
		return
	}
	d.RangeStart, d.RangeEnd = start, end
	d.rangePicking = false
	d.fireRangeChange(start, end)
	d.refreshDisplay()
	if !d.openControlled {
		d.applyOpen(false, false)
	} else {
		d.rebuildPanelBody()
	}
}

func (d *DatePicker) toggleMulti(v DateValue) {
	idx := -1
	for i, m := range d.MultiValue {
		if m.EqualDate(v) {
			idx = i
			break
		}
	}
	if idx >= 0 {
		d.MultiValue = append(d.MultiValue[:idx], d.MultiValue[idx+1:]...)
	} else {
		d.MultiValue = append(d.MultiValue, v)
		if d.Order {
			d.sortMulti()
		}
	}
	if d.NeedConfirm {
		d.hasPending = true
		d.rebuildPanelBody()
		d.refreshDisplay()
		return
	}
	d.fireMultiChange()
	d.refreshDisplay()
	d.rebuildPanelBody()
}

func (d *DatePicker) sortMulti() {
	sort.SliceStable(d.MultiValue, func(i, j int) bool {
		return d.MultiValue[i].Before(d.MultiValue[j])
	})
}

func (d *DatePicker) fireChange(v DateValue) {
	if d.OnChange != nil {
		d.OnChange(v, d.formatOne(v))
	}
}

func (d *DatePicker) fireRangeChange(start, end DateValue) {
	if d.OnChangeRange != nil {
		d.OnChangeRange(start, end, [2]string{d.formatOne(start), d.formatOne(end)})
	}
	// also fire OnChange with end for simple listeners
	if d.OnChange != nil {
		d.OnChange(end, d.formatOne(end))
	}
}

func (d *DatePicker) fireMultiChange() {
	if d.OnChangeMulti != nil {
		ss := make([]string, len(d.MultiValue))
		for i, v := range d.MultiValue {
			ss[i] = d.formatOne(v)
		}
		d.OnChangeMulti(append([]DateValue(nil), d.MultiValue...), ss)
	}
	if d.OnChange != nil {
		if len(d.MultiValue) > 0 {
			last := d.MultiValue[len(d.MultiValue)-1]
			d.OnChange(last, d.formatOne(last))
		} else {
			d.OnChange(DateValue{}, "")
		}
	}
}

func mapDatePickerPlacement(p DatePickerPlacement) primitive.Placement {
	switch p {
	case DatePickerBottomRight:
		return primitive.PlaceBottomEnd
	case DatePickerTopLeft:
		return primitive.PlaceTopStart
	case DatePickerTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}
