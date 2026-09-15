package aac

// TNS Parcor tables peers ISO 14496-3 Table 4.48 (tmp2 maps); only the
// numbers travel, filter logic is written from scratch.
var tnsTable03 = [8]float64{
	0.0, -0.43388373, -0.78183150, -0.97492790,
	0.98480773, 0.86602539, 0.64278758, 0.34202015,
}

var tnsTable04 = [16]float64{
	0.0, -0.20791170, -0.40673664, -0.58778524,
	-0.74314481, -0.86602539, -0.95105654, -0.99452192,
	0.99573416, 0.96182561, 0.89516330, 0.79801720,
	0.67369562, 0.52643216, 0.36124167, 0.18374951,
}

var tnsTable13 = [4]float64{
	0.0, -0.43388373, 0.64278758, 0.34202015,
}

var tnsTable14 = [8]float64{
	0.0, -0.20791170, -0.40673664, -0.58778524,
	0.67369562, 0.52643216, 0.36124167, 0.18374951,
}

// tnsParcor maps (coefRes, coefCompress, index) to Parcor value.
func tnsParcor(coefRes, coefCompress, idx int) float64 {
	tab := 2*coefCompress + coefRes
	switch tab {
	case 0:
		if idx < 0 || idx >= len(tnsTable03) {
			return 0
		}
		return tnsTable03[idx]
	case 1:
		if idx < 0 || idx >= len(tnsTable04) {
			return 0
		}
		return tnsTable04[idx]
	case 2:
		if idx < 0 || idx >= len(tnsTable13) {
			return 0
		}
		return tnsTable13[idx]
	default:
		if idx < 0 || idx >= len(tnsTable14) {
			return 0
		}
		return tnsTable14[idx]
	}
}

// parcorToLPC converts Parcor coefficients to LPC via Levinson-Durbin
// peers compute_lpc_coefs (normalize=0, fail=0 path), which negates
// each input (r = -autoc[i]); TNS passes its Parcor values straight
// in, so the negation applies to them.
func parcorToLPC(parcor []float64, order int) []float64 {
	lpc := make([]float64, order)
	for i := 0; i < order; i++ {
		r := -parcor[i]
		lpc[i] = r
		for j := 0; j < (i+1)>>1; j++ {
			f := lpc[j]
			b := lpc[i-1-j]
			// Preserve originals for symmetric update.
			lpc[j] = f + r*b
			lpc[i-1-j] = b + r*f
		}
	}
	return lpc
}

// applyTNSFilter runs the decode-direction AR filter over coef.
// ics supplies windows/offsets; tns holds parsed filters; samplingIdx
// selects tns_max_bands. It peers apply_tns (decode=1).
func applyTNSFilter(coef []float64, ics *icsInfo, tns *tnsInfo, samplingIdx int) {
	var maxBands int
	if ics.windowSeq[0] == 2 {
		if samplingIdx < 0 || samplingIdx >= len(tnsMax_tns_max_bands_128) {
			return
		}
		maxBands = int(tnsMax_tns_max_bands_128[samplingIdx])
	} else {
		if samplingIdx < 0 || samplingIdx >= len(tnsMax_tns_max_bands_1024) {
			return
		}
		maxBands = int(tnsMax_tns_max_bands_1024[samplingIdx])
	}
	mmm := maxBands
	if ics.maxSfb < mmm {
		mmm = ics.maxSfb
	}
	if mmm <= 0 {
		return
	}
	for w := 0; w < ics.numWindows; w++ {
		bottom := ics.numSwb
		for f := 0; f < tns.nFilt[w]; f++ {
			top := bottom
			bottom -= tns.length[w][f]
			if bottom < 0 {
				bottom = 0
			}
			order := tns.order[w][f]
			if order == 0 {
				continue
			}
			lpc := parcorToLPC(tns.coef[w][f][:order], order)
			start := int(ics.swbOffset[min(top, mmm)])
			end := int(ics.swbOffset[min(bottom, mmm)])
			// Note: ffmpeg uses bottom/top swapped? It sets
			// bottom=numSwb then top=bottom, bottom-=length, so
			// start=offset[bottom], end=offset[top]. Replicate.
			// Above I swapped; fix:
			start = int(ics.swbOffset[min(bottom, mmm)])
			end = int(ics.swbOffset[min(top, mmm)])
			if end-start <= 0 {
				continue
			}
			inc := 1
			s := start
			if tns.dir[w][f] {
				inc = -1
				s = end - 1
			}
			s += w * 128
			for m := 0; m < end-start; m++ {
				for i := 1; i <= min(m, order); i++ {
					coef[s] -= coef[s-i*inc] * lpc[i-1]
				}
				s += inc
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
