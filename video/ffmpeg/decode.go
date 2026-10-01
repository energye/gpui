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
	"fmt"
	"sync"
	"time"
	"unsafe"
)

// ErrEOF reports the stream decoded to its end.
var ErrEOF = errors.New("ffmpeg: end of stream")

// StreamInfo describes the opened video stream.
type StreamInfo struct {
	Path      string
	Width     int
	Height    int
	FPS       float64
	DurMs     int64
	Frames    int64
	CodecID   int32
	SrcPixFmt int32
}

// VideoFrame is one decoded picture, ready to wrap into
// the current library's frame type (width, height, pixels, stamp).
// RGBA shape: Pix holds 4*Width*Height (default, Pix path).
// NV12 shape (SetNV12 path): Y holds Width*Height luma and UV holds
// Width*Height/2 interleaved chroma; Pix is nil. NV12 reports which
// shape this frame carries.
// Pix comes from the Decoder's PixPool when set (player path, pooled
// reuse, steady zero-alloc); otherwise it is a fresh buffer. Same for
// Y/UV via SetPlanePool.
// Call Release exactly once when the picture is dropped before display;
// displayed frames transfer ownership to the display pool instead.
type VideoFrame struct {
	Width  int
	Height int
	Pix    []byte // RGBA, length 4*Width*Height (NV12 shape: nil)
	Y      []byte // NV12 luma, length Width*Height (RGBA shape: nil)
	UV     []byte // NV12 interleaved chroma, length Width*Height/2 (RGBA: nil)
	NV12   bool   // true when Y/UV carry the picture
	PTSMs  int64
	put    func([]byte)
	putY   func([]byte)
	putUV  func([]byte)
}

// Release returns the frame's pixels to the pool (no-op for fresh
// buffers). Call it exactly once when the picture is dropped before
// display; displayed frames transfer ownership to the display pool.
func (f *VideoFrame) Release() {
	if f == nil {
		return
	}
	put, pix := f.put, f.Pix
	putY, y := f.putY, f.Y
	putUV, uv := f.putUV, f.UV
	f.put, f.Pix = nil, nil
	f.putY, f.putUV, f.Y, f.UV = nil, nil, nil, nil
	if put != nil && len(pix) > 0 {
		put(pix)
	}
	if putY != nil && len(y) > 0 {
		putY(y)
	}
	if putUV != nil && len(uv) > 0 {
		putUV(uv)
	}
}

// Decoder owns one ffmpeg open: format, codec, packet, frame and the
// RGBA converter. Use Open, then Next until ErrEOF, then Close.
// It is not safe for concurrent Next calls; the player calls it from
// its single background thread.
type Decoder struct {
	info     StreamInfo
	fmtCtx   unsafe.Pointer
	codecCtx unsafe.Pointer
	vidIdx   int32
	tbNum    int32
	tbDen    int32
	pkt      unsafe.Pointer
	frame    unsafe.Pointer
	sws      unsafe.Pointer
	swsW     int32
	swsH     int32
	swsFmt   int32
	// nv12 selects NV12 plane output in Next (odd sizes still fall
	// back to RGBA inside, since NV12 needs even width/height).
	nv12     bool
	draining bool
	drained  bool
	shown    int64
	lastMs   int64
	closed   bool
	// pixGet/pixPut borrow and return RGBA buffers (player path wires
	// these to the display pool so steady play reuses instead of
	// allocating ~w*h*4 per frame; nil keeps fresh buffers).
	pixGet func(int) []byte
	pixPut func([]byte)
	// yGet/uvGet/putY/putUV borrow and return NV12 planes (SetNV12
	// path): Y is w*h luma bytes, UV is w*h/2 interleaved chroma.
	// Nil keeps fresh buffers. RGBA hooks stay untouched for fallback.
	yGet  func(int) []byte
	uvGet func(int) []byte
	putY  func([]byte)
	putUV func([]byte)
	// P1 硬解记账：设备引用交解码器拥有（随释放，Close 不碰），
	// 跳板句柄随本结构保活，swFrame 是复用的回传目标。
	hw       hwState
	hwPixFmt int32
	hwGetFmt uintptr
	swFrame  unsafe.Pointer
	// hwSettled 标记首帧协商是否落定（硬解帧流过 / 谈崩转软解只记一次）。
	hwSettled bool
}

// SetPixPool wires pooled RGBA reuse: get borrows a size-byte buffer,
// put returns it. Nil clears back to fresh buffers (nil 接收器直接回, 不崩).
func (d *Decoder) SetPixPool(get func(int) []byte, put func([]byte)) {
	if d == nil {
		return
	}
	d.pixGet = get
	d.pixPut = put
}

// SetPlanePool wires pooled NV12 plane reuse: yGet borrows a w*h luma
// buffer, uvGet a w*h/2 chroma buffer. Nil clears back to fresh buffers.
func (d *Decoder) SetPlanePool(yGet, uvGet func(int) []byte, putY, putUV func([]byte)) {
	if d == nil {
		return
	}
	d.yGet, d.uvGet, d.putY, d.putUV = yGet, uvGet, putY, putUV
}

// SetNV12 selects NV12 plane output in Next (odd sizes still fall back
// to RGBA inside, since NV12 needs even width/height).
func (d *Decoder) SetNV12(on bool) {
	if d == nil {
		return
	}
	d.nv12 = on
}

// RawFormatCtx exposes the underlying AVFormatContext* for demux-layer
// probes (FormatContext wrapper). The Decoder keeps ownership; do not
// close or free it, it dies with Close.
func (d *Decoder) RawFormatCtx() unsafe.Pointer {
	if d == nil {
		return nil
	}
	return d.fmtCtx
}

// CodecID reports the video codec id gathered at open (如 27=H264).
func (d *Decoder) CodecID() int32 {
	if d == nil {
		return CodecIDNone
	}
	return d.info.CodecID
}

// IsOpen reports the decoder is opened (!) and not closed.
func (d *Decoder) IsOpen() bool {
	if d == nil || d.closed || d.codecCtx == nil {
		return false
	}
	return true
}

// CodecCtx exposes the opened AVCodecContext* for codec-layer probes
// (CodecContext wrapper). The Decoder keeps ownership; do not close or
// free it, it dies with Close.
func (d *Decoder) CodecCtx() *CodecContext {
	if d == nil {
		return nil
	}
	return &CodecContext{ptr: d.codecCtx}
}

// HWStats 快照硬解水位（任意线程可读；只加键，不改旧语义）。
func (d *Decoder) HWStats() HWStats {
	if d == nil {
		return HWStats{Name: "soft"}
	}
	return d.hw.stats()
}

// Info returns the stream facts gathered at open.
func (d *Decoder) Info() StreamInfo { return d.info }

// openCodecCtx opens the decoder with threaded decode enabled
// (threads=0 means auto = CPU count, the ffmpeg/ffplay standard).
// A dict failure never fails the open: it falls back to single-thread.
func openCodecCtx(cc, decPtr unsafe.Pointer) int32 {
	d := NewDictionary()
	if d != nil {
		if err := d.Set("threads", "0", 0); err == nil {
			raw := d.Take()
			dp := raw
			ret := fOpen2(cc, decPtr, unsafe.Pointer(&dp))
			if dp != nil {
				fDictFree(&dp)
			}
			return ret
		}
		d.Free()
	}
	return fOpen2(cc, decPtr, nil)
}

// Open opens a local file path or URL with ffmpeg's own demuxer.
// The build enables file, http, https, tcp, udp, tls, rtmp and friends,
// so network URLs work without Go-side fetching.
func Open(path string) (*Decoder, error) {
	if err := ensureModDecode(); err != nil {
		return nil, err
	}
	if path == "" {
		return nil, fmt.Errorf("ffmpeg: empty path")
	}
	fNetInit()
	var fmtCtx unsafe.Pointer
	if ret := fOpenInput(&fmtCtx, path, nil, nil); ret < 0 {
		return nil, fmt.Errorf("ffmpeg: open %s: %s", path, errText(ret))
	}
	ok := false
	defer func() {
		if !ok {
			fCloseInput(&fmtCtx)
		}
	}()
	if ret := fFindInfo(fmtCtx, nil); ret < 0 {
		return nil, fmt.Errorf("ffmpeg: probe %s: %s", path, errText(ret))
	}
	var decPtr unsafe.Pointer
	vid := fBestStream(fmtCtx, MediaTypeVideo, -1, -1, &decPtr, 0)
	if vid < 0 {
		return nil, fmt.Errorf("ffmpeg: no video in %s: %s", path, errText(vid))
	}
	sPtr := streamPointer(fmtCtx, vid)
	par := loadPtr(sPtr, streamCodecPar)
	if par == nil {
		return nil, fmt.Errorf("ffmpeg: no codec params %s", path)
	}
	codecID := loadInt32(par, parCodecID)
	if decPtr == nil {
		decPtr = fFindDecoder(codecID)
	}
	if decPtr == nil {
		return nil, fmt.Errorf("ffmpeg: no decoder for codec %d (%s)", codecID, path)
	}
	cc := fAllocCtx(decPtr)
	if cc == nil {
		return nil, fmt.Errorf("ffmpeg: no codec context %s", path)
	}
	if ret := fParToCtx(cc, par); ret < 0 {
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: params %s: %s", path, errText(ret))
	}
	// P1 硬解：先谈设备再开解码器。谈崩不算打开失败，转软解继续播；
	// 只有软解也起不来才报坏片。引用交接：给解码器的那笔引用归它，
	// 我们这笔开完就放，不留悬空指针（开崩那次存进去的引用随释放走，
	// 极端下漏一笔引用、不崩）。
	hwName := "soft"
	var hwPix int32 = -1
	var hwCb uintptr
	var hwFallbacks int64
	if herr := ensureModHW(); herr == nil && !HWDisabled() {
		if dev, name, typ, derr := openHWDevice(); derr == nil {
			// 先验布局再碰内存：跨系统/跨架构头文件挪位就老实走软解，
			// 不写坏内存。布局不对也记一笔回落，不断播。
			if !ccLayoutOK(cc, codecID) {
				fBufUnref(&dev)
				hwFallbacks = 1
			} else if pix := hwPixFmtFor(decPtr, typ); pix >= 0 {
				refHWDevice(cc, dev)
				fBufUnref(&dev)
				hwCb = installGetFormat(cc, pix)
				hwName, hwPix = name, pix
			} else {
				fBufUnref(&dev)
				hwFallbacks = 1
			}
		} else {
			hwFallbacks = 1
		}
	}
	if ret := openCodecCtx(cc, decPtr); ret < 0 {
		if hwPix >= 0 {
			// 硬解开崩了： free 掉重开软解，多记一笔回落。
			hwFallbacks++
			hwName, hwPix, hwCb = "soft", -1, 0
			fFreeCtx(&cc)
			cc = fAllocCtx(decPtr)
			if cc == nil {
				return nil, fmt.Errorf("ffmpeg: no codec context %s", path)
			}
			if ret := fParToCtx(cc, par); ret < 0 {
				fFreeCtx(&cc)
				return nil, fmt.Errorf("ffmpeg: params %s: %s", path, errText(ret))
			}
			if ret := openCodecCtx(cc, decPtr); ret < 0 {
				fFreeCtx(&cc)
				return nil, fmt.Errorf("ffmpeg: open decoder %s: %s", path, errText(ret))
			}
		} else {
			fFreeCtx(&cc)
			return nil, fmt.Errorf("ffmpeg: open decoder %s: %s", path, errText(ret))
		}
	}
	pkt := fPacketAlloc()
	fr := fFrameAlloc()
	if pkt == nil || fr == nil {
		if pkt != nil {
			fPacketFree(&pkt)
		}
		if fr != nil {
			fFrameFree(&fr)
		}
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: no packet/frame %s", path)
	}
	tbNum := loadInt32(sPtr, streamTBNum)
	tbDen := loadInt32(sPtr, streamTBDen)
	if tbNum <= 0 || tbDen <= 0 {
		tbNum, tbDen = 1, 1000
	}
	fpsNum := loadInt32(sPtr, streamFPSNum)
	fpsDen := loadInt32(sPtr, streamFPSDen)
	fps := 0.0
	if fpsNum > 0 && fpsDen > 0 {
		fps = float64(fpsNum) / float64(fpsDen)
	}
	if fps <= 0 {
		fps = 30
	}
	w := loadInt32(par, parWidth)
	h := loadInt32(par, parHeight)
	if w <= 0 || h <= 0 {
		fPacketFree(&pkt)
		fFrameFree(&fr)
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: bad size %dx%d %s", w, h, path)
	}
	durMs := int64(0)
	if sd := loadInt64(sPtr, streamDuration); sd != NoPTS {
		durMs = sd * int64(tbNum) * 1000 / int64(tbDen)
	} else if fd := loadInt64(fmtCtx, fmtDuration); fd != NoPTS {
		durMs = fd / 1000
	}
	if durMs < 0 {
		durMs = 0
	}
	frames := loadInt64(sPtr, streamNbFrames)
	d := &Decoder{
		info: StreamInfo{
			Path: path, Width: int(w), Height: int(h),
			FPS: fps, DurMs: durMs, Frames: frames,
			CodecID: codecID, SrcPixFmt: loadInt32(par, parFormat),
		},
		fmtCtx: fmtCtx, codecCtx: cc, vidIdx: vid,
		tbNum: tbNum, tbDen: tbDen,
		pkt: pkt, frame: fr,
		lastMs:   -1,
		hwPixFmt: hwPix, hwGetFmt: hwCb,
	}
	d.hw.name.Store(hwName)
	d.hw.fallbacks.Store(hwFallbacks)
	ok = true
	return d, nil
}

// ptsToMs converts a stream-timestamp to milliseconds.
func (d *Decoder) ptsToMs(pts int64) int64 {
	if pts == NoPTS {
		if d.info.FPS > 0 {
			return d.shown * 1000 / int64(d.info.FPS)
		}
		if d.lastMs >= 0 {
			return d.lastMs + 40
		}
		return 0
	}
	return pts * int64(d.tbNum) * 1000 / int64(d.tbDen)
}

// nv12Bytes reports Y and UV plane sizes for w×h.
func nv12Bytes(w, h int) (yN, uvN int) { return w * h, w * h / 2 }

// borrowPlanes borrows pooled NV12 planes (fresh buffers when no plane
// pool is wired). ok=false returns what it borrowed, so callers never
// leak a half-built pair.
func (d *Decoder) borrowPlanes(yN, uvN int) (y, uv []byte, ok bool) {
	if d.yGet != nil {
		y = d.yGet(yN)
		if len(y) != yN {
			if d.putY != nil && len(y) > 0 {
				d.putY(y)
			}
			return nil, nil, false
		}
	} else {
		y = make([]byte, yN)
	}
	if d.uvGet != nil {
		uv = d.uvGet(uvN)
		if len(uv) != uvN {
			if d.putUV != nil && len(uv) > 0 {
				d.putUV(uv)
			}
			if d.putY != nil {
				d.putY(y)
			}
			return nil, nil, false
		}
	} else {
		uv = make([]byte, uvN)
	}
	return y, uv, true
}

// giveBackPlanes returns a half-built pair (wrong pool sizes, shape
// surprise): wrong sizes drop and count, never pollute the pool.
func (d *Decoder) giveBackPlanes(y, uv []byte) {
	if d.putY != nil && len(y) > 0 {
		d.putY(y)
	}
	if d.putUV != nil && len(uv) > 0 {
		d.putUV(uv)
	}
}

// convertNV12 rearranges one decoded soft AVFrame into pooled NV12
// planes: NV12 input is a direct copy, YUV420P takes one cheap
// interleave pass (no arithmetic). Hard frames never reach here:
// frameToVideo diverts them to swFrameNV12From (direct plane copy off
// the transfer target). Odd sizes and exotic sources fall back to
// RGBA. convertNV12 unrefs the frame exactly once on every path,
// like convertFrame.
func (d *Decoder) convertNV12(ms int64) (*VideoFrame, error) {
	if err := ensureModDecode(); err != nil {
		return nil, err
	}
	w := int(loadInt32(d.frame, frameWidth))
	h := int(loadInt32(d.frame, frameHeight))
	srcFmt := loadInt32(d.frame, frameFormat)
	if w < 1 || h < 1 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("ffmpeg: bad frame %dx%d fmt %d", w, h, srcFmt)
	}
	if w%2 != 0 || h%2 != 0 || w*h/2 <= 0 {
		return d.convertFrame(ms)
	}
	if srcFmt != PixFmtNV12 && srcFmt != PixFmtYUV420P {
		return d.convertFrame(ms)
	}
	yN, uvN := nv12Bytes(w, h)
	y, uv, ok := d.borrowPlanes(yN, uvN)
	if !ok {
		return nil, fmt.Errorf("ffmpeg: pool gave wrong NV12 sizes, want %d/%d", yN, uvN)
	}
	if copyNV12Frame(d.frame, y, uv, w, h, srcFmt) {
		fFrameUnref(d.frame)
		return &VideoFrame{Width: w, Height: h, Y: y, UV: uv, NV12: true, PTSMs: ms, putY: d.putY, putUV: d.putUV}, nil
	}
	if interleavePlanar(d.frame, y, uv, w, h, srcFmt) {
		fr := &VideoFrame{Width: w, Height: h, Y: y, UV: uv, NV12: true, PTSMs: ms, putY: d.putY, putUV: d.putUV}
		fFrameUnref(d.frame)
		return fr, nil
	}
	d.giveBackPlanes(y, uv)
	return nil, fmt.Errorf("ffmpeg: unsupported src fmt %d for NV12", srcFmt)
}

// copyNV12Frame copies AVFrame planes straight out when the source is
// already NV12 in memory: row copies only, no arithmetic. It reports
// false for any other pixel format.
func copyNV12Frame(fr unsafe.Pointer, y, uv []byte, w, h int, srcFmt int32) bool {
	if srcFmt != PixFmtNV12 {
		return false
	}
	var srcPtrs [8]unsafe.Pointer
	var srcStrides [8]int32
	for i := 0; i < 2; i++ {
		srcPtrs[i] = loadPtr(fr, frameData+uintptr(i)*8)
		srcStrides[i] = loadInt32(fr, frameLinesize+uintptr(i)*4)
		if srcPtrs[i] == nil || srcStrides[i] <= 0 {
			return false
		}
	}
	yp := unsafe.Pointer(&y[0])
	uvp := unsafe.Pointer(&uv[0])
	copyPlane(yp, srcPtrs[0], w, h, int(srcStrides[0]), w)
	copyPlane(uvp, srcPtrs[1], w, h/2, int(srcStrides[1]), w)
	return true
}

// copyPlane copies h rows of rowLen bytes honoring a source stride.
func copyPlane(dst, src unsafe.Pointer, rowLen, h, srcStride, _ int) {
	for y := 0; y < h; y++ {
		d := unsafe.Slice((*byte)(unsafe.Add(dst, uintptr(y*rowLen))), rowLen)
		s := unsafe.Slice((*byte)(unsafe.Add(src, uintptr(y*srcStride))), rowLen)
		copy(d, s)
	}
}

// interleavePlanar rearranges planar YUV420P into NV12 (Y direct row
// copy, U/V interleaved into UV): one cheap pass, no arithmetic.
// It reports false for non-YUV420P sources (caller falls back to RGBA).
func interleavePlanar(fr unsafe.Pointer, y, uv []byte, w, h int, srcFmt int32) bool {
	if srcFmt != PixFmtYUV420P {
		return false
	}
	var srcPtrs [8]unsafe.Pointer
	var srcStrides [8]int32
	for i := 0; i < 3; i++ {
		srcPtrs[i] = loadPtr(fr, frameData+uintptr(i)*8)
		srcStrides[i] = loadInt32(fr, frameLinesize+uintptr(i)*4)
		if srcPtrs[i] == nil || srcStrides[i] <= 0 {
			return false
		}
	}
	copyPlane(unsafe.Pointer(&y[0]), srcPtrs[0], w, h, int(srcStrides[0]), w)
	hw, hh := w/2, h/2
	for y := 0; y < hh; y++ {
		u := unsafe.Slice((*byte)(unsafe.Add(srcPtrs[1], uintptr(y*int(srcStrides[1])))), hw)
		v := unsafe.Slice((*byte)(unsafe.Add(srcPtrs[2], uintptr(y*int(srcStrides[2])))), hw)
		row := uv[y*w : y*w+w]
		for x := 0; x < hw; x++ {
			row[2*x], row[2*x+1] = u[x], v[x]
		}
	}
	return true
}

// ensureSws rebuilds the RGBA converter when the frame shape changes.
func (d *Decoder) ensureSws(w, h, srcFmt int32) error {
	if err := ensureModDecode(); err != nil {
		return err
	}
	if d.sws != nil && d.swsW == w && d.swsH == h && d.swsFmt == srcFmt {
		return nil
	}
	if d.sws != nil {
		fSwsFreeCtx(d.sws)
		d.sws = nil
	}
	sws := fSwsGetCtx(w, h, srcFmt, w, h, PixFmtRGBA, SWSBilinear, nil, nil, nil)
	if sws == nil {
		return fmt.Errorf("ffmpeg: no scaler %dx%d fmt %d", w, h, srcFmt)
	}
	d.sws, d.swsW, d.swsH, d.swsFmt = sws, w, h, srcFmt
	return nil
}

// convertFrame scales one decoded AVFrame into a pooled RGBA buffer when
// the Decoder runs with a PixPool (player path), else a fresh buffer.
func (d *Decoder) convertFrame(ms int64) (*VideoFrame, error) {
	return d.convertFrameFrom(d.frame, ms)
}

// convertFrameFrom scales the given AVFrame (d.frame 或回传后的 swFrame).
func (d *Decoder) convertFrameFrom(fr unsafe.Pointer, ms int64) (*VideoFrame, error) {
	if err := ensureModDecode(); err != nil {
		return nil, err
	}
	w := loadInt32(fr, frameWidth)
	h := loadInt32(fr, frameHeight)
	srcFmt := loadInt32(fr, frameFormat)
	if w <= 0 || h <= 0 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("ffmpeg: bad frame %dx%d fmt %d", w, h, srcFmt)
	}
	if err := d.ensureSws(w, h, srcFmt); err != nil {
		return nil, err
	}
	var srcPtrs [8]unsafe.Pointer
	var srcStrides [8]int32
	for i := 0; i < 8; i++ {
		srcPtrs[i] = loadPtr(fr, frameData+uintptr(i)*8)
		srcStrides[i] = loadInt32(fr, frameLinesize+uintptr(i)*4)
	}
	size := int(w) * int(h) * 4
	var pix []byte
	if d.pixGet != nil {
		pix = d.pixGet(size)
		if len(pix) != size {
			if d.pixPut != nil && len(pix) > 0 {
				d.pixPut(pix)
			}
			return nil, fmt.Errorf("ffmpeg: pool gave %d bytes, want %d", len(pix), size)
		}
	} else {
		pix = make([]byte, size)
	}
	var dstPtrs [8]unsafe.Pointer
	var dstStrides [8]int32
	dstPtrs[0] = unsafe.Pointer(&pix[0])
	dstStrides[0] = w * 4
	got := fSwsScale(d.sws, &srcPtrs[0], &srcStrides[0], 0, h, &dstPtrs[0], &dstStrides[0])
	if got <= 0 {
		if d.pixPut != nil && d.pixGet != nil {
			d.pixPut(pix)
		}
		return nil, fmt.Errorf("ffmpeg: scale got %d", got)
	}
	return &VideoFrame{Width: int(w), Height: int(h), Pix: pix, PTSMs: ms, put: d.pixPut}, nil
}

// transferHW 把硬解帧拷回内存（swFrame 复用，用后解引用不清掉）。
// 回传耗时由调用方计时记均值；失败回错，调用方记一笔回落并跳过该帧。
func (d *Decoder) transferHW() error {
	if err := ensureModHW(); err != nil {
		return err
	}
	if d.swFrame == nil {
		d.swFrame = fFrameAlloc()
		if d.swFrame == nil {
			return fmt.Errorf("ffmpeg: no sw frame")
		}
	} else {
		fFrameUnref(d.swFrame)
	}
	var hw HWDevice
	if err := hw.HwframeTransferData(d.swFrame, d.frame, 0); err != nil {
		return err
	}
	return nil
}

// frameToVideo 把刚收到的 d.frame 变成 RGBA：硬解帧先回传再转，
// 软帧直转。首帧落定协商：流过硬解帧置 active，谈崩（软帧）记一笔转软解。
// 回传失败记一笔并跳过该帧（继续播，不崩）。
func (d *Decoder) frameToVideo(ms int64) (*VideoFrame, error) {
	if d.hwPixFmt >= 0 && loadInt32(d.frame, frameFormat) == d.hwPixFmt {
		t0 := time.Now()
		if err := d.transferHW(); err != nil {
			d.hw.fallbacks.Add(1)
			fFrameUnref(d.frame)
			return nil, err
		}
		d.hw.xferCount.Add(1)
		d.hw.xferNs.Add(uint64(time.Since(t0).Nanoseconds()))
		d.hw.active.Store(true)
		d.hwSettled = true
		fr, err := d.convertFrameFrom(d.swFrame, ms)
		fFrameUnref(d.frame)
		if d.nv12 && err == nil && fr != nil && !fr.NV12 {
			if nv := d.swFrameNV12From(d.swFrame, fr); nv != nil {
				return nv, nil
			}
			// Plane read failed (shape surprise): keep the RGBA
			// frame, never drop a picture for the plane path.
		}
		fFrameUnref(d.swFrame)
		return fr, err
	}
	if d.hwPixFmt >= 0 && !d.hwSettled {
		// 设备绑上了但解码器没出硬解帧：谈判没成，转软解继续，只记一次。
		d.hwSettled = true
		d.hw.active.Store(false)
		d.hw.fallbacks.Add(1)
	}
	// NV12 shape: soft frames take one cheap interleave pass here.
	// Hard frames never reach this branch: frameToVideo diverts them
	// to swFrameNV12 (direct plane copy off the transfer target).
	// Odd sizes and exotic sources fall back to RGBA. convertNV12
	// unrefs the frame exactly once on every path, like convertFrame.
	if d.nv12 {
		return d.convertNV12(ms)
	}
	fr, err := d.convertFrame(ms)
	fFrameUnref(d.frame)
	return fr, err
}

// swFrameNV12 reads the back-transferred swFrame straight into pooled
// NV12 planes (the transfer target is NV12 already; src is the transfer
// frame, not the consumed hw frame). The actual source format is read
// off the frame and must be NV12, else nil. It reports nil on any shape
// surprise (caller keeps the RGBA frame, never drops it).
// No extra scaler runs here, so the hard path never pays RGBA twice.
func (d *Decoder) swFrameNV12From(src unsafe.Pointer, fr *VideoFrame) *VideoFrame {
	if loadInt32(src, frameFormat) != PixFmtNV12 {
		return nil
	}
	yN, uvN := nv12Bytes(fr.Width, fr.Height)
	y, uv, ok := d.borrowPlanes(yN, uvN)
	if !ok {
		return nil
	}
	if !copyNV12Frame(src, y, uv, fr.Width, fr.Height, PixFmtNV12) {
		d.giveBackPlanes(y, uv)
		return nil
	}
	nv := &VideoFrame{Width: fr.Width, Height: fr.Height, Y: y, UV: uv, NV12: true, PTSMs: fr.PTSMs, putY: d.putY, putUV: d.putUV}
	fr.Release()
	return nv
}

// Next decodes the next displayable picture in presentation order.
// It returns ErrEOF after the drained tail. Packet filtering, send and
// receive follow the doc/examples/demux_decode shape: read one packet,
// send it, then drain every ready frame before reading again.
func (d *Decoder) Next() (*VideoFrame, error) {
	if err := ensureModDecode(); err != nil {
		return nil, err
	}
	if d.closed {
		return nil, fmt.Errorf("ffmpeg: decoder closed")
	}
	if d.drained {
		return nil, ErrEOF
	}
	for {
		if !d.draining {
			ret := fReadFrame(d.fmtCtx, d.pkt)
			if ret == AvErrorEOF {
				d.draining = true
				// Flush: NULL packet tells the decoder no more input.
				_ = fSendPacket(d.codecCtx, nil)
			} else if ret < 0 {
				return nil, fmt.Errorf("ffmpeg: read: %s", errText(ret))
			} else {
				if loadInt32(d.pkt, pktStreamIndex) != d.vidIdx {
					fPacketUnref(d.pkt)
					continue
				}
				ret := fSendPacket(d.codecCtx, d.pkt)
				fPacketUnref(d.pkt)
				if ret == AvErrorEAGAIN {
					// Decoder full: receive one frame below, then keep
					// looping to read again.
				} else if ret < 0 {
					// Corrupt packet: skip it, keep the stream playing.
					continue
				}
			}
		}
		ret := fRecvFrame(d.codecCtx, d.frame)
		if ret == AvErrorEAGAIN {
			if d.draining {
				d.drained = true
				return nil, ErrEOF
			}
			continue
		}
		if ret == AvErrorEOF {
			d.drained = true
			return nil, ErrEOF
		}
		if ret < 0 {
			if d.draining {
				d.drained = true
				return nil, ErrEOF
			}
			continue
		}
		pts := loadInt64(d.frame, frameBestEffort)
		if pts == NoPTS {
			pts = loadInt64(d.frame, framePTS)
		}
		if pts == NoPTS {
			// No container stamp at all (some seeks land on frames the
			// demuxer reports without timing): fall back to the frame
			// rate grid instead of inheriting a stale lastMs that can
			// catapult the stamp past the landing (backward seeks used
			// to show 2000 after seeking to 1000).
			if d.info.FPS > 0 {
				ms := d.shown * 1000 / int64(d.info.FPS)
				if d.lastMs >= 0 && ms <= d.lastMs {
					ms = d.lastMs + 1
				}
				fr, err := d.frameToVideo(ms)
				if err != nil {
					continue
				}
				d.shown++
				d.lastMs = ms
				return fr, nil
			}
		}
		ms := d.ptsToMs(pts)
		// Keep stamps monotonic: some tails repeat a stamp.
		if d.lastMs >= 0 && ms <= d.lastMs {
			if d.info.FPS > 0 {
				ms = d.lastMs + int64(1000/d.info.FPS)
				if ms <= d.lastMs {
					ms = d.lastMs + 1
				}
			} else {
				ms = d.lastMs + 1
			}
		}
		fr, err := d.frameToVideo(ms)
		if err != nil {
			continue
		}
		d.shown++
		d.lastMs = ms
		return fr, nil
	}
}

// SeekTo jumps to the keyframe at or before targetMs and flushes the
// decoder, so the next Next call decodes forward from the landing.
// It mirrors the player contract: millisecond in, landing stamp out.
func (d *Decoder) SeekTo(targetMs int64) (int64, error) {
	if err := ensureModDecode(); err != nil {
		return 0, err
	}
	if d.closed {
		return 0, fmt.Errorf("ffmpeg: decoder closed")
	}
	ts := targetMs * int64(d.tbDen) / (1000 * int64(d.tbNum))
	if ts < 0 {
		ts = 0
	}
	if ret := fSeekFrame(d.fmtCtx, d.vidIdx, ts, SeekBackward); ret < 0 {
		return 0, fmt.Errorf("ffmpeg: seek %dms: %s", targetMs, errText(ret))
	}
	fFlushBuf(d.codecCtx)
	d.draining, d.drained = false, false
	// 硬解上下文不清零复用：Flush 只清解码器状态，设备与协商不动；
	// swFrame 解引用一次，防跳后首帧复用 stale 数据。
	if d.swFrame != nil {
		fFrameUnref(d.swFrame)
	}
	// Reset the stamp guard: a backward seek legitimately replays
	// earlier stamps, and the monotonic fixup in Next must not push
	// them forward (it only guards the forward tail within one play
	// segment). shown restarts too: the next picture is the new
	// segment's head (segment-relative numbering for the NoPTS
	// fallback path).
	d.lastMs = -1
	d.shown = 0
	landed := ts * int64(d.tbNum) * 1000 / int64(d.tbDen)
	return landed, nil
}

// Close frees every ffmpeg object owned by this open.
// 释放顺序：解码器（含它拥有的硬解设备引用）、回传帧、包、帧容器、
// 格式上下文。跳板句柄随结构丢弃（进程级上限内可忽略）。
func (d *Decoder) Close() {
	mustUse(ensureModDecode())
	if d.closed {
		return
	}
	d.closed = true
	if d.sws != nil {
		fSwsFreeCtx(d.sws)
		d.sws = nil
	}
	if d.swFrame != nil {
		fFrameFree(&d.swFrame)
	}
	d.hwGetFmt = 0
	if d.pkt != nil {
		fPacketFree(&d.pkt)
	}
	if d.frame != nil {
		fFrameFree(&d.frame)
	}
	if d.codecCtx != nil {
		fFreeCtx(&d.codecCtx)
	}
	if d.fmtCtx != nil {
		fCloseInput(&d.fmtCtx)
	}
}

// ensureModDecode 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modDecodeOnce sync.Once

func ensureModDecode() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	if err := ensureModPacket(); err != nil {
		return err
	}
	if err := ensureModFrame(); err != nil {
		return err
	}
	if err := ensureModScale(); err != nil {
		return err
	}
	modDecodeOnce.Do(func() {})
	return nil
}
