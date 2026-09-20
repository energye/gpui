package kit

// DateEngineGenerateConfig is the pluggable date implementation
// (antd GenerateConfig<DateType>). Components only talk to this
// interface, so swapping the date library never touches component
// code. The std implementation below is pure civil math with no
// timezone database.
type DateEngineGenerateConfig interface {
	// Name identifies the implementation (e.g. "std").
	Name() string
	// DaysInMonth returns the month length (leap-aware).
	DaysInMonth(year, month int) int
	// IsLeapYear reports Gregorian leap years.
	IsLeapYear(year int) bool
	// DayOfWeek returns 0=Sunday..6=Saturday (proleptic Gregorian).
	DayOfWeek(d DateEngineDate) int
	// Compare orders full datetimes: -1, 0, +1.
	Compare(a, b DateEngineDate) int
	// AddDays shifts by days across month/year bounds.
	AddDays(d DateEngineDate, n int) DateEngineDate
	// AddMonths shifts by months, clamping the day.
	AddMonths(d DateEngineDate, n int) DateEngineDate
	// AddYears shifts by years, clamping Feb 29.
	AddYears(d DateEngineDate, n int) DateEngineDate
	// AddQuarters shifts by quarters.
	AddQuarters(d DateEngineDate, n int) DateEngineDate
	// StartOfDay/Month/Year truncate the clock or calendar.
	StartOfDay(d DateEngineDate) DateEngineDate
	StartOfMonth(d DateEngineDate) DateEngineDate
	StartOfYear(d DateEngineDate) DateEngineDate
	// QuarterOf returns 1..4.
	QuarterOf(d DateEngineDate) int
	// WeekNumber returns the week-of-year label for weekStart.
	WeekNumber(d DateEngineDate, weekStart int) int
}

// StdDateEngineGenerateConfig is the default pure-Go implementation.
type StdDateEngineGenerateConfig struct{}

// DefaultDateEngineGenerateConfig returns the std implementation.
func DefaultDateEngineGenerateConfig() DateEngineGenerateConfig {
	return StdDateEngineGenerateConfig{}
}

// Name identifies the std implementation.
func (StdDateEngineGenerateConfig) Name() string { return "std" }

// StdDateEngineIsLeapYear is the shared leap rule (also used by IsValid).
func StdDateEngineIsLeapYear(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

// StdDateEngineDaysInMonth is the shared month length.
func StdDateEngineDaysInMonth(y, m int) int {
	switch m {
	case 1, 3, 5, 7, 8, 10, 12:
		return 31
	case 4, 6, 9, 11:
		return 30
	case 2:
		if StdDateEngineIsLeapYear(y) {
			return 29
		}
		return 28
	default:
		return 0
	}
}

// IsLeapYear reports Gregorian leap years.
func (StdDateEngineGenerateConfig) IsLeapYear(y int) bool {
	return StdDateEngineIsLeapYear(y)
}

// DaysInMonth returns the month length (leap-aware).
func (StdDateEngineGenerateConfig) DaysInMonth(y, m int) int {
	return StdDateEngineDaysInMonth(y, m)
}

// daysFromCivil counts days since 1970-01-01 (Howard Hinnant).
func daysFromCivil(y, m, d int) int {
	if m <= 2 {
		y--
		m += 12
	}
	era := 0
	if y >= 0 {
		era = y / 400
	} else {
		era = (y - 399) / 400
	}
	yoe := y - era*400
	doy := (153*(m-3)+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

// civilFromDays inverts daysFromCivil.
func civilFromDays(z int) (y, m, d int) {
	z += 719468
	era := 0
	if z >= 0 {
		era = z / 146097
	} else {
		era = (z - 146096) / 146097
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	if mp < 10 {
		m = mp + 3
	} else {
		m = mp - 9
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

// DayOfWeek returns 0=Sunday..6=Saturday.
func (StdDateEngineGenerateConfig) DayOfWeek(d DateEngineDate) int {
	days := daysFromCivil(d.Year, d.Month, d.Day)
	w := (days + 4) % 7
	if w < 0 {
		w += 7
	}
	return w
}

// Compare orders full datetimes: -1, 0, +1.
func (StdDateEngineGenerateConfig) Compare(a, b DateEngineDate) int {
	for _, pair := range [][2]int{
		{a.Year, b.Year}, {a.Month, b.Month}, {a.Day, b.Day},
		{a.Hour, b.Hour}, {a.Minute, b.Minute}, {a.Second, b.Second},
	} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// AddDays shifts by days across month/year bounds.
func (c StdDateEngineGenerateConfig) AddDays(d DateEngineDate, n int) DateEngineDate {
	y, m, day := civilFromDays(daysFromCivil(d.Year, d.Month, d.Day) + n)
	d.Year, d.Month, d.Day = y, m, day
	return d
}

// AddMonths shifts by months, clamping the day.
func (c StdDateEngineGenerateConfig) AddMonths(d DateEngineDate, n int) DateEngineDate {
	total := (d.Year*12 + (d.Month - 1)) + n
	y := total / 12
	m := total%12 + 1
	if m <= 0 {
		m += 12
		y--
	}
	dim := c.DaysInMonth(y, m)
	if d.Day > dim {
		d.Day = dim
	}
	d.Year, d.Month = y, m
	return d
}

// AddYears shifts by years, clamping Feb 29.
func (c StdDateEngineGenerateConfig) AddYears(d DateEngineDate, n int) DateEngineDate {
	return c.AddMonths(d, n*12)
}

// AddQuarters shifts by quarters.
func (c StdDateEngineGenerateConfig) AddQuarters(d DateEngineDate, n int) DateEngineDate {
	return c.AddMonths(d, n*3)
}

// StartOfDay truncates the clock.
func (StdDateEngineGenerateConfig) StartOfDay(d DateEngineDate) DateEngineDate {
	d.Hour, d.Minute, d.Second = 0, 0, 0
	return d
}

// StartOfMonth truncates to the first day.
func (StdDateEngineGenerateConfig) StartOfMonth(d DateEngineDate) DateEngineDate {
	d.Day = 1
	d.Hour, d.Minute, d.Second = 0, 0, 0
	return d
}

// StartOfYear truncates to January first.
func (StdDateEngineGenerateConfig) StartOfYear(d DateEngineDate) DateEngineDate {
	d.Month, d.Day = 1, 1
	d.Hour, d.Minute, d.Second = 0, 0, 0
	return d
}

// QuarterOf returns 1..4.
func (StdDateEngineGenerateConfig) QuarterOf(d DateEngineDate) int {
	return (d.Month-1)/3 + 1
}

// WeekNumber returns the week-of-year label: week 1 holds Jan 1,
// weeks run weekStart..weekStart+6.
func (c StdDateEngineGenerateConfig) WeekNumber(d DateEngineDate, weekStart int) int {
	if weekStart < 0 || weekStart > 6 {
		weekStart = 0
	}
	jan1 := DateEngineDate{Year: d.Year, Month: 1, Day: 1}
	first := (c.DayOfWeek(jan1) - weekStart + 7) % 7
	doy := daysFromCivil(d.Year, d.Month, d.Day) - daysFromCivil(d.Year, 1, 1)
	return (doy+first)/7 + 1
}
