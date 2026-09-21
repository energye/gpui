package h264

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// Picture is one decoded 8-bit 4:2:0 frame in raster order.
// Luma is Width x Height; each chroma plane is half width and height.
// Stride equals plane width (no padding) in this stage.
//
// Ownership (S1b-I reference-count pool): a picture's buffers are
// shared by up to five parties — the decoding decoder (d.pic), the
// reference buffer (DPB), the player's reorder queue (pending), B
// worker snapshots (PrimeFrame rosters), and cross-lap carry (bHeld).
// refs counts live owners atomically (B workers share across
// goroutines); the last Release returns the buffers to the pool.
// Owners: Retain on take, Release on drop. A nil picture is a no-op
// for both, so untested/test paths stay branch-free.
type Picture struct {
	Width    uint32
	Height   uint32
	Y        []uint8
	Cb       []uint8
	Cr       []uint8
	FrameNum uint32
	POC      int32
	IsIDR    bool
	// Motion archive for B direct mode: per-4x4 motion of both lists,
	// copied at FinishPicture while the decoder arrays are still alive.
	// Rf holds -1 for intra/empty slots. MotW4/MotH4 are grid extents;
	// nil grids mean no archive (synthetic pictures).
	MotW4 int
	MotH4 int
	MV0x  []int16
	MV0y  []int16
	Rf0   []int8
	MV1x  []int16
	MV1y  []int16
	Rf1   []int8
	UseM  []uint8

	refs int32 // live owners; 0 == in pool or fresh
}

// Retain takes one ownership of the picture.
func (p *Picture) Retain() {
	if p == nil {
		return
	}
	atomic.AddInt32(&p.refs, 1)
}

// Release drops one ownership; the last owner returns the buffers to
// the pool. Never touch the picture after Release.
func (p *Picture) Release() {
	if p == nil {
		return
	}
	if atomic.AddInt32(&p.refs, -1) != 0 {
		return
	}
	picPoolPut(p)
}

// picPoolKey identifies reusable buffers: pixel size. Motion-archive
// arrays ride along opportunistically (reused when the grid matches,
// else reallocated on the cold resolution-change path).
type picPoolKey struct {
	w, h uint32
}

var picPool = struct {
	sync.Mutex
	bySize map[picPoolKey][]*Picture
	total  int
}{bySize: make(map[picPoolKey][]*Picture)}

// picPoolPerKey caps cached frames per size, picPoolTotal caps the
// whole cache (resolution flapping never parks unbounded memory).
const picPoolPerKey = 16
const picPoolTotal = 64

func picPoolPut(p *Picture) {
	if p == nil || p.Width == 0 || p.Height == 0 {
		return
	}
	// Only pool sane decode sizes (tests use tiny synthetics; pooling
	// them is harmless but pointless — the cap bounds it either way).
	k := picPoolKey{w: p.Width, h: p.Height}
	picPool.Lock()
	defer picPool.Unlock()
	if len(picPool.bySize[k]) >= picPoolPerKey || picPool.total >= picPoolTotal {
		return
	}
	picPool.bySize[k] = append(picPool.bySize[k], p)
	picPool.total++
}

// acquirePicture hands out a blank (mid-grey) picture like NewPicture,
// reusing a pooled buffer when one fits. Decode always overwrites every
// pixel (FinishPicture enforces full coverage), so reuse is exact; the
// grey refill keeps the observable state identical to NewPicture either
// way. NewPicture stays fresh-allocating (public contract for tests and
// synthetic pictures); only the decode hot paths acquire.
func acquirePicture(w, h uint32) (*Picture, error) {
	if w == 0 || h == 0 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("%w: size %dx%d", ErrBadSPS, w, h)
	}
	if w%2 != 0 || h%2 != 0 {
		return nil, fmt.Errorf("%w: odd size %dx%d", ErrBadSPS, w, h)
	}
	k := picPoolKey{w: w, h: h}
	picPool.Lock()
	var p *Picture
	if q := picPool.bySize[k]; len(q) > 0 {
		p = q[len(q)-1]
		picPool.bySize[k] = q[:len(q)-1]
		picPool.total--
	}
	picPool.Unlock()
	if p == nil {
		fresh, err := NewPicture(w, h)
		if err != nil {
			return nil, err
		}
		// Fresh pictures carry refs=0 (NewPicture contract); the
		// decoder owns this one.
		atomic.StoreInt32(&fresh.refs, 1)
		return fresh, nil
	}
	// Pooled buffers skip the grey refill: decode overwrites every
	// pixel (FinishPicture enforces full MB coverage; Crop fully
	// copies), so the fill is ~2MB of dead stores per 1536x864 frame.
	// Stamps still clear (cheap, and read before write). Any
	// read-before-write would surface in the exact gates (they pin
	// every pixel of B/480p/720p/VR2 clips).
	p.FrameNum, p.POC, p.IsIDR = 0, 0, false
	p.MotW4, p.MotH4 = 0, 0
	atomic.StoreInt32(&p.refs, 1)
	return p, nil
}

// NewPicture allocates a blank (mid-grey) picture.
func NewPicture(w, h uint32) (*Picture, error) {
	if w == 0 || h == 0 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("%w: size %dx%d", ErrBadSPS, w, h)
	}
	if w%2 != 0 || h%2 != 0 {
		return nil, fmt.Errorf("%w: odd size %dx%d", ErrBadSPS, w, h)
	}
	p := &Picture{Width: w, Height: h}
	p.Y = make([]uint8, w*h)
	p.Cb = make([]uint8, w*h/4)
	p.Cr = make([]uint8, w*h/4)
	for i := range p.Y {
		p.Y[i] = 16
	}
	for i := range p.Cb {
		p.Cb[i] = 128
		p.Cr[i] = 128
	}
	return p, nil
}

// At returns the luma sample at (x, y).
func (p *Picture) At(x, y uint32) uint8 {
	if p == nil || x >= p.Width || y >= p.Height {
		return 0
	}
	return p.Y[y*p.Width+x]
}

// SetY writes one luma sample (clipped to the picture).
func (p *Picture) SetY(x, y uint32, v uint8) {
	if p == nil || x >= p.Width || y >= p.Height {
		return
	}
	p.Y[y*p.Width+x] = v
}

// DPB is the decoded picture buffer: short roster of decoded pictures
// for future inter prediction. IDR pictures flush it.
type DPB struct {
	pics []*Picture
	max  int
}

// NewDPB builds a buffer holding at most max pictures.
func NewDPB(max int) *DPB {
	if max < 1 {
		max = 1
	}
	if max > 16 {
		max = 16
	}
	return &DPB{max: max}
}

// Store adds a picture, flushing on IDR and evicting the oldest on overflow.
// Non-reference pictures (disposable) are not kept for prediction.
// The buffer takes one ownership of stored pictures (Retain); evicted,
// unmarked and flushed victims are released.
func (d *DPB) Store(p *Picture, isRef bool) {
	if d == nil || p == nil {
		return
	}
	if p.IsIDR {
		for _, q := range d.pics {
			q.Release()
		}
		d.pics = d.pics[:0]
	}
	if !isRef {
		return
	}
	p.Retain()
	d.pics = append(d.pics, p)
	for len(d.pics) > d.max {
		victim := d.pics[0]
		d.pics = d.pics[1:]
		victim.Release()
	}
}

// frameNumWrap orders short-term pictures across a frame_num wrap
// (spec 8.2.4.1, all-frame case): pictures numbered above the current
// one already wrapped around, so they count as older (negative).
// Sorting by raw FrameNum puts the pre-wrap elder first and silently
// corrupts every frame after the first wrap (oceans s17).
func frameNumWrap(fn, cur uint32, maxFN int64) int64 {
	f := int64(fn)
	if f > int64(cur) {
		return f - maxFN
	}
	return f
}

// buildRefList0 constructs reference list 0 for one P slice: buffered
// pictures newest-first, then the slice-header reordering steps, sized
// to RefL0Count. Unavailable slots stay nil so refFor fails readable
// instead of sampling a wrong picture. Reorder IDC 0/1 are short-term
// subtraction/addition (both legal in frames); field (frame/field IDC 1
// with field pics) and long-term (IDC 2) reorderings stop readable.
func (d *Decoder) buildRefList0(h *SliceHeader) ([]*Picture, error) {
	if d.sps == nil {
		return nil, fmt.Errorf("%w: slice without sequence sets", ErrMissingPPS)
	}
	n := int(h.RefL0Count)
	if n < 1 {
		n = 1
	}
	bits, err := frameNumBits(d.sps)
	if err != nil {
		return nil, err
	}
	maxPicNum := int64(1) << uint(bits)
	cur := h.FrameNum
	byAge := append([]*Picture(nil), d.dpb.pics...)
	// P slices order short-term by wrapped frame number (spec 8.2.4.3):
	// the picture whose number is closest behind the current one comes
	// first. Picture order is by PicNum, not decode order — do NOT fall
	// back to insertion order when the DPB holds fewer than max (a
	// short early roster still decodes the newest first; insertion
	// order would predict s100 from a grandparent and smear ±1 edges
	// exactly like oceans s100).
	sort.Slice(byAge, func(i, j int) bool {
		wi, wj := frameNumWrap(byAge[i].FrameNum, cur, maxPicNum), frameNumWrap(byAge[j].FrameNum, cur, maxPicNum)
		if wi != wj {
			return wi > wj
		}
		return byAge[i].FrameNum > byAge[j].FrameNum
	})
	list := make([]*Picture, 0, n)
	for _, p := range byAge {
		if len(list) >= n {
			break
		}
		list = append(list, p)
	}
	for len(list) < n {
		list = append(list, nil)
	}
	return d.applyRefMod(list, h.RefModL0, h.FrameNum, n)
}

// applyRefMod reshapes one initialised reference list with the
// slice-header reordering steps (short-term subtraction/addition;
// long-term stops readable). Missing targets leave a hole like the
// reference decoder's zeroed entry. P and B lists share it.
func (d *Decoder) applyRefMod(list []*Picture, ops []RefModOp, frameNum uint32, n int) ([]*Picture, error) {
	if len(ops) == 0 {
		return list, nil
	}
	bits, err := frameNumBits(d.sps)
	if err != nil {
		return nil, err
	}
	maxPicNum := int32(1) << uint(bits)
	pred := int32(frameNum)
	for index, op := range ops {
		if index >= n {
			break
		}
		var pic *Picture
		switch op.IDC {
		case 0, 1:
			diff := int32(op.Arg) + 1
			if diff > maxPicNum {
				return nil, fmt.Errorf("%w: abs_diff %d", ErrBadSliceHeader, op.Arg)
			}
			if op.IDC == 0 {
				pred = (pred - diff) % maxPicNum
			} else {
				pred = (pred + diff) % maxPicNum
			}
			if pred < 0 {
				pred += maxPicNum
			}
			for _, p := range d.dpb.pics {
				if int32(p.FrameNum) == pred {
					pic = p
					break
				}
			}
			if pic == nil {
				// Missing reorder target: leave a hole like the
				// reference decoder's zeroed entry.
				list[index] = nil
				continue
			}
		case 2:
			return nil, fmt.Errorf("%w: long-term reference reorder", ErrStageScope)
		default:
			return nil, fmt.Errorf("%w: ref mod idc %d", ErrBadSliceHeader, op.IDC)
		}
		i := index
		for i < n && (list[i] == nil || list[i].FrameNum != pic.FrameNum) {
			i++
		}
		if i >= n {
			i = n - 1
		}
		copy(list[index+1:i+1], list[index:i])
		list[index] = pic
	}
	return list, nil
}

// Crop returns the top-left w×h view as an owned copy (display size
// after sequence cropping; references keep the aligned original).
func (p *Picture) Crop(w, h uint32) (*Picture, error) {
	if p == nil {
		return nil, fmt.Errorf("%w: crop of nil picture", ErrBadSPS)
	}
	if w == 0 || h == 0 || w > p.Width || h > p.Height || w%2 != 0 || h%2 != 0 {
		return nil, fmt.Errorf("%w: crop %dx%d from %dx%d", ErrBadSPS, w, h, p.Width, p.Height)
	}
	out, err := acquirePicture(w, h)
	if err != nil {
		return nil, err
	}
	for y := uint32(0); y < h; y++ {
		copy(out.Y[y*w:(y+1)*w], p.Y[y*p.Width:y*p.Width+w])
	}
	cw, ch := w/2, h/2
	pw := p.Width / 2
	for y := uint32(0); y < ch; y++ {
		copy(out.Cb[y*cw:(y+1)*cw], p.Cb[y*pw:y*pw+cw])
		copy(out.Cr[y*cw:(y+1)*cw], p.Cr[y*pw:y*pw+cw])
	}
	return out, nil
}

// coloc returns the colocated block's motion for frame direct mode: the
// 4x4 at (x4, y4) in the first list-1 reference, both lists. ok=false
// when the archive is missing, so the caller treats the block as
// intra/outside.
func (p *Picture) coloc(x4, y4 int) (r0 int8, x0, y0 int16, r1 int8, x1, y1 int16, ok bool) {
	if p == nil || p.Rf0 == nil || p.MotW4 <= 0 || p.MotH4 <= 0 {
		return -1, 0, 0, -1, 0, 0, false
	}
	if x4 < 0 || y4 < 0 || x4 >= p.MotW4 || y4 >= p.MotH4 {
		return -1, 0, 0, -1, 0, 0, false
	}
	i := y4*p.MotW4 + x4
	return p.Rf0[i], p.MV0x[i], p.MV0y[i], p.Rf1[i], p.MV1x[i], p.MV1y[i], true
}

// archiveMotion snapshots the current picture's per-4x4 motion of both
// lists for later B direct-mode colocated reads. Only reference pictures
// call it; the decoder arrays are reused by the next picture.
//
// S1b-I: pooled arrays are reused in place when the grid matches (the
// common steady-resolution case) — no per-frame motion garbage.
func (p *Picture) archiveMotion(d *Decoder) {
	if p == nil || d == nil {
		return
	}
	w4, h4 := d.mbW*4, d.mbH*4
	n := w4 * h4
	p.MotW4, p.MotH4 = w4, h4
	if len(p.MV0x) != n {
		p.MV0x = append([]int16(nil), d.mvX...)
		p.MV0y = append([]int16(nil), d.mvY...)
		p.Rf0 = append([]int8(nil), d.refIdx...)
		p.MV1x = append([]int16(nil), d.mvX1...)
		p.MV1y = append([]int16(nil), d.mvY1...)
		p.Rf1 = append([]int8(nil), d.refIdx1...)
		p.UseM = append([]uint8(nil), d.useM...)
		_ = n
		return
	}
	copy(p.MV0x, d.mvX)
	copy(p.MV0y, d.mvY)
	copy(p.Rf0, d.refIdx)
	copy(p.MV1x, d.mvX1)
	copy(p.MV1y, d.mvY1)
	copy(p.Rf1, d.refIdx1)
	copy(p.UseM, d.useM)
	_ = n
}

// DropBuffered releases decoder-held pictures (the in-flight frame plus
// the reference roster) for teardown paths: worker teardown, sequential
// decoder replacement, harvest teardown. Display pictures already handed
// out keep their own counts and stay alive. Safe on nil/empty decoders.
func (d *Decoder) DropBuffered() {
	if d == nil {
		return
	}
	if d.pic != nil {
		p := d.pic
		d.pic = nil
		p.Release()
	}
	if d.dpb != nil {
		for _, q := range d.dpb.pics {
			q.Release()
		}
		d.dpb.pics = d.dpb.pics[:0]
	}
	d.lastStored = nil
	d.decoded = 0
	d.slices = 0
}

// fixPOC rebuilds the full display order for poc type 0 wrapping
// (spec 8.2.1.1): the header only carries low bits, high bits come from
// the previous reference picture. Other poc types keep the header stub
// until VR2d wires them. IDR resets the count.
func (d *Decoder) fixPOC(h *SliceHeader, sps *SPS) {
	if sps == nil || sps.POCType != 0 {
		return
	}
	lsb := h.POC
	if h.IsIDR {
		h.POC = lsb
		if h.NalRefIDC != 0 {
			d.pocMSB, d.pocPrevLSB, d.pocHave = 0, lsb, true
		}
		return
	}
	if !d.pocHave {
		h.POC = lsb
		return
	}
	bits, err := spsPOCLSB(sps)
	if err != nil {
		return
	}
	maxLSB := int32(1) << uint(bits)
	msb := d.pocMSB
	if lsb < d.pocPrevLSB && d.pocPrevLSB-lsb >= maxLSB/2 {
		msb = d.pocMSB + maxLSB
	} else if lsb > d.pocPrevLSB && lsb-d.pocPrevLSB > maxLSB/2 {
		msb = d.pocMSB - maxLSB
	}
	h.POC = msb + lsb
	if h.NalRefIDC != 0 {
		d.pocMSB, d.pocPrevLSB = msb, lsb
	}
}

// fixPOCType12 rebuilds poc type 1/2 display order from the wrap
// accumulator (spec 8.2.1.2/8.2.1.3, all-frame case): abs bumps by
// max_frame_num whenever the number goes backwards, poc = 2*abs for
// references (non-reference reference-identical streams like oceans
// never take the minus-1 branch, which only applies when the stream
// actually marks pictures disposable). IDR resets the count.
func (d *Decoder) fixPOCType12(h *SliceHeader, sps *SPS) {
	if sps == nil || sps.POCType == 0 {
		return
	}
	bits, err := frameNumBits(sps)
	if err != nil {
		return
	}
	maxFN := int64(1) << uint(bits)
	if h.IsIDR {
		d.fnOffset, d.fnPrev, d.fnHave = 0, h.FrameNum, true
		h.POC = int32(2 * int64(h.FrameNum))
		return
	}
	if !d.fnHave {
		d.fnOffset, d.fnPrev, d.fnHave = 0, h.FrameNum, true
		h.POC = int32(2 * int64(h.FrameNum))
		return
	}
	if h.FrameNum < d.fnPrev {
		d.fnOffset += maxFN
	}
	d.fnPrev = h.FrameNum
	h.POC = int32(2 * (d.fnOffset + int64(h.FrameNum)))
}

// buildRefListsB initialises both reference lists for one B slice from
// display order: list 0 takes past pictures (display order descending)
// then future ones (ascending); list 1 takes future first, then past.
// Each list is sized by its active count, reshaped by its own reordering
// steps, and padded with nil holes. Long-term pictures stop readable:
// this stage only tracks short-term frames.
func (d *Decoder) buildRefListsB(h *SliceHeader) (l0, l1 []*Picture, err error) {
	if d.sps == nil {
		return nil, nil, fmt.Errorf("%w: slice without sequence sets", ErrMissingPPS)
	}
	for _, m := range h.MMCO {
		if m.Op >= 3 {
			return nil, nil, fmt.Errorf("%w: long-term marking op %d", ErrStageScope, m.Op)
		}
	}
	if h.LongTermRef {
		return nil, nil, fmt.Errorf("%w: long-term IDR reference", ErrStageScope)
	}
	var past, future []*Picture
	for _, p := range d.dpb.pics {
		if p.POC < h.POC {
			past = append(past, p)
		} else if p.POC > h.POC {
			future = append(future, p)
		} else {
			return nil, nil, fmt.Errorf("%w: duplicate poc %d", ErrBadSliceHeader, h.POC)
		}
	}
	sort.Slice(past, func(i, j int) bool { return past[i].POC > past[j].POC })
	sort.Slice(future, func(i, j int) bool { return future[i].POC < future[j].POC })
	mk := func(order []*Picture, n int) []*Picture {
		if n < 1 {
			n = 1
		}
		out := make([]*Picture, 0, n)
		for _, p := range order {
			if len(out) >= n {
				break
			}
			out = append(out, p)
		}
		for len(out) < n {
			out = append(out, nil)
		}
		return out
	}
	l0 = mk(append(append([]*Picture(nil), past...), future...), int(h.RefL0Count))
	l1 = mk(append(append([]*Picture(nil), future...), past...), int(h.RefL1Count))
	if l0, err = d.applyRefMod(l0, h.RefModL0, h.FrameNum, len(l0)); err != nil {
		return nil, nil, err
	}
	if l1, err = d.applyRefMod(l1, h.RefModL1, h.FrameNum, len(l1)); err != nil {
		return nil, nil, err
	}
	return l0, l1, nil
}

// evictOldest drops the smallest-FrameNumWrap buffered picture: the
// sliding-window victim when the buffer already holds its maximum
// (spec 8.2.5.3). Raw FrameNum comparison evicts the newest right
// after a wrap and keeps stale pictures forever; the caller passes the
// picture being stored as cur.
func (d *DPB) evictOldest(curFN uint32, maxFN int64) {
	if d == nil || len(d.pics) == 0 {
		return
	}
	m := 0
	mw := frameNumWrap(d.pics[0].FrameNum, curFN, maxFN)
	for i := range d.pics {
		if w := frameNumWrap(d.pics[i].FrameNum, curFN, maxFN); w < mw {
			m, mw = i, w
		}
	}
	victim := d.pics[m]
	d.pics = append(d.pics[:m], d.pics[m+1:]...)
	victim.Release()
}

// unmarkShort drops one short-term picture by frame number (explicit
// marking op 1): missing targets are ignored like the reference decoder.
func (d *DPB) unmarkShort(frameNum uint32) {
	if d == nil {
		return
	}
	for i, p := range d.pics {
		if p.FrameNum == frameNum {
			d.pics = append(d.pics[:i], d.pics[i+1:]...)
			p.Release()
			return
		}
	}
}

// applyMarking runs one slice's explicit reference operations before
// list construction. Op 1 unmarks one short-term picture; op 2 without
// long-term storage is a no-op here. Sliding-window eviction still
// happens at store time in FinishPicture.
func (d *Decoder) applyMarking(h *SliceHeader) error {
	if h == nil || !h.AdaptiveMarking {
		return nil
	}
	for _, m := range h.MMCO {
		switch m.Op {
		case 1:
			d.dpb.unmarkShort(uint32(m.Arg1))
		case 2:
			return nil
		default:
			return nil
		}
	}
	return nil
}

// Latest returns the most recent reference picture.
func (d *DPB) Latest() *Picture {
	if d == nil || len(d.pics) == 0 {
		return nil
	}
	return d.pics[len(d.pics)-1]
}

// List0 returns up to n newest references, newest first.
func (d *DPB) List0(n int) []*Picture {
	if d == nil || n <= 0 || len(d.pics) == 0 {
		return nil
	}
	if n > len(d.pics) {
		n = len(d.pics)
	}
	out := make([]*Picture, n)
	for i := 0; i < n; i++ {
		out[i] = d.pics[len(d.pics)-1-i]
	}
	return out
}

// Len reports buffered pictures.
func (d *DPB) Len() int {
	if d == nil {
		return 0
	}
	return len(d.pics)
}

// ByPOC finds a buffered picture by picture order count.
func (d *DPB) ByPOC(poc int32) *Picture {
	if d == nil {
		return nil
	}
	for _, p := range d.pics {
		if p.POC == poc {
			return p
		}
	}
	return nil
}
