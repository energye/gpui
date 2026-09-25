package ffmpeg

import (
	"unsafe"

	"github.com/ebitengine/purego"
)

// Packet 包模块: AVPacket 全量导出, 结构体方法直接可用.
//
// Say it plain: 一个包就是解复用器吐出来的一段压缩数据, 送进解码器
// 之前先在这里过手 (引用、拷贝、挂边数据、调时间戳).

// Packet owns one AVPacket* (nil-safe, 记得 Free/Unref).
type Packet struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle for cross-module calls (codec/format).
func (p *Packet) Ptr() unsafe.Pointer {
	if p == nil {
		return nil
	}
	return p.ptr
}

// PacketSideData owns one AVPacketSideData* element (不单独释放,
// 跟着包走; 独立分配的用 Free 方法).
type PacketSideData struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (s *PacketSideData) Ptr() unsafe.Pointer {
	if s == nil {
		return nil
	}
	return s.ptr
}

var (
	fPacketAlloc          func() unsafe.Pointer
	fPacketClone          func(unsafe.Pointer) unsafe.Pointer
	fPacketFree           func(*unsafe.Pointer)
	fPacketGetSideData    func(unsafe.Pointer, int32, *uintptr) unsafe.Pointer
	fPacketNewSideData    func(unsafe.Pointer, int32, uintptr) unsafe.Pointer
	fPacketPackDictionary func(unsafe.Pointer, *uintptr) unsafe.Pointer
	fPacketRef            func(unsafe.Pointer, unsafe.Pointer) int32
	fPacketCopyProps      func(unsafe.Pointer, unsafe.Pointer) int32
	fPacketMoveRef        func(unsafe.Pointer, unsafe.Pointer)
	fPacketUnref          func(unsafe.Pointer)
	fPacketFreeSideData   func(unsafe.Pointer)
	fPacketAddSideData    func(unsafe.Pointer, int32, unsafe.Pointer, uintptr) int32
	fPacketShrinkSideData func(unsafe.Pointer, int32, uintptr) int32
	fPacketNewPacket      func(unsafe.Pointer, int32) int32
	fPacketGrowPacket     func(unsafe.Pointer, int32) int32
	fPacketShrinkPacket   func(unsafe.Pointer, int32)
	fPacketFromData       func(unsafe.Pointer, unsafe.Pointer, int32) int32
	fPacketMakeRefcounted func(unsafe.Pointer) int32
	fPacketMakeWritable   func(unsafe.Pointer) int32
	fPacketRescaleTs      func(unsafe.Pointer, AVRational, AVRational)
	fPacketUnpackDict     func(unsafe.Pointer, uintptr, *unsafe.Pointer) int32
	fPacketSideDataAdd    func(*unsafe.Pointer, *int32, int32, unsafe.Pointer, uintptr, int32) unsafe.Pointer
	fPacketSideDataFree   func(*unsafe.Pointer, *int32)
	fPacketSideDataGet    func(unsafe.Pointer, int32, int32) unsafe.Pointer
	fPacketSideDataName   func(int32) string
	fPacketSideDataNew    func(*unsafe.Pointer, *int32, int32, uintptr, int32) unsafe.Pointer
	fPacketSideDataRemove func(unsafe.Pointer, *int32, int32)
)

func registerPacket(h uintptr) {
	purego.RegisterLibFunc(&fPacketAlloc, h, "av_packet_alloc")
	purego.RegisterLibFunc(&fPacketClone, h, "av_packet_clone")
	purego.RegisterLibFunc(&fPacketFree, h, "av_packet_free")
	purego.RegisterLibFunc(&fPacketGetSideData, h, "av_packet_get_side_data")
	purego.RegisterLibFunc(&fPacketNewSideData, h, "av_packet_new_side_data")
	purego.RegisterLibFunc(&fPacketPackDictionary, h, "av_packet_pack_dictionary")
	purego.RegisterLibFunc(&fPacketRef, h, "av_packet_ref")
	purego.RegisterLibFunc(&fPacketCopyProps, h, "av_packet_copy_props")
	purego.RegisterLibFunc(&fPacketMoveRef, h, "av_packet_move_ref")
	purego.RegisterLibFunc(&fPacketUnref, h, "av_packet_unref")
	purego.RegisterLibFunc(&fPacketFreeSideData, h, "av_packet_free_side_data")
	purego.RegisterLibFunc(&fPacketAddSideData, h, "av_packet_add_side_data")
	purego.RegisterLibFunc(&fPacketShrinkSideData, h, "av_packet_shrink_side_data")
	purego.RegisterLibFunc(&fPacketNewPacket, h, "av_new_packet")
	purego.RegisterLibFunc(&fPacketGrowPacket, h, "av_grow_packet")
	purego.RegisterLibFunc(&fPacketShrinkPacket, h, "av_shrink_packet")
	purego.RegisterLibFunc(&fPacketFromData, h, "av_packet_from_data")
	purego.RegisterLibFunc(&fPacketMakeRefcounted, h, "av_packet_make_refcounted")
	purego.RegisterLibFunc(&fPacketMakeWritable, h, "av_packet_make_writable")
	purego.RegisterLibFunc(&fPacketRescaleTs, h, "av_packet_rescale_ts")
	purego.RegisterLibFunc(&fPacketUnpackDict, h, "av_packet_unpack_dictionary")
	purego.RegisterLibFunc(&fPacketSideDataAdd, h, "av_packet_side_data_add")
	purego.RegisterLibFunc(&fPacketSideDataFree, h, "av_packet_side_data_free")
	purego.RegisterLibFunc(&fPacketSideDataGet, h, "av_packet_side_data_get")
	purego.RegisterLibFunc(&fPacketSideDataName, h, "av_packet_side_data_name")
	purego.RegisterLibFunc(&fPacketSideDataNew, h, "av_packet_side_data_new")
	purego.RegisterLibFunc(&fPacketSideDataRemove, h, "av_packet_side_data_remove")
}

// NewPacket allocates an empty packet (记得 Free).
func NewPacket() *Packet {
	if err := ensureLoaded(); err != nil {
		return nil
	}
	ptr := fPacketAlloc()
	if ptr == nil {
		return nil
	}
	return &Packet{ptr: ptr}
}

// Free releases the packet and nils the handle.
func (p *Packet) Free() {
	if p == nil || p.ptr == nil {
		return
	}
	ptr := p.ptr
	p.ptr = nil
	fPacketFree(&ptr)
}

// Clone copies the packet (引用计数, 记得 Free 新包).
func (p *Packet) Clone() *Packet {
	if p == nil || p.ptr == nil {
		return nil
	}
	ptr := fPacketClone(p.ptr)
	if ptr == nil {
		return nil
	}
	return &Packet{ptr: ptr}
}

// Ref copies src into dst (引用, dst 需已分配).
func (p *Packet) Ref(src *Packet) error {
	if p == nil || src == nil {
		return errNilPacket
	}
	if ret := fPacketRef(p.ptr, src.ptr); ret < 0 {
		return codeErr("av_packet_ref", ret)
	}
	return nil
}

// CopyProps copies only metadata (不碰数据).
func (p *Packet) CopyProps(src *Packet) error {
	if p == nil || src == nil {
		return errNilPacket
	}
	if ret := fPacketCopyProps(p.ptr, src.ptr); ret < 0 {
		return codeErr("av_packet_copy_props", ret)
	}
	return nil
}

// MoveRef steals src's data (src 被清空, dst 接管).
func (p *Packet) MoveRef(src *Packet) {
	if p == nil || src == nil {
		return
	}
	fPacketMoveRef(p.ptr, src.ptr)
}

// Unref drops the data reference (包壳留着复用).
func (p *Packet) Unref() {
	if p == nil || p.ptr == nil {
		return
	}
	fPacketUnref(p.ptr)
}

// FreeSideData drops all side data.
func (p *Packet) FreeSideData() {
	if p == nil || p.ptr == nil {
		return
	}
	fPacketFreeSideData(p.ptr)
}

// NewPacketData allocates packet data of size bytes.
func (p *Packet) NewPacketData(size int) error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketNewPacket(p.ptr, int32(size)); ret < 0 {
		return codeErr("av_new_packet", ret)
	}
	return nil
}

// Grow extends packet data by growBy bytes.
func (p *Packet) Grow(growBy int) error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketGrowPacket(p.ptr, int32(growBy)); ret < 0 {
		return codeErr("av_grow_packet", ret)
	}
	return nil
}

// Shrink truncates packet data to size bytes.
func (p *Packet) Shrink(size int) {
	if p == nil {
		return
	}
	fPacketShrinkPacket(p.ptr, int32(size))
}

// MakeRefcounted makes the data refcounted (可共享).
func (p *Packet) MakeRefcounted() error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketMakeRefcounted(p.ptr); ret < 0 {
		return codeErr("av_packet_make_refcounted", ret)
	}
	return nil
}

// MakeWritable makes the data writable (独占, 写前调).
func (p *Packet) MakeWritable() error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketMakeWritable(p.ptr); ret < 0 {
		return codeErr("av_packet_make_writable", ret)
	}
	return nil
}

// RescaleTs converts stamps from src to dst time base.
func (p *Packet) RescaleTs(src, dst AVRational) {
	if p == nil {
		return
	}
	fPacketRescaleTs(p.ptr, src, dst)
}

// GetSideData returns side data of type (nil when absent).
func (p *Packet) GetSideData(typ int32) unsafe.Pointer {
	if p == nil || p.ptr == nil {
		return nil
	}
	var size uintptr
	return fPacketGetSideData(p.ptr, typ, &size)
}

// NewSideData allocates size bytes of side data of type.
func (p *Packet) NewSideData(typ int32, size int) unsafe.Pointer {
	if p == nil || p.ptr == nil {
		return nil
	}
	return fPacketNewSideData(p.ptr, typ, uintptr(size))
}

// AddSideData attaches an existing data buffer as side data.
func (p *Packet) AddSideData(typ int32, data unsafe.Pointer, size int) error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketAddSideData(p.ptr, typ, data, uintptr(size)); ret < 0 {
		return codeErr("av_packet_add_side_data", ret)
	}
	return nil
}

// ShrinkSideData truncates side data of type to size bytes.
func (p *Packet) ShrinkSideData(typ int32, size int) error {
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketShrinkSideData(p.ptr, typ, uintptr(size)); ret < 0 {
		return codeErr("av_packet_shrink_side_data", ret)
	}
	return nil
}
