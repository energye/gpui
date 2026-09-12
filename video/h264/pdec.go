package h264

import "fmt"

// P-slice macroblock decoding: skip, inter partitions, intra fallback.

func (d *Decoder) decodeSkip(h *SliceHeader, addr int) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	x0, y0 := mbx*4, mby*4
	mx, my := d.predMotion(x0, y0, 4, 0)
	if d.refPic == nil {
		return fmt.Errorf("%w: skip without reference", ErrBadSliceHeader)
	}
	d.storeMV(mbx*16, mby*16, 16, 16, mx, my, 0)
	predY := predictLumaBlock(d.refPic, mbx*16, mby*16, 16, 16, mx, my)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			d.pic.SetY(uint32(mbx*16+x), uint32(mby*16+y), predY[y*16+x])
		}
	}
	cw, ch := int(d.pic.Width/2), int(d.pic.Height/2)
	for comp := 0; comp < 2; comp++ {
		plane := d.pic.Cb
		refPlane := d.refPic.Cb
		if comp == 1 {
			plane = d.pic.Cr
			refPlane = d.refPic.Cr
		}
		pred := predictChromaBlock(refPlane, cw, ch, mbx*8, mby*8, 8, 8, mx, my)
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				plane[(mby*8+y)*cw+mbx*8+x] = pred[y*8+x]
			}
		}
	}
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			bx, by := mbx*4+x, mby*4+y
			d.modes[by*stride+bx] = -1
			d.setNnz(d.nnzY, stride, bx, by, 0)
		}
	}
	cstride := d.mbW * 2
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			d.setNnz(d.nnzCb, cstride, mbx*2+x, mby*2+y, 0)
			d.setNnz(d.nnzCr, cstride, mbx*2+x, mby*2+y, 0)
		}
	}
	d.qps[addr] = d.qpY
	d.fIDC[addr] = h.DisableFilter
	d.fA[addr] = h.FilterAlpha
	d.fB[addr] = h.FilterBeta
	d.mbIntra[addr] = false
	d.skipCnt++
	return nil
}

func (d *Decoder) decodeMBP(r *Reader, pps *PPS, h *SliceHeader, addr int) error {
	mbx, mby := addr%d.mbW, addr/d.mbW
	mbType, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("mb type: %w", err)
	}
	if mbType > 30 {
		return fmt.Errorf("%w: mb type %d in P slice", ErrBadSliceHeader, mbType)
	}
	if mbType >= 5 {
		intraType := mbType - 5
		if intraType == 25 {
			return d.decodePCM(r, mbx, mby)
		}
		return d.decodeIntraMB(r, pps, h, addr, mbx, mby, intraType)
	}
	// Inter 0..4.
	if pps.Transform8x8 {
		// 8x8 transform flag would follow CBP; Baseline never sets it.
		// Guard so a future High stream fails readable instead of skewing.
		_ = pps
	}
	type part struct {
		px, py, w, h int
		mx, my       int16
		ref          int8
	}
	refFor := func(idx int8) (*Picture, error) {
		if len(d.refList) == 0 {
			if d.refPic != nil && idx == 0 {
				return d.refPic, nil
			}
			return nil, fmt.Errorf("%w: ref %d without list", ErrBadSliceHeader, idx)
		}
		if int(idx) < 0 || int(idx) >= len(d.refList) || d.refList[idx] == nil {
			return nil, fmt.Errorf("%w: ref idx %d", ErrBadSliceHeader, idx)
		}
		return d.refList[idx], nil
	}
	var parts []part
	x0, y0 := mbx*4, mby*4
	px0, py0 := mbx*16, mby*16
	readRef := func() (int8, error) {
		if h.RefL0Count <= 1 {
			return 0, nil
		}
		if h.RefL0Count == 2 {
			b, err := r.ReadBits(1)
			if err != nil {
				return 0, err
			}
			return int8(b ^ 1), nil
		}
		v, err := r.ReadUE()
		if err != nil {
			return 0, err
		}
		if v >= h.RefL0Count {
			return 0, fmt.Errorf("%w: ref idx %d", ErrBadSliceHeader, v)
		}
		return int8(v), nil
	}
	readMVD := func(predX, predY int16) (int16, int16, error) {
		dx, err := r.ReadSE()
		if err != nil {
			return 0, 0, err
		}
		dy, err := r.ReadSE()
		if err != nil {
			return 0, 0, err
		}
		return predX + int16(dx), predY + int16(dy), nil
	}
	switch mbType {
	case 0: // P_16x16
		ref, err := readRef()
		if err != nil {
			return err
		}
		px, py := d.predMotion(x0, y0, 4, ref)
		mx, my, err := readMVD(px, py)
		if err != nil {
			return err
		}
		d.storeMV(px0, py0, 16, 16, mx, my, ref)
		parts = append(parts, part{px0, py0, 16, 16, mx, my, ref})
	case 1: // P_16x8: refs first, then MVDs in order.
		var refs [2]int8
		for i := 0; i < 2; i++ {
			ref, err := readRef()
			if err != nil {
				return err
			}
			refs[i] = ref
		}
		for i := 0; i < 2; i++ {
			ref := refs[i]
			py := py0 + i*8
			var px, pyv int16
			if i == 0 {
				px, pyv = d.pred16x8Top(x0, y0, ref)
			} else {
				px, pyv = d.pred16x8Bottom(x0, y0+2, ref)
			}
			mx, my, err := readMVD(px, pyv)
			if err != nil {
				return err
			}
			d.storeMV(px0, py, 16, 8, mx, my, ref)
			parts = append(parts, part{px0, py, 16, 8, mx, my, ref})
		}
	case 2: // P_8x16: refs first, then MVDs.
		var refs [2]int8
		for i := 0; i < 2; i++ {
			ref, err := readRef()
			if err != nil {
				return err
			}
			refs[i] = ref
		}
		for i := 0; i < 2; i++ {
			ref := refs[i]
			px := px0 + i*8
			var pxx, pyy int16
			if i == 0 {
				pxx, pyy = d.pred8x16Left(x0, y0, ref)
			} else {
				pxx, pyy = d.pred8x16Right(x0+2, y0, ref)
			}
			mx, my, err := readMVD(pxx, pyy)
			if err != nil {
				return err
			}
			d.storeMV(px, py0, 8, 16, mx, my, ref)
			parts = append(parts, part{px, py0, 8, 16, mx, my, ref})
		}
	case 3, 4: // P_8x8 / P_8x8ref0
		ref0Only := mbType == 4
		var subTypes [4]uint32
		for i := 0; i < 4; i++ {
			v, err := r.ReadUE()
			if err != nil {
				return fmt.Errorf("sub type: %w", err)
			}
			if v > 3 {
				return fmt.Errorf("%w: sub type %d", ErrBadSliceHeader, v)
			}
			subTypes[i] = v
		}
		var refs [4]int8
		for i := 0; i < 4; i++ {
			if ref0Only {
				refs[i] = 0
				continue
			}
			ref, err := readRef()
			if err != nil {
				return err
			}
			refs[i] = ref
		}
		// 8x8 order: 0 TL, 1 TR, 2 BL, 3 BR. MVDs follow refs.
		for i := 0; i < 4; i++ {
			bx8, by8 := i%2, i/2
			ox, oy := px0+bx8*8, py0+by8*8
			qx, qy := x0+bx8*2, y0+by8*2
			ref := refs[i]
			switch subTypes[i] {
			case 0: // 8x8
				px, py := d.predMotion(qx, qy, 2, ref)
				mx, my, err := readMVD(px, py)
				if err != nil {
					return err
				}
				d.storeMV(ox, oy, 8, 8, mx, my, ref)
				parts = append(parts, part{ox, oy, 8, 8, mx, my, ref})
			case 1: // 8x4 top,bottom
				for k := 0; k < 2; k++ {
					py := oy + k*4
					sqy := qy + k
					px, pyv := d.predMotion(qx, sqy, 2, ref)
					mx, my, err := readMVD(px, pyv)
					if err != nil {
						return err
					}
					d.storeMV(ox, py, 8, 4, mx, my, ref)
					parts = append(parts, part{ox, py, 8, 4, mx, my, ref})
				}
			case 2: // 4x8 left,right
				for k := 0; k < 2; k++ {
					px := ox + k*4
					sqx := qx + k
					pvx, pvy := d.predMotion(sqx, qy, 1, ref)
					mx, my, err := readMVD(pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, oy, 4, 8, mx, my, ref)
					parts = append(parts, part{px, oy, 4, 8, mx, my, ref})
				}
			default: // 4x4 raster
				for k := 0; k < 4; k++ {
					px := ox + (k%2)*4
					py := oy + (k/2)*4
					sqx, sqy := qx+(k%2), qy+(k/2)
					pvx, pvy := d.predMotion(sqx, sqy, 1, ref)
					mx, my, err := readMVD(pvx, pvy)
					if err != nil {
						return err
					}
					d.storeMV(px, py, 4, 4, mx, my, ref)
					parts = append(parts, part{px, py, 4, 4, mx, my, ref})
				}
			}
		}
	}
	// Motion compensation into full-MB prediction buffers.
	predY := make([]uint8, 256)
	for y := range predY {
		predY[y] = 128
	}
	predCb := make([]uint8, 64)
	predCr := make([]uint8, 64)
	for i := range predCb {
		predCb[i], predCr[i] = 128, 128
	}
	if len(parts) == 0 {
		return fmt.Errorf("%w: no partitions", ErrBadSliceHeader)
	}
	for _, pt := range parts {
		rp, err := refFor(pt.ref)
		if err != nil {
			return err
		}
		blk := predictLumaBlock(rp, pt.px, pt.py, pt.w, pt.h, pt.mx, pt.my)
		for y := 0; y < pt.h; y++ {
			for x := 0; x < pt.w; x++ {
				ox, oy := pt.px-mbx*16+x, pt.py-mby*16+y
				predY[oy*16+ox] = blk[y*pt.w+x]
			}
		}
		cw := pt.w / 2
		ch := pt.h / 2
		cx, cy := (pt.px-mbx*16)/2, (pt.py-mby*16)/2
		// Chroma origin in the subsampled plane.
		ccx, ccy := mbx*8+cx, mby*8+cy
		rw, rh := int(d.pic.Width/2), int(d.pic.Height/2)
		rp2, err := refFor(pt.ref)
		if err != nil {
			return err
		}
		cb := predictChromaBlock(rp2.Cb, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my)
		cr := predictChromaBlock(rp2.Cr, rw, rh, ccx, ccy, cw, ch, pt.mx, pt.my)
		for y := 0; y < ch; y++ {
			for x := 0; x < cw; x++ {
				predCb[(cy+y)*8+cx+x] = cb[y*cw+x]
				predCr[(cy+y)*8+cx+x] = cr[y*cw+x]
			}
		}
	}
	// CBP + QP delta + residual (inter table).
	cbpUE, err := r.ReadUE()
	if err != nil {
		return fmt.Errorf("cbp: %w", err)
	}
	if cbpUE > 47 {
		return fmt.Errorf("%w: cbp %d", ErrBadSliceHeader, cbpUE)
	}
	cbp := uint32(golombToInterCBP[cbpUE])
	if cbp != 0 {
		delta, err := r.ReadSE()
		if err != nil {
			return fmt.Errorf("qp delta: %w", err)
		}
		d.qpY += delta
		for d.qpY < 0 {
			d.qpY += 52
		}
		for d.qpY > 51 {
			d.qpY -= 52
		}
	}
	d.qps[addr] = d.qpY
	d.fIDC[addr] = h.DisableFilter
	d.fA[addr] = h.FilterAlpha
	d.fB[addr] = h.FilterBeta
	d.mbIntra[addr] = false
	stride := d.mbW * 4
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			d.modes[(mby*4+y)*stride+mbx*4+x] = -1
		}
	}
	if err := d.reconstructInter(r, mbx, mby, cbp, predY, predCb, predCr); err != nil {
		return err
	}
	return nil
}

func (d *Decoder) reconstructInter(r *Reader, mbx, mby int, cbp uint32, predY, predCb, predCr []uint8) error {
	stride := d.mbW * 4
	for _, b := range [16]int{0, 1, 4, 5, 2, 3, 6, 7, 8, 9, 12, 13, 10, 11, 14, 15} {
		bx, by := mbx*4+b%4, mby*4+b/4
		var coeff [16]int32
		tc := 0
		i8 := (b/8)*2 + (b%4)/2
		if cbp&(1<<uint(i8)) != 0 {
			nC := d.blockNC(d.nnzY, stride, bx, by)
			c, total, err := d.readLumaBlock(r, nC)
			if err != nil {
				return err
			}
			coeff, tc = c, total
		}
		res := ITransform4x4(coeff, uint32(d.qpY))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				lx, ly := (bx-mbx*4)*4+x, (by-mby*4)*4+y
				v := int32(predY[ly*16+lx]) + res[y*4+x]
				d.pic.SetY(uint32(bx*4+x), uint32(by*4+y), clipPixel(v))
			}
		}
		d.setNnz(d.nnzY, stride, bx, by, tc)
	}
	return d.reconstructInterChroma(r, mbx, mby, (cbp>>4)&3, predCb, predCr)
}

func (d *Decoder) reconstructInterChroma(r *Reader, mbx, mby int, cbpC uint32, predCb, predCr []uint8) error {
	planes := [][]uint8{d.pic.Cb, d.pic.Cr}
	preds := [][]uint8{predCb, predCr}
	grids := [][]int8{d.nnzCb, d.nnzCr}
	cstride := d.mbW * 2
	qps := [2]int32{
		ChromaQP(d.qpY, d.cOff0),
		ChromaQP(d.qpY, d.cOff1),
	}
	var dcRs [2][4]int32
	if cbpC > 0 {
		for comp := 0; comp < 2; comp++ {
			dcRaw, err := DecodeResidualBlock(r, 0, 4, 0, true)
			if err != nil {
				return fmt.Errorf("chroma dc: %w", err)
			}
			var dcArr [4]int32
			copy(dcArr[:], dcRaw[:4])
			dcRs[comp] = ITransformChromaDC(dcArr, uint32(qps[comp]))
		}
	}
	for comp := 0; comp < 2; comp++ {
		pred := preds[comp]
		dcR := dcRs[comp]
		for b := 0; b < 4; b++ {
			var coeff [16]int32
			acNZ := 0
			if cbpC == 2 {
				bx, by := mbx*2+b%2, mby*2+b/2
				nC := d.blockNC(grids[comp], cstride, bx, by)
				ac, err := DecodeResidualBlock(r, SelectTable(nC), 15, 1, false)
				if err != nil {
					return fmt.Errorf("chroma ac c%d b%d nC=%d: %w", comp, b, nC, err)
				}
				coeff = ac
				for _, v := range ac {
					if v != 0 {
						acNZ++
					}
				}
			}
			res := ITransform4x4WithDC(coeff, dcR[b], uint32(qps[comp]))
			bx, by := mbx*2+b%2, mby*2+b/2
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					v := int32(pred[((by-mby*2)*4+y)*8+(bx-mbx*2)*4+x]) + res[y*4+x]
					px, py := uint32(bx*4+x), uint32(by*4+y)
					planes[comp][py*d.pic.Width/2+px] = clipPixel(v)
				}
			}
			d.setNnz(grids[comp], cstride, bx, by, acNZ)
		}
	}
	return nil
}
