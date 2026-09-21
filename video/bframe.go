package video

// B player wiring: frame threading inside one long IDR group.
//
// Say it plain: S2 already runs whole groups on workers, but a group of
// 250 frames still decodes on one worker (nothing inside may run
// alone). B splits that group by reference DAG: a frame starts as soon
// as its anchors finish, on a private decoder primed with its reference
// roster. P backbones stay ordered, B seas run wide.
//
// Peer (ffmpeg, read-only, no vendoring): same pthread_frame.c notes as
// s2_player.go (thread_count rule, delay line) plus :645
// ff_thread_await_progress (our done channels per frame).
// Difference: ffmpeg waits per MB-row on shared state; we wait per
// whole frame on private decoders (video/h264 PlanFrameGroup +
// PrimeFrame), emit by the same PTS rule as decodeStep, and bound one
// lap to a segment so memory stays flat (S7 cap honored per lap).
//
// Exactness contract: a clean lap yields the same pictures in the same
// order as decodeStep sample by sample (same splitters, same param
// feed, same finish, same stamps, same pending emit rule). Any surprise
// (plan refusal, worker error, travelling generation) leaves the needle
// and pending untouched and reports ran=false, so the caller replays
// sequentially — dirty clips keep today's conceal path bit for bit.
// Workers touch no Player locks and no shared decoder state.

import (
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/video/h264"
)

// bMinRemain is the smallest remaining group worth threading: shorter
// tails stay on S2/sequential (setup costs more than the win).
const bMinRemain = 12

// bSegFrames bounds one lap's emitted frames (two reorder horizons:
// the emit rule holds depth+1 pictures back, so a segment shorter
// than that never emits). Forward reference edges (B to a later
// anchor) may extend the execution window a little past the emit
// point, capped by bExecCapFrames/bExecCapBytes below. The lookahead
// overlap re-decodes next lap (bounded, noted, accepted).
const bSegFrames = 32

// bExecCapFrames caps executed frames per lap (emit + lookahead).
const bExecCapFrames = 48

// bExecCapBytes caps executed YUV bytes per lap (same S7 family as
// s2WindowMaxBytes; big frames bind here first).
const bExecCapBytes = 64 << 20

// bWorkerCount mirrors s2WorkerCount (ffmpeg thread_count rule).
func bWorkerCount() int { return s2WorkerCount(0) }

// maybeDecodeBFrame runs one threaded segment when the needle sits
// inside a long group, else reports ran=false and the caller falls
// through to S2/sequential. Rides the S2Parallel opt-in (no new flag:
// same parallel family, same exactness contract).
func (p *Player) maybeDecodeBFrame(gen int64) (emitted []*pendingPic, done bool, err error, ran bool) {
	if p.buffered || !s2ParallelCodecOK(p.codec) || len(p.s2starts) < 1 {
		return nil, false, nil, false
	}
	atomic.AddInt64(&p.bTries, 1)
	workers := bWorkerCount()
	if workers <= 1 {
		return nil, false, nil, false
	}
	// While a seek travels, stay sequential (same reason as S2).
	p.dmu.Lock()
	travelling := p.seekActive
	p.dmu.Unlock()
	if travelling {
		return nil, false, nil, false
	}
	p.dmu.Lock()
	pos := p.pos
	starts := p.s2starts
	samples := p.samples
	yuv := p.s2yuv
	p.dmu.Unlock()
	// Locate the group holding pos.
	g := -1
	for i, s := range starts {
		if s > pos {
			break
		}
		g = i
	}
	if g < 0 {
		return nil, false, nil, false
	}
	gHead := starts[g]
	gEnd := len(samples)
	if g+1 < len(starts) {
		gEnd = starts[g+1]
	}
	if pos < gHead || pos >= gEnd || gEnd-pos < bMinRemain {
		return nil, false, nil, false
	}
	return p.decodeBFrame(gHead, gEnd, pos, gen, workers, yuv)
}

// decodeBFrame decodes [pos, segEnd) in parallel (execution may reach
// execEnd for forward-edge cover), emitting decodeStep's exact rule.
// didParallel=false means nothing was claimed: run S2/sequential.
// A generation change mid-lap discards the work (the seeker owns the
// needle now) and reports didParallel=true with no emissions.
func (p *Player) decodeBFrame(gHead, gEnd, pos int, gen int64, workers, yuv int) (emitted []*pendingPic, done bool, err error, didParallel bool) {
	samples := p.samples
	src := p.source
	avcc := p.avcc
	codec := p.codec
	epoch := p.epoch
	base0 := p.base0
	depth := p.reorderDepth
	path := p.path
	p.dmu.Lock()
	if atomic.LoadInt64(&p.generation) != gen {
		p.dmu.Unlock()
		return nil, false, nil, false
	}
	// S2 precedent: never claim into a deep queue (the display owns
	// those pictures; a parallel lap would double-queue or strand the
	// exact-resume chain). The sequential path drains first; B laps
	// resume when pending is at/below the reorder horizon.
	if len(p.pending) > depth+1 {
		p.dmu.Unlock()
		return nil, false, nil, false
	}
	if pos < 0 || pos >= len(samples) || gHead < 0 || gEnd > len(samples) || gHead >= gEnd || pos < gHead || pos >= gEnd {
		p.dmu.Unlock()
		return nil, false, nil, false
	}
	p.dmu.Unlock()

	// Segment: emit [pos, segEnd). Execution may extend past it for
	// forward-edge cover (below). Segments are sized to cover the
	// reorder tail (S2's windows cover depth+group for the same
	// reason): a lap that cannot satisfy the emit rule emits nothing
	// and must not claim — the sequential path grows pending until the
	// rule fires, then B resumes.
	segEnd := pos + bSegFrames
	if segEnd > gEnd {
		segEnd = gEnd
	}
	if segEnd-pos <= depth {
		return nil, false, nil, false
	}
	maxExec := pos + bExecCapFrames
	if maxExec > gEnd {
		maxExec = gEnd
	}
	if yuv > 0 && int64(bExecCapBytes)/int64(yuv) < int64(maxExec-pos) {
		maxExec = pos + int(bExecCapBytes/int64(yuv))
		if maxExec <= pos {
			return nil, false, nil, false
		}
	}
	if segEnd > maxExec {
		segEnd = maxExec
	}

	// Read blobs + split units sync (Source stays single-threaded).
	type frameIn struct {
		units [][]byte
		num   int
		fed   bool
	}
	// Walk from the group head; the planner entry is group-head
	// shaped (full DPB/POC state known there). Sub-laps within one
	// player lap chain through held (exact post-store roster), so any
	// needle inside the group can fan — every wait resolves to held
	// (pre-window, complete) or doneCh (window, completing).
	walkFrom := gHead
	ins := make([]frameIn, 0, maxExec-walkFrom)
	for i := walkFrom; i < maxExec; i++ {
		buf := make([]byte, samples[i].Size)
		if _, rerr := readSourceRange(src, buf, int64(samples[i].Offset)); rerr != nil {
			return nil, false, nil, false
		}
		units, uerr := SplitUnits(codec, buf, avcc.LengthSize)
		if uerr != nil {
			return nil, false, nil, false
		}
		if rerr := RejectUnits(codec, units, samples[i].Number, path); rerr != nil {
			// A fatal unit (F17 and friends) must surface loudly, not
			// fall back silently: decodeStep would return err here, so
			// the lap reports it the same way.
			if streamFatal(rerr) {
				return nil, false, rerr, true
			}
			return nil, false, nil, false
		}
		fed := false
		for _, u := range units {
			if typ, _ := h264.NALType(u); typ == h264.NALSliceNonIDR || typ == h264.NALSliceIDR {
				fed = true
				break
			}
		}
		ins = append(ins, frameIn{units: units, num: samples[i].Number, fed: fed})
	}
	// Group-head shaped entry (full DPB/POC state known at gHead);
	// sub-laps chain through held (audited carry below): every wait
	// resolves to held (pre-window, complete) or doneCh (window,
	// completing). No cross-lap mutable state — held is rebuilt from
	// completed pictures each commit.
	raws := make([][][]byte, len(ins))
	for i, in := range ins {
		raws[i] = in.units
	}
	plans, ok := h264.PlanFrameGroup(avcc, raws)
	if !ok {
		return nil, false, nil, false
	}
	// Held-sector audit: reconcile the retained roster against the
	// incoming window before any worker spawns. Pre-window members
	// must be present in held (keyed by sample index); in-window
	// members complete under doneCh. Anything else falls back with the
	// needle and held untouched (no half-built state leaks).
	// Cold start (held == nil, e.g. the opener consumed the head):
	// replay [gHead, pos) through a throwaway sequential decoder
	// (same code/feed — harvest output verified equal to live above).
	// Harvested pictures are immutable post-store.
	p.dmu.Lock()
	held := p.bHeld
	if p.bGen != gen || p.bHeldHead != gHead {
		held = nil
	}
	p.dmu.Unlock()
	if held == nil && pos > gHead {
		if harv, ok := p.harvestHeadLocked(gHead, pos, gen); ok {
			held = harv
		}
	}
	compatible, carry := bAuditHeld(gHead, pos, maxExec, plans, held)
	if !compatible {
		return nil, false, nil, false
	}
	// Extend execution over forward edges (union, bounded by maxExec —
	// the walk covers to maxExec; a crosser past it falls back so no
	// wait ever points outside the executed set). Only frames at or
	// past pos extend (the head re-decodes but never pulls the window
	// wider: its refs are all in-window by the planner invariant).
	// The second leg covers the keep roster below: snapshots of every
	// future frame in the walk must land inside the executed set, else
	// the commit would fall back with nothing to show for the work.
	// Both legs iterate to a fixed point (indices stay inside the walk
	// by the planner invariant, so no crosser can escape maxExec).
	execEnd := segEnd
	for {
		extended := false
		for f := pos - gHead; f < execEnd-gHead; f++ {
			if f < 0 || f >= len(plans) || !plans[f].Fed {
				continue
			}
			for _, r := range plans[f].Refs {
				switch {
				case gHead+r >= execEnd && gHead+r < maxExec:
					execEnd = gHead + r + 1
					extended = true
				case gHead+r >= maxExec:

					return nil, false, nil, false
				}
			}
			for _, s := range plans[f].Snapshot {
				switch {
				case gHead+s >= execEnd && gHead+s < maxExec:
					execEnd = gHead + s + 1
					extended = true
				case gHead+s >= maxExec:

					return nil, false, nil, false
				}
			}
		}
		for f := segEnd - gHead; f < len(plans); f++ {
			if f < 0 || f >= len(plans) || !plans[f].Fed {
				continue
			}
			for _, s := range plans[f].Snapshot {
				if gHead+s >= execEnd && gHead+s < maxExec {
					execEnd = gHead + s + 1
					extended = true
				}
			}
		}
		if !extended {
			break
		}
	}
	if execEnd <= pos || execEnd > maxExec {
		return nil, false, nil, false
	}
	// Stream ends only when the emit point reaches the last sample.
	tail := segEnd == len(samples)

	// Per-lap worker crew (fresh decoders: no cross-lap state, same
	// philosophy as S2's reset-per-window). Private param sets each
	// (AddNALU writes maps), group param feed up front.
	pool := make([]*h264.Decoder, workers)
	for w := range pool {
		wps := h264.NewParamSets()
		if err := wps.FromAVCC(avcc); err != nil {
			return nil, false, nil, false
		}
		dec := h264.NewDecoder(wps)
		for _, raw := range avcc.SPS {
			if err := dec.DecodeNALU(raw); err != nil {

				return nil, false, nil, false
			}
		}
		for _, raw := range avcc.PPS {
			if err := dec.DecodeNALU(raw); err != nil {

				return nil, false, nil, false
			}
		}
		// Group-head in-band sets (standard muxing repeats avcc here):
		// pre-feed so every worker parses with identical sets. Workers
		// skip in-band SPS/PPS inside their own frame below (fed once
		// here): re-feeding mid-lap mutates the param tables the plan
		// parsed against.
		for _, u := range ins[0].units {
			_, _, typ, terr := h264.NALUHeader(u)
			if terr != nil {

				return nil, false, nil, false
			}
			if typ == h264.NALSPS || typ == h264.NALPPS {
				if err := dec.DecodeNALU(u); err != nil {

					return nil, false, nil, false
				}
			}
		}
		pool[w] = dec
	}
	var poolMu sync.Mutex
	free := make([]bool, workers)
	for w := range free {
		free[w] = true
	}
	take := func() (int, *h264.Decoder) {
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

	n := execEnd - gHead
	doneCh := make([]chan struct{}, n)
	for i := range doneCh {
		doneCh[i] = make(chan struct{})
	}
	disp := make([]*h264.Picture, n)
	stored := make([]*h264.Picture, n)
	// Pre-window members ([gHead, pos)) are already decoded by the
	// sequential path or earlier laps: publish reference pictures
	// straight into stored (display pictures are immutable after
	// FinishPicture) so waits resolve uniformly through doneCh. The
	// audit above proved every NEEDED one is present in carry.
	// Disposable members store nil (never referenced — planner
	// invariant: snapshots hold reference pictures only).
	for gi := gHead; gi < pos; gi++ {
		gf := gi - gHead
		if gf < 0 || gf >= len(plans) || !plans[gf].Fed {
			close(doneCh[gf])
			continue
		}
		stored[gf] = carry[gi]
		close(doneCh[gf])
	}
	var wg sync.WaitGroup
	var needFallback atomic.Bool
	for f := pos; f < execEnd; f++ {
		gf := f - gHead
		if gf < 0 || gf >= len(plans) || !plans[gf].Fed {
			close(doneCh[f-gHead])
			continue
		}
		wg.Add(1)
		go func(f, gf int) {
			defer wg.Done()
			// Wait for the full roster (snapshot ∪ refs): all members
			// complete before the worker reads them. Indices run
			// strictly earlier (planner invariant): acyclic. Waiting
			// never holds a pool slot (acquired after), so deep
			// chains cannot deadlock the crew.
			waited := make(map[int]bool)
			wait := func(gx int) bool {
				if waited[gx] {
					return true
				}
				waited[gx] = true
				if gx < 0 || gHead+gx < gHead || gHead+gx >= execEnd {
					return false
				}
				<-doneCh[gHead+gx-gHead]
				return true
			}
			for _, s := range plans[gf].Snapshot {
				if !wait(s) {
					needFallback.Store(true)
					close(doneCh[f-gHead])
					return
				}
			}
			for _, r := range plans[gf].Refs {
				if !wait(r) {
					needFallback.Store(true)
					close(doneCh[f-gHead])
					return
				}
			}
			if needFallback.Load() {
				close(doneCh[f-gHead])
				return
			}
			w, dec := take()
			snap := make([]*h264.Picture, 0, len(plans[gf].Snapshot))
			for _, s := range plans[gf].Snapshot {
				pic := stored[gHead+s-gHead]
				if pic == nil {
					give(w)
					needFallback.Store(true)
					close(doneCh[f-gHead])
					return
				}
				snap = append(snap, pic)
			}
			dec.PrimeFrame(snap, plans[gf].Seed)
			var pic *h264.Picture
			failed := false
			for _, u := range ins[gf].units {
				_, _, typ, terr := h264.NALUHeader(u)
				if terr != nil {
					failed = true
					break
				}
				if typ == h264.NALSPS || typ == h264.NALPPS {
					continue
				}
				if derr := dec.DecodeNALU(u); derr != nil {
					failed = true
					break
				}
			}
			var st *h264.Picture
			if !failed {
				var ferr error
				pic, ferr = dec.FinishPicture()
				if ferr != nil {
					failed = true
				} else {
					st = dec.StoredRef()
				}
			}
			give(w)
			if failed {
				needFallback.Store(true)
			} else {
				disp[f-gHead] = pic
				stored[f-gHead] = st
			}
			close(doneCh[f-gHead])
		}(f, gf)
	}
	wg.Wait()

	if needFallback.Load() {
		return nil, false, nil, false
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	if atomic.LoadInt64(&p.generation) != gen {
		return nil, false, nil, true
	}
	// Retain the future roster FIRST (no player state touched): snapshots
	// of frames at or past the new needle, restricted to completed
	// pictures. Pre-window members come from carry (audited), in-window
	// from stored. Anything else falls back with the needle and pending
	// untouched (no half-built state leaks). This must precede the
	// pending append below: a keep hole must not strand appended
	// pictures on a fallback lap.
	keep := make(map[int]*h264.Picture)
	for f := segEnd - gHead; f < len(plans); f++ {
		if f < 0 {
			continue
		}
		for _, s := range plans[f].Snapshot {
			gi := gHead + s
			switch {
			case gi < pos:
				if pic := carry[gi]; pic != nil {
					keep[gi] = pic
				} else {
					return nil, false, nil, false
				}
			case gi < execEnd:
				if stored[gi-gHead] != nil {
					keep[gi] = stored[gi-gHead]
				}
			default:
				// Cannot exist (the extension pass covered the
				// window): fall back rather than keep a hole.
				return nil, false, nil, false
			}
		}
	}
	// Exact mid-group resume (S2 ends windows on group heads where a
	// fresh decoder is correct; B ends mid-group, so the continuation
	// decoder inherits the post-store roster + POC accumulator — same
	// DPB order, same derivation, bit-identical downstream). The
	// roster mixes in-window stored pictures with pre-window carry
	// (audited above).
	if !p.primeResumeLocked(gHead, segEnd, plans, carry, stored, pos, execEnd) {
		return nil, false, nil, false
	}
	for k, pic := range disp {
		if pic == nil {
			continue
		}
		idx := gHead + k
		// Only [pos, segEnd) appends (the head re-decode serves
		// reference continuity; its display pictures already queued
		// via primeFirst/earlier laps — appending again double-shows).
		// Deduplicate by SAMPLE (not PTS: PTS repeats across
		// loop/seek epochs are legal; spos is the decode-order key).
		if idx < pos || idx >= segEnd {
			continue
		}
		dup := false
		for _, q := range p.pending {
			if q.spos == idx {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		pts := base0 + epoch + (samples[idx].PTSMs - base0)
		p.pending = append(p.pending, &pendingPic{pic: pic, pts: pts, spos: idx})
	}
	p.bHeld = keep
	p.bHeldHead = gHead
	p.bGen = gen
	p.pos = segEnd
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
		atomic.AddInt64(&p.bWindows, 1)
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
	atomic.AddInt64(&p.bWindows, 1)
	return emitted, true, nil, true
}

// primeResumeLocked installs the exact continuation decoder after a
// clean lap: fresh shell, same params, DPB roster + POC accumulator
// primed to the post-store state at segEnd. stored is gHead-
// relative; carry covers pre-window members (audited). Caller holds
// dmu. False means fall back (needle untouched, sequential replays).
func (p *Player) primeResumeLocked(gHead, segEnd int, plans []h264.FramePlan, carry map[int]*h264.Picture, stored []*h264.Picture, pos, execEnd int) bool {
	gf := segEnd - 1 - gHead
	if gf < 0 || gf >= len(plans) || !plans[gf].Fed {
		return false
	}
	nd, err := NewDecoder(p.codec)
	if err != nil {
		return false
	}
	if err := feedParams(nd, p.avcc, p.path, ""); err != nil {
		return false
	}
	hd, ok := nd.(*h264Decoder)
	if !ok {
		return false
	}
	roster := make([]*h264.Picture, 0, len(plans[gf].Roster))
	for _, s := range plans[gf].Roster {
		gi := gHead + s
		var pic *h264.Picture
		switch {
		case gi < pos:
			pic = carry[gi]
		case gi < execEnd:
			pic = stored[gi-gHead]
		default:
			return false
		}
		if pic == nil {
			return false
		}
		roster = append(roster, pic)
	}
	hd.d.PrimeFrame(roster, plans[gf].NextSeed)
	p.dec = nd
	return true
}

// bAuditHeld reconciles the retained roster against the incoming
// window. It returns the carry map workers read pre-window members
// from: in-window frames resolve to their doneCh-gated slots (stored),
// pre-window frames to held. compatible=false falls back (needle and
// held untouched).
//
// Snapshot indices are walk-relative with ins[0] == gHead (the walk
// always starts at the group head), so index s means sample gHead+s.
func bAuditHeld(gHead, pos, maxExec int, plans []h264.FramePlan, held map[int]*h264.Picture) (bool, map[int]*h264.Picture) {
	carry := make(map[int]*h264.Picture, len(held))
	for gi, pic := range held {
		if gi >= gHead && gi < pos && pic != nil {
			carry[gi] = pic
		}
	}
	need := make(map[int]bool)
	for f, pl := range plans {
		gi := gHead + f
		if gi < pos || gi >= maxExec || !pl.Fed {
			continue
		}
		for _, s := range pl.Snapshot {
			if gHead+s < pos {
				need[gHead+s] = true
			}
		}
		for _, r := range pl.Refs {
			if gHead+r < pos {
				need[gHead+r] = true
			}
		}
	}
	for gi := range need {
		if carry[gi] == nil {
			return false, nil
		}
	}
	return true, carry
}

// harvestHeadLocked replays the opener-consumed head ([gHead, pos))
// through a throwaway sequential decoder and returns its stored
// reference pictures keyed by sample index. Same code, same feed as
// the pixel path — harvest output is bit-identical to what the opener
// decoded (verified by the oracle gates, not by trust). False falls
// back (nothing mutated).
func (p *Player) harvestHeadLocked(gHead, pos int, gen int64) (map[int]*h264.Picture, bool) {
	if gHead < 0 || pos <= gHead || pos > len(p.samples) {
		return nil, false
	}
	nd, err := NewDecoder(p.codec)
	if err != nil {
		return nil, false
	}
	if err := feedParams(nd, p.avcc, p.path, ""); err != nil {
		return nil, false
	}
	hd, ok := nd.(*h264Decoder)
	if !ok {
		return nil, false
	}
	out := make(map[int]*h264.Picture)
	for i := gHead; i < pos; i++ {
		s := p.samples[i]
		buf := make([]byte, s.Size)
		if _, rerr := readSourceRange(p.source, buf, int64(s.Offset)); rerr != nil {
			return nil, false
		}
		units, uerr := SplitUnits(p.codec, buf, p.avcc.LengthSize)
		if uerr != nil {
			return nil, false
		}
		if rerr := RejectUnits(p.codec, units, s.Number, p.path); rerr != nil {
			return nil, false
		}
		if !fedUnits(units) {
			continue
		}
		// In-band parameter sets ride on the GROUP HEAD sample
		// (standard muxing repeats avcc there); the harvest feeds
		// them like the worker pool does (same pre-feed rule), so
		// parsing state matches the planned walk exactly.
		if i == gHead {
			for _, u := range units {
				_, _, typ, terr := h264.NALUHeader(u)
				if terr != nil {
					return nil, false
				}
				if typ == h264.NALSPS || typ == h264.NALPPS {
					if derr := nd.DecodeNALU(u); derr != nil {
						return nil, false
					}
				}
			}
		}
		for _, u := range units {
			if derr := nd.DecodeNALU(u); derr != nil {
				return nil, false
			}
		}
		if _, ferr := nd.FinishPicture(); ferr != nil {
			return nil, false
		}
		if st := hd.d.StoredRef(); st != nil {
			out[i] = st
		}
		if atomic.LoadInt64(&p.generation) != gen {
			return nil, false
		}
	}
	return out, true
}

// fedUnits reports whether a sample carries slice data.
func fedUnits(units [][]byte) bool {
	for _, u := range units {
		if typ, _ := h264.NALType(u); typ == h264.NALSliceNonIDR || typ == h264.NALSliceIDR {
			return true
		}
	}
	return false
}
