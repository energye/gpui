package kit

import (
	"strings"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

// DateEngineLocaleText carries picker copy (antd locale/example.json).
type DateEngineLocaleText struct {
	Locale          string
	Placeholder     string
	RangePlace      [2]string
	Today           string
	Now             string
	Ok              string
	Clear           string
	MonthBeforeYear bool
	ShortWeekDays   [7]string
	ShortMonths     [12]string
	FullMonths      [12]string
}

// ResolveDateEngineLocale returns en defaults with zh-CN overrides.
// Unknown locales fall back to en so panels never render empty.
func ResolveDateEngineLocale(locale string) DateEngineLocaleText {
	loc := DateEngineLocaleText{
		Locale:          "en_US",
		Placeholder:     "Select date",
		RangePlace:      [2]string{"Start date", "End date"},
		Today:           "Today",
		Now:             "Now",
		Ok:              "OK",
		Clear:           "Clear",
		MonthBeforeYear: true,
		ShortWeekDays:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		ShortMonths:     [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		FullMonths:      [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
	}
	if len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z') {
		loc.Locale = "zh_CN"
		loc.Placeholder = "请选择日期"
		loc.RangePlace = [2]string{"开始日期", "结束日期"}
		loc.Today = "今天"
		loc.Now = "此刻"
		loc.Ok = "确定"
		loc.Clear = "清除"
		loc.ShortWeekDays = [7]string{"日", "一", "二", "三", "四", "五", "六"}
		loc.ShortMonths = [12]string{"1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"}
		loc.FullMonths = loc.ShortMonths
	}
	return loc
}

// WeekHeader rotates short weekdays by weekStart.
func (l DateEngineLocaleText) WeekHeader(weekStart int) [7]string {
	var out [7]string
	for i := 0; i < 7; i++ {
		out[i] = l.ShortWeekDays[(weekStart+i)%7]
	}
	return out
}

// ShortMonth returns the month abbreviation (1-based).
func (l DateEngineLocaleText) ShortMonth(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return l.ShortMonths[m-1]
}

// FullMonth returns the full month name (1-based).
func (l DateEngineLocaleText) FullMonth(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return l.FullMonths[m-1]
}

// MonthByName matches short or full month names case-insensitively.
func (l DateEngineLocaleText) MonthByName(s string) int {
	for i := 0; i < 12; i++ {
		if strings.EqualFold(l.ShortMonths[i], s) || strings.EqualFold(l.FullMonths[i], s) {
			return i + 1
		}
	}
	return 0
}

// PanelTitle renders the header copy for a panel.
func (l DateEngineLocaleText) PanelTitle(mode DateEngineMode, year, month int) string {
	switch mode {
	case DateEngineModeMonth:
		return itoaDateEngine(year)
	case DateEngineModeYear:
		return itoaDateEngine(DecadeStartOf(year)) + "-" + itoaDateEngine(DecadeStartOf(year)+9)
	case DateEngineModeDecade:
		return itoaDateEngine(CenturyStartOf(year)) + "-" + itoaDateEngine(CenturyStartOf(year)+99)
	default:
		m := l.FullMonth(month)
		if l.MonthBeforeYear {
			return m + " " + itoaDateEngine(year)
		}
		return itoaDateEngine(year) + " " + m
	}
}

// DateEngineResolved carries panel chrome from seed.
type DateEngineResolved struct {
	PanelBg  theme.Color
	Text     theme.Color
	Selected theme.Color
	OnSelect theme.Color
	Disabled theme.Color
	InRange  theme.Color
	Focus    scope.FocusRing
}

// ResolveDateEngine resolves panel chrome (props > ctx theme > seed).
func ResolveDateEngine(seed theme.Tokens) DateEngineResolved {
	bg := seed.ColorBgElevated
	if bg.A <= 0 {
		bg = seed.ColorBgContainer
	}
	return DateEngineResolved{
		PanelBg:  bg,
		Text:     seed.ColorText,
		Selected: seed.ColorPrimary,
		OnSelect: theme.Hex("#ffffff"),
		Disabled: seed.ColorTextDisabled,
		InRange:  seed.ColorPrimaryBg,
		Focus:    scope.ResolveFocusRing(seed),
	}
}
