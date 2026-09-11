package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkBox(typ string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(buf[0:], size)
	copy(buf[4:], typ)
	copy(buf[8:], payload)
	return buf
}

func mkFullBox(typ string, version byte, flags uint32, body []byte) []byte {
	payload := make([]byte, 4+len(body))
	payload[0] = version
	payload[1] = byte(flags >> 16)
	payload[2] = byte(flags >> 8)
	payload[3] = byte(flags)
	copy(payload[4:], body)
	return mkBox(typ, payload)
}

func mkFtyp() []byte {
	payload := make([]byte, 16)
	copy(payload[0:], "isom")
	binary.BigEndian.PutUint32(payload[4:], 0)
	copy(payload[8:], "isom")
	copy(payload[12:], "mp41")
	return mkBox("ftyp", payload)
}

func mkMvhd(timescale uint32, duration uint32) []byte {
	body := make([]byte, 100)
	binary.BigEndian.PutUint32(body[8:], timescale)
	binary.BigEndian.PutUint32(body[12:], duration)
	binary.BigEndian.PutUint32(body[16:], 0x00010000)
	binary.BigEndian.PutUint16(body[20:], 0x0100)
	return mkFullBox("mvhd", 0, 0, body)
}

func tkhdMatrix(rotation int) []byte {
	m := make([]byte, 36)
	put := func(i int, v int32) {
		binary.BigEndian.PutUint32(m[i*4:], uint32(v))
	}
	const one = int32(0x00010000)
	const negOne = int32(-0x00010000)
	switch rotation {
	case 90:
		put(0, 0)
		put(1, one)
		put(2, 0)
		put(3, negOne)
		put(4, 0)
		put(5, 0)
		put(6, 0)
		put(7, 0)
		put(8, 0x40000000)
	case 180:
		put(0, negOne)
		put(1, 0)
		put(2, 0)
		put(3, 0)
		put(4, negOne)
		put(5, 0)
		put(6, 0)
		put(7, 0)
		put(8, 0x40000000)
	case 270:
		put(0, 0)
		put(1, negOne)
		put(2, 0)
		put(3, one)
		put(4, 0)
		put(5, 0)
		put(6, 0)
		put(7, 0)
		put(8, 0x40000000)
	default:
		put(0, one)
		put(1, 0)
		put(2, 0)
		put(3, 0)
		put(4, one)
		put(5, 0)
		put(6, 0)
		put(7, 0)
		put(8, 0x40000000)
	}
	return m
}

func mkTkhd(id, w, h uint32, rotation int) []byte {
	body := make([]byte, 80)
	binary.BigEndian.PutUint32(body[8:], id)
	binary.BigEndian.PutUint32(body[16:], 1000)
	copy(body[36:], tkhdMatrix(rotation))
	binary.BigEndian.PutUint32(body[72:], w<<16)
	binary.BigEndian.PutUint32(body[76:], h<<16)
	return mkFullBox("tkhd", 0, 7, body)
}

func mkMdhd(timescale uint32, duration uint32) []byte {
	body := make([]byte, 20)
	binary.BigEndian.PutUint32(body[8:], timescale)
	binary.BigEndian.PutUint32(body[12:], duration)
	return mkFullBox("mdhd", 0, 0, body)
}

func mkHdlr(handler string) []byte {
	body := make([]byte, 25)
	copy(body[4:], handler)
	copy(body[20:], "test\x00")
	return mkFullBox("hdlr", 0, 0, body)
}

func mkAvc1(codedW, codedH uint16, withPasp bool) []byte {
	entry := make([]byte, 86)
	binary.BigEndian.PutUint32(entry[0:], 0) // patched later
	copy(entry[4:], "avc1")
	binary.BigEndian.PutUint16(entry[14:], 1)
	binary.BigEndian.PutUint16(entry[32:], codedW)
	binary.BigEndian.PutUint16(entry[34:], codedH)
	binary.BigEndian.PutUint32(entry[36:], 0x00480000)
	binary.BigEndian.PutUint32(entry[40:], 0x00480000)
	binary.BigEndian.PutUint16(entry[48:], 1)
	binary.BigEndian.PutUint16(entry[82:], 0x0018)
	binary.BigEndian.PutUint16(entry[84:], 0xFFFF)
	avcc := mkBox("avcC", []byte{0x01, 0x64, 0x00, 0x1E, 0xFF, 0xE1, 0x00, 0x0A, 0x01, 0x02, 0x03})
	children := bytes.Clone(avcc)
	if withPasp {
		pasp := make([]byte, 8)
		binary.BigEndian.PutUint32(pasp[0:], 1)
		binary.BigEndian.PutUint32(pasp[4:], 1)
		children = append(children, mkBox("pasp", pasp)...)
	}
	entry = append(entry, children...)
	binary.BigEndian.PutUint32(entry[0:], uint32(len(entry)))
	return entry
}

func mkStsd(codedW, codedH uint16, withPasp bool) []byte {
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body[0:], 1)
	body = append(body, mkAvc1(codedW, codedH, withPasp)...)
	return mkFullBox("stsd", 0, 0, body)
}

func mkStts(entries [][2]uint32) []byte {
	body := make([]byte, 4+len(entries)*8)
	binary.BigEndian.PutUint32(body[0:], uint32(len(entries)))
	for i, e := range entries {
		binary.BigEndian.PutUint32(body[4+i*8:], e[0])
		binary.BigEndian.PutUint32(body[4+i*8+4:], e[1])
	}
	return mkFullBox("stts", 0, 0, body)
}

func mkStsc(entries [][3]uint32) []byte {
	body := make([]byte, 4+len(entries)*12)
	binary.BigEndian.PutUint32(body[0:], uint32(len(entries)))
	for i, e := range entries {
		binary.BigEndian.PutUint32(body[4+i*12:], e[0])
		binary.BigEndian.PutUint32(body[4+i*12+4:], e[1])
		binary.BigEndian.PutUint32(body[4+i*12+8:], e[2])
	}
	return mkFullBox("stsc", 0, 0, body)
}

func mkStsz(sizes []uint32) []byte {
	body := make([]byte, 8+len(sizes)*4)
	binary.BigEndian.PutUint32(body[0:], 0)
	binary.BigEndian.PutUint32(body[4:], uint32(len(sizes)))
	for i, s := range sizes {
		binary.BigEndian.PutUint32(body[8+i*4:], s)
	}
	return mkFullBox("stsz", 0, 0, body)
}

func mkStszUniform(sampleSize, count uint32) []byte {
	body := make([]byte, 8)
	binary.BigEndian.PutUint32(body[0:], sampleSize)
	binary.BigEndian.PutUint32(body[4:], count)
	return mkFullBox("stsz", 0, 0, body)
}

func mkStco(offsets []uint32) []byte {
	body := make([]byte, 4+len(offsets)*4)
	binary.BigEndian.PutUint32(body[0:], uint32(len(offsets)))
	for i, o := range offsets {
		binary.BigEndian.PutUint32(body[4+i*4:], o)
	}
	return mkFullBox("stco", 0, 0, body)
}

func mkCo64(offsets []uint64) []byte {
	body := make([]byte, 4+len(offsets)*8)
	binary.BigEndian.PutUint32(body[0:], uint32(len(offsets)))
	for i, o := range offsets {
		binary.BigEndian.PutUint64(body[4+i*8:], o)
	}
	return mkFullBox("co64", 0, 0, body)
}

func mkStss(samples []uint32) []byte {
	body := make([]byte, 4+len(samples)*4)
	binary.BigEndian.PutUint32(body[0:], uint32(len(samples)))
	for i, s := range samples {
		binary.BigEndian.PutUint32(body[4+i*4:], s)
	}
	return mkFullBox("stss", 0, 0, body)
}

func mkCtts(entries [][2]int64) []byte {
	body := make([]byte, 4+len(entries)*8)
	binary.BigEndian.PutUint32(body[0:], uint32(len(entries)))
	for i, e := range entries {
		binary.BigEndian.PutUint32(body[4+i*8:], uint32(e[0]))
		binary.BigEndian.PutUint32(body[4+i*8+4:], uint32(e[1]))
	}
	return mkFullBox("ctts", 0, 0, body)
}

func mkElst() []byte {
	body := make([]byte, 4+12)
	binary.BigEndian.PutUint32(body[0:], 1)
	binary.BigEndian.PutUint32(body[4:], 180000)
	binary.BigEndian.PutUint32(body[8:], 0)
	binary.BigEndian.PutUint32(body[12:], 0x00010000)
	return mkFullBox("elst", 0, 0, body)
}

type stblOpt struct {
	withStss   bool
	stss       []uint32
	withCtts   bool
	ctts       [][2]int64
	withCo64   bool
	uniformSz  uint32
	withPasp   bool
	withElst   bool
	extraBoxes [][]byte
}

func buildVideoTrak(opt stblOpt) []byte {
	sizes := []uint32{100, 200, 150, 250}
	var stszBox []byte
	if opt.uniformSz != 0 {
		stszBox = mkStszUniform(opt.uniformSz, 4)
	} else {
		stszBox = mkStsz(sizes)
	}
	var chunkBox []byte
	if opt.withCo64 {
		chunkBox = mkCo64([]uint64{1000, 2000})
	} else {
		chunkBox = mkStco([]uint32{1000, 2000})
	}
	stblPayload := bytes.Join([][]byte{
		mkStsd(1280, 720, opt.withPasp),
		mkStts([][2]uint32{{4, 45000}}),
		mkStsc([][3]uint32{{1, 2, 1}}),
		stszBox,
		chunkBox,
	}, nil)
	if opt.withStss {
		stblPayload = append(stblPayload, mkStss(opt.stss)...)
	} else if len(opt.stss) == 0 && !opt.withStss {
		// absent: all keyframes
	} else {
		stblPayload = append(stblPayload, mkStss(opt.stss)...)
	}
	if opt.withCtts {
		stblPayload = append(stblPayload, mkCtts(opt.ctts)...)
	}
	for _, b := range opt.extraBoxes {
		stblPayload = append(stblPayload, b...)
	}
	stbl := mkBox("stbl", stblPayload)
	dinf := mkBox("dinf", mkBox("dref", mkFullBox("dref_dummy", 0, 0, nil)[:0]))
	vmhd := mkFullBox("vmhd", 0, 1, make([]byte, 8))
	minf := mkBox("minf", bytes.Join([][]byte{vmhd, dinf, stbl}, nil))
	mdia := mkBox("mdia", bytes.Join([][]byte{
		mkMdhd(90000, 180000),
		mkHdlr("vide"),
		minf,
	}, nil))
	trakPayload := bytes.Join([][]byte{mkTkhd(1, 1280, 720, 0), mdia}, nil)
	if opt.withElst {
		edts := mkBox("edts", mkElst())
		trakPayload = bytes.Join([][]byte{mkTkhd(1, 1280, 720, 0), edts, mdia}, nil)
	}
	return mkBox("trak", trakPayload)
}

func buildMP4(trakBoxes ...[]byte) []byte {
	mvhd := mkMvhd(1000, 2000)
	moovPayload := append([]byte{}, mvhd...)
	for _, t := range trakBoxes {
		moovPayload = append(moovPayload, t...)
	}
	moov := mkBox("moov", moovPayload)
	mdat := mkBox("mdat", make([]byte, 800))
	return bytes.Join([][]byte{mkFtyp(), moov, mdat}, nil)
}

func TestParseMinimalVideo(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1, 3}, withPasp: true}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !m.HasVideo() {
		t.Fatal("HasVideo = false")
	}
	v := m.Video
	if v.Width != 1280 || v.Height != 720 {
		t.Fatalf("width/height = %d/%d", v.Width, v.Height)
	}
	if v.CodedWidth != 1280 || v.CodedHeight != 720 {
		t.Fatalf("coded = %d/%d", v.CodedWidth, v.CodedHeight)
	}
	if v.Codec != "avc1" {
		t.Fatalf("codec = %q", v.Codec)
	}
	if v.SampleCount != 4 {
		t.Fatalf("samples = %d", v.SampleCount)
	}
	if len(v.Keyframes) != 2 {
		t.Fatalf("keyframes = %d", len(v.Keyframes))
	}
	if v.DurationMs != 2000 {
		t.Fatalf("durationMs = %d", v.DurationMs)
	}
	if v.FrameRate < 1.99 || v.FrameRate > 2.01 {
		t.Fatalf("fps = %v", v.FrameRate)
	}
	if len(v.AVCConfig) == 0 {
		t.Fatal("avcC missing")
	}
	if v.PixelAspectH != 1 || v.PixelAspectV != 1 {
		t.Fatalf("pasp = %d/%d", v.PixelAspectH, v.PixelAspectV)
	}
	wantOff := []uint64{1000, 1100, 2000, 2150}
	for i, w := range wantOff {
		if v.Samples[i].Offset != w {
			t.Fatalf("sample %d offset = %d want %d", i+1, v.Samples[i].Offset, w)
		}
	}
	wantDTSMs := []int64{0, 500, 1000, 1500}
	for i, w := range wantDTSMs {
		if v.Samples[i].DTSMs != w || v.Samples[i].PTSMs != w {
			t.Fatalf("sample %d dts/ptSms = %d/%d want %d", i+1, v.Samples[i].DTSMs, v.Samples[i].PTSMs, w)
		}
	}
	if v.Keyframes[0].SampleNumber != 1 || v.Keyframes[1].SampleNumber != 3 {
		t.Fatalf("keyframe samples = %v", v.Keyframes)
	}
	if v.Keyframes[0].Offset != 1000 || v.Keyframes[1].Offset != 2000 {
		t.Fatalf("keyframe offsets = %v", v.Keyframes)
	}
	k, ok := v.KeyframeNear(1200)
	if !ok || k.SampleNumber != 3 {
		t.Fatalf("KeyframeNear(1200) = %v %v", k, ok)
	}
}

func TestProbe(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1, 3}}))
	if !Probe(data) {
		t.Fatal("Probe = false for mp4")
	}
	if Probe([]byte("not a video file at all........")) {
		t.Fatal("Probe = true for junk")
	}
	if Probe(nil) {
		t.Fatal("Probe = true for nil")
	}
}

func TestNoStssMeansAllKeyframes(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Video.Keyframes) != 4 {
		t.Fatalf("keyframes without stss = %d want 4", len(m.Video.Keyframes))
	}
}

func TestCttsOffsets(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{
		withStss: true, stss: []uint32{1, 3},
		withCtts: true, ctts: [][2]int64{{2, 90000}, {2, 0}},
	}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	v := m.Video
	if !v.HasCTTS {
		t.Fatal("HasCTTS = false")
	}
	if v.Samples[0].PTSMs != 1000 || v.Samples[1].PTSMs != 1500 {
		t.Fatalf("pts = %d %d", v.Samples[0].PTSMs, v.Samples[1].PTSMs)
	}
	if v.Samples[2].PTSMs != 1000 || v.Samples[3].PTSMs != 1500 {
		t.Fatalf("pts = %d %d", v.Samples[2].PTSMs, v.Samples[3].PTSMs)
	}
}

func TestCo64(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1}, withCo64: true}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Video.Samples[0].Offset != 1000 {
		t.Fatalf("offset = %d", m.Video.Samples[0].Offset)
	}
}

func TestUniformStsz(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1}, uniformSz: 200}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Video.Samples[1].Size != 200 {
		t.Fatalf("size = %d", m.Video.Samples[1].Size)
	}
	if m.Video.Samples[2].Offset != 2000 {
		t.Fatalf("offset = %d", m.Video.Samples[2].Offset)
	}
}

func TestRotation90(t *testing.T) {
	tkhd := mkTkhd(1, 720, 1280, 90)
	mdia := mkBox("mdia", bytes.Join([][]byte{
		mkMdhd(90000, 180000),
		mkHdlr("vide"),
		mkBox("minf", bytes.Join([][]byte{
			mkFullBox("vmhd", 0, 1, make([]byte, 8)),
			mkBox("dinf", []byte{}),
			mkBox("stbl", bytes.Join([][]byte{
				mkStsd(1280, 720, false),
				mkStts([][2]uint32{{4, 45000}}),
				mkStsc([][3]uint32{{1, 2, 1}}),
				mkStsz([]uint32{100, 200, 150, 250}),
				mkStco([]uint32{1000, 2000}),
				mkStss([]uint32{1}),
			}, nil)),
		}, nil)),
	}, nil))
	trak := mkBox("trak", bytes.Join([][]byte{tkhd, mdia}, nil))
	data := buildMP4(trak)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Video.Rotation != 90 {
		t.Fatalf("rotation = %d", m.Video.Rotation)
	}
}

func TestEditList(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1}, withElst: true}))
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !m.Video.HasEditList || len(m.Video.EditList) != 1 {
		t.Fatalf("edit list = %+v", m.Video.EditList)
	}
}

func TestErrors(t *testing.T) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1}}))

	t.Run("truncated", func(t *testing.T) {
		_, err := Parse(data[:len(data)-20])
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrTruncated) && !errors.Is(err, ErrBadBox) {
			t.Fatalf("wrong error: %v", err)
		}
	})

	t.Run("missingMoov", func(t *testing.T) {
		_, err := Parse(mkFtyp())
		if !errors.Is(err, ErrNoMoov) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("noVideoTrack", func(t *testing.T) {
		audioTrak := mkBox("trak", mkBox("mdia", bytes.Join([][]byte{
			mkMdhd(48000, 48000),
			mkHdlr("soun"),
			mkBox("minf", mkBox("stbl", bytes.Join([][]byte{
				mkFullBox("stsd", 0, 0, append(func() []byte {
					b := make([]byte, 4)
					binary.BigEndian.PutUint32(b, 1)
					return b
				}(), func() []byte {
					e := make([]byte, 8)
					binary.BigEndian.PutUint32(e[0:], 8)
					copy(e[4:], "mp4a")
					return e
				}()...)),
				mkStts([][2]uint32{{10, 1024}}),
				mkStsc([][3]uint32{{1, 10, 1}}),
				mkStsz([]uint32{10, 10, 10, 10, 10, 10, 10, 10, 10, 10}),
				mkStco([]uint32{1000}),
			}, nil))),
		}, nil)))
		_, err := Parse(buildMP4(audioTrak))
		if !errors.Is(err, ErrNoVideoTrack) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("fragmented", func(t *testing.T) {
		frag := append(bytes.Clone(data), mkBox("moof", mkBox("traf", []byte{1, 2, 3}))...)
		_, err := Parse(frag)
		if !errors.Is(err, ErrFragmented) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("junk", func(t *testing.T) {
		_, err := Parse([]byte("12345678junkjunkjunkjunk"))
		if err == nil {
			t.Fatal("expected error")
		}
	})
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

func BenchmarkParseMinimal(b *testing.B) {
	data := buildMP4(buildVideoTrak(stblOpt{withStss: true, stss: []uint32{1, 3}}))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}
