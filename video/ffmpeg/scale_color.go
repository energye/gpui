//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package ffmpeg

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ScaleColor 转色模块: SwsContext + 图像工具 + 像素格式描述,
// 结构体方法直接可用.
//
// Say it plain: 把解出来的 YUV 转成屏幕要的 RGBA (尺寸也能顺手缩放),
// 转之前先问一句支不支持这个格式.

// 常用像素格式:
// YUV420P=0, RGB24=2, RGBA=26, BGRA=28, NV12=23, GRAY8=8.
const (
	PixFmtNone    int32 = -1
	PixFmtYUV420P int32 = 0
	PixFmtYUYV422 int32 = 1
	PixFmtRGB24   int32 = 2
	PixFmtBGR24   int32 = 3
	PixFmtGRAY8   int32 = 8
	PixFmtNV12    int32 = 23
	PixFmtNV21    int32 = 24
	PixFmtARGB    int32 = 25
	PixFmtRGBA    int32 = 26
	PixFmtABGR    int32 = 27
	PixFmtBGRA    int32 = 28
)

// 缩放算法.
const (
	SwsFastBilinear int32 = 1
	SwsBilinear     int32 = 2
	SwsBicubic      int32 = 4
	SwsX            int32 = 8
	SwsPoint        int32 = 16

	// SWSBilinear 是旧名别名 (decode 直用, 值同 SwsBilinear).
	SWSBilinear int32 = 2
	SwsArea     int32 = 32
)

// Scaler owns one SwsContext* (nil-safe, 记得 Free).
type Scaler struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (s *Scaler) Ptr() unsafe.Pointer {
	mustUse(ensureModScale())
	if s == nil {
		return nil
	}
	return s.ptr
}

// Image holder for image-util calls (无状态).
type Image struct{}

var (
	fSwsGetCtx                    func(int32, int32, int32, int32, int32, int32, int32, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fSwsGetCachedCtx              func(unsafe.Pointer, int32, int32, int32, int32, int32, int32, int32, unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) unsafe.Pointer
	fSwsScale                     func(unsafe.Pointer, *unsafe.Pointer, *int32, int32, int32, *unsafe.Pointer, *int32) int32
	fSwsScaleFrame                func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) int32
	fSwsFreeCtx                   func(unsafe.Pointer)
	fSwsAllocCtx                  func() unsafe.Pointer
	fSwsInitCtx                   func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) int32
	fSwsIsIn                      func(int32) int32
	fSwsIsOut                     func(int32) int32
	fSwsIsEndian                  func(int32) int32
	fImgBufSize                   func(int32, int32, int32, int32) int32
	fImgAlloc                     func(*unsafe.Pointer, *int32, int32, int32, int32, int32) int32
	fImgCopyPlane                 func(unsafe.Pointer, int32, unsafe.Pointer, int32, int32, int32)
	fImgCheckSize                 func(uint32, uint32, int32, unsafe.Pointer) int32
	fImgCheckSize2                func(uint32, uint32, int64, int32, int32, unsafe.Pointer) int32
	fImgCheckSar                  func(uint32, uint32, AVRational) int32
	fPixDescGet                   func(int32) unsafe.Pointer
	fPixGetName                   func(int32) string
	fPixFmtFromName               func(string) int32
	fSwscaleConfiguration         func() string
	fSwscaleLicense               func() string
	fSwscaleVersionNum            func() uint32
	fSwsAllocVec                  func(length int32) unsafe.Pointer
	fSwsConvertPalette8ToPacked24 func(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer)
	fSwsConvertPalette8ToPacked32 func(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer)
	fSwsFrameEnd                  func(c unsafe.Pointer)
	fSwsFrameStart                func(c unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer) int32
	fSwsFreeFilter                func(filter unsafe.Pointer)
	fSwsFreeVec                   func(a unsafe.Pointer)
	fSwsGetClass                  func() unsafe.Pointer
	fSwsGetCoefficients           func(colorspace int32) unsafe.Pointer
	fSwsGetColorspaceDetails      func(c unsafe.Pointer, inv_table *unsafe.Pointer, srcRange *int32, table *unsafe.Pointer, dstRange *int32, brightness *int32, contrast *int32, saturation *int32) int32
	fSwsGetDefaultFilter          func(lumaGBlur float32, chromaGBlur float32, lumaSharpen float32, chromaSharpen float32, chromaHShift float32, chromaVShift float32, verbose int32) unsafe.Pointer
	fSwsGetGaussianVec            func(variance float64, quality float64) unsafe.Pointer
	fSwsNormalizeVec              func(a unsafe.Pointer, height float64)
	fSwsReceiveSlice              func(c unsafe.Pointer, slice_start uint32, slice_height uint32) int32
	fSwsReceiveSliceAlignment     func(c unsafe.Pointer) uint32
	fSwsScaleVec                  func(a unsafe.Pointer, scalar float64)
	fSwsSendSlice                 func(c unsafe.Pointer, slice_start uint32, slice_height uint32) int32
	fSwsSetColorspaceDetails      func(c unsafe.Pointer, inv_table unsafe.Pointer, srcRange int32, table unsafe.Pointer, dstRange int32, brightness int32, contrast int32, saturation int32) int32
)

// ensureModScale 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modScaleOnce sync.Once

func ensureModScale() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modScaleOnce.Do(func() { registerScaleColor(libHandle) })
	return nil
}

func registerScaleColor(h uintptr) {
	purego.RegisterLibFunc(&fSwsGetCtx, h, "sws_getContext")
	purego.RegisterLibFunc(&fSwsGetCachedCtx, h, "sws_getCachedContext")
	purego.RegisterLibFunc(&fSwsScale, h, "sws_scale")
	purego.RegisterLibFunc(&fSwsScaleFrame, h, "sws_scale_frame")
	purego.RegisterLibFunc(&fSwsFreeCtx, h, "sws_freeContext")
	purego.RegisterLibFunc(&fSwsAllocCtx, h, "sws_alloc_context")
	purego.RegisterLibFunc(&fSwsInitCtx, h, "sws_init_context")
	purego.RegisterLibFunc(&fSwsIsIn, h, "sws_isSupportedInput")
	purego.RegisterLibFunc(&fSwsIsOut, h, "sws_isSupportedOutput")
	purego.RegisterLibFunc(&fSwsIsEndian, h, "sws_isSupportedEndiannessConversion")
	purego.RegisterLibFunc(&fImgBufSize, h, "av_image_get_buffer_size")
	purego.RegisterLibFunc(&fImgAlloc, h, "av_image_alloc")
	purego.RegisterLibFunc(&fImgCopyPlane, h, "av_image_copy_plane")
	purego.RegisterLibFunc(&fImgCheckSize, h, "av_image_check_size")
	purego.RegisterLibFunc(&fImgCheckSize2, h, "av_image_check_size2")
	purego.RegisterLibFunc(&fImgCheckSar, h, "av_image_check_sar")
	purego.RegisterLibFunc(&fPixDescGet, h, "av_pix_fmt_desc_get")
	purego.RegisterLibFunc(&fPixGetName, h, "av_get_pix_fmt_name")
	purego.RegisterLibFunc(&fPixFmtFromName, h, "av_get_pix_fmt")
	purego.RegisterLibFunc(&fSwscaleConfiguration, h, "swscale_configuration")
	purego.RegisterLibFunc(&fSwscaleLicense, h, "swscale_license")
	purego.RegisterLibFunc(&fSwscaleVersionNum, h, "swscale_version")
	purego.RegisterLibFunc(&fSwsAllocVec, h, "sws_allocVec")
	purego.RegisterLibFunc(&fSwsConvertPalette8ToPacked24, h, "sws_convertPalette8ToPacked24")
	purego.RegisterLibFunc(&fSwsConvertPalette8ToPacked32, h, "sws_convertPalette8ToPacked32")
	purego.RegisterLibFunc(&fSwsFrameEnd, h, "sws_frame_end")
	purego.RegisterLibFunc(&fSwsFrameStart, h, "sws_frame_start")
	purego.RegisterLibFunc(&fSwsFreeFilter, h, "sws_freeFilter")
	purego.RegisterLibFunc(&fSwsFreeVec, h, "sws_freeVec")
	purego.RegisterLibFunc(&fSwsGetClass, h, "sws_get_class")
	purego.RegisterLibFunc(&fSwsGetCoefficients, h, "sws_getCoefficients")
	purego.RegisterLibFunc(&fSwsGetColorspaceDetails, h, "sws_getColorspaceDetails")
	purego.RegisterLibFunc(&fSwsGetDefaultFilter, h, "sws_getDefaultFilter")
	purego.RegisterLibFunc(&fSwsGetGaussianVec, h, "sws_getGaussianVec")
	purego.RegisterLibFunc(&fSwsNormalizeVec, h, "sws_normalizeVec")
	purego.RegisterLibFunc(&fSwsReceiveSlice, h, "sws_receive_slice")
	purego.RegisterLibFunc(&fSwsReceiveSliceAlignment, h, "sws_receive_slice_alignment")
	purego.RegisterLibFunc(&fSwsScaleVec, h, "sws_scaleVec")
	purego.RegisterLibFunc(&fSwsSendSlice, h, "sws_send_slice")
	purego.RegisterLibFunc(&fSwsSetColorspaceDetails, h, "sws_setColorspaceDetails")
}

// NewScaler builds a converter srcW x srcH/srcFmt -> dstW x dstH/dstFmt
// (记得 Free; flags 用 SwsBilinear 等).
func NewScaler(srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags int32) *Scaler {
	mustUse(ensureModScale())
	ptr := fSwsGetCtx(srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags, nil, nil, nil)
	if ptr == nil {
		return nil
	}
	return &Scaler{ptr: ptr}
}

// Scale converts srcSliceY..srcSliceY+srcSliceH rows into dst
// (返回输出行数; dstStride 如 RGBA 按 w*4).
func (s *Scaler) Scale(srcPtrs *unsafe.Pointer, srcStrides *int32, srcY, srcH int32, dstPtrs *unsafe.Pointer, dstStrides *int32) int {
	mustUse(ensureModScale())
	if s == nil || s.ptr == nil {
		return 0
	}
	return int(fSwsScale(s.ptr, srcPtrs, srcStrides, srcY, srcH, dstPtrs, dstStrides))
}

// ScaleFrame converts src frame into dst frame (帧对帧, 记得两边帧都备好缓冲).
func (s *Scaler) ScaleFrame(dst, src *Frame) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if s == nil || dst == nil || src == nil {
		return errNilScale
	}
	if ret := fSwsScaleFrame(s.ptr, dst.ptr, src.ptr); ret < 0 {
		return codeErr("sws_scale_frame", ret)
	}
	return nil
}

// Free releases the converter and nils the holder.
func (s *Scaler) Free() {
	mustUse(ensureModScale())
	if s == nil || s.ptr == nil {
		return
	}
	fSwsFreeCtx(s.ptr)
	s.ptr = nil
}

// CachedScaler reuses one converter across size/format changes
// (sws_getCachedContext; 传旧 Scaler 复用, nil 建新; 返回的和入参是
// 同一个 holder, 别 double-Free).
func CachedScaler(old *Scaler, srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags int32) *Scaler {
	if ensureModScale() != nil {
		return nil
	}
	var ctx unsafe.Pointer
	if old != nil {
		ctx = old.ptr
	}
	ptr := fSwsGetCachedCtx(ctx, srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags, nil, nil, nil)
	if ptr == nil {
		return nil
	}
	if old != nil {
		old.ptr = ptr
		return old
	}
	return &Scaler{ptr: ptr}
}

// AllocContext allocates an empty converter for manual option setup
// (sws_alloc_context + sws_init_context; 配好选项后 Init, 记得 Free).
func AllocScalerContext() *Scaler {
	if ensureModScale() != nil {
		return nil
	}
	ptr := fSwsAllocCtx()
	if ptr == nil {
		return nil
	}
	return &Scaler{ptr: ptr}
}

// InitContext initializes a manually-configured converter
// (sws_init_context; srcFilter/dstFilter 传 nil 用默认).
func (s *Scaler) InitContext(srcFilter, dstFilter unsafe.Pointer) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if s == nil || s.ptr == nil {
		return errNilScale
	}
	if ret := fSwsInitCtx(s.ptr, srcFilter, dstFilter); ret < 0 {
		return codeErr("sws_init_context", ret)
	}
	return nil
}

// IsEndianSupported reports whether pixFmt converts with correct byte
// order on this host (sws_isSupportedEndiannessConversion;
// 正数表支持, 0 表不支持, 与 IsSupportedInput 同理).
func IsEndianSupported(pixFmt int32) bool {
	if ensureModScale() != nil {
		return false
	}
	return fSwsIsEndian(pixFmt) != 0
}

// ImageBufferSize returns the byte size of a packed image
// (av_image_get_buffer_size; C 形参顺序是 (pixFmt, w, h, align);
// align 传 1 最紧, 32 最快).
func ImageBufferSize(pixFmt, w, h, align int32) int32 {
	if ensureModScale() != nil {
		return 0
	}
	return fImgBufSize(pixFmt, w, h, align)
}

// ImageAlloc allocates a packed image buffer and fills pointers/lines
// (av_image_alloc; 返回总字节数, pointers/lines 由包内写, 用 Mem.Free 放).
func ImageAlloc(pointers *unsafe.Pointer, lines *int32, w, h, pixFmt, align int32) int32 {
	if ensureModScale() != nil {
		return AvErrorEAGAIN
	}
	return fImgAlloc(pointers, lines, w, h, pixFmt, align)
}

// ImageCopyPlane copies one plane with stride (av_image_copy_plane).
func ImageCopyPlane(dst unsafe.Pointer, dstStride int32, src unsafe.Pointer, srcStride, byteWidth, height int32) {
	if ensureModScale() != nil {
		return
	}
	fImgCopyPlane(dst, dstStride, src, srcStride, byteWidth, height)
}

// ImageCheckSize validates w/h for allocation (av_image_check_size;
// 0 为合法, 负数为错码).
func ImageCheckSize(w, h uint32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	var md MediaDesc
	_ = md
	if ensureModScale() != nil {
		return errNilScale
	}
	if ret := fImgCheckSize(w, h, 0, nil); ret < 0 {
		return codeErr("av_image_check_size", ret)
	}
	return nil
}

// ImageCheckSize2 validates w/h against a pixel budget
// (av_image_check_size2; maxPixels 传实际像素数, 如 w*h;
// log_offset 传 0, 日志上下文传 nil).
func ImageCheckSize2(w, h uint32, maxPixels int64, pixFmt int32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ensureModScale() != nil {
		return errNilScale
	}
	if ret := fImgCheckSize2(w, h, maxPixels, pixFmt, 0, nil); ret < 0 {
		return codeErr("av_image_check_size2", ret)
	}
	return nil
}

// ImageCheckSar validates a sample aspect ratio (av_image_check_sar;
// 0 为合法).
func ImageCheckSar(w, h uint32, sar AVRational) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ensureModScale() != nil {
		return errNilScale
	}
	if ret := fImgCheckSar(w, h, sar); ret < 0 {
		return codeErr("av_image_check_sar", ret)
	}
	return nil
}

// PixFmtDesc gets the pixel format descriptor (av_pix_fmt_desc_get;
// 常量借用不释放).
func PixFmtDesc(pixFmt int32) unsafe.Pointer {
	if ensureModScale() != nil {
		return nil
	}
	return fPixDescGet(pixFmt)
}

// IsSupportedInput reports the pixel format converts in
// (sws_isSupportedInput; 正数表支持, 0 表不支持).
func IsSupportedInput(pixFmt int32) bool {
	if ensureModScale() != nil {
		return false
	}
	return fSwsIsIn(pixFmt) > 0
}

// IsSupportedOutput reports the pixel format converts out
// (sws_isSupportedOutput; 正数表支持, 0 表不支持).
func IsSupportedOutput(pixFmt int32) bool {
	if ensureModScale() != nil {
		return false
	}
	return fSwsIsOut(pixFmt) > 0
}

// BufferSize returns bytes for a w x h image (align 一般 1/32;
// C 形参顺序是 (pixFmt, w, h, align), 别传反).
func (Image) BufferSize(w, h, pixFmt, align int32) int {
	if ensureModScale() != nil {
		return 0
	}
	return int(fImgBufSize(pixFmt, w, h, align))
}

// CheckSize validates dimensions (0 通过, 负数说明太大或格式不对).
func (Image) CheckSize(w, h uint32, pixFmt int32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ensureModScale() != nil {
		return errNilScale
	}
	if ret := fImgCheckSize(w, h, pixFmt, nil); ret < 0 {
		return codeErr("av_image_check_size", ret)
	}
	return nil
}

// PixFmtName returns the name, e.g. "yuv420p".
func PixFmtName(pixFmt int32) string {
	if ensureModScale() != nil {
		return ""
	}
	return fPixGetName(pixFmt)
}

// PixFmtFromName parses a name, e.g. "rgba" (PixFmtNone 对不上).
func PixFmtFromName(name string) int32 {
	if ensureModScale() != nil {
		return PixFmtNone
	}
	return fPixFmtFromName(name)
}

// SwscaleConfiguration returns the libswscale build configuration string.
func (self *Scaler) SwscaleConfiguration() string {
	mustUse(ensureModScale())
	if self == nil || ensureModScale() != nil {
		return ""
	}
	return fSwscaleConfiguration()
}

// SwscaleLicense returns the libswscale license string.
func (self *Scaler) SwscaleLicense() string {
	mustUse(ensureModScale())
	if self == nil || ensureModScale() != nil {
		return ""
	}
	return fSwscaleLicense()
}

// SwscaleVersion returns the libswscale version number.
func (self *Scaler) SwscaleVersion() uint32 {
	mustUse(ensureModScale())
	if self == nil || ensureModScale() != nil {
		return 0
	}
	return fSwscaleVersionNum()
}

// SwsAllocVec 新建向量（对 sws_allocVec；参数 length；成功回 C 指针（调用方拥有，记得 SwsFreeVec），失败回 nil；无状态调用）。
func (self *Scaler) SwsAllocVec(length int32) unsafe.Pointer {
	mustUse(ensureModScale())
	return fSwsAllocVec(length)
}

// SwsConvertPalette8ToPacked24 调色板转 packed 像素（对 sws_convertPalette8ToPacked24；参数 src、dst、num_pixels、palette；按签名取回值；无状态调用）。
func (self *Scaler) SwsConvertPalette8ToPacked24(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer) {
	mustUse(ensureModScale())
	fSwsConvertPalette8ToPacked24(src, dst, num_pixels, palette)
}

// SwsConvertPalette8ToPacked32 调色板转 packed 像素（对 sws_convertPalette8ToPacked32；参数 src、dst、num_pixels、palette；按签名取回值；无状态调用）。
func (self *Scaler) SwsConvertPalette8ToPacked32(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer) {
	mustUse(ensureModScale())
	fSwsConvertPalette8ToPacked32(src, dst, num_pixels, palette)
}

// SwsFrameEnd 切片帧结束（对 sws_frame_end；参数 c；按签名取回值；无状态调用）。
func (self *Scaler) SwsFrameEnd(c unsafe.Pointer) {
	mustUse(ensureModScale())
	fSwsFrameEnd(c)
}

// SwsFrameStart 切片帧开始（对 sws_frame_start；参数 c、dst、src（都是 AVFrame*）；先调它，再 Send/Receive 切片，最后 FrameEnd；成功回 nil，失败回 error；无状态调用）。
func (self *Scaler) SwsFrameStart(c unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ret := fSwsFrameStart(c, dst, src); ret < 0 {
		return codeErr("sws_frame_start", ret)
	}
	return nil
}

// SwsFreeFilter 释放滤波器（对 sws_freeFilter；参数 filter；按签名取回值；无状态调用）。
func (self *Scaler) SwsFreeFilter(filter unsafe.Pointer) {
	mustUse(ensureModScale())
	fSwsFreeFilter(filter)
}

// SwsFreeVec 释放向量（对 sws_freeVec；参数 a；按签名取回值；无状态调用）。
func (self *Scaler) SwsFreeVec(a unsafe.Pointer) {
	mustUse(ensureModScale())
	fSwsFreeVec(a)
}

// SwsGetClass 取转色器选项类（对 sws_get_class；无参数；回静态 AVClass 借用不释放；无状态调用）。
func (self *Scaler) SwsGetClass() unsafe.Pointer {
	mustUse(ensureModScale())
	return fSwsGetClass()
}

// SwsGetCoefficients 取色空间系数表（对 sws_getCoefficients；参数 colorspace（SWS_CS_*）；回静态表借用不释放，不认识的颜色空间回 nil；无状态调用）。
func (self *Scaler) SwsGetCoefficients(colorspace int32) unsafe.Pointer {
	mustUse(ensureModScale())
	return fSwsGetCoefficients(colorspace)
}

// SwsGetColorspaceDetails 取出色空间转换细节（对 sws_getColorspaceDetails；参数 c、inv_table、srcRange、table、dstRange、brightness、contrast、saturation；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Scaler) SwsGetColorspaceDetails(c unsafe.Pointer, inv_table *unsafe.Pointer, srcRange *int32, table *unsafe.Pointer, dstRange *int32, brightness *int32, contrast *int32, saturation *int32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ret := fSwsGetColorspaceDetails(c, inv_table, srcRange, table, dstRange, brightness, contrast, saturation); ret < 0 {
		return codeErr("sws_getColorspaceDetails", ret)
	}
	return nil
}

// SwsGetDefaultFilter 取默认缩放滤波器（对 sws_getDefaultFilter；参数 lumaGBlur、chromaGBlur、lumaSharpen、chromaSharpen、chromaHShift、chromaVShift、verbose；回 C 指针（调用方拥有，记得 SwsFreeFilter），失败回 nil；无状态调用）。
func (self *Scaler) SwsGetDefaultFilter(lumaGBlur float32, chromaGBlur float32, lumaSharpen float32, chromaSharpen float32, chromaHShift float32, chromaVShift float32, verbose int32) unsafe.Pointer {
	mustUse(ensureModScale())
	return fSwsGetDefaultFilter(lumaGBlur, chromaGBlur, lumaSharpen, chromaSharpen, chromaHShift, chromaVShift, verbose)
}

// SwsGetGaussianVec 生成高斯向量（对 sws_getGaussianVec；参数 variance、quality；成功回 C 指针（调用方拥有，记得 SwsFreeVec），失败回 nil；无状态调用）。
func (self *Scaler) SwsGetGaussianVec(variance float64, quality float64) unsafe.Pointer {
	mustUse(ensureModScale())
	return fSwsGetGaussianVec(variance, quality)
}

// SwsNormalizeVec 向量归一化（对 sws_normalizeVec；参数 a、height；按签名取回值；无状态调用）。
func (self *Scaler) SwsNormalizeVec(a unsafe.Pointer, height float64) {
	mustUse(ensureModScale())
	fSwsNormalizeVec(a, height)
}

// SwsReceiveSlice 切片模式取一行（对 sws_receive_slice；参数 c、slice_start、slice_height（都须按 SwsReceiveSliceAlignment 对齐，尾片除外）；成功回 nil，EAGAIN 表要先 Send 喂数据，失败回 error；无状态调用）。
func (self *Scaler) SwsReceiveSlice(c unsafe.Pointer, slice_start uint32, slice_height uint32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ret := fSwsReceiveSlice(c, slice_start, slice_height); ret < 0 {
		return codeErr("sws_receive_slice", ret)
	}
	return nil
}

// SwsReceiveSliceAlignment 切片模式取一行（对 sws_receive_slice_alignment；参数 c；回数值；无状态调用）。
func (self *Scaler) SwsReceiveSliceAlignment(c unsafe.Pointer) uint32 {
	mustUse(ensureModScale())
	return fSwsReceiveSliceAlignment(c)
}

// SwsScaleVec 转一批行，解码主路用它（对 sws_scaleVec；参数 a、scalar；按签名取回值；无状态调用）。
func (self *Scaler) SwsScaleVec(a unsafe.Pointer, scalar float64) {
	mustUse(ensureModScale())
	fSwsScaleVec(a, scalar)
}

// SwsSendSlice 切片模式喂一行（对 sws_send_slice；参数 c、slice_start、slice_height；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Scaler) SwsSendSlice(c unsafe.Pointer, slice_start uint32, slice_height uint32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ret := fSwsSendSlice(c, slice_start, slice_height); ret < 0 {
		return codeErr("sws_send_slice", ret)
	}
	return nil
}

// SwsSetColorspaceDetails 设置色空间转换细节（对 sws_setColorspaceDetails；参数 c、inv_table、srcRange、table、dstRange、brightness、contrast、saturation；成功回 nil，失败回 error（字串已是人话）；无状态调用）。
func (self *Scaler) SwsSetColorspaceDetails(c unsafe.Pointer, inv_table unsafe.Pointer, srcRange int32, table unsafe.Pointer, dstRange int32, brightness int32, contrast int32, saturation int32) error {
	if err := ensureModScale(); err != nil {
		return err
	}
	if ret := fSwsSetColorspaceDetails(c, inv_table, srcRange, table, dstRange, brightness, contrast, saturation); ret < 0 {
		return codeErr("sws_setColorspaceDetails", ret)
	}
	return nil
}
