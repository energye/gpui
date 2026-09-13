package h264

import (
	"errors"
	"strings"
	"testing"
)

// VR2d-4 interlace (F12) gate: field pictures and MBAFF frames are
// recognized at parse time and refused with a readable message instead
// of decoding silently wrong. Field decoding itself stays out of scope
// per the rare-format rule (name the tool, no crash, VR6 case留档 —
// this file is the留档).

// buildFieldSlice crafts a minimal IDR I-slice matching the default
// buildPPS (CAVLC, deblocking present) and poc-type-0 buildSPS, with the
// field_pic_flag/bottom bits set as asked. It only needs to parse far
// enough to reach the decoder interlace gate.
func buildFieldSlice(fieldPic, bottom bool) []byte {
	w := &writer{}
	w.ue(0) // first_mb
	w.ue(2) // slice_type I
	w.ue(0) // pps_id
	w.bits(0, 4)
	if fieldPic {
		w.bit(1)
		w.bit(boolBit(bottom))
	} else {
		w.bit(0)
	}
	w.ue(0) // idr_pic_id
	w.bits(0, 4)
	w.bit(0) // no_output_prior
	w.bit(0) // long_term_ref
	w.se(0)  // qp_delta
	w.ue(0)  // disable_filter_idc
	w.se(0)  // alpha
	w.se(0)  // beta
	return append([]byte{0x65}, w.flush()...)
}

// interlaceDecoder returns a decoder fed with an interlaced SPS plus the
// default PPS, keeping the parsed sets for header checks.
func interlaceDecoder(t *testing.T, mbaff bool) (*Decoder, *PPS, *SPS) {
	t.Helper()
	spsRaw := buildSPS(spsOpt{profile: 66, level: 30, wMBs: 5, hMap: 5, interlaced: true, mbaff: mbaff})
	ppsRaw := buildPPS(ppsOpt{})
	dec := NewDecoder(nil)
	if err := dec.DecodeNALU(spsRaw); err != nil {
		t.Fatalf("sps: %v", err)
	}
	if err := dec.DecodeNALU(ppsRaw); err != nil {
		t.Fatalf("pps: %v", err)
	}
	q, s, err := dec.ps.RequireForSlice(0)
	if err != nil {
		t.Fatalf("sets: %v", err)
	}
	return dec, q, s
}

func TestInterlaceSPSDetected(t *testing.T) {
	s, err := ParseSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 5, hMap: 5, interlaced: true, mbaff: true}))
	if err != nil {
		t.Fatalf("sps: %v", err)
	}
	if s.FrameMBsOnly || !s.Interlaced || !s.MBAFF {
		t.Fatalf("flags = frameOnly %v interlaced %v mbaff %v, want false/true/true",
			s.FrameMBsOnly, s.Interlaced, s.MBAFF)
	}
	s, err = ParseSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 5, hMap: 5, interlaced: true}))
	if err != nil {
		t.Fatalf("sps: %v", err)
	}
	if s.FrameMBsOnly || !s.Interlaced || s.MBAFF {
		t.Fatalf("flags = frameOnly %v interlaced %v mbaff %v, want false/true/false",
			s.FrameMBsOnly, s.Interlaced, s.MBAFF)
	}
	// Progressive clips keep parsing exactly as before.
	s, err = ParseSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 5, hMap: 5}))
	if err != nil {
		t.Fatalf("sps: %v", err)
	}
	if !s.FrameMBsOnly || s.Interlaced || s.MBAFF {
		t.Fatalf("flags = frameOnly %v interlaced %v mbaff %v, want true/false/false",
			s.FrameMBsOnly, s.Interlaced, s.MBAFF)
	}
}

func TestInterlaceFieldRejected(t *testing.T) {
	dec, q, s := interlaceDecoder(t, false)
	for _, bottom := range []bool{false, true} {
		h, _, err := ParseSliceHeader(buildFieldSlice(true, bottom), q, s)
		if err != nil {
			t.Fatalf("bottom=%v header: %v", bottom, err)
		}
		if !h.FieldPic || h.BottomField != bottom {
			t.Fatalf("bottom=%v flags = field %v bottom %v", bottom, h.FieldPic, h.BottomField)
		}
		err = dec.DecodeNALU(buildFieldSlice(true, bottom))
		if !errors.Is(err, ErrStageScope) {
			t.Fatalf("bottom=%v err = %v, want stage scope", bottom, err)
		}
		if !strings.Contains(err.Error(), "F12") || !strings.Contains(err.Error(), "field") {
			t.Fatalf("bottom=%v err names no tool: %v", bottom, err)
		}
	}
}

func TestInterlaceMBAFFFrameRejected(t *testing.T) {
	dec, q, s := interlaceDecoder(t, true)
	h, _, err := ParseSliceHeader(buildFieldSlice(false, false), q, s)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if h.FieldPic {
		t.Fatal("frame slice parsed as field")
	}
	err = dec.DecodeNALU(buildFieldSlice(false, false))
	if !errors.Is(err, ErrStageScope) {
		t.Fatalf("err = %v, want stage scope", err)
	}
	if !strings.Contains(err.Error(), "F12") || !strings.Contains(err.Error(), "MBAFF") {
		t.Fatalf("err names no tool: %v", err)
	}
}

func TestInterlaceSequenceFrameRejected(t *testing.T) {
	dec, _, _ := interlaceDecoder(t, false)
	err := dec.DecodeNALU(buildFieldSlice(false, false))
	if !errors.Is(err, ErrStageScope) {
		t.Fatalf("err = %v, want stage scope", err)
	}
	if !strings.Contains(err.Error(), "F12") {
		t.Fatalf("err names no tool: %v", err)
	}
}
