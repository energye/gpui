package aac

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func baselineASC(t *testing.T) [][2]string {
	t.Helper()
	buf, err := os.ReadFile("../testdata/a1_ffmpeg.json")
	if err != nil {
		t.Fatal(err)
	}
	var b struct {
		Clips []struct {
			File     string `json:"file"`
			ASCHex   string `json:"asc_hex"`
			Profile  string `json:"profile"`
			AOT      int    `json:"aot"`
			Rate     int    `json:"sample_rate"`
			Channels int    `json:"channels"`
		} `json:"clips"`
	}
	if err := json.Unmarshal(buf, &b); err != nil {
		t.Fatal(err)
	}
	var out [][2]string
	for _, c := range b.Clips {
		out = append(out, [2]string{c.ASCHex, c.Profile})
	}
	return out
}

// TestConfigBaseline pins ASC decode against the checked-in A1 baseline
// (esds bytes from the two in-warehouse clips, ffprobe profile peer).
func TestConfigBaseline(t *testing.T) {
	v := baselineASC(t)
	if len(v) == 0 {
		t.Fatal("empty baseline")
	}
	for _, p := range v {
		asc, err := hex.DecodeString(p[0])
		if err != nil {
			t.Fatal(err)
		}
		c, err := ParseASC(asc, true)
		if err != nil {
			t.Fatalf("asc %s: %v", p[0], err)
		}
		if c.ProfileName() != p[1] {
			t.Fatalf("asc %s profile = %q, want %q", p[0], c.ProfileName(), p[1])
		}
		if c.ObjectType != AOTLC || c.Channels != 2 {
			t.Fatalf("asc %s = aot %d ch %d, want 2/2", p[0], c.ObjectType, c.Channels)
		}
		if c.FrameLength != 1024 {
			t.Fatalf("asc %s frame = %d, want 1024", p[0], c.FrameLength)
		}
	}
}

// TestConfigReject pins namable ASC failures (empty, truncated, bad rate,
// bad chan map) without panics.
func TestConfigReject(t *testing.T) {
	for _, b := range [][]byte{nil, {}, {0x12}, {0xff, 0xff}} {
		if _, err := ParseASC(b, true); err == nil {
			t.Fatalf("asc %x parses", b)
		}
	}
	// Explicit-rate escape with zero rate is invalid.
	if _, err := ParseASC([]byte{0x97, 0x70, 0x00, 0x00, 0x00, 0x20}, true); err == nil {
		t.Fatal("zero explicit rate parses")
	}
}

// TestADTSSplit pins bare-stream framing: one crafted 7-byte header +
// payload splits to one raw block; junk sync and truncation are namable.
func TestADTSSplit(t *testing.T) {
	payload := []byte{0x11, 0x22, 0x33, 0x44}
	frameLen := 7 + len(payload)
	hdr := []byte{
		0xff, 0xf1, 0x50,
		0x80 | byte((frameLen>>11)&0x03),
		byte((frameLen >> 3) & 0xff),
		byte(((frameLen & 0x07) << 5) | 0x1f),
		0xfc,
	}
	stream := append(append([]byte(nil), hdr...), payload...)
	blocks, err := SplitADTS(stream)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	if len(blocks) != 1 || len(blocks[0]) != len(payload) {
		t.Fatalf("blocks = %d x %d, want 1 x %d", len(blocks), len(blocks[0]), len(payload))
	}
	for i := range payload {
		if blocks[0][i] != payload[i] {
			t.Fatalf("byte %d = %02x, want %02x", i, blocks[0][i], payload[i])
		}
	}
	h, err := ParseHeader(stream)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	if h.ObjectType != AOTLC || h.SampleRate != 44100 || h.ChanConfig != 2 {
		t.Fatalf("header = %d/%d/%d, want 2/44100/2", h.ObjectType, h.SampleRate, h.ChanConfig)
	}
	if _, err := ParseHeader([]byte("not adts!")); err == nil {
		t.Fatal("junk header parses")
	}
	if _, err := SplitADTS(stream[:5]); err == nil {
		t.Fatal("truncated stream splits")
	}
}

// TestDecoderPending pins the honest boundary: Configure gates
// object type and channel config; DecodePacket errors namably on short
// packets instead of fake PCM.
func TestDecoderPending(t *testing.T) {
	v := baselineASC(t)
	asc, _ := hex.DecodeString(v[0][0])
	var d Decoder
	if err := d.Configure(asc); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if d.Config() == nil || d.Config().SampleRate == 0 {
		t.Fatal("config missing after configure")
	}
	if _, err := d.DecodePacket([]byte{0x21, 0x10}, 0); err == nil {
		t.Fatal("packet fakes PCM")
	}
	var e Decoder
	if _, err := e.DecodePacket([]byte{0x21}, 0); err == nil {
		t.Fatal("unconfigured packet decodes")
	}
	// PCE-required chan_config 0 stays unsupported.
	if err := e.Configure([]byte{0x12, 0x00}); err == nil {
		t.Fatal("chan0 configures")
	}
}
