package h265

// V2-2 step B3d: ten-frame B sequence gate. The closed-GOP baseline
// (single IDR + 3 P + 6 B, decode order 0/3/2/1/6/5/4/9/8/7) decodes
// end to end (reconstruct + deblock, filtered pictures as references,
// decode order) byte-exact against the reference decoder's frame dumps.
//
// Pinned truth (96x96 yuv420p, from the peer's whole-stream dump,
// display order poc 0..9):
// 7993d0cf/c3067976/50fe7700/ecb36179/2469a828/abecd202/130a5562/1e587d96/775780d1/69420010.
// Baseline: ../testdata/v2b_ffmpeg.json (§12 V2 row). The clip is
// tracked (15KB in repo); only display-order md5s assert here.
//
// Peer (read-only, ideas only, no code copied):
// hevcdec.c hls_slice_header B branch + hls_prediction_unit B branch
// + luma_mc_bi/chroma_mc_bi (tmp lane + (a+b+64)>>7 combine) +
// mvs.c bidirectional merge/MVP (X-first try order, future-ref rule)
// + refs.c frame_rps/slice_rpl (L0 bef+aft, L1 aft+bef).

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

type v2bFrame struct {
	POC   int    `json:"poc"`
	Slice string `json:"slice"`
	MD5   string `json:"md5"`
}

type v2bClip struct {
	File        string     `json:"file"`
	DecodeOrder []int      `json:"decode_order"`
	Frames      []v2bFrame `json:"frames"`
}

type v2bBaseline struct {
	Clips []v2bClip `json:"clips"`
}

func loadBFrames(t *testing.T) (*ParamSets, *mp4.Movie, *HVCC, *os.File, []v2bFrame) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "v2b_ffmpeg.json"))
	if err != nil {
		t.Fatalf("b baseline: %v", err)
	}
	var b v2bBaseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("b baseline json: %v", err)
	}
	if len(b.Clips) == 0 {
		t.Fatal("b baseline has no clips")
	}
	clip := b.Clips[0]
	path := filepath.Join("..", "testdata", clip.File)
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
	return ps, m, h, f, clip.Frames
}

// pickBColoc selects the colocated pictures for one inter frame: col
// follows the header flag (L0 or L1 entry), col1 is always L1[0] for B
// (the peer's second MVP list source).
func pickBColoc(dec *interDec, sh *SliceHeader, rpl0, rpl1 []refPic) (*picState, *picState) {
	var col, col1 *picState
	if len(rpl0) > 0 {
		idx := int(sh.CollocatedRefIdx)
		if sh.ColocFromL0 && idx < len(rpl0) {
			col = dec.pics[rpl0[idx].poc]
		} else if !sh.ColocFromL0 && idx < len(rpl1) {
			col = dec.pics[rpl1[idx].poc]
		} else {
			col = dec.pics[rpl0[0].poc]
		}
	}
	if sh.Type == SliceB && len(rpl1) > 0 {
		col1 = dec.pics[rpl1[0].poc]
	}
	return col, col1
}

func TestV22BSequenceExact(t *testing.T) {
	ps, m, h, f, frames := loadBFrames(t)
	defer f.Close()
	if len(frames) != 10 {
		t.Fatalf("b frames %d, want 10", len(frames))
	}
	want := map[int]string{}
	for _, fr := range frames {
		want[fr.POC] = fr.MD5
	}
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
	byPOC := map[int]*Picture{}
	for si := range m.Video.Samples {
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
			dec.pics[sh.POC] = &picState{pic: pic, poc: sh.POC, rpl0: nil, rpl1: nil, grid: grid}
			dec.order = append(dec.order, sh.POC)
		} else {
			rpl0, err := dec.buildRPL(sh)
			if err != nil {
				t.Fatalf("S%d rpl0: %v", si, err)
			}
			rpl1, err := dec.buildRPL1(sh)
			if err != nil {
				t.Fatalf("S%d rpl1: %v", si, err)
			}
			col, col1 := pickBColoc(dec, sh, rpl0, rpl1)
			grid := &mvGrid{w: dec.minPUW, h: dec.minPUH, f: make([]mvCand, dec.minPUW*dec.minPUH)}
			mots, err := dec.deriveFrame(fs, sh, rpl0, rpl1, col, col1, grid)
			if err != nil {
				t.Fatalf("S%d derive: %v", si, err)
			}
			pic, err = dec.reconInter(fs, mots, rpl0, rpl1)
			if err != nil {
				t.Fatalf("S%d recon: %v", si, err)
			}
			if err := dec.FilterP(pic, fs, pps, grid, rpl0, rpl1); err != nil {
				t.Fatalf("S%d filter: %v", si, err)
			}
			dec.pics[sh.POC] = &picState{pic: pic, poc: sh.POC, rpl0: rpl0, rpl1: rpl1, grid: grid}
			dec.order = append(dec.order, sh.POC)
		}
		byPOC[sh.POC] = pic
	}
	for p := 0; p < 10; p++ {
		pic := byPOC[p]
		if pic == nil {
			t.Fatalf("poc %d missing", p)
		}
		out := append(append(append([]byte{}, pic.Y...), pic.Cb...), pic.Cr...)
		sum := md5.Sum(out)
		if got := hex.EncodeToString(sum[:]); got != want[p] {
			t.Fatalf("poc %d md5 %s, want %s", p, got, want[p])
		}
	}
}
