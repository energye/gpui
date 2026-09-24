package h264

// S2xB nested kernel: GOP fan-out outside, frame DAG inside.
//
// Say it plain: S2 fans whole IDR groups over workers, but a 250-frame
// group still decoded on one worker (nothing inside ran alone). B plans
// the reference DAG inside one group so frames start as soon as their
// anchors finish. This file nests them: each long GOP decodes on the
// proven plan+prime recipe (bframe.go PlanFrameGroup + bprime.go
// PrimeFrame, same DecodeNALU+FinishPicture pixel path), short GOPs
// stay on the plain sequential lane (crew setup costs more than the
// win below a dozen frames, same family as video/bframe.go bMinRemain).
//
// Peer (ffmpeg, read-only, ideas only, no code copied): same
// pthread_frame.c notes as s2_parallel.go (thread_count rule) and
// bframe.go (:645 await-progress ~= our per-frame done channels).
// Difference: two lodging levels (GOP crew outside, frame crew inside)
// instead of one shared pool with row-level waits; coarser, zero
// shared mutable pixel state, race-free by design.
//
// Exactness contract: any surprise (split error, plan refusal, unfed
// sample, snapshot gap, worker error) falls back to decodeS2GOP for
// that GOP, so output (and errors) stay bit-identical to sequential
// either way. Display pictures are owned copies (Crop) or caller
// shares; worker roster shares end in DropBuffered, never in the
// output. Player wiring follows; this gate pins the kernel only.

import (
	"errors"
	"runtime"
	"sync"
)

// s2NestedMinFrames is the smallest GOP worth frame-threading: shorter
// groups stay sequential (same family as video/bframe.go bMinRemain).
const s2NestedMinFrames = 12

// errSnapshotGap marks a worker whose snapshot reference never stored
// (internal tripwire only: the GOP falls back to sequential, which
// replays the exact behavior).
var errSnapshotGap = errors.New("h264: s2 nested snapshot gap")

// s2NestedInnerWorkers caps one GOP's frame crew. The DAG width binds
// first (P backbones stay ordered, B seas run a few wide: the 1080p
// probe showed 5-wide ~= dependency bound, the ENERGY crew curve caps
// at 3), so extra crew only parks decoders, never speeds frames.
func s2NestedInnerWorkers() int {
	w := s2WorkerCount(0)
	if w > 4 {
		w = 4
	}
	if w < 1 {
		w = 1
	}
	return w
}

// decodeS2GOPNested decodes one IDR GOP, frame-threading long groups
// and running short ones sequentially. Falls back to decodeS2GOP on
// any surprise (same bytes, same errors as sequential).
func decodeS2GOPNested(blobs [][]byte, avcc *AVCC) ([]*Picture, error) {
	if len(blobs) < s2NestedMinFrames {
		return decodeS2GOP(blobs, avcc)
	}
	frames := make([][][]byte, len(blobs))
	for i, blob := range blobs {
		units, err := SplitAVCC(blob, avcc.LengthSize)
		if err != nil {
			// Sequential replays the same split error exactly.
			return decodeS2GOP(blobs, avcc)
		}
		frames[i] = units
	}
	plans, ok := PlanFrameGroup(avcc, frames)
	if !ok {
		return decodeS2GOP(blobs, avcc)
	}
	for _, pl := range plans {
		if !pl.Fed {
			// Unfed samples take a different FinishPicture path
			// (no slices decoded errors); stay sequential so the
			// behavior matches exactly.
			return decodeS2GOP(blobs, avcc)
		}
	}
	if pics, ok := s2NestedExec(avcc, frames, plans, s2NestedInnerWorkers()); ok {
		return pics, nil
	}
	return decodeS2GOP(blobs, avcc)
}

// s2NestedExec runs one long GOP's frame DAG on a private crew and
// returns caller-owned display pictures in decode order. ok=false
// means "run sequential instead". Port of the bRunParallel recipe
// (bframe_test.go) to production: same wait set (snapshot union refs,
// strictly earlier so no cycle), same take-before-wait discipline (a
// waiter never holds a pool slot, so deep chains cannot deadlock the
// crew), same PrimeFrame+DecodeNALU+FinishPicture pixel path.
func s2NestedExec(avcc *AVCC, frames [][][]byte, plans []FramePlan, workers int) (disp []*Picture, ok bool) {
	n := len(frames)
	disp = make([]*Picture, n)
	aligned := make([]*Picture, n)
	done := make([]chan struct{}, n)
	for i := range done {
		done[i] = make(chan struct{})
	}
	if workers < 1 {
		workers = 1
	}
	pool := make([]*Decoder, workers)
	free := make([]bool, workers)
	for w := range pool {
		wps := NewParamSets()
		if err := wps.FromAVCC(avcc); err != nil {
			return nil, false
		}
		pool[w] = NewDecoder(wps)
		free[w] = true
	}
	var poolMu sync.Mutex
	take := func() (int, *Decoder) {
		for {
			poolMu.Lock()
			for w := range pool {
				if free[w] {
					free[w] = false
					d := pool[w]
					poolMu.Unlock()
					return w, d
				}
			}
			poolMu.Unlock()
			runtime.Gosched()
		}
	}
	give := func(w int) {
		poolMu.Lock()
		free[w] = true
		poolMu.Unlock()
	}
	// teardown drops worker roster shares (PrimeFrame retains). Caller
	// display shares stay alive on their own counts.
	teardown := func() {
		for _, d := range pool {
			d.DropBuffered()
		}
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for f := range frames {
		wg.Add(1)
		go func(f int) {
			defer wg.Done()
			waited := make(map[int]bool, len(plans[f].Snapshot)+len(plans[f].Refs))
			for _, r := range plans[f].Snapshot {
				if !waited[r] {
					waited[r] = true
					<-done[r]
				}
			}
			for _, r := range plans[f].Refs {
				if !waited[r] {
					waited[r] = true
					<-done[r]
				}
			}
			mu.Lock()
			if firstErr != nil {
				mu.Unlock()
				close(done[f])
				return
			}
			mu.Unlock()
			w, dec := take()
			snap := make([]*Picture, len(plans[f].Snapshot))
			for i, si := range plans[f].Snapshot {
				snap[i] = aligned[si]
				if snap[i] == nil {
					give(w)
					mu.Lock()
					if firstErr == nil {
						firstErr = errSnapshotGap
					}
					mu.Unlock()
					close(done[f])
					return
				}
			}
			dec.PrimeFrame(snap, plans[f].Seed)
			var pic *Picture
			var err error
			for _, u := range frames[f] {
				if err = dec.DecodeNALU(u); err != nil {
					break
				}
			}
			if err == nil {
				pic, err = dec.FinishPicture()
			}
			stored := dec.lastStored
			give(w)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				close(done[f])
				return
			}
			disp[f] = pic
			aligned[f] = stored
			close(done[f])
		}(f)
	}
	wg.Wait()
	if firstErr != nil {
		for _, pic := range disp {
			if pic != nil {
				pic.Release()
			}
		}
		teardown()
		return nil, false
	}
	teardown()
	return disp, true
}
