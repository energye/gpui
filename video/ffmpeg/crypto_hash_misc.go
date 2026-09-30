package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// CryptoHashMisc 加解密哈希杂项模块: HWDevice/Crypto/Muxer/Prober/Samples/Util 全量导出.
//
// Say it plain: 硬解上下文、加解密哈希、写文件、猜格式、采样本，全是按需直调的小件.
// HWDevice owns one AVHWDeviceContext* (硬解设备, 记得 Unref).
type HWDevice struct{ ptr unsafe.Pointer }

func (self *HWDevice) Ptr() unsafe.Pointer {
	if self == nil {
		return nil
	}
	return self.ptr
}

// Crypto owns one digest/cipher context (记得对应 Free).
type Crypto struct{ ptr unsafe.Pointer }

func (self *Crypto) Ptr() unsafe.Pointer {
	mustUse(ensureModCryptoHw())
	if self == nil {
		return nil
	}
	return self.ptr
}

// Muxer holder for output write calls (无状态, 挂 FormatContext 上也行, 这里直调).
type Muxer struct{}

// Prober holder for probe/guess calls (无状态).
type Prober struct{}

// Samples holder for sample-buffer helpers (无状态).
type Samples struct{}

// Util holder for misc queries (无状态).
type Util struct{}

var (
	fAvAc3ParseHeader                         func(buf unsafe.Pointer, size uintptr, bitstream_id unsafe.Pointer, frame_size unsafe.Pointer) unsafe.Pointer
	fAvAdtsHeaderParse                        func(buf unsafe.Pointer, samples unsafe.Pointer, frames unsafe.Pointer) unsafe.Pointer
	fAvAesAlloc                               func() unsafe.Pointer
	fAvAesCrypt                               func(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvAesCtrAlloc                            func() unsafe.Pointer
	fAvAesCtrCrypt                            func(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, size int32)
	fAvAesCtrFree                             func(a unsafe.Pointer)
	fAvAesCtrGetIv                            func(a unsafe.Pointer) unsafe.Pointer
	fAvAesCtrIncrementIv                      func(a unsafe.Pointer)
	fAvAesCtrInit                             func(a unsafe.Pointer, key unsafe.Pointer) int32
	fAvAesCtrSetFullIv                        func(a unsafe.Pointer, iv unsafe.Pointer)
	fAvAesCtrSetIv                            func(a unsafe.Pointer, iv unsafe.Pointer)
	fAvAesCtrSetRandomIv                      func(a unsafe.Pointer)
	fAvAesInit                                func(a unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) int32
	fAvAllocVdpaucontext                      func() unsafe.Pointer
	fAvAppendPacket                           func(s unsafe.Pointer, pkt unsafe.Pointer, size int32) int32
	fAvAppendPathComponent                    func(path unsafe.Pointer, component unsafe.Pointer) unsafe.Pointer
	fAvAssert0Fpu                             func()
	fAvBase64Decode                           func(out unsafe.Pointer, in unsafe.Pointer, out_size int32) int32
	fAvBase64Encode                           func(out unsafe.Pointer, out_size int32, in unsafe.Pointer, in_size int32) unsafe.Pointer
	fAvBasename                               func(path unsafe.Pointer) unsafe.Pointer
	fAvBesselI0                               func(x float64) float64
	fAvBlowfishAlloc                          func() unsafe.Pointer
	fAvBlowfishCrypt                          func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvBlowfishCryptEcb                       func(ctx unsafe.Pointer, xl unsafe.Pointer, xr unsafe.Pointer, decrypt int32)
	fAvBlowfishInit                           func(ctx unsafe.Pointer, key unsafe.Pointer, key_len int32)
	fAvBmgGet                                 func(lfg unsafe.Pointer, out float64)
	fAvBprintf                                func(buf unsafe.Pointer, fmt unsafe.Pointer)
	fAvCalloc                                 func(nmemb uintptr, size uintptr) unsafe.Pointer
	fAvCamelliaAlloc                          func() unsafe.Pointer
	fAvCamelliaCrypt                          func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvCamelliaInit                           func(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) int32
	fAvCast5Alloc                             func() unsafe.Pointer
	fAvCast5Crypt                             func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, decrypt int32)
	fAvCast5Crypt2                            func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvAdler32Update                          func(adler uint32, buf unsafe.Pointer, ln uintptr) uint32
	fAvCast5Init                              func(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) int32
	fAvChromaLocationEnumToPos                func(xpos *int32, ypos *int32, pos int32) int32
	fAvChromaLocationFromName                 func(name string) int32
	fAvChromaLocationName                     func(location int32) unsafe.Pointer
	fAvChromaLocationPosToEnum                func(xpos int32, ypos int32) int32
	fAvCmpI                                   func(a AVInteger, b AVInteger) int32
	fAvCpbPropertiesAlloc                     func(size unsafe.Pointer) unsafe.Pointer
	fAvCrcGetTable                            func(crc_id int32) unsafe.Pointer
	fAvCrc                                    func(ctx unsafe.Pointer, crc uint32, buf unsafe.Pointer, ln uintptr) uint32
	fAvCrcInit                                func(ctx unsafe.Pointer, le int32, bits int32, poly uint32, ctx_size int32) int32
	fAvCspApproximateTrcGamma                 func(trc int32) float64
	fAvCspLumaCoeffsFromAvcsp                 func(csp int32) unsafe.Pointer
	fAvCspPrimariesDescFromId                 func(prm int32) unsafe.Pointer
	fAvCspPrimariesIdFromDesc                 func(prm unsafe.Pointer) int32
	fAvCspTrcFuncFromId                       func(trc int32) unsafe.Pointer
	fAvD2q                                    func(d float64, max int32) AVRational
	fAvD3d11vaAllocContext                    func() unsafe.Pointer
	fAvDctCalc                                func(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer
	fAvDctEnd                                 func(s unsafe.Pointer) unsafe.Pointer
	fAvDctInit                                func(nbits int32, typ unsafe.Pointer) unsafe.Pointer
	fAvDefaultGetCategory                     func(ptr unsafe.Pointer) unsafe.Pointer
	fAvDefaultItemName                        func(ctx unsafe.Pointer) unsafe.Pointer
	fAvDemuxerIterate                         func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvDesAlloc                               func() unsafe.Pointer
	fAvDesCrypt                               func(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvDesInit                                func(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) int32
	fAvDesMac                                 func(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32)
	fAvDetectionBboxAlloc                     func(nb_bboxes uint32, out_size unsafe.Pointer) unsafe.Pointer
	fAvDetectionBboxCreateSideData            func(frame unsafe.Pointer, nb_bboxes uint32) unsafe.Pointer
	fAvDiracParseSequenceHeader               func(dsh *unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr, log_ctx unsafe.Pointer) int32
	fAvDirname                                func(path unsafe.Pointer) unsafe.Pointer
	fAvDispositionFromString                  func(disp string) int32
	fAvDispositionToString                    func(disposition int32) unsafe.Pointer
	fAvDivI                                   func(a AVInteger, b AVInteger) AVInteger
	fAvDivQ                                   func(b AVRational, c AVRational) AVRational
	fAvDownmixInfoUpdateSideData              func(frame unsafe.Pointer) unsafe.Pointer
	fAvDvCodecProfile                         func(width int32, height int32, pix_fmt int32) unsafe.Pointer
	fAvDvCodecProfile2                        func(width int32, height int32, pix_fmt int32, frame_rate AVRational) unsafe.Pointer
	fAvDvFrameProfile                         func(sys unsafe.Pointer, frame unsafe.Pointer, buf_size uint32) unsafe.Pointer
	fAvDynarray2Add                           func(tab_ptr *unsafe.Pointer, nb_ptr unsafe.Pointer, elem_size uintptr, elem_data unsafe.Pointer) unsafe.Pointer
	fAvDynarrayAdd                            func(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer)
	fAvDynarrayAddNofree                      func(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer) int32
	fAvEncryptionInfoAddSideData              func(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer
	fAvEncryptionInfoAlloc                    func(subsample_count uint32, key_id_size uint32, iv_size uint32) unsafe.Pointer
	fAvEncryptionInfoClone                    func(info unsafe.Pointer) unsafe.Pointer
	fAvEncryptionInfoFree                     func(info unsafe.Pointer)
	fAvEncryptionInfoGetSideData              func(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer
	fAvEncryptionInitInfoAddSideData          func(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer
	fAvEncryptionInitInfoAlloc                func(system_id_size uint32, num_key_ids uint32, key_id_size uint32, data_size uint32) unsafe.Pointer
	fAvEncryptionInitInfoFree                 func(info unsafe.Pointer)
	fAvEncryptionInitInfoGetSideData          func(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer
	fAvEscape                                 func(dst *unsafe.Pointer, src unsafe.Pointer, special_chars unsafe.Pointer, mode unsafe.Pointer, flags int32) unsafe.Pointer
	fAvExecutorAlloc                          func(callbacks unsafe.Pointer, thread_count int32) unsafe.Pointer
	fAvExecutorExecute                        func(e unsafe.Pointer, t unsafe.Pointer)
	fAvExecutorFree                           func(e *unsafe.Pointer)
	fAvExprCountFunc                          func(e unsafe.Pointer, counter unsafe.Pointer, size int32, arg int32) int32
	fAvExprCountVars                          func(e unsafe.Pointer, counter unsafe.Pointer, size int32) int32
	fAvExprEval                               func(e unsafe.Pointer, const_values unsafe.Pointer, opaque unsafe.Pointer) float64
	fAvExprFree                               func(e unsafe.Pointer)
	fAvExprParse                              func(expr *unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, func1_names unsafe.Pointer, cb4 unsafe.Pointer, func2_names unsafe.Pointer, cb6 unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) int32
	fAvExprParseAndEval                       func(res unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, const_values unsafe.Pointer, func1_names unsafe.Pointer, cb5 unsafe.Pointer, func2_names unsafe.Pointer, cb7 unsafe.Pointer, opaque unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) int32
	fAvFastMalloc                             func(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr)
	fAvFastMallocz                            func(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr)
	fAvFastPaddedMalloc                       func(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr)
	fAvFastPaddedMallocz                      func(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr)
	fAvFastRealloc                            func(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) unsafe.Pointer
	fAvFftCalc                                func(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer
	fAvFftEnd                                 func(s unsafe.Pointer) unsafe.Pointer
	fAvFftInit                                func(nbits int32, inverse int32) unsafe.Pointer
	fAvFftPermute                             func(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer
	fAvFileMap                                func(filename unsafe.Pointer, bufptr *unsafe.Pointer, size unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) int32
	fAvFilenameNumberTest                     func(filename unsafe.Pointer) int32
	fAvFileUnmap                              func(bufptr unsafe.Pointer, size uintptr)
	fAvFilterIterate                          func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvFindBestPixFmtOf2                      func(dst_pix_fmt1 int32, dst_pix_fmt2 int32, src_pix_fmt int32, has_alpha int32, loss_ptr *int32) int32
	fAvFindDefaultStreamIndex                 func(s unsafe.Pointer) int32
	fAvFindInfoTag                            func(arg unsafe.Pointer, arg_size int32, tag1 unsafe.Pointer, info unsafe.Pointer) int32
	fAvFindInputFormat                        func(short_name unsafe.Pointer) unsafe.Pointer
	fAvFindNearestQIdx                        func(q AVRational, q_list unsafe.Pointer) int32
	fAvFindProgramFromStream                  func(ic unsafe.Pointer, last unsafe.Pointer, s int32) unsafe.Pointer
	fAvFmtCtxGetDurationEstimationMethod      func(ctx unsafe.Pointer) unsafe.Pointer
	fAvForceCpuFlags                          func(flags int32)
	fAvFormatInjectGlobalSideData             func(s unsafe.Pointer)
	fAvFourccMakeString                       func(buf unsafe.Pointer, fourcc uint32) unsafe.Pointer
	fAvGcd                                    func(a int64, b int64) int64
	fAvGcdQ                                   func(a AVRational, b AVRational, max_den int32, def AVRational) AVRational
	fAvGetAltSampleFmt                        func(sample_fmt int32, planar int32) int32
	fAvGetAudioFrameDuration                  func(avctx unsafe.Pointer, frame_bytes int32) int32
	fAvGetAudioFrameDuration2                 func(par unsafe.Pointer, frame_bytes int32) int32
	fAvGetBitsPerSample                       func(codec_id int32) int32
	fAvGetCpuFlags                            func() int32
	fAvGetExactBitsPerSample                  func(codec_id int32) int32
	fAvGetFrameFilename                       func(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32) int32
	fAvGetFrameFilename2                      func(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32, flags int32) int32
	fAvGetKnownColorName                      func(color_idx int32, rgb *unsafe.Pointer) unsafe.Pointer
	fAvGetMediaTypeString                     func(media_type int32) unsafe.Pointer
	fAvGetOutputTimestamp                     func(s unsafe.Pointer, stream int32, dts unsafe.Pointer, wall unsafe.Pointer) int32
	fAvGetPacket                              func(s unsafe.Pointer, pkt unsafe.Pointer, size int32) int32
	fAvGetPaddedBitsPerPixel                  func(pixdesc unsafe.Pointer) int32
	fAvGetPcmCodec                            func(fmt int32, be int32) unsafe.Pointer
	fAvGetPictureTypeChar                     func(pict_type int32) byte
	fAvGetProfileName                         func(codec unsafe.Pointer, profile int32) unsafe.Pointer
	fAvGetRandomSeed                          func() uint32
	fAvGetTimeBaseQ                           func() AVRational
	fAvGetToken                               func(buf *unsafe.Pointer, term unsafe.Pointer) unsafe.Pointer
	fAvHashAlloc                              func(ctx *unsafe.Pointer, name unsafe.Pointer) int32
	fAvHashFinal                              func(ctx unsafe.Pointer, dst unsafe.Pointer)
	fAvHashFinalB64                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFinalBin                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFinalHex                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFreep                              func(ctx *unsafe.Pointer)
	fAvHashGetName                            func(ctx unsafe.Pointer) unsafe.Pointer
	fAvHashGetSize                            func(ctx unsafe.Pointer) int32
	fAvHashInit                               func(ctx unsafe.Pointer)
	fAvHashNames                              func(i int32) unsafe.Pointer
	fAvHashUpdate                             func(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr)
	fAvHexDump                                func(f unsafe.Pointer, buf unsafe.Pointer, size int32)
	fAvHexDumpLog                             func(avcl unsafe.Pointer, level int32, buf unsafe.Pointer, size int32)
	fAvHmacAlloc                              func(typ int32) unsafe.Pointer
	fAvHmacCalc                               func(ctx unsafe.Pointer, data unsafe.Pointer, len uint32, key unsafe.Pointer, keylen uint32, out unsafe.Pointer, outlen uint32) int32
	fAvHmacFinal                              func(ctx unsafe.Pointer, out unsafe.Pointer, outlen uint32) int32
	fAvHmacFree                               func(ctx unsafe.Pointer)
	fAvHmacInit                               func(ctx unsafe.Pointer, key unsafe.Pointer, keylen uint32)
	fAvHmacUpdate                             func(ctx unsafe.Pointer, data unsafe.Pointer, len uint32)
	fAvHwdeviceCtxAlloc                       func(typ int32) unsafe.Pointer
	fAvHwdeviceCtxCreate                      func(device_ctx *unsafe.Pointer, typ int32, device unsafe.Pointer, opts unsafe.Pointer, flags int32) int32
	fAvHwdeviceCtxCreateDerived               func(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, flags int32) int32
	fAvHwdeviceCtxCreateDerivedOpts           func(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, options unsafe.Pointer, flags int32) int32
	fAvHwdeviceCtxInit                        func(ref unsafe.Pointer) int32
	fAvHwdeviceFindTypeByName                 func(name string) int32
	fAvHwdeviceGetHwframeConstraints          func(ref unsafe.Pointer, hwconfig unsafe.Pointer) unsafe.Pointer
	fAvHwdeviceGetTypeName                    func(typ int32) unsafe.Pointer
	fAvHwdeviceHwconfigAlloc                  func(device_ctx unsafe.Pointer) unsafe.Pointer
	fAvHwdeviceIterateTypes                   func(prev int32) int32
	fAvHwframeConstraintsFree                 func(constraints *unsafe.Pointer)
	fAvHwframeCtxAlloc                        func(device_ctx unsafe.Pointer) unsafe.Pointer
	fAvHwframeCtxCreateDerived                func(derived_frame_ctx *unsafe.Pointer, format int32, derived_device_ctx unsafe.Pointer, source_frame_ctx unsafe.Pointer, flags int32) int32
	fAvHwframeCtxInit                         func(ref unsafe.Pointer) int32
	fAvHwframeGetBuffer                       func(hwframe_ctx unsafe.Pointer, frame unsafe.Pointer, flags int32) int32
	fAvHwframeMap                             func(dst unsafe.Pointer, src unsafe.Pointer, flags int32) int32
	fAvHwframeTransferData                    func(dst unsafe.Pointer, src unsafe.Pointer, flags int32) int32
	fAvHwframeTransferGetFormats              func(hwframe_ctx unsafe.Pointer, dir int32, formats *unsafe.Pointer, flags int32) int32
	fAvI2int                                  func(a AVInteger) int64
	fAvIamfAudioElementAddLayer               func(audio_element unsafe.Pointer) unsafe.Pointer
	fAvIamfAudioElementAlloc                  func() unsafe.Pointer
	fAvIamfAudioElementFree                   func(audio_element *unsafe.Pointer)
	fAvIamfAudioElementGetClass               func() unsafe.Pointer
	fAvIamfMixPresentationAddSubmix           func(mix_presentation unsafe.Pointer) unsafe.Pointer
	fAvIamfMixPresentationAlloc               func() unsafe.Pointer
	fAvIamfMixPresentationFree                func(mix_presentation *unsafe.Pointer)
	fAvIamfMixPresentationGetClass            func() unsafe.Pointer
	fAvIamfParamDefinitionAlloc               func(typ unsafe.Pointer, nb_subblocks uint32, size unsafe.Pointer) unsafe.Pointer
	fAvIamfParamDefinitionGetClass            func() unsafe.Pointer
	fAvIamfSubmixAddElement                   func(submix unsafe.Pointer) unsafe.Pointer
	fAvIamfSubmixAddLayout                    func(submix unsafe.Pointer) unsafe.Pointer
	fAvImdctCalc                              func(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer
	fAvImdctHalf                              func(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer
	fAvInitPacket                             func(pkt unsafe.Pointer) unsafe.Pointer
	fAvInputAudioDeviceNext                   func(d unsafe.Pointer) unsafe.Pointer
	fAvInputVideoDeviceNext                   func(d unsafe.Pointer) unsafe.Pointer
	fAvInt2i                                  func(a int64) AVInteger
	fAvIntListLengthForSize                   func(elsize uint32, list unsafe.Pointer, term uint64) uint32
	fAvInterleavedWriteFrame                  func(s unsafe.Pointer, pkt unsafe.Pointer) int32
	fAvInterleavedWriteUncodedFrame           func(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) int32
	fAvJniGetJavaVm                           func(log_ctx unsafe.Pointer) unsafe.Pointer
	fAvJniSetJavaVm                           func(vm unsafe.Pointer, log_ctx unsafe.Pointer) unsafe.Pointer
	fAvLfgInit                                func(c unsafe.Pointer, seed uint32)
	fAvLfgInitFromData                        func(c unsafe.Pointer, data unsafe.Pointer, length uint32) int32
	fAvLog                                    func(avcl unsafe.Pointer, level int32, fmt unsafe.Pointer)
	fAvLzo1xDecode                            func(out unsafe.Pointer, outlen unsafe.Pointer, in unsafe.Pointer, inlen unsafe.Pointer) unsafe.Pointer
	fAvMatchExt                               func(filename unsafe.Pointer, extensions unsafe.Pointer) int32
	fAvMatchList                              func(name unsafe.Pointer, list unsafe.Pointer, separator byte) int32
	fAvMatchName                              func(name unsafe.Pointer, names unsafe.Pointer) int32
	fAvMaxAlloc                               func(max uintptr)
	fAvMd5Alloc                               func() unsafe.Pointer
	fAvMd5Final                               func(ctx unsafe.Pointer, dst unsafe.Pointer)
	fAvMd5Init                                func(ctx unsafe.Pointer)
	fAvMd5Sum                                 func(dst unsafe.Pointer, src unsafe.Pointer, len uintptr)
	fAvMd5Update                              func(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr)
	fAvMdctCalc                               func(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer
	fAvMdctEnd                                func(s unsafe.Pointer) unsafe.Pointer
	fAvMdctInit                               func(nbits int32, inverse int32, scale float64) unsafe.Pointer
	fAvMediacodecAllocContext                 func() unsafe.Pointer
	fAvMediacodecDefaultFree                  func(avctx unsafe.Pointer)
	fAvMediacodecDefaultInit                  func(avctx unsafe.Pointer, ctx unsafe.Pointer, surface unsafe.Pointer) int32
	fAvMediacodecReleaseBuffer                func(buffer unsafe.Pointer, render int32) int32
	fAvMediacodecRenderBufferAtTime           func(buffer unsafe.Pointer, time int64) int32
	fAvModI                                   func(quot *AVInteger, a AVInteger, b AVInteger) AVInteger
	fAvMulI                                   func(a AVInteger, b AVInteger) AVInteger
	fAvMulQ                                   func(b AVRational, c AVRational) AVRational
	fAvMurmur3Alloc                           func() unsafe.Pointer
	fAvMurmur3Final                           func(c unsafe.Pointer, dst unsafe.Pointer)
	fAvMurmur3Init                            func(c unsafe.Pointer)
	fAvMurmur3InitSeeded                      func(c unsafe.Pointer, seed uint64)
	fAvMurmur3Update                          func(c unsafe.Pointer, src unsafe.Pointer, len uintptr)
	fAvMuxerIterate                           func(opaque *unsafe.Pointer) unsafe.Pointer
	fAvNearerQ                                func(q AVRational, q1 AVRational, q2 AVRational) int32
	fAvNewProgram                             func(s unsafe.Pointer, id int32) unsafe.Pointer
	fAvOutputAudioDeviceNext                  func(d unsafe.Pointer) unsafe.Pointer
	fAvOutputVideoDeviceNext                  func(d unsafe.Pointer) unsafe.Pointer
	fAvPixelutilsGetSadFn                     func(w_bits int32, h_bits int32, aligned int32, log_ctx unsafe.Pointer) unsafe.Pointer
	fAvPktDump2                               func(f unsafe.Pointer, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer)
	fAvPktDumpLog2                            func(avcl unsafe.Pointer, level int32, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer)
	fAvProbeInputBuffer                       func(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) int32
	fAvProbeInputBuffer2                      func(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) int32
	fAvProbeInputFormat                       func(pd unsafe.Pointer, is_opened int32) unsafe.Pointer
	fAvProbeInputFormat2                      func(pd unsafe.Pointer, is_opened int32, score_max unsafe.Pointer) unsafe.Pointer
	fAvProbeInputFormat3                      func(pd unsafe.Pointer, is_opened int32, score_ret unsafe.Pointer) unsafe.Pointer
	fAvProgramAddStreamIndex                  func(ac unsafe.Pointer, progid int32, idx uint32)
	fAvQ2intfloat                             func(q AVRational) uint32
	fAvQsvAllocContext                        func() unsafe.Pointer
	fAvRandomBytes                            func(buf unsafe.Pointer, len uintptr) int32
	fAvRc4Alloc                               func() unsafe.Pointer
	fAvRc4Crypt                               func(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvRc4Init                                func(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) int32
	fAvRdftCalc                               func(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer
	fAvRdftEnd                                func(s unsafe.Pointer) unsafe.Pointer
	fAvRdftInit                               func(nbits int32, trans unsafe.Pointer) unsafe.Pointer
	fAvReadImageLine                          func(dst unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32)
	fAvReadImageLine2                         func(dst unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32, dst_element_size int32)
	fAvReadPause                              func(s unsafe.Pointer) int32
	fAvReadPlay                               func(s unsafe.Pointer) int32
	fAvReduce                                 func(dst_num unsafe.Pointer, dst_den unsafe.Pointer, num int64, den int64, max int64) int32
	fAvRipemdAlloc                            func() unsafe.Pointer
	fAvRipemdFinal                            func(context unsafe.Pointer, digest unsafe.Pointer)
	fAvRipemdInit                             func(context unsafe.Pointer, bits int32) int32
	fAvRipemdUpdate                           func(context unsafe.Pointer, data unsafe.Pointer, len uintptr)
	fAvSamplesAlloc                           func(audio_data *unsafe.Pointer, linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) int32
	fAvSamplesAllocArrayAndSamples            func(audio_data *unsafe.Pointer, linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) int32
	fAvSamplesCopy                            func(dst unsafe.Pointer, src unsafe.Pointer, dst_offset int32, src_offset int32, nb_samples int32, nb_channels int32, sample_fmt int32) int32
	fAvSamplesFillArrays                      func(audio_data *unsafe.Pointer, linesize *int32, buf unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) int32
	fAvSamplesGetBufferSize                   func(linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) int32
	fAvSamplesSetSilence                      func(audio_data unsafe.Pointer, offset int32, nb_samples int32, nb_channels int32, sample_fmt int32) int32
	fAvSdpCreate                              func(ac unsafe.Pointer, n_files int32, buf unsafe.Pointer, size int32) int32
	fAvSetOptionsString                       func(ctx unsafe.Pointer, opts unsafe.Pointer, key_val_sep unsafe.Pointer, pairs_sep unsafe.Pointer) int32
	fAvSha512Alloc                            func() unsafe.Pointer
	fAvSha512Final                            func(context unsafe.Pointer, digest unsafe.Pointer)
	fAvSha512Init                             func(context unsafe.Pointer, bits int32) int32
	fAvSha512Update                           func(context unsafe.Pointer, data unsafe.Pointer, len uintptr)
	fAvShaAlloc                               func() unsafe.Pointer
	fAvShaFinal                               func(context unsafe.Pointer, digest unsafe.Pointer)
	fAvShaInit                                func(context unsafe.Pointer, bits int32) int32
	fAvShaUpdate                              func(ctx unsafe.Pointer, data unsafe.Pointer, len uintptr)
	fAvShrI                                   func(a AVInteger, s int32) AVInteger
	fAvSizeMult                               func(a uintptr, b uintptr, r unsafe.Pointer) int32
	fAvSmallStrptime                          func(p unsafe.Pointer, fmt unsafe.Pointer, dt unsafe.Pointer) unsafe.Pointer
	fAvSscanf                                 func(str unsafe.Pointer, format unsafe.Pointer) int32
	fAvStrcasecmp                             func(a unsafe.Pointer, b unsafe.Pointer) int32
	fAvStrdup                                 func(s unsafe.Pointer) unsafe.Pointer
	fAvStrndup                                func(s string, ln uintptr) unsafe.Pointer
	fAvStreamAddSideData                      func(st unsafe.Pointer, typ unsafe.Pointer, data unsafe.Pointer, size uintptr) unsafe.Pointer
	fAvStreamGetClass                         func() unsafe.Pointer
	fAvStreamGetCodecTimebase                 func(st unsafe.Pointer) unsafe.Pointer
	fAvStreamGetParser                        func(s unsafe.Pointer) unsafe.Pointer
	fAvStreamGetSideData                      func(stream unsafe.Pointer, typ unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer
	fAvStreamGroupGetClass                    func() unsafe.Pointer
	fAvStreamNewSideData                      func(stream unsafe.Pointer, typ unsafe.Pointer, size uintptr) unsafe.Pointer
	fAvStrireplace                            func(str unsafe.Pointer, from unsafe.Pointer, to unsafe.Pointer) unsafe.Pointer
	fAvStristart                              func(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) int32
	fAvStristr                                func(haystack unsafe.Pointer, needle unsafe.Pointer) unsafe.Pointer
	fAvStrlcat                                func(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr
	fAvStrlcpy                                func(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr
	fAvStrncasecmp                            func(a unsafe.Pointer, b unsafe.Pointer, n uintptr) int32
	fAvStrnstr                                func(haystack unsafe.Pointer, needle unsafe.Pointer, hay_length uintptr) unsafe.Pointer
	fAvStrstart                               func(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) int32
	fAvStrtod                                 func(numstr unsafe.Pointer, tail *unsafe.Pointer) float64
	fAvStrtok                                 func(s unsafe.Pointer, delim unsafe.Pointer, saveptr *unsafe.Pointer) unsafe.Pointer
	fAvSubI                                   func(a AVInteger, b AVInteger) AVInteger
	fAvSubQ                                   func(b AVRational, c AVRational) AVRational
	fAvTeaAlloc                               func() unsafe.Pointer
	fAvTeaCrypt                               func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvTeaInit                                func(ctx unsafe.Pointer, key unsafe.Pointer, rounds int32)
	fAvThreadMessageFlush                     func(mq unsafe.Pointer)
	fAvThreadMessageQueueAlloc                func(mq *unsafe.Pointer, nelem uint32, elsize uint32) int32
	fAvThreadMessageQueueFree                 func(mq *unsafe.Pointer)
	fAvThreadMessageQueueNbElems              func(mq unsafe.Pointer) int32
	fAvThreadMessageQueueRecv                 func(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) int32
	fAvThreadMessageQueueSend                 func(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) int32
	fAvThreadMessageQueueSetErrRecv           func(mq unsafe.Pointer, err int32)
	fAvThreadMessageQueueSetErrSend           func(mq unsafe.Pointer, err int32)
	fAvThreadMessageQueueSetFreeFunc          func(mq unsafe.Pointer, free_func unsafe.Pointer)
	fAvTimegm                                 func(tm unsafe.Pointer) int64
	fAvTreeDestroy                            func(t unsafe.Pointer)
	fAvTreeEnumerate                          func(t unsafe.Pointer, opaque unsafe.Pointer, cmp unsafe.Pointer, enu unsafe.Pointer)
	fAvTreeFind                               func(root unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next unsafe.Pointer) unsafe.Pointer
	fAvTreeInsert                             func(rootp *unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next *unsafe.Pointer) unsafe.Pointer
	fAvTreeNodeAlloc                          func() unsafe.Pointer
	fAvTsMakeTimeString2                      func(buf unsafe.Pointer, ts int64, tb AVRational) unsafe.Pointer
	fAvTwofishAlloc                           func() unsafe.Pointer
	fAvTwofishCrypt                           func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvTwofishInit                            func(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) int32
	fAvTxInit                                 func(ctx *unsafe.Pointer, tx unsafe.Pointer, typ unsafe.Pointer, inv int32, len int32, scale unsafe.Pointer, flags uint64) int32
	fAvTxUninit                               func(ctx *unsafe.Pointer)
	fAvUtf8Decode                             func(codep unsafe.Pointer, bufp *unsafe.Pointer, buf_end unsafe.Pointer, flags uint32) int32
	fAvUuidParse                              func(in unsafe.Pointer, uu unsafe.Pointer) int32
	fAvUuidParseRange                         func(in_start unsafe.Pointer, in_end unsafe.Pointer, uu unsafe.Pointer) int32
	fAvUuidUnparse                            func(uu unsafe.Pointer, out unsafe.Pointer)
	fAvUuidUrnParse                           func(in unsafe.Pointer, uu unsafe.Pointer) int32
	fAvVbprintf                               func(buf unsafe.Pointer, fmt unsafe.Pointer, vl_arg unsafe.Pointer)
	fAvVdpauAllocContext                      func() unsafe.Pointer
	fAvVdpauBindContext                       func(avctx unsafe.Pointer, device unsafe.Pointer, get_proc_address unsafe.Pointer, flags uint32) int32
	fAvVdpauGetSurfaceParameters              func(avctx unsafe.Pointer, typ unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer) int32
	fAvVdpauHwaccelGetRender2                 func(arg0 unsafe.Pointer) unsafe.Pointer
	fAvVdpauHwaccelSetRender2                 func(arg0 unsafe.Pointer, arg1 unsafe.Pointer) unsafe.Pointer
	fAvVideoEncParamsAlloc                    func(typ unsafe.Pointer, nb_blocks uint32, out_size unsafe.Pointer) unsafe.Pointer
	fAvVideoEncParamsCreateSideData           func(frame unsafe.Pointer, typ unsafe.Pointer, nb_blocks uint32) unsafe.Pointer
	fAvVideoHintAlloc                         func(nb_rects uintptr, out_size unsafe.Pointer) unsafe.Pointer
	fAvVideoHintCreateSideData                func(frame unsafe.Pointer, nb_rects uintptr) unsafe.Pointer
	fAvVkfmtFromPixfmt                        func(p int32) unsafe.Pointer
	fAvVkFrameAlloc                           func() unsafe.Pointer
	fAvVlog                                   func(avcl unsafe.Pointer, level int32, fmt unsafe.Pointer, vl unsafe.Pointer)
	fAvVorbisParseFrame                       func(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32) int32
	fAvVorbisParseFrameFlags                  func(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32, flags unsafe.Pointer) unsafe.Pointer
	fAvVorbisParseFree                        func(s *unsafe.Pointer)
	fAvVorbisParseInit                        func(extradata unsafe.Pointer, extradata_size int32) unsafe.Pointer
	fAvVorbisParseReset                       func(s unsafe.Pointer)
	fAvWriteFrame                             func(s unsafe.Pointer, pkt unsafe.Pointer) int32
	fAvWriteImageLine                         func(src unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32)
	fAvWriteImageLine2                        func(src unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, src_element_size int32)
	fAvWriteTrailer                           func(s unsafe.Pointer) int32
	fAvWriteUncodedFrame                      func(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) int32
	fAvWriteUncodedFrameQuery                 func(s unsafe.Pointer, stream_index int32) int32
	fAvXiphlacing                             func(s unsafe.Pointer, v uint32) uint32
	fAvXteaAlloc                              func() unsafe.Pointer
	fAvXteaCrypt                              func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvXteaInit                               func(ctx unsafe.Pointer, key unsafe.Pointer)
	fAvXteaLeCrypt                            func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvXteaLeInit                             func(ctx unsafe.Pointer, key unsafe.Pointer)
	fSwresampleConfiguration                  func() unsafe.Pointer
	fSwresampleLicense                        func() unsafe.Pointer
	fSwresampleVersion                        func() uint32
	fSwriAudioConvert                         func(ctx unsafe.Pointer, out unsafe.Pointer, in unsafe.Pointer, len int32) int32
	fSwriAudioConvertAlloc                    func(out_fmt unsafe.Pointer, in_fmt unsafe.Pointer, channels int32, ch_map unsafe.Pointer, flags int32) unsafe.Pointer
	fSwriAudioConvertFree                     func(ctx *unsafe.Pointer)
	fSwriResampleDspInit                      func(c unsafe.Pointer)
	fSwriResampleDspX86Init                   func(c unsafe.Pointer)
	fAvAddI                                   func(a AVInteger, b AVInteger) AVInteger
	fAvDynamicHdrVividAlloc                   func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrVividCreateSideData          func(frame unsafe.Pointer) unsafe.Pointer
	fAvformatTransferInternalStreamTimingInfo func(ofmt unsafe.Pointer, ost unsafe.Pointer, ist unsafe.Pointer, copy_tb unsafe.Pointer) unsafe.Pointer
	fAvImageCopy                              func(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt int32, width int32, height int32)
	fAvImageCopyPlaneUcFrom                   func(dst unsafe.Pointer, dst_linesize uintptr, src unsafe.Pointer, src_linesize uintptr, bytewidth uintptr, height int32)
	fAvImageCopyToBuffer                      func(dst unsafe.Pointer, dst_size int32, src_data unsafe.Pointer, src_linesize unsafe.Pointer, pix_fmt int32, width int32, height int32, align int32) int32
	fAvImageCopyUcFrom                        func(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt int32, width int32, height int32)
	fAvImageFillArrays                        func(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, src unsafe.Pointer, pix_fmt int32, width int32, height int32, align int32) int32
	fAvImageFillBlack                         func(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt int32, colorRange int32, width int32, height int32) int32
	fAvImageFillColor                         func(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt int32, color unsafe.Pointer, width int32, height int32, flags int32) int32
	fAvImageFillLinesizes                     func(linesizes unsafe.Pointer, pix_fmt int32, width int32) int32
	fAvImageFillMaxPixsteps                   func(max_pixsteps unsafe.Pointer, max_pixstep_comps unsafe.Pointer, pixdesc unsafe.Pointer)
	fAvImageFillPlaneSizes                    func(size unsafe.Pointer, pix_fmt int32, height int32, linesizes unsafe.Pointer) int32
	fAvImageFillPointers                      func(data unsafe.Pointer, pix_fmt int32, height int32, ptr unsafe.Pointer, linesizes unsafe.Pointer) int32
	fAvImageGetLinesize                       func(pix_fmt int32, width int32, plane int32) int32
	fAvLog2                                   func(v uint32) int32
	fAvLog216bit                              func(v uint32) int32
	fAvLog2I                                  func(a AVInteger) int32
	fAvParseCpuCaps                           func(flags unsafe.Pointer, s unsafe.Pointer) int32
	fAvPixFmtCountPlanes                      func(pix_fmt int32) int32
	fAvPixFmtGetChromaSubSample               func(pix_fmt int32, h_shift *int32, v_shift *int32) int32
	fAvPixFmtSwapEndianness                   func(pix_fmt int32) int32
)

// ensureModCrypto 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modCryptoOnce sync.Once

func ensureModCrypto() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	if err := ensureModBufferMem(); err != nil {
		return err
	}
	modCryptoOnce.Do(func() { registerCryptoHashMisc(libHandle) })
	return nil
}

// registerCryptoHwPriv 绑 x86/VDPAU 专有符号（独立成组，见 ensureModCryptoHw）。
func registerCryptoHwPriv(h uintptr) {
	purego.RegisterLibFunc(&fAvAllocVdpaucontext, h, "av_alloc_vdpaucontext")
	purego.RegisterLibFunc(&fAvVdpauAllocContext, h, "av_vdpau_alloc_context")
	purego.RegisterLibFunc(&fAvVdpauBindContext, h, "av_vdpau_bind_context")
	purego.RegisterLibFunc(&fAvVdpauGetSurfaceParameters, h, "av_vdpau_get_surface_parameters")
	purego.RegisterLibFunc(&fAvVdpauHwaccelGetRender2, h, "av_vdpau_hwaccel_get_render2")
	purego.RegisterLibFunc(&fAvVdpauHwaccelSetRender2, h, "av_vdpau_hwaccel_set_render2")
	purego.RegisterLibFunc(&fSwriResampleDspX86Init, h, "swri_resample_dsp_x86_init")
}

// ensureModCryptoHw 开硬解私有房的灯（VDPAU 六件 + x86 重采样一件）。
// 大白话：这 7 个符号只在 x86-linux 上有，ARM/386/win 上没有；
// 单独成组后，其他平台用解码编码转色烧字都不经过这里，不会误崩。
// 只有真调 VDPAU/x86 私有函数那一刻才报错（fail fast，不装绿）。
var modCryptoHwOnce sync.Once

func ensureModCryptoHw() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modCryptoHwOnce.Do(func() { registerCryptoHwPriv(libHandle) })
	return nil
}

func registerCryptoHashMisc(h uintptr) {
	purego.RegisterLibFunc(&fAvAc3ParseHeader, h, "av_ac3_parse_header")
	purego.RegisterLibFunc(&fAvAdtsHeaderParse, h, "av_adts_header_parse")
	purego.RegisterLibFunc(&fAvAdler32Update, h, "av_adler32_update")
	purego.RegisterLibFunc(&fAvAesAlloc, h, "av_aes_alloc")
	purego.RegisterLibFunc(&fAvAesCrypt, h, "av_aes_crypt")
	purego.RegisterLibFunc(&fAvAesCtrAlloc, h, "av_aes_ctr_alloc")
	purego.RegisterLibFunc(&fAvAesCtrCrypt, h, "av_aes_ctr_crypt")
	purego.RegisterLibFunc(&fAvAesCtrFree, h, "av_aes_ctr_free")
	purego.RegisterLibFunc(&fAvAesCtrGetIv, h, "av_aes_ctr_get_iv")
	purego.RegisterLibFunc(&fAvAesCtrIncrementIv, h, "av_aes_ctr_increment_iv")
	purego.RegisterLibFunc(&fAvAesCtrInit, h, "av_aes_ctr_init")
	purego.RegisterLibFunc(&fAvAesCtrSetFullIv, h, "av_aes_ctr_set_full_iv")
	purego.RegisterLibFunc(&fAvAesCtrSetIv, h, "av_aes_ctr_set_iv")
	purego.RegisterLibFunc(&fAvAesCtrSetRandomIv, h, "av_aes_ctr_set_random_iv")
	purego.RegisterLibFunc(&fAvAesInit, h, "av_aes_init")
	purego.RegisterLibFunc(&fAvAppendPacket, h, "av_append_packet")
	purego.RegisterLibFunc(&fAvAppendPathComponent, h, "av_append_path_component")
	purego.RegisterLibFunc(&fAvAssert0Fpu, h, "av_assert0_fpu")
	purego.RegisterLibFunc(&fAvBase64Decode, h, "av_base64_decode")
	purego.RegisterLibFunc(&fAvBase64Encode, h, "av_base64_encode")
	purego.RegisterLibFunc(&fAvBasename, h, "av_basename")
	purego.RegisterLibFunc(&fAvBesselI0, h, "av_bessel_i0")
	purego.RegisterLibFunc(&fAvBlowfishAlloc, h, "av_blowfish_alloc")
	purego.RegisterLibFunc(&fAvBlowfishCrypt, h, "av_blowfish_crypt")
	purego.RegisterLibFunc(&fAvBlowfishCryptEcb, h, "av_blowfish_crypt_ecb")
	purego.RegisterLibFunc(&fAvBlowfishInit, h, "av_blowfish_init")
	purego.RegisterLibFunc(&fAvBmgGet, h, "av_bmg_get")
	purego.RegisterLibFunc(&fAvBprintf, h, "av_bprintf")
	purego.RegisterLibFunc(&fAvCalloc, h, "av_calloc")
	purego.RegisterLibFunc(&fAvCamelliaAlloc, h, "av_camellia_alloc")
	purego.RegisterLibFunc(&fAvCamelliaCrypt, h, "av_camellia_crypt")
	purego.RegisterLibFunc(&fAvCamelliaInit, h, "av_camellia_init")
	purego.RegisterLibFunc(&fAvCast5Alloc, h, "av_cast5_alloc")
	purego.RegisterLibFunc(&fAvCast5Crypt, h, "av_cast5_crypt")
	purego.RegisterLibFunc(&fAvCast5Crypt2, h, "av_cast5_crypt2")
	purego.RegisterLibFunc(&fAvCast5Init, h, "av_cast5_init")
	purego.RegisterLibFunc(&fAvChromaLocationEnumToPos, h, "av_chroma_location_enum_to_pos")
	purego.RegisterLibFunc(&fAvChromaLocationFromName, h, "av_chroma_location_from_name")
	purego.RegisterLibFunc(&fAvChromaLocationName, h, "av_chroma_location_name")
	purego.RegisterLibFunc(&fAvChromaLocationPosToEnum, h, "av_chroma_location_pos_to_enum")
	purego.RegisterLibFunc(&fAvCmpI, h, "av_cmp_i")
	purego.RegisterLibFunc(&fAvCpbPropertiesAlloc, h, "av_cpb_properties_alloc")
	purego.RegisterLibFunc(&fAvCrcGetTable, h, "av_crc_get_table")
	purego.RegisterLibFunc(&fAvCrc, h, "av_crc")
	purego.RegisterLibFunc(&fAvCrcInit, h, "av_crc_init")
	purego.RegisterLibFunc(&fAvCspApproximateTrcGamma, h, "av_csp_approximate_trc_gamma")
	purego.RegisterLibFunc(&fAvCspLumaCoeffsFromAvcsp, h, "av_csp_luma_coeffs_from_avcsp")
	purego.RegisterLibFunc(&fAvCspPrimariesDescFromId, h, "av_csp_primaries_desc_from_id")
	purego.RegisterLibFunc(&fAvCspPrimariesIdFromDesc, h, "av_csp_primaries_id_from_desc")
	purego.RegisterLibFunc(&fAvCspTrcFuncFromId, h, "av_csp_trc_func_from_id")
	purego.RegisterLibFunc(&fAvD2q, h, "av_d2q")
	purego.RegisterLibFunc(&fAvD3d11vaAllocContext, h, "av_d3d11va_alloc_context")
	purego.RegisterLibFunc(&fAvDctCalc, h, "av_dct_calc")
	purego.RegisterLibFunc(&fAvDctEnd, h, "av_dct_end")
	purego.RegisterLibFunc(&fAvDctInit, h, "av_dct_init")
	purego.RegisterLibFunc(&fAvDefaultGetCategory, h, "av_default_get_category")
	purego.RegisterLibFunc(&fAvDefaultItemName, h, "av_default_item_name")
	purego.RegisterLibFunc(&fAvDemuxerIterate, h, "av_demuxer_iterate")
	purego.RegisterLibFunc(&fAvDesAlloc, h, "av_des_alloc")
	purego.RegisterLibFunc(&fAvDesCrypt, h, "av_des_crypt")
	purego.RegisterLibFunc(&fAvDesInit, h, "av_des_init")
	purego.RegisterLibFunc(&fAvDesMac, h, "av_des_mac")
	purego.RegisterLibFunc(&fAvDetectionBboxAlloc, h, "av_detection_bbox_alloc")
	purego.RegisterLibFunc(&fAvDetectionBboxCreateSideData, h, "av_detection_bbox_create_side_data")
	purego.RegisterLibFunc(&fAvDiracParseSequenceHeader, h, "av_dirac_parse_sequence_header")
	purego.RegisterLibFunc(&fAvDirname, h, "av_dirname")
	purego.RegisterLibFunc(&fAvDispositionFromString, h, "av_disposition_from_string")
	purego.RegisterLibFunc(&fAvDispositionToString, h, "av_disposition_to_string")
	purego.RegisterLibFunc(&fAvDivI, h, "av_div_i")
	purego.RegisterLibFunc(&fAvDivQ, h, "av_div_q")
	purego.RegisterLibFunc(&fAvDownmixInfoUpdateSideData, h, "av_downmix_info_update_side_data")
	purego.RegisterLibFunc(&fAvDvCodecProfile, h, "av_dv_codec_profile")
	purego.RegisterLibFunc(&fAvDvCodecProfile2, h, "av_dv_codec_profile2")
	purego.RegisterLibFunc(&fAvDvFrameProfile, h, "av_dv_frame_profile")
	purego.RegisterLibFunc(&fAvDynarray2Add, h, "av_dynarray2_add")
	purego.RegisterLibFunc(&fAvDynarrayAdd, h, "av_dynarray_add")
	purego.RegisterLibFunc(&fAvDynarrayAddNofree, h, "av_dynarray_add_nofree")
	purego.RegisterLibFunc(&fAvEncryptionInfoAddSideData, h, "av_encryption_info_add_side_data")
	purego.RegisterLibFunc(&fAvEncryptionInfoAlloc, h, "av_encryption_info_alloc")
	purego.RegisterLibFunc(&fAvEncryptionInfoClone, h, "av_encryption_info_clone")
	purego.RegisterLibFunc(&fAvEncryptionInfoFree, h, "av_encryption_info_free")
	purego.RegisterLibFunc(&fAvEncryptionInfoGetSideData, h, "av_encryption_info_get_side_data")
	purego.RegisterLibFunc(&fAvEncryptionInitInfoAddSideData, h, "av_encryption_init_info_add_side_data")
	purego.RegisterLibFunc(&fAvEncryptionInitInfoAlloc, h, "av_encryption_init_info_alloc")
	purego.RegisterLibFunc(&fAvEncryptionInitInfoFree, h, "av_encryption_init_info_free")
	purego.RegisterLibFunc(&fAvEncryptionInitInfoGetSideData, h, "av_encryption_init_info_get_side_data")
	purego.RegisterLibFunc(&fAvEscape, h, "av_escape")
	purego.RegisterLibFunc(&fAvExecutorAlloc, h, "av_executor_alloc")
	purego.RegisterLibFunc(&fAvExecutorExecute, h, "av_executor_execute")
	purego.RegisterLibFunc(&fAvExecutorFree, h, "av_executor_free")
	purego.RegisterLibFunc(&fAvExprCountFunc, h, "av_expr_count_func")
	purego.RegisterLibFunc(&fAvExprCountVars, h, "av_expr_count_vars")
	purego.RegisterLibFunc(&fAvExprEval, h, "av_expr_eval")
	purego.RegisterLibFunc(&fAvExprFree, h, "av_expr_free")
	purego.RegisterLibFunc(&fAvExprParse, h, "av_expr_parse")
	purego.RegisterLibFunc(&fAvExprParseAndEval, h, "av_expr_parse_and_eval")
	purego.RegisterLibFunc(&fAvFastMalloc, h, "av_fast_malloc")
	purego.RegisterLibFunc(&fAvFastMallocz, h, "av_fast_mallocz")
	purego.RegisterLibFunc(&fAvFastPaddedMalloc, h, "av_fast_padded_malloc")
	purego.RegisterLibFunc(&fAvFastPaddedMallocz, h, "av_fast_padded_mallocz")
	purego.RegisterLibFunc(&fAvFastRealloc, h, "av_fast_realloc")
	purego.RegisterLibFunc(&fAvFftCalc, h, "av_fft_calc")
	purego.RegisterLibFunc(&fAvFftEnd, h, "av_fft_end")
	purego.RegisterLibFunc(&fAvFftInit, h, "av_fft_init")
	purego.RegisterLibFunc(&fAvFftPermute, h, "av_fft_permute")
	purego.RegisterLibFunc(&fAvFileMap, h, "av_file_map")
	purego.RegisterLibFunc(&fAvFilenameNumberTest, h, "av_filename_number_test")
	purego.RegisterLibFunc(&fAvFileUnmap, h, "av_file_unmap")
	purego.RegisterLibFunc(&fAvFilterIterate, h, "av_filter_iterate")
	purego.RegisterLibFunc(&fAvFindBestPixFmtOf2, h, "av_find_best_pix_fmt_of_2")
	purego.RegisterLibFunc(&fAvFindDefaultStreamIndex, h, "av_find_default_stream_index")
	purego.RegisterLibFunc(&fAvFindInfoTag, h, "av_find_info_tag")
	purego.RegisterLibFunc(&fAvFindInputFormat, h, "av_find_input_format")
	purego.RegisterLibFunc(&fAvFindNearestQIdx, h, "av_find_nearest_q_idx")
	purego.RegisterLibFunc(&fAvFindProgramFromStream, h, "av_find_program_from_stream")
	purego.RegisterLibFunc(&fAvFmtCtxGetDurationEstimationMethod, h, "av_fmt_ctx_get_duration_estimation_method")
	purego.RegisterLibFunc(&fAvForceCpuFlags, h, "av_force_cpu_flags")
	purego.RegisterLibFunc(&fAvFormatInjectGlobalSideData, h, "av_format_inject_global_side_data")
	purego.RegisterLibFunc(&fAvFourccMakeString, h, "av_fourcc_make_string")
	purego.RegisterLibFunc(&fAvGcd, h, "av_gcd")
	purego.RegisterLibFunc(&fAvGcdQ, h, "av_gcd_q")
	purego.RegisterLibFunc(&fAvGetAltSampleFmt, h, "av_get_alt_sample_fmt")
	purego.RegisterLibFunc(&fAvGetAudioFrameDuration, h, "av_get_audio_frame_duration")
	purego.RegisterLibFunc(&fAvGetAudioFrameDuration2, h, "av_get_audio_frame_duration2")
	purego.RegisterLibFunc(&fAvGetBitsPerSample, h, "av_get_bits_per_sample")
	purego.RegisterLibFunc(&fAvGetCpuFlags, h, "av_get_cpu_flags")
	purego.RegisterLibFunc(&fAvGetExactBitsPerSample, h, "av_get_exact_bits_per_sample")
	purego.RegisterLibFunc(&fAvGetFrameFilename, h, "av_get_frame_filename")
	purego.RegisterLibFunc(&fAvGetFrameFilename2, h, "av_get_frame_filename2")
	purego.RegisterLibFunc(&fAvGetKnownColorName, h, "av_get_known_color_name")
	purego.RegisterLibFunc(&fAvGetMediaTypeString, h, "av_get_media_type_string")
	purego.RegisterLibFunc(&fAvGetOutputTimestamp, h, "av_get_output_timestamp")
	purego.RegisterLibFunc(&fAvGetPacket, h, "av_get_packet")
	purego.RegisterLibFunc(&fAvGetPaddedBitsPerPixel, h, "av_get_padded_bits_per_pixel")
	purego.RegisterLibFunc(&fAvGetPcmCodec, h, "av_get_pcm_codec")
	purego.RegisterLibFunc(&fAvGetPictureTypeChar, h, "av_get_picture_type_char")
	purego.RegisterLibFunc(&fAvGetProfileName, h, "av_get_profile_name")
	purego.RegisterLibFunc(&fAvGetRandomSeed, h, "av_get_random_seed")
	purego.RegisterLibFunc(&fAvGetTimeBaseQ, h, "av_get_time_base_q")
	purego.RegisterLibFunc(&fAvGetToken, h, "av_get_token")
	purego.RegisterLibFunc(&fAvHashAlloc, h, "av_hash_alloc")
	purego.RegisterLibFunc(&fAvHashFinal, h, "av_hash_final")
	purego.RegisterLibFunc(&fAvHashFinalB64, h, "av_hash_final_b64")
	purego.RegisterLibFunc(&fAvHashFinalBin, h, "av_hash_final_bin")
	purego.RegisterLibFunc(&fAvHashFinalHex, h, "av_hash_final_hex")
	purego.RegisterLibFunc(&fAvHashFreep, h, "av_hash_freep")
	purego.RegisterLibFunc(&fAvHashGetName, h, "av_hash_get_name")
	purego.RegisterLibFunc(&fAvHashGetSize, h, "av_hash_get_size")
	purego.RegisterLibFunc(&fAvHashInit, h, "av_hash_init")
	purego.RegisterLibFunc(&fAvHashNames, h, "av_hash_names")
	purego.RegisterLibFunc(&fAvHashUpdate, h, "av_hash_update")
	purego.RegisterLibFunc(&fAvHexDump, h, "av_hex_dump")
	purego.RegisterLibFunc(&fAvHexDumpLog, h, "av_hex_dump_log")
	purego.RegisterLibFunc(&fAvHmacAlloc, h, "av_hmac_alloc")
	purego.RegisterLibFunc(&fAvHmacCalc, h, "av_hmac_calc")
	purego.RegisterLibFunc(&fAvHmacFinal, h, "av_hmac_final")
	purego.RegisterLibFunc(&fAvHmacFree, h, "av_hmac_free")
	purego.RegisterLibFunc(&fAvHmacInit, h, "av_hmac_init")
	purego.RegisterLibFunc(&fAvHmacUpdate, h, "av_hmac_update")
	purego.RegisterLibFunc(&fAvHwdeviceCtxAlloc, h, "av_hwdevice_ctx_alloc")
	purego.RegisterLibFunc(&fAvHwdeviceCtxCreate, h, "av_hwdevice_ctx_create")
	purego.RegisterLibFunc(&fAvHwdeviceCtxCreateDerived, h, "av_hwdevice_ctx_create_derived")
	purego.RegisterLibFunc(&fAvHwdeviceCtxCreateDerivedOpts, h, "av_hwdevice_ctx_create_derived_opts")
	purego.RegisterLibFunc(&fAvHwdeviceCtxInit, h, "av_hwdevice_ctx_init")
	purego.RegisterLibFunc(&fAvHwdeviceFindTypeByName, h, "av_hwdevice_find_type_by_name")
	purego.RegisterLibFunc(&fAvHwdeviceGetHwframeConstraints, h, "av_hwdevice_get_hwframe_constraints")
	purego.RegisterLibFunc(&fAvHwdeviceGetTypeName, h, "av_hwdevice_get_type_name")
	purego.RegisterLibFunc(&fAvHwdeviceHwconfigAlloc, h, "av_hwdevice_hwconfig_alloc")
	purego.RegisterLibFunc(&fAvHwdeviceIterateTypes, h, "av_hwdevice_iterate_types")
	purego.RegisterLibFunc(&fAvHwframeConstraintsFree, h, "av_hwframe_constraints_free")
	purego.RegisterLibFunc(&fAvHwframeCtxAlloc, h, "av_hwframe_ctx_alloc")
	purego.RegisterLibFunc(&fAvHwframeCtxCreateDerived, h, "av_hwframe_ctx_create_derived")
	purego.RegisterLibFunc(&fAvHwframeCtxInit, h, "av_hwframe_ctx_init")
	purego.RegisterLibFunc(&fAvHwframeGetBuffer, h, "av_hwframe_get_buffer")
	purego.RegisterLibFunc(&fAvHwframeMap, h, "av_hwframe_map")
	purego.RegisterLibFunc(&fAvHwframeTransferData, h, "av_hwframe_transfer_data")
	purego.RegisterLibFunc(&fAvHwframeTransferGetFormats, h, "av_hwframe_transfer_get_formats")
	purego.RegisterLibFunc(&fAvI2int, h, "av_i2int")
	purego.RegisterLibFunc(&fAvIamfAudioElementAddLayer, h, "av_iamf_audio_element_add_layer")
	purego.RegisterLibFunc(&fAvIamfAudioElementAlloc, h, "av_iamf_audio_element_alloc")
	purego.RegisterLibFunc(&fAvIamfAudioElementFree, h, "av_iamf_audio_element_free")
	purego.RegisterLibFunc(&fAvIamfAudioElementGetClass, h, "av_iamf_audio_element_get_class")
	purego.RegisterLibFunc(&fAvIamfMixPresentationAddSubmix, h, "av_iamf_mix_presentation_add_submix")
	purego.RegisterLibFunc(&fAvIamfMixPresentationAlloc, h, "av_iamf_mix_presentation_alloc")
	purego.RegisterLibFunc(&fAvIamfMixPresentationFree, h, "av_iamf_mix_presentation_free")
	purego.RegisterLibFunc(&fAvIamfMixPresentationGetClass, h, "av_iamf_mix_presentation_get_class")
	purego.RegisterLibFunc(&fAvIamfParamDefinitionAlloc, h, "av_iamf_param_definition_alloc")
	purego.RegisterLibFunc(&fAvIamfParamDefinitionGetClass, h, "av_iamf_param_definition_get_class")
	purego.RegisterLibFunc(&fAvIamfSubmixAddElement, h, "av_iamf_submix_add_element")
	purego.RegisterLibFunc(&fAvIamfSubmixAddLayout, h, "av_iamf_submix_add_layout")
	purego.RegisterLibFunc(&fAvImdctCalc, h, "av_imdct_calc")
	purego.RegisterLibFunc(&fAvImdctHalf, h, "av_imdct_half")
	purego.RegisterLibFunc(&fAvInitPacket, h, "av_init_packet")
	purego.RegisterLibFunc(&fAvInputAudioDeviceNext, h, "av_input_audio_device_next")
	purego.RegisterLibFunc(&fAvInputVideoDeviceNext, h, "av_input_video_device_next")
	purego.RegisterLibFunc(&fAvInt2i, h, "av_int2i")
	purego.RegisterLibFunc(&fAvIntListLengthForSize, h, "av_int_list_length_for_size")
	purego.RegisterLibFunc(&fAvInterleavedWriteFrame, h, "av_interleaved_write_frame")
	purego.RegisterLibFunc(&fAvInterleavedWriteUncodedFrame, h, "av_interleaved_write_uncoded_frame")
	purego.RegisterLibFunc(&fAvJniGetJavaVm, h, "av_jni_get_java_vm")
	purego.RegisterLibFunc(&fAvJniSetJavaVm, h, "av_jni_set_java_vm")
	purego.RegisterLibFunc(&fAvLfgInit, h, "av_lfg_init")
	purego.RegisterLibFunc(&fAvLfgInitFromData, h, "av_lfg_init_from_data")
	purego.RegisterLibFunc(&fAvLog, h, "av_log")
	purego.RegisterLibFunc(&fAvLzo1xDecode, h, "av_lzo1x_decode")
	purego.RegisterLibFunc(&fAvMatchExt, h, "av_match_ext")
	purego.RegisterLibFunc(&fAvMatchList, h, "av_match_list")
	purego.RegisterLibFunc(&fAvMatchName, h, "av_match_name")
	purego.RegisterLibFunc(&fAvMaxAlloc, h, "av_max_alloc")
	purego.RegisterLibFunc(&fAvMd5Alloc, h, "av_md5_alloc")
	purego.RegisterLibFunc(&fAvMd5Final, h, "av_md5_final")
	purego.RegisterLibFunc(&fAvMd5Init, h, "av_md5_init")
	purego.RegisterLibFunc(&fAvMd5Sum, h, "av_md5_sum")
	purego.RegisterLibFunc(&fAvMd5Update, h, "av_md5_update")
	purego.RegisterLibFunc(&fAvMdctCalc, h, "av_mdct_calc")
	purego.RegisterLibFunc(&fAvMdctEnd, h, "av_mdct_end")
	purego.RegisterLibFunc(&fAvMdctInit, h, "av_mdct_init")
	purego.RegisterLibFunc(&fAvMediacodecAllocContext, h, "av_mediacodec_alloc_context")
	purego.RegisterLibFunc(&fAvMediacodecDefaultFree, h, "av_mediacodec_default_free")
	purego.RegisterLibFunc(&fAvMediacodecDefaultInit, h, "av_mediacodec_default_init")
	purego.RegisterLibFunc(&fAvMediacodecReleaseBuffer, h, "av_mediacodec_release_buffer")
	purego.RegisterLibFunc(&fAvMediacodecRenderBufferAtTime, h, "av_mediacodec_render_buffer_at_time")
	purego.RegisterLibFunc(&fAvModI, h, "av_mod_i")
	purego.RegisterLibFunc(&fAvMulI, h, "av_mul_i")
	purego.RegisterLibFunc(&fAvMulQ, h, "av_mul_q")
	purego.RegisterLibFunc(&fAvMurmur3Alloc, h, "av_murmur3_alloc")
	purego.RegisterLibFunc(&fAvMurmur3Final, h, "av_murmur3_final")
	purego.RegisterLibFunc(&fAvMurmur3Init, h, "av_murmur3_init")
	purego.RegisterLibFunc(&fAvMurmur3InitSeeded, h, "av_murmur3_init_seeded")
	purego.RegisterLibFunc(&fAvMurmur3Update, h, "av_murmur3_update")
	purego.RegisterLibFunc(&fAvMuxerIterate, h, "av_muxer_iterate")
	purego.RegisterLibFunc(&fAvNearerQ, h, "av_nearer_q")
	purego.RegisterLibFunc(&fAvNewProgram, h, "av_new_program")
	purego.RegisterLibFunc(&fAvOutputAudioDeviceNext, h, "av_output_audio_device_next")
	purego.RegisterLibFunc(&fAvOutputVideoDeviceNext, h, "av_output_video_device_next")
	purego.RegisterLibFunc(&fAvPixelutilsGetSadFn, h, "av_pixelutils_get_sad_fn")
	purego.RegisterLibFunc(&fAvPktDump2, h, "av_pkt_dump2")
	purego.RegisterLibFunc(&fAvPktDumpLog2, h, "av_pkt_dump_log2")
	purego.RegisterLibFunc(&fAvProbeInputBuffer, h, "av_probe_input_buffer")
	purego.RegisterLibFunc(&fAvProbeInputBuffer2, h, "av_probe_input_buffer2")
	purego.RegisterLibFunc(&fAvProbeInputFormat, h, "av_probe_input_format")
	purego.RegisterLibFunc(&fAvProbeInputFormat2, h, "av_probe_input_format2")
	purego.RegisterLibFunc(&fAvProbeInputFormat3, h, "av_probe_input_format3")
	purego.RegisterLibFunc(&fAvProgramAddStreamIndex, h, "av_program_add_stream_index")
	purego.RegisterLibFunc(&fAvQ2intfloat, h, "av_q2intfloat")
	purego.RegisterLibFunc(&fAvQsvAllocContext, h, "av_qsv_alloc_context")
	purego.RegisterLibFunc(&fAvRandomBytes, h, "av_random_bytes")
	purego.RegisterLibFunc(&fAvRc4Alloc, h, "av_rc4_alloc")
	purego.RegisterLibFunc(&fAvRc4Crypt, h, "av_rc4_crypt")
	purego.RegisterLibFunc(&fAvRc4Init, h, "av_rc4_init")
	purego.RegisterLibFunc(&fAvRdftCalc, h, "av_rdft_calc")
	purego.RegisterLibFunc(&fAvRdftEnd, h, "av_rdft_end")
	purego.RegisterLibFunc(&fAvRdftInit, h, "av_rdft_init")
	purego.RegisterLibFunc(&fAvReadImageLine, h, "av_read_image_line")
	purego.RegisterLibFunc(&fAvReadImageLine2, h, "av_read_image_line2")
	purego.RegisterLibFunc(&fAvReadPause, h, "av_read_pause")
	purego.RegisterLibFunc(&fAvReadPlay, h, "av_read_play")
	purego.RegisterLibFunc(&fAvReduce, h, "av_reduce")
	purego.RegisterLibFunc(&fAvRipemdAlloc, h, "av_ripemd_alloc")
	purego.RegisterLibFunc(&fAvRipemdFinal, h, "av_ripemd_final")
	purego.RegisterLibFunc(&fAvRipemdInit, h, "av_ripemd_init")
	purego.RegisterLibFunc(&fAvRipemdUpdate, h, "av_ripemd_update")
	purego.RegisterLibFunc(&fAvSamplesAlloc, h, "av_samples_alloc")
	purego.RegisterLibFunc(&fAvSamplesAllocArrayAndSamples, h, "av_samples_alloc_array_and_samples")
	purego.RegisterLibFunc(&fAvSamplesCopy, h, "av_samples_copy")
	purego.RegisterLibFunc(&fAvSamplesFillArrays, h, "av_samples_fill_arrays")
	purego.RegisterLibFunc(&fAvSamplesGetBufferSize, h, "av_samples_get_buffer_size")
	purego.RegisterLibFunc(&fAvSamplesSetSilence, h, "av_samples_set_silence")
	purego.RegisterLibFunc(&fAvSdpCreate, h, "av_sdp_create")
	purego.RegisterLibFunc(&fAvSetOptionsString, h, "av_set_options_string")
	purego.RegisterLibFunc(&fAvSha512Alloc, h, "av_sha512_alloc")
	purego.RegisterLibFunc(&fAvSha512Final, h, "av_sha512_final")
	purego.RegisterLibFunc(&fAvSha512Init, h, "av_sha512_init")
	purego.RegisterLibFunc(&fAvSha512Update, h, "av_sha512_update")
	purego.RegisterLibFunc(&fAvShaAlloc, h, "av_sha_alloc")
	purego.RegisterLibFunc(&fAvShaFinal, h, "av_sha_final")
	purego.RegisterLibFunc(&fAvShaInit, h, "av_sha_init")
	purego.RegisterLibFunc(&fAvShaUpdate, h, "av_sha_update")
	purego.RegisterLibFunc(&fAvShrI, h, "av_shr_i")
	purego.RegisterLibFunc(&fAvSizeMult, h, "av_size_mult")
	purego.RegisterLibFunc(&fAvSmallStrptime, h, "av_small_strptime")
	purego.RegisterLibFunc(&fAvSscanf, h, "av_sscanf")
	purego.RegisterLibFunc(&fAvStrcasecmp, h, "av_strcasecmp")
	purego.RegisterLibFunc(&fAvStrdup, h, "av_strdup")
	purego.RegisterLibFunc(&fAvStrndup, h, "av_strndup")
	purego.RegisterLibFunc(&fAvStreamAddSideData, h, "av_stream_add_side_data")
	purego.RegisterLibFunc(&fAvStreamGetClass, h, "av_stream_get_class")
	purego.RegisterLibFunc(&fAvStreamGetCodecTimebase, h, "av_stream_get_codec_timebase")
	purego.RegisterLibFunc(&fAvStreamGetParser, h, "av_stream_get_parser")
	purego.RegisterLibFunc(&fAvStreamGetSideData, h, "av_stream_get_side_data")
	purego.RegisterLibFunc(&fAvStreamGroupGetClass, h, "av_stream_group_get_class")
	purego.RegisterLibFunc(&fAvStreamNewSideData, h, "av_stream_new_side_data")
	purego.RegisterLibFunc(&fAvStrireplace, h, "av_strireplace")
	purego.RegisterLibFunc(&fAvStristart, h, "av_stristart")
	purego.RegisterLibFunc(&fAvStristr, h, "av_stristr")
	purego.RegisterLibFunc(&fAvStrlcat, h, "av_strlcat")
	purego.RegisterLibFunc(&fAvStrlcpy, h, "av_strlcpy")
	purego.RegisterLibFunc(&fAvStrncasecmp, h, "av_strncasecmp")
	purego.RegisterLibFunc(&fAvStrnstr, h, "av_strnstr")
	purego.RegisterLibFunc(&fAvStrstart, h, "av_strstart")
	purego.RegisterLibFunc(&fAvStrtod, h, "av_strtod")
	purego.RegisterLibFunc(&fAvStrtok, h, "av_strtok")
	purego.RegisterLibFunc(&fAvSubI, h, "av_sub_i")
	purego.RegisterLibFunc(&fAvSubQ, h, "av_sub_q")
	purego.RegisterLibFunc(&fAvTeaAlloc, h, "av_tea_alloc")
	purego.RegisterLibFunc(&fAvTeaCrypt, h, "av_tea_crypt")
	purego.RegisterLibFunc(&fAvTeaInit, h, "av_tea_init")
	purego.RegisterLibFunc(&fAvThreadMessageFlush, h, "av_thread_message_flush")
	purego.RegisterLibFunc(&fAvThreadMessageQueueAlloc, h, "av_thread_message_queue_alloc")
	purego.RegisterLibFunc(&fAvThreadMessageQueueFree, h, "av_thread_message_queue_free")
	purego.RegisterLibFunc(&fAvThreadMessageQueueNbElems, h, "av_thread_message_queue_nb_elems")
	purego.RegisterLibFunc(&fAvThreadMessageQueueRecv, h, "av_thread_message_queue_recv")
	purego.RegisterLibFunc(&fAvThreadMessageQueueSend, h, "av_thread_message_queue_send")
	purego.RegisterLibFunc(&fAvThreadMessageQueueSetErrRecv, h, "av_thread_message_queue_set_err_recv")
	purego.RegisterLibFunc(&fAvThreadMessageQueueSetErrSend, h, "av_thread_message_queue_set_err_send")
	purego.RegisterLibFunc(&fAvThreadMessageQueueSetFreeFunc, h, "av_thread_message_queue_set_free_func")
	purego.RegisterLibFunc(&fAvTimegm, h, "av_timegm")
	purego.RegisterLibFunc(&fAvTreeDestroy, h, "av_tree_destroy")
	purego.RegisterLibFunc(&fAvTreeEnumerate, h, "av_tree_enumerate")
	purego.RegisterLibFunc(&fAvTreeFind, h, "av_tree_find")
	purego.RegisterLibFunc(&fAvTreeInsert, h, "av_tree_insert")
	purego.RegisterLibFunc(&fAvTreeNodeAlloc, h, "av_tree_node_alloc")
	purego.RegisterLibFunc(&fAvTsMakeTimeString2, h, "av_ts_make_time_string2")
	purego.RegisterLibFunc(&fAvTwofishAlloc, h, "av_twofish_alloc")
	purego.RegisterLibFunc(&fAvTwofishCrypt, h, "av_twofish_crypt")
	purego.RegisterLibFunc(&fAvTwofishInit, h, "av_twofish_init")
	purego.RegisterLibFunc(&fAvTxInit, h, "av_tx_init")
	purego.RegisterLibFunc(&fAvTxUninit, h, "av_tx_uninit")
	purego.RegisterLibFunc(&fAvUtf8Decode, h, "av_utf8_decode")
	purego.RegisterLibFunc(&fAvUuidParse, h, "av_uuid_parse")
	purego.RegisterLibFunc(&fAvUuidParseRange, h, "av_uuid_parse_range")
	purego.RegisterLibFunc(&fAvUuidUnparse, h, "av_uuid_unparse")
	purego.RegisterLibFunc(&fAvUuidUrnParse, h, "av_uuid_urn_parse")
	purego.RegisterLibFunc(&fAvVbprintf, h, "av_vbprintf")
	purego.RegisterLibFunc(&fAvVideoEncParamsAlloc, h, "av_video_enc_params_alloc")
	purego.RegisterLibFunc(&fAvVideoEncParamsCreateSideData, h, "av_video_enc_params_create_side_data")
	purego.RegisterLibFunc(&fAvVideoHintAlloc, h, "av_video_hint_alloc")
	purego.RegisterLibFunc(&fAvVideoHintCreateSideData, h, "av_video_hint_create_side_data")
	purego.RegisterLibFunc(&fAvVkfmtFromPixfmt, h, "av_vkfmt_from_pixfmt")
	purego.RegisterLibFunc(&fAvVkFrameAlloc, h, "av_vk_frame_alloc")
	purego.RegisterLibFunc(&fAvVlog, h, "av_vlog")
	purego.RegisterLibFunc(&fAvVorbisParseFrame, h, "av_vorbis_parse_frame")
	purego.RegisterLibFunc(&fAvVorbisParseFrameFlags, h, "av_vorbis_parse_frame_flags")
	purego.RegisterLibFunc(&fAvVorbisParseFree, h, "av_vorbis_parse_free")
	purego.RegisterLibFunc(&fAvVorbisParseInit, h, "av_vorbis_parse_init")
	purego.RegisterLibFunc(&fAvVorbisParseReset, h, "av_vorbis_parse_reset")
	purego.RegisterLibFunc(&fAvWriteFrame, h, "av_write_frame")
	purego.RegisterLibFunc(&fAvWriteImageLine, h, "av_write_image_line")
	purego.RegisterLibFunc(&fAvWriteImageLine2, h, "av_write_image_line2")
	purego.RegisterLibFunc(&fAvWriteTrailer, h, "av_write_trailer")
	purego.RegisterLibFunc(&fAvWriteUncodedFrame, h, "av_write_uncoded_frame")
	purego.RegisterLibFunc(&fAvWriteUncodedFrameQuery, h, "av_write_uncoded_frame_query")
	purego.RegisterLibFunc(&fAvXiphlacing, h, "av_xiphlacing")
	purego.RegisterLibFunc(&fAvXteaAlloc, h, "av_xtea_alloc")
	purego.RegisterLibFunc(&fAvXteaCrypt, h, "av_xtea_crypt")
	purego.RegisterLibFunc(&fAvXteaInit, h, "av_xtea_init")
	purego.RegisterLibFunc(&fAvXteaLeCrypt, h, "av_xtea_le_crypt")
	purego.RegisterLibFunc(&fAvXteaLeInit, h, "av_xtea_le_init")
	purego.RegisterLibFunc(&fSwresampleConfiguration, h, "swresample_configuration")
	purego.RegisterLibFunc(&fSwresampleLicense, h, "swresample_license")
	purego.RegisterLibFunc(&fSwresampleVersion, h, "swresample_version")
	purego.RegisterLibFunc(&fSwriAudioConvert, h, "swri_audio_convert")
	purego.RegisterLibFunc(&fSwriAudioConvertAlloc, h, "swri_audio_convert_alloc")
	purego.RegisterLibFunc(&fSwriAudioConvertFree, h, "swri_audio_convert_free")
	purego.RegisterLibFunc(&fSwriResampleDspInit, h, "swri_resample_dsp_init")
	purego.RegisterLibFunc(&fAvAddI, h, "av_add_i")
	purego.RegisterLibFunc(&fAvDynamicHdrVividAlloc, h, "av_dynamic_hdr_vivid_alloc")
	purego.RegisterLibFunc(&fAvDynamicHdrVividCreateSideData, h, "av_dynamic_hdr_vivid_create_side_data")
	purego.RegisterLibFunc(&fAvformatTransferInternalStreamTimingInfo, h, "avformat_transfer_internal_stream_timing_info")
	purego.RegisterLibFunc(&fAvImageCopy, h, "av_image_copy")
	purego.RegisterLibFunc(&fAvImageCopyPlaneUcFrom, h, "av_image_copy_plane_uc_from")
	purego.RegisterLibFunc(&fAvImageCopyToBuffer, h, "av_image_copy_to_buffer")
	purego.RegisterLibFunc(&fAvImageCopyUcFrom, h, "av_image_copy_uc_from")
	purego.RegisterLibFunc(&fAvImageFillArrays, h, "av_image_fill_arrays")
	purego.RegisterLibFunc(&fAvImageFillBlack, h, "av_image_fill_black")
	purego.RegisterLibFunc(&fAvImageFillColor, h, "av_image_fill_color")
	purego.RegisterLibFunc(&fAvImageFillLinesizes, h, "av_image_fill_linesizes")
	purego.RegisterLibFunc(&fAvImageFillMaxPixsteps, h, "av_image_fill_max_pixsteps")
	purego.RegisterLibFunc(&fAvImageFillPlaneSizes, h, "av_image_fill_plane_sizes")
	purego.RegisterLibFunc(&fAvImageFillPointers, h, "av_image_fill_pointers")
	purego.RegisterLibFunc(&fAvImageGetLinesize, h, "av_image_get_linesize")
	purego.RegisterLibFunc(&fAvLog2, h, "av_log2")
	purego.RegisterLibFunc(&fAvLog216bit, h, "av_log2_16bit")
	purego.RegisterLibFunc(&fAvLog2I, h, "av_log2_i")
	purego.RegisterLibFunc(&fAvParseCpuCaps, h, "av_parse_cpu_caps")
	purego.RegisterLibFunc(&fAvPixFmtCountPlanes, h, "av_pix_fmt_count_planes")
	purego.RegisterLibFunc(&fAvPixFmtGetChromaSubSample, h, "av_pix_fmt_get_chroma_sub_sample")
	purego.RegisterLibFunc(&fAvPixFmtSwapEndianness, h, "av_pix_fmt_swap_endianness")
}

// Ac3ParseHeader 解析 AC3 帧头拿帧长（对 av_ac3_parse_header；参数 buf、size、bitstream_id、frame_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Ac3ParseHeader(buf unsafe.Pointer, size uintptr, bitstream_id unsafe.Pointer, frame_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAc3ParseHeader(buf, size, bitstream_id, frame_size)
}

// AdtsHeaderParse 解析 ADTS 头拿采样数（对 av_adts_header_parse；参数 buf、samples、frames；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) AdtsHeaderParse(buf unsafe.Pointer, samples unsafe.Pointer, frames unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAdtsHeaderParse(buf, samples, frames)
}

// AesAlloc AES 分组加解密的小件（对 av_aes_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) AesAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAesAlloc()
}

// AesCrypt AES 分组加解密的小件（对 av_aes_crypt；参数 a、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) AesCrypt(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvAesCrypt(a, dst, src, count, iv, decrypt)
}

// AesCtrAlloc AES-CTR 流模式加解密的小件（对 av_aes_ctr_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) AesCtrAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAesCtrAlloc()
}

// AesCtrCrypt AES-CTR 流模式加解密的小件（对 av_aes_ctr_crypt；参数 a、dst、src、size；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrCrypt(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvAesCtrCrypt(a, dst, src, size)
}

// AesCtrFree AES-CTR 流模式加解密的小件（对 av_aes_ctr_free；参数 a；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrFree(a unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvAesCtrFree(a)
}

// AesCtrGetIv AES-CTR 流模式加解密的小件（对 av_aes_ctr_get_iv；参数 a；回 C 指针，失败回 nil；无状态调用）。
func (self *Crypto) AesCtrGetIv(a unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAesCtrGetIv(a)
}

// AesCtrIncrementIv AES-CTR 流模式加解密的小件（对 av_aes_ctr_increment_iv；参数 a；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrIncrementIv(a unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvAesCtrIncrementIv(a)
}

// AesCtrInit AES-CTR 流模式加解密的小件（对 av_aes_ctr_init；参数 a、key；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) AesCtrInit(a unsafe.Pointer, key unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvAesCtrInit(a, key); ret < 0 {
		return codeErr("av_aes_ctr_init", ret)
	}
	return nil
}

// AesCtrSetFullIv AES-CTR 流模式加解密的小件（对 av_aes_ctr_set_full_iv；参数 a、iv；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrSetFullIv(a unsafe.Pointer, iv unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvAesCtrSetFullIv(a, iv)
}

// AesCtrSetIv AES-CTR 流模式加解密的小件（对 av_aes_ctr_set_iv；参数 a、iv；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrSetIv(a unsafe.Pointer, iv unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvAesCtrSetIv(a, iv)
}

// AesCtrSetRandomIv AES-CTR 流模式加解密的小件（对 av_aes_ctr_set_random_iv；参数 a；按签名取回值；无状态调用）。
func (self *Crypto) AesCtrSetRandomIv(a unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvAesCtrSetRandomIv(a)
}

// AesInit AES 分组加解密的小件（对 av_aes_init；参数 a、key、key_bits、decrypt；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) AesInit(a unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvAesInit(a, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_aes_init", ret)
	}
	return nil
}

// AllocVdpaucontext 新建 VDPAU 硬解上下文，后面绑解码器用（对 av_alloc_vdpaucontext；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) AllocVdpaucontext() unsafe.Pointer {
	mustUse(ensureModCryptoHw())
	return fAvAllocVdpaucontext()
}

// AppendPacket 把新读到的数据追加到包尾巴上（对 av_append_packet；参数 s、pkt、size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) AppendPacket(s unsafe.Pointer, pkt unsafe.Pointer, size int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvAppendPacket(s, pkt, size); ret < 0 {
		return codeErr("av_append_packet", ret)
	}
	return nil
}

// AppendPathComponent 把一段路径拼到路径尾巴上，自动补斜杠（对 av_append_path_component；参数 path、component；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) AppendPathComponent(path unsafe.Pointer, component unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvAppendPathComponent(path, component)
}

// Assert0Fpu 断言浮点单元状态正常，排查看用（对 av_assert0_fpu；无参数无回值；错了直接崩进程，只在可疑时调）。
func (self *Util) Assert0Fpu() {
	mustUse(ensureModCrypto())
	fAvAssert0Fpu()
}

// Base64Decode 把 Base64 字符串解回原始字节（对 av_base64_decode；参数 out、in、out_size；回解出的字节数，负数是出错码；in 传 nil 会崩，out 得事先备好够大的地方）。
func (self *Crypto) Base64Decode(out unsafe.Pointer, in unsafe.Pointer, out_size int32) int32 {
	mustUse(ensureModCrypto())
	return fAvBase64Decode(out, in, out_size)
}

// Base64Encode 把原始字节编成 Base64 字符串（对 av_base64_encode；参数 out、out_size、in、in_size；回 C 指针，失败回 nil；无状态调用）。
func (self *Crypto) Base64Encode(out unsafe.Pointer, out_size int32, in unsafe.Pointer, in_size int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvBase64Encode(out, out_size, in, in_size)
}

// Basename 取路径里的文件名（去目录部分）（对 av_basename；参数 path；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) Basename(path unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvBasename(path)
}

// BesselI0 算零阶贝塞尔函数值，滤波器设计用（对 av_bessel_i0；参数 x；回浮点数；无状态，可用零值直接调）。
func (self *Util) BesselI0(x float64) float64 {
	mustUse(ensureModCrypto())
	return fAvBesselI0(x)
}

// BlowfishAlloc Blowfish 加解密的小件（对 av_blowfish_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) BlowfishAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvBlowfishAlloc()
}

// BlowfishCrypt Blowfish 加解密的小件（对 av_blowfish_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) BlowfishCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvBlowfishCrypt(ctx, dst, src, count, iv, decrypt)
}

// BlowfishCryptEcb Blowfish 加解密的小件（对 av_blowfish_crypt_ecb；参数 ctx、xl、xr、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) BlowfishCryptEcb(ctx unsafe.Pointer, xl unsafe.Pointer, xr unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvBlowfishCryptEcb(ctx, xl, xr, decrypt)
}

// BlowfishInit Blowfish 设密钥（对 av_blowfish_init；参数 ctx、key、key_len；key_len 是字节数不是位数（16 字节钥匙就传 16，传 128 会读出界）；ctx/key 传 nil 会崩，得传真对象）。
func (self *Crypto) BlowfishInit(ctx unsafe.Pointer, key unsafe.Pointer, key_len int32) {
	mustUse(ensureModCrypto())
	fAvBlowfishInit(ctx, key, key_len)
}

// BmgGet 伯努利高斯模型取数，噪声建模用（对 av_bmg_get；参数 lfg、out；按签名取回值；无状态，可用零值直接调）。
func (self *Util) BmgGet(lfg unsafe.Pointer, out float64) {
	mustUse(ensureModCrypto())
	fAvBmgGet(lfg, out)
}

// Bprintf 往打印缓冲里追加格式化字符串（对 av_bprintf；参数 buf、fmt，后面跟的变参 purego 传不准所以没接，只能干拼好的无百分号字串；变参版走 variadic_go.go 的拼串路；buf 传 nil 会崩，得传真 BPrint）。
func (self *Util) Bprintf(buf unsafe.Pointer, fmt unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvBprintf(buf, fmt)
}

// Calloc 按元素个数清零分配内存（对 av_calloc；参数 nmemb、size；回新内存块，申请不到回 nil，用完拿 Mem.Free 放；无状态，可用零值直接调）。
func (self *Util) Calloc(nmemb uintptr, size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCalloc(nmemb, size)
}

// CamelliaAlloc Camellia 加解密的小件（对 av_camellia_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) CamelliaAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCamelliaAlloc()
}

// CamelliaCrypt Camellia 加解密的小件（对 av_camellia_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) CamelliaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvCamelliaCrypt(ctx, dst, src, count, iv, decrypt)
}

// CamelliaInit Camellia 加解密的小件（对 av_camellia_init；参数 ctx、key、key_bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) CamelliaInit(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvCamelliaInit(ctx, key, key_bits); ret < 0 {
		return codeErr("av_camellia_init", ret)
	}
	return nil
}

// Cast5Alloc CAST5 加解密的小件（对 av_cast5_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) Cast5Alloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCast5Alloc()
}

// Cast5Crypt CAST5 加解密的小件（对 av_cast5_crypt；参数 ctx、dst、src、count、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) Cast5Crypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvCast5Crypt(ctx, dst, src, count, decrypt)
}

// Cast5Crypt2 CAST5 加解密的小件（对 av_cast5_crypt2；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) Cast5Crypt2(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvCast5Crypt2(ctx, dst, src, count, iv, decrypt)
}

// Cast5Init CAST5 加解密的小件（对 av_cast5_init；参数 ctx、key、key_bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) Cast5Init(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvCast5Init(ctx, key, key_bits); ret < 0 {
		return codeErr("av_cast5_init", ret)
	}
	return nil
}

// ChromaLocationEnumToPos 把色度位置枚举换算成横竖偏移（对 av_chroma_location_enum_to_pos；参数 xpos、ypos、pos；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ChromaLocationEnumToPos(xpos *int32, ypos *int32, pos int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvChromaLocationEnumToPos(xpos, ypos, pos); ret < 0 {
		return codeErr("av_chroma_location_enum_to_pos", ret)
	}
	return nil
}

// ChromaLocationFromName 按名字找色度位置编号（对 av_chroma_location_from_name；参数 name；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ChromaLocationFromName(name string) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvChromaLocationFromName(name); ret < 0 {
		return codeErr("av_chroma_location_from_name", ret)
	}
	return nil
}

// ChromaLocationName 按编号查色度位置名字（对 av_chroma_location_name；参数 location；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) ChromaLocationName(location int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvChromaLocationName(location)
}

// ChromaLocationPosToEnum 把横竖偏移换算回色度位置枚举（对 av_chroma_location_pos_to_enum；参数 xpos、ypos；回数值；无状态，可用零值直接调）。
func (self *Util) ChromaLocationPosToEnum(xpos int32, ypos int32) int32 {
	mustUse(ensureModCrypto())
	return fAvChromaLocationPosToEnum(xpos, ypos)
}

// CmpI compares two integers, -1/0/1 (av_cmp_i takes AVInteger BY VALUE;
// pass Int2i(n) results, never pointers).
func (self *Util) CmpI(a AVInteger, b AVInteger) int32 {
	mustUse(ensureModCrypto())
	return fAvCmpI(a, b)
}

// CpbPropertiesAlloc 码率平滑缓冲属性操作（对 av_cpb_properties_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) CpbPropertiesAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCpbPropertiesAlloc(size)
}

// CrcGetTable 取 ffmpeg 内置的标准 CRC 表，不用自己初始化（对 av_crc_get_table；参数 crc_id（枚举数，如 3=AV_CRC_32_IEEE）；回静态表借用不释放；无状态调用）。
func (self *Crypto) CrcGetTable(crc_id int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCrcGetTable(crc_id)
}

// CrcInit 初始化一套 CRC 表，后面算校验用（对 av_crc_init；参数 ctx、le、bits、poly、ctx_size；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) CrcInit(ctx unsafe.Pointer, le int32, bits int32, poly uint32, ctx_size int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvCrcInit(ctx, le, bits, poly, ctx_size); ret < 0 {
		return codeErr("av_crc_init", ret)
	}
	return nil
}

// Adler32Update extends an Adler-32 checksum over ln bytes at buf
// (pass the previous result back in as adler to chain buffers).
func (self *Crypto) Adler32Update(adler uint32, buf unsafe.Pointer, ln uintptr) uint32 {
	mustUse(ensureModCrypto())
	return fAvAdler32Update(adler, buf, ln)
}

// Crc updates a CRC checksum over ln bytes at buf using a table
// prepared by CrcInit (pass the previous result back in as crc to chain).
func (self *Crypto) Crc(ctx unsafe.Pointer, crc uint32, buf unsafe.Pointer, ln uintptr) uint32 {
	mustUse(ensureModCrypto())
	return fAvCrc(ctx, crc, buf, ln)
}

// CspApproximateTrcGamma 颜色空间系数查询（对 av_csp_approximate_trc_gamma；参数 trc（传输特性枚举数）；回浮点数；无状态，可用零值直接调）。
func (self *Util) CspApproximateTrcGamma(trc int32) float64 {
	mustUse(ensureModCrypto())
	return fAvCspApproximateTrcGamma(trc)
}

// CspLumaCoeffsFromAvcsp 颜色空间系数查询（对 av_csp_luma_coeffs_from_avcsp；参数 csp（颜色空间枚举数）；回静态表借用不释放；无状态，可用零值直接调）。
func (self *Util) CspLumaCoeffsFromAvcsp(csp int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCspLumaCoeffsFromAvcsp(csp)
}

// CspPrimariesDescFromId 颜色空间系数查询（对 av_csp_primaries_desc_from_id；参数 prm（色原色枚举数）；回静态表借用不释放；无状态，可用零值直接调）。
func (self *Util) CspPrimariesDescFromId(prm int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCspPrimariesDescFromId(prm)
}

// CspPrimariesIdFromDesc 色原色描述反查枚举（对 av_csp_primaries_id_from_desc；参数 prm（描述表指针）；回枚举数，对不上回 0（UNSPECIFIED）；无状态，可用零值直接调）。
func (self *Util) CspPrimariesIdFromDesc(prm unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvCspPrimariesIdFromDesc(prm)
}

// CspTrcFuncFromId 颜色空间系数查询（对 av_csp_trc_func_from_id；参数 trc（传输特性枚举数）；回静态函数指针不释放；无状态，可用零值直接调）。
func (self *Util) CspTrcFuncFromId(trc int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvCspTrcFuncFromId(trc)
}

// D2q 把浮点数转成最接近的分数，误差不超 max（对 av_d2q；参数 d、max；回分数（分子分母）；无状态，可用零值直接调）。
func (self *Util) D2q(d float64, max int32) AVRational {
	mustUse(ensureModCrypto())
	return fAvD2q(d, max)
}

// D3d11vaAllocContext Windows D3D11VA 硬解的小件（对 av_d3d11va_alloc_context；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) D3d11vaAllocContext() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvD3d11vaAllocContext()
}

// DctCalc 离散余弦变换小件（对 av_dct_calc；参数 s、data；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DctCalc(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDctCalc(s, data)
}

// DctEnd 离散余弦变换小件（对 av_dct_end；参数 s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DctEnd(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDctEnd(s)
}

// DctInit 离散余弦变换小件（对 av_dct_init；参数 nbits、typ；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DctInit(nbits int32, typ unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDctInit(nbits, typ)
}

// DefaultGetCategory 问选项上下文属于哪类（对 av_default_get_category；参数 ptr；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DefaultGetCategory(ptr unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDefaultGetCategory(ptr)
}

// DefaultItemName 问选项条目的名字（对 av_default_item_name；参数 ctx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DefaultItemName(ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDefaultItemName(ctx)
}

// DemuxerIterate 逐个列出支持的解复用器（对 av_demuxer_iterate；参数 opaque；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DemuxerIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDemuxerIterate(opaque)
}

// DesAlloc DES 加解密的小件（对 av_des_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) DesAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDesAlloc()
}

// DesCrypt DES 加解密的小件（对 av_des_crypt；参数 d、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) DesCrypt(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvDesCrypt(d, dst, src, count, iv, decrypt)
}

// DesInit DES 加解密的小件（对 av_des_init；参数 d、key、key_bits、decrypt；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) DesInit(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvDesInit(d, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_des_init", ret)
	}
	return nil
}

// DesMac DES 加解密的小件（对 av_des_mac；参数 d、dst、src、count；按签名取回值；无状态调用）。
func (self *Crypto) DesMac(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32) {
	mustUse(ensureModCrypto())
	fAvDesMac(d, dst, src, count)
}

// DetectionBboxAlloc 目标检测框元数据操作（对 av_detection_bbox_alloc；参数 nb_bboxes、out_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DetectionBboxAlloc(nb_bboxes uint32, out_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDetectionBboxAlloc(nb_bboxes, out_size)
}

// DetectionBboxCreateSideData 目标检测框元数据操作（对 av_detection_bbox_create_side_data；参数 frame、nb_bboxes；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DetectionBboxCreateSideData(frame unsafe.Pointer, nb_bboxes uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDetectionBboxCreateSideData(frame, nb_bboxes)
}

// DiracParseSequenceHeader 解析 Dirac 序列头（对 av_dirac_parse_sequence_header；参数 dsh、buf、buf_size、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) DiracParseSequenceHeader(dsh *unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr, log_ctx unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvDiracParseSequenceHeader(dsh, buf, buf_size, log_ctx); ret < 0 {
		return codeErr("av_dirac_parse_sequence_header", ret)
	}
	return nil
}

// Dirname 取路径里的目录部分（对 av_dirname；参数 path；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) Dirname(path unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDirname(path)
}

// DispositionFromString 把用途字符串换成用途掩码（对 av_disposition_from_string；参数 disp（如 "default"）；回掩码数，对不上回负错码；无状态，可用零值直接调）。
func (self *Util) DispositionFromString(disp string) int32 {
	mustUse(ensureModCrypto())
	return fAvDispositionFromString(disp)
}

// DispositionToString 把用途掩码换成人话字符串（对 av_disposition_to_string；参数 disposition；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DispositionToString(disposition int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDispositionToString(disposition)
}

// DivI divides two integers by value (av_div_i takes AVInteger BY VALUE).
func (self *Util) DivI(a AVInteger, b AVInteger) AVInteger {
	mustUse(ensureModCrypto())
	return fAvDivI(a, b)
}

// DivQ 分数相除（对 av_div_q；参数 b、c；回分数（分子分母）；无状态，可用零值直接调）。
func (self *Util) DivQ(b AVRational, c AVRational) AVRational {
	mustUse(ensureModCrypto())
	return fAvDivQ(b, c)
}

// DownmixInfoUpdateSideData 下混信息元数据操作（对 av_downmix_info_update_side_data；参数 frame；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DownmixInfoUpdateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDownmixInfoUpdateSideData(frame)
}

// DvCodecProfile DV 格式描述查询（对 av_dv_codec_profile；参数 width、height、pix_fmt（像素枚举数）；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DvCodecProfile(width int32, height int32, pix_fmt int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDvCodecProfile(width, height, pix_fmt)
}

// DvCodecProfile2 DV 格式描述查询（对 av_dv_codec_profile2；参数 width、height、pix_fmt（像素枚举数）、frame_rate；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DvCodecProfile2(width int32, height int32, pix_fmt int32, frame_rate AVRational) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDvCodecProfile2(width, height, pix_fmt, frame_rate)
}

// DvFrameProfile DV 格式描述查询（对 av_dv_frame_profile；参数 sys、frame、buf_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) DvFrameProfile(sys unsafe.Pointer, frame unsafe.Pointer, buf_size uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDvFrameProfile(sys, frame, buf_size)
}

// Dynarray2Add 往动态数组追加定长元素，自动扩容（对 av_dynarray2_add；参数 tab_ptr、nb_ptr、elem_size、elem_data；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Dynarray2Add(tab_ptr *unsafe.Pointer, nb_ptr unsafe.Pointer, elem_size uintptr, elem_data unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDynarray2Add(tab_ptr, nb_ptr, elem_size, elem_data)
}

// DynarrayAdd 动态数组追加（对 av_dynarray_add；参数 tab_ptr、nb_ptr、elem；按签名取回值；无状态，可用零值直接调）。
func (self *Util) DynarrayAdd(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvDynarrayAdd(tab_ptr, nb_ptr, elem)
}

// DynarrayAddNofree 动态数组追加（对 av_dynarray_add_nofree；参数 tab_ptr、nb_ptr、elem；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) DynarrayAddNofree(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvDynarrayAddNofree(tab_ptr, nb_ptr, elem); ret < 0 {
		return codeErr("av_dynarray_add_nofree", ret)
	}
	return nil
}

// EncryptionInfoAddSideData 加密信息元数据操作（对 av_encryption_info_add_side_data；参数 info、side_data_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) EncryptionInfoAddSideData(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInfoAddSideData(info, side_data_size)
}

// EncryptionInfoAlloc 加密信息元数据操作（对 av_encryption_info_alloc；参数 subsample_count、key_id_size、iv_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) EncryptionInfoAlloc(subsample_count uint32, key_id_size uint32, iv_size uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInfoAlloc(subsample_count, key_id_size, iv_size)
}

// EncryptionInfoClone 加密信息元数据操作（对 av_encryption_info_clone；参数 info；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) EncryptionInfoClone(info unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInfoClone(info)
}

// EncryptionInfoFree 加密信息元数据操作（对 av_encryption_info_free；参数 info；按签名取回值；无状态，可用零值直接调）。
func (self *Util) EncryptionInfoFree(info unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvEncryptionInfoFree(info)
}

// EncryptionInfoGetSideData 加密信息元数据操作（对 av_encryption_info_get_side_data；参数 side_data、side_data_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) EncryptionInfoGetSideData(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInfoGetSideData(side_data, side_data_size)
}

// EncryptionInitInfoAddSideData 加密信息元数据操作（对 av_encryption_init_info_add_side_data；参数 info、side_data_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) EncryptionInitInfoAddSideData(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInitInfoAddSideData(info, side_data_size)
}

// EncryptionInitInfoAlloc 加密信息元数据操作（对 av_encryption_init_info_alloc；参数 system_id_size、num_key_ids、key_id_size、data_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) EncryptionInitInfoAlloc(system_id_size uint32, num_key_ids uint32, key_id_size uint32, data_size uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInitInfoAlloc(system_id_size, num_key_ids, key_id_size, data_size)
}

// EncryptionInitInfoFree 加密信息元数据操作（对 av_encryption_init_info_free；参数 info；按签名取回值；无状态，可用零值直接调）。
func (self *Util) EncryptionInitInfoFree(info unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvEncryptionInitInfoFree(info)
}

// EncryptionInitInfoGetSideData 加密信息元数据操作（对 av_encryption_init_info_get_side_data；参数 side_data、side_data_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) EncryptionInitInfoGetSideData(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEncryptionInitInfoGetSideData(side_data, side_data_size)
}

// Escape 转义特殊字符（对 av_escape；参数 dst、src、special_chars、mode、flags；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Escape(dst *unsafe.Pointer, src unsafe.Pointer, special_chars unsafe.Pointer, mode unsafe.Pointer, flags int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvEscape(dst, src, special_chars, mode, flags)
}

// ExecutorAlloc 线程池执行器（对 av_executor_alloc；参数 callbacks、thread_count；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) ExecutorAlloc(callbacks unsafe.Pointer, thread_count int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvExecutorAlloc(callbacks, thread_count)
}

// ExecutorExecute 线程池执行器（对 av_executor_execute；参数 e、t；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ExecutorExecute(e unsafe.Pointer, t unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvExecutorExecute(e, t)
}

// ExecutorFree 线程池执行器（对 av_executor_free；参数 e；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ExecutorFree(e *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvExecutorFree(e)
}

// ExprCountFunc 表达式解析求值（对 av_expr_count_func；参数 e、counter、size、arg；回数值或个数；无状态，可用零值直接调）。
func (self *Util) ExprCountFunc(e unsafe.Pointer, counter unsafe.Pointer, size int32, arg int32) int32 {
	mustUse(ensureModCrypto())
	return fAvExprCountFunc(e, counter, size, arg)
}

// ExprCountVars 表达式解析求值（对 av_expr_count_vars；参数 e、counter、size；回数值或个数；无状态，可用零值直接调）。
func (self *Util) ExprCountVars(e unsafe.Pointer, counter unsafe.Pointer, size int32) int32 {
	mustUse(ensureModCrypto())
	return fAvExprCountVars(e, counter, size)
}

// ExprEval 表达式解析求值（对 av_expr_eval；参数 e、const_values、opaque；回浮点数；无状态，可用零值直接调）。
func (self *Util) ExprEval(e unsafe.Pointer, const_values unsafe.Pointer, opaque unsafe.Pointer) float64 {
	mustUse(ensureModCrypto())
	return fAvExprEval(e, const_values, opaque)
}

// ExprFree 表达式解析求值（对 av_expr_free；参数 e；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ExprFree(e unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvExprFree(e)
}

// ExprParse 表达式解析求值（对 av_expr_parse；参数 expr、s、const_names、func1_names、cb4、func2_names、cb6、log_offset、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ExprParse(expr *unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, func1_names unsafe.Pointer, cb4 unsafe.Pointer, func2_names unsafe.Pointer, cb6 unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvExprParse(expr, s, const_names, func1_names, cb4, func2_names, cb6, log_offset, log_ctx); ret < 0 {
		return codeErr("av_expr_parse", ret)
	}
	return nil
}

// ExprParseAndEval 表达式解析求值（对 av_expr_parse_and_eval；参数 res、s、const_names、const_values、func1_names、cb5、func2_names、cb7、opaque、log_offset、log_ctx；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ExprParseAndEval(res unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, const_values unsafe.Pointer, func1_names unsafe.Pointer, cb5 unsafe.Pointer, func2_names unsafe.Pointer, cb7 unsafe.Pointer, opaque unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvExprParseAndEval(res, s, const_names, const_values, func1_names, cb5, func2_names, cb7, opaque, log_offset, log_ctx); ret < 0 {
		return codeErr("av_expr_parse_and_eval", ret)
	}
	return nil
}

// FastMalloc 按需扩容的小内存分配（对 av_fast_malloc；参数 ptr、size、min_size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FastMalloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	mustUse(ensureModCrypto())
	fAvFastMalloc(ptr, size, min_size)
}

// FastMallocz 按需扩容的小内存分配（对 av_fast_mallocz；参数 ptr、size、min_size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FastMallocz(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	mustUse(ensureModCrypto())
	fAvFastMallocz(ptr, size, min_size)
}

// FastPaddedMalloc 按需分配带填充的缓冲，不够才扩（对 av_fast_padded_malloc；参数 ptr、size、min_size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FastPaddedMalloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	mustUse(ensureModCrypto())
	fAvFastPaddedMalloc(ptr, size, min_size)
}

// FastPaddedMallocz 清零版按需分配带填充缓冲（对 av_fast_padded_mallocz；参数 ptr、size、min_size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FastPaddedMallocz(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	mustUse(ensureModCrypto())
	fAvFastPaddedMallocz(ptr, size, min_size)
}

// FastRealloc 按需扩缓冲，顺手回新指针（对 av_fast_realloc；参数 ptr、size、min_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) FastRealloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFastRealloc(ptr, size, min_size)
}

// FftCalc 傅里叶变换小件（对 av_fft_calc；参数 s、z；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FftCalc(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFftCalc(s, z)
}

// FftEnd 傅里叶变换小件（对 av_fft_end；参数 s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FftEnd(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFftEnd(s)
}

// FftInit 傅里叶变换小件（对 av_fft_init；参数 nbits、inverse；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FftInit(nbits int32, inverse int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFftInit(nbits, inverse)
}

// FftPermute 傅里叶变换小件（对 av_fft_permute；参数 s、z；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FftPermute(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFftPermute(s, z)
}

// FileMap 把文件映射进内存（对 av_file_map；参数 filename、bufptr、size、log_offset、log_ctx；回 0 是成，负数是出错码；bufptr/size 两个槽都得给真内存，不能传 nil；拿到的缓冲用完拿 FileUnmap 放；filename 传 nil 会崩）。
func (self *Util) FileMap(filename unsafe.Pointer, bufptr *unsafe.Pointer, size unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvFileMap(filename, bufptr, size, log_offset, log_ctx); ret < 0 {
		return codeErr("av_file_map", ret)
	}
	return nil
}

// FilenameNumberTest 看文件名像不像编号序列（对 av_filename_number_test；参数 filename；回 1 是像、0 是不像（C 源码里 >=0 都是正常回值，不是出错）；filename 传 nil 会崩）。
func (self *Util) FilenameNumberTest(filename unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvFilenameNumberTest(filename)
}

// FileUnmap 释放文件映射的内存（对 av_file_unmap；参数 bufptr、size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FileUnmap(bufptr unsafe.Pointer, size uintptr) {
	mustUse(ensureModCrypto())
	fAvFileUnmap(bufptr, size)
}

// FilterIterate 逐个列出编译进来的滤镜（对 av_filter_iterate；参数 opaque；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) FilterIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFilterIterate(opaque)
}

// FindBestPixFmtOf2 在两个候选里挑离源格式损失小的（对 av_find_best_pix_fmt_of_2；参数 dst_pix_fmt1、dst_pix_fmt2、src_pix_fmt、has_alpha、loss_ptr；回数值；无状态，可用零值直接调）。
func (self *Prober) FindBestPixFmtOf2(dst_pix_fmt1 int32, dst_pix_fmt2 int32, src_pix_fmt int32, has_alpha int32, loss_ptr *int32) int32 {
	mustUse(ensureModCrypto())
	return fAvFindBestPixFmtOf2(dst_pix_fmt1, dst_pix_fmt2, src_pix_fmt, has_alpha, loss_ptr)
}

// FindDefaultStreamIndex 找默认播的那条流序号（对 av_find_default_stream_index；参数 s（格式上下文）；回流序号，无流回 -1；nil 上下文会崩，传真对象）.
func (self *Util) FindDefaultStreamIndex(s unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvFindDefaultStreamIndex(s)
}

// FindInfoTag 在信息串里按标签取值（对 av_find_info_tag；参数 arg、arg_size、tag1、info；找着回 1、没找着回 0（C 源码里 >=0 都是正常回值，不是出错）；arg 得是真缓冲，arg_size 含结尾零；info 传 nil 会崩）。
func (self *Util) FindInfoTag(arg unsafe.Pointer, arg_size int32, tag1 unsafe.Pointer, info unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvFindInfoTag(arg, arg_size, tag1, info)
}

// FindInputFormat 按名字找输入格式（对 av_find_input_format；参数 short_name；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Prober) FindInputFormat(short_name unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFindInputFormat(short_name)
}

// FindNearestQIdx returns the index of the table entry nearest to q
// (av_find_nearest_q_idx returns an index; q_list ends with a zero-den entry).
func (self *Util) FindNearestQIdx(q AVRational, q_list unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvFindNearestQIdx(q, q_list)
}

// FindProgramFromStream 按流序号找它属于哪个节目（对 av_find_program_from_stream；参数 ic、last、s；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) FindProgramFromStream(ic unsafe.Pointer, last unsafe.Pointer, s int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFindProgramFromStream(ic, last, s)
}

// FmtCtxGetDurationEstimationMethod 问时长是怎么估出来的（对 av_fmt_ctx_get_duration_estimation_method；参数 ctx；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FmtCtxGetDurationEstimationMethod(ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFmtCtxGetDurationEstimationMethod(ctx)
}

// ForceCpuFlags 强制 CPU 特性开关，调试压测用（对 av_force_cpu_flags；参数 flags；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ForceCpuFlags(flags int32) {
	mustUse(ensureModCrypto())
	fAvForceCpuFlags(flags)
}

// FormatInjectGlobalSideData 把全局附加数据注入每条流（对 av_format_inject_global_side_data；参数 s；按签名取回值；无状态，可用零值直接调）。
func (self *Util) FormatInjectGlobalSideData(s unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvFormatInjectGlobalSideData(s)
}

// FourccMakeString 把 fourcc 数字拼成人话四字符（对 av_fourcc_make_string；参数 buf、fourcc；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) FourccMakeString(buf unsafe.Pointer, fourcc uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvFourccMakeString(buf, fourcc)
}

// Gcd 算最大公约数（对 av_gcd；参数 a、b；回数值或个数；无状态，可用零值直接调）。
func (self *Util) Gcd(a int64, b int64) int64 {
	mustUse(ensureModCrypto())
	return fAvGcd(a, b)
}

// GcdQ 算最大公约数（对 av_gcd_q；参数 a、b、max_den、def；回分数（分子分母）；无状态，可用零值直接调）。
func (self *Util) GcdQ(a AVRational, b AVRational, max_den int32, def AVRational) AVRational {
	mustUse(ensureModCrypto())
	return fAvGcdQ(a, b, max_den, def)
}

// GetAltSampleFmt 找采样格式的平替（packed 反 planar）（对 av_get_alt_sample_fmt；参数 sample_fmt、planar；回数值或个数；无状态，可用零值直接调）。
func (self *Util) GetAltSampleFmt(sample_fmt int32, planar int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetAltSampleFmt(sample_fmt, planar)
}

// GetAudioFrameDuration 按采样数算一帧音频多少毫秒（对 av_get_audio_frame_duration；参数 avctx、frame_bytes；回数值或个数；无状态，可用零值直接调）。
func (self *Samples) GetAudioFrameDuration(avctx unsafe.Pointer, frame_bytes int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetAudioFrameDuration(avctx, frame_bytes)
}

// GetAudioFrameDuration2 按参数算一帧音频时长（新版）（对 av_get_audio_frame_duration2；参数 par、frame_bytes；回数值或个数；无状态，可用零值直接调）。
func (self *Samples) GetAudioFrameDuration2(par unsafe.Pointer, frame_bytes int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetAudioFrameDuration2(par, frame_bytes)
}

// GetBitsPerSample 问编码每采样占几位（对 av_get_bits_per_sample；参数 codec_id（编码枚举数）；回位数；无状态，可用零值直接调）。
func (self *Util) GetBitsPerSample(codec_id int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetBitsPerSample(codec_id)
}

// GetCpuFlags 问 CPU 支持哪些指令集（对 av_get_cpu_flags；无参数；回标志位掩码；无状态，可用零值直接调）。
func (self *Util) GetCpuFlags() int32 {
	mustUse(ensureModCrypto())
	return fAvGetCpuFlags()
}

// GetExactBitsPerSample 问编码每采样精确占几位（对 av_get_exact_bits_per_sample；参数 codec_id（编码枚举数）；回位数；无状态，可用零值直接调）。
func (self *Util) GetExactBitsPerSample(codec_id int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetExactBitsPerSample(codec_id)
}

// GetFrameFilename 按编号拼帧文件名（对 av_get_frame_filename；参数 buf、buf_size、path、number；回数值或个数；无状态，可用零值直接调）。
func (self *Util) GetFrameFilename(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetFrameFilename(buf, buf_size, path, number)
}

// GetFrameFilename2 按编号拼帧文件名（对 av_get_frame_filename2；参数 buf、buf_size、path、number、flags；回 0 是成、-1 是格式不对；buf/buf_size/path 都得是真的，传 nil 会崩）。
func (self *Util) GetFrameFilename2(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvGetFrameFilename2(buf, buf_size, path, number, flags); ret < 0 {
		return codeErr("av_get_frame_filename2", ret)
	}
	return nil
}

// GetKnownColorName 按编号查已知颜色名（对 av_get_known_color_name；参数 color_idx、rgb；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) GetKnownColorName(color_idx int32, rgb *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvGetKnownColorName(color_idx, rgb)
}

// GetMediaTypeString 把媒体类型换成人话（video/audio）（对 av_get_media_type_string；参数 media_type；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) GetMediaTypeString(media_type int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvGetMediaTypeString(media_type)
}

// GetOutputTimestamp 问输出流当前写到几点（对 av_get_output_timestamp；参数 s、stream、dts、wall；回数值或个数；无状态，可用零值直接调）。
func (self *Util) GetOutputTimestamp(s unsafe.Pointer, stream int32, dts unsafe.Pointer, wall unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvGetOutputTimestamp(s, stream, dts, wall)
}

// GetPacket 从流里取一包数据（对 av_get_packet；参数 s、pkt、size；回数值或个数；无状态，可用零值直接调）。
func (self *Util) GetPacket(s unsafe.Pointer, pkt unsafe.Pointer, size int32) int32 {
	mustUse(ensureModCrypto())
	return fAvGetPacket(s, pkt, size)
}

// GetPaddedBitsPerPixel 问像素格式含填充每像素占几位（对 av_get_padded_bits_per_pixel；参数 pixdesc；回数值或个数；无状态，可用零值直接调）。
func (self *Util) GetPaddedBitsPerPixel(pixdesc unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvGetPaddedBitsPerPixel(pixdesc)
}

// GetPcmCodec 按采样格式位宽端序找对应 PCM 编码（对 av_get_pcm_codec；参数 fmt（采样枚举数）、be；回编码指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) GetPcmCodec(fmt int32, be int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvGetPcmCodec(fmt, be)
}

// GetPictureTypeChar 把图像类型换成单字母（I/P/B）（对 av_get_picture_type_char；参数 pict_type（图像类型枚举数）；回单字母；无状态，可用零值直接调）。
func (self *Util) GetPictureTypeChar(pict_type int32) byte {
	mustUse(ensureModCrypto())
	return fAvGetPictureTypeChar(pict_type)
}

// GetProfileName 按编码和档次查档次名（对 av_get_profile_name；参数 codec、profile；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) GetProfileName(codec unsafe.Pointer, profile int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvGetProfileName(codec, profile)
}

// GetRandomSeed 取随机种子（对 av_get_random_seed；无参数；回 uint32 种子；无状态，可用零值直接调）。
func (self *Util) GetRandomSeed() uint32 {
	mustUse(ensureModCrypto())
	return fAvGetRandomSeed()
}

// GetTimeBaseQ 问编码对应的时基分数（对 av_get_time_base_q；无参数；回分数值；无状态，可用零值直接调）。
func (self *Util) GetTimeBaseQ() AVRational {
	mustUse(ensureModCrypto())
	return fAvGetTimeBaseQ()
}

// GetToken 按分隔符取一段（对 av_get_token；参数 buf、term；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) GetToken(buf *unsafe.Pointer, term unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvGetToken(buf, term)
}

// HashAlloc 新建一个哈希算子（对 av_hash_alloc；参数 ctx、name；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) HashAlloc(ctx *unsafe.Pointer, name unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHashAlloc(ctx, name); ret < 0 {
		return codeErr("av_hash_alloc", ret)
	}
	return nil
}

// HashFinal 收尾并取出哈希结果（对 av_hash_final；参数 ctx、dst；按签名取回值；无状态调用）。
func (self *Crypto) HashFinal(ctx unsafe.Pointer, dst unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvHashFinal(ctx, dst)
}

// HashFinalB64 收尾并取出哈希结果（对 av_hash_final_b64；参数 ctx、dst、size；按签名取回值；无状态调用）。
func (self *Crypto) HashFinalB64(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvHashFinalB64(ctx, dst, size)
}

// HashFinalBin 收尾并取出哈希结果（对 av_hash_final_bin；参数 ctx、dst、size；按签名取回值；无状态调用）。
func (self *Crypto) HashFinalBin(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvHashFinalBin(ctx, dst, size)
}

// HashFinalHex 收尾并取出哈希结果（对 av_hash_final_hex；参数 ctx、dst、size；按签名取回值；无状态调用）。
func (self *Crypto) HashFinalHex(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvHashFinalHex(ctx, dst, size)
}

// HashFreep 释放哈希算子并把指针清零（对 av_hash_freep；参数 ctx；按签名取回值；无状态调用）。
func (self *Crypto) HashFreep(ctx *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvHashFreep(ctx)
}

// HashGetName 问哈希算法叫什么名字（对 av_hash_get_name；参数 ctx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) HashGetName(ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHashGetName(ctx)
}

// HashGetSize 问哈希结果占多少字节（对 av_hash_get_size；参数 ctx；回字节数；ctx 传 nil 会崩，得传真上下文）。
func (self *Crypto) HashGetSize(ctx unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvHashGetSize(ctx)
}

// HashInit 哈希算子复位，准备算新数据（对 av_hash_init；参数 ctx；按签名取回值；无状态调用）。
func (self *Crypto) HashInit(ctx unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvHashInit(ctx)
}

// HashNames 逐个列出支持的哈希名（对 av_hash_names；参数 i；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) HashNames(i int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHashNames(i)
}

// HashUpdate 把一段数据喂给哈希算子（对 av_hash_update；参数 ctx、src、len；按签名取回值；无状态调用）。
func (self *Crypto) HashUpdate(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvHashUpdate(ctx, src, len)
}

// HexDump 把二进制按十六进制打到日志（对 av_hex_dump；参数 f、buf、size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) HexDump(f unsafe.Pointer, buf unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvHexDump(f, buf, size)
}

// HexDumpLog 带级别把二进制按十六进制打日志（对 av_hex_dump_log；参数 avcl、level、buf、size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) HexDumpLog(avcl unsafe.Pointer, level int32, buf unsafe.Pointer, size int32) {
	mustUse(ensureModCrypto())
	fAvHexDumpLog(avcl, level, buf, size)
}

// HmacAlloc 新建一个 HMAC 算子（对 av_hmac_alloc；参数 typ 是哈希类型枚举数（0=MD5、1=SHA1、2=SHA224、3=SHA256 等，见 hmac.h）；成功回新算子，用完拿 HmacFree 放；无状态调用）。
func (self *Crypto) HmacAlloc(typ int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHmacAlloc(typ)
}

// HmacCalc 一步算出整块数据的 HMAC（对 av_hmac_calc；参数 ctx、data、len、key、keylen、out、outlen；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) HmacCalc(ctx unsafe.Pointer, data unsafe.Pointer, len uint32, key unsafe.Pointer, keylen uint32, out unsafe.Pointer, outlen uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHmacCalc(ctx, data, len, key, keylen, out, outlen); ret < 0 {
		return codeErr("av_hmac_calc", ret)
	}
	return nil
}

// HmacFinal 收尾并取出 HMAC 结果（对 av_hmac_final；参数 ctx、out、outlen；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) HmacFinal(ctx unsafe.Pointer, out unsafe.Pointer, outlen uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHmacFinal(ctx, out, outlen); ret < 0 {
		return codeErr("av_hmac_final", ret)
	}
	return nil
}

// HmacFree 释放 HMAC 算子（对 av_hmac_free；参数 ctx；按签名取回值；无状态调用）。
func (self *Crypto) HmacFree(ctx unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvHmacFree(ctx)
}

// HmacInit 给 HMAC 装密钥并复位（对 av_hmac_init；参数 ctx、key、keylen；按签名取回值；无状态调用）。
func (self *Crypto) HmacInit(ctx unsafe.Pointer, key unsafe.Pointer, keylen uint32) {
	mustUse(ensureModCrypto())
	fAvHmacInit(ctx, key, keylen)
}

// HmacUpdate 把数据喂给 HMAC（对 av_hmac_update；参数 ctx、data、len；按签名取回值；无状态调用）。
func (self *Crypto) HmacUpdate(ctx unsafe.Pointer, data unsafe.Pointer, len uint32) {
	mustUse(ensureModCrypto())
	fAvHmacUpdate(ctx, data, len)
}

// HwdeviceCtxAlloc 按类型建硬解设备上下文（对 av_hwdevice_ctx_alloc；参数 typ 是设备类型枚举数（0=NONE，见 HwdeviceIterateTypes/FindTypeByName）；回新设备引用，用完拿缓冲释放；typ 越界回 nil 不崩）。
func (self *HWDevice) HwdeviceCtxAlloc(typ int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHwdeviceCtxAlloc(typ)
}

// HwdeviceCtxCreate 建或配硬解设备上下文（对 av_hwdevice_ctx_create；参数 device_ctx、typ、device、opts、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwdeviceCtxCreate(device_ctx *unsafe.Pointer, typ int32, device unsafe.Pointer, opts unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwdeviceCtxCreate(device_ctx, typ, device, opts, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create", ret)
	}
	return nil
}

// HwdeviceCtxCreateDerived 建或配硬解设备上下文（对 av_hwdevice_ctx_create_derived；参数 dst_ctx、typ、src_ctx、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwdeviceCtxCreateDerived(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwdeviceCtxCreateDerived(dst_ctx, typ, src_ctx, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create_derived", ret)
	}
	return nil
}

// HwdeviceCtxCreateDerivedOpts 建或配硬解设备上下文（对 av_hwdevice_ctx_create_derived_opts；参数 dst_ctx、typ、src_ctx、options、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwdeviceCtxCreateDerivedOpts(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, options unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwdeviceCtxCreateDerivedOpts(dst_ctx, typ, src_ctx, options, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create_derived_opts", ret)
	}
	return nil
}

// HwdeviceCtxInit 建或配硬解设备上下文（对 av_hwdevice_ctx_init；参数 ref；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwdeviceCtxInit(ref unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwdeviceCtxInit(ref); ret < 0 {
		return codeErr("av_hwdevice_ctx_init", ret)
	}
	return nil
}

// HwdeviceFindTypeByName 查硬解设备类型（对 av_hwdevice_find_type_by_name；参数 name；回数值或个数；无状态调用）。
func (self *HWDevice) HwdeviceFindTypeByName(name string) int32 {
	mustUse(ensureModCrypto())
	return fAvHwdeviceFindTypeByName(name)
}

// HwdeviceGetHwframeConstraints 查硬解设备类型（对 av_hwdevice_get_hwframe_constraints；参数 ref、hwconfig；回 C 指针，失败回 nil；无状态调用）。
func (self *HWDevice) HwdeviceGetHwframeConstraints(ref unsafe.Pointer, hwconfig unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHwdeviceGetHwframeConstraints(ref, hwconfig)
}

// HwdeviceGetTypeName 查硬解设备类型（对 av_hwdevice_get_type_name；参数 typ；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) HwdeviceGetTypeName(typ int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHwdeviceGetTypeName(typ)
}

// HwdeviceHwconfigAlloc 查硬解设备类型（对 av_hwdevice_hwconfig_alloc；参数 device_ctx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) HwdeviceHwconfigAlloc(device_ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHwdeviceHwconfigAlloc(device_ctx)
}

// HwdeviceIterateTypes 查硬解设备类型（对 av_hwdevice_iterate_types；参数 prev；回数值或个数；无状态调用）。
func (self *HWDevice) HwdeviceIterateTypes(prev int32) int32 {
	mustUse(ensureModCrypto())
	return fAvHwdeviceIterateTypes(prev)
}

// HwframeConstraintsFree 申请或搬运硬解帧缓冲（对 av_hwframe_constraints_free；参数 constraints；按签名取回值；无状态调用）。
func (self *HWDevice) HwframeConstraintsFree(constraints *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvHwframeConstraintsFree(constraints)
}

// HwframeCtxAlloc 申请或搬运硬解帧缓冲（对 av_hwframe_ctx_alloc；参数 device_ctx；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) HwframeCtxAlloc(device_ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvHwframeCtxAlloc(device_ctx)
}

// HwframeCtxCreateDerived 从已有帧上下文派生新帧上下文（对 av_hwframe_ctx_create_derived；参数 derived_frame_ctx（收新引用的槽）、format（目标像素格式枚举数）、derived_device_ctx、source_frame_ctx、flags；成功回 nil，失败回 error；各上下文传 nil 会崩，得传真对象）。
func (self *HWDevice) HwframeCtxCreateDerived(derived_frame_ctx *unsafe.Pointer, format int32, derived_device_ctx unsafe.Pointer, source_frame_ctx unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwframeCtxCreateDerived(derived_frame_ctx, format, derived_device_ctx, source_frame_ctx, flags); ret < 0 {
		return codeErr("av_hwframe_ctx_create_derived", ret)
	}
	return nil
}

// HwframeCtxInit 申请或搬运硬解帧缓冲（对 av_hwframe_ctx_init；参数 ref；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwframeCtxInit(ref unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwframeCtxInit(ref); ret < 0 {
		return codeErr("av_hwframe_ctx_init", ret)
	}
	return nil
}

// HwframeGetBuffer 申请或搬运硬解帧缓冲（对 av_hwframe_get_buffer；参数 hwframe_ctx、frame、flags；回数值；无状态调用）。
func (self *HWDevice) HwframeGetBuffer(hwframe_ctx unsafe.Pointer, frame unsafe.Pointer, flags int32) int32 {
	mustUse(ensureModCrypto())
	return fAvHwframeGetBuffer(hwframe_ctx, frame, flags)
}

// HwframeMap 申请或搬运硬解帧缓冲（对 av_hwframe_map；参数 dst、src、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwframeMap(dst unsafe.Pointer, src unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwframeMap(dst, src, flags); ret < 0 {
		return codeErr("av_hwframe_map", ret)
	}
	return nil
}

// HwframeTransferData 申请或搬运硬解帧缓冲（对 av_hwframe_transfer_data；参数 dst、src、flags；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) HwframeTransferData(dst unsafe.Pointer, src unsafe.Pointer, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvHwframeTransferData(dst, src, flags); ret < 0 {
		return codeErr("av_hwframe_transfer_data", ret)
	}
	return nil
}

// HwframeTransferGetFormats 问帧上下文支持哪些传输格式（对 av_hwframe_transfer_get_formats；参数 hwframe_ctx、dir（传输方向枚举数：0=到硬件、1=从硬件回内存）、formats（收格式数组的槽，用完拿 Mem.Free 放）、flags；回格式个数，负数是出错码；hwframe_ctx 传 nil 会崩）。
func (self *HWDevice) HwframeTransferGetFormats(hwframe_ctx unsafe.Pointer, dir int32, formats *unsafe.Pointer, flags int32) int32 {
	mustUse(ensureModCrypto())
	return fAvHwframeTransferGetFormats(hwframe_ctx, dir, formats, flags)
}

// I2int 把整数换成内部 int 表示（对 av_i2int；参数 a；回数值或个数；无状态，可用零值直接调）。
func (self *Util) I2int(a AVInteger) int64 {
	mustUse(ensureModCrypto())
	return fAvI2int(a)
}

// IamfAudioElementAddLayer 给 IAMF 音频元素加一层（对 av_iamf_audio_element_add_layer；参数 audio_element；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfAudioElementAddLayer(audio_element unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfAudioElementAddLayer(audio_element)
}

// IamfAudioElementAlloc 新建 IAMF 音频元素（对 av_iamf_audio_element_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) IamfAudioElementAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfAudioElementAlloc()
}

// IamfAudioElementFree 释放 IAMF 音频元素（对 av_iamf_audio_element_free；参数 audio_element；按签名取回值；无状态，可用零值直接调）。
func (self *Util) IamfAudioElementFree(audio_element *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvIamfAudioElementFree(audio_element)
}

// IamfAudioElementGetClass 取 IAMF 音频元素的选项类（对 av_iamf_audio_element_get_class；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfAudioElementGetClass() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfAudioElementGetClass()
}

// IamfMixPresentationAddSubmix 给 IAMF 混音展示加子混音（对 av_iamf_mix_presentation_add_submix；参数 mix_presentation；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfMixPresentationAddSubmix(mix_presentation unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfMixPresentationAddSubmix(mix_presentation)
}

// IamfMixPresentationAlloc 新建 IAMF 混音展示（对 av_iamf_mix_presentation_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) IamfMixPresentationAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfMixPresentationAlloc()
}

// IamfMixPresentationFree 释放 IAMF 混音展示（对 av_iamf_mix_presentation_free；参数 mix_presentation；按签名取回值；无状态，可用零值直接调）。
func (self *Util) IamfMixPresentationFree(mix_presentation *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvIamfMixPresentationFree(mix_presentation)
}

// IamfMixPresentationGetClass 取 IAMF 混音展示的选项类（对 av_iamf_mix_presentation_get_class；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfMixPresentationGetClass() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfMixPresentationGetClass()
}

// IamfParamDefinitionAlloc 新建 IAMF 参数定义（对 av_iamf_param_definition_alloc；参数 typ、nb_subblocks、size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) IamfParamDefinitionAlloc(typ unsafe.Pointer, nb_subblocks uint32, size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfParamDefinitionAlloc(typ, nb_subblocks, size)
}

// IamfParamDefinitionGetClass 取 IAMF 参数定义的选项类（对 av_iamf_param_definition_get_class；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfParamDefinitionGetClass() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfParamDefinitionGetClass()
}

// IamfSubmixAddElement 给 IAMF 子混音加元素（对 av_iamf_submix_add_element；参数 submix；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfSubmixAddElement(submix unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfSubmixAddElement(submix)
}

// IamfSubmixAddLayout 给 IAMF 子混音加布局（对 av_iamf_submix_add_layout；参数 submix；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) IamfSubmixAddLayout(submix unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvIamfSubmixAddLayout(submix)
}

// ImdctCalc 算反向修正离散余弦变换（对 av_imdct_calc；参数 s、output、input；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) ImdctCalc(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvImdctCalc(s, output, input)
}

// ImdctHalf 算半长 IMDCT（对 av_imdct_half；参数 s、output、input；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) ImdctHalf(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvImdctHalf(s, output, input)
}

// InitPacket 初始化空包结构体（对 av_init_packet；参数 pkt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) InitPacket(pkt unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvInitPacket(pkt)
}

// InputAudioDeviceNext 逐个列出输入音频设备（对 av_input_audio_device_next；参数 d；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) InputAudioDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvInputAudioDeviceNext(d)
}

// InputVideoDeviceNext 逐个列出输入视频设备（对 av_input_video_device_next；参数 d；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) InputVideoDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvInputVideoDeviceNext(d)
}

// Int2i 把内部 int 表示换回整数（对 av_int2i；参数 a；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Int2i(a int64) AVInteger {
	mustUse(ensureModCrypto())
	return fAvInt2i(a)
}

// InterleavedWriteFrame 按时间戳交织排序后写一包（对 av_interleaved_write_frame；参数 s、pkt；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) InterleavedWriteFrame(s unsafe.Pointer, pkt unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvInterleavedWriteFrame(s, pkt); ret < 0 {
		return codeErr("av_interleaved_write_frame", ret)
	}
	return nil
}

// InterleavedWriteUncodedFrame 交织排序后直接写一帧裸数据（对 av_interleaved_write_uncoded_frame；参数 s、stream_index、frame；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) InterleavedWriteUncodedFrame(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvInterleavedWriteUncodedFrame(s, stream_index, frame); ret < 0 {
		return codeErr("av_interleaved_write_uncoded_frame", ret)
	}
	return nil
}

// JniGetJavaVm 取安卓 Java 虚拟机指针（对 av_jni_get_java_vm；参数 log_ctx；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) JniGetJavaVm(log_ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvJniGetJavaVm(log_ctx)
}

// JniSetJavaVm 设置安卓 Java 虚拟机指针（对 av_jni_set_java_vm；参数 vm、log_ctx；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) JniSetJavaVm(vm unsafe.Pointer, log_ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvJniSetJavaVm(vm, log_ctx)
}

// LfgInit 简单随机数发生器（对 av_lfg_init；参数 c、seed；按签名取回值；无状态，可用零值直接调）。
func (self *Util) LfgInit(c unsafe.Pointer, seed uint32) {
	mustUse(ensureModCrypto())
	fAvLfgInit(c, seed)
}

// LfgInitFromData 简单随机数发生器（对 av_lfg_init_from_data；参数 c、data、length；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) LfgInitFromData(c unsafe.Pointer, data unsafe.Pointer, length uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvLfgInitFromData(c, data, length); ret < 0 {
		return codeErr("av_lfg_init_from_data", ret)
	}
	return nil
}

// Log 发一条日志（定长三参版，不带变参）（对 av_log；参数 avcl、level、fmt；fmt 里别带百分号，不然 C 会乱读寄存器，真要拼串走 variadic_go.go 的 Logf；avcl 传 nil 是整条不打）。
func (self *Util) Log(avcl unsafe.Pointer, level int32, fmt unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvLog(avcl, level, fmt)
}

// Lzo1xDecode 解 LZO 压缩块（对 av_lzo1x_decode；参数 out、outlen、in、inlen；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Lzo1xDecode(out unsafe.Pointer, outlen unsafe.Pointer, in unsafe.Pointer, inlen unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvLzo1xDecode(out, outlen, in, inlen)
}

// MatchExt 看文件名后缀在不在列表里（对 av_match_ext；参数 filename、extensions；后缀名对上回 1、对不上回 0（C 源码 format.c 里 filename 传 nil 直接回 0 不崩）；extensions 传 nil 会崩）。
func (self *Util) MatchExt(filename unsafe.Pointer, extensions unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvMatchExt(filename, extensions)
}

// MatchList 看名字在不在分隔符列表里（对 av_match_list；参数 name、list、separator；找着回 1、找不着回 0（C 源码里 >=0 都是正常回值，不是出错）；separator 是单个字符比如逗号；name/list 传 nil 会崩）。
func (self *Util) MatchList(name unsafe.Pointer, list unsafe.Pointer, separator byte) int32 {
	mustUse(ensureModCrypto())
	return fAvMatchList(name, list, separator)
}

// MatchName 看名字和逗号分隔的模式串匹不匹配（对 av_match_name；参数 name、names；对上回 1、对不上回 0（C 源码 avstring.c 里 name 或 names 传 nil 直接回 0 不崩）；"ALL" 通配全对上，前头带减号的是反选）。
func (self *Util) MatchName(name unsafe.Pointer, names unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvMatchName(name, names)
}

// MaxAlloc 问单次最多能申请多少内存（对 av_max_alloc；参数 max；按签名取回值；无状态，可用零值直接调）。
func (self *Util) MaxAlloc(max uintptr) {
	mustUse(ensureModCrypto())
	fAvMaxAlloc(max)
}

// Md5Alloc 算 MD5（对 av_md5_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) Md5Alloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMd5Alloc()
}

// Md5Final 算 MD5（对 av_md5_final；参数 ctx、dst；按签名取回值；无状态调用）。
func (self *Crypto) Md5Final(ctx unsafe.Pointer, dst unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvMd5Final(ctx, dst)
}

// Md5Init 算 MD5（对 av_md5_init；参数 ctx；按签名取回值；无状态调用）。
func (self *Crypto) Md5Init(ctx unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvMd5Init(ctx)
}

// Md5Sum 算 MD5（对 av_md5_sum；参数 dst、src、len；按签名取回值；无状态调用）。
func (self *Crypto) Md5Sum(dst unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvMd5Sum(dst, src, len)
}

// Md5Update 算 MD5（对 av_md5_update；参数 ctx、src、len；按签名取回值；无状态调用）。
func (self *Crypto) Md5Update(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvMd5Update(ctx, src, len)
}

// MdctCalc 算修正离散余弦变换（对 av_mdct_calc；参数 s、output、input；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) MdctCalc(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMdctCalc(s, output, input)
}

// MdctEnd 释放 MDCT 上下文（对 av_mdct_end；参数 s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) MdctEnd(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMdctEnd(s)
}

// MdctInit 新建 MDCT 上下文（对 av_mdct_init；参数 nbits、inverse、scale；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) MdctInit(nbits int32, inverse int32, scale float64) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMdctInit(nbits, inverse, scale)
}

// MediacodecAllocContext 安卓 MediaCodec 硬解的小件（对 av_mediacodec_alloc_context；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) MediacodecAllocContext() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMediacodecAllocContext()
}

// MediacodecDefaultFree 安卓 MediaCodec 硬解的小件（对 av_mediacodec_default_free；参数 avctx；按签名取回值；无状态调用）。
func (self *HWDevice) MediacodecDefaultFree(avctx unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvMediacodecDefaultFree(avctx)
}

// MediacodecDefaultInit 安卓 MediaCodec 硬解的小件（对 av_mediacodec_default_init；参数 avctx、ctx、surface；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) MediacodecDefaultInit(avctx unsafe.Pointer, ctx unsafe.Pointer, surface unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvMediacodecDefaultInit(avctx, ctx, surface); ret < 0 {
		return codeErr("av_mediacodec_default_init", ret)
	}
	return nil
}

// MediacodecReleaseBuffer 安卓 MediaCodec 硬解的小件（对 av_mediacodec_release_buffer；参数 buffer、render；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) MediacodecReleaseBuffer(buffer unsafe.Pointer, render int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvMediacodecReleaseBuffer(buffer, render); ret < 0 {
		return codeErr("av_mediacodec_release_buffer", ret)
	}
	return nil
}

// MediacodecRenderBufferAtTime 安卓 MediaCodec 硬解的小件（对 av_mediacodec_render_buffer_at_time；参数 buffer、time；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *HWDevice) MediacodecRenderBufferAtTime(buffer unsafe.Pointer, time int64) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvMediacodecRenderBufferAtTime(buffer, time); ret < 0 {
		return codeErr("av_mediacodec_render_buffer_at_time", ret)
	}
	return nil
}

// ModI divides and writes the quotient through quot (av_mod_i takes
// AVInteger BY VALUE and returns the remainder BY VALUE).
func (self *Util) ModI(quot *AVInteger, a AVInteger, b AVInteger) AVInteger {
	mustUse(ensureModCrypto())
	return fAvModI(quot, a, b)
}

// MulI multiplies two integers by value (av_mul_i takes AVInteger BY VALUE).
func (self *Util) MulI(a AVInteger, b AVInteger) AVInteger {
	mustUse(ensureModCrypto())
	return fAvMulI(a, b)
}

// MulQ 分数相乘（对 av_mul_q；参数 b、c；回分数（分子分母）；无状态，可用零值直接调）。
func (self *Util) MulQ(b AVRational, c AVRational) AVRational {
	mustUse(ensureModCrypto())
	return fAvMulQ(b, c)
}

// Murmur3Alloc 算 Murmur3 非加密哈希（对 av_murmur3_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) Murmur3Alloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMurmur3Alloc()
}

// Murmur3Final 算 Murmur3 非加密哈希（对 av_murmur3_final；参数 c、dst；按签名取回值；无状态调用）。
func (self *Crypto) Murmur3Final(c unsafe.Pointer, dst unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvMurmur3Final(c, dst)
}

// Murmur3Init 算 Murmur3 非加密哈希（对 av_murmur3_init；参数 c；按签名取回值；无状态调用）。
func (self *Crypto) Murmur3Init(c unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvMurmur3Init(c)
}

// Murmur3InitSeeded 算 Murmur3 非加密哈希（对 av_murmur3_init_seeded；参数 c、seed；按签名取回值；无状态调用）。
func (self *Crypto) Murmur3InitSeeded(c unsafe.Pointer, seed uint64) {
	mustUse(ensureModCrypto())
	fAvMurmur3InitSeeded(c, seed)
}

// Murmur3Update 算 Murmur3 非加密哈希（对 av_murmur3_update；参数 c、src、len；按签名取回值；无状态调用）。
func (self *Crypto) Murmur3Update(c unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvMurmur3Update(c, src, len)
}

// MuxerIterate 逐个列出支持的复用器（对 av_muxer_iterate；参数 opaque；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) MuxerIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvMuxerIterate(opaque)
}

// NearerQ picks the nearer of q1/q2 to q: >0 means q1 wins, <0 means q2
// wins (av_nearer_q returns a comparison, never an error).
func (self *Util) NearerQ(q AVRational, q1 AVRational, q2 AVRational) int32 {
	mustUse(ensureModCrypto())
	return fAvNearerQ(q, q1, q2)
}

// NewProgram 在盒子里新建一个节目（对 av_new_program；参数 s、id；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) NewProgram(s unsafe.Pointer, id int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvNewProgram(s, id)
}

// OutputAudioDeviceNext 逐个列出输出音频设备（对 av_output_audio_device_next；参数 d；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) OutputAudioDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvOutputAudioDeviceNext(d)
}

// OutputVideoDeviceNext 逐个列出输出视频设备（对 av_output_video_device_next；参数 d；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) OutputVideoDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvOutputVideoDeviceNext(d)
}

// PixelutilsGetSadFn 取算块差异的函数指针（对 av_pixelutils_get_sad_fn；参数 w_bits、h_bits、aligned、log_ctx；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) PixelutilsGetSadFn(w_bits int32, h_bits int32, aligned int32, log_ctx unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvPixelutilsGetSadFn(w_bits, h_bits, aligned, log_ctx)
}

// PktDump2 把包内容打到日志（新版）（对 av_pkt_dump2；参数 f、pkt、dump_payload、st；按签名取回值；无状态，可用零值直接调）。
func (self *Util) PktDump2(f unsafe.Pointer, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvPktDump2(f, pkt, dump_payload, st)
}

// PktDumpLog2 带级别把包内容打日志（新版）（对 av_pkt_dump_log2；参数 avcl、level、pkt、dump_payload、st；按签名取回值；无状态，可用零值直接调）。
func (self *Util) PktDumpLog2(avcl unsafe.Pointer, level int32, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvPktDumpLog2(avcl, level, pkt, dump_payload, st)
}

// ProbeInputBuffer 看一 buffered 数据像哪种盒子（对 av_probe_input_buffer；参数 pb、fmt、url、logctx、offset、max_probe_size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Prober) ProbeInputBuffer(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvProbeInputBuffer(pb, fmt, url, logctx, offset, max_probe_size); ret < 0 {
		return codeErr("av_probe_input_buffer", ret)
	}
	return nil
}

// ProbeInputBuffer2 看一 buffered 数据像哪种盒子（对 av_probe_input_buffer2；参数 pb、fmt、url、logctx、offset、max_probe_size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Prober) ProbeInputBuffer2(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvProbeInputBuffer2(pb, fmt, url, logctx, offset, max_probe_size); ret < 0 {
		return codeErr("av_probe_input_buffer2", ret)
	}
	return nil
}

// ProbeInputFormat 看一段数据像哪种盒子（对 av_probe_input_format；参数 pd、is_opened；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Prober) ProbeInputFormat(pd unsafe.Pointer, is_opened int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvProbeInputFormat(pd, is_opened)
}

// ProbeInputFormat2 看一段数据像哪种盒子（对 av_probe_input_format2；参数 pd、is_opened、score_max；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Prober) ProbeInputFormat2(pd unsafe.Pointer, is_opened int32, score_max unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvProbeInputFormat2(pd, is_opened, score_max)
}

// ProbeInputFormat3 看一段数据像哪种盒子（对 av_probe_input_format3；参数 pd、is_opened、score_ret；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Prober) ProbeInputFormat3(pd unsafe.Pointer, is_opened int32, score_ret unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvProbeInputFormat3(pd, is_opened, score_ret)
}

// ProgramAddStreamIndex 把流序号加入节目（对 av_program_add_stream_index；参数 ac、progid、idx；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ProgramAddStreamIndex(ac unsafe.Pointer, progid int32, idx uint32) {
	mustUse(ensureModCrypto())
	fAvProgramAddStreamIndex(ac, progid, idx)
}

// Q2intfloat 把分数换成浮点（对 av_q2intfloat；参数 q；回数值；无状态，可用零值直接调）。
func (self *Util) Q2intfloat(q AVRational) uint32 {
	mustUse(ensureModCrypto())
	return fAvQ2intfloat(q)
}

// QsvAllocContext 新建 QSV 硬解上下文（对 av_qsv_alloc_context；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) QsvAllocContext() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvQsvAllocContext()
}

// RandomBytes 取随机字节（对 av_random_bytes；参数 buf、len；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) RandomBytes(buf unsafe.Pointer, len uintptr) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvRandomBytes(buf, len); ret < 0 {
		return codeErr("av_random_bytes", ret)
	}
	return nil
}

// Rc4Alloc RC4 流加解密的小件（对 av_rc4_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) Rc4Alloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvRc4Alloc()
}

// Rc4Crypt RC4 流加解密的小件（对 av_rc4_crypt；参数 d、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) Rc4Crypt(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvRc4Crypt(d, dst, src, count, iv, decrypt)
}

// Rc4Init RC4 流加解密的小件（对 av_rc4_init；参数 d、key、key_bits、decrypt；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) Rc4Init(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvRc4Init(d, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_rc4_init", ret)
	}
	return nil
}

// RdftCalc 算实数离散傅里叶变换（对 av_rdft_calc；参数 s、data；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) RdftCalc(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvRdftCalc(s, data)
}

// RdftEnd 释放 RDFT 上下文（对 av_rdft_end；参数 s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) RdftEnd(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvRdftEnd(s)
}

// RdftInit 新建 RDFT 上下文（对 av_rdft_init；参数 nbits、trans；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) RdftInit(nbits int32, trans unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvRdftInit(nbits, trans)
}

// ReadImageLine 读一行图像像素（对 av_read_image_line；参数 dst、data、linesize、desc、x、y、c、w、read_pal_component；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ReadImageLine(dst unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32) {
	mustUse(ensureModCrypto())
	fAvReadImageLine(dst, data, linesize, desc, x, y, c, w, read_pal_component)
}

// ReadImageLine2 读一行图像像素（带元素大小）（对 av_read_image_line2；参数 dst、data、linesize、desc、x、y、c、w、read_pal_component、dst_element_size；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ReadImageLine2(dst unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32, dst_element_size int32) {
	mustUse(ensureModCrypto())
	fAvReadImageLine2(dst, data, linesize, desc, x, y, c, w, read_pal_component, dst_element_size)
}

// ReadPause 暂停网络读流（对 av_read_pause；参数 s；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ReadPause(s unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvReadPause(s); ret < 0 {
		return codeErr("av_read_pause", ret)
	}
	return nil
}

// ReadPlay 恢复网络读流（对 av_read_play；参数 s；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ReadPlay(s unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvReadPlay(s); ret < 0 {
		return codeErr("av_read_play", ret)
	}
	return nil
}

// Reduce 分数约分（对 av_reduce；参数 dst_num、dst_den、num、den、max；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) Reduce(dst_num unsafe.Pointer, dst_den unsafe.Pointer, num int64, den int64, max int64) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvReduce(dst_num, dst_den, num, den, max); ret < 0 {
		return codeErr("av_reduce", ret)
	}
	return nil
}

// RipemdAlloc 算 RIPEMD（对 av_ripemd_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) RipemdAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvRipemdAlloc()
}

// RipemdFinal 算 RIPEMD（对 av_ripemd_final；参数 context、digest；按签名取回值；无状态调用）。
func (self *Crypto) RipemdFinal(context unsafe.Pointer, digest unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvRipemdFinal(context, digest)
}

// RipemdInit 算 RIPEMD（对 av_ripemd_init；参数 context、bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) RipemdInit(context unsafe.Pointer, bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvRipemdInit(context, bits); ret < 0 {
		return codeErr("av_ripemd_init", ret)
	}
	return nil
}

// RipemdUpdate 算 RIPEMD（对 av_ripemd_update；参数 context、data、len；按签名取回值；无状态调用）。
func (self *Crypto) RipemdUpdate(context unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvRipemdUpdate(context, data, len)
}

// SamplesAlloc 分配采样缓冲（对 av_samples_alloc；参数 audio_data、linesize、nb_channels、nb_samples、sample_fmt、align；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Samples) SamplesAlloc(audio_data *unsafe.Pointer, linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSamplesAlloc(audio_data, linesize, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_alloc", ret)
	}
	return nil
}

// SamplesAllocArrayAndSamples 分配采样指针数组加缓冲（对 av_samples_alloc_array_and_samples；参数 audio_data、linesize、nb_channels、nb_samples、sample_fmt、align；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Samples) SamplesAllocArrayAndSamples(audio_data *unsafe.Pointer, linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSamplesAllocArrayAndSamples(audio_data, linesize, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_alloc_array_and_samples", ret)
	}
	return nil
}

// SamplesCopy 拷采样数据（对 av_samples_copy；参数 dst、src、dst_offset、src_offset、nb_samples、nb_channels、sample_fmt；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Samples) SamplesCopy(dst unsafe.Pointer, src unsafe.Pointer, dst_offset int32, src_offset int32, nb_samples int32, nb_channels int32, sample_fmt int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSamplesCopy(dst, src, dst_offset, src_offset, nb_samples, nb_channels, sample_fmt); ret < 0 {
		return codeErr("av_samples_copy", ret)
	}
	return nil
}

// SamplesFillArrays 把现成内存填成采样指针数组（对 av_samples_fill_arrays；参数 audio_data、linesize、buf、nb_channels、nb_samples、sample_fmt、align；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Samples) SamplesFillArrays(audio_data *unsafe.Pointer, linesize *int32, buf unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSamplesFillArrays(audio_data, linesize, buf, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_fill_arrays", ret)
	}
	return nil
}

// SamplesGetBufferSize 算采样缓冲要多少字节（对 av_samples_get_buffer_size；参数 linesize、nb_channels、nb_samples、sample_fmt、align；回数值；无状态，可用零值直接调）。
func (self *Samples) SamplesGetBufferSize(linesize *int32, nb_channels int32, nb_samples int32, sample_fmt int32, align int32) int32 {
	mustUse(ensureModCrypto())
	return fAvSamplesGetBufferSize(linesize, nb_channels, nb_samples, sample_fmt, align)
}

// SamplesSetSilence 把采样缓冲置成静音（对 av_samples_set_silence；参数 audio_data、offset、nb_samples、nb_channels、sample_fmt；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Samples) SamplesSetSilence(audio_data unsafe.Pointer, offset int32, nb_samples int32, nb_channels int32, sample_fmt int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSamplesSetSilence(audio_data, offset, nb_samples, nb_channels, sample_fmt); ret < 0 {
		return codeErr("av_samples_set_silence", ret)
	}
	return nil
}

// SdpCreate 按流拼 SDP 描述串（对 av_sdp_create；参数 ac、n_files、buf、size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) SdpCreate(ac unsafe.Pointer, n_files int32, buf unsafe.Pointer, size int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSdpCreate(ac, n_files, buf, size); ret < 0 {
		return codeErr("av_sdp_create", ret)
	}
	return nil
}

// SetOptionsString 按字符串批量设选项（对 av_set_options_string；参数 ctx、opts、key_val_sep、pairs_sep；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) SetOptionsString(ctx unsafe.Pointer, opts unsafe.Pointer, key_val_sep unsafe.Pointer, pairs_sep unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSetOptionsString(ctx, opts, key_val_sep, pairs_sep); ret < 0 {
		return codeErr("av_set_options_string", ret)
	}
	return nil
}

// Sha512Alloc 算 SHA（对 av_sha512_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) Sha512Alloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvSha512Alloc()
}

// Sha512Final 算 SHA（对 av_sha512_final；参数 context、digest；按签名取回值；无状态调用）。
func (self *Crypto) Sha512Final(context unsafe.Pointer, digest unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvSha512Final(context, digest)
}

// Sha512Init 算 SHA（对 av_sha512_init；参数 context、bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) Sha512Init(context unsafe.Pointer, bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvSha512Init(context, bits); ret < 0 {
		return codeErr("av_sha512_init", ret)
	}
	return nil
}

// Sha512Update 算 SHA（对 av_sha512_update；参数 context、data、len；按签名取回值；无状态调用）。
func (self *Crypto) Sha512Update(context unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvSha512Update(context, data, len)
}

// ShaAlloc 算 SHA（对 av_sha_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) ShaAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvShaAlloc()
}

// ShaFinal 算 SHA（对 av_sha_final；参数 context、digest；按签名取回值；无状态调用）。
func (self *Crypto) ShaFinal(context unsafe.Pointer, digest unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvShaFinal(context, digest)
}

// ShaInit 算 SHA（对 av_sha_init；参数 context、bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) ShaInit(context unsafe.Pointer, bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvShaInit(context, bits); ret < 0 {
		return codeErr("av_sha_init", ret)
	}
	return nil
}

// ShaUpdate 算 SHA（对 av_sha_update；参数 ctx、data、len；按签名取回值；无状态调用）。
func (self *Crypto) ShaUpdate(ctx unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	mustUse(ensureModCrypto())
	fAvShaUpdate(ctx, data, len)
}

// ShrI 整数右移（对 av_shr_i；参数 a、s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) ShrI(a AVInteger, s int32) AVInteger {
	mustUse(ensureModCrypto())
	return fAvShrI(a, s)
}

// SizeMult 算字节数乘法，防溢出（对 av_size_mult；参数 a、b、r；回数值；无状态，可用零值直接调）。
func (self *Util) SizeMult(a uintptr, b uintptr, r unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvSizeMult(a, b, r)
}

// SmallStrptime 解析时间字符串成结构体（对 av_small_strptime；参数 p、fmt、dt；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) SmallStrptime(p unsafe.Pointer, fmt unsafe.Pointer, dt unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvSmallStrptime(p, fmt, dt)
}

// Sscanf 按格式从字符串里取值（对 av_sscanf；参数 str、format，后面跟的变参 purego 传不准所以没接，只能数格式里本来就不带取值的情形；真要取值走 Go 的 fmt.Sscanf；str/format 传 nil 会崩）。
func (self *Util) Sscanf(str unsafe.Pointer, format unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvSscanf(str, format)
}

// Strcasecmp 字符串小工具（对 av_strcasecmp；参数 a、b；回 0 是两串一样（只认 ASCII 大小写，中文等多字节按字节比），负数是 a 小、正数是 a 大；a/b 传 nil 会崩）。
func (self *Util) Strcasecmp(a unsafe.Pointer, b unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvStrcasecmp(a, b)
}

// Strdup 字符串小工具（对 av_strdup；参数 s；回新串（C 里拿 av_malloc 分的），用完拿 Mem.Free 放；s 传 nil 回 nil 不崩）。
func (self *Util) Strdup(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStrdup(s)
}

// StrNDup copies at most n bytes of s into fresh malloc'd memory and
// returns it as a Go string (the C copy is freed before returning).
func (self *Util) StrNDup(s string, n int) string {
	mustUse(ensureModCrypto())
	p := fAvStrndup(s, uintptr(n))
	if p == nil {
		return ""
	}
	defer fMemFree(p)
	return cstr(p)
}

// IntListLengthForSize returns the element count of a terminated integer
// list (terminator not counted; elsize is 1, 2, 4 or 8).
func (self *Util) IntListLengthForSize(elsize uint32, list unsafe.Pointer, term uint64) uint32 {
	mustUse(ensureModCrypto())
	return fAvIntListLengthForSize(elsize, list, term)
}

// StreamAddSideData 字符串小工具（对 av_stream_add_side_data；参数 st、typ、data、size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamAddSideData(st unsafe.Pointer, typ unsafe.Pointer, data unsafe.Pointer, size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamAddSideData(st, typ, data, size)
}

// StreamGetClass 字符串小工具（对 av_stream_get_class；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamGetClass() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamGetClass()
}

// StreamGetCodecTimebase 字符串小工具（对 av_stream_get_codec_timebase；参数 st；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamGetCodecTimebase(st unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamGetCodecTimebase(st)
}

// StreamGetParser 字符串小工具（对 av_stream_get_parser；参数 s；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamGetParser(s unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamGetParser(s)
}

// StreamGetSideData 字符串小工具（对 av_stream_get_side_data；参数 stream、typ、size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamGetSideData(stream unsafe.Pointer, typ unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamGetSideData(stream, typ, size)
}

// StreamGroupGetClass 字符串小工具（对 av_stream_group_get_class；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) StreamGroupGetClass() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamGroupGetClass()
}

// StreamNewSideData 字符串小工具（对 av_stream_new_side_data；参数 stream、typ、size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) StreamNewSideData(stream unsafe.Pointer, typ unsafe.Pointer, size uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStreamNewSideData(stream, typ, size)
}

// Strireplace 字符串小工具（对 av_strireplace；参数 str、from、to；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Strireplace(str unsafe.Pointer, from unsafe.Pointer, to unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStrireplace(str, from, to)
}

// Stristart 字符串小工具（对 av_stristart；参数 str、pfx、ptr；前头对上回 1、对不上回 0（只认 ASCII 大小写），对上了 ptr 才会被填成前缀后头的位置；str/pfx 传 nil 会崩，ptr 传 nil 是只问对没对上不取位置）。
func (self *Util) Stristart(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvStristart(str, pfx, ptr)
}

// Stristr 字符串小工具（对 av_stristr；参数 haystack、needle；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Stristr(haystack unsafe.Pointer, needle unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStristr(haystack, needle)
}

// Strlcat 字符串小工具（对 av_strlcat；参数 dst、src、size；回数值；无状态，可用零值直接调）。
func (self *Util) Strlcat(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr {
	mustUse(ensureModCrypto())
	return fAvStrlcat(dst, src, size)
}

// Strlcpy 字符串小工具（对 av_strlcpy；参数 dst、src、size；回数值；无状态，可用零值直接调）。
func (self *Util) Strlcpy(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr {
	mustUse(ensureModCrypto())
	return fAvStrlcpy(dst, src, size)
}

// Strncasecmp 字符串小工具（对 av_strncasecmp；参数 a、b、n；只比前 n 个，只认 ASCII 大小写；回 0 是前 n 个一样，n 传 0 直接回 0 不比；a/b 传 nil 会崩）。
func (self *Util) Strncasecmp(a unsafe.Pointer, b unsafe.Pointer, n uintptr) int32 {
	mustUse(ensureModCrypto())
	return fAvStrncasecmp(a, b, n)
}

// Strnstr 字符串小工具（对 av_strnstr；参数 haystack、needle、hay_length；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Strnstr(haystack unsafe.Pointer, needle unsafe.Pointer, hay_length uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStrnstr(haystack, needle, hay_length)
}

// Strstart 字符串小工具（对 av_strstart；参数 str、pfx、ptr；前头对上回 1、对不上回 0，对上了 ptr 才会被填成前缀后头的位置；str/pfx 传 nil 会崩，ptr 传 nil 是只问对没对上不取位置）。
func (self *Util) Strstart(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvStrstart(str, pfx, ptr)
}

// Strtod 字符串小工具（对 av_strtod；参数 numstr、tail；回浮点数；无状态，可用零值直接调）。
func (self *Util) Strtod(numstr unsafe.Pointer, tail *unsafe.Pointer) float64 {
	mustUse(ensureModCrypto())
	return fAvStrtod(numstr, tail)
}

// Strtok 字符串小工具（对 av_strtok；参数 s、delim、saveptr；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) Strtok(s unsafe.Pointer, delim unsafe.Pointer, saveptr *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvStrtok(s, delim, saveptr)
}

// SubI 整数相减（对 av_sub_i；参数 a、b；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) SubI(a AVInteger, b AVInteger) AVInteger {
	mustUse(ensureModCrypto())
	return fAvSubI(a, b)
}

// SubQ 分数相减（对 av_sub_q；参数 b、c；回分数（分子分母）；无状态，可用零值直接调）。
func (self *Util) SubQ(b AVRational, c AVRational) AVRational {
	mustUse(ensureModCrypto())
	return fAvSubQ(b, c)
}

// TeaAlloc TEA 加解密的小件（对 av_tea_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) TeaAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTeaAlloc()
}

// TeaCrypt TEA 加解密的小件（对 av_tea_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) TeaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvTeaCrypt(ctx, dst, src, count, iv, decrypt)
}

// TeaInit TEA 加解密的小件（对 av_tea_init；参数 ctx、key、rounds；按签名取回值；无状态调用）。
func (self *Crypto) TeaInit(ctx unsafe.Pointer, key unsafe.Pointer, rounds int32) {
	mustUse(ensureModCrypto())
	fAvTeaInit(ctx, key, rounds)
}

// ThreadMessageFlush 清线程消息队列（对 av_thread_message_flush；参数 mq；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ThreadMessageFlush(mq unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvThreadMessageFlush(mq)
}

// ThreadMessageQueueAlloc 新建线程消息队列（对 av_thread_message_queue_alloc；参数 mq、nelem、elsize；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueAlloc(mq *unsafe.Pointer, nelem uint32, elsize uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvThreadMessageQueueAlloc(mq, nelem, elsize); ret < 0 {
		return codeErr("av_thread_message_queue_alloc", ret)
	}
	return nil
}

// ThreadMessageQueueFree 释放线程消息队列（对 av_thread_message_queue_free；参数 mq；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueFree(mq *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvThreadMessageQueueFree(mq)
}

// ThreadMessageQueueNbElems 问队列里攒了几条（对 av_thread_message_queue_nb_elems；参数 mq；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueNbElems(mq unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvThreadMessageQueueNbElems(mq); ret < 0 {
		return codeErr("av_thread_message_queue_nb_elems", ret)
	}
	return nil
}

// ThreadMessageQueueRecv 从队列取一条（对 av_thread_message_queue_recv；参数 mq、msg、flags；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueRecv(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvThreadMessageQueueRecv(mq, msg, flags); ret < 0 {
		return codeErr("av_thread_message_queue_recv", ret)
	}
	return nil
}

// ThreadMessageQueueSend 往队列塞一条（对 av_thread_message_queue_send；参数 mq、msg、flags；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueSend(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvThreadMessageQueueSend(mq, msg, flags); ret < 0 {
		return codeErr("av_thread_message_queue_send", ret)
	}
	return nil
}

// ThreadMessageQueueSetErrRecv 设队列收端错误（对 av_thread_message_queue_set_err_recv；参数 mq、err；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueSetErrRecv(mq unsafe.Pointer, err int32) {
	mustUse(ensureModCrypto())
	fAvThreadMessageQueueSetErrRecv(mq, err)
}

// ThreadMessageQueueSetErrSend 设队列发端错误（对 av_thread_message_queue_set_err_send；参数 mq、err；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueSetErrSend(mq unsafe.Pointer, err int32) {
	mustUse(ensureModCrypto())
	fAvThreadMessageQueueSetErrSend(mq, err)
}

// ThreadMessageQueueSetFreeFunc 设队列元素释放函数（对 av_thread_message_queue_set_free_func；参数 mq、free_func；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ThreadMessageQueueSetFreeFunc(mq unsafe.Pointer, free_func unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvThreadMessageQueueSetFreeFunc(mq, free_func)
}

// Timegm 把时间结构体换成秒数（对 av_timegm；参数 tm；回自 1970 年起的秒数；tm 传 nil 会崩，得传真 struct tm）。
func (self *Util) Timegm(tm unsafe.Pointer) int64 {
	mustUse(ensureModCrypto())
	return fAvTimegm(tm)
}

// TreeDestroy 释放整棵树（对 av_tree_destroy；参数 t；按签名取回值；无状态，可用零值直接调）。
func (self *Util) TreeDestroy(t unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvTreeDestroy(t)
}

// TreeEnumerate 遍历树每个节点（对 av_tree_enumerate；参数 t、opaque、cmp、enu；按签名取回值；无状态，可用零值直接调）。
func (self *Util) TreeEnumerate(t unsafe.Pointer, opaque unsafe.Pointer, cmp unsafe.Pointer, enu unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvTreeEnumerate(t, opaque, cmp, enu)
}

// TreeFind 在树里找键（对 av_tree_find；参数 root、key、cmp、next；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) TreeFind(root unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTreeFind(root, key, cmp, next)
}

// TreeInsert 往树里插键（对 av_tree_insert；参数 rootp、key、cmp、next；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) TreeInsert(rootp *unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next *unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTreeInsert(rootp, key, cmp, next)
}

// TreeNodeAlloc 新建树节点（对 av_tree_node_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) TreeNodeAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTreeNodeAlloc()
}

// TsMakeTimeString2 把时间戳拼成人话（新版）（对 av_ts_make_time_string2；参数 buf、ts、tb；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) TsMakeTimeString2(buf unsafe.Pointer, ts int64, tb AVRational) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTsMakeTimeString2(buf, ts, tb)
}

// TwofishAlloc Twofish 加解密的小件（对 av_twofish_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) TwofishAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvTwofishAlloc()
}

// TwofishCrypt Twofish 加解密的小件（对 av_twofish_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) TwofishCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvTwofishCrypt(ctx, dst, src, count, iv, decrypt)
}

// TwofishInit Twofish 加解密的小件（对 av_twofish_init；参数 ctx、key、key_bits；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Crypto) TwofishInit(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvTwofishInit(ctx, key, key_bits); ret < 0 {
		return codeErr("av_twofish_init", ret)
	}
	return nil
}

// TxInit 新建变换上下文（对 av_tx_init；参数 ctx、tx、typ、inv、len、scale、flags；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) TxInit(ctx *unsafe.Pointer, tx unsafe.Pointer, typ unsafe.Pointer, inv int32, len int32, scale unsafe.Pointer, flags uint64) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvTxInit(ctx, tx, typ, inv, len, scale, flags); ret < 0 {
		return codeErr("av_tx_init", ret)
	}
	return nil
}

// TxUninit 释放变换上下文（对 av_tx_uninit；参数 ctx；按签名取回值；无状态，可用零值直接调）。
func (self *Util) TxUninit(ctx *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvTxUninit(ctx)
}

// Utf8Decode 解一个 UTF8 字符（对 av_utf8_decode；参数 codep、bufp、buf_end、flags；回读到的字节数，0 是缓冲到头了没得读，负数是字节流坏了；bufp 会被推到下一个字符开头；四个指针槽都得是真的，传 nil 会崩）。
func (self *Util) Utf8Decode(codep unsafe.Pointer, bufp *unsafe.Pointer, buf_end unsafe.Pointer, flags uint32) int32 {
	mustUse(ensureModCrypto())
	return fAvUtf8Decode(codep, bufp, buf_end, flags)
}

// UuidParse 解析 UUID 字符串（对 av_uuid_parse；参数 in、uu；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) UuidParse(in unsafe.Pointer, uu unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvUuidParse(in, uu); ret < 0 {
		return codeErr("av_uuid_parse", ret)
	}
	return nil
}

// UuidParseRange 解析一段 UUID 字符串（对 av_uuid_parse_range；参数 in_start、in_end、uu；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) UuidParseRange(in_start unsafe.Pointer, in_end unsafe.Pointer, uu unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvUuidParseRange(in_start, in_end, uu); ret < 0 {
		return codeErr("av_uuid_parse_range", ret)
	}
	return nil
}

// UuidUnparse 把 UUID 拼成字符串（对 av_uuid_unparse；参数 uu、out；按签名取回值；无状态，可用零值直接调）。
func (self *Util) UuidUnparse(uu unsafe.Pointer, out unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvUuidUnparse(uu, out)
}

// UuidUrnParse 解析 URN 形式的 UUID（对 av_uuid_urn_parse；参数 in、uu；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) UuidUrnParse(in unsafe.Pointer, uu unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvUuidUrnParse(in, uu); ret < 0 {
		return codeErr("av_uuid_urn_parse", ret)
	}
	return nil
}

// Vbprintf 往打印缓冲追加（va_list 版）（对 av_vbprintf；参数 buf、fmt、vl_arg；按签名取回值；无状态，可用零值直接调）。
func (self *Util) Vbprintf(buf unsafe.Pointer, fmt unsafe.Pointer, vl_arg unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvVbprintf(buf, fmt, vl_arg)
}

// VdpauAllocContext VDPAU 硬解的小件（对 av_vdpau_alloc_context；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *HWDevice) VdpauAllocContext() unsafe.Pointer {
	mustUse(ensureModCryptoHw())
	return fAvVdpauAllocContext()
}

// VdpauBindContext 把解码上下文绑到 VDPAU 设备上（对 av_vdpau_bind_context；参数 avctx（须是开好的解码上下文）、device、get_proc_address、flags；回 0 是成，负数是出错码；device 传 0 会失败回错不崩，avctx 传 nil 会崩）。
func (self *HWDevice) VdpauBindContext(avctx unsafe.Pointer, device unsafe.Pointer, get_proc_address unsafe.Pointer, flags uint32) error {
	if err := ensureModCryptoHw(); err != nil {
		return err
	}
	if ret := fAvVdpauBindContext(avctx, device, get_proc_address, flags); ret < 0 {
		return codeErr("av_vdpau_bind_context", ret)
	}
	return nil
}

// VdpauGetSurfaceParameters VDPAU 硬解的小件（对 av_vdpau_get_surface_parameters；参数 avctx、typ、width、height；回数值或个数；无状态调用）。
func (self *HWDevice) VdpauGetSurfaceParameters(avctx unsafe.Pointer, typ unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer) int32 {
	mustUse(ensureModCryptoHw())
	return fAvVdpauGetSurfaceParameters(avctx, typ, width, height)
}

// VdpauHwaccelGetRender2 VDPAU 硬解的小件（对 av_vdpau_hwaccel_get_render2；参数 arg0；回 C 指针，失败回 nil；无状态调用）。
func (self *HWDevice) VdpauHwaccelGetRender2(arg0 unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCryptoHw())
	return fAvVdpauHwaccelGetRender2(arg0)
}

// VdpauHwaccelSetRender2 VDPAU 硬解的小件（对 av_vdpau_hwaccel_set_render2；参数 arg0、arg1；回 C 指针，失败回 nil；无状态调用）。
func (self *HWDevice) VdpauHwaccelSetRender2(arg0 unsafe.Pointer, arg1 unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCryptoHw())
	return fAvVdpauHwaccelSetRender2(arg0, arg1)
}

// VideoEncParamsAlloc 新建视频编码参数（对 av_video_enc_params_alloc；参数 typ、nb_blocks、out_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VideoEncParamsAlloc(typ unsafe.Pointer, nb_blocks uint32, out_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVideoEncParamsAlloc(typ, nb_blocks, out_size)
}

// VideoEncParamsCreateSideData 给帧挂视频编码参数（对 av_video_enc_params_create_side_data；参数 frame、typ、nb_blocks；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VideoEncParamsCreateSideData(frame unsafe.Pointer, typ unsafe.Pointer, nb_blocks uint32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVideoEncParamsCreateSideData(frame, typ, nb_blocks)
}

// VideoHintAlloc 视频提示元数据操作（对 av_video_hint_alloc；参数 nb_rects、out_size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VideoHintAlloc(nb_rects uintptr, out_size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVideoHintAlloc(nb_rects, out_size)
}

// VideoHintCreateSideData 视频提示元数据操作（对 av_video_hint_create_side_data；参数 frame、nb_rects；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VideoHintCreateSideData(frame unsafe.Pointer, nb_rects uintptr) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVideoHintCreateSideData(frame, nb_rects)
}

// VkfmtFromPixfmt 按像素格式找 Vulkan 格式（对 av_vkfmt_from_pixfmt；参数 p；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VkfmtFromPixfmt(p int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVkfmtFromPixfmt(p)
}

// VkFrameAlloc 新建 Vulkan 帧（对 av_vk_frame_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) VkFrameAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVkFrameAlloc()
}

// Vlog 发一条日志（va_list 版）（对 av_vlog；参数 avcl、level、fmt、vl；按签名取回值；无状态，可用零值直接调）。
func (self *Util) Vlog(avcl unsafe.Pointer, level int32, fmt unsafe.Pointer, vl unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvVlog(avcl, level, fmt, vl)
}

// VorbisParseFrame 解析 Vorbis 帧头（对 av_vorbis_parse_frame；参数 s、buf、buf_size；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) VorbisParseFrame(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvVorbisParseFrame(s, buf, buf_size); ret < 0 {
		return codeErr("av_vorbis_parse_frame", ret)
	}
	return nil
}

// VorbisParseFrameFlags 解析 Vorbis 帧头（对 av_vorbis_parse_frame_flags；参数 s、buf、buf_size、flags；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) VorbisParseFrameFlags(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32, flags unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVorbisParseFrameFlags(s, buf, buf_size, flags)
}

// VorbisParseFree 解析 Vorbis 帧头（对 av_vorbis_parse_free；参数 s；按签名取回值；无状态，可用零值直接调）。
func (self *Util) VorbisParseFree(s *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvVorbisParseFree(s)
}

// VorbisParseInit 解析 Vorbis 帧头（对 av_vorbis_parse_init；参数 extradata、extradata_size；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) VorbisParseInit(extradata unsafe.Pointer, extradata_size int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvVorbisParseInit(extradata, extradata_size)
}

// VorbisParseReset 解析 Vorbis 帧头（对 av_vorbis_parse_reset；参数 s；按签名取回值；无状态，可用零值直接调）。
func (self *Util) VorbisParseReset(s unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvVorbisParseReset(s)
}

// WriteFrame 写一包数据，不做交织排序（对 av_write_frame；参数 s、pkt；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) WriteFrame(s unsafe.Pointer, pkt unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvWriteFrame(s, pkt); ret < 0 {
		return codeErr("av_write_frame", ret)
	}
	return nil
}

// WriteImageLine 写一行图像像素（对 av_write_image_line；参数 src、data、linesize、desc、x、y、c、w；按签名取回值；无状态，可用零值直接调）。
func (self *Muxer) WriteImageLine(src unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32) {
	mustUse(ensureModCrypto())
	fAvWriteImageLine(src, data, linesize, desc, x, y, c, w)
}

// WriteImageLine2 写一行图像像素（带元素大小）（对 av_write_image_line2；参数 src、data、linesize、desc、x、y、c、w、src_element_size；按签名取回值；无状态，可用零值直接调）。
func (self *Muxer) WriteImageLine2(src unsafe.Pointer, data unsafe.Pointer, linesize unsafe.Pointer, desc unsafe.Pointer, x int32, y int32, c int32, w int32, src_element_size int32) {
	mustUse(ensureModCrypto())
	fAvWriteImageLine2(src, data, linesize, desc, x, y, c, w, src_element_size)
}

// WriteTrailer 写文件尾并收尾（对 av_write_trailer；参数 s；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) WriteTrailer(s unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvWriteTrailer(s); ret < 0 {
		return codeErr("av_write_trailer", ret)
	}
	return nil
}

// WriteUncodedFrame 直接写一帧裸数据（对 av_write_uncoded_frame；参数 s、stream_index、frame；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) WriteUncodedFrame(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvWriteUncodedFrame(s, stream_index, frame); ret < 0 {
		return codeErr("av_write_uncoded_frame", ret)
	}
	return nil
}

// WriteUncodedFrameQuery 直接写一帧裸数据（对 av_write_uncoded_frame_query；参数 s、stream_index；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Muxer) WriteUncodedFrameQuery(s unsafe.Pointer, stream_index int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvWriteUncodedFrameQuery(s, stream_index); ret < 0 {
		return codeErr("av_write_uncoded_frame_query", ret)
	}
	return nil
}

// Xiphlacing Xiph 封包（打包重叠头）（对 av_xiphlacing；参数 s、v；回数值或个数；无状态，可用零值直接调）。
func (self *Util) Xiphlacing(s unsafe.Pointer, v uint32) uint32 {
	mustUse(ensureModCrypto())
	return fAvXiphlacing(s, v)
}

// XteaAlloc XTEA 加解密的小件（对 av_xtea_alloc；无参数；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态调用）。
func (self *Crypto) XteaAlloc() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvXteaAlloc()
}

// XteaCrypt XTEA 加解密的小件（对 av_xtea_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) XteaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvXteaCrypt(ctx, dst, src, count, iv, decrypt)
}

// XteaInit XTEA 加解密的小件（对 av_xtea_init；参数 ctx、key；按签名取回值；无状态调用）。
func (self *Crypto) XteaInit(ctx unsafe.Pointer, key unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvXteaInit(ctx, key)
}

// XteaLeCrypt XTEA 加解密的小件（对 av_xtea_le_crypt；参数 ctx、dst、src、count、iv、decrypt；按签名取回值；无状态调用）。
func (self *Crypto) XteaLeCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	mustUse(ensureModCrypto())
	fAvXteaLeCrypt(ctx, dst, src, count, iv, decrypt)
}

// XteaLeInit XTEA 加解密的小件（对 av_xtea_le_init；参数 ctx、key；按签名取回值；无状态调用）。
func (self *Crypto) XteaLeInit(ctx unsafe.Pointer, key unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvXteaLeInit(ctx, key)
}

// SwresampleConfiguration 问重采样库编译配置（对 swresample_configuration；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) SwresampleConfiguration() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fSwresampleConfiguration()
}

// SwresampleLicense 问重采样库许可证（对 swresample_license；无参数；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) SwresampleLicense() unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fSwresampleLicense()
}

// SwresampleVersion 问重采样库版本号（对 swresample_version；无参数；回数值；无状态，可用零值直接调）。
func (self *Util) SwresampleVersion() uint32 {
	mustUse(ensureModCrypto())
	return fSwresampleVersion()
}

// SwriAudioConvert 底层采样格式转换（对 swri_audio_convert；参数 ctx、out、in、len；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) SwriAudioConvert(ctx unsafe.Pointer, out unsafe.Pointer, in unsafe.Pointer, len int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fSwriAudioConvert(ctx, out, in, len); ret < 0 {
		return codeErr("swri_audio_convert", ret)
	}
	return nil
}

// SwriAudioConvertAlloc 新建底层采样转换器（对 swri_audio_convert_alloc；参数 out_fmt、in_fmt、channels、ch_map、flags；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) SwriAudioConvertAlloc(out_fmt unsafe.Pointer, in_fmt unsafe.Pointer, channels int32, ch_map unsafe.Pointer, flags int32) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fSwriAudioConvertAlloc(out_fmt, in_fmt, channels, ch_map, flags)
}

// SwriAudioConvertFree 释放底层采样转换器（对 swri_audio_convert_free；参数 ctx；按签名取回值；无状态，可用零值直接调）。
func (self *Util) SwriAudioConvertFree(ctx *unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fSwriAudioConvertFree(ctx)
}

// SwriResampleDspInit 初始化重采样 DSP 函数表（对 swri_resample_dsp_init；参数 c；按签名取回值；无状态，可用零值直接调）。
func (self *Util) SwriResampleDspInit(c unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fSwriResampleDspInit(c)
}

// SwriResampleDspX86Init 初始化 x86 重采样 DSP 函数表（对 swri_resample_dsp_x86_init；参数 c；按签名取回值；无状态，可用零值直接调）。
func (self *Util) SwriResampleDspX86Init(c unsafe.Pointer) {
	mustUse(ensureModCryptoHw())
	fSwriResampleDspX86Init(c)
}

// AddI adds two integers by value (av_add_i takes AVInteger BY VALUE).
func (self *Util) AddI(a AVInteger, b AVInteger) AVInteger {
	mustUse(ensureModCrypto())
	return fAvAddI(a, b)
}

// DynamicHdrVividAlloc 新建 Vivid HDR 动态元数据（对 av_dynamic_hdr_vivid_alloc；参数 size；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DynamicHdrVividAlloc(size unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDynamicHdrVividAlloc(size)
}

// DynamicHdrVividCreateSideData 给帧挂 Vivid HDR 元数据（对 av_dynamic_hdr_vivid_create_side_data；参数 frame；成功回 C 指针，失败回 nil；新建的记得调对应 Free；无状态，可用零值直接调）。
func (self *Util) DynamicHdrVividCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvDynamicHdrVividCreateSideData(frame)
}

// AvformatTransferInternalStreamTimingInfo 把内部流时间信息搬到新上下文（对 avformat_transfer_internal_stream_timing_info；参数 ofmt、ost、ist、copy_tb；回 C 指针，失败回 nil；无状态，可用零值直接调）。
func (self *Util) AvformatTransferInternalStreamTimingInfo(ofmt unsafe.Pointer, ost unsafe.Pointer, ist unsafe.Pointer, copy_tb unsafe.Pointer) unsafe.Pointer {
	mustUse(ensureModCrypto())
	return fAvformatTransferInternalStreamTimingInfo(ofmt, ost, ist, copy_tb)
}

// ImageCopy 算图片大小或拷图片平面（对 av_image_copy；参数 dst_data、dst_linesizes、src_data、src_linesizes、pix_fmt、width、height；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ImageCopy(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt int32, width int32, height int32) {
	mustUse(ensureModCrypto())
	fAvImageCopy(dst_data, dst_linesizes, src_data, src_linesizes, pix_fmt, width, height)
}

// ImageCopyPlaneUcFrom 算图片大小或拷图片平面（对 av_image_copy_plane_uc_from；参数 dst、dst_linesize、src、src_linesize、bytewidth、height；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ImageCopyPlaneUcFrom(dst unsafe.Pointer, dst_linesize uintptr, src unsafe.Pointer, src_linesize uintptr, bytewidth uintptr, height int32) {
	mustUse(ensureModCrypto())
	fAvImageCopyPlaneUcFrom(dst, dst_linesize, src, src_linesize, bytewidth, height)
}

// ImageCopyToBuffer 算图片大小或拷图片平面（对 av_image_copy_to_buffer；参数 dst、dst_size、src_data、src_linesize、pix_fmt、width、height、align；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ImageCopyToBuffer(dst unsafe.Pointer, dst_size int32, src_data unsafe.Pointer, src_linesize unsafe.Pointer, pix_fmt int32, width int32, height int32, align int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvImageCopyToBuffer(dst, dst_size, src_data, src_linesize, pix_fmt, width, height, align); ret < 0 {
		return codeErr("av_image_copy_to_buffer", ret)
	}
	return nil
}

// ImageCopyUcFrom 算图片大小或拷图片平面（对 av_image_copy_uc_from；参数 dst_data、dst_linesizes、src_data、src_linesizes、pix_fmt、width、height；按签名取回值；无状态，可用零值直接调）。
func (self *Util) ImageCopyUcFrom(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt int32, width int32, height int32) {
	mustUse(ensureModCrypto())
	fAvImageCopyUcFrom(dst_data, dst_linesizes, src_data, src_linesizes, pix_fmt, width, height)
}

// ImageFillArrays 算图片大小或拷图片平面（对 av_image_fill_arrays；参数 dst_data、dst_linesize、src、pix_fmt、width、height、align；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ImageFillArrays(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, src unsafe.Pointer, pix_fmt int32, width int32, height int32, align int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvImageFillArrays(dst_data, dst_linesize, src, pix_fmt, width, height, align); ret < 0 {
		return codeErr("av_image_fill_arrays", ret)
	}
	return nil
}

// ImageFillBlack 把整张图涂成黑色（对 av_image_fill_black；参数 dst_data、dst_linesize（ptrdiff 数组）、pix_fmt（像素格式枚举数）、colorRange（颜色范围枚举数）、width、height；回 nil 是成，负数是出错码；dst_data 传 nil 只试不写；width/height 传 0 会崩，得传真尺寸）。
func (self *Util) ImageFillBlack(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt int32, colorRange int32, width int32, height int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvImageFillBlack(dst_data, dst_linesize, pix_fmt, colorRange, width, height); ret < 0 {
		return codeErr("av_image_fill_black", ret)
	}
	return nil
}

// ImageFillColor 算图片大小或拷图片平面（对 av_image_fill_color；参数 dst_data、dst_linesize、pix_fmt、color、width、height、flags；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ImageFillColor(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt int32, color unsafe.Pointer, width int32, height int32, flags int32) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvImageFillColor(dst_data, dst_linesize, pix_fmt, color, width, height, flags); ret < 0 {
		return codeErr("av_image_fill_color", ret)
	}
	return nil
}

// ImageFillLinesizes 算图片大小或拷图片平面（对 av_image_fill_linesizes；参数 linesizes、pix_fmt、width；回数值或个数；无状态，可用零值直接调）。
func (self *Util) ImageFillLinesizes(linesizes unsafe.Pointer, pix_fmt int32, width int32) int32 {
	mustUse(ensureModCrypto())
	return fAvImageFillLinesizes(linesizes, pix_fmt, width)
}

// ImageFillMaxPixsteps 取像素格式每分量最大步长（对 av_image_fill_max_pixsteps；参数 max_pixsteps、max_pixstep_comps（各 4 个 int 的槽）、pixdesc（须是真像素描述，传 nil 会崩）；无回值，结果写进前两个槽）。
func (self *Util) ImageFillMaxPixsteps(max_pixsteps unsafe.Pointer, max_pixstep_comps unsafe.Pointer, pixdesc unsafe.Pointer) {
	mustUse(ensureModCrypto())
	fAvImageFillMaxPixsteps(max_pixsteps, max_pixstep_comps, pixdesc)
}

// ImageFillPlaneSizes 算图片大小或拷图片平面（对 av_image_fill_plane_sizes；参数 size、pix_fmt、height、linesizes；回数值；无状态，可用零值直接调）。
func (self *Util) ImageFillPlaneSizes(size unsafe.Pointer, pix_fmt int32, height int32, linesizes unsafe.Pointer) int32 {
	mustUse(ensureModCrypto())
	return fAvImageFillPlaneSizes(size, pix_fmt, height, linesizes)
}

// ImageFillPointers 算图片大小或拷图片平面（对 av_image_fill_pointers；参数 data、pix_fmt、height、ptr、linesizes；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ImageFillPointers(data unsafe.Pointer, pix_fmt int32, height int32, ptr unsafe.Pointer, linesizes unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvImageFillPointers(data, pix_fmt, height, ptr, linesizes); ret < 0 {
		return codeErr("av_image_fill_pointers", ret)
	}
	return nil
}

// ImageGetLinesize 算图片大小或拷图片平面（对 av_image_get_linesize；参数 pix_fmt、width、plane；回数值或个数；无状态，可用零值直接调）。
func (self *Util) ImageGetLinesize(pix_fmt int32, width int32, plane int32) int32 {
	mustUse(ensureModCrypto())
	return fAvImageGetLinesize(pix_fmt, width, plane)
}

// Log2 is floor(log2(v)) (av_log2 is a macro/inline over an unsigned int).
func (self *Util) Log2(v uint32) int32 {
	mustUse(ensureModCrypto())
	return fAvLog2(v)
}

// Log216bit is floor(log2(v)) for 16-bit values (av_log2_16bit).
func (self *Util) Log216bit(v uint32) int32 {
	mustUse(ensureModCrypto())
	return fAvLog216bit(v)
}

// Log2I is floor(log2(a)) for big integers (av_log2_i takes AVInteger BY VALUE).
func (self *Util) Log2I(a AVInteger) int32 {
	mustUse(ensureModCrypto())
	return fAvLog2I(a)
}

// ParseCpuCaps 解析 CPU 特性字符串（对 av_parse_cpu_caps；参数 flags、s；成功回 nil，失败回 error（字串已是人话）；无状态，可用零值直接调）。
func (self *Util) ParseCpuCaps(flags unsafe.Pointer, s unsafe.Pointer) error {
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if ret := fAvParseCpuCaps(flags, s); ret < 0 {
		return codeErr("av_parse_cpu_caps", ret)
	}
	return nil
}

// PixFmtCountPlanes 像素格式查名字查属性（对 av_pix_fmt_count_planes；参数 pix_fmt；回数值或个数；无状态，可用零值直接调）。
func (self *Util) PixFmtCountPlanes(pix_fmt int32) int32 {
	mustUse(ensureModCrypto())
	return fAvPixFmtCountPlanes(pix_fmt)
}

// PixFmtGetChromaSubSample 像素格式查名字查属性（对 av_pix_fmt_get_chroma_sub_sample；参数 pix_fmt、h_shift、v_shift；回数值或个数；无状态，可用零值直接调）。
func (self *Util) PixFmtGetChromaSubSample(pix_fmt int32, h_shift *int32, v_shift *int32) int32 {
	mustUse(ensureModCrypto())
	return fAvPixFmtGetChromaSubSample(pix_fmt, h_shift, v_shift)
}

// PixFmtSwapEndianness 像素格式查名字查属性（对 av_pix_fmt_swap_endianness；参数 pix_fmt；回数值或个数；无状态，可用零值直接调）。
func (self *Util) PixFmtSwapEndianness(pix_fmt int32) int32 {
	mustUse(ensureModCrypto())
	return fAvPixFmtSwapEndianness(pix_fmt)
}
