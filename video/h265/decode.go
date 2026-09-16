// V2-1 stub decoder: the registry entry for H.265 headers.
//
// It carries the parsed hvcC (VPS/SPS/PPS kept, LengthSize for sample
// splits) and reports yuv420p sampling so the color chain stays shared.
// Slice NALUs are refused with ErrNotDecodable (V2-2 owns pixels); header
// units are accepted so open-time parameter feeding never fails.
package h265

import (
	"errors"
	"fmt"

	"github.com/energye/gpui/video/h264"
)

// Decoder is the V2-1 registry decoder: headers in, honest refusal out.
type Decoder struct {
	hvcc *HVCC
	fed  int
}

// NewDecoder builds a clean decoder (fresh state per open/seek, same as
// the H.264 entry).
func NewDecoder(hvcc *HVCC) *Decoder { return &Decoder{hvcc: hvcc} }

// HVCCLengthSize reports the sample length prefix size (0 when unknown).
func (d *Decoder) HVCCLengthSize() int {
	if d == nil || d.hvcc == nil {
		return 0
	}
	return d.hvcc.LengthSize
}

// DecodeNALU accepts header units (VPS/SPS/PPS/SEI/AUD) and refuses slice
// payloads with a namable error so the player isolates the frame instead
// of flowering later.
func (d *Decoder) DecodeNALU(nalu []byte) error {
	t, ok := NALType(nalu)
	if !ok {
		return fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	d.fed++
	switch t {
	case NALVPS, NALSPS, NALPPS, NALPrefixSEI, NALSuffixSEI, NALAUD:
		return nil
	default:
		return fmt.Errorf("%w: nal type %d (%s)", ErrNotDecodable, t, TypeName(t))
	}
}

// FinishPicture always refuses: V2-1 carries no pixel engine (V2-2).
func (d *Decoder) FinishPicture() (*h264.Picture, error) {
	return nil, ErrNotDecodable
}

// Sampling reports the shared 8-bit 4:2:0 layout V2-2 will emit.
func (d *Decoder) Sampling() string { return "yuv420p" }

// IsNotDecodable reports the V2-1 honest refusal (player triage peeks at
// it without importing h265 error strings elsewhere).
func IsNotDecodable(err error) bool { return errors.Is(err, ErrNotDecodable) }
