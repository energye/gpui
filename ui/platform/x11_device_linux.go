//go:build linux

package platform

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// X11 device hot-plug (S6-P1 H 组 X11 侧).
//
// 监听 XI2 层级变化 (XI_HierarchyChanged, evtype 11) 报键盘/鼠标/触控/笔的
// 增减：Added → EventDeviceAdded，Removed → EventDeviceRemoved。
// 分类先查 XIQueryDevice(触控看 XITouchClass，笔看名字)，查不到回落到
// hierarchy 自带的 use(键/鼠)；移除时设备已不在，优先回放 Added 时缓存
// 的分类，保证触控笔移除不坍缩成鼠标。
//
// Wayland 座位能力那半无环境先空着(见 wayland_seat_linux.go 注释)，本文件只做 X11。

const (
	xiHierarchyChanged = 11
	xiAllDevices       = 0

	xiFlagMasterAdded   = 1 << 0
	xiFlagMasterRemoved = 1 << 1
	xiFlagSlaveAdded    = 1 << 2
	xiFlagSlaveRemoved  = 1 << 3

	xiUseMasterPointer = 1
	xiUseMasterKeyboard = 2
	xiUseSlavePointer  = 3
	xiUseSlaveKeyboard = 4
	xiUseFloatingSlave = 5

	xiClassTouch = 8
)

// XIHierarchyEvent(Xlib 版，64 位)字段偏移，见 /usr/include/X11/extensions/XInput2.h。
const (
	xiHierFlagsOff   = 48
	xiHierNumInfoOff = 52
	xiHierInfoOff    = 56
	xiHierEventSize  = 64
)

// XIHierarchyInfo 每条 20 字节。
const (
	xiInfoDeviceOff = 0
	xiInfoAttachOff = 4
	xiInfoUseOff    = 8
	xiInfoFlagsOff  = 16
	xiInfoSize      = 20
)

// XIDeviceInfo(64 位)字段偏移。
const (
	xiDevIDOff         = 0
	xiDevNameOff       = 8
	xiDevUseOff        = 16
	xiDevNumClassesOff = 28
	xiDevClassesOff    = 32
	xiDevSize          = 40
)

var (
	xiDevOnce         sync.Once
	xiDevOK           bool
	xXIQueryDevice    func(dpy uintptr, deviceid int32, ndevices *int32) uintptr
	xXIFreeDeviceInfo func(info uintptr) uintptr
)

func x11ResolveXIDeviceQuery() bool {
	xiDevOnce.Do(func() {
		for _, name := range []string{"libXi.so.6", "libXi.so"} {
			lib, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				continue
			}
			if _, err := purego.Dlsym(lib, "XIQueryDevice"); err != nil {
				continue
			}
			if _, err := purego.Dlsym(lib, "XIFreeDeviceInfo"); err != nil {
				continue
			}
			purego.RegisterLibFunc(&xXIQueryDevice, lib, "XIQueryDevice")
			purego.RegisterLibFunc(&xXIFreeDeviceInfo, lib, "XIFreeDeviceInfo")
			xiDevOK = true
			return
		}
	})
	return xiDevOK
}

// x11SelectHierarchy 在 win 上选 XI_HierarchyChanged(XIAllDevices)。
// 返回 XI 扩展 opcode(0 = 不可用)。与触控选择按 deviceid 共存，互不覆盖。
// 版本号必须与触控探测一致(2.2)：同一 Display 上 QueryVersion 先定后不可变，
// 两次不同版本第二次必 BadValue 致命(Xlib 默认 handler 直接杀进程)。
func x11SelectHierarchy(dpy, win uintptr) (int32, bool) {
	if dpy == 0 || win == 0 || !x11ResolveXI() {
		return 0, false
	}
	name := append([]byte("XInputExtension"), 0)
	var major, firstEvent, firstError int32
	if xXIQueryExtension(dpy, &name[0], &major, &firstEvent, &firstError) == 0 {
		return 0, false
	}
	var vmajor, vminor int32 = 2, 2
	if xXIQueryVersion(dpy, &vmajor, &vminor) != 0 {
		return 0, false
	}
	if vmajor < 2 {
		return 0, false
	}
	// evtype 11 落在 mask byte1 bit3。
	mask := [2]byte{0, 1 << 3}
	sel := xiEventMask{deviceid: xiAllDevices, maskLen: int32(len(mask)), mask: &mask[0]}
	if xXISelectEvents(dpy, win, unsafe.Pointer(&sel), 1) != 0 {
		return 0, false
	}
	return major, true
}

// xiHierarchyInfo 是解析后的单条层级变化。
type xiHierarchyInfo struct {
	deviceid int
	use      int
	flags    int
}

// parseXIHierarchy 解析 GetEventData 后的 XIHierarchyEvent。
// 只做纯字节解析，不碰 X 库，可单测。
func parseXIHierarchy(data []byte) ([]xiHierarchyInfo, bool) {
	if len(data) < xiHierEventSize {
		return nil, false
	}
	n := int(readI32(data, xiHierNumInfoOff))
	if n <= 0 || n > 64 {
		return nil, false
	}
	infoPtr := readU64(data, xiHierInfoOff)
	if infoPtr == 0 {
		return nil, false
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(infoPtr))), n*xiInfoSize)
	out := make([]xiHierarchyInfo, 0, n)
	for i := 0; i < n; i++ {
		base := i * xiInfoSize
		out = append(out, xiHierarchyInfo{
			deviceid: int(readI32(raw, base+xiInfoDeviceOff)),
			use:      int(readI32(raw, base+xiInfoUseOff)),
			flags:    int(readI32(raw, base+xiInfoFlagsOff)),
		})
	}
	return out, true
}

// x11IsPenName reports pen devices by name heuristic (case-insensitive).
func x11IsPenName(name string) bool {
	s := strings.ToLower(name)
	for _, k := range []string{"pen", "stylus", "wacom", "eraser", "tablet", "huion"} {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// x11UseToClass 只看 use 的回落分类(查不到设备信息时用)。
func x11UseToClass(use int) DeviceClass {
	switch use {
	case xiUseMasterKeyboard, xiUseSlaveKeyboard:
		return DeviceKeyboard
	default:
		return DeviceMouse
	}
}

// x11QueryDeviceInfo 查单个设备的名/use/是否触控。失败回 ok=false。
func x11QueryDeviceInfo(dpy uintptr, deviceid int) (name string, use int, hasTouch bool, ok bool) {
	if dpy == 0 || !x11ResolveXIDeviceQuery() || xXIQueryDevice == nil {
		return "", 0, false, false
	}
	var ndev int32
	info := xXIQueryDevice(dpy, int32(deviceid), &ndev)
	if info == 0 || ndev <= 0 {
		return "", 0, false, false
	}
	defer func() {
		if xXIFreeDeviceInfo != nil {
			xXIFreeDeviceInfo(info)
		}
	}()
	// XIQueryDevice(deviceid) 一般只回一个；按 id 匹配，找不到取第一个。
	base := info
	count := int(ndev)
	for i := 0; i < count; i++ {
		entry := unsafe.Slice((*byte)(unsafe.Pointer(base+uintptr(i*xiDevSize))), xiDevSize)
		id := int(readI32(entry, xiDevIDOff))
		if count > 1 && id != deviceid {
			continue
		}
		namePtr := readU64(entry, xiDevNameOff)
		if namePtr != 0 {
			name = x11CString((*byte)(unsafe.Pointer(uintptr(namePtr))))
		}
		use = int(readI32(entry, xiDevUseOff))
		ncls := int(readI32(entry, xiDevNumClassesOff))
		clsPtr := readU64(entry, xiDevClassesOff)
		if ncls > 0 && ncls < 64 && clsPtr != 0 {
			arr := unsafe.Slice((*uintptr)(unsafe.Pointer(uintptr(clsPtr))), ncls)
			for _, cp := range arr {
				if cp == 0 {
					continue
				}
				typ := int(*(*int32)(unsafe.Pointer(uintptr(cp))))
				if typ == xiClassTouch {
					hasTouch = true
					break
				}
			}
		}
		return name, use, hasTouch, true
	}
	return "", 0, false, false
}

// x11ClassifyDevice 综合分类：触控类优先，笔看名，其余看 use。
func x11ClassifyDevice(dpy uintptr, deviceid, use int) (DeviceClass, string) {
	if name, quse, hasTouch, ok := x11QueryDeviceInfo(dpy, deviceid); ok {
		if hasTouch {
			return DeviceTouch, name
		}
		if x11IsPenName(name) {
			return DevicePen, name
		}
		if quse != 0 {
			use = quse
		}
		return x11UseToClass(use), name
	}
	return x11UseToClass(use), ""
}

// xiHierarchyToEvents 把解析后的 infos 转成平台事件。
// Added 存缓存，Removed 优先回放缓存(设备已消失查不到)。
func xiHierarchyToEvents(st *x11State, infos []xiHierarchyInfo) []Event {
	var out []Event
	for _, in := range infos {
		added := in.flags&(xiFlagMasterAdded|xiFlagSlaveAdded) != 0
		removed := in.flags&(xiFlagMasterRemoved|xiFlagSlaveRemoved) != 0
		if !added && !removed {
			continue
		}
		// 同一条又加又减极罕见，按加处理。
		if added {
			class, name := x11ClassifyDevice(st.display, in.deviceid, in.use)
			st.devMu.Lock()
			if st.devClasses == nil {
				st.devClasses = make(map[int]DeviceClass)
			}
			if st.devNames == nil {
				st.devNames = make(map[int]string)
			}
			st.devClasses[in.deviceid] = class
			st.devNames[in.deviceid] = name
			st.devMu.Unlock()
			out = append(out, Event{Type: EventDeviceAdded, DeviceClass: class, DeviceName: name})
			continue
		}
		st.devMu.Lock()
		class, ok := st.devClasses[in.deviceid]
		name := st.devNames[in.deviceid]
		if ok {
			delete(st.devClasses, in.deviceid)
			delete(st.devNames, in.deviceid)
		}
		st.devMu.Unlock()
		if !ok {
			class, name = x11ClassifyDevice(st.display, in.deviceid, in.use)
		}
		out = append(out, Event{Type: EventDeviceRemoved, DeviceClass: class, DeviceName: name})
	}
	return out
}

// decodeXIHierarchy 处理一个 GenericEvent：XI 层级变化 → 设备增减事件。
func (h *x11Host) decodeXIHierarchy(buf []byte) ([]Event, bool) {
	st := h.st
	if st == nil || !st.xiHierarchy || st.xiMajor == 0 || len(buf) < 56 {
		return nil, false
	}
	if int32(readU32(buf, xiEvExtensionOff)) != st.xiMajor {
		return nil, false
	}
	if int(readU32(buf, xiEvTypeOff)) != xiHierarchyChanged {
		return nil, false
	}
	if xXIGetEventData == nil || xXIFreeEventData == nil {
		return nil, false
	}
	if xXIGetEventData(st.display, unsafe.Pointer(&buf[0])) == 0 {
		return nil, false
	}
	defer xXIFreeEventData(st.display, unsafe.Pointer(&buf[0]))
	dataPtr := *(*unsafe.Pointer)(unsafe.Pointer(&buf[48]))
	if dataPtr == nil {
		return nil, false
	}
	data := unsafe.Slice((*byte)(dataPtr), xiHierEventSize)
	infos, ok := parseXIHierarchy(data)
	if !ok || len(infos) == 0 {
		return nil, false
	}
	evs := xiHierarchyToEvents(st, infos)
	if len(evs) == 0 {
		return nil, false
	}
	return evs, true
}

// x11SeedDeviceCache 启动时把现存设备记入缓存，免得老设备移除时分类丢失。
func x11SeedDeviceCache(st *x11State) {
	if st == nil || st.display == 0 || !x11ResolveXIDeviceQuery() || xXIQueryDevice == nil {
		return
	}
	var ndev int32
	info := xXIQueryDevice(st.display, xiAllDevices, &ndev)
	if info == 0 || ndev <= 0 || ndev > 128 {
		return
	}
	defer func() {
		if xXIFreeDeviceInfo != nil {
			xXIFreeDeviceInfo(info)
		}
	}()
	for i := 0; i < int(ndev); i++ {
		entry := unsafe.Slice((*byte)(unsafe.Pointer(info + uintptr(i*xiDevSize))), xiDevSize)
		id := int(readI32(entry, xiDevIDOff))
		var name string
		if p := readU64(entry, xiDevNameOff); p != 0 {
			name = x11CString((*byte)(unsafe.Pointer(uintptr(p))))
		}
		use := int(readI32(entry, xiDevUseOff))
		ncls := int(readI32(entry, xiDevNumClassesOff))
		clsPtr := readU64(entry, xiDevClassesOff)
		hasTouch := false
		if ncls > 0 && ncls < 64 && clsPtr != 0 {
			arr := unsafe.Slice((*uintptr)(unsafe.Pointer(uintptr(clsPtr))), ncls)
			for _, cp := range arr {
				if cp == 0 {
					continue
				}
				if int(*(*int32)(unsafe.Pointer(uintptr(cp)))) == xiClassTouch {
					hasTouch = true
					break
				}
			}
		}
		var class DeviceClass
		switch {
		case hasTouch:
			class = DeviceTouch
		case x11IsPenName(name):
			class = DevicePen
		default:
			class = x11UseToClass(use)
		}
		st.devMu.Lock()
		if st.devClasses == nil {
			st.devClasses = make(map[int]DeviceClass)
		}
		if st.devNames == nil {
			st.devNames = make(map[int]string)
		}
		st.devClasses[id] = class
		st.devNames[id] = name
		st.devMu.Unlock()
	}
}
