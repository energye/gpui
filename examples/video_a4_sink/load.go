// A4 gate probes (VW6 §12 A4 row): identity + conversion + WAV chain
// + null-pump sync + device honesty. Same clip and numbers as
// video/testdata/a4_ffmpeg.json, so the window never drifts from the
// gate. Numbers come from the real engine, never hand-written; the
// baseline JSON is read for expect values.
//
// The speaker itself is exercised by the live window (a headless test
// cannot hear); what runs headless is everything around it: the shell
// identity, the exact float->s16 bytes, the WAV chain carrying those
// bytes, the sound-led sync through a counting sink, and the honest
// device probe (present or readable-unavailable, never faked).
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"time"

	govideo "github.com/energye/gpui/video"
	"github.com/energye/gpui/video/mp4"
)

type a4Packet struct {
	Size  int   `json:"size"`
	PTSMs int64 `json:"pts_ms"`
}

type a4Baseline struct {
	Clip  string `json:"clip"`
	Audio struct {
		Profile      string     `json:"profile"`
		SampleRate   int        `json:"sample_rate"`
		Channels     int        `json:"channels"`
		Samples      int        `json:"samples"`
		ASCHex       string     `json:"asc_hex"`
		FirstPackets []a4Packet `json:"first_packets"`
	} `json:"audio"`
	Video struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"video"`
	DiffBudgetMs int64 `json:"diff_budget_ms"`
	Device       struct {
		SinkSpec string `json:"sink_spec"`
	} `json:"device_baseline"`
}

type a4Group struct {
	Name   string `json:"name"`
	Passed int    `json:"passed"`
	Total  int    `json:"total"`
	Note   string `json:"note"`
}

type a4Evidence struct {
	Groups    []a4Group `json:"groups"`
	Passed    int       `json:"passed"`
	Total     int       `json:"total"`
	Failed    int       `json:"failed_items"`
	Clips     string    `json:"clips"`
	Backend   string    `json:"backend"`
	Available bool      `json:"available"`
	Reason    string    `json:"reason"`
	ErrText   string    `json:"err"`
}

func resolveA4(name string) string {
	for _, p := range []string{
		"video/testdata/" + name,
		"../../video/testdata/" + name,
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "video/testdata/" + name
}

func resolveA4Vectors() string {
	for _, p := range []string{
		"examples/video_a4_sink/testdata/a4_vectors.json",
		"testdata/a4_vectors.json",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "testdata/a4_vectors.json"
}

func loadA4Baseline() (a4Baseline, error) {
	var base a4Baseline
	buf, err := os.ReadFile(resolveA4("a4_ffmpeg.json"))
	if err != nil {
		return base, fmt.Errorf("基线缺了: %w", err)
	}
	if err := json.Unmarshal(buf, &base); err != nil {
		return base, fmt.Errorf("基线坏了: %w", err)
	}
	if base.Clip == "" {
		return base, fmt.Errorf("基线没片子")
	}
	return base, nil
}

type handClock struct{ now int64 }

func (h *handClock) at() int64 { return h.now }

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

// checkA4Identity pins the gate clip shell against the baseline: video
// dims plus LC AAC rate/channels/packet tables.
func checkA4Identity(base a4Baseline) error {
	m, err := mp4.ParseFile(resolveA4(base.Clip))
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

type a4Vector struct {
	In   float64 `json:"in"`
	Want int     `json:"want"`
}

// checkA4Convert pins FloatToS16 against the vector data file (plus the
// NaN guard, which JSON cannot spell: NaN must come out silence).
func checkA4Convert() (int, int, error) {
	buf, err := os.ReadFile(resolveA4Vectors())
	if err != nil {
		return 0, 0, fmt.Errorf("向量缺了: %w", err)
	}
	var vf struct {
		Vectors []a4Vector `json:"vectors"`
	}
	if err := json.Unmarshal(buf, &vf); err != nil {
		return 0, 0, fmt.Errorf("向量坏了: %w", err)
	}
	if len(vf.Vectors) == 0 {
		return 0, 0, fmt.Errorf("向量没数据")
	}
	for i, v := range vf.Vectors {
		got := make([]int16, 1)
		FloatToS16(got, []float32{float32(v.In)})
		if int(got[0]) != v.Want {
			return 0, len(vf.Vectors), fmt.Errorf("向量%d对不上", i)
		}
		raw := S16Bytes(nil, got)
		if len(raw) != 2 || int(int16(binary.LittleEndian.Uint16(raw))) != v.Want {
			return 0, len(vf.Vectors), fmt.Errorf("向量%d字节序不对", i)
		}
	}
	nan := make([]int16, 1)
	FloatToS16(nan, []float32{float32(math.NaN())})
	if nan[0] != 0 {
		return 0, len(vf.Vectors), fmt.Errorf("NaN没垫静音")
	}
	return len(vf.Vectors), len(vf.Vectors), nil
}

// recSink records speaker bytes in memory (chain proof without
// hardware; never a window verdict on its own).
type recSink struct {
	rate, ch int
	blocks   [][]byte
}

func (s *recSink) WritePCM(pcm []byte) error {
	s.blocks = append(s.blocks, append([]byte(nil), pcm...))
	return nil
}
func (s *recSink) Backend() string       { return "record" }
func (s *recSink) ObtainedRate() int     { return s.rate }
func (s *recSink) ObtainedChannels() int { return s.ch }
func (s *recSink) DeviceError() string   { return "" }
func (s *recSink) Paused() bool          { return false }
func (s *recSink) Pause()                {}
func (s *recSink) Resume() error         { return nil }
func (s *recSink) Close() error          { return nil }

// checkA4Wav plays the head through the real decoder into memory and
// proves the WAV chain carries the speaker bytes exactly (same bytes
// paplay would get, verifiable without hardware).
func checkA4Wav(base a4Baseline) error {
	h := &handClock{}
	p, err := govideo.OpenFile(resolveA4(base.Clip), govideo.Options{NowMs: h.at})
	if err != nil {
		return fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if !p.HasAudio() {
		return fmt.Errorf("没进声路")
	}
	if ai, err := govideo.ProbeAudio(resolveA4(base.Clip)); err != nil {
		return fmt.Errorf("声信息读不出: %v", err)
	} else if ai.SampleRate != base.Audio.SampleRate || ai.Channels != base.Audio.Channels {
		return fmt.Errorf("声规格对不上")
	}
	rec := &recSink{rate: base.Audio.SampleRate, ch: base.Audio.Channels}
	pm := &Pump{Sink: rec}
	for i := 0; i < 400 && len(rec.blocks) < 3; i++ {
		h.now += 10
		if _, done, err := pm.Once(p); err != nil {
			return fmt.Errorf("泵坏了: %v", err)
		} else if done {
			break
		}
		if len(rec.blocks) == 1 && len(rec.blocks[0]) > 0 {
			// First block landed; spec already fixed at sink build.
		}
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
	}
	blocks := rec.blocks
	if len(blocks) < 3 {
		return fmt.Errorf("声音出太少")
	}
	var pcm []byte
	for _, b := range blocks {
		pcm = append(pcm, b...)
	}
	file := EncodeWav(base.Audio.SampleRate, base.Audio.Channels, pcm)
	if len(file) != 44+len(pcm) || string(file[:4]) != "RIFF" || string(file[8:12]) != "WAVE" {
		return fmt.Errorf("WAV头不对")
	}
	if !bytes.Equal(file[44:], pcm) {
		return fmt.Errorf("WAV数据不对")
	}
	return nil
}

// checkA4Pump pins sound-led sync through a counting sink (no speaker):
// both stamps rise monotonically, master reads audio, drops stay zero,
// the gap ends inside budget. The speaker changes nothing above this:
// PumpOnce is the only caller of WritePCM.
func checkA4Pump(base a4Baseline) (avdiff int64, vshown, ashown int, err error) {
	h := &handClock{}
	p, err := govideo.OpenFile(resolveA4(base.Clip), govideo.Options{NowMs: h.at})
	if err != nil {
		return 0, 0, 0, fmt.Errorf("打不开: %v", err)
	}
	defer p.Close()
	if !p.HasAudio() || p.Master() != govideo.MasterAudio {
		return 0, 0, 0, fmt.Errorf("主钟不对")
	}
	pm := &Pump{Sink: &nullSink{rate: base.Audio.SampleRate, ch: base.Audio.Channels}}
	var vpts, apts []int64
	for i := 0; i < 120; i++ {
		h.now += 25
		if _, _, err := pm.Once(p); err != nil {
			return 0, len(vpts), len(apts), fmt.Errorf("泵坏了: %v", err)
		}
		snap := pm.Snapshot()
		if snap.PlayedPkts > int64(len(apts)) {
			apts = append(apts, snap.LastPTS)
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
	if st.Master != govideo.MasterAudio {
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

// checkA4Device reports the honest speaker state: backend name when a
// writer exists, otherwise the readable reason. It never fails the
// gate by itself; the window verdict decides green vs skip.
func checkA4Device() (backend string, available bool, reason string) {
	return ProbeHostAudio()
}

// loadA4 runs the A4 gate groups: identity 1 + convert N + wav 1 +
// pump 1 + device 0/1 (device counts only when a writer exists;
// headless stays honest-unavailable instead of faking a pass).
func loadA4() a4Evidence {
	ev := a4Evidence{Clips: "vr_a2_av.mp4"}
	base, err := loadA4Baseline()
	if err != nil {
		ev.ErrText = err.Error()
		return ev
	}

	idPass, idNote := 0, "壳全对"
	if err := checkA4Identity(base); err != nil {
		idNote = err.Error()
	} else {
		idPass = 1
	}
	ev.Groups = append(ev.Groups, a4Group{Name: "identity", Passed: idPass, Total: 1, Note: idNote})

	cvPass, cvTotal := 0, 0
	cvNote := "转s16"
	if got, total, err := checkA4Convert(); err != nil {
		cvNote = err.Error()
		cvTotal = total
	} else {
		cvPass, cvTotal = got, total
		cvNote = fmt.Sprintf("%d向量逐位对", got)
	}
	ev.Groups = append(ev.Groups, a4Group{Name: "convert", Passed: cvPass, Total: cvTotal, Note: cvNote})

	wvPass, wvNote := 0, "WAV链"
	if err := checkA4Wav(base); err != nil {
		wvNote = err.Error()
	} else {
		wvPass = 1
		wvNote = "头3包WAV逐位对"
	}
	ev.Groups = append(ev.Groups, a4Group{Name: "wav", Passed: wvPass, Total: 1, Note: wvNote})

	ppPass, ppNote := 0, "声领画随"
	if d, v, a, err := checkA4Pump(base); err != nil {
		ppNote = err.Error()
	} else {
		ppPass = 1
		ppNote = fmt.Sprintf("声领画随 画%d 音%d 差%d", v, a, d)
	}
	ev.Groups = append(ev.Groups, a4Group{Name: "pump", Passed: ppPass, Total: 1, Note: ppNote})

	backend, available, reason := checkA4Device()
	ev.Backend, ev.Available, ev.Reason = backend, available, reason
	dvPass, dvTotal, dvNote := 0, 1, reason
	if available {
		dvPass = 1
		dvNote = backend + "就绪"
	} else {
		dvNote = "无喇叭:" + reason
	}
	ev.Groups = append(ev.Groups, a4Group{Name: "device", Passed: dvPass, Total: dvTotal, Note: dvNote})

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
			if g.Name == "device" {
				continue
			}
			if g.Passed != g.Total {
				ev.ErrText = g.Name + ":" + g.Note
				break
			}
		}
	}
	return ev
}

func (ev a4Evidence) infoLines() []string {
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
