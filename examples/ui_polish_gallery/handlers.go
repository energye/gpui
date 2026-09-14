// Gallery input wiring: map each live node to its component handler.
//
// Handlers call the real component methods (Button PointerDown/Up/Move,
// Alert ClickClose, Tag Close/Click, CheckableTag Toggle, FloatButton
// Click, Typography Copy/Edit/Expand/Click, Sider trigger, Splitter
// drag/collapse) so the window behaves like the unit tests.
package main

import (
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/kit/alert"
	"github.com/energye/gpui/ui/kit/button"
	floatbutton "github.com/energye/gpui/ui/kit/float-button"
	"github.com/energye/gpui/ui/kit/layout"
	"github.com/energye/gpui/ui/kit/splitter"
	"github.com/energye/gpui/ui/kit/tag"
	"github.com/energye/gpui/ui/kit/typography"
	"github.com/energye/gpui/ui/rendering"
)

// tracked keeps component pointers alive next to their nodes so handlers
// can call methods. Keyed by node.
type tracked struct {
	btn   *button.Button
	al    *alert.Alert
	tg    *tag.Tag
	ctg   *tag.CheckableTag
	fbtn  *floatbutton.FloatButton
	fgrp  *floatbutton.FloatButtonGroup
	typo  *typography.Typography
	sider *layout.Sider
	split *splitter.Splitter
	label string
}

var trackedByNode = map[rendering.RenderObject]*tracked{}

func track(node rendering.RenderObject, t *tracked) {
	if node == nil || t == nil {
		return
	}
	trackedByNode[node] = t
}

// registerHandlers wires node (and its descendants) to component handlers.
// Called by Scene.rebuild for every mounted live node.
func (s *Scene) registerHandlers(page string, node rendering.RenderObject) {
	if node == nil {
		return
	}
	s.registerNode(page, node)
	for _, ch := range node.Children() {
		s.registerNode(page, ch)
	}
}

func (s *Scene) registerNode(page string, node rendering.RenderObject) {
	t, ok := trackedByNode[node]
	if !ok || t == nil {
		return
	}
	switch {
	case t.btn != nil:
		b := t.btn
		s.registerHandler(&CompHandler{
			Label: "button/" + t.label,
			Node:  node,
			Move:  func(lx, ly float64) { b.PointerMove(lx, ly) },
			Down:  func(lx, ly float64) bool { return b.PointerDown(lx, ly) },
			Up:    func(lx, ly float64) bool { return b.PointerUp(lx, ly) },
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					b.KeyPress(keyEvent(key))
					return true
				}
				return false
			},
			Focusable: func() bool { return b.Focusable() },
		})
	case t.al != nil:
		a := t.al
		s.registerHandler(&CompHandler{
			Label: "alert/" + t.label,
			Node:  node,
			Down: func(lx, ly float64) bool {
				if inClose(a, node, lx, ly) {
					return a.ClickClose()
				}
				return false
			},
			Up: func(lx, ly float64) bool { return false },
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					return a.PressCloseKey(key)
				}
				return false
			},
			Focusable: func() bool { return a.CloseFocusable() },
		})
	case t.tg != nil:
		g := t.tg
		s.registerHandler(&CompHandler{
			Label: "tag/" + t.label,
			Node:  node,
			Down: func(lx, ly float64) bool {
				if inTagClose(g, node, lx, ly) {
					g.Close()
					return true
				}
				return false
			},
			Up: func(lx, ly float64) bool {
				g.Click()
				return true
			},
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					g.PressKey(key)
					return true
				}
				return false
			},
			Focusable: func() bool { return g.Focusable() },
		})
	case t.ctg != nil:
		c := t.ctg
		s.registerHandler(&CompHandler{
			Label: "tag/" + t.label,
			Node:  node,
			Down:  func(lx, ly float64) bool { return false },
			Up: func(lx, ly float64) bool {
				c.Toggle()
				return true
			},
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					c.PressKey(key)
					return true
				}
				return false
			},
			Focusable: func() bool { return c.Focusable() },
		})
	case t.fbtn != nil:
		f := t.fbtn
		s.registerHandler(&CompHandler{
			Label: "float-button/" + t.label,
			Node:  node,
			Move:  func(lx, ly float64) { f.SetHover(true) },
			Down:  func(lx, ly float64) bool { f.SetPressed(true); return true },
			Up: func(lx, ly float64) bool {
				f.SetPressed(false)
				return f.Click()
			},
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					return f.Click()
				}
				return false
			},
			Focusable: func() bool { return f.Focusable() },
		})
	case t.typo != nil:
		y := t.typo
		s.registerHandler(&CompHandler{
			Label: "typography/" + t.label,
			Node:  node,
			Down: func(lx, ly float64) bool {
				if y.Copyable() && inTypoAction(y, node, lx, ly) {
					return y.PressCopy()
				}
				if y.Editable() && !y.IsEditing() {
					return y.StartEdit()
				}
				if y.Expandable() && inTypoAction(y, node, lx, ly) {
					return y.ToggleExpand()
				}
				if y.Kind() == typography.KindLink {
					return y.Click()
				}
				return false
			},
			Up: func(lx, ly float64) bool { return false },
			Key: func(key string) bool {
				if y.IsEditing() {
					return y.PressKey(key)
				}
				if key == "Enter" || key == "Space" {
					if y.Copyable() {
						return y.PressCopy()
					}
					if y.Expandable() {
						return y.ToggleExpand()
					}
					return y.Click()
				}
				return false
			},
			Focusable: func() bool { return y.Focusable() },
		})
	case t.sider != nil:
		sd := t.sider
		s.registerHandler(&CompHandler{
			Label: "layout/" + t.label,
			Node:  node,
			Move: func(lx, ly float64) {
				sd.SetTriggerHovered(inSiderTrigger(sd, node, lx, ly))
			},
			Down: func(lx, ly float64) bool {
				if inSiderTrigger(sd, node, lx, ly) {
					sd.ActivateTrigger()
					return true
				}
				return false
			},
			Up: func(lx, ly float64) bool { return false },
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					return siderKey(sd, key)
				}
				return false
			},
			Focusable: func() bool { return sd.TriggerFocusable() },
		})
	case t.split != nil:
		sp := t.split
		st := &splitDrag{}
		s.registerHandler(&CompHandler{
			Label: "splitter/" + t.label,
			Node:  node,
			Move: func(lx, ly float64) {
				if st.active {
					sp.UpdateDrag(splitDelta(sp, st, lx, ly))
				}
			},
			Down: func(lx, ly float64) bool {
				bar := splitBarAt(sp, node, lx, ly)
				if bar < 0 {
					return false
				}
				st.bar = bar
				st.startLX, st.startLY = lx, ly
				st.active = sp.BeginDrag(bar)
				return st.active
			},
			Up: func(lx, ly float64) bool {
				if !st.active {
					return false
				}
				if lx < 0 || ly < 0 {
					sp.EndDrag()
					st.active = false
					return false
				}
				sp.UpdateDrag(splitDelta(sp, st, lx, ly))
				sp.EndDrag()
				st.active = false
				return true
			},
			Key: func(key string) bool {
				if key == "Enter" || key == "Space" {
					return sp.KeyActivate(key)
				}
				return false
			},
			Focusable: func() bool { return true },
		})
	}
	// Display-only components (progress/spin/skeleton/watermark/divider/
	// statistic/flex/grid/masonry/space/timeline/border-beam/icon): no
	// pointer or keyboard interaction per their §6.4 (decorative or
	// display). They are still hit-testable for hover highlight but
	// consume nothing, so clicks pass through to nothing and the window
	// stays stable. Explicit no-op keeps the contract visible.
}

// inTypoAction maps local coords to the copy/edit/expand affordance
// (right-edge action zone).
func inTypoAction(y *typography.Typography, node rendering.RenderObject, lx, ly float64) bool {
	if y == nil {
		return false
	}
	sz := node.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return false
	}
	const hit = 40.0
	return lx >= sz.Width-hit && lx < sz.Width && ly >= 0 && ly < sz.Height
}

// siderKey toggles the trigger via keyboard (Enter/Space).
func siderKey(sd *layout.Sider, key string) bool {
	if sd == nil || !sd.TriggerFocusable() {
		return false
	}
	if key == "Enter" || key == "Space" {
		sd.HandleTriggerKey(focus.KeyEnter)
		return true
	}
	return false
}

// inSiderTrigger maps local coords to the trigger zone: zero trigger is
// the top-left box, normal trigger is the bottom bar of sider width.
func inSiderTrigger(sd *layout.Sider, node rendering.RenderObject, lx, ly float64) bool {
	if sd == nil || !sd.TriggerVisible() {
		return false
	}
	sz := node.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return false
	}
	if sd.IsZeroTrigger() {
		tw := sd.EffectiveTriggerWidth()
		th := sd.EffectiveTriggerHeight()
		return lx >= 0 && ly >= 0 && lx < tw && ly < th
	}
	tw := sz.Width
	th := sd.EffectiveTriggerHeight()
	return lx >= 0 && lx < tw && ly >= sz.Height-th && ly < sz.Height
}

// splitDrag tracks one in-progress bar drag in gallery coords.
type splitDrag struct {
	bar           int
	startLX       float64
	startLY       float64
	active        bool
}

// splitBarAt finds the bar under local coords (horizontal: vertical bars).
func splitBarAt(sp *splitter.Splitter, node rendering.RenderObject, lx, ly float64) int {
	if sp == nil || sp.BarCount() <= 0 {
		return -1
	}
	sizes := sp.PanelSizes()
	if len(sizes) == 0 {
		return 0
	}
	const tol = 12.0
	x := 0.0
	for i, w := range sizes {
		x += w
		if i+1 < len(sizes) {
			if lx >= x-tol && lx < x+tol {
				return i
			}
		}
	}
	return -1
}

// splitDelta converts gallery local delta to bar delta (horizontal).
func splitDelta(sp *splitter.Splitter, st *splitDrag, lx, ly float64) float64 {
	if sp == nil || st == nil {
		return 0
	}
	if sp.IsVertical() {
		return ly - st.startLY
	}
	return lx - st.startLX
}

// keyEvent builds the focus-layer key event for activation keys.
func keyEvent(key string) focus.KeyEvent {
	code := focus.KeySpace
	if key == "Enter" {
		code = focus.KeyEnter
	}
	return focus.KeyEvent{KeyCode: code, Pressed: true}
}

// inClose maps local coords to the alert close box (right-anchored 44px).
func inClose(a *alert.Alert, node rendering.RenderObject, lx, ly float64) bool {
	if a == nil || !a.Closable() || a.Hidden() {
		return false
	}
	sz := node.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return false
	}
	const hit = 44.0
	return lx >= sz.Width-hit && lx < sz.Width && ly >= 0 && ly < hit
}

// inTagClose maps local coords to the tag close glyph (right edge).
func inTagClose(g *tag.Tag, node rendering.RenderObject, lx, ly float64) bool {
	if g == nil || !g.HasClose() || g.Hidden() {
		return false
	}
	sz := node.Size()
	if sz.Width <= 0 || sz.Height <= 0 {
		return false
	}
	const hit = 28.0
	cy := sz.Height / 2
	return lx >= sz.Width-hit && lx < sz.Width && ly >= cy-14 && ly < cy+14
}
