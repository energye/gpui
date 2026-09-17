package h265

// V2-2 step 5: loop-filter gate. Reconstruct gives the pre-filter
// picture (TestV22PrefilterExact); LoopFilter applies deblocking then
// SAO in spec order, landing byte-exact on the reference decoder's
// first-frame output (thread_count=1).
//
// Pinned truth: 96x96 yuv420p md5 6e2dab65094d0fa8f8945cacfc73ee93
// (ffmpeg rawvideo dump of video/testdata/v2_h265.mp4 frame 0; the
// same bytes the v2_ffmpeg baseline was built from). Diagnosis note:
// with the reference decoder's SAO stage skipped, both sides land on
// 001f8053cf4c12a2277ea697084100c1; a future red here bisects by
// re-adding that staged comparison (deblock hook) before blaming SAO.
// Two loop bugs died for it (ideas only, no code copied):
//   1. Bs on every interior 8-grid line over-filtered inside large
//      TUs; Bs rides TU-boundary leaves only (peer sets it per TU).
//   2. a hand-copied Tc table dropped one entry, shifting every QP
//      >= 26 up one slot (qp28 read tc3, peer reads tc2).

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func reconstructIDR(t *testing.T) (*FrameSyntax, *SPS, *PPS, *Picture) {
	t.Helper()
	ps, nalu := loadIDR(t)
	fs, err := ParseFrameSyntax(nalu, ps, 0)
	if err != nil {
		t.Fatalf("idr: %v", err)
	}
	q, ok := ps.PPS[fs.SH.PPSID]
	if !ok {
		t.Fatalf("pps %d missing", fs.SH.PPSID)
	}
	s, ok := ps.SPS[q.SPSID]
	if !ok {
		t.Fatalf("sps %d missing", q.SPSID)
	}
	pic, err := Reconstruct(fs, s)
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	return fs, s, q, pic
}

func yuvMD5(pic *Picture) string {
	out := append(append(append([]byte{}, pic.Y...), pic.Cb...), pic.Cr...)
	sum := md5.Sum(out)
	return hex.EncodeToString(sum[:])
}

func TestV22LoopFilterExact(t *testing.T) {
	fs, s, q, pic := reconstructIDR(t)
	if err := LoopFilter(pic, fs, s, q); err != nil {
		t.Fatalf("loop filter: %v", err)
	}
	if len(pic.Y)+len(pic.Cb)+len(pic.Cr) != 96*96*3/2 {
		t.Fatalf("yuv %d bytes, want %d", len(pic.Y)+len(pic.Cb)+len(pic.Cr), 96*96*3/2)
	}
	if got := yuvMD5(pic); got != "6e2dab65094d0fa8f8945cacfc73ee93" {
		t.Fatalf("filtered md5 %s, want 6e2dab65094d0fa8f8945cacfc73ee93", got)
	}
}
