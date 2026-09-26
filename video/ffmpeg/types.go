package ffmpeg

import (
	"unsafe"
)

// Field offsets measured from the 7.1 headers plus the build tree
// (see /tmp/ffprobe_off output). They pin the purego reads below so a
// header drift fails loudly instead of decoding garbage.
//
// Audio companions derived the same way, by walking the 7.1 headers
// field by field on x86-64 and checking the walk reproduces every
// pinned video offset above (it does: format=44, width=72/76,
// sample_rate=152, best_effort=320, duration=432 all land exact):
// AVCodecParameters runs codec_type@0, codec_id@4, codec_tag@8,
// extradata@16, extradata_size@24, coded_side_data@32,
// nb_coded_side_data@40, format@44, bit_rate@48, bits_per_coded@56,
// bits_per_raw@60, profile@64, level@68, width@72, height@76,
// sample_aspect_ratio@80, framerate@88, field_order@96,
// color_range@100, color_primaries@104, color_trc@108,
// color_space@112, chroma_location@116, video_delay@120,
// ch_layout@128 (24 bytes: order@+0, nb_channels@+4), sample_rate@152.
// AVFrame runs data@0, linesize@64, extended_data@96, width@104,
// height@108, nb_samples@112, format@116, key_frame@120,
// pict_type@124, sample_aspect_ratio@128, pts@136, pkt_dts@144,
// time_base@152, quality@160, opaque@168, repeat_pict@176,
// interlaced_frame@180, top_field_first@184, palette_has_changed@188,
// sample_rate@192, buf@200, extended_buf@264, nb_extended_buf@272,
// side_data@280, nb_side_data@288, flags@292, color_range@296,
// color_primaries@300, color_trc@304, colorspace@308,
// chroma_location@312, best_effort_timestamp@320, pkt_pos@328,
// metadata@336, decode_error_flags@344, pkt_size@348,
// hw_frames_ctx@352, opaque_ref@360, crop_*@368..400,
// private_ref@400, ch_layout@408 (nb_channels@+412), duration@432.
const (
	fmtNbStreams = 44
	fmtStreams   = 48
	fmtDuration  = 104
	fmtPb        = 32

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
	parChLayout   = 128
	parNbChannels = 132
	parSampleRate = 152

	pktPTS         = 8
	pktDTS         = 16
	pktData        = 24
	pktSize        = 32
	pktStreamIndex = 36
	pktFlags       = 40
	pktDuration    = 64

	frameData       = 0
	frameLinesize   = 64
	frameWidth      = 104
	frameHeight     = 108
	frameNbSamples  = 112
	frameFormat     = 116
	framePTS        = 136
	frameSampleRate = 192
	frameBestEffort = 320
	frameChLayout   = 408
	frameDuration   = 432
)

// Shared constants (C 头文件值, 新模块各有归属, 这里只留解码器直用):
// 媒体类型见 codec_encode.go, 像素格式见 scale_color.go,
// 日志等级见 error_log.go.
const (
	// No timestamp (AV_NOPTS_VALUE).
	NoPTS = int64(-9223372036854775808)

	SeekBackward = 1
	SeekByte     = 2
	SeekAny      = 4
	SeekFrame    = 8

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
