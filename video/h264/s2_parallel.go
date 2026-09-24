package h264

// S2 frame-level GOP kernel (desktop only; Player wiring follows).
//
// Peer (ffmpeg, read-only, no vendoring):
//   libavcodec/pthread_internal.h:26 MAX_AUTO_THREADS 16
//   libavcodec/pthread_frame.c:917-923 default thread_count =
//     FFMIN(nb_cpus+1, MAX_AUTO_THREADS)
//   libavcodec/pthread_frame.c:948-949 delay = thread_count-1
//   libavcodec/pthread_frame.c:122/143 prev_thread/next_decoding fields,
//     :551 round-robin submit, :569-585 submit-while-no-result loop
//   libavcodec/h264dec.c:660-667 IDR clears reference state via idr(h)
//   libavcodec/pthread_slice.c:127 same thread_count rule for slices
// Ours: video/h264/s2_parallel.go (this file) against video/h264/mb.go:36
// Decoder (stateful DPB) and video/player.go decodeStep/decodeLoop/pending
// (existing reorder skeleton; Player wiring follows, not in this step).
//
// Rule: GOPs starting with IDR decode independently (IDR clears the DPB,
// h264dec.c:660-667), so each GOP runs on a fresh Decoder and the outputs
// concatenate in GOP order to the sequential bit stream. Long GOPs nest
// frame-DAG threading inside (s2_nested.go); short ones stay sequential.
// Anything else (empty starts, unsorted starts, first start != 0) falls
// back to the sequential path with no goroutines. Pure Go goroutines: correctness is
// arch-independent (amd64/arm64/i386); speedup varies by core count and is
// only logged, never a hard line (see the gate).

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

// s2MaxAutoThreads mirrors ffmpeg MAX_AUTO_THREADS
// (pthread_internal.h:26).
const s2MaxAutoThreads = 16

// s2WorkerCount mirrors ffmpeg's default thread_count
// (pthread_frame.c:917-923): cores+1 capped at 16; want>0 pins it.
func s2WorkerCount(want int) int {
	if want > 0 {
		if want > s2MaxAutoThreads {
			return s2MaxAutoThreads
		}
		if want < 1 {
			return 1
		}
		return want
	}
	n := runtime.NumCPU() + 1
	if n < 1 {
		n = 1
	}
	if n > s2MaxAutoThreads {
		n = s2MaxAutoThreads
	}
	return n
}

// s2GOPStats observes one parallel run. MaxInFlight proves workers really
// overlapped (>1 on multi-GOP clips); walls/speedup stay in test logs.
type s2GOPStats struct {
	MaxInFlight int32
}

// decodeS2GOP decodes one IDR GOP sequentially on a fresh Decoder.
func decodeS2GOP(blobs [][]byte, avcc *AVCC) ([]*Picture, error) {
	dec := NewDecoder(nil)
	for _, raw := range avcc.SPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("s2 sps: %w", err)
		}
	}
	for _, raw := range avcc.PPS {
		if err := dec.DecodeNALU(raw); err != nil {
			return nil, fmt.Errorf("s2 pps: %w", err)
		}
	}
	pics := make([]*Picture, 0, len(blobs))
	for i, blob := range blobs {
		units, err := SplitAVCC(blob, avcc.LengthSize)
		if err != nil {
			return nil, fmt.Errorf("s2 split %d: %w", i, err)
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				return nil, fmt.Errorf("s2 decode %d: %w", i, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			return nil, fmt.Errorf("s2 finish %d: %w", i, err)
		}
		pics = append(pics, pic)
	}
	return pics, nil
}

// decodeGOPsParallel decodes IDR GOPs concurrently and concatenates in GOP
// order (decode order). blobs holds one sample payload per index; starts
// holds the sample index of each GOP head (starts[0] must be 0, sorted).
func decodeGOPsParallel(blobs [][]byte, starts []int, avcc *AVCC, workers int, stats *s2GOPStats) ([]*Picture, error) {
	if len(blobs) == 0 || len(starts) == 0 || starts[0] != 0 {
		return nil, fmt.Errorf("s2: bad gop table (blobs=%d starts=%v)", len(blobs), starts)
	}
	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] || starts[i] >= len(blobs) {
			return nil, fmt.Errorf("s2: bad gop table (blobs=%d starts=%v)", len(blobs), starts)
		}
	}
	w := s2WorkerCount(workers)
	if len(starts) <= 1 || w <= 1 {
		pics, err := decodeS2GOPNested(blobs, avcc)
		if err != nil {
			return nil, err
		}
		if stats != nil {
			atomic.StoreInt32(&stats.MaxInFlight, 1)
		}
		return pics, nil
	}
	nGOP := len(starts)
	results := make([][]*Picture, nGOP)
	sem := make(chan struct{}, w)
	var wg sync.WaitGroup
	var inFlight, maxFlight int32
	var firstErr error
	var mu sync.Mutex
	for g := 0; g < nGOP; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxFlight)
				if cur <= old || atomic.CompareAndSwapInt32(&maxFlight, old, cur) {
					break
				}
			}
			defer atomic.AddInt32(&inFlight, -1)
			s0 := starts[g]
			s1 := len(blobs)
			if g+1 < nGOP {
				s1 = starts[g+1]
			}
			pics, err := decodeS2GOPNested(blobs[s0:s1], avcc)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			results[g] = pics
		}(g)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	out := make([]*Picture, 0, len(blobs))
	for _, gp := range results {
		if len(gp) == 0 {
			return nil, fmt.Errorf("s2: empty gop result")
		}
		out = append(out, gp...)
	}
	if stats != nil {
		atomic.StoreInt32(&stats.MaxInFlight, atomic.LoadInt32(&maxFlight))
	}
	return out, nil
}
