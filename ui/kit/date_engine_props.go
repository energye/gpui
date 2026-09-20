package kit

// DateEnginePicker selects the selector family.
type DateEnginePicker string

const (
	DateEnginePickerDate     DateEnginePicker = "date"
	DateEnginePickerWeek     DateEnginePicker = "week"
	DateEnginePickerMonth    DateEnginePicker = "month"
	DateEnginePickerQuarter  DateEnginePicker = "quarter"
	DateEnginePickerYear     DateEnginePicker = "year"
	DateEnginePickerTime     DateEnginePicker = "time"
	DateEnginePickerDatetime DateEnginePicker = "datetime"
)

// DateEngineMode selects the visible panel.
type DateEngineMode string

const (
	DateEngineModeTime   DateEngineMode = "time"
	DateEngineModeDate   DateEngineMode = "date"
	DateEngineModeMonth  DateEngineMode = "month"
	DateEngineModeYear   DateEngineMode = "year"
	DateEngineModeDecade DateEngineMode = "decade"
)

// DateEngineDate is a proleptic Gregorian civil datetime.
// Zero value (Year == 0) means invalid / empty.
type DateEngineDate struct {
	Year   int
	Month  int
	Day    int
	Hour   int
	Minute int
	Second int
}

// DateOf builds a date at midnight.
func DateEngineDateOf(y, m, d int) DateEngineDate {
	return DateEngineDate{Year: y, Month: m, Day: d}
}

// IsValid reports whether the date holds a real civil value.
func (d DateEngineDate) IsValid() bool {
	return d.Year >= 1 && d.Year <= 9999 && d.Month >= 1 && d.Month <= 12 &&
		d.Day >= 1 && d.Day <= StdDateEngineDaysInMonth(d.Year, d.Month) &&
		d.Hour >= 0 && d.Hour <= 23 && d.Minute >= 0 && d.Minute <= 59 &&
		d.Second >= 0 && d.Second <= 59
}

// SameDay reports y/m/d equality ignoring the clock.
func (d DateEngineDate) SameDay(o DateEngineDate) bool {
	return d.Year == o.Year && d.Month == o.Month && d.Day == o.Day
}

// DateEngineDisabledTime lists unselectable clock units.
type DateEngineDisabledTime struct {
	Hours   []int
	Minutes []int
	Seconds []int
}

// DateEnginePreset is one shortcut entry. ValueFunc (5.8.0+ function
// values) wins over Value when both are set.
type DateEnginePreset struct {
	Label     string
	Value     []DateEngineDate
	ValueFunc func() []DateEngineDate
}

// Resolve returns the preset dates, calling ValueFunc when present.
func (p DateEnginePreset) Resolve() []DateEngineDate {
	if p.ValueFunc != nil {
		return p.ValueFunc()
	}
	return p.Value
}

// DateEngineProps configures one date engine host.
type DateEngineProps struct {
	Picker          DateEnginePicker
	Mode            DateEngineMode
	ModeSet         bool
	DefaultMode     DateEngineMode
	Locale          string
	WeekStart       int
	WeekStartSet    bool
	BuddhistEra     bool
	Formats         []string
	MinDate         *DateEngineDate
	MinDateSet      bool
	MaxDate         *DateEngineDate
	MaxDateSet      bool
	DisabledDate    func(current DateEngineDate, from *DateEngineDate, pickerType DateEnginePicker) bool
	DisabledTime    func(current DateEngineDate, part string) DateEngineDisabledTime
	Order           bool
	OrderSet        bool
	AllowEmpty      [2]bool
	Range           bool
	Multiple        bool
	Presets         []DateEnginePreset
	ShowTime        bool
	DefaultValue    *DateEngineDate
	DefaultRange    [2]*DateEngineDate
	DefaultMultiple []DateEngineDate
	DefaultPicker   *DateEngineDate
}

// DefaultDateEngineProps returns antd 6.5.1 aligned defaults.
func DefaultDateEngineProps() DateEngineProps {
	return DateEngineProps{
		Picker:       DateEnginePickerDate,
		Order:        true,
		OrderSet:     false,
		WeekStart:    -1,
		WeekStartSet: false,
	}
}

// ResolveDateEngineWeekStart returns explicit weekStart else locale
// default (zh* Monday=1, others Sunday=0).
func ResolveDateEngineWeekStart(props DateEngineProps, locale string) int {
	if props.WeekStartSet && props.WeekStart >= 0 && props.WeekStart <= 6 {
		return props.WeekStart
	}
	if props.WeekStart >= 0 && props.WeekStart <= 6 && props.WeekStartSet {
		return props.WeekStart
	}
	if len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z') {
		return 1
	}
	return 0
}

// ResolveDateEngineOrder returns explicit order else true.
func ResolveDateEngineOrder(props DateEngineProps) bool {
	if props.OrderSet {
		return props.Order
	}
	return true
}

// DisplayDateEngineFormat returns formats[0] else the picker default.
func DisplayDateEngineFormat(props DateEngineProps) string {
	if len(props.Formats) > 0 && props.Formats[0] != "" {
		return props.Formats[0]
	}
	switch props.Picker {
	case DateEnginePickerTime:
		return "HH:mm:ss"
	case DateEnginePickerDatetime:
		return "YYYY-MM-DD HH:mm:ss"
	case DateEnginePickerMonth:
		return "YYYY-MM"
	case DateEnginePickerQuarter:
		return "YYYY-[Q]Q"
	case DateEnginePickerYear:
		return "YYYY"
	case DateEnginePickerWeek:
		return "YYYY-wo"
	default:
		return "YYYY-MM-DD"
	}
}

// MatchDateEngineFormats returns the format list for parsing.
func MatchDateEngineFormats(props DateEngineProps) []string {
	if len(props.Formats) > 0 {
		return props.Formats
	}
	return []string{DisplayDateEngineFormat(props)}
}
