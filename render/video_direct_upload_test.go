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
