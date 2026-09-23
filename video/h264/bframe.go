package h264

// B frame-DAG planner: group-internal frame threading without touching
// the pixel engine.
//
// Say it plain: frames inside one IDR group reference each other, so
// they cannot all run at once — but most only wait for a few anchors.
// This planner walks the group's slice headers (cheap Exp-Golomb,
// no pixel work) with a REAL decoder holding DUMMY pictures, reusing
// the exact POC/marking/list/DPB code the pixel path runs. Each frame
// gets its dependency set (which group frames must finish first) and
// its DPB snapshot (the reference roster in order). Workers then decode
// frames on private decoders primed with that snapshot; output is
// bit-identical to sequential decode by construction (same code, same
// state, only the execution order changes).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/pthread_frame.c:645 ff_thread_await_progress (our Refs
//   wait channels), :910 ff_frame_thread_init thread_count (our
//   executor's worker cap mirrors S2), :949 delay = thread_count-1
//   (we need no delay line: whole-group DAG, emit by PTS rule).
// Difference: ffmpeg waits per MB-row on shared decoder state; we wait
// per whole frame on private decoders (coarser, but zero shared
// mutable state — no atomics in the pixel path, race-free by design).
// Ours: this file (plan) + PrimeFrame (prime) + video/bframe.go
// (execute). Any plan/execute surprise falls back to sequential replay
// (same S2 contract), so dirty streams keep today's behavior bit for
// bit. Set GPUI_B_OFF=1 to force sequential (same switch family as
// GPUI_SCALAR_CONVERT).

import (
	"os"
)

// POCSeed carries the display-order accumulator across a task boundary:
// the shadow values just before this frame, so the worker derives the
// identical POC through the same fixPOC code.
type POCSeed struct {
	MSB     int32
	PrevLSB int32
	Have    bool
	FnOff   int64
	FnPrev  uint32
	FnHave  bool
}

// FramePlan is one frame's parallel contract: wait for Refs (group
// frame indices), then decode with the DPB holding Snapshot (group
// frame indices, in roster order) and the POC accumulator at Seed.
// Roster/NextSeed describe the state AFTER this frame (post-store
// roster order + POC accumulator), so a later lap — or a sequential
// resume mid-group — can prime an exact continuation.
type FramePlan struct {
	Fed      bool // sample fed a picture (else skip: no task, no output)
	IsIDR    bool
	Refs     []int
	Snapshot []int
	Seed     POCSeed
	Roster   []int
	NextSeed POCSeed
}

// bOff reports the GPUI_B_OFF kill switch (evaluated per plan, not at
// init, so tests can pin both shapes in one binary).
func bOff() bool {
	switch os.Getenv("GPUI_B_OFF") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}

// PlanFrameGroup walks one IDR group's frames (raw NALUs per frame, VCL
// plus pass-through non-VCL) and returns one plan per frame. ok=false
// means "decode sequentially" (anything the planner does not model:
// mid-stream parameter sets, split frames, header surprises — the
// sequential path then surfaces them exactly as today, so behavior
// stays bit-identical either way).
//
// The planner builds a private parameter set from avcc plus the first
// frame's in-band sets (the standard muxing: SEI/SPS/PPS/SEI/IDR);
// in-band sets past the first frame fall back (mid-stream state
// change, rare and not worth modelling).
func PlanFrameGroup(avcc *AVCC, frames [][][]byte) ([]FramePlan, bool) {
	if bOff() || len(frames) == 0 || avcc == nil {
		return nil, false
	}
	ps := NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		return nil, false
	}
	sh := NewDecoder(ps)
	dummies := make([]*Picture, len(frames))
	byPtr := make(map[*Picture]int, len(frames))
	plans := make([]FramePlan, len(frames))
	for f, units := range frames {
		// Classify units without full parsing (cheap first-byte peek).
		// In-band parameter sets ride along in frame 0 (standard
		// muxing); the shadow absorbs them exactly like the pixel
		// path, so later walks parse with identical sets.
		var vcl [][]byte
		for _, u := range units {
			_, _, typ, err := NALUHeader(u)
			if err != nil {
				return nil, false
			}
			switch typ {
			case NALSPS, NALPPS:
				if f != 0 {
					// Mid-stream state change: stay sequential.
					return nil, false
				}
				if _, err := sh.ps.AddNALU(u); err != nil {
					return nil, false
				}
			case NALSliceNonIDR, NALSliceIDR:
				vcl = append(vcl, u)
			}
		}
		if len(vcl) == 0 {
			continue // unfed sample: no task, no state change
		}
		seed := POCSeed{
			MSB: sh.pocMSB, PrevLSB: sh.pocPrevLSB, Have: sh.pocHave,
			FnOff: sh.fnOffset, FnPrev: sh.fnPrev, FnHave: sh.fnHave,
		}
		// Snapshot = roster order at frame start (after the previous
		// frame's store, before this frame's marking — the worker
		// replays this frame's own marking itself during real decode).
		snapshot := append([]*Picture(nil), sh.dpb.pics...)
		var refs []*Picture
		isIDR := false
		curRef := false
		var lastH *SliceHeader
		var lastSPS *SPS
		for si, u := range vcl {
			ppsID, err := peekSlicePPSID(u)
			if err != nil {
				return nil, false
			}
			pps, sps, err := sh.ps.RequireForSlice(ppsID)
			if err != nil {
				return nil, false
			}
			if si == 0 {
				if sh.sps == nil {
					sh.sps = sps
				}
			}
			h, _, err := ParseSliceHeader(u, pps, sps)
			if err != nil {
				return nil, false
			}
			if si == 0 && h.FirstMB != 0 {
				// Frame split across samples: not modelled.
				return nil, false
			}
			// Per-slice assignments mirror the pixel path (last slice
			// wins for FrameNum/POC/IsIDR/ref flag). fixPOC rewrites
			// h.POC in place (low bits -> display order): the SHADOW
			// walk must advance the accumulator EXACTLY like the pixel
			// path — which is this same sequence — so no re-derivation
			// gap. (A past draft re-parsed headers for dummy numbering
			// and double-advanced MSB; the Seed/NextSeed gate pins the
			// single-advance rule: NextSeed[f] == Seed[f+1].)
			isIDR = h.IsIDR
			curRef = h.NalRefIDC != 0
			sh.fixPOC(h, sps)
			sh.fixPOCType12(h, sps)
			// Lists decode from the pre-marking roster (spec 8.2.5):
			// this frame's own marking applies after, in shadowStore.
			var l0, l1 []*Picture
			var lerr error
			switch {
			case h.IsP() && !h.IsIDR:
				l0, lerr = sh.buildRefList0(h)
			case h.IsB():
				l0, l1, lerr = sh.buildRefListsB(h)
			}
			if lerr != nil {
				// Long-term refs, duplicate POC, bad reorder: the pixel
				// path errors identically — stay sequential so it
				// surfaces there exactly as today.
				return nil, false
			}
			for _, p := range l0 {
				if p != nil {
					refs = append(refs, p)
				}
			}
			for _, p := range l1 {
				if p != nil {
					refs = append(refs, p)
				}
			}
			lastH, lastSPS = h, sps
		}
		if lastH == nil || lastSPS == nil {
			return nil, false
		}
		plan := FramePlan{Fed: true, IsIDR: isIDR, Seed: seed}
		seen := make(map[int]bool, len(refs))
		for _, p := range refs {
			fi, ok := byPtr[p]
			if !ok {
				// References outside the walked group (or self): the
				// pixel path would fail readable — stay sequential.
				return nil, false
			}
			if !seen[fi] {
				seen[fi] = true
				plan.Refs = append(plan.Refs, fi)
			}
		}
		for _, p := range snapshot {
			fi, ok := byPtr[p]
			if !ok {
				return nil, false
			}
			plan.Snapshot = append(plan.Snapshot, fi)
		}
		// Dummy numbering = display values (FrameNum from the header
		// as parsed for ordering; POC low bits + wrapped MSB — the
		// pixel path stores d.pic.POC, which fixPOC rewrote in place
		// on lastH above, so lastH.POC here is already the display
		// order the eviction/marking paths compare).
		dummy := &Picture{
			FrameNum: lastH.FrameNum,
			POC:      lastH.POC,
			IsIDR:    isIDR,
		}
		dummies[f] = dummy
		byPtr[dummy] = f
		if err := shadowStore(sh, lastSPS, dummy, curRef, lastH); err != nil {
			return nil, false
		}
		// Post-store continuation state (roster order + POC
		// accumulator) for exact mid-group resumes.
		for _, p := range sh.dpb.pics {
			fi, ok := byPtr[p]
			if !ok {
				return nil, false
			}
			plan.Roster = append(plan.Roster, fi)
		}
		plan.NextSeed = POCSeed{
			MSB: sh.pocMSB, PrevLSB: sh.pocPrevLSB, Have: sh.pocHave,
			FnOff: sh.fnOffset, FnPrev: sh.fnPrev, FnHave: sh.fnHave,
		}
		plans[f] = plan
	}
	return plans, true
}

// shadowStore replays FinishPicture's buffer side (marking + store,
// no pixels, no motion archive) on the shadow roster: adaptive executes
// its ops on the old roster, non-adaptive slides the window when full.
func shadowStore(sh *Decoder, sps *SPS, dummy *Picture, curRef bool, h *SliceHeader) error {
	if h != nil && h.AdaptiveMarking {
		for _, m := range h.MMCO {
			switch m.Op {
			case 1:
				sh.dpb.unmarkShort(uint32(m.Arg1))
			case 2:
			default:
			}
		}
	} else if curRef && sps != nil && sps.NumRefFrames > 0 {
		bits, err := frameNumBits(sps)
		if err != nil {
			return err
		}
		maxPicNum := int64(1) << uint(bits)
		for sh.dpb.Len() >= int(sps.NumRefFrames) {
			sh.dpb.evictOldest(dummy.FrameNum, maxPicNum)
		}
	}
	sh.dpb.Store(dummy, curRef)
	return nil
}
