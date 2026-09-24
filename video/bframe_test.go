package video

// B player gate: the opt-in threaded player shows a PTS-monotonic run
// whose every pixel equals the direct sequential oracle at the same
// stamp (drops allowed — same bounded catch-up contract as S2), fires
// laps inside long groups, and stays identical with GPUI_B_OFF=1.
// Clip: testdata/vr_b_long.mp4 (96x96/Main/80 samples/2 IDR GOPs x40,
// 72% B — the long-group shape S2 cannot fan; missing file FAILs,
// never Skips).

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

type bShown struct {
	pts int64
	pix []byte
}

// bPlayAll plays name to Ended and copies every shown Pix (streaming
// Pix is recycled on the next Poll, so copy during the tick). parallel
// toggles Options.S2Parallel (B rides that opt-in). Returns shown
// frames + B laps fired + laps whose segment straddled an IDR boundary.
func bPlayAll(t *testing.T, name string, parallel bool) ([]bShown, int64, int64) {
	t.Helper()
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, S2Parallel: parallel})
	if err != nil {
		t.Fatalf("open %s parallel=%v: %v", name, parallel, err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatalf("%s buffered, want streaming", name)
	}
	deadline := time.Now().Add(90 * time.Second)
	var out []bShown
	for {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			cp := make([]byte, len(fr.Pix))
			copy(cp, fr.Pix)
			out = append(out, bShown{pts: fr.PTSMs, pix: cp})
		}
		if done {
			return out, atomic.LoadInt64(&p.bWindows), atomic.LoadInt64(&p.bSpans)
		}
		if time.Now().After(deadline) {
			t.Fatalf("not ended %s parallel=%v, shown %d", name, parallel, len(out))
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// bOracle decodes every sample straight through (no player, no drops)
// into a PTS-keyed pixel oracle (same recipe as S2's gate).
func bOracle(t *testing.T, name string) map[int64][]byte {
	t.Helper()
	movie, err := mp4.ParseFile(name)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	avcc, err := h264.ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	dec, err := NewDecoder(CodecH264)
	if err != nil {
		t.Fatalf("decoder: %v", err)
	}
	if err := feedParams(dec, avcc, name, ""); err != nil {
		t.Fatalf("params: %v", err)
	}
	var sps *h264.SPS
	if len(avcc.SPS) > 0 {
		sps, err = h264.ParseSPS(avcc.SPS[0])
		if err != nil {
			t.Fatalf("sps: %v", err)
		}
	}
	copt := color.Options{}
	if sps != nil && sps.VUI != nil {
		copt = color.OptionsFromVUI(sps.VUI.FullRange, sps.VUI.ColourPresent, sps.VUI.ColourMatrix)
	}
	sampling := dec.Sampling()
	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	type one struct {
		pts int64
		s   int
		pix []byte
	}
	var all []one
	reset := func() {
		dec, err = NewDecoder(CodecH264)
		if err != nil {
			t.Fatalf("decoder: %v", err)
		}
		if err := feedParams(dec, avcc, name, ""); err != nil {
			t.Fatalf("params: %v", err)
		}
	}
	for i, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitUnits(CodecH264, buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split %d: %v", i, err)
		}
		failed := false
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				// Mirror openBuffered concealment: non-fatal sample
				// errors skip the sample on a fresh decoder; fatal
				// ones fail loudly. The player under test does the
				// same through decodeStep, so the oracle stays exact.
				if streamFatal(err) {
					t.Fatalf("decode %d fatal: %v", i, err)
				}
				reset()
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			if streamFatal(err) {
				t.Fatalf("finish %d fatal: %v", i, err)
			}
			reset()
			continue
		}
		cf, err := color.Convert(sampling, pic.Y, pic.Cb, pic.Cr, int(pic.Width), int(pic.Height), copt)
		if err != nil {
			t.Fatalf("color %d: %v", i, err)
		}
		all = append(all, one{pts: s.PTSMs, s: i, pix: cf.Pix})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].pts != all[j].pts {
			return all[i].pts < all[j].pts
		}
		return all[i].s < all[j].s
	})
	out := make(map[int64][]byte, len(all))
	for _, o := range all {
		if _, dup := out[o.pts]; !dup {
			out[o.pts] = o.pix
		}
	}
	return out
}

func bCheckShown(t *testing.T, shown []bShown, oracle map[int64][]byte) {
	t.Helper()
	if len(shown) == 0 {
		t.Fatal("no frames shown")
	}
	for i := 1; i < len(shown); i++ {
		if shown[i].pts <= shown[i-1].pts {
			t.Fatalf("pts not monotonic at %d: %d <= %d", i, shown[i].pts, shown[i-1].pts)
		}
	}
	for i, s := range shown {
		want, ok := oracle[s.pts]
		if !ok {
			t.Fatalf("frame %d pts %d not in sequential oracle", i, s.pts)
		}
		if len(s.pix) != len(want) {
			t.Fatalf("frame %d pts %d pix len %d want %d", i, s.pts, len(s.pix), len(want))
		}
		for j := range s.pix {
			if s.pix[j] != want[j] {
				t.Fatalf("frame %d pts %d pix byte %d got=%d want=%d", i, s.pts, j, s.pix[j], want[j])
			}
		}
	}
}

func TestBPlayerExact(t *testing.T) {
	name := filepath.Join("testdata", "vr_b_long.mp4")
	if fi, err := os.Stat(name); err != nil {
		t.Fatalf("long clip absent: %v", err)
	} else if fi.Size() != 20110 {
		t.Fatalf("long clip bytes %d want 20110 (re-record baseline if the clip changed)", fi.Size())
	}
	oracle := bOracle(t, name)
	shown, laps, spans := bPlayAll(t, name, true)
	bCheckShown(t, shown, oracle)
	if laps < 1 {
		t.Fatalf("B never fired on the long-group clip")
	}
	if spans < 1 {
		t.Fatalf("B never straddled the IDR boundary (spans=0, want >=1 on 80f/2gops)")
	}
	t.Logf("B player 80f/2gops: shown=%d laps=%d spans=%d oracle=%d", len(shown), laps, spans, len(oracle))
}

// TestBPlayerOffParity pins the kill switch: GPUI_B_OFF=1 with the
// opt-in on plays oracle-exact with zero laps and zero spans.
func TestBPlayerOffParity(t *testing.T) {
	name := filepath.Join("testdata", "vr_b_long.mp4")
	t.Setenv("GPUI_B_OFF", "1")
	oracle := bOracle(t, name)
	shown, laps, spans := bPlayAll(t, name, true)
	if laps != 0 {
		t.Fatalf("B fired with GPUI_B_OFF=1")
	}
	if spans != 0 {
		t.Fatalf("B spanned with GPUI_B_OFF=1")
	}
	bCheckShown(t, shown, oracle)
}
