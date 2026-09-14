// Gallery interaction backbone (industrial target: antd parity).
//
// Scene owns live wrappers through handler closures so pointer/keyboard
// reach the real components. Hit order is reverse registration (last
// mounted = topmost). Coordinates are logical px, Y-down, same as
// platform.Event.
package main

import (
	"fmt"
	"os"

	"github.com/energye/gpui/ui/rendering"
)

// CompHandler routes platform input to one live component node.
// Local coords are relative to Node origin (0..w, 0..h).
type CompHandler struct {
	Label string
	Node  rendering.RenderObject
	Move  func(lx, ly float64)
	Down  func(lx, ly float64) bool
	Up    func(lx, ly float64) bool
	Key   func(key string) bool
	// Focusable reports Tab reachability (disabled => false).
	Focusable func() bool
}

func (s *Scene) clearInteractions() {
	s.handlers = nil
	s.handlerOrder = nil
	s.pressNode = nil
	s.hoverNode = nil
	s.focusNode = nil
	s.focusIdx = -1
}

func (s *Scene) registerHandler(h *CompHandler) {
	if h == nil || h.Node == nil {
		return
	}
	if s.handlers == nil {
		s.handlers = map[rendering.RenderObject]*CompHandler{}
	}
	s.handlers[h.Node] = h
	s.handlerOrder = append(s.handlerOrder, h.Node)
}

// absOrigin sums offsets up to the root (Root at 0,0).
func absOrigin(n rendering.RenderObject) (x, y float64) {
	for c := n; c != nil; c = c.Parent() {
		o := c.Offset()
		x += o.X
		y += o.Y
	}
	return x, y
}

func (s *Scene) handlerAt(x, y float64) (*CompHandler, float64, float64) {
	// Topmost last: reverse registration order.
	for i := len(s.handlerOrder) - 1; i >= 0; i-- {
		n := s.handlerOrder[i]
		h := s.handlers[n]
		if h == nil {
			continue
		}
		ox, oy := absOrigin(n)
		sz := n.Size()
		if sz.Width <= 0 || sz.Height <= 0 {
			continue
		}
		if x >= ox && y >= oy && x < ox+sz.Width && y < oy+sz.Height {
			return h, x - ox, y - oy
		}
	}
	return nil, 0, 0
}

// DispatchMove updates hover; returns handler label or "".
func (s *Scene) DispatchMove(x, y float64) string {
	h, lx, ly := s.handlerAt(x, y)
	var node rendering.RenderObject
	if h != nil {
		node = h.Node
	}
	if node != s.hoverNode {
		s.hoverNode = node
	}
	if h != nil && h.Move != nil {
		h.Move(lx, ly)
		return h.Label
	}
	return ""
}

// DispatchDown presses; returns true when consumed.
func (s *Scene) DispatchDown(x, y float64) (string, bool) {
	h, lx, ly := s.handlerAt(x, y)
	s.pressNode = nil
	if h == nil {
		return "", false
	}
	s.pressNode = h.Node
	if h.Down != nil {
		ok := h.Down(lx, ly)
		if ok {
			fmt.Fprintf(os.Stderr, "gallery: press %s\n", h.Label)
		}
		return h.Label, ok
	}
	return h.Label, false
}

// DispatchUp releases; click fires when press started and ended inside.
func (s *Scene) DispatchUp(x, y float64) (string, bool) {
	h, lx, ly := s.handlerAt(x, y)
	if h != nil && h.Up != nil {
		ok := h.Up(lx, ly)
		if s.pressNode != nil && s.pressNode != h.Node {
			// Press started elsewhere: let previous handler release outside.
			if prev, ok2 := s.handlers[s.pressNode]; ok2 && prev.Up != nil {
				prev.Up(-1, -1)
			}
		}
		s.pressNode = nil
		if ok {
			fmt.Fprintf(os.Stderr, "gallery: activate %s\n", h.Label)
		}
		return h.Label, ok
	}
	if s.pressNode != nil {
		if prev, ok2 := s.handlers[s.pressNode]; ok2 && prev.Up != nil {
			prev.Up(-1, -1)
		}
		s.pressNode = nil
	}
	return "", false
}

// FocusNext moves Tab focus; returns label or "".
func (s *Scene) FocusNext() string {
	if len(s.handlerOrder) == 0 {
		return ""
	}
	n := len(s.handlerOrder)
	for step := 1; step <= n; step++ {
		s.focusIdx = (s.focusIdx + step) % n
		// Linear scan from focusIdx for next focusable.
		for k := 0; k < n; k++ {
			idx := (s.focusIdx + k) % n
			h := s.handlers[s.handlerOrder[idx]]
			if h != nil && h.Focusable != nil && h.Focusable() {
				s.focusIdx = idx
				s.focusNode = h.Node
				fmt.Fprintf(os.Stderr, "gallery: focus %s\n", h.Label)
				return h.Label
			}
		}
		break
	}
	return ""
}

// DispatchKey routes Enter/Space/Escape/Tab; returns true when consumed.
func (s *Scene) DispatchKey(key string) (string, bool) {
	if key == "Tab" {
		return s.FocusNext(), true
	}
	if key == "Escape" {
		s.pressNode = nil
		// Fall through to focused/hovered routing so open bubbles
		// (tooltip/popover) close via Escape like their unit tests.
	}
	// Focused handler first.
	if s.focusNode != nil {
		if h, ok := s.handlers[s.focusNode]; ok && h != nil && h.Key != nil {
			if h.Key(key) {
				return h.Label, true
			}
		}
	}
	// Otherwise try hovered handler.
	if s.hoverNode != nil {
		if h, ok := s.handlers[s.hoverNode]; ok && h != nil && h.Key != nil {
			if h.Key(key) {
				return h.Label, true
			}
		}
	}
	return "", false
}
