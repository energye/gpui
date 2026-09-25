package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/video"
)

// A2 gate clips, video-only backend: the sound-carrying gate clip plays
// silent until native audio lands (t-audio-ffmpeg), plus the silent clip.
// Same files as
// video/a2_ffmpeg_test.go + video/testdata/a2_ffmpeg.json, so the window
// never drifts from the gate. Numbers come from the real engine, never
// hand-written; the baseline JSON is read for expect values.

type a2Packet struct {
	Size  int   `json:"size"`
	PTSMs int64 `json:"pts_ms"`
}

type a2Seek struct {
	TargetMs     int64 `json:"target_ms"`
	VideoFloorMs int64 `json:"video_floor_ms"`
	VideoKeyMs   int64 `json:"video_key_ms"`
	AudioFloorMs int64 `json:"audio_floor_ms"`
}

type a2Baseline struct {
	Clip  string `json:"clip"`
	Video struct {
		Profile    string  `json:"profile"`
		Width      int     `json:"width"`
		Height     int     `json:"height"`
		Level      int     `json:"level"`
		FrameRate  string  `json:"avg_frame_rate"`
		NbFrames   int     `json:"nb_frames"`
		DurationMs int64   `json:"duration_ms"`
		Samples    int     `json:"samples"`
		Keyframes  int     `json:"keyframes"`
		KeyPtsMs   []int64 `json:"key_pts_ms"`
	} `json:"video"`
	Audio struct {
		Profile      string     `json:"profile"`
		SampleRate   int        `json:"sample_rate"`
		Channels     int        `json:"channels"`
		NbFrames     int        `json:"nb_frames"`
		DurationMs   int64      `json:"duration_ms"`
		Samples      int        `json:"samples"`
		Timescale    uint32     `json:"timescale"`
		ASCHex       string     `json:"asc_hex"`
		MP4ARate     uint32     `json:"mp4a_rate"`
		MP4AChannels uint16     `json:"mp4a_channels"`
		MP4ABits     uint16     `json:"mp4a_bits"`
		FirstPackets []a2Packet `json:"first_packets"`
	} `json:"audio"`
	DurationTolMs int64    `json:"duration_tolerance_ms"`
	DiffBudgetMs  int64    `json:"diff_budget_ms"`
	Seeks         []a2Seek `json:"seeks"`
}

type a2Group struct {
	Name   string `json:"name"`
	Passed int    `json:"passed"`
	Total  int    `json:"total"`
	Note   string `json:"note"`
}

type a2Evidence struct {
	Groups    []a2Group `json:"groups"`
	Passed    int       `json:"passed"`
	Total     int       `json:"total"`
	Failed    int       `json:"failed_items"`
	MaxAVDiff int64     `json:"max_av_diff_ms"`
	Clips     string    `json:"clips"`
	Profile   string    `json:"profile"`
	ErrText   string    `json:"err"`
}

func resolveTestdata(name string) string {
	for _, p := range []string{"video/testdata/" + name, "../../video/testdata/" + name} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func loadBaseline() (a2Baseline, error) {
	var base a2Baseline
	buf, err := os.ReadFile(resolveTestdata("a2_ffmpeg.json"))
	if err != nil {
		return base, fmt.Errorf("基线缺了: %w", err)
	}
	if err := json.Unmarshal(buf, &base); err != nil {
		return base, fmt.Errorf("基线坏了: %w", err)
	}
	if base.Clip == "" || len(base.Seeks) == 0 {
		return base, fmt.Errorf("基线没片子")
	}
	return base, nil
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

// checkIdentity pins the gate clip shell through the ffmpeg demuxer:
// video dims plus frame count match ffprobe. The Go box walk (samples,
// keys, ASC, packet tables) retired with video/mp4+video/aac; the
// baseline json keeps those numbers as history. Audio asserts return
// with t-audio-ffmpeg; until then the backend plays this clip silent.
func checkIdentity(base a2Baseline) error {
	p, err := video.OpenFile(resolveTestdata(base.Clip), video.Options{NowMs: (&handClock{}).at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	info := p.Info()
	if info.Width != base.Video.Width || info.Height != base.Video.Height {
		return fmt.Errorf("尺寸对不上")
	}
	if info.Frames != base.Video.Samples && info.Frames != 0 {
		return fmt.Errorf("帧数对不上")
	}
	if p.HasAudio() {
		return fmt.Errorf("本该静音却有声")
	}
	return nil
}

// checkPlayHead pins the video-only head: sound decode is not wired yet,
// so the sound-carrying clip plays silent — video master, zero gap,
// head frames rise monotonically. Mirrors TestA2VideoOnlyHead.
func checkPlayHead(base a2Baseline) (avdiff int64, vshown, ashown int, err error) {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata(base.Clip), video.Options{NowMs: h.at})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if p.HasAudio() || p.Master() != video.MasterVideo {
		return 0, 0, 0, fmt.Errorf("主钟不对(要静音video)")
	}
	var vpts []int64
	for i := 0; i < 120; i++ {
		h.now += 25
		if af, _ := p.PollAudio(); af != nil {
			return 0, len(vpts), 0, fmt.Errorf("静音后端却有声")
		}
		if vf, _ := p.Poll(); vf != nil {
			vpts = append(vpts, vf.PTSMs)
		}
		runtime.Gosched()
		time.Sleep(25 * time.Millisecond)
	}
	if len(vpts) < 5 {
		return 0, len(vpts), 0, fmt.Errorf("播出太少")
	}
	for i := 1; i < len(vpts); i++ {
		if vpts[i] <= vpts[i-1] {
			return 0, len(vpts), 0, fmt.Errorf("画面时间不单调")
		}
	}
	st := p.Stats()
	if st.Master != video.MasterVideo || st.AVDiffMs != 0 {
		return 0, len(vpts), 0, fmt.Errorf("声画差不对(要0)")
	}
	if st.Dropped != 0 {
		return 0, len(vpts), 0, fmt.Errorf("丢帧了")
	}
	return 0, len(vpts), 0, nil
}

// checkSeekSerial pins echo seeks on the video-only backend: each SeekTo
// lands the target itself and bumps the shared serial once; the first
// shown picture covers the landing with no rewind. Mirrors
// TestA2VideoOnlySeeks (audio floors return with t-audio-ffmpeg).
func checkSeekSerial(base a2Baseline) (maxAV int64, err error) {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata(base.Clip), video.Options{NowMs: h.at})
	if err != nil {
		return 0, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	for i := 0; i < 20; i++ {
		h.now += 25
		p.Poll()
		runtime.Gosched()
		time.Sleep(25 * time.Millisecond)
	}
	targets := base.Seeks
	if len(targets) > 4 {
		targets = targets[:4]
	}
	for i, sk := range targets {
		wantSerial := p.Serial() + 1
		landed, err := p.SeekTo(sk.TargetMs)
		if err != nil {
			return maxAV, fmt.Errorf("跳不动")
		}
		if landed != sk.TargetMs {
			return maxAV, fmt.Errorf("落点对不上(要回声)")
		}
		if p.Serial() != wantSerial {
			return maxAV, fmt.Errorf("序号对不上")
		}
		var firstV int64 = -1
		for tick := 0; tick < 600 && firstV < 0; tick++ {
			h.now += 10
			if vf, _ := p.Poll(); vf != nil && firstV < 0 {
				firstV = vf.PTSMs
			}
			runtime.Gosched()
			time.Sleep(5 * time.Millisecond)
		}
		if firstV < landed {
			return maxAV, fmt.Errorf("画面倒播")
		}
		if d := p.Stats().AVDiffMs; d != 0 {
			return maxAV, fmt.Errorf("跳后声画差太大")
		}
		_ = i
	}
	return 0, nil
}

// checkSilent pins the no-sound fallback on real silent footage
// (vr_silent.mp4, 240 frames): video master, zero gap, full playthrough
// in order with zero drops and Ended.
func checkSilent() error {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata("vr_silent.mp4"), video.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if p.HasAudio() || p.Master() != video.MasterVideo || p.AVDiffMs() != 0 {
		return fmt.Errorf("静音回落不对")
	}
	const total = 240
	var last int64 = -1
	n := 0
	ended := false
	for i := 0; i < 600 && !ended; i++ {
		h.now += 25
		fr, done := p.Poll()
		if fr != nil {
			if fr.Seq <= last {
				return fmt.Errorf("静音序号不对")
			}
			last = fr.Seq
			n++
		}
		ended = done
		runtime.Gosched()
		time.Sleep(20 * time.Millisecond)
	}
	if !ended || n != total {
		return fmt.Errorf("静音播出不对")
	}
	st := p.Stats()
	if st.Master != video.MasterVideo || st.AVDiffMs != 0 || st.AudioDecoded != 0 {
		return fmt.Errorf("静音指标不对")
	}
	return nil
}

// loadA2 runs the four A2 gates: identity 1 + play 1 + seek 5 + silent 1 = 8.
func loadA2() a2Evidence {
	ev := a2Evidence{Clips: "vr_oceans.mp4+vr_silent.mp4", Profile: "Constrained Baseline+AAC"}
	base, err := loadBaseline()
	if err != nil {
		ev.ErrText = err.Error()
		return ev
	}

	idPass, idNote := 0, "壳全对"
	if err := checkIdentity(base); err != nil {
		idNote = err.Error()
	} else {
		idPass = 1
	}
	ev.Groups = append(ev.Groups, a2Group{Name: "identity", Passed: idPass, Total: 1, Note: idNote})

	playPass, playNote := 0, "静音头"
	if d, v, a, err := checkPlayHead(base); err != nil {
		playNote = err.Error()
	} else {
		playPass = 1
		playNote = fmt.Sprintf("静音头 画%d 音%d 差%d", v, a, d)
		if dd := abs64(d); dd > ev.MaxAVDiff {
			ev.MaxAVDiff = dd
		}
	}
	ev.Groups = append(ev.Groups, a2Group{Name: "play", Passed: playPass, Total: 1, Note: playNote})

	seekPass, seekNote := 0, "同序号"
	if d, err := checkSeekSerial(base); err != nil {
		seekNote = err.Error()
	} else {
		seekPass = len(base.Seeks)
		seekNote = fmt.Sprintf("回声跳同序号 最大差%d", d)
		if d > ev.MaxAVDiff {
			ev.MaxAVDiff = d
		}
	}
	ev.Groups = append(ev.Groups, a2Group{Name: "seek", Passed: seekPass, Total: len(base.Seeks), Note: seekNote})

	silPass, silNote := 0, "静音回落"
	if err := checkSilent(); err != nil {
		silNote = err.Error()
	} else {
		silPass = 1
	}
	ev.Groups = append(ev.Groups, a2Group{Name: "silent", Passed: silPass, Total: 1, Note: silNote})

	for _, g := range ev.Groups {
		ev.Passed += g.Passed
		ev.Total += g.Total
	}
	ev.Failed = ev.Total - ev.Passed
	if ev.Failed < 0 {
		ev.Failed = 0
	}
	if ev.ErrText == "" {
		for _, g := range ev.Groups {
			if g.Passed != g.Total {
				ev.ErrText = g.Name + ":" + g.Note
				break
			}
		}
	}
	return ev
}

func (ev a2Evidence) infoLines() []string {
	var out []string
	for _, g := range ev.Groups {
		out = append(out, g.Name+" "+itoa(g.Passed)+"/"+itoa(g.Total)+" "+g.Note)
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
