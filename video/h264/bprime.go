package h264

// B worker prime: one task's starting state on a reused decoder.
//
// PrimeFrame installs the planner's snapshot (reference roster in
// order), the POC accumulator seed, and zeroes the motion scratch the
// per-frame reset block does not cover. It also zeroes the CABAC
// context carry (cabCtx/lastQPD/lastQPDHit/sliceQP): single-slice
// frames re-init identically either way, and multi-slice frames must
// not inherit a predecessor's second-slice context. After it, feeding
// the frame's NALUs runs the unmodified pixel path to bit-identical
// output: same DPB order, same POC derivation, same marking, same
// lists.
//
// Reset gap the resume path must cover: buffers written per FRAME but
// never reset per frame — cabCtx/lastQPD/lastQPDHit/sliceQP (CABAC
// context, rebuilt per slice by cabacInitSlice — multi-slice frames
// share one CABAC engine in the ORIGINAL path) stay on the primed
// worker through decodeSliceCabac; the pixel path re-inits per slice
// identically, so no prime gap. pocMSB/fnOffset ride the Seed.

// PrimeFrame primes one task: snapshot holds live reference pictures
// in roster order (aligned with the plan's Snapshot indices).
//
// Ownership: the worker takes one share of every snapshot picture
// (Retain) and drops its previous roster first, so a reused pool
// decoder never pins the last task's frames. The caller must keep its
// own shares alive for the call (stored/carry roster); the worker
// releases its shares at task teardown (DropBuffered).
//
// Order matters: retain-new BEFORE releasing old. Consecutive tasks
// usually share anchors (same picture in both rosters) — releasing
// first would drop a shared picture to zero, recycle it into the pool,
// and hand its buffers to another frame while this roster still points
// at them. Retaining first keeps shared pictures above zero throughout
// the swap (caught by TestBPlanPrimeMatchesSequential on 1080p).
func (d *Decoder) PrimeFrame(snapshot []*Picture, seed POCSeed) {
	if d.dpb == nil {
		d.dpb = NewDPB(16)
	}
	for _, q := range snapshot {
		q.Retain()
	}
	for _, q := range d.dpb.pics {
		q.Release()
	}
	d.dpb.pics = append(d.dpb.pics[:0], snapshot...)
	d.pocMSB, d.pocPrevLSB, d.pocHave = seed.MSB, seed.PrevLSB, seed.Have
	d.fnOffset, d.fnPrev, d.fnHave = seed.FnOff, seed.FnPrev, seed.FnHave
	d.lastQPD, d.lastQPDHit, d.sliceQP = 0, false, 0
	for i := range d.cabCtx {
		d.cabCtx[i] = 0
	}
	// Fresh-picture invariant: the pixel path allocates d.pic lazily
	// on the first slice of a frame (nil => NewPicture). A reused pool
	// decoder arrives with d.pic pointing at the PREVIOUS task's
	// picture (post-FinishPicture d.pic is nilled — except every
	// FinishPicture ends with d.pic = nil; EXCEPT the disposable path
	// also nils. Belt-and-braces: nil it here so no task ever decodes
	// into a stale picture).
	d.pic = nil
	d.decoded = 0
	d.slices = 0
	d.pendMark = d.pendMark[:0]
	d.pendAdapt = false
	// Residual-scratch audit (decoder fields the per-frame reset does
	// NOT cover, enumerated 2026-09-21 by grepping d.* writes outside
	// the reset block + init): every field below is written before
	// read inside one frame's decode (intra modes/modes, nnz grids,
	// qps/fIDC/fA/fB per-MB, mv/mvd/refIdx/useM/direct4/mbDirect per
	// partition, mbSlice/mbT8/skipped/mbI16/cmode/cbpArr per MB,
	// qpY/cOff/weights/scaling per slice, curIsRef/curIsB/slices/
	// decoded/skipCnt/skipRun/cab per frame, DPB/POC via snapshot+seed
	// above) — EXCEPT the cross-frame CABAC context zeroed above and
	// the sticky fault/param/spec layer (lastSEI/sps/ps, primed from
	// the same avcc on every worker). A field added later that breaks
	// this contract breaks the PrimeFrame gate (bframe_test.go pins
	// bit-exactness on 10 clips + ENERGY head + race), not silently.
	for i := range d.refTmp {
		d.refTmp[i] = 0
	}
	for i := range d.refTmp1 {
		d.refTmp1[i] = 0
	}
	for i := range d.mvX {
		d.mvX[i] = 0
	}
	for i := range d.mvY {
		d.mvY[i] = 0
	}
	for i := range d.mvdX {
		d.mvdX[i] = 0
	}
	for i := range d.mvdY {
		d.mvdY[i] = 0
	}
	for i := range d.mvX1 {
		d.mvX1[i] = 0
	}
	for i := range d.mvY1 {
		d.mvY1[i] = 0
	}
	for i := range d.refIdx {
		d.refIdx[i] = -1
	}
	for i := range d.refIdx1 {
		d.refIdx1[i] = -1
	}
}

// StoredRef returns the aligned reference picture FinishPicture just
// stored (nil for disposable frames). Parallel executors share it as
// snapshot input; it is never mutated after publication (pixel reads
// only), so sharing across workers is race-free.
func (d *Decoder) StoredRef() *Picture {
	if d == nil {
		return nil
	}
	return d.lastStored
}

// DPBList exposes the live reference roster for audits (read-only
// view; callers must not mutate the pictures).
func (d *Decoder) DPBList() []*Picture {
	if d == nil || d.dpb == nil {
		return nil
	}
	return d.dpb.pics
}
