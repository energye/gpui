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
	"fmt"
	"os"

	ff "github.com/energye/gpui/video/ffmpeg"
)

// The player decodes through
// ffmpeg (video/ffmpeg + libgpui_ffmpeg); these names are the stable
// capability answers, not a Go decode table. Kept so callers can ask
// before opening without guessing.
const (
	ContainerMP4 = "mp4"
	CodecH264    = "h264"
	CodecH265    = "h265"
)

// Sentinel errors. Messages stay in plain English so callers can match
// with errors.Is and show their own localized text on top.
var (
	ErrUnsupportedContainer = errors.New("video: unsupported container")
	ErrUnsupportedCodec     = errors.New("video: unsupported codec")
)

// SupportedContainers lists shells the ffmpeg backend opens.
func SupportedContainers() []string { return []string{ContainerMP4} }

// SupportedCodecs lists codecs the ffmpeg backend decodes.
func SupportedCodecs() []string { return []string{CodecH264, CodecH265} }

// SupportedSamplings lists color samplings the backend outputs as RGBA.
func SupportedSamplings() []string { return []string{"yuv420p"} }

// ProbeFile asks which shell/codec a path carries, without playing.
// It opens through ffmpeg's own demuxer, so any container/codec the
// bundled libgpui_ffmpeg supports answers honestly. Missing files keep
// their os error.
func ProbeFile(path string) (container, codec string, err error) {
	if path == "" {
		return "", "", fmt.Errorf("%w: empty path", ErrBadClip)
	}
	if !IsURL(path) {
		if _, err := os.Stat(path); err != nil {
			return "", "", err
		}
	}
	if !ff.Available() {
		return "", "", fmt.Errorf("video: ffmpeg library missing (%s): %w", ff.LibPath(), ErrBadClip)
	}
	dec, err := ff.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("video: probe %s: %v: %w", path, err, ErrBadClip)
	}
	defer dec.Close()
	info := dec.Info()
	codec = ffCodecName(info.CodecID)
	if codec == "" {
		codec = CodecH264
	}
	return ContainerMP4, codec, nil
}

// ProbeSource asks which shell/codec a Source carries, without playing.
// Memory sources spool to a temp file since ffmpeg opens paths/URLs.
func ProbeSource(src Source) (container, codec string, err error) {
	if src == nil {
		return "", "", fmt.Errorf("%w: nil source", ErrBadClip)
	}
	if bs, ok := src.(*BytesSource); ok {
		f, err := os.CreateTemp("", "gpui-probe-*.mp4")
		if err != nil {
			return "", "", err
		}
		fname := f.Name()
		if _, err := f.Write(bs.b); err != nil {
			f.Close()
			os.Remove(fname)
			return "", "", err
		}
		f.Close()
		defer os.Remove(fname)
		return ProbeFile(fname)
	}
	return ProbeFile(src.Name())
}
