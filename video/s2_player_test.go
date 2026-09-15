package video

// S2 Player wiring gate: the opt-in parallel player plays bit-exact vs
// the sequential path, really runs windows in parallel, and seeks land
// the same. Throughput uses the wall clock (not the hand-clock count:
// the hand rig advances a full stamp per 1ms real, so any batched
// decoder drops by construction; production uses the wall clock at
// ~200ms/frame, where a ~16ms window is negligible).
//
// Peer: libavcodec/pthread_internal.h:26 cap 16 +
// pthread_frame.c:912-923 cores+1 + :949 delay +
// :122/:139/:143/:492/:565/:573/:579 submit loop +
// h264dec.c:438 idr() + :668 idr(h) on IDR slice (our window rule: only
// groups headed by a keyframe run parallel) + pthread_slice.c:120-130
// same thread rule, against video/s2_player.go + video/player.go hooks
// (Options.S2Parallel, default sequential keeps every existing gate).
// Clip: testdata/vr_stream_long.mp4 (tracked 230K, 320x240/Main/200
// samples/40 IDR GOPs x5; missing file FAILs, never Skips).
// Small clip vr5_seek.mp4 stays buffered: plays through, zero windows.

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

type s2Shown struct {
	pts int64
	pix []byte
}

// s2PlayAll plays name to Ended and copies every shown Pix (streaming
// Pix is recycled on the next Poll, so copy during the tick like the
// windows do). parallel toggles Options.S2Parallel. The hand clock
// keeps stamps deterministic; the wall clock measures throughput.
func s2PlayAll(t *testing.T, name string, parallel bool, now func() int64) ([]s2Shown, int64, time.Duration) {
	t.Helper()
	var h *handClock
	opt := Options{S2Parallel: parallel}
	if now != nil {
		opt.NowMs = now
	} else {
		h = &handClock{}
		opt.NowMs = h.at
	}
	p, err := OpenFile(name, opt)
	if err != nil {
		t.Fatalf("open %s parallel=%v: %v", name, parallel, err)
	}
	defer p.Close()
	t0 := time.Now()
	deadline := t0.Add(90 * time.Second)
	var out []s2Shown
	for {
		if h != nil {
			h.now += 200
		}
		fr, done := p.Poll()
		if fr != nil {
			cp := make([]byte, len(fr.Pix))
			copy(cp, fr.Pix)
			out = append(out, s2Shown{pts: fr.PTSMs, pix: cp})
		}
		if done {
			return out, p.S2Windows(), time.Since(t0)
		}
		if time.Now().After(deadline) {
			t.Fatalf("not ended %s parallel=%v, shown %d", name, parallel, len(out))
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// s2SequentialRGBA decodes name sequentially (one decoder, sample order)
// and returns display-order RGBA keyed by PTS. Same splitters, same
// param feed, same color registry as the player, so any wiring drift
// shows as a byte difference at the same stamp.
func s2SequentialRGBA(t *testing.T, name string) map[int64][]byte {
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
	for i, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitUnits(CodecH264, buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split %d: %v", i, err)
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("decode %d: %v", i, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			t.Fatalf("finish %d: %v", i, err)
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

func TestS2PlayerParallelExact(t *testing.T) {
	name := filepath.Join("testdata", "vr_stream_long.mp4")
	fi, err := os.Stat(name)
	if err != nil {
		t.Fatalf("long clip absent (tracked, want 234721B): %v", err)
	}
	if fi.Size() != 234721 {
		t.Fatalf("long clip bytes %d want 234721 (re-record baseline if the clip changed)", fi.Size())
	}
	// Parallel play (opt-in). Drops are allowed (bounded catch-up, same
	// contract as the existing long-clip gate): order + Ended + drop
	// accounting is the guarantee, and every shown pixel must equal the
	// sequential decode at the same stamp.
	shown, windows, parWall := s2PlayAll(t, name, true, nil)
	if len(shown) == 0 {
		t.Fatal("no frames shown")
	}
	for i := 1; i < len(shown); i++ {
		if shown[i].pts <= shown[i-1].pts {
			t.Fatalf("pts not monotonic at %d: %d <= %d", i, shown[i].pts, shown[i-1].pts)
		}
	}
	if windows < 1 {
		t.Fatalf("s2windows = %d, want >= 1 (40 GOPs must trip the parallel path)", windows)
	}
	oracle := s2SequentialRGBA(t, name)
	for i, s := range shown {
		want, ok := oracle[s.pts]
		if !ok {
			t.Fatalf("frame %d pts %d not in sequential oracle", i, s.pts)
		}
		if !equalBytes(s.pix, want) {
			t.Fatalf("frame %d pts %d pixels differ parallel vs sequential", i, s.pts)
		}
	}
	// Sequential reference wall on the same rig (log only, never a hard
	// line: CI boxes differ; the kernel gate already pins ~2.18x on the
	// decode itself).
	_, seqWindows, seqWall := s2PlayAll(t, name, false, nil)
	if seqWindows != 0 {
		t.Fatalf("s2windows = %d without opt-in, want 0 (default stays sequential)", seqWindows)
	}
	t.Logf("s2 player 200f/40gops: shown=%d/%d windows=%d parWall=%v seqWall=%v oracle=%d",
		len(shown), 200, windows, parWall, seqWall, len(oracle))
}

func TestS2PlayerSeekParity(t *testing.T) {
	name := filepath.Join("testdata", "vr_stream_long.mp4")
	if _, err := os.Stat(name); err != nil {
		t.Fatalf("long clip absent: %v", err)
	}
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, S2Parallel: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	// Let the background run a little, then jump mid-clip. Async model
	// (same as waitSeekLanded in stream_long_test.go): SeekTo only
	// reparks the needle; the background drops forward frames until the
	// landing decodes. Frames already queued ahead of the seek may still
	// poll out first, so only the first frame at/above the landing
	// counts — and it must equal the landing exactly, then monotonic.
	for i := 0; i < 5; i++ {
		h.now += 200
		p.Poll()
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	landed, err := p.SeekTo(20000)
	if err != nil {
		t.Fatalf("seek: %v", err)
	}
	deadline := time.Now().Add(60 * time.Second)
	var first int64 = -1
	var prev int64 = -1
	seen := false
	for {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			if !seen && fr.PTSMs < landed {
				continue
			}
			if !seen {
				first = fr.PTSMs
				seen = true
			}
			if prev >= 0 && fr.PTSMs <= prev {
				t.Fatalf("post-seek pts not monotonic: %d <= %d", fr.PTSMs, prev)
			}
			prev = fr.PTSMs
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("seek play not ended, first=%d", first)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if !seen {
		t.Fatal("no frames after seek")
	}
	if first != landed {
		t.Fatalf("first after seek = %d, want landing %d", first, landed)
	}
}

func TestS2PlayerSmallClipUntouched(t *testing.T) {
	name := filepath.Join("testdata", "vr5_seek.mp4")
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, S2Parallel: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if !p.Buffered() {
		t.Fatal("vr5_seek.mp4 should stay buffered (<=64 frames)")
	}
	if got := p.S2Windows(); got != 0 {
		t.Fatalf("s2windows = %d on buffered path, want 0", got)
	}
	deadline := time.Now().Add(30 * time.Second)
	var n int
	for {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			n++
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("small clip not ended")
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	if n != 10 {
		t.Fatalf("shown = %d, want 10", n)
	}
}
