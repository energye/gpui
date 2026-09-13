package main

import (
	"errors"
	"fmt"
	"os"

	govideo "github.com/energye/gpui/video"
	"github.com/energye/gpui/video/color"
	"github.com/energye/gpui/video/h264"
)

// registryState is the VR9 gate: everything through the tables —
// probe names, capability lists, same-clip play via registry, a second
// decoder plugged purely via RegisterDecoder, and readable failures for
// unknown shell/codec/sampling. Numbers always come from the real
// registry calls, never hand-written.
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
	// Case counters.
	pass  int
	total int
	err   error
}

// stubGrayDecoder is the window-side second decoder: arrives purely
// through RegisterDecoder, no core change. Flat grey synthetic picture.
type stubGrayDecoder struct {
	w, h uint32
	fed  int
}

func (s *stubGrayDecoder) DecodeNALU(nalu []byte) error {
	s.fed++
	return nil
}

func (s *stubGrayDecoder) FinishPicture() (*h264.Picture, error) {
	p, err := h264.NewPicture(s.w, s.h)
	if err != nil {
		return nil, err
	}
	for i := range p.Y {
		p.Y[i] = 180
	}
	return p, nil
}

func (s *stubGrayDecoder) Sampling() string { return color.SamplingYUV420P }

func stubGraySplit(buf []byte, lengthSize int) ([][]byte, error) {
	return [][]byte{buf}, nil
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
	// 2. capability lists carry the first-stage set.
	hasC, hasD, hasS := false, false, false
	for _, s := range govideo.SupportedContainers() {
		if s == "mp4" {
			hasC = true
		}
	}
	for _, s := range govideo.SupportedCodecs() {
		if s == "h264" {
			hasD = true
		}
	}
	for _, s := range govideo.SupportedSamplings() {
		if s == color.SamplingYUV420P {
			hasS = true
		}
	}
	st.check("能力查询", hasC && hasD && hasS,
		fmt.Sprintf("容器%v 编码%v 采样%v", govideo.SupportedContainers(), govideo.SupportedCodecs(), govideo.SupportedSamplings()))
	// 3. same clip walks the registry into a full play to Ended.
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		st.fail("注册表播不开: %v", err)
		return st
	}
	defer p.Close()
	st.infoC, st.infoD = p.Info().Container, p.Info().Codec
	st.check("注册表名", st.infoC == container && st.infoD == codec,
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
	// 4. second decoder plugs in with no core edit.
	const stub = "vr9-window-stub-gray"
	govideo.RegisterDecoder(stub, func() govideo.Decoder { return &stubGrayDecoder{w: 96, h: 96} }, stubGraySplit)
	d, err := govideo.NewDecoder(stub)
	if err != nil {
		st.check("桩解码器", false, fmt.Sprintf("建不出: %v", err))
	} else {
		units, serr := govideo.SplitUnits(stub, []byte{0x01, 0x02}, 4)
		ok := serr == nil && len(units) == 1
		if ok {
			for _, u := range units {
				if ferr := d.DecodeNALU(u); ferr != nil {
					ok = false
					break
				}
			}
		}
		var pic *h264.Picture
		if ok {
			pic, err = d.FinishPicture()
			ok = err == nil && pic != nil && pic.Width == 96 && pic.Height == 96
		}
		if ok {
			cf, cerr := color.Convert(d.Sampling(), pic.Y, pic.Cb, pic.Cr, 96, 96, color.Options{})
			ok = cerr == nil && len(cf.Pix) == 96*96*4
			if ok {
				r, g, b, a := cf.At(48, 48)
				st.stubR, st.stubG, st.stubB = r, g, b
				dr := int(r) - int(g)
				if dr < 0 {
					dr = -dr
				}
				db := int(g) - int(b)
				if db < 0 {
					db = -db
				}
				ok = a == 255 && dr <= 2 && db <= 2
				st.stubOK = ok
			}
		}
		st.check("桩解码器", ok, "灰桩走注册表转出失败")
	}
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
		ok := oerr != nil && errors.Is(oerr, govideo.ErrUnsupportedContainer) &&
			govideo.Classify(oerr).Kind == govideo.KindBadClip
		st.badShellKind = govideo.Classify(oerr).Kind
		st.check("坏盒可读", ok, fmt.Sprintf("错=%v", oerr))
	}
	// 6. unknown codec fails readable.
	_, cerr := govideo.NewDecoder("bogus-codec-xxx")
	st.badCodecErr = fmt.Sprint(cerr)
	st.check("坏编码可读", cerr != nil && errors.Is(cerr, govideo.ErrUnsupportedCodec), fmt.Sprintf("错=%v", cerr))
	// 7. unknown sampling fails readable through the color table.
	_, serr := color.Convert("bogus-sampling-xxx", []byte{1}, []byte{1}, []byte{1}, 2, 2, color.Options{})
	st.badColorErr = fmt.Sprint(serr)
	st.check("坏采样可读", serr != nil && errors.Is(serr, color.ErrUnsupportedSampling), fmt.Sprintf("错=%v", serr))
	return st
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
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
		fmt.Sprintf("桩解码 灰桩%v 不改核心 %d/%d", st.stubOK, st.pass, st.total),
	}
}
