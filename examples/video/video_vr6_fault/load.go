package main

import (
	"fmt"
	"os"
	"path/filepath"

	govideo "github.com/energye/gpui/video"
)

// faultState is one VR6 gate: a bad input that must fail readable (or, for
// F20 flower, open with isolated frames) without panic or hang.
type faultState struct {
	name string
	pass bool
	note string
}

func resolveClip(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func resolveNonMP4() string {
	for _, p := range []string{
		"video/fault.go",
		"../../video/fault.go",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/fault.go"
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

	// 3. 截断尾(尾部被切:ffmpeg 拥有拆盒,打开失败可读,或打开后播半截尾)
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
			ok("截断尾", true, "截断打开后播半截(诚实)")
			return
		}
		switch govideo.Classify(err).Kind {
		case govideo.KindBadClip, govideo.KindTruncated, govideo.KindBadBox:
			ok("截断尾", true, govideo.Classify(err).Readable())
		default:
			ok("截断尾", false, "未知分类:"+govideo.Classify(err).Readable())
		}
	}()

	// 4. 花屏隔离(中间盲破坏2KB:ffmpeg 原生吸收,不崩即过)
	func() {
		raw, err := os.ReadFile(resolveClip("vr5_seek.mp4"))
		if err != nil {
			ok("花屏隔离", false, "读基片失败")
			return
		}
		cp := append([]byte(nil), raw...)
		mid := len(cp) / 2
		for i := 0; i < 2048 && mid+i < len(cp); i++ {
			cp[mid+i] = 0
		}
		bad := filepath.Join(dir, "flower.mp4")
		if err := os.WriteFile(bad, cp, 0o644); err != nil {
			ok("花屏隔离", false, "写坏片失败")
			return
		}
		p, err := govideo.OpenFile(bad, govideo.Options{})
		if err != nil {
			switch govideo.Classify(err).Kind {
			case govideo.KindBadClip, govideo.KindTruncated, govideo.KindBadBox:
				ok("花屏隔离", true, "拒收可读:"+govideo.Classify(err).Readable())
			default:
				ok("花屏隔离", false, "未知分类:"+err.Error())
			}
			return
		}
		defer p.Close()
		if p.Info().Concealed != 0 || govideo.Classify(p.ConcealedFault()).Kind != govideo.KindUnknown {
			ok("花屏隔离", false, "本该零隔离零故障")
			return
		}
		govideo.WallSleep(300)
		if f, _ := p.Poll(); f == nil {
			ok("花屏隔离", false, "隔离后黑屏")
			return
		}
		ok("花屏隔离", true, fmt.Sprintf("原生吸收/剩%d帧", p.Info().Frames))
	}()

	// 5. 后端分类(Go 解码桶随包退役;只剩壳/片/封顶桶,未知兜底)
	func() {
		cases := []struct {
			name string
			err  error
			want string
		}{
			{"坏片", fmt.Errorf("x: %w", govideo.ErrBadClip), govideo.KindBadClip},
			{"无帧", fmt.Errorf("x: %w", govideo.ErrNoFrames), govideo.KindBadClip},
			{"超封顶", fmt.Errorf("x: %w", govideo.ErrMemOverCap), govideo.KindMemOverCap},
			{"ffmpeg层", fmt.Errorf("x: ffmpeg: 原生解码失败"), govideo.KindBadClip},
			{"截断", fmt.Errorf("x: unexpected EOF in stream"), govideo.KindTruncated},
		}
		for _, c := range cases {
			if govideo.Classify(c.err).Kind != c.want {
				ok("后端分类", false, c.name+"分错桶")
				return
			}
		}
		if govideo.Classify(fmt.Errorf("x: 全新失败")).Kind != govideo.KindUnknown {
			ok("后端分类", false, "未知没兜底")
			return
		}
		ok("后端分类", true, "壳/片/封顶/未知全对")
	}()

	// 6. 退役别名(老调用方仍能编译;Classify 永不返回它们)
	func() {
		aliases := []string{
			govideo.KindMissingParam, govideo.KindF17, govideo.KindF20,
			govideo.KindLevel, govideo.KindInterlace, govideo.KindProfile,
			govideo.KindColor, govideo.KindAudio, govideo.KindH265,
		}
		for _, a := range aliases {
			if a == "" {
				ok("退役别名", false, "别名空了")
				return
			}
		}
		ok("退役别名", true, "9个别名仍在")
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
