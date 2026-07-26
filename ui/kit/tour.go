package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Tour defaults — docs/antd/tour.md §6.2 / §6.10
// Source: components/tour/style/index.ts (prepareComponentToken + mergeToken)
// https://ant.design/components/tour
const (
	// DefaultTourFontSize is content fontSize.
	DefaultTourFontSize = 14.0
	// DefaultTourTitleFont is title size (slightly stronger than body).
	DefaultTourTitleFont = 16.0
	// DefaultTourPadding is panel padding (token padding).
	DefaultTourPadding = 16.0
	// DefaultTourGap is internal content gap (paddingXS / marginXS).
	DefaultTourInnerGap = 8.0
	// DefaultTourRadius is tourBorderRadius ← borderRadiusLG.
	DefaultTourRadius = 8.0
	// DefaultTourMaxWidth is CSS width reference for the panel.
	DefaultTourMaxWidth = 520.0
	// DefaultTourMinWidth keeps a usable card when content is short.
	DefaultTourMinWidth = 280.0
	// DefaultTourGapOffset is gap.offset default.
	DefaultTourGapOffset = 6.0
	// DefaultTourGapRadius is gap.radius default.
	DefaultTourGapRadius = 2.0
	// DefaultTourIndicator is indicatorWidth/Height.
	DefaultTourIndicator = 6.0
	// DefaultTourCloseBtn is closeBtnSize ≈ fontSize * lineHeight.
	DefaultTourCloseBtn = 22.0
	// DefaultTourFocusOutset approximates focus-visible outset.
	DefaultTourFocusOutset = 1.5
	// DefaultTourZIndex is zIndexPopup (zIndexPopupBase+70 ≈ 1001); kit uses OverlayZTour.
	DefaultTourZIndex = 1001
	// DefaultTourArrow shows the placement arrow.
	DefaultTourArrow = true
	// DefaultTourKeyboard enables Esc close.
	DefaultTourKeyboard = true
	// DefaultTourMask enables the dim mask.
	DefaultTourMask = true
	// Tour locale strings (antd TourLocale).
	DefaultTourLocalePrev   = "Previous"
	DefaultTourLocaleNext   = "Next"
	DefaultTourLocaleFinish = "Finish"
)

// TourType is antd type (default | primary).
type TourType int

const (
	// TourTypeDefault is neutral elevated panel. Default.
	TourTypeDefault TourType = iota
	// TourTypePrimary uses primary fill + inverse text.
	TourTypePrimary
)

// TourPlacement is antd placement relative to target.
type TourPlacement int

const (
	// TourBottom is the default placement.
	TourBottom TourPlacement = iota
	TourBottomLeft
	TourBottomRight
	TourTop
	TourTopLeft
	TourTopRight
	TourLeft
	TourLeftTop
	TourLeftBottom
	TourRight
	TourRightTop
	TourRightBottom
	// TourCenter centers the panel in the viewport (or when target empty).
	TourCenter
)

// TourGap controls the highlight hole padding and radius (antd gap).
type TourGap struct {
	// Offset is uniform inset when OffsetX/Y are both zero and Offset > 0.
	// When OffsetX or OffsetY is set (via SetGapXY), those win.
	Offset  float64
	OffsetX float64
	OffsetY float64
	Radius  float64
	// set flags distinguish "unset → default" from explicit zero.
	offsetSet bool
	xySet     bool
	radiusSet bool
}

// TourButtonProps is antd nextButtonProps / prevButtonProps (P0 subset).
type TourButtonProps struct {
	// Children overrides the button label.
	Children string
	// OnClick is invoked before the default next/prev action (antd allows custom).
	OnClick func()
	// Style is a shallow chrome override (optional).
	Style Style
	// Disabled forces disabled chrome when true.
	Disabled bool
}

// TourStyles is a shallow semantic styles map (antd styles; P0: mask/section/title/description).
type TourStyles struct {
	Root        Style
	Mask        Style
	Section     Style
	Cover       Style
	Header      Style
	Title       Style
	Description Style
	Footer      Style
	Actions     Style
	Indicators  Style
	Close       Style
}

// TourStep is one guided step card (antd TourStep).
//
// Breaking vs M5 lite: Body → Description; prefer Title/Description over ad-hoc fields.
type TourStep struct {
	// Title is the step heading.
	Title string
	// TitleNode overrides Title when non-nil.
	TitleNode core.Node
	// Description is the main body (antd description). Replaces deprecated Body.
	Description string
	// DescriptionNode overrides Description when non-nil.
	DescriptionNode core.Node
	// Body is a deprecated alias for Description (kept so smoke demos compile once;
	// prefer Description). Empty when Description is set.
	// Deprecated: use Description.
	Body string
	// Cover is an optional image/video node above the title.
	Cover core.Node
	// Target is the absolute highlight rect (host updates each frame). Empty → center.
	Target core.Rect
	// Placement overrides Tour-level placement when not zero-value default with placeSet.
	Placement TourPlacement
	// placeSet marks Placement as explicit (including TourBottom).
	placeSet bool
	// Type overrides Tour-level type when typeSet.
	Type    TourType
	typeSet bool
	// Mask / MaskColor override Tour mask when maskSet.
	// Mask=false disables mask for this step even if Tour mask is on.
	Mask      bool
	MaskColor render.RGBA
	maskSet   bool
	// Arrow overrides Tour arrow when arrowSet.
	Arrow    bool
	arrowSet bool
	// NextButtonProps / PrevButtonProps customize footer buttons.
	NextButtonProps *TourButtonProps
	PrevButtonProps *TourButtonProps
	// OnClose is step-level close hook (fires with tour OnClose).
	OnClose func()
}

// SetPlacement marks step placement as explicit.
func (s *TourStep) SetPlacement(p TourPlacement) {
	if s == nil {
		return
	}
	s.Placement = p
	s.placeSet = true
}

// SetType marks step type as explicit.
func (s *TourStep) SetType(tp TourType) {
	if s == nil {
		return
	}
	s.Type = tp
	s.typeSet = true
}

// SetMask sets step mask override (bool + optional color).
func (s *TourStep) SetMask(on bool, color render.RGBA) {
	if s == nil {
		return
	}
	s.Mask = on
	s.MaskColor = color
	s.maskSet = true
}

// SetArrow sets step arrow override.
func (s *TourStep) SetArrow(on bool) {
	if s == nil {
		return
	}
	s.Arrow = on
	s.arrowSet = true
}

// descriptionText resolves Description, falling back to deprecated Body.
func (s TourStep) descriptionText() string {
	if s.Description != "" {
		return s.Description
	}
	return s.Body
}

// Tour is Ant Design Tour — multi-step spotlight guide.
//
//	OverlayPortal
//	  └─ FocusScope
//	       └─ tourLayer
//	            ├─ Mask? (full viewport; hole is visual only)
//	            ├─ highlight hole (Decorated stroke)
//	            └─ panel (cover / title / description / footer)
//
// Product contract: docs/antd/tour.md §6 (P0 DoD).
// Portal/Scope stay stable across step changes (rebuild refreshes layer content only).
type Tour struct {
	Portal *primitive.OverlayPortal
	Scope  *primitive.FocusScope

	Steps []TourStep

	// Open is the current visibility (also set by SetOpen).
	Open bool
	// Current is the 0-based step index (antd current).
	// Breaking: was Index.
	Current int
	// Index is a deprecated alias of Current for transitional call sites.
	// Prefer Current / SetCurrent. Kept in sync by SetCurrent/Next/Prev.
	// Deprecated: use Current.
	Index int

	// Type is default | primary (panel chrome).
	Type TourType
	// Placement is the default step placement.
	Placement TourPlacement
	// Mask enables the dim layer (false → non-modal).
	Mask bool
	// MaskColor overrides colorBgMask when A>0.
	MaskColor render.RGBA
	// Gap controls highlight hole geometry.
	Gap TourGap
	// Arrow shows a simple placement caret (P0 geometry simplified).
	Arrow bool
	// Keyboard enables Esc → close (antd keyboard, default true).
	Keyboard bool
	// CloseIcon shows the header close control (default true).
	CloseIcon bool
	// DisabledInteraction blocks pointer through the highlight hole.
	DisabledInteraction bool
	// ZIndex is informational; portal uses OverlayZTour (maps ~1001 ladder).
	ZIndex int

	// Face / Theme / Viewport / Style
	Face      text.Face
	Theme     *core.Theme
	Viewport  core.Size
	Style     Style
	Styles    TourStyles
	AriaLabel string

	// Callbacks
	OnChange func(current int)
	OnClose  func()
	OnFinish func()

	// IndicatorsRender customizes the step dots / text (current is 0-based).
	IndicatorsRender func(current, total int) core.Node
	// ActionsRender wraps or replaces footer action buttons.
	// origin is the default Prev+Next row node.
	ActionsRender func(origin core.Node, current, total int) core.Node

	// Locale strings (0 → defaults).
	LocalePrev   string
	LocaleNext   string
	LocaleFinish string

	// internal
	layer *tourLayer
	trap  overlayFocusTrap

	openControlled    bool
	currentControlled bool
	defaultCurrent    int
	defaultCurrentSet bool
	appliedDefault    bool

	// chrome hooks for tests
	panel       *primitive.Decorated
	maskNode    *primitive.Mask
	holeNode    *primitive.Decorated
	titleNode   *primitive.Text
	bodyNode    *primitive.Text
	infoNode    *primitive.Text
	prevBtn     *Button
	nextBtn     *Button
	closeBtn    *Button
	footerNode  core.Node
	indicatorsN core.Node

	// metrics cache (L2)
	fontSize  float64
	titleFont float64
	padding   float64
	innerGap  float64
	radius    float64
	maxWidth  float64
	indicator float64
	gapOffX   float64
	gapOffY   float64
	gapRadius float64
}

type tourLayer struct {
	core.NodeBase
	tour *Tour
}

// NewTour creates a closed tour with optional initial steps.
func NewTour(steps ...TourStep) *Tour {
	t := &Tour{
		Steps:     steps,
		Mask:      DefaultTourMask,
		Arrow:     DefaultTourArrow,
		Keyboard:  DefaultTourKeyboard,
		CloseIcon: true,
		Placement: TourBottom,
		Type:      TourTypeDefault,
		ZIndex:    DefaultTourZIndex,
	}
	t.ensureShell()
	return t
}

// Node returns the portal host node to place in the tree.

// ensureBuilt materializes the control tree if missing (#9).
func (t *Tour) ensureBuilt() {
	if t == nil {
		return
	}
	// Portal created in NewTour / SetOpen path
}

// structureChange rebuilds the control tree (#9).
func (t *Tour) structureChange() {
	if t == nil {
		return
	}
	// Tour refreshes layer content via SetOpen/step APIs
}

// chromeChange refreshes chrome via rebuild (#9).
func (t *Tour) chromeChange() {
	if t == nil {
		return
	}
}

func (t *Tour) Node() core.Node {
	if t == nil {
		return nil
	}
	t.ensureShell()
	return t.Portal
}

// Panel returns the current panel node (test hook; nil when closed/not built).
func (t *Tour) Panel() core.Node {
	if t == nil {
		return nil
	}
	return t.panel
}

// MaskNode returns the mask (test hook).
func (t *Tour) MaskNode() *primitive.Mask {
	if t == nil {
		return nil
	}
	return t.maskNode
}

// HoleNode returns the highlight stroke box (test hook).
func (t *Tour) HoleNode() *primitive.Decorated {
	if t == nil {
		return nil
	}
	return t.holeNode
}

// IsOpen reports visibility.
func (t *Tour) IsOpen() bool {
	return t != nil && t.Open
}

// SetSteps replaces the step list and clamps current.
func (t *Tour) SetSteps(steps ...TourStep) {
	if t == nil {
		return
	}
	t.Steps = steps
	t.clampCurrent()
	t.invalidateStep()
}

// SetOpen shows/hides the tour (antd open — controlled).
func (t *Tour) SetOpen(open bool) {
	if t == nil {
		return
	}
	t.ensureShell()
	t.openControlled = true
	was := t.Open
	t.Open = open
	if t.Portal != nil {
		t.Portal.SetOpen(open)
	}
	if open {
		if !t.currentControlled && t.defaultCurrentSet && !t.appliedDefault {
			t.Current = t.defaultCurrent
			t.Index = t.Current
			t.appliedDefault = true
			t.clampCurrent()
		}
		t.layer.MarkNeedsLayout()
		if t.Keyboard {
			t.trap.wire(t.Scope, true, t.onEscape)
		} else {
			t.trap.wire(t.Scope, false, nil)
		}
		var prefer core.Node
		if t.nextBtn != nil {
			prefer = t.nextBtn.Root
		}
		t.trap.enter(t.Scope, t.Portal, prefer)
	} else {
		t.trap.wire(t.Scope, false, nil)
		if was {
			t.trap.leave(t.Scope, t.Portal)
			if t.OnClose != nil {
				t.OnClose()
			}
			if step, ok := t.stepAt(t.Current); ok && step.OnClose != nil {
				step.OnClose()
			}
		}
	}
}

// Close ends the tour (same as SetOpen(false) without forcing controlled if already closed path).
func (t *Tour) Close() {
	if t == nil {
		return
	}
	t.SetOpen(false)
}

// SetCurrent jumps to step index (antd current — controlled).
func (t *Tour) SetCurrent(i int) {
	if t == nil {
		return
	}
	t.currentControlled = true
	t.setCurrentInternal(i, false)
}

// SetDefaultCurrent sets uncontrolled initial current (ignored once controlled).
func (t *Tour) SetDefaultCurrent(i int) {
	if t == nil {
		return
	}
	t.defaultCurrent = i
	t.defaultCurrentSet = true
	if !t.currentControlled && !t.Open {
		t.Current = i
		t.Index = i
		t.clampCurrent()
	}
}

// SetOnChange sets the step-change callback.
func (t *Tour) SetOnChange(fn func(current int)) {
	if t == nil {
		return
	}
	t.OnChange = fn
}

// SetOnClose sets the close callback.
func (t *Tour) SetOnClose(fn func()) {
	if t == nil {
		return
	}
	t.OnClose = fn
}

// SetOnFinish sets the finish callback (last step Next).
func (t *Tour) SetOnFinish(fn func()) {
	if t == nil {
		return
	}
	t.OnFinish = fn
}

// SetType sets Tour type (default | primary).
func (t *Tour) SetType(tp TourType) {
	if t == nil {
		return
	}
	t.Type = tp
	t.invalidateStep()
}

// SetPlacement sets default placement.
func (t *Tour) SetPlacement(p TourPlacement) {
	if t == nil {
		return
	}
	t.Placement = p
	t.invalidateStep()
}

// SetMask enables/disables the dim mask (false → non-modal).
func (t *Tour) SetMask(on bool) {
	if t == nil {
		return
	}
	t.Mask = on
	t.invalidateStep()
}

// SetMaskColor overrides mask color (A>0 applies).
func (t *Tour) SetMaskColor(c render.RGBA) {
	if t == nil {
		return
	}
	t.MaskColor = c
	t.invalidateStep()
}

// SetGap sets uniform gap offset + radius (antd gap).
func (t *Tour) SetGap(offset, radius float64) {
	if t == nil {
		return
	}
	t.Gap.Offset = offset
	t.Gap.Radius = radius
	t.Gap.offsetSet = true
	t.Gap.radiusSet = true
	t.Gap.xySet = false
	t.invalidateStep()
}

// SetGapXY sets per-axis offset + radius.
func (t *Tour) SetGapXY(offsetX, offsetY, radius float64) {
	if t == nil {
		return
	}
	t.Gap.OffsetX = offsetX
	t.Gap.OffsetY = offsetY
	t.Gap.Radius = radius
	t.Gap.xySet = true
	t.Gap.radiusSet = true
	t.invalidateStep()
}

// SetArrow toggles the placement arrow.
func (t *Tour) SetArrow(on bool) {
	if t == nil {
		return
	}
	t.Arrow = on
	t.invalidateStep()
}

// SetKeyboard enables Esc close.
func (t *Tour) SetKeyboard(on bool) {
	if t == nil {
		return
	}
	t.Keyboard = on
	if t.Open {
		if on {
			t.trap.wire(t.Scope, true, t.onEscape)
		} else {
			t.trap.wire(t.Scope, false, nil)
		}
	}
}

// SetCloseIcon shows/hides the close control.
func (t *Tour) SetCloseIcon(on bool) {
	if t == nil {
		return
	}
	t.CloseIcon = on
	t.invalidateStep()
}

// SetDisabledInteraction blocks highlight-hole interaction.
func (t *Tour) SetDisabledInteraction(on bool) {
	if t == nil {
		return
	}
	t.DisabledInteraction = on
	t.invalidateStep()
}

// SetIndicatorsRender customizes the indicator area.
func (t *Tour) SetIndicatorsRender(fn func(current, total int) core.Node) {
	if t == nil {
		return
	}
	t.IndicatorsRender = fn
	t.invalidateStep()
}

// SetActionsRender customizes the footer actions.
func (t *Tour) SetActionsRender(fn func(origin core.Node, current, total int) core.Node) {
	if t == nil {
		return
	}
	t.ActionsRender = fn
	t.invalidateStep()
}

// SetStyles sets shallow semantic style overrides.
func (t *Tour) SetStyles(st TourStyles) {
	if t == nil {
		return
	}
	t.Styles = st
	t.invalidateStep()
}

// SetTheme sets the theme override.
func (t *Tour) SetTheme(th *core.Theme) {
	if t == nil {
		return
	}
	t.Theme = th
	t.invalidateStep()
}

// SetFace applies the product font.
func (t *Tour) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	if t.titleNode != nil {
		t.titleNode.Face = face
	}
	if t.bodyNode != nil {
		t.bodyNode.Face = face
	}
	if t.infoNode != nil {
		t.infoNode.Face = face
	}
	if t.prevBtn != nil {
		t.prevBtn.SetFace(face)
	}
	if t.nextBtn != nil {
		t.nextBtn.SetFace(face)
	}
	if t.closeBtn != nil {
		t.closeBtn.SetFace(face)
	}
}

// SetAriaLabel sets the dialog accessible name.
func (t *Tour) SetAriaLabel(s string) {
	if t == nil {
		return
	}
	t.AriaLabel = s
	if t.layer != nil {
		if s != "" {
			t.layer.Label = s
		} else {
			t.layer.Label = "Tour"
		}
	}
}

// Next advances one step, or finishes on the last step.
func (t *Tour) Next() {
	if t == nil || len(t.Steps) == 0 {
		if t != nil {
			t.finish()
		}
		return
	}
	if t.Current+1 < len(t.Steps) {
		next := t.Current + 1
		if t.currentControlled {
			if t.OnChange != nil {
				t.OnChange(next)
			}
			return
		}
		t.setCurrentInternal(next, true)
		return
	}
	t.finish()
}

// Prev goes back one step.
func (t *Tour) Prev() {
	if t == nil || t.Current <= 0 {
		return
	}
	prev := t.Current - 1
	if t.currentControlled {
		if t.OnChange != nil {
			t.OnChange(prev)
		}
		return
	}
	t.setCurrentInternal(prev, true)
}

// Sync repositions for viewport.
// Deprecated: prefer Tree.Layout (automatic). Kept for one-shot forced refresh.
func (t *Tour) Sync() {
	if t == nil {
		return
	}
	if t.prevBtn != nil {
		t.prevBtn.SyncState()
	}
	if t.nextBtn != nil {
		t.nextBtn.SyncState()
	}
	if t.closeBtn != nil {
		t.closeBtn.SyncState()
	}
	if t.Open && t.Portal != nil {
		if t.layer != nil {
			t.layer.MarkNeedsLayout()
		}
		t.Portal.SetOpen(true)
	}
}

// AttachTicker is a no-op for Tour P0 (no continuous animation).
// Present so gallery trackTicker stays uniform; loading on steps is N/A.
func (t *Tour) AttachTicker(_ *core.Tree) {}

// Resolved metrics for tests (L2).
func (t *Tour) ResolvedFontSize() float64  { t.resolveMetrics(); return t.fontSize }
func (t *Tour) ResolvedTitleFont() float64 { t.resolveMetrics(); return t.titleFont }
func (t *Tour) ResolvedPadding() float64   { t.resolveMetrics(); return t.padding }
func (t *Tour) ResolvedRadius() float64    { t.resolveMetrics(); return t.radius }
func (t *Tour) ResolvedGapOffset() (x, y float64) {
	t.resolveMetrics()
	return t.gapOffX, t.gapOffY
}
func (t *Tour) ResolvedGapRadius() float64 { t.resolveMetrics(); return t.gapRadius }
func (t *Tour) ResolvedIndicator() float64 { t.resolveMetrics(); return t.indicator }

// ── internals ────────────────────────────────────────────────────────

func (t *Tour) theme() *core.Theme {
	var n core.Node
	if t.Portal != nil {
		n = t.Portal
	}
	return themeOf(t.Theme, n)
}

func (t *Tour) resolveMetrics() {
	if t == nil {
		return
	}
	th := t.theme()
	t.fontSize = th.SizeOr(core.TokenFontSize, DefaultTourFontSize)
	t.titleFont = DefaultTourTitleFont
	if t.fontSize > 0 {
		// title slightly larger; keep ≥ body
		if t.titleFont < t.fontSize {
			t.titleFont = t.fontSize
		}
	}
	t.padding = th.SizeOr(core.TokenPadding, DefaultTourPadding)
	if t.padding <= 0 {
		t.padding = DefaultTourPadding
	}
	t.innerGap = th.SizeOr(core.TokenMarginXS, DefaultTourInnerGap)
	if t.innerGap <= 0 {
		t.innerGap = DefaultTourInnerGap
	}
	t.radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultTourRadius)
	if t.radius <= 0 {
		t.radius = DefaultTourRadius
	}
	t.maxWidth = DefaultTourMaxWidth
	t.indicator = DefaultTourIndicator
	// gap
	if t.Gap.xySet {
		t.gapOffX = t.Gap.OffsetX
		t.gapOffY = t.Gap.OffsetY
	} else if t.Gap.offsetSet {
		t.gapOffX = t.Gap.Offset
		t.gapOffY = t.Gap.Offset
	} else {
		t.gapOffX = DefaultTourGapOffset
		t.gapOffY = DefaultTourGapOffset
	}
	if t.Gap.radiusSet {
		t.gapRadius = t.Gap.Radius
	} else {
		t.gapRadius = DefaultTourGapRadius
	}
}

func (t *Tour) ensureShell() {
	if t.Portal != nil && t.layer != nil && t.Scope != nil {
		return
	}
	t.layer = &tourLayer{tour: t}
	t.layer.Init(t.layer)
	t.layer.Hit = core.HitDefer
	t.layer.Role = "dialog"
	if t.AriaLabel != "" {
		t.layer.Label = t.AriaLabel
	} else {
		t.layer.Label = "Tour"
	}
	t.Scope = primitive.NewFocusScope(t.layer)
	t.trap.wire(t.Scope, t.Open && t.Keyboard, t.onEscape)
	t.Portal = primitive.NewOverlayPortal(t.Scope)
	t.Portal.ID = "" // unique auto-id
	t.Portal.ZOrder = OverlayZTour
	if t.Open {
		t.Portal.SetOpen(true)
	}
}

func (t *Tour) invalidateStep() {
	t.ensureShell()
	if t.layer != nil {
		t.layer.MarkNeedsLayout()
		t.layer.MarkNeedsPaint()
	}
	if t.Open && t.Portal != nil {
		t.Portal.SetOpen(true)
		if tr := t.Portal.Tree(); tr != nil {
			tr.MarkDirty()
		}
	}
}

func (t *Tour) onEscape() {
	if t == nil || !t.Open || !t.Keyboard {
		return
	}
	t.SetOpen(false)
}

func (t *Tour) clampCurrent() {
	if t.Current < 0 {
		t.Current = 0
	}
	if len(t.Steps) > 0 && t.Current >= len(t.Steps) {
		t.Current = len(t.Steps) - 1
	}
	if len(t.Steps) == 0 {
		t.Current = 0
	}
	t.Index = t.Current
}

func (t *Tour) setCurrentInternal(i int, fireChange bool) {
	if i < 0 {
		i = 0
	}
	if len(t.Steps) > 0 && i >= len(t.Steps) {
		i = len(t.Steps) - 1
	}
	if t.Current == i {
		t.Index = i
		return
	}
	t.Current = i
	t.Index = i
	if fireChange && t.OnChange != nil {
		t.OnChange(t.Current)
	}
	t.invalidateStep()
}

func (t *Tour) finish() {
	if t.OnFinish != nil {
		t.OnFinish()
	}
	// close without double-firing if already closed
	if t.Open {
		t.SetOpen(false)
	}
}

func (t *Tour) stepAt(i int) (TourStep, bool) {
	if t == nil || i < 0 || i >= len(t.Steps) {
		return TourStep{}, false
	}
	return t.Steps[i], true
}

func (t *Tour) resolvedType(step TourStep) TourType {
	if step.typeSet {
		return step.Type
	}
	return t.Type
}

func (t *Tour) resolvedPlacement(step TourStep) TourPlacement {
	if step.placeSet {
		return step.Placement
	}
	return t.Placement
}

func (t *Tour) resolvedMask(step TourStep) (on bool, color render.RGBA) {
	on = t.Mask
	color = t.MaskColor
	if step.maskSet {
		on = step.Mask
		if step.MaskColor.A > 0 {
			color = step.MaskColor
		}
	}
	return on, color
}

func (t *Tour) resolvedArrow(step TourStep) bool {
	if step.arrowSet {
		return step.Arrow
	}
	return t.Arrow
}

func (t *Tour) localePrev() string {
	if t.LocalePrev != "" {
		return t.LocalePrev
	}
	return DefaultTourLocalePrev
}
func (t *Tour) localeNext() string {
	if t.LocaleNext != "" {
		return t.LocaleNext
	}
	return DefaultTourLocaleNext
}
func (t *Tour) localeFinish() string {
	if t.LocaleFinish != "" {
		return t.LocaleFinish
	}
	return DefaultTourLocaleFinish
}

// ── layer layout / paint ─────────────────────────────────────────────

func (l *tourLayer) TypeID() string { return TypeTour }

func (l *tourLayer) Layout(c core.Constraints) core.Size {
	t := l.tour
	if t == nil {
		out := c.Tighten(core.Size{})
		l.SetSize(out)
		return out
	}
	t.resolveMetrics()
	th := t.theme()

	var portal *primitive.OverlayPortal
	var vp core.Size
	portal = t.Portal
	vp = t.Viewport
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	l.ClearChildren()
	t.panel = nil
	t.maskNode = nil
	t.holeNode = nil
	t.titleNode = nil
	t.bodyNode = nil
	t.infoNode = nil
	t.prevBtn = nil
	t.nextBtn = nil
	t.closeBtn = nil
	t.footerNode = nil
	t.indicatorsN = nil

	step, hasStep := t.stepAt(t.Current)
	maskOn, maskColor := t.resolvedMask(step)
	tp := t.resolvedType(step)
	place := t.resolvedPlacement(step)
	showArrow := t.resolvedArrow(step)

	// ── mask ──
	if maskOn {
		mask := primitive.NewMask()
		mask.Width, mask.Height = vw, vh
		if maskColor.A > 0 {
			mask.Color = maskColor
		} else if st := t.Styles.Mask; st.hasBG() {
			mask.Color = st.Background
		} else {
			// leave default 0.45 so primitive.Mask swaps to TokenColorBgMask
			mask.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
		}
		// Mask click closes (antd default for modal tour).
		mask.OnDismiss = func() { t.SetOpen(false) }
		_ = mask.Layout(core.Tight(vw, vh))
		mask.SetOffset(core.Point{})
		l.AddChild(mask)
		t.maskNode = mask
	}

	// ── highlight hole ──
	target := core.Rect{}
	if hasStep {
		target = step.Target
	}
	holeRect := expandGap(target, t.gapOffX, t.gapOffY)
	if !holeRect.Empty() {
		hole := primitive.NewDecorated()
		hole.Width = holeRect.Width()
		hole.Height = holeRect.Height()
		hole.BorderWidth = 2
		hole.BorderColor = th.Color(core.TokenColorPrimary)
		hole.Radius = t.gapRadius
		hole.Background = render.RGBA{}
		if t.DisabledInteraction {
			hole.Hit = core.HitTarget // absorb clicks
		} else {
			hole.Hit = core.HitDefer
		}
		_ = hole.Layout(core.Tight(hole.Width, hole.Height))
		hole.SetOffset(core.Point{X: holeRect.Min.X, Y: holeRect.Min.Y})
		l.AddChild(hole)
		t.holeNode = hole
	}

	// ── panel content ──
	titleStr := ""
	descStr := ""
	if hasStep {
		titleStr = step.Title
		descStr = step.descriptionText()
	} else {
		titleStr = "Done"
	}

	// a11y label from title when present
	if t.AriaLabel == "" && titleStr != "" {
		l.Label = titleStr
	}

	// colors by type
	var panelBG, titleCol, bodyCol render.RGBA
	if tp == TourTypePrimary {
		panelBG = th.Color(core.TokenColorPrimary)
		titleCol = th.Color(core.TokenColorTextInverse)
		if titleCol.A == 0 {
			titleCol = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		bodyCol = titleCol
		bodyCol.A = 0.85
	} else {
		// antd colorBgElevated ≈ colorBgContainer in default algorithm
		panelBG = th.Color(core.TokenColorBgContainer)
		titleCol = th.Color(core.TokenColorText)
		bodyCol = th.Color(core.TokenColorTextSecondary)
	}
	if st := t.Styles.Section; st.hasBG() {
		panelBG = st.Background
	}
	if st := t.Styles.Title; st.Text.A > 0 {
		titleCol = st.Text
	}
	if st := t.Styles.Description; st.Text.A > 0 {
		bodyCol = st.Text
	}
	if t.Style.hasBG() {
		panelBG = t.Style.Background
	}

	// header row: title + close
	var titleNode core.Node
	if hasStep && step.TitleNode != nil {
		titleNode = step.TitleNode
	} else {
		tx := primitive.NewText(titleStr)
		tx.FontSize = t.titleFont
		tx.Face = t.Face
		tx.Color = titleCol
		t.titleNode = tx
		titleNode = tx
	}

	var header core.Node
	if t.CloseIcon {
		close := NewButton("×")
		close.SetType(ButtonText)
		close.SetFace(t.Face)
		close.SetAriaLabel("Close")
		if tp == TourTypePrimary {
			close.SetStyle(Style{Text: titleCol})
		}
		close.SetOnClick(func() { t.SetOpen(false) })
		t.closeBtn = close
		row := primitive.Row(titleNode, primitive.Spacer(), close.Node())
		row.Gap = t.innerGap
		row.CrossAlign = core.CrossCenter
		header = row
	} else {
		header = titleNode
	}

	// description
	var bodyNode core.Node
	if hasStep && step.DescriptionNode != nil {
		bodyNode = step.DescriptionNode
	} else if descStr != "" {
		bx := primitive.NewText(descStr)
		bx.FontSize = t.fontSize
		bx.Face = t.Face
		bx.Color = bodyCol
		t.bodyNode = bx
		bodyNode = bx
	}

	// cover
	var coverNode core.Node
	if hasStep && step.Cover != nil {
		coverNode = step.Cover
	}

	// indicators
	total := len(t.Steps)
	cur := t.Current
	var indicators core.Node
	if t.IndicatorsRender != nil {
		indicators = t.IndicatorsRender(cur, total)
	} else {
		indicators = t.buildDefaultIndicators(th, tp, cur, total)
	}
	t.indicatorsN = indicators
	// also keep text form for simple tests
	tot := total
	if tot < 1 {
		tot = 1
	}
	info := primitive.NewText(fmt.Sprintf("%d / %d", cur+1, tot))
	info.FontSize = t.fontSize - 2
	if info.FontSize < 10 {
		info.FontSize = 10
	}
	info.Face = t.Face
	info.Color = bodyCol
	t.infoNode = info

	// actions: Prev / Next|Finish
	prevLabel := t.localePrev()
	nextLabel := t.localeNext()
	isLast := total == 0 || cur+1 >= total
	if isLast {
		nextLabel = t.localeFinish()
	}
	var prevProps, nextProps *TourButtonProps
	if hasStep {
		prevProps = step.PrevButtonProps
		nextProps = step.NextButtonProps
	}
	if prevProps != nil && prevProps.Children != "" {
		prevLabel = prevProps.Children
	}
	if nextProps != nil && nextProps.Children != "" {
		nextLabel = nextProps.Children
	}

	prev := NewButton(prevLabel)
	prev.SetFace(t.Face)
	if tp == TourTypePrimary {
		prev.SetType(ButtonDefault)
		// primary tour: prev is translucent on primary
		prev.SetStyle(Style{
			Background: render.RGBA{R: 1, G: 1, B: 1, A: 0.15},
			Text:       titleCol,
			Border:     render.RGBA{},
		})
	}
	if cur == 0 {
		prev.SetDisabled(true)
	}
	if prevProps != nil {
		if prevProps.Disabled {
			prev.SetDisabled(true)
		}
		if prevProps.Style.hasBG() || prevProps.Style.Text.A > 0 {
			prev.SetStyle(prevProps.Style)
		}
	}
	prev.SetOnClick(func() {
		if prevProps != nil && prevProps.OnClick != nil {
			prevProps.OnClick()
		}
		t.Prev()
	})
	t.prevBtn = prev

	next := NewButton(nextLabel)
	next.SetFace(t.Face)
	if tp == TourTypePrimary {
		next.SetType(ButtonDefault)
		next.SetStyle(Style{
			Background: render.RGBA{R: 1, G: 1, B: 1, A: 1},
			Text:       th.Color(core.TokenColorPrimary),
		})
	} else {
		next.SetType(ButtonPrimary)
	}
	if nextProps != nil {
		if nextProps.Disabled {
			next.SetDisabled(true)
		}
		if nextProps.Style.hasBG() || nextProps.Style.Text.A > 0 || nextProps.Style.Border.A > 0 {
			next.SetStyle(nextProps.Style)
		}
	}
	next.SetOnClick(func() {
		if nextProps != nil && nextProps.OnClick != nil {
			nextProps.OnClick()
		}
		t.Next()
	})
	t.nextBtn = next

	actions := primitive.Row(prev.Node(), next.Node())
	actions.Gap = t.innerGap
	actions.CrossAlign = core.CrossCenter
	var actionsNode core.Node = actions
	if t.ActionsRender != nil {
		actionsNode = t.ActionsRender(actions, cur, total)
	}

	// footer: indicators left, actions right
	var indNode core.Node = indicators
	if indNode == nil {
		indNode = info
	}
	footer := primitive.Row(indNode, primitive.Spacer(), actionsNode)
	footer.Gap = t.innerGap
	footer.CrossAlign = core.CrossCenter
	t.footerNode = footer

	// column stack
	kids := make([]core.Node, 0, 6)
	if coverNode != nil {
		kids = append(kids, coverNode)
	}
	if header != nil {
		kids = append(kids, header)
	}
	if bodyNode != nil {
		kids = append(kids, bodyNode)
	}
	// keep info text in tree for simple "n / total" tests when custom indicators used
	if t.IndicatorsRender != nil {
		kids = append(kids, info)
	}
	kids = append(kids, footer)
	col := primitive.Column(kids...)
	col.Gap = t.innerGap
	col.CrossAlign = core.CrossStart

	// P0: arrow is acknowledged via resolvedArrow (geometry P1).
	_ = showArrow

	panel := primitive.NewDecorated(col)
	panel.SkinType = TypeTour
	panel.Padding = primitive.All(t.padding)
	panel.Radius = t.radius
	if st := t.Styles.Section; st.Radius > 0 || st.ForceRadius {
		panel.Radius = st.Radius
	}
	panel.Background = panelBG
	panel.MinWidth = DefaultTourMinWidth
	if t.Styles.Section.Border.A > 0 {
		panel.BorderWidth = 1
		panel.BorderColor = t.Styles.Section.Border
	}
	// layout panel with max width constraint
	maxW := t.maxWidth
	if maxW > vw-16 {
		maxW = vw - 16
	}
	if maxW < 160 {
		maxW = 160
	}
	_ = panel.Layout(core.Loose(maxW, vh))
	// placement
	px, py := placePanel(place, holeRect, panel.Size(), vw, vh)
	panel.SetOffset(core.Point{X: px, Y: py})
	l.AddChild(panel)
	t.panel = panel

	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (t *Tour) buildDefaultIndicators(th *core.Theme, tp TourType, cur, total int) core.Node {
	if total <= 0 {
		return nil
	}
	active := th.Color(core.TokenColorPrimary)
	// antd colorFill ≈ fillSecondary / quaternary text
	idle := th.Color(core.TokenColorFillSecondary)
	if tp == TourTypePrimary {
		active = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		idle = render.RGBA{R: 1, G: 1, B: 1, A: 0.35}
	}
	if idle.A == 0 {
		idle = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	dots := make([]core.Node, 0, total)
	sz := t.indicator
	if sz <= 0 {
		sz = DefaultTourIndicator
	}
	for i := 0; i < total; i++ {
		d := primitive.NewDecorated()
		d.Width = sz
		d.Height = sz
		d.Radius = sz / 2
		if i == cur {
			d.Background = active
		} else {
			d.Background = idle
		}
		d.Hit = core.HitDefer
		_ = d.Layout(core.Tight(sz, sz))
		dots = append(dots, d)
	}
	row := primitive.Row(dots...)
	row.Gap = t.innerGap
	row.CrossAlign = core.CrossCenter
	return row
}

func (l *tourLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *tourLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

// ── geometry helpers ─────────────────────────────────────────────────

func expandGap(target core.Rect, ox, oy float64) core.Rect {
	if target.Empty() {
		return target
	}
	return core.NewRect(
		target.Min.X-ox,
		target.Min.Y-oy,
		target.Width()+ox*2,
		target.Height()+oy*2,
	)
}

func placePanel(place TourPlacement, target core.Rect, panel core.Size, vw, vh float64) (x, y float64) {
	const margin = 8.0
	const gap = 12.0
	// center when no target or explicit center
	if target.Empty() || place == TourCenter {
		x = (vw - panel.Width) / 2
		y = (vh - panel.Height) / 2
		return clampPanel(x, y, panel, vw, vh, margin)
	}
	cx := (target.Min.X + target.Max.X) / 2
	cy := (target.Min.Y + target.Max.Y) / 2
	switch place {
	case TourBottom, TourBottomLeft, TourBottomRight:
		y = target.Max.Y + gap
		switch place {
		case TourBottomLeft:
			x = target.Min.X
		case TourBottomRight:
			x = target.Max.X - panel.Width
		default:
			x = cx - panel.Width/2
		}
	case TourTop, TourTopLeft, TourTopRight:
		y = target.Min.Y - panel.Height - gap
		switch place {
		case TourTopLeft:
			x = target.Min.X
		case TourTopRight:
			x = target.Max.X - panel.Width
		default:
			x = cx - panel.Width/2
		}
	case TourLeft, TourLeftTop, TourLeftBottom:
		x = target.Min.X - panel.Width - gap
		switch place {
		case TourLeftTop:
			y = target.Min.Y
		case TourLeftBottom:
			y = target.Max.Y - panel.Height
		default:
			y = cy - panel.Height/2
		}
	case TourRight, TourRightTop, TourRightBottom:
		x = target.Max.X + gap
		switch place {
		case TourRightTop:
			y = target.Min.Y
		case TourRightBottom:
			y = target.Max.Y - panel.Height
		default:
			y = cy - panel.Height/2
		}
	default:
		x = cx - panel.Width/2
		y = target.Max.Y + gap
	}
	return clampPanel(x, y, panel, vw, vh, margin)
}

func clampPanel(x, y float64, panel core.Size, vw, vh, margin float64) (float64, float64) {
	if x < margin {
		x = margin
	}
	if y < margin {
		y = margin
	}
	if x+panel.Width > vw-margin {
		x = math.Max(margin, vw-panel.Width-margin)
	}
	if y+panel.Height > vh-margin {
		y = math.Max(margin, vh-panel.Height-margin)
	}
	return x, y
}
