package kit

import (
	"fmt"
	"math"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Calendar defaults — components/calendar/style prepareComponentToken.
// docs/antd/calendar.md §6.2 / §6.10
// https://ant.design/components/calendar
const (
	DefaultCalendarFontSize          = 14.0
	DefaultCalendarFontSizeSM        = 12.0
	DefaultCalendarRadius            = 6.0
	DefaultCalendarRadiusLG          = 8.0
	DefaultCalendarLineWidth         = 1.0
	DefaultCalendarFocusOutset       = 1.5
	DefaultCalendarYearControlWidth  = 80.0
	DefaultCalendarMonthControlWidth = 70.0
	DefaultCalendarMiniContentHeight = 256.0
	DefaultCalendarDateValueHeight   = 24.0 // controlHeightSM
	DefaultCalendarWeekHeight        = 18.0 // controlHeightSM * 0.75
	DefaultCalendarDateContentHeight = 92.0
	DefaultCalendarHeaderPadV        = 12.0 // paddingSM
	DefaultCalendarBodyPadV          = 8.0  // paddingXS
	DefaultCalendarMiniCell          = 28.0
	DefaultCalendarFullCellMinH      = 116.0 // value + content + pads (approx)
	DefaultCalendarGap               = 4.0
	DefaultCalendarHeaderGap         = 8.0 // marginXS
	calendarSpinRPS                  = 0.9
)

// CalendarMode is antd mode: month | year.
type CalendarMode int

const (
	// CalendarMonth shows the date panel (default).
	CalendarMonth CalendarMode = iota
	// CalendarYear shows the month panel (12 months).
	CalendarYear
)

func (m CalendarMode) String() string {
	if m == CalendarYear {
		return "year"
	}
	return "month"
}

// CalendarSelectSource is onSelect info.source.
type CalendarSelectSource int

const (
	// CalendarSourceDate is a day cell click.
	CalendarSourceDate CalendarSelectSource = iota
	// CalendarSourceMonth is a month cell / month header select.
	CalendarSourceMonth
	// CalendarSourceYear is a year header select.
	CalendarSourceYear
	// CalendarSourceCustomize is headerRender onChange.
	CalendarSourceCustomize
)

func (s CalendarSelectSource) String() string {
	switch s {
	case CalendarSourceMonth:
		return "month"
	case CalendarSourceYear:
		return "year"
	case CalendarSourceCustomize:
		return "customize"
	default:
		return "date"
	}
}

// CalendarCellType is cellRender info.type (date | month).
type CalendarCellType int

const (
	// CalendarCellDate is a day cell in month mode.
	CalendarCellDate CalendarCellType = iota
	// CalendarCellMonth is a month cell in year mode.
	CalendarCellMonth
)

// CalendarCellInfo is passed to CellRender / FullCellRender.
type CalendarCellInfo struct {
	Type       CalendarCellType
	OriginNode core.Node
	Today      DateValue
}

// CalendarHeaderConfig is passed to HeaderRender (antd headerRender object).
type CalendarHeaderConfig struct {
	Value        DateValue
	Type         CalendarMode
	OnChange     func(DateValue)
	OnTypeChange func(CalendarMode)
}

// Calendar is Ant Design Calendar (data display).
//
//	Flex Column (Root, role=grid) — identity stable across rebuild
//	  ├─ Header (year/month selects + mode radio) or HeaderRender
//	  └─ Body (weekdays + date grid | month grid)
//
// Product contract: docs/antd/calendar.md §6 (P0 DoD).
type Calendar struct {
	Root   *primitive.Flex
	header core.Node
	body   *primitive.Flex

	// Value is the selected / displayed date.
	Value DateValue
	// DefaultValue seeds uncontrolled value (antd defaultValue).
	DefaultValue DateValue
	// Mode is month (date panel) or year (month panel).
	Mode CalendarMode
	// Fullscreen defaults true; false = card/mini mode.
	Fullscreen bool
	// ShowWeek shows ISO week numbers (antd showWeek, default false).
	ShowWeek bool
	// Disabled disables the whole calendar.
	Disabled bool
	// Loading shows a spinner and swallows selection.
	Loading bool
	// Controlled: selection only fires callbacks; parent must SetValue.
	Controlled bool

	// ValidRange optional [start,end] inclusive; zero ends ignored.
	ValidRange [2]DateValue
	// DisabledDate returns true for non-selectable dates.
	DisabledDate func(DateValue) bool
	// CellRender customizes cell content (inside default cell chrome).
	CellRender func(current DateValue, info CalendarCellInfo) core.Node
	// FullCellRender replaces the entire cell chrome when non-nil return.
	FullCellRender func(current DateValue, info CalendarCellInfo) core.Node
	// HeaderRender replaces the default header when non-nil.
	HeaderRender func(cfg CalendarHeaderConfig) core.Node

	OnChange      func(DateValue)
	OnSelect      func(DateValue, CalendarSelectSource)
	OnPanelChange func(DateValue, CalendarMode)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// panel view (may differ from value while browsing)
	panelYear  int
	panelMonth int // 1..12

	appliedDef bool
	// resolved metrics (L2 / tests)
	fontSize          float64
	fontSizeSM        float64
	radius            float64
	lineWidth         float64
	yearControlWidth  float64
	monthControlWidth float64
	miniContentH      float64
	dateValueH        float64
	weekH             float64
	dateContentH      float64
	cellW             float64
	cellH             float64

	// chrome colors for tests
	bgColor      render.RGBA
	textColor    render.RGBA
	primaryColor render.RGBA
	splitColor   render.RGBA
	disabledText render.RGBA
	itemActiveBg render.RGBA
	itemHoverBg  render.RGBA

	// day/month pressables for keyboard / hit tests
	dayCells   []*primitive.Pressable
	monthCells []*primitive.Pressable

	// loading spinner
	spinCanvas *primitive.Canvas
	spinPhase  float64
	ticker     *core.Tree
	tickHooked bool
}

// NewCalendar creates a Calendar with Ant defaults (mode=month, fullscreen=true, value=today).
func NewCalendar() *Calendar {
	now := Today()
	c := &Calendar{
		Fullscreen: true,
		Mode:       CalendarMonth,
		Value:      now,
		panelYear:  now.Year,
		panelMonth: now.Month,
	}
	c.rebuild()
	return c
}

// Node returns the stable root.
func (c *Calendar) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// HeaderNode returns the current header node (tests).
func (c *Calendar) HeaderNode() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.header
}

// BodyNode returns the body flex (tests).
func (c *Calendar) BodyNode() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.body
}

// DayCells returns pressable day cells in month panel order (tests).
func (c *Calendar) DayCells() []*primitive.Pressable {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.dayCells
}

// MonthCells returns pressable month cells in year panel (tests).
func (c *Calendar) MonthCells() []*primitive.Pressable {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.monthCells
}

// GetValue returns the current value.
func (c *Calendar) GetValue() DateValue {
	if c == nil {
		return DateValue{}
	}
	return c.Value
}

// PanelYear returns the visible panel year.
func (c *Calendar) PanelYear() int {
	if c == nil {
		return 0
	}
	c.ensurePanel()
	return c.panelYear
}

// PanelMonth returns the visible panel month (1..12).
func (c *Calendar) PanelMonth() int {
	if c == nil {
		return 0
	}
	c.ensurePanel()
	return c.panelMonth
}

// IsFullscreen reports fullscreen mode.
func (c *Calendar) IsFullscreen() bool {
	if c == nil {
		return true
	}
	return c.Fullscreen
}

// ResolvedFontSize returns resolved middle font size.
func (c *Calendar) ResolvedFontSize() float64 {
	c.resolveMetrics()
	return c.fontSize
}

// ResolvedRadius returns root corner radius (card uses LG when !fullscreen).
func (c *Calendar) ResolvedRadius() float64 {
	c.resolveMetrics()
	return c.radius
}

// ResolvedYearControlWidth returns year select min width.
func (c *Calendar) ResolvedYearControlWidth() float64 {
	c.resolveMetrics()
	return c.yearControlWidth
}

// ResolvedMonthControlWidth returns month select min width.
func (c *Calendar) ResolvedMonthControlWidth() float64 {
	c.resolveMetrics()
	return c.monthControlWidth
}

// ResolvedMiniContentHeight returns mini panel content height token.
func (c *Calendar) ResolvedMiniContentHeight() float64 {
	c.resolveMetrics()
	return c.miniContentH
}

// ResolvedDateValueHeight returns date value line height.
func (c *Calendar) ResolvedDateValueHeight() float64 {
	c.resolveMetrics()
	return c.dateValueH
}

// ResolvedWeekHeight returns weekday header line height.
func (c *Calendar) ResolvedWeekHeight() float64 {
	c.resolveMetrics()
	return c.weekH
}

// ResolvedCellSize returns current cell width/height used by the panel.
func (c *Calendar) ResolvedCellSize() (w, h float64) {
	c.resolveMetrics()
	return c.cellW, c.cellH
}

// PrimaryColor returns resolved primary (tests / L2).
func (c *Calendar) PrimaryColor() render.RGBA {
	c.resolveMetrics()
	return c.primaryColor
}

// BackgroundColor returns container background.
func (c *Calendar) BackgroundColor() render.RGBA {
	c.resolveMetrics()
	return c.bgColor
}

// TextColor returns primary text color.
func (c *Calendar) TextColor() render.RGBA {
	c.resolveMetrics()
	return c.textColor
}

// DisabledTextColor returns disabled text token.
func (c *Calendar) DisabledTextColor() render.RGBA {
	c.resolveMetrics()
	return c.disabledText
}

// ---------------------------------------------------------------------------
// Setters
// ---------------------------------------------------------------------------

// SetValue writes the selected value without firing OnChange. Marks controlled
// only when SetControlled(true) was used, or callers may set Controlled themselves.
func (c *Calendar) SetValue(v DateValue) {
	if c == nil {
		return
	}
	c.Value = v
	if v.Valid {
		c.panelYear, c.panelMonth = v.Year, v.Month
	}
	c.rebuild()
}

// SetDefaultValue seeds uncontrolled value when still at constructor default
// and not controlled.
func (c *Calendar) SetDefaultValue(v DateValue) {
	if c == nil || !v.Valid {
		return
	}
	c.DefaultValue = v
	if c.Controlled {
		return
	}
	if !c.appliedDef {
		c.appliedDef = true
		c.Value = v
		c.panelYear, c.panelMonth = v.Year, v.Month
		c.rebuild()
	}
}

// SetControlled marks parent-owned value (antd value={…}).
func (c *Calendar) SetControlled(v bool) { c.Controlled = v }

// SetMode sets month|year panel mode and fires onPanelChange.
func (c *Calendar) SetMode(m CalendarMode) {
	if c == nil {
		return
	}
	if c.Mode == m {
		return
	}
	c.Mode = m
	c.firePanelChange()
	c.rebuild()
}

// Mode returns the current mode.
func (c *Calendar) ModeOf() CalendarMode {
	if c == nil {
		return CalendarMonth
	}
	return c.Mode
}

// SetPanelMonth sets visible year/month (1..12). Fires onPanelChange when changed.
func (c *Calendar) SetPanelMonth(year, month int) {
	if c == nil {
		return
	}
	if year == 0 {
		year = c.panelYear
	}
	if month < 1 {
		month = 1
	}
	if month > 12 {
		month = 12
	}
	if c.panelYear == year && c.panelMonth == month {
		return
	}
	c.panelYear, c.panelMonth = year, month
	c.firePanelChange()
	c.rebuild()
}

// SetFullscreen toggles full vs card mode.
func (c *Calendar) SetFullscreen(v bool) {
	if c == nil || c.Fullscreen == v {
		return
	}
	c.Fullscreen = v
	c.rebuild()
}

// SetShowWeek toggles ISO week column.
func (c *Calendar) SetShowWeek(v bool) {
	if c == nil || c.ShowWeek == v {
		return
	}
	c.ShowWeek = v
	c.rebuild()
}

// SetDisabledDate sets the per-date disable predicate.
func (c *Calendar) SetDisabledDate(fn func(DateValue) bool) {
	if c == nil {
		return
	}
	c.DisabledDate = fn
	c.rebuild()
}

// SetValidRange sets inclusive display/select range (zero = open end).
func (c *Calendar) SetValidRange(start, end DateValue) {
	if c == nil {
		return
	}
	c.ValidRange = [2]DateValue{start, end}
	c.rebuild()
}

// SetCellRender sets antd cellRender.
func (c *Calendar) SetCellRender(fn func(DateValue, CalendarCellInfo) core.Node) {
	if c == nil {
		return
	}
	c.CellRender = fn
	c.rebuild()
}

// SetFullCellRender sets antd fullCellRender.
func (c *Calendar) SetFullCellRender(fn func(DateValue, CalendarCellInfo) core.Node) {
	if c == nil {
		return
	}
	c.FullCellRender = fn
	c.rebuild()
}

// SetHeaderRender sets antd headerRender.
func (c *Calendar) SetHeaderRender(fn func(CalendarHeaderConfig) core.Node) {
	if c == nil {
		return
	}
	c.HeaderRender = fn
	c.rebuild()
}

// SetDisabled disables the whole calendar.
func (c *Calendar) SetDisabled(v bool) {
	if c == nil || c.Disabled == v {
		return
	}
	c.Disabled = v
	c.rebuild()
}

// SetLoading toggles loading spinner (Ticker).
func (c *Calendar) SetLoading(v bool) {
	if c == nil || c.Loading == v {
		return
	}
	c.Loading = v
	c.rebuild()
	c.syncTicker()
}

// SetTheme sets an explicit theme override.
func (c *Calendar) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetFace sets the font face.
func (c *Calendar) SetFace(face text.Face) {
	if c == nil {
		return
	}
	c.Face = face
	c.rebuild()
}

// SetStyle sets optional visual overrides.
func (c *Calendar) SetStyle(st Style) {
	if c == nil {
		return
	}
	c.Style = st
	c.rebuild()
}

// SetAriaLabel sets the accessible name.
func (c *Calendar) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	c.applyA11y()
}

// SetOnChange sets the value-change callback.
func (c *Calendar) SetOnChange(fn func(DateValue)) {
	if c != nil {
		c.OnChange = fn
	}
}

// SetOnSelect sets the select callback (includes source).
func (c *Calendar) SetOnSelect(fn func(DateValue, CalendarSelectSource)) {
	if c != nil {
		c.OnSelect = fn
	}
}

// SetOnPanelChange sets panel year/month/mode change callback.
func (c *Calendar) SetOnPanelChange(fn func(DateValue, CalendarMode)) {
	if c != nil {
		c.OnPanelChange = fn
	}
}

// SelectDate selects a date programmatically (respects disabled / loading).
func (c *Calendar) SelectDate(v DateValue) {
	if c == nil || !v.Valid {
		return
	}
	c.internalSelect(v, CalendarSourceDate)
}

// SelectDay selects day-of-month in the current panel month.
func (c *Calendar) SelectDay(day int) {
	if c == nil || day < 1 {
		return
	}
	c.ensurePanel()
	max := daysInMonth(c.panelYear, time.Month(c.panelMonth))
	if day > max {
		day = max
	}
	c.SelectDate(DateOf(c.panelYear, c.panelMonth, day))
}

// AttachTicker binds loading spin to the tree.
func (c *Calendar) AttachTicker(t *core.Tree) {
	if c == nil {
		return
	}
	c.ticker = t
	c.syncTicker()
}

// Tick advances the loading spinner. Returns still-active for Tree ticker.
func (c *Calendar) Tick(dt float64) bool {
	if c == nil || !c.Loading {
		return false
	}
	c.spinPhase += dt * calendarSpinRPS
	if c.spinPhase > 1 {
		c.spinPhase -= math.Floor(c.spinPhase)
	}
	if c.spinCanvas != nil {
		c.spinCanvas.MarkNeedsPaint()
	} else if c.Root != nil {
		c.Root.MarkNeedsPaint()
	}
	return true
}

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

func (c *Calendar) theme() *core.Theme {
	if c == nil {
		return DefaultTheme()
	}
	if c.Theme != nil {
		return c.Theme
	}
	if c.Root == nil {
		return DefaultTheme()
	}
	return themeOf(c.Theme, c.Root)
}

func (c *Calendar) ensurePanel() {
	if c.panelYear == 0 || c.panelMonth < 1 || c.panelMonth > 12 {
		v := c.Value
		if !v.Valid {
			v = Today()
		}
		c.panelYear, c.panelMonth = v.Year, v.Month
	}
}

func (c *Calendar) resolveMetrics() {
	th := c.theme()
	if th == nil {
		th = DefaultTheme()
	}
	c.fontSize = th.SizeOr(core.TokenFontSize, DefaultCalendarFontSize)
	c.fontSizeSM = th.SizeOr(core.TokenFontSizeSM, DefaultCalendarFontSizeSM)
	c.lineWidth = th.SizeOr(core.TokenLineWidth, DefaultCalendarLineWidth)
	c.yearControlWidth = DefaultCalendarYearControlWidth
	c.monthControlWidth = DefaultCalendarMonthControlWidth
	c.miniContentH = DefaultCalendarMiniContentHeight
	c.dateValueH = th.SizeOr(core.TokenControlHeightSM, DefaultCalendarDateValueHeight)
	c.weekH = c.dateValueH * 0.75
	if c.weekH < 1 {
		c.weekH = DefaultCalendarWeekHeight
	}
	// dateContentHeight ≈ (fontHeightSM + marginXS) * 3 + lineWidth*2
	fhSM := c.fontSizeSM * 1.5714
	if fhSM < c.fontSizeSM {
		fhSM = c.fontSizeSM
	}
	marginXS := th.SizeOr(core.TokenMarginXS, 8)
	c.dateContentH = (fhSM+marginXS)*3 + c.lineWidth*2
	if c.dateContentH < 40 {
		c.dateContentH = DefaultCalendarDateContentHeight
	}

	if c.Fullscreen {
		c.radius = th.SizeOr(core.TokenBorderRadius, DefaultCalendarRadius)
		c.cellW = 0 // flexible stretch
		c.cellH = c.dateValueH + c.dateContentH + marginXS
		if c.cellH < DefaultCalendarFullCellMinH {
			c.cellH = DefaultCalendarFullCellMinH
		}
	} else {
		c.radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultCalendarRadiusLG)
		c.cellW = DefaultCalendarMiniCell
		c.cellH = DefaultCalendarMiniCell
	}

	c.bgColor = th.Color(core.TokenColorBgContainer)
	c.textColor = th.Color(core.TokenColorText)
	c.primaryColor = th.Color(core.TokenColorPrimary)
	c.splitColor = th.Color(core.TokenColorSplit)
	c.disabledText = th.Color(core.TokenColorDisabledText)
	c.itemActiveBg = th.Color(core.TokenColorPrimaryBg)
	if c.itemActiveBg.A == 0 {
		c.itemActiveBg = th.Color(core.TokenColorBgTextHover)
	}
	c.itemHoverBg = th.Color(core.TokenColorBgTextHover)
	if c.Style.hasBG() {
		c.bgColor = c.Style.Background
	}
	if c.Style.hasText() {
		c.textColor = c.Style.Text
	}
	if c.Style.hasRadius() {
		c.radius = c.Style.Radius
	}
}

func (c *Calendar) isDisabledDate(v DateValue) bool {
	if !v.Valid {
		return true
	}
	if c.ValidRange[0].Valid && v.Before(c.ValidRange[0]) && !v.EqualDate(c.ValidRange[0]) {
		// Before start (date-only): reject if strictly before start day
		if v.Year < c.ValidRange[0].Year ||
			(v.Year == c.ValidRange[0].Year && v.Month < c.ValidRange[0].Month) ||
			(v.Year == c.ValidRange[0].Year && v.Month == c.ValidRange[0].Month && v.Day < c.ValidRange[0].Day) {
			return true
		}
	}
	if c.ValidRange[1].Valid {
		end := c.ValidRange[1]
		if v.Year > end.Year ||
			(v.Year == end.Year && v.Month > end.Month) ||
			(v.Year == end.Year && v.Month == end.Month && v.Day > end.Day) {
			return true
		}
	}
	if c.DisabledDate != nil && c.DisabledDate(v) {
		return true
	}
	return false
}

func (c *Calendar) firePanelChange() {
	if c.OnPanelChange == nil {
		return
	}
	v := c.Value
	if !v.Valid {
		v = DateOf(c.panelYear, c.panelMonth, 1)
	} else {
		// keep day when possible while reporting panel
		v = DateOf(c.panelYear, c.panelMonth, v.Day)
		if !v.Valid {
			v = DateOf(c.panelYear, c.panelMonth, 1)
		}
	}
	c.OnPanelChange(v, c.Mode)
}

func (c *Calendar) internalSelect(v DateValue, source CalendarSelectSource) {
	if c.Disabled || c.Loading || !v.Valid {
		return
	}
	if c.isDisabledDate(v) {
		return
	}

	old := c.Value
	panelChanged := false
	if c.Mode == CalendarMonth {
		if v.Year != c.panelYear || v.Month != c.panelMonth {
			c.panelYear, c.panelMonth = v.Year, v.Month
			panelChanged = true
		}
	}

	// Year panel: selecting a month switches back to month mode (antd).
	modeChanged := false
	if source == CalendarSourceMonth && c.Mode == CalendarYear {
		c.Mode = CalendarMonth
		c.panelYear, c.panelMonth = v.Year, v.Month
		modeChanged = true
		panelChanged = true
	}

	if !c.Controlled {
		c.Value = v
	}

	if c.OnSelect != nil {
		c.OnSelect(v, source)
	}
	if !old.EqualDate(v) && c.OnChange != nil {
		// only fire onChange when value actually changes (antd isSameDate)
		// In controlled mode parent hasn't updated yet; still fire with new date.
		c.OnChange(v)
	}
	if panelChanged || modeChanged {
		c.firePanelChange()
	}
	c.rebuild()
}

func (c *Calendar) headerOnChange(v DateValue) {
	if !v.Valid {
		return
	}
	// Header year/month change selects with customize/year/month source.
	src := CalendarSourceCustomize
	c.panelYear, c.panelMonth = v.Year, v.Month
	if !c.Controlled {
		// keep day when possible
		day := 1
		if c.Value.Valid {
			day = c.Value.Day
		}
		nv := DateOf(v.Year, v.Month, day)
		if !nv.Valid {
			nv = DateOf(v.Year, v.Month, 1)
		}
		old := c.Value
		c.Value = nv
		if c.OnSelect != nil {
			c.OnSelect(nv, src)
		}
		if !old.EqualDate(nv) && c.OnChange != nil {
			c.OnChange(nv)
		}
	} else {
		nv := DateOf(v.Year, v.Month, 1)
		if c.Value.Valid {
			nv = DateOf(v.Year, v.Month, c.Value.Day)
			if !nv.Valid {
				nv = DateOf(v.Year, v.Month, 1)
			}
		}
		if c.OnSelect != nil {
			c.OnSelect(nv, src)
		}
		if c.OnChange != nil && !c.Value.EqualDate(nv) {
			c.OnChange(nv)
		}
	}
	c.firePanelChange()
	c.rebuild()
}

func (c *Calendar) applyA11y() {
	if c.Root == nil {
		return
	}
	b := c.Root.Base()
	b.Role = "grid"
	if c.AriaLabel != "" {
		b.Label = c.AriaLabel
	} else {
		b.Label = "calendar"
	}
}

func (c *Calendar) syncTicker() {
	if c.ticker == nil {
		return
	}
	if c.Loading && !c.tickHooked {
		c.ticker.AddTicker(c)
		c.tickHooked = true
	}
	// Keep hooked while attached; Tick no-ops when !Loading.
}

func (c *Calendar) rebuild() {
	c.ensurePanel()
	c.resolveMetrics()

	if c.Root == nil {
		c.Root = primitive.Column()
	} else {
		c.Root.ClearChildren()
	}
	c.dayCells = c.dayCells[:0]
	c.monthCells = c.monthCells[:0]
	c.spinCanvas = nil

	// Header
	if c.HeaderRender != nil {
		cfg := CalendarHeaderConfig{
			Value: c.headerValue(),
			Type:  c.Mode,
			OnChange: func(v DateValue) {
				c.headerOnChange(v)
			},
			OnTypeChange: func(m CalendarMode) {
				c.SetMode(m)
			},
		}
		c.header = c.HeaderRender(cfg)
	} else {
		c.header = c.buildDefaultHeader()
	}
	if c.header != nil {
		c.Root.AddChild(c.header)
	}

	// Body
	c.body = primitive.Column()
	c.body.CrossAlign = core.CrossStretch
	if c.Fullscreen {
		c.body.Padding = primitive.EdgeInsets{Top: DefaultCalendarBodyPadV, Bottom: DefaultCalendarBodyPadV}
	} else {
		c.body.Padding = primitive.EdgeInsets{Top: 4, Bottom: 4}
	}

	if c.Mode == CalendarYear {
		c.body.AddChild(c.buildMonthGrid())
	} else {
		c.body.AddChild(c.buildWeekHeader())
		c.body.AddChild(c.buildDateGrid())
	}

	// Loading overlay
	if c.Loading {
		stack := primitive.NewStack()
		stack.AddChild(c.body)
		spin := primitive.NewCanvas(28, 28, c.paintSpinner)
		c.spinCanvas = spin
		stack.AddChild(primitive.Positioned(core.AlignCenter, spin))
		c.Root.AddChild(stack)
	} else {
		c.Root.AddChild(c.body)
	}

	c.Root.Gap = DefaultCalendarGap
	c.Root.CrossAlign = core.CrossStretch
	c.Root.MainAlign = core.MainStart
	if c.Fullscreen {
		c.Root.Padding = primitive.EdgeInsets{}
	} else {
		// Soft card inset; outer border is gallery wrapper (antd card demo).
		c.Root.Padding = primitive.EdgeInsets{Top: 4, Bottom: 4, Left: 4, Right: 4}
	}

	c.applyA11y()
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}

func (c *Calendar) paintSpinner(pc *core.PaintContext, size core.Size) {
	if pc == nil || c == nil {
		return
	}
	cx, cy := size.Width/2, size.Height/2
	r := math.Min(size.Width, size.Height) * 0.38
	if r < 6 {
		r = 6
	}
	track := c.theme().Color(core.TokenColorSplit)
	pc.StrokeLocalCircle(cx, cy, r, 2, track)
	start := -math.Pi/2 + c.spinPhase*2*math.Pi
	const segs = 18
	pts := make([]float64, 0, (segs+1)*2)
	for i := 0; i <= segs*3/4; i++ {
		a := start + float64(i)*2*math.Pi/float64(segs)
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, 2, c.primaryColor)
}

func (c *Calendar) headerValue() DateValue {
	if c.Value.Valid {
		// align to panel month keeping day
		v := DateOf(c.panelYear, c.panelMonth, c.Value.Day)
		if v.Valid {
			return v
		}
	}
	return DateOf(c.panelYear, c.panelMonth, 1)
}

func (c *Calendar) buildDefaultHeader() core.Node {
	th := c.theme()
	row := primitive.Row()
	row.MainAlign = core.MainEnd
	row.CrossAlign = core.CrossCenter
	row.Gap = DefaultCalendarHeaderGap
	row.Padding = primitive.EdgeInsets{
		Top: DefaultCalendarHeaderPadV, Bottom: DefaultCalendarHeaderPadV,
	}

	selSz := InputMiddle
	radioSz := RadioMiddle
	if !c.Fullscreen {
		selSz = InputSmall
		radioSz = RadioSmall
	}

	// Year select
	year := c.panelYear
	yOpts := make([]SelectOption, 0, 20)
	for i := year - 10; i < year+10; i++ {
		yOpts = append(yOpts, SelectOption{
			Label: fmt.Sprintf("%d", i),
			Value: fmt.Sprintf("%d", i),
		})
	}
	ySel := NewSelect("Year", yOpts...)
	ySel.SetSize(selSz)
	ySel.SetFace(c.Face)
	ySel.SetTheme(th)
	ySel.SetFixedWidth(c.yearControlWidth)
	ySel.SetValue(fmt.Sprintf("%d", year))
	ySel.SetDisabled(c.Disabled || c.Loading)
	ySel.SetOnChange(func(s string) {
		var y int
		if _, err := fmt.Sscanf(s, "%d", &y); err == nil && y > 0 {
			day := 1
			if c.Value.Valid {
				day = c.Value.Day
			}
			c.headerOnChange(DateOf(y, c.panelMonth, day))
		}
	})

	// Month select (only when mode=month)
	var mSel *Select
	if c.Mode == CalendarMonth {
		months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		mOpts := make([]SelectOption, 12)
		for i, lab := range months {
			mOpts[i] = SelectOption{Label: lab, Value: fmt.Sprintf("%d", i+1)}
		}
		mSel = NewSelect("Month", mOpts...)
		mSel.SetSize(selSz)
		mSel.SetFace(c.Face)
		mSel.SetTheme(th)
		mSel.SetFixedWidth(c.monthControlWidth)
		mSel.SetValue(fmt.Sprintf("%d", c.panelMonth))
		mSel.SetDisabled(c.Disabled || c.Loading)
		mSel.SetOnChange(func(s string) {
			var m int
			if _, err := fmt.Sscanf(s, "%d", &m); err == nil && m >= 1 && m <= 12 {
				day := 1
				if c.Value.Valid {
					day = c.Value.Day
				}
				c.headerOnChange(DateOf(c.panelYear, m, day))
			}
		})
	}

	// Mode radio
	mode := NewRadioGroup()
	mode.SetOptionType(RadioOptionButton)
	mode.SetSize(radioSz)
	mode.SetFace(c.Face)
	mode.SetTheme(th)
	mode.SetOptions(
		RadioOption{Label: "Month", Value: "month"},
		RadioOption{Label: "Year", Value: "year"},
	)
	if c.Mode == CalendarYear {
		mode.SetValue("year")
	} else {
		mode.SetValue("month")
	}
	mode.SetDisabled(c.Disabled || c.Loading)
	mode.SetOnChange(func(v string) {
		if v == "year" {
			c.SetMode(CalendarYear)
		} else {
			c.SetMode(CalendarMonth)
		}
	})

	row.AddChild(ySel.Node())
	if mSel != nil {
		row.AddChild(mSel.Node())
	}
	row.AddChild(mode.Node())
	return row
}

func (c *Calendar) buildWeekHeader() core.Node {
	row := primitive.Row()
	row.Gap = 0
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainStart
	if c.ShowWeek {
		row.AddChild(c.weekLabel(""))
	}
	for _, d := range []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"} {
		row.AddChild(c.weekLabel(d))
	}
	return row
}

func (c *Calendar) weekLabel(s string) core.Node {
	lab := primitive.NewText(s)
	lab.FontSize = c.fontSizeSM
	lab.Face = c.Face
	lab.Color = c.theme().Color(core.TokenColorTextSecondary)
	box := primitive.NewDecorated(lab)
	box.BorderWidth = 0
	box.Background = render.RGBA{}
	box.SetCenterContent(true)
	box.StretchChild = true
	if c.Fullscreen {
		box.Height = c.weekH + 8
		return primitive.NewFlexible(1, box)
	}
	box.Width = c.cellW
	box.Height = c.weekH
	return box
}

func (c *Calendar) buildDateGrid() core.Node {
	grid := primitive.Column()
	grid.Gap = 2
	grid.CrossAlign = core.CrossStretch

	first := time.Date(c.panelYear, time.Month(c.panelMonth), 1, 0, 0, 0, 0, time.UTC)
	startPad := int(first.Weekday()) // Sunday=0
	daysIn := daysInMonth(c.panelYear, time.Month(c.panelMonth))
	// previous month fill
	prevY, prevM := c.panelYear, c.panelMonth-1
	if prevM < 1 {
		prevM = 12
		prevY--
	}
	prevDays := daysInMonth(prevY, time.Month(prevM))

	// 6 weeks × 7 days
	cells := make([]DateValue, 0, 42)
	for i := 0; i < startPad; i++ {
		d := prevDays - startPad + 1 + i
		cells = append(cells, DateOf(prevY, prevM, d))
	}
	for d := 1; d <= daysIn; d++ {
		cells = append(cells, DateOf(c.panelYear, c.panelMonth, d))
	}
	nextY, nextM := c.panelYear, c.panelMonth+1
	if nextM > 12 {
		nextM = 1
		nextY++
	}
	for len(cells) < 42 {
		d := len(cells) - startPad - daysIn + 1
		cells = append(cells, DateOf(nextY, nextM, d))
	}

	today := Today()
	for w := 0; w < 6; w++ {
		row := primitive.Row()
		row.Gap = 0
		row.CrossAlign = core.CrossStart
		if c.ShowWeek {
			// ISO week of the Thursday of this row (or first in-view day)
			ref := cells[w*7+3] // mid-week
			if !ref.Valid {
				ref = cells[w*7]
			}
			wk := weekOfYear(ref)
			lab := primitive.NewText(fmt.Sprintf("%d", wk))
			lab.FontSize = c.fontSizeSM
			lab.Face = c.Face
			lab.Color = c.theme().Color(core.TokenColorTextTertiary)
			box := primitive.NewDecorated(lab)
			box.BorderWidth = 0
			box.SetCenterContent(true)
			box.StretchChild = true
			if c.Fullscreen {
				box.Height = c.cellH
				row.AddChild(primitive.NewFlexible(1, box))
			} else {
				box.Width, box.Height = c.cellW, c.cellH
				row.AddChild(box)
			}
		}
		for d := 0; d < 7; d++ {
			cell := cells[w*7+d]
			row.AddChild(c.buildDateCell(cell, today))
		}
		grid.AddChild(row)
	}
	return grid
}

func (c *Calendar) buildDateCell(v DateValue, today DateValue) core.Node {
	inView := v.Valid && v.Year == c.panelYear && v.Month == c.panelMonth
	selected := v.Valid && c.Value.Valid && v.EqualDate(c.Value)
	isToday := v.Valid && today.Valid && v.EqualDate(today)
	disabled := c.Disabled || c.Loading || c.isDisabledDate(v)

	// origin node (default chrome content)
	origin := c.defaultDateInner(v, inView, selected, isToday, disabled)

	info := CalendarCellInfo{
		Type:       CalendarCellDate,
		OriginNode: origin,
		Today:      today,
	}

	var content core.Node
	if c.FullCellRender != nil {
		if n := c.FullCellRender(v, info); n != nil {
			content = n
		}
	}
	if content == nil {
		// default cell + optional cellRender content
		var inner core.Node = origin
		if c.CellRender != nil {
			extra := c.CellRender(v, info)
			if extra != nil {
				col := primitive.Column(origin, extra)
				col.Gap = 2
				col.CrossAlign = core.CrossStretch
				inner = col
			}
		}
		content = inner
	}

	dec := primitive.NewDecorated(content)
	dec.BorderWidth = 0
	dec.Radius = 0
	dec.SetCenterContent(false)
	dec.StretchChild = true
	if selected && inView {
		dec.Background = c.itemActiveBg
	} else {
		dec.Background = render.RGBA{}
	}
	if isToday && c.Fullscreen {
		// top border today mark (approx with full border primary)
		dec.BorderWidth = c.lineWidth * 2
		dec.BorderColor = c.primaryColor
	}
	if c.Fullscreen {
		dec.Height = c.cellH
		dec.Padding = primitive.EdgeInsets{Top: 4, Left: 8, Right: 8, Bottom: 4}
	} else {
		dec.Width, dec.Height = c.cellW, c.cellH
		dec.Radius = thSize(c.theme(), core.TokenBorderRadiusSM, 4)
		dec.SetCenterContent(true)
		if selected {
			dec.Background = c.primaryColor
		}
		if isToday && !selected {
			dec.BorderWidth = c.lineWidth
			dec.BorderColor = c.primaryColor
		}
	}

	p := primitive.NewPressable(dec)
	p.ShowFocusRing = true
	p.FocusRingOutset = DefaultCalendarFocusOutset
	p.FocusRingRadius = dec.Radius
	p.EnableRipple = false
	p.State.Disabled = disabled
	if disabled {
		p.Focusable = false
	}
	vv := v
	p.Click = func() {
		if disabled {
			return
		}
		c.internalSelect(vv, CalendarSourceDate)
	}
	// hover fill via pressable colors for mini
	if !disabled && !selected {
		p.ColorHovered = c.itemHoverBg
	}
	c.dayCells = append(c.dayCells, p)

	if c.Fullscreen {
		return primitive.NewFlexible(1, p)
	}
	return p
}

func (c *Calendar) defaultDateInner(v DateValue, inView, selected, isToday, disabled bool) core.Node {
	day := ""
	if v.Valid {
		if c.Fullscreen {
			day = fmt.Sprintf("%02d", v.Day)
		} else {
			day = fmt.Sprintf("%d", v.Day)
		}
	}
	lab := primitive.NewText(day)
	lab.FontSize = c.fontSize
	lab.Face = c.Face
	switch {
	case disabled:
		lab.Color = c.disabledText
	case selected && !c.Fullscreen:
		lab.Color = c.theme().Color(core.TokenColorTextInverse)
		if lab.Color.A == 0 {
			lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
	case selected && c.Fullscreen:
		lab.Color = c.primaryColor
	case !inView:
		lab.Color = c.theme().Color(core.TokenColorTextTertiary)
	case isToday:
		lab.Color = c.primaryColor
	default:
		lab.Color = c.textColor
	}
	if c.Fullscreen {
		// value row right-aligned; height via decorated min
		wrap := primitive.NewDecorated(lab)
		wrap.BorderWidth = 0
		wrap.Background = render.RGBA{}
		wrap.Height = c.dateValueH
		wrap.SetCenterContent(false)
		wrap.StretchChild = true
		return wrap
	}
	return lab
}

func (c *Calendar) buildMonthGrid() core.Node {
	grid := primitive.Column()
	grid.Gap = 8
	grid.CrossAlign = core.CrossStretch
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	today := Today()
	for r := 0; r < 4; r++ {
		row := primitive.Row()
		row.Gap = 8
		row.CrossAlign = core.CrossStretch
		for col := 0; col < 3; col++ {
			m := r*3 + col + 1
			v := DateOf(c.panelYear, m, 1)
			row.AddChild(c.buildMonthCell(v, months[m-1], today))
		}
		grid.AddChild(row)
	}
	return grid
}

func (c *Calendar) buildMonthCell(v DateValue, label string, today DateValue) core.Node {
	selected := c.Value.Valid && v.Valid && c.Value.Year == v.Year && c.Value.Month == v.Month
	isToday := today.Valid && today.Year == v.Year && today.Month == v.Month
	// disable if entire month outside validRange roughly: check day 1 and last day
	disabled := c.Disabled || c.Loading
	if !disabled && v.Valid {
		// month disabled only if all days disabled — approximate with day 1
		if c.isDisabledDate(v) && c.isDisabledDate(DateOf(v.Year, v.Month, daysInMonth(v.Year, time.Month(v.Month)))) {
			disabled = true
		}
	}

	originLab := primitive.NewText(label)
	originLab.FontSize = c.fontSize
	originLab.Face = c.Face
	if disabled {
		originLab.Color = c.disabledText
	} else if selected && !c.Fullscreen {
		originLab.Color = c.theme().Color(core.TokenColorTextInverse)
		if originLab.Color.A == 0 {
			originLab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
	} else if selected {
		originLab.Color = c.primaryColor
	} else {
		originLab.Color = c.textColor
	}
	origin := originLab

	info := CalendarCellInfo{Type: CalendarCellMonth, OriginNode: origin, Today: today}
	var content core.Node
	if c.FullCellRender != nil {
		if n := c.FullCellRender(v, info); n != nil {
			content = n
		}
	}
	if content == nil {
		var inner core.Node = origin
		if c.CellRender != nil {
			if extra := c.CellRender(v, info); extra != nil {
				col := primitive.Column(origin, extra)
				col.Gap = 2
				col.CrossAlign = core.CrossCenter
				inner = col
			}
		}
		content = inner
	}

	dec := primitive.NewDecorated(content)
	dec.BorderWidth = 0
	dec.Radius = thSize(c.theme(), core.TokenBorderRadius, 6)
	dec.SetCenterContent(true)
	dec.StretchChild = true
	dec.Padding = primitive.All(8)
	if selected {
		if c.Fullscreen {
			dec.Background = c.itemActiveBg
		} else {
			dec.Background = c.primaryColor
		}
	}
	if isToday && !selected {
		dec.BorderWidth = c.lineWidth
		dec.BorderColor = c.primaryColor
	}
	if c.Fullscreen {
		dec.Height = c.dateValueH + 24
	} else {
		dec.Height = c.cellH + 8
	}

	p := primitive.NewPressable(dec)
	p.ShowFocusRing = true
	p.FocusRingOutset = DefaultCalendarFocusOutset
	p.FocusRingRadius = dec.Radius
	p.EnableRipple = false
	p.State.Disabled = disabled
	if disabled {
		p.Focusable = false
	}
	if !disabled && !selected {
		p.ColorHovered = c.itemHoverBg
	}
	vv := v
	p.Click = func() {
		if disabled {
			return
		}
		// select first of month, switch to month mode
		c.internalSelect(vv, CalendarSourceMonth)
	}
	c.monthCells = append(c.monthCells, p)
	return primitive.NewFlexible(1, p)
}

func thSize(th *core.Theme, token string, fallback float64) float64 {
	if th == nil {
		return fallback
	}
	return th.SizeOr(token, fallback)
}
