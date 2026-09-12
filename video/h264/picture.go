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
// instead of sampling a wrong picture. Frames only: field (IDC 1) and
// long-term (IDC 2) reorderings stop readable for a later stage.
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
	if len(h.RefModL0) == 0 {
		return list, nil
	}
	bits, err := frameNumBits(d.sps)
	if err != nil {
		return nil, err
	}
	maxPicNum := int32(1) << uint(bits)
	pred := int32(h.FrameNum)
	for index, op := range h.RefModL0 {
		if index >= n {
			break
		}
		var pic *Picture
		switch op.IDC {
		case 0:
			diff := int32(op.Arg) + 1
			if diff > maxPicNum {
				return nil, fmt.Errorf("%w: abs_diff %d", ErrBadSliceHeader, op.Arg)
			}
			pred = (pred - diff) % maxPicNum
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
		case 1:
			return nil, fmt.Errorf("%w: field reference reorder", ErrStageScope)
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
