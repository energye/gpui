package ffmpeg

import (
	"testing"
	"unsafe"
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
