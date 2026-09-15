package h264

// S3 gate: residual full-block batch (map + levels) equals the scalar留守
// bit for bit.
// Peer: ffmpeg libavcodec/cabac_functions.h:114-138 get_cabac_inline
// (table contents and mask-selected update, our batch reuses them
// unchanged) + libavcodec/x86/cabac.h:110-213 BRANCHLESS_GET_CABAC
// (register-kept low/range across bins, our pure-Go shape: locals across
// the block's bins) + libavcodec/h264_cabac.c:1591-1705
// decode_cabac_residual_internal (sig/last map shape, our batch covers
// map) + :1724-1765 STORE_BLOCK (level prefix + bypass suffix shape,
// our batch covers levels) against video/h264/cabac_s3.go; oracle is the
// scalar留守 itself (no new vectors here, VR2 exact pins pixels vs
// ffmpeg).
// Profile (2026-09-15, same machine): High detail clip h_8x8 CABAC
// ~38% of decode (SigLast+Levels+BinFast), Main m_main ~15%, Main 720p
// ~1% (mcParts/Deblock dominate). So the batch pays on High detail
// clips (h_8x8 ~1.29x, h_scale ~1.17x end-to-end fast vs scalar),
// Main large clips are insensitive (~0.9-1.03x, expected, not a FAIL).
// Pass lines: fast/scalar pictures agree byte-exact on every tracked
// CABAC clip; kernel fast/scalar agree and allocate zero; VR2 exact stays
// green in both modes (normal + GPUI_SCALAR_CONVERT=1 double run);
// benchmark reports the block-kernel multiple (no threshold: Main large
// clips are end-to-end insensitive, the kernel isolates the batch;
// High detail clips report their own end-to-end ratio in logs only).

import (
	"os"
	"testing"
)

// s3Clips are tracked CABAC clips (committed, all in git): every VR2
// CABAC-entropy clip so the map+levels batch runs on each residual
// shape (4x4 all cats + 8x8 luma + B reorder + weights + fade + two
// sizes). CAVLC clips are excluded by design (batch never runs there;
// their exact stays pinned by VR2 parity). Absent files FAIL, not skip.
// 1080p stays out (local-only, not in git, same rule as VR2).
var s3Clips = []string{
	"../testdata/vr2_m_main.mp4",
	"../testdata/vr2_h_8x8.mp4",
	"../testdata/vr2_h_scale.mp4",
	"../testdata/vr2_m_bframes.mp4",
	"../testdata/vr2_m_bpyr.mp4",
	"../testdata/vr2_m_fadeout.mp4",
	"../testdata/vr2_480p.mp4",
	"../testdata/vr2_720p.mp4",
}

func s3DecodeClip(t *testing.T, mp4Path string) []*Picture {
	t.Helper()
	if _, err := os.Stat(mp4Path); err != nil {
		t.Fatalf("tracked S3 clip %s absent: %v", mp4Path, err)
	}
	pics, _ := decodeClip(t, mp4Path, avccWrap)
	return pics
}

func s3PicturesEqual(t *testing.T, a, b []*Picture) {
	t.Helper()
	if len(a) != len(b) {
		t.Fatalf("picture count %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Width != b[i].Width || a[i].Height != b[i].Height {
			t.Fatalf("frame %d size %dx%d vs %dx%d", i, a[i].Width, a[i].Height, b[i].Width, b[i].Height)
		}
		if len(a[i].Y) != len(b[i].Y) || len(a[i].Cb) != len(b[i].Cb) || len(a[i].Cr) != len(b[i].Cr) {
			t.Fatalf("frame %d plane sizes differ", i)
		}
		for j := range a[i].Y {
			if a[i].Y[j] != b[i].Y[j] {
				t.Fatalf("frame %d Y byte %d fast=%d scalar=%d", i, j, a[i].Y[j], b[i].Y[j])
			}
		}
		for j := range a[i].Cb {
			if a[i].Cb[j] != b[i].Cb[j] {
				t.Fatalf("frame %d Cb byte %d fast=%d scalar=%d", i, j, a[i].Cb[j], b[i].Cb[j])
			}
		}
		for j := range a[i].Cr {
			if a[i].Cr[j] != b[i].Cr[j] {
				t.Fatalf("frame %d Cr byte %d fast=%d scalar=%d", i, j, a[i].Cr[j], b[i].Cr[j])
			}
		}
	}
}

// Fast batch agrees with the scalar留守 on full-clip pictures.
func TestS3BatchMatchesScalar(t *testing.T) {
	old := cabacScalarForced
	defer func() { cabacScalarForced = old }()
	for _, clip := range s3Clips {
		cabacScalarForced = false
		fast := s3DecodeClip(t, clip)
		cabacScalarForced = true
		scalar := s3DecodeClip(t, clip)
		s3PicturesEqual(t, fast, scalar)
	}
}

// Fast 4x4 + 8x8 block paths allocate zero (hot path: no per-block heap,
// same lock as S1 qpel/deblock).
func TestS3ZeroAlloc(t *testing.T) {
	old := cabacScalarForced
	defer func() { cabacScalarForced = old }()
	cabacScalarForced = false
	d := s3KernelDec()
	snapCtx := d.cabCtx
	snapLow, snapRng, snapPos, snapNbits := d.cab.low, d.cab.rng, d.cab.pos, d.cab.nbits
	reset := func() {
		d.cab.low, d.cab.rng, d.cab.pos, d.cab.nbits = snapLow, snapRng, snapPos, snapNbits
		d.cabCtx = snapCtx
	}
	if n := testing.AllocsPerRun(20, func() {
		reset()
		_, _, _ = d.cabacCoeffData(0, 16, 0)
	}); n != 0 {
		t.Fatalf("4x4 fast allocs = %v want 0", n)
	}
	if n := testing.AllocsPerRun(20, func() {
		reset()
		_, _, _ = d.cabacCoeffData8x8()
	}); n != 0 {
		t.Fatalf("8x8 fast allocs = %v want 0", n)
	}
}

// s3KernelDec builds one deterministic map-kernel stimulus (stimulus
// only; oracle is the scalar留守, so no golden file needed).
func s3KernelDec() *Decoder {
	buf := make([]byte, 8192)
	for i := range buf {
		buf[i] = byte(i*31 + 7)
	}
	d := NewDecoder(nil)
	tail := append(append([]byte(nil), buf...), make([]byte, 16)...)
	cab, err := newCabacDec(tail)
	if err != nil {
		panic(err)
	}
	d.cab = cab
	initCabacCtx(&d.cabCtx, cabacInitI[:], 26)
	return d
}

// Math baseline (no threads): one 4x4 block through fast vs scalar.
// Reports the S3 block-batch multiple; both sides must allocate zero and
// agree (equality is pinned by TestS3BatchMatchesScalar on real clips,
// this pins the kernel in isolation).
func BenchmarkS3MapKernelVsScalar(b *testing.B) {
	tmpl := s3KernelDec()
	snapCtx := tmpl.cabCtx
	snapLow, snapRng, snapPos, snapNbits := tmpl.cab.low, tmpl.cab.rng, tmpl.cab.pos, tmpl.cab.nbits
	old := cabacScalarForced
	defer func() { cabacScalarForced = old }()
	run := func(d *Decoder) {
		d.cab.low, d.cab.rng, d.cab.pos, d.cab.nbits = snapLow, snapRng, snapPos, snapNbits
		d.cabCtx = snapCtx
		for k := 0; k < 16; k++ {
			_, _, _ = d.cabacCoeffData(0, 16, 0)
		}
	}
	b.Run("fast", func(b *testing.B) {
		cabacScalarForced = false
		d := s3KernelDec()
		run(d)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			run(d)
		}
	})
	b.Run("scalar", func(b *testing.B) {
		cabacScalarForced = true
		d := s3KernelDec()
		run(d)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			run(d)
		}
	})
}
