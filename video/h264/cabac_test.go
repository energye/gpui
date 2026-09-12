package h264

import "testing"

// CABAC numeric cross-check (VR2c gate, first half): table shapes, spot
// values transcribed from the reference decoder's tables
// (libavcodec/cabac.c + h264_cabac.c), and hand-verified init outputs.
// The pixel-exact Main clip gate is the second half; both must pass.

// TestCabacTableShapes guards dimensions and value ranges.
func TestCabacTableShapes(t *testing.T) {
	if len(cabacNormShift) != 512 || len(cabacLPSRange) != 512 {
		t.Fatalf("engine tables = %d/%d want 512/512",
			len(cabacNormShift), len(cabacLPSRange))
	}
	if len(cabacMLPSState) != 256 {
		t.Fatalf("mlps = %d want 256", len(cabacMLPSState))
	}
	if len(cabacInitI) != 2048 || len(cabacInitPB) != 6144 {
		t.Fatalf("init tables = %d/%d want 2048/6144",
			len(cabacInitI), len(cabacInitPB))
	}
	for i, v := range cabacNormShift {
		if v > 9 {
			t.Fatalf("normShift[%d] = %d out of range", i, v)
		}
	}
	for i, v := range cabacMLPSState {
		if v > 127 {
			t.Fatalf("mlps[%d] = %d out of range", i, v)
		}
	}
}

// TestCabacTableSpots cross-checks engine-table heads against the
// reference decoder's transcription.
func TestCabacTableSpots(t *testing.T) {
	normHead := [16]uint8{9, 8, 7, 7, 6, 6, 6, 6, 5, 5, 5, 5, 5, 5, 5, 5}
	for i, want := range normHead {
		if cabacNormShift[i] != want {
			t.Fatalf("normShift[%d] = %d want %d", i, cabacNormShift[i], want)
		}
	}
	if cabacNormShift[16] != 4 || cabacNormShift[32] != 3 ||
		cabacNormShift[64] != 2 || cabacNormShift[128] != 1 ||
		cabacNormShift[256] != 0 || cabacNormShift[511] != 0 {
		t.Fatal("normShift octave steps wrong")
	}
	// LPS range head: entries pair up, magnitudes shrink down the column.
	lpsHead := [16]int8{-128, -128, -128, -128, -128, -128, 123, 123,
		116, 116, 111, 111, 105, 105, 100, 100}
	for i, want := range lpsHead {
		if cabacLPSRange[i] != want {
			t.Fatalf("lpsRange[%d] = %d want %d", i, cabacLPSRange[i], want)
		}
	}
	mlpsHead := [16]uint8{127, 126, 77, 76, 77, 76, 75, 74,
		75, 74, 75, 74, 73, 72, 73, 72}
	for i, want := range mlpsHead {
		if cabacMLPSState[i] != want {
			t.Fatalf("mlps[%d] = %d want %d", i, cabacMLPSState[i], want)
		}
	}
	// Init (m,n) heads: I and PB-idc0 share contexts 0..10.
	initHead := [7][2]int8{{20, -15}, {2, 54}, {3, 74}, {20, -15},
		{2, 54}, {3, 74}, {-28, 127}}
	for i, want := range initHead {
		if cabacInitI[2*i] != want[0] || cabacInitI[2*i+1] != want[1] {
			t.Fatalf("initI[%d] = (%d,%d) want (%d,%d)",
				i, cabacInitI[2*i], cabacInitI[2*i+1], want[0], want[1])
		}
		if cabacInitPB[2*i] != want[0] || cabacInitPB[2*i+1] != want[1] {
			t.Fatalf("initPB0[%d] = (%d,%d) want (%d,%d)",
				i, cabacInitPB[2*i], cabacInitPB[2*i+1], want[0], want[1])
		}
	}
	// PB-idc0 contexts 11..12 diverge from I (I leaves them unused).
	if cabacInitPB[2*11] != 23 || cabacInitPB[2*11+1] != 33 ||
		cabacInitPB[2*12] != 23 || cabacInitPB[2*12+1] != 2 {
		t.Fatalf("initPB0[11..12] = (%d,%d)/(%d,%d) want (23,33)/(23,2)",
			cabacInitPB[22], cabacInitPB[23], cabacInitPB[24], cabacInitPB[25])
	}
	if cabacInitI[2*11] != 0 || cabacInitI[2*11+1] != 0 {
		t.Fatalf("initI[11] = (%d,%d) want unused (0,0)",
			cabacInitI[22], cabacInitI[23])
	}
}

// TestCabacInitSpots pins the (m,n,qp) init formula on hand-computed
// cases: pre = 2*(((m*qp)>>4)+n)-127, folded by pre^=pre>>31 (bitwise
// NOT for negatives, matching the reference line for line), clamped to
// 124+(pre&1), packed as state = pre with bit0 = MPS.
func TestCabacInitSpots(t *testing.T) {
	var st [1024]uint8
	// Gate clip frame 0 runs QP 23: ctx0 (20,-15) -> 2*(28-15)-127 =
	// -101, folded to ~(-101) = 100.
	initCabacCtx(&st, cabacInitI[:], 23)
	if st[0] != 100 {
		t.Fatalf("init ctx0 qp23 = %d want 100", st[0])
	}
	// Same slice, ctx6 (-28,127): ((-28*23)>>4) = -41, |2*86-127| = 45.
	if st[6] != 45 {
		t.Fatalf("init ctx6 qp23 = %d want 45", st[6])
	}
	// Gate clip frame 1 runs QP 23 on PB-idc0: ctx11 (23,33) -> 2*66-127 = 5.
	initCabacCtx(&st, cabacInitPB[:2048], 23)
	if st[11] != 5 {
		t.Fatalf("init PB ctx11 qp23 = %d want 5", st[11])
	}
	// Clamp arms with a full-range table: (0,127) at qp51 gives
	// 2*127-127 = 127 -> clamp 124+1 = 125.
	big := make([]int8, 2048)
	big[0], big[1] = 0, 127
	big[2], big[3] = 0, 127
	initCabacCtx(&st, big, 51)
	// (0,127) at qp51: 2*127-127 = 127 -> clamp 124+1 = 125.
	if st[0] != 125 || st[1] != 125 {
		t.Fatalf("init clamp = %d/%d want 125/125", st[0], st[1])
	}
}

// TestCabacEngineGolden decodes a fixed byte string with real init
// states: exact-output regression for the arithmetic core (bins plus
// the evolved states catch engine- and table-side drift alike).
func TestCabacEngineGolden(t *testing.T) {
	raw := []byte{0x9B, 0x74, 0x2D, 0xC1, 0x55, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00}
	cab, err := newCabacDec(raw)
	if err != nil {
		t.Fatal(err)
	}
	var st [1024]uint8
	initCabacCtx(&st, cabacInitI[:], 23)
	var bins []int
	for i := 0; i < 12; i++ {
		bins = append(bins, cab.bin(&st[i]))
	}
	want := []int{0, 0, 0, 0, 1, 1, 1, 0, 1, 0, 0, 0}
	wantSt := []uint8{102, 16, 23, 102, 10, 31, 47, 9, 30, 24, 6, 124}
	for i := range want {
		if bins[i] != want[i] {
			t.Fatalf("bin %d = %d want %d (bins=%v)", i, bins[i], want[i], bins)
		}
		if st[i] != wantSt[i] {
			t.Fatalf("state %d = %d want %d", i, st[i], wantSt[i])
		}
	}
}
