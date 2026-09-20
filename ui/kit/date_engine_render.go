package kit

// DateEngineCell is one panel unit.
type DateEngineCell struct {
	Date     DateEngineDate
	InView   bool
	Disabled bool
	IsToday  bool
	Selected bool
	InRange  bool
	RangeEnd bool
	Label    string
}

// MonthDateEngineMatrix returns the 6x7 date panel grid for year/month.
// Weeks run weekStart..weekStart+6; leading/trailing days fill the grid.
func MonthDateEngineMatrix(gen DateEngineGenerateConfig, year, month, weekStart int) [6][7]DateEngineDate {
	if gen == nil {
		gen = DefaultDateEngineGenerateConfig()
	}
	var grid [6][7]DateEngineDate
	first := DateEngineDate{Year: year, Month: month, Day: 1}
	lead := (gen.DayOfWeek(first) - weekStart + 7) % 7
	start := gen.AddDays(first, -lead)
	for i := 0; i < 42; i++ {
		d := gen.AddDays(start, i)
		grid[i/7][i%7] = d
	}
	return grid
}

// MonthDateEngineCells decorates the matrix with view/disabled/selection.
func MonthDateEngineCells(in *DateEngineInstance, year, month int) [6][7]DateEngineCell {
	gen := DefaultDateEngineGenerateConfig()
	ws := 0
	var rng [2]*DateEngineDate
	var sel *DateEngineDate
	if in != nil {
		in.mu.Lock()
		gen = in.gen
		ws = ResolveDateEngineWeekStart(in.props, in.localeLocked())
		rng = in.rangeV
		sel = in.value
		in.mu.Unlock()
	}
	matrix := MonthDateEngineMatrix(gen, year, month, ws)
	today := DateEngineToday(gen)
	var out [6][7]DateEngineCell
	for r := 0; r < 6; r++ {
		for c := 0; c < 7; c++ {
			d := matrix[r][c]
			cell := DateEngineCell{
				Date:    d,
				InView:  d.Month == month,
				IsToday: d.SameDay(today),
				Label:   itoaDateEngine(d.Day),
			}
			if in != nil {
				cell.Disabled = in.IsDisabledDate(d)
			}
			if sel != nil && d.SameDay(*sel) {
				cell.Selected = true
			}
			if rng[0] != nil && rng[1] != nil {
				lo, hi := rng[0], rng[1]
				if gen.Compare(*lo, *hi) > 0 {
					lo, hi = hi, lo
				}
				day := gen.StartOfDay(d)
				if gen.Compare(day, gen.StartOfDay(*lo)) >= 0 && gen.Compare(day, gen.StartOfDay(*hi)) <= 0 {
					cell.InRange = true
				}
				if d.SameDay(*rng[0]) || d.SameDay(*rng[1]) {
					cell.RangeEnd = true
				}
			}
			out[r][c] = cell
		}
	}
	return out
}

// QuarterDateEngineCells returns the 4 quarter cells of a year.
func QuarterDateEngineCells(year int) [4]DateEngineDate {
	return [4]DateEngineDate{
		{Year: year, Month: 1, Day: 1},
		{Year: year, Month: 4, Day: 1},
		{Year: year, Month: 7, Day: 1},
		{Year: year, Month: 10, Day: 1},
	}
}

// YearMonthDateEngineCells returns the 12 month cells of a year.
func YearMonthDateEngineCells(year int) [12]DateEngineDate {
	var out [12]DateEngineDate
	for m := 1; m <= 12; m++ {
		out[m-1] = DateEngineDate{Year: year, Month: m, Day: 1}
	}
	return out
}

// YearDateEngineCells returns the 10 year cells of a decade.
func YearDateEngineCells(decadeStart int) [10]DateEngineDate {
	var out [10]DateEngineDate
	for i := 0; i < 10; i++ {
		out[i] = DateEngineDate{Year: decadeStart + i, Month: 1, Day: 1}
	}
	return out
}

// DecadeDateEngineCells returns the 10 decade cells of a century.
func DecadeDateEngineCells(centuryStart int) [10][2]int {
	var out [10][2]int
	for i := 0; i < 10; i++ {
		out[i] = [2]int{centuryStart + i*10, centuryStart + i*10 + 9}
	}
	return out
}

// DecadeStartOf returns the decade floor of a year.
func DecadeStartOf(year int) int {
	return year / 10 * 10
}

// CenturyStartOf returns the century floor of a year.
func CenturyStartOf(year int) int {
	return year / 100 * 100
}

// ClampDateEnginePanel clamps a panel date into min/max bounds.
func ClampDateEnginePanel(gen DateEngineGenerateConfig, d DateEngineDate, minD, maxD *DateEngineDate) DateEngineDate {
	if gen == nil {
		gen = DefaultDateEngineGenerateConfig()
	}
	if minD != nil {
		floor := gen.StartOfMonth(*minD)
		if gen.Compare(gen.StartOfMonth(d), floor) < 0 {
			return floor
		}
	}
	if maxD != nil {
		ceil := gen.StartOfMonth(*maxD)
		if gen.Compare(gen.StartOfMonth(d), ceil) > 0 {
			return ceil
		}
	}
	return d
}

func itoaDateEngine(v int) string {
	if v == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
