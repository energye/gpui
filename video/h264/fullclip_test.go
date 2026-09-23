package h264

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// Full-clip gates for the two 60fps reference clips (1080p and 2K).
// Both files are local captures, not committed fixtures: skip cleanly
// when absent (same rule as the other oracle-gated tests here).

type refClip struct {
	file    string
	w, h    int
	samples int // demuxed video samples; one picture per sample
}

var refClips = []refClip{
	{"1080p_1920_1080_60fps.mp4", 1920, 1080, 641},
	{"2k_2560_1440_60fps.mp4", 2560, 1440, 2304},
}

// decodeRefClipCount runs the shipped entry path (demux + AVCC split +
// DecodeNALU + FinishPicture) over every sample, checking size, and
// releasing each picture immediately. Memory stays flat (one live
// picture plus DPB refs) no matter the clip length: never accumulate
// all frames, or a 2304-frame 2K clip pins ~12GB and OOMs the machine.
func decodeRefClipCount(t *testing.T, path string, w, h, wantSamples int) {
	t.Helper()
	movie, err := mp4.ParseFile(path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	if v == nil {
		t.Fatal("no video track")
	}
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	ps := NewParamSets()
	if err := ps.FromAVCC(avcc); err != nil {
		t.Fatalf("param sets: %v", err)
	}
	dec := NewDecoder(ps)
	n := 0
	for _, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", s.Number, err)
		}
		for _, u := range units {
			if err := dec.DecodeNALU(u); err != nil {
				t.Fatalf("sample %d: %v", s.Number, err)
			}
		}
		pic, err := dec.FinishPicture()
		if err != nil {
			t.Fatalf("finish sample %d: %v", s.Number, err)
		}
		if int(pic.Width) != w || int(pic.Height) != h {
			gotW, gotH := pic.Width, pic.Height
			pic.Release()
			t.Fatalf("pic %d size %dx%d want %dx%d", n, gotW, gotH, w, h)
		}
		pic.Release()
		n++
	}
	if n != wantSamples {
		t.Fatalf("pictures %d want %d", n, wantSamples)
	}
}

// TestRefClipErrorFree decodes both reference clips end to end: every
// sample must decode and every picture must finish with zero errors.
// Streaming + immediate Release: flat memory on both clips.
func TestRefClipErrorFree(t *testing.T) {
	for _, c := range refClips {
		c := c
		t.Run(c.file, func(t *testing.T) {
			path := "../testdata/" + c.file
			if _, err := os.Stat(path); err != nil {
				t.Skipf("clip absent: %v", err)
			}
			decodeRefClipCount(t, path, c.w, c.h, c.samples)
		})
	}
}

// TestRefClipPrefixFFmpegParity decodes the first 12 display frames of
// each reference clip through the shipped path and compares every
// byte (Y, Cb, Cr) against the system ffmpeg decoder's rawvideo
// output. Skips when the clip or the ffmpeg binary is absent.
func TestRefClipPrefixFFmpegParity(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg binary absent: %v", err)
	}
	const wantDisp = 12
	for _, c := range refClips {
		c := c
		t.Run(c.file, func(t *testing.T) {
			path := "../testdata/" + c.file
			if _, err := os.Stat(path); err != nil {
				t.Skipf("clip absent: %v", err)
			}
			// Decode enough samples to cover the first GOP's head:
			// display POCs 0..2*(wantDisp-1) arrive within the first
			// wantDisp+2 samples for these clips' I/P/B pattern.
			movie, err := mp4.ParseFile(path)
			if err != nil {
				t.Fatalf("demux: %v", err)
			}
			v := movie.Video
			avcc, err := ParseAVCC(v.AVCConfig)
			if err != nil {
				t.Fatalf("avcc: %v", err)
			}
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer f.Close()
			ps := NewParamSets()
			if err := ps.FromAVCC(avcc); err != nil {
				t.Fatalf("param sets: %v", err)
			}
			dec := NewDecoder(ps)
			byPOC := map[int32]*Picture{}
			nsamp := wantDisp + 4
			if nsamp > len(v.Samples) {
				nsamp = len(v.Samples)
			}
			for _, s := range v.Samples[:nsamp] {
				buf := make([]byte, s.Size)
				if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
					t.Fatalf("sample %d: %v", s.Number, err)
				}
				units, err := SplitAVCC(buf, avcc.LengthSize)
				if err != nil {
					t.Fatalf("split sample %d: %v", s.Number, err)
				}
				for _, u := range units {
					if err := dec.DecodeNALU(u); err != nil {
						t.Fatalf("sample %d: %v", s.Number, err)
					}
				}
				pic, err := dec.FinishPicture()
				if err != nil {
					t.Fatalf("finish sample %d: %v", s.Number, err)
				}
				byPOC[pic.POC] = pic
			}
			defer func() {
				for _, p := range byPOC {
					p.Release()
				}
			}()
			var ordered []*Picture
			for i := 0; i < wantDisp; i++ {
				p, ok := byPOC[int32(i*2)]
				if !ok {
					t.Fatalf("display poc %d missing after %d samples", i*2, nsamp)
				}
				ordered = append(ordered, p)
			}
			cmd := exec.Command("ffmpeg", "-v", "error", "-i", path,
				"-frames:v", fmt.Sprint(wantDisp),
				"-f", "rawvideo", "-pix_fmt", "yuv420p", "-")
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatalf("pipe: %v", err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatalf("start: %v", err)
			}
			fs := c.w * c.h
			cs := fs / 4
			for i, pic := range ordered {
				ob := make([]byte, fs+2*cs)
				if _, err := io.ReadFull(pipe, ob); err != nil {
					t.Fatalf("ffmpeg frame %d: %v", i, err)
				}
				if !bytes.Equal(pic.Y, ob[:fs]) {
					bad := 0
					for j := range pic.Y {
						if pic.Y[j] != ob[j] {
							bad++
						}
					}
					t.Fatalf("frame %d (poc %d): %d luma bytes differ", i, pic.POC, bad)
				}
				if !bytes.Equal(pic.Cb, ob[fs:fs+cs]) || !bytes.Equal(pic.Cr, ob[fs+cs:]) {
					t.Fatalf("frame %d (poc %d): chroma differs", i, pic.POC)
				}
			}
			pipe.Close()
			if err := cmd.Wait(); err != nil {
				t.Fatalf("ffmpeg wait: %v", err)
			}
		})
	}
}
