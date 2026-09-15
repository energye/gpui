package video

// S2 Player wiring: bounded lookahead over IDR groups.
//
// Say it plain: long clips decode group by group in the background, and
// groups that start fresh (IDR keyframes) can run on several cores at
// once. Short clips keep the old single-thread path bit for bit.
//
// Peer (ffmpeg, read-only, no vendoring):
//   libavcodec/pthread_internal.h:26 MAX_AUTO_THREADS 16
//   libavcodec/pthread_frame.c:912-923 default thread_count =
//     FFMIN(nb_cpus+1, MAX_AUTO_THREADS)
//   libavcodec/pthread_frame.c:949 delay = thread_count-1
//   libavcodec/pthread_frame.c:122/139/143 prev_thread/next fields,
//     :492 submit_packet, :565/:573/:579 submit-while-no-result loop
//   libavcodec/h264dec.c:438 idr(), :668 idr(h) on IDR slice (a group
//     headed by IDR clears reference state, so groups decode alone)
//   libavcodec/pthread_slice.c:120-130 same thread_count rule
// Ours: this file against video/player.go decodeStep/decodeLoop/pending
// (the existing reorder skeleton from §11.6 S2) and video/h264/
// s2_parallel.go (the kernel this wiring feeds; same grouping rule).
//
// Differences from ffmpeg, written down so nobody has to guess:
//   - ffmpeg runs generic frame threading with a delay pipeline and
//     waits on references; we only run groups headed by a keyframe
//     (each on a fresh decoder) and anything else stays sequential.
//   - ffmpeg pipelines submit/collect; we decode one bounded window at
//     a time and emit in order, then take the next window. Simpler,
//     exact, and memory stays flat.
//
// Exactness contract: a clean window yields the same pictures in the
// same order as running decodeStep sample by sample (same splitters,
// same param feed, same finish, same stamps, same pending emit rule).
// Any read/split/decode surprise inside a window throws the window away,
// parks the needle back on the window head, and the caller replays those
// samples through decodeStep, so dirty clips keep today's conceal path
// bit for bit. Fatal stream errors (F17 and friends via streamFatal)
// still kill the stream loudly, never concealed.
//
// Locking: blob reads run one by one on the caller thread (the Source
// stays single-threaded); only the CPU decode fans out. Workers touch
// no Player locks. The generation number throws a travelled window away
// when a seek reparks the needle mid-window.

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// s2WorkerCap mirrors ffmpeg MAX_AUTO_THREADS
// (pthread_internal.h:26).
const s2WorkerCap = 16

// s2WindowMaxFrames caps one window's pictures by the foreground need:
// one emitted group per call (enough for the display queue) plus the
// reorder tail. Small on purpose: the queue cap is 4, so parking tens
// of pictures per call only feeds catch-up drops. The head group here
// is 5 pictures (4 IDR GOPs of 5 on the tracked long clip), so the cap
// must cover at least one full group plus depth, else no window ever
// runs (and the wiring silently never fires).
const s2WindowMaxFrames = 8

// s2WindowMaxBytes caps one window's YUV bytes (big frames bind here, so
// a 4K window never parks hundreds of megabytes transient).
const s2WindowMaxBytes = 64 << 20

// s2WorkerCount mirrors ffmpeg's default thread_count
// (pthread_frame.c:912-923): cores+1 capped at 16; want>0 pins it.
func s2WorkerCount(want int) int {
	if want > 0 {
		if want > s2WorkerCap {
			return s2WorkerCap
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
	if n > s2WorkerCap {
		n = s2WorkerCap
	}
	return n
}

// s2ParallelCodecOK gates the wiring by codec capability: only codecs
// whose keyframe groups decode alone may run here (today that is the
// registered H.264 entry, whose IDR clears reference state per
// h264dec.c:438/:668). Anything else stays sequential.
func s2ParallelCodecOK(codec string) bool {
	return codec == CodecH264
}

// buildS2Starts maps keyframes to decode-order group heads. It returns
// nil when there is nothing worth running in parallel (fewer than two
// groups, unsorted heads, or the first head is not sample 0), and the
// caller falls back to the sequential path with no goroutines.
func buildS2Starts(samples []mp4.Sample, keyframes []mp4.Keyframe) []int {
	if len(samples) == 0 || len(keyframes) < 2 {
		return nil
	}
	posOf := make(map[int]int, len(samples))
	for i, s := range samples {
		if _, dup := posOf[s.Number]; !dup {
			posOf[s.Number] = i
		}
	}
	starts := make([]int, 0, len(keyframes))
	for _, k := range keyframes {
		p, ok := posOf[k.SampleNumber]
		if !ok {
			return nil
		}
		starts = append(starts, p)
	}
	if len(starts) < 2 || starts[0] != 0 {
		return nil
	}
	for i := 1; i < len(starts); i++ {
		if starts[i] <= starts[i-1] || starts[i] >= len(samples) {
			return nil
		}
	}
	return starts
}

// s2WindowForPos picks the window at pos: pos must sit exactly on a
// group head. One lap claims the head group plus one lookahead group —
// two whole groups, group fan-out (one worker per group, semaphored by
// the ffmpeg thread_count rule). The head emits this lap (decodeStep's
// exact rule: group samples minus reorder tail); the lookahead stays
// in pending for the next laps. The needle advances at most one window
// per lap, so catch-up drops stay where the sequential path leaves
// them. The last group alone (no lookahead left) runs single-group.
// ok=false means run one sequential step instead.
func s2WindowForPos(starts []int, pos, workers, maxFrames, maxBytes, yuvPerFrame int) (g0, g1 int, ok bool) {
	if len(starts) < 1 || workers <= 1 || pos < 0 {
		return 0, 0, false
	}
	g := -1
	for i, s := range starts {
		if s == pos {
			g = i
			break
		}
		if s > pos {
			break
		}
	}
	if g < 0 || g >= len(starts) {
		return 0, 0, false
	}
	n := 2
	if n > len(starts)-g {
		n = len(starts) - g
	}
	if n < 1 {
		return 0, 0, false
	}
	endOf := func(n int) int {
		if g+n < len(starts) {
			return starts[g+n]
		}
		return -1
	}
	for n > 1 && endOf(n) >= 0 {
		frames := endOf(n) - pos
		if frames <= maxFrames && (yuvPerFrame <= 0 || frames*yuvPerFrame <= maxBytes) {
			break
		}
		n--
	}
	if n < 1 {
		return 0, 0, false
	}
	return g, g + n, true
}

// s2GOPJob is one group's in-memory payloads for a worker.
type s2GOPJob struct {
	base  int // decode-order index of blobs[0]
	blobs [][]byte
}

// s2GOPResult carries one group's pictures back in sample order; a nil
// slot means its sample fed nothing (mirrors decodeStep's !fed skip).
type s2GOPResult struct {
	pics []*h264.Picture
}

// s2DecodeBlobs feeds blobs on a fresh decoder and returns one picture
// per fed sample (nil where the sample fed nothing, mirroring
// decodeStep's !fed skip). Errors propagate for the caller to triage
// (fatal loud, else sequential fallback).
func s2DecodeBlobs(codec string, avcc *h264.AVCC, path string, nums []int, blobs [][]byte) ([]*h264.Picture, error) {
	dec, err := NewDecoder(codec)
	if err != nil {
		return nil, err
	}
	if err := feedParams(dec, avcc, path, ""); err != nil {
		return nil, err
	}
	out := make([]*h264.Picture, len(blobs))
	for k, blob := range blobs {
		units, err := SplitUnits(codec, blob, avcc.LengthSize)
		if err != nil {
			return nil, err
		}
		if err := RejectUnits(codec, units, nums[k], path); err != nil {
			return nil, err
		}
		fed := false
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				return nil, err
			}
			fed = true
		}
		if !fed {
			continue
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			return nil, err
		}
		out[k] = pic
	}
	return out, nil
}

// maybeDecodeS2Window runs one bounded parallel window when the needle
// sits on a group head, else reports ok=false and the caller runs a
// plain decodeStep. The window is exactly the head group (bounded by
// the transient cap); its samples decode on workers and emit in order
// through the same pending skeleton. Buffered clips never reach here
// (their pixels live in the open-time cache).
func (p *Player) maybeDecodeS2Window(gen int64) (emitted []*pendingPic, done bool, err error, ok bool) {
	if p.buffered || !s2ParallelCodecOK(p.codec) || len(p.s2starts) < 1 {
		return nil, false, nil, false
	}
	// While a seek travels, stay sequential: one sample per lap keeps
	// the landing exact (a claimed window would decode past it and the
	// seek filter would drop the landing's predecessors in bulk,
	// skipping the stamp under the hand clock).
	p.dmu.Lock()
	travelling := p.seekActive
	p.dmu.Unlock()
	if travelling {
		return nil, false, nil, false
	}
	p.dmu.Lock()
	pos := p.pos
	starts := p.s2starts
	yuv := p.s2yuv
	p.dmu.Unlock()
	workers := s2WorkerCount(0)
	g0, g1, wok := s2WindowForPos(starts, pos, workers, s2WindowMaxFrames, s2WindowMaxBytes, yuv)
	if !wok {
		return nil, false, nil, false
	}
	return p.decodeS2Window(g0, g1, gen)
}

// decodeS2Window decodes groups [g0,g1) with group fan-out (one
// worker per group), then emits decodeStep's exact rule on the head
// group only: same stamps, same pending tail. The lookahead group(s)
// stay in pending for the next laps. didParallel=false means nothing
// was claimed: run decodeStep instead.
// A generation change mid-window discards the work (the seeker owns the
// needle now) and reports didParallel=true with no emissions.
func (p *Player) decodeS2Window(g0, g1 int, gen int64) (emitted []*pendingPic, done bool, err error, didParallel bool) {
	// Simple batch (reviewable): read the window blobs sync (Source
	// stays single-threaded), fan groups over workers (one group per
	// goroutine, semaphored by the ffmpeg thread_count rule), emit
	// decodeStep's exact rule on the whole window minus the reorder
	// tail. Same splitters/params/finish/stamps as the sequential
	// chain; any read/split/decode surprise rewinds the needle and
	// replays sequentially (today's conceal path). Fatal stream errors
	// propagate loud. A generation change mid-window discards the work.
	p.dmu.Lock()
	if atomic.LoadInt64(&p.generation) != gen {
		p.dmu.Unlock()
		return nil, false, nil, false
	}
	if p.pos != p.s2starts[g0] {
		if len(p.pending) > p.reorderDepth {
			p.dmu.Unlock()
			return nil, false, nil, false
		}
		p.pos = p.s2starts[g0]
		p.pending = nil
		p.resetDecoderLocked()
	}
	starts := p.s2starts
	samples := p.samples
	s0 := starts[g0]
	s1 := len(samples)
	if g1 < len(starts) {
		s1 = starts[g1]
	}
	if s1 <= s0 {
		p.dmu.Unlock()
		return nil, false, nil, false
	}
	avcc := p.avcc
	codec := p.codec
	epoch := p.epoch
	base0 := p.base0
	depth := p.reorderDepth
	path := p.path
	src := p.source
	p.pos = s1 // claim the window; a seek overwrites this and bumps gen
	p.dmu.Unlock()

	blobs := make([][]byte, 0, s1-s0)
	for i := s0; i < s1; i++ {
		buf := make([]byte, samples[i].Size)
		if _, rerr := readSourceRange(src, buf, int64(samples[i].Offset)); rerr != nil {
			p.dmu.Lock()
			if atomic.LoadInt64(&p.generation) == gen {
				p.pos = s0
			}
			p.dmu.Unlock()
			return nil, false, nil, false
		}
		blobs = append(blobs, buf)
	}

	jobs := make([]s2GOPJob, 0, g1-g0)
	for g := g0; g < g1; g++ {
		b0 := starts[g] - s0
		b1 := len(blobs)
		if g+1 < g1 {
			b1 = starts[g+1] - s0
		} else {
			b1 = s1 - s0
		}
		if b0 < 0 {
			b0 = 0
		}
		if b1 > len(blobs) {
			b1 = len(blobs)
		}
		if b1 < b0 {
			b1 = b0
		}
		jobs = append(jobs, s2GOPJob{base: starts[g], blobs: blobs[b0:b1]})
	}

	var wg sync.WaitGroup
	results := make([]s2GOPResult, len(jobs))
	var firstFatal error
	var needFallback atomic.Bool
	var mu sync.Mutex
	sem := make(chan struct{}, s2WorkerCount(0))
	for j := range jobs {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if needFallback.Load() {
				return
			}
			job := jobs[j]
			nums := make([]int, len(job.blobs))
			for k := range job.blobs {
				nums[k] = samples[job.base+k].Number
			}
			pics, derr := s2DecodeBlobs(codec, avcc, path, nums, job.blobs)
			if derr != nil {
				if streamFatal(derr) {
					mu.Lock()
					if firstFatal == nil {
						firstFatal = derr
					}
					mu.Unlock()
					return
				}
				needFallback.Store(true)
				return
			}
			results[j].pics = pics
		}(j)
	}
	wg.Wait()

	if firstFatal != nil {
		return nil, false, firstFatal, true
	}
	if needFallback.Load() {
		p.dmu.Lock()
		if atomic.LoadInt64(&p.generation) == gen {
			p.pos = s0
		}
		p.dmu.Unlock()
		return nil, false, nil, false
	}

	p.dmu.Lock()
	defer p.dmu.Unlock()
	if atomic.LoadInt64(&p.generation) != gen {
		return nil, false, nil, true
	}
	// The window ends on a group head, so the sequential decoder would
	// resume on an IDR either way; rebuild it fresh here so later
	// sequential steps never inherit pre-window reference state.
	p.resetDecoderLocked()
	for j := range jobs {
		for k, pic := range results[j].pics {
			if pic == nil {
				continue
			}
			idx := jobs[j].base + k
			pts := base0 + epoch + (samples[idx].PTSMs - base0)
			p.pending = append(p.pending, &pendingPic{pic: pic, pts: pts, spos: idx})
		}
	}
	tail := s1 == len(samples)
	if !tail {
		for len(p.pending) > depth {
			best := 0
			for i := 1; i < len(p.pending); i++ {
				if p.pending[i].pts < p.pending[best].pts ||
					(p.pending[i].pts == p.pending[best].pts && p.pending[i].spos < p.pending[best].spos) {
					best = i
				}
			}
			emitted = append(emitted, p.pending[best])
			p.pending = append(p.pending[:best], p.pending[best+1:]...)
		}
		atomic.AddInt64(&p.s2windows, 1)
		return emitted, false, nil, true
	}
	ordered := p.pending
	p.pending = nil
	for len(ordered) > 0 {
		best := 0
		for i := 1; i < len(ordered); i++ {
			if ordered[i].pts < ordered[best].pts ||
				(ordered[i].pts == ordered[best].pts && ordered[i].spos < ordered[best].spos) {
				best = i
			}
		}
		emitted = append(emitted, ordered[best])
		ordered = append(ordered[:best], ordered[best+1:]...)
	}
	atomic.AddInt64(&p.s2windows, 1)
	return emitted, true, nil, true
}
