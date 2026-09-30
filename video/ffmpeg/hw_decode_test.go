//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package ffmpeg

import (
	"testing"
)

// P1 硬解门禁只认真路：真设备建得起来就断言硬解帧在流，
// 建不起来就断言软解回落（fallbacks 如实 + 照样出画面），
// 禁止静默假绿。两种分支都是 PASS，但走的断言不同。
func TestHWRealPathOrHonestFallback(t *testing.T) {
	if !Available() {
		t.Skipf("ffmpeg lib missing: %v", loadErr)
	}
	t.Logf("hw types in build: %q", ListHWTypes())
	d, err := Open("../testdata/vr2_720p.mp4")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()
	info := d.Info()
	var sum, sq uint64
	n := 0
	for i := 0; i < 5; i++ {
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
		for _, b := range fr.Pix {
			sum += uint64(b)
			sq += uint64(b) * uint64(b)
		}
		n += len(fr.Pix)
	}
	mean := float64(sum) / float64(n)
	variance := float64(sq)/float64(n) - mean*mean
	st := d.HWStats()
	t.Logf("hw stats: active=%v name=%q fallbacks=%d xfer_avg=%.3fms mean=%.1f var=%.0f",
		st.Active, st.Name, st.Fallbacks, st.TransferMsAvg, mean, variance)
	if variance < 10 {
		t.Fatalf("frames look flat (var %.0f), decode broken", variance)
	}
	if st.Active {
		if st.Name == "" || st.Name == "soft" {
			t.Fatalf("active but name=%q, dishonest", st.Name)
		}
		if st.TransferMsAvg < 0 {
			t.Fatalf("negative transfer avg %f", st.TransferMsAvg)
		}
	} else {
		// 没流硬解帧：必须有一笔回落记录，名字老实记 soft，画面不断。
		if st.Name != "soft" {
			t.Fatalf("inactive but name=%q, want soft", st.Name)
		}
		if st.Fallbacks < 1 {
			t.Fatalf("inactive with fallbacks=%d, silent fallback", st.Fallbacks)
		}
	}
}

// 软硬对照：同一片子强制软解与自动档的画面要一致（均值接近、都不平）。
// 本机无硬解时两边都是软解，断言退化为确定性自洽（均值相等）。
func TestHWSoftContrast(t *testing.T) {
	if !Available() {
		t.Skipf("ffmpeg lib missing: %v", loadErr)
	}
	meanOf := func(forceSoft bool) (float64, float64) {
		if forceSoft {
			t.Setenv("GPUI_VIDEO_HW", "off")
		}
		d, err := Open("../testdata/vr2_720p.mp4")
		if err != nil {
			t.Fatalf("open (soft=%v): %v", forceSoft, err)
		}
		defer d.Close()
		fr, err := d.Next()
		if err != nil {
			t.Fatalf("first frame (soft=%v): %v", forceSoft, err)
		}
		var sum, sq uint64
		for _, b := range fr.Pix {
			sum += uint64(b)
			sq += uint64(b) * uint64(b)
		}
		mean := float64(sum) / float64(len(fr.Pix))
		variance := float64(sq)/float64(len(fr.Pix)) - mean*mean
		return mean, variance
	}
	mAuto, vAuto := meanOf(false)
	mSoft, vSoft := meanOf(true)
	t.Logf("auto mean=%.2f var=%.0f soft mean=%.2f var=%.0f", mAuto, vAuto, mSoft, vSoft)
	if vAuto < 10 || vSoft < 10 {
		t.Fatalf("flat frame auto_var=%.0f soft_var=%.0f", vAuto, vSoft)
	}
	diff := mAuto - mSoft
	if diff < 0 {
		diff = -diff
	}
	if diff > 8 {
		t.Fatalf("hw/sw first-frame mean drift %.2f, want <= 8", diff)
	}
}
