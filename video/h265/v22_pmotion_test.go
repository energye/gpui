package h265

// P-frame motion gate (V2-2 step P3b): derive every PU's final vector
// from the parsed syntax and diff it against the reference decoder's
// MVFINAL truth (per-PU pf/ref/mv).
//
// Numbers below were verified PU-by-PU against a pristine ffmpeg build
// (hevcdec.c hls_prediction_unit + mvs.c merge/MVP derivation, ideas
// only, no code copied). The clip is single-reference P (L0 only,
// colocated = L0[0] of the previous picture, no weights on wire), so
// every pf is L0 and every ref index is 0.
//
// The truth's shape does the proving: 57/51/51/51 PUs against the
// syntax gate's 57/57 51/51 51/51 51/51 CU/PU counts, all 210 vectors
// exact. Two motion paths fire here: merge PUs inherit the y72-80
// strip flow (e.g. (32,72) = (13,0) with zero residual bins), AMVP PUs
// add the parsed MVD onto the predictor (e.g. (0,72) = (13,4)).
// One-PU coverage is honest: this clip's 210/210 PUs are single-PU
// CUs, so multi-PU corner gates stay refused, not silently passed.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

type pmvTruth struct {
	X, Y     int
	W, H     int
	PF       int
	Ref      int
	MVX, MVY int32
}

func loadPMVTruth(t *testing.T) map[string][]pmvTruth {
	t.Helper()
	p := filepath.Join("..", "testdata", "v2_pmv.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Skipf("pmv truth: %v", err)
	}
	var m map[string][]pmvTruth
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("pmv json: %v", err)
	}
	return m
}

func loadPFrames(t *testing.T) (*ParamSets, *mp4.Movie, *HVCC, *os.File) {
	t.Helper()
	b := loadV2Baseline(t)
	path := filepath.Join("..", "testdata", b.Clips[0].File)
	m, err := mp4.ParseFile(path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	h, err := ParseHVCC(m.Video.HEVCConfig)
	if err != nil {
		t.Fatalf("hvcC: %v", err)
	}
	ps := NewParamSets()
	if err := ps.FromHVCC(h); err != nil {
		t.Fatalf("param sets: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return ps, m, h, f
}

// TestV22PMotionExact derives all 210 P-frame PU vectors and checks
// every pf/ref/mv against the reference MVFINAL lines.
func TestV22PMotionExact(t *testing.T) {
	truth := loadPMVTruth(t)
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
	var prev []refPic
	var prevGrid *mvGrid
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
		if sh.Type == SliceI {
			grid := &mvGrid{w: dec.minPUW, h: dec.minPUH, f: make([]mvCand, dec.minPUW*dec.minPUH)}
			rpl := []refPic{{poc: sh.POC}}
			dec.pics[sh.POC] = &picState{poc: sh.POC, rpl0: rpl, grid: grid}
			dec.order = append(dec.order, sh.POC)
			prev = rpl
			prevGrid = grid
			continue
		}
		rpl, err := dec.buildRPL(sh)
		if err != nil {
			t.Fatalf("S%d rpl: %v", si, err)
		}
		for _, rp := range rpl {
			if _, ok := dec.pics[rp.poc]; !ok {
				t.Fatalf("S%d ref POC %d missing", si, rp.poc)
			}
		}
		colPOC := rpl[0].poc
		if int(sh.CollocatedRefIdx) < len(rpl) {
			colPOC = rpl[sh.CollocatedRefIdx].poc
		}
		col := dec.pics[colPOC]
		grid := &mvGrid{w: dec.minPUW, h: dec.minPUH, f: make([]mvCand, dec.minPUW*dec.minPUH)}
		mots, err := dec.deriveFrame(fs, sh, rpl, nil, col, nil, grid)
		if err != nil {
			t.Fatalf("S%d derive: %v", si, err)
		}
		key := fmt.Sprint(si + 1)
		want := truth[key]
		if len(mots) != len(want) {
			t.Fatalf("S%d PUs %d, want %d", si, len(mots), len(want))
		}
		for i, mo := range mots {
			w := want[i]
			if mo.x0 != w.X || mo.y0 != w.Y || mo.w != w.W || mo.h != w.H ||
				mo.mv.pred != w.PF || mo.mv.ref0 != w.Ref ||
				mo.mv.x0 != w.MVX || mo.mv.y0 != w.MVY {
				t.Fatalf("S%d PU%d (%d,%d %dx%d) pf=%d ref=%d mv=%d,%d, want (%d,%d %dx%d) pf=%d ref=%d mv=%d,%d",
					si, i, mo.x0, mo.y0, mo.w, mo.h, mo.mv.pred, mo.mv.ref0, mo.mv.x0, mo.mv.y0,
					w.X, w.Y, w.W, w.H, w.PF, w.Ref, w.MVX, w.MVY)
			}
		}
		dec.pics[sh.POC] = &picState{poc: sh.POC, rpl0: rpl, grid: grid}
		dec.order = append(dec.order, sh.POC)
		prev = rpl
		prevGrid = grid
	}
	_ = prev
	_ = prevGrid
}
