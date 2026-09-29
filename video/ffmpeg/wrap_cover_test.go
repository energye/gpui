package ffmpeg

import (
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

// TestWrapCover pins every previously registered-but-unwrapped API behind
// a Go method: one call each, honest errors on bad input, nil-safe on
// empty holders. The so ships a trimmed codec set (34 codecs, 4
// encoders, no aac), so encoder asserts only check path shape.
func TestWrapCover(t *testing.T) {
	if err := ensureModResample(); err != nil {
		t.Skipf("lib missing: %v", err)
	}
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	// codec batch: close/encoder-lookup/iterate/parse/bsf-list
	cc := &CodecContext{}
	cc.Close()
	if FindEncoderByName("no-such-encoder-xyz") != nil {
		t.Fatal("bogus encoder found")
	}
	if all := IterateAll(); len(all) == 0 {
		t.Fatal("codec iterate empty")
	}
	if len(IterateBSFFilters()) == 0 {
		t.Fatal("bsf iterate empty")
	}
	if len(IterateParsers()) == 0 {
		t.Fatal("parser iterate empty")
	}
	lst := AllocBSFList()
	if lst == nil {
		t.Fatal("bsf list alloc nil")
	}
	lst.Free()
	lst2 := AllocBSFList()
	if err := lst2.AppendByName("no-such-filter-xyz", nil); err == nil {
		t.Fatal("bsf append bogus accepted")
	}
	lst2.Free()
	if _, err := ParseBSFList("no-such-filter-xyz"); err == nil {
		t.Fatal("bsf parse bogus accepted")
	}
	// packet batch
	p := NewPacket()
	if p == nil {
		t.Fatal("NewPacket nil")
	}
	defer p.Free()
	if n := SideDataName(1); n == "" {
		t.Fatal("SideDataName empty")
	}
	// frame batch
	f := NewFrame()
	if f == nil {
		t.Fatal("NewFrame nil")
	}
	defer f.Free()
	if n := FrameSideDataName(1); n == "" {
		t.Fatal("FrameSideDataName empty")
	}
	if f.GetPlaneBuffer(0) != nil {
		t.Fatal("empty frame plane buffer non-nil")
	}
	// scale batch
	if got := ImageBufferSize(PixFmtYUV420P, 16, 16, 1); got <= 0 {
		t.Fatalf("ImageBufferSize = %d", got)
	}
	IsEndianSupported(PixFmtYUV420P)
	if err := ImageCheckSize(16, 16); err != nil {
		t.Fatal(err)
	}
	if err := ImageCheckSize2(16, 16, 16*16, PixFmtYUV420P); err != nil {
		t.Fatal(err)
	}
	if err := ImageCheckSar(16, 16, AVRational{Num: 1, Den: 1}); err != nil {
		t.Fatal(err)
	}
	if PixFmtDesc(PixFmtYUV420P) == nil {
		t.Fatal("PixFmtDesc nil")
	}
	cached := CachedScaler(nil, 16, 16, PixFmtYUV420P, 16, 16, PixFmtRGBA, SwsBilinear)
	if cached == nil {
		t.Fatal("CachedScaler nil")
	}
	defer cached.Free()
	reused := CachedScaler(cached, 16, 16, PixFmtYUV420P, 16, 16, PixFmtRGBA, SwsBilinear)
	if reused == nil {
		t.Fatal("CachedScaler reuse nil")
	}
	// device batch
	var dl DeviceList
	if dl.Configuration() == "" || dl.License() == "" {
		t.Fatal("device config/license empty")
	}
	// buffer batch
	var m Mem
	if m.AllocArray(4, 8) == nil {
		t.Fatal("AllocArray nil")
	}
	b0 := NewBufferZeroed(64)
	if b0 == nil {
		t.Fatal("NewBufferZeroed nil")
	}
	defer b0.Unref()
	// opt batch on a real object
	var rs Resampler
	swr := rs.Alloc2()
	if swr == nil {
		t.Fatal("swr alloc nil")
	}
	defer fSwrFree(&swr)
	o := OptObject{ptr: swr}
	if err := o.SetImageSize("nosuchopt", 1, 1, 0); err == nil {
		t.Fatal("SetImageSize bogus accepted")
	}
	if _, _, err := o.GetImageSize("nosuchopt", 0); err == nil {
		t.Fatal("GetImageSize bogus accepted")
	}
	if err := o.SetPixFmt("nosuchopt", 1, 0); err == nil {
		t.Fatal("SetPixFmt bogus accepted")
	}
	if _, err := o.GetPixFmt("nosuchopt", 0); err == nil {
		t.Fatal("GetPixFmt bogus accepted")
	}
	if err := o.SetVideoRate("nosuchopt", AVRational{1, 25}, 0); err == nil {
		t.Fatal("SetVideoRate bogus accepted")
	}
	if _, err := o.GetVideoRate("nosuchopt", 0); err == nil {
		t.Fatal("GetVideoRate bogus accepted")
	}
	if err := o.SetBin("nosuchopt", nil, 0, 0); err == nil {
		t.Fatal("SetBin bogus accepted")
	}
	if err := o.SetFromString("nosuchopt=1", "", "=", ",", 0); err == nil {
		t.Fatal("SetFromString bogus accepted")
	}
	if !o.IsDefaultByName("nosuchopt", 0) {
		t.Fatal("IsDefaultByName bogus false")
	}
	if _, err := o.GetArraySize("nosuchopt", 0); err == nil {
		t.Fatal("GetArraySize bogus accepted")
	}
	if _, err := o.EvalInt(nil, "1"); err == nil {
		t.Fatal("EvalInt nil-opt accepted")
	}
	if o.FlagIsSet("x", "y") {
		t.Fatal("FlagIsSet bogus true")
	}
	FreeOptions(nil)
	if _, err := o.Get("nosuchopt", 0); err == nil {
		t.Fatal("Get bogus accepted")
	}
	if _, err := o.QueryRanges("nosuchopt", 0); err == nil {
		t.Fatal("QueryRanges bogus accepted")
	}
	if _, err := o.Serialize(0, 0, '=', ','); err == nil {
		t.Fatal("Serialize bogus accepted")
	}
	o.ShowOptions(nil, 0, 0)
	d := NewDictionary()
	if d == nil {
		t.Fatal("NewDictionary nil")
	}
	defer d.Free()
	if err := d.Set("k", "v", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := d.GetString(',', '='); err != nil {
		t.Fatal(err)
	}
	// error batch: human errors, version strings, log switch, clock/math/cpu
if StrError(-1094995529) == "" {
t.Fatal("StrError empty")
}
var lib Library
if v := lib.Version(); len(v) < 3 || v[:3] != "7.1" {
t.Fatalf("Version = %q", v)
}
if lib.License() == "" || lib.Configuration() == "" {
t.Fatal("License/Configuration empty")
}
var lg Log
lv := lg.Level()
lg.SetLevel(LogError)
if lg.Level() != LogError {
t.Fatal("SetLevel not sticky")
}
lg.SetLevel(lv)
var mth Math
if mth.Rescale(2, 1, 2) != 1 {
t.Fatal("Rescale(2,1,2) != 1")
}
var clk Clock
if clk.NowUs() <= 0 {
t.Fatal("NowUs not positive")
}
var cpu Cpu
if cpu.Count() <= 0 {
t.Fatal("Cpu.Count not positive")
}
// media batch: bogus channel name -> AV_CHAN_NONE(-1)
var md MediaDesc
cn, freeCn := featCStr("no-such-channel-xyz")
defer freeCn()
if got := md.ChannelFromString(cn); got != -1 {
t.Fatalf("ChannelFromString bogus = %d, want -1", got)
}
// NULL would segfault inside ffmpeg, so a bogus-order layout stands in:
// an invalid order must report an error, not crash.
var zl [64]byte
*(*int32)(unsafe.Pointer(&zl[0])) = 99
if err := md.ChannelLayoutCheck(unsafe.Pointer(&zl[0])); err == nil {
t.Fatal("ChannelLayoutCheck bogus order accepted")
}
// filter batch: bogus filter name -> nil
var fg FilterGraph
bn, freeBn := featCStr("no-such-filter-xyz")
defer freeBn()
if fg.GetByName(bn) != nil {
t.Fatal("bogus filter found")
}
// demux batch: bogus paths -> readable errors
if _, err := Open("/no/such/file.mp4"); err == nil {
t.Fatal("Open bogus accepted")
}
if _, err := OpenAudio("/no/such/file.mp4"); err == nil {
t.Fatal("OpenAudio bogus accepted")
}
// crypto batch: alloc/free roundtrip
var cr Crypto
if cr.AesSize() <= 0 {
t.Fatal("AesSize not positive")
}
_ = unsafe.Pointer(nil)
}

// TestWrapCoverDemux fills the format_demux module gap: real-file demux
// (feat_small.mp4 via Open), in-memory AVIO roundtrip, version/config
// strings, guessers, protocol/directory enums. Out-param slots get real
// memory, never NULL; every write-stream pairs with Close/Closep;
// network/tls handshakes are not touched (no network in unit tests).
func TestWrapCoverDemux(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var fx FormatContext
var mem Mem
// ---- version/config/license strings (avformat.h, 非空即可) ----
if cstr(fx.Configuration()) == "" {
t.Fatal("Format Configuration empty")
}
if cstr(fx.License()) == "" {
t.Fatal("Format License empty")
}
if fx.GetClass() == nil {
t.Fatal("Format GetClass nil")
}
if got := fx.Version(); got>>16 != 61 {
t.Fatalf("Format Version = %#x, want major 61 (7.x)", got)
}
// ---- network init/deinit pair (无网络也调得通) ----
if err := fx.NetworkInit(); err != nil {
t.Fatalf("NetworkInit: %v", err)
}
if err := fx.NetworkDeinit(); err != nil {
t.Fatalf("NetworkDeinit: %v", err)
}
// ---- stream-group names (avformat.c switch 表, 越界回 nil) ----
if got := cstr(fx.StreamGroupName(1)); got != "IAMF Audio Element" {
t.Fatalf("StreamGroupName(1) = %q", got)
}
if fx.StreamGroupName(99) != nil {
t.Fatal("StreamGroupName(99) non-nil")
}
// ---- guessers: base版无复用器(只看片不写片), 猜复用器/编码走 full 版 ----
// base 下 GuessFormat 诚实回 nil, 不算错; 真断言只在 full 版跑.
mp4Name, freeMp4 := featCStr("clip.mp4")
defer freeMp4()
if guessed := fx.GuessFormat(nil, mp4Name, nil); guessed != nil {
gfPtr := Format{ptr: guessed}
// C 只看复用器默认编码不用 short_name: mp4 默认是 x264 版 H264,
// 无 x264 的 LGPL 包里是 MPEG4(12), 见 movenc.c video_codec.
h264Name, freeH264 := featCStr("h264")
defer freeH264()
if got := gfPtr.GuessCodec(h264Name, mp4Name, nil, 0); got != 12 {
t.Fatalf("GuessCodec(mp4,h264,video) = %d, want 12 (MPEG4, 无 x264 版默认)", got)
}
if got := gfPtr.GuessCodec(nil, nil, nil, 4); got != 0 {
t.Fatalf("GuessCodec(attachment) = %d, want 0 (NONE)", got)
}
if _, err := gfPtr.QueryCodec(12, 0); err != nil {
t.Fatalf("QueryCodec(mp4,MPEG4): %v", err)
}
} else if IsFull() {
t.Fatal("GuessFormat(clip.mp4) nil on full lib")
} else {
t.Logf("GuessFormat(clip.mp4) nil on base lib (base 无复用器, 符合预期)")
}
movTags := fx.GetMovVideoTags()
riffTags := fx.GetRiffVideoTags()
movAudio := fx.GetMovAudioTags()
riffAudio := fx.GetRiffAudioTags()
if movTags == nil || riffTags == nil || movAudio == nil || riffAudio == nil {
t.Fatal("mov/riff tag tables nil")
}
// C 里直接 strcmp, 传 nil 会崩, 只用 file 名查 (file 协议必在).
fileProto, freeFileProto := featCStr("file")
defer freeFileProto()
if fx.ProtocolGetClass(fileProto) == nil {
t.Fatal("ProtocolGetClass(file) nil")
}
// ---- protocol enums: file 协议必在输入输出两边 ----
var opaque unsafe.Pointer
foundFileIn := false
for i := 0; i < 200; i++ {
name := fx.EnumProtocols(&opaque, 0)
if name == nil {
break
}
if cstr(name) == "file" {
foundFileIn = true
}
}
if !foundFileIn {
t.Fatal("EnumProtocols(input) missing file")
}
opaque = nil
foundFileOut := false
for i := 0; i < 200; i++ {
name := fx.EnumProtocols(&opaque, 1)
if name == nil {
break
}
if cstr(name) == "file" {
foundFileOut = true
}
}
if !foundFileOut {
t.Fatal("EnumProtocols(output) missing file")
}
httpURL, freeHTTP := featCStr("http://example.com/clip.mp4")
defer freeHTTP()
if fx.FindProtocolName(httpURL) == nil {
t.Fatal("FindProtocolName(http) nil")
}
cwdURL, freeCWD := featCStr(".")
defer freeCWD()
if err := fx.Check(cwdURL, 1); err != nil {
t.Fatalf("Check(.,read): %v", err)
}
// ---- directory: 打开当前目录读一项再关 (真实目录, 非网络) ----
dotStr, freeDot := featCStr(".")
defer freeDot()
var dirCtx unsafe.Pointer
if err := fx.OpenDir(&dirCtx, dotStr, nil); err != nil {
t.Fatalf("OpenDir(.): %v", err)
} else {
defer func() {
if dirCtx != nil {
_ = fx.CloseDir(&dirCtx)
}
}()
var entry unsafe.Pointer
if err := fx.ReadDir(dirCtx, &entry); err != nil {
t.Fatalf("ReadDir: %v", err)
}
if entry != nil {
fx.FreeDirectoryEntry(&entry)
}
}
// ---- real-file demux: feat_small.mp4 (L3 同款, 这里只验 demux 层) ----
dec, err := Open("../testdata/feat_small.mp4")
if err != nil {
t.Fatalf("Open(feat_small): %v", err)
}
defer dec.Close()
info := dec.Info()
if info.Width <= 0 || info.Height <= 0 {
t.Fatalf("feat_small size = %dx%d", info.Width, info.Height)
}
fmtPtr := dec.RawFormatCtx()
if fmtPtr == nil {
t.Fatal("FormatCtx nil")
}
fctx := FormatContext{ptr: fmtPtr}
if fctx.NbStreams() < 1 {
t.Fatal("NbStreams < 1")
}
st := fctx.StreamAt(0)
if st == nil {
t.Fatal("StreamAt(0) nil")
}
if st.Index() != 0 {
t.Fatalf("StreamAt(0).Index = %d, want 0", st.Index())
}
if st.CodecPar() == nil {
t.Fatal("CodecPar nil")
}
tb := st.TimeBase()
if tb.Den <= 0 {
t.Fatalf("TimeBase = %+v", tb)
}
// FindBestStream: 0=视频必中, 4=附件必无 (-5=STREAM_NOT_FOUND 类错)
if got := fctx.FindBestStream(0, -1, -1, nil, 0); got < 0 {
t.Fatalf("FindBestStream(video) = %d", got)
}
// Open 内部已经探过流参数, 这里不再二次探 (二次探要读包, 已读过一包的
// 上下文再探会踩内部状态, 直接崩; 要验 FindStreamInfo 用新开的盒子).
// FindStreamInfo 的真覆盖在 L3 的 Open 路径里, 这里只验空指针守卫.
var nilFctx *FormatContext
if err := nilFctx.FindStreamInfo(nil); err == nil {
t.Fatal("FindStreamInfo(nil ctx) accepted")
}
vspec, freeVspec := featCStr("v")
defer freeVspec()
if err := fctx.MatchStreamSpecifier(st.Ptr(), vspec); err != nil {
t.Fatalf("MatchStreamSpecifier(v): %v", err)
}
aspec, freeAspec := featCStr("a")
defer freeAspec()
if err := fctx.MatchStreamSpecifier(st.Ptr(), aspec); err == nil {
t.Logf("MatchStreamSpecifier(video vs a): matched (spec 语义以 C 为准)")
}
if got := fctx.GuessFrameRate(st.Ptr(), nil); got.Num <= 0 || got.Den <= 0 {
t.Fatalf("GuessFrameRate = %+v", got)
}
if got := fctx.GuessSampleAspectRatio(st.Ptr(), nil); got.Den <= 0 {
t.Fatalf("GuessSampleAspectRatio = %+v", got)
}
if n := st.IndexGetEntriesCount(); n < 0 {
t.Fatalf("IndexGetEntriesCount = %d", n)
}
if err := st.AddIndexEntry(0, 0, 100, 10, 1); err != nil {
t.Logf("AddIndexEntry honest error: %v", err)
}
if got := st.IndexSearchTimestamp(0, 0); got < 0 {
t.Logf("IndexSearchTimestamp(0) = %d (无索引时负数正常)", got)
}
pkt := NewPacket()
if pkt == nil {
t.Fatal("NewPacket nil")
}
defer pkt.Free()
if err := fctx.ReadFrame(pkt.Ptr()); err != nil {
t.Fatalf("ReadFrame: %v", err)
}
if err := fctx.SeekFrame(0, 0, 4); err != nil {
t.Logf("SeekFrame honest error: %v", err)
}
if err := fctx.SeekFile(-1, 0, 0, 0, 0); err != nil {
t.Logf("SeekFile honest error: %v", err)
}
if err := fctx.QueueAttachedPictures(); err != nil {
t.Logf("QueueAttachedPictures honest error: %v", err)
}
fctx.DumpFormat(0, nil, 0)
if err := fctx.Flush(); err != nil {
t.Fatalf("Format Flush: %v", err)
}
// ---- dyn-buf AVIO roundtrip: 写 4 字节, 读回验内容 ----
// 写端: OpenDynBuf -> Write/W8/Wb16/Wl16/PutStr -> GetDynBuf/CloseDynBuf
var wctx unsafe.Pointer
if err := fx.OpenDynBuf(&wctx); err != nil {
t.Fatalf("OpenDynBuf: %v", err)
}
wio := IOContext{ptr: wctx}
helloStr, freeHello := featCStr("hi")
defer freeHello()
if err := wio.PutStr(helloStr); err != nil {
t.Fatalf("PutStr: %v", err)
}
if err := wio.PutStr16le(helloStr); err != nil {
t.Fatalf("PutStr16le: %v", err)
}
if err := wio.PutStr16be(helloStr); err != nil {
t.Fatalf("PutStr16be: %v", err)
}
wio.W8(0x41)
wio.Wb16(0x0102)
wio.Wb24(0x010203)
wio.Wb32(0x01020304)
wio.Wb64(0x0102030405060708)
wio.Wl16(0x0102)
wio.Wl24(0x010203)
wio.Wl32(0x01020304)
wio.Wl64(0x0102030405060708)
raw := mem.Alloc(4)
if raw == nil {
t.Fatal("avio raw buf nil")
}
defer mem.Free(raw)
copy(unsafe.Slice((*byte)(raw), 4), []byte{9, 8, 7, 6})
wio.Write(raw, 4)
// PrintStringArray/Vprintf 只许调在写流上: C 最后走 avio_write,
// 写进只读流会在 flush_buffer 里转圈出不来 (buf_end=buf_ptr, size 不减).
// 这里给真数组 [hi hi nil], 顺手把空 fmt 的 Vprintf 真串路径也验了.
arrBuf := mem.Alloc(24)
if arrBuf == nil {
t.Fatal("string array buf nil")
}
defer mem.Free(arrBuf)
*(*unsafe.Pointer)(unsafe.Add(arrBuf, 0)) = helloStr
*(*unsafe.Pointer)(unsafe.Add(arrBuf, 8)) = helloStr
*(*unsafe.Pointer)(unsafe.Add(arrBuf, 16)) = nil
wio.PrintStringArray(arrBuf)
emptyFmt, freeEmptyFmt := featCStr("")
defer freeEmptyFmt()
vaZero := mem.AllocZ(32)
if vaZero == nil {
t.Fatal("vprintf va_list nil")
}
defer mem.Free(vaZero)
if err := wio.Vprintf(emptyFmt, vaZero); err != nil {
t.Fatalf("Vprintf(empty): %v", err)
}
wio.WriteMarker(0, 0)
wio.Flush()
var peek unsafe.Pointer
if n := wio.GetDynBuf(&peek); n <= 0 || peek == nil {
t.Fatalf("GetDynBuf = %d", n)
}
var dyn unsafe.Pointer
n, err := wio.CloseDynBuf(&dyn)
if err != nil || n <= 0 || dyn == nil {
t.Fatalf("CloseDynBuf = (%d, %v)", n, err)
}
defer mem.Free(dyn)
got := unsafe.Slice((*byte)(dyn), n)
if got[0] != 'h' || got[1] != 'i' || got[2] != 0 {
t.Fatalf("dyn buf head = %v, want hi\\0", got[:3])
}
// 读端: 内存 AVIO 不接读回调就是空流, R8/读系列只验不崩不验值;
// 真正的内容读写走 dyn-buf 写端已验, 读值用 SeekPos/Size/Feof 验状态.
memBuf := mem.Alloc(4096)
if memBuf == nil {
t.Fatal("avio ctx buf nil")
}
defer mem.Free(memBuf)
rioPtr := fx.AllocIOContext(memBuf, 4096, 0, nil, nil, nil, nil)
if rioPtr == nil {
t.Fatal("AllocIOContext nil")
}
rio := IOContext{ptr: rioPtr}
defer fx.ContextFree(&rioPtr)
_ = rio.Size()
_ = rio.R8()
_ = rio.Rb16()
_ = rio.Rb24()
_ = rio.Rb32()
_ = rio.Rb64()
_ = rio.Rl16()
_ = rio.Rl24()
_ = rio.Rl32()
_ = rio.Rl64()
if pos, err := rio.SeekPos(0, 0); err != nil || pos != 0 {
t.Fatalf("SeekPos(0) = (%d, %v)", pos, err)
}
if _, err := rio.SeekPos(0, 0); err != nil {
t.Fatalf("SeekPos rewind: %v", err)
}
sbuf := mem.Alloc(16)
if sbuf == nil {
t.Fatal("getstr buf nil")
}
defer mem.Free(sbuf)
// 空流上 GetStr 系列回 0 或报错都算诚实, 只验不崩.
_ = rio.GetStr(16, sbuf, 16)
_ = rio.GetStr16le(16, sbuf, 16)
_ = rio.GetStr16be(16, sbuf, 16)
part := mem.Alloc(2)
if part == nil {
t.Fatal("partial buf nil")
}
defer mem.Free(part)
if _, err := rio.ReadPartial(part, 2); err == nil {
t.Logf("ReadPartial(空流) 读到数据 (C 行为为准)")
}
full := mem.Alloc(3)
if full == nil {
t.Fatal("full buf nil")
}
defer mem.Free(full)
if _, err := rio.Read(full, 3); err == nil {
t.Logf("Read(空流) 读到数据 (C 行为为准)")
}
if rio.Skip(1) < 0 {
t.Fatal("Skip failed")
}
bp := NewBPrint(64, 4096)
if bp == nil {
t.Fatal("NewBPrint nil")
}
defer bp.Free()
if _, err := rio.SeekPos(0, 0); err != nil {
t.Fatalf("SeekPos rewind10: %v", err)
}
// 空流上 ReadToBprint 回 ENOMEM 也算诚实 (C 要分配缓冲), 只验不崩.
if err := rio.ReadToBprint(bp.Ptr(), 4096); err != nil {
t.Logf("ReadToBprint(空流) honest error: %v", err)
}
if err := rio.Pause(0); err != nil {
t.Logf("Pause honest error (非网络流): %v", err)
}
// Handshake 在 C 里直接解 opaque, 内存 AVIO 的 opaque 是 nil 会崩,
// 不调它 (TLS/网络流才用得上, 单测无网络); 注释留给后人.
// PrintStringArray/Vprintf 已在上面的写端验过, 读端不再调
// (往只读流写字会在 C 的 flush_buffer 里转圈出不来); WriteMarker
// 读端安全 (C 看到 write_data_type 为空提前返回).
rio.WriteMarker(0, 0)
// C 里直接 strcmp, nil 会崩, 这里只验 file 名那条 (上面已验过).
if fctx.ProtocolGetClass(fileProto) == nil {
t.Fatal("ProtocolGetClass(file) nil (second check)")
}
if got := rio.SeekTime(-1, 0, 0); got < 0 {
t.Logf("SeekTime honest negative: %d", got)
}
// ---- UrlSplit: 标准网址拆段 ----
protoB := mem.Alloc(16)
authB := mem.Alloc(16)
hostB := mem.Alloc(64)
pathB := mem.Alloc(64)
portB := mem.Alloc(4)
urlStr, freeURL := featCStr("http://example.com:8080/clip.mp4")
defer freeURL()
if protoB == nil || authB == nil || hostB == nil || pathB == nil || portB == nil {
t.Fatal("urlsplit bufs nil")
}
defer mem.Free(protoB)
defer mem.Free(authB)
defer mem.Free(hostB)
defer mem.Free(pathB)
defer mem.Free(portB)
fx.UrlSplit(protoB, 16, authB, 16, hostB, 64, portB, pathB, 64, urlStr)
if cstr(protoB) != "http" {
t.Fatalf("UrlSplit proto = %q, want http", cstr(protoB))
}
if cstr(hostB) != "example.com" {
t.Fatalf("UrlSplit host = %q", cstr(hostB))
}
if cstr(pathB) != "/clip.mp4" {
t.Fatalf("UrlSplit path = %q", cstr(pathB))
}
// ---- nil-safe: 空 holder 不崩 ----
var nilFx *FormatContext
if nilFx.NbStreams() != 0 || nilFx.StreamAt(0) != nil {
t.Fatal("nil FormatContext not safe")
}
var nilSt *Stream
if nilSt.Index() != -1 || nilSt.CodecPar() != nil {
t.Fatal("nil Stream not safe")
}
var nilIO *IOContext
if nilIO.R8() != 0 || nilIO.Rb16() != 0 || nilIO.Feof() != 0 || nilIO.Size() != 0 {
t.Fatal("nil IOContext not safe")
}
nilIO.Flush()
}

// TestWrapCoverFilter fills the filter_graph module gap: metadata queries,
// wrapper-built buffer->scale->sink chain, frame push/pull with value pins
// (W/H/Format/Type/SAR/TB from link negotiation), segment API full cycle,
// error paths on bogus names. Out-param slots get real memory, never NULL;
// every graph pairs with GraphFree. C 头核过: pad 数组在 AVFilter 头 16/24
// 字节处; 写只读流会转圈 (demux 教训) 所以推拉只走配好的真链.
func TestWrapCoverFilter(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var fg FilterGraph
var mem Mem
// ---- 纯元数据 (avfilter.h/buffersink.c, 不建图) ----
if cstr(fg.Configuration()) == "" {
t.Fatal("Filter Configuration empty")
}
if cstr(fg.License()) == "" {
t.Fatal("Filter License empty")
}
if got := fg.Version(); got>>16 != 10 {
t.Fatalf("Filter Version = %#x, want major 10 (7.x)", got)
}
if fg.GetClass() == nil {
t.Fatal("Filter GetClass nil")
}
scaleName, freeScaleName := featCStr("scale")
defer freeScaleName()
scaleFilt := fg.GetByName(scaleName)
if scaleFilt == nil {
t.Fatal("GetByName(scale) nil")
}
bogusFilt, freeBogusFilt := featCStr("no-such-filter-xyz")
defer freeBogusFilt()
if fg.GetByName(bogusFilt) != nil {
t.Fatal("GetByName(bogus) non-nil")
}
var fltPtr Filter
fltPtr = Filter{ptr: scaleFilt}
if got := fltPtr.FilterPadCount(0); got != 1 {
t.Fatalf("scale inputs = %d, want 1", got)
}
if got := fltPtr.FilterPadCount(1); got != 1 {
t.Fatalf("scale outputs = %d, want 1", got)
}
inPads := loadPtr(scaleFilt, 16)
outPads := loadPtr(scaleFilt, 24)
if inPads == nil || outPads == nil {
t.Fatal("scale pads nil")
}
if got := cstr(fg.PadGetName(inPads, 0)); got == "" {
t.Fatal("PadGetName(input) empty")
}
if got := fg.PadGetType(inPads, 0); got != 0 {
t.Fatalf("PadGetType(scale in) = %d, want 0 (video)", got)
}
// ---- 包装搭链: buffer(320x240) -> scale(160x120) -> sink ----
gptr := fg.GraphAlloc()
if gptr == nil {
t.Fatal("GraphAlloc nil")
}
g := FilterGraph{ptr: gptr}
defer g.GraphFree(&gptr)
mkFilter := func(kind, name, args string) unsafe.Pointer {
kn, kf := featCStr(kind)
defer kf()
nn, nf := featCStr(name)
defer nf()
f := fg.GetByName(kn)
if f == nil {
t.Fatalf("GetByName(%s) nil", kind)
}
var argPtr unsafe.Pointer
var af func()
if args != "" {
argPtr, af = featCStr(args)
defer af()
}
var ctx unsafe.Pointer
if err := g.GraphCreateFilter(&ctx, f, nn, argPtr, nil, gptr); err != nil {
t.Fatalf("GraphCreateFilter(%s): %v", kind, err)
}
if ctx == nil {
t.Fatalf("GraphCreateFilter(%s) nil ctx", kind)
}
return ctx
}
srcCtx := mkFilter("buffer", "src", "video_size=320x240:pix_fmt=0:time_base=1/25:pixel_aspect=1/1")
midCtx := mkFilter("scale", "mid", "160:120")
sinkCtx := mkFilter("buffersink", "sink", "")
srcFC := FilterContext{ptr: srcCtx}
midFC := FilterContext{ptr: midCtx}
if err := srcFC.Link(0, midCtx, 0); err != nil {
t.Fatalf("Link src->mid: %v", err)
}
if err := midFC.Link(0, sinkCtx, 0); err != nil {
t.Fatalf("Link mid->sink: %v", err)
}
if got := g.GraphConfig(nil); got != 0 {
t.Fatalf("GraphConfig = %d, want 0", got)
}
dump := g.GraphDump(nil)
if dump == nil || cstr(dump) == "" {
t.Fatal("GraphDump empty")
}
mem.Free(dump)
srcName, freeSrcName := featCStr("src")
defer freeSrcName()
if g.GraphGetFilter(srcName) == nil {
t.Fatal("GraphGetFilter(src) nil")
}
bogusGet, freeBogusGet := featCStr("no-such-instance-xyz")
defer freeBogusGet()
if g.GraphGetFilter(bogusGet) != nil {
t.Fatal("GraphGetFilter(bogus) non-nil")
}
g.GraphSetAutoConvert(0)
// ---- 推两帧拉两帧, 值按链协商结果钉死 ----
mkYUV := func(w, h int32) *Frame {
f := NewFrame()
if f == nil {
t.Fatal("NewFrame nil")
}
*(*int32)(unsafe.Add(f.Ptr(), frameWidth)) = w
*(*int32)(unsafe.Add(f.Ptr(), frameHeight)) = h
*(*int32)(unsafe.Add(f.Ptr(), frameFormat)) = PixFmtYUV420P
if err := f.GetBuffer(32); err != nil {
t.Fatalf("frame GetBuffer: %v", err)
}
return f
}
in1 := mkYUV(320, 240)
defer in1.Free()
in2 := mkYUV(320, 240)
defer in2.Free()
out1 := NewFrame()
if out1 == nil {
t.Fatal("NewFrame(out1) nil")
}
defer out1.Free()
out2 := NewFrame()
if out2 == nil {
t.Fatal("NewFrame(out2) nil")
}
defer out2.Free()
fsrc := FilterSource{ptr: srcCtx}
fsink := FilterSink{ptr: sinkCtx}
sinkFC := FilterContext{ptr: sinkCtx}
if err := fsrc.AddFrame(in1.Ptr()); err != nil {
t.Fatalf("AddFrame: %v", err)
}
if err := srcFC.WriteFrame(in2.Ptr()); err != nil {
t.Fatalf("WriteFrame: %v", err)
}
// 推完两帧立刻查失败计数 (后面拉空/EAGAIN 会涨).
if got := srcFC.GetNbFailedRequests(); got != 0 {
t.Fatalf("GetNbFailedRequests = %d, want 0", got)
}
if got := fsink.GetFrame(out1.Ptr()); got != 0 {
t.Fatalf("GetFrame = %d, want 0", got)
}
if got := fsink.GetFrameFlags(out2.Ptr(), 0); got != 0 {
t.Fatalf("GetFrameFlags = %d, want 0", got)
}
// 队列拉空后再推第三帧 (连推三帧中间那帧的槽还没腾出来, 会报 ENOMEM).
// AddFrameFlags 和 WriteFrame 同路不同旗, 这里补 flags 版.
// 注意 AddFrame 会把帧搬进图里 (move_ref), in1 第一次推完就空了,
// 这里拿 WriteFrame 留住的 in2 推 (它走 clone 路, 缓冲还在).
if err := srcFC.AddFrameFlags(in2.Ptr(), 0); err != nil {
t.Fatalf("AddFrameFlags: %v", err)
}
if got := *(*int32)(unsafe.Add(out1.Ptr(), frameWidth)); got != 160 {
t.Fatalf("pulled w = %d, want 160", got)
}
if got := *(*int32)(unsafe.Add(out1.Ptr(), frameHeight)); got != 120 {
t.Fatalf("pulled h = %d, want 120", got)
}
if got := fsink.GetW(); got != 160 {
t.Fatalf("GetW = %d, want 160", got)
}
if got := fsink.GetH(); got != 120 {
t.Fatalf("GetH = %d, want 120", got)
}
if got := fsink.GetFormat(); got != 0 {
t.Fatalf("GetFormat = %d, want 0 (yuv420p)", got)
}
if got := fsink.GetType(); got != 0 {
t.Fatalf("GetType = %d, want 0 (video)", got)
}
if got := fsink.GetSampleAspectRatio(); got != (AVRational{1, 1}) {
t.Fatalf("GetSampleAspectRatio = %+v, want {1 1}", got)
}
if got := fsink.GetTimeBase(); got != (AVRational{1, 25}) {
t.Fatalf("GetTimeBase = %+v, want {1 25}", got)
}
if got := fsink.GetFrameRate(); got != (AVRational{0, 1}) {
t.Fatalf("GetFrameRate = %+v, want {0 1} (buffer 源不设帧率)", got)
}
if got := fsink.GetSampleRate(); got != 0 {
t.Fatalf("GetSampleRate(video) = %d, want 0", got)
}
if got := sinkFC.GetChannels(); got != 0 {
t.Fatalf("GetChannels(video) = %d, want 0", got)
}
if got := fsink.GetColorRange(); got < 0 {
t.Fatalf("GetColorRange = %d", got)
}
if got := fsink.GetColorspace(); got < 0 {
t.Fatalf("GetColorspace = %d", got)
}
if fsink.GetHwFramesCtx() != nil {
t.Fatal("GetHwFramesCtx non-nil (无硬解)")
}
chBuf := mem.AllocZ(32)
if chBuf == nil {
t.Fatal("chlayout buf nil")
}
defer mem.Free(chBuf)
if got := fsink.GetChLayout(chBuf); got != 0 {
t.Fatalf("GetChLayout = %d, want 0", got)
}
if got := fsink.GetSamples(out1.Ptr(), 1024); got >= 0 {
t.Fatalf("GetSamples(video) = %d, want < 0", got)
}
if got := fsrc.GetStatus(); got != 0 {
t.Fatalf("GetStatus = %d, want 0", got)
}
fsink.SetFrameSize(1)
param := fsrc.ParametersAlloc()
if param == nil {
t.Fatal("ParametersAlloc nil")
}
defer mem.Free(param)
if err := fsrc.ParametersSet(param); err != nil {
t.Fatalf("ParametersSet: %v", err)
}
if err := srcFC.Close(0, 0); err != nil {
t.Fatalf("buffersrc Close: %v", err)
}
if err := g.GraphRequestOldest(); err == nil {
t.Logf("GraphRequestOldest after drain: success (C 语义为准)")
}
// ---- 命令通道: 野命令诚实报错 ----
cmdStr, freeCmd := featCStr("no-such-cmd-xyz")
defer freeCmd()
argStr, freeArg := featCStr("1")
defer freeArg()
resBuf := mem.Alloc(64)
if resBuf == nil {
t.Fatal("command res buf nil")
}
defer mem.Free(resBuf)
if _, err := midFC.ProcessCommand(cmdStr, argStr, resBuf, 64, 0); err == nil {
t.Fatal("ProcessCommand(bogus) accepted")
}
if err := g.GraphSendCommand(cmdStr, cmdStr, argStr, resBuf, 64, 0); err == nil {
t.Fatal("GraphSendCommand(bogus target) accepted")
}
// 排队只管收不管验 (C 直接入队, 真错发的时候才报), 野名字也收下算过.
if err := g.GraphQueueCommand(cmdStr, cmdStr, argStr, 0, 0); err != nil {
t.Fatalf("GraphQueueCommand: %v", err)
}
// ---- 独立小滤镜上验 Init/Config 错误路 (AllocFilter 要真图, 零值调等于传空图会崩) ----
gtmpPtr := fg.GraphAlloc()
if gtmpPtr == nil {
t.Fatal("GraphAlloc(tmp) nil")
}
gtmp := FilterGraph{ptr: gtmpPtr}
defer gtmp.GraphFree(&gtmpPtr)
lonelyName, freeLonely := featCStr("lonely")
defer freeLonely()
lonelyAlloc := gtmp.GraphAllocFilter(scaleFilt, lonelyName)
if lonelyAlloc == nil {
t.Fatal("GraphAllocFilter nil")
}
lonelyFC := FilterContext{ptr: lonelyAlloc}
defer lonelyFC.Free()
scaleArgs, freeScaleArgs := featCStr("320:240")
defer freeScaleArgs()
if err := lonelyFC.InitStr(scaleArgs); err != nil {
t.Fatalf("InitStr(scale 320:240): %v", err)
}
lonely2 := gtmp.GraphAllocFilter(scaleFilt, lonelyName)
if lonely2 == nil {
t.Fatal("GraphAllocFilter2 nil")
}
lonelyFC2 := FilterContext{ptr: lonely2}
defer lonelyFC2.Free()
var emptyDict unsafe.Pointer
if err := lonelyFC2.InitDict(&emptyDict); err != nil {
t.Fatalf("InitDict(nil): %v", err)
}
if err := lonelyFC.ConfigLinks(); err == nil {
t.Logf("ConfigLinks(未连线滤镜) success (C 语义为准)")
}
// ---- 段 API 全周期 (avfilter.h 新链路, flags 全传 0) ----
g2ptr := fg.GraphAlloc()
if g2ptr == nil {
t.Fatal("GraphAlloc(g2) nil")
}
g2 := FilterGraph{ptr: g2ptr}
defer g2.GraphFree(&g2ptr)
segStr, freeSeg := featCStr("scale=320:240")
defer freeSeg()
var seg unsafe.Pointer
if err := g2.GraphSegmentParse(segStr, 0, &seg); err != nil {
t.Fatalf("GraphSegmentParse: %v", err)
}
if seg == nil {
t.Fatal("GraphSegmentParse nil seg")
}
defer g2.GraphSegmentFree(&seg)
if err := g2.GraphSegmentCreateFilters(seg, 0); err != nil {
t.Fatalf("SegmentCreateFilters: %v", err)
}
if err := g2.GraphSegmentApplyOpts(seg, 0); err != nil {
t.Fatalf("SegmentApplyOpts: %v", err)
}
if err := g2.GraphSegmentInit(seg, 0); err != nil {
t.Fatalf("SegmentInit: %v", err)
}
var segIn, segOut unsafe.Pointer
if err := g2.GraphSegmentLink(seg, 0, &segIn, &segOut); err != nil {
t.Fatalf("SegmentLink: %v", err)
}
if segIn != nil {
g2.InoutFree(&segIn)
}
if segOut != nil {
g2.InoutFree(&segOut)
}
var segIn2, segOut2 unsafe.Pointer
if err := g2.GraphSegmentApply(seg, 0, &segIn2, &segOut2); err != nil {
t.Fatalf("SegmentApply: %v", err)
}
if segIn2 != nil {
g2.InoutFree(&segIn2)
}
if segOut2 != nil {
g2.InoutFree(&segOut2)
}
// ---- Inout/LinkFree 空槽守卫 + 野名字解析报错 ----
ioPtr := fg.InoutAlloc()
if ioPtr == nil {
t.Fatal("InoutAlloc nil")
}
fg.InoutFree(&ioPtr)
if ioPtr != nil {
t.Fatal("InoutFree did not null the slot")
}
var nilLink unsafe.Pointer
fg.LinkFree(&nilLink)
bogusParse, freeBogusParse := featCStr("no-such-filter-xyz!!!")
defer freeBogusParse()
g3ptr := fg.GraphAlloc()
if g3ptr == nil {
t.Fatal("GraphAlloc(g3) nil")
}
g3 := FilterGraph{ptr: g3ptr}
defer g3.GraphFree(&g3ptr)
if err := g3.GraphParse(bogusParse, nil, nil, nil); err == nil {
t.Fatal("GraphParse(bogus) accepted")
}
var p2in, p2out unsafe.Pointer
if err := g3.GraphParse2(bogusParse, &p2in, &p2out); err == nil {
t.Fatal("GraphParse2(bogus) accepted")
}
if err := g3.GraphParsePtr(bogusParse, &p2in, &p2out, nil); err == nil {
t.Fatal("GraphParsePtr(bogus) accepted")
}
var bogusSeg unsafe.Pointer
if err := g3.GraphSegmentParse(bogusParse, 0, &bogusSeg); err != nil {
t.Fatalf("GraphSegmentParse(bogus): %v", err)
}
// 解析只记名不验名, 野名字在建滤镜那步才爆 (CreateFilters 回 FILTER_NOT_FOUND).
if bogusSeg == nil {
t.Fatal("GraphSegmentParse(bogus) nil seg")
}
defer g3.GraphSegmentFree(&bogusSeg)
if err := g3.GraphSegmentCreateFilters(bogusSeg, 0); err == nil {
t.Fatal("SegmentCreateFilters(bogus) accepted")
}
// ---- InsertFilter: 第二张小图上真插一个 null 级滤镜再配通 ----
g4ptr := fg.GraphAlloc()
if g4ptr == nil {
t.Fatal("GraphAlloc(g4) nil")
}
g4 := FilterGraph{ptr: g4ptr}
defer g4.GraphFree(&g4ptr)
mk4 := func(kind, name, args string) unsafe.Pointer {
kn, kf := featCStr(kind)
defer kf()
nn, nf := featCStr(name)
defer nf()
f := fg.GetByName(kn)
if f == nil {
t.Fatalf("GetByName(%s) nil", kind)
}
var argPtr unsafe.Pointer
var af func()
if args != "" {
argPtr, af = featCStr(args)
defer af()
}
var ctx unsafe.Pointer
if err := g4.GraphCreateFilter(&ctx, f, nn, argPtr, nil, g4ptr); err != nil {
t.Fatalf("g4 create %s: %v", kind, err)
}
return ctx
}
s4 := mk4("buffer", "s4", "video_size=160x120:pix_fmt=0:time_base=1/25:pixel_aspect=1/1")
e4 := mk4("buffersink", "e4", "")
s4FC := FilterContext{ptr: s4}
if err := s4FC.Link(0, e4, 0); err != nil {
t.Fatalf("g4 link: %v", err)
}
outs := loadPtr(s4, 56)
if outs == nil {
t.Fatal("s4 outputs nil")
}
realLink := *(*unsafe.Pointer)(unsafe.Add(outs, 0))
if realLink == nil {
t.Fatal("s4 link[0] nil")
}
nullFiltName, freeNullName := featCStr("scale")
defer freeNullName()
nullFilt := fg.GetByName(nullFiltName)
if nullFilt == nil {
t.Fatal("GetByName(scale) nil (second check)")
}
nullName, freeNull := featCStr("mid4")
defer freeNull()
var nullCtx unsafe.Pointer
if err := g4.GraphCreateFilter(&nullCtx, nullFilt, nullName, nil, nil, g4ptr); err != nil {
t.Fatalf("g4 create scale2: %v", err)
}
if err := g4.InsertFilter(realLink, nullCtx, 0, 0); err != nil {
t.Fatalf("InsertFilter: %v", err)
}
if got := g4.GraphConfig(nil); got != 0 {
t.Fatalf("g4 GraphConfig after insert = %d", got)
}
// ---- nil-safe: 空 holder 不崩 ----
var nilFG *FilterGraph
if nilFG.GraphAlloc() == nil {
t.Logf("GraphAlloc on nil receiver: nil (C 无图可配)")
}
var nilFC *FilterContext
if err := nilFC.Link(0, nil, 0); err == nil {
t.Fatal("Link(nil ctx) accepted")
}
var nilSink *FilterSink
if nilSink.GetW() != 0 || nilSink.GetH() != 0 || nilSink.GetType() != 0 {
t.Fatal("nil FilterSink not safe")
}
var nilSrc *FilterSource
if err := nilSrc.AddFrame(nil); err == nil {
t.Fatal("AddFrame(nil ctx) accepted")
}
}

// TestWrapCoverDemuxFree fills the leftover demux gap: alloc/free pairs,
// bogus open paths, stream groups on a bare context. C 头核过:
// avformat_close_input 空槽直接回 (可传 nil 槽); avio_closep 必解槽
// (只给真 Open 出来的流); 组加流要求同盒 (跨盒回 EINVAL, 传 nil 崩).
func TestWrapCoverDemuxFree(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var fx FormatContext
// ---- AllocContext + CloseInput: 建裸盒再关, 槽被置空 ----
bare := fx.AllocContext()
if bare == nil {
t.Fatal("AllocContext nil")
}
fx.CloseInput(&bare)
if bare != nil {
t.Fatal("CloseInput did not null the slot")
}
// CloseInput 空槽直接回, 不崩.
var nilSlot unsafe.Pointer
fx.CloseInput(&nilSlot)
// ---- OpenInput/Open2 非法路径 -> 可读报错 ----
bogusPath, freeBogus := featCStr("/no/such/file.mp4")
defer freeBogus()
var badCtx unsafe.Pointer
if err := fx.OpenInput(&badCtx, bogusPath, nil, nil); err == nil {
t.Fatal("OpenInput bogus accepted")
}
var badIO unsafe.Pointer
if err := fx.Open2(&badIO, "/no/such/file.xyz", 2, nil, nil); err == nil {
t.Fatal("Open2 bogus accepted")
}
// ---- AllocOutputContext2: base 无复用器诚实报错, full 建出来记得放 ----
var outCtx unsafe.Pointer
if err := fx.AllocOutputContext2(&outCtx, nil, "mp4", nil); err != nil {
if IsFull() {
t.Fatalf("AllocOutputContext2(mp4) on full: %v", err)
}
t.Logf("AllocOutputContext2(mp4) on base honest error: %v", err)
} else {
if outCtx == nil {
t.Fatal("AllocOutputContext2 success with nil ctx")
}
outFree := FormatContext{ptr: outCtx}
outFree.FreeContext()
}
// ---- 真文件 Open + Closep: 读写流配对, 槽被置空 ----
tmp := t.TempDir() + "/closep.bin"
var fileIO unsafe.Pointer
if err := fx.Open(&fileIO, tmp, 2); err != nil {
t.Fatalf("Open(tmp,write): %v", err)
}
if fileIO == nil {
t.Fatal("Open success with nil ctx")
}
if err := fx.Closep(&fileIO); err != nil {
t.Fatalf("Closep: %v", err)
}
if fileIO != nil {
t.Fatal("Closep did not null the slot")
}
// ---- 流组: 裸盒上建组加流 (同盒才收, InitOutput 真写走 L3/full) ----
grpCtx := fx.AllocContext()
if grpCtx == nil {
t.Fatal("AllocContext(group) nil")
}
gctx := FormatContext{ptr: grpCtx}
defer gctx.FreeContext()
grp := gctx.StreamGroupCreate(3, nil)
if grp == nil {
t.Fatal("StreamGroupCreate(tile_grid) nil")
}
nst := gctx.NewStream(nil)
if nst == nil {
t.Fatal("NewStream(bare ctx) nil")
}
if err := gctx.StreamGroupAddStream(grp, nst); err != nil {
t.Fatalf("StreamGroupAddStream(same ctx): %v", err)
}
// ---- nil 接收器守卫 (调 C 前就拦, 不碰指针) ----
var nilFx *FormatContext
if err := nilFx.InitOutput(nil); err == nil {
t.Fatal("InitOutput(nil ctx) accepted")
}
var nilIO2 *IOContext
if err := nilIO2.Accept(nil); err == nil {
t.Fatal("Accept(nil ctx) accepted")
}
if err := nilIO2.Handshake(); err == nil {
t.Fatal("Handshake(nil ctx) accepted")
}
var nilSt2 *Stream
if nilSt2.IndexGetEntry(0) != nil {
t.Fatal("IndexGetEntry(nil) non-nil")
}
if nilSt2.IndexGetEntryFromTimestamp(0, 0) != nil {
t.Fatal("IndexGetEntryFromTimestamp(nil) non-nil")
}
}

// TestWrapCoverMath pins pure arithmetic bindings to known answers
// (答案照 ffmpeg 源码头文件算: AddQ 通分, CompareMod 见 mathematics.h 例,
// RescaleDelta 首调走 simple_round 分支).
func TestWrapCoverMath(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var mth Math
if got := mth.AddQ(AVRational{1, 2}, AVRational{1, 3}); got != (AVRational{5, 6}) {
t.Fatalf("AddQ = %+v, want {5 6}", got)
}
if got := mth.AddStable(AVRational{1, 25}, 100, AVRational{1, 25}, 1); got != 101 {
t.Fatalf("AddStable = %d, want 101", got)
}
if got := mth.CompareTs(0, AVRational{1, 25}, 1, AVRational{1, 25}); got != -1 {
t.Fatalf("CompareTs = %d, want -1", got)
}
if got := mth.CompareTs(5, AVRational{1, 1}, 5, AVRational{1, 1}); got != 0 {
t.Fatalf("CompareTs equal = %d, want 0", got)
}
if got := mth.CompareMod(0x11, 0x02, 0x10); got >= 0 {
t.Fatalf("CompareMod(0x11,0x02,0x10) = %d, want < 0", got)
}
if got := mth.CompareMod(0x11, 0x02, 0x20); got <= 0 {
t.Fatalf("CompareMod(0x11,0x02,0x20) = %d, want > 0", got)
}
if got := mth.RescaleQ(25, AVRational{1, 25}, AVRational{1, 1000}); got != 1000 {
t.Fatalf("RescaleQ = %d, want 1000", got)
}
if got := mth.RescaleRnd(3, 1, 2, 2); got != 1 {
t.Fatalf("RescaleRnd DOWN = %d, want 1", got)
}
var last int64 = NoPTS
if got := mth.RescaleDelta(AVRational{1, 25}, 100, AVRational{1, 25}, 1, &last, AVRational{1, 1000}); got != 4000 || last != 101 {
t.Fatalf("RescaleDelta = %d last = %d, want 4000/101", got, last)
}
var lib2 Library
if lib2.UtilVersion() == 0 {
t.Fatal("UtilVersion zero")
}
var cpu2 Cpu
if cpu2.MaxAlign() <= 0 {
t.Fatal("MaxAlign not positive")
}
var clk2 Clock
if clk2.NowRelativeUs() <= 0 {
t.Fatal("NowRelativeUs not positive")
}
_ = clk2.IsMonotonic()
}

// TestWrapCoverErrorLog fills the error_log module gap: flags switch,
// RescaleQRnd arithmetic, SleepUs, ForceCount, log-callback install,
// DefaultCallback early-return path, FormatLine/FormatLine2 with a real
// C string + zeroed va_list (no NULL pointers into C).
func TestWrapCoverErrorLog(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var lg Log
var mth Math
var clk Clock
var cpu Cpu
// RescaleQRnd: 3*1/2=1.5, rounding decides 1 or 2
// (AV_ROUND_DOWN=2, UP=3, NEAR_INF=5, 见 mathematics.h).
if got := mth.RescaleQRnd(3, AVRational{1, 2}, AVRational{1, 1}, 2); got != 1 {
t.Fatalf("RescaleQRnd DOWN = %d, want 1", got)
}
if got := mth.RescaleQRnd(3, AVRational{1, 2}, AVRational{1, 1}, 3); got != 2 {
t.Fatalf("RescaleQRnd UP = %d, want 2", got)
}
if got := mth.RescaleQRnd(3, AVRational{1, 2}, AVRational{1, 1}, 5); got != 2 {
t.Fatalf("RescaleQRnd NEAR_INF = %d, want 2", got)
}
if got := mth.RescaleQRnd(25, AVRational{1, 25}, AVRational{1, 1000}, 5); got != 1000 {
t.Fatalf("RescaleQRnd 25fps->ms = %d, want 1000", got)
}
// Flags: bit0 SKIP_REPEATED=1, bit1 PRINT_LEVEL=2 (见 log.h),
// set/get must be sticky, then restore.
origFlags := lg.Flags()
lg.SetFlags(origFlags | 1)
if lg.Flags() != origFlags|1 {
t.Fatalf("SetFlags SKIP_REPEATED not sticky: %d", lg.Flags())
}
lg.SetFlags(origFlags | 2)
if lg.Flags()&2 == 0 {
t.Fatalf("SetFlags PRINT_LEVEL not sticky: %d", lg.Flags())
}
lg.SetFlags(origFlags)
if lg.Flags() != origFlags {
t.Fatalf("SetFlags restore = %d, want %d", lg.Flags(), origFlags)
}
// SleepUs: 0 and 1ms both succeed (av_usleep 回 0 表成功).
if err := clk.SleepUs(0); err != nil {
t.Fatalf("SleepUs(0): %v", err)
}
if err := clk.SleepUs(1000); err != nil {
t.Fatalf("SleepUs(1000): %v", err)
}
// ForceCount: pin to current count, verify, then 0 restores auto-detect
// (cpu.h: Count < 1 disables forcing).
origCount := cpu.Count()
if origCount <= 0 {
t.Fatalf("Cpu.Count = %d", origCount)
}
cpu.ForceCount(origCount)
if cpu.Count() != origCount {
t.Fatalf("ForceCount pinned = %d, want %d", cpu.Count(), origCount)
}
cpu.ForceCount(0)
if cpu.Count() <= 0 {
t.Fatal("ForceCount(0) restore failed")
}
// NewLogCallback: nil -> 0, real func -> nonzero trampoline.
if NewLogCallback(nil) != 0 {
t.Fatal("NewLogCallback(nil) != 0")
}
cb := NewLogCallback(func(ptr unsafe.Pointer, level int32, fmt, msg unsafe.Pointer) {})
if cb == 0 {
t.Fatal("NewLogCallback(func) == 0")
}
// FormatLine/FormatLine2: real C string + zeroed va_list + 1KB line buf.
// fmt 无 % 转换, 零 va_list 不会被解引用; flags 清零免得加 [error] 前缀.
fmtPtr, freeFmt := featCStr("test-log-line\n")
defer freeFmt()
var mem Mem
vaList := mem.AllocZ(32)
if vaList == nil {
t.Fatal("va_list alloc nil")
}
defer mem.Free(vaList)
lineBuf := mem.Alloc(1024)
if lineBuf == nil {
t.Fatal("line buf alloc nil")
}
defer mem.Free(lineBuf)
lg.SetFlags(0)
var prefix int32 = 1
if got := lg.FormatLine2(nil, LogError, fmtPtr, vaList, lineBuf, 1024, &prefix); got != 14 {
lg.SetFlags(origFlags)
t.Fatalf("FormatLine2 = %d, want 14", got)
}
lg.SetFlags(origFlags)
if prefix != 1 {
t.Fatalf("FormatLine2 prefix = %d, want 1 (ends with newline)", prefix)
}
if got := cstr(lineBuf); got != "test-log-line\n" {
t.Fatalf("FormatLine2 line = %q", got)
}
var prefix2 int32 = 1
lg.FormatLine(nil, LogError, fmtPtr, vaList, lineBuf, 1024, &prefix2)
if got := cstr(lineBuf); got != "test-log-line\n" {
t.Fatalf("FormatLine line = %q", got)
}
// DefaultCallback early-return path: level 100 > LogError(16),
// C 直接返回不碰 fmt/args, 传 nil 安全; 低 level 传 nil 会崩, 不测.
oldLevel := lg.Level()
lg.SetLevel(LogError)
lg.DefaultCallback(nil, 100, nil, nil)
lg.SetLevel(oldLevel)
// Callback install/uninstall must not crash; no log fires in between
// so the no-op Go trampoline is never entered.
lg.SetGoCallback(cb)
lg.SetGoCallback(0)
lg.SetCallback(nil)
// Restore default output so later tests keep stderr logging.
if addr, err := purego.Dlsym(libHandle, "av_log_default_callback"); err == nil && addr != 0 {
lg.SetCallback(*(*unsafe.Pointer)(unsafe.Pointer(&addr)))
}
}

// TestWrapCoverMediaDesc fills the media_desc module gap: every pure query
// pinned to C-header answers (pixdesc.h/channel_layout.h/parseutils.h/
// timecode.h/display.h/spherical.c/stereo3d.c). Layout/display/parse/out-param
// functions get real buffers, never NULL (NULL segfaults inside ffmpeg);
// allocs pair with Mem.Free, side-data creators get a real NewFrame.
func TestWrapCoverMediaDesc(t *testing.T) {
if !Available() {
t.Skipf("lib missing: %s", LibPath())
}
var md MediaDesc
var mem Mem
// ---- pix/color pure queries (pixdesc.h/pixdesc.c) ----
if got := md.GetPixFmt("yuv420p"); got != 0 {
t.Fatalf("GetPixFmt(yuv420p) = %d, want 0", got)
}
if got := md.GetPixFmt("no-such-fmt-xyz"); got != -1 {
t.Fatalf("GetPixFmt bogus = %d, want -1 (AV_PIX_FMT_NONE)", got)
}
if got := cstr(md.GetPixFmtName(0)); got != "yuv420p" {
t.Fatalf("GetPixFmtName(0) = %q, want yuv420p", got)
}
if md.GetPixFmtName(-999) != nil {
t.Fatal("GetPixFmtName(-999) non-nil")
}
if got := md.GetPixFmtLoss(0, 0, 0); got != 0 {
t.Fatalf("GetPixFmtLoss same = %d, want 0", got)
}
desc420 := md.PixFmtDescGet(0)
if desc420 == nil {
t.Fatal("PixFmtDescGet(0) nil")
}
if md.PixFmtDescGet(-999) != nil {
t.Fatal("PixFmtDescGet(-999) non-nil")
}
if got := md.PixFmtDescGetId(desc420); got != 0 {
t.Fatalf("PixFmtDescGetId = %d, want 0", got)
}
if md.PixFmtDescNext(nil) == nil {
t.Fatal("PixFmtDescNext(nil) nil (want first descriptor)")
}
strBuf := mem.Alloc(64)
if strBuf == nil {
t.Fatal("pixfmt string buf nil")
}
defer mem.Free(strBuf)
if md.GetPixFmtString(strBuf, 64, 0) == nil {
t.Fatal("GetPixFmtString nil")
}
if got := cstr(strBuf); !strings.Contains(got, "yuv420p") {
t.Fatalf("GetPixFmtString = %q, want contains yuv420p", got)
}
if got := md.GetBitsPerPixel(desc420); got != 12 {
t.Fatalf("GetBitsPerPixel(yuv420p) = %d, want 12", got)
}
descRGB := md.PixFmtDescGet(2)
if descRGB == nil {
t.Fatal("PixFmtDescGet(2) nil")
}
if got := md.GetBitsPerPixel(descRGB); got != 24 {
t.Fatalf("GetBitsPerPixel(rgb24) = %d, want 24", got)
}
if got := md.ColorRangeFromName("tv"); got != 1 {
t.Fatalf("ColorRangeFromName(tv) = %d, want 1 (MPEG)", got)
}
if got := md.ColorRangeFromName("no-such-range-xyz"); got >= 0 {
t.Fatalf("ColorRangeFromName bogus = %d, want < 0", got)
}
if got := cstr(md.ColorRangeName(1)); got != "tv" {
t.Fatalf("ColorRangeName(1) = %q, want tv", got)
}
if md.ColorRangeName(999) != nil {
t.Fatal("ColorRangeName(999) non-nil")
}
priStr, freePri := featCStr("bt709")
defer freePri()
if got := md.ColorPrimariesFromName(priStr); got != 1 {
t.Fatalf("ColorPrimariesFromName(bt709) = %d, want 1", got)
}
bogusPri, freeBogusPri := featCStr("no-such-primaries-xyz")
defer freeBogusPri()
if got := md.ColorPrimariesFromName(bogusPri); got >= 0 {
t.Fatalf("ColorPrimariesFromName bogus = %d, want < 0", got)
}
if got := cstr(md.ColorPrimariesName(1)); got != "bt709" {
t.Fatalf("ColorPrimariesName(1) = %q, want bt709", got)
}
if got := md.ColorSpaceFromName("bt709"); got != 1 {
t.Fatalf("ColorSpaceFromName(bt709) = %d, want 1", got)
}
if got := md.ColorSpaceFromName("no-such-space-xyz"); got >= 0 {
t.Fatalf("ColorSpaceFromName bogus = %d, want < 0", got)
}
if got := cstr(md.ColorSpaceName(1)); got != "bt709" {
t.Fatalf("ColorSpaceName(1) = %q, want bt709", got)
}
trcStr, freeTrc := featCStr("bt709")
defer freeTrc()
if got := md.ColorTransferFromName(trcStr); got != 1 {
t.Fatalf("ColorTransferFromName(bt709) = %d, want 1", got)
}
bogusTrc, freeBogusTrc := featCStr("no-such-trc-xyz")
defer freeBogusTrc()
if got := md.ColorTransferFromName(bogusTrc); got >= 0 {
t.Fatalf("ColorTransferFromName bogus = %d, want < 0", got)
}
if got := cstr(md.ColorTransferName(1)); got != "bt709" {
t.Fatalf("ColorTransferName(1) = %q, want bt709", got)
}
if md.ColorTransferName(999) != nil {
t.Fatal("ColorTransferName(999) non-nil")
}
// ---- single channel name/description (channel_layout.h) ----
nameBuf := mem.Alloc(32)
if nameBuf == nil {
t.Fatal("channel name buf nil")
}
defer mem.Free(nameBuf)
if got := md.ChannelName(nameBuf, 32, 0); got <= 0 {
t.Fatalf("ChannelName(FL) = %d, want > 0", got)
}
if got := cstr(nameBuf); got != "FL" {
t.Fatalf("ChannelName(FL) = %q, want FL", got)
}
if n, err := md.ChannelDescription(nameBuf, 32, 0); err != nil || n <= 0 {
t.Fatalf("ChannelDescription(FL) = (%d, %v)", n, err)
}
flStr, freeFL := featCStr("FL")
defer freeFL()
if got := md.ChannelFromString(flStr); got != 0 {
t.Fatalf("ChannelFromString(FL) = %d, want 0", got)
}
bp := NewBPrint(64, 1024)
if bp == nil {
t.Fatal("NewBPrint nil")
}
defer bp.Free()
md.ChannelNameBprint(bp.Ptr(), 0)
md.ChannelDescriptionBprint(bp.Ptr(), 0)
fin, err := bp.Finalize()
if err != nil {
t.Fatalf("BPrint Finalize: %v", err)
}
defer mem.Free(fin)
if got := cstr(fin); !strings.Contains(got, "FL") {
t.Fatalf("BPrint channel names = %q, want contains FL", got)
}
// ---- layout lifecycle (AVChannelLayout = 24 bytes, 用 32 字节零内存) ----
layout := mem.AllocZ(32)
if layout == nil {
t.Fatal("layout alloc nil")
}
defer mem.Free(layout)
md.ChannelLayoutDefault(layout, 2)
if err := md.ChannelLayoutCheck(layout); err != nil {
t.Fatalf("Default(2) Check: %v", err)
}
descBuf := mem.Alloc(64)
if descBuf == nil {
t.Fatal("layout describe buf nil")
}
defer mem.Free(descBuf)
if n, err := md.ChannelLayoutDescribe(layout, descBuf, 64); err != nil || n <= 0 {
t.Fatalf("Describe(stereo) = (%d, %v)", n, err)
}
if got := cstr(descBuf); got != "stereo" {
t.Fatalf("Describe(stereo) = %q, want stereo", got)
}
layoutCopy := mem.AllocZ(32)
if layoutCopy == nil {
t.Fatal("layout copy alloc nil")
}
defer mem.Free(layoutCopy)
if err := md.ChannelLayoutCopy(layoutCopy, layout); err != nil {
t.Fatalf("ChannelLayoutCopy: %v", err)
}
if eq, err := md.ChannelLayoutCompare(layout, layoutCopy); err != nil || eq != 0 {
t.Fatalf("Compare same = (%d, %v), want (0, nil)", eq, err)
}
monoLayout := mem.AllocZ(32)
if monoLayout == nil {
t.Fatal("mono layout alloc nil")
}
defer mem.Free(monoLayout)
md.ChannelLayoutDefault(monoLayout, 1)
if eq, err := md.ChannelLayoutCompare(layout, monoLayout); err != nil || eq != 1 {
t.Fatalf("Compare stereo/mono = (%d, %v), want (1, nil)", eq, err)
}
if idx, err := md.ChannelLayoutIndexFromChannel(layout, 0); err != nil || idx != 0 {
t.Fatalf("IndexFromChannel(FL) = (%d, %v), want (0, nil)", idx, err)
}
if _, err := md.ChannelLayoutIndexFromChannel(layout, 2); err == nil {
t.Fatal("IndexFromChannel(FC in stereo) accepted")
}
if idx, err := md.ChannelLayoutIndexFromString(layout, flStr); err != nil || idx != 0 {
t.Fatalf("IndexFromString(FL) = (%d, %v), want (0, nil)", idx, err)
}
bogusChan, freeBogusChan := featCStr("no-such-channel-xyz")
defer freeBogusChan()
if _, err := md.ChannelLayoutIndexFromString(layout, bogusChan); err == nil {
t.Fatal("IndexFromString bogus accepted")
}
if got := md.ChannelLayoutChannelFromIndex(layout, 0); got != 0 {
t.Fatalf("ChannelFromIndex(0) = %d, want 0 (FL)", got)
}
if got := md.ChannelLayoutChannelFromIndex(layout, 99); got != -1 {
t.Fatalf("ChannelFromIndex(99) = %d, want -1", got)
}
if got := md.ChannelLayoutChannelFromString(layout, flStr); got != 0 {
t.Fatalf("ChannelFromString(layout,FL) = %d, want 0", got)
}
if got := md.ChannelLayoutChannelFromString(layout, bogusChan); got != -1 {
t.Fatalf("ChannelFromString(layout,bogus) = %d, want -1", got)
}
if got := md.ChannelLayoutSubset(layout, 1); got != 1 {
t.Fatalf("Subset(FL) = %d, want 1", got)
}
if _, err := md.ChannelLayoutAmbisonicOrder(layout); err == nil {
t.Fatal("AmbisonicOrder(stereo) accepted")
}
bp2 := NewBPrint(64, 1024)
if bp2 == nil {
t.Fatal("NewBPrint2 nil")
}
defer bp2.Free()
if err := md.ChannelLayoutDescribeBprint(layout, bp2.Ptr()); err != nil {
t.Fatalf("DescribeBprint: %v", err)
}
maskLayout := mem.AllocZ(32)
if maskLayout == nil {
t.Fatal("mask layout alloc nil")
}
defer mem.Free(maskLayout)
if err := md.ChannelLayoutFromMask(maskLayout, 3); err != nil {
t.Fatalf("FromMask(3): %v", err)
}
if err := md.ChannelLayoutCheck(maskLayout); err != nil {
t.Fatalf("FromMask Check: %v", err)
}
if err := md.ChannelLayoutFromMask(maskLayout, 0); err == nil {
t.Fatal("FromMask(0) accepted")
}
strLayout := mem.AllocZ(32)
if strLayout == nil {
t.Fatal("str layout alloc nil")
}
defer mem.Free(strLayout)
stereoStr, freeStereoStr := featCStr("stereo")
defer freeStereoStr()
if err := md.ChannelLayoutFromString(strLayout, stereoStr); err != nil {
t.Fatalf("FromString(stereo): %v", err)
}
bogusLayoutStr, freeBogusLayoutStr := featCStr("no-such-layout-xyz")
defer freeBogusLayoutStr()
badLayout := mem.AllocZ(32)
if badLayout == nil {
t.Fatal("bad layout alloc nil")
}
defer mem.Free(badLayout)
if err := md.ChannelLayoutFromString(badLayout, bogusLayoutStr); err == nil {
t.Fatal("FromString bogus accepted")
}
var opaque unsafe.Pointer
if md.ChannelLayoutStandard(&opaque) == nil {
t.Fatal("ChannelLayoutStandard first nil")
}
retypeLayout := mem.AllocZ(32)
if retypeLayout == nil {
t.Fatal("retype layout alloc nil")
}
defer mem.Free(retypeLayout)
md.ChannelLayoutDefault(retypeLayout, 2)
if r, err := md.ChannelLayoutRetype(retypeLayout, 2, 0); err != nil || r != 0 {
t.Fatalf("Retype(native->custom) = (%d, %v), want (0, nil)", r, err)
}
md.ChannelLayoutUninit(retypeLayout)
customLayout := mem.AllocZ(32)
if customLayout == nil {
t.Fatal("custom layout alloc nil")
}
defer mem.Free(customLayout)
if err := md.ChannelLayoutCustomInit(customLayout, 2); err != nil {
t.Fatalf("CustomInit(2): %v", err)
}
md.ChannelLayoutUninit(customLayout)
if err := md.ChannelLayoutCheck(customLayout); err == nil {
t.Fatal("Check(uninitialized) accepted")
}
md.ChannelLayoutUninit(layout)
md.ChannelLayoutUninit(layoutCopy)
md.ChannelLayoutUninit(monoLayout)
md.ChannelLayoutUninit(maskLayout)
md.ChannelLayoutUninit(strLayout)
// ---- parse utils (parseutils.h; out-param 全给真内存) ----
rgba := mem.Alloc(4)
if rgba == nil {
t.Fatal("rgba buf nil")
}
defer mem.Free(rgba)
redStr, freeRed := featCStr("red")
defer freeRed()
if err := md.ParseColor(rgba, redStr, -1, nil); err != nil {
t.Fatalf("ParseColor(red): %v", err)
}
if got := unsafe.Slice((*byte)(rgba), 4); got[0] != 255 || got[1] != 0 || got[2] != 0 || got[3] != 255 {
t.Fatalf("ParseColor(red) = %v, want [255 0 0 255]", got)
}
bogusColor, freeBogusColor := featCStr("no-such-color-xyz")
defer freeBogusColor()
if err := md.ParseColor(rgba, bogusColor, -1, nil); err == nil {
t.Fatal("ParseColor bogus accepted")
}
qbuf := mem.Alloc(8)
if qbuf == nil {
t.Fatal("ratio buf nil")
}
defer mem.Free(qbuf)
ratioStr, freeRatio := featCStr("1:2")
defer freeRatio()
if err := md.ParseRatio(qbuf, ratioStr, 1001000, 0, nil); err != nil {
t.Fatalf("ParseRatio(1:2): %v", err)
}
if got := *(*AVRational)(qbuf); got != (AVRational{1, 2}) {
t.Fatalf("ParseRatio(1:2) = %+v, want {1 2}", got)
}
bogusRatio, freeBogusRatio := featCStr("no-such-ratio-xyz")
defer freeBogusRatio()
if err := md.ParseRatio(qbuf, bogusRatio, 1001000, 0, nil); err == nil {
t.Fatal("ParseRatio bogus accepted")
}
tvbuf := mem.Alloc(8)
if tvbuf == nil {
t.Fatal("timeval buf nil")
}
defer mem.Free(tvbuf)
durStr, freeDur := featCStr("00:01:00")
defer freeDur()
if err := md.ParseTime(tvbuf, durStr, 1); err != nil {
t.Fatalf("ParseTime(1min): %v", err)
}
if got := *(*int64)(tvbuf); got != 60000000 {
t.Fatalf("ParseTime(1min) = %d, want 60000000", got)
}
bogusTime, freeBogusTime := featCStr("no-such-time-xyz")
defer freeBogusTime()
if err := md.ParseTime(tvbuf, bogusTime, 1); err == nil {
t.Fatal("ParseTime bogus accepted")
}
ratebuf := mem.Alloc(8)
if ratebuf == nil {
t.Fatal("rate buf nil")
}
defer mem.Free(ratebuf)
fpsStr, freeFps := featCStr("25")
defer freeFps()
if err := md.ParseVideoRate(ratebuf, fpsStr); err != nil {
t.Fatalf("ParseVideoRate(25): %v", err)
}
if got := *(*AVRational)(ratebuf); got != (AVRational{25, 1}) {
t.Fatalf("ParseVideoRate(25) = %+v, want {25 1}", got)
}
bogusRate, freeBogusRate := featCStr("no-such-rate-xyz")
defer freeBogusRate()
if err := md.ParseVideoRate(ratebuf, bogusRate); err == nil {
t.Fatal("ParseVideoRate bogus accepted")
}
wbuf := mem.Alloc(4)
hbuf := mem.Alloc(4)
if wbuf == nil || hbuf == nil {
t.Fatal("video size bufs nil")
}
defer mem.Free(wbuf)
defer mem.Free(hbuf)
sizeStr, freeSize := featCStr("640x480")
defer freeSize()
if err := md.ParseVideoSize(wbuf, hbuf, sizeStr); err != nil {
t.Fatalf("ParseVideoSize(640x480): %v", err)
}
if *(*int32)(wbuf) != 640 || *(*int32)(hbuf) != 480 {
t.Fatalf("ParseVideoSize = %dx%d, want 640x480", *(*int32)(wbuf), *(*int32)(hbuf))
}
bogusSize, freeBogusSize := featCStr("no-such-size-xyz")
defer freeBogusSize()
if err := md.ParseVideoSize(wbuf, hbuf, bogusSize); err == nil {
t.Fatal("ParseVideoSize bogus accepted")
}
// ---- timecode (timecode.h/timecode.c) ----
if err := md.TimecodeCheckFrameRate(AVRational{25, 1}); err != nil {
t.Fatalf("CheckFrameRate(25): %v", err)
}
if err := md.TimecodeCheckFrameRate(AVRational{7, 1}); err == nil {
t.Fatal("CheckFrameRate(7fps) accepted")
}
if got := md.TimecodeAdjustNtscFramenum2(100, 30); got != 100 {
t.Fatalf("Adjust(100,30) = %d, want 100", got)
}
if got := md.TimecodeAdjustNtscFramenum2(50, 25); got != 50 {
t.Fatalf("Adjust passthrough = %d, want 50", got)
}
if got := md.TimecodeGetSmpte(AVRational{25, 1}, 0, 1, 2, 3, 4); got != 0x04030201 {
t.Fatalf("GetSmpte(01:02:03:04) = %#x, want 0x04030201", got)
}
tcbuf := mem.Alloc(32)
if tcbuf == nil {
t.Fatal("timecode buf nil")
}
defer mem.Free(tcbuf)
if err := md.TimecodeInit(tcbuf, AVRational{25, 1}, 0, 0, nil); err != nil {
t.Fatalf("TimecodeInit: %v", err)
}
if got := md.TimecodeGetSmpteFromFramenum(tcbuf, 0); got != 0 {
t.Fatalf("GetSmpteFromFramenum(0) = %#x, want 0", got)
}
tcstr := mem.Alloc(32)
if tcstr == nil {
t.Fatal("timecode str buf nil")
}
defer mem.Free(tcstr)
if md.TimecodeMakeString(tcbuf, tcstr, 0) == nil {
t.Fatal("TimecodeMakeString nil")
}
if got := cstr(tcstr); got != "00:00:00:00" {
t.Fatalf("TimecodeMakeString(0) = %q, want 00:00:00:00", got)
}
if md.TimecodeMakeSmpteTcString(tcstr, 0x04030201, 0) == nil {
t.Fatal("MakeSmpteTcString nil")
}
if md.TimecodeMakeSmpteTcString2(tcstr, AVRational{25, 1}, 0x04030201, 0, 0) == nil {
t.Fatal("MakeSmpteTcString2 nil")
}
if md.TimecodeMakeMpegTcString(tcstr, 0) == nil {
t.Fatal("MakeMpegTcString nil")
}
if got := cstr(tcstr); got != "00:00:00:00" {
t.Fatalf("MakeMpegTcString(0) = %q, want 00:00:00:00", got)
}
tc2 := mem.Alloc(32)
if tc2 == nil {
t.Fatal("timecode2 buf nil")
}
defer mem.Free(tc2)
if err := md.TimecodeInitFromComponents(tc2, AVRational{25, 1}, 0, 1, 2, 3, 4, nil); err != nil {
t.Fatalf("InitFromComponents: %v", err)
}
tc3 := mem.Alloc(32)
if tc3 == nil {
t.Fatal("timecode3 buf nil")
}
defer mem.Free(tc3)
tcSrc, freeTcSrc := featCStr("01:02:03:04")
defer freeTcSrc()
if err := md.TimecodeInitFromString(tc3, AVRational{25, 1}, tcSrc, nil); err != nil {
t.Fatalf("InitFromString: %v", err)
}
bogusTc, freeBogusTc := featCStr("no-such-timecode-xyz")
defer freeBogusTc()
if err := md.TimecodeInitFromString(tc3, AVRational{25, 1}, bogusTc, nil); err == nil {
t.Fatal("InitFromString bogus accepted")
}
// ---- display matrix (display.h; 9xint32 = 36 字节真矩阵) ----
matrix := mem.Alloc(36)
if matrix == nil {
t.Fatal("matrix alloc nil")
}
defer mem.Free(matrix)
md.DisplayRotationSet(matrix, 90.0)
if got := md.DisplayRotationGet(matrix); got < -90.1 || got > -89.9 {
t.Fatalf("RotationGet(90 clockwise) = %f, want ~-90 (get 读的是逆时针角)", got)
}
md.DisplayMatrixFlip(matrix, 1, 0)
_ = md.DisplayRotationGet(matrix)
// ---- metadata allocs (size 传 nil 表不回写, 回来记得 Free) ----
if md.AmbientViewingEnvironmentAlloc(nil) == nil {
t.Fatal("AmbientViewingEnvironmentAlloc nil")
} else {
mem.Free(md.AmbientViewingEnvironmentAlloc(nil))
}
if md.ContentLightMetadataAlloc(nil) == nil {
t.Fatal("ContentLightMetadataAlloc nil")
} else {
mem.Free(md.ContentLightMetadataAlloc(nil))
}
doviMeta := md.DoviMetadataAlloc(nil)
if doviMeta == nil {
t.Fatal("DoviMetadataAlloc nil")
}
defer mem.Free(doviMeta)
if md.DoviAlloc(nil) == nil {
t.Fatal("DoviAlloc nil")
} else {
mem.Free(md.DoviAlloc(nil))
}
if md.DoviFindLevel(doviMeta, 0) != nil {
t.Fatal("DoviFindLevel(zeroed) non-nil")
}
if md.DynamicHdrPlusAlloc(nil) == nil {
t.Fatal("DynamicHdrPlusAlloc nil")
} else {
mem.Free(md.DynamicHdrPlusAlloc(nil))
}
if md.FilmGrainParamsAlloc(nil) == nil {
t.Fatal("FilmGrainParamsAlloc nil")
} else {
mem.Free(md.FilmGrainParamsAlloc(nil))
}
if md.MasteringDisplayMetadataAlloc() == nil {
t.Fatal("MasteringDisplayMetadataAlloc nil")
} else {
mem.Free(md.MasteringDisplayMetadataAlloc())
}
if md.MasteringDisplayMetadataAllocSize(nil) == nil {
t.Fatal("MasteringDisplayMetadataAllocSize nil")
} else {
mem.Free(md.MasteringDisplayMetadataAllocSize(nil))
}
if md.SphericalAlloc(nil) == nil {
t.Fatal("SphericalAlloc nil")
} else {
mem.Free(md.SphericalAlloc(nil))
}
if md.Stereo3dAlloc() == nil {
t.Fatal("Stereo3dAlloc nil")
} else {
mem.Free(md.Stereo3dAlloc())
}
if md.Stereo3dAllocSize(nil) == nil {
t.Fatal("Stereo3dAllocSize nil")
} else {
mem.Free(md.Stereo3dAllocSize(nil))
}
// ---- side-data creators (须是真 Frame, 传 nil 会崩) ----
fr := NewFrame()
if fr == nil {
t.Fatal("NewFrame nil")
}
defer fr.Free()
if md.AmbientViewingEnvironmentCreateSideData(fr.Ptr()) == nil {
t.Fatal("AmbientCreateSideData nil")
}
if md.ContentLightMetadataCreateSideData(fr.Ptr()) == nil {
t.Fatal("ContentLightCreateSideData nil")
}
if md.DynamicHdrPlusCreateSideData(fr.Ptr()) == nil {
t.Fatal("DynamicHdrPlusCreateSideData nil")
}
if md.FilmGrainParamsCreateSideData(fr.Ptr()) == nil {
t.Fatal("FilmGrainParamsCreateSideData nil")
}
// 空白帧没设像素格式, Select 诚实回 nil (要是真解出来的帧才选得出).
if md.FilmGrainParamsSelect(fr.Ptr()) != nil {
t.Fatal("FilmGrainParamsSelect(blank frame) non-nil")
}
if md.MasteringDisplayMetadataCreateSideData(fr.Ptr()) == nil {
t.Fatal("MasteringCreateSideData nil")
}
if md.Stereo3dCreateSideData(fr.Ptr()) == nil {
t.Fatal("Stereo3dCreateSideData nil")
}
// ---- HDR10+ T.35 双向 (hdr_dynamic_metadata.h) ----
hdrPlus := md.DynamicHdrPlusAlloc(nil)
if hdrPlus == nil {
t.Fatal("hdrPlus alloc nil")
}
defer mem.Free(hdrPlus)
t35bogus := mem.Alloc(4)
if t35bogus == nil {
t.Fatal("t35 buf nil")
}
defer mem.Free(t35bogus)
if err := md.DynamicHdrPlusFromT35(hdrPlus, t35bogus, 4); err == nil {
t.Fatal("FromT35(zeroes) accepted")
}
var t35out unsafe.Pointer
sizeBuf := mem.Alloc(8)
if sizeBuf == nil {
t.Fatal("t35 size buf nil")
}
defer mem.Free(sizeBuf)
// 全零结构体的分数分母全是 0, C 里做 num/den 除法直接崩,
// ToT35 这条路只验 data=nil/size=nil 的参数守卫 (两条都回 EINVAL),
// 不调全零结构体的真正序列化.
if _, err := md.DynamicHdrPlusToT35(nil, &t35out, sizeBuf); err == nil {
t.Fatal("ToT35(nil) accepted")
}
t35zero := mem.Alloc(4)
if t35zero == nil {
t.Fatal("t35zero buf nil")
}
defer mem.Free(t35zero)
if _, err := md.DynamicHdrPlusToT35(hdrPlus, &t35zero, nil); err == nil {
t.Fatal("ToT35(size=nil, buf!=nil) accepted")
}
// ---- spherical / stereo3d 名表 (spherical.c/stereo3d.c) ----
sphStr, freeSph := featCStr("equirectangular")
defer freeSph()
if got := md.SphericalFromName(sphStr); got != 0 {
t.Fatalf("SphericalFromName(equirectangular) = %d, want 0", got)
}
bogusSph, freeBogusSph := featCStr("no-such-projection-xyz")
defer freeBogusSph()
if got := md.SphericalFromName(bogusSph); got != -1 {
t.Fatalf("SphericalFromName bogus = %d, want -1", got)
}
if got := cstr(md.SphericalProjectionName(0)); got != "equirectangular" {
t.Fatalf("SphericalProjectionName(0) = %q", got)
}
if got := cstr(md.SphericalProjectionName(99)); got != "unknown" {
t.Fatalf("SphericalProjectionName(99) = %q, want unknown", got)
}
sphMap := mem.AllocZ(128)
if sphMap == nil {
t.Fatal("spherical map alloc nil")
}
defer mem.Free(sphMap)
leftB := mem.Alloc(8)
topB := mem.Alloc(8)
rightB := mem.Alloc(8)
bottomB := mem.Alloc(8)
if leftB == nil || topB == nil || rightB == nil || bottomB == nil {
t.Fatal("tile bounds bufs nil")
}
defer mem.Free(leftB)
defer mem.Free(topB)
defer mem.Free(rightB)
defer mem.Free(bottomB)
md.SphericalTileBounds(sphMap, 640, 480, leftB, topB, rightB, bottomB)
if *(*uintptr)(leftB) != 0 || *(*uintptr)(topB) != 0 || *(*uintptr)(rightB) != 0 || *(*uintptr)(bottomB) != 0 {
t.Fatal("SphericalTileBounds(zeroed) nonzero")
}
sbsStr, freeSbs := featCStr("side by side")
defer freeSbs()
if got := md.Stereo3dFromName(sbsStr); got != 1 {
t.Fatalf("Stereo3dFromName(side by side) = %d, want 1", got)
}
bogusS3d, freeBogusS3d := featCStr("no-such-stereo-xyz")
defer freeBogusS3d()
if got := md.Stereo3dFromName(bogusS3d); got != -1 {
t.Fatalf("Stereo3dFromName bogus = %d, want -1", got)
}
if got := cstr(md.Stereo3dTypeName(1)); got != "side by side" {
t.Fatalf("Stereo3dTypeName(1) = %q", got)
}
if got := cstr(md.Stereo3dTypeName(999)); got != "unknown" {
t.Fatalf("Stereo3dTypeName(999) = %q, want unknown", got)
}
leftView, freeLeftView := featCStr("left")
defer freeLeftView()
if got := md.Stereo3dViewFromName(leftView); got != 1 {
t.Fatalf("Stereo3dViewFromName(left) = %d, want 1", got)
}
bogusView, freeBogusView := featCStr("no-such-view-xyz")
defer freeBogusView()
if got := md.Stereo3dViewFromName(bogusView); got != -1 {
t.Fatalf("Stereo3dViewFromName bogus = %d, want -1", got)
}
if got := cstr(md.Stereo3dViewName(1)); got != "left" {
t.Fatalf("Stereo3dViewName(1) = %q, want left", got)
}
if got := md.Stereo3dPrimaryEyeFromName(leftView); got != 1 {
t.Fatalf("Stereo3dPrimaryEyeFromName(left) = %d, want 1", got)
}
bogusEye, freeBogusEye := featCStr("no-such-eye-xyz")
defer freeBogusEye()
if got := md.Stereo3dPrimaryEyeFromName(bogusEye); got != -1 {
t.Fatalf("Stereo3dPrimaryEyeFromName bogus = %d, want -1", got)
}
if got := cstr(md.Stereo3dPrimaryEyeName(1)); got != "left" {
t.Fatalf("Stereo3dPrimaryEyeName(1) = %q, want left", got)
}
// ---- nil-safe: 空 holder 调 Ptr 回 nil 不崩 ----
var nilMd *MediaDesc
if nilMd.Ptr() != nil {
t.Fatal("nil MediaDesc Ptr non-nil")
}
}
