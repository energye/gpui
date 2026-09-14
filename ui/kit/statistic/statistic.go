// Package statistic implements the Statistic control (docs/antd/statistic.md §6).
//
// Scope is P0 in §6.8: value/title/prefix/suffix, precision and separators,
// formatter, loading skeleton, shallow semantic styles/classNames and the
// Timer countdown/countup path with format/onChange/onFinish. Pixel CountUp,
// deep function styles, ConfigProvider defaults and debug previews stay P1.
package statistic

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// TimerType selects the Timer mode. Empty means a plain Statistic.
type TimerType string

const (
	// TimerNone is a plain Statistic without ticking.
	TimerNone TimerType = ""
	// TimerCountdown counts down to value (Unix ms) and fires onFinish once.
	TimerCountdown TimerType = "countdown"
	// TimerCountup counts up from value (Unix ms).
	TimerCountup TimerType = "countup"
)

// Defaults mirror docs/antd/statistic.md §6.10.
const (
	// DefaultDecimalSeparator is the decimal point.
	DefaultDecimalSeparator = "."
	// DefaultGroupSeparator is the thousands separator.
	DefaultGroupSeparator = ","
	// DefaultFormat is the Timer template.
	DefaultFormat = "HH:mm:ss"
	// TitleFontSizeFallback mirrors the component token titleFontSize.
	TitleFontSizeFallback = 14.0
	// ContentFontSizeFallback mirrors fontSizeHeading3.
	ContentFontSizeFallback = 24.0
	// GapFallback mirrors marginXXS.
	GapFallback = 4.0
	// SkeletonTopPadFallback mirrors padding.
	SkeletonTopPadFallback = 16.0
	// SkeletonBarWidth is the loading placeholder width.
	SkeletonBarWidth = 120.0
	// SkeletonBarHeight is the loading placeholder height.
	SkeletonBarHeight = 16.0
)

// Style is a shallow semantic override. Zero value means unset.
type Style struct {
	Color       theme.Color
	HasColor    bool
	FontSize    float64
	HasFontSize bool
}

// ColorStyle builds a color-only Style.
func ColorStyle(c theme.Color) Style { return Style{Color: c, HasColor: true} }

// FontSizeStyle builds a font-size-only Style.
func FontSizeStyle(px float64) Style { return Style{FontSize: px, HasFontSize: true} }

// ClassNames holds shallow semantic hooks (no CSS engine).
type ClassNames struct {
	Root    string
	Header  string
	Title   string
	Content string
	Value   string
	Prefix  string
	Suffix  string
}

// Styles holds shallow semantic styles. Function forms stay P1.
type Styles struct {
	Root    Style
	Header  Style
	Title   Style
	Content Style
	Value   Style
	Prefix  Style
	Suffix  Style
}

// Statistic is the Statistic widget mapped to ui/rendering + ui/theme +
// ui/scheduler. Owns one repaint-boundary RenderBox; Timer/loading drive a
// scheduler ticker, never a second frame loop.
type Statistic struct {
	value            any
	title            string
	prefix           string
	suffix           string
	hasPrecision     bool
	precision        int
	decimalSeparator string
	groupSeparator   string
	formatter        func(any) string
	loading          bool
	format           string
	timerType        TimerType
	onChange         func(diffMs float64)
	onFinish         func()
	finished         bool

	titleNode  rendering.RenderObject
	prefixNode rendering.RenderObject
	suffixNode rendering.RenderObject

	styles     Styles
	valueStyle Style
	classNames ClassNames

	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	// textFace is the paint-only font face for title/value/affixes.
	textFace  text.Face

	nowFunc  func() int64
	attached *scheduler.TickerRegistry

	node *rendering.RenderBox
}

// NewStatistic creates a plain Statistic with antd defaults.
func NewStatistic() *Statistic {
	s := &Statistic{
		value:            0,
		decimalSeparator: DefaultDecimalSeparator,
		groupSeparator:   DefaultGroupSeparator,
		format:           DefaultFormat,
		timerType:        TimerNone,
	}
	s.node = rendering.NewRenderBox()
	s.node.SetRepaintBoundary(true)
	self := s
	s.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	s.syncNode()
	return s
}

// NewStatisticTimer creates a Timer Statistic with the given type.
func NewStatisticTimer(t TimerType) *Statistic {
	s := NewStatistic()
	s.SetTimerType(t)
	return s
}

// Value returns the raw value (Timer: Unix ms target).
func (s *Statistic) Value() any {
	if s == nil {
		return 0
	}
	return s.value
}

// SetValue sets string|number content (Timer: Unix ms). Resets finish.
func (s *Statistic) SetValue(v any) {
	if s == nil {
		return
	}
	s.value = v
	s.finished = false
	s.syncNode()
	s.markLayoutDirty()
}

// DisplayText returns the current formatted value. Timer uses the counter
// template; a custom formatter overrides both paths (P0 animated approx).
func (s *Statistic) DisplayText() string {
	if s == nil {
		return "0"
	}
	if s.timerType != TimerNone {
		diff := s.diffMs(s.nowMs())
		if s.formatter != nil {
			return s.formatter(float64(diff))
		}
		return FormatTimeStr(diff, s.effectiveFormat())
	}
	if s.formatter != nil {
		return s.formatter(s.value)
	}
	return FormatNumber(s.value, s.hasPrecision, s.precision, s.decimalSeparator, s.groupSeparator)
}

// FullContentText joins prefix, value and suffix for readable assertions.
func (s *Statistic) FullContentText() string {
	if s == nil {
		return ""
	}
	return s.prefix + s.DisplayText() + s.suffix
}

// SetTitle sets the header title string.
func (s *Statistic) SetTitle(t string) {
	if s == nil {
		return
	}
	s.title = t
	s.syncNode()
	s.markLayoutDirty()
}

// Title returns the title string.
func (s *Statistic) Title() string {
	if s == nil {
		return ""
	}
	return s.title
}

// HasTitle reports whether a header row exists.
func (s *Statistic) HasTitle() bool {
	return s != nil && (s.title != "" || s.titleNode != nil)
}

// SetTitleNode stores an optional custom title node (painted at the header).
func (s *Statistic) SetTitleNode(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.titleNode = n
	s.syncNode()
	s.markLayoutDirty()
}

// TitleNode returns the custom title node, if any.
func (s *Statistic) TitleNode() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.titleNode
}

// TitleNodeHost is the §6.10 host alias.
func (s *Statistic) TitleNodeHost() rendering.RenderObject { return s.TitleNode() }

// SetPrefix sets the value prefix string.
func (s *Statistic) SetPrefix(p string) {
	if s == nil {
		return
	}
	s.prefix = p
	s.syncNode()
	s.markLayoutDirty()
}

// Prefix returns the prefix string.
func (s *Statistic) Prefix() string {
	if s == nil {
		return ""
	}
	return s.prefix
}

// SetPrefixNode stores an optional custom prefix node.
func (s *Statistic) SetPrefixNode(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.prefixNode = n
	s.syncNode()
	s.markLayoutDirty()
}

// PrefixNode returns the custom prefix node, if any.
func (s *Statistic) PrefixNode() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.prefixNode
}

// PrefixHost is the §6.10 host alias.
func (s *Statistic) PrefixHost() rendering.RenderObject { return s.PrefixNode() }

// SetSuffix sets the value suffix string.
func (s *Statistic) SetSuffix(p string) {
	if s == nil {
		return
	}
	s.suffix = p
	s.syncNode()
	s.markLayoutDirty()
}

// Suffix returns the suffix string.
func (s *Statistic) Suffix() string {
	if s == nil {
		return ""
	}
	return s.suffix
}

// SetSuffixNode stores an optional custom suffix node.
func (s *Statistic) SetSuffixNode(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.suffixNode = n
	s.syncNode()
	s.markLayoutDirty()
}

// SuffixNode returns the custom suffix node, if any.
func (s *Statistic) SuffixNode() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.suffixNode
}

// SuffixHost is the §6.10 host alias.
func (s *Statistic) SuffixHost() rendering.RenderObject { return s.SuffixNode() }

// HasPrefix reports prefix content.
func (s *Statistic) HasPrefix() bool {
	return s != nil && (s.prefix != "" || s.prefixNode != nil)
}

// HasSuffix reports suffix content.
func (s *Statistic) HasSuffix() bool {
	return s != nil && (s.suffix != "" || s.suffixNode != nil)
}

// SetPrecision enables fixed decimals.
func (s *Statistic) SetPrecision(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.hasPrecision = true
	s.precision = n
	s.syncNode()
	s.markLayoutDirty()
}

// ClearPrecision disables fixed decimals.
func (s *Statistic) ClearPrecision() {
	if s == nil {
		return
	}
	s.hasPrecision = false
	s.precision = 0
	s.syncNode()
	s.markLayoutDirty()
}

// HasPrecision reports fixed-decimal mode.
func (s *Statistic) HasPrecision() bool { return s != nil && s.hasPrecision }

// Precision returns the decimal count.
func (s *Statistic) Precision() int {
	if s == nil {
		return 0
	}
	return s.precision
}

// SetDecimalSeparator sets the decimal point.
func (s *Statistic) SetDecimalSeparator(sep string) {
	if s == nil {
		return
	}
	s.decimalSeparator = sep
	s.syncNode()
	s.markLayoutDirty()
}

// DecimalSeparator returns the decimal point.
func (s *Statistic) DecimalSeparator() string {
	if s == nil {
		return DefaultDecimalSeparator
	}
	return s.decimalSeparator
}

// SetGroupSeparator sets the thousands separator ("" disables).
func (s *Statistic) SetGroupSeparator(sep string) {
	if s == nil {
		return
	}
	s.groupSeparator = sep
	s.syncNode()
	s.markLayoutDirty()
}

// GroupSeparator returns the thousands separator.
func (s *Statistic) GroupSeparator() string {
	if s == nil {
		return DefaultGroupSeparator
	}
	return s.groupSeparator
}

// SetFormatter overrides builtin formatting (animated approx uses final).
func (s *Statistic) SetFormatter(fn func(any) string) {
	if s == nil {
		return
	}
	s.formatter = fn
	s.syncNode()
	s.markLayoutDirty()
}

// ClearFormatter restores builtin formatting.
func (s *Statistic) ClearFormatter() {
	if s == nil {
		return
	}
	s.formatter = nil
	s.syncNode()
	s.markLayoutDirty()
}

// HasFormatter reports a custom formatter.
func (s *Statistic) HasFormatter() bool { return s != nil && s.formatter != nil }

// SetLoading toggles the skeleton content (title stays visible).
func (s *Statistic) SetLoading(b bool) {
	if s == nil {
		return
	}
	if s.loading == b {
		return
	}
	s.loading = b
	s.syncNode()
	s.markLayoutDirty()
}

// IsLoading reports skeleton mode.
func (s *Statistic) IsLoading() bool { return s != nil && s.loading }

// SetFormat sets the Timer template ("" restores default).
func (s *Statistic) SetFormat(f string) {
	if s == nil {
		return
	}
	if f == "" {
		f = DefaultFormat
	}
	s.format = f
	s.syncNode()
	s.markLayoutDirty()
}

// Format returns the effective Timer template.
func (s *Statistic) Format() string { return s.effectiveFormat() }

func (s *Statistic) effectiveFormat() string {
	if s == nil || s.format == "" {
		return DefaultFormat
	}
	return s.format
}

// SetTimerType selects none|countdown|countup. Resets finish.
func (s *Statistic) SetTimerType(t TimerType) {
	if s == nil {
		return
	}
	if t != TimerNone && t != TimerCountdown && t != TimerCountup {
		t = TimerNone
	}
	s.timerType = t
	s.finished = false
	s.syncNode()
	s.markLayoutDirty()
}

// TimerType returns the Timer mode.
func (s *Statistic) TimerType() TimerType {
	if s == nil {
		return TimerNone
	}
	return s.timerType
}

// SetOnChange sets the Timer diff callback (ms).
func (s *Statistic) SetOnChange(fn func(diffMs float64)) {
	if s == nil {
		return
	}
	s.onChange = fn
}

// SetOnFinish sets the countdown completion callback (fires once).
func (s *Statistic) SetOnFinish(fn func()) {
	if s == nil {
		return
	}
	s.onFinish = fn
}

// Finished reports countdown completion.
func (s *Statistic) Finished() bool { return s != nil && s.finished }

// SetNowFunc injects the clock (tests); nil restores wall time.
func (s *Statistic) SetNowFunc(fn func() int64) {
	if s == nil {
		return
	}
	s.nowFunc = fn
}

func (s *Statistic) nowMs() int64 {
	if s != nil && s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now().UnixMilli()
}

func (s *Statistic) diffMs(now int64) int64 {
	if s == nil {
		return 0
	}
	target := toMillis(s.value)
	switch s.timerType {
	case TimerCountdown:
		d := target - now
		if d < 0 {
			d = 0
		}
		return d
	case TimerCountup:
		d := now - target
		if d < 0 {
			d = 0
		}
		return d
	default:
		return 0
	}
}

// DiffMs exposes the current Timer delta for assertions.
func (s *Statistic) DiffMs(now int64) int64 { return s.diffMs(now) }

// SetContentStyle overrides content (prefix/value/suffix row).
func (s *Statistic) SetContentStyle(st Style) {
	if s == nil {
		return
	}
	sizeChanged := st.HasFontSize != s.styles.Content.HasFontSize ||
		(st.HasFontSize && st.FontSize != s.styles.Content.FontSize)
	s.styles.Content = st
	s.syncNode()
	if sizeChanged {
		s.markLayoutDirty()
	} else {
		s.markPaintDirty()
	}
}

// SetValueStyle is deprecated: maps to content with lower priority.
func (s *Statistic) SetValueStyle(st Style) {
	if s == nil {
		return
	}
	sizeChanged := st.HasFontSize != s.valueStyle.HasFontSize ||
		(st.HasFontSize && st.FontSize != s.valueStyle.FontSize)
	s.valueStyle = st
	s.syncNode()
	if sizeChanged {
		s.markLayoutDirty()
	} else {
		s.markPaintDirty()
	}
}

// SetTitleStyle overrides the title.
func (s *Statistic) SetTitleStyle(st Style) {
	if s == nil {
		return
	}
	sizeChanged := st.HasFontSize != s.styles.Title.HasFontSize ||
		(st.HasFontSize && st.FontSize != s.styles.Title.FontSize)
	s.styles.Title = st
	s.syncNode()
	if sizeChanged {
		s.markLayoutDirty()
	} else {
		s.markPaintDirty()
	}
}

// SetPrefixStyle overrides the prefix.
func (s *Statistic) SetPrefixStyle(st Style) {
	if s == nil {
		return
	}
	s.styles.Prefix = st
	s.markPaintDirty()
}

// SetSuffixStyle overrides the suffix.
func (s *Statistic) SetSuffixStyle(st Style) {
	if s == nil {
		return
	}
	s.styles.Suffix = st
	s.markPaintDirty()
}

// SetRootStyle overrides the root.
func (s *Statistic) SetRootStyle(st Style) {
	if s == nil {
		return
	}
	s.styles.Root = st
	s.markPaintDirty()
}

// SetHeaderStyle overrides the header row.
func (s *Statistic) SetHeaderStyle(st Style) {
	if s == nil {
		return
	}
	s.styles.Header = st
	s.markPaintDirty()
}

// SetStyle is the root alias in §6.10.
func (s *Statistic) SetStyle(st Style) { s.SetRootStyle(st) }

// SetStyles replaces all shallow styles.
func (s *Statistic) SetStyles(st Styles) {
	if s == nil {
		return
	}
	sizeChanged := st.Content.HasFontSize != s.styles.Content.HasFontSize ||
		(st.Content.HasFontSize && st.Content.FontSize != s.styles.Content.FontSize) ||
		st.Title.HasFontSize != s.styles.Title.HasFontSize ||
		(st.Title.HasFontSize && st.Title.FontSize != s.styles.Title.FontSize)
	s.styles = st
	s.syncNode()
	if sizeChanged {
		s.markLayoutDirty()
	} else {
		s.markPaintDirty()
	}
}

// Styles returns the shallow styles.
func (s *Statistic) Styles() Styles {
	if s == nil {
		return Styles{}
	}
	return s.styles
}

// ValueStyle returns the deprecated content fallback.
func (s *Statistic) ValueStyle() Style {
	if s == nil {
		return Style{}
	}
	return s.valueStyle
}

// SetClassNames stores shallow hooks.
func (s *Statistic) SetClassNames(c ClassNames) {
	if s == nil {
		return
	}
	s.classNames = c
	s.markPaintDirty()
}

// ClassNames returns the hooks.
func (s *Statistic) ClassNames() ClassNames {
	if s == nil {
		return ClassNames{}
	}
	return s.classNames
}

// SetProvider selects the theme source (nil restores process default).
func (s *Statistic) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.syncNode()
	s.markLayoutDirty()
}

// SetTheme pins exact tokens (nil clears to provider).
func (s *Statistic) SetTheme(t *theme.Tokens) {
	if s == nil {
		return
	}
	s.override = t
	s.syncNode()
	s.markLayoutDirty()
}

// SetFace is a theme alias kept for §6.10 naming.
func (s *Statistic) SetFace(t *theme.Tokens) { s.SetTheme(t) }

// SetTextFace sets the paint-only font face for title/value/affixes
// (nil clears; layout keeps the rune estimate so headless tests stay stable).
func (s *Statistic) SetTextFace(f text.Face) {
	if s == nil {
		return
	}
	s.textFace = f
	if s.node != nil {
		s.node.MarkNeedsPaint()
	}
}

func (s *Statistic) tokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveTitleFontSize resolves title size (style > token > fallback).
func (s *Statistic) EffectiveTitleFontSize() float64 {
	if s != nil && s.styles.Title.HasFontSize && s.styles.Title.FontSize > 0 {
		return s.styles.Title.FontSize
	}
	tok := s.tokens()
	if tok.FontSize > 0 {
		return tok.FontSize
	}
	return TitleFontSizeFallback
}

// EffectiveContentFontSize resolves content size (style > fallback 24).
func (s *Statistic) EffectiveContentFontSize() float64 {
	if s != nil {
		if s.styles.Content.HasFontSize && s.styles.Content.FontSize > 0 {
			return s.styles.Content.FontSize
		}
		if s.valueStyle.HasFontSize && s.valueStyle.FontSize > 0 {
			return s.valueStyle.FontSize
		}
	}
	return ContentFontSizeFallback
}

// TitleFontSize is the §6.2 assertion alias.
func (s *Statistic) TitleFontSize() float64 { return s.EffectiveTitleFontSize() }

// ContentFontSize is the §6.2 assertion alias.
func (s *Statistic) ContentFontSize() float64 { return s.EffectiveContentFontSize() }

// Gap resolves header/content and prefix/value spacing (token > fallback).
func (s *Statistic) Gap() float64 {
	tok := s.tokens()
	if tok.MarginXXS > 0 {
		return tok.MarginXXS
	}
	return GapFallback
}

// SkeletonTopPad resolves the loading top pad (token > fallback).
func (s *Statistic) SkeletonTopPad() float64 {
	tok := s.tokens()
	if tok.Padding > 0 {
		return tok.Padding
	}
	return SkeletonTopPadFallback
}

// SkeletonBarSize returns the loading bar size.
func (s *Statistic) SkeletonBarSize() (w, h float64) { return SkeletonBarWidth, SkeletonBarHeight }

// EffectiveTitleColor resolves title ink (style > description token).
func (s *Statistic) EffectiveTitleColor() theme.Color {
	if s != nil && s.styles.Title.HasColor {
		return s.styles.Title.Color
	}
	return s.tokens().ColorTextSecondary
}

// EffectiveContentColor resolves content ink (content > valueStyle > heading).
func (s *Statistic) EffectiveContentColor() theme.Color {
	if s != nil {
		if s.styles.Content.HasColor {
			return s.styles.Content.Color
		}
		if s.valueStyle.HasColor {
			return s.valueStyle.Color
		}
	}
	return s.tokens().ColorTextHeading
}

// TitleColor is the assertion alias.
func (s *Statistic) TitleColor() theme.Color { return s.EffectiveTitleColor() }

// ContentColor is the assertion alias.
func (s *Statistic) ContentColor() theme.Color { return s.EffectiveContentColor() }

// SetAriaLabel sets the accessible name ("" keeps plain group).
func (s *Statistic) SetAriaLabel(v string) {
	if s == nil {
		return
	}
	s.ariaLabel = v
}

// AriaLabel returns the accessible name.
func (s *Statistic) AriaLabel() string {
	if s == nil {
		return ""
	}
	return s.ariaLabel
}

// Role returns group per §6.6.
func (s *Statistic) Role() string { return "group" }

// Focusable is always false: display control never takes Tab.
func (s *Statistic) Focusable() bool { return false }

// Node returns the tree node (layout/paint/hit through it).
func (s *Statistic) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	s.syncNode()
	return s.node
}

// ChromeNode is the §6.10 alias.
func (s *Statistic) ChromeNode() rendering.RenderObject { return s.Node() }

// ContentNodeHost exposes the content row host (root for shallow P0).
func (s *Statistic) ContentNodeHost() rendering.RenderObject { return s.Node() }

// ValueNodeHost exposes the value host (root for shallow P0).
func (s *Statistic) ValueNodeHost() rendering.RenderObject { return s.Node() }

// PreferredSize computes the content size without constraints.
func (s *Statistic) PreferredSize() rendering.Size {
	if s == nil {
		return rendering.Size{}
	}
	s.syncNode()
	if s.node == nil {
		return rendering.Size{}
	}
	return s.node.Size()
}

// Layout sizes the node under constraints.
func (s *Statistic) Layout(c rendering.Constraints) rendering.Size {
	if s == nil {
		return rendering.Size{}
	}
	s.syncNode()
	return s.node.Layout(c)
}

// Attach registers the loading/Timer ticker.
func (s *Statistic) Attach(reg *scheduler.TickerRegistry) {
	if s == nil || reg == nil {
		return
	}
	if s.attached != nil && s.attached != reg {
		s.attached.Remove(s)
	}
	s.attached = reg
	reg.Add(s)
}

// Detach unregisters the ticker.
func (s *Statistic) Detach() {
	if s == nil || s.attached == nil {
		return
	}
	s.attached.Remove(s)
	s.attached = nil
}

// Tick refreshes Timer display and fires callbacks. Countdown returns false
// once to stop, mirroring clearInterval in the reference.
func (s *Statistic) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	if s.timerType != TimerNone {
		now := s.nowMs()
		diff := s.diffMs(now)
		if s.onChange != nil {
			s.onChange(float64(diff))
		}
		s.syncNode()
		s.markPaintDirty()
		if s.timerType == TimerCountdown && diff <= 0 {
			if !s.finished {
				s.finished = true
				if s.onFinish != nil {
					s.onFinish()
				}
			}
			return false
		}
		return true
	}
	return true
}

// WantsFrame reports ticker demand (loading or running Timer).
func (s *Statistic) WantsFrame() bool {
	if s == nil {
		return false
	}
	if s.loading {
		return true
	}
	return s.timerType != TimerNone && !s.finished
}

func (s *Statistic) markLayoutDirty() {
	if s == nil || s.node == nil {
		return
	}
	s.node.MarkNeedsLayout()
}

func (s *Statistic) markPaintDirty() {
	if s == nil || s.node == nil {
		return
	}
	s.node.MarkNeedsPaint()
}

func measure(s string, fontSize float64) (w, h float64) {
	if s == "" {
		return 0, 0
	}
	if fontSize <= 0 {
		fontSize = 14
	}
	return rendering.EstimateTextSize(s, fontSize, 0.55)
}

func (s *Statistic) measureAll() (titleW, titleH, prefixW, prefixH, valueW, valueH, suffixW, suffixH float64) {
	titleFS := s.EffectiveTitleFontSize()
	contentFS := s.EffectiveContentFontSize()
	if s.titleNode != nil {
		sz := s.titleNode.Layout(rendering.Loose(1e9, 1e9))
		titleW, titleH = sz.Width, sz.Height
	} else {
		titleW, titleH = measure(s.title, titleFS)
	}
	if s.prefixNode != nil {
		sz := s.prefixNode.Layout(rendering.Loose(1e9, 1e9))
		prefixW, prefixH = sz.Width, sz.Height
	} else {
		prefixW, prefixH = measure(s.prefix, contentFS)
	}
	valueW, valueH = measure(s.DisplayText(), contentFS)
	if s.suffixNode != nil {
		sz := s.suffixNode.Layout(rendering.Loose(1e9, 1e9))
		suffixW, suffixH = sz.Width, sz.Height
	} else {
		suffixW, suffixH = measure(s.suffix, contentFS)
	}
	return titleW, titleH, prefixW, prefixH, valueW, valueH, suffixW, suffixH
}

func (s *Statistic) syncNode() {
	if s == nil || s.node == nil {
		return
	}
	gap := s.Gap()
	titleW, titleH, prefixW, _, valueW, valueH, suffixW, suffixH := s.measureAll()
	var contentW, contentH float64
	if s.loading {
		barW, barH := s.SkeletonBarSize()
		contentW = barW
		contentH = s.SkeletonTopPad() + barH
	} else {
		contentW = valueW
		contentH = valueH
		if s.HasPrefix() {
			contentW += prefixW + gap
			if ph := prefixHeight(s, contentH); ph > contentH {
				contentH = ph
			}
		}
		if s.HasSuffix() {
			contentW += gap + suffixW
			if sh := suffixHeight(s, contentH); sh > contentH {
				contentH = sh
			}
		}
		_ = prefixW
		_ = suffixW
		_ = suffixH
	}
	rootW := contentW
	if s.HasTitle() && titleW > rootW {
		rootW = titleW
	}
	rootH := contentH
	if s.HasTitle() {
		rootH += titleH + gap
	}
	if rootW < 0 {
		rootW = 0
	}
	if rootH < 0 {
		rootH = 0
	}
	s.node.FixedWidth = rootW
	s.node.FixedHeight = rootH
}

func prefixHeight(s *Statistic, fallback float64) float64 {
	_, _, _, ph, _, _, _, _ := s.measureAll()
	if ph > 0 {
		return ph
	}
	return fallback
}

func suffixHeight(s *Statistic, fallback float64) float64 {
	_, _, _, _, _, _, _, sh := s.measureAll()
	if sh > 0 {
		return sh
	}
	return fallback
}

func (s *Statistic) paint(pc *rendering.PaintContext, _ rendering.Size) {
	if pc == nil || s == nil {
		return
	}
	titleFS := s.EffectiveTitleFontSize()
	contentFS := s.EffectiveContentFontSize()
	gap := s.Gap()
	titleCol := s.EffectiveTitleColor()
	contentCol := s.EffectiveContentColor()
	tok := s.tokens()

	y := 0.0
	var titleH float64
	if s.HasTitle() {
		if s.titleNode == nil {
			_, th := measure(s.title, titleFS)
			titleH = th
			if pc.DC != nil && s.title != "" {
				if s.textFace != nil {
					pc.DC.SetFont(s.textFace)
				}
				pc.DC.SetRGBA(titleCol.R, titleCol.G, titleCol.B, titleCol.A)
				ax, ay := pc.Abs(0, titleFS*0.8)
				pc.DC.DrawString(s.title, ax, ay)
			}
		} else {
			titleH = s.titleNode.Size().Height
			if titleH <= 0 {
				_, th := measure(s.title, titleFS)
				titleH = th
			}
			s.titleNode.Paint(pc.WithOrigin(pc.OriginX, pc.OriginY))
		}
		y = titleH + gap
	}
	if s.loading {
		barW, barH := s.SkeletonBarSize()
		fill := tok.ColorFillSecondary
		rendering.FillRoundRect(pc, 0, y+s.SkeletonTopPad(), barW, barH, tok.Radius, fill.R, fill.G, fill.B, fill.A)
		return
	}
	valueStr := s.DisplayText()
	var prefixW, suffixW, valueW float64
	if s.prefixNode == nil {
		prefixW, _ = measure(s.prefix, contentFS)
	} else {
		prefixW = s.prefixNode.Size().Width
	}
	valueW, _ = measure(valueStr, contentFS)
	if s.suffixNode == nil {
		suffixW, _ = measure(s.suffix, contentFS)
	} else {
		suffixW = s.suffixNode.Size().Width
	}
	_ = valueW
	_ = suffixW
	x := 0.0
	ascent := contentFS * 0.8
	if s.HasPrefix() {
		if s.prefixNode == nil {
			if pc.DC != nil && s.prefix != "" {
				if s.textFace != nil {
					pc.DC.SetFont(s.textFace)
				}
				pc.DC.SetRGBA(contentCol.R, contentCol.G, contentCol.B, contentCol.A)
				ax, ay := pc.Abs(x, y+ascent)
				pc.DC.DrawString(s.prefix, ax, ay)
			}
			x += prefixW + gap
		} else {
			s.prefixNode.Paint(pc.WithOrigin(pc.OriginX+x, pc.OriginY+y))
			x += prefixW + gap
		}
	}
	if pc.DC != nil && valueStr != "" {
		if s.textFace != nil {
			pc.DC.SetFont(s.textFace)
		}
		pc.DC.SetRGBA(contentCol.R, contentCol.G, contentCol.B, contentCol.A)
		ax, ay := pc.Abs(x, y+ascent)
		pc.DC.DrawString(valueStr, ax, ay)
	}
	x += valueW
	if s.HasSuffix() {
		x += gap
		if s.suffixNode == nil {
			if pc.DC != nil && s.suffix != "" {
				if s.textFace != nil {
					pc.DC.SetFont(s.textFace)
				}
				pc.DC.SetRGBA(contentCol.R, contentCol.G, contentCol.B, contentCol.A)
				ax, ay := pc.Abs(x, y+ascent)
				pc.DC.DrawString(s.suffix, ax, ay)
			}
		} else {
			s.suffixNode.Paint(pc.WithOrigin(pc.OriginX+x, pc.OriginY+y))
		}
	}
}

// FormatNumber mirrors Number.tsx: illegal input passes through, int groups
// by groupSeparator, precision pads/truncates the decimal with decimalSeparator.
func FormatNumber(v any, hasPrecision bool, precision int, decimalSeparator, groupSeparator string) string {
	raw := valueToString(v)
	neg, intPart, decPart, ok := splitNumber(raw)
	if !ok {
		return raw
	}
	if intPart == "" {
		intPart = "0"
	}
	if groupSeparator != "" {
		intPart = groupInt(intPart, groupSeparator)
	}
	if hasPrecision {
		if precision < 0 {
			precision = 0
		}
		if precision == 0 {
			decPart = ""
		} else {
			for len(decPart) < precision {
				decPart += "0"
			}
			if len(decPart) > precision {
				decPart = decPart[:precision]
			}
		}
	}
	if decPart == "" {
		return neg + intPart
	}
	if decimalSeparator == "" {
		decimalSeparator = DefaultDecimalSeparator
	}
	return neg + intPart + decimalSeparator + decPart
}

func valueToString(v any) string {
	switch t := v.(type) {
	case nil:
		return "0"
	case string:
		if t == "" {
			return "0"
		}
		return t
	case int:
		return strconv.Itoa(t)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint:
		return strconv.FormatUint(uint64(t), 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func splitNumber(s string) (neg, intPart, decPart string, ok bool) {
	if s == "" || s == "-" {
		return "", s, "", false
	}
	i := 0
	if s[0] == '-' {
		neg = "-"
		i = 1
	}
	intStart := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	intPart = s[intStart:i]
	if i < len(s) {
		if s[i] != '.' {
			return "", s, "", false
		}
		i++
		decStart := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		decPart = s[decStart:i]
		if i != len(s) {
			return "", s, "", false
		}
		// Reject bare "-" with dot but no digits.
		if intPart == "" && decPart == "" {
			return "", s, "", false
		}
		return neg, intPart, decPart, true
	}
	if intPart == "" {
		return "", s, "", false
	}
	return neg, intPart, "", true
}

func groupInt(intPart, sep string) string {
	n := len(intPart)
	if n <= 3 {
		return intPart
	}
	var b strings.Builder
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

// FormatTimeStr mirrors utils.formatTimeStr: repeated letters pad to width,
// [...] stays literal. Duration is non-negative milliseconds.
func FormatTimeStr(durationMs int64, format string) string {
	if durationMs < 0 {
		durationMs = 0
	}
	if format == "" {
		format = DefaultFormat
	}
	keep := []string{}
	var tmp strings.Builder
	for i := 0; i < len(format); {
		if format[i] == '[' {
			j := strings.IndexByte(format[i+1:], ']')
			if j < 0 {
				tmp.WriteString(format[i:])
				break
			}
			keep = append(keep, format[i+1:i+1+j])
			tmp.WriteString("[]")
			i += j + 2
			continue
		}
		tmp.WriteByte(format[i])
		i++
	}
	cur := tmp.String()
	left := durationMs
	units := []struct {
		name string
		ms   int64
	}{
		{"Y", 1000 * 60 * 60 * 24 * 365},
		{"M", 1000 * 60 * 60 * 24 * 30},
		{"D", 1000 * 60 * 60 * 24},
		{"H", 1000 * 60 * 60},
		{"m", 1000 * 60},
		{"s", 1000},
		{"S", 1},
	}
	for _, u := range units {
		if !strings.Contains(cur, u.name) {
			continue
		}
		val := left / u.ms
		left -= val * u.ms
		cur = replaceRuns(cur, u.name[0], strconv.FormatInt(val, 10))
	}
	if len(keep) > 0 {
		idx := 0
		var out strings.Builder
		for i := 0; i < len(cur); {
			if i+1 < len(cur) && cur[i] == '[' && cur[i+1] == ']' {
				if idx < len(keep) {
					out.WriteString(keep[idx])
					idx++
				}
				i += 2
				continue
			}
			out.WriteByte(cur[i])
			i++
		}
		return out.String()
	}
	return cur
}

func replaceRuns(s string, ch byte, digits string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != ch {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == ch {
			j++
		}
		width := j - i
		pad := digits
		for len(pad) < width {
			pad = "0" + pad
		}
		b.WriteString(pad)
		i = j
	}
	return b.String()
}

func toMillis(v any) int64 {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return int64(t)
	case int8:
		return int64(t)
	case int16:
		return int64(t)
	case int32:
		return int64(t)
	case int64:
		return t
	case uint:
		return int64(t)
	case uint8:
		return int64(t)
	case uint16:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	case float32:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	case time.Time:
		return t.UnixMilli()
	default:
		return 0
	}
}
