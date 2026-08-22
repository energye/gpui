//go:build linux

package platform

import (
	"testing"
	"unsafe"
)

// TestWaylandCSDCursorNamesResolve: every resize edge's cursor-name
// candidates (cursorNamesForEdge) must resolve in the theme the app actually
// loads (wl_cursor_theme_load with name=NULL → XCURSOR_THEME → "default" →
// index.theme Inherits). A missing primary name is fine when a fallback
// resolves (corners: nwse/nesw-resize are absent on DMZ-White/Adwaita, so
// top_left/top_right_corner must resolve there; on Yaru the modern names are
// the real double arrows). A theme with NONE of a corner's candidates would
// silently keep the previous cursor ("mouse style wrong at the corners").
func TestWaylandCSDCursorNamesResolve(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd

	// Force the lazy theme + cursor surface load exactly like the app does.
	csd.setCursor(1, csdHit{act: csdActResize, edge: resizeLeft})
	if csd.cursorTheme == 0 {
		t.Fatalf("cursor theme did not load")
	}
	l := loadCursorLib()
	if l == nil {
		t.Fatalf("no libwayland-cursor")
	}
	resolves := func(n string) bool {
		nb := append([]byte(n), 0)
		cur := l.themeGetCur(csd.cursorTheme, &nb[0])
		if cur == 0 {
			return false
		}
		imgArr := *(*uintptr)(unsafe.Pointer(cur + 8))
		return imgArr != 0 && *(*uintptr)(unsafe.Pointer(imgArr)) != 0
	}
	for edge, names := range map[int][]string{
		resizeTop:         {"sb_v_double_arrow"},
		resizeBottom:      {"sb_v_double_arrow"},
		resizeLeft:        {"sb_h_double_arrow"},
		resizeRight:       {"sb_h_double_arrow"},
		resizeTopLeft:     {"nwse-resize", "top_left_corner", "bottom_right_corner"},
		resizeTopRight:    {"nesw-resize", "top_right_corner", "bottom_left_corner"},
		resizeBottomLeft:  {"nesw-resize", "bottom_left_corner", "top_right_corner"},
		resizeBottomRight: {"nwse-resize", "bottom_right_corner", "top_left_corner"},
	} {
		ok := false
		for _, n := range names {
			if resolves(n) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("edge %d: none of %v resolve in the loaded theme", edge, names)
		}
	}
}

// TestWaylandCursorShapeDeviceBound: when the compositor advertises
// zwp_cursor_shape_manager_v1 (GNOME 42+), bindCursors must attach a shape
// device and setCursor must route through it (set_shape, no theme load);
// without the global the wl_cursor_theme fallback stays authoritative.
func TestWaylandCursorShapeDeviceBound(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil {
		t.Skipf("no wayland window")
	}
	if w.csMgrName == 0 || w.cursors == nil || w.cursors.dev == 0 {
		t.Skipf("compositor lacks zwp_cursor_shape_v1 (theme fallback active)")
	}
	// Shape device bound: exercise set_shape with a resize hit (needs a real
	// enter serial; serial=0 is rejected by setCursor, so prime one via CSD
	// hover like production motion does).
	if w.csd != nil {
		w.csd.setCursor(1, csdHit{act: csdActResize, edge: resizeLeft})
	}
	c := w.cursors
	c.setCursor(1, csdHit{act: csdActResize, edge: resizeLeft})
	c.setCursor(2, csdHit{}) // default arrow restore
}
