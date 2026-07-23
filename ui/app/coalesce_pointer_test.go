package app

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestCoalescePointerMovesKeepsLast(t *testing.T) {
	evs := []platform.Event{
		{Type: platform.EventPointer, Pointer: platform.PointerDown, X: 1, Y: 1},
		{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 2, Y: 2},
		{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 3, Y: 3},
		{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 10, Y: 20},
		{Type: platform.EventPointer, Pointer: platform.PointerUp, X: 10, Y: 20},
	}
	out := coalescePointerMoves(evs)
	if len(out) != 3 {
		t.Fatalf("want 3 events (down, last move, up), got %d", len(out))
	}
	if out[0].Pointer != platform.PointerDown {
		t.Fatal("first must be down")
	}
	if out[1].Pointer != platform.PointerMove || out[1].X != 10 || out[1].Y != 20 {
		t.Fatalf("move must be last sample: %+v", out[1])
	}
	if out[2].Pointer != platform.PointerUp {
		t.Fatal("last must be up")
	}
}

func TestCoalescePointerMovesPreservesInterleaved(t *testing.T) {
	evs := []platform.Event{
		{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 1, Y: 1},
		{Type: platform.EventScroll, X: 0, Y: 0, ScrollDY: 1},
		{Type: platform.EventPointer, Pointer: platform.PointerMove, X: 5, Y: 5},
	}
	out := coalescePointerMoves(evs)
	if len(out) != 3 {
		t.Fatalf("interleaved non-move must flush pending: len=%d", len(out))
	}
}
