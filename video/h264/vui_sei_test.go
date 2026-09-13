package h264

import (
	"os"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// VR2d gate: display markings match header tracking (ffprobe truth:
// SAR 1:1, 5 fps, studio/limited range, progressive, left chroma).
func TestVUIHeaderTracking(t *testing.T) {
	for _, tc := range []struct {
		clip    string
		reorder uint32
	}{
		{"../testdata/vr2_m_bframes.mp4", 2},
		{"../testdata/vr2_480p.mp4", 1},
		{"../testdata/vr2_m_main.mp4", 0},
	} {
		movie, err := mp4.ParseFile(tc.clip)
		if err != nil {
			t.Fatalf("%s demux: %v", tc.clip, err)
		}
		avcc, err := ParseAVCC(movie.Video.AVCConfig)
		if err != nil {
			t.Fatalf("%s avcc: %v", tc.clip, err)
		}
		s, err := ParseSPS(avcc.SPS[0])
		if err != nil {
			t.Fatalf("%s sps: %v", tc.clip, err)
		}
		if !s.VUIPresent || s.VUI == nil {
			t.Fatalf("%s: vui missing", tc.clip)
		}
		w, h := s.VUI.SAR()
		if w != 1 || h != 1 {
			t.Fatalf("%s sar = %d:%d want 1:1", tc.clip, w, h)
		}
		if s.VUI.FPS() != 5 {
			t.Fatalf("%s fps = %v want 5", tc.clip, s.VUI.FPS())
		}
		if s.VUI.FullRange {
			t.Fatalf("%s: full range, want studio/limited", tc.clip)
		}
		if s.VUI.PicStructPresent {
			t.Fatalf("%s: pic struct, want progressive", tc.clip)
		}
		if s.VUI.NumReorderFrames != tc.reorder {
			t.Fatalf("%s reorder = %d want %d", tc.clip, s.VUI.NumReorderFrames, tc.reorder)
		}
	}
}

// VR2d gate: the gate clip's SEI user data passes through (x264 build
// note, UUID dc45e9bd...eeef) and lands on Decoder.LastSEI.
func TestSEIUserDataPassthrough(t *testing.T) {
	mp4Path := "../testdata/vr2_m_bframes.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	v := movie.Video
	avcc, err := ParseAVCC(v.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	f, err := os.Open(mp4Path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	s := v.Samples[0]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		t.Fatalf("sample 0: %v", err)
	}
	units, err := SplitAVCC(buf, avcc.LengthSize)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	var raw []byte
	for _, u := range units {
		if typ, _ := NALType(u); typ == NALSei {
			raw = u
		}
	}
	if raw == nil {
		t.Fatal("sample 0: no sei")
	}
	msgs, err := ParseSEI(raw, nil)
	if err != nil {
		t.Fatalf("sei: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Type != SEIUserDataUnreg {
		t.Fatalf("sei types = %v want single type-5", msgs)
	}
	wantUUID := [16]byte{0xdc, 0x45, 0xe9, 0xbd, 0xe6, 0xd9, 0x48, 0xb7, 0x96, 0x2c, 0xd8, 0x20, 0xd9, 0x23, 0xee, 0xef}
	if msgs[0].UUID != wantUUID {
		t.Fatalf("uuid = %x want x264 build note", msgs[0].UUID)
	}
	if len(msgs[0].UserData) == 0 || string(msgs[0].UserData[:4]) != "x264" {
		t.Fatalf("user data head = %q want x264 note", msgs[0].UserData)
	}
	dec := NewDecoder(nil)
	for _, sp := range avcc.SPS {
		if err := dec.DecodeNALU(sp); err != nil {
			t.Fatalf("sps: %v", err)
		}
	}
	for _, pp := range avcc.PPS {
		if err := dec.DecodeNALU(pp); err != nil {
			t.Fatalf("pps: %v", err)
		}
	}
	if err := dec.DecodeNALU(raw); err != nil {
		t.Fatalf("sei nalu: %v", err)
	}
	if got := dec.LastSEI(); len(got) != 1 || got[0].Type != SEIUserDataUnreg {
		t.Fatalf("LastSEI = %v want single type-5", got)
	}
}

// SEI file vectors: recovery point and HRD-timed pic timing parse exact.
func TestSEIFileVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/sei_recovery.bin")
	if err != nil {
		t.Fatalf("recovery vector: %v", err)
	}
	msgs, err := ParseSEI(raw, nil)
	if err != nil {
		t.Fatalf("recovery: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("recovery messages = %d want 1", len(msgs))
	}
	rp := msgs[0].Recovery
	if msgs[0].Type != SEIRecoveryPoint || rp.FrameCnt != 2 || !rp.ExactMatch || rp.BrokenLink || rp.ChangingSlice != 1 {
		t.Fatalf("recovery = %+v want cnt2/exact/link-ok/changing1", rp)
	}
	raw, err = os.ReadFile("testdata/sei_pictiming.bin")
	if err != nil {
		t.Fatalf("pictiming vector: %v", err)
	}
	vui := &VUI{NalHRD: HRD{Present: true, CpbRemovalDelayLength: 4, DpbOutputDelayLength: 4}}
	msgs, err = ParseSEI(raw, vui)
	if err != nil {
		t.Fatalf("pictiming: %v", err)
	}
	pt := msgs[0].Timing
	if msgs[0].Type != SEIPicTiming || !pt.HasDelays || pt.CpbRemoval != 5 || pt.DpbOutput != 9 {
		t.Fatalf("timing = %+v want cpb5/dpb9", pt)
	}
}

// Crop geometry: the 480p clip codes 864x480 and displays 854x480.
func TestCropGeometry(t *testing.T) {
	mp4Path := "../testdata/vr2_480p.mp4"
	if _, err := os.Stat(mp4Path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
	movie, err := mp4.ParseFile(mp4Path)
	if err != nil {
		t.Fatalf("demux: %v", err)
	}
	avcc, err := ParseAVCC(movie.Video.AVCConfig)
	if err != nil {
		t.Fatalf("avcc: %v", err)
	}
	s, err := ParseSPS(avcc.SPS[0])
	if err != nil {
		t.Fatalf("sps: %v", err)
	}
	if !s.HasCropping || s.Width != 854 || s.Height != 480 {
		t.Fatalf("display = %dx%d crop %v want 854x480 cropped", s.Width, s.Height, s.HasCropping)
	}
	if s.AlignedWidth != 864 || s.AlignedHeight != 480 {
		t.Fatalf("aligned = %dx%d want 864x480", s.AlignedWidth, s.AlignedHeight)
	}
	full, err := NewPicture(864, 480)
	if err != nil {
		t.Fatalf("picture: %v", err)
	}
	for i := range full.Y {
		full.Y[i] = uint8(i % 251)
	}
	cr, err := full.Crop(854, 480)
	if err != nil {
		t.Fatalf("crop: %v", err)
	}
	if cr.Width != 854 || cr.Height != 480 || len(cr.Y) != 854*480 {
		t.Fatalf("cropped = %dx%d len %d", cr.Width, cr.Height, len(cr.Y))
	}
	if cr.Y[853] != full.Y[853] || cr.Y[854] != full.Y[864] {
		t.Fatal("crop kept the wrong columns")
	}
}
