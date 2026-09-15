// Fragmented MP4 (B1): moof segment index + sample assembly.
//
// Peer (read-only, no code copied):
//
//	libavformat/mov.c:1946 mov_read_moof (moof_offset/implicit_offset,
//	  update_frag_index, recurse traf) + :6057 mov_read_tfhd (defaults) +
//	:6125 mov_read_trex (per-track defaults) + :6151 mov_read_tfdt (base
//	  decode time) + :6190 mov_read_trun (sample table append, keyframe
//	  from flags, DTS from tfdt/track_end) + libavformat/isom.h:408-431
//	  flag bits.
//	libavformat/seek.c + mov.c:12247 mov_seek_fragment (segment-first)
//	shape is covered by video/seek_index.go once tables exist.
//	Top-level sidx boxes are ignored (ffmpeg use_tfdt default: tfdt
//	times the samples); they fall into the default branch in demux.go.
//
// Shape copied: trex defaults -> tfhd override -> trun entries; base
// offset = explicit / moof / implicit (same priority as tfhd); DTS =
// tfdt else track_end; PTS = DTS + shift + CTS (shift covers negative
// CTS, same as mov_update_dts_shift); keyframe = !(NON_SYNC|DEPENDS_YES).
// Sample tables stay in decode (DTS) order; seeking reuses the S8 index.
package mp4

import (
	"encoding/binary"
	"fmt"
)

// tfhd/trun flag bits (isom.h:408-421, constants only).
// tfhdDurationIsEmpty only marks empty segments (no samples to time),
// so no bit is kept for it.
const (
	tfhdBaseDataOffset    = 0x01
	tfhdStsdID            = 0x02
	tfhdDefaultDuration   = 0x08
	tfhdDefaultSize       = 0x10
	tfhdDefaultFlags      = 0x20
	tfhdDefaultBaseIsMoof = 0x020000
	trunDataOffset        = 0x01
	trunFirstSampleFlags  = 0x04
	trunSampleDuration    = 0x100
	trunSampleSize        = 0x200
	trunSampleFlags       = 0x400
	trunSampleCTS         = 0x800
	fragFlagNonSync       = 0x00010000
	fragFlagDependsYes    = 0x01000000
)

// trexEntry is one mvex/trex default row.
type trexEntry struct {
	trackID  uint32
	duration uint32
	size     uint32
	flags    uint32
}

// moofData is one top-level moof box with its file offset.
type moofData struct {
	offset  int64 // file offset of the moof header (moof_offset peer)
	payload []byte
}

// fragBuilt is one assembled fragment sample (decode order).
type fragBuilt struct {
	offset   uint64
	size     uint32
	dts      int64
	cts      int32
	duration uint32
	keyframe bool
}

func parseTrex(payload []byte) (trexEntry, error) {
	_, _, body, ok := fullbox(payload)
	if !ok || len(body) < 20 {
		return trexEntry{}, fmt.Errorf("%w: short trex", ErrBadBox)
	}
	return trexEntry{
		trackID:  binary.BigEndian.Uint32(body[0:]),
		duration: binary.BigEndian.Uint32(body[8:]),
		size:     binary.BigEndian.Uint32(body[12:]),
		flags:    binary.BigEndian.Uint32(body[16:]),
	}, nil
}

// parseMvexFromMoov collects trex defaults from moov/mvex.
func parseMvexFromMoov(moovPayload []byte) map[uint32]trexEntry {
	out := map[uint32]trexEntry{}
	c := &cursor{buf: moovPayload}
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return out
		}
		if typ != "mvex" {
			continue
		}
		cc := &cursor{buf: p}
		for !cc.done() {
			et, ep, err := cc.next()
			if err != nil {
				break
			}
			if et != "trex" {
				continue
			}
			if e, err := parseTrex(ep); err == nil && e.trackID != 0 {
				out[e.trackID] = e
			}
		}
	}
	return out
}

type tfhdInfo struct {
	trackID         uint32
	hasBase         bool
	baseOffset      uint64
	baseIsMoof      bool
	defaultDuration uint32
	hasDuration     bool
	defaultSize     uint32
	hasSize         bool
	defaultFlags    uint32
	hasFlags        bool
}

func parseTfhd(payload []byte) (tfhdInfo, error) {
	_, flags, body, ok := fullbox(payload)
	if !ok || len(body) < 4 {
		return tfhdInfo{}, fmt.Errorf("%w: short tfhd", ErrBadBox)
	}
	ti := tfhdInfo{trackID: binary.BigEndian.Uint32(body[0:])}
	if ti.trackID == 0 {
		return tfhdInfo{}, fmt.Errorf("%w: tfhd track id 0", ErrBadBox)
	}
	off := 4
	if flags&tfhdBaseDataOffset != 0 {
		if len(body) < off+8 {
			return tfhdInfo{}, fmt.Errorf("%w: short tfhd base", ErrBadBox)
		}
		ti.hasBase = true
		ti.baseOffset = binary.BigEndian.Uint64(body[off:])
		off += 8
	}
	if flags&tfhdStsdID != 0 {
		if len(body) < off+4 {
			return tfhdInfo{}, fmt.Errorf("%w: short tfhd stsd", ErrBadBox)
		}
		off += 4
	}
	if flags&tfhdDefaultDuration != 0 {
		if len(body) < off+4 {
			return tfhdInfo{}, fmt.Errorf("%w: short tfhd duration", ErrBadBox)
		}
		ti.defaultDuration = binary.BigEndian.Uint32(body[off:])
		ti.hasDuration = true
		off += 4
	}
	if flags&tfhdDefaultSize != 0 {
		if len(body) < off+4 {
			return tfhdInfo{}, fmt.Errorf("%w: short tfhd size", ErrBadBox)
		}
		ti.defaultSize = binary.BigEndian.Uint32(body[off:])
		ti.hasSize = true
		off += 4
	}
	if flags&tfhdDefaultFlags != 0 {
		if len(body) < off+4 {
			return tfhdInfo{}, fmt.Errorf("%w: short tfhd flags", ErrBadBox)
		}
		ti.defaultFlags = binary.BigEndian.Uint32(body[off:])
		ti.hasFlags = true
		off += 4
	}
	ti.baseIsMoof = flags&tfhdDefaultBaseIsMoof != 0
	return ti, nil
}

func parseTfdt(payload []byte) (uint64, bool, error) {
	ver, _, body, ok := fullbox(payload)
	if !ok || len(body) < 4 {
		return 0, false, fmt.Errorf("%w: short tfdt", ErrBadBox)
	}
	if ver == 1 {
		if len(body) < 8 {
			return 0, false, fmt.Errorf("%w: short tfdt64", ErrBadBox)
		}
		return binary.BigEndian.Uint64(body[0:]), true, nil
	}
	return uint64(binary.BigEndian.Uint32(body[0:])), true, nil
}

type trunEntry struct {
	duration uint32
	hasDur   bool
	size     uint32
	hasSize  bool
	flags    uint32
	hasFlags bool
	cts      int32
	hasCTS   bool
}

type trunInfo struct {
	hasDataOffset bool
	dataOffset    int32
	hasFirstFlags bool
	firstFlags    uint32
	entries       []trunEntry
}

func parseTrun(payload []byte) (trunInfo, error) {
	_, flags, body, ok := fullbox(payload)
	if !ok || len(body) < 4 {
		return trunInfo{}, fmt.Errorf("%w: short trun", ErrBadBox)
	}
	ti := trunInfo{}
	count := binary.BigEndian.Uint32(body[0:])
	if count > 1<<20 {
		return trunInfo{}, fmt.Errorf("%w: trun count %d too large", ErrBadBox, count)
	}
	off := 4
	if flags&trunDataOffset != 0 {
		if len(body) < off+4 {
			return trunInfo{}, fmt.Errorf("%w: short trun data offset", ErrBadBox)
		}
		ti.hasDataOffset = true
		ti.dataOffset = int32(binary.BigEndian.Uint32(body[off:]))
		off += 4
	}
	if flags&trunFirstSampleFlags != 0 {
		if len(body) < off+4 {
			return trunInfo{}, fmt.Errorf("%w: short trun first flags", ErrBadBox)
		}
		ti.hasFirstFlags = true
		ti.firstFlags = binary.BigEndian.Uint32(body[off:])
		off += 4
	}
	need := 0
	if flags&trunSampleDuration != 0 {
		need += 4
	}
	if flags&trunSampleSize != 0 {
		need += 4
	}
	if flags&trunSampleFlags != 0 {
		need += 4
	}
	if flags&trunSampleCTS != 0 {
		need += 4
	}
	if uint64(count)*uint64(need)+uint64(off) > uint64(len(body)) {
		return trunInfo{}, fmt.Errorf("%w: trun entries overrun", ErrBadBox)
	}
	for i := uint32(0); i < count; i++ {
		var e trunEntry
		if flags&trunSampleDuration != 0 {
			e.duration = binary.BigEndian.Uint32(body[off:])
			e.hasDur = true
			off += 4
		}
		if flags&trunSampleSize != 0 {
			e.size = binary.BigEndian.Uint32(body[off:])
			e.hasSize = true
			off += 4
		}
		if flags&trunSampleFlags != 0 {
			e.flags = binary.BigEndian.Uint32(body[off:])
			e.hasFlags = true
			off += 4
		}
		if flags&trunSampleCTS != 0 {
			e.cts = int32(binary.BigEndian.Uint32(body[off:]))
			e.hasCTS = true
			off += 4
		}
		ti.entries = append(ti.entries, e)
	}
	return ti, nil
}

// attachFragments assembles moof samples onto the parsed tracks (B1).
// Plain moov tables win when present; empty-moov frag tracks get their
// samples from moofs. Mixed (moov samples + moofs) appends fragments
// after the base tables in file order. A video track with neither fails
// with the pre-B1 sentinel so old gates keep their bucket.
func attachFragments(m *Movie, moovPayload []byte, moofs []moofData) error {
	if len(moofs) == 0 {
		return fmt.Errorf("%w", ErrFragmented)
	}
	m.FragCount = len(moofs)
	trexMap := parseMvexFromMoov(moovPayload)
	for _, t := range m.Tracks {
		if t.Handler != "vide" {
			continue
		}
		baseN := len(t.Samples)
		var baseEnd int64
		if baseN > 0 {
			for _, s := range t.Samples {
				if e := s.PTS + int64(s.fragDur); e > baseEnd {
					baseEnd = e
				}
			}
		}
		built, hasCTTS, err := assembleFragTrack(t.ID, trexMap, moofs, baseEnd)
		if err != nil {
			// No fragment samples for this track: keep base tables when
			// they exist, else keep the pre-B1 failure for this track.
			if baseN > 0 {
				continue
			}
			return err
		}
		if hasCTTS {
			t.HasCTTS = true
		}
		// Negative-CTS shift peer (mov_update_dts_shift): PTS = DTS +
		// shift + CTS so no stamp goes below the DTS base.
		appendFragSamples(t, built, len(moofs))
	}
	for _, t := range m.Tracks {
		if t.Handler == "vide" && len(t.Samples) > 0 {
			return nil
		}
	}
	return fmt.Errorf("%w: no fragment samples", ErrFragmented)
}

// appendFragSamples moves assembled samples onto the track tables with
// the negative-CTS shift applied (mov_update_dts_shift peer: PTS = DTS +
// shift + CTS so no stamp goes below the DTS base).
func appendFragSamples(t *Track, built []fragBuilt, fragCount int) {
	shift := fragCTSShift(built)
	start := len(t.Samples)
	for i, s := range built {
		pts := s.dts + shift + int64(s.cts)
		num := start + i + 1
		smp := Sample{
			Number:   num,
			Size:     s.size,
			Offset:   s.offset,
			DTS:      s.dts,
			PTS:      pts,
			DTSMs:    ticksToMs(s.dts, t.Timescale),
			PTSMs:    ticksToMs(pts, t.Timescale),
			Keyframe: s.keyframe,
			fragDur:  s.duration,
		}
		t.Samples = append(t.Samples, smp)
		if s.keyframe {
			t.Keyframes = append(t.Keyframes, Keyframe{
				SampleNumber: num,
				Offset:       s.offset,
				DTSMs:        smp.DTSMs,
				PTSMs:        smp.PTSMs,
			})
		}
	}
	t.SampleCount = len(t.Samples)
	t.FragCount = fragCount
	fillFragDuration(t)
}

// fragCTSShift returns the negative-CTS shift peer (mov_update_dts_shift).
func fragCTSShift(built []fragBuilt) int64 {
	var shift int64
	for _, s := range built {
		if int64(s.cts) < 0 && -int64(s.cts) > shift {
			shift = -int64(s.cts)
		}
	}
	return shift
}

func assembleFragTrack(trackID uint32, trexMap map[uint32]trexEntry, moofs []moofData, baseEndDTS int64) ([]fragBuilt, bool, error) {
	var out []fragBuilt
	trackEnd := baseEndDTS
	hasCTTS := false
	trex := trexMap[trackID]
	for _, mf := range moofs {
		implicit := mf.offset // mov_read_moof resets implicit to moof_offset
		c := &cursor{buf: mf.payload}
		for !c.done() {
			typ, p, err := c.next()
			if err != nil {
				return nil, false, fmt.Errorf("%w: moof child %v", ErrFragmented, err)
			}
			if typ != "traf" {
				continue
			}
			built, end, cts, err := assembleTraf(trackID, trex, p, mf.offset, &implicit, trackEnd)
			if err != nil {
				return nil, false, err
			}
			if built == nil {
				continue // traf for another track
			}
			out = append(out, built...)
			if cts {
				hasCTTS = true
			}
			trackEnd = end
		}
	}
	if len(out) == 0 {
		return nil, false, fmt.Errorf("%w: no fragment samples for track %d", ErrFragmented, trackID)
	}
	return out, hasCTTS, nil
}

// assembleTraf builds samples from one traf for trackID. Returns nil when
// the traf belongs to another track. implicit tracks the running base
// peer; trackEnd is the fallback DTS when tfdt is absent.
func assembleTraf(trackID uint32, trex trexEntry, payload []byte, moofOffset int64, implicit *int64, trackEnd int64) ([]fragBuilt, int64, bool, error) {
	c := &cursor{buf: payload}
	var tfhd *tfhdInfo
	var tfdt uint64
	hasTfdt := false
	var truns []trunInfo
	for !c.done() {
		typ, p, err := c.next()
		if err != nil {
			return nil, trackEnd, false, fmt.Errorf("%w: traf child %v", ErrFragmented, err)
		}
		switch typ {
		case "tfhd":
			ti, err := parseTfhd(p)
			if err != nil {
				return nil, trackEnd, false, err
			}
			tfhd = &ti
		case "tfdt":
			v, ok, err := parseTfdt(p)
			if err != nil {
				return nil, trackEnd, false, err
			}
			tfdt, hasTfdt = v, ok
		case "trun":
			ti, err := parseTrun(p)
			if err != nil {
				return nil, trackEnd, false, err
			}
			truns = append(truns, ti)
		default:
		}
	}
	if tfhd == nil {
		return nil, trackEnd, false, fmt.Errorf("%w: trun without tfhd", ErrFragmented)
	}
	if tfhd.trackID != trackID {
		return nil, trackEnd, false, nil
	}
	// Resolve defaults: trun entry -> tfhd -> trex (mov_read_tfhd peer).
	defDuration := trex.duration
	defSize := trex.size
	defFlags := trex.flags
	if tfhd.hasDuration {
		defDuration = tfhd.defaultDuration
	}
	if tfhd.hasSize {
		defSize = tfhd.defaultSize
	}
	if tfhd.hasFlags {
		defFlags = tfhd.defaultFlags
	}
	var base uint64
	switch {
	case tfhd.hasBase:
		base = tfhd.baseOffset
	case tfhd.baseIsMoof:
		base = uint64(moofOffset)
	default:
		base = uint64(*implicit)
	}
	dts := int64(tfdt)
	if !hasTfdt {
		dts = trackEnd
	}
	var out []fragBuilt
	hasCTTS := false
	for _, tr := range truns {
		off := int64(base)
		if tr.hasDataOffset {
			off += int64(tr.dataOffset)
		}
		if off < 0 {
			return nil, trackEnd, false, fmt.Errorf("%w: negative sample base", ErrFragmented)
		}
		for i, e := range tr.entries {
			dur := defDuration
			if e.hasDur {
				dur = e.duration
			}
			sz := defSize
			if e.hasSize {
				sz = e.size
			}
			fl := defFlags
			if i == 0 && tr.hasFirstFlags {
				fl = tr.firstFlags
			}
			if e.hasFlags {
				fl = e.flags
			}
			if dur == 0 {
				return nil, trackEnd, false, fmt.Errorf("%w: zero duration track %d", ErrFragmented, trackID)
			}
			if sz == 0 {
				return nil, trackEnd, false, fmt.Errorf("%w: zero size track %d", ErrFragmented, trackID)
			}
			var cts int32
			if e.hasCTS {
				cts = e.cts
				hasCTTS = true
			}
			key := (fl & (fragFlagNonSync | fragFlagDependsYes)) == 0
			out = append(out, fragBuilt{
				offset:   uint64(off),
				size:     sz,
				dts:      dts,
				cts:      cts,
				duration: dur,
				keyframe: key,
			})
			off += int64(sz)
			dts += int64(dur)
		}
		*implicit = off
	}
	if len(out) == 0 {
		return nil, trackEnd, false, nil
	}
	return out, dts, hasCTTS, nil
}
