package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 版本装载门禁：默认基础版，GPUI_FFMPEG_VARIANT=full 才进高级版，
// GPUI_FFMPEG_PATH 直接指文件时指哪找哪。
// 大白话：不断言库文件在不在（高级版 so 还没编出来），只断言选路逻辑对。
func TestVariantRouting(t *testing.T) {
	// 默认：基础版旧名。
	os.Unsetenv("GPUI_FFMPEG_VARIANT")
	os.Unsetenv("GPUI_FFMPEG_PATH")
	if got := libRelNameFor("base"); !strings.HasSuffix(got, "libgpui_ffmpeg.so") &&
		!strings.HasSuffix(got, "libgpui_ffmpeg.dll") &&
		!strings.HasSuffix(got, "libgpui_ffmpeg.dylib") {
		t.Fatalf("base name = %q", got)
	}
	if got := libRelNameFor("full"); !strings.Contains(got, "_full.") {
		t.Fatalf("full name = %q, want _full suffix", got)
	}
	// 同目录不同名（方案 A）。
	base := libRelNameFor("base")
	full := libRelNameFor("full")
	if filepath.Dir(base) != filepath.Dir(full) {
		t.Fatalf("dir differs: base=%q full=%q", base, full)
	}
	// 默认 base。
	if got := ffmpegVariant(); got != "base" {
		t.Fatalf("default variant = %q, want base", got)
	}
	// 显式 full（大小写都认）。
	for _, v := range []string{"full", "FULL", "Full"} {
		t.Setenv("GPUI_FFMPEG_VARIANT", v)
		if got := ffmpegVariant(); got != "full" {
			t.Fatalf("variant %q -> %q, want full", v, got)
		}
	}
	// 别的字都回 base。
	t.Setenv("GPUI_FFMPEG_VARIANT", "xxx")
	if got := ffmpegVariant(); got != "base" {
		t.Fatalf("variant xxx -> %q, want base", got)
	}
	os.Unsetenv("GPUI_FFMPEG_VARIANT")
	// PATH 直接指文件时不受版本限制。
	t.Setenv("GPUI_FFMPEG_PATH", "/tmp/custom_full.so")
	paths := candidatePaths()
	if len(paths) != 1 || paths[0] != "/tmp/custom_full.so" {
		t.Fatalf("PATH override = %v", paths)
	}
	os.Unsetenv("GPUI_FFMPEG_PATH")
}
