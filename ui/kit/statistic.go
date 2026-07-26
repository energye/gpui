package kit

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Statistic tokens — components/statistic/style/index.ts
// docs/antd/statistic.md §6.2
const (
	// DefaultStatisticTitleFontSize is titleFontSize (fontSize).
	DefaultStatisticTitleFontSize = 14.0
	// DefaultStatisticContentFontSize is contentFontSize (fontSizeHeading3).
	DefaultStatisticContentFontSize = 24.0
	// DefaultStatisticGap is marginXXS for header pad / prefix·suffix spacing.
	DefaultStatisticGap = 4.0
	// DefaultStatisticSkeletonPad is padding above skeleton when loading.
	DefaultStatisticSkeletonPad = 16.0
	// DefaultStatisticSkeletonW is the loading bar width (approx content).
	DefaultStatisticSkeletonW = 100.0
	// DefaultStatisticSkeletonH is the loading bar height (≈ content font).
	DefaultStatisticSkeletonH = 24.0
	// DefaultStatisticRadius is borderRadius fallback for style overrides.
	DefaultStatisticRadius = 6.0
	// DefaultStatisticLineWidth is lineWidth.
	DefaultStatisticLineWidth = 1.0
	// DefaultStatisticFocusRingOutset approximates Ant focus-visible outset.
	DefaultStatisticFocusRingOutset = 1.5
	// DefaultStatisticDecimalSeparator is antd decimalSeparator.
	DefaultStatisticDecimalSeparator = "."
	// DefaultStatisticGroupSeparator is antd groupSeparator.
	DefaultStatisticGroupSeparator = ","
	// DefaultStatisticFormat is Timer format default.
	DefaultStatisticFormat = "HH:mm:ss"
)

// StatisticTimerType is antd Statistic.Timer type.
type StatisticTimerType int

const (
	// StatisticTimerNone is a plain Statistic (no timer).
	StatisticTimerNone StatisticTimerType = iota
	// StatisticTimerCountdown counts down to value (Unix ms).
	StatisticTimerCountdown
	// StatisticTimerCountup counts up from value (Unix ms).
	StatisticTimerCountup
)

// StatisticClassNames holds shallow semantic class tags (antd classNames).
type StatisticClassNames struct {
	Root    string
	Header  string
	Title   string
	Content string
	Value   string
	Prefix  string
	Suffix  string
}

// Statistic is Ant Design Statistic — title + formatted value (+ optional Timer).
//
//	statisticHost (RepaintBoundary; OnMount binds Ticker for loading|Timer)
//	  └─ Column
//	       header? · content|skeleton
//
// Product contract: docs/antd/statistic.md §6 (P0 DoD).
// Root pointer stays stable across rebuild when possible (ClearChildren).
type Statistic struct {
	Root *statisticHost
	col  *primitive.Flex

	header     *primitive.Decorated
	titleLab   *primitive.Text
	content    *primitive.Decorated
	valueLab   *primitive.Text
	prefixHost *primitive.Decorated
	suffixHost *primitive.Decorated
	skeleton   *Skeleton

	// --- product fields ---
	value any // string | number; nil → 0

	Title     string
	TitleNode core.Node

	Prefix     string
	PrefixNode core.Node
	Suffix     string
	SuffixNode core.Node

	precision    int
	precisionSet bool

	decimalSeparator string
	groupSeparator   string

	Loading   bool
	Formatter func(value any) string

	// Timer
	timerType StatisticTimerType
	format    string // empty → DefaultStatisticFormat
	OnChange  func(diffMs float64)
	OnFinish  func()

	// Semantic shallow styles
	Style        Style // root
	HeaderStyle  Style
	TitleStyle   Style
	ContentStyle Style // styles.content (+ valueStyle compat)
	ValueStyle   Style
	PrefixStyle  Style
	SuffixStyle  Style
	ClassNames   StatisticClassNames

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme

	// timer runtime
	displayOverride string // last formatted timer text; empty → use value path
	finished        bool
	lastDiffMs      float64
	nowFn           func() int64 // injectable clock (Unix ms); nil → time.Now

	// loading shimmer
	life tickerLifecycle

	// cached L2 colors
	titleColor   render.RGBA
	contentColor render.RGBA
}

type statisticHost struct {
	primitive.RepaintBoundary
	st *Statistic
}

func (h *statisticHost) TypeID() string { return TypeStatistic }

func (h *statisticHost) OnMount() {
	if h == nil || h.st == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.st.life.attach(t, h.st, h.st.needsTicker())
	}
}

func (h *statisticHost) OnUnmount() {
	if h != nil && h.st != nil {
		h.st.life.unmount()
	}
}

// NewStatistic creates a Statistic with antd defaults (value=0, no title, loading=false).
func NewStatistic() *Statistic {
	s := &Statistic{
		decimalSeparator: DefaultStatisticDecimalSeparator,
		groupSeparator:   DefaultStatisticGroupSeparator,
	}
	s.rebuild()
	return s
}

// NewStatisticTimer creates Statistic.Timer with the given type (countdown|countup).
func NewStatisticTimer(typ StatisticTimerType) *Statistic {
	s := NewStatistic()
	s.timerType = typ
	s.format = DefaultStatisticFormat
	s.syncTicker()
	return s
}

// Node returns the mount root.

// ensureBuilt materializes the control tree if missing (#9).
func (s *Statistic) ensureBuilt() {
	if s == nil {
		return
	}
	if s.Root == nil {
		s.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (s *Statistic) structureChange() {
	if s == nil {
		return
	}
	s.rebuild()
}

// chromeChange refreshes chrome; defaults to structure rebuild when colors are baked in rebuild (#9).
func (s *Statistic) chromeChange() {
	if s == nil {
		return
	}
	s.ensureBuilt()
	s.rebuild()
}

func (s *Statistic) Node() core.Node {
	if s == nil {
		return nil
	}
	s.ensureBuilt()
	return s.Root
}

// ChromeNode returns the visual shell.
func (s *Statistic) ChromeNode() core.Node { return s.Node() }

// TitleNodeHost returns the title text node when string-backed.
func (s *Statistic) TitleNodeHost() core.Node {
	if s == nil || s.titleLab == nil {
		return nil
	}
	return s.titleLab
}

// ContentNodeHost returns the content container (nil when loading skeleton).
func (s *Statistic) ContentNodeHost() core.Node {
	if s == nil || s.content == nil {
		return nil
	}
	return s.content
}

// ValueNodeHost returns the value text node.
func (s *Statistic) ValueNodeHost() core.Node {
	if s == nil || s.valueLab == nil {
		return nil
	}
	return s.valueLab
}

// PrefixHost returns the prefix container (may be nil).
func (s *Statistic) PrefixHost() core.Node {
	if s == nil || s.prefixHost == nil {
		return nil
	}
	return s.prefixHost
}

// SuffixHost returns the suffix container (may be nil).
func (s *Statistic) SuffixHost() core.Node {
	if s == nil || s.suffixHost == nil {
		return nil
	}
	return s.suffixHost
}

// SkeletonNode returns the loading skeleton when present.
func (s *Statistic) SkeletonNode() core.Node {
	if s == nil || s.skeleton == nil {
		return nil
	}
	return s.skeleton.Node()
}

// --- getters ---

// Value returns the raw value (default 0).
func (s *Statistic) Value() any {
	if s == nil || s.value == nil {
		return 0
	}
	return s.value
}

// DisplayText returns the current formatted value string.
func (s *Statistic) DisplayText() string {
	if s == nil {
		return "0"
	}
	if s.timerType != StatisticTimerNone {
		if s.displayOverride != "" {
			return s.displayOverride
		}
		return s.formatTimerNow()
	}
	return s.formatValue(s.Value())
}

// IsLoading reports loading.
func (s *Statistic) IsLoading() bool {
	return s != nil && s.Loading
}

// HasTitle reports whether a title is shown.
func (s *Statistic) HasTitle() bool {
	if s == nil {
		return false
	}
	if s.TitleNode != nil {
		return true
	}
	return s.Title != ""
}

// HasPrefix reports whether a prefix is shown.
func (s *Statistic) HasPrefix() bool {
	if s == nil {
		return false
	}
	return s.PrefixNode != nil || s.Prefix != ""
}

// HasSuffix reports whether a suffix is shown.
func (s *Statistic) HasSuffix() bool {
	if s == nil {
		return false
	}
	return s.SuffixNode != nil || s.Suffix != ""
}

// Finished reports whether countdown has finished.
func (s *Statistic) Finished() bool {
	return s != nil && s.finished
}

// TimerType returns the timer mode.
func (s *Statistic) TimerType() StatisticTimerType {
	if s == nil {
		return StatisticTimerNone
	}
	return s.timerType
}

// Format returns the timer format template.
func (s *Statistic) Format() string {
	if s == nil || s.format == "" {
		return DefaultStatisticFormat
	}
	return s.format
}

// DecimalSeparator returns the decimal separator.
func (s *Statistic) DecimalSeparator() string {
	if s == nil || s.decimalSeparator == "" {
		return DefaultStatisticDecimalSeparator
	}
	return s.decimalSeparator
}

// GroupSeparator returns the group separator.
func (s *Statistic) GroupSeparator() string {
	if s == nil {
		return DefaultStatisticGroupSeparator
	}
	// empty string is valid (no grouping) — only unset uses default via NewStatistic
	return s.groupSeparator
}

// Precision returns (n, ok).
func (s *Statistic) Precision() (int, bool) {
	if s == nil || !s.precisionSet {
		return 0, false
	}
	return s.precision, true
}

// TitleFontSize returns resolved title font size.
func (s *Statistic) TitleFontSize() float64 {
	if s != nil && s.TitleStyle.FontSize > 0 {
		return s.TitleStyle.FontSize
	}
	th := s.theme()
	if fs := th.SizeOr(core.TokenFontSize, 0); fs > 0 {
		return fs
	}
	return DefaultStatisticTitleFontSize
}

// ContentFontSize returns resolved content/value font size.
func (s *Statistic) ContentFontSize() float64 {
	if s != nil && s.ValueStyle.FontSize > 0 {
		return s.ValueStyle.FontSize
	}
	if s != nil && s.ContentStyle.FontSize > 0 {
		return s.ContentStyle.FontSize
	}
	return DefaultStatisticContentFontSize
}

// Gap returns marginXXS spacing (header pad / prefix·suffix).
func (s *Statistic) Gap() float64 {
	th := s.theme()
	if g := th.SizeOr(core.TokenMarginXS, 0); g > 0 {
		return g
	}
	if g := th.SizeOr(core.TokenPaddingXS, 0); g > 0 {
		return g
	}
	return DefaultStatisticGap
}

// TitleColor returns resolved title color.
func (s *Statistic) TitleColor() render.RGBA {
	if s != nil && s.TitleStyle.hasText() {
		return s.TitleStyle.Text
	}
	if s != nil && s.titleColor.A > 0 {
		return s.titleColor
	}
	return s.theme().Color(core.TokenColorTextSecondary)
}

// ContentColor returns resolved content color.
func (s *Statistic) ContentColor() render.RGBA {
	if s != nil && s.ContentStyle.hasText() {
		return s.ContentStyle.Text
	}
	if s != nil && s.contentColor.A > 0 {
		return s.contentColor
	}
	return s.theme().Color(core.TokenColorText)
}

// ValueColor returns resolved value color (ValueStyle > ContentStyle > token).
func (s *Statistic) ValueColor() render.RGBA {
	if s != nil && s.ValueStyle.hasText() {
		return s.ValueStyle.Text
	}
	return s.ContentColor()
}

// --- setters ---

// SetValue sets the statistic value (string|number). Timer: Unix milliseconds.
func (s *Statistic) SetValue(v any) {
	if s == nil {
		return
	}
	s.value = v
	s.finished = false
	if s.timerType != StatisticTimerNone {
		s.refreshTimerDisplay(false)
	}
	s.structureChange()
}

// SetTitle sets the string title.
func (s *Statistic) SetTitle(t string) {
	if s == nil {
		return
	}
	s.Title = t
	s.TitleNode = nil
	s.rebuild()
}

// SetTitleNode sets a custom title node (overrides Title string).
func (s *Statistic) SetTitleNode(n core.Node) {
	if s == nil {
		return
	}
	s.TitleNode = n
	s.rebuild()
}

// SetPrefix sets the string prefix.
func (s *Statistic) SetPrefix(p string) {
	if s == nil {
		return
	}
	s.Prefix = p
	s.PrefixNode = nil
	s.structureChange()
}

// SetPrefixNode sets a custom prefix node.
func (s *Statistic) SetPrefixNode(n core.Node) {
	if s == nil {
		return
	}
	s.PrefixNode = n
	s.rebuild()
}

// SetSuffix sets the string suffix.
func (s *Statistic) SetSuffix(suf string) {
	if s == nil {
		return
	}
	s.Suffix = suf
	s.SuffixNode = nil
	s.rebuild()
}

// SetSuffixNode sets a custom suffix node.
func (s *Statistic) SetSuffixNode(n core.Node) {
	if s == nil {
		return
	}
	s.SuffixNode = n
	s.structureChange()
}

// SetPrecision enables numeric precision.
func (s *Statistic) SetPrecision(n int) {
	if s == nil {
		return
	}
	s.precision = n
	s.precisionSet = true
	s.rebuild()
}

// ClearPrecision clears precision (antd unset).
func (s *Statistic) ClearPrecision() {
	if s == nil {
		return
	}
	s.precisionSet = false
	s.precision = 0
	s.rebuild()
}

// SetDecimalSeparator sets the decimal separator (default ".").
func (s *Statistic) SetDecimalSeparator(sep string) {
	if s == nil {
		return
	}
	s.decimalSeparator = sep
	s.structureChange()
}

// SetGroupSeparator sets the thousand separator (default ",").
func (s *Statistic) SetGroupSeparator(sep string) {
	if s == nil {
		return
	}
	s.groupSeparator = sep
	s.rebuild()
}

// SetFormatter sets a custom value formatter (overrides built-in number format).
func (s *Statistic) SetFormatter(fn func(value any) string) {
	if s == nil {
		return
	}
	s.Formatter = fn
	s.rebuild()
}

// SetLoading toggles the content skeleton (Ticker).
func (s *Statistic) SetLoading(v bool) {
	if s == nil {
		return
	}
	s.Loading = v
	s.syncTicker()
	s.structureChange()
}

// SetFormat sets the Timer format template (dayjs-like units).
func (s *Statistic) SetFormat(f string) {
	if s == nil {
		return
	}
	s.format = f
	if s.timerType != StatisticTimerNone {
		s.refreshTimerDisplay(false)
	}
	s.rebuild()
}

// SetTimerType switches Timer mode (none disables timer).
func (s *Statistic) SetTimerType(typ StatisticTimerType) {
	if s == nil {
		return
	}
	s.timerType = typ
	s.finished = false
	if typ != StatisticTimerNone && s.format == "" {
		s.format = DefaultStatisticFormat
	}
	s.syncTicker()
	if typ != StatisticTimerNone {
		s.refreshTimerDisplay(false)
	} else {
		s.displayOverride = ""
	}
	s.structureChange()
}

// SetOnChange sets Timer onChange (diff milliseconds).
func (s *Statistic) SetOnChange(fn func(diffMs float64)) {
	if s == nil {
		return
	}
	s.OnChange = fn
}

// SetOnFinish sets countdown onFinish (once).
func (s *Statistic) SetOnFinish(fn func()) {
	if s == nil {
		return
	}
	s.OnFinish = fn
}

// SetStyle sets root style overrides.
func (s *Statistic) SetStyle(st Style) {
	if s == nil {
		return
	}
	s.Style = st
	s.rebuild()
}

// SetContentStyle sets styles.content (preferred over valueStyle).
func (s *Statistic) SetContentStyle(st Style) {
	if s == nil {
		return
	}
	s.ContentStyle = st
	s.structureChange()
}

// SetValueStyle sets styles.value / deprecated valueStyle mapping to content when value empty.
// Prefer SetContentStyle for antd valueStyle migration; this applies to the value span.
func (s *Statistic) SetValueStyle(st Style) {
	if s == nil {
		return
	}
	s.ValueStyle = st
	// antd deprecates valueStyle → styles.content; if content unset, mirror text/size.
	if !s.ContentStyle.hasText() && st.hasText() {
		s.ContentStyle.Text = st.Text
	}
	if s.ContentStyle.FontSize == 0 && st.FontSize > 0 {
		s.ContentStyle.FontSize = st.FontSize
	}
	s.rebuild()
}

// SetTitleStyle sets styles.title.
func (s *Statistic) SetTitleStyle(st Style) {
	if s == nil {
		return
	}
	s.TitleStyle = st
	s.structureChange()
}

// SetHeaderStyle sets styles.header.
func (s *Statistic) SetHeaderStyle(st Style) {
	if s == nil {
		return
	}
	s.HeaderStyle = st
	s.rebuild()
}

// SetPrefixStyle sets styles.prefix.
func (s *Statistic) SetPrefixStyle(st Style) {
	if s == nil {
		return
	}
	s.PrefixStyle = st
	s.rebuild()
}

// SetSuffixStyle sets styles.suffix.
func (s *Statistic) SetSuffixStyle(st Style) {
	if s == nil {
		return
	}
	s.SuffixStyle = st
	s.structureChange()
}

// SetClassNames sets shallow semantic class tags.
func (s *Statistic) SetClassNames(cn StatisticClassNames) {
	if s == nil {
		return
	}
	s.ClassNames = cn
	s.rebuild()
}

// SetFace sets the font face.
func (s *Statistic) SetFace(face text.Face) {
	if s == nil {
		return
	}
	s.Face = face
	s.rebuild()
}

// SetTheme sets the theme and rebuilds.
func (s *Statistic) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.structureChange()
}

// SetAriaLabel sets an accessible name on the root.
func (s *Statistic) SetAriaLabel(lab string) {
	if s == nil {
		return
	}
	s.AriaLabel = lab
	if s.Root != nil {
		s.Root.Base().Label = lab
	}
}

// SetNowFunc injects a clock for tests (Unix ms). nil restores time.Now.
func (s *Statistic) SetNowFunc(fn func() int64) {
	if s == nil {
		return
	}
	s.nowFn = fn
}

// AttachTicker registers loading/timer animation (also automatic OnMount).
func (s *Statistic) AttachTicker(t *core.Tree) {
	if s == nil {
		return
	}
	s.life.attach(t, s, s.needsTicker())
	if s.skeleton != nil && s.Loading {
		s.skeleton.AttachTicker(t)
	}
}

// Tick advances loading shimmer / timer. Implements core.Ticker.
func (s *Statistic) Tick(dt float64) (still bool) {
	if s == nil || !s.needsTicker() {
		return false
	}
	var nt *core.Tree
	if s.Root != nil {
		nt = s.Root.Tree()
	}
	if !s.life.stillMounted(nt) {
		return false
	}
	if s.Loading && s.skeleton != nil {
		_ = s.skeleton.Tick(dt)
	}
	if s.timerType != StatisticTimerNone && !s.finished {
		s.refreshTimerDisplay(true)
		// update value label without full rebuild when possible
		if s.valueLab != nil {
			s.valueLab.Value = s.DisplayText()
			s.valueLab.MarkNeedsLayout()
			s.valueLab.MarkNeedsPaint()
		} else {
			s.structureChange()
		}
		if s.Root != nil {
			s.Root.MarkNeedsPaint()
		}
	}
	return s.needsTicker()
}

func (s *Statistic) needsTicker() bool {
	if s == nil {
		return false
	}
	if s.Loading {
		return true
	}
	return s.timerType != StatisticTimerNone && !s.finished
}

func (s *Statistic) syncTicker() {
	s.life.setActive(s.needsTicker())
}

func (s *Statistic) theme() *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	var n core.Node
	if s != nil && s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Statistic) nowMs() int64 {
	if s != nil && s.nowFn != nil {
		return s.nowFn()
	}
	return time.Now().UnixMilli()
}

// --- formatting (antd Number.tsx + utils.ts) ---

var statisticNumRe = regexp.MustCompile(`^(-?)(\d*)(?:\.(\d+))?$`)

func statisticStringify(v any) string {
	if v == nil {
		return "0"
	}
	switch x := v.(type) {
	case string:
		return x
	case int:
		return strconv.Itoa(x)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 64)
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Sprint(x)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(x)
	}
}

func (s *Statistic) formatValue(v any) string {
	if s != nil && s.Formatter != nil {
		return s.Formatter(v)
	}
	return formatStatisticNumber(v, s)
}

func formatStatisticNumber(v any, s *Statistic) string {
	val := statisticStringify(v)
	cells := statisticNumRe.FindStringSubmatch(val)
	if cells == nil || val == "-" {
		return val
	}
	negative := cells[1]
	intPart := cells[2]
	if intPart == "" {
		intPart = "0"
	}
	decimal := ""
	if len(cells) > 3 {
		decimal = cells[3]
	}

	groupSep := DefaultStatisticGroupSeparator
	decSep := DefaultStatisticDecimalSeparator
	var precision *int
	if s != nil {
		groupSep = s.GroupSeparator()
		decSep = s.DecimalSeparator()
		if s.precisionSet {
			p := s.precision
			precision = &p
		}
	}

	// thousand grouping: /\B(?=(\d{3})+(?!\d))/g
	if groupSep != "" {
		intPart = groupDigits(intPart, groupSep)
	}

	if precision != nil {
		p := *precision
		if p < 0 {
			p = 0
		}
		if p == 0 {
			decimal = ""
		} else {
			if len(decimal) < p {
				decimal = decimal + strings.Repeat("0", p-len(decimal))
			} else if len(decimal) > p {
				decimal = decimal[:p]
			}
		}
	}

	out := negative + intPart
	if decimal != "" {
		out += decSep + decimal
	}
	return out
}

func groupDigits(intPart, sep string) string {
	// keep leading zeros path simple; group from right
	n := len(intPart)
	if n <= 3 {
		return intPart
	}
	var b strings.Builder
	// first chunk may be shorter
	rem := n % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(intPart[:rem])
	for i := rem; i < n; i += 3 {
		b.WriteString(sep)
		b.WriteString(intPart[i : i+3])
	}
	return b.String()
}

// formatTimeStr mirrors antd statistic/utils.ts formatTimeStr.
func formatTimeStr(duration float64, format string) string {
	if duration < 0 {
		duration = 0
	}
	left := int64(duration)

	units := []struct {
		name string
		unit int64
	}{
		{"Y", 1000 * 60 * 60 * 24 * 365},
		{"M", 1000 * 60 * 60 * 24 * 30},
		{"D", 1000 * 60 * 60 * 24},
		{"H", 1000 * 60 * 60},
		{"m", 1000 * 60},
		{"s", 1000},
		{"S", 1},
	}

	// escape [literal]
	escapeRe := regexp.MustCompile(`\[[^\]]*\]`)
	keepList := escapeRe.FindAllString(format, -1)
	for i, k := range keepList {
		if len(k) >= 2 {
			keepList[i] = k[1 : len(k)-1]
		}
	}
	template := escapeRe.ReplaceAllString(format, "[]")

	for _, u := range units {
		if !strings.Contains(template, u.name) {
			continue
		}
		value := left / u.unit
		left -= value * u.unit
		// replace runs of the unit letter with zero-padded value
		runRe := regexp.MustCompile(u.name + "+")
		template = runRe.ReplaceAllStringFunc(template, func(match string) string {
			return padLeft(strconv.FormatInt(value, 10), len(match))
		})
	}

	idx := 0
	return escapeRe.ReplaceAllStringFunc(template, func(_ string) string {
		if idx >= len(keepList) {
			return ""
		}
		s := keepList[idx]
		idx++
		return s
	})
}

func padLeft(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return strings.Repeat("0", n-len(s)) + s
}

func (s *Statistic) formatTimerNow() string {
	diff := s.timerDiffMs()
	if diff < 0 {
		diff = 0
	}
	format := s.Format()
	// When formatter is set on Timer, antd still uses internal formatCounter via formatter prop override.
	// P0: if user Formatter set, call with raw value; else formatTimeStr.
	if s.Formatter != nil {
		return s.Formatter(s.Value())
	}
	return formatTimeStr(diff, format)
}

func (s *Statistic) timerDiffMs() float64 {
	target := statisticToUnixMs(s.Value())
	now := float64(s.nowMs())
	if s.timerType == StatisticTimerCountup {
		d := now - target
		if d < 0 {
			return 0
		}
		return d
	}
	// countdown
	d := target - now
	if d < 0 {
		return 0
	}
	return d
}

func statisticToUnixMs(v any) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	case float64:
		return x
	case string:
		// try parse float
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f
		}
		// try time parse
		if t, err := time.Parse(time.RFC3339, x); err == nil {
			return float64(t.UnixMilli())
		}
	}
	return 0
}

func (s *Statistic) refreshTimerDisplay(fireCallbacks bool) {
	if s == nil || s.timerType == StatisticTimerNone {
		return
	}
	diff := s.timerDiffMs()
	s.lastDiffMs = diff
	s.displayOverride = formatTimeStr(diff, s.Format())
	if s.Formatter != nil {
		s.displayOverride = s.Formatter(s.Value())
	}
	if fireCallbacks && s.OnChange != nil {
		s.OnChange(diff)
	}
	if s.timerType == StatisticTimerCountdown && diff <= 0 && !s.finished {
		s.finished = true
		s.syncTicker()
		if fireCallbacks && s.OnFinish != nil {
			s.OnFinish()
		}
	}
}

// AdvanceTimerForTest forces one timer refresh (callbacks optional).
func (s *Statistic) AdvanceTimerForTest(fireCallbacks bool) {
	if s == nil {
		return
	}
	s.refreshTimerDisplay(fireCallbacks)
	if s.valueLab != nil {
		s.valueLab.Value = s.DisplayText()
	} else {
		s.structureChange()
	}
}

// --- rebuild ---

func (s *Statistic) rebuild() {
	if s == nil {
		return
	}
	th := s.theme()
	gap := s.Gap()

	s.titleColor = th.Color(core.TokenColorTextSecondary)
	if s.titleColor.A < 0.05 {
		s.titleColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	s.contentColor = th.Color(core.TokenColorText)
	if s.contentColor.A < 0.05 {
		s.contentColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
	}

	// host root
	if s.Root == nil {
		h := &statisticHost{st: s}
		h.Init(h)
		h.Hit = core.HitDefer
		h.SetRepaintBoundary(true)
		s.Root = h
	} else {
		s.Root.ClearChildren()
		s.Root.st = s
	}
	s.Root.Base().Role = "group"
	if s.AriaLabel != "" {
		s.Root.Base().Label = s.AriaLabel
	} else {
		s.Root.Base().Label = ""
	}
	if s.ClassNames.Root != "" {
		s.Root.Base().Key = s.ClassNames.Root
	}

	// column
	if s.col == nil {
		s.col = primitive.Column()
	} else {
		s.col.ClearChildren()
	}
	s.col.Gap = 0
	s.col.CrossAlign = core.CrossStart
	s.col.MainAlign = core.MainStart

	// --- header / title ---
	s.header = nil
	s.titleLab = nil
	if s.HasTitle() {
		var titleNode core.Node
		if s.TitleNode != nil {
			titleNode = s.TitleNode
		} else {
			lab := primitive.NewText(s.Title)
			lab.FontSize = s.TitleFontSize()
			lab.Face = s.Face
			if s.TitleStyle.Face != nil {
				lab.Face = s.TitleStyle.Face
			}
			col := s.TitleColor()
			lab.Color = col
			if s.ClassNames.Title != "" {
				lab.Base().Key = s.ClassNames.Title
			}
			s.titleLab = lab
			titleNode = lab
		}
		hdr := primitive.NewDecorated(titleNode)
		hdr.Hit = core.HitDefer
		hdr.Padding = primitive.EdgeInsets{Bottom: gap}
		if s.ClassNames.Header != "" {
			hdr.Base().Key = s.ClassNames.Header
		}
		applyStatStyle(hdr, s.HeaderStyle)
		s.header = hdr
		s.col.AddChild(hdr)
	}

	// --- content or skeleton ---
	s.content = nil
	s.valueLab = nil
	s.prefixHost = nil
	s.suffixHost = nil
	s.skeleton = nil

	if s.Loading {
		//debug
		sk := NewSkeleton()
		sk.SetParagraph(false)
		sk.SetTitleWidth(DefaultStatisticSkeletonW)
		sk.SetStyles(SkeletonStyles{Title: Style{Height: DefaultStatisticSkeletonH}})
		sk.Theme = th
		sk.SetActive(true)
		// pad top like antd statistic-skeleton paddingTop
		wrap := primitive.NewDecorated(sk.Node())
		wrap.Hit = core.HitDefer
		pad := DefaultStatisticSkeletonPad
		if p := th.SizeOr(core.TokenPadding, 0); p > 0 {
			pad = p
		}
		wrap.Padding = primitive.EdgeInsets{Top: pad}
		s.skeleton = sk
		s.col.AddChild(wrap)
	} else {
		row := primitive.Row()
		row.Gap = 0
		row.CrossAlign = core.CrossCenter
		row.MainAlign = core.MainStart

		// prefix
		if s.HasPrefix() {
			var pNode core.Node
			if s.PrefixNode != nil {
				pNode = s.PrefixNode
			} else {
				pl := primitive.NewText(s.Prefix)
				pl.FontSize = s.ContentFontSize()
				pl.Face = s.Face
				pl.Color = s.ContentColor()
				if s.PrefixStyle.hasText() {
					pl.Color = s.PrefixStyle.Text
				}
				if s.PrefixStyle.FontSize > 0 {
					pl.FontSize = s.PrefixStyle.FontSize
				}
				pNode = pl
			}
			ph := primitive.NewDecorated(pNode)
			ph.Hit = core.HitDefer
			ph.Padding = primitive.EdgeInsets{Right: gap}
			if s.ClassNames.Prefix != "" {
				ph.Base().Key = s.ClassNames.Prefix
			}
			applyStatStyle(ph, s.PrefixStyle)
			s.prefixHost = ph
			row.AddChild(ph)
		}

		// value
		txt := s.DisplayText()
		vl := primitive.NewText(txt)
		vl.FontSize = s.ContentFontSize()
		vl.Face = s.Face
		if s.ValueStyle.Face != nil {
			vl.Face = s.ValueStyle.Face
		} else if s.ContentStyle.Face != nil {
			vl.Face = s.ContentStyle.Face
		}
		vl.Color = s.ValueColor()
		if s.ClassNames.Value != "" {
			vl.Base().Key = s.ClassNames.Value
		}
		// value style bg/radius wrap
		var valueNode core.Node = vl
		if s.ValueStyle.hasBG() || s.ValueStyle.hasBorder() || s.ValueStyle.hasRadius() {
			vw := primitive.NewDecorated(vl)
			vw.Hit = core.HitDefer
			applyStatStyle(vw, s.ValueStyle)
			valueNode = vw
		}
		s.valueLab = vl
		row.AddChild(valueNode)

		// suffix
		if s.HasSuffix() {
			var sNode core.Node
			if s.SuffixNode != nil {
				sNode = s.SuffixNode
			} else {
				sl := primitive.NewText(s.Suffix)
				sl.FontSize = s.ContentFontSize()
				sl.Face = s.Face
				sl.Color = s.ContentColor()
				if s.SuffixStyle.hasText() {
					sl.Color = s.SuffixStyle.Text
				}
				if s.SuffixStyle.FontSize > 0 {
					sl.FontSize = s.SuffixStyle.FontSize
				}
				sNode = sl
			}
			sh := primitive.NewDecorated(sNode)
			sh.Hit = core.HitDefer
			sh.Padding = primitive.EdgeInsets{Left: gap}
			if s.ClassNames.Suffix != "" {
				sh.Base().Key = s.ClassNames.Suffix
			}
			applyStatStyle(sh, s.SuffixStyle)
			s.suffixHost = sh
			row.AddChild(sh)
		}

		ct := primitive.NewDecorated(row)
		ct.Hit = core.HitDefer
		if s.ClassNames.Content != "" {
			ct.Base().Key = s.ClassNames.Content
		}
		applyStatStyle(ct, s.ContentStyle)
		s.content = ct
		s.col.AddChild(ct)
	}

	// root style shell — wrap column in decorated for bg/border
	shell := primitive.NewDecorated(s.col)
	shell.Hit = core.HitDefer
	applyStatStyle(shell, s.Style)
	s.Root.AddChild(shell)

	// theme hook
	s.Root.SetThemeHook(func(th *core.Theme) {
		if th != nil {
			s.Theme = th
		}
		s.rebuild()
	})

	s.syncTicker()
}

func applyStatStyle(d *primitive.Decorated, st Style) {
	if d == nil {
		return
	}
	if st.hasBG() {
		d.Background = st.Background
	}
	if st.hasBorder() {
		d.BorderColor = st.Border
		d.BorderWidth = DefaultStatisticLineWidth
	}
	if st.hasRadius() {
		d.Radius = st.Radius
	}
	if st.Width > 0 {
		d.Width = st.Width
	}
	if st.Height > 0 {
		d.Height = st.Height
	}
}
