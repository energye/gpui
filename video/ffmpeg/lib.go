package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	loadMu  sync.Mutex
	loaded  bool
	loadErr error
	libPath string
	// libHandle 留着给数据符号用：函数走 RegisterLibFunc，
	// 数据（const int / const char[]）走 Dlsym 取地址再读。
	libHandle uintptr

	fNetInit func() int32
	// Handles stay as unsafe.Pointer end to end: purego passes them
	// straight through and the load helpers read fields with unsafe.Add,
	// so go vet's unsafeptr check stays quiet.
	fOpenInput   func(*unsafe.Pointer, string, unsafe.Pointer, unsafe.Pointer) int32
	fFindInfo    func(unsafe.Pointer, unsafe.Pointer) int32
	fBestStream  func(unsafe.Pointer, int32, int32, int32, *unsafe.Pointer, int32) int32
	fFindDecoder func(int32) unsafe.Pointer
	fAllocCtx    func(unsafe.Pointer) unsafe.Pointer
	fParToCtx    func(unsafe.Pointer, unsafe.Pointer) int32
	fOpen2       func(unsafe.Pointer, unsafe.Pointer, unsafe.Pointer) int32
	fSendPacket  func(unsafe.Pointer, unsafe.Pointer) int32
	fRecvFrame   func(unsafe.Pointer, unsafe.Pointer) int32
	fReadFrame   func(unsafe.Pointer, unsafe.Pointer) int32
	fSeekFrame   func(unsafe.Pointer, int32, int64, int32) int32
	fCloseInput  func(*unsafe.Pointer)
	fFreeCtx     func(*unsafe.Pointer)
	fFlushBuf    func(unsafe.Pointer)
)

// libRelName returns the repository-relative library path for this platform.
// 基础版文件名（旧名不动，默认加载）。
func libRelName() string { return libRelNameFor("base") }

// libRelNameFull returns the full-version path (same dir, _full suffix).
// 高级版文件名（同目录 _full 后缀，显式指定才加载）。
func libRelNameFull() string { return libRelNameFor("full") }

// libRelNameFor maps variant to file name: base keeps the old name,
// full adds _full before the extension.
func libRelNameFor(variant string) string {
	full := variant == "full"
	switch runtime.GOOS {
	case "windows":
		name := "libgpui_ffmpeg.dll"
		if full {
			name = "libgpui_ffmpeg_full.dll"
		}
		if runtime.GOARCH == "arm64" {
			return filepath.Join("lib", "ffmpeg", "win-arm64", name)
		}
		return filepath.Join("lib", "ffmpeg", "win-x64", name)
	case "darwin":
		name := "libgpui_ffmpeg.dylib"
		if full {
			name = "libgpui_ffmpeg_full.dylib"
		}
		if runtime.GOARCH == "arm64" {
			return filepath.Join("lib", "ffmpeg", "darwin-arm64", name)
		}
		return filepath.Join("lib", "ffmpeg", "darwin-x64", name)
	default:
		name := "libgpui_ffmpeg.so"
		if full {
			name = "libgpui_ffmpeg_full.so"
		}
		switch runtime.GOARCH {
		case "arm64":
			return filepath.Join("lib", "ffmpeg", "linux-arm64", name)
		case "386":
			return filepath.Join("lib", "ffmpeg", "linux-386", name)
		case "arm":
			return filepath.Join("lib", "ffmpeg", "linux-arm", name)
		default:
			return filepath.Join("lib", "ffmpeg", "linux-x64", name)
		}
	}
}

// ffmpegVariant reads GPUI_FFMPEG_VARIANT: "full" enters full version,
// anything else (including empty) stays on base version.
// 默认基础版，GPUI_FFMPEG_VARIANT=full 才进高级版。
// GPUI_FFMPEG_PATH 直接指文件时不受此限制（指哪加载哪）。
func ffmpegVariant() string {
	if v := os.Getenv("GPUI_FFMPEG_VARIANT"); v == "full" || v == "FULL" || v == "Full" {
		return "full"
	}
	return "base"
}

// candidatePaths lists library locations to try in order.
// 默认只找基础版；GPUI_FFMPEG_VARIANT=full 才找高级版；
// GPUI_FFMPEG_PATH 直接指文件时指哪找哪（基础高级都行）。
func candidatePaths() []string {
	if p := os.Getenv("GPUI_FFMPEG_PATH"); p != "" {
		return []string{p}
	}
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	rel := libRelNameFor(ffmpegVariant())
	if exe, err := os.Executable(); err == nil {
		add(filepath.Join(filepath.Dir(exe), rel))
		// go test binaries live deep in /tmp; walk up a few levels too.
		dir := filepath.Dir(exe)
		for i := 0; i < 3; i++ {
			dir = filepath.Dir(dir)
			add(filepath.Join(dir, rel))
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		add(filepath.Join(cwd, rel))
		add(filepath.Join(cwd, "..", rel))
		add(filepath.Join(cwd, "..", "..", rel))
		// When tests run inside video/ffmpeg, the repo root is two up.
	}
	// Absolute dev fallback keeps local runs working when invoked from
	// odd directories; production callers set GPUI_FFMPEG_PATH instead.
	if home := os.Getenv("HOME"); home != "" {
		add(filepath.Join(home, "app", "projects", "gogpu", "gpui", rel))
	}
	return out
}

// ensureLoaded 只负责 dlopen（找到库、打开、记住句柄），不再绑任何函数。
// 大白话：开门只开门，屋里 13 间房的灯各房自己开——谁用谁开，开过不再开。
// 之前一次全绑 1024 个，ARM/386/win 上缺 6-7 个 x86 专有符号（VDPAU 等）
// 直接 panic，连解码都用不了；现在缺的符号只在真用到它那间房时才报错。
func ensureLoaded() error {
	loadMu.Lock()
	defer loadMu.Unlock()
	if loaded {
		return loadErr
	}
	loaded = true
	var last string
	for _, p := range candidatePaths() {
		if _, err := os.Stat(p); err != nil {
			last = p
			continue
		}
		h, err := purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("ffmpeg: dlopen %s: %w", p, err)
			return loadErr
		}
		libPath = p
		libHandle = h
		return nil
	}
	loadErr = fmt.Errorf("ffmpeg: library not found (tried %q... last %s)", libRelName(), last)
	return loadErr
}

// modCoreOnce 守 16 个核心函数：只开库、读包、送包收帧，解码看片全靠它们。
// 各模块 ensure 进门先调它（sync.Once，开过即过）。
var modCoreOnce sync.Once

func ensureModCore() error {
	if err := ensureLoaded(); err != nil {
		return err
	}
	modCoreOnce.Do(func() {
		h := libHandle
		purego.RegisterLibFunc(&fNetInit, h, "avformat_network_init")
		purego.RegisterLibFunc(&fOpenInput, h, "avformat_open_input")
		purego.RegisterLibFunc(&fFindInfo, h, "avformat_find_stream_info")
		purego.RegisterLibFunc(&fBestStream, h, "av_find_best_stream")
		purego.RegisterLibFunc(&fFindDecoder, h, "avcodec_find_decoder")
		purego.RegisterLibFunc(&fAllocCtx, h, "avcodec_alloc_context3")
		purego.RegisterLibFunc(&fParToCtx, h, "avcodec_parameters_to_context")
		purego.RegisterLibFunc(&fOpen2, h, "avcodec_open2")
		purego.RegisterLibFunc(&fSendPacket, h, "avcodec_send_packet")
		purego.RegisterLibFunc(&fRecvFrame, h, "avcodec_receive_frame")
		purego.RegisterLibFunc(&fReadFrame, h, "av_read_frame")
		purego.RegisterLibFunc(&fSeekFrame, h, "av_seek_frame")
		purego.RegisterLibFunc(&fCloseInput, h, "avformat_close_input")
		purego.RegisterLibFunc(&fFreeCtx, h, "avcodec_free_context")
		purego.RegisterLibFunc(&fFlushBuf, h, "avcodec_flush_buffers")
	})
	return nil
}

// mustUse 给不返回 error 的小函数用：库不在就直接报人话 panic。
// 大白话：取指针、读名字这类小函数没法返回值报错，库没加载还硬调
// 原来是野指针崩，现在直接说库没找到，不玩神秘崩溃。
func mustUse(err error) {
	if err != nil {
		panic(err)
	}
}

// Available reports whether the shared library loads on this machine.
func Available() bool { return ensureLoaded() == nil }

// Variant reports which library version loads: "base" or "full".
// 默认 base；GPUI_FFMPEG_VARIANT=full 切 full。
func Variant() string { return ffmpegVariant() }

// LibPath reports the loaded file, "" when unavailable.
func LibPath() string {
	if ensureLoaded() != nil {
		return ""
	}
	return libPath
}

// IsFull reports whether the loaded library is the full version.
// 高级版才有写文件（复用+编码+烧字），基础版调写接口会报可读错。
func IsFull() bool {
	if ensureLoaded() != nil {
		return false
	}
	return ffmpegVariant() == "full"
}

// Version returns the ffmpeg version string (for example "7.1.5").
func Version() (string, error) {
	if err := ensureModErrorLog(); err != nil {
		return "", err
	}
	return fVerInfo(), nil
}

// errText turns a negative AVERROR into a readable string.
func errText(code int32) string {
	if ensureModErrorLog() != nil {
		return fmt.Sprintf("ffmpeg error %d", code)
	}
	buf := make([]byte, 256)
	ret := fErrStrerror(code, unsafe.Pointer(&buf[0]), uintptr(len(buf)))
	if ret != 0 {
		return fmt.Sprintf("ffmpeg error %d", code)
	}
	n := 0
	for n < len(buf) && buf[n] != 0 {
		n++
	}
	return string(buf[:n])
}
