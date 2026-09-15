package aac

import (
	"fmt"
)

// tnsInfo holds parsed TNS data for one channel (filter applied in
// rawblock.go after stereo; presence flag drives it).
type tnsInfo struct {
	present bool
	nFilt   [8]int
	length  [8][4]int
	order   [8][4]int
	dir     [8][4]bool
	coef    [8][4][20]float64
}

// pulseInfo holds pulse data.
type pulseInfo struct {
	present bool
	num     int
	pos     [4]int
	amp     [4]int
}

// channelPayload is one decoded SCE payload before IMDCT.
type channelPayload struct {
	ch     singleChannel
	coef   []float64 // 1024 or 8*128 lines
	tns    tnsInfo
	pulse  pulseInfo
	msMask []bool // only for CPE right? stored per CPE
}

// parseTNS reads TNS data (ff_aac_decode_tns). SamplingIdx selects
// max order; isShort selects short-window path.
func parseTNS(r *bitReaderMSB, ics *icsInfo, objectType int) (tnsInfo, error) {
	var t tnsInfo
	isShort := ics.windowSeq[0] == 2
	maxOrder := 12
	if objectType == AOTMain {
		maxOrder = 20
	}
	if isShort {
		maxOrder = 7
	}
	numWindows := ics.numWindows
	if numWindows < 1 {
		numWindows = 1
	}
	if numWindows > 8 {
		return t, fmt.Errorf("%w: windows %d", ErrBadADTS, numWindows)
	}
	for w := 0; w < numWindows; w++ {
		var n int
		var err error
		if isShort {
			n, err = r.read(1)
		} else {
			n, err = r.read(2)
		}
		if err != nil {
			return t, err
		}
		t.nFilt[w] = n
		if n == 0 {
			continue
		}
		t.present = true
		coefRes, err := r.read1()
		if err != nil {
			return t, err
		}
		for f := 0; f < n; f++ {
			var lengthBits, orderBits int
			if isShort {
				lengthBits = 4
				orderBits = 3
			} else {
				lengthBits = 6
				orderBits = 5
			}
			l, err := r.read(lengthBits)
			if err != nil {
				return t, err
			}
			o, err := r.read(orderBits)
			if err != nil {
				return t, err
			}
			if o > maxOrder {
				return t, fmt.Errorf("%w: tns order %d", ErrBadADTS, o)
			}
			t.length[w][f] = l
			t.order[w][f] = o
			if o == 0 {
				continue
			}
			d, err := r.read1()
			if err != nil {
				return t, err
			}
			c, err := r.read1()
			if err != nil {
				return t, err
			}
			t.dir[w][f] = d == 1
			coefLen := coefRes + 3 - c
			_ = coefLen
			for i := 0; i < o; i++ {
				idx, err := r.read(coefRes + 3 - c)
				if err != nil {
					return t, err
				}
				t.coef[w][f][i] = tnsParcor(coefRes, c, idx)
			}
		}
	}
	return t, nil
}
