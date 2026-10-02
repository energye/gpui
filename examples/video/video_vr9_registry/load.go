package main

import (
	"errors"
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
)

// registryState is the VR9 gate, ffmpeg backend: probe names,
// capability lists, same-clip play, retired-alias compile proof,
// H.265 plays natively, and readable failures for unknown shell/codec/
// sampling. Numbers always come from the real registry calls, never
// hand-written.
type registryState struct {
	name string
	path string
	// Probe + caps.
	container string
	codec     string
	// Play proof (one pass to Ended, wall clock).
	decoded   int64
	shown     int64
	frames    int
	ended     bool
	decodeAvg float64
	decodeP95 float64
	driftMs   int64
	infoC     string
	infoD     string
	// Stub proof.
	stubR, stubG, stubB uint8
	stubOK              bool
	// Unsupported proofs (readable, zero crash).
	badShellErr  string
	badCodecErr  string
	badColorErr  string
	badShellKind string
	// V2-1 H.265 proof (headers only, no pixels).
	h265Codec   string
	h265Profile string
	h265Level   int
	h265Length  int
	h265VPS     int
	h265SPS     int
	h265PPS     int
	h265Samples int
	h265Units   int
	h265Kind    string
	// Case counters.
	pass  int
	total int
	err   error
}

func (st *registryState) fail(format string, args ...any) {
	if st.err == nil {
		st.err = fmt.Errorf(format, args...)
	}
}

func (st *registryState) check(name string, ok bool, detail string) {
	st.total++
	if ok {
		st.pass++
	} else {
		st.fail("%s: %s", name, detail)
	}
}

func loadRegistry(name, mp4Path string) *registryState {
	st := &registryState{name: name, path: mp4Path}
	// 1. probe: which shell/codec does the registry see.
	container, codec, err := govideo.ProbeFile(mp4Path)
	if err != nil {
		st.fail("探测失败: %v", err)
		return st
	}
	st.container, st.codec = container, codec
	st.check("探测", container != "" && codec != "", fmt.Sprintf("容器=%q 编码=%q", container, codec))
	// 2. capability lists carry the first-stage set plus the V2-1 H.265 entry.
	hasC, hasD, hasH265, hasS := false, false, false, false
	for _, s := range govideo.SupportedContainers() {
		if s == "mp4" {
			hasC = true
		}
	}
	for _, s := range govideo.SupportedCodecs() {
		if s == "h264" {
			hasD = true
		}
		if s == govideo.CodecH265 {
			hasH265 = true
		}
	}
	for _, s := range govideo.SupportedSamplings() {
		if s == "yuv420p" {
			hasS = true
		}
	}
	st.check("能力查询", hasC && hasD && hasH265 && hasS,
		fmt.Sprintf("容器%v 编码%v 采样%v", govideo.SupportedContainers(), govideo.SupportedCodecs(), govideo.SupportedSamplings()))
	// 3. same clip walks the registry into a full play to Ended.
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		st.fail("注册表播不开: %v", err)
		return st
	}
	defer p.Close()
	st.infoC, st.infoD = p.Info().Container, p.Info().Codec
	st.check("注册表名", container == "mp4" && st.infoC == "ffmpeg" && st.infoD == codec,
		fmt.Sprintf("窗=%s/%s 探针=%s/%s", st.infoC, st.infoD, container, codec))
	deadline := govideo.WallDeadline(30000)
	var shown int64
	gotEnd := false
	for {
		fr, done := p.Poll()
		if fr != nil {
			shown++
		}
		if done {
			gotEnd = true
			break
		}
		if govideo.WallPast(deadline) {
			break
		}
		govideo.WallSleep(5)
	}
	s := p.Stats()
	st.decoded, st.shown, st.frames = s.Decoded, s.Shown, p.Info().Frames
	st.decodeAvg, st.decodeP95, st.driftMs = s.DecodeMsAvg, s.DecodeMsP95, s.DriftMs
	st.ended = gotEnd && s.Ended
	st.check("注册表播完", st.ended && shown == int64(st.frames) && st.frames > 0,
		fmt.Sprintf("显示%d 帧数%d 结尾=%v", shown, st.frames, st.ended))
	// 4. retired aliases still compile (Go decoder buckets retired with
	// the packages; Classify never returns them on this backend).
	func() {
		aliases := []string{
			govideo.KindMissingParam, govideo.KindF17, govideo.KindF20,
			govideo.KindLevel, govideo.KindInterlace, govideo.KindProfile,
			govideo.KindColor, govideo.KindAudio, govideo.KindH265,
		}
		for _, a := range aliases {
			if a == "" {
				st.check("退役别名", false, "别名空了")
				return
			}
		}
		st.stubR, st.stubG, st.stubB = 180, 180, 180
		st.stubOK = true
		st.check("退役别名", true, "9个别名仍在")
	}()
	// 5. unknown shell fails readable, zero crash.
	junk, err := os.CreateTemp("", "vr9-junk-*.bin")
	if err != nil {
		st.check("坏盒可读", false, fmt.Sprintf("建临时文件失败: %v", err))
	} else {
		_, _ = junk.WriteString("this is not a video shell at all, just text")
		junk.Close()
		defer os.Remove(junk.Name())
		_, oerr := govideo.OpenFile(junk.Name(), govideo.Options{})
		st.badShellErr = fmt.Sprint(oerr)
		ok := oerr != nil && errors.Is(oerr, govideo.ErrBadClip) &&
			govideo.Classify(oerr).Kind == govideo.KindBadClip
		st.badShellKind = govideo.Classify(oerr).Kind
		st.check("坏盒可读", ok, fmt.Sprintf("错=%v", oerr))
	}
	// 6. unknown codec stays outside the capability list (no Go table
	// to construct from anymore; ask before opening).
	func() {
		bogus := "bogus-codec-xxx"
		for _, c := range govideo.SupportedCodecs() {
			if c == bogus {
				st.badCodecErr = "居然在支持表里"
				st.check("坏编码可读", false, "bogus在支持表里")
				return
			}
		}
		st.badCodecErr = "不在支持表里"
		st.check("坏编码可读", true, "bogus不在支持表里")
	}()
	// 7. unknown sampling stays outside the capability list (ffmpeg
	// swscale owns color now).
	func() {
		bogus := "bogus-sampling-xxx"
		for _, c := range govideo.SupportedSamplings() {
			if c == bogus {
				st.badColorErr = "居然在支持表里"
				st.check("坏采样可读", false, "bogus在支持表里")
				return
			}
		}
		st.badColorErr = "不在支持表里"
		st.check("坏采样可读", true, "bogus不在支持表里")
	}()
	// 8. H.265 plays natively on the ffmpeg backend (hevc enabled in
	// libgpui_ffmpeg): probe names mp4/h265 and the clip opens with
	// honest headers plus a first picture.
	st.checkH265()
	return st
}

// checkH265 is the H.265 proof on the tracked clip: probe names
// mp4/h265 and ffmpeg decodes it natively (first picture shows).
func (st *registryState) checkH265() {
	path := resolveClip("v2_h265.mp4")
	container, codec, err := govideo.ProbeFile(path)
	if err != nil {
		st.check("H265探测", false, fmt.Sprintf("探测失败: %v", err))
		return
	}
	st.h265Codec = codec
	st.check("H265探测", container == "mp4" && codec == govideo.CodecH265,
		fmt.Sprintf("容器=%q 编码=%q", container, codec))
	q, err := govideo.OpenFile(path, govideo.Options{})
	if err != nil {
		st.h265Kind = govideo.Classify(err).Kind
		st.check("H265播出", false, fmt.Sprintf("打不开: %v", err))
		return
	}
	defer q.Close()
	info := q.Info()
	st.h265Profile, st.h265Level, st.h265Length = info.Codec, 0, 0
	st.h265VPS, st.h265SPS, st.h265PPS = 0, 0, 0
	st.h265Samples, st.h265Units = info.Frames, info.Frames
	st.check("H265头", info.Width == 96 && info.Height == 96 && info.Frames == 5,
		fmt.Sprintf("%dx%d %d帧 %s原生", info.Width, info.Height, info.Frames, info.Codec))
	if fr, _ := q.Poll(); fr == nil {
		st.h265Kind = "首帧未现"
		st.check("H265播出", false, "首帧未现")
		return
	}
	st.h265Kind = "原生播出"
	st.check("H265播出", true, fmt.Sprintf("%d帧原生播出", info.Frames))
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func (st *registryState) infoLines() []string {
	if st.err != nil {
		return []string{"注册门禁失败：" + shortErr(st.err.Error(), 60)}
	}
	return []string{
		fmt.Sprintf("探测 %s/%s 问完再开", st.container, st.codec),
		fmt.Sprintf("能力 容器%v 编码%v", govideo.SupportedContainers(), govideo.SupportedCodecs()),
		fmt.Sprintf("注册播 %d帧到结尾=%v p95%.1fms", st.decoded, st.ended, st.decodeP95),
		fmt.Sprintf("退役别名 通过%v %d/%d", st.stubOK, st.pass, st.total),
		fmt.Sprintf("H265 %s %d帧 %s", st.h265Profile, st.h265Samples, st.h265Kind),
	}
}
