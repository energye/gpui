package h264

import (
	"fmt"
	"sort"
)

// Picture is one decoded 8-bit 4:2:0 frame in raster order.
// Luma is Width x Height; each chroma plane is half width and height.
// Stride equals plane width (no padding) in this stage.
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
func (d *DPB) Store(p *Picture, isRef bool) {
	if d == nil || p == nil {
		return
	}
	if p.IsIDR {
		d.pics = d.pics[:0]
	}
	if !isRef {
		return
	}
	d.pics = append(d.pics, p)
	for len(d.pics) > d.max {
		d.pics = d.pics[1:]
	}
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
	byAge := append([]*Picture(nil), d.dpb.pics...)
	sort.Slice(byAge, func(i, j int) bool { return byAge[i].FrameNum > byAge[j].FrameNum })
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
	out := &Picture{Width: w, Height: h}
	out.Y = make([]uint8, w*h)
	for y := uint32(0); y < h; y++ {
		copy(out.Y[y*w:(y+1)*w], p.Y[y*p.Width:y*p.Width+w])
	}
	cw, ch := w/2, h/2
	pw := p.Width / 2
	out.Cb = make([]uint8, cw*ch)
	out.Cr = make([]uint8, cw*ch)
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
func (p *Picture) archiveMotion(d *Decoder) {
	if p == nil || d == nil {
		return
	}
	w4, h4 := d.mbW*4, d.mbH*4
	n := w4 * h4
	p.MotW4, p.MotH4 = w4, h4
	p.MV0x = append([]int16(nil), d.mvX...)
	p.MV0y = append([]int16(nil), d.mvY...)
	p.Rf0 = append([]int8(nil), d.refIdx...)
	p.MV1x = append([]int16(nil), d.mvX1...)
	p.MV1y = append([]int16(nil), d.mvY1...)
	p.Rf1 = append([]int8(nil), d.refIdx1...)
	p.UseM = append([]uint8(nil), d.useM...)
	_ = n
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

// evictOldest drops the smallest-FrameNum buffered picture: the
// sliding-window mark when the buffer already holds its maximum.
func (d *DPB) evictOldest() {
	if d == nil || len(d.pics) == 0 {
		return
	}
	m := 0
	for i := range d.pics {
		if d.pics[i].FrameNum < d.pics[m].FrameNum {
			m = i
		}
	}
	d.pics = append(d.pics[:m], d.pics[m+1:]...)
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
