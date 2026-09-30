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
	"unsafe"
)

// 跨平台选型表：不换系统直接验全表，不碰显卡不碰库。
func TestHWPreferenceCoversPlatforms(t *testing.T) {
	cases := map[string][]string{
		"linux":   {"vaapi", "drm", "cuda", "vdpau"},
		"windows": {"d3d11va", "dxva2", "d3d12va", "qsv", "cuda"},
		"darwin":  {"videotoolbox"},
		"android": {"mediacodec"},
	}
	for goos, want := range cases {
		got := hwTypePreferenceFor(goos)
		if len(got) != len(want) {
			t.Fatalf("%s: got %q want %q", goos, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: [%d] got %q want %q", goos, i, got[i], want[i])
			}
		}
	}
	// 未知系统走默认 Linux 路（vaapi 打头），不空着。
	if got := hwTypePreferenceFor("plan9"); len(got) == 0 || got[0] != "vaapi" {
		t.Fatalf("unknown goos got %q, want vaapi first", got)
	}
}

// 非 Linux 从不碰文件系统：要的都是默认设备（传空，ffmpeg 自建）。
func TestHWDeviceCandidatesNoFSOffLinux(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "android", "plan9"} {
		for _, typ := range []string{"d3d11va", "dxva2", "videotoolbox", "mediacodec", "vaapi", "cuda"} {
			got := hwDeviceCandidatesFor(goos, typ)
			if len(got) != 1 || got[0] != "" {
				t.Fatalf("%s/%s: got %q, want [\"\"]", goos, typ, got)
			}
		}
	}
	// Linux 非显卡类型也不碰节点。
	for _, typ := range []string{"cuda", "vdpau", "qsv"} {
		got := hwDeviceCandidatesFor("linux", typ)
		if len(got) != 1 || got[0] != "" {
			t.Fatalf("linux/%s: got %q, want [\"\"]", typ, got)
		}
	}
}

// 布局自检不碰库：空指针、32 位一律否，不写坏内存。
func TestCCLayoutOKGuards(t *testing.T) {
	if ccLayoutOK(nil, 27) {
		t.Fatal("ccLayoutOK(nil) = true, want false")
	}
	if hwSupportedArch() != (unsafe.Sizeof(uintptr(0)) == 8) {
		t.Fatal("hwSupportedArch honest value mismatch")
	}
}

// 强制软解开关各写法都认（跨平台对比窗用）。
func TestHWDisabledForms(t *testing.T) {
	for _, v := range []string{"off", "OFF", "soft", "Soft", "disable", "disabled", " off "} {
		t.Setenv("GPUI_VIDEO_HW", v)
		if !HWDisabled() {
			t.Fatalf("GPUI_VIDEO_HW=%q: want disabled", v)
		}
	}
	t.Setenv("GPUI_VIDEO_HW", "")
	if HWDisabled() {
		t.Fatal("GPUI_VIDEO_HW empty: want auto (enabled)")
	}
}
