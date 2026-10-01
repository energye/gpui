//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type videoUploadSize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type videoUploadCases struct {
	Aligned   []videoUploadSize `json:"aligned"`
	Unaligned []videoUploadSize `json:"unaligned"`
	Reuse     struct {
		W      int `json:"w"`
		H      int `json:"h"`
		OtherW int `json:"otherW"`
		OtherH int `json:"otherH"`
	} `json:"reuse"`
	Draw struct {
		W    int     `json:"w"`
		H    int     `json:"h"`
		X    float64 `json:"x"`
		Y    float64 `json:"y"`
		DstW float64 `json:"dstW"`
		DstH float64 `json:"dstH"`
	} `json:"draw"`
}

func loadVideoUploadCases(t *testing.T) videoUploadCases {
	t.Helper()
	path := filepath.Join("testdata", "video_direct_upload.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v videoUploadCases
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(v.Aligned) == 0 || len(v.Unaligned) == 0 {
		t.Fatalf("%s has no cases", path)
	}
	return v
}

func TestVideoUploadAlignedCountsOnce(t *testing.T) {
	cases := loadVideoUploadCases(t)
	dev := openNoopVideoDevice(t)
	pool := NewVideoTexturePool(dev)
	defer pool.Close()
	beforePool := pool.Stats().Uploads
	beforeTotal := VideoUploadTotal()
	for _, c := range cases.Aligned {
		s, err := pool.Acquire(c.W, c.H)
		if err != nil {
			t.Fatalf("Acquire %dx%d: %v", c.W, c.H, err)
		}
		pix := make([]byte, c.W*c.H*4)
		if err := pool.Upload(s, pix); err != nil {
			t.Fatalf("Upload %dx%d: %v", c.W, c.H, err)
		}
	}
	st := pool.Stats()
	if got := st.Uploads - beforePool; got != uint64(len(cases.Aligned)) {
		t.Fatalf("pool uploads delta=%d, want %d", got, len(cases.Aligned))
	}
	if got := VideoUploadTotal() - beforeTotal; got != uint64(len(cases.Aligned)) {
		t.Fatalf("total uploads delta=%d, want %d", got, len(cases.Aligned))
	}
	if st.Evictions != 0 {
		t.Fatalf("Evictions=%d, want 0", st.Evictions)
	}
}

func TestVideoUploadRejectsBadInput(t *testing.T) {
	cases := loadVideoUploadCases(t)
	dev := openNoopVideoDevice(t)
	pool := NewVideoTexturePool(dev)
	defer pool.Close()
	for _, c := range cases.Unaligned {
		s, err := pool.Acquire(c.W, c.H)
		if err != nil {
			t.Fatalf("Acquire %dx%d: %v", c.W, c.H, err)
		}
		if err := pool.Upload(s, make([]byte, c.W*c.H*4)); err == nil {
			t.Fatalf("Upload %dx%d should fail row alignment", c.W, c.H)
		}
		pool.Release(s)
	}
	s, err := pool.Acquire(cases.Aligned[0].W, cases.Aligned[0].H)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := pool.Upload(s, make([]byte, 10)); err == nil {
		t.Fatal("Upload short pixels should fail")
	}
	pool.Release(s)
}

func TestVideoAcquireForFrameReuse(t *testing.T) {
	cases := loadVideoUploadCases(t)
	dev := openNoopVideoDevice(t)
	pool := NewVideoTexturePool(dev)
	defer pool.Close()
	r := cases.Reuse
	s, err := pool.Acquire(r.W, r.H)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	same, rebuilt, err := pool.AcquireForFrame(s, r.W, r.H)
	if err != nil || rebuilt || same != s {
		t.Fatalf("same size reuse rebuilt=%v err=%v", rebuilt, err)
	}
	nxt, rebuilt, err := pool.AcquireForFrame(same, r.OtherW, r.OtherH)
	if err != nil || !rebuilt || nxt == nil {
		t.Fatalf("size change rebuilt=%v err=%v", rebuilt, err)
	}
	if nxt.W != r.OtherW || nxt.H != r.OtherH {
		t.Fatalf("new slot size=%dx%d, want %dx%d", nxt.W, nxt.H, r.OtherW, r.OtherH)
	}
	pool.Release(nxt)
	if _, _, err := pool.AcquireForFrame(nil, 0, 10); err == nil {
		t.Fatal("AcquireForFrame 0 width should fail")
	}
	var nilPool *VideoTexturePool
	if _, _, err := nilPool.AcquireForFrame(nil, r.W, r.H); err == nil {
		t.Fatal("AcquireForFrame on nil pool should fail")
	}
}

func TestVideoDrawFrameMarksDamage(t *testing.T) {
	cases := loadVideoUploadCases(t)
	d := cases.Draw
	dc := NewContext(64, 64)
	img, err := NewImageBuf(d.W, d.H, FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	if !dc.DrawVideoFrame(img, VideoDrawOptions{X: d.X, Y: d.Y, DstWidth: d.DstW, DstHeight: d.DstH}) {
		t.Fatal("DrawVideoFrame=false, want true")
	}
	if len(dc.FrameDamage()) == 0 {
		t.Fatal("DrawVideoFrame left no damage")
	}
	if dc.DrawVideoFrame(nil, VideoDrawOptions{}) {
		t.Fatal("DrawVideoFrame(nil)=true, want false")
	}
}

func TestVideoSlotFailClosedWithoutGPU(t *testing.T) {
	cases := loadVideoUploadCases(t)
	dev := openNoopVideoDevice(t)
	pool := NewVideoTexturePool(dev)
	defer pool.Close()
	s, err := pool.Acquire(cases.Aligned[0].W, cases.Aligned[0].H)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if s.packedView().IsNil() {
		t.Fatal("packedView is nil, want packed handle")
	}
	dc := NewContext(64, 64)
	if dc.DrawVideoSlot(nil, VideoDrawOptions{}) {
		t.Fatal("DrawVideoSlot(nil)=true, want false")
	}
	neg := *s
	neg.Texture = nil
	if dc.DrawVideoSlot(&neg, VideoDrawOptions{}) {
		t.Fatal("DrawVideoSlot without texture=true, want false")
	}
	if dc.DrawVideoSlot(s, VideoDrawOptions{Opacity: -1}) {
		t.Fatal("DrawVideoSlot negative opacity=true, want false")
	}
	if dc.DrawVideoSlot(s, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18}) {
		t.Fatal("DrawVideoSlot without GPU=true, want false (fail closed to generic path)")
	}
	pool.Release(s)
}

func TestVideoBridgeFallbackConverged(t *testing.T) {
	cases := loadVideoUploadCases(t)
	c := cases.Aligned[0]
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	dc := NewContext(64, 64)
	pix := make([]byte, c.W*c.H*4)
	for i := range pix {
		pix[i] = 0x7f
	}
	// Noop has no GPU session: Upload succeeds, DrawVideoSlot fails
	// closed, bridge converges to one block-copy fallback draw.
	direct, ok := br.Show(dc, c.W, c.H, pix, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok {
		t.Fatal("Bridge Show ok=false, want true via fallback")
	}
	if direct {
		t.Fatal("Bridge Show direct=true without GPU, want fallback")
	}
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 1 || st.Fallbacks != 1 {
		t.Fatalf("bridge stats=%+v, want frames=1 uploads=1 fallbacks=1", st)
	}
	if st.Evictions != 0 {
		t.Fatalf("bridge evictions=%d, want 0", st.Evictions)
	}
	if len(dc.FrameDamage()) == 0 {
		t.Fatal("Bridge fallback left no damage")
	}
	// Second frame same size reuses slot: uploads grow, no rebuild.
	direct, ok = br.Show(dc, c.W, c.H, pix, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("second Show direct=%v ok=%v, want false/true", direct, ok)
	}
	if st := br.Stats(); st.Frames != 2 || st.Uploads != 2 || st.Fallbacks != 2 {
		t.Fatalf("second stats=%+v, want 2/2/2", st)
	}
	// Bad args draw nothing.
	if _, ok := br.Show(dc, 0, 10, pix, VideoDrawOptions{}); ok {
		t.Fatal("Show bad size ok=true, want false")
	}
	if _, ok := br.Show(nil, c.W, c.H, pix, VideoDrawOptions{}); ok {
		t.Fatal("Show nil context ok=true, want false")
	}
}

func TestVideoBridgeNilDeviceFallbackOnly(t *testing.T) {
	cases := loadVideoUploadCases(t)
	c := cases.Aligned[0]
	br := NewVideoBridge(nil)
	defer br.Close()
	dc := NewContext(64, 64)
	pix := make([]byte, c.W*c.H*4)
	direct, ok := br.Show(dc, c.W, c.H, pix, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("nil-device Show direct=%v ok=%v, want false/true", direct, ok)
	}
	if st := br.Stats(); st.Frames != 1 || st.Uploads != 0 || st.Fallbacks != 1 {
		t.Fatalf("nil-device stats=%+v, want 1/0/1", st)
	}
}

func TestVideoBridgeTickUploadPaintDraw(t *testing.T) {
	cases := loadVideoUploadCases(t)
	c := cases.Aligned[0]
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	pix := make([]byte, c.W*c.H*4)
	for i := range pix {
		pix[i] = 0x7f
	}
	// Tick: upload copies synchronously (noop uploads fine).
	if !br.UploadFrame(c.W, c.H, pix) {
		t.Fatal("UploadFrame = false, want true")
	}
	// Paint: draw without a GPU session falls back honestly.
	dc := NewContext(64, 64)
	direct, ok := br.DrawCurrent(dc, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("DrawCurrent direct=%v ok=%v, want false/true", direct, ok)
	}
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 1 || st.Fallbacks != 1 {
		t.Fatalf("split stats=%+v, want frames=1 uploads=1 fallbacks=1", st)
	}
	// Bad upload draws nothing.
	if br.UploadFrame(0, 10, nil) {
		t.Fatal("UploadFrame bad size = true, want false")
	}
	var nilBridge *VideoBridge
	if _, ok := nilBridge.DrawCurrent(dc, VideoDrawOptions{}); ok {
		t.Fatal("nil bridge DrawCurrent ok=true, want false")
	}
}

func TestVideoBridgeShowSeqUploadsOnce(t *testing.T) {
	cases := loadVideoUploadCases(t)
	c := cases.Aligned[0]
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	dc := NewContext(64, 64)
	pix := make([]byte, c.W*c.H*4)
	for i := range pix {
		pix[i] = 0x7f
	}
	// First view uploads (noop has no GPU session: falls back after upload).
	direct, ok := br.ShowSeq(7, dc, c.W, c.H, pix, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("first ShowSeq direct=%v ok=%v, want false/true", direct, ok)
	}
	// Second view same seq redraws without re-uploading.
	direct, ok = br.ShowSeq(7, dc, c.W, c.H, pix, VideoDrawOptions{X: 32, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("redraw ShowSeq direct=%v ok=%v, want false/true", direct, ok)
	}
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 1 || st.Fallbacks != 1 || st.Redraws != 1 {
		t.Fatalf("seq stats=%+v, want frames=1 uploads=1 fallbacks=1 redraws=1", st)
	}
	// New seq uploads again.
	if _, ok := br.ShowSeq(8, dc, c.W, c.H, pix, VideoDrawOptions{}); !ok {
		t.Fatal("new seq ShowSeq ok=false, want true")
	}
	if st := br.Stats(); st.Frames != 2 || st.Uploads != 2 {
		t.Fatalf("new seq stats=%+v, want frames=2 uploads=2", st)
	}
}

func TestVideoBridgeEnsureDevice(t *testing.T) {
	br := NewVideoBridge(nil)
	defer br.Close()
	dev := openNoopVideoDevice(t)
	br.EnsureDevice(dev)
	br.EnsureDevice(dev)
	dc := NewContext(64, 64)
	cases := loadVideoUploadCases(t)
	c := cases.Aligned[0]
	pix := make([]byte, c.W*c.H*4)
	if _, ok := br.Show(dc, c.W, c.H, pix, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18}); !ok {
		t.Fatal("Show after EnsureDevice ok=false, want true")
	}
	if st := br.Stats(); st.Uploads != 1 {
		t.Fatalf("stats=%+v, want uploads=1", st)
	}
}
