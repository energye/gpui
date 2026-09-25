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

	fNetInit   func() int32
	fNetDeinit func() int32
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
	fSeekFile    func(unsafe.Pointer, int32, int64, int64, int64, int32) int32
	fCloseInput  func(*unsafe.Pointer)
	fFreeCtx     func(*unsafe.Pointer)
	fFlushBuf    func(unsafe.Pointer)
	fBufUnref    func(*unsafe.Pointer)
	fHWCreate    func(*unsafe.Pointer, int32, string, unsafe.Pointer, int32) int32
)

// libRelName returns the repository-relative library path for this platform.
func libRelName() string {
	switch runtime.GOOS {
	case "windows":
		if runtime.GOARCH == "arm64" {
			return filepath.Join("lib", "ffmpeg", "win-arm64", "libgpui_ffmpeg.dll")
		}
		return filepath.Join("lib", "ffmpeg", "win-x64", "libgpui_ffmpeg.dll")
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return filepath.Join("lib", "ffmpeg", "darwin-arm64", "libgpui_ffmpeg.dylib")
		}
		return filepath.Join("lib", "ffmpeg", "darwin-x64", "libgpui_ffmpeg.dylib")
	default:
		switch runtime.GOARCH {
		case "arm64":
			return filepath.Join("lib", "ffmpeg", "linux-arm64", "libgpui_ffmpeg.so")
		case "386":
			return filepath.Join("lib", "ffmpeg", "linux-386", "libgpui_ffmpeg.so")
		case "arm":
			return filepath.Join("lib", "ffmpeg", "linux-arm", "libgpui_ffmpeg.so")
		default:
			return filepath.Join("lib", "ffmpeg", "linux-x64", "libgpui_ffmpeg.so")
		}
	}
}

// candidatePaths lists library locations to try in order.
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
	rel := libRelName()
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

// ensureLoaded dlopens the library once and registers every function.
// It returns a readable error when the file is missing so callers can
// skip instead of crashing.
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
		purego.RegisterLibFunc(&fNetInit, h, "avformat_network_init")
		purego.RegisterLibFunc(&fNetDeinit, h, "avformat_network_deinit")
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
		purego.RegisterLibFunc(&fSeekFile, h, "avformat_seek_file")
		purego.RegisterLibFunc(&fCloseInput, h, "avformat_close_input")
		purego.RegisterLibFunc(&fFreeCtx, h, "avcodec_free_context")
		purego.RegisterLibFunc(&fFlushBuf, h, "avcodec_flush_buffers")
		purego.RegisterLibFunc(&fBufUnref, h, "av_buffer_unref")
		purego.RegisterLibFunc(&fHWCreate, h, "av_hwdevice_ctx_create")
		registerPacket(h)
		registerFrame(h)
		registerDictOpt(h)
		registerBufferMem(h)
		registerErrorLog(h)
		registerFormatDemux(h)
		registerCodecEncode(h)
		registerFilterGraph(h)
		registerDeviceIo(h)
		registerResampleAudio(h)
		registerMediaDesc(h)
		registerCryptoHashMisc(h)
		registerScaleColor(h)
		fLogSetLevel(LogError)
		return nil
	}
	loadErr = fmt.Errorf("ffmpeg: library not found (tried %q... last %s)", libRelName(), last)
	return loadErr
}

// Available reports whether the shared library loads on this machine.
func Available() bool { return ensureLoaded() == nil }

// LibPath reports the loaded file, "" when unavailable.
func LibPath() string {
	if ensureLoaded() != nil {
		return ""
	}
	return libPath
}

// Version returns the ffmpeg version string (for example "7.1.5").
func Version() (string, error) {
	if err := ensureLoaded(); err != nil {
		return "", err
	}
	return fVerInfo(), nil
}

// errText turns a negative AVERROR into a readable string.
func errText(code int32) string {
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
