package h264

import (
	"os"
	"strings"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// TestBRefLists pins the B-slice reference tables of the VR2d gate clip
// (vr2_m_bframes, decode order I0/P6/B2/B4/P8, display POC 0/2/4/6/8).
// Lists are checked by display order; macroblock payload still stops at
// the VR2d stage gate.
func TestBRefLists(t *testing.T) {
	mp4Path := "../testdata/vr2_m_bframes.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	sampleUnits := func(i int) [][]byte {
		t.Helper()
		s := v.Samples[i]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", i, err)
		}
		return units
	}
	sliceOf := func(i int) []byte {
		t.Helper()
		for _, u := range sampleUnits(i) {
			if typ, _ := NALType(u); typ == NALSliceNonIDR || typ == NALSliceIDR {
				return u
			}
		}
		t.Fatalf("sample %d: no slice", i)
		return nil
	}

	dec := NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			t.Fatalf("sps: %v", err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			t.Fatalf("pps: %v", err)
		}
	}
	pps, sps, err := dec.ps.RequireForSlice(0)
	if err != nil {
		t.Fatalf("sets: %v", err)
	}
	// Samples 0 (IDR) and 1 (P) decode end to end, filling the buffer.
	for _, i := range []int{0, 1} {
		for _, u := range sampleUnits(i) {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("sample %d: %v", i, err)
			}
		}
		if _, err := dec.FinishPicture(); err != nil {
			t.Fatalf("finish %d: %v", i, err)
		}
	}
	pocs := func(list []*Picture) []int32 {
		var out []int32
		for _, p := range list {
			if p == nil {
				out = append(out, -1)
				continue
			}
			out = append(out, p.POC)
		}
		return out
	}
	eq := func(tag string, got []int32, want ...int32) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s length = %v, want %v", tag, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s = %v, want %v", tag, got, want)
			}
		}
	}

	// Sample 2 (B poc2): past [I0], future [P6].
	h2, _, err := ParseSliceHeader(sliceOf(2), pps, sps)
	if err != nil {
		t.Fatalf("b2 header: %v", err)
	}
	if !h2.IsB() || h2.POC != 2 {
		t.Fatalf("b2 = type %d poc %d, want B poc 2", h2.Type, h2.POC)
	}
	dec.fixPOC(h2, sps)
	l0, l1, err := dec.buildRefListsB(h2)
	if err != nil {
		t.Fatalf("b2 lists: %v", err)
	}
	eq("b2 l0", pocs(l0), 0)
	eq("b2 l1", pocs(l1), 6)

	// Payload still stops readable at the macroblock stage.
	if err := dec.DecodeNALU(sliceOf(2)); err == nil || !strings.Contains(err.Error(), "macroblocks") {
		t.Fatalf("b2 payload err = %v, want VR2d macroblocks gate", err)
	}

	// Sample 3 (B poc4, non-reference): buffer also holds the B2 ref.
	dec.dpb.Store(&Picture{Width: 96, Height: 96, Y: make([]uint8, 96*96),
		Cb: make([]uint8, 48*48), Cr: make([]uint8, 48*48),
		FrameNum: 2, POC: 2}, true)
	h3, _, err := ParseSliceHeader(sliceOf(3), pps, sps)
	if err != nil {
		t.Fatalf("b3 header: %v", err)
	}
	dec.fixPOC(h3, sps)
	l0, l1, err = dec.buildRefListsB(h3)
	if err != nil {
		t.Fatalf("b3 lists: %v", err)
	}
	eq("b3 l0", pocs(l0), 2, 0)
	eq("b3 l1", pocs(l1), 6)
}
