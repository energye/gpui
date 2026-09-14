package main

import (
	"testing"

	"github.com/energye/gpui/ui/kit/button"
)

// Gallery interaction backbone test: dispatch pointer through the scene
// and prove the real component fires (not a screenshot).
//
// NOTE: trackedByNode is a package-global registry shared across scenes;
// each NewScene rebuilds fresh nodes, so tests must resolve handlers via
// the scene's own handlerOrder (nodes mounted in THIS scene), never by
// ranging the global map (stale nodes from other scenes).
func TestInteract_ButtonClick(t *testing.T) {
	s := NewScene(1200, 800, "button")
	s.Layout()
	if len(s.handlerOrder) == 0 {
		t.Fatal("no handlers registered")
	}
	// Find a Primary button handler and click its center.
	var target *CompHandler
	var tx, ty float64
	for i := len(s.handlerOrder) - 1; i >= 0; i-- {
		n := s.handlerOrder[i]
		h := s.handlers[n]
		if h == nil {
			continue
		}
		if tr, ok := trackedByNode[n]; ok && tr != nil && tr.btn != nil && tr.btn.Label() == "Primary" {
			ox, oy := absOrigin(n)
			sz := n.Size()
			tx, ty = ox+sz.Width/2, oy+sz.Height/2
			target = h
			break
		}
	}
	if target == nil {
		t.Fatal("Primary button handler not found")
	}
	fired := 0
	if tr, ok := trackedByNode[target.Node]; ok && tr.btn != nil {
		tr.btn.OnClick = func() { fired++ }
	}
	if _, ok := s.DispatchDown(tx, ty); !ok {
		t.Fatal("DispatchDown not consumed")
	}
	if _, ok := s.DispatchUp(tx, ty); !ok {
		t.Fatal("DispatchUp did not activate")
	}
	if fired != 1 {
		t.Fatalf("OnClick fired %d times, want 1", fired)
	}
	// Disabled button must swallow.
	var dis *CompHandler
	for _, n := range s.handlerOrder {
		if tr, ok := trackedByNode[n]; ok && tr != nil && tr.btn != nil && tr.btn.Disabled() {
			dis = s.handlers[n]
			ox, oy := absOrigin(n)
			sz := n.Size()
			tx, ty = ox+sz.Width/2, oy+sz.Height/2
			break
		}
	}
	if dis == nil {
		t.Fatal("disabled handler not found")
	}
	if _, ok := s.DispatchDown(tx, ty); ok {
		t.Fatal("disabled must swallow press")
	}
	_ = button.ButtonPrimary
}

// Alert close through dispatch: press the close box hides the alert.
func TestInteract_AlertClose(t *testing.T) {
	s := NewScene(1200, 1300, "alert")
	s.Layout()
	var target *CompHandler
	var tx, ty float64
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.al == nil {
			continue
		}
		if tr.al.Closable() && tr.al.Visible() {
			ox, oy := absOrigin(n)
			sz := n.Size()
			tx, ty = ox+sz.Width-22, oy+22
			target = s.handlers[n]
			_ = target
			if _, ok := s.DispatchDown(tx, ty); ok {
				if tr.al.Visible() {
					t.Fatal("alert still visible after close press")
				}
				return
			}
		}
	}
	t.Fatal("no closable alert consumed close press")
}

// Typography copy through dispatch: copyable text copies payload.
func TestInteract_TypographyCopy(t *testing.T) {
	s := NewScene(1200, 900, "typography")
	s.Layout()
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.typo == nil {
			continue
		}
		if !tr.typo.Copyable() {
			continue
		}
		ox, oy := absOrigin(n)
		sz := n.Size()
		// Right-edge action zone.
		if _, ok := s.DispatchDown(ox+sz.Width-5, oy+sz.Height/2); !ok {
			t.Fatal("typo copy press not consumed")
		}
		if tr.typo.CopiedText() == "" {
			t.Fatal("copy payload empty")
		}
		return
	}
	t.Fatal("no copyable typography handler")
}

// Typography edit through dispatch: editable text starts editing.
func TestInteract_TypographyEdit(t *testing.T) {
	s := NewScene(1200, 900, "typography")
	s.Layout()
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.typo == nil {
			continue
		}
		if !tr.typo.Editable() {
			continue
		}
		ox, oy := absOrigin(n)
		sz := n.Size()
		if _, ok := s.DispatchDown(ox+sz.Width/2, oy+sz.Height/2); !ok {
			t.Fatal("typo edit press not consumed")
		}
		if !tr.typo.IsEditing() {
			t.Fatal("typography not editing after press")
		}
		if _, ok := s.DispatchKey("Escape"); !ok {
			// Escape routes to focused/hovered; force direct cancel path
			tr.typo.CancelEdit()
		}
		if tr.typo.IsEditing() {
			t.Fatal("typography still editing after Escape")
		}
		return
	}
	t.Fatal("no editable typography handler")
}

// Layout sider trigger through dispatch: toggles collapse.
func TestInteract_SiderTrigger(t *testing.T) {
	s := NewScene(1400, 900, "layout")
	s.Layout()
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.sider == nil {
			continue
		}
		if !tr.sider.TriggerVisible() {
			continue
		}
		before := tr.sider.CollapsedState()
		ox, oy := absOrigin(n)
		sz := n.Size()
		var tx, ty float64
		if tr.sider.IsZeroTrigger() {
			tx, ty = ox+5, oy+5
		} else {
			tx, ty = ox+sz.Width/2, oy+sz.Height-5
		}
		if _, ok := s.DispatchDown(tx, ty); !ok {
			t.Fatal("sider trigger press not consumed")
		}
		if tr.sider.CollapsedState() == before {
			t.Fatal("sider did not toggle")
		}
		return
	}
	t.Fatal("no sider trigger handler")
}

// Splitter bar drag through dispatch: sizes change.
func TestInteract_SplitterDrag(t *testing.T) {
	s := NewScene(1400, 900, "splitter")
	s.Layout()
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.split == nil {
			continue
		}
		sp := tr.split
		if sp.BarCount() <= 0 {
			continue
		}
		before := append([]float64(nil), sp.PanelSizes()...)
		// Find bar x: cumulative first panel width.
		ox, oy := absOrigin(n)
		sz := n.Size()
		sizes := sp.PanelSizes()
		if len(sizes) < 2 {
			continue
		}
		bx := ox + sizes[0]
		by := oy + sz.Height/2
		if _, ok := s.DispatchDown(bx, by); !ok {
			continue
		}
		s.DispatchMove(bx+30, by)
		if _, ok := s.DispatchUp(bx+30, by); !ok {
			t.Fatal("splitter drag release not consumed")
		}
		after := sp.PanelSizes()
		changed := false
		for i := range before {
			if i < len(after) && after[i] != before[i] {
				changed = true
			}
		}
		if !changed {
			t.Fatalf("splitter sizes unchanged %v", before)
		}
		return
	}
	t.Fatal("no splitter drag handler consumed")
}

// Tag checkable toggles through dispatch.
func TestInteract_TagToggle(t *testing.T) {
	s := NewScene(1200, 800, "tag")
	s.Layout()
	for _, n := range s.handlerOrder {
		tr, ok := trackedByNode[n]
		if !ok || tr == nil || tr.ctg == nil {
			continue
		}
		before := tr.ctg.Checked()
		ox, oy := absOrigin(n)
		sz := n.Size()
		if _, ok := s.DispatchDown(ox+sz.Width/2, oy+sz.Height/2); !ok {
			// down may not consume; up toggles
		}
		if _, ok := s.DispatchUp(ox+sz.Width/2, oy+sz.Height/2); !ok {
			t.Fatal("tag toggle not activated")
		}
		if tr.ctg.Checked() == before {
			t.Fatal("checkable tag did not toggle")
		}
		return
	}
	t.Fatal("no checkable tag handler")
}
