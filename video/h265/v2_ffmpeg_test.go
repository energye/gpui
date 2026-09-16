package h265

// V2-1 H.265 header parity: the box must be recognized, the hvcC parsed,
// and the registry must answer the h265 name — without touching pixels.
// Baseline: ../testdata/v2_ffmpeg.json (ffprobe 4.4.2 + ffmpeg 4.4.2,
// §12 V2 row). The clip is tracked (4.8KB in repo); pixel bytes/md5 in
// the baseline are the V2-2 contrast source, not asserted here.
//
// Peer (read-only, no code copied): libavcodec/hevc/parse.c:79
// ff_hevc_decode_extradata (hvcC shape) + libavcodec/hevc/hevcdec.c:4190
// hevc_decode_init + :4271 ff_hevc_decoder (entry shape) +
// libavformat/mov.c:3419 (hvcC read with avcC).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

type v2HVCC struct {
	Version    int `json:"version"`
	ProfileIDC int `json:"profile_idc"`
	LevelIDC   int `json:"level_idc"`
	LengthSize int `json:"length_size"`
	VPSCount   int `json:"vps_count"`
	SPSCount   int `json:"sps_count"`
	PPSCount   int `json:"pps_count"`
	Arrays     int `json:"arrays"`
}

type v2Stream struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CodecName    string `json:"codec_name"`
	CodecTag     string `json:"codec_tag"`
	Profile      string `json:"profile"`
	Level        int    `json:"level"`
	AvgFrameRate string `json:"avg_frame_rate"`
	NbFrames     int    `json:"nb_frames"`
	Duration     string `json:"duration"`
}

type v2Clip struct {
	File   string   `json:"file"`
	Stream v2Stream `json:"stream"`
	HVCC   v2HVCC   `json:"hvcc"`
}

type v2Baseline struct {
	Clips []v2Clip `json:"clips"`
}

func loadV2Baseline(t *testing.T) v2Baseline {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "v2_ffmpeg.json"))
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	var b v2Baseline
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatalf("baseline json: %v", err)
	}
	if len(b.Clips) == 0 {
		t.Fatal("baseline has no clips")
	}
	return b
}

// TestV2HVCCParity pins header truth: box codec, display size, sample
// count and hvcC array/shape match the ffmpeg baseline on the same clip.
func TestV2HVCCParity(t *testing.T) {
	b := loadV2Baseline(t)
	for _, clip := range b.Clips {
		path := filepath.Join("..", "testdata", clip.File)
		m, err := mp4.ParseFile(path)
		if err != nil {
			t.Fatalf("%s: demux: %v", clip.File, err)
		}
		v := m.Video
		if v == nil {
			t.Fatalf("%s: no video track", clip.File)
		}
		if v.Codec != clip.Stream.CodecTag {
			t.Fatalf("%s: codec %q want %q", clip.File, v.Codec, clip.Stream.CodecTag)
		}
		if int(v.Width) != clip.Stream.Width || int(v.Height) != clip.Stream.Height {
			t.Fatalf("%s: size %dx%d want %dx%d", clip.File, v.Width, v.Height, clip.Stream.Width, clip.Stream.Height)
		}
		if len(v.Samples) != clip.Stream.NbFrames {
			t.Fatalf("%s: samples %d want %d", clip.File, len(v.Samples), clip.Stream.NbFrames)
		}
		if len(v.AVCConfig) != 0 {
			t.Fatalf("%s: AVCConfig %d bytes on HEVC track, want empty", clip.File, len(v.AVCConfig))
		}
		h, err := ParseHVCC(v.HEVCConfig)
		if err != nil {
			t.Fatalf("%s: hvcC: %v", clip.File, err)
		}
		if int(h.Profile) != clip.HVCC.ProfileIDC {
			t.Fatalf("%s: profile_idc %d want %d", clip.File, h.Profile, clip.HVCC.ProfileIDC)
		}
		if int(h.Level) != clip.HVCC.LevelIDC {
			t.Fatalf("%s: level %d want %d", clip.File, h.Level, clip.HVCC.LevelIDC)
		}
		if h.LengthSize != clip.HVCC.LengthSize {
			t.Fatalf("%s: length size %d want %d", clip.File, h.LengthSize, clip.HVCC.LengthSize)
		}
		if len(h.VPS) != clip.HVCC.VPSCount || len(h.SPS) != clip.HVCC.SPSCount || len(h.PPS) != clip.HVCC.PPSCount {
			t.Fatalf("%s: vps/sps/pps %d/%d/%d want %d/%d/%d", clip.File, len(h.VPS), len(h.SPS), len(h.PPS), clip.HVCC.VPSCount, clip.HVCC.SPSCount, clip.HVCC.PPSCount)
		}
	}
}

// TestV2SplitUnits pins sample framing: every sample of the clip splits
// through the hvcC length size into non-empty units with a readable 2-byte
// HEVC header (V2-1 headers only, no pixel assertion).
func TestV2SplitUnits(t *testing.T) {
	b := loadV2Baseline(t)
	for _, clip := range b.Clips {
		path := filepath.Join("..", "testdata", clip.File)
		m, err := mp4.ParseFile(path)
		if err != nil {
			t.Fatalf("%s: demux: %v", clip.File, err)
		}
		v := m.Video
		h, err := ParseHVCC(v.HEVCConfig)
		if err != nil {
			t.Fatalf("%s: hvcC: %v", clip.File, err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("%s: open: %v", clip.File, err)
		}
		for _, s := range v.Samples {
			buf := make([]byte, s.Size)
			if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
				f.Close()
				t.Fatalf("%s: sample %d unreadable: %v", clip.File, s.Number, err)
			}
			units, err := SplitHVCC(buf, h.LengthSize)
			if err != nil {
				f.Close()
				t.Fatalf("%s: split sample %d: %v", clip.File, s.Number, err)
			}
			for _, u := range units {
				if _, ok := NALType(u); !ok {
					f.Close()
					t.Fatalf("%s: sample %d unit without HEVC header", clip.File, s.Number)
				}
			}
		}
		f.Close()
	}
}

// TestV2HeadersRefusePixels pins the V2-1 honesty line: header units feed
// clean, but any slice payload and FinishPicture refuse with the namable
// error (V2-2 owns pixels), never a fake picture.
func TestV2HeadersRefusePixels(t *testing.T) {
	b := loadV2Baseline(t)
	for _, clip := range b.Clips {
		path := filepath.Join("..", "testdata", clip.File)
		m, err := mp4.ParseFile(path)
		if err != nil {
			t.Fatalf("%s: demux: %v", clip.File, err)
		}
		h, err := ParseHVCC(m.Video.HEVCConfig)
		if err != nil {
			t.Fatalf("%s: hvcC: %v", clip.File, err)
		}
		d := NewDecoder(h)
		for _, raw := range h.VPS {
			if err := d.DecodeNALU(raw); err != nil {
				t.Fatalf("%s: vps feed: %v", clip.File, err)
			}
		}
		for _, raw := range h.SPS {
			if err := d.DecodeNALU(raw); err != nil {
				t.Fatalf("%s: sps feed: %v", clip.File, err)
			}
		}
		for _, raw := range h.PPS {
			if err := d.DecodeNALU(raw); err != nil {
				t.Fatalf("%s: pps feed: %v", clip.File, err)
			}
		}
		// Slice payload (type 1, forbidden bit clear): must refuse.
		if err := d.DecodeNALU([]byte{0x02, 0x01, 0x00}); !IsNotDecodable(err) {
			t.Fatalf("%s: slice err = %v, want ErrNotDecodable", clip.File, err)
		}
		if _, err := d.FinishPicture(); !IsNotDecodable(err) {
			t.Fatalf("%s: finish err = %v, want ErrNotDecodable", clip.File, err)
		}
	}
}

// TestV2BadHVCC pins readable failure: short/version/bad units fail with
// the hvcC bucket, never a panic.
func TestV2BadHVCC(t *testing.T) {
	if _, err := ParseHVCC([]byte{1, 2, 3}); err == nil {
		t.Fatal("short hvcC parses")
	}
	bad := make([]byte, 24)
	bad[0] = 2
	if _, err := ParseHVCC(bad); err == nil {
		t.Fatal("bad version parses")
	}
	if _, err := SplitHVCC([]byte{0x00}, 3); err == nil {
		t.Fatal("bad length size splits")
	}
	if _, err := SplitHVCC([]byte{}, 4); err == nil {
		t.Fatal("empty sample splits")
	}
}
