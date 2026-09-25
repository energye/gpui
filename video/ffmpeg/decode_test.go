package ffmpeg

import (
	"testing"
)

// First-frames smoke: opens a checked-in clip through the real .so and
// decodes three pictures. Skips when the library is absent (CI without
// artifacts), fails loudly when present but broken.
func TestDecodeFirstFrames(t *testing.T) {
	if !Available() {
		t.Skipf("ffmpeg lib missing: %v", loadErr)
	}
	v, err := Version()
	if err != nil || v == "" {
		t.Fatalf("version: %v %q", err, v)
	}
	t.Logf("lib %s version %s", LibPath(), v)
	d, err := Open("../testdata/vr2_720p.mp4")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	info := d.Info()
	if info.Width <= 0 || info.Height <= 0 {
		t.Fatalf("bad info %+v", info)
	}
	t.Logf("info %+v", info)
	for i := 0; i < 3; i++ {
		fr, err := d.Next()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if fr.Width != info.Width || fr.Height != info.Height {
			t.Fatalf("frame %d size %dx%d want %dx%d", i, fr.Width, fr.Height, info.Width, info.Height)
		}
		if len(fr.Pix) != fr.Width*fr.Height*4 {
			t.Fatalf("frame %d pix %d", i, len(fr.Pix))
		}
		// Not black, not flat: real pictures vary.
		var sum, sq uint64
		for _, b := range fr.Pix {
			sum += uint64(b)
			sq += uint64(b) * uint64(b)
		}
		mean := float64(sum) / float64(len(fr.Pix))
		variance := float64(sq)/float64(len(fr.Pix)) - mean*mean
		t.Logf("frame %d pts=%d mean=%.1f var=%.0f", i, fr.PTSMs, mean, variance)
		if variance < 10 {
			t.Fatalf("frame %d looks flat (var %.0f)", i, variance)
		}
	}
	// Seek back to zero and decode one frame: proves SeekTo + flush.
	if _, err := d.SeekTo(0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	fr, err := d.Next()
	if err != nil {
		t.Fatalf("post-seek next: %v", err)
	}
	if len(fr.Pix) != fr.Width*fr.Height*4 {
		t.Fatalf("post-seek pix %d", len(fr.Pix))
	}
}
