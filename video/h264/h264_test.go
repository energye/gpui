package h264

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writer builds RBSP payloads bit by bit for round-trip tests.
type writer struct {
	buf []byte
	cur uint8
	n   int
}

func (w *writer) bit(b uint32) {
	w.cur = (w.cur << 1) | uint8(b&1)
	w.n++
	if w.n == 8 {
		w.buf = append(w.buf, w.cur)
		w.cur = 0
		w.n = 0
	}
}

func (w *writer) bits(v uint32, n int) {
	for i := n - 1; i >= 0; i-- {
		w.bit((v >> uint(i)) & 1)
	}
}

func (w *writer) ue(v uint32) {
	n := uint32(0)
	t := v + 1
	for t >>= 1; t != 0; t >>= 1 {
		n++
	}
	for i := uint32(0); i < n; i++ {
		w.bit(0)
	}
	w.bit(1)
	if n > 0 {
		w.bits(v+1-(1<<n), int(n))
	}
}

func (w *writer) se(v int32) {
	if v <= 0 {
		w.ue(uint32(-v) * 2)
	} else {
		w.ue(uint32(v)*2 - 1)
	}
}

func (w *writer) flush() []byte {
	w.bit(1)
	for w.n != 0 {
		w.bit(0)
	}
	return append([]byte(nil), w.buf...)
}

type spsOpt struct {
	profile uint8
	level   uint8
	spsID   uint32
	chroma  uint32
	wMBs    uint32
	hMap    uint32
	pocType uint32
	crop    []uint32
}

func buildSPS(o spsOpt) []byte {
	w := &writer{}
	w.bits(uint32(o.profile), 8)
	w.bits(0, 8)
	w.bits(uint32(o.level), 8)
	w.ue(o.spsID)
	if isHighFamily(o.profile) {
		w.ue(o.chroma)
		w.ue(0)
		w.ue(0)
		w.bit(0)
		w.bit(0)
	}
	w.ue(0)
	w.ue(o.pocType)
	if o.pocType == 0 {
		w.ue(0)
	} else if o.pocType == 1 {
		w.bit(0)
		w.se(0)
		w.se(0)
		w.ue(0)
	}
	w.ue(1)
	w.bit(0)
	w.ue(o.wMBs)
	w.ue(o.hMap)
	w.bit(1)
	w.bit(1)
	if len(o.crop) == 4 {
		w.bit(1)
		for _, c := range o.crop {
			w.ue(c)
		}
	} else {
		w.bit(0)
	}
	w.bit(0)
	return append([]byte{0x67}, w.flush()...)
}

type ppsOpt struct {
	ppsID  uint32
	spsID  uint32
	cabac  bool
	groups uint32
	ext    bool
}

func buildPPS(o ppsOpt) []byte {
	w := &writer{}
	w.ue(o.ppsID)
	w.ue(o.spsID)
	w.bit(boolBit(o.cabac))
	w.bit(0)
	w.ue(o.groups)
	w.ue(0)
	w.ue(1)
	w.bit(0)
	w.bits(0, 2)
	w.se(-26 + 26)
	w.se(0)
	w.se(0)
	w.bit(1)
	w.bit(0)
	w.bit(0)
	if o.ext {
		w.bit(1)
		w.bit(0)
		w.se(0)
	}
	return append([]byte{0x68}, w.flush()...)
}

func boolBit(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

func buildSlice(typ int, refIDC int, firstMB uint32) []byte {
	w := &writer{}
	w.ue(firstMB)
	hdr := byte(refIDC<<5 | typ)
	return append([]byte{hdr}, w.flush()...)
}

func TestBitReaderUESE(t *testing.T) {
	cases := []struct {
		bits string
		ue   uint32
		se   int32
	}{
		{"1", 0, 0},
		{"010", 1, 1},
		{"011", 2, -1},
		{"00100", 3, 2},
		{"00101", 4, -2},
		{"00110", 5, 3},
		{"00111", 6, -3},
		{"0001000", 7, 4},
	}
	for _, c := range cases {
		var buf []byte
		var cur uint8
		n := 0
		for _, ch := range c.bits {
			cur = (cur << 1)
			if ch == '1' {
				cur |= 1
			}
			n++
			if n == 8 {
				buf = append(buf, cur)
				cur = 0
				n = 0
			}
		}
		if n > 0 {
			cur <<= uint(8 - n)
			buf = append(buf, cur)
		}
		r := NewReader(buf)
		if v, err := r.ReadUE(); err != nil || v != c.ue {
			t.Fatalf("ue %s = %d,%v want %d", c.bits, v, err, c.ue)
		}
		r = NewReader(buf)
		if v, err := r.ReadSE(); err != nil || v != c.se {
			t.Fatalf("se %s = %d,%v want %d", c.bits, v, err, c.se)
		}
	}
}

func TestUnescape(t *testing.T) {
	got := UnescapeRBSP([]byte{0, 0, 3, 0, 5, 0, 0, 3, 1})
	want := []byte{0, 0, 0, 5, 0, 0, 1}
	if len(got) != len(want) {
		t.Fatalf("len = %d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %02x want %02x", i, got[i], want[i])
		}
	}
}

func TestAnnexBSplit(t *testing.T) {
	sps := []byte{0x67, 0x42, 0x1E}
	pps := []byte{0x68, 0xCE}
	idr := []byte{0x65, 0x88, 0x84}
	stream := []byte{0, 0, 0, 1}
	stream = append(stream, sps...)
	stream = append(stream, 0, 0, 1)
	stream = append(stream, pps...)
	stream = append(stream, 0, 0, 1)
	stream = append(stream, idr...)
	units, err := SplitAnnexB(stream)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(units) != 3 {
		t.Fatalf("units = %d want 3", len(units))
	}
	for i, want := range [][]byte{sps, pps, idr} {
		if len(units[i]) != len(want) {
			t.Fatalf("unit %d len = %d want %d", i, len(units[i]), len(want))
		}
		for j := range want {
			if units[i][j] != want[j] {
				t.Fatalf("unit %d byte %d", i, j)
			}
		}
	}
	if _, err := SplitAnnexB([]byte{1, 2, 3, 4}); err == nil {
		t.Fatal("expected error for no start code")
	}
}

func TestAVCCSplit(t *testing.T) {
	u1 := []byte{0x65, 0x01, 0x02}
	u2 := []byte{0x41, 0x03}
	var sample []byte
	for _, u := range [][]byte{u1, u2} {
		sample = append(sample, 0, 0, 0, byte(len(u)))
		sample = append(sample, u...)
	}
	units, err := SplitAVCC(sample, 4)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(units) != 2 || len(units[0]) != 3 || len(units[1]) != 2 {
		t.Fatalf("units = %v", units)
	}
	if _, err := SplitAVCC([]byte{0, 0, 0}, 4); err == nil {
		t.Fatal("expected truncation error")
	}
	if _, err := SplitAVCC(sample, 3); err == nil {
		t.Fatal("expected length-size error")
	}
}

// realAVCC is the out-of-band box of a 1536x864 High@4.2 phone-style clip.
// Independent oracles: ffprobe (profile=High level=42 pix_fmt=yuv420p,
// width=1536 height=864 fps=48) and ffmpeg trace_headers (max_num_ref_frames=4,
// wMBs=95 hMap=53, no cropping, CABAC on). Note ffprobe's stream refs=1 is
// actual usage, not the SPS max field, so the SPS assertion is 4.
var realAVCC = []byte{
	0x01, 0x64, 0x00, 0x2a, 0xff, 0xe1, 0x00, 0x1c,
	0x67, 0x64, 0x00, 0x2a, 0xac, 0xd9, 0x40, 0x60,
	0x06, 0xdb, 0x01, 0x6a, 0x02, 0x02, 0x02, 0x80,
	0x00, 0x00, 0x03, 0x00, 0x80, 0x00, 0x00, 0x30,
	0x47, 0x8c, 0x18, 0xcb, 0x01, 0x00, 0x05, 0x68,
	0xef, 0x84, 0xf2, 0xc0, 0xfd, 0xf8, 0xf8, 0x00,
}

func TestAVCCParseReal(t *testing.T) {
	a, err := ParseAVCC(realAVCC)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	if a.Profile != 100 || a.ProfileName != "High" {
		t.Fatalf("profile = %d %q", a.Profile, a.ProfileName)
	}
	if a.Level != 42 {
		t.Fatalf("level = %d", a.Level)
	}
	if a.LengthSize != 4 {
		t.Fatalf("length size = %d", a.LengthSize)
	}
	if len(a.SPS) != 1 || len(a.PPS) != 1 {
		t.Fatalf("sets = %d/%d", len(a.SPS), len(a.PPS))
	}
	ps := NewParamSets()
	if err := ps.FromAVCC(a); err != nil {
		t.Fatalf("from avcc: %v", err)
	}
	if !ps.HasSPS() || !ps.HasPPS() {
		t.Fatal("sets missing after avcc load")
	}
	s := ps.SPS[0]
	if s.ProfileIDC != 100 || s.Profile != "High" {
		t.Fatalf("sps profile = %d %q", s.ProfileIDC, s.Profile)
	}
	if s.LevelIDC != 42 || !LevelSupported(s.LevelIDC) {
		t.Fatalf("sps level = %d", s.LevelIDC)
	}
	if s.Width != 1536 || s.Height != 864 {
		t.Fatalf("size = %dx%d", s.Width, s.Height)
	}
	if s.ChromaFormat != 1 {
		t.Fatalf("chroma = %d", s.ChromaFormat)
	}
	if s.NumRefFrames != 4 {
		t.Fatalf("refs = %d", s.NumRefFrames)
	}
	q := ps.PPS[0]
	if q.SPSID != 0 {
		t.Fatalf("pps sps id = %d", q.SPSID)
	}
	if !q.EntropyCABAC {
		t.Fatal("high clip should use CABAC entropy")
	}
}

func TestSPSProfiles(t *testing.T) {
	for _, tc := range []struct {
		profile uint8
		name    string
	}{
		{66, "Baseline"},
		{77, "Main"},
		{100, "High"},
	} {
		raw := buildSPS(spsOpt{profile: tc.profile, level: 31, wMBs: 79, hMap: 44})
		s, err := ParseSPS(raw)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if s.Profile != tc.name || s.ProfileIDC != tc.profile {
			t.Fatalf("profile = %d %q want %s", s.ProfileIDC, s.Profile, tc.name)
		}
		if !IsBaselineMainHigh(s.ProfileIDC) {
			t.Fatalf("%s not in first-stage set", tc.name)
		}
		if s.Width != 1280 || s.Height != 720 {
			t.Fatalf("%s size = %dx%d", tc.name, s.Width, s.Height)
		}
		if s.LevelIDC != 31 || s.Level != "3.1" {
			t.Fatalf("level = %d %q", s.LevelIDC, s.Level)
		}
	}
}

func TestSPSLevels(t *testing.T) {
	for _, lv := range []uint8{10, 20, 30, 31, 40, 42, 50, 52} {
		if !LevelSupported(lv) {
			t.Fatalf("level %d should be supported", lv)
		}
		raw := buildSPS(spsOpt{profile: 77, level: lv, wMBs: 79, hMap: 44})
		s, err := ParseSPS(raw)
		if err != nil {
			t.Fatalf("level %d: %v", lv, err)
		}
		if s.LevelIDC != lv {
			t.Fatalf("level = %d want %d", s.LevelIDC, lv)
		}
	}
	for _, lv := range []uint8{9, 60, 0} {
		if LevelSupported(lv) {
			t.Fatalf("level %d should not be supported", lv)
		}
	}
}

func TestSPSCropping(t *testing.T) {
	raw := buildSPS(spsOpt{profile: 77, level: 31, wMBs: 53, hMap: 29, crop: []uint32{0, 5, 0, 0}})
	s, err := ParseSPS(raw)
	if err != nil {
		t.Fatalf("crop sps: %v", err)
	}
	if s.Width != 854 || s.Height != 480 {
		t.Fatalf("cropped size = %dx%d want 854x480", s.Width, s.Height)
	}
	if !s.HasCropping {
		t.Fatal("crop flag lost")
	}
}

func TestSPSPocTypes(t *testing.T) {
	for _, poc := range []uint32{0, 1, 2} {
		raw := buildSPS(spsOpt{profile: 66, level: 30, wMBs: 53, hMap: 29, pocType: poc})
		if _, err := ParseSPS(raw); err != nil {
			t.Fatalf("poc %d: %v", poc, err)
		}
	}
	if _, err := ParseSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 0, hMap: 0})); err != nil {
		t.Fatalf("tiny sps: %v", err)
	}
}

func TestSPSBad(t *testing.T) {
	if _, err := ParseSPS([]byte{}); err == nil {
		t.Fatal("empty should fail")
	}
	if _, err := ParseSPS([]byte{0x68, 0x00}); err == nil {
		t.Fatal("pps type as sps should fail")
	}
	if _, err := ParseSPS([]byte{0xE7, 0x42}); err == nil {
		t.Fatal("forbidden bit should fail")
	}
	if _, err := ParseSPS([]byte{0x67}); err == nil {
		t.Fatal("truncated should fail")
	}
}

func TestPPSRoundTrip(t *testing.T) {
	for _, cabac := range []bool{false, true} {
		raw := buildPPS(ppsOpt{ppsID: 0, spsID: 0, cabac: cabac})
		q, err := ParsePPS(raw)
		if err != nil {
			t.Fatalf("cabac=%v: %v", cabac, err)
		}
		if q.EntropyCABAC != cabac {
			t.Fatalf("cabac = %v want %v", q.EntropyCABAC, cabac)
		}
		if !q.DeblockingPresent {
			t.Fatal("deblock flag lost")
		}
	}
	raw := buildPPS(ppsOpt{ppsID: 2, spsID: 1, cabac: true, ext: true})
	q, err := ParsePPS(raw)
	if err != nil {
		t.Fatalf("ext pps: %v", err)
	}
	if q.ID != 2 || q.SPSID != 1 || !q.Transform8x8 {
		t.Fatalf("ext pps = %+v", q)
	}
}

func TestPPSSliceGroups(t *testing.T) {
	raw := buildPPS(ppsOpt{groups: 1})
	if _, err := ParsePPS(raw); !errors.Is(err, ErrSliceGroups) {
		t.Fatalf("err = %v want slice groups", err)
	}
}

func TestMissingParams(t *testing.T) {
	ps := NewParamSets()
	if _, _, err := ps.RequireForSlice(0); !errors.Is(err, ErrMissingPPS) {
		t.Fatalf("err = %v", err)
	}
	if err := ps.AddPPS(buildPPS(ppsOpt{})); err != nil {
		t.Fatalf("pps: %v", err)
	}
	if _, _, err := ps.RequireForSlice(0); !errors.Is(err, ErrMissingSPS) {
		t.Fatalf("err = %v", err)
	}
	if err := ps.AddSPS(buildSPS(spsOpt{profile: 66, level: 30, wMBs: 53, hMap: 29})); err != nil {
		t.Fatalf("sps: %v", err)
	}
	if _, _, err := ps.RequireForSlice(0); err != nil {
		t.Fatalf("complete sets: %v", err)
	}
}

func TestFrameSplit(t *testing.T) {
	sps := buildSPS(spsOpt{profile: 66, level: 30, wMBs: 53, hMap: 29})
	pps := buildPPS(ppsOpt{})
	idr := buildSlice(NALSliceIDR, 3, 0)
	p1 := buildSlice(NALSliceNonIDR, 2, 0)
	p2 := buildSlice(NALSliceNonIDR, 2, 0)
	frames, err := SplitFrames([][]byte{sps, pps, idr, p1, p2})
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(frames) != 3 {
		t.Fatalf("frames = %d want 3", len(frames))
	}
	if !frames[0].IsIDR || frames[0].SliceCount != 1 {
		t.Fatalf("frame0 = %+v", frames[0])
	}
	multi, err := SplitFrames([][]byte{p1, buildSlice(NALSliceNonIDR, 2, 5)})
	if err != nil {
		t.Fatalf("multi: %v", err)
	}
	if len(multi) != 1 || multi[0].SliceCount != 2 {
		t.Fatalf("multi = %+v", multi)
	}
	aud := []byte{0x09, 0x10}
	frames, err = SplitFrames([][]byte{sps, idr, aud, p1})
	if err != nil {
		t.Fatalf("aud split: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("aud frames = %d want 2", len(frames))
	}
}

func TestFrameSplitRejects(t *testing.T) {
	if _, err := SplitFrames([][]byte{{0x42, 0x80}}); !errors.Is(err, ErrDataPartitioning) {
		t.Fatalf("partA err = %v", err)
	}
	if _, err := SplitFrames([][]byte{{0x54, 0x80}}); !errors.Is(err, ErrUnsupportedNAL) {
		t.Fatalf("ext err = %v", err)
	}
	if _, err := SplitFrames([][]byte{{0x65}}); !errors.Is(err, ErrBadSliceHeader) {
		t.Fatalf("truncated slice err = %v", err)
	}
	got, err := SplitFrames(nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("empty = %v,%v want 0 frames no error", got, err)
	}
}

func TestNoForbiddenImports(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "github.com/energye/gpui/"
	forbidden := []string{
		prefix + "ui",
		prefix + "render",
		prefix + "gpu",
		"import " + "\"C\"",
	}
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 4 || e.Name()[len(e.Name())-3:] != ".go" {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		s := string(buf)
		for _, f := range forbidden {
			if strings.Contains(s, f) {
				t.Fatalf("%s contains forbidden import %q", e.Name(), f)
			}
		}
	}
}

func BenchmarkParseSPS(b *testing.B) {
	raw := buildSPS(spsOpt{profile: 100, level: 42, wMBs: 95, hMap: 53})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseSPS(raw); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSplitFrames(b *testing.B) {
	units := [][]byte{
		buildSPS(spsOpt{profile: 66, level: 30, wMBs: 53, hMap: 29}),
		buildPPS(ppsOpt{}),
		buildSlice(NALSliceIDR, 3, 0),
		buildSlice(NALSliceNonIDR, 2, 0),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := SplitFrames(units); err != nil {
			b.Fatal(err)
		}
	}
}
