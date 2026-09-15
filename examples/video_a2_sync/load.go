package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/energye/gpui/video"
	"github.com/energye/gpui/video/mp4"
)

// A2 gate clips: the AV gate clip (sound leads, picture follows) plus
// the silent clip (old picture-only path). Same files as
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

// checkIdentity pins the gate clip shell: Main video dims + packet
// tables plus LC AAC rate/channels/ASC against the ffprobe baseline.
func checkIdentity(base a2Baseline) error {
	m, err := mp4.ParseFile(resolveTestdata(base.Clip))
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	v := m.Video
	if v == nil {
		return fmt.Errorf("没视频轨")
	}
	if int(v.Width) != base.Video.Width || int(v.Height) != base.Video.Height {
		return fmt.Errorf("尺寸对不上")
	}
	if len(v.Samples) != base.Video.Samples || len(v.Keyframes) != base.Video.Keyframes {
		return fmt.Errorf("采样/关键帧对不上")
	}
	for i, want := range base.Video.KeyPtsMs {
		if v.Keyframes[i].PTSMs != want {
			return fmt.Errorf("键%d对不上", i)
		}
	}
	a := m.Audio
	if a == nil {
		return fmt.Errorf("没声音轨")
	}
	if int(a.SampleRate) != base.Audio.SampleRate || int(a.Channels) != base.Audio.Channels {
		return fmt.Errorf("声道对不上")
	}
	if len(a.Samples) != base.Audio.Samples {
		return fmt.Errorf("声音包数对不上")
	}
	for i, want := range base.Audio.FirstPackets {
		s, ok := a.SampleAt(i)
		if !ok || int(s.Size) != want.Size || s.PTSMs != want.PTSMs {
			return fmt.Errorf("声音包%d对不上", i)
		}
	}
	return nil
}

// checkPlayHead pins sound-leads play: both stamps rise monotonically,
// master reads audio, drops stay zero, gap ends inside budget.
func checkPlayHead(base a2Baseline) (avdiff int64, vshown, ashown int, err error) {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata(base.Clip), video.Options{NowMs: h.at})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if !p.HasAudio() || p.Master() != video.MasterAudio {
		return 0, 0, 0, fmt.Errorf("主钟不对")
	}
	var vpts, apts []int64
	for i := 0; i < 120; i++ {
		h.now += 25
		if af, _ := p.PollAudio(); af != nil {
			apts = append(apts, af.PTSMs)
		}
		if vf, _ := p.Poll(); vf != nil {
			vpts = append(vpts, vf.PTSMs)
		}
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
	if len(vpts) < 5 || len(apts) < 5 {
		return 0, len(vpts), len(apts), fmt.Errorf("播出太少")
	}
	for i := 1; i < len(vpts); i++ {
		if vpts[i] <= vpts[i-1] {
			return 0, len(vpts), len(apts), fmt.Errorf("画面时间不单调")
		}
	}
	for i := 1; i < len(apts); i++ {
		if apts[i] <= apts[i-1] {
			return 0, len(vpts), len(apts), fmt.Errorf("声音时间不单调")
		}
	}
	st := p.Stats()
	if st.Master != video.MasterAudio {
		return 0, len(vpts), len(apts), fmt.Errorf("主钟掉了")
	}
	if st.Dropped != 0 {
		return 0, len(vpts), len(apts), fmt.Errorf("丢帧了")
	}
	if d := st.AVDiffMs; d < -base.DiffBudgetMs || d > base.DiffBudgetMs {
		return 0, len(vpts), len(apts), fmt.Errorf("声画差太大")
	}
	return st.AVDiffMs, len(vpts), len(apts), nil
}

// checkSeekSerial pins one shared serial: landing hits the video floor,
// the serial bumps once, first stamps converge (video floor, audio
// floor +0..100ms packet cadence), gap inside budget.
func checkSeekSerial(base a2Baseline) (maxAV int64, err error) {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata(base.Clip), video.Options{NowMs: h.at})
	if err != nil {
		return 0, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	for i := 0; i < 20; i++ {
		h.now += 25
		p.PollAudio()
		p.Poll()
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
	for i, sk := range base.Seeks {
		landed, err := p.SeekTo(sk.TargetMs)
		if err != nil {
			return maxAV, fmt.Errorf("跳不动")
		}
		if landed != sk.VideoFloorMs || p.Serial() != int64(i+1) {
			return maxAV, fmt.Errorf("落点/序号对不上")
		}
		var firstV, firstA int64 = -1, -1
		var aSerial int64 = -1
		for tick := 0; tick < 400 && (firstV < 0 || firstA < 0); tick++ {
			h.now += 10
			if af, _ := p.PollAudio(); af != nil && firstA < 0 {
				firstA, aSerial = af.PTSMs, af.Serial
			}
			if vf, _ := p.Poll(); vf != nil && firstV < 0 {
				firstV = vf.PTSMs
			}
			runtime.Gosched()
			time.Sleep(2 * time.Millisecond)
		}
		if firstV != sk.VideoFloorMs && firstV != sk.VideoFloorMs+100 {
			return maxAV, fmt.Errorf("画面落点对不上")
		}
		if firstA < sk.AudioFloorMs || firstA-sk.AudioFloorMs > 100 {
			return maxAV, fmt.Errorf("声音落点对不上")
		}
		if aSerial != int64(i+1) {
			return maxAV, fmt.Errorf("声音序号对不上")
		}
		if d := p.Stats().AVDiffMs; d < -base.DiffBudgetMs || d > base.DiffBudgetMs {
			return maxAV, fmt.Errorf("跳后声画差太大")
		}
		if d := abs64(p.Stats().AVDiffMs); d > maxAV {
			maxAV = d
		}
	}
	return maxAV, nil
}

// checkSilent pins the no-sound fallback: video master, zero gap, the
// old 5-frame playthrough untouched.
func checkSilent() error {
	h := &handClock{}
	p, err := video.OpenFile(resolveTestdata("vr2_720p.mp4"), video.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if p.HasAudio() || p.Master() != video.MasterVideo || p.AVDiffMs() != 0 {
		return fmt.Errorf("静音回落不对")
	}
	n := 0
	ended := false
	for i := 0; i < 8 && !ended; i++ {
		h.now += 200
		fr, done := p.Poll()
		if fr != nil {
			n++
		}
		ended = done
	}
	if !ended || n != 5 {
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
	ev := a2Evidence{Clips: "vr_a2_av.mp4+vr2_720p.mp4", Profile: "Main+AAC"}
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

	playPass, playNote := 0, "声领画随"
	if d, v, a, err := checkPlayHead(base); err != nil {
		playNote = err.Error()
	} else {
		playPass = 1
		playNote = fmt.Sprintf("声领画随 画%d 音%d 差%d", v, a, d)
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
		seekNote = fmt.Sprintf("5跳同序号 最大差%d", d)
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
