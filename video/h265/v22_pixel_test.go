package h265

// V2-2 step 4: pre-filter pixel gate. The IDR frame reconstructs
// (dequant + inverse transform + intra prediction + add, decode order,
// no loop filters yet) byte-exact against the reference decoder with
// its loop filters skipped (thread_count=1, first frame).
//
// Pinned truth: 96x96 yuv420p md5 e5697068db00e21334f57c9560ac0373
// (peer pre-filter dump; post-filter baseline 6e2dab65094d0fa8f8945cacfc73ee93
// stays the p3 deblock/SAO target). Three pixel bugs died for this,
// all found by diffing against peer dumps (ideas only, no code copied):
//   1. residual levels attached in scan order; the peer attaches
//      greater1/remaining/signs last-first (values landed mirrored);
//   2. angular reads sat one sample right (the peer's ref pointer
//      already carries the -1 offset);
//   3. intra luma 4x4 always rides the DST path, even DC-only blocks
//      (the peer checks it before the DC fast path);
//   4. negative-angle extension filled one slot too high and clobbered
//      the corner sample the first pixels read.

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func TestV22PrefilterExact(t *testing.T) {
	ps, nalu := loadIDR(t)
	fs, err := ParseFrameSyntax(nalu, ps, 0)
	if err != nil {
		t.Fatalf("idr: %v", err)
	}
	if len(fs.Leaves) != 246 {
		t.Fatalf("leaves %d, want 246", len(fs.Leaves))
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
	out := append(append(append([]byte{}, pic.Y...), pic.Cb...), pic.Cr...)
	if len(out) != 96*96*3/2 {
		t.Fatalf("yuv %d bytes, want %d", len(out), 96*96*3/2)
	}
	sum := md5.Sum(out)
	if got := hex.EncodeToString(sum[:]); got != "e5697068db00e21334f57c9560ac0373" {
		t.Fatalf("prefilter md5 %s, want e5697068db00e21334f57c9560ac0373", got)
	}
}
