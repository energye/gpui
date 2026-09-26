package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CodecEncode 编解码模块: AVCodecContext/AVCodec/AVCodecParameters/
// Parser/BSF 全量导出, 结构体方法直接可用.
//
// Say it plain: 先按编码名字找到解码器, 开个上下文, 把流参数灌进去,
// 打开后一包进一帧出 (发送包、收帧两步走, EAGAIN 说明要先收再送).
// 编码反过来一帧进一包出. 切包过滤 (比如 h264_mp4toannexb) 走 BSF.

// CodecContext owns one AVCodecContext* (nil-safe, 记得 FreeContext).
type CodecContext struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (c *CodecContext) Ptr() unsafe.Pointer {
	if c == nil {
		return nil
	}
	return c.ptr
}

// Codec owns one AVCodec* (跟着库走, 不释放).
type Codec struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (c *Codec) Ptr() unsafe.Pointer {
	if c == nil {
		return nil
	}
	return c.ptr
}

// CodecParameters owns one AVCodecParameters* (跟着流走或独立分配,
// 不单独释放; 独立分配的用 Free).
type CodecParameters struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (c *CodecParameters) Ptr() unsafe.Pointer {
	if c == nil {
		return nil
	}
	return c.ptr
}

// Parser owns one AVCodecParserContext* (记得 Close).
type Parser struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (p *Parser) Ptr() unsafe.Pointer {
	if p == nil {
		return nil
	}
	return p.ptr
}

// BitStreamFilter owns one AVBSFContext* (记得 Free).
type BitStreamFilter struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (b *BitStreamFilter) Ptr() unsafe.Pointer {
	if b == nil {
		return nil
	}
	return b.ptr
}

// 常用编码 id (codec_id.h 枚举值, 头文件钉死):
// NONE=0, MPEG1VIDEO=1, MPEG2VIDEO=2, MPEG4=12, H264=27, HEVC=173,
// VP9=167, AV1=225, MP2=86016, MP3=86017, AAC=86018, AC3=86019,
// DTS=86020, VORBIS=86021, Opus=86076, FLAC=86028.
const (
	CodecIDNone   int32 = 0
	CodecIDMPEG4  int32 = 12
	CodecIDH264   int32 = 27
	CodecIDHEVC   int32 = 173
	CodecIDH265   int32 = 173
	CodecIDVP9    int32 = 167
	CodecIDAV1    int32 = 225
	CodecIDMP3    int32 = 86017
	CodecIDAAC    int32 = 86018
	CodecIDAC3    int32 = 86019
	CodecIDVorbis int32 = 86021
	CodecIDOpus   int32 = 86076
	// 字幕编码 (codec_id.h: FIRST_SUBTITLE=0x17000=94208, MOV_TEXT 是第 6 个=94213).
	CodecIDMovText int32 = 94213
)

// 媒体类型 (avutil.h 枚举: UNKNOWN=-1, VIDEO=0, AUDIO=1, ...).
const (
	MediaTypeUnknown  int32 = -1
	MediaTypeVideo    int32 = 0
	MediaTypeAudio    int32 = 1
	MediaTypeData     int32 = 2
	MediaTypeSubtitle int32 = 3
)

var (
	fCodecAllocCtx                 func(unsafe.Pointer) unsafe.Pointer
	fCodecClose                    func(unsafe.Pointer) int32
	fCodecFindDecoder              func(int32) unsafe.Pointer
	fCodecFindDecByNam             func(string) unsafe.Pointer
	fCodecFindEncoder              func(int32) unsafe.Pointer
	fCodecFindEncByNam             func(string) unsafe.Pointer
	fCodecFlushBuf                 func(unsafe.Pointer)
	fCodecFreeCtx                  func(*unsafe.Pointer)
	fCodecGetName                  func(int32) string
	fCodecGetType                  func(int32) int32
	fCodecIsOpen                   func(unsafe.Pointer) int32
	fCodecOpen2                    func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) int32
	fCodecSendPacket               func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecRecvFrame                func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecSendFrame                func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecRecvPacket               func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecParAlloc                 func() unsafe.Pointer
	fCodecParFree                  func(*unsafe.Pointer)
	fCodecParCopy                  func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecParFromCtx               func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecParToCtx                 func(unsafe.Pointer, unsafe.Pointer) int32
	fCodecIsDecoder                func(unsafe.Pointer) int32
	fCodecIsEncoder                func(unsafe.Pointer) int32
	fCodecIterate                  func(*unsafe.Pointer) unsafe.Pointer
	fParserInit                    func(int32) unsafe.Pointer
	fParserParse2                  func(unsafe.Pointer, unsafe.Pointer, *unsafe.Pointer, *int32, unsafe.Pointer, int32, int64, int64, int64) int32
	fParserClose                   func(unsafe.Pointer)
	fBSFAlloc                      func(unsafe.Pointer, *unsafe.Pointer) int32
	fBSFInit                       func(unsafe.Pointer) int32
	fBSFSendPacket                 func(unsafe.Pointer, unsafe.Pointer) int32
	fBSFRecvPacket                 func(unsafe.Pointer, unsafe.Pointer) int32
	fBSFFlush                      func(unsafe.Pointer)
	fBSFFree                       func(*unsafe.Pointer)
	fBSFGetByName                  func(string) unsafe.Pointer
	fBSFListAlloc                  func() unsafe.Pointer
	fBSFListAppend                 func(unsafe.Pointer, unsafe.Pointer) int32
	fBSFListAppend2                func(unsafe.Pointer, string, *unsafe.Pointer) int32
	fBSFListFinalize               func(unsafe.Pointer, *unsafe.Pointer) int32
	fBSFListFree                   func(*unsafe.Pointer)
	fBSFListParseStr               func(string, *unsafe.Pointer) int32
	fAvBsfGetClass                 func() unsafe.Pointer
	fAvBsfGetNullFilter            func(bsf *unsafe.Pointer) int32
	fAvBsfIterate                  func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvcodecAlignDimensions        func(s unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer)
	fAvcodecAlignDimensions2       func(s unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer, arg3 unsafe.Pointer)
	fAvcodecConfiguration          func() unsafe.Pointer
	fAvcodecDctAlloc               func() unsafe.Pointer
	fAvcodecDctGetClass            func() unsafe.Pointer
	fAvcodecDctInit                func(arg0 unsafe.Pointer) int32
	fAvcodecDecodeSubtitle2        func(avctx unsafe.Pointer, sub unsafe.Pointer, got_sub_ptr unsafe.Pointer, avpkt unsafe.Pointer) int32
	fAvcodecDefaultExecute         func(c unsafe.Pointer, fn unsafe.Pointer, arg unsafe.Pointer, ret unsafe.Pointer, count int32, size int32) int32
	fAvcodecDefaultExecute2        func(c unsafe.Pointer, fn unsafe.Pointer, arg unsafe.Pointer, ret unsafe.Pointer, count int32) int32
	fAvcodecDefaultGetBuffer2      func(s unsafe.Pointer, frame unsafe.Pointer, flags int32) int32
	fAvcodecDefaultGetEncodeBuffer func(s unsafe.Pointer, pkt unsafe.Pointer, flags int32) int32
	fAvcodecDefaultGetFormat       func(s unsafe.Pointer, fmt unsafe.Pointer) unsafe.Pointer
	fAvcodecDescriptorGet          func(id unsafe.Pointer) unsafe.Pointer
	fAvcodecDescriptorGetByName    func(name unsafe.Pointer) unsafe.Pointer
	fAvcodecDescriptorNext         func(prev unsafe.Pointer) unsafe.Pointer
	fAvcodecEncodeSubtitle         func(avctx unsafe.Pointer, buf unsafe.Pointer, buf_size int32, sub unsafe.Pointer) int32
	fAvcodecFillAudioFrame         func(frame unsafe.Pointer, nb_channels int32, sample_fmt unsafe.Pointer, buf unsafe.Pointer, buf_size int32, align int32) unsafe.Pointer
	fAvcodecFindBestPixFmtOfList   func(pix_fmt_list unsafe.Pointer, src_pix_fmt int32, has_alpha int32, loss_ptr *int32) int32
	fAvcodecGetClass               func() unsafe.Pointer
	fAvcodecGetHwConfig            func(codec unsafe.Pointer, index int32) unsafe.Pointer
	fAvcodecGetHwFramesParameters  func(avctx unsafe.Pointer, device_ref unsafe.Pointer, hw_pix_fmt unsafe.Pointer, out_frames_ref *unsafe.Pointer) int32
	fAvCodecGetId                  func(tags unsafe.Pointer, tag uint32) unsafe.Pointer
	fAvcodecGetSubtitleRectClass   func() unsafe.Pointer
	fAvcodecGetSupportedConfig     func(avctx unsafe.Pointer, codec unsafe.Pointer, config unsafe.Pointer, flags uint32, out_configs *unsafe.Pointer, out_num_configs unsafe.Pointer) int32
	fAvCodecGetTag                 func(tags unsafe.Pointer, id unsafe.Pointer) uint32
	fAvCodecGetTag2                func(tags unsafe.Pointer, id unsafe.Pointer, tag unsafe.Pointer) int32
	fAvcodecLicense                func() unsafe.Pointer
	fAvcodecPixFmtToCodecTag       func(pix_fmt int32) uint32
	fAvcodecProfileName            func(codec_id unsafe.Pointer, profile int32) unsafe.Pointer
	fAvcodecString                 func(buf unsafe.Pointer, buf_size int32, enc unsafe.Pointer, encode int32)
	fAvcodecVersion                func() uint32
	fAvParserIterate               func(opaque *unsafe.Pointer) unsafe.Pointer
	fSubtitleFree                  func(sub unsafe.Pointer)
)

// ensureModCodecEncode 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modCodecEncodeOnce sync.Once

func ensureModCodecEncode() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modCodecEncodeOnce.Do(func() { registerCodecEncode(libHandle) })
	return nil
}

func registerCodecEncode(h uintptr) {
	purego.RegisterLibFunc(&fCodecAllocCtx, h, "avcodec_alloc_context3")
	purego.RegisterLibFunc(&fCodecClose, h, "avcodec_close")
	purego.RegisterLibFunc(&fCodecFindDecoder, h, "avcodec_find_decoder")
	purego.RegisterLibFunc(&fCodecFindDecByNam, h, "avcodec_find_decoder_by_name")
	purego.RegisterLibFunc(&fCodecFindEncoder, h, "avcodec_find_encoder")
	purego.RegisterLibFunc(&fCodecFindEncByNam, h, "avcodec_find_encoder_by_name")
	purego.RegisterLibFunc(&fCodecFlushBuf, h, "avcodec_flush_buffers")
	purego.RegisterLibFunc(&fCodecFreeCtx, h, "avcodec_free_context")
	purego.RegisterLibFunc(&fCodecGetName, h, "avcodec_get_name")
	purego.RegisterLibFunc(&fCodecGetType, h, "avcodec_get_type")
	purego.RegisterLibFunc(&fCodecIsOpen, h, "avcodec_is_open")
	purego.RegisterLibFunc(&fCodecOpen2, h, "avcodec_open2")
	purego.RegisterLibFunc(&fCodecSendPacket, h, "avcodec_send_packet")
	purego.RegisterLibFunc(&fCodecRecvFrame, h, "avcodec_receive_frame")
	purego.RegisterLibFunc(&fCodecSendFrame, h, "avcodec_send_frame")
	purego.RegisterLibFunc(&fCodecRecvPacket, h, "avcodec_receive_packet")
	purego.RegisterLibFunc(&fCodecParAlloc, h, "avcodec_parameters_alloc")
	purego.RegisterLibFunc(&fCodecParFree, h, "avcodec_parameters_free")
	purego.RegisterLibFunc(&fCodecParCopy, h, "avcodec_parameters_copy")
	purego.RegisterLibFunc(&fCodecParFromCtx, h, "avcodec_parameters_from_context")
	purego.RegisterLibFunc(&fCodecParToCtx, h, "avcodec_parameters_to_context")
	purego.RegisterLibFunc(&fCodecIsDecoder, h, "av_codec_is_decoder")
	purego.RegisterLibFunc(&fCodecIsEncoder, h, "av_codec_is_encoder")
	purego.RegisterLibFunc(&fCodecIterate, h, "av_codec_iterate")
	purego.RegisterLibFunc(&fParserInit, h, "av_parser_init")
	purego.RegisterLibFunc(&fParserParse2, h, "av_parser_parse2")
	purego.RegisterLibFunc(&fParserClose, h, "av_parser_close")
	purego.RegisterLibFunc(&fBSFAlloc, h, "av_bsf_alloc")
	purego.RegisterLibFunc(&fBSFInit, h, "av_bsf_init")
	purego.RegisterLibFunc(&fBSFSendPacket, h, "av_bsf_send_packet")
	purego.RegisterLibFunc(&fBSFRecvPacket, h, "av_bsf_receive_packet")
	purego.RegisterLibFunc(&fBSFFlush, h, "av_bsf_flush")
	purego.RegisterLibFunc(&fBSFFree, h, "av_bsf_free")
	purego.RegisterLibFunc(&fBSFGetByName, h, "av_bsf_get_by_name")
	purego.RegisterLibFunc(&fBSFListAlloc, h, "av_bsf_list_alloc")
	purego.RegisterLibFunc(&fBSFListAppend, h, "av_bsf_list_append")
	purego.RegisterLibFunc(&fBSFListAppend2, h, "av_bsf_list_append2")
	purego.RegisterLibFunc(&fBSFListFinalize, h, "av_bsf_list_finalize")
	purego.RegisterLibFunc(&fBSFListFree, h, "av_bsf_list_free")
	purego.RegisterLibFunc(&fBSFListParseStr, h, "av_bsf_list_parse_str")
	purego.RegisterLibFunc(&fAvBsfGetClass, h, "av_bsf_get_class")
	purego.RegisterLibFunc(&fAvBsfGetNullFilter, h, "av_bsf_get_null_filter")
	purego.RegisterLibFunc(&fAvBsfIterate, h, "av_bsf_iterate")
	purego.RegisterLibFunc(&fAvcodecAlignDimensions, h, "avcodec_align_dimensions")
	purego.RegisterLibFunc(&fAvcodecAlignDimensions2, h, "avcodec_align_dimensions2")
	purego.RegisterLibFunc(&fAvcodecConfiguration, h, "avcodec_configuration")
	purego.RegisterLibFunc(&fAvcodecDctAlloc, h, "avcodec_dct_alloc")
	purego.RegisterLibFunc(&fAvcodecDctGetClass, h, "avcodec_dct_get_class")
	purego.RegisterLibFunc(&fAvcodecDctInit, h, "avcodec_dct_init")
	purego.RegisterLibFunc(&fAvcodecDecodeSubtitle2, h, "avcodec_decode_subtitle2")
	purego.RegisterLibFunc(&fAvcodecDefaultExecute, h, "avcodec_default_execute")
	purego.RegisterLibFunc(&fAvcodecDefaultExecute2, h, "avcodec_default_execute2")
	purego.RegisterLibFunc(&fAvcodecDefaultGetBuffer2, h, "avcodec_default_get_buffer2")
	purego.RegisterLibFunc(&fAvcodecDefaultGetEncodeBuffer, h, "avcodec_default_get_encode_buffer")
	purego.RegisterLibFunc(&fAvcodecDefaultGetFormat, h, "avcodec_default_get_format")
	purego.RegisterLibFunc(&fAvcodecDescriptorGet, h, "avcodec_descriptor_get")
	purego.RegisterLibFunc(&fAvcodecDescriptorGetByName, h, "avcodec_descriptor_get_by_name")
	purego.RegisterLibFunc(&fAvcodecDescriptorNext, h, "avcodec_descriptor_next")
	purego.RegisterLibFunc(&fAvcodecEncodeSubtitle, h, "avcodec_encode_subtitle")
	purego.RegisterLibFunc(&fAvcodecFillAudioFrame, h, "avcodec_fill_audio_frame")
	purego.RegisterLibFunc(&fAvcodecFindBestPixFmtOfList, h, "avcodec_find_best_pix_fmt_of_list")
	purego.RegisterLibFunc(&fAvcodecGetClass, h, "avcodec_get_class")
	purego.RegisterLibFunc(&fAvcodecGetHwConfig, h, "avcodec_get_hw_config")
	purego.RegisterLibFunc(&fAvcodecGetHwFramesParameters, h, "avcodec_get_hw_frames_parameters")
	purego.RegisterLibFunc(&fAvCodecGetId, h, "av_codec_get_id")
	purego.RegisterLibFunc(&fAvcodecGetSubtitleRectClass, h, "avcodec_get_subtitle_rect_class")
	purego.RegisterLibFunc(&fAvcodecGetSupportedConfig, h, "avcodec_get_supported_config")
	purego.RegisterLibFunc(&fAvCodecGetTag, h, "av_codec_get_tag")
	purego.RegisterLibFunc(&fAvCodecGetTag2, h, "av_codec_get_tag2")
	purego.RegisterLibFunc(&fAvcodecLicense, h, "avcodec_license")
	purego.RegisterLibFunc(&fAvcodecPixFmtToCodecTag, h, "avcodec_pix_fmt_to_codec_tag")
	purego.RegisterLibFunc(&fAvcodecProfileName, h, "avcodec_profile_name")
	purego.RegisterLibFunc(&fAvcodecString, h, "avcodec_string")
	purego.RegisterLibFunc(&fAvcodecVersion, h, "avcodec_version")
	purego.RegisterLibFunc(&fAvParserIterate, h, "av_parser_iterate")
	purego.RegisterLibFunc(&fSubtitleFree, h, "avsubtitle_free")
}

// FindDecoder finds a decoder by codec id (nil when absent).
func FindDecoder(id int32) *Codec {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	ptr := fCodecFindDecoder(id)
	if ptr == nil {
		return nil
	}
	return &Codec{ptr: ptr}
}

// FindDecoderByName finds a decoder by name, e.g. "h264" (nil when absent).
func FindDecoderByName(name string) *Codec {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	ptr := fCodecFindDecByNam(name)
	if ptr == nil {
		return nil
	}
	return &Codec{ptr: ptr}
}

// FindEncoder finds an encoder by codec id (nil when absent).
func FindEncoder(id int32) *Codec {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	ptr := fCodecFindEncoder(id)
	if ptr == nil {
		return nil
	}
	return &Codec{ptr: ptr}
}

// IsDecoder reports the codec decodes.
func (c *Codec) IsDecoder() bool {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return false
	}
	return fCodecIsDecoder(c.ptr) > 0
}

// IsEncoder reports the codec encodes.
func (c *Codec) IsEncoder() bool {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return false
	}
	return fCodecIsEncoder(c.ptr) > 0
}

// AllocContext opens a context holder for codec (记得 FreeContext).
func (c *Codec) AllocContext() *CodecContext {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return nil
	}
	ptr := fCodecAllocCtx(c.ptr)
	if ptr == nil {
		return nil
	}
	return &CodecContext{ptr: ptr}
}

// FreeContext releases the context and nils the holder.
func (c *CodecContext) FreeContext() {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return
	}
	ptr := c.ptr
	c.ptr = nil
	fCodecFreeCtx(&ptr)
}

// Open opens the context (options 传 nil 用默认).
func (c *CodecContext) Open(codec *Codec, options unsafe.Pointer) error {
	mustUse(ensureModCodecEncode())
	if c == nil || codec == nil {
		return errNilCodec
	}
	if ret := fCodecOpen2(c.ptr, codec.ptr, options); ret < 0 {
		return codeErr("avcodec_open2", ret)
	}
	return nil
}

// IsOpen reports the context is opened.
func (c *CodecContext) IsOpen() bool {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return false
	}
	return fCodecIsOpen(c.ptr) > 0
}

// SendPacket feeds one packet (nil flushes; EAGAIN 先收一帧再送).
func (c *CodecContext) SendPacket(p *Packet) error {
	mustUse(ensureModCodecEncode())
	if c == nil {
		return errNilCodec
	}
	var pp unsafe.Pointer
	if p != nil {
		pp = p.ptr
	}
	if ret := fCodecSendPacket(c.ptr, pp); ret < 0 {
		return codeErr("avcodec_send_packet", ret)
	}
	return nil
}

// ReceiveFrame pulls one decoded frame (EAGAIN 说明要先送包).
func (c *CodecContext) ReceiveFrame(f *Frame) error {
	mustUse(ensureModCodecEncode())
	if c == nil || f == nil {
		return errNilCodec
	}
	if ret := fCodecRecvFrame(c.ptr, f.ptr); ret < 0 {
		return codeErr("avcodec_receive_frame", ret)
	}
	return nil
}

// SendFrame feeds one frame for encoding (nil flushes).
func (c *CodecContext) SendFrame(f *Frame) error {
	mustUse(ensureModCodecEncode())
	if c == nil {
		return errNilCodec
	}
	var fp unsafe.Pointer
	if f != nil {
		fp = f.ptr
	}
	if ret := fCodecSendFrame(c.ptr, fp); ret < 0 {
		return codeErr("avcodec_send_frame", ret)
	}
	return nil
}

// ReceivePacket pulls one encoded packet.
func (c *CodecContext) ReceivePacket(p *Packet) error {
	mustUse(ensureModCodecEncode())
	if c == nil || p == nil {
		return errNilCodec
	}
	if ret := fCodecRecvPacket(c.ptr, p.ptr); ret < 0 {
		return codeErr("avcodec_receive_packet", ret)
	}
	return nil
}

// FlushBuffers resets the codec (seek 后调, 丢掉内部缓存).
func (c *CodecContext) FlushBuffers() {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return
	}
	fCodecFlushBuf(c.ptr)
}

// NewCodecParameters allocates empty parameters (记得 Free).
func NewCodecParameters() *CodecParameters {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	ptr := fCodecParAlloc()
	if ptr == nil {
		return nil
	}
	return &CodecParameters{ptr: ptr}
}

// Free releases the parameters holder.
func (p *CodecParameters) Free() {
	mustUse(ensureModCodecEncode())
	if p == nil || p.ptr == nil {
		return
	}
	ptr := p.ptr
	p.ptr = nil
	fCodecParFree(&ptr)
}

// Copy duplicates src parameters.
func (p *CodecParameters) Copy(src *CodecParameters) error {
	mustUse(ensureModCodecEncode())
	if p == nil || src == nil {
		return errNilCodec
	}
	if ret := fCodecParCopy(p.ptr, src.ptr); ret < 0 {
		return codeErr("avcodec_parameters_copy", ret)
	}
	return nil
}

// FromContext fills parameters from an opened context.
func (p *CodecParameters) FromContext(c *CodecContext) error {
	mustUse(ensureModCodecEncode())
	if p == nil || c == nil {
		return errNilCodec
	}
	if ret := fCodecParFromCtx(p.ptr, c.ptr); ret < 0 {
		return codeErr("avcodec_parameters_from_context", ret)
	}
	return nil
}

// ToContext fills a fresh context from parameters (open 前调).
func (p *CodecParameters) ToContext(c *CodecContext) error {
	mustUse(ensureModCodecEncode())
	if p == nil || c == nil {
		return errNilCodec
	}
	if ret := fCodecParToCtx(c.ptr, p.ptr); ret < 0 {
		return codeErr("avcodec_parameters_to_context", ret)
	}
	return nil
}

// CodecName returns the short name, e.g. "h264".
func CodecName(id int32) string {
	if ensureModCodecEncode() != nil {
		return ""
	}
	return fCodecGetName(id)
}

// CodecType returns the media type of a codec id.
func CodecType(id int32) int32 {
	if ensureModCodecEncode() != nil {
		return MediaTypeUnknown
	}
	return fCodecGetType(id)
}

// NewParser opens a parser for codec id (记得 Close; 如 H264 喂裸流切帧).
func NewParser(id int32) *Parser {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	ptr := fParserInit(id)
	if ptr == nil {
		return nil
	}
	return &Parser{ptr: ptr}
}

// Close releases the parser.
func (p *Parser) Close() {
	mustUse(ensureModCodecEncode())
	if p == nil || p.ptr == nil {
		return
	}
	fParserClose(p.ptr)
	p.ptr = nil
}

// Parse2 splits raw bytes into frames (av_parser_parse2; CodecContext
// 传 nil 只切分不解码, poutbuf/outSize 由包内返回, pts/dts/pos 照抄包).
func (p *Parser) Parse2(codecCtx unsafe.Pointer, outBuf *unsafe.Pointer, outSize *int32, buf unsafe.Pointer, bufSize int32, pts, dts, pos int64) int32 {
	mustUse(ensureModCodecEncode())
	if p == nil || p.ptr == nil {
		return AvErrorEAGAIN
	}
	return fParserParse2(p.ptr, codecCtx, outBuf, outSize, buf, bufSize, pts, dts, pos)
}

// NewBitStreamFilter allocates a filter by name,
// e.g. "h264_mp4toannexb" (记得 Free, 用前 Init + CopyParameters).
func NewBitStreamFilter(name string) *BitStreamFilter {
	if err := ensureModCodecEncode(); err != nil {
		return nil
	}
	filter := fBSFGetByName(name)
	if filter == nil {
		return nil
	}
	var ctx unsafe.Pointer
	if ret := fBSFAlloc(filter, &ctx); ret < 0 || ctx == nil {
		return nil
	}
	return &BitStreamFilter{ptr: ctx}
}

// Init initializes the filter (参数配好后调).
func (b *BitStreamFilter) Init() error {
	mustUse(ensureModCodecEncode())
	if b == nil {
		return errNilCodec
	}
	if ret := fBSFInit(b.ptr); ret < 0 {
		return codeErr("av_bsf_init", ret)
	}
	return nil
}

// SendPacket feeds one packet into the filter.
func (b *BitStreamFilter) SendPacket(p *Packet) error {
	mustUse(ensureModCodecEncode())
	if b == nil || p == nil {
		return errNilCodec
	}
	if ret := fBSFSendPacket(b.ptr, p.ptr); ret < 0 {
		return codeErr("av_bsf_send_packet", ret)
	}
	return nil
}

// ReceivePacket pulls one filtered packet.
func (b *BitStreamFilter) ReceivePacket(p *Packet) error {
	mustUse(ensureModCodecEncode())
	if b == nil || p == nil {
		return errNilCodec
	}
	if ret := fBSFRecvPacket(b.ptr, p.ptr); ret < 0 {
		return codeErr("av_bsf_receive_packet", ret)
	}
	return nil
}

// Flush resets the filter.
func (b *BitStreamFilter) Flush() {
	mustUse(ensureModCodecEncode())
	if b == nil || b.ptr == nil {
		return
	}
	fBSFFlush(b.ptr)
}

// Free releases the filter.
func (b *BitStreamFilter) Free() {
	mustUse(ensureModCodecEncode())
	if b == nil || b.ptr == nil {
		return
	}
	ptr := b.ptr
	b.ptr = nil
	fBSFFree(&ptr)
}

// BitStreamFilterList chains several filters ("h264_mp4toannexb",
// "null" 等逗号串一次配好): Alloc 建空链, Append/AppendByName 逐个加,
// Finalize 封口吐单个可用 filter, Free 丢弃整链.
type BitStreamFilterList struct{ ptr unsafe.Pointer }

// AllocBSFList allocates an empty filter chain (记得 Free/Finalize).
func AllocBSFList() *BitStreamFilterList {
	if ensureModCodecEncode() != nil {
		return nil
	}
	ptr := fBSFListAlloc()
	if ptr == nil {
		return nil
	}
	return &BitStreamFilterList{ptr: ptr}
}

// Append adds an open filter context to the chain.
func (l *BitStreamFilterList) Append(bsf *BitStreamFilter) error {
	mustUse(ensureModCodecEncode())
	if l == nil || l.ptr == nil || bsf == nil {
		return errNilCodec
	}
	if ret := fBSFListAppend(l.ptr, bsf.ptr); ret < 0 {
		return codeErr("av_bsf_list_append", ret)
	}
	return nil
}

// AppendByName adds a filter by name with options (options 传 nil 用默认).
func (l *BitStreamFilterList) AppendByName(name string, options *unsafe.Pointer) error {
	mustUse(ensureModCodecEncode())
	if l == nil || l.ptr == nil {
		return errNilCodec
	}
	if ret := fBSFListAppend2(l.ptr, name, options); ret < 0 {
		return codeErr("av_bsf_list_append2", ret)
	}
	return nil
}

// Finalize seals the chain into one usable filter (链本身被吃掉, 别再 Free).
func (l *BitStreamFilterList) Finalize(out **BitStreamFilter) error {
	mustUse(ensureModCodecEncode())
	var ctx unsafe.Pointer
	if l == nil || l.ptr == nil || out == nil {
		return errNilCodec
	}
	lst := l.ptr
	if ret := fBSFListFinalize(unsafe.Pointer(&lst), &ctx); ret < 0 {
		return codeErr("av_bsf_list_finalize", ret)
	}
	l.ptr = nil
	if ctx == nil {
		return errNilCodec
	}
	*out = &BitStreamFilter{ptr: ctx}
	return nil
}

// Free drops the whole chain.
func (l *BitStreamFilterList) Free() {
	mustUse(ensureModCodecEncode())
	if l == nil || l.ptr == nil {
		return
	}
	lst := l.ptr
	l.ptr = nil
	fBSFListFree(&lst)
}

// ParseBSFList parses "filter1,filter2" into one usable filter
// (av_bsf_list_parse_str; 逗号串转单个 filter, 记得 Free).
func ParseBSFList(s string) (*BitStreamFilter, error) {
	if ensureModCodecEncode() != nil {
		return nil, errNilCodec
	}
	var ctx unsafe.Pointer
	if ret := fBSFListParseStr(s, &ctx); ret < 0 {
		return nil, codeErr("av_bsf_list_parse_str", ret)
	}
	if ctx == nil {
		return nil, errNilCodec
	}
	return &BitStreamFilter{ptr: ctx}, nil
}

// BsfGetClass 取码流过滤器的选项类（对 av_bsf_get_class；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (self *BitStreamFilter) BsfGetClass() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvBsfGetClass()
}

// BsfGetNullFilter 取空过滤器（直通）（对 av_bsf_get_null_filter；参数 bsf；回数值或个数；nil 接收器直接回零值，不崩）。
func (self *BitStreamFilter) BsfGetNullFilter(bsf *unsafe.Pointer) int32 {
	mustUse(ensureModCodecEncode())
	return fAvBsfGetNullFilter(bsf)
}

// BsfIterate 逐个列出码流过滤器（对 av_bsf_iterate；参数 opaque；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (self *BitStreamFilter) BsfIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	// NOTE: opaque 传 nil 会撞空指针 — 用 IterateBSFFilters.
	if opaque == nil {
		return nil
	}
	return fAvBsfIterate(opaque)
}

// IterateBSFFilters walks every compiled bitstream filter.
func IterateBSFFilters() []unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	var opaque unsafe.Pointer
	var out []unsafe.Pointer
	for {
		ptr := fAvBsfIterate(&opaque)
		if ptr == nil {
			break
		}
		out = append(out, ptr)
		if len(out) > 4096 {
			break
		}
	}
	return out
}

// AvcodecAlignDimensions 按编码要求对齐宽高（对 avcodec_align_dimensions；参数 s、width、height；按签名取回值；无状态调用）。
func (self *Codec) AvcodecAlignDimensions(s unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer) {
	mustUse(ensureModCodecEncode())
	fAvcodecAlignDimensions(s, width, height)
}

// AvcodecAlignDimensions2 按编码要求对齐宽高（对 avcodec_align_dimensions2；参数 s、width、height、arg3；按签名取回值；无状态调用）。
func (self *Codec) AvcodecAlignDimensions2(s unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer, arg3 unsafe.Pointer) {
	mustUse(ensureModCodecEncode())
	fAvcodecAlignDimensions2(s, width, height, arg3)
}

// AvcodecConfiguration 问编解码库编译配置（对 avcodec_configuration；无参数；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecConfiguration() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecConfiguration()
}

// AvcodecDctAlloc 新建 DCT 上下文（对 avcodec_dct_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Codec) AvcodecDctAlloc() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDctAlloc()
}

// AvcodecDctGetClass 取 DCT 的选项类（对 avcodec_dct_get_class；无参数；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecDctGetClass() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDctGetClass()
}

// AvcodecDctInit 初始化 DCT 上下文（对 avcodec_dct_init；参数 arg0；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Codec) AvcodecDctInit(arg0 unsafe.Pointer) error {
	mustUse(ensureModCodecEncode())
	if ret := fAvcodecDctInit(arg0); ret < 0 {
		return codeErr("avcodec_dct_init", ret)
	}
	return nil
}

// AvcodecDecodeSubtitle2 解一包字幕（对 avcodec_decode_subtitle2；参数 avctx、sub、got_sub_ptr、avpkt；成功回 nil，失败回 error（字串已是人话）；nil 接收器直接回零值，不崩）。
func (self *Codec) AvcodecDecodeSubtitle2(avctx unsafe.Pointer, sub unsafe.Pointer, got_sub_ptr unsafe.Pointer, avpkt unsafe.Pointer) error {
	mustUse(ensureModCodecEncode())
	if ret := fAvcodecDecodeSubtitle2(avctx, sub, got_sub_ptr, avpkt); ret < 0 {
		return codeErr("avcodec_decode_subtitle2", ret)
	}
	return nil
}

// SubtitleFree frees all allocated data in a decoded AVSubtitle struct
// (pair with AvcodecDecodeSubtitle2 when got_sub is set).
func (self *Codec) SubtitleFree(sub unsafe.Pointer) {
	mustUse(ensureModCodecEncode())
	if sub == nil {
		return
	}
	fSubtitleFree(sub)
}

// AvcodecDefaultExecute 默认多线程执行一批任务（对 avcodec_default_execute；参数 c、fn、arg、ret、count、size；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Codec) AvcodecDefaultExecute(c unsafe.Pointer, fn unsafe.Pointer, arg unsafe.Pointer, ret unsafe.Pointer, count int32, size int32) error {
	mustUse(ensureModCodecEncode())
	if ret := fAvcodecDefaultExecute(c, fn, arg, ret, count, size); ret < 0 {
		return codeErr("avcodec_default_execute", ret)
	}
	return nil
}

// AvcodecDefaultExecute2 默认多线程执行一批任务（新版）（对 avcodec_default_execute2；参数 c、fn、arg、ret、count；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Codec) AvcodecDefaultExecute2(c unsafe.Pointer, fn unsafe.Pointer, arg unsafe.Pointer, ret unsafe.Pointer, count int32) error {
	mustUse(ensureModCodecEncode())
	if ret := fAvcodecDefaultExecute2(c, fn, arg, ret, count); ret < 0 {
		return codeErr("avcodec_default_execute2", ret)
	}
	return nil
}

// AvcodecDefaultGetBuffer2 默认申请解码帧缓冲（对 avcodec_default_get_buffer2；参数 s、frame、flags；回数值或个数；无状态调用）。
func (self *Codec) AvcodecDefaultGetBuffer2(s unsafe.Pointer, frame unsafe.Pointer, flags int32) int32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecDefaultGetBuffer2(s, frame, flags)
}

// AvcodecDefaultGetEncodeBuffer 默认申请编码包缓冲（对 avcodec_default_get_encode_buffer；参数 s、pkt、flags；回数值或个数；无状态调用）。
func (self *Codec) AvcodecDefaultGetEncodeBuffer(s unsafe.Pointer, pkt unsafe.Pointer, flags int32) int32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecDefaultGetEncodeBuffer(s, pkt, flags)
}

// AvcodecDefaultGetFormat 默认协商像素格式（对 avcodec_default_get_format；参数 s、fmt；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecDefaultGetFormat(s unsafe.Pointer, fmt unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDefaultGetFormat(s, fmt)
}

// AvcodecDescriptorGet 查编码描述（对 avcodec_descriptor_get；参数 id；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecDescriptorGet(id unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDescriptorGet(id)
}

// AvcodecDescriptorGetByName 查编码描述（对 avcodec_descriptor_get_by_name；参数 name；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Codec) AvcodecDescriptorGetByName(name unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDescriptorGetByName(name)
}

// AvcodecDescriptorNext 查编码描述（对 avcodec_descriptor_next；参数 prev；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecDescriptorNext(prev unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecDescriptorNext(prev)
}

// AvcodecEncodeSubtitle 编一帧字幕（对 avcodec_encode_subtitle；参数 avctx、buf、buf_size、sub；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Codec) AvcodecEncodeSubtitle(avctx unsafe.Pointer, buf unsafe.Pointer, buf_size int32, sub unsafe.Pointer) error {
	mustUse(ensureModCodecEncode())
	if ret := fAvcodecEncodeSubtitle(avctx, buf, buf_size, sub); ret < 0 {
		return codeErr("avcodec_encode_subtitle", ret)
	}
	return nil
}

// AvcodecFillAudioFrame 给音频帧填缓冲（对 avcodec_fill_audio_frame；参数 frame、nb_channels、sample_fmt、buf、buf_size、align；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecFillAudioFrame(frame unsafe.Pointer, nb_channels int32, sample_fmt unsafe.Pointer, buf unsafe.Pointer, buf_size int32, align int32) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecFillAudioFrame(frame, nb_channels, sample_fmt, buf, buf_size, align)
}

// AvcodecFindBestPixFmtOfList 在列表里挑损失最小的像素格式（对 avcodec_find_best_pix_fmt_of_list；参数 pix_fmt_list、src_pix_fmt、has_alpha、loss_ptr；回数值或个数；无状态调用）。
func (self *Codec) AvcodecFindBestPixFmtOfList(pix_fmt_list unsafe.Pointer, src_pix_fmt int32, has_alpha int32, loss_ptr *int32) int32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecFindBestPixFmtOfList(pix_fmt_list, src_pix_fmt, has_alpha, loss_ptr)
}

// AvcodecGetClass 取编解码上下文的选项类（对 avcodec_get_class；无参数；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecGetClass() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecGetClass()
}

// AvcodecGetHwConfig 查编码器的硬解配置（对 avcodec_get_hw_config；参数 codec、index；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecGetHwConfig(codec unsafe.Pointer, index int32) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecGetHwConfig(codec, index)
}

// AvcodecGetHwFramesParameters 取硬解帧参数（对 avcodec_get_hw_frames_parameters；参数 avctx、device_ref、hw_pix_fmt、out_frames_ref；回数值或个数；无状态调用）。
func (self *Codec) AvcodecGetHwFramesParameters(avctx unsafe.Pointer, device_ref unsafe.Pointer, hw_pix_fmt unsafe.Pointer, out_frames_ref *unsafe.Pointer) int32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecGetHwFramesParameters(avctx, device_ref, hw_pix_fmt, out_frames_ref)
}

// CodecGetId 按标签查编码编号（对 av_codec_get_id；参数 tags、tag；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) CodecGetId(tags unsafe.Pointer, tag uint32) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvCodecGetId(tags, tag)
}

// AvcodecGetSubtitleRectClass 取字幕矩形的选项类（对 avcodec_get_subtitle_rect_class；无参数；回 C 指针，失败回 nil；无状态调用）。
func (self *Codec) AvcodecGetSubtitleRectClass() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecGetSubtitleRectClass()
}

// AvcodecGetSupportedConfig 查编码器支持的配置（对 avcodec_get_supported_config；参数 avctx、codec、config、flags、out_configs、out_num_configs；回数值或个数；无状态调用）。
func (self *Codec) AvcodecGetSupportedConfig(avctx unsafe.Pointer, codec unsafe.Pointer, config unsafe.Pointer, flags uint32, out_configs *unsafe.Pointer, out_num_configs unsafe.Pointer) int32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecGetSupportedConfig(avctx, codec, config, flags, out_configs, out_num_configs)
}

// CodecGetTag 按编码查标签（对 av_codec_get_tag；参数 tags、id；回数值或个数；无状态调用）。
func (self *Codec) CodecGetTag(tags unsafe.Pointer, id unsafe.Pointer) uint32 {
	mustUse(ensureModCodecEncode())
	return fAvCodecGetTag(tags, id)
}

// CodecGetTag2 按编码查标签（对 av_codec_get_tag2；参数 tags、id、tag；回数值或个数；无状态调用）。
func (self *Codec) CodecGetTag2(tags unsafe.Pointer, id unsafe.Pointer, tag unsafe.Pointer) int32 {
	mustUse(ensureModCodecEncode())
	return fAvCodecGetTag2(tags, id, tag)
}

// AvcodecLicense 问编解码库许可证（对 avcodec_license；无参数；回 C 指针，失败回 nil；nil 接收器直接回零值，不崩）。
func (self *Codec) AvcodecLicense() unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecLicense()
}

// AvcodecPixFmtToCodecTag 按像素格式查编码标签（对 avcodec_pix_fmt_to_codec_tag；参数 pix_fmt；回数值或个数；nil 接收器直接回零值，不崩）。
func (self *Codec) AvcodecPixFmtToCodecTag(pix_fmt int32) uint32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecPixFmtToCodecTag(pix_fmt)
}

// Close releases a codec context opened by Open (avcodec_close;
// nil-safe, idempotent — matches FreeContext's guard shape).
func (c *CodecContext) Close() {
	mustUse(ensureModCodecEncode())
	if c == nil || c.ptr == nil {
		return
	}
	fCodecClose(c.ptr)
	c.ptr = nil
}

// FindEncoderByName looks up an encoder by name, e.g. "aac"
// (avcodec_find_encoder_by_name; nil when absent, no error signal).
func FindEncoderByName(name string) *Codec {
	if ensureModCodecEncode() != nil {
		return nil
	}
	ptr := fCodecFindEncByNam(name)
	if ptr == nil {
		return nil
	}
	return &Codec{ptr: ptr}
}

// Iterate walks every compiled codec (av_codec_iterate; opaque 传
// nil 开头复位, 之后每次把上次的 opaque 传回直到回 nil).
// NOTE: opaque 必须是有效指针槽, 传 Go nil 会撞空指针 — 用 IterateAll.
func (self *Codec) Iterate(opaque *unsafe.Pointer) *Codec {
	mustUse(ensureModCodecEncode())
	if opaque == nil {
		return nil
	}
	ptr := fCodecIterate(opaque)
	if ptr == nil {
		return nil
	}
	return &Codec{ptr: ptr}
}

// IterateAll walks every compiled codec and returns them
// (av_codec_iterate 的 Go 友好版, opaque 槽包内管).
func IterateAll() []*Codec {
	mustUse(ensureModCodecEncode())
	var opaque unsafe.Pointer
	var out []*Codec
	for {
		ptr := fCodecIterate(&opaque)
		if ptr == nil {
			break
		}
		out = append(out, &Codec{ptr: ptr})
		if len(out) > 4096 {
			break
		}
	}
	return out
}

// AvcodecProfileName 查档次名字（对 avcodec_profile_name；参数 codec_id、profile；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Codec) AvcodecProfileName(codec_id unsafe.Pointer, profile int32) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	return fAvcodecProfileName(codec_id, profile)
}

// AvcodecString 把编解码信息拼成人话（对 avcodec_string；参数 buf、buf_size、enc、encode；按签名取回值；nil 接收器直接回零值，不崩）。
func (self *Codec) AvcodecString(buf unsafe.Pointer, buf_size int32, enc unsafe.Pointer, encode int32) {
	mustUse(ensureModCodecEncode())
	fAvcodecString(buf, buf_size, enc, encode)
}

// AvcodecVersion 问编解码库版本号（对 avcodec_version；无参数；回数值或个数；nil 接收器直接回零值，不崩）。
func (self *Codec) AvcodecVersion() uint32 {
	mustUse(ensureModCodecEncode())
	return fAvcodecVersion()
}

// ParserIterate 逐个列出切帧器（对 av_parser_iterate；参数 opaque；成功回 C 指针，失败回 nil；新建的记得调对应 Free；nil 接收器直接回零值，不崩）。
func (self *Parser) ParserIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	// NOTE: opaque 传 nil 会撞空指针 — 用 IterateParsers.
	if opaque == nil {
		return nil
	}
	return fAvParserIterate(opaque)
}

// IterateParsers walks every compiled parser.
func IterateParsers() []unsafe.Pointer {
	mustUse(ensureModCodecEncode())
	var opaque unsafe.Pointer
	var out []unsafe.Pointer
	for {
		ptr := fAvParserIterate(&opaque)
		if ptr == nil {
			break
		}
		out = append(out, ptr)
		if len(out) > 4096 {
			break
		}
	}
	return out
}
