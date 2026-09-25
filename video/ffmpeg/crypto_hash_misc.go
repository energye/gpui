package ffmpeg

import (
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
	fAvAssert0Fpu                             func() unsafe.Pointer
	fAvBase64Decode                           func(out unsafe.Pointer, in unsafe.Pointer, out_size int32) unsafe.Pointer
	fAvBase64Encode                           func(out unsafe.Pointer, out_size int32, in unsafe.Pointer, in_size int32) unsafe.Pointer
	fAvBasename                               func(path unsafe.Pointer) unsafe.Pointer
	fAvBesselI0                               func(x float64) float64
	fAvBlowfishAlloc                          func() unsafe.Pointer
	fAvBlowfishCrypt                          func(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32)
	fAvBlowfishCryptEcb                       func(ctx unsafe.Pointer, xl unsafe.Pointer, xr unsafe.Pointer, decrypt int32)
	fAvBlowfishInit                           func(ctx unsafe.Pointer, key unsafe.Pointer, key_len int32)
	fAvBmgGet                                 func(lfg unsafe.Pointer, out float64)
	fAvBprintf                                func(src unsafe.Pointer, cb1 unsafe.Pointer) unsafe.Pointer
	fAvCalloc                                 func(arg0 unsafe.Pointer, cb1 unsafe.Pointer) unsafe.Pointer
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
	fAvCmpI                                   func(a unsafe.Pointer, b unsafe.Pointer) int32
	fAvCpbPropertiesAlloc                     func(size unsafe.Pointer) unsafe.Pointer
	fAvCrcGetTable                            func(crc_id unsafe.Pointer) unsafe.Pointer
	fAvCrc                                    func(ctx unsafe.Pointer, crc uint32, buf unsafe.Pointer, ln uintptr) uint32
	fAvCrcInit                                func(ctx unsafe.Pointer, le int32, bits int32, poly uint32, ctx_size int32) int32
	fAvCspApproximateTrcGamma                 func(trc unsafe.Pointer) float64
	fAvCspLumaCoeffsFromAvcsp                 func(csp unsafe.Pointer) unsafe.Pointer
	fAvCspPrimariesDescFromId                 func(prm unsafe.Pointer) unsafe.Pointer
	fAvCspPrimariesIdFromDesc                 func(prm unsafe.Pointer) unsafe.Pointer
	fAvCspTrcFuncFromId                       func(trc unsafe.Pointer) unsafe.Pointer
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
	fAvDispositionFromString                  func(disp unsafe.Pointer) unsafe.Pointer
	fAvDispositionToString                    func(disposition int32) unsafe.Pointer
	fAvDivI                                   func(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer
	fAvDivQ                                   func(b AVRational, c AVRational) AVRational
	fAvDownmixInfoUpdateSideData              func(frame unsafe.Pointer) unsafe.Pointer
	fAvDvCodecProfile                         func(width int32, height int32, pix_fmt unsafe.Pointer) unsafe.Pointer
	fAvDvCodecProfile2                        func(width int32, height int32, pix_fmt unsafe.Pointer, frame_rate AVRational) unsafe.Pointer
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
	fAvFileMap                                func(filename unsafe.Pointer, bufptr *unsafe.Pointer, size unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) unsafe.Pointer
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
	fAvGetBitsPerSample                       func(codec_id unsafe.Pointer) int32
	fAvGetCpuFlags                            func() unsafe.Pointer
	fAvGetExactBitsPerSample                  func(codec_id unsafe.Pointer) int32
	fAvGetFrameFilename                       func(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32) int32
	fAvGetFrameFilename2                      func(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32, flags int32) unsafe.Pointer
	fAvGetKnownColorName                      func(color_idx int32, rgb *unsafe.Pointer) unsafe.Pointer
	fAvGetMediaTypeString                     func(media_type int32) unsafe.Pointer
	fAvGetOutputTimestamp                     func(s unsafe.Pointer, stream int32, dts unsafe.Pointer, wall unsafe.Pointer) int32
	fAvGetPacket                              func(s unsafe.Pointer, pkt unsafe.Pointer, size int32) int32
	fAvGetPaddedBitsPerPixel                  func(pixdesc unsafe.Pointer) int32
	fAvGetPcmCodec                            func(fmt unsafe.Pointer, be int32) unsafe.Pointer
	fAvGetPictureTypeChar                     func(pict_type unsafe.Pointer) byte
	fAvGetProfileName                         func(codec unsafe.Pointer, profile int32) unsafe.Pointer
	fAvGetRandomSeed                          func() unsafe.Pointer
	fAvGetTimeBaseQ                           func() unsafe.Pointer
	fAvGetToken                               func(buf *unsafe.Pointer, term unsafe.Pointer) unsafe.Pointer
	fAvHashAlloc                              func(ctx *unsafe.Pointer, name unsafe.Pointer) int32
	fAvHashFinal                              func(ctx unsafe.Pointer, dst unsafe.Pointer)
	fAvHashFinalB64                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFinalBin                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFinalHex                           func(ctx unsafe.Pointer, dst unsafe.Pointer, size int32)
	fAvHashFreep                              func(ctx *unsafe.Pointer)
	fAvHashGetName                            func(ctx unsafe.Pointer) unsafe.Pointer
	fAvHashGetSize                            func(ctx unsafe.Pointer) unsafe.Pointer
	fAvHashInit                               func(ctx unsafe.Pointer)
	fAvHashNames                              func(i int32) unsafe.Pointer
	fAvHashUpdate                             func(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr)
	fAvHexDump                                func(f unsafe.Pointer, buf unsafe.Pointer, size int32)
	fAvHexDumpLog                             func(avcl unsafe.Pointer, level int32, buf unsafe.Pointer, size int32)
	fAvHmacAlloc                              func(typ unsafe.Pointer) unsafe.Pointer
	fAvHmacCalc                               func(ctx unsafe.Pointer, data unsafe.Pointer, len uint32, key unsafe.Pointer, keylen uint32, out unsafe.Pointer, outlen uint32) int32
	fAvHmacFinal                              func(ctx unsafe.Pointer, out unsafe.Pointer, outlen uint32) int32
	fAvHmacFree                               func(ctx unsafe.Pointer)
	fAvHmacInit                               func(ctx unsafe.Pointer, key unsafe.Pointer, keylen uint32)
	fAvHmacUpdate                             func(ctx unsafe.Pointer, data unsafe.Pointer, len uint32)
	fAvHwdeviceCtxAlloc                       func(typ unsafe.Pointer) unsafe.Pointer
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
	fAvHwframeCtxCreateDerived                func(derived_frame_ctx *unsafe.Pointer, format unsafe.Pointer, derived_device_ctx unsafe.Pointer, source_frame_ctx unsafe.Pointer, flags int32) int32
	fAvHwframeCtxInit                         func(ref unsafe.Pointer) int32
	fAvHwframeGetBuffer                       func(hwframe_ctx unsafe.Pointer, frame unsafe.Pointer, flags int32) int32
	fAvHwframeMap                             func(dst unsafe.Pointer, src unsafe.Pointer, flags int32) int32
	fAvHwframeTransferData                    func(dst unsafe.Pointer, src unsafe.Pointer, flags int32) int32
	fAvHwframeTransferGetFormats              func(hwframe_ctx unsafe.Pointer, dir unsafe.Pointer, formats *unsafe.Pointer, flags int32) int32
	fAvI2int                                  func(a unsafe.Pointer) int64
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
	fAvInt2i                                  func(a int64) unsafe.Pointer
	fAvIntListLengthForSize                   func(elsize uint32, list unsafe.Pointer, term uint64) uint32
	fAvInterleavedWriteFrame                  func(s unsafe.Pointer, pkt unsafe.Pointer) int32
	fAvInterleavedWriteUncodedFrame           func(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) int32
	fAvJniGetJavaVm                           func(log_ctx unsafe.Pointer) unsafe.Pointer
	fAvJniSetJavaVm                           func(vm unsafe.Pointer, log_ctx unsafe.Pointer) unsafe.Pointer
	fAvLfgInit                                func(c unsafe.Pointer, seed uint32)
	fAvLfgInitFromData                        func(c unsafe.Pointer, data unsafe.Pointer, length uint32) int32
	fAvLog                                    func(arg0 unsafe.Pointer, arg1 unsafe.Pointer, cb2 unsafe.Pointer, cb3 unsafe.Pointer) unsafe.Pointer
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
	fAvModI                                   func(quot unsafe.Pointer, a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer
	fAvMulI                                   func(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer
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
	fAvReadImageLine                          func(dst unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32)
	fAvReadImageLine2                         func(dst unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32, dst_element_size int32)
	fAvReadPause                              func(s unsafe.Pointer) int32
	fAvReadPlay                               func(s unsafe.Pointer) int32
	fAvReduce                                 func(dst_num unsafe.Pointer, dst_den unsafe.Pointer, num int64, den int64, max int64) int32
	fAvRipemdAlloc                            func() unsafe.Pointer
	fAvRipemdFinal                            func(context unsafe.Pointer, digest unsafe.Pointer)
	fAvRipemdInit                             func(context unsafe.Pointer, bits int32) int32
	fAvRipemdUpdate                           func(context unsafe.Pointer, data unsafe.Pointer, len uintptr)
	fAvSamplesAlloc                           func(audio_data *unsafe.Pointer, linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) int32
	fAvSamplesAllocArrayAndSamples            func(audio_data *unsafe.Pointer, linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) int32
	fAvSamplesCopy                            func(dst unsafe.Pointer, src unsafe.Pointer, dst_offset int32, src_offset int32, nb_samples int32, nb_channels int32, sample_fmt unsafe.Pointer) int32
	fAvSamplesFillArrays                      func(audio_data *unsafe.Pointer, linesize unsafe.Pointer, buf unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) int32
	fAvSamplesGetBufferSize                   func(linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) int32
	fAvSamplesSetSilence                      func(audio_data unsafe.Pointer, offset int32, nb_samples int32, nb_channels int32, sample_fmt unsafe.Pointer) int32
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
	fAvShrI                                   func(a unsafe.Pointer, s int32) unsafe.Pointer
	fAvSizeMult                               func(a uintptr, b uintptr, r unsafe.Pointer) int32
	fAvSmallStrptime                          func(p unsafe.Pointer, fmt unsafe.Pointer, dt unsafe.Pointer) unsafe.Pointer
	fAvSscanf                                 func(str unsafe.Pointer, format unsafe.Pointer) int32
	fAvStrcasecmp                             func(a unsafe.Pointer, b unsafe.Pointer) int32
	fAvStrdup                                 func(arg0 unsafe.Pointer) unsafe.Pointer
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
	fAvStrstart                               func(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) unsafe.Pointer
	fAvStrtod                                 func(numstr unsafe.Pointer, tail *unsafe.Pointer) float64
	fAvStrtok                                 func(s unsafe.Pointer, delim unsafe.Pointer, saveptr *unsafe.Pointer) unsafe.Pointer
	fAvSubI                                   func(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer
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
	fAvTimegm                                 func(tm unsafe.Pointer) unsafe.Pointer
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
	fAvUtf8Decode                             func(codep unsafe.Pointer, bufp *unsafe.Pointer, buf_end unsafe.Pointer, flags uint32) unsafe.Pointer
	fAvUuidParse                              func(in unsafe.Pointer, uu unsafe.Pointer) int32
	fAvUuidParseRange                         func(in_start unsafe.Pointer, in_end unsafe.Pointer, uu unsafe.Pointer) int32
	fAvUuidUnparse                            func(uu unsafe.Pointer, out unsafe.Pointer)
	fAvUuidUrnParse                           func(in unsafe.Pointer, uu unsafe.Pointer) int32
	fAvVbprintf                               func(buf unsafe.Pointer, fmt unsafe.Pointer, vl_arg unsafe.Pointer)
	fAvVdpauAllocContext                      func() unsafe.Pointer
	fAvVdpauBindContext                       func(avctx unsafe.Pointer, device unsafe.Pointer, get_proc_address unsafe.Pointer, flags uint32) unsafe.Pointer
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
	fAvWriteImageLine                         func(src unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32)
	fAvWriteImageLine2                        func(src unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, src_element_size int32)
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
	fAvAddI                                   func(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrVividAlloc                   func(size unsafe.Pointer) unsafe.Pointer
	fAvDynamicHdrVividCreateSideData          func(frame unsafe.Pointer) unsafe.Pointer
	fAvformatTransferInternalStreamTimingInfo func(ofmt unsafe.Pointer, ost unsafe.Pointer, ist unsafe.Pointer, copy_tb unsafe.Pointer) unsafe.Pointer
	fAvImageCopy                              func(dst_data unsafe.Pointer, dst_linesizes int32, src_data unsafe.Pointer, src_linesizes int32, pix_fmt unsafe.Pointer, width int32, height int32)
	fAvImageCopyPlaneUcFrom                   func(dst unsafe.Pointer, dst_linesize unsafe.Pointer, src unsafe.Pointer, src_linesize unsafe.Pointer, bytewidth unsafe.Pointer, height int32)
	fAvImageCopyToBuffer                      func(dst unsafe.Pointer, dst_size int32, src_data unsafe.Pointer, src_linesize int32, pix_fmt unsafe.Pointer, width int32, height int32, align int32) int32
	fAvImageCopyUcFrom                        func(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt unsafe.Pointer, width int32, height int32)
	fAvImageFillArrays                        func(dst_data unsafe.Pointer, dst_linesize int32, src unsafe.Pointer, pix_fmt unsafe.Pointer, width int32, height int32, align int32) int32
	fAvImageFillBlack                         func(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt unsafe.Pointer, rng unsafe.Pointer, width int32, height int32) int32
	fAvImageFillColor                         func(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt unsafe.Pointer, color uint32, width int32, height int32, flags int32) int32
	fAvImageFillLinesizes                     func(linesizes int32, pix_fmt unsafe.Pointer, width int32) int32
	fAvImageFillMaxPixsteps                   func(max_pixsteps int32, max_pixstep_comps int32, pixdesc unsafe.Pointer) unsafe.Pointer
	fAvImageFillPlaneSizes                    func(size uintptr, pix_fmt unsafe.Pointer, height int32, linesizes unsafe.Pointer) int32
	fAvImageFillPointers                      func(data unsafe.Pointer, pix_fmt unsafe.Pointer, height int32, ptr unsafe.Pointer, linesizes int32) int32
	fAvImageGetLinesize                       func(pix_fmt unsafe.Pointer, width int32, plane int32) int32
	fAvLog2                                   func(cb0 unsafe.Pointer) unsafe.Pointer
	fAvLog216bit                              func(v uint32) unsafe.Pointer
	fAvLog2I                                  func(a unsafe.Pointer) int32
	fAvParseCpuCaps                           func(flags unsafe.Pointer, s unsafe.Pointer) int32
	fAvPixFmtCountPlanes                      func(pix_fmt int32) int32
	fAvPixFmtGetChromaSubSample               func(pix_fmt int32, h_shift *int32, v_shift *int32) int32
	fAvPixFmtSwapEndianness                   func(pix_fmt int32) int32
)

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
	purego.RegisterLibFunc(&fAvAllocVdpaucontext, h, "av_alloc_vdpaucontext")
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
	purego.RegisterLibFunc(&fAvVdpauAllocContext, h, "av_vdpau_alloc_context")
	purego.RegisterLibFunc(&fAvVdpauBindContext, h, "av_vdpau_bind_context")
	purego.RegisterLibFunc(&fAvVdpauGetSurfaceParameters, h, "av_vdpau_get_surface_parameters")
	purego.RegisterLibFunc(&fAvVdpauHwaccelGetRender2, h, "av_vdpau_hwaccel_get_render2")
	purego.RegisterLibFunc(&fAvVdpauHwaccelSetRender2, h, "av_vdpau_hwaccel_set_render2")
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
	purego.RegisterLibFunc(&fSwriResampleDspX86Init, h, "swri_resample_dsp_x86_init")
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

func (self *Util) Ac3ParseHeader(buf unsafe.Pointer, size uintptr, bitstream_id unsafe.Pointer, frame_size unsafe.Pointer) unsafe.Pointer {
	return fAvAc3ParseHeader(buf, size, bitstream_id, frame_size)
}

func (self *Util) AdtsHeaderParse(buf unsafe.Pointer, samples unsafe.Pointer, frames unsafe.Pointer) unsafe.Pointer {
	return fAvAdtsHeaderParse(buf, samples, frames)
}

func (self *Crypto) AesAlloc() unsafe.Pointer {
	return fAvAesAlloc()
}

func (self *Crypto) AesCrypt(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvAesCrypt(a, dst, src, count, iv, decrypt)
}

func (self *Crypto) AesCtrAlloc() unsafe.Pointer {
	return fAvAesCtrAlloc()
}

func (self *Crypto) AesCtrCrypt(a unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, size int32) {
	fAvAesCtrCrypt(a, dst, src, size)
}

func (self *Crypto) AesCtrFree(a unsafe.Pointer) {
	fAvAesCtrFree(a)
}

func (self *Crypto) AesCtrGetIv(a unsafe.Pointer) unsafe.Pointer {
	return fAvAesCtrGetIv(a)
}

func (self *Crypto) AesCtrIncrementIv(a unsafe.Pointer) {
	fAvAesCtrIncrementIv(a)
}

func (self *Crypto) AesCtrInit(a unsafe.Pointer, key unsafe.Pointer) error {
	if ret := fAvAesCtrInit(a, key); ret < 0 {
		return codeErr("av_aes_ctr_init", ret)
	}
	return nil
}

func (self *Crypto) AesCtrSetFullIv(a unsafe.Pointer, iv unsafe.Pointer) {
	fAvAesCtrSetFullIv(a, iv)
}

func (self *Crypto) AesCtrSetIv(a unsafe.Pointer, iv unsafe.Pointer) {
	fAvAesCtrSetIv(a, iv)
}

func (self *Crypto) AesCtrSetRandomIv(a unsafe.Pointer) {
	fAvAesCtrSetRandomIv(a)
}

func (self *Crypto) AesInit(a unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if ret := fAvAesInit(a, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_aes_init", ret)
	}
	return nil
}

func (self *HWDevice) AllocVdpaucontext() unsafe.Pointer {
	return fAvAllocVdpaucontext()
}

func (self *Util) AppendPacket(s unsafe.Pointer, pkt unsafe.Pointer, size int32) error {
	if ret := fAvAppendPacket(s, pkt, size); ret < 0 {
		return codeErr("av_append_packet", ret)
	}
	return nil
}

func (self *Util) AppendPathComponent(path unsafe.Pointer, component unsafe.Pointer) unsafe.Pointer {
	return fAvAppendPathComponent(path, component)
}

func (self *Util) Assert0Fpu() unsafe.Pointer {
	return fAvAssert0Fpu()
}

func (self *Crypto) Base64Decode(out unsafe.Pointer, in unsafe.Pointer, out_size int32) unsafe.Pointer {
	return fAvBase64Decode(out, in, out_size)
}

func (self *Crypto) Base64Encode(out unsafe.Pointer, out_size int32, in unsafe.Pointer, in_size int32) unsafe.Pointer {
	return fAvBase64Encode(out, out_size, in, in_size)
}

func (self *Util) Basename(path unsafe.Pointer) unsafe.Pointer {
	return fAvBasename(path)
}

func (self *Util) BesselI0(x float64) float64 {
	return fAvBesselI0(x)
}

func (self *Crypto) BlowfishAlloc() unsafe.Pointer {
	return fAvBlowfishAlloc()
}

func (self *Crypto) BlowfishCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvBlowfishCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) BlowfishCryptEcb(ctx unsafe.Pointer, xl unsafe.Pointer, xr unsafe.Pointer, decrypt int32) {
	fAvBlowfishCryptEcb(ctx, xl, xr, decrypt)
}

func (self *Crypto) BlowfishInit(ctx unsafe.Pointer, key unsafe.Pointer, key_len int32) {
	fAvBlowfishInit(ctx, key, key_len)
}

func (self *Util) BmgGet(lfg unsafe.Pointer, out float64) {
	fAvBmgGet(lfg, out)
}

func (self *Util) Bprintf(src unsafe.Pointer, cb1 unsafe.Pointer) unsafe.Pointer {
	return fAvBprintf(src, cb1)
}

func (self *Util) Calloc(arg0 unsafe.Pointer, cb1 unsafe.Pointer) unsafe.Pointer {
	return fAvCalloc(arg0, cb1)
}

func (self *Crypto) CamelliaAlloc() unsafe.Pointer {
	return fAvCamelliaAlloc()
}

func (self *Crypto) CamelliaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvCamelliaCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) CamelliaInit(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if ret := fAvCamelliaInit(ctx, key, key_bits); ret < 0 {
		return codeErr("av_camellia_init", ret)
	}
	return nil
}

func (self *Crypto) Cast5Alloc() unsafe.Pointer {
	return fAvCast5Alloc()
}

func (self *Crypto) Cast5Crypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, decrypt int32) {
	fAvCast5Crypt(ctx, dst, src, count, decrypt)
}

func (self *Crypto) Cast5Crypt2(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvCast5Crypt2(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) Cast5Init(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if ret := fAvCast5Init(ctx, key, key_bits); ret < 0 {
		return codeErr("av_cast5_init", ret)
	}
	return nil
}

func (self *Util) ChromaLocationEnumToPos(xpos *int32, ypos *int32, pos int32) error {
	if ret := fAvChromaLocationEnumToPos(xpos, ypos, pos); ret < 0 {
		return codeErr("av_chroma_location_enum_to_pos", ret)
	}
	return nil
}

func (self *Util) ChromaLocationFromName(name string) error {
	if ret := fAvChromaLocationFromName(name); ret < 0 {
		return codeErr("av_chroma_location_from_name", ret)
	}
	return nil
}

func (self *Util) ChromaLocationName(location int32) unsafe.Pointer {
	return fAvChromaLocationName(location)
}

func (self *Util) ChromaLocationPosToEnum(xpos int32, ypos int32) int32 {
	return fAvChromaLocationPosToEnum(xpos, ypos)
}

func (self *Util) CmpI(a unsafe.Pointer, b unsafe.Pointer) error {
	if ret := fAvCmpI(a, b); ret < 0 {
		return codeErr("av_cmp_i", ret)
	}
	return nil
}

func (self *Util) CpbPropertiesAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvCpbPropertiesAlloc(size)
}

func (self *Crypto) CrcGetTable(crc_id unsafe.Pointer) unsafe.Pointer {
	return fAvCrcGetTable(crc_id)
}

func (self *Crypto) CrcInit(ctx unsafe.Pointer, le int32, bits int32, poly uint32, ctx_size int32) error {
	if ret := fAvCrcInit(ctx, le, bits, poly, ctx_size); ret < 0 {
		return codeErr("av_crc_init", ret)
	}
	return nil
}

// Adler32Update extends an Adler-32 checksum over ln bytes at buf
// (pass the previous result back in as adler to chain buffers).
func (self *Crypto) Adler32Update(adler uint32, buf unsafe.Pointer, ln uintptr) uint32 {
	return fAvAdler32Update(adler, buf, ln)
}

// Crc updates a CRC checksum over ln bytes at buf using a table
// prepared by CrcInit (pass the previous result back in as crc to chain).
func (self *Crypto) Crc(ctx unsafe.Pointer, crc uint32, buf unsafe.Pointer, ln uintptr) uint32 {
	return fAvCrc(ctx, crc, buf, ln)
}

func (self *Util) CspApproximateTrcGamma(trc unsafe.Pointer) float64 {
	return fAvCspApproximateTrcGamma(trc)
}

func (self *Util) CspLumaCoeffsFromAvcsp(csp unsafe.Pointer) unsafe.Pointer {
	return fAvCspLumaCoeffsFromAvcsp(csp)
}

func (self *Util) CspPrimariesDescFromId(prm unsafe.Pointer) unsafe.Pointer {
	return fAvCspPrimariesDescFromId(prm)
}

func (self *Util) CspPrimariesIdFromDesc(prm unsafe.Pointer) unsafe.Pointer {
	return fAvCspPrimariesIdFromDesc(prm)
}

func (self *Util) CspTrcFuncFromId(trc unsafe.Pointer) unsafe.Pointer {
	return fAvCspTrcFuncFromId(trc)
}

func (self *Util) D2q(d float64, max int32) AVRational {
	return fAvD2q(d, max)
}

func (self *Util) D3d11vaAllocContext() unsafe.Pointer {
	return fAvD3d11vaAllocContext()
}

func (self *Util) DctCalc(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
	return fAvDctCalc(s, data)
}

func (self *Util) DctEnd(s unsafe.Pointer) unsafe.Pointer {
	return fAvDctEnd(s)
}

func (self *Util) DctInit(nbits int32, typ unsafe.Pointer) unsafe.Pointer {
	return fAvDctInit(nbits, typ)
}

func (self *Util) DefaultGetCategory(ptr unsafe.Pointer) unsafe.Pointer {
	return fAvDefaultGetCategory(ptr)
}

func (self *Util) DefaultItemName(ctx unsafe.Pointer) unsafe.Pointer {
	return fAvDefaultItemName(ctx)
}

func (self *Util) DemuxerIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	return fAvDemuxerIterate(opaque)
}

func (self *Crypto) DesAlloc() unsafe.Pointer {
	return fAvDesAlloc()
}

func (self *Crypto) DesCrypt(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvDesCrypt(d, dst, src, count, iv, decrypt)
}

func (self *Crypto) DesInit(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if ret := fAvDesInit(d, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_des_init", ret)
	}
	return nil
}

func (self *Crypto) DesMac(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32) {
	fAvDesMac(d, dst, src, count)
}

func (self *Util) DetectionBboxAlloc(nb_bboxes uint32, out_size unsafe.Pointer) unsafe.Pointer {
	return fAvDetectionBboxAlloc(nb_bboxes, out_size)
}

func (self *Util) DetectionBboxCreateSideData(frame unsafe.Pointer, nb_bboxes uint32) unsafe.Pointer {
	return fAvDetectionBboxCreateSideData(frame, nb_bboxes)
}

func (self *Util) DiracParseSequenceHeader(dsh *unsafe.Pointer, buf unsafe.Pointer, buf_size uintptr, log_ctx unsafe.Pointer) error {
	if ret := fAvDiracParseSequenceHeader(dsh, buf, buf_size, log_ctx); ret < 0 {
		return codeErr("av_dirac_parse_sequence_header", ret)
	}
	return nil
}

func (self *Util) Dirname(path unsafe.Pointer) unsafe.Pointer {
	return fAvDirname(path)
}

func (self *Util) DispositionFromString(disp unsafe.Pointer) unsafe.Pointer {
	return fAvDispositionFromString(disp)
}

func (self *Util) DispositionToString(disposition int32) unsafe.Pointer {
	return fAvDispositionToString(disposition)
}

func (self *Util) DivI(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer {
	return fAvDivI(a, b)
}

func (self *Util) DivQ(b AVRational, c AVRational) AVRational {
	return fAvDivQ(b, c)
}

func (self *Util) DownmixInfoUpdateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvDownmixInfoUpdateSideData(frame)
}

func (self *Util) DvCodecProfile(width int32, height int32, pix_fmt unsafe.Pointer) unsafe.Pointer {
	return fAvDvCodecProfile(width, height, pix_fmt)
}

func (self *Util) DvCodecProfile2(width int32, height int32, pix_fmt unsafe.Pointer, frame_rate AVRational) unsafe.Pointer {
	return fAvDvCodecProfile2(width, height, pix_fmt, frame_rate)
}

func (self *Util) DvFrameProfile(sys unsafe.Pointer, frame unsafe.Pointer, buf_size uint32) unsafe.Pointer {
	return fAvDvFrameProfile(sys, frame, buf_size)
}

func (self *Util) Dynarray2Add(tab_ptr *unsafe.Pointer, nb_ptr unsafe.Pointer, elem_size uintptr, elem_data unsafe.Pointer) unsafe.Pointer {
	return fAvDynarray2Add(tab_ptr, nb_ptr, elem_size, elem_data)
}

func (self *Util) DynarrayAdd(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer) {
	fAvDynarrayAdd(tab_ptr, nb_ptr, elem)
}

func (self *Util) DynarrayAddNofree(tab_ptr unsafe.Pointer, nb_ptr unsafe.Pointer, elem unsafe.Pointer) error {
	if ret := fAvDynarrayAddNofree(tab_ptr, nb_ptr, elem); ret < 0 {
		return codeErr("av_dynarray_add_nofree", ret)
	}
	return nil
}

func (self *Util) EncryptionInfoAddSideData(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer {
	return fAvEncryptionInfoAddSideData(info, side_data_size)
}

func (self *Util) EncryptionInfoAlloc(subsample_count uint32, key_id_size uint32, iv_size uint32) unsafe.Pointer {
	return fAvEncryptionInfoAlloc(subsample_count, key_id_size, iv_size)
}

func (self *Util) EncryptionInfoClone(info unsafe.Pointer) unsafe.Pointer {
	return fAvEncryptionInfoClone(info)
}

func (self *Util) EncryptionInfoFree(info unsafe.Pointer) {
	fAvEncryptionInfoFree(info)
}

func (self *Util) EncryptionInfoGetSideData(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer {
	return fAvEncryptionInfoGetSideData(side_data, side_data_size)
}

func (self *Util) EncryptionInitInfoAddSideData(info unsafe.Pointer, side_data_size unsafe.Pointer) unsafe.Pointer {
	return fAvEncryptionInitInfoAddSideData(info, side_data_size)
}

func (self *Util) EncryptionInitInfoAlloc(system_id_size uint32, num_key_ids uint32, key_id_size uint32, data_size uint32) unsafe.Pointer {
	return fAvEncryptionInitInfoAlloc(system_id_size, num_key_ids, key_id_size, data_size)
}

func (self *Util) EncryptionInitInfoFree(info unsafe.Pointer) {
	fAvEncryptionInitInfoFree(info)
}

func (self *Util) EncryptionInitInfoGetSideData(side_data unsafe.Pointer, side_data_size uintptr) unsafe.Pointer {
	return fAvEncryptionInitInfoGetSideData(side_data, side_data_size)
}

func (self *Util) Escape(dst *unsafe.Pointer, src unsafe.Pointer, special_chars unsafe.Pointer, mode unsafe.Pointer, flags int32) unsafe.Pointer {
	return fAvEscape(dst, src, special_chars, mode, flags)
}

func (self *Util) ExecutorAlloc(callbacks unsafe.Pointer, thread_count int32) unsafe.Pointer {
	return fAvExecutorAlloc(callbacks, thread_count)
}

func (self *Util) ExecutorExecute(e unsafe.Pointer, t unsafe.Pointer) {
	fAvExecutorExecute(e, t)
}

func (self *Util) ExecutorFree(e *unsafe.Pointer) {
	fAvExecutorFree(e)
}

func (self *Util) ExprCountFunc(e unsafe.Pointer, counter unsafe.Pointer, size int32, arg int32) int32 {
	return fAvExprCountFunc(e, counter, size, arg)
}

func (self *Util) ExprCountVars(e unsafe.Pointer, counter unsafe.Pointer, size int32) int32 {
	return fAvExprCountVars(e, counter, size)
}

func (self *Util) ExprEval(e unsafe.Pointer, const_values unsafe.Pointer, opaque unsafe.Pointer) float64 {
	return fAvExprEval(e, const_values, opaque)
}

func (self *Util) ExprFree(e unsafe.Pointer) {
	fAvExprFree(e)
}

func (self *Util) ExprParse(expr *unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, func1_names unsafe.Pointer, cb4 unsafe.Pointer, func2_names unsafe.Pointer, cb6 unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) error {
	if ret := fAvExprParse(expr, s, const_names, func1_names, cb4, func2_names, cb6, log_offset, log_ctx); ret < 0 {
		return codeErr("av_expr_parse", ret)
	}
	return nil
}

func (self *Util) ExprParseAndEval(res unsafe.Pointer, s unsafe.Pointer, const_names unsafe.Pointer, const_values unsafe.Pointer, func1_names unsafe.Pointer, cb5 unsafe.Pointer, func2_names unsafe.Pointer, cb7 unsafe.Pointer, opaque unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) error {
	if ret := fAvExprParseAndEval(res, s, const_names, const_values, func1_names, cb5, func2_names, cb7, opaque, log_offset, log_ctx); ret < 0 {
		return codeErr("av_expr_parse_and_eval", ret)
	}
	return nil
}

func (self *Util) FastMalloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	fAvFastMalloc(ptr, size, min_size)
}

func (self *Util) FastMallocz(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	fAvFastMallocz(ptr, size, min_size)
}

func (self *Util) FastPaddedMalloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	fAvFastPaddedMalloc(ptr, size, min_size)
}

func (self *Util) FastPaddedMallocz(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) {
	fAvFastPaddedMallocz(ptr, size, min_size)
}

func (self *Util) FastRealloc(ptr unsafe.Pointer, size unsafe.Pointer, min_size uintptr) unsafe.Pointer {
	return fAvFastRealloc(ptr, size, min_size)
}

func (self *Util) FftCalc(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer {
	return fAvFftCalc(s, z)
}

func (self *Util) FftEnd(s unsafe.Pointer) unsafe.Pointer {
	return fAvFftEnd(s)
}

func (self *Util) FftInit(nbits int32, inverse int32) unsafe.Pointer {
	return fAvFftInit(nbits, inverse)
}

func (self *Util) FftPermute(s unsafe.Pointer, z unsafe.Pointer) unsafe.Pointer {
	return fAvFftPermute(s, z)
}

func (self *Util) FileMap(filename unsafe.Pointer, bufptr *unsafe.Pointer, size unsafe.Pointer, log_offset int32, log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvFileMap(filename, bufptr, size, log_offset, log_ctx)
}

func (self *Util) FilenameNumberTest(filename unsafe.Pointer) error {
	if ret := fAvFilenameNumberTest(filename); ret < 0 {
		return codeErr("av_filename_number_test", ret)
	}
	return nil
}

func (self *Util) FileUnmap(bufptr unsafe.Pointer, size uintptr) {
	fAvFileUnmap(bufptr, size)
}

func (self *Util) FilterIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	return fAvFilterIterate(opaque)
}

func (self *Prober) FindBestPixFmtOf2(dst_pix_fmt1 int32, dst_pix_fmt2 int32, src_pix_fmt int32, has_alpha int32, loss_ptr *int32) int32 {
	return fAvFindBestPixFmtOf2(dst_pix_fmt1, dst_pix_fmt2, src_pix_fmt, has_alpha, loss_ptr)
}

func (self *Util) FindDefaultStreamIndex(s unsafe.Pointer) error {
	if ret := fAvFindDefaultStreamIndex(s); ret < 0 {
		return codeErr("av_find_default_stream_index", ret)
	}
	return nil
}

func (self *Util) FindInfoTag(arg unsafe.Pointer, arg_size int32, tag1 unsafe.Pointer, info unsafe.Pointer) error {
	if ret := fAvFindInfoTag(arg, arg_size, tag1, info); ret < 0 {
		return codeErr("av_find_info_tag", ret)
	}
	return nil
}

func (self *Prober) FindInputFormat(short_name unsafe.Pointer) unsafe.Pointer {
	return fAvFindInputFormat(short_name)
}

func (self *Util) FindNearestQIdx(q AVRational, q_list unsafe.Pointer) error {
	if ret := fAvFindNearestQIdx(q, q_list); ret < 0 {
		return codeErr("av_find_nearest_q_idx", ret)
	}
	return nil
}

func (self *Util) FindProgramFromStream(ic unsafe.Pointer, last unsafe.Pointer, s int32) unsafe.Pointer {
	return fAvFindProgramFromStream(ic, last, s)
}

func (self *Util) FmtCtxGetDurationEstimationMethod(ctx unsafe.Pointer) unsafe.Pointer {
	return fAvFmtCtxGetDurationEstimationMethod(ctx)
}

func (self *Util) ForceCpuFlags(flags int32) {
	fAvForceCpuFlags(flags)
}

func (self *Util) FormatInjectGlobalSideData(s unsafe.Pointer) {
	fAvFormatInjectGlobalSideData(s)
}

func (self *Util) FourccMakeString(buf unsafe.Pointer, fourcc uint32) unsafe.Pointer {
	return fAvFourccMakeString(buf, fourcc)
}

func (self *Util) Gcd(a int64, b int64) int64 {
	return fAvGcd(a, b)
}

func (self *Util) GcdQ(a AVRational, b AVRational, max_den int32, def AVRational) AVRational {
	return fAvGcdQ(a, b, max_den, def)
}

func (self *Util) GetAltSampleFmt(sample_fmt int32, planar int32) int32 {
	return fAvGetAltSampleFmt(sample_fmt, planar)
}

func (self *Samples) GetAudioFrameDuration(avctx unsafe.Pointer, frame_bytes int32) int32 {
	return fAvGetAudioFrameDuration(avctx, frame_bytes)
}

func (self *Samples) GetAudioFrameDuration2(par unsafe.Pointer, frame_bytes int32) int32 {
	return fAvGetAudioFrameDuration2(par, frame_bytes)
}

func (self *Util) GetBitsPerSample(codec_id unsafe.Pointer) int32 {
	return fAvGetBitsPerSample(codec_id)
}

func (self *Util) GetCpuFlags() unsafe.Pointer {
	return fAvGetCpuFlags()
}

func (self *Util) GetExactBitsPerSample(codec_id unsafe.Pointer) int32 {
	return fAvGetExactBitsPerSample(codec_id)
}

func (self *Util) GetFrameFilename(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32) int32 {
	return fAvGetFrameFilename(buf, buf_size, path, number)
}

func (self *Util) GetFrameFilename2(buf unsafe.Pointer, buf_size int32, path unsafe.Pointer, number int32, flags int32) unsafe.Pointer {
	return fAvGetFrameFilename2(buf, buf_size, path, number, flags)
}

func (self *Util) GetKnownColorName(color_idx int32, rgb *unsafe.Pointer) unsafe.Pointer {
	return fAvGetKnownColorName(color_idx, rgb)
}

func (self *Util) GetMediaTypeString(media_type int32) unsafe.Pointer {
	return fAvGetMediaTypeString(media_type)
}

func (self *Util) GetOutputTimestamp(s unsafe.Pointer, stream int32, dts unsafe.Pointer, wall unsafe.Pointer) int32 {
	return fAvGetOutputTimestamp(s, stream, dts, wall)
}

func (self *Util) GetPacket(s unsafe.Pointer, pkt unsafe.Pointer, size int32) int32 {
	return fAvGetPacket(s, pkt, size)
}

func (self *Util) GetPaddedBitsPerPixel(pixdesc unsafe.Pointer) int32 {
	return fAvGetPaddedBitsPerPixel(pixdesc)
}

func (self *Util) GetPcmCodec(fmt unsafe.Pointer, be int32) unsafe.Pointer {
	return fAvGetPcmCodec(fmt, be)
}

func (self *Util) GetPictureTypeChar(pict_type unsafe.Pointer) byte {
	return fAvGetPictureTypeChar(pict_type)
}

func (self *Util) GetProfileName(codec unsafe.Pointer, profile int32) unsafe.Pointer {
	return fAvGetProfileName(codec, profile)
}

func (self *Util) GetRandomSeed() unsafe.Pointer {
	return fAvGetRandomSeed()
}

func (self *Util) GetTimeBaseQ() unsafe.Pointer {
	return fAvGetTimeBaseQ()
}

func (self *Util) GetToken(buf *unsafe.Pointer, term unsafe.Pointer) unsafe.Pointer {
	return fAvGetToken(buf, term)
}

func (self *Crypto) HashAlloc(ctx *unsafe.Pointer, name unsafe.Pointer) error {
	if ret := fAvHashAlloc(ctx, name); ret < 0 {
		return codeErr("av_hash_alloc", ret)
	}
	return nil
}

func (self *Crypto) HashFinal(ctx unsafe.Pointer, dst unsafe.Pointer) {
	fAvHashFinal(ctx, dst)
}

func (self *Crypto) HashFinalB64(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	fAvHashFinalB64(ctx, dst, size)
}

func (self *Crypto) HashFinalBin(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	fAvHashFinalBin(ctx, dst, size)
}

func (self *Crypto) HashFinalHex(ctx unsafe.Pointer, dst unsafe.Pointer, size int32) {
	fAvHashFinalHex(ctx, dst, size)
}

func (self *Crypto) HashFreep(ctx *unsafe.Pointer) {
	fAvHashFreep(ctx)
}

func (self *Crypto) HashGetName(ctx unsafe.Pointer) unsafe.Pointer {
	return fAvHashGetName(ctx)
}

func (self *Crypto) HashGetSize(ctx unsafe.Pointer) unsafe.Pointer {
	return fAvHashGetSize(ctx)
}

func (self *Crypto) HashInit(ctx unsafe.Pointer) {
	fAvHashInit(ctx)
}

func (self *Crypto) HashNames(i int32) unsafe.Pointer {
	return fAvHashNames(i)
}

func (self *Crypto) HashUpdate(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	fAvHashUpdate(ctx, src, len)
}

func (self *Util) HexDump(f unsafe.Pointer, buf unsafe.Pointer, size int32) {
	fAvHexDump(f, buf, size)
}

func (self *Util) HexDumpLog(avcl unsafe.Pointer, level int32, buf unsafe.Pointer, size int32) {
	fAvHexDumpLog(avcl, level, buf, size)
}

func (self *Crypto) HmacAlloc(typ unsafe.Pointer) unsafe.Pointer {
	return fAvHmacAlloc(typ)
}

func (self *Crypto) HmacCalc(ctx unsafe.Pointer, data unsafe.Pointer, len uint32, key unsafe.Pointer, keylen uint32, out unsafe.Pointer, outlen uint32) error {
	if ret := fAvHmacCalc(ctx, data, len, key, keylen, out, outlen); ret < 0 {
		return codeErr("av_hmac_calc", ret)
	}
	return nil
}

func (self *Crypto) HmacFinal(ctx unsafe.Pointer, out unsafe.Pointer, outlen uint32) error {
	if ret := fAvHmacFinal(ctx, out, outlen); ret < 0 {
		return codeErr("av_hmac_final", ret)
	}
	return nil
}

func (self *Crypto) HmacFree(ctx unsafe.Pointer) {
	fAvHmacFree(ctx)
}

func (self *Crypto) HmacInit(ctx unsafe.Pointer, key unsafe.Pointer, keylen uint32) {
	fAvHmacInit(ctx, key, keylen)
}

func (self *Crypto) HmacUpdate(ctx unsafe.Pointer, data unsafe.Pointer, len uint32) {
	fAvHmacUpdate(ctx, data, len)
}

func (self *HWDevice) HwdeviceCtxAlloc(typ unsafe.Pointer) unsafe.Pointer {
	return fAvHwdeviceCtxAlloc(typ)
}

func (self *HWDevice) HwdeviceCtxCreate(device_ctx *unsafe.Pointer, typ int32, device unsafe.Pointer, opts unsafe.Pointer, flags int32) error {
	if ret := fAvHwdeviceCtxCreate(device_ctx, typ, device, opts, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create", ret)
	}
	return nil
}

func (self *HWDevice) HwdeviceCtxCreateDerived(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, flags int32) error {
	if ret := fAvHwdeviceCtxCreateDerived(dst_ctx, typ, src_ctx, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create_derived", ret)
	}
	return nil
}

func (self *HWDevice) HwdeviceCtxCreateDerivedOpts(dst_ctx *unsafe.Pointer, typ int32, src_ctx unsafe.Pointer, options unsafe.Pointer, flags int32) error {
	if ret := fAvHwdeviceCtxCreateDerivedOpts(dst_ctx, typ, src_ctx, options, flags); ret < 0 {
		return codeErr("av_hwdevice_ctx_create_derived_opts", ret)
	}
	return nil
}

func (self *HWDevice) HwdeviceCtxInit(ref unsafe.Pointer) error {
	if ret := fAvHwdeviceCtxInit(ref); ret < 0 {
		return codeErr("av_hwdevice_ctx_init", ret)
	}
	return nil
}

func (self *HWDevice) HwdeviceFindTypeByName(name string) int32 {
	return fAvHwdeviceFindTypeByName(name)
}

func (self *HWDevice) HwdeviceGetHwframeConstraints(ref unsafe.Pointer, hwconfig unsafe.Pointer) unsafe.Pointer {
	return fAvHwdeviceGetHwframeConstraints(ref, hwconfig)
}

func (self *HWDevice) HwdeviceGetTypeName(typ int32) unsafe.Pointer {
	return fAvHwdeviceGetTypeName(typ)
}

func (self *HWDevice) HwdeviceHwconfigAlloc(device_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvHwdeviceHwconfigAlloc(device_ctx)
}

func (self *HWDevice) HwdeviceIterateTypes(prev int32) int32 {
	return fAvHwdeviceIterateTypes(prev)
}

func (self *HWDevice) HwframeConstraintsFree(constraints *unsafe.Pointer) {
	fAvHwframeConstraintsFree(constraints)
}

func (self *HWDevice) HwframeCtxAlloc(device_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvHwframeCtxAlloc(device_ctx)
}

func (self *HWDevice) HwframeCtxCreateDerived(derived_frame_ctx *unsafe.Pointer, format unsafe.Pointer, derived_device_ctx unsafe.Pointer, source_frame_ctx unsafe.Pointer, flags int32) error {
	if ret := fAvHwframeCtxCreateDerived(derived_frame_ctx, format, derived_device_ctx, source_frame_ctx, flags); ret < 0 {
		return codeErr("av_hwframe_ctx_create_derived", ret)
	}
	return nil
}

func (self *HWDevice) HwframeCtxInit(ref unsafe.Pointer) error {
	if ret := fAvHwframeCtxInit(ref); ret < 0 {
		return codeErr("av_hwframe_ctx_init", ret)
	}
	return nil
}

func (self *HWDevice) HwframeGetBuffer(hwframe_ctx unsafe.Pointer, frame unsafe.Pointer, flags int32) int32 {
	return fAvHwframeGetBuffer(hwframe_ctx, frame, flags)
}

func (self *HWDevice) HwframeMap(dst unsafe.Pointer, src unsafe.Pointer, flags int32) error {
	if ret := fAvHwframeMap(dst, src, flags); ret < 0 {
		return codeErr("av_hwframe_map", ret)
	}
	return nil
}

func (self *HWDevice) HwframeTransferData(dst unsafe.Pointer, src unsafe.Pointer, flags int32) error {
	if ret := fAvHwframeTransferData(dst, src, flags); ret < 0 {
		return codeErr("av_hwframe_transfer_data", ret)
	}
	return nil
}

func (self *HWDevice) HwframeTransferGetFormats(hwframe_ctx unsafe.Pointer, dir unsafe.Pointer, formats *unsafe.Pointer, flags int32) int32 {
	return fAvHwframeTransferGetFormats(hwframe_ctx, dir, formats, flags)
}

func (self *Util) I2int(a unsafe.Pointer) int64 {
	return fAvI2int(a)
}

func (self *Util) IamfAudioElementAddLayer(audio_element unsafe.Pointer) unsafe.Pointer {
	return fAvIamfAudioElementAddLayer(audio_element)
}

func (self *Util) IamfAudioElementAlloc() unsafe.Pointer {
	return fAvIamfAudioElementAlloc()
}

func (self *Util) IamfAudioElementFree(audio_element *unsafe.Pointer) {
	fAvIamfAudioElementFree(audio_element)
}

func (self *Util) IamfAudioElementGetClass() unsafe.Pointer {
	return fAvIamfAudioElementGetClass()
}

func (self *Util) IamfMixPresentationAddSubmix(mix_presentation unsafe.Pointer) unsafe.Pointer {
	return fAvIamfMixPresentationAddSubmix(mix_presentation)
}

func (self *Util) IamfMixPresentationAlloc() unsafe.Pointer {
	return fAvIamfMixPresentationAlloc()
}

func (self *Util) IamfMixPresentationFree(mix_presentation *unsafe.Pointer) {
	fAvIamfMixPresentationFree(mix_presentation)
}

func (self *Util) IamfMixPresentationGetClass() unsafe.Pointer {
	return fAvIamfMixPresentationGetClass()
}

func (self *Util) IamfParamDefinitionAlloc(typ unsafe.Pointer, nb_subblocks uint32, size unsafe.Pointer) unsafe.Pointer {
	return fAvIamfParamDefinitionAlloc(typ, nb_subblocks, size)
}

func (self *Util) IamfParamDefinitionGetClass() unsafe.Pointer {
	return fAvIamfParamDefinitionGetClass()
}

func (self *Util) IamfSubmixAddElement(submix unsafe.Pointer) unsafe.Pointer {
	return fAvIamfSubmixAddElement(submix)
}

func (self *Util) IamfSubmixAddLayout(submix unsafe.Pointer) unsafe.Pointer {
	return fAvIamfSubmixAddLayout(submix)
}

func (self *Util) ImdctCalc(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	return fAvImdctCalc(s, output, input)
}

func (self *Util) ImdctHalf(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	return fAvImdctHalf(s, output, input)
}

func (self *Util) InitPacket(pkt unsafe.Pointer) unsafe.Pointer {
	return fAvInitPacket(pkt)
}

func (self *Util) InputAudioDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	return fAvInputAudioDeviceNext(d)
}

func (self *Util) InputVideoDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	return fAvInputVideoDeviceNext(d)
}

func (self *Util) Int2i(a int64) unsafe.Pointer {
	return fAvInt2i(a)
}

func (self *Muxer) InterleavedWriteFrame(s unsafe.Pointer, pkt unsafe.Pointer) error {
	if ret := fAvInterleavedWriteFrame(s, pkt); ret < 0 {
		return codeErr("av_interleaved_write_frame", ret)
	}
	return nil
}

func (self *Muxer) InterleavedWriteUncodedFrame(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) error {
	if ret := fAvInterleavedWriteUncodedFrame(s, stream_index, frame); ret < 0 {
		return codeErr("av_interleaved_write_uncoded_frame", ret)
	}
	return nil
}

func (self *Util) JniGetJavaVm(log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvJniGetJavaVm(log_ctx)
}

func (self *Util) JniSetJavaVm(vm unsafe.Pointer, log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvJniSetJavaVm(vm, log_ctx)
}

func (self *Util) LfgInit(c unsafe.Pointer, seed uint32) {
	fAvLfgInit(c, seed)
}

func (self *Util) LfgInitFromData(c unsafe.Pointer, data unsafe.Pointer, length uint32) error {
	if ret := fAvLfgInitFromData(c, data, length); ret < 0 {
		return codeErr("av_lfg_init_from_data", ret)
	}
	return nil
}

func (self *Util) Log(arg0 unsafe.Pointer, arg1 unsafe.Pointer, cb2 unsafe.Pointer, cb3 unsafe.Pointer) unsafe.Pointer {
	return fAvLog(arg0, arg1, cb2, cb3)
}

func (self *Util) Lzo1xDecode(out unsafe.Pointer, outlen unsafe.Pointer, in unsafe.Pointer, inlen unsafe.Pointer) unsafe.Pointer {
	return fAvLzo1xDecode(out, outlen, in, inlen)
}

func (self *Util) MatchExt(filename unsafe.Pointer, extensions unsafe.Pointer) error {
	if ret := fAvMatchExt(filename, extensions); ret < 0 {
		return codeErr("av_match_ext", ret)
	}
	return nil
}

func (self *Util) MatchList(name unsafe.Pointer, list unsafe.Pointer, separator byte) error {
	if ret := fAvMatchList(name, list, separator); ret < 0 {
		return codeErr("av_match_list", ret)
	}
	return nil
}

func (self *Util) MatchName(name unsafe.Pointer, names unsafe.Pointer) error {
	if ret := fAvMatchName(name, names); ret < 0 {
		return codeErr("av_match_name", ret)
	}
	return nil
}

func (self *Util) MaxAlloc(max uintptr) {
	fAvMaxAlloc(max)
}

func (self *Crypto) Md5Alloc() unsafe.Pointer {
	return fAvMd5Alloc()
}

func (self *Crypto) Md5Final(ctx unsafe.Pointer, dst unsafe.Pointer) {
	fAvMd5Final(ctx, dst)
}

func (self *Crypto) Md5Init(ctx unsafe.Pointer) {
	fAvMd5Init(ctx)
}

func (self *Crypto) Md5Sum(dst unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	fAvMd5Sum(dst, src, len)
}

func (self *Crypto) Md5Update(ctx unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	fAvMd5Update(ctx, src, len)
}

func (self *Util) MdctCalc(s unsafe.Pointer, output unsafe.Pointer, input unsafe.Pointer) unsafe.Pointer {
	return fAvMdctCalc(s, output, input)
}

func (self *Util) MdctEnd(s unsafe.Pointer) unsafe.Pointer {
	return fAvMdctEnd(s)
}

func (self *Util) MdctInit(nbits int32, inverse int32, scale float64) unsafe.Pointer {
	return fAvMdctInit(nbits, inverse, scale)
}

func (self *HWDevice) MediacodecAllocContext() unsafe.Pointer {
	return fAvMediacodecAllocContext()
}

func (self *HWDevice) MediacodecDefaultFree(avctx unsafe.Pointer) {
	fAvMediacodecDefaultFree(avctx)
}

func (self *HWDevice) MediacodecDefaultInit(avctx unsafe.Pointer, ctx unsafe.Pointer, surface unsafe.Pointer) error {
	if ret := fAvMediacodecDefaultInit(avctx, ctx, surface); ret < 0 {
		return codeErr("av_mediacodec_default_init", ret)
	}
	return nil
}

func (self *HWDevice) MediacodecReleaseBuffer(buffer unsafe.Pointer, render int32) error {
	if ret := fAvMediacodecReleaseBuffer(buffer, render); ret < 0 {
		return codeErr("av_mediacodec_release_buffer", ret)
	}
	return nil
}

func (self *HWDevice) MediacodecRenderBufferAtTime(buffer unsafe.Pointer, time int64) error {
	if ret := fAvMediacodecRenderBufferAtTime(buffer, time); ret < 0 {
		return codeErr("av_mediacodec_render_buffer_at_time", ret)
	}
	return nil
}

func (self *Util) ModI(quot unsafe.Pointer, a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer {
	return fAvModI(quot, a, b)
}

func (self *Util) MulI(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer {
	return fAvMulI(a, b)
}

func (self *Util) MulQ(b AVRational, c AVRational) AVRational {
	return fAvMulQ(b, c)
}

func (self *Crypto) Murmur3Alloc() unsafe.Pointer {
	return fAvMurmur3Alloc()
}

func (self *Crypto) Murmur3Final(c unsafe.Pointer, dst unsafe.Pointer) {
	fAvMurmur3Final(c, dst)
}

func (self *Crypto) Murmur3Init(c unsafe.Pointer) {
	fAvMurmur3Init(c)
}

func (self *Crypto) Murmur3InitSeeded(c unsafe.Pointer, seed uint64) {
	fAvMurmur3InitSeeded(c, seed)
}

func (self *Crypto) Murmur3Update(c unsafe.Pointer, src unsafe.Pointer, len uintptr) {
	fAvMurmur3Update(c, src, len)
}

func (self *Util) MuxerIterate(opaque *unsafe.Pointer) unsafe.Pointer {
	return fAvMuxerIterate(opaque)
}

func (self *Util) NearerQ(q AVRational, q1 AVRational, q2 AVRational) error {
	if ret := fAvNearerQ(q, q1, q2); ret < 0 {
		return codeErr("av_nearer_q", ret)
	}
	return nil
}

func (self *Util) NewProgram(s unsafe.Pointer, id int32) unsafe.Pointer {
	return fAvNewProgram(s, id)
}

func (self *Util) OutputAudioDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	return fAvOutputAudioDeviceNext(d)
}

func (self *Util) OutputVideoDeviceNext(d unsafe.Pointer) unsafe.Pointer {
	return fAvOutputVideoDeviceNext(d)
}

func (self *Util) PixelutilsGetSadFn(w_bits int32, h_bits int32, aligned int32, log_ctx unsafe.Pointer) unsafe.Pointer {
	return fAvPixelutilsGetSadFn(w_bits, h_bits, aligned, log_ctx)
}

func (self *Util) PktDump2(f unsafe.Pointer, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer) {
	fAvPktDump2(f, pkt, dump_payload, st)
}

func (self *Util) PktDumpLog2(avcl unsafe.Pointer, level int32, pkt unsafe.Pointer, dump_payload int32, st unsafe.Pointer) {
	fAvPktDumpLog2(avcl, level, pkt, dump_payload, st)
}

func (self *Prober) ProbeInputBuffer(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) error {
	if ret := fAvProbeInputBuffer(pb, fmt, url, logctx, offset, max_probe_size); ret < 0 {
		return codeErr("av_probe_input_buffer", ret)
	}
	return nil
}

func (self *Prober) ProbeInputBuffer2(pb unsafe.Pointer, fmt *unsafe.Pointer, url unsafe.Pointer, logctx unsafe.Pointer, offset uint32, max_probe_size uint32) error {
	if ret := fAvProbeInputBuffer2(pb, fmt, url, logctx, offset, max_probe_size); ret < 0 {
		return codeErr("av_probe_input_buffer2", ret)
	}
	return nil
}

func (self *Prober) ProbeInputFormat(pd unsafe.Pointer, is_opened int32) unsafe.Pointer {
	return fAvProbeInputFormat(pd, is_opened)
}

func (self *Prober) ProbeInputFormat2(pd unsafe.Pointer, is_opened int32, score_max unsafe.Pointer) unsafe.Pointer {
	return fAvProbeInputFormat2(pd, is_opened, score_max)
}

func (self *Prober) ProbeInputFormat3(pd unsafe.Pointer, is_opened int32, score_ret unsafe.Pointer) unsafe.Pointer {
	return fAvProbeInputFormat3(pd, is_opened, score_ret)
}

func (self *Util) ProgramAddStreamIndex(ac unsafe.Pointer, progid int32, idx uint32) {
	fAvProgramAddStreamIndex(ac, progid, idx)
}

func (self *Util) Q2intfloat(q AVRational) uint32 {
	return fAvQ2intfloat(q)
}

func (self *Util) QsvAllocContext() unsafe.Pointer {
	return fAvQsvAllocContext()
}

func (self *Util) RandomBytes(buf unsafe.Pointer, len uintptr) error {
	if ret := fAvRandomBytes(buf, len); ret < 0 {
		return codeErr("av_random_bytes", ret)
	}
	return nil
}

func (self *Crypto) Rc4Alloc() unsafe.Pointer {
	return fAvRc4Alloc()
}

func (self *Crypto) Rc4Crypt(d unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvRc4Crypt(d, dst, src, count, iv, decrypt)
}

func (self *Crypto) Rc4Init(d unsafe.Pointer, key unsafe.Pointer, key_bits int32, decrypt int32) error {
	if ret := fAvRc4Init(d, key, key_bits, decrypt); ret < 0 {
		return codeErr("av_rc4_init", ret)
	}
	return nil
}

func (self *Util) RdftCalc(s unsafe.Pointer, data unsafe.Pointer) unsafe.Pointer {
	return fAvRdftCalc(s, data)
}

func (self *Util) RdftEnd(s unsafe.Pointer) unsafe.Pointer {
	return fAvRdftEnd(s)
}

func (self *Util) RdftInit(nbits int32, trans unsafe.Pointer) unsafe.Pointer {
	return fAvRdftInit(nbits, trans)
}

func (self *Util) ReadImageLine(dst unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32) {
	fAvReadImageLine(dst, data, linesize, desc, x, y, c, w, read_pal_component)
}

func (self *Util) ReadImageLine2(dst unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, read_pal_component int32, dst_element_size int32) {
	fAvReadImageLine2(dst, data, linesize, desc, x, y, c, w, read_pal_component, dst_element_size)
}

func (self *Util) ReadPause(s unsafe.Pointer) error {
	if ret := fAvReadPause(s); ret < 0 {
		return codeErr("av_read_pause", ret)
	}
	return nil
}

func (self *Util) ReadPlay(s unsafe.Pointer) error {
	if ret := fAvReadPlay(s); ret < 0 {
		return codeErr("av_read_play", ret)
	}
	return nil
}

func (self *Util) Reduce(dst_num unsafe.Pointer, dst_den unsafe.Pointer, num int64, den int64, max int64) error {
	if ret := fAvReduce(dst_num, dst_den, num, den, max); ret < 0 {
		return codeErr("av_reduce", ret)
	}
	return nil
}

func (self *Crypto) RipemdAlloc() unsafe.Pointer {
	return fAvRipemdAlloc()
}

func (self *Crypto) RipemdFinal(context unsafe.Pointer, digest unsafe.Pointer) {
	fAvRipemdFinal(context, digest)
}

func (self *Crypto) RipemdInit(context unsafe.Pointer, bits int32) error {
	if ret := fAvRipemdInit(context, bits); ret < 0 {
		return codeErr("av_ripemd_init", ret)
	}
	return nil
}

func (self *Crypto) RipemdUpdate(context unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	fAvRipemdUpdate(context, data, len)
}

func (self *Samples) SamplesAlloc(audio_data *unsafe.Pointer, linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) error {
	if ret := fAvSamplesAlloc(audio_data, linesize, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_alloc", ret)
	}
	return nil
}

func (self *Samples) SamplesAllocArrayAndSamples(audio_data *unsafe.Pointer, linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) error {
	if ret := fAvSamplesAllocArrayAndSamples(audio_data, linesize, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_alloc_array_and_samples", ret)
	}
	return nil
}

func (self *Samples) SamplesCopy(dst unsafe.Pointer, src unsafe.Pointer, dst_offset int32, src_offset int32, nb_samples int32, nb_channels int32, sample_fmt unsafe.Pointer) error {
	if ret := fAvSamplesCopy(dst, src, dst_offset, src_offset, nb_samples, nb_channels, sample_fmt); ret < 0 {
		return codeErr("av_samples_copy", ret)
	}
	return nil
}

func (self *Samples) SamplesFillArrays(audio_data *unsafe.Pointer, linesize unsafe.Pointer, buf unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) error {
	if ret := fAvSamplesFillArrays(audio_data, linesize, buf, nb_channels, nb_samples, sample_fmt, align); ret < 0 {
		return codeErr("av_samples_fill_arrays", ret)
	}
	return nil
}

func (self *Samples) SamplesGetBufferSize(linesize unsafe.Pointer, nb_channels int32, nb_samples int32, sample_fmt unsafe.Pointer, align int32) int32 {
	return fAvSamplesGetBufferSize(linesize, nb_channels, nb_samples, sample_fmt, align)
}

func (self *Samples) SamplesSetSilence(audio_data unsafe.Pointer, offset int32, nb_samples int32, nb_channels int32, sample_fmt unsafe.Pointer) error {
	if ret := fAvSamplesSetSilence(audio_data, offset, nb_samples, nb_channels, sample_fmt); ret < 0 {
		return codeErr("av_samples_set_silence", ret)
	}
	return nil
}

func (self *Util) SdpCreate(ac unsafe.Pointer, n_files int32, buf unsafe.Pointer, size int32) error {
	if ret := fAvSdpCreate(ac, n_files, buf, size); ret < 0 {
		return codeErr("av_sdp_create", ret)
	}
	return nil
}

func (self *Util) SetOptionsString(ctx unsafe.Pointer, opts unsafe.Pointer, key_val_sep unsafe.Pointer, pairs_sep unsafe.Pointer) error {
	if ret := fAvSetOptionsString(ctx, opts, key_val_sep, pairs_sep); ret < 0 {
		return codeErr("av_set_options_string", ret)
	}
	return nil
}

func (self *Crypto) Sha512Alloc() unsafe.Pointer {
	return fAvSha512Alloc()
}

func (self *Crypto) Sha512Final(context unsafe.Pointer, digest unsafe.Pointer) {
	fAvSha512Final(context, digest)
}

func (self *Crypto) Sha512Init(context unsafe.Pointer, bits int32) error {
	if ret := fAvSha512Init(context, bits); ret < 0 {
		return codeErr("av_sha512_init", ret)
	}
	return nil
}

func (self *Crypto) Sha512Update(context unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	fAvSha512Update(context, data, len)
}

func (self *Crypto) ShaAlloc() unsafe.Pointer {
	return fAvShaAlloc()
}

func (self *Crypto) ShaFinal(context unsafe.Pointer, digest unsafe.Pointer) {
	fAvShaFinal(context, digest)
}

func (self *Crypto) ShaInit(context unsafe.Pointer, bits int32) error {
	if ret := fAvShaInit(context, bits); ret < 0 {
		return codeErr("av_sha_init", ret)
	}
	return nil
}

func (self *Crypto) ShaUpdate(ctx unsafe.Pointer, data unsafe.Pointer, len uintptr) {
	fAvShaUpdate(ctx, data, len)
}

func (self *Util) ShrI(a unsafe.Pointer, s int32) unsafe.Pointer {
	return fAvShrI(a, s)
}

func (self *Util) SizeMult(a uintptr, b uintptr, r unsafe.Pointer) int32 {
	return fAvSizeMult(a, b, r)
}

func (self *Util) SmallStrptime(p unsafe.Pointer, fmt unsafe.Pointer, dt unsafe.Pointer) unsafe.Pointer {
	return fAvSmallStrptime(p, fmt, dt)
}

func (self *Util) Sscanf(str unsafe.Pointer, format unsafe.Pointer) error {
	if ret := fAvSscanf(str, format); ret < 0 {
		return codeErr("av_sscanf", ret)
	}
	return nil
}

func (self *Util) Strcasecmp(a unsafe.Pointer, b unsafe.Pointer) error {
	if ret := fAvStrcasecmp(a, b); ret < 0 {
		return codeErr("av_strcasecmp", ret)
	}
	return nil
}

func (self *Util) Strdup(arg0 unsafe.Pointer) unsafe.Pointer {
	return fAvStrdup(arg0)
}

// StrNDup copies at most n bytes of s into fresh malloc'd memory and
// returns it as a Go string (the C copy is freed before returning).
func (self *Util) StrNDup(s string, n int) string {
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
	return fAvIntListLengthForSize(elsize, list, term)
}

func (self *Util) StreamAddSideData(st unsafe.Pointer, typ unsafe.Pointer, data unsafe.Pointer, size uintptr) unsafe.Pointer {
	return fAvStreamAddSideData(st, typ, data, size)
}

func (self *Util) StreamGetClass() unsafe.Pointer {
	return fAvStreamGetClass()
}

func (self *Util) StreamGetCodecTimebase(st unsafe.Pointer) unsafe.Pointer {
	return fAvStreamGetCodecTimebase(st)
}

func (self *Util) StreamGetParser(s unsafe.Pointer) unsafe.Pointer {
	return fAvStreamGetParser(s)
}

func (self *Util) StreamGetSideData(stream unsafe.Pointer, typ unsafe.Pointer, size unsafe.Pointer) unsafe.Pointer {
	return fAvStreamGetSideData(stream, typ, size)
}

func (self *Util) StreamGroupGetClass() unsafe.Pointer {
	return fAvStreamGroupGetClass()
}

func (self *Util) StreamNewSideData(stream unsafe.Pointer, typ unsafe.Pointer, size uintptr) unsafe.Pointer {
	return fAvStreamNewSideData(stream, typ, size)
}

func (self *Util) Strireplace(str unsafe.Pointer, from unsafe.Pointer, to unsafe.Pointer) unsafe.Pointer {
	return fAvStrireplace(str, from, to)
}

func (self *Util) Stristart(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) error {
	if ret := fAvStristart(str, pfx, ptr); ret < 0 {
		return codeErr("av_stristart", ret)
	}
	return nil
}

func (self *Util) Stristr(haystack unsafe.Pointer, needle unsafe.Pointer) unsafe.Pointer {
	return fAvStristr(haystack, needle)
}

func (self *Util) Strlcat(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr {
	return fAvStrlcat(dst, src, size)
}

func (self *Util) Strlcpy(dst unsafe.Pointer, src unsafe.Pointer, size uintptr) uintptr {
	return fAvStrlcpy(dst, src, size)
}

func (self *Util) Strncasecmp(a unsafe.Pointer, b unsafe.Pointer, n uintptr) error {
	if ret := fAvStrncasecmp(a, b, n); ret < 0 {
		return codeErr("av_strncasecmp", ret)
	}
	return nil
}

func (self *Util) Strnstr(haystack unsafe.Pointer, needle unsafe.Pointer, hay_length uintptr) unsafe.Pointer {
	return fAvStrnstr(haystack, needle, hay_length)
}

func (self *Util) Strstart(str unsafe.Pointer, pfx unsafe.Pointer, ptr *unsafe.Pointer) unsafe.Pointer {
	return fAvStrstart(str, pfx, ptr)
}

func (self *Util) Strtod(numstr unsafe.Pointer, tail *unsafe.Pointer) float64 {
	return fAvStrtod(numstr, tail)
}

func (self *Util) Strtok(s unsafe.Pointer, delim unsafe.Pointer, saveptr *unsafe.Pointer) unsafe.Pointer {
	return fAvStrtok(s, delim, saveptr)
}

func (self *Util) SubI(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer {
	return fAvSubI(a, b)
}

func (self *Util) SubQ(b AVRational, c AVRational) AVRational {
	return fAvSubQ(b, c)
}

func (self *Crypto) TeaAlloc() unsafe.Pointer {
	return fAvTeaAlloc()
}

func (self *Crypto) TeaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvTeaCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) TeaInit(ctx unsafe.Pointer, key unsafe.Pointer, rounds int32) {
	fAvTeaInit(ctx, key, rounds)
}

func (self *Util) ThreadMessageFlush(mq unsafe.Pointer) {
	fAvThreadMessageFlush(mq)
}

func (self *Util) ThreadMessageQueueAlloc(mq *unsafe.Pointer, nelem uint32, elsize uint32) error {
	if ret := fAvThreadMessageQueueAlloc(mq, nelem, elsize); ret < 0 {
		return codeErr("av_thread_message_queue_alloc", ret)
	}
	return nil
}

func (self *Util) ThreadMessageQueueFree(mq *unsafe.Pointer) {
	fAvThreadMessageQueueFree(mq)
}

func (self *Util) ThreadMessageQueueNbElems(mq unsafe.Pointer) error {
	if ret := fAvThreadMessageQueueNbElems(mq); ret < 0 {
		return codeErr("av_thread_message_queue_nb_elems", ret)
	}
	return nil
}

func (self *Util) ThreadMessageQueueRecv(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) error {
	if ret := fAvThreadMessageQueueRecv(mq, msg, flags); ret < 0 {
		return codeErr("av_thread_message_queue_recv", ret)
	}
	return nil
}

func (self *Util) ThreadMessageQueueSend(mq unsafe.Pointer, msg unsafe.Pointer, flags uint32) error {
	if ret := fAvThreadMessageQueueSend(mq, msg, flags); ret < 0 {
		return codeErr("av_thread_message_queue_send", ret)
	}
	return nil
}

func (self *Util) ThreadMessageQueueSetErrRecv(mq unsafe.Pointer, err int32) {
	fAvThreadMessageQueueSetErrRecv(mq, err)
}

func (self *Util) ThreadMessageQueueSetErrSend(mq unsafe.Pointer, err int32) {
	fAvThreadMessageQueueSetErrSend(mq, err)
}

func (self *Util) ThreadMessageQueueSetFreeFunc(mq unsafe.Pointer, free_func unsafe.Pointer) {
	fAvThreadMessageQueueSetFreeFunc(mq, free_func)
}

func (self *Util) Timegm(tm unsafe.Pointer) unsafe.Pointer {
	return fAvTimegm(tm)
}

func (self *Util) TreeDestroy(t unsafe.Pointer) {
	fAvTreeDestroy(t)
}

func (self *Util) TreeEnumerate(t unsafe.Pointer, opaque unsafe.Pointer, cmp unsafe.Pointer, enu unsafe.Pointer) {
	fAvTreeEnumerate(t, opaque, cmp, enu)
}

func (self *Util) TreeFind(root unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next unsafe.Pointer) unsafe.Pointer {
	return fAvTreeFind(root, key, cmp, next)
}

func (self *Util) TreeInsert(rootp *unsafe.Pointer, key unsafe.Pointer, cmp unsafe.Pointer, next *unsafe.Pointer) unsafe.Pointer {
	return fAvTreeInsert(rootp, key, cmp, next)
}

func (self *Util) TreeNodeAlloc() unsafe.Pointer {
	return fAvTreeNodeAlloc()
}

func (self *Util) TsMakeTimeString2(buf unsafe.Pointer, ts int64, tb AVRational) unsafe.Pointer {
	return fAvTsMakeTimeString2(buf, ts, tb)
}

func (self *Crypto) TwofishAlloc() unsafe.Pointer {
	return fAvTwofishAlloc()
}

func (self *Crypto) TwofishCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvTwofishCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) TwofishInit(ctx unsafe.Pointer, key unsafe.Pointer, key_bits int32) error {
	if ret := fAvTwofishInit(ctx, key, key_bits); ret < 0 {
		return codeErr("av_twofish_init", ret)
	}
	return nil
}

func (self *Util) TxInit(ctx *unsafe.Pointer, tx unsafe.Pointer, typ unsafe.Pointer, inv int32, len int32, scale unsafe.Pointer, flags uint64) error {
	if ret := fAvTxInit(ctx, tx, typ, inv, len, scale, flags); ret < 0 {
		return codeErr("av_tx_init", ret)
	}
	return nil
}

func (self *Util) TxUninit(ctx *unsafe.Pointer) {
	fAvTxUninit(ctx)
}

func (self *Util) Utf8Decode(codep unsafe.Pointer, bufp *unsafe.Pointer, buf_end unsafe.Pointer, flags uint32) unsafe.Pointer {
	return fAvUtf8Decode(codep, bufp, buf_end, flags)
}

func (self *Util) UuidParse(in unsafe.Pointer, uu unsafe.Pointer) error {
	if ret := fAvUuidParse(in, uu); ret < 0 {
		return codeErr("av_uuid_parse", ret)
	}
	return nil
}

func (self *Util) UuidParseRange(in_start unsafe.Pointer, in_end unsafe.Pointer, uu unsafe.Pointer) error {
	if ret := fAvUuidParseRange(in_start, in_end, uu); ret < 0 {
		return codeErr("av_uuid_parse_range", ret)
	}
	return nil
}

func (self *Util) UuidUnparse(uu unsafe.Pointer, out unsafe.Pointer) {
	fAvUuidUnparse(uu, out)
}

func (self *Util) UuidUrnParse(in unsafe.Pointer, uu unsafe.Pointer) error {
	if ret := fAvUuidUrnParse(in, uu); ret < 0 {
		return codeErr("av_uuid_urn_parse", ret)
	}
	return nil
}

func (self *Util) Vbprintf(buf unsafe.Pointer, fmt unsafe.Pointer, vl_arg unsafe.Pointer) {
	fAvVbprintf(buf, fmt, vl_arg)
}

func (self *HWDevice) VdpauAllocContext() unsafe.Pointer {
	return fAvVdpauAllocContext()
}

func (self *HWDevice) VdpauBindContext(avctx unsafe.Pointer, device unsafe.Pointer, get_proc_address unsafe.Pointer, flags uint32) unsafe.Pointer {
	return fAvVdpauBindContext(avctx, device, get_proc_address, flags)
}

func (self *HWDevice) VdpauGetSurfaceParameters(avctx unsafe.Pointer, typ unsafe.Pointer, width unsafe.Pointer, height unsafe.Pointer) int32 {
	return fAvVdpauGetSurfaceParameters(avctx, typ, width, height)
}

func (self *HWDevice) VdpauHwaccelGetRender2(arg0 unsafe.Pointer) unsafe.Pointer {
	return fAvVdpauHwaccelGetRender2(arg0)
}

func (self *HWDevice) VdpauHwaccelSetRender2(arg0 unsafe.Pointer, arg1 unsafe.Pointer) unsafe.Pointer {
	return fAvVdpauHwaccelSetRender2(arg0, arg1)
}

func (self *Util) VideoEncParamsAlloc(typ unsafe.Pointer, nb_blocks uint32, out_size unsafe.Pointer) unsafe.Pointer {
	return fAvVideoEncParamsAlloc(typ, nb_blocks, out_size)
}

func (self *Util) VideoEncParamsCreateSideData(frame unsafe.Pointer, typ unsafe.Pointer, nb_blocks uint32) unsafe.Pointer {
	return fAvVideoEncParamsCreateSideData(frame, typ, nb_blocks)
}

func (self *Util) VideoHintAlloc(nb_rects uintptr, out_size unsafe.Pointer) unsafe.Pointer {
	return fAvVideoHintAlloc(nb_rects, out_size)
}

func (self *Util) VideoHintCreateSideData(frame unsafe.Pointer, nb_rects uintptr) unsafe.Pointer {
	return fAvVideoHintCreateSideData(frame, nb_rects)
}

func (self *Util) VkfmtFromPixfmt(p int32) unsafe.Pointer {
	return fAvVkfmtFromPixfmt(p)
}

func (self *Util) VkFrameAlloc() unsafe.Pointer {
	return fAvVkFrameAlloc()
}

func (self *Util) Vlog(avcl unsafe.Pointer, level int32, fmt unsafe.Pointer, vl unsafe.Pointer) {
	fAvVlog(avcl, level, fmt, vl)
}

func (self *Util) VorbisParseFrame(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32) error {
	if ret := fAvVorbisParseFrame(s, buf, buf_size); ret < 0 {
		return codeErr("av_vorbis_parse_frame", ret)
	}
	return nil
}

func (self *Util) VorbisParseFrameFlags(s unsafe.Pointer, buf unsafe.Pointer, buf_size int32, flags unsafe.Pointer) unsafe.Pointer {
	return fAvVorbisParseFrameFlags(s, buf, buf_size, flags)
}

func (self *Util) VorbisParseFree(s *unsafe.Pointer) {
	fAvVorbisParseFree(s)
}

func (self *Util) VorbisParseInit(extradata unsafe.Pointer, extradata_size int32) unsafe.Pointer {
	return fAvVorbisParseInit(extradata, extradata_size)
}

func (self *Util) VorbisParseReset(s unsafe.Pointer) {
	fAvVorbisParseReset(s)
}

func (self *Muxer) WriteFrame(s unsafe.Pointer, pkt unsafe.Pointer) error {
	if ret := fAvWriteFrame(s, pkt); ret < 0 {
		return codeErr("av_write_frame", ret)
	}
	return nil
}

func (self *Muxer) WriteImageLine(src unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32) {
	fAvWriteImageLine(src, data, linesize, desc, x, y, c, w)
}

func (self *Muxer) WriteImageLine2(src unsafe.Pointer, data unsafe.Pointer, linesize int32, desc unsafe.Pointer, x int32, y int32, c int32, w int32, src_element_size int32) {
	fAvWriteImageLine2(src, data, linesize, desc, x, y, c, w, src_element_size)
}

func (self *Muxer) WriteTrailer(s unsafe.Pointer) error {
	if ret := fAvWriteTrailer(s); ret < 0 {
		return codeErr("av_write_trailer", ret)
	}
	return nil
}

func (self *Muxer) WriteUncodedFrame(s unsafe.Pointer, stream_index int32, frame unsafe.Pointer) error {
	if ret := fAvWriteUncodedFrame(s, stream_index, frame); ret < 0 {
		return codeErr("av_write_uncoded_frame", ret)
	}
	return nil
}

func (self *Muxer) WriteUncodedFrameQuery(s unsafe.Pointer, stream_index int32) error {
	if ret := fAvWriteUncodedFrameQuery(s, stream_index); ret < 0 {
		return codeErr("av_write_uncoded_frame_query", ret)
	}
	return nil
}

func (self *Util) Xiphlacing(s unsafe.Pointer, v uint32) uint32 {
	return fAvXiphlacing(s, v)
}

func (self *Crypto) XteaAlloc() unsafe.Pointer {
	return fAvXteaAlloc()
}

func (self *Crypto) XteaCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvXteaCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) XteaInit(ctx unsafe.Pointer, key unsafe.Pointer) {
	fAvXteaInit(ctx, key)
}

func (self *Crypto) XteaLeCrypt(ctx unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer, count int32, iv unsafe.Pointer, decrypt int32) {
	fAvXteaLeCrypt(ctx, dst, src, count, iv, decrypt)
}

func (self *Crypto) XteaLeInit(ctx unsafe.Pointer, key unsafe.Pointer) {
	fAvXteaLeInit(ctx, key)
}

func (self *Util) SwresampleConfiguration() unsafe.Pointer {
	return fSwresampleConfiguration()
}

func (self *Util) SwresampleLicense() unsafe.Pointer {
	return fSwresampleLicense()
}

func (self *Util) SwresampleVersion() uint32 {
	return fSwresampleVersion()
}

func (self *Util) SwriAudioConvert(ctx unsafe.Pointer, out unsafe.Pointer, in unsafe.Pointer, len int32) error {
	if ret := fSwriAudioConvert(ctx, out, in, len); ret < 0 {
		return codeErr("swri_audio_convert", ret)
	}
	return nil
}

func (self *Util) SwriAudioConvertAlloc(out_fmt unsafe.Pointer, in_fmt unsafe.Pointer, channels int32, ch_map unsafe.Pointer, flags int32) unsafe.Pointer {
	return fSwriAudioConvertAlloc(out_fmt, in_fmt, channels, ch_map, flags)
}

func (self *Util) SwriAudioConvertFree(ctx *unsafe.Pointer) {
	fSwriAudioConvertFree(ctx)
}

func (self *Util) SwriResampleDspInit(c unsafe.Pointer) {
	fSwriResampleDspInit(c)
}

func (self *Util) SwriResampleDspX86Init(c unsafe.Pointer) {
	fSwriResampleDspX86Init(c)
}

func (self *Util) AddI(a unsafe.Pointer, b unsafe.Pointer) unsafe.Pointer {
	return fAvAddI(a, b)
}

func (self *Util) DynamicHdrVividAlloc(size unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrVividAlloc(size)
}

func (self *Util) DynamicHdrVividCreateSideData(frame unsafe.Pointer) unsafe.Pointer {
	return fAvDynamicHdrVividCreateSideData(frame)
}

func (self *Util) AvformatTransferInternalStreamTimingInfo(ofmt unsafe.Pointer, ost unsafe.Pointer, ist unsafe.Pointer, copy_tb unsafe.Pointer) unsafe.Pointer {
	return fAvformatTransferInternalStreamTimingInfo(ofmt, ost, ist, copy_tb)
}

func (self *Util) ImageCopy(dst_data unsafe.Pointer, dst_linesizes int32, src_data unsafe.Pointer, src_linesizes int32, pix_fmt unsafe.Pointer, width int32, height int32) {
	fAvImageCopy(dst_data, dst_linesizes, src_data, src_linesizes, pix_fmt, width, height)
}

func (self *Util) ImageCopyPlaneUcFrom(dst unsafe.Pointer, dst_linesize unsafe.Pointer, src unsafe.Pointer, src_linesize unsafe.Pointer, bytewidth unsafe.Pointer, height int32) {
	fAvImageCopyPlaneUcFrom(dst, dst_linesize, src, src_linesize, bytewidth, height)
}

func (self *Util) ImageCopyToBuffer(dst unsafe.Pointer, dst_size int32, src_data unsafe.Pointer, src_linesize int32, pix_fmt unsafe.Pointer, width int32, height int32, align int32) error {
	if ret := fAvImageCopyToBuffer(dst, dst_size, src_data, src_linesize, pix_fmt, width, height, align); ret < 0 {
		return codeErr("av_image_copy_to_buffer", ret)
	}
	return nil
}

func (self *Util) ImageCopyUcFrom(dst_data unsafe.Pointer, dst_linesizes unsafe.Pointer, src_data unsafe.Pointer, src_linesizes unsafe.Pointer, pix_fmt unsafe.Pointer, width int32, height int32) {
	fAvImageCopyUcFrom(dst_data, dst_linesizes, src_data, src_linesizes, pix_fmt, width, height)
}

func (self *Util) ImageFillArrays(dst_data unsafe.Pointer, dst_linesize int32, src unsafe.Pointer, pix_fmt unsafe.Pointer, width int32, height int32, align int32) error {
	if ret := fAvImageFillArrays(dst_data, dst_linesize, src, pix_fmt, width, height, align); ret < 0 {
		return codeErr("av_image_fill_arrays", ret)
	}
	return nil
}

func (self *Util) ImageFillBlack(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt unsafe.Pointer, rng unsafe.Pointer, width int32, height int32) error {
	if ret := fAvImageFillBlack(dst_data, dst_linesize, pix_fmt, rng, width, height); ret < 0 {
		return codeErr("av_image_fill_black", ret)
	}
	return nil
}

func (self *Util) ImageFillColor(dst_data unsafe.Pointer, dst_linesize unsafe.Pointer, pix_fmt unsafe.Pointer, color uint32, width int32, height int32, flags int32) error {
	if ret := fAvImageFillColor(dst_data, dst_linesize, pix_fmt, color, width, height, flags); ret < 0 {
		return codeErr("av_image_fill_color", ret)
	}
	return nil
}

func (self *Util) ImageFillLinesizes(linesizes int32, pix_fmt unsafe.Pointer, width int32) int32 {
	return fAvImageFillLinesizes(linesizes, pix_fmt, width)
}

func (self *Util) ImageFillMaxPixsteps(max_pixsteps int32, max_pixstep_comps int32, pixdesc unsafe.Pointer) unsafe.Pointer {
	return fAvImageFillMaxPixsteps(max_pixsteps, max_pixstep_comps, pixdesc)
}

func (self *Util) ImageFillPlaneSizes(size uintptr, pix_fmt unsafe.Pointer, height int32, linesizes unsafe.Pointer) int32 {
	return fAvImageFillPlaneSizes(size, pix_fmt, height, linesizes)
}

func (self *Util) ImageFillPointers(data unsafe.Pointer, pix_fmt unsafe.Pointer, height int32, ptr unsafe.Pointer, linesizes int32) error {
	if ret := fAvImageFillPointers(data, pix_fmt, height, ptr, linesizes); ret < 0 {
		return codeErr("av_image_fill_pointers", ret)
	}
	return nil
}

func (self *Util) ImageGetLinesize(pix_fmt unsafe.Pointer, width int32, plane int32) int32 {
	return fAvImageGetLinesize(pix_fmt, width, plane)
}

func (self *Util) Log2(cb0 unsafe.Pointer) unsafe.Pointer {
	return fAvLog2(cb0)
}

func (self *Util) Log216bit(v uint32) unsafe.Pointer {
	return fAvLog216bit(v)
}

func (self *Util) Log2I(a unsafe.Pointer) error {
	if ret := fAvLog2I(a); ret < 0 {
		return codeErr("av_log2_i", ret)
	}
	return nil
}

func (self *Util) ParseCpuCaps(flags unsafe.Pointer, s unsafe.Pointer) error {
	if ret := fAvParseCpuCaps(flags, s); ret < 0 {
		return codeErr("av_parse_cpu_caps", ret)
	}
	return nil
}

func (self *Util) PixFmtCountPlanes(pix_fmt int32) int32 {
	return fAvPixFmtCountPlanes(pix_fmt)
}

func (self *Util) PixFmtGetChromaSubSample(pix_fmt int32, h_shift *int32, v_shift *int32) int32 {
	return fAvPixFmtGetChromaSubSample(pix_fmt, h_shift, v_shift)
}

func (self *Util) PixFmtSwapEndianness(pix_fmt int32) int32 {
	return fAvPixFmtSwapEndianness(pix_fmt)
}
