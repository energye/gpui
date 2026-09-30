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
	mustUse(ensureModPacket())
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

// ensureModPacket 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modPacketOnce sync.Once

func ensureModPacket() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modPacketOnce.Do(func() { registerPacket(libHandle) })
	return nil
}

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
	mustUse(ensureModPacket())
	ptr := fPacketAlloc()
	if ptr == nil {
		return nil
	}
	return &Packet{ptr: ptr}
}

// Free releases the packet and nils the handle.
func (p *Packet) Free() {
	mustUse(ensureModPacket())
	if p == nil || p.ptr == nil {
		return
	}
	ptr := p.ptr
	p.ptr = nil
	fPacketFree(&ptr)
}

// Clone copies the packet (引用计数, 记得 Free 新包).
func (p *Packet) Clone() *Packet {
	mustUse(ensureModPacket())
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
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	mustUse(ensureModPacket())
	if p == nil || src == nil {
		return
	}
	fPacketMoveRef(p.ptr, src.ptr)
}

// Unref drops the data reference (包壳留着复用).
func (p *Packet) Unref() {
	mustUse(ensureModPacket())
	if p == nil || p.ptr == nil {
		return
	}
	fPacketUnref(p.ptr)
}

// FreeSideData drops all side data.
func (p *Packet) FreeSideData() {
	mustUse(ensureModPacket())
	if p == nil || p.ptr == nil {
		return
	}
	fPacketFreeSideData(p.ptr)
}

// NewPacketData allocates packet data of size bytes.
func (p *Packet) NewPacketData(size int) error {
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	mustUse(ensureModPacket())
	if p == nil {
		return
	}
	fPacketShrinkPacket(p.ptr, int32(size))
}

// MakeRefcounted makes the data refcounted (可共享).
func (p *Packet) MakeRefcounted() error {
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	mustUse(ensureModPacket())
	if p == nil {
		return
	}
	fPacketRescaleTs(p.ptr, src, dst)
}

// StreamIndex 读包属于哪条流（读 AVPacket.stream_index）。
func (p *Packet) StreamIndex() int {
	if p == nil || p.ptr == nil {
		return -1
	}
	return int(loadInt32(p.ptr, pktStreamIndex))
}

// SetStreamIndex 写包属于哪条流（写 AVPacket.stream_index；转封装换流时调）。
func (p *Packet) SetStreamIndex(i int) {
	if p == nil || p.ptr == nil {
		return
	}
	*(*int32)(unsafe.Add(p.ptr, pktStreamIndex)) = int32(i)
}

// IsKey 读包是不是关键帧（读 AVPacket.flags 的 KEY 位）。
func (p *Packet) IsKey() bool {
	if p == nil || p.ptr == nil {
		return false
	}
	return loadInt32(p.ptr, pktFlags)&1 != 0
}

// DurationMs 读包时长换算成毫秒（按流时基算；未知回 0）。
func (p *Packet) DurationMs(tb AVRational) int64 {
	if p == nil || p.ptr == nil || tb.Num <= 0 || tb.Den <= 0 {
		return 0
	}
	return loadInt64(p.ptr, pktDuration) * int64(tb.Num) * 1000 / int64(tb.Den)
}

// GetSideData returns side data of type (nil when absent).
func (p *Packet) GetSideData(typ int32) unsafe.Pointer {
	mustUse(ensureModPacket())
	if p == nil || p.ptr == nil {
		return nil
	}
	var size uintptr
	return fPacketGetSideData(p.ptr, typ, &size)
}

// NewSideData allocates size bytes of side data of type.
func (p *Packet) NewSideData(typ int32, size int) unsafe.Pointer {
	mustUse(ensureModPacket())
	if p == nil || p.ptr == nil {
		return nil
	}
	return fPacketNewSideData(p.ptr, typ, uintptr(size))
}

// AddSideData attaches an existing data buffer as side data.
func (p *Packet) AddSideData(typ int32, data unsafe.Pointer, size int) error {
	if err := ensureModPacket(); err != nil {
		return err
	}
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
	if err := ensureModPacket(); err != nil {
		return err
	}
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketShrinkSideData(p.ptr, typ, uintptr(size)); ret < 0 {
		return codeErr("av_packet_shrink_side_data", ret)
	}
	return nil
}

// PackDictionary serializes a dictionary into bytes for attaching to a
// packet (av_packet_pack_dictionary; size 由包内返回, 用完 av_free 风格
// 的 Mem.Free 释放 — 见 buffer_mem.go).
func (d *Dictionary) PackDictionary() (unsafe.Pointer, uintptr) {
	mustUse(ensureModPacket())
	if d == nil {
		return nil, 0
	}
	var size uintptr
	ptr := fPacketPackDictionary(d.ptr, &size)
	return ptr, size
}

// FromData wraps an external data buffer as packet payload without
// copying (av_packet_from_data; 调用后别再碰 data, 归包管).
func (p *Packet) FromData(data unsafe.Pointer, size int) error {
	if err := ensureModPacket(); err != nil {
		return err
	}
	if p == nil {
		return errNilPacket
	}
	if ret := fPacketFromData(p.ptr, data, int32(size)); ret < 0 {
		return codeErr("av_packet_from_data", ret)
	}
	return nil
}

// UnpackDictionary deserializes packet-attached bytes back into a fresh
// dictionary (av_packet_unpack_dictionary).
func UnpackDictionary(data unsafe.Pointer, size int) (*Dictionary, error) {
	if err := ensureModPacket(); err != nil {
		return nil, err
	}
	if ensureModPacket() != nil {
		return nil, errNilDict
	}
	var dict unsafe.Pointer
	if ret := fPacketUnpackDict(data, uintptr(size), &dict); ret < 0 {
		return nil, codeErr("av_packet_unpack_dictionary", ret)
	}
	if dict == nil {
		return nil, errNilDict
	}
	return &Dictionary{ptr: dict}, nil
}

// SideDataName names a side-data type id (av_packet_side_data_name).
func SideDataName(typ int32) string {
	if ensureModPacket() != nil {
		return ""
	}
	return fPacketSideDataName(typ)
}

// SideDataAdd appends a typed entry to a side-data array
// (av_packet_side_data_add; sd/nb_sd 照 av_packet_side_data_* 族传).
func SideDataAdd(sd *unsafe.Pointer, nbSd *int32, typ int32, data unsafe.Pointer, size int, flags int32) unsafe.Pointer {
	if ensureModPacket() != nil {
		return nil
	}
	return fPacketSideDataAdd(sd, nbSd, typ, data, uintptr(size), flags)
}

// SideDataFree frees a side-data array (av_packet_side_data_free).
func SideDataFree(sd *unsafe.Pointer, nbSd *int32) {
	if ensureModPacket() != nil {
		return
	}
	fPacketSideDataFree(sd, nbSd)
}

// SideDataGet finds a typed entry in a side-data array
// (av_packet_side_data_get; 无状态查询, nil 表安全).
func SideDataGet(sd unsafe.Pointer, nbSd, typ int32) unsafe.Pointer {
	if ensureModPacket() != nil {
		return nil
	}
	return fPacketSideDataGet(sd, nbSd, typ)
}

// SideDataNew allocates a typed entry in a side-data array
// (av_packet_side_data_new).
func SideDataNew(sd *unsafe.Pointer, nbSd *int32, typ int32, size int, flags int32) unsafe.Pointer {
	if ensureModPacket() != nil {
		return nil
	}
	return fPacketSideDataNew(sd, nbSd, typ, uintptr(size), flags)
}

// SideDataRemove deletes a typed entry (av_packet_side_data_remove).
func SideDataRemove(sd unsafe.Pointer, nbSd *int32, typ int32) {
	if ensureModPacket() != nil {
		return
	}
	fPacketSideDataRemove(sd, nbSd, typ)
}
