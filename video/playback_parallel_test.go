//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package video

// and parallel-vs-sequential plays agree stamp for stamp. Pixel truth is
// ffmpeg-vs-ffmpeg (same backend, same run shape); the old Go-decoder
// oracle retired with the Go decode path.
// Clip: testdata/vr_stream_long.mp4 (tracked 230K, 320x240/Main/200
// samples; missing file FAILs, never Skips).

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// s2PlayAll plays name to Ended and copies every shown Pix (streaming
// Pix is recycled on the next Poll, so copy during the tick like the
// windows do). parallel toggles Options.S2Parallel (compat no-op on the
// ffmpeg backend). Paced ticks with yields keep the background decoding.
func s2PlayAll(t *testing.T, name string, parallel bool) (map[int64][]byte, []int64) {
	t.Helper()
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, S2Parallel: parallel})
	if err != nil {
		t.Fatalf("open %s parallel=%v: %v", name, parallel, err)
	}
	defer p.Close()
	deadline := time.Now().Add(90 * time.Second)
	pix := map[int64][]byte{}
	var pts []int64
	for {
		if time.Now().After(deadline) {
			t.Fatalf("not ended %s parallel=%v, shown %d", name, parallel, len(pts))
		}
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			cp := make([]byte, len(fr.Pix))
			copy(cp, fr.Pix)
			if _, dup := pix[fr.PTSMs]; !dup {
				pix[fr.PTSMs] = cp
				pts = append(pts, fr.PTSMs)
			}
		}
		if done {
			return pix, pts
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
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
	// Same backend both ways: full play, monotonic stamps, Ended; the
	// option changes nothing but must not break anything.
	parPix, parPTS := s2PlayAll(t, name, true)
	seqPix, seqPTS := s2PlayAll(t, name, false)
	for _, list := range [][]int64{parPTS, seqPTS} {
		if len(list) < 190 {
			t.Fatalf("shown = %d, want >= 190 (bounded catch-up drops ok)", len(list))
		}
		for i := 1; i < len(list); i++ {
			if list[i] <= list[i-1] {
				t.Fatalf("pts not monotonic at %d: %d <= %d", i, list[i], list[i-1])
			}
		}
	}
	// Common stamps must show identical pixels (deterministic backend).
	common := 0
	for pts, want := range seqPix {
		got, ok := parPix[pts]
		if !ok {
			continue
		}
		common++
		if !equalBytes(got, want) {
			t.Fatalf("pts %d pixels differ parallel vs sequential (same backend)", pts)
		}
	}
	if common < 100 {
		t.Fatalf("common stamps = %d, want >= 100", common)
	}
	t.Logf("s2 ffmpeg parity: par=%d seq=%d common=%d", len(parPTS), len(seqPTS), common)
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
	// Let the background run a little, then jump mid-clip. Echo landing
	// (20000 sits on the 200ms grid); the first picture must equal it
	// exactly — poll first at the frozen stamp, tick only on misses, so
	// the exact frame is never eaten as stale before it is seen.
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
	if landed != 20000 {
		t.Fatalf("landed = %d, want 20000 (echo)", landed)
	}
	deadline := time.Now().Add(60 * time.Second)
	var first int64 = -1
	var prev int64 = -1
	seen := false
	firstRound := true
	for {
		if time.Now().After(deadline) {
			t.Fatalf("seek play not ended, first=%d", first)
		}
		fr, done := p.Poll()
		if fr != nil {
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
		if !firstRound {
			h.now += 200
		}
		firstRound = false
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
	if p.Buffered() {
		t.Fatal("vr5_seek.mp4 buffered, want ffmpeg streaming path")
	}
	if got := p.S2Windows(); got != 0 {
		t.Fatalf("s2windows = %d, want 0 (ffmpeg owns threading)", got)
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
