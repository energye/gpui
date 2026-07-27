package platform_test

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestHeadlessInjectClick(t *testing.T) {
	h := platform.NewHeadless(200, 100)
	defer h.Close()
	h.InjectClick(10, 20)
	evs := h.PumpEvents()
	if len(evs) != 2 {
		t.Fatalf("events=%d want 2", len(evs))
	}
	if evs[0].Pointer != platform.PointerDown || evs[1].Pointer != platform.PointerUp {
		t.Fatalf("kinds %v %v", evs[0].Pointer, evs[1].Pointer)
	}
	if evs[0].X != 10 || evs[0].Y != 20 {
		t.Fatalf("pos=%v,%v", evs[0].X, evs[0].Y)
	}
}

func TestHeadlessResize(t *testing.T) {
	h := platform.NewHeadless(100, 100)
	defer h.Close()
	h.Resize(320, 240)
	w, ht := h.Size()
	if w != 320 || ht != 240 {
		t.Fatalf("size=%dx%d", w, ht)
	}
	evs := h.PumpEvents()
	if len(evs) != 1 || evs[0].Type != platform.EventResize {
		t.Fatalf("events=%v", evs)
	}
}

func TestCapsHas(t *testing.T) {
	c := platform.HeadlessCaps
	if !c.Has(platform.CapPointer) {
		t.Fatal("missing CapPointer")
	}
	if !c.Has(platform.CapIME) {
		t.Fatal("Headless must advertise CapIME for composition inject path")
	}
	// Linux true-window host does not set CapIME (see ime.go / Caps contract).
}

func TestHeadlessIMEInjectAndPosition(t *testing.T) {
	h := platform.NewHeadless(200, 100)
	defer h.Close()
	if !h.Caps().Has(platform.CapIME) {
		t.Fatal("want CapIME")
	}
	h.InjectIME("ni", false)
	h.InjectIME("", true)
	h.InjectText("你")
	evs := h.PumpEvents()
	if len(evs) != 3 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Type != platform.EventIME || evs[0].IMEText != "ni" || evs[0].IMEEnd {
		t.Fatalf("ev0=%+v", evs[0])
	}
	if evs[1].Type != platform.EventIME || !evs[1].IMEEnd {
		t.Fatalf("ev1=%+v", evs[1])
	}
	if evs[2].Type != platform.EventText || evs[2].Text != "你" {
		t.Fatalf("ev2=%+v", evs[2])
	}
	if !platform.SetIMEPositionIfSupported(h, 12, 34) {
		t.Fatal("Headless should implement IMEPositioner")
	}
	x, y, n := h.LastIMEPosition()
	if x != 12 || y != 34 || n != 1 {
		t.Fatalf("ime pos x=%v y=%v n=%d", x, y, n)
	}
}

func TestDetectDisplayBackendGPUI(t *testing.T) {
	t.Setenv("GPUI_DISPLAY", "x11")
	if platform.DetectDisplayBackend() != platform.DisplayX11 {
		t.Fatalf("GPUI_DISPLAY=x11 → want X11 got %v", platform.DetectDisplayBackend())
	}
	t.Setenv("GPUI_DISPLAY", "wayland")
	if platform.DetectDisplayBackend() != platform.DisplayWayland {
		t.Fatalf("GPUI_DISPLAY=wayland → want Wayland")
	}
	t.Setenv("GPUI_DISPLAY", "auto")
	// auto falls through to session env — just ensure it doesn't panic
	_ = platform.DetectDisplayBackend()
}

func TestParseDisplayBackend(t *testing.T) {
	cases := map[string]platform.DisplayBackend{
		"x11": platform.DisplayX11, "X": platform.DisplayX11,
		"wayland": platform.DisplayWayland, "wl": platform.DisplayWayland,
		"auto": platform.DisplayAuto, "": platform.DisplayAuto,
	}
	for in, want := range cases {
		if got := platform.ParseDisplayBackend(in); got != want {
			t.Fatalf("Parse(%q)=%v want %v", in, got, want)
		}
	}
}

func TestPlatformKindString(t *testing.T) {
	if platform.PlatformX11.String() != "x11" {
		t.Fatal(platform.PlatformX11.String())
	}
	if platform.PlatformWayland.String() != "wayland" {
		t.Fatal(platform.PlatformWayland.String())
	}
}
