package aac

import (
	"fmt"
	"math"
)

// pow43 returns i^(4/3), the dequantized magnitude peers
// codebook_vector vals (precomputed pow43 in ffmpeg).
func pow43(i int) float64 {
	if i == 0 {
		return 0
	}
	return math.Pow(float64(i), 4.0/3.0)
}

// vecVals returns the magnitude table for a book (1..11).
func vecVals(book int) []float64 {
	switch book {
	case 1, 2:
		return []float64{-1, 0, 1}
	case 3, 4:
		v := make([]float64, 16)
		for i := range v {
			v[i] = pow43(i)
		}
		return v
	case 5, 6:
		// 9 entries peers codebook_vector4_vals.
		return []float64{-pow43(4), -pow43(3), -pow43(2), -1, 0, 1, pow43(2), pow43(3), pow43(4)}
	case 7, 8, 9, 10, 11:
		v := make([]float64, 16)
		for i := range v {
			v[i] = pow43(i)
		}
		return v
	}
	return nil
}

// vecIdx returns the index table for a book.
func vecIdx(book int) []uint16 {
	switch book {
	case 1, 2, 3, 4:
		return vecIdx02[:]
	case 5, 6:
		return vecIdx4[:]
	case 7, 8:
		return vecIdx6[:]
	case 9, 10:
		return vecIdx8[:]
	case 11:
		return vecIdx10[:]
	}
	return nil
}

// readSigns reads n sign bits MSB-first as an integer.
func readSigns(r *bitReaderMSB, n int) (int, error) {
	if n == 0 {
		return 0, nil
	}
	return r.read(n)
}

// applySigns applies n sign bits (MSB-first) to vals in order, skipping
// zeros. It returns signed values. This peers VMUL sign ordering:
// signs are consumed for nonzero values in index order.
func applySigns(vals []float64, signs, n int) []float64 {
	out := make([]float64, len(vals))
	si := 0
	for i, v := range vals {
		if v != 0 {
			if si < n {
				if (signs>>uint(n-1-si))&1 == 1 {
					v = -v
				}
				si++
			}
		}
		out[i] = v
	}
	return out
}

// decodeSpectrumBlock decodes one scalefactor band into coef.
// book is 1..11; off/length are in lines; scale is the dequant scale.
// random is the noise RNG state for book 13 (nil otherwise).
func decodeSpectrumBlock(r *bitReaderMSB, book int, coef []float64, off, length int, scale float64, random *uint32) error {
	if book < 1 || book > 11 {
		return fmt.Errorf("%w: book %d", ErrBadADTS, book)
	}
	if err := ensureHuff(); err != nil {
		return err
	}
	tree := bookTree[book-1]
	idxTab := vecIdx(book)
	vals := vecVals(book)
	switch book {
	case 1, 2:
		// Unsigned 4-tuples (VMUL4 path, case 0): signs live in the
		// 3-entry table itself, no sign bits in the stream.
		pos := off
		end := off + length
		for pos < end {
			sym, err := tree.decode(r)
			if err != nil {
				return err
			}
			v := idxTab[sym]
			for k := 0; k < 4 && pos < end; k++ {
				vi := (v >> uint(k*2)) & 3
				if int(vi) >= len(vals) {
					return fmt.Errorf("%w: book%d val %d", ErrBadADTS, book, vi)
				}
				coef[pos] = vals[vi] * scale
				pos++
			}
		}
	case 3, 4:
		// Signed 4-tuples (VMUL4S path, case 1): nnz sign bits,
		// consumed MSB-first, skipping zero positions per the
		// vecIdx mask (bits 12:15). Replicates the shift register
		// exactly: value k takes the current MSB, then shifts iff
		// its mask bit is set.
		pos := off
		end := off + length
		for pos < end {
			sym, err := tree.decode(r)
			if err != nil {
				return err
			}
			v := idxTab[sym]
			nnz := int((v >> 8) & 15)
			mask := int((v >> 12) & 15)
			signs := 0
			if nnz > 0 {
				s, err := readSigns(r, nnz)
				if err != nil {
					return err
				}
				signs = s
			}
			reg := uint32(signs) << uint(32-nnz)
			for k := 0; k < 4 && pos < end; k++ {
				vi := (v >> uint(k*2)) & 3
				val := vals[vi] * scale
				if reg>>31 == 1 {
					val = -val
				}
				if (mask>>uint(k))&1 == 1 {
					reg <<= 1
				}
				coef[pos] = val
				pos++
			}
		}
	case 5, 6:
		// Unsigned pairs (VMUL2 path, case 2): 9-entry signed table,
		// no sign bits in the stream.
		pos := off
		end := off + length
		for pos < end {
			sym, err := tree.decode(r)
			if err != nil {
				return err
			}
			v := idxTab[sym]
			v0 := v & 15
			v1 := (v >> 4) & 15
			if int(v0) >= len(vals) || int(v1) >= len(vals) {
				return fmt.Errorf("%w: book%d val", ErrBadADTS, book)
			}
			coef[pos] = vals[v0] * scale
			if pos+1 < end {
				coef[pos+1] = vals[v1] * scale
			}
			pos += 2
		}
	case 7, 8, 9, 10:
		// Signed pairs (VMUL2S path, cases 3/4): nnz sign bits,
		// routed exactly like ffmpeg: v1 takes bit0 of the shifted
		// sign word, v0 takes bit1.
		pos := off
		end := off + length
		for pos < end {
			sym, err := tree.decode(r)
			if err != nil {
				return err
			}
			v := idxTab[sym]
			v0 := v & 15
			v1 := (v >> 4) & 15
			if int(v0) >= len(vals) || int(v1) >= len(vals) {
				return fmt.Errorf("%w: book%d val", ErrBadADTS, book)
			}
			nnz := int((v >> 8) & 15)
			shift := int((v >> 12) & 15)
			sign := 0
			if nnz > 0 {
				s, err := readSigns(r, nnz)
				if err != nil {
					return err
				}
				sign = s << uint(shift)
			}
			val0 := vals[v0] * scale
			val1 := vals[v1] * scale
			if (sign>>1)&1 == 1 {
				val0 = -val0
			}
			if sign&1 == 1 {
				val1 = -val1
			}
			coef[pos] = val0
			if pos+1 < end {
				coef[pos+1] = val1
			}
			pos += 2
		}
	case 11:
		pos := off
		end := off + length
		for pos < end {
			sym, err := tree.decode(r)
			if err != nil {
				return err
			}
			v := idxTab[sym]
			if v == 0 {
				coef[pos] = 0
				if pos+1 < end {
					coef[pos+1] = 0
				}
				pos += 2
				continue
			}
			nnz := int((v >> 12) & 15)
			nzt := int((v >> 8) & 15)
			bits := 0
			if nnz > 0 {
				b, err := r.read(nnz)
				if err != nil {
					return err
				}
				// Align to MSB for VMUL-style consumption.
				bits = b << uint(32-nnz)
			}
			for j := 0; j < 2; j++ {
				esc := (nzt>>uint(j))&1 == 1
				var val float64
				if esc {
					// Escape: count leading ones (max 8 per spec).
					ones := 0
					for {
						b, err := r.read1()
						if err != nil {
							return err
						}
						if b == 0 {
							break
						}
						ones++
						if ones > 8 {
							return fmt.Errorf("%w: ESC overflow", ErrBadADTS)
						}
					}
					b := ones + 4
					n, err := r.read(b)
					if err != nil {
						return err
					}
					n += 1 << uint(b)
					val = pow43(n)
					if bits&0x80000000 != 0 {
						val = -val
					}
					bits <<= 1
				} else {
					var base int
					if j == 0 {
						base = int(v & 15)
					} else {
						base = int((v >> 4) & 15)
					}
					val = vals[base]
					if val != 0 {
						if bits&0x80000000 != 0 {
							val = -val
						}
						bits <<= 1
					} else {
						// Zero consumes no sign bit in ffmpeg's
						// bits<<=!!v logic; our bits already shifted
						// only for nonzeros above, so do nothing.
					}
					// ffmpeg does bits<<=!!v; replicate: shift only
					// when val (pre-sign) nonzero. Above we shifted
					// for nonzero; for zero skip. But our escape
					// branch always shifts; non-escape shifts only
					// for nonzero. Fix: undo shift for zero.
					if vals[base] == 0 {
						// No shift happened above? We shifted only
						// inside nonzero. Good, nothing to undo.
					}
				}
				// Defer scale multiply to band end (ffmpeg does
				// vector_fmul_scalar after the band). Apply here.
				if pos+j < end {
					coef[pos+j] = val * scale
				}
			}
			pos += 2
		}
	}
	return nil
}
