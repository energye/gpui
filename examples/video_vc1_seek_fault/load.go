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

	// 3. 截断尾
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

	// 4. 花屏隔离（好尾照播）
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
		govideo.WallSleep(300)
		if f, _ := p.Poll(); f == nil {
			ok("花屏隔离", false, "隔离后黑屏")
			return
		}
		ok("花屏隔离", true, fmt.Sprintf("隔离2帧/剩%d帧/F20", p.Info().Frames))
	}()

	// 5. F17分区（归口；检出由h264单测锁）
	func() {
		_, err := h264.SplitFrames([][]byte{{0x42, 0x00}})
		if !errors.Is(err, h264.ErrDataPartitioning) {
			ok("F17分区", false, "没拦住分区")
			return
		}
		ok("F17分区", govideo.Classify(err).Kind == govideo.KindF17, govideo.Classify(err).Readable())
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
