package ffmpeg

import (
	"fmt"
	"math"
	"sync"
	"unsafe"
)

// AudioStream decodes one sound track: ffmpeg owns demux + decode, swr
// normalizes every frame to float interleaved stereo at 48000Hz.
//
// Say it plain: 打开一条音轨, 以后每帧声音都掰成一样的格式吐出来.
// Open with OpenAudio, pull with Next until ErrNoAudioDone, Close frees.
// Not safe for concurrent Next calls; the player drives it from its
// single background thread.
type AudioStream struct {
	info     AudioInfo2
	idx      int32
	tbNum    int32
	tbDen    int32
	fmtCtx   unsafe.Pointer
	codecCtx unsafe.Pointer
	pkt      unsafe.Pointer
	frame    unsafe.Pointer
	swr      unsafe.Pointer
	swrFmt   int32
	swrRate  int32
	swrCh    int32
	lastMs   int64
	draining bool
	drained  bool
	closed   bool
}

// AudioInfo2 mirrors StreamInfo for sound: identity facts for one track.
type AudioInfo2 struct {
	Path       string
	CodecID    int32
	SampleRate int
	Channels   int
	SrcFmt     int32
}

// AudioFrame2 is one resampled sound chunk: interleaved float32 PCM.
type AudioFrame2 struct {
	Data       []float32
	SampleRate int
	Channels   int
	Samples    int
	PTSMs      int64
}

// ErrNoAudioDone ends the sound stream (drained tail played out).
var ErrNoAudioDone = fmt.Errorf("ffmpeg: end of audio")

// errNoAudioTrack reports a clip without sound (open-time, no stream).
var errNoAudioTrack = fmt.Errorf("ffmpeg: no audio track")

// OutRate is the normalized output rate every frame converts to.
const OutRate = 48000

// OutChannels is the normalized channel count (stereo).
const OutChannels = 2

// Info returns the track facts gathered at open.
func (a *AudioStream) Info() AudioInfo2 { return a.info }

// OpenAudio opens the best audio stream in path and normalizes it to
// float interleaved stereo at 48000Hz. Silent clips report
// errNoAudioTrack (callers map it to video.ErrNoAudio).
func OpenAudio(path string) (*AudioStream, error) {
	if err := ensureModAudio(); err != nil {
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
	idx := fBestStream(fmtCtx, MediaTypeAudio, -1, -1, &decPtr, 0)
	if idx < 0 {
		return nil, fmt.Errorf("%w in %s", errNoAudioTrack, path)
	}
	sPtr := streamPointer(fmtCtx, idx)
	par := loadPtr(sPtr, streamCodecPar)
	if par == nil {
		return nil, fmt.Errorf("ffmpeg: no audio params %s", path)
	}
	codecID := loadInt32(par, parCodecID)
	if decPtr == nil {
		decPtr = fFindDecoder(codecID)
	}
	if decPtr == nil {
		return nil, fmt.Errorf("ffmpeg: no audio decoder for codec %d (%s)", codecID, path)
	}
	cc := fAllocCtx(decPtr)
	if cc == nil {
		return nil, fmt.Errorf("ffmpeg: no audio codec context %s", path)
	}
	if ret := fParToCtx(cc, par); ret < 0 {
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: audio params %s: %s", path, errText(ret))
	}
	if ret := fOpen2(cc, decPtr, nil); ret < 0 {
		fFreeCtx(&cc)
		return nil, fmt.Errorf("ffmpeg: open audio decoder %s: %s", path, errText(ret))
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
		return nil, fmt.Errorf("ffmpeg: no audio packet/frame %s", path)
	}
	tbNum := loadInt32(sPtr, streamTBNum)
	tbDen := loadInt32(sPtr, streamTBDen)
	if tbNum <= 0 || tbDen <= 0 {
		tbNum, tbDen = 1, 1000
	}
	a := &AudioStream{
		info: AudioInfo2{
			Path: path, CodecID: codecID,
			SampleRate: int(loadInt32(par, parSampleRate)),
			Channels:   int(loadInt32(par, parNbChannels)),
			SrcFmt:     loadInt32(par, parFormat),
		},
		idx: idx, tbNum: tbNum, tbDen: tbDen,
		fmtCtx: fmtCtx, codecCtx: cc, pkt: pkt, frame: fr,
		lastMs: -1,
	}
	ok = true
	return a, nil
}

// ptsToMs converts a stream-timestamp to milliseconds.
func (a *AudioStream) ptsToMs(pts int64) int64 {
	if pts == NoPTS {
		if a.lastMs >= 0 {
			return a.lastMs + 21
		}
		return 0
	}
	return pts * int64(a.tbNum) * 1000 / int64(a.tbDen)
}

// Next decodes the next sound chunk in presentation order (float
// interleaved stereo at 48000Hz). It returns ErrNoAudioDone after the
// drained tail.
func (a *AudioStream) Next() (*AudioFrame2, error) {
	if err := ensureModCore(); err != nil {
		var z1 *AudioFrame2
		return z1, err
	}
	if err := ensureModFrame(); err != nil {
		var z1 *AudioFrame2
		return z1, err
	}
	if err := ensureModPacket(); err != nil {
		var z1 *AudioFrame2
		return z1, err
	}
	if a.closed {
		return nil, fmt.Errorf("ffmpeg: audio closed")
	}
	if a.drained {
		return nil, ErrNoAudioDone
	}
	for {
		if !a.draining {
			ret := fReadFrame(a.fmtCtx, a.pkt)
			if ret == AvErrorEOF {
				a.draining = true
				_ = fSendPacket(a.codecCtx, nil)
			} else if ret < 0 {
				return nil, fmt.Errorf("ffmpeg: audio read: %s", errText(ret))
			} else {
				if loadInt32(a.pkt, pktStreamIndex) != a.idx {
					fPacketUnref(a.pkt)
					continue
				}
				ret := fSendPacket(a.codecCtx, a.pkt)
				fPacketUnref(a.pkt)
				if ret == AvErrorEAGAIN {
					// Decoder full: receive one frame below, then keep
					// looping to read again.
				} else if ret < 0 {
					// Corrupt packet: skip it, keep the sound playing.
					continue
				}
			}
		}
		ret := fRecvFrame(a.codecCtx, a.frame)
		if ret == AvErrorEAGAIN {
			if a.draining {
				a.drained = true
				return nil, ErrNoAudioDone
			}
			continue
		}
		if ret == AvErrorEOF {
			a.drained = true
			return nil, ErrNoAudioDone
		}
		if ret < 0 {
			if a.draining {
				a.drained = true
				return nil, ErrNoAudioDone
			}
			continue
		}
		pts := loadInt64(a.frame, frameBestEffort)
		if pts == NoPTS {
			pts = loadInt64(a.frame, framePTS)
		}
		ms := a.ptsToMs(pts)
		// Keep stamps monotonic: some tails repeat a stamp.
		if a.lastMs >= 0 && ms <= a.lastMs {
			ms = a.lastMs + 1
		}
		fr, err := a.convertFrame(ms)
		fFrameUnref(a.frame)
		if err != nil {
			continue
		}
		a.lastMs = ms
		return fr, nil
	}
}

// SeekTo jumps to the keyframe at or before targetMs and flushes the
// decoder, so the next Next call decodes forward from the landing.
func (a *AudioStream) SeekTo(targetMs int64) (int64, error) {
	if err := ensureModCore(); err != nil {
		var z1 int64
		return z1, err
	}
	if a.closed {
		return 0, fmt.Errorf("ffmpeg: audio closed")
	}
	ts := targetMs * int64(a.tbDen) / (1000 * int64(a.tbNum))
	if ts < 0 {
		ts = 0
	}
	if ret := fSeekFrame(a.fmtCtx, a.idx, ts, SeekBackward); ret < 0 {
		return 0, fmt.Errorf("ffmpeg: audio seek %dms: %s", targetMs, errText(ret))
	}
	fFlushBuf(a.codecCtx)
	a.draining, a.drained = false, false
	a.lastMs = -1
	landed := ts * int64(a.tbNum) * 1000 / int64(a.tbDen)
	return landed, nil
}

// Close frees every ffmpeg object owned by this open.
func (a *AudioStream) Close() {
	mustUse(ensureModCore())
	mustUse(ensureModFrame())
	mustUse(ensureModPacket())
	mustUse(ensureModResample())
	if a.closed {
		return
	}
	a.closed = true
	if a.swr != nil {
		fSwrFree(&a.swr)
	}
	if a.pkt != nil {
		fPacketFree(&a.pkt)
	}
	if a.frame != nil {
		fFrameFree(&a.frame)
	}
	if a.codecCtx != nil {
		fFreeCtx(&a.codecCtx)
	}
	if a.fmtCtx != nil {
		fCloseInput(&a.fmtCtx)
	}
}

// ensureSwr builds (or rebuilds on shape change) the resampler from
// the decoded frame's real format to float interleaved stereo at
// 48000Hz. Layouts ride av_channel_layout_default (mono/stereo/5.1/7.1
// by channel count); rates and formats ride av_opt_set_int/_sample_fmt
// exactly like the swresample.h doc example; then swr_init locks in.
func (a *AudioStream) ensureSwr(inFmt, inRate, inCh int32) error {
	mustUse(ensureModDictOpt())
	mustUse(ensureModResample())
	if a.swr != nil && a.swrFmt == inFmt && a.swrRate == inRate && a.swrCh == inCh {
		return nil
	}
	if a.swr != nil {
		fSwrFree(&a.swr)
		a.swr = nil
	}
	var rs Resampler
	swr := rs.Alloc2()
	if swr == nil {
		return fmt.Errorf("ffmpeg: no resampler")
	}
	ok := false
	defer func() {
		if !ok {
			fSwrFree(&swr)
		}
	}()
	var md MediaDesc
	var inLayout [32]byte
	var outLayout [32]byte
	md.ChannelLayoutDefault(unsafe.Pointer(&inLayout[0]), inCh)
	md.ChannelLayoutDefault(unsafe.Pointer(&outLayout[0]), OutChannels)
	o := OptObject{ptr: swr}
	if err := o.SetInt("in_sample_rate", int64(inRate), 0); err != nil {
		return fmt.Errorf("ffmpeg: swr in rate: %w", err)
	}
	if ret := fOptSetChlayout(swr, "in_chlayout", unsafe.Pointer(&inLayout[0]), 0); ret < 0 {
		return fmt.Errorf("ffmpeg: swr in layout: %s", errText(ret))
	}
	if ret := fOptSetSampleFm(swr, "in_sample_fmt", inFmt, 0); ret < 0 {
		return fmt.Errorf("ffmpeg: swr in fmt: %s", errText(ret))
	}
	if err := o.SetInt("out_sample_rate", OutRate, 0); err != nil {
		return fmt.Errorf("ffmpeg: swr out rate: %w", err)
	}
	if ret := fOptSetChlayout(swr, "out_chlayout", unsafe.Pointer(&outLayout[0]), 0); ret < 0 {
		return fmt.Errorf("ffmpeg: swr out layout: %s", errText(ret))
	}
	if ret := fOptSetSampleFm(swr, "out_sample_fmt", sampleFmtFloat, 0); ret < 0 {
		return fmt.Errorf("ffmpeg: swr out fmt: %s", errText(ret))
	}
	rs2 := Resampler{ptr: swr}
	if err := rs2.Init(); err != nil {
		return fmt.Errorf("ffmpeg: swr init: %w", err)
	}
	a.swr, a.swrFmt, a.swrRate, a.swrCh = swr, inFmt, inRate, inCh
	ok = true
	return nil
}

// convertFrame resamples one decoded AVFrame into float interleaved
// stereo at 48000Hz with stamp ms.
func (a *AudioStream) convertFrame(ms int64) (*AudioFrame2, error) {
	nb := loadInt32(a.frame, frameNbSamples)
	inFmt := loadInt32(a.frame, frameFormat)
	inRate := loadInt32(a.frame, frameSampleRate)
	if nb <= 0 || nb > 1<<16 {
		return nil, fmt.Errorf("ffmpeg: bad audio frame nb %d", nb)
	}
	if inRate <= 0 {
		inRate = int32(a.info.SampleRate)
	}
	if inRate <= 0 {
		return nil, fmt.Errorf("ffmpeg: bad audio rate %d", inRate)
	}
	inCh := loadInt32(a.frame, frameChLayout+4)
	if inCh <= 0 || inCh > 8 {
		inCh = int32(a.info.Channels)
	}
	if inCh <= 0 || inCh > 8 {
		return nil, fmt.Errorf("ffmpeg: bad audio channels %d", inCh)
	}
	if err := a.ensureSwr(inFmt, inRate, inCh); err != nil {
		return nil, err
	}
	var inPtrs [8]unsafe.Pointer
	for i := 0; i < 8; i++ {
		inPtrs[i] = loadPtr(a.frame, frameData+uintptr(i)*8)
	}
	// Headroom covers resample delay plus one input frame.
	rs := Resampler{ptr: a.swr}
	outCount := rs.GetOutSamples(nb) + 64
	if outCount <= 0 || outCount > 1<<18 {
		return nil, fmt.Errorf("ffmpeg: bad resample count %d", outCount)
	}
	raw := make([]byte, int(outCount)*OutChannels*4)
	var outPtrs [1]unsafe.Pointer
	outPtrs[0] = unsafe.Pointer(&raw[0])
	got, err := rs.ConvertCount(unsafe.Pointer(&outPtrs[0]), outCount, unsafe.Pointer(&inPtrs[0]), nb)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: resample: %w", err)
	}
	if got == 0 {
		return nil, fmt.Errorf("ffmpeg: resample ate frame")
	}
	data := make([]float32, int(got)*OutChannels)
	for i := range data {
		data[i] = float32FromLE(raw[i*4:])
	}
	return &AudioFrame2{
		Data: data, SampleRate: OutRate, Channels: OutChannels,
		Samples: int(got), PTSMs: ms,
	}, nil
}

// sampleFmtFloat is AV_SAMPLE_FMT_FLT (interleaved float32), walked
// from samplefmt.h: NONE=-1, U8=0, S16=1, S32=2, FLT=3.
const sampleFmtFloat = 3

// itoa formats a small int without importing strconv (keeps this file
// dependency-light like the rest of the package).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// float32FromLE reads one little-endian float32 (resampler output is
// native-endian float; amd64/arm64 hosts are little-endian).
func float32FromLE(b []byte) float32 {
	bits := uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
	return math.Float32frombits(bits)
}

// HasAudioTrack reports whether path carries a sound stream (probe
// only, no decode). False on any probe failure: callers treat that as
// silent, never as an error.
func HasAudioTrack(path string) bool {
	if path == "" || ensureModAudio() != nil {
		return false
	}
	fNetInit()
	var fmtCtx unsafe.Pointer
	if ret := fOpenInput(&fmtCtx, path, nil, nil); ret < 0 {
		return false
	}
	defer fCloseInput(&fmtCtx)
	if ret := fFindInfo(fmtCtx, nil); ret < 0 {
		return false
	}
	var decPtr unsafe.Pointer
	return fBestStream(fmtCtx, MediaTypeAudio, -1, -1, &decPtr, 0) >= 0
}

// ensureModAudio 开本模块的灯：先保核心房亮，再开依赖房，最后开自己这间。
// 大白话：用到这间房的功能才进来开灯（sync.Once，开过不再开）;
// 缺符号只在这间第一次用时报错，不连累别的功能。
var modAudioOnce sync.Once

func ensureModAudio() error {
	if err := ensureModCore(); err != nil {
		return err
	}
	if err := ensureModPacket(); err != nil {
		return err
	}
	if err := ensureModFrame(); err != nil {
		return err
	}
	if err := ensureModDictOpt(); err != nil {
		return err
	}
	if err := ensureModResample(); err != nil {
		return err
	}
	modAudioOnce.Do(func() {})
	return nil
}
