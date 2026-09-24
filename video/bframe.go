package video

// B player wiring: frame threading inside one long IDR group, one lap
// may straddle the next IDR boundary.
//
// Say it plain: S2 already runs whole groups on workers, but a group of
// 250 frames still decodes on one worker (nothing inside may run
// alone). B splits that group by reference DAG: a frame starts as soon
// as its anchors finish, on a private decoder primed with its reference
// roster. P backbones stay ordered, B seas run wide. Near a group tail
// the lap keeps walking into the next IDR group (its head frame waits
// on nothing, so both sides run together) and emits across the boundary
// in order; the roster rekeys to the new group at commit.
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
	if pos < gHead || pos >= gEnd {
		return nil, false, nil, false
	}
	// Total work gates the lap, not the current group's tail: near the
	// boundary the segment straddles into the next IDR group (span),
	// so a short tail still runs with the next head. A short final
	// tail (no next group) stays sequential.
	if len(samples)-pos < bMinRemain {
		return nil, false, nil, false
	}
	return p.decodeBFrame(starts, g, pos, gen, workers, yuv)
}

// decodeBFrame decodes [pos, segEnd) in parallel (execution may reach
// execEnd for forward-edge cover), emitting decodeStep's exact rule.
// didParallel=false means nothing was claimed: run S2/sequential.
// A generation change mid-lap discards the work (the seeker owns the
// needle now) and reports didParallel=true with no emissions.
//
// starts/g locate the group holding pos; the segment may straddle into
// the next IDR group (span): the next head is an IDR (empty wait set),
// so both sides run on the one crew and emit in order. Groups stay
// independent (IDR clears the DPB, same contract as the S2 kernel), so
// no frame ever waits across the boundary.
func (p *Player) decodeBFrame(starts []int, g, pos int, gen int64, workers, yuv int) (emitted []*pendingPic, done bool, err error, didParallel bool) {
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
	p.dmu.Unlock()

	if g < 0 || g >= len(starts) || pos < 0 || pos >= len(samples) {
		return nil, false, nil, false
	}
	gHead := starts[g]
	gEnd := len(samples)
	if g+1 < len(starts) {
		gEnd = starts[g+1]
	}
	if gHead < 0 || gEnd > len(samples) || gHead >= gEnd || pos < gHead || pos >= gEnd {
		return nil, false, nil, false
	}
	// Execution budget from pos (frames), same caps as ever: the span
	// shares one budget across the boundary, so transient memory stays
	// flat whether the lap straddles or not.
	budget := bExecCapFrames
	if yuv > 0 {
		if bf := int(bExecCapBytes / int64(yuv)); bf < budget {
			budget = bf
			if budget <= 0 {
				return nil, false, nil, false
			}
		}
	}
	maxSpan := pos + budget
	if maxSpan > len(samples) {
		maxSpan = len(samples)
	}
	if maxSpan <= pos {
		return nil, false, nil, false
	}

	// Segment: emit [pos, segEnd). Execution may extend past it for
	// forward-edge cover (below). Segments are sized to cover the
	// reorder tail (S2's windows cover depth+group for the same
	// reason): a lap that cannot satisfy the emit rule emits nothing
	// and must not claim — the sequential path grows pending until the
	// rule fires, then B resumes. segEnd may pass gEnd (span); the walk
	// below covers every executed frame either way.
	segEnd := pos + bSegFrames
	if segEnd > maxSpan {
		segEnd = maxSpan
	}
	if segEnd-pos <= depth {
		return nil, false, nil, false
	}
	maxExec := maxSpan
	span := segEnd > gEnd
	if !span {
		// Single-group lap, exactly today's shape: the walk ends at
		// the execution budget (or sooner at the group end). A short
		// tail without a crossing stays sequential (setup costs more
		// than the win, same family as bMinRemain).
		if gEnd-pos < bMinRemain {
			return nil, false, nil, false
		}
		if maxExec > gEnd {
			maxExec = gEnd
		}
	}
	// Plan horizon: the walk plans one extra budget past the emit
	// point WITHOUT executing it (headers + shadow DPB only, no
	// pixels, no workers). The commit keeps the already-decoded
	// pictures the next lap's snapshots need; without this lookahead
	// a cap-bound lap (segEnd == maxExec, e.g. 1080p's 21-frame YUV
	// cap) keeps nothing, the next audit fails deterministically, and
	// every later lap burns a full walk just to fall back. Planning is
	// header-cheap; executing stays capped by maxExec above.
	planEnd := segEnd + budget
	if planEnd > len(samples) {
		planEnd = len(samples)
	}
	if !span && planEnd > gEnd {
		planEnd = gEnd
	}
	if planEnd < maxExec {
		planEnd = maxExec
	}
	// Span lap: the walk covers [gHead, gEnd) plus the next group's
	// head section [gEnd, planEnd). The second group is planned from
	// its own head (group-head shaped, IDR clears), so its frames wait
	// only inside their own group. Execution still ends at maxExec.

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
	ins := make([]frameIn, 0, planEnd-walkFrom)
	for i := walkFrom; i < planEnd; i++ {
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
	// Plan the walk. Sector A is the group-head shaped plan over
	// [gHead, walkAEnd): full DPB/POC state is known at gHead, and
	// sub-laps chain through held (audited carry below), so every wait
	// resolves to held (pre-window, complete) or doneCh (window,
	// completing). No cross-lap mutable state — held is rebuilt from
	// completed pictures each commit. A span lap appends sector B
	// [gEnd, planEnd) planned from its own head (next IDR clears, so B
	// waits only inside B); nextHead/sectB mark that second sector
	// (sectA is plans itself, walk-relative to gHead). The walk runs
	// to planEnd (one budget past the emit point) so the commit can
	// keep the next lap's references; execution still ends at maxExec.
	walkAEnd := planEnd
	if span && walkAEnd > gEnd {
		walkAEnd = gEnd
	}
	raws := make([][][]byte, 0, walkAEnd-walkFrom)
	for _, in := range ins {
		if len(raws) >= walkAEnd-walkFrom {
			break
		}
		raws = append(raws, in.units)
	}
	plans, ok := h264.PlanFrameGroup(avcc, raws)
	if !ok {
		return nil, false, nil, false
	}
	nextHead, sectB := -1, []h264.FramePlan(nil)
	if span {
		nextHead = gEnd
		bRaw := make([][][]byte, 0, planEnd-gEnd)
		for _, in := range ins[gEnd-walkFrom:] {
			bRaw = append(bRaw, in.units)
		}
		if len(bRaw) == 0 {
			return nil, false, nil, false
		}
		var bok bool
		sectB, bok = h264.PlanFrameGroup(avcc, bRaw)
		if !bok {
			return nil, false, nil, false
		}
		// Sector B's first plan must be the IDR head (empty wait set
		// proves the boundary needs no cross-sector wait — the two
		// sectors are independent by construction).
		if len(sectB) == 0 || !sectB[0].Fed || !sectB[0].IsIDR ||
			len(sectB[0].Refs) != 0 || len(sectB[0].Snapshot) != 0 {
			return nil, false, nil, false
		}
		// In-band parameter sets ride on each group head (standard
		// muxing repeats avcc there). Workers pre-feed heads once and
		// skip in-band sets inside frames, so the two heads must carry
		// identical SPS/PPS — otherwise one sector would parse with
		// the other's tables. Anything else falls back (exact, just
		// less parallel).
		if !bSameHeadSets(ins[0].units, ins[gEnd-walkFrom].units) {
			return nil, false, nil, false
		}
		// Stitch sector B after A with walk-relative indices: B's
		// planner ran from its own head, so every index shifts by the
		// sector-A length. B waits stay inside B by construction
		// (fresh DPB); anything crossing back falls back.
		off := gEnd - gHead
		for _, pl := range sectB {
			for _, r := range pl.Refs {
				if r < 0 || r >= len(sectB) {
					return nil, false, nil, false
				}
			}
			for _, s := range pl.Snapshot {
				if s < 0 || s >= len(sectB) {
					return nil, false, nil, false
				}
			}
			cp := pl
			for i := range cp.Refs {
				cp.Refs[i] += off
			}
			for i := range cp.Snapshot {
				cp.Snapshot[i] += off
			}
			for i := range cp.Roster {
				cp.Roster[i] += off
			}
			// No cross-sector waits: B frames reference B only
			// (the head's empty set above proves the boundary).
			for _, r := range cp.Refs {
				if gHead+r < gEnd {
					return nil, false, nil, false
				}
			}
			for _, s := range cp.Snapshot {
				if gHead+s < gEnd {
					return nil, false, nil, false
				}
			}
			plans = append(plans, cp)
		}
		_ = nextHead
	}
	if len(plans) != len(ins) {
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
		// Stale roster from an old generation/group: no future lap
		// can use it (the audit keys on this group), so drop its
		// shares now — otherwise they pin pooled buffers forever.
		for _, q := range p.bHeld {
			q.Release()
		}
		p.bHeld = nil
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
	// Carry owns one share of every pre-window picture from here to
	// commit/abandon: the harvest decoder is torn down below and the
	// old held roster is replaced at commit, so borrowed pointers
	// alone would dangle into recycled buffers.
	for _, pic := range carry {
		pic.Retain()
	}
	// dropCarry releases carry shares on pre-spawn exits (no workers
	// or decoded pictures exist yet — the full abandon below covers
	// post-spawn exits).
	dropCarry := func() {
		for _, pic := range carry {
			pic.Release()
		}
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
					dropCarry()
					return nil, false, nil, false
				}
			}
			for _, s := range plans[f].Snapshot {
				switch {
				case gHead+s >= execEnd && gHead+s < maxExec:
					execEnd = gHead + s + 1
					extended = true
				case gHead+s >= maxExec:
					dropCarry()
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
		dropCarry()
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
	// abandon drops a lap's private shares without touching player
	// state (needle, pending, held): decoded display pictures, carry
	// shares, and worker reference rosters. Workers run to completion
	// first (wg.Wait above), so no task reads while we release.
	abandon := func() {
		for _, pic := range disp {
			pic.Release()
		}
		dropCarry()
		for _, dec := range pool {
			dec.DropBuffered()
		}
	}
	var wg sync.WaitGroup
	var needFallback atomic.Bool
	// Union wait lists: Snapshot ∪ Refs per frame, computed once. The
	// dispatcher walks them in order instead of every frame
	// allocating a dedup map.
	waits := make([][]int, n)
	for f := pos; f < execEnd; f++ {
		gf := f - gHead
		if gf < 0 || gf >= len(plans) || !plans[gf].Fed {
			continue
		}
		seen := make(map[int]bool, len(plans[gf].Snapshot)+len(plans[gf].Refs))
		for _, s := range plans[gf].Snapshot {
			if !seen[s] {
				seen[s] = true
				waits[gf] = append(waits[gf], s)
			}
		}
		for _, r := range plans[gf].Refs {
			if !seen[r] {
				seen[r] = true
				waits[gf] = append(waits[gf], r)
			}
		}
	}
	// Persistent crew (B2-b): one goroutine per worker for the whole
	// lap; tasks flow in frame order through a rendezvous channel. The
	// dispatcher below waits each frame's roster BEFORE handing it
	// out, so workers only decode — no per-frame goroutines, no
	// Gosched spin, no take/give mutex storm. Each worker owns its
	// decoder for the lap (PrimeFrame re-primes per task; the commit
	// drops all worker rosters at the end).
	type bTask struct {
		f, gf int
	}
	tasks := make(chan bTask)
	for w := range pool {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			dec := pool[w]
			// Snapshot buffer reused across this worker's tasks:
			// PrimeFrame copies the pointers into the DPB at call
			// time, so reslicing afterwards is safe.
			var snapBuf []*h264.Picture
			for t := range tasks {
				if needFallback.Load() {
					close(doneCh[t.f-gHead])
					continue
				}
				pl := plans[t.gf]
				if cap(snapBuf) < len(pl.Snapshot) {
					snapBuf = make([]*h264.Picture, len(pl.Snapshot))
				} else {
					snapBuf = snapBuf[:len(pl.Snapshot)]
				}
				ok := true
				for i, s := range pl.Snapshot {
					pic := stored[gHead+s-gHead]
					if pic == nil {
						ok = false
						break
					}
					snapBuf[i] = pic
				}
				var pic *h264.Picture
				var st *h264.Picture
				failed := !ok
				if !failed {
					dec.PrimeFrame(snapBuf, pl.Seed)
					for _, u := range ins[t.gf].units {
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
				}
				if !failed {
					var ferr error
					pic, ferr = dec.FinishPicture()
					if ferr != nil {
						failed = true
					} else {
						st = dec.StoredRef()
					}
				}
				if failed {
					needFallback.Store(true)
				} else {
					disp[t.f-gHead] = pic
					stored[t.f-gHead] = st
				}
				close(doneCh[t.f-gHead])
			}
		}(w)
	}
	// Ordered dispatch: frame order, waits resolved before handoff.
	// All waits point strictly earlier (planner invariant) at frames
	// already dispatched or pre-window-closed, so the dispatcher never
	// blocks on an unsent task — no wait cycle. A closed doneCh on the
	// fallback path unblocks dependents, which then skip via the
	// needFallback check (worker side and below).
	for f := pos; f < execEnd; f++ {
		gf := f - gHead
		if gf < 0 || gf >= len(plans) || !plans[gf].Fed {
			close(doneCh[f-gHead])
			continue
		}
		badWait := false
		for _, s := range waits[gf] {
			if gHead+s < gHead || gHead+s >= execEnd {
				badWait = true
				break
			}
			<-doneCh[gHead+s-gHead]
		}
		if badWait || needFallback.Load() {
			if badWait {
				needFallback.Store(true)
			}
			close(doneCh[f-gHead])
			continue
		}
		tasks <- bTask{f, gf}
	}
	close(tasks)
	wg.Wait()

	if needFallback.Load() {
		abandon()
		return nil, false, nil, false
	}
	p.dmu.Lock()
	defer p.dmu.Unlock()
	if atomic.LoadInt64(&p.generation) != gen {
		abandon()
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
		// Lookahead-only frames (at/past the executed horizon) are
		// retained opportunistically: their references decode on a
		// later lap, so a miss here is ignored — the next audit
		// fails closed if it truly needs the picture. Only the
		// executed horizon must account exactly.
		beyondExec := gHead+f >= maxExec
		for _, s := range plans[f].Snapshot {
			gi := gHead + s
			switch {
			case gi < pos:
				if pic := carry[gi]; pic != nil {
					keep[gi] = pic
				} else if !beyondExec {
					abandon()
					return nil, false, nil, false
				}
			case gi < execEnd:
				if stored[gi-gHead] != nil {
					keep[gi] = stored[gi-gHead]
				}
			default:
				// Planned but not executed (the planEnd lookahead
				// past maxExec): the next lap decodes these itself
				// when its window reaches them, so there is nothing
				// to keep. Under-retention is fail-closed (the next
				// audit falls back), never a wrong picture.
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
		abandon()
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
		// Non-appended display shares end here (pending owns the
		// appended ones; reference shares live in keep/workers).
		if idx < pos || idx >= segEnd {
			pic.Release()
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
			pic.Release()
			continue
		}
		pts := base0 + epoch + (samples[idx].PTSMs - base0)
		p.pending = append(p.pending, &pendingPic{pic: pic, pts: pts, spos: idx})
	}
	// Commit the roster swap: keep takes its own shares first, then the
	// old held roster and the carry shares drop (a kept picture never
	// hits zero in between — counts only move down after the new share
	// lands). Workers are done (their snapshots were borrowed from
	// stored/carry), so their rosters drop last.
	for _, pic := range keep {
		pic.Retain()
	}
	for _, pic := range p.bHeld {
		pic.Release()
	}
	p.bHeld = keep
	if span {
		// The needle crossed into the next IDR group: rekey the
		// roster to the new head so the next lap audits against it.
		// keep holds B-sector pictures only (stitching verified B
		// waits never cross back), keyed by global sample index.
		p.bHeldHead = gEnd
	} else {
		p.bHeldHead = gHead
	}
	p.bGen = gen
	for _, pic := range carry {
		pic.Release()
	}
	for _, dec := range pool {
		dec.DropBuffered()
	}
	p.pos = segEnd
	if span {
		atomic.AddInt64(&p.bSpans, 1)
	}
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
			dropDecoderPictures(nd)
			return false
		}
		if pic == nil {
			dropDecoderPictures(nd)
			return false
		}
		roster = append(roster, pic)
	}
	hd.d.PrimeFrame(roster, plans[gf].NextSeed)
	// The continuation decoder takes over: the old decoder's buffered
	// shares (roster + in-flight) drop — roster pictures stay alive by
	// their new worker shares plus keep/carry/pending counts.
	dropDecoderPictures(p.dec)
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
			dropDecoderPictures(nd)
			return nil, false
		}
		units, uerr := SplitUnits(p.codec, buf, p.avcc.LengthSize)
		if uerr != nil {
			dropDecoderPictures(nd)
			return nil, false
		}
		if rerr := RejectUnits(p.codec, units, s.Number, p.path); rerr != nil {
			dropDecoderPictures(nd)
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
					dropDecoderPictures(nd)
					return nil, false
				}
				if typ == h264.NALSPS || typ == h264.NALPPS {
					if derr := nd.DecodeNALU(u); derr != nil {
						dropDecoderPictures(nd)
						return nil, false
					}
				}
			}
		}
		for _, u := range units {
			if derr := nd.DecodeNALU(u); derr != nil {
				dropDecoderPictures(nd)
				return nil, false
			}
		}
		if disp, ferr := nd.FinishPicture(); ferr != nil {
			dropDecoderPictures(nd)
			return nil, false
		} else {
			// The display share ends here (already queued via
			// primeFirst/earlier laps); only reference pictures
			// travel forward via StoredRef below.
			disp.Release()
		}
		if st := hd.d.StoredRef(); st != nil {
			out[i] = st
		}
		if atomic.LoadInt64(&p.generation) != gen {
			dropDecoderPictures(nd)
			return nil, false
		}
	}
	// Carry owns one share of each harvested picture; the throwaway
	// decoder's shares drop with it. Display pictures decoded here are
	// already queued via primeFirst/earlier laps — only the reference
	// pictures travel forward (StoredRef is nil for disposables).
	for _, pic := range out {
		pic.Retain()
	}
	dropDecoderPictures(nd)
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

// bSameHeadSets reports whether two group heads carry identical
// in-band SPS/PPS (standard muxing repeats avcc on each head).
// Workers pre-feed heads once and skip in-band sets inside frames,
// so a span lap needs both heads to parse with identical tables.
func bSameHeadSets(aUnits, bUnits [][]byte) bool {
	collect := func(units [][]byte) [][]byte {
		var out [][]byte
		for _, u := range units {
			_, _, typ, err := h264.NALUHeader(u)
			if err != nil {
				continue
			}
			if typ == h264.NALSPS || typ == h264.NALPPS {
				out = append(out, u)
			}
		}
		return out
	}
	a, b := collect(aUnits), collect(bUnits)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
