package h264

import "testing"

// TestBDirectSpatial pins spatial-direct derivation on a synthetic 2x2 MB
// picture: the bottom-right MB reads top/left/diagonal neighbours plus
// the colocated block, with no bitstream involved.
func TestBDirectSpatial(t *testing.T) {
	d := NewDecoder(nil)
	d.mbW, d.mbH = 2, 2
	n4 := 8 * 8
	d.mvX = make([]int16, n4)
	d.mvY = make([]int16, n4)
	d.refIdx = make([]int8, n4)
	d.mvX1 = make([]int16, n4)
	d.mvY1 = make([]int16, n4)
	d.refIdx1 = make([]int8, n4)
	d.useM = make([]uint8, n4)
	d.mbSlice = make([]int, 4)
	for i := range d.refIdx {
		d.refIdx[i] = -1
		d.refIdx1[i] = -1
	}
	mkP := func(w, h int) *Picture {
		return &Picture{Width: uint32(w * 16), Height: uint32(h * 16),
			MotW4: w * 4, MotH4: h * 4,
			Rf0: make([]int8, w*4*h*4), Rf1: make([]int8, w*4*h*4),
			MV0x: make([]int16, w*4*h*4), MV0y: make([]int16, w*4*h*4),
			MV1x: make([]int16, w*4*h*4), MV1y: make([]int16, w*4*h*4)}
	}
	// Neighbours of MB(1,1): left MB(0,1) L0 ref0 mv(4,0); top MB(1,0)
	// L0 ref0 mv(8,0); diagonal MB(0,0)... top-right of MB(1,1) is
	// outside (2 outweighs), so C falls back to top-left MB(0,0), left
	// intra (ref -1). L1 everywhere intra.
	setL0 := func(mbx, mby int, ref int8, mx, my int16) {
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				i := (mby*4+y)*8 + mbx*4 + x
				d.refIdx[i] = ref
				d.mvX[i], d.mvY[i] = mx, my
				if ref >= 0 {
					d.useM[i] |= useL0
				}
			}
		}
	}
	setL0(0, 1, 0, 4, 0)
	setL0(1, 0, 0, 8, 0)
	h := &SliceHeader{DirectSpatial: true}
	moving := mkP(2, 2)
	moving.Rf0[(1*4)*8+1*4] = 1 // colocated L0 ref1: not stationary
	moving.MV0x[(1*4)*8+1*4] = 6
	got, err := d.bDirectMB(1, 1, h, moving)
	if err != nil {
		t.Fatalf("direct: %v", err)
	}
	// L0: neighbours ref 0,0,-1 -> min 0, two match -> median((4,0),(8,0),(0,0)).
	if got[0].ref != 0 || !got[0].use {
		t.Fatalf("l0 = %+v, want ref 0 use", got[0])
	}
	if got[0].mx != 4 || got[0].my != 0 {
		t.Fatalf("l0 mv = (%d,%d), want (4,0)", got[0].mx, got[0].my)
	}
	// L1: no neighbour uses it -> ref -1 -> backward dropped.
	if got[1].use || got[1].ref != -1 {
		t.Fatalf("l1 = %+v, want unused ref -1", got[1])
	}
	// Stationary colocated zeroes the zero-ref side: both sides ref 0
	// here, so both motions go to zero.
	still := mkP(2, 2)
	still.Rf0[(1*4)*8+1*4] = 0
	still.MV0x[(1*4)*8+1*4] = 1 // quarter-pel still counts as still
	setL0(1, 0, 1, 8, 0)        // top now ref1: min stays 0 via left
	got, err = d.bDirectMB(1, 1, h, still)
	if err != nil {
		t.Fatalf("direct still: %v", err)
	}
	if got[0].mx != 0 || got[0].my != 0 {
		t.Fatalf("still l0 mv = (%d,%d), want (0,0)", got[0].mx, got[0].my)
	}
	// Temporal direct stops readable.
	hT := &SliceHeader{DirectSpatial: false}
	if _, err := d.bDirectMB(1, 1, hT, still); err == nil {
		t.Fatal("temporal direct: want stage error")
	}
}
