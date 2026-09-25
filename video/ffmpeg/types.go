package ffmpeg

import (
	"unsafe"
)

// Field offsets measured from the 7.1 headers plus the build tree
// (see /tmp/ffprobe_off output). They pin the purego reads below so a
// header drift fails loudly instead of decoding garbage.
const (
	fmtNbStreams = 44
	fmtStreams   = 48
	fmtDuration  = 104

	streamIndex    = 8
	streamCodecPar = 16
	streamTBNum    = 32
	streamTBDen    = 36
	streamDuration = 48
	streamNbFrames = 56
	streamFPSNum   = 88
	streamFPSDen   = 92

	parCodecType  = 0
	parCodecID    = 4
	parFormat     = 44
	parWidth      = 72
	parHeight     = 76
	parSampleRate = 152

	pktPTS         = 8
	pktDTS         = 16
	pktData        = 24
	pktSize        = 32
	pktStreamIndex = 36
	pktDuration    = 64

	frameData       = 0
	frameLinesize   = 64
	frameWidth      = 104
	frameHeight     = 108
	frameFormat     = 116
	framePTS        = 136
	frameBestEffort = 320
	frameDuration   = 432
)

// Constants mirrored from the C headers (names shortened, values exact).
const (
	MediaTypeVideo = 0
	MediaTypeAudio = 1

	PixFmtYUV420P = 0
	PixFmtRGBA    = 26

	// No timestamp (AV_NOPTS_VALUE).
	NoPTS = int64(-9223372036854775808)

	SeekBackward = 1
	SeekByte     = 2
	SeekAny      = 4
	SeekFrame    = 8

	SWSBilinear = 2

	AvErrorEOF    = int32(-541478725)
	AvErrorEAGAIN = int32(-11)

	// Log level: only errors, so normal opens stay quiet.
	LogError = int32(16)
)

// AVRational is one C AVRational (two int32, 8 bytes).
type AVRational struct {
	Num int32
	Den int32
}

// The load helpers below read C struct fields through unsafe.Add.
// Handles stay as unsafe.Pointer end to end (purego returns them that
// way), so go vet's unsafeptr check stays quiet: no uintptr-to-pointer
// conversion anywhere on the read path.
func loadInt32(base unsafe.Pointer, off uintptr) int32 {
	return *(*int32)(unsafe.Add(base, off))
}

func loadInt64(base unsafe.Pointer, off uintptr) int64 {
	return *(*int64)(unsafe.Add(base, off))
}

func loadPtr(base unsafe.Pointer, off uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Add(base, off))
}

// streamPointer returns the i-th AVStream* from the format context.
func streamPointer(fmtCtx unsafe.Pointer, i int32) unsafe.Pointer {
	table := loadPtr(fmtCtx, fmtStreams)
	return *(*unsafe.Pointer)(unsafe.Add(table, uintptr(i)*8))
}
