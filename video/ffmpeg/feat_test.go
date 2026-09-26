package ffmpeg

// 五个视频功能演示测试：每个功能产一个文件，全放 ../testdata/ 里，方便直接打开看。
//
// 大白话：拆段、字幕、提音频、缩放转码、烧字，一个功能一个测试，
// 产五个小文件。片源用仓里现成小片，输出也都是小文件。
// 写文件要高级版库，没有就 Skip；缺字体（烧字用）也 Skip，不硬撑。

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// featGate 通用门：库在且是高级版（能写文件）才跑。
func featGate(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	if !IsFull() {
		t.Skipf("need full lib (write/encode), current=%s variant=%s", LibPath(), Variant())
	}
}

// featProbe 轻验：文件能重开、时长大于 0、能解出第一帧。
func featProbe(t *testing.T, path string) {
	t.Helper()
	d, err := Open(path)
	if err != nil {
		t.Fatalf("reopen %s: %v", path, err)
	}
	defer d.Close()
	if d.Info().DurMs <= 0 {
		t.Fatalf("%s bad duration", path)
	}
	fr, err := d.Next()
	if err != nil {
		t.Fatalf("%s decode: %v", path, err)
	}
	if fr.Width <= 0 || fr.Height <= 0 {
		t.Fatalf("%s bad size %dx%d", path, fr.Width, fr.Height)
	}
	fr.Release()
}

// featSubProbe 数盒子里字幕轨条数。
func featSubProbe(t *testing.T, path string) int {
	t.Helper()
	var inPtr unsafe.Pointer
	if ret := fOpenInput(&inPtr, path, nil, nil); ret < 0 {
		t.Fatalf("probe open: %s", errText(ret))
	}
	defer fCloseInput(&inPtr)
	inFc := &FormatContext{ptr: inPtr}
	n := 0
	for i := 0; i < inFc.NbStreams(); i++ {
		st := inFc.StreamAt(i)
		if st == nil {
			continue
		}
		if par := (&CodecParameters{ptr: st.CodecPar()}); par.ptr != nil {
			if loadInt32(par.ptr, parCodecType) == MediaTypeSubtitle {
				n++
			}
		}
	}
	return n
}

// TestFeatSplitRanges 多区间拆段：同一原片切头尾两段，各存一个 mp4。
// 原片用仓里 22M 的 vr_oceans（已有），每段只取 1 秒，输出几百 KB。
func TestFeatSplitRanges(t *testing.T) {
	featGate(t)
	src := "../testdata/vr_oceans.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	bounds := [][2]int64{{0, 1000}, {1000, 2000}}
	names := []string{"feat_split_head.mp4", "feat_split_mid.mp4"}
	for i, b := range bounds {
		out := filepath.Join("../testdata", names[i])
		if err := RemuxSegment(src, b[0], b[1], out, fmt.Sprintf("split seg %d", i)); err != nil {
			t.Fatalf("seg %d: %v", i, err)
		}
		featProbe(t, out)
	}
}

// TestFeatSubtitles 多字幕版本：同一段挂不同字幕，各存一个 mp4。
// 中英文各一句，验证 mov_text 字幕轨写进去且能认出来。
func TestFeatSubtitles(t *testing.T) {
	featGate(t)
	src := "../testdata/vr_oceans.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	subs := []string{"第一段测试字幕", "hello feature test"}
	names := []string{"feat_sub_cn.mp4", "feat_sub_en.mp4"}
	for i, text := range subs {
		out := filepath.Join("../testdata", names[i])
		if err := RemuxSegment(src, 0, 1000, out, text); err != nil {
			t.Fatalf("sub %d: %v", i, err)
		}
		if n := featSubProbe(t, out); n < 1 {
			t.Fatalf("%s no subtitle track", out)
		}
		featProbe(t, out)
	}
}

// TestFeatAudioExtract 音频单独提取：取原片头 1 秒声音，存成 wav。
// 大白话：音频接口吐的是 48k 双声道浮点，转成 16 位整型按 wav 格式落盘，
// 播放器都能直接放。
func TestFeatAudioExtract(t *testing.T) {
	featGate(t)
	src := "../testdata/vr_oceans.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	a, err := OpenAudio(src)
	if err != nil {
		t.Fatalf("open audio: %v", err)
	}
	defer a.Close()
	var pcm []int16
	for {
		fr, err := a.Next()
		if err != nil {
			break // 读完或到尾都停，有多少算多少
		}
		if fr.PTSMs >= 1000 {
			break
		}
		for _, s := range fr.Data {
			if s > 1 {
				s = 1
			}
			if s < -1 {
				s = -1
			}
			pcm = append(pcm, int16(s*32767))
		}
	}
	if len(pcm) == 0 {
		t.Fatalf("no audio samples")
	}
	out := filepath.Join("../testdata", "feat_audio.wav")
	if err := featWriteWAV(out, pcm, OutRate, OutChannels); err != nil {
		t.Fatalf("write wav: %v", err)
	}
	// 回验：RIFF 头在，长度对得上。
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat wav: %v", err)
	}
	if want := int64(44 + len(pcm)*2); fi.Size() != want {
		t.Fatalf("wav size %d want %d", fi.Size(), want)
	}
}

// featWriteWAV 写 16 位 PCM wav（小端，双声道）。
func featWriteWAV(path string, pcm []int16, rate, ch int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var h [44]byte
	copy(h[0:], "RIFF")
	binary.LittleEndian.PutUint32(h[4:], uint32(36+len(pcm)*2))
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1)
	binary.LittleEndian.PutUint16(h[22:], uint16(ch))
	binary.LittleEndian.PutUint32(h[24:], uint32(rate))
	binary.LittleEndian.PutUint32(h[28:], uint32(rate*ch*2))
	binary.LittleEndian.PutUint16(h[32:], uint16(ch*2))
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], uint32(len(pcm)*2))
	if _, err := f.Write(h[:]); err != nil {
		return err
	}
	raw := unsafe.Slice((*byte)(unsafe.Pointer(&pcm[0])), len(pcm)*2)
	_, err = f.Write(raw)
	return err
}

// featRawVideo 开原片视频解码（裸包路子）：返回输入盒、视频流序号、时基、
// 解码器，以及流参数里的宽高和像素格式（直接读，不走选项查询）。
func featRawVideo(t *testing.T, src string) (inPtr unsafe.Pointer, vid int32, tb AVRational, dec *CodecContext, w, h, pixFmt int32) {
	t.Helper()
	fNetInit()
	if ret := fOpenInput(&inPtr, src, nil, nil); ret < 0 {
		t.Fatalf("open %s: %s", src, errText(ret))
	}
	if ret := fFindInfo(inPtr, nil); ret < 0 {
		fCloseInput(&inPtr)
		t.Fatalf("probe %s: %s", src, errText(ret))
	}
	inFc := &FormatContext{ptr: inPtr}
	vid = fBestStream(inPtr, MediaTypeVideo, -1, -1, nil, 0)
	if vid < 0 {
		fCloseInput(&inPtr)
		t.Fatalf("%s no video stream", src)
	}
	st := inFc.StreamAt(int(vid))
	tb = st.TimeBase()
	inPar := &CodecParameters{ptr: st.CodecPar()}
	w = loadInt32(inPar.ptr, parWidth)
	h = loadInt32(inPar.ptr, parHeight)
	pixFmt = loadInt32(inPar.ptr, parFormat)
	if w <= 0 || h <= 0 {
		fCloseInput(&inPtr)
		t.Fatalf("%s bad size %dx%d", src, w, h)
	}
	codecID := loadInt32(inPar.ptr, parCodecID)
	decCtx := FindDecoder(codecID)
	if decCtx == nil {
		fCloseInput(&inPtr)
		t.Fatalf("decoder %d missing", codecID)
	}
	dec = decCtx.AllocContext()
	if err := inPar.ToContext(dec); err != nil {
		fCloseInput(&inPtr)
		t.Fatalf("to ctx: %v", err)
	}
	if err := dec.Open(decCtx, nil); err != nil {
		fCloseInput(&inPtr)
		t.Fatalf("open decoder: %v", err)
	}
	return inPtr, vid, tb, dec, w, h, pixFmt
}

// featMuxEncoder 给编码器配输出盒：mp4 盒 + 一条视频流，参数从编码器拷。
func featMuxEncoder(t *testing.T, dst string, enc *CodecContext) (outPtr unsafe.Pointer, streamIdx int32, streamTb AVRational, done func(ok bool)) {
	t.Helper()
	if ret := fAvformatAllocOutputContext2(&outPtr, nil, "mp4", nil); ret < 0 {
		t.Fatalf("alloc mp4: %s", errText(ret))
	}
	done = func(ok bool) {
		if !ok {
			fAvformatFreeContext(outPtr)
		}
	}
	outFc := &FormatContext{ptr: outPtr}
	outStPtr := outFc.NewStream(nil)
	if outStPtr == nil {
		t.Fatalf("new stream")
	}
	outSt := &Stream{ptr: outStPtr}
	outPar := &CodecParameters{ptr: outSt.CodecPar()}
	if err := outPar.FromContext(enc); err != nil {
		t.Fatalf("par from ctx: %v", err)
	}
	var pb unsafe.Pointer
	var fc FormatContext
	if err := fc.Open(&pb, dst, AVIOFlagWrite); err != nil {
		t.Fatalf("open out %s: %v", dst, err)
	}
	outFc.SetPb(pb)
	outIO := &IOContext{ptr: pb}
	if err := outFc.WriteHeader(nil); err != nil {
		outIO.Close()
		t.Fatalf("write header: %v", err)
	}
	streamIdx = int32(outSt.Index())
	streamTb = outSt.TimeBase()
	_ = outIO
	return outPtr, streamIdx, streamTb, done
}

// TestFeatScaleTranscode 缩放转码：480p 原片解 5 帧，缩成 320x180，
// 用 mpeg4 重编码存成小 mp4。验证编码链路真能出片。
func TestFeatScaleTranscode(t *testing.T) {
	featGate(t)
	src := "../testdata/vr2_480p.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	dst := filepath.Join("../testdata", "feat_small.mp4")
	n := featScaleFile(t, src, dst, 320, 180, 5)
	if n <= 0 {
		t.Fatalf("wrote 0 packets")
	}
	featProbe(t, dst)
	d, err := Open(dst)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d.Close()
	if d.Info().Width != 320 || d.Info().Height != 180 {
		t.Fatalf("size %dx%d want 320x180", d.Info().Width, d.Info().Height)
	}
}

// featScaleFile 解码→scale 滤镜→mpeg4 编码→mp4 落盘，返回写包数。
func featScaleFile(t *testing.T, src, dst string, w, h, maxFrames int) int {
	t.Helper()
	inPtr, vid, tb, dec, srcW, srcH, pixFmt := featRawVideo(t, src)
	defer fCloseInput(&inPtr)
	defer dec.Close()
	fps := int32(30)
	bufArgs := fmt.Sprintf("video_size=%dx%d:pix_fmt=%d:time_base=%d/%d:frame_rate=%d/1:pixel_aspect=1/1",
		srcW, srcH, pixFmt, tb.Num, tb.Den, fps)
	_, fsrc, fsink, freeGraph, err := openFeatChain(bufArgs, [][2]string{{"scale", fmt.Sprintf("%d:%d", w, h)}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	defer freeGraph()
	srcF := &FilterSource{ptr: fsrc}
	sinkF := &FilterSink{ptr: fsink}
	enc, err := openFeatEncoder(CodecIDMPEG4, int32(w), int32(h), PixFmtYUV420P, tb, 400000)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	defer enc.Close()
	outPtr, streamIdx, streamTb, done := featMuxEncoder(t, dst, enc)
	ok := false
	defer func() { done(ok) }()
	var mux Muxer
	wrote := featPump(t, inPtr, vid, dec, srcF, sinkF, enc, &mux, outPtr, streamIdx, tb, streamTb, maxFrames)
	if ret := fAvWriteTrailer(outPtr); ret < 0 {
		t.Fatalf("trailer: %s", errText(ret))
	}
	fAvformatFreeContext(outPtr)
	ok = true
	return wrote
}

// featPump 解→滤→编→写主循环：读包送解码、收帧推滤镜、拉帧送编码、收包写盘。
// 时基全程同一套（解码流时基），帧序号直传，最多处理 maxFrames 个解码帧。
func featPump(t *testing.T, inPtr unsafe.Pointer, vid int32, dec *CodecContext, srcF *FilterSource, sinkF *FilterSink, enc *CodecContext, mux *Muxer, outPtr unsafe.Pointer, streamIdx int32, tb, streamTb AVRational, maxFrames int) int {
	t.Helper()
	pkt := NewPacket()
	if pkt == nil {
		t.Fatalf("no packet")
	}
	defer pkt.Free()
	frm := NewFrame()
	if frm == nil {
		t.Fatalf("no frame")
	}
	defer frm.Free()
	out := NewFrame()
	if out == nil {
		t.Fatalf("no out frame")
	}
	defer out.Free()
	epkt := NewPacket()
	if epkt == nil {
		t.Fatalf("no enc packet")
	}
	defer epkt.Free()
	wrote := 0
	gotFrames := 0
	// 拉编码包写盘小闭包。
	drainEnc := func() {
		for {
			epkt.Unref()
			if err := enc.ReceivePacket(epkt); err != nil {
				break // EAGAIN/EOF 都是排空信号
			}
			epkt.RescaleTs(tb, streamTb)
			epkt.SetStreamIndex(int(streamIdx))
			if err := mux.InterleavedWriteFrame(outPtr, epkt.ptr); err != nil {
				t.Fatalf("write pkt: %v", err)
			}
			wrote++
		}
	}
	// 拉滤镜帧送编码小闭包。
	drainSink := func() {
		for {
			out.Unref()
			if ret := sinkF.GetFrame(out.ptr); ret < 0 {
				break
			}
			if err := enc.SendFrame(out); err != nil {
				t.Fatalf("send frame: %v", err)
			}
			drainEnc()
		}
	}
	for gotFrames < maxFrames {
		pkt.Unref()
		if ret := fReadFrame(inPtr, pkt.ptr); ret < 0 {
			break
		}
		if int32(pkt.StreamIndex()) != vid {
			continue
		}
		if err := dec.SendPacket(pkt); err != nil {
			continue
		}
		for {
			frm.Unref()
			if err := dec.ReceiveFrame(frm); err != nil {
				break
			}
			gotFrames++
			if err := srcF.AddFrame(frm.ptr); err != nil {
				t.Fatalf("filter in: %v", err)
			}
			drainSink()
			if gotFrames >= maxFrames {
				break
			}
		}
	}
	// 收尾：编码器刷空。
	_ = enc.SendFrame(nil)
	drainEnc()
	return wrote
}

// TestFeatBurnSubtitle 画面烧字：解 5 帧，每帧右下角烧一行字，重编码存盘。
// 大白话：字幕轨的字能开关，烧进画面的字关不掉，抖音水印就是这个路子。
// 要系统字库，没有就 Skip。
func TestFeatBurnSubtitle(t *testing.T) {
	featGate(t)
	font := "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	if _, err := os.Stat(font); err != nil {
		t.Skipf("font missing: %s", font)
	}
	src := "../testdata/vr2_480p.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	dst := filepath.Join("../testdata", "feat_burn.mp4")
	n := featBurnFile(t, src, dst, font, "feat burn test", 5)
	if n <= 0 {
		t.Fatalf("wrote 0 packets")
	}
	featProbe(t, dst)
}

// featBurnFile 解码→drawtext 烧字→mpeg4 编码→mp4 落盘。
func featBurnFile(t *testing.T, src, dst, font, text string, maxFrames int) int {
	t.Helper()
	inPtr, vid, tb, dec, srcW, srcH, pixFmt := featRawVideo(t, src)
	defer fCloseInput(&inPtr)
	defer dec.Close()
	// drawtext 只要 YUV 就行；先转 yuv420p 再烧，省得编码器再挑格式。
	bufArgs := fmt.Sprintf("video_size=%dx%d:pix_fmt=%d:time_base=%d/%d:pixel_aspect=1/1",
		srcW, srcH, pixFmt, tb.Num, tb.Den)
	drawArgs := fmt.Sprintf("fontfile=%s:text='%s':fontsize=24:fontcolor=white:x=10:y=10", font, text)
	_, fsrc, fsink, freeGraph, err := openFeatChain(bufArgs, [][2]string{{"format", "yuv420p"}, {"drawtext", drawArgs}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	defer freeGraph()
	srcF := &FilterSource{ptr: fsrc}
	sinkF := &FilterSink{ptr: fsink}
	enc, err := openFeatEncoder(CodecIDMPEG4, srcW, srcH, PixFmtYUV420P, tb, 400000)
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	defer enc.Close()
	outPtr, streamIdx, streamTb, done := featMuxEncoder(t, dst, enc)
	ok := false
	defer func() { done(ok) }()
	var mux Muxer
	wrote := featPump(t, inPtr, vid, dec, srcF, sinkF, enc, &mux, outPtr, streamIdx, tb, streamTb, maxFrames)
	if ret := fAvWriteTrailer(outPtr); ret < 0 {
		t.Fatalf("trailer: %s", errText(ret))
	}
	fAvformatFreeContext(outPtr)
	ok = true
	return wrote
}
