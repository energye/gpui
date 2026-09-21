package h264

// B gate: parallel plan + prime execution equals sequential decode bit
// for bit. Oracle is the unmodified sequential path (same decoder code,
// same feed); the planner only reorders execution. Clips cover B
// chains, B-pyramid (B refs B), skip-heavy B, CABAC, and High profile.

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

type bClip struct {
	avcc   *AVCC
	frames [][][]byte
	fed    []bool
}

// bLoadClip demuxes one testdata clip into raw units per sample.
func bLoadClip(t *testing.T, path string) bClip {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
	}
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
	clip := bClip{avcc: avcc}
	for _, s := range v.Samples {
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", s.Number, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample %d: %v", s.Number, err)
		}
		clip.frames = append(clip.frames, units)
		fed := false
		for _, u := range units {
			if typ, _ := NALType(u); typ == NALSliceNonIDR || typ == NALSliceIDR {
				fed = true
				break
			}
		}
		clip.fed = append(clip.fed, fed)
	}
	return clip
}

// bDecodeOne runs one frame's units on dec through FinishPicture.
func bDecodeOne(dec *Decoder, units [][]byte) (*Picture, error) {
	for _, u := range units {
		if err := dec.DecodeNALU(u); err != nil {
			return nil, err
		}
	}
	return dec.FinishPicture()
}

func bPicKey(p *Picture) (w, h uint32, poc int32, fn uint32, idr bool) {
	return p.Width, p.Height, p.POC, p.FrameNum, p.IsIDR
}

func bCheckPic(t *testing.T, tag string, got, want *Picture) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s: nil mismatch got=%v want=%v", tag, got == nil, want == nil)
	}
	if got == nil {
		return
	}
	gw, gh, gp, gf, gi := bPicKey(got)
	ww, wh, wp, wf, wi := bPicKey(want)
	if gw != ww || gh != wh || gp != wp || gf != wf || gi != wi {
		t.Fatalf("%s: meta got=%dx%d poc=%d fn=%d idr=%v want=%dx%d poc=%d fn=%d idr=%v",
			tag, gw, gh, gp, gf, gi, ww, wh, wp, wf, wi)
	}
	if len(got.Y) != len(want.Y) || len(got.Cb) != len(want.Cb) || len(got.Cr) != len(want.Cr) {
		t.Fatalf("%s: plane size mismatch", tag)
	}
	for i := range got.Y {
		if got.Y[i] != want.Y[i] {
			t.Fatalf("%s: Y byte %d got=%d want=%d", tag, i, got.Y[i], want.Y[i])
		}
	}
	for i := range got.Cb {
		if got.Cb[i] != want.Cb[i] {
			t.Fatalf("%s: Cb byte %d got=%d want=%d", tag, i, got.Cb[i], want.Cb[i])
		}
	}
	for i := range got.Cr {
		if got.Cr[i] != want.Cr[i] {
			t.Fatalf("%s: Cr byte %d got=%d want=%d", tag, i, got.Cr[i], want.Cr[i])
		}
	}
}

// bRunParallel executes plans on a pooled-decoder crew (reused decoders
// prove PrimeFrame makes tasks self-contained), waiting on Refs.
// Waiting never holds a pool slot (a waiter blocks before acquiring),
// so deep chains cannot deadlock the crew.
func bRunParallel(t *testing.T, avcc *AVCC, frames [][][]byte, plans []FramePlan, workers int) (disp, aligned []*Picture) {
	t.Helper()
	n := len(frames)
	disp = make([]*Picture, n)
	aligned = make([]*Picture, n)
	done := make([]chan struct{}, n)
	for i := range done {
		done[i] = make(chan struct{})
	}
	pool := make([]*Decoder, workers)
	free := make([]bool, workers)
	for w := range pool {
		wps := NewParamSets()
		if err := wps.FromAVCC(avcc); err != nil {
			t.Fatalf("worker sets: %v", err)
		}
		pool[w] = NewDecoder(wps)
		free[w] = true
	}
	var poolMu sync.Mutex
	take := func() (int, *Decoder) {
		for {
			poolMu.Lock()
			for w := range pool {
				if free[w] {
					free[w] = false
					d := pool[w]
					poolMu.Unlock()
					return w, d
				}
			}
			poolMu.Unlock()
			runtime.Gosched()
		}
	}
	give := func(w int) {
		poolMu.Lock()
		free[w] = true
		poolMu.Unlock()
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for f := range frames {
		if !plans[f].Fed {
			close(done[f])
			continue
		}
		wg.Add(1)
		go func(f int) {
			defer wg.Done()
			// Wait for the full roster, not just Refs: snapshot
			// members are all completed pictures the worker reads
			// (DPB order included); Refs ⊆ snapshot for single-slice
			// frames, union covers multi-slice marking drift. All
			// indices are strictly earlier, so no wait cycle.
			waited := make(map[int]bool, len(plans[f].Snapshot)+len(plans[f].Refs))
			for _, r := range plans[f].Snapshot {
				if !waited[r] {
					waited[r] = true
					<-done[r]
				}
			}
			for _, r := range plans[f].Refs {
				if !waited[r] {
					waited[r] = true
					<-done[r]
				}
			}
			mu.Lock()
			if firstErr != nil {
				mu.Unlock()
				close(done[f])
				return
			}
			mu.Unlock()
			w, dec := take()
			snap := make([]*Picture, len(plans[f].Snapshot))
			for i, si := range plans[f].Snapshot {
				snap[i] = aligned[si]
				if snap[i] == nil {
					give(w)
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("frame %d: snapshot %d not stored", f, si)
					}
					mu.Unlock()
					close(done[f])
					return
				}
			}
			dec.PrimeFrame(snap, plans[f].Seed)
			pic, err := bDecodeOne(dec, frames[f])
			stored := dec.lastStored
			give(w)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				close(done[f])
				return
			}
			disp[f] = pic
			aligned[f] = stored
			close(done[f])
		}(f)
	}
	wg.Wait()
	if firstErr != nil {
		t.Fatalf("parallel: %v", firstErr)
	}
	return disp, aligned
}

// bCheckStored pins the aligned reference picture (what snapshots
// share): nil-ness must agree, stored bytes must match exactly.
func bCheckStored(t *testing.T, tag string, f int, got, want *Picture) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("%s frame %d: stored nil mismatch", tag, f)
	}
	if got == nil {
		return
	}
	bCheckPic(t, tag, got, want)
}

func bPlanSanity(t *testing.T, plans []FramePlan, fed []bool) {
	t.Helper()
	for f, p := range plans {
		if p.Fed != fed[f] {
			t.Fatalf("frame %d: plan fed=%v want=%v", f, p.Fed, fed[f])
		}
		if !p.Fed {
			continue
		}
		if p.IsIDR && len(p.Refs) != 0 {
			t.Fatalf("frame %d: IDR refs=%v want empty", f, p.Refs)
		}
		for _, r := range p.Refs {
			if r < 0 || r >= f {
				t.Fatalf("frame %d: ref %d not strictly earlier", f, r)
			}
			if !plans[r].Fed {
				t.Fatalf("frame %d: ref %d unfed", f, r)
			}
		}
		for _, s := range p.Snapshot {
			if s < 0 || s >= f {
				t.Fatalf("frame %d: snapshot %d not strictly earlier", f, s)
			}
		}
	}
}

func TestBPlanPrimeMatchesSequential(t *testing.T) {
	clips := []string{
		"../testdata/vr2_m_bframes.mp4",
		"../testdata/vr2_m_bpyr.mp4",
		"../testdata/vr2_b_mixed.mp4",
		"../testdata/vr2_b_skip.mp4",
		"../testdata/vr2_b_intra.mp4",
		"../testdata/vr2_h_cavlc.mp4",
		"../testdata/vr2_m_main.mp4",
		"../testdata/vr2_m_fadeout.mp4",
		"../testdata/vr2_720p.mp4",
		"../testdata/vr2_1080p.mp4",
	}
	for _, path := range clips {
		t.Run(path, func(t *testing.T) {
			clip := bLoadClip(t, path)
			plans, ok := PlanFrameGroup(clip.avcc, clip.frames)
			if !ok {
				t.Fatalf("plan refused clean clip %s", path)
			}
			bPlanSanity(t, plans, clip.fed)
			// Sequential oracle (unmodified path).
			basePS := NewParamSets()
			if err := basePS.FromAVCC(clip.avcc); err != nil {
				t.Fatalf("sets: %v", err)
			}
			base := NewDecoder(basePS)
			want := make([]*Picture, len(clip.frames))
			wantStored := make([]*Picture, len(clip.frames))
			for f, units := range clip.frames {
				if !clip.fed[f] {
					continue
				}
				pic, err := bDecodeOne(base, units)
				if err != nil {
					t.Fatalf("sequential frame %d: %v", f, err)
				}
				want[f] = pic
				wantStored[f] = base.lastStored
			}
			// Parallel plan+prime execution.
			got, gotStored := bRunParallel(t, clip.avcc, clip.frames, plans, 3)
			for f := range clip.frames {
				bCheckPic(t, path, got[f], want[f])
				bCheckStored(t, path, f, gotStored[f], wantStored[f])
			}
		})
	}
}

// TestBEnergyHead runs the same gate on the real-world clip head
// (external data: skips when absent, never fails the suite).
func TestBEnergyHead(t *testing.T) {
	path := os.Getenv("VIDEO_PATH")
	if path == "" {
		path = "/home/yanghy/视频/ENERGY Designer-项目新建&恢复.mp4"
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("energy clip absent: %v", err)
	}
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
	const head = 16
	n := head
	if len(v.Samples) < n {
		n = len(v.Samples)
	}
	var frames [][][]byte
	var fed []bool
	for i := 0; i < n; i++ {
		s := v.Samples[i]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split %d: %v", i, err)
		}
		frames = append(frames, units)
		fd := false
		for _, u := range units {
			if typ, _ := NALType(u); typ == NALSliceNonIDR || typ == NALSliceIDR {
				fd = true
				break
			}
		}
		fed = append(fed, fd)
	}
	plans, ok := PlanFrameGroup(avcc, frames)
	if !ok {
		t.Fatalf("plan refused energy head")
	}
	bPlanSanity(t, plans, fed)
	basePS := NewParamSets()
	if err := basePS.FromAVCC(avcc); err != nil {
		t.Fatalf("sets: %v", err)
	}
	base := NewDecoder(basePS)
	var want, wantStored []*Picture
	for f, units := range frames {
		if !fed[f] {
			want = append(want, nil)
			wantStored = append(wantStored, nil)
			continue
		}
		pic, err := bDecodeOne(base, units)
		if err != nil {
			t.Fatalf("sequential frame %d: %v", f, err)
		}
		want = append(want, pic)
		wantStored = append(wantStored, base.lastStored)
	}
	got, gotStored := bRunParallel(t, avcc, frames, plans, 4)
	for f := range frames {
		bCheckPic(t, "energy", got[f], want[f])
		bCheckStored(t, "energy", f, gotStored[f], wantStored[f])
	}
	runtime.Gosched()
}
