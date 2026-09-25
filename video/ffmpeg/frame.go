package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// Frame 帧模块: AVFrame 全量导出, 结构体方法直接可用.
//
// Say it plain: 一帧就是解码器吐出来的一张图 (还没转成屏幕要的
// RGBA), 在这里管借还、拷贝、挂边数据.

// Frame owns one AVFrame* (nil-safe, 记得 Free/Unref).
type Frame struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle for cross-module calls (codec/scale).
func (f *Frame) Ptr() unsafe.Pointer {
	if f == nil {
		return nil
	}
	return f.ptr
}

// FrameSideData owns one AVFrameSideData* (跟着帧走).
type FrameSideData struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (s *FrameSideData) Ptr() unsafe.Pointer {
	if s == nil {
		return nil
	}
	return s.ptr
}

var (
	fFrameAlloc          func() unsafe.Pointer
	fFrameFree           func(*unsafe.Pointer)
	fFrameRef            func(unsafe.Pointer, unsafe.Pointer) int32
	fFrameClone          func(unsafe.Pointer) unsafe.Pointer
	fFrameUnref          func(unsafe.Pointer)
	fFrameMoveRef        func(unsafe.Pointer, unsafe.Pointer)
	fFrameCopy           func(unsafe.Pointer, unsafe.Pointer) int32
	fFrameCopyProps      func(unsafe.Pointer, unsafe.Pointer) int32
	fFrameGetBuffer      func(unsafe.Pointer, int32) int32
	fFrameIsWritable     func(unsafe.Pointer) int32
	fFrameMakeWritable   func(unsafe.Pointer) int32
	fFrameApplyCropping  func(unsafe.Pointer, int32) int32
	fFrameGetPlaneBuffer func(unsafe.Pointer, int32) unsafe.Pointer
	fFrameGetSideData    func(unsafe.Pointer, int32) unsafe.Pointer
	fFrameNewSideData    func(unsafe.Pointer, int32, uintptr) unsafe.Pointer
	fFrameNewSideDataBuf func(unsafe.Pointer, int32, unsafe.Pointer) unsafe.Pointer
	fFrameRemoveSideData func(unsafe.Pointer, int32)
	fFrameSideDataAdd    func(*unsafe.Pointer, *int32, int32, *unsafe.Pointer, uint32) unsafe.Pointer
	fFrameSideDataClone  func(*unsafe.Pointer, *int32, unsafe.Pointer, uint32) int32
	fFrameSideDataDesc   func(int32) unsafe.Pointer
	fFrameSideDataFree   func(*unsafe.Pointer, *int32)
	fFrameSideDataGetC   func(unsafe.Pointer, int32, int32) unsafe.Pointer
	fFrameSideDataName   func(int32) string
	fFrameSideDataNew    func(*unsafe.Pointer, *int32, int32, uintptr, uint32) unsafe.Pointer
	fFrameSideDataRemove func(*unsafe.Pointer, *int32, int32)
	fFrameReplace        func(unsafe.Pointer, unsafe.Pointer) int32
)

func registerFrame(h uintptr) {
	purego.RegisterLibFunc(&fFrameAlloc, h, "av_frame_alloc")
	purego.RegisterLibFunc(&fFrameFree, h, "av_frame_free")
	purego.RegisterLibFunc(&fFrameRef, h, "av_frame_ref")
	purego.RegisterLibFunc(&fFrameClone, h, "av_frame_clone")
	purego.RegisterLibFunc(&fFrameUnref, h, "av_frame_unref")
	purego.RegisterLibFunc(&fFrameMoveRef, h, "av_frame_move_ref")
	purego.RegisterLibFunc(&fFrameCopy, h, "av_frame_copy")
	purego.RegisterLibFunc(&fFrameCopyProps, h, "av_frame_copy_props")
	purego.RegisterLibFunc(&fFrameGetBuffer, h, "av_frame_get_buffer")
	purego.RegisterLibFunc(&fFrameIsWritable, h, "av_frame_is_writable")
	purego.RegisterLibFunc(&fFrameMakeWritable, h, "av_frame_make_writable")
	purego.RegisterLibFunc(&fFrameApplyCropping, h, "av_frame_apply_cropping")
	purego.RegisterLibFunc(&fFrameGetPlaneBuffer, h, "av_frame_get_plane_buffer")
	purego.RegisterLibFunc(&fFrameGetSideData, h, "av_frame_get_side_data")
	purego.RegisterLibFunc(&fFrameNewSideData, h, "av_frame_new_side_data")
	purego.RegisterLibFunc(&fFrameNewSideDataBuf, h, "av_frame_new_side_data_from_buf")
	purego.RegisterLibFunc(&fFrameRemoveSideData, h, "av_frame_remove_side_data")
	purego.RegisterLibFunc(&fFrameSideDataAdd, h, "av_frame_side_data_add")
	purego.RegisterLibFunc(&fFrameSideDataClone, h, "av_frame_side_data_clone")
	purego.RegisterLibFunc(&fFrameSideDataDesc, h, "av_frame_side_data_desc")
	purego.RegisterLibFunc(&fFrameSideDataFree, h, "av_frame_side_data_free")
	purego.RegisterLibFunc(&fFrameSideDataGetC, h, "av_frame_side_data_get_c")
	purego.RegisterLibFunc(&fFrameSideDataName, h, "av_frame_side_data_name")
	purego.RegisterLibFunc(&fFrameSideDataNew, h, "av_frame_side_data_new")
	purego.RegisterLibFunc(&fFrameSideDataRemove, h, "av_frame_side_data_remove")
	purego.RegisterLibFunc(&fFrameReplace, h, "av_frame_replace")
}

// NewFrame allocates an empty frame (记得 Free).
func NewFrame() *Frame {
	if err := ensureLoaded(); err != nil {
		return nil
	}
	ptr := fFrameAlloc()
	if ptr == nil {
		return nil
	}
	return &Frame{ptr: ptr}
}

// Free releases the frame and nils the handle.
func (f *Frame) Free() {
	if f == nil || f.ptr == nil {
		return
	}
	ptr := f.ptr
	f.ptr = nil
	fFrameFree(&ptr)
}

// Clone copies the frame (引用计数, 记得 Free 新帧).
func (f *Frame) Clone() *Frame {
	if f == nil || f.ptr == nil {
		return nil
	}
	ptr := fFrameClone(f.ptr)
	if ptr == nil {
		return nil
	}
	return &Frame{ptr: ptr}
}

// Ref copies src into dst (引用, dst 需已分配).
func (f *Frame) Ref(src *Frame) error {
	if f == nil || src == nil {
		return errNilFrame
	}
	if ret := fFrameRef(f.ptr, src.ptr); ret < 0 {
		return codeErr("av_frame_ref", ret)
	}
	return nil
}

// Replace swaps dst's contents with src's (引用计数, 老数据先丢).
func (f *Frame) Replace(src *Frame) error {
	if f == nil || src == nil {
		return errNilFrame
	}
	if ret := fFrameReplace(f.ptr, src.ptr); ret < 0 {
		return codeErr("av_frame_replace", ret)
	}
	return nil
}

// Copy copies pixels + metadata (深拷贝).
func (f *Frame) Copy(src *Frame) error {
	if f == nil || src == nil {
		return errNilFrame
	}
	if ret := fFrameCopy(f.ptr, src.ptr); ret < 0 {
		return codeErr("av_frame_copy", ret)
	}
	return nil
}

// CopyProps copies only metadata (不碰像素).
func (f *Frame) CopyProps(src *Frame) error {
	if f == nil || src == nil {
		return errNilFrame
	}
	if ret := fFrameCopyProps(f.ptr, src.ptr); ret < 0 {
		return codeErr("av_frame_copy_props", ret)
	}
	return nil
}

// MoveRef steals src's data (src 被清空, dst 接管).
func (f *Frame) MoveRef(src *Frame) {
	if f == nil || src == nil {
		return
	}
	fFrameMoveRef(f.ptr, src.ptr)
}

// Unref drops the data reference (帧壳留着复用).
func (f *Frame) Unref() {
	if f == nil || f.ptr == nil {
		return
	}
	fFrameUnref(f.ptr)
}

// GetBuffer allocates pixel buffers (解码前调, align 一般 0/32).
func (f *Frame) GetBuffer(align int) error {
	if f == nil {
		return errNilFrame
	}
	if ret := fFrameGetBuffer(f.ptr, int32(align)); ret < 0 {
		return codeErr("av_frame_get_buffer", ret)
	}
	return nil
}

// IsWritable reports whether pixels can be written directly.
func (f *Frame) IsWritable() bool {
	if f == nil || f.ptr == nil {
		return false
	}
	return fFrameIsWritable(f.ptr) > 0
}

// MakeWritable makes pixels writable (独占, 写前调).
func (f *Frame) MakeWritable() error {
	if f == nil {
		return errNilFrame
	}
	if ret := fFrameMakeWritable(f.ptr); ret < 0 {
		return codeErr("av_frame_make_writable", ret)
	}
	return nil
}

// ApplyCropping crops pixels per frame crop fields (flags 一般 0).
func (f *Frame) ApplyCropping(flags int) error {
	if f == nil {
		return errNilFrame
	}
	if ret := fFrameApplyCropping(f.ptr, int32(flags)); ret < 0 {
		return codeErr("av_frame_apply_cropping", ret)
	}
	return nil
}

// GetSideData returns side data of type (nil when absent).
func (f *Frame) GetSideData(typ int32) *FrameSideData {
	if f == nil || f.ptr == nil {
		return nil
	}
	ptr := fFrameGetSideData(f.ptr, typ)
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// NewSideData allocates size bytes of side data of type.
func (f *Frame) NewSideData(typ int32, size int) *FrameSideData {
	if f == nil || f.ptr == nil {
		return nil
	}
	ptr := fFrameNewSideData(f.ptr, typ, uintptr(size))
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// RemoveSideData drops side data of type.
func (f *Frame) RemoveSideData(typ int32) {
	if f == nil || f.ptr == nil {
		return
	}
	fFrameRemoveSideData(f.ptr, typ)
}

// GetPlaneBuffer borrows the buffer behind one plane (av_frame_get_plane_buffer;
// 常量持有不释放, 别 Free 它).
func (f *Frame) GetPlaneBuffer(plane int32) unsafe.Pointer {
	if f == nil || f.ptr == nil {
		return nil
	}
	return fFrameGetPlaneBuffer(f.ptr, plane)
}

// NewSideDataFromBuf attaches an existing buffer as typed side data
// (av_frame_new_side_data_from_buf; buf 归帧管, 别再动).
func (f *Frame) NewSideDataFromBuf(typ int32, buf *Buffer) *FrameSideData {
	if f == nil || f.ptr == nil || buf == nil {
		return nil
	}
	ptr := fFrameNewSideDataBuf(f.ptr, typ, buf.ptr)
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// FrameSideDataAdd appends a typed entry wrapping an existing buffer
// (av_frame_side_data_add; sd/nb_sd 照族传).
func FrameSideDataAdd(sd *unsafe.Pointer, nbSd *int32, typ int32, buf *Buffer, flags uint32) *FrameSideData {
	if ensureLoaded() != nil {
		return nil
	}
	var bp unsafe.Pointer
	if buf != nil {
		bp = buf.ptr
	}
	ptr := fFrameSideDataAdd(sd, nbSd, typ, &bp, flags)
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// FrameSideDataClone clones one entry into an array
// (av_frame_side_data_clone).
func FrameSideDataClone(dst *unsafe.Pointer, nbDst *int32, src *FrameSideData, flags uint32) error {
	var sp unsafe.Pointer
	if src != nil {
		sp = src.ptr
	}
	if ret := fFrameSideDataClone(dst, nbDst, sp, flags); ret < 0 {
		return codeErr("av_frame_side_data_clone", ret)
	}
	return nil
}

// FrameSideDataDesc describes a side-data type (av_frame_side_data_desc;
// 常量描述不释放).
func FrameSideDataDesc(typ int32) unsafe.Pointer {
	if ensureLoaded() != nil {
		return nil
	}
	return fFrameSideDataDesc(typ)
}

// FrameSideDataFree frees a side-data array (av_frame_side_data_free).
func FrameSideDataFree(sd *unsafe.Pointer, nbSd *int32) {
	if ensureLoaded() != nil {
		return
	}
	fFrameSideDataFree(sd, nbSd)
}

// FrameSideDataGet finds a typed entry (av_frame_side_data_get_c;
// 常量借用不释放).
func FrameSideDataGet(sd unsafe.Pointer, nbSd, typ int32) *FrameSideData {
	if ensureLoaded() != nil {
		return nil
	}
	ptr := fFrameSideDataGetC(sd, nbSd, typ)
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// FrameSideDataName names a side-data type (av_frame_side_data_name).
func FrameSideDataName(typ int32) string {
	if ensureLoaded() != nil {
		return ""
	}
	return fFrameSideDataName(typ)
}

// FrameSideDataNew allocates a typed entry (av_frame_side_data_new).
func FrameSideDataNew(sd *unsafe.Pointer, nbSd *int32, typ int32, size int, flags uint32) *FrameSideData {
	if ensureLoaded() != nil {
		return nil
	}
	ptr := fFrameSideDataNew(sd, nbSd, typ, uintptr(size), flags)
	if ptr == nil {
		return nil
	}
	return &FrameSideData{ptr: ptr}
}

// FrameSideDataRemove deletes a typed entry (av_frame_side_data_remove).
func FrameSideDataRemove(sd *unsafe.Pointer, nbSd *int32, typ int32) {
	if ensureLoaded() != nil {
		return
	}
	fFrameSideDataRemove(sd, nbSd, typ)
}
