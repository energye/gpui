package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// ResampleAudio 重采样模块: SwrContext/AudioFifo/采样格式 全量导出, 结构体方法直接可用.
//
// Say it plain: 把声音转成要的采样率声道格式, 播之前先对齐.

// Resampler holder.
type Resampler struct{ ptr unsafe.Pointer }

func (x *Resampler) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

// AudioFifo holder.
type AudioFifo struct{ ptr unsafe.Pointer }

func (x *AudioFifo) Ptr() unsafe.Pointer {
	if x == nil {
		return nil
	}
	return x.ptr
}

var (
	fAvAudioFifoAlloc     func(sample_fmt unsafe.Pointer, channels int32, nb_samples int32) unsafe.Pointer
	fAvAudioFifoDrain     func(af unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoFree      func(af unsafe.Pointer)
	fAvAudioFifoPeek      func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoPeekAt    func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32, offset int32) int32
	fAvAudioFifoRead      func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoRealloc   func(af unsafe.Pointer, nb_samples int32) int32
	fAvAudioFifoReset     func(af unsafe.Pointer)
	fAvAudioFifoSize      func(af unsafe.Pointer) int32
	fAvAudioFifoSpace     func(af unsafe.Pointer) int32
	fAvAudioFifoWrite     func(af unsafe.Pointer, data unsafe.Pointer, nb_samples int32) int32
	fAvGetBytesPerSample  func(sample_fmt unsafe.Pointer) int32
	fAvGetPackedSampleFmt func(sample_fmt unsafe.Pointer) unsafe.Pointer
	fAvGetPlanarSampleFmt func(sample_fmt unsafe.Pointer) unsafe.Pointer
	fAvGetSampleFmt       func(name unsafe.Pointer) unsafe.Pointer
	fAvGetSampleFmtName   func(sample_fmt unsafe.Pointer) unsafe.Pointer
	fAvGetSampleFmtString func(buf unsafe.Pointer, buf_size int32, sample_fmt unsafe.Pointer) unsafe.Pointer
	fAvSampleFmtIsPlanar  func(sample_fmt unsafe.Pointer) int32
	fSwrAlloc             func() unsafe.Pointer
	fSwrAllocSetOpts2     func(ps *unsafe.Pointer, out_ch_layout unsafe.Pointer, out_sample_fmt unsafe.Pointer, out_sample_rate int32, in_ch_layout unsafe.Pointer, in_sample_fmt unsafe.Pointer, in_sample_rate int32, log_offset int32, log_ctx unsafe.Pointer) int32
	fSwrBuildMatrix2      func(in_layout unsafe.Pointer, out_layout unsafe.Pointer, center_mix_level float64, surround_mix_level float64, lfe_mix_level float64, maxval float64, rematrix_volume float64, matrix unsafe.Pointer, stride unsafe.Pointer, matrix_encoding unsafe.Pointer, log_context unsafe.Pointer) int32
	fSwrClose             func(s unsafe.Pointer)
	fSwrConfigFrame       func(swr unsafe.Pointer, out unsafe.Pointer, in unsafe.Pointer) int32
	fSwrConvert           func(s unsafe.Pointer, out unsafe.Pointer, out_count int32, in unsafe.Pointer, in_count int32) int32
	fSwrConvertFrame      func(swr unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) int32
	fSwrDropOutput        func(s unsafe.Pointer, count int32) int32
	fSwrFree              func(s *unsafe.Pointer)
	fSwrGetClass          func() unsafe.Pointer
	fSwrGetDelay          func(s unsafe.Pointer, base int64) int64
	fSwrGetOutSamples     func(s unsafe.Pointer, in_samples int32) int32
	fSwrInit              func(s unsafe.Pointer) int32
	fSwrInjectSilence     func(s unsafe.Pointer, count int32) int32
	fSwrIsInitialized     func(s unsafe.Pointer) int32
	fSwrNextPts           func(s unsafe.Pointer, pts int64) int64
	fSwrSetChannelMapping func(s unsafe.Pointer, channel_map unsafe.Pointer) int32
	fSwrSetCompensation   func(s unsafe.Pointer, sample_delta int32, compensation_distance int32) int32
	fSwrSetMatrix         func(s unsafe.Pointer, matrix unsafe.Pointer, stride int32) int32
)

func registerResampleAudio(h uintptr) {
	purego.RegisterLibFunc(&fAvAudioFifoAlloc, h, "av_audio_fifo_alloc")
	purego.RegisterLibFunc(&fAvAudioFifoDrain, h, "av_audio_fifo_drain")
	purego.RegisterLibFunc(&fAvAudioFifoFree, h, "av_audio_fifo_free")
	purego.RegisterLibFunc(&fAvAudioFifoPeek, h, "av_audio_fifo_peek")
	purego.RegisterLibFunc(&fAvAudioFifoPeekAt, h, "av_audio_fifo_peek_at")
	purego.RegisterLibFunc(&fAvAudioFifoRead, h, "av_audio_fifo_read")
	purego.RegisterLibFunc(&fAvAudioFifoRealloc, h, "av_audio_fifo_realloc")
	purego.RegisterLibFunc(&fAvAudioFifoReset, h, "av_audio_fifo_reset")
	purego.RegisterLibFunc(&fAvAudioFifoSize, h, "av_audio_fifo_size")
	purego.RegisterLibFunc(&fAvAudioFifoSpace, h, "av_audio_fifo_space")
	purego.RegisterLibFunc(&fAvAudioFifoWrite, h, "av_audio_fifo_write")
	purego.RegisterLibFunc(&fAvGetBytesPerSample, h, "av_get_bytes_per_sample")
	purego.RegisterLibFunc(&fAvGetPackedSampleFmt, h, "av_get_packed_sample_fmt")
	purego.RegisterLibFunc(&fAvGetPlanarSampleFmt, h, "av_get_planar_sample_fmt")
	purego.RegisterLibFunc(&fAvGetSampleFmt, h, "av_get_sample_fmt")
	purego.RegisterLibFunc(&fAvGetSampleFmtName, h, "av_get_sample_fmt_name")
	purego.RegisterLibFunc(&fAvGetSampleFmtString, h, "av_get_sample_fmt_string")
	purego.RegisterLibFunc(&fAvSampleFmtIsPlanar, h, "av_sample_fmt_is_planar")
	purego.RegisterLibFunc(&fSwrAlloc, h, "swr_alloc")
	purego.RegisterLibFunc(&fSwrAllocSetOpts2, h, "swr_alloc_set_opts2")
	purego.RegisterLibFunc(&fSwrBuildMatrix2, h, "swr_build_matrix2")
	purego.RegisterLibFunc(&fSwrClose, h, "swr_close")
	purego.RegisterLibFunc(&fSwrConfigFrame, h, "swr_config_frame")
	purego.RegisterLibFunc(&fSwrConvert, h, "swr_convert")
	purego.RegisterLibFunc(&fSwrConvertFrame, h, "swr_convert_frame")
	purego.RegisterLibFunc(&fSwrDropOutput, h, "swr_drop_output")
	purego.RegisterLibFunc(&fSwrFree, h, "swr_free")
	purego.RegisterLibFunc(&fSwrGetClass, h, "swr_get_class")
	purego.RegisterLibFunc(&fSwrGetDelay, h, "swr_get_delay")
	purego.RegisterLibFunc(&fSwrGetOutSamples, h, "swr_get_out_samples")
	purego.RegisterLibFunc(&fSwrInit, h, "swr_init")
	purego.RegisterLibFunc(&fSwrInjectSilence, h, "swr_inject_silence")
	purego.RegisterLibFunc(&fSwrIsInitialized, h, "swr_is_initialized")
	purego.RegisterLibFunc(&fSwrNextPts, h, "swr_next_pts")
	purego.RegisterLibFunc(&fSwrSetChannelMapping, h, "swr_set_channel_mapping")
	purego.RegisterLibFunc(&fSwrSetCompensation, h, "swr_set_compensation")
	purego.RegisterLibFunc(&fSwrSetMatrix, h, "swr_set_matrix")
}

func (x *Resampler) Alloc(sample_fmt unsafe.Pointer, channels int32, nb_samples int32) unsafe.Pointer {
	return fAvAudioFifoAlloc(sample_fmt, channels, nb_samples)
}

func (x *AudioFifo) Drain(nb_samples int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoDrain(x.ptr, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_drain", ret)
	}
	return nil
}

func (x *AudioFifo) Free() {
	if x == nil {
		return
	}
	fAvAudioFifoFree(x.ptr)
}

func (x *AudioFifo) Peek(data unsafe.Pointer, nb_samples int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoPeek(x.ptr, data, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_peek", ret)
	}
	return nil
}

func (x *AudioFifo) PeekAt(data unsafe.Pointer, nb_samples int32, offset int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoPeekAt(x.ptr, data, nb_samples, offset); ret < 0 {
		return codeErr("av_audio_fifo_peek_at", ret)
	}
	return nil
}

func (x *AudioFifo) Read(data unsafe.Pointer, nb_samples int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoRead(x.ptr, data, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_read", ret)
	}
	return nil
}

func (x *AudioFifo) Realloc(nb_samples int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoRealloc(x.ptr, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_realloc", ret)
	}
	return nil
}

func (x *AudioFifo) Reset() {
	if x == nil {
		return
	}
	fAvAudioFifoReset(x.ptr)
}

func (x *AudioFifo) Size() int32 {
	if x == nil {
		return 0
	}
	return fAvAudioFifoSize(x.ptr)
}

func (x *AudioFifo) Space() error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoSpace(x.ptr); ret < 0 {
		return codeErr("av_audio_fifo_space", ret)
	}
	return nil
}

func (x *AudioFifo) Write(data unsafe.Pointer, nb_samples int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fAvAudioFifoWrite(x.ptr, data, nb_samples); ret < 0 {
		return codeErr("av_audio_fifo_write", ret)
	}
	return nil
}

func (x *Resampler) GetBytesPerSample(sample_fmt unsafe.Pointer) int32 {
	return fAvGetBytesPerSample(sample_fmt)
}

func (x *Resampler) GetPackedSampleFmt(sample_fmt unsafe.Pointer) unsafe.Pointer {
	return fAvGetPackedSampleFmt(sample_fmt)
}

func (x *Resampler) GetPlanarSampleFmt(sample_fmt unsafe.Pointer) unsafe.Pointer {
	return fAvGetPlanarSampleFmt(sample_fmt)
}

func (x *Resampler) GetSampleFmt(name unsafe.Pointer) unsafe.Pointer {
	return fAvGetSampleFmt(name)
}

func (x *Resampler) GetSampleFmtName(sample_fmt unsafe.Pointer) unsafe.Pointer {
	return fAvGetSampleFmtName(sample_fmt)
}

func (x *Resampler) GetSampleFmtString(buf unsafe.Pointer, buf_size int32, sample_fmt unsafe.Pointer) unsafe.Pointer {
	return fAvGetSampleFmtString(buf, buf_size, sample_fmt)
}

func (x *Resampler) SampleFmtIsPlanar(sample_fmt unsafe.Pointer) int32 {
	return fAvSampleFmtIsPlanar(sample_fmt)
}

func (x *Resampler) Alloc2() unsafe.Pointer {
	return fSwrAlloc()
}

func (x *Resampler) AllocSetOpts2(ps *unsafe.Pointer, out_ch_layout unsafe.Pointer, out_sample_fmt unsafe.Pointer, out_sample_rate int32, in_ch_layout unsafe.Pointer, in_sample_fmt unsafe.Pointer, in_sample_rate int32, log_offset int32, log_ctx unsafe.Pointer) error {
	if ret := fSwrAllocSetOpts2(ps, out_ch_layout, out_sample_fmt, out_sample_rate, in_ch_layout, in_sample_fmt, in_sample_rate, log_offset, log_ctx); ret < 0 {
		return codeErr("swr_alloc_set_opts2", ret)
	}
	return nil
}

func (x *Resampler) BuildMatrix2(in_layout unsafe.Pointer, out_layout unsafe.Pointer, center_mix_level float64, surround_mix_level float64, lfe_mix_level float64, maxval float64, rematrix_volume float64, matrix unsafe.Pointer, stride unsafe.Pointer, matrix_encoding unsafe.Pointer, log_context unsafe.Pointer) error {
	if ret := fSwrBuildMatrix2(in_layout, out_layout, center_mix_level, surround_mix_level, lfe_mix_level, maxval, rematrix_volume, matrix, stride, matrix_encoding, log_context); ret < 0 {
		return codeErr("swr_build_matrix2", ret)
	}
	return nil
}

func (x *Resampler) Close() {
	if x == nil {
		return
	}
	fSwrClose(x.ptr)
}

func (x *Resampler) ConfigFrame(out unsafe.Pointer, in unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConfigFrame(x.ptr, out, in); ret < 0 {
		return codeErr("swr_config_frame", ret)
	}
	return nil
}

func (x *Resampler) Convert(out unsafe.Pointer, out_count int32, in unsafe.Pointer, in_count int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConvert(x.ptr, out, out_count, in, in_count); ret < 0 {
		return codeErr("swr_convert", ret)
	}
	return nil
}

func (x *Resampler) ConvertFrame(output unsafe.Pointer, input unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrConvertFrame(x.ptr, output, input); ret < 0 {
		return codeErr("swr_convert_frame", ret)
	}
	return nil
}

func (x *Resampler) DropOutput(count int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrDropOutput(x.ptr, count); ret < 0 {
		return codeErr("swr_drop_output", ret)
	}
	return nil
}

func (x *Resampler) Free(s *unsafe.Pointer) {
	fSwrFree(s)
}

func (x *Resampler) GetClass() unsafe.Pointer {
	return fSwrGetClass()
}

func (x *Resampler) GetDelay(base int64) int64 {
	if x == nil {
		return 0
	}
	return fSwrGetDelay(x.ptr, base)
}

func (x *Resampler) GetOutSamples(in_samples int32) int32 {
	if x == nil {
		return 0
	}
	return fSwrGetOutSamples(x.ptr, in_samples)
}

func (x *Resampler) Init() error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrInit(x.ptr); ret < 0 {
		return codeErr("swr_init", ret)
	}
	return nil
}

func (x *Resampler) InjectSilence(count int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrInjectSilence(x.ptr, count); ret < 0 {
		return codeErr("swr_inject_silence", ret)
	}
	return nil
}

func (x *Resampler) IsInitialized() int32 {
	if x == nil {
		return 0
	}
	return fSwrIsInitialized(x.ptr)
}

func (x *Resampler) NextPts(pts int64) int64 {
	if x == nil {
		return 0
	}
	return fSwrNextPts(x.ptr, pts)
}

func (x *Resampler) SetChannelMapping(channel_map unsafe.Pointer) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetChannelMapping(x.ptr, channel_map); ret < 0 {
		return codeErr("swr_set_channel_mapping", ret)
	}
	return nil
}

func (x *Resampler) SetCompensation(sample_delta int32, compensation_distance int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetCompensation(x.ptr, sample_delta, compensation_distance); ret < 0 {
		return codeErr("swr_set_compensation", ret)
	}
	return nil
}

func (x *Resampler) SetMatrix(matrix unsafe.Pointer, stride int32) error {
	if x == nil {
		return errNilFF
	}
	if ret := fSwrSetMatrix(x.ptr, matrix, stride); ret < 0 {
		return codeErr("swr_set_matrix", ret)
	}
	return nil
}
