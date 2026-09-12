package h264

import "testing"

// Scaling-list gate (F9, first half): defaults transcription, explicit
// delta parsing (order + default escape) and picture/sequence fallback
// chains. Pixel-exact High clips are the second half.

// TestScalingDefaults pins the JVT default heads (raster order).
func TestScalingDefaults(t *testing.T) {
	if [4]uint8(defaultScaling4[0][:4]) != [4]uint8{6, 13, 20, 28} {
		t.Fatalf("jvt4 intra head = %v", defaultScaling4[0][:4])
	}
	if [4]uint8(defaultScaling4[1][:4]) != [4]uint8{10, 14, 20, 24} {
		t.Fatalf("jvt4 inter head = %v", defaultScaling4[1][:4])
	}
	if [8]uint8(defaultScaling8[0][:8]) != [8]uint8{6, 10, 13, 16, 18, 23, 25, 27} {
		t.Fatalf("jvt8 intra head = %v", defaultScaling8[0][:8])
	}
	if [8]uint8(defaultScaling8[1][:8]) != [8]uint8{9, 13, 15, 17, 19, 21, 22, 24} {
		t.Fatalf("jvt8 inter head = %v", defaultScaling8[1][:8])
	}
}

// TestScalingListExplicit decodes a hand-built scaling_list(): values 9
// everywhere except raster position 5 (scan index 4), which reads 14
// then returns to 9. Bits: present + SE(1) + SE(0)x3 + SE(5) + SE(-5)
// + SE(0)x10 + stop = AE 28 5F FF.
func TestScalingListExplicit(t *testing.T) {
	r := NewReader([]byte{0xAE, 0x28, 0x5F, 0xFF})
	present, err := r.ReadBits(1)
	if err != nil || present != 1 {
		t.Fatalf("present = %d,%v", present, err)
	}
	got, err := parseScalingList(r, 16, defaultScaling4[0][:])
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got {
		want := uint8(9)
		if i == 5 {
			want = 14
		}
		if v != want {
			t.Fatalf("pos %d = %d want %d (all=%v)", i, v, want, got)
		}
	}
}

// TestScalingList8x8Order decodes a scan-order ramp (8,9,...,71) and
// pins the zigzag unscrambling: raster values must equal 8 plus the
// scan index of each position. Bits: present + SE(0) + SE(1)x63 + stop.
func TestScalingList8x8Order(t *testing.T) {
	raw := []byte{0xD2, 0x49, 0x24, 0x92, 0x49, 0x24, 0x92, 0x49,
		0x24, 0x92, 0x49, 0x24, 0x92, 0x49, 0x24, 0x92,
		0x49, 0x24, 0x92, 0x49, 0x24, 0x92, 0x49, 0x25}
	r := NewReader(raw)
	present, err := r.ReadBits(1)
	if err != nil || present != 1 {
		t.Fatalf("present = %d,%v", present, err)
	}
	got, err := parseScalingList(r, 64, defaultScaling8[0][:])
	if err != nil {
		t.Fatal(err)
	}
	want := [64]uint8{
		8, 9, 13, 14, 22, 23, 35, 36,
		10, 12, 15, 21, 24, 34, 37, 50,
		11, 16, 20, 25, 33, 38, 49, 51,
		17, 19, 26, 32, 39, 48, 52, 61,
		18, 27, 31, 40, 47, 53, 60, 62,
		28, 30, 41, 46, 54, 59, 63, 68,
		29, 42, 45, 55, 58, 64, 67, 69,
		43, 44, 56, 57, 65, 66, 70, 71,
	}
	for i, v := range got {
		if v != want[i] {
			t.Fatalf("pos %d = %d want %d", i, v, want[i])
		}
	}
}
// (next hits 0 at scan 0) resolves to the JVT list with no more data.
// Bits: present + SE(-8) + stop = 84 60.
func TestScalingListEscape(t *testing.T) {
	r := NewReader([]byte{0x84, 0x60})
	present, err := r.ReadBits(1)
	if err != nil || present != 1 {
		t.Fatalf("present = %d,%v", present, err)
	}
	got, err := parseScalingList(r, 16, defaultScaling4[0][:])
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range got {
		if v != defaultScaling4[0][i] {
			t.Fatalf("pos %d = %d want jvt %d", i, v, defaultScaling4[0][i])
		}
	}
}

// TestResolveScaling pins the fallback chains on synthetic sets.
func TestResolveScaling(t *testing.T) {
	flat := [16]uint8{16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16}
	var sc4 [6][16]uint8
	var sc8 [2][64]uint8

	// No matrices anywhere: flat.
	resolveScaling(&PPS{}, &SPS{}, &sc4, &sc8)
	if sc4[0] != flat || sc4[5] != flat {
		t.Fatalf("flat resolve = %v / %v", sc4[0][:4], sc4[5][:4])
	}
	for j := 0; j < 2; j++ {
		for i, v := range sc8[j] {
			if v != 16 {
				t.Fatalf("flat8[%d][%d] = %d", j, i, v)
			}
		}
	}

	// Picture matrix present, nothing explicit: JVT everywhere.
	resolveScaling(&PPS{Scaling: scalingRaw{present: true}}, &SPS{}, &sc4, &sc8)
	if [4]uint8(sc4[0][:4]) != [4]uint8{6, 13, 20, 28} {
		t.Fatalf("jvt0 = %v", sc4[0][:4])
	}
	if [4]uint8(sc4[3][:4]) != [4]uint8{10, 14, 20, 24} {
		t.Fatalf("jvt3 = %v", sc4[3][:4])
	}
	if [4]uint8(sc8[0][:4]) != [4]uint8{6, 10, 13, 16} || [4]uint8(sc8[1][:4]) != [4]uint8{9, 13, 15, 17} {
		t.Fatalf("jvt8 = %v / %v", sc8[0][:4], sc8[1][:4])
	}

	// Explicit luma lists chain into chroma (1<-0, 2<-1, 4<-3, 5<-4).
	nine := [16]uint8{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}
	seven := [16]uint8{7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7}
	pps := &PPS{Scaling: scalingRaw{present: true, m4: 0x09}}
	pps.Scaling.l4[0] = nine
	pps.Scaling.l4[3] = seven
	resolveScaling(pps, &SPS{}, &sc4, &sc8)
	for _, i := range []int{0, 1, 2} {
		if sc4[i] != nine {
			t.Fatalf("chain sc4[%d] = %v want 9s", i, sc4[i][:4])
		}
	}
	for _, i := range []int{3, 4, 5} {
		if sc4[i] != seven {
			t.Fatalf("chain sc4[%d] = %v want 7s", i, sc4[i][:4])
		}
	}

	// Picture matrix absent: sequence lists ride through untouched
	// (no chroma chaining); absent sequence lists mean JVT here.
	five := [16]uint8{5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}
	sps := &SPS{Scaling: scalingRaw{present: true, m4: 0x01}}
	sps.Scaling.l4[0] = five
	resolveScaling(&PPS{}, sps, &sc4, &sc8)
	if sc4[0] != five {
		t.Fatalf("sps ride sc4[0] = %v want 5s", sc4[0][:4])
	}
	if [4]uint8(sc4[1][:4]) != [4]uint8{6, 13, 20, 28} {
		t.Fatalf("sps absent sc4[1] = %v want jvt", sc4[1][:4])
	}
}
