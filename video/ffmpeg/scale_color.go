package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// ScaleColor 转色模块: SwsContext + 图像工具 + 像素格式描述,
// 结构体方法直接可用.
//
// Say it plain: 把解出来的 YUV 转成屏幕要的 RGBA (尺寸也能顺手缩放),
// 转之前先问一句支不支持这个格式.

// 常用像素格式 (pixfmt.h 枚举: NONE=-1 起递增):
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

// 缩放算法 (swscale.h: FAST_BILINEAR=1, BILINEAR=2, BICUBIC=4, ...).
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
	fImgCheckSize2                func(uint32, uint32, int64, int32, unsafe.Pointer) int32
	fImgCheckSar                  func(uint32, uint32, AVRational) int32
	fPixDescGet                   func(int32) unsafe.Pointer
	fPixGetName                   func(int32) string
	fPixFmtFromName               func(string) int32
	fSwsAllocVec                  func(length int32) unsafe.Pointer
	fSwsConvertPalette8ToPacked24 func(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer)
	fSwsConvertPalette8ToPacked32 func(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer)
	fSwsFrameEnd                  func(c unsafe.Pointer)
	fSwsFrameStart                func(c unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer) int32
	fSwsFreeFilter                func(filter unsafe.Pointer)
	fSwsFreeVec                   func(a unsafe.Pointer)
	fSwsGetClass                  func() unsafe.Pointer
	fSwsGetCoefficients           func(colorspace int32) unsafe.Pointer
	fSwsGetColorspaceDetails      func(c unsafe.Pointer, inv_table *unsafe.Pointer, srcRange unsafe.Pointer, table *unsafe.Pointer, dstRange unsafe.Pointer, brightness unsafe.Pointer, contrast unsafe.Pointer, saturation unsafe.Pointer) int32
	fSwsGetDefaultFilter          func(lumaGBlur float32, chromaGBlur float32, lumaSharpen float32, chromaSharpen float32, chromaHShift float32, chromaVShift float32, verbose int32) unsafe.Pointer
	fSwsGetGaussianVec            func(variance float64, quality float64) unsafe.Pointer
	fSwsNormalizeVec              func(a unsafe.Pointer, height float64)
	fSwsReceiveSlice              func(c unsafe.Pointer, slice_start uint32, slice_height uint32) int32
	fSwsReceiveSliceAlignment     func(c unsafe.Pointer) uint32
	fSwsScaleVec                  func(a unsafe.Pointer, scalar float64)
	fSwsSendSlice                 func(c unsafe.Pointer, slice_start uint32, slice_height uint32) int32
	fSwsSetColorspaceDetails      func(c unsafe.Pointer, inv_table int32, srcRange int32, table int32, dstRange int32, brightness int32, contrast int32, saturation int32) int32
)

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
	if err := ensureLoaded(); err != nil {
		return nil
	}
	ptr := fSwsGetCtx(srcW, srcH, srcFmt, dstW, dstH, dstFmt, flags, nil, nil, nil)
	if ptr == nil {
		return nil
	}
	return &Scaler{ptr: ptr}
}

// Scale converts srcSliceY..srcSliceY+srcSliceH rows into dst
// (返回输出行数; dstStride 如 RGBA 按 w*4).
func (s *Scaler) Scale(srcPtrs *unsafe.Pointer, srcStrides *int32, srcY, srcH int32, dstPtrs *unsafe.Pointer, dstStrides *int32) int {
	if s == nil || s.ptr == nil {
		return 0
	}
	return int(fSwsScale(s.ptr, srcPtrs, srcStrides, srcY, srcH, dstPtrs, dstStrides))
}

// ScaleFrame converts src frame into dst frame (帧对帧, 记得两边帧都备好缓冲).
func (s *Scaler) ScaleFrame(dst, src *Frame) error {
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
	if s == nil || s.ptr == nil {
		return
	}
	fSwsFreeCtx(s.ptr)
	s.ptr = nil
}

// IsSupportedInput reports the pixel format converts in.
func IsSupportedInput(pixFmt int32) bool {
	if ensureLoaded() != nil {
		return false
	}
	return fSwsIsIn(pixFmt) > 0
}

// IsSupportedOutput reports the pixel format converts out.
func IsSupportedOutput(pixFmt int32) bool {
	if ensureLoaded() != nil {
		return false
	}
	return fSwsIsOut(pixFmt) > 0
}

// BufferSize returns bytes for a w x h image (align 一般 1/32).
func (Image) BufferSize(w, h, pixFmt, align int32) int {
	if ensureLoaded() != nil {
		return 0
	}
	return int(fImgBufSize(w, h, pixFmt, align))
}

// CheckSize validates dimensions (0 通过, 负数说明太大或格式不对).
func (Image) CheckSize(w, h uint32, pixFmt int32) error {
	if ensureLoaded() != nil {
		return errNilScale
	}
	if ret := fImgCheckSize(w, h, pixFmt, nil); ret < 0 {
		return codeErr("av_image_check_size", ret)
	}
	return nil
}

// PixFmtName returns the name, e.g. "yuv420p".
func PixFmtName(pixFmt int32) string {
	if ensureLoaded() != nil {
		return ""
	}
	return fPixGetName(pixFmt)
}

// PixFmtFromName parses a name, e.g. "rgba" (PixFmtNone 对不上).
func PixFmtFromName(name string) int32 {
	if ensureLoaded() != nil {
		return PixFmtNone
	}
	return fPixFmtFromName(name)
}

func (self *Scaler) SwsAllocVec(length int32) unsafe.Pointer {
	return fSwsAllocVec(length)
}

func (self *Scaler) SwsConvertPalette8ToPacked24(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer) {
	fSwsConvertPalette8ToPacked24(src, dst, num_pixels, palette)
}

func (self *Scaler) SwsConvertPalette8ToPacked32(src unsafe.Pointer, dst unsafe.Pointer, num_pixels int32, palette unsafe.Pointer) {
	fSwsConvertPalette8ToPacked32(src, dst, num_pixels, palette)
}

func (self *Scaler) SwsFrameEnd(c unsafe.Pointer) {
	fSwsFrameEnd(c)
}

func (self *Scaler) SwsFrameStart(c unsafe.Pointer, dst unsafe.Pointer, src unsafe.Pointer) error {
	if ret := fSwsFrameStart(c, dst, src); ret < 0 {
		return codeErr("sws_frame_start", ret)
	}
	return nil
}

func (self *Scaler) SwsFreeFilter(filter unsafe.Pointer) {
	fSwsFreeFilter(filter)
}

func (self *Scaler) SwsFreeVec(a unsafe.Pointer) {
	fSwsFreeVec(a)
}

func (self *Scaler) SwsGetClass() unsafe.Pointer {
	return fSwsGetClass()
}

func (self *Scaler) SwsGetCoefficients(colorspace int32) unsafe.Pointer {
	return fSwsGetCoefficients(colorspace)
}

func (self *Scaler) SwsGetColorspaceDetails(c unsafe.Pointer, inv_table *unsafe.Pointer, srcRange unsafe.Pointer, table *unsafe.Pointer, dstRange unsafe.Pointer, brightness unsafe.Pointer, contrast unsafe.Pointer, saturation unsafe.Pointer) error {
	if ret := fSwsGetColorspaceDetails(c, inv_table, srcRange, table, dstRange, brightness, contrast, saturation); ret < 0 {
		return codeErr("sws_getColorspaceDetails", ret)
	}
	return nil
}

func (self *Scaler) SwsGetDefaultFilter(lumaGBlur float32, chromaGBlur float32, lumaSharpen float32, chromaSharpen float32, chromaHShift float32, chromaVShift float32, verbose int32) unsafe.Pointer {
	return fSwsGetDefaultFilter(lumaGBlur, chromaGBlur, lumaSharpen, chromaSharpen, chromaHShift, chromaVShift, verbose)
}

func (self *Scaler) SwsGetGaussianVec(variance float64, quality float64) unsafe.Pointer {
	return fSwsGetGaussianVec(variance, quality)
}

func (self *Scaler) SwsNormalizeVec(a unsafe.Pointer, height float64) {
	fSwsNormalizeVec(a, height)
}

func (self *Scaler) SwsReceiveSlice(c unsafe.Pointer, slice_start uint32, slice_height uint32) error {
	if ret := fSwsReceiveSlice(c, slice_start, slice_height); ret < 0 {
		return codeErr("sws_receive_slice", ret)
	}
	return nil
}

func (self *Scaler) SwsReceiveSliceAlignment(c unsafe.Pointer) uint32 {
	return fSwsReceiveSliceAlignment(c)
}

func (self *Scaler) SwsScaleVec(a unsafe.Pointer, scalar float64) {
	fSwsScaleVec(a, scalar)
}

func (self *Scaler) SwsSendSlice(c unsafe.Pointer, slice_start uint32, slice_height uint32) error {
	if ret := fSwsSendSlice(c, slice_start, slice_height); ret < 0 {
		return codeErr("sws_send_slice", ret)
	}
	return nil
}

func (self *Scaler) SwsSetColorspaceDetails(c unsafe.Pointer, inv_table int32, srcRange int32, table int32, dstRange int32, brightness int32, contrast int32, saturation int32) error {
	if ret := fSwsSetColorspaceDetails(c, inv_table, srcRange, table, dstRange, brightness, contrast, saturation); ret < 0 {
		return codeErr("sws_setColorspaceDetails", ret)
	}
	return nil
}
