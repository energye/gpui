package video

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/energye/gpui/video/aac"
	"github.com/energye/gpui/video/mp4"
)

type a1Packet struct {
	Size  int   `json:"size"`
	PTSMs int64 `json:"pts_ms"`
}

type a1Clip struct {
	File            string     `json:"file"`
	CodecName       string     `json:"codec_name"`
	Profile         string     `json:"profile"`
	SampleRate      int        `json:"sample_rate"`
	Channels        int        `json:"channels"`
	NbFrames        int        `json:"nb_frames"`
	ASCHex          string     `json:"asc_hex"`
	AOT             int        `json:"aot"`
	SamplingIndex   int        `json:"sampling_index"`
	ChanConfig      int        `json:"chan_config"`
	MP4ARate        uint32     `json:"mp4a_rate"`
	MP4AChannels    uint16     `json:"mp4a_channels"`
	MP4ABits        uint16     `json:"mp4a_bits"`
	AudioSamples    int        `json:"audio_samples"`
	AudioTimescale  uint32     `json:"audio_timescale"`
	AudioDurationMs int64      `json:"audio_duration_ms"`
	DurationTolMs   int64      `json:"duration_tolerance_ms"`
	FirstPackets    []a1Packet `json:"first_packets"`
}

type a1Baseline struct {
	Clips []a1Clip `json:"clips"`
}

func loadA1Baseline(t *testing.T) a1Baseline {
	t.Helper()
	buf, err := os.ReadFile("testdata/a1_ffmpeg.json")
	if err != nil {
		t.Fatal(err)
	}
	var b a1Baseline
	if err := json.Unmarshal(buf, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Clips) == 0 {
		t.Fatal("empty baseline")
	}
	return b
}

// TestA1AudioDemux pins the A1 landing-1 demux: ffprobe audio fields,
// esds ASC bytes, mp4a entry rate/channels, packet tables and first
// packet sizes match the checked-in baseline; video tables stay intact.
func TestA1AudioDemux(t *testing.T) {
	b := loadA1Baseline(t)
	for _, c := range b.Clips {
		path := "testdata/" + c.File
		movie, err := mp4.ParseFile(path)
		if err != nil {
			t.Fatalf("%s parse: %v", c.File, err)
		}
		if movie.Video == nil {
			t.Fatalf("%s: video lost after A1", c.File)
		}
		if movie.Audio == nil {
			t.Fatalf("%s: audio missing", c.File)
		}
		a := movie.Audio
		if a.Codec != "mp4a" {
			t.Fatalf("%s audio codec = %q, want mp4a", c.File, a.Codec)
		}
		if a.SampleRate != c.MP4ARate || a.Channels != c.MP4AChannels || a.BitsPerSample != c.MP4ABits {
			t.Fatalf("%s mp4a = %d/%d/%d, want %d/%d/%d", c.File, a.SampleRate, a.Channels, a.BitsPerSample, c.MP4ARate, c.MP4AChannels, c.MP4ABits)
		}
		if hex.EncodeToString(a.ASC) != c.ASCHex {
			t.Fatalf("%s asc = %x, want %s", c.File, a.ASC, c.ASCHex)
		}
		if a.SampleCount != c.AudioSamples {
			t.Fatalf("%s audio samples = %d, want %d", c.File, a.SampleCount, c.AudioSamples)
		}
		if a.Timescale != c.AudioTimescale {
			t.Fatalf("%s audio timescale = %d, want %d", c.File, a.Timescale, c.AudioTimescale)
		}
		tol := c.DurationTolMs
		if tol == 0 {
			tol = 50
		}
		if d := a.DurationMs - c.AudioDurationMs; d < -tol || d > tol {
			t.Fatalf("%s audio duration = %d, want %d +- %d", c.File, a.DurationMs, c.AudioDurationMs, tol)
		}
		for i, want := range c.FirstPackets {
			s, ok := a.SampleAt(i)
			if !ok {
				t.Fatalf("%s packet %d missing", c.File, i)
			}
			if int(s.Size) != want.Size || s.PTSMs != want.PTSMs {
				t.Fatalf("%s pkt%d = %d/%d, want %d/%d", c.File, i, s.Size, s.PTSMs, want.Size, want.PTSMs)
			}
			if !s.Keyframe {
				t.Fatalf("%s pkt%d not keyframe", c.File, i)
			}
		}
	}
}

// TestA1AudioInfo pins ProbeAudio + ASC decode + audio registry: profile,
// rate, channels and sample counts match ffprobe; Configure accepts the
// esds ASC; DecodePacket returns real 1024-sample PCM (landing 2);
// first packet bytes read back.
func TestA1AudioInfo(t *testing.T) {
	b := loadA1Baseline(t)
	found := false
	for _, s := range SupportedAudioCodecs() {
		if s == CodecAAC {
			found = true
		}
	}
	if !found {
		t.Fatalf("audio codecs %v lack %q", SupportedAudioCodecs(), CodecAAC)
	}
	for _, c := range b.Clips {
		path := "testdata/" + c.File
		info, err := ProbeAudio(path)
		if err != nil {
			t.Fatalf("%s probe audio: %v", c.File, err)
		}
		if info.Profile != c.Profile || info.SampleRate != c.SampleRate || info.Channels != c.Channels {
			t.Fatalf("%s info = %s/%d/%d, want %s/%d/%d", c.File, info.Profile, info.SampleRate, info.Channels, c.Profile, c.SampleRate, c.Channels)
		}
		if info.Samples != c.NbFrames {
			t.Fatalf("%s samples = %d, want %d (ffprobe nb_frames)", c.File, info.Samples, c.NbFrames)
		}
		asc, _ := hex.DecodeString(c.ASCHex)
		cfg, err := aac.ParseASC(asc, true)
		if err != nil {
			t.Fatalf("%s asc parse: %v", c.File, err)
		}
		if cfg.ObjectType != c.AOT || cfg.SamplingIndex != c.SamplingIndex || cfg.ChanConfig != c.ChanConfig {
			t.Fatalf("%s asc = %d/%d/%d, want %d/%d/%d", c.File, cfg.ObjectType, cfg.SamplingIndex, cfg.ChanConfig, c.AOT, c.SamplingIndex, c.ChanConfig)
		}
		d, err := NewAudioDecoder(CodecAAC)
		if err != nil {
			t.Fatalf("new audio decoder: %v", err)
		}
		if err := d.Configure(asc); err != nil {
			t.Fatalf("%s configure: %v", c.File, err)
		}
		pkt, pts, err := ReadAudioPacket(path, 0)
		if err != nil {
			t.Fatalf("%s read pkt0: %v", c.File, err)
		}
		if len(pkt) != c.FirstPackets[0].Size || pts != c.FirstPackets[0].PTSMs {
			t.Fatalf("%s pkt0 = %d/%d, want %d/%d", c.File, len(pkt), pts, c.FirstPackets[0].Size, c.FirstPackets[0].PTSMs)
		}
		pcm, err := d.DecodePacket(pkt, pts)
		if err != nil {
			t.Fatalf("%s decode pkt0: %v", c.File, err)
		}
		if pcm.Samples != 1024 || pcm.Channels != 2 || len(pcm.Data) != 2048 {
			t.Fatalf("%s pcm shape = %d/%d/%d, want 1024/2/2048",
				c.File, pcm.Samples, pcm.Channels, len(pcm.Data))
		}
		if pcm.PTSMs != pts {
			t.Fatalf("%s pcm pts = %d, want %d", c.File, pcm.PTSMs, pts)
		}
		if got := Classify(errors.Join(ErrNoAudio)).Kind; got != KindAudio {
			t.Fatalf("no-audio kind = %q, want %q", got, KindAudio)
		}
	}
}

// TestA1AudioFault pins readable audio failures: silent clips, junk and
// bad ASC/ADTS land in the audio bucket, never a panic.
func TestA1AudioFault(t *testing.T) {
	if _, err := ProbeAudio("testdata/vr2_720p.mp4"); !errors.Is(err, ErrNoAudio) {
		t.Fatalf("silent probe err = %v, want no-audio", err)
	}
	if got := Classify(ErrNoAudio).Kind; got != KindAudio {
		t.Fatalf("no-audio kind = %q, want %q", got, KindAudio)
	}
	d, err := NewAudioDecoder(CodecAAC)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Configure(nil); !errors.Is(err, aac.ErrBadASC) {
		t.Fatalf("empty asc err = %v, want bad-asc", err)
	}
	if err := d.Configure([]byte{0xff, 0xff, 0xff}); err == nil {
		t.Fatal("junk asc configures")
	} else if got := Classify(err).Kind; got != KindAudio {
		t.Fatalf("junk asc kind = %q, want %q", got, KindAudio)
	}
	if _, err := aac.ParseHeader([]byte("not adts header.....")); !errors.Is(err, aac.ErrBadADTS) {
		t.Fatalf("junk adts err = %v, want bad-adts", err)
	}
	if got := Classify(aac.ErrBadADTS).Kind; got != KindAudio {
		t.Fatalf("bad-adts kind = %q, want %q", got, KindAudio)
	}
	if _, err := d.DecodePacket(nil, 0); !errors.Is(err, aac.ErrBadASC) && !errors.Is(err, aac.ErrTruncated) {
		t.Fatalf("empty packet err = %v, want truncated/config", err)
	}
}
