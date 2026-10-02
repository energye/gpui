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
	"math"
	"os"
	"path/filepath"
	"testing"

	ffmpeg "github.com/energye/gpui/video/ffmpeg"
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

func TestVideoPlanePoolBorrowReturnNoLeak(t *testing.T) {
	dev := openNoopVideoDevice(t)
	pool := NewVideoPlanePool(dev)
	defer pool.Close()
	s, err := pool.Acquire(1280, 720)
	if err != nil {
		t.Fatalf("Acquire 1280x720: %v", err)
	}
	if _, _, err := pool.AcquireForFrame(s, 1280, 720); err != nil {
		t.Fatalf("reuse: %v", err)
	}
	if _, _, err := pool.AcquireForFrame(s, 1920, 1080); err != nil {
		t.Fatalf("size change: %v", err)
	}
	pool.Release(s)
	st := pool.Stats()
	if st.Evictions != 0 {
		t.Fatalf("Evictions=%d, want 0", st.Evictions)
	}
	if _, err := pool.Acquire(127, 64); err == nil {
		t.Fatal("Acquire odd width ok=true, want error")
	}
}

func TestVideoPlaneUploadTightAndPadded(t *testing.T) {
	dev := openNoopVideoDevice(t)
	pool := NewVideoPlanePool(dev)
	defer pool.Close()
	// 1280 rows are 256-aligned: direct upload, no scratch.
	s, err := pool.Acquire(1280, 720)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	before := VideoUploadTotal()
	if err := pool.UploadPlanes(s, make([]byte, 1280*720), make([]byte, 1280*720/2)); err != nil {
		t.Fatalf("UploadPlanes tight: %v", err)
	}
	if s.padY != nil || s.padUV != nil {
		t.Fatal("tight upload allocated scratch, want direct")
	}
	// 1920 rows need padding: scratch once, reused after.
	p, err := pool.Acquire(1920, 1080)
	if err != nil {
		t.Fatalf("Acquire 1080p: %v", err)
	}
	y, uv := make([]byte, 1920*1080), make([]byte, 1920*1080/2)
	if err := pool.UploadPlanes(p, y, uv); err != nil {
		t.Fatalf("UploadPlanes padded: %v", err)
	}
	if len(p.padY) != 2048*1080 || len(p.padUV) != 2048*540 {
		t.Fatalf("scratch y=%d uv=%d, want %d/%d", len(p.padY), len(p.padUV), 2048*1080, 2048*540)
	}
	if err := pool.UploadPlanes(p, y, uv); err != nil {
		t.Fatalf("UploadPlanes reuse: %v", err)
	}
	if got := VideoUploadTotal() - before; got != 3 {
		t.Fatalf("total uploads delta=%d, want 3", got)
	}
	if err := pool.UploadPlanes(p, make([]byte, 10), uv); err == nil {
		t.Fatal("short Y ok=true, want error")
	}
	pool.Release(s)
	pool.Release(p)
}

func TestNV12ConvertPrimaries(t *testing.T) {
	w, h := 4, 2
	n, un := w*h, w*h/2
	// Black: Y=16, UV=128.
	y := make([]byte, n)
	uv := make([]byte, un)
	for i := range y {
		y[i] = 16
	}
	for i := range uv {
		uv[i] = 128
	}
	dst := make([]byte, n*4)
	nv12ToRGBA(dst, y, uv, w, h)
	for i := 0; i < n; i++ {
		if dst[4*i] > 1 || dst[4*i+1] > 1 || dst[4*i+2] > 1 || dst[4*i+3] != 255 {
			t.Fatalf("black pixel %d = %v, want ~0,0,0,255", i, dst[4*i:4*i+4])
		}
	}
	// White: Y=235, UV=128.
	for i := range y {
		y[i] = 235
	}
	nv12ToRGBA(dst, y, uv, w, h)
	for i := 0; i < n; i++ {
		if dst[4*i] < 254 || dst[4*i+1] < 254 || dst[4*i+2] < 254 {
			t.Fatalf("white pixel %d = %v, want ~255", i, dst[4*i:4*i+4])
		}
	}
	// Red: Y=81, U=90, V=240.
	for i := range y {
		y[i] = 81
	}
	for i := 0; i < un; i += 2 {
		uv[i], uv[i+1] = 90, 240
	}
	nv12ToRGBA(dst, y, uv, w, h)
	for i := 0; i < n; i++ {
		if dst[4*i] < 250 || dst[4*i+1] > 5 || dst[4*i+2] > 5 {
			t.Fatalf("red pixel %d = %v, want ~255,0,0", i, dst[4*i:4*i+4])
		}
	}
}

func TestVideoBridgePlanesFallback(t *testing.T) {
	dev := openNoopVideoDevice(t)
	br := NewVideoBridge(dev)
	defer br.Close()
	w, h := 64, 36
	y := make([]byte, w*h)
	uv := make([]byte, w*h/2)
	for i := range y {
		y[i] = 180
	}
	for i := range uv {
		uv[i] = 128
	}
	if !br.UploadPlanesFrame(w, h, y, uv) {
		t.Fatal("UploadPlanesFrame = false, want true")
	}
	dc := NewContext(64, 64)
	direct, ok := br.DrawCurrent(dc, VideoDrawOptions{X: 0, Y: 0, DstWidth: 32, DstHeight: 18})
	if !ok || direct {
		t.Fatalf("DrawCurrent direct=%v ok=%v, want false/true (GPU entry fails closed)", direct, ok)
	}
	st := br.Stats()
	if st.Frames != 1 || st.Uploads != 1 || st.Fallbacks != 1 {
		t.Fatalf("stats=%+v, want frames=1 uploads=1 fallbacks=1", st)
	}
	if len(dc.FrameDamage()) == 0 {
		t.Fatal("planes fallback left no damage")
	}
	if br.UploadPlanesFrame(127, 36, y, uv) {
		t.Fatal("odd width ok=true, want false")
	}
	var nilBridge *VideoBridge
	if nilBridge.UploadPlanesFrame(w, h, y, uv) {
		t.Fatal("nil bridge ok=true, want false")
	}
}

func TestNV12ConvertMatchesDecoder(t *testing.T) {
	// End-to-end parity, test-only oracle: decode one tracked clip both
	// shapes and compare decoder RGBA against nv12ToRGBA. Pinned from
	// measurement (mean ~1.1, over25 ~0.5% on edge pixels): mean <= 2,
	// over25 fraction <= 1%.
	path := "../video/testdata/vr2_720p.mp4"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("clip absent: %v", err)
	}
	dr, err := ffmpeg.Open(path)
	if err != nil {
		t.Skipf("open: %v", err)
	}
	var rgb []byte
	w, h := 0, 0
	for i := 0; i < 3; i++ {
		fr, err := dr.Next()
		if err != nil {
			dr.Close()
			t.Fatalf("rgba Next: %v", err)
		}
		if i == 0 {
			w, h = fr.Width, fr.Height
			rgb = append([]byte(nil), fr.Pix...)
		}
		fr.Release()
	}
	dr.Close()
	dn, err := ffmpeg.Open(path)
	if err != nil {
		t.Skipf("reopen: %v", err)
	}
	defer dn.Close()
	dn.SetNV12(true)
	var y, uv []byte
	for i := 0; i < 3; i++ {
		fr, err := dn.Next()
		if err != nil {
			t.Fatalf("nv12 Next: %v", err)
		}
		if i == 0 {
			y, uv = append([]byte(nil), fr.Y...), append([]byte(nil), fr.UV...)
		}
		fr.Release()
	}
	out := make([]byte, w*h*4)
	nv12ToRGBA(out, y, uv, w, h)
	n := w * h
	var sum, over25 float64
	for i := 0; i < n; i++ {
		worst := 0.0
		for c := 0; c < 3; c++ {
			d := math.Abs(float64(out[4*i+c]) - float64(rgb[4*i+c]))
			sum += d
			if d > worst {
				worst = d
			}
		}
		if worst > 25 {
			over25++
		}
	}
	mean, frac := sum/float64(3*n), over25/float64(n)
	t.Logf("nv12 convert parity: meanAbsDiff=%.3f over25=%.4f%%", mean, 100*frac)
	if mean > 2 {
		t.Fatalf("meanAbsDiff=%.3f, want <= 2", mean)
	}
	if frac > 0.01 {
		t.Fatalf("over25 fraction=%.4f%%, want <= 1%%", 100*frac)
	}
}
