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
	fDeviceListDevs   func(unsafe.Pointer, unsafe.Pointer, *unsafe.Pointer) int32
	fDeviceFreeList   func(*unsafe.Pointer)
	fDeviceListInput  func(unsafe.Pointer, *unsafe.Pointer) int32
	fDeviceListOutput func(unsafe.Pointer, *unsafe.Pointer) int32
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

// ListDevices lists devices of a source/sink context (ctx 传 FormatContext.Ptr()).
func (d *DeviceList) ListDevices(ctx unsafe.Pointer) error {
	if err := ensureModDeviceIo(); err != nil {
		return err
	}
	if d == nil {
		return errNilDevice
	}
	if ret := fDeviceListDevs(ctx, ctx, &d.ptr); ret < 0 {
		return codeErr("avdevice_list_devices", ret)
	}
	return nil
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

// ListInputSources lists capture devices for an input format.
func (d *DeviceList) ListInputSources(format unsafe.Pointer) error {
	if err := ensureModDeviceIo(); err != nil {
		return err
	}
	if d == nil {
		return errNilDevice
	}
	if ret := fDeviceListInput(format, &d.ptr); ret < 0 {
		return codeErr("avdevice_list_input_sources", ret)
	}
	return nil
}

// ListOutputSinks lists playback devices for an output format.
func (d *DeviceList) ListOutputSinks(format unsafe.Pointer) error {
	if err := ensureModDeviceIo(); err != nil {
		return err
	}
	if d == nil {
		return errNilDevice
	}
	if ret := fDeviceListOutput(format, &d.ptr); ret < 0 {
		return codeErr("avdevice_list_output_sinks", ret)
	}
	return nil
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

// AppToDev sends an app-to-device control message (音量/暂停等走它;
// type 用 AVAppToDevMessageType 常量, data 传 nil 表无负载).
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

// DevToApp reads a device-to-app control message (设备状态回调用它).
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
