package kit

import (
	qrcode "github.com/skip2/go-qrcode"
)

// QRCodeGenerateConfig is the pluggable QR matrix implementation.
// Components only talk to this interface, so swapping the encode
// library never touches component code. The default backend is the
// MIT-licensed pure-Go skip2/go-qrcode.
type QRCodeGenerateConfig interface {
	// Name identifies the implementation (e.g. "skip2").
	Name() string
	// Encode returns the raw module matrix (true = dark) for one
	// value at the given level. Empty value returns (nil, nil):
	// antd renders null, it never crashes.
	Encode(value string, level QRErrorLevel) ([][]bool, error)
}

// skip2QRCodeGenerate is the default pure-Go backend.
type skip2QRCodeGenerate struct{}

// DefaultQRCodeGenerateConfig returns the default backend.
func DefaultQRCodeGenerateConfig() QRCodeGenerateConfig {
	return skip2QRCodeGenerate{}
}

// Name identifies the default backend.
func (skip2QRCodeGenerate) Name() string { return "skip2" }

func skip2Level(l QRErrorLevel) qrcode.RecoveryLevel {
	switch l {
	case QRErrorLevelL:
		return qrcode.Low
	case QRErrorLevelQ:
		return qrcode.High
	case QRErrorLevelH:
		return qrcode.Highest
	default:
		return qrcode.Medium
	}
}

// Encode returns the raw module matrix for one value (quiet zone
// stripped: the backend bakes in a 4-module border, but margin belongs
// to the draw layer per qr-code.md §6.11).
func (skip2QRCodeGenerate) Encode(value string, level QRErrorLevel) ([][]bool, error) {
	if value == "" {
		return nil, nil
	}
	q, err := qrcode.New(value, skip2Level(level))
	if err != nil {
		return nil, err
	}
	return trimQRCodeQuietZone(q.Bitmap()), nil
}

// trimQRCodeQuietZone cuts all-white outer rows and columns.
func trimQRCodeQuietZone(b [][]bool) [][]bool {
	top, bottom := 0, len(b)
	for top < bottom {
		blank := true
		for _, v := range b[top] {
			if v {
				blank = false
				break
			}
		}
		if !blank {
			break
		}
		top++
	}
	for bottom > top {
		blank := true
		for _, v := range b[bottom-1] {
			if v {
				blank = false
				break
			}
		}
		if !blank {
			break
		}
		bottom--
	}
	if top >= bottom {
		return nil
	}
	left, right := 0, len(b[0])
	for left < right {
		blank := true
		for r := top; r < bottom; r++ {
			if b[r][left] {
				blank = false
				break
			}
		}
		if !blank {
			break
		}
		left++
	}
	for right > left {
		blank := true
		for r := top; r < bottom; r++ {
			if b[r][right-1] {
				blank = false
				break
			}
		}
		if !blank {
			break
		}
		right--
	}
	out := make([][]bool, 0, bottom-top)
	for r := top; r < bottom; r++ {
		out = append(out, append([]bool(nil), b[r][left:right]...))
	}
	return out
}

// QRCodeBoostLevels returns the attempt order for boostLevel: levels
// above the requested one first (high to requested), then below down
// to L so over-long content still encodes.
func QRCodeBoostLevels(requested QRErrorLevel) []QRErrorLevel {
	all := []QRErrorLevel{QRErrorLevelH, QRErrorLevelQ, QRErrorLevelM, QRErrorLevelL}
	idx := 2
	for i, l := range all {
		if l == requested {
			idx = i
			break
		}
	}
	out := append([]QRErrorLevel(nil), all[:idx+1]...)
	for _, l := range all[idx+1:] {
		out = append(out, l)
	}
	return out
}

// EncodeQRCodeValue encodes one value, boosting when enabled.
func EncodeQRCodeValue(gen QRCodeGenerateConfig, value string, requested QRErrorLevel, boost bool) (matrix [][]bool, used QRErrorLevel, err error) {
	if gen == nil {
		gen = DefaultQRCodeGenerateConfig()
	}
	if value == "" {
		return nil, requested, nil
	}
	if !boost {
		m, err := gen.Encode(value, requested)
		return m, requested, err
	}
	var lastErr error
	for _, l := range QRCodeBoostLevels(requested) {
		m, err := gen.Encode(value, l)
		if err == nil {
			return m, l, nil
		}
		lastErr = err
	}
	return nil, requested, lastErr
}
