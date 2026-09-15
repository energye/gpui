package aac

import (
	"math"
	"sync"
)

// Window tables computed once (sine + KBD). Peers sine_1024/sine_128
// and aac_kbd_long_1024/aac_kbd_short_128 in ffmpeg, but computed from
// formulas (spec 4.6.4.6.1 sine, 4.6.4.6.2 KBD), not copied.
var (
	winOnce  sync.Once
	sine1024 []float64
	sine128  []float64
	kbd1024  []float64
	kbd128   []float64
)

func besselI0(x float64) float64 {
	// Series + asymptotic peers av_bessel_i0; standard math, own code.
	ax := math.Abs(x)
	if ax < 3.75 {
		y := (x / 3.75)
		y *= y
		return 1 + y*(3.5156229+y*(3.0899424+y*(1.2067492+y*(0.2659732+y*(0.0360768+y*0.0045813)))))
	}
	y := 3.75 / ax
	return (math.Exp(ax) / math.Sqrt(ax)) * (0.39894228 + y*(0.01328592+y*(0.00225319+y*(-0.00157565+y*(0.00916281+y*(-0.02057706+y*(0.02635537+y*(-0.01647633+y*0.00392377))))))))
}

func kbdWindow(n int, alpha float64) []float64 {
	// Peers ff_kbd_window_init (kbdwin.c) formula.
	w := make([]float64, n)
	half := n / 2
	tmp := make([]float64, half+1)
	alpha2 := 4 * (alpha * math.Pi / float64(n)) * (alpha * math.Pi / float64(n))
	scale := 0.0
	for i := 0; i <= half; i++ {
		t := float64(i*(n-i)) * alpha2
		tmp[i] = besselI0(math.Sqrt(t))
		scale += tmp[i] * (1 + boolToFloat(i != 0 && i != half))
	}
	scale = 1.0 / (scale + 1)
	sum := 0.0
	for i := 0; i <= half; i++ {
		sum += tmp[i]
		w[i] = math.Sqrt(sum * scale)
	}
	for i := half + 1; i < n; i++ {
		sum += tmp[n-i]
		w[i] = math.Sqrt(sum * scale)
	}
	return w
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func ensureWindows() {
	winOnce.Do(func() {
		// Half windows only; ffmpeg builds the same halves with
		// ff_sine_window_init/ff_kbd_window_init and folds in
		// vector_fmul_window. sine_1024[i]=sin((i+.5)*pi/2048).
		sine1024 = make([]float64, 1024)
		for i := range sine1024 {
			sine1024[i] = math.Sin(math.Pi * (float64(i) + 0.5) / 2048)
		}
		sine128 = make([]float64, 128)
		for i := range sine128 {
			sine128[i] = math.Sin(math.Pi * (float64(i) + 0.5) / 256)
		}
		kbd1024 = kbdWindow(1024, 4.0)
		kbd128 = kbdWindow(128, 6.0)
	})
}

// imdctLongFolded transforms 1024 coeffs to the 1024 folded time
// samples ffmpeg chains forward (buf_mdct). It peers the HALF inverse
// MDCT (libavutil/tx_template.c ff_tx_mdct_naive_inv, len=1024 without
// AV_TX_FULL_IMDCT): the unfold lives in vectorFmulWindow, not here.
// Scale is (1/1024)/32768: 1/1024 inverts the encoder MDCT and 1/32768
// undoes its forward gain (aacenc.c scale=32768.0, decoder MDCT_INIT
// divides sval by 32768 in aacdec.c).
// Naive O(N^2); landing 2 correctness first, speed later.
func imdctLongFolded(in []float64) []float64 {
	const L = 1024
	const half = 512
	const phase = math.Pi / 4096.0
	out := make([]float64, L)
	scale := 1.0 / float64(L) / 32768.0
	for i := 0; i < half; i++ {
		iD := phase * float64(4*half-2*i-1)
		iU := phase * float64(6*half+2*i+1)
		sumD, sumU := 0.0, 0.0
		for j := 0; j < L; j++ {
			if in[j] == 0 {
				continue
			}
			a := 2*float64(j) + 1
			sumD += in[j] * math.Cos(a*iD)
			sumU += in[j] * math.Cos(a*iU)
		}
		out[i] = sumD * scale
		out[i+half] = -sumU * scale
	}
	return out
}

// imdctShortFolded transforms 128 coeffs to 128 folded samples
// (same (1/128)/32768 rule).
func imdctShortFolded(in []float64) []float64 {
	const L = 128
	const half = 64
	const phase = math.Pi / 512.0
	out := make([]float64, L)
	scale := 1.0 / float64(L) / 32768.0
	for i := 0; i < half; i++ {
		iD := phase * float64(4*half-2*i-1)
		iU := phase * float64(6*half+2*i+1)
		sumD, sumU := 0.0, 0.0
		for j := 0; j < L; j++ {
			if in[j] == 0 {
				continue
			}
			a := 2*float64(j) + 1
			sumD += in[j] * math.Cos(a*iD)
			sumU += in[j] * math.Cos(a*iU)
		}
		out[i] = sumD * scale
		out[i+half] = -sumU * scale
	}
	return out
}

// vectorFmulWindow peers float_dsp.c vector_fmul_window_c: it unfolds
// the folded IMDCT outputs with the window and overlaps in one step.
// It writes dst[do:do+2*length] from src0 (overlap state) and src1
// (fresh IMDCT fold) through win (length-2*length half window).
func vectorFmulWindow(dst []float64, do int, src0 []float64, o0 int, src1 []float64, o1 int, win []float64, wo int, length int) {
	for k := 0; k < length; k++ {
		j := length - 1 - k
		s0 := src0[o0+k]
		s1 := src1[o1+j]
		wi := win[wo+k]
		wj := win[wo+length+j]
		dst[do+k] = s0*wj - s1*wi
		dst[do+length+j] = s0*wi + s1*wj
	}
}

// Window sequence values (ISO 14496-3 Table 4.82).
const (
	wsOnlyLong  = 0
	wsLongStart = 1
	wsShort     = 2
	wsLongStop  = 3
)

// imdctAndWindowing1024 runs IMDCT plus windowing/overlap for one
// channel's 1024 spectral lines and returns 1024 PCM samples. saved is
// the 1024-sample overlap state, updated in place. It peers ffmpeg's
// imdct_and_windowing (aacdec_dsp_template.c) for 1024-line frames,
// covering ONLY_LONG, LONG_START, EIGHT_SHORT and LONG_STOP in both
// directions.
func imdctAndWindowing1024(coef, saved []float64, ics *icsInfo) []float64 {
	ensureWindows()
	swindow := sine128
	lwindowPrev, swindowPrev := sine1024, sine128
	if ics.useKbWindow[0] {
		swindow = kbd128
	}
	if ics.useKbWindow[1] {
		lwindowPrev, swindowPrev = kbd1024, kbd128
	}
	buf := make([]float64, 1024)
	if ics.windowSeq[0] == wsShort {
		for i := 0; i < 1024; i += 128 {
			copy(buf[i:i+128], imdctShortFolded(coef[i:i+128]))
		}
	} else {
		copy(buf, imdctLongFolded(coef))
	}
	out := make([]float64, 1024)
	temp := make([]float64, 128)
	prevSeq, curSeq := ics.windowSeq[1], ics.windowSeq[0]
	if (prevSeq == wsOnlyLong || prevSeq == wsLongStop) &&
		(curSeq == wsOnlyLong || curSeq == wsLongStart) {
		vectorFmulWindow(out, 0, saved, 0, buf, 0, lwindowPrev, 0, 512)
	} else {
		copy(out[:448], saved[:448])
		if curSeq == wsShort {
			vectorFmulWindow(out, 448+0*128, saved, 448, buf, 0*128, swindowPrev, 0, 64)
			vectorFmulWindow(out, 448+1*128, buf, 0*128+64, buf, 1*128, swindow, 0, 64)
			vectorFmulWindow(out, 448+2*128, buf, 1*128+64, buf, 2*128, swindow, 0, 64)
			vectorFmulWindow(out, 448+3*128, buf, 2*128+64, buf, 3*128, swindow, 0, 64)
			vectorFmulWindow(temp, 0, buf, 3*128+64, buf, 4*128, swindow, 0, 64)
			copy(out[448+4*128:], temp[:64])
		} else {
			vectorFmulWindow(out, 448, saved, 448, buf, 0, swindowPrev, 0, 64)
			copy(out[576:], buf[64:64+448])
		}
	}
	if curSeq == wsShort {
		copy(saved[0:64], temp[64:128])
		vectorFmulWindow(saved, 64, buf, 4*128+64, buf, 5*128, swindow, 0, 64)
		vectorFmulWindow(saved, 192, buf, 5*128+64, buf, 6*128, swindow, 0, 64)
		vectorFmulWindow(saved, 320, buf, 6*128+64, buf, 7*128, swindow, 0, 64)
		copy(saved[448:512], buf[7*128+64:7*128+128])
	} else if curSeq == wsLongStart {
		copy(saved[0:448], buf[512:960])
		copy(saved[448:512], buf[7*128+64:7*128+128])
	} else {
		copy(saved[0:512], buf[512:1024])
	}
	return out
}
