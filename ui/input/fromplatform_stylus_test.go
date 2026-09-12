package input

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestFromPlatform_Stylus(t *testing.T) {
	down := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerDown,
		X: 10.5, Y: 20.25, StylusID: 0,
		StylusPressure: 0.5, StylusTiltX: 10, StylusTiltY: -5,
	}, Modifiers{})
	if down.Kind != KindStylus {
		t.Fatalf("kind = %s, want stylus", down.Kind)
	}
	if down.Stylus.Kind != PointerDown || down.Stylus.ID != 0 {
		t.Fatalf("phase/id = %s/%d", down.Stylus.Kind, down.Stylus.ID)
	}
	if down.Stylus.X != 10.5 || down.Stylus.Y != 20.25 {
		t.Fatalf("pos = (%.2f,%.2f)", down.Stylus.X, down.Stylus.Y)
	}
	if down.Stylus.Pressure != 0.5 || down.Stylus.TiltX != 10 || down.Stylus.TiltY != -5 {
		t.Fatalf("pen = %+v", down.Stylus)
	}
	if down.Stylus.Eraser {
		t.Fatal("eraser should be false")
	}
	eraser := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerMove,
		StylusPressure: 1, StylusEraser: true,
	}, Modifiers{})
	if eraser.Stylus.Kind != PointerMove || !eraser.Stylus.Eraser {
		t.Fatalf("eraser = %+v", eraser.Stylus)
	}
}

func TestFromPlatform_StylusPressureRange(t *testing.T) {
	// Sensor-less fallback 1 passes through; hover/unknown 0 is preserved
	// (backend fills 1, FromPlatform only clamps the range).
	full := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerDown, StylusPressure: 1,
	}, Modifiers{})
	if full.Stylus.Pressure != 1 {
		t.Fatalf("pressure = %v, want 1", full.Stylus.Pressure)
	}
	hover := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerMove, StylusPressure: 0,
	}, Modifiers{})
	if hover.Stylus.Pressure != 0 {
		t.Fatalf("hover pressure = %v, want 0", hover.Stylus.Pressure)
	}
	neg := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerMove, StylusPressure: -2,
	}, Modifiers{})
	if neg.Stylus.Pressure != 0 {
		t.Fatalf("negative clamped = %v, want 0", neg.Stylus.Pressure)
	}
	over := FromPlatform(platform.Event{
		Type: platform.EventStylus, Pointer: platform.PointerDown, StylusPressure: 5,
	}, Modifiers{})
	if over.Stylus.Pressure != 1 {
		t.Fatalf("overflow clamped = %v, want 1", over.Stylus.Pressure)
	}
}

func TestFromPlatform_StylusPhaseClamp(t *testing.T) {
	cases := []struct {
		in   platform.PointerKind
		want PointerKind
	}{
		{platform.PointerDown, PointerDown},
		{platform.PointerUp, PointerUp},
		{platform.PointerCancel, PointerCancel},
		{platform.PointerMove, PointerMove},
		{platform.PointerEnter, PointerMove},
		{platform.PointerScroll, PointerMove},
	}
	for _, c := range cases {
		ev := FromPlatform(platform.Event{Type: platform.EventStylus, Pointer: c.in}, Modifiers{})
		if ev.Kind != KindStylus || ev.Stylus.Kind != c.want {
			t.Errorf("phase %v: got %s/%s, want stylus/%s", c.in, ev.Kind, ev.Stylus.Kind, c.want)
		}
	}
}

func TestFromStylus(t *testing.T) {
	ev := FromStylus(StylusEvent{Kind: PointerUp, ID: 1, X: 3, Y: 4, Pressure: 0.75}, Modifiers{Alt: true})
	if ev.Kind != KindStylus || ev.Stylus.ID != 1 || ev.Stylus.Pressure != 0.75 {
		t.Fatalf("stylus = %+v", ev)
	}
	if !ev.Modifiers.Alt {
		t.Fatal("mods not preserved")
	}
}

func TestStylusEventString(t *testing.T) {
	if KindStylus.String() != "stylus" {
		t.Fatalf("kind string = %q", KindStylus.String())
	}
	if (Event{Kind: KindStylus}.Kind.String()) != "stylus" {
		t.Fatalf("event kind string mismatch")
	}
}
