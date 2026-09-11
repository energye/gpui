package h264

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// firstSliceNALUs opens an MP4 through the real demux path and returns the
// first maxSlices slice NALUs (types 1/5) plus their parameter sets.
func firstSliceNALUs(t *testing.T, path string, maxSlices int) (*ParamSets, *AVCC, [][]byte) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("test clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(path)
	if err != nil {
		t.Fatalf("demux %s: %v", path, err)
	}
	v := movie.Video
	if v == nil {
		t.Fatalf("no video track in %s", path)
	}
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	ps := NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		t.Fatalf("param sets: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	var slices [][]byte
	for _, s := range v.Samples {
		if len(slices) >= maxSlices {
			break
		}
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", s.Number, err)
		}
		for _, u := range units {
			if _, err := ps.AddNALU(u); err != nil {
				t.Fatalf("in-band param: %v", err)
			}
			if typ, _ := NALType(u); typ == NALSliceNonIDR || typ == NALSliceIDR {
				slices = append(slices, u)
				if len(slices) >= maxSlices {
					break
				}
			}
		}
	}
	if len(slices) == 0 {
		t.Fatalf("no slices in %s", path)
	}
	return ps, avcc, slices
}

func activeSets(t *testing.T, ps *ParamSets) (*PPS, *SPS) {
	t.Helper()
	q, ok := ps.PPS[0]
	if !ok {
		for _, q = range ps.PPS {
			break
		}
	}
	s, ok := ps.SPS[q.SPSID]
	if !ok {
		t.Fatalf("sps %d missing", q.SPSID)
	}
	return q, s
}

// Oracle: ffmpeg trace_headers on vr2_b_intra.mp4 first IDR plus
// ffprobe (Constrained Baseline, level 1.0, 96x96).
func TestSliceHeaderIntraClip(t *testing.T) {
	ps, _, slices := firstSliceNALUs(t, "../testdata/vr2_b_intra.mp4", 1)
	q, s := activeSets(t, ps)
	h, err := ParseSliceHeader(slices[0], q, s)
	if err != nil {
		t.Fatalf("slice header: %v", err)
	}
	if h.FirstMB != 0 {
		t.Fatalf("first mb = %d", h.FirstMB)
	}
	if !h.IsI() || h.Type != SliceI {
		t.Fatalf("type = %d", h.Type)
	}
	if !h.IsIDR {
		t.Fatal("first slice of intra clip should be IDR")
	}
	if h.FrameNum != 0 {
		t.Fatalf("frame num = %d", h.FrameNum)
	}
	if h.IDRPicID != 0 {
		t.Fatalf("idr pic id = %d", h.IDRPicID)
	}
	if h.QPDelta != 2 {
		t.Fatalf("qp delta = %d", h.QPDelta)
	}
	if h.DisableFilter != 0 {
		t.Fatalf("disable filter = %d", h.DisableFilter)
	}
}

// Oracle: real High CABAC phone-style clip exercises weight-table skip,
// CABAC init, marking and POC paths.
func TestSliceHeaderRealClip(t *testing.T) {
	ps, _, slices := firstSliceNALUs(t, "/home/yanghy/视频/ENERGY Designer-Tab.mp4", 4)
	q, s := activeSets(t, ps)
	if s.ProfileIDC != 100 {
		t.Fatalf("profile = %d", s.ProfileIDC)
	}
	seenIDR := false
	var parsed []*SliceHeader
	for i, raw := range slices {
		h, err := ParseSliceHeader(raw, q, s)
		if err != nil {
			t.Fatalf("slice %d: %v", i, err)
		}
		parsed = append(parsed, h)
		if h.IsIDR {
			seenIDR = true
			if !h.IsI() {
				t.Fatalf("slice %d: idr not intra", i)
			}
		}
		if h.RefL0Count == 0 && (h.IsP() || h.IsB()) {
			t.Fatalf("slice %d: no refs", i)
		}
	}
	if !seenIDR {
		t.Fatal("no IDR in first slices")
	}
	// Trace oracle on the same file: slice1 is P (raw type 5, qp 2),
	// slice3 is a disposable B (raw type 6, nal_ref_idc 0, no marking).
	if !parsed[1].IsP() {
		t.Fatalf("slice1 type = %d want P", parsed[1].Type)
	}
	if parsed[1].QPDelta != 2 {
		t.Fatalf("slice1 qp = %d want 2", parsed[1].QPDelta)
	}
	if !parsed[3].IsB() || parsed[3].NalRefIDC != 0 {
		t.Fatalf("slice3 = type %d ref %d want B/0", parsed[3].Type, parsed[3].NalRefIDC)
	}
	if len(parsed[3].MMCO) != 0 || parsed[3].AdaptiveMarking {
		t.Fatal("disposable picture must carry no marking ops")
	}
}

func TestSliceHeaderMixedClip(t *testing.T) {
	ps, _, slices := firstSliceNALUs(t, "../testdata/vr2_b_mixed.mp4", 6)
	q, s := activeSets(t, ps)
	seenP := false
	for i, raw := range slices {
		h, err := ParseSliceHeader(raw, q, s)
		if err != nil {
			t.Fatalf("slice %d: %v", i, err)
		}
		if h.IsP() {
			seenP = true
		}
	}
	if !seenP {
		t.Fatal("mixed clip should contain P slices")
	}
}

func TestSliceHeaderBad(t *testing.T) {
	ps := NewParamSets()
	if err := ps.AddSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 5, hMap: 5})); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddPPS(buildPPS(ppsOpt{})); err != nil {
		t.Fatal(err)
	}
	q, s := activeSets(t, ps)
	wrongPPS := buildSlice(NALSliceIDR, 3, 0)
	wrongPPS[1] ^= 0xFF
	if _, err := ParseSliceHeader(wrongPPS, q, s); err == nil {
		t.Fatal("wrong pps id should fail")
	}
	if _, err := ParseSliceHeader([]byte{0x65}, q, s); err == nil {
		t.Fatal("truncated should fail")
	}
	if _, err := ParseSliceHeader(buildSlice(NALSliceIDR, 3, 0), nil, s); err == nil {
		t.Fatal("nil sets should fail")
	}
}

func TestDPB(t *testing.T) {
	d := NewDPB(2)
	if d.Len() != 0 {
		t.Fatal("empty dpb")
	}
	a, _ := NewPicture(16, 16)
	a.POC = 0
	b, _ := NewPicture(16, 16)
	b.POC = 2
	c, _ := NewPicture(16, 16)
	c.POC = 4
	c.IsIDR = true
	d.Store(a)
	d.Store(b)
	if d.Len() != 2 {
		t.Fatalf("len = %d", d.Len())
	}
	d.Store(c)
	if d.Len() != 1 || d.ByPOC(4) == nil {
		t.Fatal("idr should flush")
	}
	if _, err := NewPicture(0, 16); err == nil {
		t.Fatal("zero width should fail")
	}
}
