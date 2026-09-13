package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	govideo "github.com/energye/gpui/video"
	"github.com/energye/gpui/video/h264"
	"github.com/energye/gpui/video/mp4"
)

// faultState is one VR6 gate: a bad input that must fail readable (or, for
// F20 flower, open with isolated frames) without panic or hang.
type faultState struct {
	name string
	pass bool
	note string
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func resolveNonMP4() string {
	for _, p := range []string{
		"video/color/testdata/vr3_vectors.json",
		"../../video/color/testdata/vr3_vectors.json",
		"video/fault.go",
		"../../video/fault.go",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/fault.go"
}

// firstPSlice returns the second sample (a P slice) plus its header sets.
func firstPSlice() ([][]byte, *h264.AVCC, error) {
	m, err := mp4.ParseFile(resolveClip("vr2_m_bframes.mp4"))
	if err != nil {
		return nil, nil, err
	}
	avcc, err := h264.ParseAVCC(m.Video.AVCConfig)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(resolveClip("vr2_m_bframes.mp4"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	s := m.Video.Samples[1]
	buf := make([]byte, s.Size)
	if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
		return nil, nil, err
	}
	units, err := h264.SplitAVCC(buf, avcc.LengthSize)
	if err != nil {
		return nil, nil, err
	}
	return units, avcc, nil
}

// loadFault runs the whole bad-input matrix synchronously at startup.
// Every case is bounded (no network, no hang); panics would fail the run,
// so a returned pass/total already proves zero-crash.
func loadFault() []*faultState {
	dir, err := os.MkdirTemp("", "vr6fault")
	if err != nil {
		return []*faultState{{name: "建临时目录", note: err.Error()}}
	}
	defer os.RemoveAll(dir)

	out := []*faultState{}
	ok := func(name string, pass bool, note string) {
		out = append(out, &faultState{name: name, pass: pass, note: note})
	}

	// 1. 缺文件
	func() {
		_, err := govideo.OpenFile(filepath.Join(dir, "does-not-exist.mp4"), govideo.Options{})
		if err == nil {
			ok("缺文件", false, "居然打开了")
			return
		}
		f := govideo.Classify(err)
		ok("缺文件", f.Kind == govideo.KindBadClip, f.Readable())
	}()

	// 2. 非MP4
	func() {
		_, err := govideo.OpenFile(resolveNonMP4(), govideo.Options{})
		if err == nil {
			ok("非MP4", false, "居然打开了")
			return
		}
		f := govideo.Classify(err)
		switch f.Kind {
		case govideo.KindTruncated, govideo.KindBadBox, govideo.KindBadClip:
			ok("非MP4", true, f.Readable())
		default:
			ok("非MP4", false, "未知分类:"+f.Readable())
		}
	}()

	// 3. 截断尾（尾部moov被切，打开失败但可读）
	func() {
		raw, err := os.ReadFile(resolveClip("vr2_m_bframes.mp4"))
		if err != nil {
			ok("截断尾", false, "读基片失败:"+err.Error())
			return
		}
		bad := filepath.Join(dir, "trunc.mp4")
		if err := os.WriteFile(bad, raw[:len(raw)-200], 0o644); err != nil {
			ok("截断尾", false, "写坏片失败")
			return
		}
		_, err = govideo.OpenFile(bad, govideo.Options{})
		if err == nil {
			ok("截断尾", false, "截断居然打开了")
			return
		}
		if !errors.Is(err, mp4.ErrTruncated) {
			ok("截断尾", false, "没点名截断:"+err.Error())
			return
		}
		ok("截断尾", govideo.Classify(err).Kind == govideo.KindTruncated, govideo.Classify(err).Readable())
	}()

	// 4. 花屏流F20（中间P清零，隔离2帧，双IDR尾照常播）
	func() {
		m, err := mp4.ParseFile(resolveClip("vr5_seek.mp4"))
		if err != nil {
			ok("花屏隔离", false, "解析基片失败")
			return
		}
		raw, err := os.ReadFile(resolveClip("vr5_seek.mp4"))
		if err != nil {
			ok("花屏隔离", false, "读基片失败")
			return
		}
		cp := append([]byte(nil), raw...)
		s := m.Video.Samples[1]
		for i := int64(0); i < int64(s.Size); i++ {
			cp[int64(s.Offset)+i] = 0
		}
		bad := filepath.Join(dir, "flower.mp4")
		if err := os.WriteFile(bad, cp, 0o644); err != nil {
			ok("花屏隔离", false, "写坏片失败")
			return
		}
		p, err := govideo.OpenFile(bad, govideo.Options{})
		if err != nil {
			ok("花屏隔离", false, "隔离失败:"+shortErr(err.Error(), 40))
			return
		}
		defer p.Close()
		if p.Info().Concealed != 2 {
			ok("花屏隔离", false, fmt.Sprintf("隔离%d帧要2帧", p.Info().Concealed))
			return
		}
		if govideo.Classify(p.ConcealedFault()).Kind != govideo.KindF20 {
			ok("花屏隔离", false, "没点名F20")
			return
		}
		// Fresh opens anchor one step before the head stamp, so wait a
		// frame interval before polling (no black, just clock postage).
		govideo.WallSleep(300)
		if f, _ := p.Poll(); f == nil {
			ok("花屏隔离", false, "隔离后黑屏")
			return
		}
		ok("花屏隔离", true, fmt.Sprintf("隔离2帧/剩%d帧/F20", p.Info().Frames))
	}()

	// 5. 缺参数（切片没喂参数集）
	func() {
		units, _, err := firstPSlice()
		if err != nil {
			ok("缺参数", false, "取切片失败")
			return
		}
		dec := h264.NewDecoder(nil)
		err = dec.DecodeNALU(units[0])
		if !errors.Is(err, h264.ErrMissingPPS) && !errors.Is(err, h264.ErrMissingSPS) {
			ok("缺参数", false, "没点名缺参数")
			return
		}
		ok("缺参数", govideo.Classify(err).Kind == govideo.KindMissingParam, govideo.Classify(err).Readable())
	}()

	// 6. F17数据分区
	func() {
		_, err := h264.SplitFrames([][]byte{{0x42, 0x00}})
		if !errors.Is(err, h264.ErrDataPartitioning) {
			ok("F17分区", false, "没拦住分区")
			return
		}
		ok("F17分区", govideo.Classify(err).Kind == govideo.KindF17, govideo.Classify(err).Readable())
	}()

	// 7. F17扩展切片
	func() {
		_, err := h264.SplitFrames([][]byte{{0x74, 0x00}})
		if !errors.Is(err, h264.ErrUnsupportedNAL) {
			ok("F17扩展", false, "没拦住扩展")
			return
		}
		ok("F17扩展", govideo.Classify(err).Kind == govideo.KindF17, govideo.Classify(err).Readable())
	}()

	// 8. F17条带组（归口；检出由h264单测TestPPSSliceGroups锁）
	func() {
		err := fmt.Errorf("x: %w", h264.ErrSliceGroups)
		ok("F17条带组", govideo.Classify(err).Kind == govideo.KindF17, govideo.Classify(err).Readable())
	}()

	// 9. F20丢参考（有参数无参考）
	func() {
		units, avcc, err := firstPSlice()
		if err != nil {
			ok("F20丢参考", false, "取切片失败")
			return
		}
		dec := h264.NewDecoder(nil)
		for _, raw := range avcc.SPS {
			_ = dec.DecodeNALU(raw)
		}
		for _, raw := range avcc.PPS {
			_ = dec.DecodeNALU(raw)
		}
		err = dec.DecodeNALU(units[0])
		if !errors.Is(err, h264.ErrLostReference) {
			ok("F20丢参考", false, "没点名F20")
			return
		}
		ok("F20丢参考", govideo.Classify(err).Kind == govideo.KindF20, govideo.Classify(err).Readable())
	}()

	// 10. 超限等级F2（6.0带不动）
	func() {
		err := fmt.Errorf("x: %w", h264.ErrUnsupportedLevel)
		ok("超限等级", govideo.Classify(err).Kind == govideo.KindLevel, govideo.Classify(err).Readable())
	}()

	// 11. F12隔行（归口；检出由h264单测4项锁）
	func() {
		err := fmt.Errorf("x: %w: F12 interlace field picture", h264.ErrStageScope)
		ok("F12隔行", govideo.Classify(err).Kind == govideo.KindInterlace, govideo.Classify(err).Readable())
	}()

	// 12. 好片干净（零误伤）
	func() {
		p, err := govideo.OpenFile(resolveClip("vr2_m_bframes.mp4"), govideo.Options{})
		if err != nil {
			ok("好片干净", false, "好片打不开")
			return
		}
		defer p.Close()
		if p.Info().Concealed != 0 {
			ok("好片干净", false, "好片被隔离")
			return
		}
		ok("好片干净", true, fmt.Sprintf("%d帧零隔离", p.Info().Frames))
	}()

	return out
}

func (st *faultState) infoLine() string {
	mark := "过"
	if !st.pass {
		mark = "挂"
	}
	return fmt.Sprintf("%s %s：%s", mark, st.name, shortErr(st.note, 40))
}
