package mp4

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Probe reports whether data looks like an MP4 shell.
// It checks the ftyp brand or the presence of a moov box.
func Probe(data []byte) bool {
	if len(data) < 8 {
		return false
	}
	c := &cursor{buf: data}
	for !c.done() {
		typ, payload, err := c.next()
		if err != nil {
			return false
		}
		switch typ {
		case "ftyp":
			if len(payload) < 8 {
				return false
			}
			major := string(payload[0:4])
			if isMP4Brand(major) {
				return true
			}
			for off := 8; off+4 <= len(payload); off += 4 {
				if isMP4Brand(string(payload[off : off+4])) {
					return true
				}
			}
		case "moov", "moof", "mdat", "free", "skip", "wide":
			if typ == "moov" {
				return true
			}
		default:
		}
		if len(payload) == 0 {
			break
		}
	}
	return false
}

func isMP4Brand(s string) bool {
	switch s {
	case "isom", "iso2", "iso4", "iso5", "iso6", "mp41", "mp42", "M4V ", "M4A ", "avc1", "dash", "msdh", "msix":
		return true
	}
	return false
}

// Parse parses a whole file image held in memory.
func Parse(data []byte) (*Movie, error) {
	return ParseReader(bytes.NewReader(data), int64(len(data)))
}

// ParseFile opens a path and parses its shell.
func ParseFile(path string) (*Movie, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return ParseReader(f, fi.Size())
}

// ParseReader parses boxes from r with known total size.
func ParseReader(r io.ReaderAt, total int64) (*Movie, error) {
	if total < 8 {
		return nil, fmt.Errorf("%w: file too small", ErrTruncated)
	}
	m := &Movie{}
	var moovPayload []byte
	var ftypSeen bool
	off := int64(0)
	for off < total {
		b, err := readBoxHeader(r, off, total)
		if err != nil {
			return nil, err
		}
		switch b.typ {
		case "ftyp":
			payload, err := readBytes(r, b.payloadOff, b.payloadEnd)
			if err != nil {
				return nil, err
			}
			parseFtyp(payload, m)
			ftypSeen = true
		case "moov":
			payload, err := readBytes(r, b.payloadOff, b.payloadEnd)
			if err != nil {
				return nil, err
			}
			moovPayload = payload
		case "moof":
			m.Fragmented = true
		case "mdat", "free", "skip", "wide", "uuid":
		default:
		}
		if b.size == 0 {
			break
		}
		off = b.payloadEnd
	}
	if moovPayload == nil {
		if m.Fragmented {
			return nil, fmt.Errorf("%w: found moof without moov", ErrFragmented)
		}
		if !ftypSeen {
			return nil, fmt.Errorf("%w: no ftyp/moov", ErrNoMoov)
		}
		return nil, fmt.Errorf("%w", ErrNoMoov)
	}
	if err := parseMoov(moovPayload, m); err != nil {
		return nil, err
	}
	if m.Fragmented {
		return nil, fmt.Errorf("%w", ErrFragmented)
	}
	if len(m.Tracks) == 0 {
		return nil, fmt.Errorf("%w: moov has no tracks", ErrNoVideoTrack)
	}
	for _, t := range m.Tracks {
		if t.Handler == "vide" {
			if m.Video == nil {
				m.Video = t
			}
		}
	}
	if m.Video == nil {
		return nil, fmt.Errorf("%w", ErrNoVideoTrack)
	}
	return m, nil
}

func parseFtyp(payload []byte, m *Movie) {
	if len(payload) < 8 {
		return
	}
	m.MajorBrand = string(payload[0:4])
	for off := 8; off+4 <= len(payload); off += 4 {
		m.Compatible = append(m.Compatible, string(payload[off:off+4]))
	}
}

type trackBuilder struct {
	id           uint32
	handler      string
	tkWidth      uint32
	tkHeight     uint32
	rotation     int
	timescale    uint32
	duration     uint64
	codec        string
	codedW       uint32
	codedH       uint32
	avcConfig    []byte
	paspH        uint32
	paspV        uint32
	stts         [][2]uint32
	stsc         [][3]uint32
	sampleSizes  []uint32
	uniformSize  uint32
	hasSizeTable bool
	chunkOffsets []uint64
	syncSet      map[int]bool
	hasStss      bool
	ctts         [][2]int64
	cttsVersion  byte
	editList     []EditEntry
	hasEditList  bool
}

func parseMoov(payload []byte, m *Movie) error {
	c := &cursor{buf: payload}
	var builders []*trackBuilder
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return err
		}
		switch typ {
		case "mvhd":
			ts, dur := parseMvhd(p)
			m.Timescale = ts
			m.Duration = dur
			m.DurationMs = ticksToMs(int64(dur), ts)
		case "trak":
			tb := &trackBuilder{syncSet: map[int]bool{}}
			if err := parseTrak(p, tb); err != nil {
				return err
			}
			builders = append(builders, tb)
		case "mvex":
			m.Fragmented = true
		default:
		}
	}
	for _, tb := range builders {
		t, err := buildTrack(tb)
		if err != nil {
			return err
		}
		m.Tracks = append(m.Tracks, t)
	}
	return nil
}

func parseMvhd(p []byte) (uint32, uint64) {
	ver, _, body, ok := fullbox(p)
	if !ok {
		return 0, 0
	}
	if ver == 1 {
		if len(body) < 28 {
			return 0, 0
		}
		ts := binary.BigEndian.Uint32(body[16:])
		dur := binary.BigEndian.Uint64(body[20:])
		if ts == 0 {
			ts = 1
		}
		return ts, dur
	}
	if len(body) < 16 {
		return 0, 0
	}
	ts := binary.BigEndian.Uint32(body[8:])
	dur := binary.BigEndian.Uint32(body[12:])
	if ts == 0 {
		ts = 1
	}
	return ts, uint64(dur)
}

func parseTrak(payload []byte, tb *trackBuilder) error {
	c := &cursor{buf: payload}
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return err
		}
		switch typ {
		case "tkhd":
			id, w, h, rot := parseTkhd(p)
			tb.id = id
			tb.tkWidth = w
			tb.tkHeight = h
			tb.rotation = rot
		case "edts":
			cc := &cursor{buf: p}
			for !cc.done() {
				et, ep, err := cc.next()
				if err != nil {
					return err
				}
				if et == "elst" {
					tb.editList = parseElst(ep)
					tb.hasEditList = len(tb.editList) > 0
				}
			}
		case "mdia":
			if err := parseMdia(p, tb); err != nil {
				return err
			}
		default:
		}
	}
	return nil
}

func parseTkhd(p []byte) (id, w, h uint32, rotation int) {
	ver, _, body, ok := fullbox(p)
	if !ok {
		return 0, 0, 0, 0
	}
	off := 0
	if ver == 1 {
		if len(body) < 92 {
			return 0, 0, 0, 0
		}
		id = binary.BigEndian.Uint32(body[16:])
		off = 48
	} else {
		if len(body) < 80 {
			return 0, 0, 0, 0
		}
		id = binary.BigEndian.Uint32(body[8:])
		off = 36
	}
	if len(body) < off+36 {
		return id, 0, 0, 0
	}
	matrix := make([]int32, 9)
	for i := 0; i < 9; i++ {
		matrix[i] = int32(binary.BigEndian.Uint32(body[off+i*4:]))
	}
	rotation = matrixRotation(matrix)
	if len(body) < off+44 {
		return id, 0, 0, rotation
	}
	w = binary.BigEndian.Uint32(body[off+36:]) >> 16
	h = binary.BigEndian.Uint32(body[off+40:]) >> 16
	return id, w, h, rotation
}

func matrixRotation(m []int32) int {
	const one = int32(0x00010000)
	const negOne = int32(-0x00010000)
	a, b, c, d := m[0], m[1], m[3], m[4]
	switch {
	case a == one && b == 0 && c == 0 && d == one:
		return 0
	case a == 0 && b == one && c == negOne && d == 0:
		return 90
	case a == negOne && b == 0 && c == 0 && d == negOne:
		return 180
	case a == 0 && b == negOne && c == one && d == 0:
		return 270
	default:
		return 0
	}
}

func parseMdia(payload []byte, tb *trackBuilder) error {
	c := &cursor{buf: payload}
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return err
		}
		switch typ {
		case "mdhd":
			ts, dur := parseMdhd(p)
			tb.timescale = ts
			tb.duration = dur
		case "hdlr":
			tb.handler = parseHdlr(p)
		case "minf":
			if err := parseMinf(p, tb); err != nil {
				return err
			}
		default:
		}
	}
	return nil
}

func parseMdhd(p []byte) (uint32, uint64) {
	ver, _, body, ok := fullbox(p)
	if !ok {
		return 1, 0
	}
	if ver == 1 {
		if len(body) < 28 {
			return 1, 0
		}
		ts := binary.BigEndian.Uint32(body[16:])
		dur := binary.BigEndian.Uint64(body[20:])
		if ts == 0 {
			ts = 1
		}
		return ts, dur
	}
	if len(body) < 16 {
		return 1, 0
	}
	ts := binary.BigEndian.Uint32(body[8:])
	dur := binary.BigEndian.Uint32(body[12:])
	if ts == 0 {
		ts = 1
	}
	return ts, uint64(dur)
}

func parseHdlr(p []byte) string {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 8 {
		return ""
	}
	return string(body[4:8])
}

func parseElst(p []byte) []EditEntry {
	ver, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil
	}
	count := binary.BigEndian.Uint32(body[0:])
	var out []EditEntry
	off := 4
	for i := uint32(0); i < count; i++ {
		if ver == 1 {
			if len(body) < off+20 {
				break
			}
			dur := binary.BigEndian.Uint64(body[off:])
			mt := int64(binary.BigEndian.Uint64(body[off+8:]))
			rate := binary.BigEndian.Uint32(body[off+16:])
			out = append(out, EditEntry{SegmentDuration: dur, MediaTime: mt, MediaRate: rate})
			off += 20
		} else {
			if len(body) < off+12 {
				break
			}
			dur := binary.BigEndian.Uint32(body[off:])
			mt := int64(int32(binary.BigEndian.Uint32(body[off+4:])))
			rate := binary.BigEndian.Uint32(body[off+8:])
			out = append(out, EditEntry{SegmentDuration: uint64(dur), MediaTime: mt, MediaRate: rate})
			off += 12
		}
	}
	return out
}

func parseMinf(payload []byte, tb *trackBuilder) error {
	c := &cursor{buf: payload}
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return err
		}
		if typ == "stbl" {
			return parseStbl(p, tb)
		}
	}
	return fmt.Errorf("%w: minf without stbl", ErrNoSampleTable)
}

func parseStbl(payload []byte, tb *trackBuilder) error {
	c := &cursor{buf: payload}
	seenStsd := false
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return err
		}
		switch typ {
		case "stsd":
			if err := parseStsdPayload(p, tb); err != nil {
				return err
			}
			seenStsd = true
		case "stts":
			tb.stts = parseStts(p)
		case "stsc":
			entries, err := parseStsc(p)
			if err != nil {
				return err
			}
			tb.stsc = entries
		case "stsz":
			sizes, uniform, hasTable, err := parseStsz(p)
			if err != nil {
				return err
			}
			tb.sampleSizes = sizes
			tb.uniformSize = uniform
			tb.hasSizeTable = hasTable
		case "stz2":
			return fmt.Errorf("%w: stz2 compact sample sizes", ErrUnsupported)
		case "stco":
			offs, err := parseStco(p)
			if err != nil {
				return err
			}
			tb.chunkOffsets = offs
		case "co64":
			offs, err := parseCo64(p)
			if err != nil {
				return err
			}
			tb.chunkOffsets = offs
		case "stss":
			set, err := parseStss(p)
			if err != nil {
				return err
			}
			tb.syncSet = set
			tb.hasStss = true
		case "ctts":
			entries, ver, err := parseCtts(p)
			if err != nil {
				return err
			}
			tb.ctts = entries
			tb.cttsVersion = ver
		case "stsh", "sdtp", "sgpd", "sbgp":
		default:
		}
	}
	if !seenStsd {
		return fmt.Errorf("%w: stbl without stsd", ErrNoSampleTable)
	}
	return nil
}

func parseStsdPayload(p []byte, tb *trackBuilder) error {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return fmt.Errorf("%w: short stsd", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if count == 0 {
		return fmt.Errorf("%w: stsd empty", ErrNoSampleTable)
	}
	off := 4
	for i := uint32(0); i < count && off+8 <= len(body); i++ {
		size := int(binary.BigEndian.Uint32(body[off:]))
		typ := ""
		if off+8 <= len(body) {
			typ = string(body[off+4 : off+8])
		}
		if size < 8 || off+size > len(body) {
			return fmt.Errorf("%w: stsd entry %d bad size", ErrBadSampleTable, i)
		}
		entry := body[off : off+size]
		if tb.handler == "" || tb.handler == "vide" {
			if isVideoSample(typ) && tb.codec == "" {
				tb.codec = typ
				parseVideoSample(entry, tb)
			} else if tb.codec == "" && i == 0 {
				tb.codec = typ
				if isVideoSample(typ) {
					parseVideoSample(entry, tb)
				}
			}
		} else if tb.codec == "" {
			tb.codec = typ
		}
		off += size
	}
	return nil
}

func isVideoSample(typ string) bool {
	switch typ {
	case "avc1", "avc3", "hvc1", "hev1", "mp4v", "encv":
		return true
	}
	return false
}

func parseVideoSample(entry []byte, tb *trackBuilder) {
	if len(entry) < 86 {
		return
	}
	tb.codedW = uint32(binary.BigEndian.Uint16(entry[32:]))
	tb.codedH = uint32(binary.BigEndian.Uint16(entry[34:]))
	if len(entry) <= 86 {
		return
	}
	inner := entry[86:]
	cc := &cursor{buf: inner}
	for !cc.done() {
		typ, p, err := cc.next()
		if err != nil {
			return
		}
		switch typ {
		case "avcC":
			cp := make([]byte, len(p))
			copy(cp, p)
			tb.avcConfig = cp
		case "pasp":
			if len(p) >= 8 {
				tb.paspH = binary.BigEndian.Uint32(p[0:])
				tb.paspV = binary.BigEndian.Uint32(p[4:])
			}
		case "clap", "colr":
		default:
		}
	}
}

func parseStts(p []byte) [][2]uint32 {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil
	}
	count := binary.BigEndian.Uint32(body[0:])
	var out [][2]uint32
	off := 4
	for i := uint32(0); i < count; i++ {
		if len(body) < off+8 {
			break
		}
		out = append(out, [2]uint32{binary.BigEndian.Uint32(body[off:]), binary.BigEndian.Uint32(body[off+4:])})
		off += 8
	}
	return out
}

func parseStsc(p []byte) ([][3]uint32, error) {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil, fmt.Errorf("%w: short stsc", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if count == 0 {
		return nil, nil
	}
	if uint64(count) > uint64(len(body))/12+1 {
		return nil, fmt.Errorf("%w: stsc count %d too large", ErrBadSampleTable, count)
	}
	var out [][3]uint32
	off := 4
	for i := uint32(0); i < count; i++ {
		if len(body) < off+12 {
			return nil, fmt.Errorf("%w: short stsc entry", ErrTruncated)
		}
		out = append(out, [3]uint32{
			binary.BigEndian.Uint32(body[off:]),
			binary.BigEndian.Uint32(body[off+4:]),
			binary.BigEndian.Uint32(body[off+8:]),
		})
		off += 12
	}
	return out, nil
}

func parseStsz(p []byte) ([]uint32, uint32, bool, error) {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 8 {
		return nil, 0, false, fmt.Errorf("%w: short stsz", ErrBadSampleTable)
	}
	uniform := binary.BigEndian.Uint32(body[0:])
	count := binary.BigEndian.Uint32(body[4:])
	if uniform != 0 {
		return nil, uniform, false, nil
	}
	if uint64(count) > uint64(len(body)-8)/4+1 {
		return nil, 0, false, fmt.Errorf("%w: stsz count %d too large", ErrBadSampleTable, count)
	}
	if len(body) < 8+int(count)*4 {
		return nil, 0, false, fmt.Errorf("%w: short stsz table", ErrTruncated)
	}
	sizes := make([]uint32, count)
	for i := range sizes {
		sizes[i] = binary.BigEndian.Uint32(body[8+i*4:])
	}
	return sizes, 0, true, nil
}

func parseStco(p []byte) ([]uint64, error) {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil, fmt.Errorf("%w: short stco", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if uint64(count) > uint64(len(body)-4)/4+1 {
		return nil, fmt.Errorf("%w: stco count %d too large", ErrBadSampleTable, count)
	}
	if len(body) < 4+int(count)*4 {
		return nil, fmt.Errorf("%w: short stco table", ErrTruncated)
	}
	out := make([]uint64, count)
	for i := range out {
		out[i] = uint64(binary.BigEndian.Uint32(body[4+i*4:]))
	}
	return out, nil
}

func parseCo64(p []byte) ([]uint64, error) {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil, fmt.Errorf("%w: short co64", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if uint64(count) > uint64(len(body)-4)/8+1 {
		return nil, fmt.Errorf("%w: co64 count %d too large", ErrBadSampleTable, count)
	}
	if len(body) < 4+int(count)*8 {
		return nil, fmt.Errorf("%w: short co64 table", ErrTruncated)
	}
	out := make([]uint64, count)
	for i := range out {
		out[i] = binary.BigEndian.Uint64(body[4+i*8:])
	}
	return out, nil
}

func parseStss(p []byte) (map[int]bool, error) {
	_, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil, fmt.Errorf("%w: short stss", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if uint64(count) > uint64(len(body)-4)/4+1 {
		return nil, fmt.Errorf("%w: stss count %d too large", ErrBadSampleTable, count)
	}
	if len(body) < 4+int(count)*4 {
		return nil, fmt.Errorf("%w: short stss table", ErrTruncated)
	}
	set := make(map[int]bool, count)
	for i := uint32(0); i < count; i++ {
		n := int(binary.BigEndian.Uint32(body[4+i*4:]))
		if n >= 1 {
			set[n] = true
		}
	}
	return set, nil
}

func parseCtts(p []byte) ([][2]int64, byte, error) {
	ver, _, body, ok := fullbox(p)
	if !ok || len(body) < 4 {
		return nil, ver, fmt.Errorf("%w: short ctts", ErrBadSampleTable)
	}
	count := binary.BigEndian.Uint32(body[0:])
	if uint64(count) > uint64(len(body)-4)/8+1 {
		return nil, ver, fmt.Errorf("%w: ctts count %d too large", ErrBadSampleTable, count)
	}
	if len(body) < 4+int(count)*8 {
		return nil, ver, fmt.Errorf("%w: short ctts table", ErrTruncated)
	}
	var out [][2]int64
	off := 4
	for i := uint32(0); i < count; i++ {
		n := int64(binary.BigEndian.Uint32(body[off:]))
		var d int64
		if ver == 1 {
			d = int64(int32(binary.BigEndian.Uint32(body[off+4:])))
		} else {
			d = int64(binary.BigEndian.Uint32(body[off+4:]))
		}
		out = append(out, [2]int64{n, d})
		off += 8
	}
	return out, ver, nil
}

func buildTrack(tb *trackBuilder) (*Track, error) {
	t := &Track{
		ID:           tb.id,
		Handler:      tb.handler,
		Codec:        tb.codec,
		CodedWidth:   tb.codedW,
		CodedHeight:  tb.codedH,
		Width:        tb.tkWidth,
		Height:       tb.tkHeight,
		Rotation:     tb.rotation,
		Timescale:    tb.timescale,
		Duration:     tb.duration,
		PixelAspectH: tb.paspH,
		PixelAspectV: tb.paspV,
		EditList:     tb.editList,
		HasEditList:  tb.hasEditList,
		HasCTTS:      len(tb.ctts) > 0,
	}
	if t.Timescale == 0 {
		t.Timescale = 1
	}
	if t.Width == 0 {
		t.Width = t.CodedWidth
	}
	if t.Height == 0 {
		t.Height = t.CodedHeight
	}
	if len(tb.avcConfig) > 0 {
		t.AVCConfig = append([]byte(nil), tb.avcConfig...)
	}
	t.DurationMs = ticksToMs(int64(t.Duration), t.Timescale)
	if t.Handler != "vide" {
		return t, nil
	}
	if t.Codec == "" {
		return nil, fmt.Errorf("%w: video track without codec", ErrNoSampleTable)
	}
	sizes, err := expandSizes(tb)
	if err != nil {
		return nil, err
	}
	n := len(sizes)
	if n == 0 {
		return nil, fmt.Errorf("%w: zero samples", ErrNoSampleTable)
	}
	offsets, err := expandOffsets(tb, sizes)
	if err != nil {
		return nil, err
	}
	dts, sttsDur := expandDTS(tb, n)
	cto := expandCTO(tb, n)
	t.SampleCount = n
	t.Samples = make([]Sample, n)
	t.Keyframes = t.Keyframes[:0]
	for i := 0; i < n; i++ {
		pts := dts[i] + cto[i]
		num := i + 1
		isKey := true
		if tb.hasStss {
			isKey = tb.syncSet[num]
		}
		s := Sample{
			Number:   num,
			Size:     sizes[i],
			Offset:   offsets[i],
			DTS:      dts[i],
			PTS:      pts,
			DTSMs:    ticksToMs(dts[i], t.Timescale),
			PTSMs:    ticksToMs(pts, t.Timescale),
			Keyframe: isKey,
		}
		t.Samples[i] = s
		if isKey {
			t.Keyframes = append(t.Keyframes, Keyframe{
				SampleNumber: num,
				Offset:       offsets[i],
				DTSMs:        s.DTSMs,
				PTSMs:        s.PTSMs,
			})
		}
	}
	dur := int64(t.Duration)
	if dur == 0 {
		dur = sttsDur
		t.Duration = uint64(dur)
		t.DurationMs = ticksToMs(dur, t.Timescale)
	}
	if dur > 0 {
		t.FrameRate = float64(n) * float64(t.Timescale) / float64(dur)
	} else if sttsDur > 0 {
		t.FrameRate = float64(n) * float64(t.Timescale) / float64(sttsDur)
	}
	return t, nil
}

func expandSizes(tb *trackBuilder) ([]uint32, error) {
	if !tb.hasSizeTable {
		if tb.uniformSize == 0 {
			count := sttsSampleCount(tb.stts)
			if count == 0 {
				return nil, fmt.Errorf("%w: no size info", ErrNoSampleTable)
			}
			sizes := make([]uint32, count)
			return sizes, nil
		}
		count := sttsSampleCount(tb.stts)
		if count == 0 {
			return nil, fmt.Errorf("%w: no size info", ErrNoSampleTable)
		}
		sizes := make([]uint32, count)
		for i := range sizes {
			sizes[i] = tb.uniformSize
		}
		return sizes, nil
	}
	return append([]uint32(nil), tb.sampleSizes...), nil
}

func sttsSampleCount(stts [][2]uint32) int {
	total := 0
	for _, e := range stts {
		total += int(e[0])
	}
	return total
}

func expandOffsets(tb *trackBuilder, sizes []uint32) ([]uint64, error) {
	n := len(sizes)
	if len(tb.chunkOffsets) == 0 {
		return nil, fmt.Errorf("%w: missing stco/co64", ErrBadSampleTable)
	}
	if len(tb.stsc) == 0 {
		return nil, fmt.Errorf("%w: missing stsc", ErrBadSampleTable)
	}
	chunks := len(tb.chunkOffsets)
	perChunk := make([]int, chunks)
	for i := 0; i < chunks; i++ {
		chunkNo := i + 1
		perChunk[i] = stscSamplesForChunk(tb.stsc, chunkNo)
		if perChunk[i] <= 0 {
			return nil, fmt.Errorf("%w: stsc gives %d samples for chunk %d", ErrBadSampleTable, perChunk[i], chunkNo)
		}
	}
	total := 0
	for _, c := range perChunk {
		total += c
	}
	if total != n {
		return nil, fmt.Errorf("%w: stsc total %d != sizes %d", ErrBadSampleTable, total, n)
	}
	offsets := make([]uint64, n)
	idx := 0
	for ci, base := range tb.chunkOffsets {
		off := base
		for k := 0; k < perChunk[ci]; k++ {
			offsets[idx] = off
			off += uint64(sizes[idx])
			idx++
		}
	}
	return offsets, nil
}

func stscSamplesForChunk(stsc [][3]uint32, chunkNo int) int {
	best := stsc[0]
	for _, e := range stsc {
		if int(e[0]) <= chunkNo {
			best = e
		} else {
			break
		}
	}
	return int(best[1])
}

func expandDTS(tb *trackBuilder, n int) ([]int64, int64) {
	dts := make([]int64, n)
	var cur int64
	idx := 0
	for _, e := range tb.stts {
		count := int(e[0])
		delta := int64(e[1])
		for k := 0; k < count && idx < n; k++ {
			dts[idx] = cur
			cur += delta
			idx++
		}
	}
	for idx < n {
		dts[idx] = cur
		idx++
	}
	return dts, cur
}

func expandCTO(tb *trackBuilder, n int) []int64 {
	cto := make([]int64, n)
	idx := 0
	for _, e := range tb.ctts {
		count := int(e[0])
		off := e[1]
		for k := 0; k < count && idx < n; k++ {
			cto[idx] = off
			idx++
		}
	}
	return cto
}
