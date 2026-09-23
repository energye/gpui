package h264

// S2 speedup probe (step S2b: measure before cutting further).
// Compares sequential vs GOP-parallel decode of the 1080p reference
// clip: same samples, pixel hashes must match bit for bit (hard line,
// like TestS2GOPParallelExactAndThroughput); walls and the ratio are
// logged only, never a hard line (speedup varies by core count and
// thermals).
//
// Env-gated (GPUI_S2_TIMING=1): a full 641-frame decode twice takes
// minutes and pins ~2GB (parallel holds all pictures), so the normal
// suite never runs it. 2K waits for a streaming parallel entry (2304
// pics ~ 12GB do not fit); 1080p's 3 GOPs answer the scaling question.

import (
	"hash/fnv"
	"os"
	"testing"
	"time"

	"github.com/energye/gpui/video/mp4"
)

func TestS2GOPSpeedup1080p(t *testing.T) {
	if os.Getenv("GPUI_S2_TIMING") == "" {
		t.Skipf("set GPUI_S2_TIMING=1 to run the 1080p seq-vs-parallel timing")
	}
	path := "../testdata/1080p_1920_1080_60fps.mp4"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("clip absent: %v", err)
	}
	movie, err := mp4.ParseFile(path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	blobs := make([][]byte, len(v.Samples))
	for i, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		blobs[i] = buf
	}
	keyIdx := map[int]bool{}
	for _, k := range v.Keyframes {
		keyIdx[k.SampleNumber] = true
	}
	var starts []int
	for i, s := range v.Samples {
		if keyIdx[s.Number] {
			starts = append(starts, i)
		}
	}
	if len(starts) < 2 || starts[0] != 0 {
		t.Skipf("need 2+ GOPs from 0, got starts=%v", starts)
	}

	// Sequential reference (single decoder, decode order, streaming
	// hash + release: flat memory).
	t0 := time.Now()
	seqDec := NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := seqDec.DecodeNALU(raw); err != nil {
			t.Fatalf("seq sps: %v", err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := seqDec.DecodeNALU(raw); err != nil {
			t.Fatalf("seq pps: %v", err)
		}
	}
	hSeq := fnv.New64a()
	nSeq := 0
	for i, blob := range blobs {
		units, err := SplitAVCC(blob, avcc.LengthSize)
		if err != nil {
			t.Fatalf("seq split %d: %v", i, err)
		}
		for _, u := range units {
			if err := seqDec.DecodeNALU(u); err != nil {
				t.Fatalf("seq decode %d: %v", i, err)
			}
		}
		pic, err := seqDec.FinishPicture()
		if err != nil {
			t.Fatalf("seq finish %d: %v", i, err)
		}
		hSeq.Write(pic.Y)
		hSeq.Write(pic.Cb)
		hSeq.Write(pic.Cr)
		pic.Release()
		nSeq++
	}
	seqWall := time.Since(t0)
	t.Logf("seq: pics=%d wall=%.1fs avg=%.2fms hash=%x",
		nSeq, seqWall.Seconds(), float64(seqWall.Microseconds())/1000/float64(nSeq), hSeq.Sum64())

	// Parallel GOPs (holds all pictures: ~2GB for 1080p, in the cap).
	var stats s2GOPStats
	t1 := time.Now()
	parPics, err := decodeGOPsParallel(blobs, starts, avcc, 0, &stats)
	if err != nil {
		t.Fatalf("parallel: %v", err)
	}
	parWall := time.Since(t1)
	hPar := fnv.New64a()
	for _, pic := range parPics {
		hPar.Write(pic.Y)
		hPar.Write(pic.Cb)
		hPar.Write(pic.Cr)
	}
	for _, pic := range parPics {
		pic.Release()
	}
	t.Logf("par: pics=%d wall=%.1fs avg=%.2fms hash=%x maxInFlight=%d workers=%d",
		len(parPics), parWall.Seconds(), float64(parWall.Microseconds())/1000/float64(len(parPics)),
		hPar.Sum64(), stats.MaxInFlight, s2WorkerCount(0))
	if len(parPics) != nSeq {
		t.Fatalf("count parallel=%d want %d", len(parPics), nSeq)
	}
	if hPar.Sum64() != hSeq.Sum64() {
		t.Fatalf("pixel hash differs: par=%x seq=%x", hPar.Sum64(), hSeq.Sum64())
	}
	if stats.MaxInFlight <= 1 {
		t.Fatalf("max_in_flight=%d want >1 (%d GOPs)", stats.MaxInFlight, len(starts))
	}
	t.Logf("speedup=%.2fx (seq %.1fs / par %.1fs, %d GOPs, %d workers; ratio only, thermals apply)",
		seqWall.Seconds()/parWall.Seconds(), seqWall.Seconds(), parWall.Seconds(), len(starts), s2WorkerCount(0))
}
