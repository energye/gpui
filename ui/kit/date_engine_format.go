package kit

import (
	"regexp"
	"strconv"
	"strings"
)

// dateEngineToken is one compiled format unit.
type dateEngineToken struct {
	kind  string
	width int
}

// splitDateEngineFormat cuts a format string into literal/token units.
// Tokens: YYYY BBBB Q wo MM M DD D HH mm ss; [...] quotes literals.
func splitDateEngineFormat(format string) []dateEngineToken {
	var out []dateEngineToken
	i := 0
	for i < len(format) {
		if format[i] == '[' {
			end := strings.IndexByte(format[i:], ']')
			if end < 0 {
				out = append(out, dateEngineToken{kind: "lit", width: len(format) - i})
				break
			}
			out = append(out, dateEngineToken{kind: "lit-" + format[i+1:i+end]})
			i += end + 1
			continue
		}
		rest := format[i:]
		switch {
		case strings.HasPrefix(rest, "YYYY"):
			out = append(out, dateEngineToken{kind: "YYYY", width: 4})
			i += 4
		case strings.HasPrefix(rest, "BBBB"):
			out = append(out, dateEngineToken{kind: "BBBB", width: 4})
			i += 4
		case strings.HasPrefix(rest, "MMMM"):
			out = append(out, dateEngineToken{kind: "MMMM"})
			i += 4
		case strings.HasPrefix(rest, "MMM"):
			out = append(out, dateEngineToken{kind: "MMM"})
			i += 3
		case strings.HasPrefix(rest, "MM"):
			out = append(out, dateEngineToken{kind: "MM"})
			i += 2
		case strings.HasPrefix(rest, "DD"):
			out = append(out, dateEngineToken{kind: "DD"})
			i += 2
		case strings.HasPrefix(rest, "HH"):
			out = append(out, dateEngineToken{kind: "HH"})
			i += 2
		case strings.HasPrefix(rest, "mm"):
			out = append(out, dateEngineToken{kind: "mm"})
			i += 2
		case strings.HasPrefix(rest, "ss"):
			out = append(out, dateEngineToken{kind: "ss"})
			i += 2
		case strings.HasPrefix(rest, "wo"):
			out = append(out, dateEngineToken{kind: "wo"})
			i += 2
		case strings.HasPrefix(rest, "Q"):
			out = append(out, dateEngineToken{kind: "Q"})
			i++
		case strings.HasPrefix(rest, "M"):
			out = append(out, dateEngineToken{kind: "M"})
			i++
		case strings.HasPrefix(rest, "D"):
			out = append(out, dateEngineToken{kind: "D"})
			i++
		default:
			out = append(out, dateEngineToken{kind: "lit-" + string(format[i])})
			i++
		}
	}
	return out
}

// FormatDateEngineDate renders d with dayjs-style tokens.
// BBBB is the Buddhist year (YYYY+543); Q the quarter; wo the
// week-of-year label for weekStart.
func FormatDateEngineDate(d DateEngineDate, format string, buddhist bool, loc DateEngineLocaleText, weekStart int) string {
	gen := DefaultDateEngineGenerateConfig()
	var b strings.Builder
	for _, t := range splitDateEngineFormat(format) {
		switch t.kind {
		case "YYYY":
			b.WriteString(pad4(d.Year))
		case "BBBB":
			b.WriteString(pad4(d.Year + 543))
		case "Q":
			b.WriteString(strconv.Itoa(gen.QuarterOf(d)))
		case "wo":
			b.WriteString(strconv.Itoa(gen.WeekNumber(d, weekStart)))
		case "MM":
			b.WriteString(pad2(d.Month))
		case "M":
			b.WriteString(strconv.Itoa(d.Month))
		case "MMM":
			b.WriteString(loc.ShortMonth(d.Month))
		case "MMMM":
			b.WriteString(loc.FullMonth(d.Month))
		case "DD":
			b.WriteString(pad2(d.Day))
		case "D":
			b.WriteString(strconv.Itoa(d.Day))
		case "HH":
			b.WriteString(pad2(d.Hour))
		case "mm":
			b.WriteString(pad2(d.Minute))
		case "ss":
			b.WriteString(pad2(d.Second))
		default:
			if strings.HasPrefix(t.kind, "lit-") {
				b.WriteString(strings.TrimPrefix(t.kind, "lit-"))
			}
		}
	}
	_ = buddhist
	return b.String()
}

func pad2(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

func pad4(v int) string {
	s := strconv.Itoa(v)
	for len(s) < 4 {
		s = "0" + s
	}
	return s
}

// dateEngineParseGroup maps one regex group to a date field.
type dateEngineParseGroup struct {
	field string
	buddh bool
}

// buildDateEngineParser compiles one format into a full-match regex.
func buildDateEngineParser(format string) (*regexp.Regexp, []dateEngineParseGroup) {
	var b strings.Builder
	var groups []dateEngineParseGroup
	b.WriteString("^")
	num := func(field string, min, max int) {
		groups = append(groups, dateEngineParseGroup{field: field})
		if max <= 2 {
			b.WriteString("(\\d{1," + strconv.Itoa(max) + "})")
		} else {
			b.WriteString("(\\d{" + strconv.Itoa(min) + "," + strconv.Itoa(max) + "})")
		}
	}
	for _, t := range splitDateEngineFormat(format) {
		switch t.kind {
		case "YYYY":
			groups = append(groups, dateEngineParseGroup{field: "Y"})
			b.WriteString("(\\d{4})")
		case "BBBB":
			groups = append(groups, dateEngineParseGroup{field: "Y", buddh: true})
			b.WriteString("(\\d{4})")
		case "MM", "M":
			num("M", 1, 2)
		case "DD", "D":
			num("D", 1, 2)
		case "HH":
			num("h", 1, 2)
		case "mm":
			num("m", 1, 2)
		case "ss":
			num("s", 1, 2)
		case "Q":
			groups = append(groups, dateEngineParseGroup{field: "Q"})
			b.WriteString("(\\d)")
		case "wo":
			groups = append(groups, dateEngineParseGroup{field: "W"})
			b.WriteString("(\\d{1,2})")
		case "MMM", "MMMM":
			groups = append(groups, dateEngineParseGroup{field: "MN"})
			b.WriteString("(.+?)")
		default:
			if strings.HasPrefix(t.kind, "lit-") {
				b.WriteString(regexp.QuoteMeta(strings.TrimPrefix(t.kind, "lit-")))
			}
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()), groups
}

// ParseDateEngineDate tries each format in order; the first full
// match wins. Returns false when nothing matches or fields are wild.
func ParseDateEngineDate(text string, formats []string, buddhist bool, loc DateEngineLocaleText) (DateEngineDate, bool) {
	for _, f := range formats {
		if d, ok := parseDateEngineOne(strings.TrimSpace(text), f, buddhist, loc); ok {
			return d, true
		}
	}
	return DateEngineDate{}, false
}

func parseDateEngineOne(text, format string, buddhist bool, loc DateEngineLocaleText) (DateEngineDate, bool) {
	re, groups := buildDateEngineParser(format)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return DateEngineDate{}, false
	}
	d := DateEngineDate{Year: -1, Month: -1, Day: 1}
	for i, g := range groups {
		raw := m[i+1]
		switch g.field {
		case "Y":
			y, err := strconv.Atoi(raw)
			if err != nil {
				return DateEngineDate{}, false
			}
			if g.buddh || buddhist {
				y -= 543
			}
			d.Year = y
		case "M":
			v, err := strconv.Atoi(raw)
			if err != nil || v < 1 || v > 12 {
				return DateEngineDate{}, false
			}
			d.Month = v
		case "D":
			v, err := strconv.Atoi(raw)
			if err != nil {
				return DateEngineDate{}, false
			}
			d.Day = v
		case "h":
			v, err := strconv.Atoi(raw)
			if err != nil || v > 23 {
				return DateEngineDate{}, false
			}
			d.Hour = v
		case "m":
			v, err := strconv.Atoi(raw)
			if err != nil || v > 59 {
				return DateEngineDate{}, false
			}
			d.Minute = v
		case "s":
			v, err := strconv.Atoi(raw)
			if err != nil || v > 59 {
				return DateEngineDate{}, false
			}
			d.Second = v
		case "Q":
			q, err := strconv.Atoi(raw)
			if err != nil || q < 1 || q > 4 {
				return DateEngineDate{}, false
			}
			d.Month = (q-1)*3 + 1
		case "MN":
			mo := loc.MonthByName(raw)
			if mo == 0 {
				return DateEngineDate{}, false
			}
			d.Month = mo
		case "W":
			// Week label alone cannot pin a day; needs year context.
			if _, err := strconv.Atoi(raw); err != nil {
				return DateEngineDate{}, false
			}
		}
	}
	if d.Year < 0 {
		return DateEngineDate{}, false
	}
	if d.Month < 0 {
		d.Month = 1
	}
	if !d.IsValid() {
		return DateEngineDate{}, false
	}
	return d, true
}
