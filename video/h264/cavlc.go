package h264

import "fmt"

// CAVLC residual decoding (the simpler entropy set, used by B/M档).
// Table facts mirror ITU-T Table 9-5/9-7/9-9/9-10; the reader builds
// prefix maps once and walks bits MSB-first.

var coeffTokenLen = [4][68]uint8{
	{
		1, 0, 0, 0,
		6, 2, 0, 0, 8, 6, 3, 0, 9, 8, 7, 5, 10, 9, 8, 6,
		11, 10, 9, 7, 13, 11, 10, 8, 13, 13, 11, 9, 13, 13, 13, 10,
		14, 14, 13, 11, 14, 14, 14, 13, 15, 15, 14, 14, 15, 15, 15, 14,
		16, 15, 15, 15, 16, 16, 16, 15, 16, 16, 16, 16, 16, 16, 16, 16,
	},
	{
		2, 0, 0, 0,
		6, 2, 0, 0, 6, 5, 3, 0, 7, 6, 6, 4, 8, 6, 6, 4,
		8, 7, 7, 5, 9, 8, 8, 6, 11, 9, 9, 6, 11, 11, 11, 7,
		12, 11, 11, 9, 12, 12, 12, 11, 12, 12, 12, 11, 13, 13, 13, 12,
		13, 13, 13, 13, 13, 14, 13, 13, 14, 14, 14, 13, 14, 14, 14, 14,
	},
	{
		4, 0, 0, 0,
		6, 4, 0, 0, 6, 5, 4, 0, 6, 5, 5, 4, 7, 5, 5, 4,
		7, 5, 5, 4, 7, 6, 6, 4, 7, 6, 6, 4, 8, 7, 7, 5,
		8, 8, 7, 6, 9, 8, 8, 7, 9, 9, 8, 8, 9, 9, 9, 8,
		10, 9, 9, 9, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10,
	},
	{
		6, 0, 0, 0,
		6, 6, 0, 0, 6, 6, 6, 0, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
	},
}

var coeffTokenBits = [4][68]uint8{
	{
		1, 0, 0, 0,
		5, 1, 0, 0, 7, 4, 1, 0, 7, 6, 5, 3, 7, 6, 5, 3,
		7, 6, 5, 4, 15, 6, 5, 4, 11, 14, 5, 4, 8, 10, 13, 4,
		15, 14, 9, 4, 11, 10, 13, 12, 15, 14, 9, 12, 11, 10, 13, 8,
		15, 1, 9, 12, 11, 14, 13, 8, 7, 10, 9, 12, 4, 6, 5, 8,
	},
	{
		3, 0, 0, 0,
		11, 2, 0, 0, 7, 7, 3, 0, 7, 10, 9, 5, 7, 6, 5, 4,
		4, 6, 5, 6, 7, 6, 5, 8, 15, 6, 5, 4, 11, 14, 13, 4,
		15, 10, 9, 4, 11, 14, 13, 12, 8, 10, 9, 8, 15, 14, 13, 12,
		11, 10, 9, 12, 7, 11, 6, 8, 9, 8, 10, 1, 7, 6, 5, 4,
	},
	{
		15, 0, 0, 0,
		15, 14, 0, 0, 11, 15, 13, 0, 8, 12, 14, 12, 15, 10, 11, 11,
		11, 8, 9, 10, 9, 14, 13, 9, 8, 10, 9, 8, 15, 14, 13, 13,
		11, 14, 10, 12, 15, 10, 13, 12, 11, 14, 9, 12, 8, 10, 13, 8,
		13, 7, 9, 12, 9, 12, 11, 10, 5, 8, 7, 6, 1, 4, 3, 2,
	},
	{
		3, 0, 0, 0,
		0, 1, 0, 0, 4, 5, 6, 0, 8, 9, 10, 11, 12, 13, 14, 15,
		16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
		32, 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47,
		48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63,
	},
}

var chromaDCTokenLen = [20]uint8{
	2, 0, 0, 0,
	6, 1, 0, 0,
	6, 6, 3, 0,
	6, 7, 7, 6,
	6, 8, 8, 7,
}

var chromaDCTokenBits = [20]uint8{
	1, 0, 0, 0,
	7, 1, 0, 0,
	4, 6, 1, 0,
	3, 3, 2, 5,
	2, 3, 2, 0,
}

var totalZerosLen = [16][16]uint8{
	{1, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 9},
	{3, 3, 3, 3, 3, 4, 4, 4, 4, 5, 5, 6, 6, 6, 6},
	{4, 3, 3, 3, 4, 4, 3, 3, 4, 5, 5, 6, 5, 6},
	{5, 3, 4, 4, 3, 3, 3, 4, 3, 4, 5, 5, 5},
	{4, 4, 4, 3, 3, 3, 3, 3, 4, 5, 4, 5},
	{6, 5, 3, 3, 3, 3, 3, 3, 4, 3, 6},
	{6, 5, 3, 3, 3, 2, 3, 4, 3, 6},
	{6, 4, 5, 3, 2, 2, 3, 3, 6},
	{6, 6, 4, 2, 2, 3, 2, 5},
	{5, 5, 3, 2, 2, 2, 4},
	{4, 4, 3, 3, 1, 3},
	{4, 4, 2, 1, 3},
	{3, 3, 1, 2},
	{2, 2, 1},
	{1, 1},
}

var totalZerosBits = [16][16]uint8{
	{1, 3, 2, 3, 2, 3, 2, 3, 2, 3, 2, 3, 2, 3, 2, 1},
	{7, 6, 5, 4, 3, 5, 4, 3, 2, 3, 2, 3, 2, 1, 0},
	{5, 7, 6, 5, 4, 3, 4, 3, 2, 3, 2, 1, 1, 0},
	{3, 7, 5, 4, 6, 5, 4, 3, 3, 2, 2, 1, 0},
	{5, 4, 3, 7, 6, 5, 4, 3, 2, 1, 1, 0},
	{1, 1, 7, 6, 5, 4, 3, 2, 1, 1, 0},
	{1, 1, 5, 4, 3, 3, 2, 1, 1, 0},
	{1, 1, 1, 3, 3, 2, 2, 1, 0},
	{1, 0, 1, 3, 2, 1, 1, 1},
	{1, 0, 1, 3, 2, 1, 1},
	{0, 1, 1, 2, 1, 3},
	{0, 1, 1, 1, 1},
	{0, 1, 1, 1},
	{0, 1, 1},
	{0, 1},
}

var chromaDCTotalZerosLen = [3][4]uint8{
	{1, 2, 3, 3},
	{1, 2, 2, 0},
	{1, 1, 0, 0},
}

var chromaDCTotalZerosBits = [3][4]uint8{
	{1, 1, 1, 0},
	{1, 1, 0, 0},
	{1, 0, 0, 0},
}

var runLen = [7][16]uint8{
	{1, 1},
	{1, 2, 2},
	{2, 2, 2, 2},
	{2, 2, 2, 3, 3},
	{2, 2, 3, 3, 3, 3},
	{2, 3, 3, 3, 3, 3, 3},
	{3, 3, 3, 3, 3, 3, 3, 4, 5, 6, 7, 8, 9, 10, 11},
}

var runBits = [7][16]uint8{
	{1, 0},
	{1, 1, 0},
	{3, 2, 1, 0},
	{3, 2, 1, 1, 0},
	{3, 2, 3, 2, 1, 0},
	{3, 0, 1, 3, 2, 5, 4},
	{7, 6, 5, 4, 3, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1},
}

type tokenEntry struct {
	tc  int
	t1  int
	val int
}

var coeffTokenMaps [4]map[int]tokenEntry
var chromaDCTokenMap map[int]tokenEntry
var totalZerosMaps [16]map[int]int
var chromaDCTotalZerosMaps [4]map[int]int
var runMaps [7]map[int]int

func init() {
	for t := 0; t < 4; t++ {
		m := map[int]tokenEntry{}
		for tc := 0; tc < 17; tc++ {
			for t1 := 0; t1 < 4; t1++ {
				l := coeffTokenLen[t][tc*4+t1]
				if l == 0 {
					continue
				}
				m[int(l)<<16|int(coeffTokenBits[t][tc*4+t1])] = tokenEntry{tc: tc, t1: t1}
			}
		}
		coeffTokenMaps[t] = m
	}
	chromaDCTokenMap = map[int]tokenEntry{}
	for tc := 0; tc < 5; tc++ {
		for t1 := 0; t1 < 4; t1++ {
			l := chromaDCTokenLen[tc*4+t1]
			if l == 0 {
				continue
			}
			chromaDCTokenMap[int(l)<<16|int(chromaDCTokenBits[tc*4+t1])] = tokenEntry{tc: tc, t1: t1}
		}
	}
	// TotalZeros rows run tc=1..15 with (17-tc) entries each: row i
	// holds tc=i+1 (zeros range 0..16-tc).
	for tc := 1; tc <= 15; tc++ {
		m := map[int]int{}
		for tz := 0; tz < 16; tz++ {
			if totalZerosLen[tc-1][tz] == 0 {
				break
			}
			m[int(totalZerosLen[tc-1][tz])<<16|int(totalZerosBits[tc-1][tz])] = tz
		}
		totalZerosMaps[tc] = m
	}
	// Chroma DC rows run tc=1..3 the same way (zeros range 0..4-tc).
	for tc := 1; tc <= 3; tc++ {
		m := map[int]int{}
		for tz := 0; tz < 4; tz++ {
			if chromaDCTotalZerosLen[tc-1][tz] == 0 {
				break
			}
			m[int(chromaDCTotalZerosLen[tc-1][tz])<<16|int(chromaDCTotalZerosBits[tc-1][tz])] = tz
		}
		chromaDCTotalZerosMaps[tc] = m
	}
	runCounts := [7]int{2, 3, 4, 5, 6, 7, 15}
	for row := 0; row < 7; row++ {
		m := map[int]int{}
		for run := 0; run < runCounts[row]; run++ {
			m[int(runLen[row][run])<<16|int(runBits[row][run])] = run
		}
		runMaps[row] = m
	}
}

// SelectTable maps neighbour count nC to a coeff_token table (Table 9-5).
func SelectTable(nC int) int {
	switch {
	case nC < 2:
		return 0
	case nC < 4:
		return 1
	case nC < 8:
		return 2
	default:
		return 3
	}
}

func readToken(r *Reader, m map[int]tokenEntry, what string) (tokenEntry, error) {
	code := 0
	for l := 1; l <= 16; l++ {
		b, err := r.ReadBit()
		if err != nil {
			return tokenEntry{}, fmt.Errorf("%w: %s: %v", ErrBadSliceHeader, what, err)
		}
		code = (code << 1) | int(b)
		if e, ok := m[l<<16|code]; ok {
			return e, nil
		}
	}
	return tokenEntry{}, fmt.Errorf("%w: %s nomatch", ErrBadSliceHeader, what)
}

func readValue(r *Reader, m map[int]int, maxLen int, what string) (int, error) {
	code := 0
	for l := 1; l <= maxLen; l++ {
		b, err := r.ReadBit()
		if err != nil {
			return 0, fmt.Errorf("%w: %s: %v", ErrBadSliceHeader, what, err)
		}
		code = (code << 1) | int(b)
		if v, ok := m[l<<16|code]; ok {
			return v, nil
		}
	}
	return 0, fmt.Errorf("%w: %s nomatch", ErrBadSliceHeader, what)
}

// DecodeCoeffToken reads TotalCoeff and TrailingOnes for 4x4 blocks.
func DecodeCoeffToken(r *Reader, table int) (tc, t1 int, err error) {
	e, err := readToken(r, coeffTokenMaps[table], "coeff token")
	if err != nil {
		return 0, 0, err
	}
	return e.tc, e.t1, nil
}

// DecodeChromaDCToken reads TotalCoeff and TrailingOnes for chroma DC.
func DecodeChromaDCToken(r *Reader) (tc, t1 int, err error) {
	e, err := readToken(r, chromaDCTokenMap, "chroma dc token")
	if err != nil {
		return 0, 0, err
	}
	return e.tc, e.t1, nil
}

// DecodeTotalZeros reads the zeros before the last coefficient.
func DecodeTotalZeros(r *Reader, tc int) (int, error) {
	if tc <= 0 || tc >= 16 {
		return 0, fmt.Errorf("%w: total zeros tc %d", ErrBadSliceHeader, tc)
	}
	return readValue(r, totalZerosMaps[tc], 9, "total zeros")
}

// DecodeChromaDCTotalZeros reads zeros for chroma DC blocks.
func DecodeChromaDCTotalZeros(r *Reader, tc int) (int, error) {
	if tc <= 0 || tc >= 4 {
		return 0, fmt.Errorf("%w: chroma total zeros tc %d", ErrBadSliceHeader, tc)
	}
	return readValue(r, chromaDCTotalZerosMaps[tc], 3, "chroma total zeros")
}

// DecodeRunBefore reads the zeros before coefficient i.
func DecodeRunBefore(r *Reader, zerosLeft int) (int, error) {
	row := zerosLeft - 1
	maxLen := 3
	if zerosLeft > 6 {
		row, maxLen = 6, 11
	}
	if row < 0 || row > 6 {
		return 0, fmt.Errorf("%w: run zeros %d", ErrBadSliceHeader, zerosLeft)
	}
	v, err := readValue(r, runMaps[row], maxLen, "run before")
	if err != nil {
		return 0, err
	}
	if v > zerosLeft {
		return 0, fmt.Errorf("%w: run %d > zeros %d", ErrBadSliceHeader, v, zerosLeft)
	}
	return v, nil
}

func countZeros(r *Reader) (int, error) {
	n := 0
	for {
		b, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		if b == 1 {
			return n, nil
		}
		n++
		if n > 32 {
			return 0, fmt.Errorf("%w: level prefix overflow", ErrBadSliceHeader)
		}
	}
}

// DecodeLevels reads the coefficient magnitudes in reverse scan order:
// TrailingOnes signs first, then remaining levels with suffix adaptation.
func DecodeLevels(r *Reader, tc, t1 int) ([]int32, error) {
	levels := make([]int32, 0, tc)
	for i := 0; i < t1; i++ {
		b, err := r.ReadBit()
		if err != nil {
			return nil, fmt.Errorf("%w: t1 sign: %v", ErrBadSliceHeader, err)
		}
		if b == 0 {
			levels = append(levels, 1)
		} else {
			levels = append(levels, -1)
		}
	}
	if t1 >= tc {
		return levels, nil
	}
	suffixLength := 0
	if tc > 10 && t1 < 3 {
		suffixLength = 1
	}
	first := true
	for len(levels) < tc {
		prefix, err := countZeros(r)
		if err != nil {
			return nil, err
		}
		var code int32
		if first {
			if prefix < 14 {
				if suffixLength != 0 {
					b, err := r.ReadBit()
					if err != nil {
						return nil, err
					}
					code = int32(prefix<<1) + int32(b)
				} else {
					code = int32(prefix)
				}
			} else if prefix == 14 {
				if suffixLength != 0 {
					b, err := r.ReadBit()
					if err != nil {
						return nil, err
					}
					code = int32(prefix<<1) + int32(b)
				} else {
					suf, err := r.ReadBits(4)
					if err != nil {
						return nil, err
					}
					code = int32(prefix) + int32(suf)
				}
			} else {
				code = 30
				if prefix >= 16 {
					if prefix > 28 {
						return nil, fmt.Errorf("%w: level prefix %d", ErrBadSliceHeader, prefix)
					}
					esc, err := r.ReadBits(prefix - 3)
					if err != nil {
						return nil, err
					}
					code += int32(1<<(prefix-3)) - 4096 + int32(esc)
				} else {
					suf, err := r.ReadBits(prefix - 3)
					if err != nil {
						return nil, err
					}
					code += int32(suf)
				}
			}
			if t1 < 3 {
				code += 2
			}
			mask := -(code & 1)
			v := ((2 + code) >> 1) ^ mask - mask
			levels = append(levels, v)
			suffixLength = 1
			if vabs(v) > 3 {
				suffixLength = 2
			}
			first = false
			continue
		}
		if prefix < 15 {
			suf, err := r.ReadBits(suffixLength)
			if err != nil {
				return nil, err
			}
			code = int32(prefix<<suffixLength) + int32(suf)
		} else {
			code = int32(15 << suffixLength)
			if prefix >= 16 {
				if prefix > 28 {
					return nil, fmt.Errorf("%w: level prefix %d", ErrBadSliceHeader, prefix)
				}
				code += int32(1<<(prefix-3)) - 4096
			}
			suf, err := r.ReadBits(prefix - 3)
			if err != nil {
				return nil, err
			}
			code += int32(suf)
		}
		mask := -(code & 1)
		v := ((2 + code) >> 1) ^ mask - mask
		levels = append(levels, v)
		if suffixLimit(suffixLength)+vabs(v) > 2*suffixLimit(suffixLength) {
			suffixLength++
		}
	}
	return levels, nil
}

func vabs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

func suffixLimit(s int) int32 {
	switch s {
	case 0:
		return 0
	case 1:
		return 3
	case 2:
		return 6
	case 3:
		return 12
	case 4:
		return 24
	case 5:
		return 48
	default:
		return 1 << 30
	}
}

// DecodeResidualBlock reads one 4x4 CAVLC block into zig-zag scan order.
// table selects the coeff_token table (use SelectTable for luma, the
// chroma variant for chroma DC), maxCoeff caps TotalCoeff (16, 15 for AC
// blocks whose DC lives elsewhere, or 4 for chroma DC), scanBase shifts
// placement (1 for AC-only blocks), zerosTable picks the total_zeros
// family.
func DecodeResidualBlock(r *Reader, table, maxCoeff, scanBase int, chromaDC bool) ([16]int32, error) {
	var out [16]int32
	var tc, t1 int
	var err error
	if chromaDC {
		tc, t1, err = DecodeChromaDCToken(r)
	} else {
		tc, t1, err = DecodeCoeffToken(r, table)
	}
	if err != nil {
		return out, err
	}
	if tc == 0 {
		return out, nil
	}
	if tc > maxCoeff {
		return out, fmt.Errorf("%w: total %d > max %d", ErrBadSliceHeader, tc, maxCoeff)
	}
	levels, err := DecodeLevels(r, tc, t1)
	if err != nil {
		return out, err
	}
	var zerosLeft int
	if tc == maxCoeff {
		zerosLeft = 0
	} else if chromaDC {
		zerosLeft, err = DecodeChromaDCTotalZeros(r, tc)
	} else {
		zerosLeft, err = DecodeTotalZeros(r, tc)
	}
	if err != nil {
		return out, err
	}
	pos := zerosLeft + tc - 1
	if pos > maxCoeff-1 || scanBase+pos > 15 {
		return out, fmt.Errorf("%w: scan %d", ErrBadSliceHeader, pos)
	}
	place := func(p int, v int32) error {
		if p < 0 || scanBase+p > 15 {
			return fmt.Errorf("%w: scan %d", ErrBadSliceHeader, p)
		}
		out[scanBase+p] = v
		return nil
	}
	if err := place(pos, levels[0]); err != nil {
		return out, err
	}
	i := 1
	for ; i < tc && zerosLeft > 0; i++ {
		run, err := DecodeRunBefore(r, zerosLeft)
		if err != nil {
			return out, err
		}
		zerosLeft -= run
		pos -= 1 + run
		if err := place(pos, levels[i]); err != nil {
			return out, err
		}
	}
	for ; i < tc; i++ {
		pos--
		if err := place(pos, levels[i]); err != nil {
			return out, err
		}
	}
	if zerosLeft < 0 {
		return out, fmt.Errorf("%w: negative zeros", ErrBadSliceHeader)
	}
	return out, nil
}
