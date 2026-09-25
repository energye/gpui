package ffmpeg

import (
	"errors"
	"fmt"
	"unsafe"
)

var (
	errNilPacket = errors.New("ffmpeg: nil packet")
	errNilFrame  = errors.New("ffmpeg: nil frame")
	errNilDict   = errors.New("ffmpeg: nil dictionary")
	errNilOpt    = errors.New("ffmpeg: nil option object")
	errNilBuffer = errors.New("ffmpeg: nil buffer")
	errNilFifo   = errors.New("ffmpeg: nil fifo")
	errNilFF     = errors.New("ffmpeg: nil holder")
	errNilCodec  = errors.New("ffmpeg: nil codec")
	errNilScale  = errors.New("ffmpeg: nil scaler")
	errNilDevice = errors.New("ffmpeg: nil device")
	errNilHW     = errors.New("ffmpeg: nil hwdevice")
)

// cstr reads a NUL-terminated C string ("" on nil).
func cstr(p unsafe.Pointer) string {
	if p == nil {
		return ""
	}
	var b []byte
	for i := uintptr(0); ; i++ {
		c := *(*byte)(unsafe.Add(p, i))
		if c == 0 {
			break
		}
		b = append(b, c)
		if len(b) > 1<<20 {
			break
		}
	}
	return string(b)
}

// codeErr turns a negative AVERROR into a readable Go error.
func codeErr(op string, code int32) error {
	if ensureLoaded() != nil {
		return fmt.Errorf("ffmpeg: %s: error %d", op, code)
	}
	return fmt.Errorf("ffmpeg: %s: %s", op, errText(code))
}
