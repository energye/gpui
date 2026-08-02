package rendering_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func TestSnapCoord_DeviceGrid(t *testing.T) {
	cases := []struct {
		v, scale, want float64
	}{
		{0.3, 1, 0},
		{0.6, 1, 1},
		{0.25, 2, 0.5},   // 0.25*2=0.5 → round=0? no: math.Round(0.5)=1 → 0.5
		{1.37, 2, 1.5},   // 2.74 → 3 → 1.5
		{0.1, 3, 0},      // 0.3 → 0
		{2.6, 3, 2.6667}, // 7.8 → 8 → 2.6667
	}
	for _, c := range cases {
		got := rendering.SnapCoord(c.v, c.scale)
		if math.Abs(got-c.want) > 0.0001 {
			t.Fatalf("SnapCoord(%v, %v)=%v want %v", c.v, c.scale, got, c.want)
		}
	}
	// scale <= 0 → identity DPR.
	if got := rendering.SnapCoord(0.5, 0); got != 1 {
		t.Fatalf("SnapCoord(0.5,0)=%v want 1 (default DPR 1)", got)
	}
}

func TestSnapLine_HairlineCenter(t *testing.T) {
	// DPR 2: a horizontal line at logical y=10.4 must land on the pixel grid
	// centered — floor(10.4*2)+0.5=20.5 physical → 10.25 logical, minus half
	// hairline (0.25) = 10.0 logical origin. The stroke covers exactly one
	// physical pixel row.
	got := rendering.SnapLine(10.4, 2)
	if math.Abs(got-10.0) > 0.0001 {
		t.Fatalf("SnapLine(10.4,2)=%v want 10.0", got)
	}
	// DPR 1: y=3.6 → floor(3.6)+0.5=3.5, minus 0.5 = 3.0.
	got = rendering.SnapLine(3.6, 1)
	if math.Abs(got-3.0) > 0.0001 {
		t.Fatalf("SnapLine(3.6,1)=%v want 3.0", got)
	}
}

func TestSnapRect_GridAlignedEdges(t *testing.T) {
	// DPR 2: rect edges must land on the device grid after snap.
	sx, sy, sw, sh := rendering.SnapRect(1.3, 2.6, 10.4, 5.2, 2)
	// x0=round(2.6)=3→1.5; x1=round(23.4)=23→11.5; sw=10
	if math.Abs(sx-1.5) > 0.0001 || math.Abs(sw-10) > 0.0001 {
		t.Fatalf("snapped rect x,w = %v,%v want 1.5,10", sx, sw)
	}
	// y0=round(5.2)=5→2.5; y1=round(15.6)=16→8; sh=5.5
	if math.Abs(sy-2.5) > 0.0001 || math.Abs(sh-5.5) > 0.0001 {
		t.Fatalf("snapped rect y,h = %v,%v want 2.5,5.5", sy, sh)
	}
}
