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

// DeviceIO 设备输入输出模块: avdevice 全量导出, 结构体方法直接可用.
//
// Say it plain: 读摄像头麦克风、写屏幕喇叭之前先列设备、打招呼.
// 自定义读写走 FormatDemux 的 IOContext, 这里只管设备.

// DeviceList owns one AVDeviceInfoList* (记得 FreeList).
type DeviceList struct{ ptr unsafe.Pointer }

// Ptr exposes the raw handle.
func (d *DeviceList) Ptr() unsafe.Pointer {
	mustUse(ensureModDeviceIo())
	if d == nil {
		return nil
	}
	return d.ptr
}

var (
	fDeviceVersion    func() uint32
	fDeviceConfig     func() string
	fDeviceLicense    func() string
	fDeviceRegister   func()
	fDeviceListDevs   func(unsafe.Pointer, *unsafe.Pointer) int32
	fDeviceFreeList   func(*unsafe.Pointer)
	fDeviceListInput  func(unsafe.Pointer, string, unsafe.Pointer, *unsafe.Pointer) int32
	fDeviceListOutput func(unsafe.Pointer, string, unsafe.Pointer, *unsafe.Pointer) int32
	fDeviceAppToDev   func(unsafe.Pointer, int32, unsafe.Pointer, uintptr) int32
	fDeviceDevToApp   func(unsafe.Pointer, int32, unsafe.Pointer, uintptr) int32
)

// ensureModDeviceIo 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modDeviceIoOnce sync.Once

func ensureModDeviceIo() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	modDeviceIoOnce.Do(func() { registerDeviceIo(libHandle) })
	return nil
}

func registerDeviceIo(h uintptr) {
	purego.RegisterLibFunc(&fDeviceVersion, h, "avdevice_version")
	purego.RegisterLibFunc(&fDeviceConfig, h, "avdevice_configuration")
	purego.RegisterLibFunc(&fDeviceLicense, h, "avdevice_license")
	purego.RegisterLibFunc(&fDeviceRegister, h, "avdevice_register_all")
	purego.RegisterLibFunc(&fDeviceListDevs, h, "avdevice_list_devices")
	purego.RegisterLibFunc(&fDeviceFreeList, h, "avdevice_free_list_devices")
	purego.RegisterLibFunc(&fDeviceListInput, h, "avdevice_list_input_sources")
	purego.RegisterLibFunc(&fDeviceListOutput, h, "avdevice_list_output_sinks")
	purego.RegisterLibFunc(&fDeviceAppToDev, h, "avdevice_app_to_dev_control_message")
	purego.RegisterLibFunc(&fDeviceDevToApp, h, "avdevice_dev_to_app_control_message")
}

// RegisterAll registers all input/output devices (用设备前调一次).
func (d *DeviceList) RegisterAll() {
	mustUse(ensureModDeviceIo())
	fDeviceRegister()
}

// Version returns the avdevice version number.
func (d *DeviceList) Version() uint32 {
	mustUse(ensureModDeviceIo())
	return fDeviceVersion()
}

// ListDevices lists devices of a device context and returns the count
// (avdevice_list_devices; ctx 须是设备上下文 (c->oformat/c->iformat 带取表口),
// 文件解复用上下文走 ENOSYS 报错不给表; 成功才把表存进 d, 记得 FreeList).
func (d *DeviceList) ListDevices(ctx unsafe.Pointer) (int, error) {
	if err := ensureModDeviceIo(); err != nil {
		return 0, err
	}
	if d == nil {
		return 0, errNilDevice
	}
	if ret := fDeviceListDevs(ctx, &d.ptr); ret < 0 {
		return 0, codeErr("avdevice_list_devices", ret)
	} else {
		return int(ret), nil
	}
}

// FreeList releases the list and nils the holder.
func (d *DeviceList) FreeList() {
	mustUse(ensureModDeviceIo())
	if d == nil || d.ptr == nil {
		return
	}
	ptr := d.ptr
	d.ptr = nil
	fDeviceFreeList(&ptr)
}

// ListInputSources lists capture devices for an input format and returns
// the count (avdevice_list_input_sources; format 传 AVInputFormat 指针 (可 nil),
// name 是设备名 (可 ""), options 传 Dictionary.Ptr() (可 nil); 瞎写名字 C 干净报
// EINVAL 不崩; 成功才把表存进 d, 记得 FreeList).
func (d *DeviceList) ListInputSources(format unsafe.Pointer, name string, options unsafe.Pointer) (int, error) {
	if err := ensureModDeviceIo(); err != nil {
		return 0, err
	}
	if d == nil {
		return 0, errNilDevice
	}
	if ret := fDeviceListInput(format, name, options, &d.ptr); ret < 0 {
		return 0, codeErr("avdevice_list_input_sources", ret)
	} else {
		return int(ret), nil
	}
}

// ListOutputSinks lists playback devices for an output format and returns
// the count (avdevice_list_output_sinks; 参数与 ListInputSources 同理,
// format 传 AVOutputFormat 指针).
func (d *DeviceList) ListOutputSinks(format unsafe.Pointer, name string, options unsafe.Pointer) (int, error) {
	if err := ensureModDeviceIo(); err != nil {
		return 0, err
	}
	if d == nil {
		return 0, errNilDevice
	}
	if ret := fDeviceListOutput(format, name, options, &d.ptr); ret < 0 {
		return 0, codeErr("avdevice_list_output_sinks", ret)
	} else {
		return int(ret), nil
	}
}

// Configuration returns the avdevice build configuration string.
func (d *DeviceList) Configuration() string {
	if ensureModDeviceIo() != nil {
		return ""
	}
	return fDeviceConfig()
}

// License returns the avdevice license string.
func (d *DeviceList) License() string {
	if ensureModDeviceIo() != nil {
		return ""
	}
	return fDeviceLicense()
}

// AppToDev sends an app-to-device control message (avdevice_app_to_dev_control_message;
// ctx 须是输出设备上下文, 文件/输入上下问走 ENOSYS; type 用 AVAppToDevMessageType
// 常量 (0=无消息/1=暂停/2=播放...), data 传 nil 表无负载 (ENOSYS 路上 C 碰都不碰)).
func (d *DeviceList) AppToDev(ctx unsafe.Pointer, typ int32, data unsafe.Pointer, size int) error {
	if err := ensureModDeviceIo(); err != nil {
		return err
	}
	if d == nil {
		return errNilDevice
	}
	if ret := fDeviceAppToDev(ctx, typ, data, uintptr(size)); ret < 0 {
		return codeErr("avdevice_app_to_dev_control_message", ret)
	}
	return nil
}

// DevToApp reads a device-to-app control message (avdevice_dev_to_app_control_message;
// 没装回调的上下文走 ENOSYS, data 传 nil 可 (C 只转发指针不读)).
func (d *DeviceList) DevToApp(ctx unsafe.Pointer, typ int32, data unsafe.Pointer, size int) error {
	if err := ensureModDeviceIo(); err != nil {
		return err
	}
	if d == nil {
		return errNilDevice
	}
	if ret := fDeviceDevToApp(ctx, typ, data, uintptr(size)); ret < 0 {
		return codeErr("avdevice_dev_to_app_control_message", ret)
	}
	return nil
}
