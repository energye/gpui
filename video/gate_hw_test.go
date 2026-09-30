//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package video

import (
	"errors"
	"runtime"
	"testing"
	"time"
)

// P1 硬解门禁（播放器侧）：Info/Stats 四键齐备且读真值。
// 真机有硬解就断言在流，无硬解就断言回落数 honest + 照播。
func TestGateHWStatsKeys(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_720p.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Skipf("ffmpeg backend unavailable: %v", err)
	}
	defer p.Close()
	info := p.Info()
	if info.HWName == "" {
		t.Fatal("info hw_name empty, key missing")
	}
	// 播几帧让水位流动。
	shown := 0
	deadline := time.Now().Add(30 * time.Second)
	for shown < 3 && time.Now().Before(deadline) {
		h.now += 200
		if fr, _ := p.Poll(); fr != nil {
			shown++
		}
		runtime.Gosched()
	}
	if shown < 3 {
		t.Fatalf("shown=%d, want >=3", shown)
	}
	st := p.Stats()
	t.Logf("hw info(active=%v name=%q fb=%d) stats(active=%v name=%q fb=%d avg=%.3fms)",
		info.HWActive, info.HWName, info.HWFallbacks,
		st.HWActive, st.HWName, st.HWFallbacks, st.HWTransferMsAvg)
	if st.HWName == "" {
		t.Fatal("stats hw_name empty, key missing")
	}
	if st.HWName != info.HWName {
		t.Fatalf("stats name=%q != info name=%q", st.HWName, info.HWName)
	}
	if st.HWFallbacks < info.HWFallbacks {
		t.Fatalf("stats fallbacks=%d < info fallbacks=%d, went backwards",
			st.HWFallbacks, info.HWFallbacks)
	}
	if st.HWActive {
		if st.HWName == "soft" {
			t.Fatalf("active with name soft, dishonest")
		}
		if st.HWTransferMsAvg < 0 {
			t.Fatalf("negative transfer avg")
		}
	} else if st.HWFallbacks < 1 {
		t.Fatalf("inactive with fallbacks=%d, silent fallback", st.HWFallbacks)
	}
}

// 强制软解档：四键老实（soft + 不 active），片子照播。
func TestGateHWForcedSoft(t *testing.T) {
	t.Setenv("GPUI_VIDEO_HW", "off")
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_720p.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Skipf("ffmpeg backend unavailable: %v", err)
	}
	defer p.Close()
	info := p.Info()
	if info.HWActive || info.HWName != "soft" {
		t.Fatalf("forced soft info = active=%v name=%q", info.HWActive, info.HWName)
	}
	h.now += 200
	fr, _ := p.Poll()
	if fr == nil {
		t.Fatal("forced soft produced no frame")
	}
	st := p.Stats()
	if st.HWActive || st.HWName != "soft" {
		t.Fatalf("forced soft stats = active=%v name=%q", st.HWActive, st.HWName)
	}
}

// 硬解错进新桶，不淹没在打不开里；普通坏片还是坏片桶。
// 各系统关键字都在这（Linux vaapi/vdpau/drm-prime，Windows
// d3d11va/d3d12va/dxva2/qsv/cuda，macOS videotoolbox，安卓 mediacodec）。
func TestGateHWFaultBucket(t *testing.T) {
	for _, msg := range []string{
		"ffmpeg: av_hwframe_transfer_data failed",
		"ffmpeg: vaapi device lost",
		"ffmpeg: d3d11va device lost",
		"ffmpeg: dxva2 init failed",
		"ffmpeg: videotoolbox session broken",
		"ffmpeg: mediacodec output failed",
	} {
		if got := Classify(errors.New(msg)); got.Kind != KindHwFallback {
			t.Fatalf("%q kind=%q, want %q", msg, got.Kind, KindHwFallback)
		}
	}
	if got := Classify(errors.New("ffmpeg: open xxx: no such file")); got.Kind != KindBadClip {
		t.Fatalf("plain ffmpeg err kind=%q, want %q", got.Kind, KindBadClip)
	}
}
