package primitive

import "testing"

func TestSnapDevice(t *testing.T) {
	if g := snapDevice(10.4, 1); g != 10 {
		t.Fatalf("scale1: %v", g)
	}
	if g := snapDevice(10.6, 1); g != 11 {
		t.Fatalf("scale1 round up: %v", g)
	}
	// HiDPI 2x: snap to 0.5 logical grid
	if g := snapDevice(10.24, 2); g != 10.0 && g != 10.5 {
		// 10.24*2=20.48 → 20 → 10.0
		if g != 10 {
			t.Fatalf("scale2: %v want 10", g)
		}
	}
	if g := snapDevice(10.3, 2); g != 10.5 {
		// 10.3*2=20.6 → 21 → 10.5
		t.Fatalf("scale2 half: %v want 10.5", g)
	}
}
