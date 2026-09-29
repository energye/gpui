package ffmpeg

import (
	"errors"
	"fmt"
	"sync"
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

// VideoFrame is one decoded picture in packed RGBA, ready to wrap into
// the current library's frame type (width, height, pixels, stamp).
// Pix comes from the Decoder's PixPool when set (player path, pooled
// reuse, steady zero-alloc); otherwise it is a fresh buffer.
// Call Release exactly once when the picture is dropped before display;
// displayed frames transfer ownership to the display pool instead.
type VideoFrame struct {
	Width  int
	Height int
	Pix    []byte // RGBA, length 4*Width*Height
	PTSMs  int64
	put    func([]byte)
}

// Release returns the frame's pixels to the pool (no-op for fresh
// buffers). Call it exactly once when the picture is dropped before
// display; displayed frames transfer ownership to the display pool.
func (f *VideoFrame) Release() {
	if f == nil {
		return
	}
	put := f.put
	pix := f.Pix
	f.put = nil
	f.Pix = nil
	if put != nil && len(pix) > 0 {
		put(pix)
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
}

// SetPixPool wires pooled RGBA reuse: get borrows a size-byte buffer,
// put returns it. Nil clears back to fresh buffers.
func (d *Decoder) SetPixPool(get func(int) []byte, put func([]byte)) {
	d.pixGet = get
	d.pixPut = put
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

// Info returns the stream facts gathered at open.
func (d *Decoder) Info() StreamInfo { return d.info }

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
	if ret := fOpen2(cc, decPtr, nil); ret < 0 {
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: open decoder %s: %s", path, errText(ret))
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
		lastMs: -1,
	}
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
	if err := ensureModDecode(); err != nil {
		return nil, err
	}
	w := loadInt32(d.frame, frameWidth)
	h := loadInt32(d.frame, frameHeight)
	srcFmt := loadInt32(d.frame, frameFormat)
	if w <= 0 || h <= 0 || w > 8192 || h > 8192 {
		return nil, fmt.Errorf("ffmpeg: bad frame %dx%d fmt %d", w, h, srcFmt)
	}
	if err := d.ensureSws(w, h, srcFmt); err != nil {
		return nil, err
	}
	var srcPtrs [8]unsafe.Pointer
	var srcStrides [8]int32
	for i := 0; i < 8; i++ {
		srcPtrs[i] = loadPtr(d.frame, frameData+uintptr(i)*8)
		srcStrides[i] = loadInt32(d.frame, frameLinesize+uintptr(i)*4)
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
				fr, err := d.convertFrame(ms)
				fFrameUnref(d.frame)
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
		fr, err := d.convertFrame(ms)
		fFrameUnref(d.frame)
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
