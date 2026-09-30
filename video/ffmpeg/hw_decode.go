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
	"errors"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

// P1 硬解：ffmpeg 硬解设备选择 + 解码器绑定 + 硬解帧回传 + 统计 + 回落。
//
// 大白话：解码默认走显卡，显卡不行自动换 CPU，帧都拿回内存再转 RGBA。
// 链路：建硬解设备 → 解码器绑设备 → 取帧时是硬解帧就拷回内存 → 老转色路。
// 任何一步走不通都记一笔回落数，转软解继续播，不崩。

// AVCodecContext 字段偏移（ffmpeg 7.1.5-gpui1 头文件实测）。
// 核法：gcc offsetof（libavcodec/avcodec.h）+ 真机读新上下文验证
// （codec_id@24=27、width@116=0、pix_fmt@140=-1、get_format@192 非空）。
const (
	ccGetFormat   = 192
	ccHwDeviceCtx = 560
)

// AVCodecHWConfig 头 12 字节：pix_fmt@0、methods@4、device_type@8（int32）。
const (
	hwcfgPixFmt = 0
	hwcfgMethod = 4
	hwcfgDevTyp = 8
)

// hwMethodDeviceCtx 是支持 hw_device_ctx 方式的 methods 位（ffmpeg 内部值，直读不断言）。
const hwMethodDeviceCtx = 1

// hwMaxPixFmt 是像素格式枚举合法上限（7.1 头文件约两百余项，取 300 放宽）。
// ccLayoutOK 只验范围，防头文件挪位写坏内存。
const hwMaxPixFmt = 300

// hwTypePreference 按平台列出硬解类型，先后是尝试顺序。
// 各系统只试自家库里编进的类型（见 tools/ffmpeg/build-one.sh）：
// Linux x64 先 VA-API（vaapi/vdpau/drm 编进，arm64/386/arm 无硬解走软解）；
// Windows 按 D3D11VA / DXVA2（编进）/ D3D12VA / QSV / NVDEC（没编进就跳过）；
// macOS 走 VideoToolbox（库未到先占位）；安卓走 MediaCodec（库未到先占位）。
func hwTypePreference() []string {
	return hwTypePreferenceFor(runtime.GOOS)
}

// hwTypePreferenceFor 按系统名给顺序（单测可不换系统直接验全表）。
func hwTypePreferenceFor(goos string) []string {
	switch goos {
	case "windows":
		return []string{"d3d11va", "dxva2", "d3d12va", "qsv", "cuda"}
	case "darwin":
		return []string{"videotoolbox"}
	case "android":
		return []string{"mediacodec"}
	default:
		return []string{"vaapi", "drm", "cuda", "vdpau"}
	}
}

// hwSupportedArch 报告本架构能不能碰硬解上下文。
// 偏移常量只对 64 位有效（ffmpeg 7.1.5-gpui1 头文件实测，指针 8 字节）；
// 32 位（386/arm）直接走软解，不碰内存，不崩。
func hwSupportedArch() bool {
	return unsafe.Sizeof(uintptr(0)) == 8
}

// HWDisabled 报告是否强制软解（GPUI_VIDEO_HW=off/soft）。
// 给测试和对比窗留的开关，默认自动（空值走硬解）。
func HWDisabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GPUI_VIDEO_HW")))
	return v == "off" || v == "soft" || v == "disable" || v == "disabled"
}

// ListHWTypes 列出本构建进的硬解类型名（如 vaapi/drm/vdpau）。
func ListHWTypes() []string {
	mustUse(ensureModCrypto())
	var hw HWDevice
	var out []string
	prev := int32(0)
	for {
		next := hw.HwdeviceIterateTypes(prev)
		if next == 0 {
			break
		}
		if p := hw.HwdeviceGetTypeName(next); p != nil {
			out = append(out, cstr(p))
		}
		prev = next
		if len(out) > 32 {
			break
		}
	}
	return out
}

// openHWDevice 按平台顺序建硬解设备，回设备引用、类型名、类型号。
// 建不起来就报错，调用方记一笔直接走软解，不崩。
// Linux 的 vaapi/drm 直连 /dev/dri/renderD* 节点（逐个试，专挑能用的），
// 不走空设备的 X11 懒恢复（那条先报连不上显示再恢复，慢且抖）。
// Windows/macOS/安卓直接走默认设备（传空，ffmpeg 自建）；
// 32 位架构直接回无设备（偏移只对 64 位有效，不碰内存）。
func openHWDevice() (devRef unsafe.Pointer, typeName string, typeNum int32, err error) {
	if !hwSupportedArch() {
		return nil, "", 0, errNoHWDevice
	}
	mustUse(ensureModCrypto())
	var hw HWDevice
	inBuild := map[string]int32{}
	for _, n := range ListHWTypes() {
		inBuild[n] = hw.HwdeviceFindTypeByName(n)
	}
	for _, want := range hwTypePreference() {
		typ, ok := inBuild[want]
		if !ok {
			continue
		}
		for _, dev := range hwDeviceCandidates(want) {
			var ref unsafe.Pointer
			var cerr error
			if dev == "" {
				cerr = hw.HwdeviceCtxCreate(&ref, typ, nil, nil, 0)
			} else {
				cptr, free := featCStrTmp(dev)
				cerr = hw.HwdeviceCtxCreate(&ref, typ, cptr, nil, 0)
				free()
			}
			if cerr != nil || ref == nil {
				continue
			}
			return ref, want, typ, nil
		}
	}
	return nil, "", 0, errNoHWDevice
}

// hwDeviceCandidates 列出某硬解类型的设备候选（按优先顺序）。
// vaapi/drm 在 Linux 下先逐个试显卡节点，最后才试空（X11 懒恢复兜底）；
// 其他类型与平台直接走空（默认行为，Windows d3d11va/dxva2、macOS
// videotoolbox、安卓 mediacodec 都是 ffmpeg 自建默认设备）。
func hwDeviceCandidates(typeName string) []string {
	return hwDeviceCandidatesFor(runtime.GOOS, typeName)
}

// hwDeviceCandidatesFor 按系统名给候选（单测可不换系统直接验全表，
// 非 Linux 从不碰文件系统）。
func hwDeviceCandidatesFor(goos, typeName string) []string {
	if goos != "linux" {
		return []string{""}
	}
	if typeName != "vaapi" && typeName != "drm" {
		return []string{""}
	}
	var out []string
	if entries, err := os.ReadDir("/dev/dri"); err == nil {
		for _, e := range entries {
			name := e.Name()
			if len(name) > 7 && name[:7] == "renderD" {
				out = append(out, "/dev/dri/"+name)
			}
		}
	}
	// 排序保证 renderD128 先于 renderD129（字典序对纯数字后缀成立）。
	sort.Strings(out)
	return append(out, "")
}

// errNoHWDevice 是平台顺序里没一家能建设备的哨兵（调用方转软解，不上抛）。
var errNoHWDevice = errors.New("ffmpeg: no usable hw device")

// hwPixFmtFor 问解码器对该设备类型的硬解像素格式（如 vaapi）。
// 找不到就回 -1，调用方走软解。
func hwPixFmtFor(decPtr unsafe.Pointer, devType int32) int32 {
	mustUse(ensureModCodecEncode())
	var codec Codec
	for i := int32(0); i < 16; i++ {
		cfg := codec.AvcodecGetHwConfig(decPtr, i)
		if cfg == nil {
			break
		}
		pix := *(*int32)(unsafe.Add(cfg, hwcfgPixFmt))
		methods := *(*int32)(unsafe.Add(cfg, hwcfgMethod))
		dt := *(*int32)(unsafe.Add(cfg, hwcfgDevTyp))
		if dt == devType && methods&hwMethodDeviceCtx != 0 && pix >= 0 {
			return pix
		}
	}
	return -1
}

// installGetFormat 把解码器的像素格式协商换成“有硬解选硬解，否则走默认首选”。
// 回跳板句柄，调用方存好别丢（丢了回调悬空）；一开一跳板，随 Decoder 走。
func installGetFormat(cc unsafe.Pointer, hwPix int32) uintptr {
	cb := purego.NewCallback(func(_ unsafe.Pointer, list unsafe.Pointer) int32 {
		first := int32(-1)
		for i := 0; i < 64; i++ {
			v := *(*int32)(unsafe.Add(list, uintptr(i)*4))
			if v == -1 {
				break
			}
			if first == -1 {
				first = v
			}
			if v == hwPix {
				return v
			}
		}
		return first
	})
	*(*unsafe.Pointer)(unsafe.Add(cc, ccGetFormat)) = *(*unsafe.Pointer)(unsafe.Pointer(&cb))
	return cb
}

// refHWDevice 把设备引用交给解码器（之后归解码器拥有，随释放）。
// 传前先加一笔引用计数，解码器那笔和我们这笔各算各的，Close 只放我们这笔。
func refHWDevice(cc unsafe.Pointer, devRef unsafe.Pointer) {
	mustUse(ensureModBufferMem())
	*(*unsafe.Pointer)(unsafe.Add(cc, ccHwDeviceCtx)) = fBufRef(devRef)
}

// ccLayoutOK 验解码器上下文布局是不是 7.1.5 那套（64 位）。
// 偏移写死最怕换头文件悄悄挪位：挪了就不是提速是写坏内存。
// 开硬解前先读已知槽对一遍（codec_id 对得上、协商函数非空、硬解槽是空、
// 像素格式在合法枚举内），有一个对不上就走软解，不碰内存。
// 注意：调时已过 ParToCtx，pix/sw 不再是 -1（已是流的真实格式），
// 只验范围不验 -1。
func ccLayoutOK(cc unsafe.Pointer, codecID int32) bool {
	if cc == nil {
		return false
	}
	if !hwSupportedArch() {
		return false
	}
	if loadInt32(cc, 24) != codecID {
		return false
	}
	for _, off := range []uintptr{140, 144} {
		v := loadInt32(cc, off)
		if v < -1 || v > hwMaxPixFmt {
			return false
		}
	}
	if *(*uintptr)(unsafe.Add(cc, ccGetFormat)) == 0 {
		return false
	}
	if *(*uintptr)(unsafe.Add(cc, ccHwDeviceCtx)) != 0 {
		return false
	}
	return true
}

// HWStats 是解码器的硬解水位（只加键，不改旧语义）。
type HWStats struct {
	Active        bool
	Name          string
	Fallbacks     int64
	TransferMsAvg float64
}

// hwState 是 Decoder 里的硬解记账（原子操作，Stats 可在任意线程读）。
type hwState struct {
	name      atomic.Value // string
	active    atomic.Bool
	fallbacks atomic.Int64
	xferCount atomic.Uint64
	xferNs    atomic.Uint64
}

func (s *hwState) stats() HWStats {
	name, _ := s.name.Load().(string)
	if name == "" {
		name = "soft"
	}
	var avg float64
	if n := s.xferCount.Load(); n > 0 {
		avg = float64(s.xferNs.Load()) / float64(n) / 1e6
	}
	return HWStats{
		Active:        s.active.Load(),
		Name:          name,
		Fallbacks:     s.fallbacks.Load(),
		TransferMsAvg: avg,
	}
}

// ensureModHW 开硬解房的灯：设备、协商、帧、缓冲都要亮。
// 符号都是本包已绑过的（crypto/codec/frame/buffer），这里只保证顺序。
var modHWOnce sync.Once

func ensureModHW() error {
	if err := ensureModDecode(); err != nil {
		return err
	}
	if err := ensureModCodecEncode(); err != nil {
		return err
	}
	if err := ensureModCrypto(); err != nil {
		return err
	}
	if err := ensureModBufferMem(); err != nil {
		return err
	}
	modHWOnce.Do(func() {})
	return nil
}
