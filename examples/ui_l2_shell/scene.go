// Demo scene for examples/ui_l2_shell only.
//
// This is NOT part of the ui library. It wires public L2 packages
// (gestures / focus / overlay / theme / rendering) into a fixed layout so the
// binary can smoke-test them. Architecture work belongs under ui/; layout and
// clamp quirks here are example-only.
package main

import (
	"fmt"

	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Metrics are demo counters (JSON at exit).
type Metrics struct {
	Taps         int    `json:"taps"`
	PanStarts    int    `json:"pan_starts"`
	PanUpdates   int    `json:"pan_updates"`
	OverlayCount int    `json:"overlay_count"`
	FocusLabel   string `json:"focus_id"`
	ArenaDepth   int    `json:"gesture_arena_depth"`
	FocusChanges int    `json:"focus_changes"`
}

// Demo layout constants (logical px). Blue block is a drag handle, not a dial.
const (
	panBoxX, panBoxY = 200.0, 24.0
	panBoxW, panBoxH = 200.0, 110.0
	knobW, knobH     = 44.0, 44.0
)

// Scene is the fixed-layout demo graph for this example.
type Scene struct {
	Width, Height float64
	Root          *rendering.AbsoluteBox

	Focus   *focus.FocusManager
	Overlay *overlay.State
	Theme   *theme.Provider

	TapBox    *rendering.RenderColorBox
	PanBox    *rendering.RenderColorBox
	FocusBoxA *rendering.RenderColorBox
	FocusBoxB *rendering.RenderColorBox
	OpenBtn   *rendering.RenderColorBox
	panInner  *rendering.RenderColorBox

	focusNodeA *focus.FocusNode
	focusNodeB *focus.FocusNode
	openNode   *focus.FocusNode

	disp *gestures.Dispatcher
	tap  *gestures.TapGestureRecognizer
	pan  *gestures.PanGestureRecognizer

	panDX, panDY       float64
	panBaseX, panBaseY float64

	overlayOpen bool
	Metrics     Metrics

	OnDirty func()
}

// NewScene builds the demo scene of the given logical size.
func NewScene(w, h float64) *Scene {
	if w < 320 {
		w = 480
	}
	if h < 240 {
		h = 360
	}
	s := &Scene{
		Width:   w,
		Height:  h,
		Focus:   focus.NewManager(),
		Overlay: overlay.New(),
		Theme:   theme.NewProvider(theme.DefaultTokens()),
		disp:    gestures.NewDispatcher(),
	}
	tok := s.Theme.Current()

	s.Root = rendering.NewAbsoluteBox(w, h)
	s.Root.Background = &rendering.Color{
		R: tok.Surface.R, G: tok.Surface.G, B: tok.Surface.B, A: 1,
	}

	s.TapBox = rendering.NewRenderColorBox(140, 80, 0.15, 0.65, 0.40, 1)
	s.Root.Place(s.TapBox, 24, 24)

	s.PanBox = rendering.NewRenderColorBox(panBoxW, panBoxH, 0.22, 0.28, 0.42, 1)
	s.Root.Place(s.PanBox, panBoxX, panBoxY)

	s.panBaseX = panBoxX + (panBoxW-knobW)/2
	s.panBaseY = panBoxY + (panBoxH-knobH)/2
	s.panInner = rendering.NewRenderColorBox(knobW, knobH, tok.Primary.R, tok.Primary.G, tok.Primary.B, 1)
	s.Root.Place(s.panInner, s.panBaseX, s.panBaseY)
	s.syncPanInner()

	s.FocusBoxA = rendering.NewRenderColorBox(110, 52, 0.40, 0.40, 0.48, 1)
	s.FocusBoxA.SetRepaintBoundary(true)
	s.Root.Place(s.FocusBoxA, 24, 160)

	s.FocusBoxB = rendering.NewRenderColorBox(110, 52, 0.40, 0.40, 0.48, 1)
	s.FocusBoxB.SetRepaintBoundary(true)
	s.Root.Place(s.FocusBoxB, 150, 160)

	s.OpenBtn = rendering.NewRenderColorBox(140, 44, tok.Primary.R, tok.Primary.G, tok.Primary.B, 1)
	s.Root.Place(s.OpenBtn, 24, 240)

	s.wireFocus()
	s.wireGestures()
	return s
}

func (s *Scene) dirty() {
	if s != nil && s.OnDirty != nil {
		s.OnDirty()
	}
}

func (s *Scene) syncPanInner() {
	if s == nil || s.panInner == nil {
		return
	}
	s.clampPanDelta()
	s.Root.Place(s.panInner, s.panBaseX+s.panDX, s.panBaseY+s.panDY)
	s.panInner.MarkNeedsPaint()
}

// clampPanDelta keeps the handle fully inside the slate pan box.
func (s *Scene) clampPanDelta() {
	if s == nil {
		return
	}
	minDX := panBoxX - s.panBaseX
	maxDX := panBoxX + panBoxW - knobW - s.panBaseX
	minDY := panBoxY - s.panBaseY
	maxDY := panBoxY + panBoxH - knobH - s.panBaseY
	if s.panDX < minDX {
		s.panDX = minDX
	}
	if s.panDX > maxDX {
		s.panDX = maxDX
	}
	if s.panDY < minDY {
		s.panDY = minDY
	}
	if s.panDY > maxDY {
		s.panDY = maxDY
	}
}

// PanKnobOffset returns the handle top-left in root coordinates (tests).
func (s *Scene) PanKnobOffset() (x, y float64) {
	if s == nil || s.panInner == nil {
		return 0, 0
	}
	o := s.panInner.Offset()
	return o.X, o.Y
}

func (s *Scene) wireFocus() {
	s.focusNodeA = focus.NewFocusNode("focus-a")
	s.focusNodeB = focus.NewFocusNode("focus-b")
	s.openNode = focus.NewFocusNode("open-overlay")
	s.focusNodeA.OnFocusChange = func(on bool) {
		s.paintFocus(s.FocusBoxA, on)
		s.Metrics.FocusLabel = s.focusLabel()
		s.Metrics.FocusChanges = s.Focus.FocusChanges()
		s.dirty()
	}
	s.focusNodeB.OnFocusChange = func(on bool) {
		s.paintFocus(s.FocusBoxB, on)
		s.Metrics.FocusLabel = s.focusLabel()
		s.Metrics.FocusChanges = s.Focus.FocusChanges()
		s.dirty()
	}
	s.openNode.OnActivate = func() { s.toggleOverlay() }
	s.Focus.Register(s.focusNodeA)
	s.Focus.Register(s.focusNodeB)
	s.Focus.Register(s.openNode)
}

func (s *Scene) paintFocus(box *rendering.RenderColorBox, on bool) {
	if box == nil {
		return
	}
	if on {
		box.R, box.G, box.B = 0.95, 0.75, 0.20
	} else {
		box.R, box.G, box.B = 0.40, 0.40, 0.48
	}
	box.MarkNeedsPaint()
}

func (s *Scene) focusLabel() string {
	if s.Focus.Primary() == nil {
		return ""
	}
	return s.Focus.Primary().DebugLabel
}

func (s *Scene) wireGestures() {
	s.disp.OnScroll = func(sd gestures.ScrollDelta) {}
}

// Layout sizes the absolute root and overlay band.
func (s *Scene) Layout() {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.FixedWidth, s.Root.FixedHeight = s.Width, s.Height
	s.Root.Layout(rendering.Constraints{
		MinWidth: s.Width, MaxWidth: s.Width,
		MinHeight: s.Height, MaxHeight: s.Height,
	})
	if s.Overlay != nil {
		s.Overlay.Layout(s.Width, s.Height)
	}
	s.Metrics.OverlayCount = s.Overlay.Len()
	s.Metrics.ArenaDepth = s.disp.Manager.ActiveCount()
	s.Metrics.FocusLabel = s.focusLabel()
}

// HandleEvent routes one platform event through overlay → gestures/focus.
func (s *Scene) HandleEvent(ev platform.Event) {
	if s == nil {
		return
	}
	switch ev.Type {
	case platform.EventKey:
		ke := focus.KeyEvent{KeyCode: ev.KeyCode, Rune: ev.Rune, Pressed: ev.Pressed}
		if ke.KeyCode == 9 {
			ke.KeyCode = focus.KeyTab
		}
		if ke.KeyCode == 13 {
			ke.KeyCode = focus.KeyEnter
		}
		if s.Focus.HandleKey(ke) {
			s.Metrics.FocusLabel = s.focusLabel()
			s.Metrics.FocusChanges = s.Focus.FocusChanges()
			s.dirty()
		}
		if ev.Pressed && (ev.Rune == 'o' || ev.Rune == 'O') {
			s.toggleOverlay()
		}
	case platform.EventPointer:
		s.handlePointer(ev)
	}
	s.Metrics.OverlayCount = s.Overlay.Len()
	s.Metrics.ArenaDepth = s.disp.Manager.ActiveCount()
}

func (s *Scene) handlePointer(ev platform.Event) {
	p := rendering.Point{X: ev.X, Y: ev.Y}
	if s.Overlay != nil && s.Overlay.Len() > 0 {
		hr := s.Overlay.HitTest(p)
		if hr.Consumed {
			if ev.Pointer == platform.PointerDown {
				s.CloseOverlay()
			}
			return
		}
	}

	ge := gestures.FromPlatform(ev)
	switch ge.Kind {
	case platform.PointerDown:
		if containsBox(s.FocusBoxA, p) {
			s.Focus.FocusFromHit(s.focusNodeA)
		} else if containsBox(s.FocusBoxB, p) {
			s.Focus.FocusFromHit(s.focusNodeB)
		} else if containsBox(s.OpenBtn, p) {
			s.Focus.FocusFromHit(s.openNode)
			s.toggleOverlay()
			return
		}
		path := s.gesturePath(p)
		s.disp.HandleDown(ge, path)
	case platform.PointerMove, platform.PointerUp, platform.PointerScroll:
		s.disp.HandleEvent(ge)
	}
	s.Metrics.FocusLabel = s.focusLabel()
	s.Metrics.FocusChanges = s.Focus.FocusChanges()
}

func (s *Scene) gesturePath(p rendering.Point) []gestures.PointerTarget {
	var path []gestures.PointerTarget
	if containsBox(s.PanBox, p) || containsBox(s.panInner, p) {
		s.pan = gestures.NewPan()
		s.pan.TouchSlop = 4
		s.wirePanCallbacks()
		path = append(path, &gestures.RecognizerTarget{Rec: s.pan})
	}
	if containsBox(s.TapBox, p) {
		s.tap = gestures.NewTap()
		s.wireTapCallback()
		path = append(path, &gestures.RecognizerTarget{Rec: s.tap})
	}
	return path
}

func (s *Scene) wireTapCallback() {
	s.tap.OnTap = func(e gestures.PointerEvent) {
		s.Metrics.Taps++
		s.TapBox.R, s.TapBox.G, s.TapBox.B = 0.45, 0.95, 0.55
		s.TapBox.MarkNeedsPaint()
		s.Root.MarkNeedsPaint()
		s.dirty()
	}
}

func (s *Scene) wirePanCallbacks() {
	s.pan.OnPanStart = func(e gestures.PointerEvent) {
		s.Metrics.PanStarts++
		s.dirty()
	}
	s.pan.OnPanUpdate = func(e gestures.PointerEvent, dx, dy float64) {
		s.Metrics.PanUpdates++
		s.panDX += dx
		s.panDY += dy
		s.syncPanInner()
		s.Root.MarkNeedsPaint()
		s.dirty()
	}
}

func (s *Scene) toggleOverlay() {
	if s.overlayOpen || s.Overlay.Len() > 0 {
		s.CloseOverlay()
		return
	}
	dim := rendering.NewRenderColorBox(s.Width, s.Height, 0, 0, 0, 0.45)
	barrier := overlay.NewBarrierEntry(0, 0, s.Width, s.Height, dim)
	card := rendering.NewRenderColorBox(240, 130, 0.95, 0.95, 0.97, 1)
	card.SetRepaintBoundary(true)
	cx := (s.Width - 240) / 2
	cy := (s.Height - 130) / 2
	s.Overlay.Insert(barrier)
	s.Overlay.Insert(overlay.NewEntry(card, cx, cy, 240, 130))
	s.overlayOpen = true
	s.Metrics.OverlayCount = s.Overlay.Len()
	s.Root.MarkNeedsPaint()
	s.dirty()
}

// CloseOverlay removes all overlay entries.
func (s *Scene) CloseOverlay() {
	if s == nil || s.Overlay == nil {
		return
	}
	s.Overlay.Clear()
	s.overlayOpen = false
	s.Metrics.OverlayCount = 0
	s.Root.MarkNeedsPaint()
	s.dirty()
}

// SnapshotMetrics returns a copy of metrics.
func (s *Scene) SnapshotMetrics() Metrics {
	if s == nil {
		return Metrics{}
	}
	s.Metrics.OverlayCount = s.Overlay.Len()
	s.Metrics.ArenaDepth = s.disp.Manager.ActiveCount()
	s.Metrics.FocusLabel = s.focusLabel()
	s.Metrics.FocusChanges = s.Focus.FocusChanges()
	return s.Metrics
}

func (s *Scene) String() string {
	m := s.SnapshotMetrics()
	return fmt.Sprintf("taps=%d pans=%d/%d focus=%q overlay=%d arena=%d",
		m.Taps, m.PanStarts, m.PanUpdates, m.FocusLabel, m.OverlayCount, m.ArenaDepth)
}

func containsBox(b *rendering.RenderColorBox, p rendering.Point) bool {
	if b == nil {
		return false
	}
	o := b.Offset()
	sz := b.Size()
	w, h := sz.Width, sz.Height
	if w <= 0 {
		w = b.Width
	}
	if h <= 0 {
		h = b.Height
	}
	return p.X >= o.X && p.Y >= o.Y && p.X < o.X+w && p.Y < o.Y+h
}
