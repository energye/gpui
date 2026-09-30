//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package ffmpeg binds the single-file libgpui_ffmpeg shared library
// through purego (no CGO, no import "C") and decodes video frames to
// packed RGBA ready for the current library's playback chain.
//
// Say it plain: this package opens a file or URL with ffmpeg's own
// demuxer, decodes the best video stream, and hands each picture to the
// caller as width, height, RGBA pixels and a millisecond stamp. The
// caller (video.Player) wraps that into its existing queue and clock
// types, so the display path does not change.
//
// The library file lives under gpui/lib/ffmpeg/<platform>/ and is chosen
// by runtime GOOS and GOARCH. Set GPUI_FFMPEG_PATH to override the search
// (tests and CI use this when the working directory differs).
//
// Coverage: every public av_/sws_/swr_ symbol the shared library exports
// is bound (13 modules by feature + data_const.go for data +
// variadic_go.go for C variadics): 991 functions via RegisterLibFunc,
// 16 data via Dlsym, 4 C variadics via Go-format wrappers
// (Asprintf, Util.Strlcatf, IOContext.Printf, Log.Logf, Log.Once,
// BPrint.BprintfF; Go formats first, then calls the non-variadic
// sibling, so float args stay correct on all platforms).
package ffmpeg
