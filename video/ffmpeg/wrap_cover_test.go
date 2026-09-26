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
	_ = unsafe.Pointer(nil)
}
