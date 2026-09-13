package color

import (
	"fmt"
	"sort"
	"sync"
)

// Sampling names the input pixel layout. Only the decoder's native
// 4:2:0 plan is converted this stage; new layouts plug in via Register.
const SamplingYUV420P = "yuv420p"

// Matrix ids mirror H.264 matrix_coefficients (Annex E). Converters only
// implement the two matrices real clips use; the rest fail loudly.
const (
	MatrixRGB         uint32 = 0
	MatrixBT709       uint32 = 1
	MatrixUnspecified uint32 = 2
	MatrixFCC         uint32 = 4
	MatrixBT470BG     uint32 = 5
	MatrixSMPTE170M   uint32 = 6
	MatrixSMPTE240M   uint32 = 7
)

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrBadSize             = fmt.Errorf("color: bad frame size")
	ErrBadPlanes           = fmt.Errorf("color: bad yuv planes")
	ErrUnsupportedSampling = fmt.Errorf("color: unsupported sampling")
	ErrUnsupportedMatrix   = fmt.Errorf("color: unsupported colour matrix")
)

// Options selects range and matrix. MatrixPresent mirrors the VUI colour
// description flag: absent (or Unspecified) means the encoder said nothing,
// and VR3 defaults to BT.601, matching the studio-range clips in gate.
type Options struct {
	FullRange     bool
	Matrix        uint32
	MatrixPresent bool
}

// OptionsFromVUI keeps the h264.VUI wiring in one place without importing
// the decoder: pass FullRange, ColourPresent and ColourMatrix straight in.
func OptionsFromVUI(fullRange, colourPresent bool, colourMatrix uint32) Options {
	return Options{FullRange: fullRange, Matrix: colourMatrix, MatrixPresent: colourPresent}
}

// MatrixName returns the short label used in errors and logs.
func MatrixName(m uint32) string {
	switch m {
	case MatrixRGB:
		return "RGB"
	case MatrixBT709:
		return "BT.709"
	case MatrixUnspecified:
		return "unspecified"
	case MatrixFCC:
		return "FCC"
	case MatrixBT470BG:
		return "BT.470BG"
	case MatrixSMPTE170M:
		return "SMPTE-170M"
	case MatrixSMPTE240M:
		return "SMPTE-240M"
	default:
		return fmt.Sprintf("matrix-%d", m)
	}
}

// Frame is one owned RGBA picture in raster order.
type Frame struct {
	Width  int
	Height int
	Pix    []byte // RGBA, length 4*Width*Height
}

// At returns the RGBA tuple at (x, y), or zeros when out of range.
func (f *Frame) At(x, y int) (r, g, b, a uint8) {
	if f == nil || x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return 0, 0, 0, 0
	}
	o := (y*f.Width + x) * 4
	if o+3 >= len(f.Pix) {
		return 0, 0, 0, 0
	}
	return f.Pix[o], f.Pix[o+1], f.Pix[o+2], f.Pix[o+3]
}

// Converter converts one planar frame into packed RGBA dst
// (length at least 4*w*h). Registered per sampling name.
type Converter func(dst, y, cb, cr []byte, w, h int, opt Options) error

var (
	mu       sync.RWMutex
	registry = map[string]Converter{}
)

func init() {
	// Default sampling. Extra layouts arrive via Register (VR9); the
	// Convert path below never hardcodes a sampling name.
	registry[SamplingYUV420P] = convert420Into
}

// Register plugs a new input sampling into the conversion table.
// Registering an existing name replaces it (tests use this to prove the
// table is consulted); production code registers once at init.
func Register(sampling string, c Converter) {
	if sampling == "" || c == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	registry[sampling] = c
}

// Supported lists registered samplings in sorted order for capability
// queries (VR9 asks first, then opens).
func Supported() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lookup returns the converter for a sampling or a naming error.
func lookup(sampling string) (Converter, error) {
	mu.RLock()
	defer mu.RUnlock()
	if c, ok := registry[sampling]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("%w: %q (have %v)", ErrUnsupportedSampling, sampling, Supported())
}

// resolveMatrix picks the conversion matrix or names the unsupported one.
// Absent/unspecified input defaults to SMPTE-170M (BT.601); BT.470BG shares
// the BT.601 coefficients, so callers see either label with equal output.
func resolveMatrix(opt Options) (uint32, error) {
	if !opt.MatrixPresent || opt.Matrix == MatrixUnspecified {
		return MatrixSMPTE170M, nil
	}
	switch opt.Matrix {
	case MatrixBT709, MatrixBT470BG, MatrixSMPTE170M:
		return opt.Matrix, nil
	default:
		return 0, fmt.Errorf("%w: %s (VR3 handles BT.709/BT.601 only)", ErrUnsupportedMatrix, MatrixName(opt.Matrix))
	}
}

// checkPlanes validates even dims and plane sizes shared by both entries.
func checkPlanes(y, cb, cr []byte, w, h int) error {
	if w <= 0 || h <= 0 || w%2 != 0 || h%2 != 0 || w > 8192 || h > 8192 {
		return fmt.Errorf("%w: size %dx%d (yuv420 needs even dims)", ErrBadSize, w, h)
	}
	if len(y) < w*h || len(cb) < w*h/4 || len(cr) < w*h/4 {
		return fmt.Errorf("%w: planes %d/%d/%d for %dx%d", ErrBadPlanes, len(y), len(cb), len(cr), w, h)
	}
	return nil
}

// Convert decodes one frame through the registry (allocates the output).
func Convert(sampling string, y, cb, cr []byte, w, h int, opt Options) (*Frame, error) {
	c, err := lookup(sampling)
	if err != nil {
		return nil, err
	}
	if err := checkPlanes(y, cb, cr, w, h); err != nil {
		return nil, err
	}
	if _, err := resolveMatrix(opt); err != nil {
		return nil, err
	}
	dst := make([]byte, w*h*4)
	if err := c(dst, y, cb, cr, w, h, opt); err != nil {
		return nil, err
	}
	return &Frame{Width: w, Height: h, Pix: dst}, nil
}

// ConvertInto reuses the caller's dst (hot path: borrow from a pool, no
// per-frame big allocation). dst must hold at least 4*w*h bytes.
func ConvertInto(sampling string, dst, y, cb, cr []byte, w, h int, opt Options) error {
	c, err := lookup(sampling)
	if err != nil {
		return err
	}
	if err := checkPlanes(y, cb, cr, w, h); err != nil {
		return err
	}
	if len(dst) < w*h*4 {
		return fmt.Errorf("%w: dst %d for %dx%d", ErrBadSize, len(dst), w, h)
	}
	if _, err := resolveMatrix(opt); err != nil {
		return err
	}
	return c(dst, y, cb, cr, w, h, opt)
}

// coeffs is one fixed-point matrix: out = (yMul*C +/- taps + 128) >> 8.
type coeffs struct {
	yMul     int
	yOff     int
	rCr      int
	gCb, gCr int
	bCb      int
}

func tableFor(matrix uint32, fullRange bool) coeffs {
	bt709 := matrix == MatrixBT709
	if fullRange {
		if bt709 {
			return coeffs{yMul: 256, yOff: 0, rCr: 403, gCb: 48, gCr: 120, bCb: 475}
		}
		return coeffs{yMul: 256, yOff: 0, rCr: 359, gCb: 88, gCr: 183, bCb: 454}
	}
	if bt709 {
		return coeffs{yMul: 298, yOff: 16, rCr: 459, gCb: 55, gCr: 136, bCb: 541}
	}
	return coeffs{yMul: 298, yOff: 16, rCr: 409, gCb: 100, gCr: 208, bCb: 516}
}

func clip8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// convert420Into is the yuv420p converter: each 2x2 luma quad shares one
// chroma pair (nearest). The loop walks two luma pixels per chroma tap
// with row slices, so the compiler drops per-pixel divisions and bounds
// checks; output bits are unchanged (vectors pin them). Chroma location
// offsets stay out of VR3; the gate clips are progressive with default
// siting, and the difference is far below the per-channel tolerance.
func convert420Into(dst, y, cb, cr []byte, w, h int, opt Options) error {
	m, err := resolveMatrix(opt)
	if err != nil {
		return err
	}
	t := tableFor(m, opt.FullRange)
	yMul, yOff := t.yMul, t.yOff
	rCr, gCb, gCr, bCb := t.rCr, t.gCb, t.gCr, t.bCb
	cw := w / 2
	for yy := 0; yy < h; yy++ {
		yRow := y[yy*w : (yy+1)*w]
		dRow := dst[yy*w*4 : (yy+1)*w*4]
		cBase := (yy >> 1) * cw
		cbRow := cb[cBase : cBase+cw]
		crRow := cr[cBase : cBase+cw]
		for xx := 0; xx < w; xx += 2 {
			ci := xx >> 1
			d := int(cbRow[ci]) - 128
			e := int(crRow[ci]) - 128
			// Shared taps: identical rounding to the per-pixel form,
			// (yMul*c +/- taps + 128) >> 8, only factored per pair.
			re := rCr * e
			ge := gCr * e
			gd := gCb * d
			bd := bCb * d
			o := xx * 4
			y0 := yMul * (int(yRow[xx]) - yOff)
			dRow[o] = clip8((y0 + re + 128) >> 8)
			dRow[o+1] = clip8((y0 - gd - ge + 128) >> 8)
			dRow[o+2] = clip8((y0 + bd + 128) >> 8)
			dRow[o+3] = 255
			y1 := yMul * (int(yRow[xx+1]) - yOff)
			dRow[o+4] = clip8((y1 + re + 128) >> 8)
			dRow[o+5] = clip8((y1 - gd - ge + 128) >> 8)
			dRow[o+6] = clip8((y1 + bd + 128) >> 8)
			dRow[o+7] = 255
		}
	}
	return nil
}
