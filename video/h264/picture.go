package h264

import "fmt"

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
func (d *DPB) Store(p *Picture) {
	if d == nil || p == nil {
		return
	}
	if p.IsIDR {
		d.pics = d.pics[:0]
	}
	d.pics = append(d.pics, p)
	for len(d.pics) > d.max {
		d.pics = d.pics[1:]
	}
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
