package kit

import (
	"fmt"
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Steps defaults — docs/antd/steps.md §6.2 / §6.10
// https://ant.design/components/steps
// Source: components/steps/style prepareComponentToken (iconSize=controlHeight, iconSizeSM≈24).
const (
	DefaultStepsIconSize     = 32.0
	DefaultStepsIconSizeSM   = 24.0
	DefaultStepsTitleFont    = 16.0
	DefaultStepsContentFont  = 14.0
	DefaultStepsIconFont     = 14.0
	DefaultStepsIconFontSM   = 12.0
	DefaultStepsGap          = 8.0
	DefaultStepsRailMin      = 24.0
	DefaultStepsPanelPadding = 12.0
	DefaultStepsFocusOutset  = 1.5
)

// StepsSize is antd size (medium | small). No large.
type StepsSize int

const (
	// StepsMiddle is the default size (icon 32).
	StepsMiddle StepsSize = iota
	// StepsSmall is compact (icon 24).
	StepsSmall
)

// StepsType is antd type. P0: default | panel. P1: dot | inline | navigation.
type StepsType int

const (
	StepsTypeDefault StepsType = iota
	StepsTypePanel
	// P1 reserved
	StepsTypeDot
	StepsTypeInline
	StepsTypeNavigation
)

// StepsVariant is antd variant (filled | outlined).
type StepsVariant int

const (
	StepsVariantFilled StepsVariant = iota
	StepsVariantOutlined
)

// StepsOrientation is antd orientation (horizontal | vertical).
type StepsOrientation int

const (
	StepsHorizontal StepsOrientation = iota
	StepsVertical
)

// StepsTitlePlacement is antd titlePlacement (horizontal | vertical).
type StepsTitlePlacement int

const (
	StepsTitleHorizontal StepsTitlePlacement = iota
	StepsTitleVertical
)

// StepsStatus is wait | process | finish | error.
type StepsStatus string

const (
	StepsWait    StepsStatus = "wait"
	StepsProcess StepsStatus = "process"
	StepsFinish  StepsStatus = "finish"
	StepsError   StepsStatus = "error"
)

// StepItem is one step (antd StepItem).
type StepItem struct {
	Title       string
	Content     string
	Description string // deprecated alias → Content
	SubTitle    string
	Disabled    bool
	Icon        string    // registry name; "loading" uses spinner (Ticker)
	IconNode    core.Node // preferred over Icon when non-nil
	Status      StepsStatus
}

// contentText returns Content, falling back to Description.
func (it StepItem) contentText() string {
	if it.Content != "" {
		return it.Content
	}
	return it.Description
}

// displayStep is one rendered slot (real item or ellipsis).
type displayStep struct {
	item        StepItem
	originIndex int // -1 for ellipsis
	ellipsis    bool
}

// Steps is Ant Design Steps (navigation stepper).
//
//	Flex root (role=navigation)
//	  [item Pressable] · rail · [item] · …
//
// Product contract: docs/antd/steps.md §6 (P0 DoD).
// Root identity is stable across rebuild (ClearChildren).
type Steps struct {
	Root *primitive.Flex

	Items []StepItem

	// Current is the active step index (0-based, antd current).
	Current int
	// Initial is the starting index offset (antd initial, default 0).
	Initial int
	// Status applies to the current step when item.status is empty (default process).
	Status StepsStatus

	Size           StepsSize
	Type           StepsType
	Variant        StepsVariant
	Orientation    StepsOrientation
	TitlePlacement StepsTitlePlacement

	// Percent is 0..100 for process icon ring; <0 means unset.
	Percent float64
	// MaxCount collapses when >=3 and items longer (antd maxCount).
	MaxCount int

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string

	// OnChange fires when a clickable step is activated (origin index).
	OnChange func(current int)

	// Controlled flags.
	currentControlled bool
	defaultCurrent    int
	appliedDefault    bool
	percentSet        bool

	// Test / chrome hooks keyed by origin index (ellipsis omitted).
	itemPress map[int]*primitive.Pressable
	iconBox   map[int]*primitive.Decorated // origin → icon chrome for size asserts
	// process icon paint state
	spinPhase   float64
	needSpinner bool
	boundTree   *core.Tree
	// last painted process color for STP-19
	processIconBg render.RGBA
}

// StepTitles builds StepItems from title strings (gallery / smoke convenience).
func StepTitles(titles ...string) []StepItem {
	out := make([]StepItem, len(titles))
	for i, t := range titles {
		out[i] = StepItem{Title: t}
	}
	return out
}

// NewSteps creates Steps with optional items (antd items).
func NewSteps(items ...StepItem) *Steps {
	s := &Steps{
		Items:          append([]StepItem(nil), items...),
		Current:        0,
		defaultCurrent: 0,
		Status:         StepsProcess,
		Size:           StepsMiddle,
		Type:           StepsTypeDefault,
		Variant:        StepsVariantFilled,
		Orientation:    StepsHorizontal,
		TitlePlacement: StepsTitleHorizontal,
		Percent:        -1,
	}
	s.rebuild()
	return s
}

// Node returns the stable root.
func (s *Steps) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// ChromeNode returns the navigation root (same as Node).
func (s *Steps) ChromeNode() core.Node { return s.Node() }

// SetItems replaces steps.
func (s *Steps) SetItems(items []StepItem) {
	if s == nil {
		return
	}
	s.Items = append([]StepItem(nil), items...)
	s.rebuild()
}

// SetCurrent sets controlled current (0-based).
func (s *Steps) SetCurrent(i int) {
	if s == nil {
		return
	}
	s.currentControlled = true
	s.Current = i
	s.rebuild()
}

// SetDefaultCurrent sets uncontrolled initial current (ignored once controlled).
func (s *Steps) SetDefaultCurrent(i int) {
	if s == nil {
		return
	}
	s.defaultCurrent = i
	if !s.currentControlled && !s.appliedDefault {
		s.Current = i
	}
	s.rebuild()
}

// SetInitial sets antd initial (index offset).
func (s *Steps) SetInitial(i int) {
	if s == nil {
		return
	}
	s.Initial = i
	s.rebuild()
}

// SetStatus sets overall status for the current step.
func (s *Steps) SetStatus(st StepsStatus) {
	if s == nil {
		return
	}
	if st == "" {
		st = StepsProcess
	}
	s.Status = st
	s.rebuild()
}

// SetSize sets middle | small.
func (s *Steps) SetSize(sz StepsSize) {
	if s == nil {
		return
	}
	s.Size = sz
	s.rebuild()
}

// SetType sets default | panel (+ P1 enums).
func (s *Steps) SetType(t StepsType) {
	if s == nil {
		return
	}
	s.Type = t
	s.rebuild()
}

// SetVariant sets filled | outlined.
func (s *Steps) SetVariant(v StepsVariant) {
	if s == nil {
		return
	}
	s.Variant = v
	s.rebuild()
}

// SetOrientation sets horizontal | vertical.
func (s *Steps) SetOrientation(o StepsOrientation) {
	if s == nil {
		return
	}
	s.Orientation = o
	s.rebuild()
}

// SetTitlePlacement sets label placement relative to icon.
func (s *Steps) SetTitlePlacement(p StepsTitlePlacement) {
	if s == nil {
		return
	}
	s.TitlePlacement = p
	s.rebuild()
}

// SetPercent sets process-step progress ring (0..100). Pass <0 to clear.
func (s *Steps) SetPercent(v float64) {
	if s == nil {
		return
	}
	if v < 0 {
		s.Percent = -1
		s.percentSet = false
	} else {
		if v > 100 {
			v = 100
		}
		s.Percent = v
		s.percentSet = true
	}
	s.rebuild()
}

// SetMaxCount sets collapse threshold (0 = off; >=3 applies when items longer).
func (s *Steps) SetMaxCount(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.MaxCount = n
	s.rebuild()
}

// SetOnChange sets the step click callback.
func (s *Steps) SetOnChange(fn func(current int)) {
	if s == nil {
		return
	}
	s.OnChange = fn
	s.rebuild()
}

// SetFace sets font face.
func (s *Steps) SetFace(face text.Face) {
	if s == nil {
		return
	}
	s.Face = face
	s.rebuild()
}

// SetTheme sets explicit theme.
func (s *Steps) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.rebuild()
}

// SetAriaLabel sets the accessible name on the navigation root.
func (s *Steps) SetAriaLabel(name string) {
	if s == nil {
		return
	}
	s.AriaLabel = name
	if s.Root != nil {
		s.Root.Base().Label = name
	}
}

// MappedCurrent returns current - initial (clamped to items).
func (s *Steps) MappedCurrent() int {
	if s == nil {
		return 0
	}
	m := s.Current - s.Initial
	n := len(s.Items)
	if n == 0 {
		return 0
	}
	if m < 0 {
		return 0
	}
	if m >= n {
		return n - 1
	}
	return m
}

// ItemStatus returns resolved status for origin index i.
func (s *Steps) ItemStatus(i int) StepsStatus {
	if s == nil || i < 0 || i >= len(s.Items) {
		return StepsWait
	}
	return s.resolveStatus(i, s.Items[i])
}

// ItemPressable returns the pressable for origin index (nil if ellipsis / missing).
func (s *Steps) ItemPressable(i int) *primitive.Pressable {
	if s == nil || s.itemPress == nil {
		return nil
	}
	return s.itemPress[i]
}

// IconSize returns the resolved icon container size for the current Size.
func (s *Steps) IconSize() float64 {
	if s == nil {
		return DefaultStepsIconSize
	}
	th := s.theme()
	if s.Size == StepsSmall {
		return th.SizeOr(core.TokenControlHeightSM, DefaultStepsIconSizeSM)
	}
	return th.SizeOr(core.TokenControlHeight, DefaultStepsIconSize)
}

// IconDecorated returns icon chrome for origin index (tests).
func (s *Steps) IconDecorated(i int) *primitive.Decorated {
	if s == nil || s.iconBox == nil {
		return nil
	}
	return s.iconBox[i]
}

// ProcessIconBg returns last process icon fill (Token primary path; tests).
func (s *Steps) ProcessIconBg() render.RGBA { return s.processIconBg }

// AttachTicker registers loading spinner for demand-frame ANIMATING.
func (s *Steps) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.needSpinner)
}

// Tick advances loading spinner phase.
func (s *Steps) Tick(dt float64) bool {
	if s == nil || !s.needSpinner {
		return false
	}
	s.spinPhase += dt * 1.2
	if s.spinPhase > 1 {
		s.spinPhase -= math.Floor(s.spinPhase)
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
	return true
}

func (s *Steps) theme() *core.Theme {
	var n core.Node
	if s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
}

func (s *Steps) resolveStatus(origin int, it StepItem) StepsStatus {
	if it.Status != "" {
		return it.Status
	}
	m := s.MappedCurrent()
	if origin < m {
		return StepsFinish
	}
	if origin == m {
		if s.Status == "" {
			return StepsProcess
		}
		return s.Status
	}
	return StepsWait
}

func (s *Steps) rebuild() {
	if s == nil {
		return
	}
	if !s.currentControlled && !s.appliedDefault {
		s.Current = s.defaultCurrent
		s.appliedDefault = true
	}

	th := s.theme()
	vertical := s.Orientation == StepsVertical
	panel := s.Type == StepsTypePanel
	iconSz := s.IconSize()
	titleFont := th.SizeOr(core.TokenFontSizeLG, DefaultStepsTitleFont)
	contentFont := th.SizeOr(core.TokenFontSize, DefaultStepsContentFont)
	iconFont := th.SizeOr(core.TokenFontSize, DefaultStepsIconFont)
	if s.Size == StepsSmall {
		iconFont = th.SizeOr(core.TokenFontSizeSM, DefaultStepsIconFontSM)
		titleFont = th.SizeOr(core.TokenFontSize, DefaultStepsContentFont)
	}
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	gap := DefaultStepsGap

	// Stable root: recreate only if axis must change.
	wantAxis := core.AxisHorizontal
	if vertical {
		wantAxis = core.AxisVertical
	}
	if s.Root == nil || s.Root.Axis != wantAxis {
		s.Root = primitive.NewFlex(wantAxis)
	} else {
		s.Root.ClearChildren()
	}
	s.Root.Gap = gap
	if panel && !vertical {
		s.Root.CrossAlign = core.CrossStretch
	} else {
		s.Root.CrossAlign = core.CrossStart
	}
	s.Root.MainAlign = core.MainStart
	s.Root.Base().Role = "navigation"
	if s.AriaLabel != "" {
		s.Root.Base().Label = s.AriaLabel
	} else {
		s.Root.Base().Label = "steps"
	}
	s.Root.SetThemeHook(func(*core.Theme) { s.rebuild() })

	s.itemPress = make(map[int]*primitive.Pressable)
	s.iconBox = make(map[int]*primitive.Decorated)
	s.needSpinner = false
	s.processIconBg = render.RGBA{}

	display := s.buildDisplaySteps()
	clickable := s.OnChange != nil

	for di, ds := range display {
		node := s.buildItemNode(ds, iconSz, titleFont, contentFont, iconFont, radius, lineW, panel, clickable, th)
		// Horizontal non-panel: grow items so rails can sit between; use flexible wrap.
		if !vertical && !panel {
			// Keep intrinsic; rail between uses Flexible grow.
			s.Root.AddChild(node)
		} else if panel && !vertical {
			// Equal-width panels via Flexible grow.
			flex := primitive.NewFlexible(1, node)
			s.Root.AddChild(flex)
		} else {
			s.Root.AddChild(node)
		}

		// Rail between display items (not for panel type).
		if !panel && di < len(display)-1 {
			rail := s.buildRail(ds, display[di+1], vertical, iconSz, lineW, th)
			s.Root.AddChild(rail)
		}
	}

	if s.boundTree != nil {
		s.boundTree.BindTicker(s, s.needSpinner)
	}
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

func (s *Steps) buildDisplaySteps() []displayStep {
	items := s.Items
	n := len(items)
	if n == 0 {
		return nil
	}
	mapped := s.MappedCurrent()
	canCollapse := s.MaxCount >= 3 && n > s.MaxCount
	if !canCollapse {
		out := make([]displayStep, n)
		for i, it := range items {
			out[i] = displayStep{item: it, originIndex: i}
		}
		return out
	}
	indexes := collapsedIndexes(n, mapped, s.MaxCount)
	out := make([]displayStep, 0, len(indexes))
	for i, idx := range indexes {
		if idx < 0 {
			// ellipsis between prev and next real indexes
			prev := indexes[i-1]
			next := indexes[i+1]
			out = append(out, displayStep{
				item:        ellipsisItem(items, mapped, prev, next),
				originIndex: -1,
				ellipsis:    true,
			})
			continue
		}
		out = append(out, displayStep{item: items[idx], originIndex: idx})
	}
	return out
}

// collapsedIndexes ports antd useDisplaySteps getCollapsedIndexes.
// Negative sentinel (-1) marks ellipsis slots.
func collapsedIndexes(total, current, maxCount int) []int {
	if total <= 0 {
		return nil
	}
	safeCurrent := current
	if safeCurrent < 0 {
		safeCurrent = 0
	}
	if safeCurrent >= total {
		safeCurrent = total - 1
	}
	target := maxCount
	if target > total {
		target = total
	}
	set := map[int]struct{}{0: {}, safeCurrent: {}, total - 1: {}}
	for distance := 1; len(set) < target && distance < total; distance++ {
		for _, index := range []int{
			safeCurrent - distance,
			safeCurrent + distance,
			distance,
			total - 1 - distance,
		} {
			if len(set) >= target {
				break
			}
			if index >= 0 && index < total {
				set[index] = struct{}{}
			}
		}
	}
	sorted := make([]int, 0, len(set))
	for i := 0; i < total; i++ {
		if _, ok := set[i]; ok {
			sorted = append(sorted, i)
		}
	}
	out := make([]int, 0, len(sorted)*2)
	for order, index := range sorted {
		if order > 0 && index-sorted[order-1] > 1 {
			out = append(out, -1) // ellipsis
		}
		out = append(out, index)
	}
	return out
}

func ellipsisItem(items []StepItem, mapped, prev, next int) StepItem {
	hasError := false
	for i := prev + 1; i < next && i < len(items); i++ {
		if items[i].Status == StepsError {
			hasError = true
			break
		}
	}
	st := StepsWait
	if hasError {
		st = StepsError
	} else if next-1 < mapped {
		st = StepsFinish
	}
	return StepItem{
		Title:    "",
		Icon:     "ellipsis",
		Status:   st,
		Disabled: true,
	}
}

func (s *Steps) buildItemNode(
	ds displayStep,
	iconSz, titleFont, contentFont, iconFont, radius, lineW float64,
	panel, clickable bool,
	th *core.Theme,
) core.Node {
	st := StepsWait
	if ds.ellipsis {
		st = ds.item.Status
		if st == "" {
			st = StepsWait
		}
	} else {
		st = s.resolveStatus(ds.originIndex, ds.item)
	}

	titlePlacementVertical := s.TitlePlacement == StepsTitleVertical || s.Orientation == StepsVertical && s.TitlePlacement == StepsTitleHorizontal && false
	// When orientation is vertical, antd still puts title to the right of icon by default
	// (titlePlacement horizontal). titlePlacement vertical stacks under icon.
	_ = titlePlacementVertical
	stackLabelUnder := s.TitlePlacement == StepsTitleVertical

	iconNode, iconDec := s.buildIcon(ds, st, iconSz, iconFont, lineW, th)
	if !ds.ellipsis && ds.originIndex >= 0 && iconDec != nil {
		s.iconBox[ds.originIndex] = iconDec
	}

	// Text column: title row (+ subTitle) + content
	textCol := primitive.Column()
	textCol.Gap = 2
	textCol.CrossAlign = core.CrossStart
	textCol.MainAlign = core.MainStart

	titleRow := primitive.Row()
	titleRow.Gap = 8
	titleRow.CrossAlign = core.CrossCenter
	titleColor := s.titleColor(st, th)
	titleKids := 0
	if ds.item.Title != "" || ds.ellipsis {
		lab := primitive.NewText(ds.item.Title)
		if ds.ellipsis {
			lab.Value = "…"
		}
		lab.FontSize = titleFont
		lab.Face = s.Face
		lab.Color = titleColor
		titleRow.AddChild(lab)
		titleKids++
	}
	if ds.item.SubTitle != "" {
		sub := primitive.NewText(ds.item.SubTitle)
		sub.FontSize = contentFont
		sub.Face = s.Face
		sub.Color = th.Color(core.TokenColorTextSecondary)
		titleRow.AddChild(sub)
		titleKids++
	}
	if titleKids > 0 {
		textCol.AddChild(titleRow)
	}
	if c := ds.item.contentText(); c != "" {
		body := primitive.NewText(c)
		body.FontSize = contentFont
		body.Face = s.Face
		body.Color = s.contentColor(st, th)
		textCol.AddChild(body)
	}

	var body core.Node
	if stackLabelUnder {
		col := primitive.Column(iconNode, textCol)
		col.Gap = 8
		col.CrossAlign = core.CrossCenter
		col.MainAlign = core.MainStart
		body = col
	} else {
		row := primitive.Row(iconNode, textCol)
		row.Gap = 8
		row.CrossAlign = core.CrossStart
		if s.Orientation == StepsVertical {
			row.CrossAlign = core.CrossStart
		} else {
			row.CrossAlign = core.CrossCenter
		}
		body = row
	}

	if panel {
		pad := DefaultStepsPanelPadding
		dec := primitive.NewDecorated(body)
		dec.Padding = primitive.All(pad)
		dec.Radius = radius
		dec.BorderWidth = lineW
		bg, bd := s.panelChrome(st, th)
		dec.Background = bg
		dec.BorderColor = bd
		body = dec
	}

	disabled := ds.item.Disabled || ds.ellipsis
	canClick := clickable && !disabled && !ds.ellipsis && ds.originIndex >= 0

	// Always wrap in Pressable for consistent hit==layout==paint and focus when clickable.
	pr := primitive.NewPressable(body)
	pr.ShowFocusRing = canClick
	pr.FocusRingRadius = radius
	pr.FocusRingOutset = DefaultStepsFocusOutset
	pr.Focusable = canClick
	pr.SetDisabled(disabled || !canClick)
	if canClick {
		origin := ds.originIndex
		pr.Click = func() {
			s.activate(origin)
		}
		pr.Base().Role = "button"
		name := ds.item.Title
		if name == "" {
			name = fmt.Sprintf("step %d", origin+1)
		}
		pr.Base().Label = name
		if st == StepsProcess || st == StepsError {
			pr.Base().Label = name + ", current"
		}
	} else {
		pr.Focusable = false
		pr.ShowFocusRing = false
		pr.Base().Role = "listitem"
		if ds.item.Title != "" {
			pr.Base().Label = ds.item.Title
		}
	}
	if !ds.ellipsis && ds.originIndex >= 0 {
		s.itemPress[ds.originIndex] = pr
	}
	return pr
}

func (s *Steps) activate(origin int) {
	if s == nil {
		return
	}
	if origin < 0 || origin >= len(s.Items) {
		return
	}
	if s.Items[origin].Disabled {
		return
	}
	next := origin + s.Initial
	if !s.currentControlled {
		s.Current = next
	}
	fn := s.OnChange
	if !s.currentControlled {
		s.rebuild()
	}
	if fn != nil {
		fn(next)
	}
}

func (s *Steps) buildIcon(
	ds displayStep,
	st StepsStatus,
	iconSz, iconFont, lineW float64,
	th *core.Theme,
) (core.Node, *primitive.Decorated) {
	bg, bd, fg := s.iconChrome(st, th)
	if st == StepsProcess {
		s.processIconBg = bg
	}

	var inner core.Node
	useCustom := ds.item.IconNode != nil || ds.item.Icon != ""
	loadingIcon := strings.EqualFold(ds.item.Icon, "loading")

	if ds.item.IconNode != nil {
		inner = ds.item.IconNode
	} else if loadingIcon {
		s.needSpinner = true
		spin := primitive.NewCanvas(iconFont+2, iconFont+2, s.paintSpinner)
		inner = spin
	} else if ds.item.Icon != "" {
		// Built-in names; unknown names fall back to check/ellipsis glyph via registry.
		name := ds.item.Icon
		if name == "ellipsis" {
			// No built-in ellipsis icon — use text.
			tx := primitive.NewText("…")
			tx.FontSize = iconFont
			tx.Face = s.Face
			tx.Color = fg
			inner = tx
		} else {
			ic := primitive.NewIcon(name)
			ic.Size = iconFont + 2
			ic.Color = fg
			inner = ic
		}
	} else {
		// Default: number / check / close
		switch st {
		case StepsFinish:
			ic := primitive.NewIcon("check")
			ic.Size = iconFont
			ic.Color = fg
			inner = ic
		case StepsError:
			ic := primitive.NewIcon("close")
			ic.Size = iconFont
			ic.Color = fg
			inner = ic
		default:
			num := ds.originIndex + 1
			if ds.ellipsis {
				num = 0
			}
			label := fmt.Sprintf("%d", num)
			if ds.ellipsis {
				label = "…"
			}
			tx := primitive.NewText(label)
			tx.FontSize = iconFont
			tx.Face = s.Face
			tx.Color = fg
			inner = tx
		}
	}

	// Percent ring around process default icon (type=default only).
	// Number stays centered; primary border + label encode percent (L2).
	// Full arc is painted via Canvas child when possible.
	showPercent := s.percentSet && s.Percent >= 0 && st == StepsProcess && s.Type == StepsTypeDefault && !useCustom
	if showPercent {
		primary := th.Color(core.TokenColorPrimary)
		// Canvas draws progress arc; text number remains as centered child.
		ring := primitive.NewCanvas(iconSz, iconSz, func(pc *core.PaintContext, sz core.Size) {
			s.paintPercentRing(pc, sz, th)
		})
		// Prefer visible number over arc-only: Decorated chrome + percent label.
		_ = ring
		dec := primitive.NewDecorated(inner)
		dec.Width, dec.Height = iconSz, iconSz
		dec.Radius = iconSz / 2
		dec.SetCenterContent(true)
		dec.Background = bg
		dec.BorderWidth = 2.5
		dec.BorderColor = primary
		dec.Base().Label = fmt.Sprintf("percent=%.0f", s.Percent)
		return dec, dec
	}

	dec := primitive.NewDecorated(inner)
	dec.Width, dec.Height = iconSz, iconSz
	dec.Radius = iconSz / 2
	dec.SetCenterContent(true)
	dec.Background = bg
	if s.Variant == StepsVariantOutlined || st == StepsWait || st == StepsError && s.Variant == StepsVariantOutlined {
		dec.BorderWidth = lineW
		dec.BorderColor = bd
	} else if s.Variant == StepsVariantOutlined {
		dec.BorderWidth = lineW
		dec.BorderColor = bd
	} else if st == StepsWait {
		// filled wait still has subtle edge
		dec.BorderWidth = 0
	}
	// Custom icon: often no fill circle chrome in antd — keep light container for hit stability.
	if useCustom && !loadingIcon && ds.item.Icon != "ellipsis" {
		// Transparent-ish container; icon color carries status.
		if st == StepsProcess || st == StepsFinish {
			dec.Background = render.RGBA{}
			dec.BorderWidth = 0
		}
	}
	return dec, dec
}

func (s *Steps) paintPercentRing(pc *core.PaintContext, sz core.Size, th *core.Theme) {
	if pc == nil {
		return
	}
	primary := th.Color(core.TokenColorPrimary)
	track := th.Color(core.TokenColorBorder)
	cx, cy := sz.Width/2, sz.Height/2
	stroke := 2.5
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	pct := s.Percent
	if pct < 0 {
		pct = 0
	}
	start := -math.Pi / 2
	end := start + 2*math.Pi*(pct/100)
	steps := 48
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		a := start + (end-start)*t
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	if len(pts) >= 4 {
		pc.StrokeLocalPolyline(pts, stroke, primary)
	}
}

func (s *Steps) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil {
		return
	}
	th := s.theme()
	col := th.Color(core.TokenColorPrimary)
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.35}
	stroke := 2.0
	if sz.Width < 14 {
		stroke = 1.5
	}
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + s.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 40
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		a := start + (end-start)*t
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	if len(pts) >= 4 {
		pc.StrokeLocalPolyline(pts, stroke, col)
	}
}

func (s *Steps) buildRail(prev, next displayStep, vertical bool, iconSz, lineW float64, th *core.Theme) core.Node {
	// Color from previous step status (finish → primary).
	st := StepsWait
	if prev.ellipsis {
		st = prev.item.Status
	} else {
		st = s.resolveStatus(prev.originIndex, prev.item)
	}
	col := th.Color(core.TokenColorBorder)
	switch st {
	case StepsFinish:
		col = th.Color(core.TokenColorPrimary)
	case StepsError:
		col = th.Color(core.TokenColorError)
	case StepsProcess:
		col = th.Color(core.TokenColorPrimary)
	}

	line := primitive.NewBox()
	line.Color = col
	if vertical {
		line.Width = lineW
		line.Height = DefaultStepsRailMin
		// center under icon: pad start roughly (icon/2 - line/2) via Flexible? keep simple.
		holder := primitive.Row(line)
		holder.Padding = primitive.EdgeInsets{Left: iconSz/2 - lineW/2}
		return holder
	}
	// Horizontal: flexible grow rail, height = lineW, vertically centered with icon.
	line.Height = lineW
	line.Width = DefaultStepsRailMin
	flexLine := primitive.NewFlexible(1, line)
	// Wrap to center vertically relative to icon row.
	row := primitive.Row(flexLine)
	row.CrossAlign = core.CrossCenter
	row.ExpandMax = false
	// Flexible needs to be direct Flex child of root — return flexLine directly.
	_ = row
	return flexLine
}

func (s *Steps) iconChrome(st StepsStatus, th *core.Theme) (bg, bd, fg render.RGBA) {
	primary := th.Color(core.TokenColorPrimary)
	primaryBg := th.Color(core.TokenColorPrimaryBg)
	textInv := th.Color(core.TokenColorTextInverse)
	textSec := th.Color(core.TokenColorTextSecondary)
	fillSec := th.Color(core.TokenColorFillSecondary)
	errC := th.Color(core.TokenColorError)
	border := th.Color(core.TokenColorBorder)
	container := th.Color(core.TokenColorBgContainer)

	switch st {
	case StepsProcess:
		if s.Variant == StepsVariantOutlined {
			bg = container
			bd = primary
			fg = primary
		} else {
			bg = primary
			bd = primary
			fg = textInv
		}
	case StepsFinish:
		if s.Variant == StepsVariantOutlined {
			bg = container
			bd = primary
			fg = primary
		} else {
			bg = primaryBg
			if bg.A < 0.05 {
				bg = primary
				fg = textInv
			} else {
				fg = primary
			}
			bd = primaryBg
		}
	case StepsError:
		if s.Variant == StepsVariantOutlined {
			bg = container
			bd = errC
			fg = errC
		} else {
			bg = errC
			bd = errC
			fg = textInv
		}
	default: // wait
		bg = fillSec
		bd = border
		fg = textSec
		if s.Variant == StepsVariantOutlined {
			bg = container
			bd = border
		}
	}
	return bg, bd, fg
}

func (s *Steps) panelChrome(st StepsStatus, th *core.Theme) (bg, bd render.RGBA) {
	ib, id, _ := s.iconChrome(st, th)
	// Panel reuses icon bg/border semantics.
	return ib, id
}

func (s *Steps) titleColor(st StepsStatus, th *core.Theme) render.RGBA {
	switch st {
	case StepsWait:
		return th.Color(core.TokenColorTextSecondary)
	case StepsError:
		return th.Color(core.TokenColorError)
	default:
		return th.Color(core.TokenColorText)
	}
}

func (s *Steps) contentColor(st StepsStatus, th *core.Theme) render.RGBA {
	if st == StepsError {
		return th.Color(core.TokenColorError)
	}
	return th.Color(core.TokenColorTextSecondary)
}

// DisplayCount returns number of rendered slots (including ellipsis). For tests.
func (s *Steps) DisplayCount() int {
	return len(s.buildDisplaySteps())
}

// HasEllipsis reports whether maxCount collapse inserted an ellipsis. For tests.
func (s *Steps) HasEllipsis() bool {
	for _, ds := range s.buildDisplaySteps() {
		if ds.ellipsis {
			return true
		}
	}
	return false
}
