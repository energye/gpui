package h265

// V2-2 step P3c: five-frame sequence gate. The IDR plus four P frames
// decode end to end (reconstruct + deblock + SAO, filtered pictures
// as references, decode order) byte-exact against the reference
// decoder's frame dumps.
//
// Pinned truth (96x96 yuv420p, from the peer's whole-stream dump):
// f0 6e2dab65094d0fa8f8945cacfc73ee93 (IDR) + f1 86a0961d63cd7b02e34802086ddbe95e
// + f2 b2e15b44c331abe0f4ab8bab44b8996d + f3 6d7aa1700181d6ac52cc7b4b8de8c3e7
// + f4 3add0a159ace42564a767d30d9070fd0. The last frame fell here:
// its deblock QP table used running QP deltas with slice-QP backing
// for skip cells, while the peer predicts each QP group from left and
// above neighbours (ideas only, no code copied). CU QP tab fill fixed it.

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

func TestV22PSequenceExact(t *testing.T) {
	want := []string{
		"6e2dab65094d0fa8f8945cacfc73ee93",
		"86a0961d63cd7b02e34802086ddbe95e",
		"b2e15b44c331abe0f4ab8bab44b8996d",
		"6d7aa1700181d6ac52cc7b4b8de8c3e7",
		"3add0a159ace42564a767d30d9070fd0",
	}
	ps, m, h, f := loadPFrames(t)
	defer f.Close()
	sps := ps.SPS[0]
	var pps *PPS
	for _, q := range ps.PPS {
		pps = q
		break
	}
	dec, err := newInterDec(sps, pps)
	if err != nil {
		t.Fatalf("inter: %v", err)
	}
	poc := 0
	for si := 0; si <= 4; si++ {
		s := m.Video.Samples[si]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("S%d sample: %v", si, err)
		}
		u, err := SplitHVCC(buf, h.LengthSize)
		if err != nil {
			t.Fatalf("S%d split: %v", si, err)
		}
		fs, err := ParseFrameSyntax(u[0], ps, poc)
		if err != nil {
			t.Fatalf("S%d syntax: %v", si, err)
		}
		sh := fs.SH
		poc = sh.POC
		var pic *Picture
		if sh.Type == SliceI {
			pic, err = Reconstruct(fs, sps)
			if err != nil {
				t.Fatalf("S%d reconstruct: %v", si, err)
			}
			if err := LoopFilter(pic, fs, sps, pps); err != nil {
				t.Fatalf("S%d loop filter: %v", si, err)
			}
			grid := &mvGrid{w: dec.minPUW, h: dec.minPUH, f: make([]mvCand, dec.minPUW*dec.minPUH)}
			rpl := []refPic{{poc: sh.POC}}
			dec.pics[sh.POC] = &picState{pic: pic, poc: sh.POC, rpl0: rpl, grid: grid}
			dec.order = append(dec.order, sh.POC)
		} else {
			rpl, err := dec.buildRPL(sh)
			if err != nil {
				t.Fatalf("S%d rpl: %v", si, err)
			}
			colPOC := rpl[0].poc
			if int(sh.CollocatedRefIdx) < len(rpl) {
				colPOC = rpl[sh.CollocatedRefIdx].poc
			}
			col := dec.pics[colPOC]
			grid := &mvGrid{w: dec.minPUW, h: dec.minPUH, f: make([]mvCand, dec.minPUW*dec.minPUH)}
			mots, err := dec.deriveFrame(fs, sh, rpl, col, grid)
			if err != nil {
				t.Fatalf("S%d derive: %v", si, err)
			}
			pic, err = dec.reconInter(fs, mots, rpl, sh.SliceQP)
			if err != nil {
				t.Fatalf("S%d recon: %v", si, err)
			}
			if err := dec.FilterP(pic, fs, pps, grid, rpl); err != nil {
				t.Fatalf("S%d filter: %v", si, err)
			}
			dec.pics[sh.POC] = &picState{pic: pic, poc: sh.POC, rpl0: rpl, grid: grid}
			dec.order = append(dec.order, sh.POC)
		}
		out := append(append(append([]byte{}, pic.Y...), pic.Cb...), pic.Cr...)
		sum := md5.Sum(out)
		if got := hex.EncodeToString(sum[:]); got != want[si] {
			t.Fatalf("S%d md5 %s, want %s", si, got, want[si])
		}
	}
}
