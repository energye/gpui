package h264

// Frame zig-zag scan for 4x4 blocks (Fig 8-14, frame mode).
var zigzag4x4 = [16]int{
	0, 1, 4, 8, 5, 2, 3, 6, 9, 12, 13, 10, 7, 11, 14, 15,
}

// dequantInit holds normAdjust row classes per qP%6 (Table 8-13 facts:
// class 0 at even-even, class 1 at mixed, class 2 at odd-odd positions).
var dequantInit = [6][3]int32{
	{10, 13, 16},
	{11, 14, 18},
	{13, 16, 20},
	{14, 18, 23},
	{16, 20, 25},
	{18, 23, 29},
}

// chromaQP maps QPY + offset (clipped 0..51) to QPC (Table 8-15, 8-bit).
var chromaQPTable = [52]int32{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17,
	18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	29, 30, 31, 32, 32, 33, 34, 34, 35, 35, 36, 36, 37, 37, 37,
	38, 38, 38, 39, 39, 39, 39,
}

// ChromaQP maps a luma QP plus index offset to the chroma QP.
func ChromaQP(qpY, offset int32) int32 {
	qpi := qpY + offset
	if qpi < 0 {
		qpi = 0
	}
	if qpi > 51 {
		qpi = 51
	}
	return chromaQPTable[qpi]
}

func levelScale(m, x, y int) int32 {
	switch {
	case x%2 == 0 && y%2 == 0:
		return dequantInit[m][0]
	case x%2 == 1 && y%2 == 1:
		return dequantInit[m][2]
	default:
		return dequantInit[m][1]
	}
}

func clipPixel(v int32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// itrans4x4Core is the exact integer inverse transform (8.5.12) on
// de-quantized coefficients in raster order, yielding residual samples.
func itrans4x4Core(c [16]int32) [16]int32 {
	var e, f [16]int32
	for i := 0; i < 4; i++ {
		s0 := c[i] + c[8+i]
		s1 := c[i] - c[8+i]
		s2 := (c[4+i] >> 1) - c[12+i]
		s3 := c[4+i] + (c[12+i] >> 1)
		e[i] = s0 + s3
		e[4+i] = s1 + s2
		e[8+i] = s1 - s2
		e[12+i] = s0 - s3
	}
	for i := 0; i < 4; i++ {
		s0 := e[4*i] + e[4*i+2]
		s1 := e[4*i] - e[4*i+2]
		s2 := (e[4*i+1] >> 1) - e[4*i+3]
		s3 := e[4*i+1] + (e[4*i+3] >> 1)
		f[4*i] = s0 + s3
		f[4*i+1] = s1 + s2
		f[4*i+2] = s1 - s2
		f[4*i+3] = s0 - s3
	}
	var out [16]int32
	for i := range out {
		out[i] = (f[i] + 32) >> 6
	}
	return out
}

// ITransform4x4 inverts one 4x4 block. coeff holds quantized levels in
// zig-zag scan order; qp is the block QP. Returns residual in raster order.
//
// De-quant scales by LevelScale(qP%6) times the flat default weight 16
// (no scaling lists in this stage) times 2^(qP/6); the product lands on a
// multiple of 64 so the 1/64 normalization is exact (matches the
// reference dequant-then->>6 pipeline with no rounding residue).
func ITransform4x4(coeff [16]int32, qp uint32) [16]int32 {
	m := int(qp % 6)
	shift := int(qp / 6)
	var c [16]int32
	for scan, v := range coeff {
		if v == 0 {
			continue
		}
		pos := zigzag4x4[scan]
		c[pos] = int32(int64(v) * int64(levelScale(m, pos%4, pos/4)) << uint(shift))
	}
	return itrans4x4Core(c)
}

// ITransform4x4WithDC inverts one 4x4 block whose DC is already
// de-quantized (luma Intra16x16 or chroma DC+AC path). ac holds raw AC
// levels in zig-zag scan order (entry 0 ignored); dc is the scaled DC
// value for raster position (0,0). Only the AC levels are scaled here.
func ITransform4x4WithDC(ac [16]int32, dc int32, qp uint32) [16]int32 {
	m := int(qp % 6)
	shift := int(qp / 6)
	var c [16]int32
	c[0] = dc
	for scan := 1; scan < 16; scan++ {
		v := ac[scan]
		if v == 0 {
			continue
		}
		pos := zigzag4x4[scan]
		c[pos] = int32(int64(v) * int64(levelScale(m, pos%4, pos/4)) << uint(shift))
	}
	return itrans4x4Core(c)
}

// ITransformLumaDC inverts the 4x4 luma DC Hadamard block (8.5.10).
// dc holds 16 quantized DC levels in zig-zag scan order; the output is
// 16 de-quantized DC values in 4x4 block-index order.
func ITransformLumaDC(dc [16]int32, qp uint32) [16]int32 {
	m := int(qp % 6)
	var c [16]int32
	for scan, v := range dc {
		c[zigzag4x4[scan]] = v
	}
	var e [16]int32
	for i := 0; i < 4; i++ {
		s0 := c[i] + c[8+i] + c[4+i] + c[12+i]
		s1 := c[i] + c[8+i] - c[4+i] - c[12+i]
		s2 := c[i] - c[8+i] - c[4+i] + c[12+i]
		s3 := c[i] - c[8+i] + c[4+i] - c[12+i]
		e[i], e[4+i], e[8+i], e[12+i] = s0, s1, s2, s3
	}
	var f [16]int32
	for i := 0; i < 4; i++ {
		s0 := e[4*i] + e[4*i+2] + e[4*i+1] + e[4*i+3]
		s1 := e[4*i] + e[4*i+2] - e[4*i+1] - e[4*i+3]
		s2 := e[4*i] - e[4*i+2] - e[4*i+1] + e[4*i+3]
		s3 := e[4*i] - e[4*i+2] + e[4*i+1] - e[4*i+3]
		f[4*i], f[4*i+1], f[4*i+2], f[4*i+3] = s0, s1, s2, s3
	}
	var out [16]int32
	ls := levelScale(m, 0, 0)
	if qp >= 12 {
		shift := int(qp/6) - 2
		for i := range out {
			out[i] = f[i] * ls << shift
		}
	} else {
		shift := 2 - int(qp/6)
		add := int32(1) << (shift - 1)
		for i := range out {
			out[i] = (f[i]*ls + add) >> shift
		}
	}
	return out
}

// ITransformChromaDC inverts one 2x2 chroma DC block (8.5.11).
func ITransformChromaDC(dc [4]int32, qp uint32) [4]int32 {
	a, b, c, d := dc[0], dc[1], dc[2], dc[3]
	e0 := a + b
	e1 := a - b
	e2 := c - d
	e3 := c + d
	f0 := e0 + e3
	f1 := e1 + e2
	f2 := e0 - e3
	f3 := e1 - e2
	m := int(qp % 6)
	ls := levelScale(m, 0, 0)
	var out [4]int32
	vals := [4]int32{f0, f1, f2, f3}
	if qp >= 6 {
		shift := int(qp/6) - 1
		for i, v := range vals {
			out[i] = v * ls << shift
		}
	} else {
		// Truncating shift matches the reference >>7 pipeline.
		for i, v := range vals {
			out[i] = (v * ls) >> 1
		}
	}
	return out
}
