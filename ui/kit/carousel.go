package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Carousel component tokens — prepareComponentToken / genDotsStyle / genArrowsStyle.
// docs/antd/carousel.md §6.2 · components/carousel/style/index.ts
const (
	// DefaultCarouselDotWidth is dotWidth.
	DefaultCarouselDotWidth = 16.0
	// DefaultCarouselDotHeight is dotHeight.
	DefaultCarouselDotHeight = 3.0
	// DefaultCarouselDotGap is marginXXS between indicators.
	DefaultCarouselDotGap = 4.0
	// DefaultCarouselDotOffset is edge inset of the dots bar.
	DefaultCarouselDotOffset = 12.0
	// DefaultCarouselDotActiveWidth is active indicator width (horizontal).
	DefaultCarouselDotActiveWidth = 24.0
	// DefaultCarouselArrowSize is slick arrow hit/chrome size.
	DefaultCarouselArrowSize = 16.0
	// DefaultCarouselArrowOffset is marginXS-style edge inset for arrows.
	DefaultCarouselArrowOffset = 8.0
	// DefaultCarouselAutoplaySpeedMS is autoplaySpeed default (ms).
	DefaultCarouselAutoplaySpeedMS = 3000
	// DefaultCarouselSpeedMS is transition speed default (ms); P0 may switch instantly.
	DefaultCarouselSpeedMS = 500
	// DefaultCarouselFontSize is fontSize.
	DefaultCarouselFontSize = 14.0
	// DefaultCarouselRadius is borderRadius.
	DefaultCarouselRadius = 6.0
	// DefaultCarouselLineWidth is lineWidth.
	DefaultCarouselLineWidth = 1.0
	// DefaultCarouselFocusRingOutset approximates Ant focus-visible outset.
	DefaultCarouselFocusRingOutset = 1.5
	// DefaultCarouselStageHeight matches official basic demo content height.
	DefaultCarouselStageHeight = 160.0
	// DefaultCarouselDragThreshold is the drag distance to flip a slide (px).
	DefaultCarouselDragThreshold = 40.0
)

// CarouselDotPlacement is antd dotPlacement (top|bottom|start|end).
type CarouselDotPlacement int

const (
	// CarouselDotBottom is the default (horizontal, dots under stage).
	CarouselDotBottom CarouselDotPlacement = iota
	// CarouselDotTop places dots above the stage.
	CarouselDotTop
	// CarouselDotStart places dots on the logical start (vertical).
	CarouselDotStart
	// CarouselDotEnd places dots on the logical end (vertical).
	CarouselDotEnd
)

// CarouselEffect is antd effect (scrollx|fade).
type CarouselEffect int

const (
	// CarouselScrollX is the default horizontal/vertical scroll effect (P0 may be instant).
	CarouselScrollX CarouselEffect = iota
	// CarouselFade uses fade switching (P0 may be instant; flag is product-visible).
	CarouselFade
)

// Carousel is Ant Design Carousel (data display — 走马灯).
//
//	Root Stack
//	  ├─ stage (clip + drag host) → active slide Slot
//	  ├─ dots (Positioned)
//	  └─ arrows? (Positioned prev/next)
//
// Product contract: docs/antd/carousel.md §6 (P0 DoD).
type Carousel struct {
	Root *primitive.Stack

	stage     *carouselStage
	slideSlot *primitive.Slot
	dotsHost  *primitive.Flex
	dotsWrap  core.Node // Positioned wrapper
	prevArrow *primitive.Pressable
	nextArrow *primitive.Pressable
	prevWrap  core.Node
	nextWrap  core.Node

	// Slides are the page children (antd children).
	Slides []core.Node
	// Index is the current slide (0-based).
	Index int

	Arrows         bool
	Autoplay       bool
	DotDuration    bool // autoplay: { dotDuration: true }
	AutoplaySpeed  int  // ms; 0 → DefaultCarouselAutoplaySpeedMS
	AdaptiveHeight bool
	DotPlacement   CarouselDotPlacement
	Dots           bool
	Draggable      bool
	Fade           bool // sugar → Effect=Fade
	Effect         CarouselEffect
	Infinite       bool
	Speed          int // ms recorded; P0 switch may be instant
	// Width/Height force stage preferred size when > 0.
	Width  float64
	Height float64

	Disabled  bool
	AriaLabel string

	AfterChange  func(current int)
	BeforeChange func(current, next int)

	Face  text.Face
	Theme *core.Theme
	Style Style

	// autoplayAccum seconds since last advance (or since last GoTo).
	autoplayAccum float64
	// durationProgress 0..1 for active dot fill when DotDuration.
	durationProgress float64

	boundTree *core.Tree
	// hasArrowNodes reports whether rebuild last installed arrow chrome (tests CRS-06).
	hasArrowNodes bool
	// hasDotsNodes reports whether dots chrome is present.
	hasDotsNodes bool
}

// NewCarousel creates a Carousel with antd defaults (dots on, infinite, bottom placement).
func NewCarousel(slides ...core.Node) *Carousel {
	c := &Carousel{
		Slides:       append([]core.Node(nil), slides...),
		Dots:         true,
		Infinite:     true,
		DotPlacement: CarouselDotBottom,
		Effect:       CarouselScrollX,
		Speed:        DefaultCarouselSpeedMS,
	}
	c.rebuild()
	return c
}

// Node returns the mount root (stable across Next/GoTo).

// ensureBuilt materializes the control tree if missing (#9).
func (c *Carousel) ensureBuilt() {
	if c == nil {
		return
	}
	if c.Root == nil {
		c.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (c *Carousel) structureChange() {
	if c == nil {
		return
	}
	c.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (c *Carousel) chromeChange() {
	if c == nil {
		return
	}
	c.ensureBuilt()
	c.rebuild()
}

func (c *Carousel) Node() core.Node {
	if c == nil {
		return nil
	}
	c.ensureBuilt()
	return c.Root
}

// ChromeNode returns the same root shell.
func (c *Carousel) ChromeNode() core.Node { return c.Node() }

// HasArrows reports whether arrow chrome is currently built (Arrows=true and slides>0).
func (c *Carousel) HasArrows() bool {
	if c == nil {
		return false
	}
	return c.hasArrowNodes
}

// HasDots reports whether dots chrome is currently built.
func (c *Carousel) HasDots() bool {
	if c == nil {
		return false
	}
	return c.hasDotsNodes
}

// DotWidth returns resolved inactive indicator width.
func (c *Carousel) DotWidth() float64 { return DefaultCarouselDotWidth }

// DotHeight returns resolved indicator height.
func (c *Carousel) DotHeight() float64 { return DefaultCarouselDotHeight }

// DotGap returns resolved indicator gap.
func (c *Carousel) DotGap() float64 { return DefaultCarouselDotGap }

// DotOffset returns resolved dots edge inset.
func (c *Carousel) DotOffset() float64 { return DefaultCarouselDotOffset }

// DotActiveWidth returns resolved active indicator width.
func (c *Carousel) DotActiveWidth() float64 { return DefaultCarouselDotActiveWidth }

// ArrowSize returns resolved arrow chrome size.
func (c *Carousel) ArrowSize() float64 { return DefaultCarouselArrowSize }

// ArrowOffset returns resolved arrow edge inset.
func (c *Carousel) ArrowOffset() float64 { return DefaultCarouselArrowOffset }

// AutoplaySpeedMS returns resolved autoplay interval in milliseconds.
func (c *Carousel) AutoplaySpeedMS() int {
	if c == nil || c.AutoplaySpeed <= 0 {
		return DefaultCarouselAutoplaySpeedMS
	}
	return c.AutoplaySpeed
}

// IsVertical is true when start/end placement drives a vertical track (antd).
func (c *Carousel) IsVertical() bool {
	if c == nil {
		return false
	}
	return c.DotPlacement == CarouselDotStart || c.DotPlacement == CarouselDotEnd
}

// IsFade reports fade effect (Fade flag or Effect=Fade).
func (c *Carousel) IsFade() bool {
	if c == nil {
		return false
	}
	return c.Fade || c.Effect == CarouselFade
}

// DurationProgress returns 0..1 active-dot fill (DotDuration path).
func (c *Carousel) DurationProgress() float64 {
	if c == nil {
		return 0
	}
	return c.durationProgress
}

// SetSlides replaces children and clamps index.
func (c *Carousel) SetSlides(slides ...core.Node) {
	if c == nil {
		return
	}
	c.Slides = append([]core.Node(nil), slides...)
	c.clampIndex()
	c.rebuild()
}

// SetIndex jumps to i (antd goTo without animation).
func (c *Carousel) SetIndex(i int) { c.GoTo(i) }

// GoTo switches to slide i (P0 instantaneous). Fires before/after change when index moves.
func (c *Carousel) GoTo(i int) {
	if c == nil {
		return
	}
	n := len(c.Slides)
	if n == 0 {
		c.Index = 0
		c.applySlide()
		return
	}
	next := c.normalizeIndex(i)
	if next == c.Index {
		c.applySlide()
		return
	}
	c.transitionTo(next)
}

// Next advances one slide (wraps when Infinite).
func (c *Carousel) Next() {
	if c == nil || c.Disabled || len(c.Slides) == 0 {
		return
	}
	n := len(c.Slides)
	if c.Index >= n-1 {
		if !c.Infinite {
			return
		}
		c.transitionTo(0)
		return
	}
	c.transitionTo(c.Index + 1)
}

// Prev goes to the previous slide (wraps when Infinite).
func (c *Carousel) Prev() {
	if c == nil || c.Disabled || len(c.Slides) == 0 {
		return
	}
	if c.Index <= 0 {
		if !c.Infinite {
			return
		}
		c.transitionTo(len(c.Slides) - 1)
		return
	}
	c.transitionTo(c.Index - 1)
}

// SetArrows toggles arrow chrome (default false).
func (c *Carousel) SetArrows(v bool) {
	if c == nil {
		return
	}
	c.Arrows = v
	c.rebuild()
}

// SetAutoplay enables Ticker-driven advance.
func (c *Carousel) SetAutoplay(v bool) {
	if c == nil {
		return
	}
	c.Autoplay = v
	c.autoplayAccum = 0
	c.durationProgress = 0
	c.ensureTicker()
	c.rebuild()
}

// SetDotDuration enables active-dot progress fill under autoplay.
func (c *Carousel) SetDotDuration(v bool) {
	if c == nil {
		return
	}
	c.DotDuration = v
	c.durationProgress = 0
	c.ensureTicker()
	c.rebuild()
}

// SetAutoplaySpeed sets interval in ms (0 → default 3000).
func (c *Carousel) SetAutoplaySpeed(ms int) {
	if c == nil {
		return
	}
	c.AutoplaySpeed = ms
	c.autoplayAccum = 0
	c.durationProgress = 0
	c.markPaint()
}

// SetAdaptiveHeight toggles height-follows-slide vs fixed stage height.
func (c *Carousel) SetAdaptiveHeight(v bool) {
	if c == nil {
		return
	}
	c.AdaptiveHeight = v
	c.rebuild()
}

// SetDotPlacement sets indicator placement (and vertical track for start/end).
func (c *Carousel) SetDotPlacement(p CarouselDotPlacement) {
	if c == nil {
		return
	}
	c.DotPlacement = p
	c.rebuild()
}

// SetDots toggles indicator visibility (default true).
func (c *Carousel) SetDots(v bool) {
	if c == nil {
		return
	}
	c.Dots = v
	c.rebuild()
}

// SetDraggable enables drag-to-flip on the stage.
func (c *Carousel) SetDraggable(v bool) {
	if c == nil {
		return
	}
	c.Draggable = v
	if c.stage != nil {
		c.stage.draggable = v && !c.Disabled
	}
}

// SetFade is sugar for effect=fade.
func (c *Carousel) SetFade(v bool) {
	if c == nil {
		return
	}
	c.Fade = v
	if v {
		c.Effect = CarouselFade
	} else if c.Effect == CarouselFade {
		c.Effect = CarouselScrollX
	}
	c.markPaint()
}

// SetEffect sets scrollx|fade.
func (c *Carousel) SetEffect(e CarouselEffect) {
	if c == nil {
		return
	}
	c.Effect = e
	c.Fade = e == CarouselFade
	c.markPaint()
}

// SetInfinite toggles wrap-around (default true).
func (c *Carousel) SetInfinite(v bool) {
	if c == nil {
		return
	}
	c.Infinite = v
	c.rebuild()
}

// SetSpeed records transition ms (P0 may still switch instantly).
func (c *Carousel) SetSpeed(ms int) {
	if c == nil {
		return
	}
	c.Speed = ms
}

// SetWidth forces stage preferred width when > 0.
func (c *Carousel) SetWidth(w float64) {
	if c == nil {
		return
	}
	c.Width = w
	c.rebuild()
}

// SetHeight forces stage preferred height when > 0 (ignored path when AdaptiveHeight).
func (c *Carousel) SetHeight(h float64) {
	if c == nil {
		return
	}
	c.Height = h
	c.rebuild()
}

// SetAfterChange sets the afterChange callback.
func (c *Carousel) SetAfterChange(fn func(current int)) {
	if c == nil {
		return
	}
	c.AfterChange = fn
}

// SetBeforeChange sets the beforeChange callback.
func (c *Carousel) SetBeforeChange(fn func(current, next int)) {
	if c == nil {
		return
	}
	c.BeforeChange = fn
}

// SetDisabled blocks interaction and autoplay.
func (c *Carousel) SetDisabled(v bool) {
	if c == nil {
		return
	}
	c.Disabled = v
	if c.stage != nil {
		c.stage.draggable = c.Draggable && !v
		c.stage.disabled = v
	}
	c.rebuild()
}

// SetTheme applies theme tokens on next rebuild/paint.
func (c *Carousel) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetFace sets text face for any chrome labels (arrows use canvas).
func (c *Carousel) SetFace(f text.Face) {
	if c == nil {
		return
	}
	c.Face = f
}

// SetStyle applies optional style overrides.
func (c *Carousel) SetStyle(st Style) {
	if c == nil {
		return
	}
	c.Style = st
	c.rebuild()
}

// SetAriaLabel sets accessible name on the root region.
func (c *Carousel) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	if c.Root != nil {
		c.Root.Base().Label = s
	}
}

// AttachTicker registers autoplay / dot-duration animation on the tree.
func (c *Carousel) AttachTicker(t *core.Tree) {
	if c == nil || t == nil {
		return
	}
	c.boundTree = t
	c.ensureTicker()
}

// Tick advances autoplay and duration progress. Returns true when still animating.
func (c *Carousel) Tick(dt float64) bool {
	if c == nil {
		return false
	}
	if c.Disabled || !c.Autoplay || len(c.Slides) <= 1 {
		c.durationProgress = 0
		return false
	}
	if dt < 0 {
		dt = 0
	}
	// Respect ReduceMotion for progress fill only; autoplay still steps.
	reduce := false
	if c.boundTree != nil && c.boundTree.Clock() != nil {
		reduce = c.boundTree.Clock().ReduceMotion
	}
	speedSec := float64(c.AutoplaySpeedMS()) / 1000.0
	if speedSec <= 0 {
		speedSec = float64(DefaultCarouselAutoplaySpeedMS) / 1000.0
	}
	c.autoplayAccum += dt
	if c.DotDuration {
		if reduce {
			c.durationProgress = 1
		} else {
			c.durationProgress = math.Min(1, c.autoplayAccum/speedSec)
		}
		c.refreshDotsPaint()
	}
	if c.autoplayAccum >= speedSec {
		c.autoplayAccum = 0
		c.durationProgress = 0
		c.Next()
		return c.Autoplay && !c.Disabled && len(c.Slides) > 1
	}
	return true
}

func (c *Carousel) ensureTicker() {
	if c == nil || c.boundTree == nil {
		return
	}
	if c.Autoplay && !c.Disabled && len(c.Slides) > 1 {
		c.boundTree.AddTicker(c)
	}
}

func (c *Carousel) transitionTo(next int) {
	if c == nil {
		return
	}
	n := len(c.Slides)
	if n == 0 {
		return
	}
	next = c.normalizeIndex(next)
	if next == c.Index {
		return
	}
	cur := c.Index
	if c.BeforeChange != nil {
		c.BeforeChange(cur, next)
	}
	c.Index = next
	c.autoplayAccum = 0
	c.durationProgress = 0
	c.applySlide()
	if c.AfterChange != nil {
		c.AfterChange(c.Index)
	}
}

func (c *Carousel) normalizeIndex(i int) int {
	n := len(c.Slides)
	if n == 0 {
		return 0
	}
	if c.Infinite {
		// Euclidean mod for negative.
		i = i % n
		if i < 0 {
			i += n
		}
		return i
	}
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func (c *Carousel) clampIndex() {
	if c == nil {
		return
	}
	c.Index = c.normalizeIndex(c.Index)
}

func (c *Carousel) theme() *core.Theme {
	if c != nil && c.Theme != nil {
		return c.Theme
	}
	return core.DefaultTheme()
}

func (c *Carousel) stageHeight() float64 {
	if c == nil {
		return DefaultCarouselStageHeight
	}
	if c.Height > 0 {
		return c.Height
	}
	return DefaultCarouselStageHeight
}

func (c *Carousel) markPaint() {
	if c == nil || c.Root == nil {
		return
	}
	c.Root.MarkNeedsPaint()
}

func (c *Carousel) markLayout() {
	if c == nil || c.Root == nil {
		return
	}
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}

func (c *Carousel) applySlide() {
	if c == nil {
		return
	}
	if c.slideSlot == nil || c.Root == nil {
		c.rebuild()
		return
	}
	var child core.Node
	if len(c.Slides) > 0 {
		i := c.normalizeIndex(c.Index)
		c.Index = i
		child = c.Slides[i]
	}
	c.slideSlot.SetChild(child)
	// Refresh dots active chrome without full shell rebuild when possible.
	c.rebuildDotsAndArrows()
	c.markLayout()
}

func (c *Carousel) rebuild() {
	if c == nil {
		return
	}
	c.clampIndex()

	if c.slideSlot == nil {
		c.slideSlot = primitive.NewSlot("carousel-slide", nil)
		c.slideSlot.ExpandFill = true
	}
	var child core.Node
	if len(c.Slides) > 0 {
		child = c.Slides[c.Index]
	}
	c.slideSlot.SetChild(child)

	if c.stage == nil {
		c.stage = newCarouselStage(c)
	}
	c.stage.owner = c
	c.stage.draggable = c.Draggable && !c.Disabled
	c.stage.disabled = c.Disabled
	c.stage.vertical = c.IsVertical()
	c.stage.SetChild(c.slideSlot)
	c.stage.adaptive = c.AdaptiveHeight
	c.stage.fixedH = c.stageHeight()
	c.stage.fixedW = c.Width

	if c.Root == nil {
		c.Root = primitive.NewStack()
		c.Root.Fit = true
	}
	c.Root.ClearChildren()
	c.Root.AddChild(c.stage)

	c.rebuildDotsAndArrows()

	c.Root.Base().Role = "region"
	name := c.AriaLabel
	if name == "" {
		name = "carousel"
	}
	c.Root.Base().Label = name
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
	c.ensureTicker()
}

func (c *Carousel) rebuildDotsAndArrows() {
	if c == nil || c.Root == nil {
		return
	}
	// Drop previous overlays (keep stage as first child).
	kids := c.Root.Children()
	if len(kids) > 1 {
		// Rebuild overlays only: clear all then re-add stage + overlays.
		c.Root.ClearChildren()
		c.Root.AddChild(c.stage)
	}

	c.hasDotsNodes = false
	c.hasArrowNodes = false
	c.dotsHost = nil
	c.dotsWrap = nil
	c.prevArrow = nil
	c.nextArrow = nil
	c.prevWrap = nil
	c.nextWrap = nil

	n := len(c.Slides)
	th := c.theme()
	vert := c.IsVertical()

	if c.Dots && n > 0 {
		var row *primitive.Flex
		if vert {
			row = primitive.Column()
		} else {
			row = primitive.Row()
		}
		row.Gap = c.DotGap()
		row.MainAlign = core.MainCenter
		row.CrossAlign = core.CrossCenter
		bg := th.Color(core.TokenColorBgContainer)
		for i := 0; i < n; i++ {
			idx := i
			active := idx == c.Index
			dot := c.buildDot(idx, active, vert, bg)
			row.AddChild(dot)
		}
		c.dotsHost = row
		align := core.AlignBottomCenter
		switch c.DotPlacement {
		case CarouselDotTop:
			align = core.AlignTopCenter
		case CarouselDotStart:
			align = core.AlignCenterLeft
		case CarouselDotEnd:
			align = core.AlignCenterRight
		default:
			align = core.AlignBottomCenter
		}
		// Inset via padding wrapper so Positioned aligns to outer edge + offset feel.
		pad := primitive.NewDecorated(row)
		pad.Hit = core.HitDefer
		off := c.DotOffset()
		switch c.DotPlacement {
		case CarouselDotTop:
			pad.Padding = primitive.EdgeInsets{Top: off}
		case CarouselDotStart:
			pad.Padding = primitive.EdgeInsets{Left: off}
		case CarouselDotEnd:
			pad.Padding = primitive.EdgeInsets{Right: off}
		default:
			pad.Padding = primitive.EdgeInsets{Bottom: off}
		}
		c.dotsWrap = primitive.Positioned(align, pad)
		c.Root.AddChild(c.dotsWrap)
		c.hasDotsNodes = true
	}

	if c.Arrows && n > 0 {
		c.prevArrow = c.buildArrow(true, vert)
		c.nextArrow = c.buildArrow(false, vert)
		aoff := c.ArrowOffset()
		if vert {
			// prev top, next bottom
			prevPad := primitive.NewDecorated(c.prevArrow)
			prevPad.Hit = core.HitDefer
			prevPad.Padding = primitive.EdgeInsets{Top: aoff}
			c.prevWrap = primitive.Positioned(core.AlignTopCenter, prevPad)
			nextPad := primitive.NewDecorated(c.nextArrow)
			nextPad.Hit = core.HitDefer
			nextPad.Padding = primitive.EdgeInsets{Bottom: aoff}
			c.nextWrap = primitive.Positioned(core.AlignBottomCenter, nextPad)
		} else {
			prevPad := primitive.NewDecorated(c.prevArrow)
			prevPad.Hit = core.HitDefer
			prevPad.Padding = primitive.EdgeInsets{Left: aoff}
			c.prevWrap = primitive.Positioned(core.AlignCenterLeft, prevPad)
			nextPad := primitive.NewDecorated(c.nextArrow)
			nextPad.Hit = core.HitDefer
			nextPad.Padding = primitive.EdgeInsets{Right: aoff}
			c.nextWrap = primitive.Positioned(core.AlignCenterRight, nextPad)
		}
		c.Root.AddChild(c.prevWrap)
		c.Root.AddChild(c.nextWrap)
		c.hasArrowNodes = true
		c.syncArrowEnabled()
	}
}

func (c *Carousel) syncArrowEnabled() {
	if c == nil {
		return
	}
	n := len(c.Slides)
	disablePrev := c.Disabled || n == 0 || (!c.Infinite && c.Index <= 0)
	disableNext := c.Disabled || n == 0 || (!c.Infinite && c.Index >= n-1)
	if c.prevArrow != nil {
		c.prevArrow.State.Disabled = disablePrev
		// Hide disabled arrows (antd slick-disabled opacity 0).
		if disablePrev {
			c.prevArrow.Color = render.RGBA{}
		}
	}
	if c.nextArrow != nil {
		c.nextArrow.State.Disabled = disableNext
		if disableNext {
			c.nextArrow.Color = render.RGBA{}
		}
	}
}

func (c *Carousel) buildDot(idx int, active, vert bool, bg render.RGBA) core.Node {
	w := c.DotWidth()
	h := c.DotHeight()
	if active {
		if vert {
			h = c.DotActiveWidth()
		} else {
			w = c.DotActiveWidth()
		}
	}
	// Base pill.
	base := primitive.NewDecorated()
	base.Width = w
	base.Height = h
	base.Radius = h
	if vert && active {
		base.Radius = w
	}
	op := 0.2
	if active {
		op = 0.75
	}
	if c.Disabled {
		op = 0.15
	}
	col := bg
	col.A = op
	base.Background = col
	base.Hit = core.HitDefer

	// Duration fill overlay on active when DotDuration+Autoplay.
	inner := core.Node(base)
	if active && c.DotDuration && c.Autoplay && !c.Disabled {
		fillW := w
		fillH := h
		prog := c.durationProgress
		if prog < 0 {
			prog = 0
		}
		if prog > 1 {
			prog = 1
		}
		if vert {
			fillH = h * prog
		} else {
			fillW = w * prog
		}
		fill := primitive.NewDecorated()
		fill.Width = math.Max(fillW, 0.01)
		fill.Height = math.Max(fillH, 0.01)
		fill.Radius = base.Radius
		fc := bg
		fc.A = 1
		fill.Background = fc
		fill.Hit = core.HitDefer
		stack := primitive.NewStack(primitive.Positioned(core.AlignTopLeft, base), primitive.Positioned(core.AlignTopLeft, fill))
		stack.Fit = false
		// Force stack size to full active pill.
		box := primitive.NewDecorated(stack)
		box.Width = w
		box.Height = h
		box.Hit = core.HitDefer
		inner = box
	}

	pr := primitive.NewPressable(inner)
	pr.ShowFocusRing = true
	pr.FocusRingOutset = DefaultCarouselFocusRingOutset
	pr.EnableRipple = false
	pr.State.Disabled = c.Disabled
	pr.Base().Label = fmt.Sprintf("Go to slide %d", idx+1)
	pr.Base().Role = "button"
	if !c.Disabled {
		pr.Click = func() {
			if c.Disabled {
				return
			}
			c.GoTo(idx)
		}
	}
	return pr
}

func (c *Carousel) buildArrow(prev, vert bool) *primitive.Pressable {
	size := c.ArrowSize()
	// Chevon via Canvas (white, opacity via pressable colors).
	cv := primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		// Draw a simple chevron with two lines.
		col := render.RGBA{R: 1, G: 1, B: 1, A: 0.85}
		stroke := 2.0
		cx, cy := sz.Width/2, sz.Height/2
		arm := size * 0.28
		var x0, y0, x1, y1, x2, y2 float64
		if vert {
			if prev {
				// up ^
				x0, y0 = cx-arm, cy+arm*0.4
				x1, y1 = cx, cy-arm*0.4
				x2, y2 = cx+arm, cy+arm*0.4
			} else {
				// down v
				x0, y0 = cx-arm, cy-arm*0.4
				x1, y1 = cx, cy+arm*0.4
				x2, y2 = cx+arm, cy-arm*0.4
			}
		} else {
			if prev {
				// <
				x0, y0 = cx+arm*0.4, cy-arm
				x1, y1 = cx-arm*0.4, cy
				x2, y2 = cx+arm*0.4, cy+arm
			} else {
				// >
				x0, y0 = cx-arm*0.4, cy-arm
				x1, y1 = cx+arm*0.4, cy
				x2, y2 = cx-arm*0.4, cy+arm
			}
		}
		pc.StrokeLocalLine(x0, y0, x1, y1, stroke, col)
		pc.StrokeLocalLine(x1, y1, x2, y2, stroke, col)
	})
	box := primitive.NewDecorated(cv)
	box.Width = size
	box.Height = size
	box.Hit = core.HitDefer

	pr := primitive.NewPressable(box)
	pr.ShowFocusRing = true
	pr.FocusRingOutset = DefaultCarouselFocusRingOutset
	pr.EnableRipple = false
	pr.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0} // transparent hit
	if prev {
		pr.Base().Label = "prev"
		pr.Click = func() {
			if c == nil || c.Disabled {
				return
			}
			c.Prev()
		}
	} else {
		pr.Base().Label = "next"
		pr.Click = func() {
			if c == nil || c.Disabled {
				return
			}
			c.Next()
		}
	}
	pr.Base().Role = "button"
	return pr
}

func (c *Carousel) refreshDotsPaint() {
	// Cheapest path: rebuild dots chrome to refresh duration fill width.
	if c == nil || c.Root == nil || !c.hasDotsNodes {
		return
	}
	c.rebuildDotsAndArrows()
	c.markPaint()
}

// HandleKey implements keyboard navigation when the stage (or root) is focused.
func (c *Carousel) HandleKey(ev *core.KeyEvent) {
	if c == nil || ev == nil || c.Disabled {
		return
	}
	if ev.Type != core.KeyDown {
		return
	}
	vert := c.IsVertical()
	switch ev.Key {
	case "ArrowLeft", "Left":
		if !vert {
			c.Prev()
			ev.Handled = true
		}
	case "ArrowRight", "Right":
		if !vert {
			c.Next()
			ev.Handled = true
		}
	case "ArrowUp", "Up":
		if vert {
			c.Prev()
			ev.Handled = true
		}
	case "ArrowDown", "Down":
		if vert {
			c.Next()
			ev.Handled = true
		}
	}
}

// ---------------------------------------------------------------------------
// carouselStage — clip host + optional drag + keyboard focus target
// ---------------------------------------------------------------------------

type carouselStage struct {
	core.NodeBase
	owner     *Carousel
	draggable bool
	disabled  bool
	vertical  bool
	adaptive  bool
	fixedH    float64
	fixedW    float64

	dragging bool
	startX   float64
	startY   float64
	moved    bool
}

func newCarouselStage(owner *Carousel) *carouselStage {
	s := &carouselStage{owner: owner}
	s.Init(s)
	s.Hit = core.HitTarget
	s.ClipHit = true
	s.Cursor = core.CursorDefault
	return s
}

func (s *carouselStage) TypeID() string { return "kit.carouselStage" }

func (s *carouselStage) SetChild(child core.Node) {
	s.ClearChildren()
	if child != nil {
		s.AddChild(child)
	}
}

func (s *carouselStage) Layout(c core.Constraints) core.Size {
	if s == nil {
		return core.Size{}
	}
	kids := s.Children()
	var content core.Size
	maxW := c.MaxWidth
	if s.fixedW > 0 && s.fixedW < maxW {
		maxW = s.fixedW
	}
	wantH := s.fixedH
	if s.adaptive {
		// Measure child freely on height.
		childC := core.Constraints{MaxWidth: maxW, MaxHeight: c.MaxHeight}
		if len(kids) > 0 {
			content = kids[0].Layout(childC)
			kids[0].Base().SetOffset(core.Point{})
		}
		wantH = content.Height
		if wantH <= 0 {
			wantH = s.fixedH
		}
	} else {
		childC := core.Constraints{
			MinWidth:  0,
			MaxWidth:  maxW,
			MinHeight: wantH,
			MaxHeight: wantH,
		}
		if maxW < core.Unbounded && maxW > 0 {
			// Prefer filling width when parent gives a bound.
			childC.MinWidth = maxW
		}
		if s.fixedW > 0 {
			childC.MinWidth = s.fixedW
			childC.MaxWidth = s.fixedW
		}
		if len(kids) > 0 {
			content = kids[0].Layout(childC)
			kids[0].Base().SetOffset(core.Point{})
		}
	}
	w := content.Width
	if s.fixedW > 0 {
		w = s.fixedW
	} else if maxW < core.Unbounded && maxW > w {
		w = maxW
	}
	h := wantH
	out := c.Tighten(core.Size{Width: w, Height: h})
	s.SetSize(out)
	return out
}

func (s *carouselStage) Paint(pc *core.PaintContext) {
	if s == nil || pc == nil {
		return
	}
	sz := s.Size()
	pc.PushClipLocal(0, 0, sz.Width, sz.Height)
	s.DefaultPaintChildren(pc)
	pc.Pop()
}

func (s *carouselStage) HitTest(p core.Point) core.Node {
	if s == nil {
		return nil
	}
	sz := s.Size()
	if p.X < 0 || p.Y < 0 || p.X > sz.Width || p.Y > sz.Height {
		return nil
	}
	// Prefer interactive children (dots are siblings on Stack, not here).
	if hit := s.DefaultHitTest(p); hit != nil && hit != s {
		return hit
	}
	return s
}

func (s *carouselStage) HandlePointer(ev *core.PointerEvent) {
	if s == nil || ev == nil || s.disabled {
		return
	}
	if !s.draggable {
		return
	}
	switch ev.Type {
	case core.PointerDown:
		if ev.Button != core.ButtonLeft {
			return
		}
		s.dragging = true
		s.moved = false
		s.startX, s.startY = ev.X, ev.Y
		ev.Handled = true
	case core.PointerMove:
		if !s.dragging {
			return
		}
		dx := ev.X - s.startX
		dy := ev.Y - s.startY
		if math.Abs(dx) > 3 || math.Abs(dy) > 3 {
			s.moved = true
		}
		ev.Handled = true
	case core.PointerUp, core.PointerCancel:
		if !s.dragging {
			return
		}
		s.dragging = false
		if s.moved && s.owner != nil {
			dx := ev.X - s.startX
			dy := ev.Y - s.startY
			th := DefaultCarouselDragThreshold
			if s.vertical {
				if dy <= -th {
					s.owner.Next()
				} else if dy >= th {
					s.owner.Prev()
				}
			} else {
				if dx <= -th {
					s.owner.Next()
				} else if dx >= th {
					s.owner.Prev()
				}
			}
		}
		s.moved = false
		ev.Handled = true
	}
}

func (s *carouselStage) HandleKey(ev *core.KeyEvent) {
	if s == nil || s.owner == nil {
		return
	}
	s.owner.HandleKey(ev)
}
