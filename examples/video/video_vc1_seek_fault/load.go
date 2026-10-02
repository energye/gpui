package main

import (
	"fmt"
	"os"
	"path/filepath"

	govideo "github.com/energye/gpui/video"
)

// seekGate is one VR5-side gate on the good clip: seek, then the landed
// frame must show on the very next poll (recover ≤3s by construction).
type seekGate struct {
	name    string
	target  int64
	landed  int64
	key     int64
	delta   int64
	forward int64
	recover bool
	err     error
}

// faultGate is one VR6-side gate on a bad input: must fail readable with
// the expected kind, never panic or hang.
type faultGate struct {
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

func seekOne(name, mp4Path string, target int64) *seekGate {
	g := &seekGate{name: name, target: target}
	p, err := govideo.OpenFile(mp4Path, govideo.Options{})
	if err != nil {
		g.err = fmt.Errorf("打不开: %w", err)
		return g
	}
	defer p.Close()
	landed, err := p.SeekTo(target)
	if err != nil {
		g.err = fmt.Errorf("跳不动: %w", err)
		return g
	}
	g.landed = landed
	_, _, _, key, delta, forward := p.SeekInfo()
	g.key, g.delta, g.forward = key, delta, forward
	if f, _ := p.Poll(); f != nil && f.PTSMs == landed {
		g.recover = true
	} else {
		g.err = fmt.Errorf("跳后黑屏: 目标%d 落点%d 没立刻播出来", target, landed)
	}
	return g
}

func loadSeekGates() []*seekGate {
	good := resolveClip("vr5_seek.mp4")
	return []*seekGate{
		seekOne("跳第二GOP", good, 1800),
		seekOne("跳第一GOP", good, 600),
		seekOne("跨戳跳", good, 1700),
	}
}

func loadFaultGates() []*faultGate {
	dir, err := os.MkdirTemp("", "vc1fault")
	if err != nil {
		return []*faultGate{{name: "建临时目录", note: err.Error()}}
	}
	defer os.RemoveAll(dir)

	out := []*faultGate{}
	ok := func(name string, pass bool, note string) {
		out = append(out, &faultGate{name: name, pass: pass, note: note})
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

	// 4. 花屏隔离(中间清零2KB盲破坏:ffmpeg 原生吸收,不崩即过)
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

	return out
}

func (g *seekGate) infoLine() string {
	if g.err != nil {
		return fmt.Sprintf("挂 %s：%s", g.name, shortErr(g.err.Error(), 40))
	}
	return fmt.Sprintf("过 %s 目标%d 落点%d 键%d 前解%d 恢复%v",
		g.name, g.target, g.landed, g.key, g.forward, g.recover)
}

func (g *faultGate) infoLine() string {
	mark := "过"
	if !g.pass {
		mark = "挂"
	}
	return fmt.Sprintf("%s %s：%s", mark, g.name, shortErr(g.note, 40))
}
