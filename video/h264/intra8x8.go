package h264

import "fmt"

// Intra 8x8 prediction (F6): the nine modes share the 4x4 numbering and
// direction shapes, run on 8x8 blocks with low-passed references.
// Reference taps mirror the spec filter (top/left [1 2 1]/4 runs with
// edge replication, corner blend, raw replication past missing
// top-right); the angular equations are the same diagonal groupings as
// the 4x4 modes widened to eight.

// PredIntra8x8 predicts one 8x8 luma block whose top-left 4x4 sits at
// 4x4 coords (bx, by). Returns 64 samples in raster order.
//
// No heap per call: reference/low-pass buffers are stack arrays, not
// make() slices (this runs once per 8x8 intra block — ~24k calls on an
// I-heavy 1080p frame; make() x4 per call showed as GC pressure).
func PredIntra8x8(get sampler, bx, by int32, mode int) ([64]uint8, error) {
	var out [64]uint8
	// S1b-IC direct stores (bit-identical): the old set() closure ran
	// once per pixel (64 calls per block); every mode below writes
	// out[y*8+x] directly with the same value.
	var rawT [16]uint8
	var rawL [8]uint8
	hasTop, hasTR, hasLeft := true, true, true
	for i := 0; i < 8; i++ {
		v, ok := get(bx+int32(i), by-1)
		if !ok {
			hasTop = false
		} else {
			rawT[i] = v
		}
		v, ok = get(bx-1, by+int32(i))
		if !ok {
			hasLeft = false
		} else {
			rawL[i] = v
		}
	}
	for i := 8; i < 16; i++ {
		v, ok := get(bx+int32(i), by-1)
		if !ok {
			hasTR = false
			break
		}
		rawT[i] = v
	}
	rawC, hasCorner := get(bx-1, by-1)

	// Low-passed references; missing top-right replicates raw p7.
	// S1b-IB extended tap arrays (bit-identical pixels): the old
	// at/al closures re-checked hasCorner/hasTR per tap (~70 branchy
	// calls per block across the low-pass loops + corner blend, ~60ms
	// flat per 90f profile). extT/extL bake the replication once:
	// extT[i+1] == old at(i) for i in -1..16, extL[i+1] == old al(i)
	// for i in -1..8 (peer ffmpeg h264pred row preload, same idea:
	// references contiguous first, filter branchless after). The nine
	// mode bodies below are untouched this round (S1b-IA proved their
	// closure inline neutral; T/L/set stay as the shared shape).
	var t [16]int
	var l [8]int
	var extT [18]int
	var extL [10]int
	if hasCorner {
		extT[0], extL[0] = int(rawC), int(rawC)
	} else {
		extT[0], extL[0] = int(rawT[0]), int(rawL[0])
	}
	for i := 0; i < 8; i++ {
		extT[i+1], extL[i+1] = int(rawT[i]), int(rawL[i])
	}
	if hasTR {
		for i := 8; i < 16; i++ {
			extT[i+1] = int(rawT[i])
		}
	} else {
		for i := 8; i < 16; i++ {
			extT[i+1] = int(rawT[7])
		}
	}
	for i := 0; i < 7; i++ {
		t[i] = (extT[i] + 2*extT[i+1] + extT[i+2] + 2) >> 2
		l[i] = (extL[i] + 2*extL[i+1] + extL[i+2] + 2) >> 2
	}
	t[7] = (extT[7] + 2*extT[8] + extT[9] + 2) >> 2
	l[7] = (extL[7] + 3*extL[8] + 2) >> 2
	if hasTR {
		for i := 8; i < 15; i++ {
			t[i] = (extT[i] + 2*extT[i+1] + extT[i+2] + 2) >> 2
		}
		t[15] = (extT[15] + 3*extT[16] + 2) >> 2
	} else {
		for i := 8; i < 16; i++ {
			t[i] = int(rawT[7])
		}
	}
	lt := 128
	if hasCorner {
		lt = (extL[1] + 2*int(rawC) + extT[1] + 2) >> 2
	}
	// Extended taps: T[-1] and L[-1] read the corner blend.
	T := func(i int) int {
		if i < 0 {
			return lt
		}
		return t[i]
	}
	L := func(i int) int {
		if i < 0 {
			return lt
		}
		return l[i]
	}

	needTop := func() error {
		if !hasTop {
			return fmt.Errorf("%w: intra8x8 top missing", ErrBadSliceHeader)
		}
		return nil
	}
	needLeft := func() error {
		if !hasLeft {
			return fmt.Errorf("%w: intra8x8 left missing", ErrBadSliceHeader)
		}
		return nil
	}
	needCorner := func() error {
		if !hasCorner {
			return fmt.Errorf("%w: intra8x8 corner missing", ErrBadSliceHeader)
		}
		return nil
	}

	switch mode {
	case Intra4x4Vertical:
		if err := needTop(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				out[y*8+x] = uint8(t[x])
			}
		}
	case Intra4x4Horizontal:
		if err := needLeft(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				out[y*8+x] = uint8(l[y])
			}
		}
	case Intra4x4DC:
		sum, n := 0, 0
		if hasTop {
			for i := 0; i < 8; i++ {
				sum += t[i]
			}
			n += 8
		}
		if hasLeft {
			for i := 0; i < 8; i++ {
				sum += l[i]
			}
			n += 8
		}
		mean := 128
		if n == 16 {
			mean = (sum + 8) >> 4
		} else if n == 8 {
			mean = (sum + 4) >> 3
		}
		for i := range out {
			out[i] = uint8(mean)
		}
	case Intra4x4DiagDownLeft:
		if err := needTop(); err != nil {
			return out, err
		}
		for s := 0; s < 14; s++ {
			v := (t[s] + 2*t[s+1] + t[s+2] + 2) >> 2
			for x := 0; x < 8; x++ {
				if y := s - x; y >= 0 && y < 8 {
					out[y*8+x] = uint8(v)
				}
			}
		}
		v := (t[14] + 3*t[15] + 2) >> 2
		out[7*8+7] = uint8(v)
	case Intra4x4DiagDownRight:
		if err := needTop(); err != nil {
			return out, err
		}
		if err := needLeft(); err != nil {
			return out, err
		}
		if err := needCorner(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				d := x - y
				var v int
				switch {
				case d < 0:
					v = (L(-d) + 2*L(-d-1) + L(-d-2) + 2) >> 2
				case d == 0:
					v = (l[0] + 2*lt + t[0] + 2) >> 2
				default:
					v = (T(d-2) + 2*T(d-1) + T(d) + 2) >> 2
				}
				out[y*8+x] = uint8(v)
			}
		}
	case Intra4x4VerticalRight:
		if err := needTop(); err != nil {
			return out, err
		}
		if err := needLeft(); err != nil {
			return out, err
		}
		if err := needCorner(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				k := 2*x - y
				var v int
				switch {
				case k < -1:
					v = (L(-k-1) + 2*L(-k-2) + L(-k-3) + 2) >> 2
				case k == -1:
					v = (l[0] + 2*lt + t[0] + 2) >> 2
				case k%2 == 1:
					m := (k - 1) / 2
					v = (T(m-1) + 2*T(m) + T(m+1) + 2) >> 2
				default:
					m := k / 2
					v = (T(m-1) + T(m) + 1) >> 1
				}
				out[y*8+x] = uint8(v)
			}
		}
	case Intra4x4HorizontalDown:
		if err := needTop(); err != nil {
			return out, err
		}
		if err := needLeft(); err != nil {
			return out, err
		}
		if err := needCorner(); err != nil {
			return out, err
		}
		// Mirror of VerticalRight with axes and reference sides swapped.
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				k := 2*y - x
				var v int
				switch {
				case k < -1:
					v = (T(-k-1) + 2*T(-k-2) + T(-k-3) + 2) >> 2
				case k == -1:
					v = (t[0] + 2*lt + l[0] + 2) >> 2
				case k%2 == 1:
					m := (k - 1) / 2
					v = (L(m-1) + 2*L(m) + L(m+1) + 2) >> 2
				default:
					m := k / 2
					v = (L(m-1) + L(m) + 1) >> 1
				}
				out[y*8+x] = uint8(v)
			}
		}
	case Intra4x4VerticalLeft:
		if err := needTop(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				k := 2*x + y
				var v int
				if k%2 == 0 {
					v = (t[k/2] + t[k/2+1] + 1) >> 1
				} else {
					m := (k - 1) / 2
					v = (t[m] + 2*t[m+1] + t[m+2] + 2) >> 2
				}
				out[y*8+x] = uint8(v)
			}
		}
	case Intra4x4HorizontalUp:
		if err := needLeft(); err != nil {
			return out, err
		}
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				k := x + 2*y
				var v int
				switch {
				case k >= 14:
					v = l[7]
				case k == 13:
					v = (l[6] + 3*l[7] + 2) >> 2
				case k%2 == 0:
					v = (l[k/2] + l[k/2+1] + 1) >> 1
				default:
					m := (k - 1) / 2
					v = (l[m] + 2*l[m+1] + l[m+2] + 2) >> 2
				}
				out[y*8+x] = uint8(v)
			}
		}
	default:
		return out, fmt.Errorf("%w: intra8x8 mode %d", ErrBadSliceHeader, mode)
	}
	return out, nil
}
