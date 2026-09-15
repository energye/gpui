package video

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"testing"
)

type a1PCMClip struct {
	File          string `json:"file"`
	DecodeFrom    int    `json:"decode_from"`
	DecodeCount   int    `json:"decode_count"`
	OursFirst     int    `json:"ours_first"`
	Ref           string `json:"ref"`
	RefFirstFrame int    `json:"ref_first_frame"`
	Frames        int    `json:"frames"`
}

type a1PCMBaseline struct {
	Clips  []a1PCMClip `json:"clips"`
	MaxRMS float64     `json:"max_rms"`
	MaxAbs float64     `json:"max_abs"`
}

func loadA1PCMBaseline(t *testing.T) a1PCMBaseline {
	t.Helper()
	buf, err := os.ReadFile("testdata/a1_pcm.json")
	if err != nil {
		t.Skipf("a1 pcm baseline missing: %v", err)
	}
	var b a1PCMBaseline
	if err := json.Unmarshal(buf, &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Clips) == 0 {
		t.Fatal("empty pcm baseline")
	}
	return b
}

// TestA1PCMFrames pins the A1 landing-2 waveform: the first packets of
// both warehouse clips decode (in order, keeping overlap state) to the
// checked-in ffmpeg f32le reference within 1e-6 rms / 1e-5 maxabs.
// oceans aligns packet-to-frame; f42906 skips the 2 priming packets its
// edit list trims in ffmpeg. Short/START/STOP windows ride along in the
// head packets; PCE/960-frame clips stay honest ErrUnsupported.
func TestA1PCMFrames(t *testing.T) {
	b := loadA1PCMBaseline(t)
	for _, c := range b.Clips {
		refb, err := os.ReadFile("testdata/" + c.Ref)
		if err != nil {
			t.Skipf("%s ref missing: %v", c.Ref, err)
		}
		if len(refb) < c.Frames*2048*4 {
			t.Fatalf("%s ref = %d bytes, want >= %d", c.Ref, len(refb), c.Frames*2048*4)
		}
		ref := make([]float32, c.Frames*2048)
		for i := range ref {
			ref[i] = math.Float32frombits(binary.LittleEndian.Uint32(refb[i*4:]))
		}
		path := "testdata/" + c.File
		info, err := ProbeAudio(path)
		if err != nil {
			t.Skipf("%s probe: %v", c.File, err)
		}
		d, err := NewAudioDecoder(CodecAAC)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.Configure(info.ASC); err != nil {
			t.Fatalf("%s configure: %v", c.File, err)
		}
		var outs [][]float32
		for i := c.DecodeFrom; i < c.DecodeFrom+c.DecodeCount; i++ {
			pkt, pts, err := ReadAudioPacket(path, i)
			if err != nil {
				t.Fatalf("%s read pkt%d: %v", c.File, i, err)
			}
			pcm, err := d.DecodePacket(pkt, pts)
			if err != nil {
				t.Fatalf("%s decode pkt%d: %v", c.File, i, err)
			}
			if pcm.Samples != 1024 || pcm.Channels != 2 || len(pcm.Data) != 2048 {
				t.Fatalf("%s pkt%d shape = %d/%d/%d, want 1024/2/2048",
					c.File, i, pcm.Samples, pcm.Channels, len(pcm.Data))
			}
			if pcm.PTSMs != pts {
				t.Fatalf("%s pkt%d pts = %d, want %d", c.File, i, pcm.PTSMs, pts)
			}
			outs = append(outs, append([]float32(nil), pcm.Data...))
		}
		var se, mx float64
		var n int
		for f := 0; f < c.Frames; f++ {
			mine := outs[c.OursFirst-c.DecodeFrom+f]
			for k := 0; k < 2048; k++ {
				dd := float64(mine[k] - ref[(c.RefFirstFrame+f)*2048+k])
				se += dd * dd
				if a := math.Abs(dd); a > mx {
					mx = a
				}
				n++
			}
		}
		rms := math.Sqrt(se / float64(n))
		t.Logf("%s rms=%.9f maxabs=%.9f", c.File, rms, mx)
		if rms > b.MaxRMS || mx > b.MaxAbs {
			t.Fatalf("%s rms=%.9f max=%.9f, want rms<=%g max<=%g",
				c.File, rms, mx, b.MaxRMS, b.MaxAbs)
		}
	}
}
