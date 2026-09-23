package h264

import "testing"

// Cross-list strength unit: list 0 index 0 and list 1 index 0 are
// different pictures, so the edge filters on the list-1 motion step
// (1080p poc2 MB(0,0)/MB(0,1): L0 (0,0)/(0,3), L1 (0,1)/(0,-3)).
// Bare-index compare reports equal-equal and drops the bS=1 filter;
// picture compare reports 1 like the reference (check_mv via
// ref2frm-mapped filter caches).
func TestInterBSCrossListPicture(t *testing.T) {
	p0, err := NewPicture(16, 32)
	if err != nil {
		t.Fatal(err)
	}
	p1, err := NewPicture(16, 32)
	if err != nil {
		t.Fatal(err)
	}
	mbW, mbH := 1, 2
	stride := mbW * 4
	n4 := stride * mbH * 4
	mbIntra := make([]bool, mbW*mbH)
	nnz := make([]int8, n4)
	mvX := make([]int16, n4)
	mvY := make([]int16, n4)
	mvX1 := make([]int16, n4)
	mvY1 := make([]int16, n4)
	refs := make([]int8, n4)
	refs1 := make([]int8, n4)
	useM := make([]uint8, n4)
	for y := 0; y < 8; y++ {
		for x := 0; x < 4; x++ {
			i := y*stride + x
			refs[i], refs1[i] = 0, 0
			useM[i] = useL0 | useL1
			if y < 4 {
				mvX[i], mvY[i] = 0, 0
				mvX1[i], mvY1[i] = 0, 1
			} else {
				mvX[i], mvY[i] = 0, 3
				mvX1[i], mvY1[i] = 0, -3
			}
		}
	}
	// sy=16 is the horizontal MB boundary, seg column 0.
	if got := interBS(mbIntra, nnz, mvX, mvY, refs, []*Picture{p0}, mbW, mbH,
		0, 16, true, true, nil, mvX1, mvY1, refs1, []*Picture{p1}, useM); got != 1 {
		t.Fatalf("cross-list edge bS = %d want 1", got)
	}
	// Same picture on both lists mirrors: cross motion pairs stay
	// under a full sample, so the edge stays 0.
	if got := interBS(mbIntra, nnz, mvX, mvY, refs, []*Picture{p0}, mbW, mbH,
		0, 16, true, true, nil, mvX1, mvY1, refs1, []*Picture{p0}, useM); got != 0 {
		t.Fatalf("mirrored edge bS = %d want 0", got)
	}
}
