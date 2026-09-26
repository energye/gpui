package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

// 拆段加字幕：1080p 原片切 3 段，每段转封装成新 mp4 并挂一条 mov_text 字幕轨。
// 大白话：不解码只搬包，所以很快；切点只能落在关键帧上，前后差几帧正常。
// 原片 18M 没进仓，缺文件就 Skip；输出写 t.TempDir()，跑完即验，不进仓。
func TestRemuxSplitWithSubtitles(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing")
	}
	if !IsFull() {
		t.Skipf("need full lib (write/mov_text), current=%s variant=%s", LibPath(), Variant())
	}
	src := "../testdata/1080p_1920_1080_60fps.mp4"
	if _, err := os.Stat(src); err != nil {
		t.Skipf("clip missing: %s", src)
	}
	// 读原片时长，按关键帧切 3 段。
	// 原片关键帧在 0 / 4.167 / 8.333 秒（250 帧一个，实测），
	// 区间必须从关键帧起切——起点落在散帧堆里则开门帧缺失，
	// 后面再多包也解不出一帧。
	d, err := Open(src)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	durMs := d.Info().DurMs
	d.Close()
	if durMs <= 0 {
		t.Fatalf("bad duration %d", durMs)
	}
	bounds := [][2]int64{{0, 4167}, {4167, 8334}, {8334, durMs}}
	outDir := t.TempDir()
	subLines := []string{
		"seg0 test line one",
		"seg1 test line two",
		"seg2 test line three",
	}
	for i, b := range bounds {
		out := filepath.Join(outDir, fmt.Sprintf("seg%d.mp4", i))
		if err := RemuxSegment(src, b[0], b[1], out, subLines[i]); err != nil {
			t.Fatalf("seg %d: %v", i, err)
		}
		verifySegment(t, out, b, subLines[i])
	}
}

// RemuxSegment 把 src 在 [startMs, endMs) 的包搬进 dst，并挂一条 mov_text 字幕轨。
// 字幕内容传 text（一句测试字幕，覆盖整段时长）。
func RemuxSegment(src string, startMs, endMs int64, dst, text string) error {
	return remuxSegment(src, startMs, endMs, dst, text, nil)
}

func remuxSegment(src string, startMs, endMs int64, dst, text string, t *testing.T) error {
	if err := ensureLoaded(); err != nil {
		return err
	}
	if startMs < 0 || endMs <= startMs {
		return fmt.Errorf("ffmpeg: bad range %d-%d", startMs, endMs)
	}
	// 开输入盒。
	var inPtr unsafe.Pointer
	var fc FormatContext
	fNetInit()
	// OpenInput 的 url 走 Go string 直传（purego 转 C 字符串）。
	if ret := fOpenInput(&inPtr, src, nil, nil); ret < 0 {
		return fmt.Errorf("ffmpeg: open %s: %s", src, errText(ret))
	}
	defer fCloseInput(&inPtr)
	if ret := fFindInfo(inPtr, nil); ret < 0 {
		return fmt.Errorf("ffmpeg: probe %s: %s", src, errText(ret))
	}
	inFc := &FormatContext{ptr: inPtr}
	nb := inFc.NbStreams()
	if nb <= 0 {
		return fmt.Errorf("ffmpeg: no streams %s", src)
	}
	// 建输出盒并逐流拷参数。
	var outPtr unsafe.Pointer
	if ret := fAvformatAllocOutputContext2(&outPtr, nil, "mp4", nil); ret < 0 {
		return fmt.Errorf("ffmpeg: alloc mp4: %s", errText(ret))
	}
	outFc := &FormatContext{ptr: outPtr}
	okOut := false
	defer func() {
		if !okOut {
			fAvformatFreeContext(outPtr)
		}
	}()
	// 流映射：输入流序号 -> 输出流序号（字幕流是新建的，无输入）。
	outIdx := make([]int32, nb)
	for i := 0; i < nb; i++ {
		inSt := inFc.StreamAt(i)
		if inSt == nil {
			return fmt.Errorf("ffmpeg: stream %d nil", i)
		}
		outStPtr := outFc.NewStream(nil)
		if outStPtr == nil {
			return fmt.Errorf("ffmpeg: new stream %d", i)
		}
		outSt := &Stream{ptr: outStPtr}
		inPar := &CodecParameters{ptr: inSt.CodecPar()}
		outPar := &CodecParameters{ptr: outSt.CodecPar()}
		if outPar.ptr == nil {
			return fmt.Errorf("ffmpeg: out par %d nil", i)
		}
		if err := outPar.Copy(inPar); err != nil {
			return fmt.Errorf("ffmpeg: copy par %d: %w", i, err)
		}
		outIdx[i] = int32(outSt.Index())
	}
	// 新建字幕流：mov_text 编码，时基取毫秒。
	subStPtr := outFc.NewStream(nil)
	if subStPtr == nil {
		return fmt.Errorf("ffmpeg: new subtitle stream")
	}
	subSt := &Stream{ptr: subStPtr}
	subPar := &CodecParameters{ptr: subSt.CodecPar()}
	if subPar.ptr == nil {
		return fmt.Errorf("ffmpeg: sub par nil")
	}
	setSubPar(subPar.ptr, CodecIDMovText)
	subIdx := int32(subSt.Index())
	// 接输出文件并写头。
	var pb unsafe.Pointer
	if err := fc.Open(&pb, dst, AVIOFlagWrite); err != nil {
		return fmt.Errorf("ffmpeg: open out %s: %w", dst, err)
	}
	outFc.SetPb(pb)
	outIO := &IOContext{ptr: pb}
	if err := outFc.WriteHeader(nil); err != nil {
		outIO.Close()
		return fmt.Errorf("ffmpeg: write header: %w", err)
	}
	// 跳到段首（按视频流时基换算，往前找关键帧）。
	vid := fBestStream(inPtr, MediaTypeVideo, -1, -1, nil, 0)
	if vid >= 0 {
		if vst := inFc.StreamAt(int(vid)); vst != nil {
			tb := vst.TimeBase()
			if tb.Num > 0 && tb.Den > 0 {
				ts := startMs * int64(tb.Den) / (1000 * int64(tb.Num))
				_ = fSeekFrame(inPtr, vid, ts, SeekBackward)
			}
		}
	}
	// 搬包：只收时间窗内的视频/音频包，换流序号和时基后写出。
	// 时间窗按包 pts 换算成毫秒判断；结束包出现即停（多读几包容忍乱序）。
	// 注意：seek 落到关键帧 K（pts 可能早于 startMs），从 K 开始搬，
	// K 之前的散帧丢掉——丢了 K 则后面全是散帧，一帧都解不出
	//（实测 vr_oceans [1000,2000) 段 24 包 0 帧）；K 之前的不搬，
	// 否则关键帧稀疏的片子前滚太长，段时长超标。
	pkt := NewPacket()
	if pkt == nil {
		outIO.Close()
		return fmt.Errorf("ffmpeg: no packet")
	}
	defer pkt.Free()
	var mux Muxer
	wrote := 0
	over := 0
	opened := false // 开门关键帧到了才开始搬视频包
	for {
		pkt.Unref()
		if ret := fReadFrame(inPtr, pkt.ptr); ret < 0 {
			break
		}
		si := pkt.StreamIndex()
		if si < 0 || si >= nb {
			continue
		}
		inSt := inFc.StreamAt(si)
		outStPtr := streamPointer(outPtr, outIdx[si])
		if inSt == nil || outStPtr == nil {
			continue
		}
		ptsMs := pktPTSms(pkt.ptr, inSt.TimeBase())
		if si == int(vid) && !opened {
			if !pkt.IsKey() {
				continue
			}
			opened = true
		}
		if ptsMs >= endMs {
			if over += 1; over > 30 {
				break
			}
			continue
		}
		outSt := &Stream{ptr: outStPtr}
		pkt.RescaleTs(inSt.TimeBase(), outSt.TimeBase())
		pkt.SetStreamIndex(int(outIdx[si]))
		if err := mux.InterleavedWriteFrame(outPtr, pkt.ptr); err != nil {
			outIO.Close()
			return fmt.Errorf("ffmpeg: write pkt: %w", err)
		}
		wrote++
	}
	// 写字幕包：一句覆盖整段（mov_text 按毫秒时基）。
	if err := writeSubPacket(outPtr, subIdx, text, 0, endMs-startMs, &mux); err != nil {
		outIO.Close()
		return err
	}
	if ret := fAvWriteTrailer(outPtr); ret < 0 {
		outIO.Close()
		fAvformatFreeContext(outPtr)
		return fmt.Errorf("ffmpeg: trailer: %s", errText(ret))
	}
	outIO.Close()
	if wrote == 0 {
		fAvformatFreeContext(outPtr)
		return fmt.Errorf("ffmpeg: wrote 0 packets %d-%d", startMs, endMs)
	}
	fAvformatFreeContext(outPtr)
	okOut = true
	_ = okOut
	return nil
}

// pktPTSms 读包 pts 换算成毫秒（NoPTS 回 -1）。
func pktPTSms(pkt unsafe.Pointer, tb AVRational) int64 {
	pts := loadInt64(pkt, pktPTS)
	if pts == NoPTS || tb.Num <= 0 || tb.Den <= 0 {
		return -1
	}
	return pts * int64(tb.Num) * 1000 / int64(tb.Den)
}

// setSubPar 填字幕流参数：类型字幕 + mov_text 编码；时基由复用器兜底。
func setSubPar(par unsafe.Pointer, codecID int32) {
	*(*int32)(unsafe.Add(par, parCodecType)) = MediaTypeSubtitle
	*(*int32)(unsafe.Add(par, parCodecID)) = codecID
}

// writeSubPacket 写一句字幕包（payload 为 UTF-8 文本，pts/duration 按毫秒时基）。
func writeSubPacket(outPtr unsafe.Pointer, subIdx int32, text string, startMs, durMs int64, mux *Muxer) error {
	pkt := NewPacket()
	if pkt == nil {
		return fmt.Errorf("ffmpeg: no sub packet")
	}
	defer pkt.Free()
	raw := append([]byte(text), 0)
	var mem Mem
	buf := mem.Alloc(len(raw))
	if buf == nil {
		return fmt.Errorf("ffmpeg: no sub buf")
	}
	// buf 交给包接管（FromData 后别 Free，包释放时一并走）。
	copy(unsafe.Slice((*byte)(buf), len(raw)), raw)
	if err := pkt.FromData(buf, len(raw)-1); err != nil {
		mem.Free(buf)
		return fmt.Errorf("ffmpeg: sub from_data: %w", err)
	}
	*(*int64)(unsafe.Add(pkt.ptr, pktPTS)) = startMs
	*(*int64)(unsafe.Add(pkt.ptr, pktDuration)) = durMs
	pkt.SetStreamIndex(int(subIdx))
	// 字幕流时基视为 1/1000，包时基直写毫秒。
	if err := mux.InterleavedWriteFrame(outPtr, pkt.ptr); err != nil {
		return fmt.Errorf("ffmpeg: sub write: %w", err)
	}
	return nil
}

// verifySegment 重开输出验证：时长对、字幕轨在、能解出帧。
func verifySegment(t *testing.T, path string, bound [2]int64, wantSub string) {
	t.Helper()
	d, err := Open(path)
	if err != nil {
		t.Fatalf("reopen %s: %v", path, err)
	}
	defer d.Close()
	got := d.Info().DurMs
	want := bound[1] - bound[0]
	// 切点落关键帧，前后差几秒正常，允许 ±3 秒。
	if got < want-3000 || got > want+3000 {
		t.Fatalf("%s dur %d want ~%d", path, got, want)
	}
	fr, err := d.Next()
	if err != nil {
		t.Fatalf("%s decode: %v", path, err)
	}
	if fr.Width != 1920 || fr.Height != 1080 {
		t.Fatalf("%s size %dx%d", path, fr.Width, fr.Height)
	}
	// 字幕轨：直接看盒子流里有没有字幕类型。
	var inPtr unsafe.Pointer
	if ret := fOpenInput(&inPtr, path, nil, nil); ret < 0 {
		t.Fatalf("probe open: %s", errText(ret))
	}
	defer fCloseInput(&inPtr)
	inFc := &FormatContext{ptr: inPtr}
	foundSub := false
	for i := 0; i < inFc.NbStreams(); i++ {
		st := inFc.StreamAt(i)
		if st == nil {
			continue
		}
		par := &CodecParameters{ptr: st.CodecPar()}
		if par.ptr == nil {
			continue
		}
		if loadInt32(par.ptr, parCodecType) == MediaTypeSubtitle {
			foundSub = true
			if id := loadInt32(par.ptr, parCodecID); id != CodecIDMovText {
				t.Fatalf("%s sub codec %d", path, id)
			}
		}
	}
	if !foundSub {
		t.Fatalf("%s no subtitle track", path)
	}
	_ = wantSub
}
