package hostsink

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os/exec"
	"runtime"
	"sync"
	"time"

	govideo "github.com/energye/gpui/video"
)

// FloatToS16 converts interleaved float PCM in [-1,1] to s16, clipping
// over-range (and non-finite) samples. 1.0 maps to 32767, -1.0 to
// -32768; dst must hold len(src) samples. Pure math, no device.
func FloatToS16(dst []int16, src []float32) {
	for i, v := range src {
		f := float64(v)
		if math.IsNaN(f) {
			f = 0
		}
		f *= 32768
		if f > 32767 {
			f = 32767
		} else if f < -32768 {
			f = -32768
		}
		dst[i] = int16(math.Round(f))
	}
}

// S16Bytes appends s16 samples as little-endian bytes (the wire format
// paplay/aplay read with --format=s16le / -f S16_LE).
func S16Bytes(out []byte, s16 []int16) []byte {
	for _, v := range s16 {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], uint16(v))
		out = append(out, b[0], b[1])
	}
	return out
}

// Sink is one speaker path. WritePCM blocks with backpressure (the
// device sets the pace, like SDL's audio thread); any error means the
// device is gone: the sink parks itself paused with a readable error
// and the pump thread stays alive for Resume.
type Sink interface {
	WritePCM(pcm []byte) error
	Backend() string
	ObtainedRate() int
	ObtainedChannels() int
	DeviceError() string
	Paused() bool
	Pause()
	Resume() error
	Close() error
}

// Backends in preference order (mirrors audio_open's fallback lists).
const (
	BackendPaplay = "paplay-pulse"
	BackendAplay  = "aplay-alsa"
)

// ProbeHostAudio names the backend NewHostSink would open without
// opening anything. available false carries the honest reason (tried
// paplay then aplay via PATH; darwin/windows have no writer yet).
// Callers with no speaker play video silently instead of failing.
func ProbeHostAudio() (backend string, available bool, reason string) {
	if runtime.GOOS != "linux" {
		return "", false, fmt.Sprintf("no speaker writer on %s yet (linux paplay/aplay only)", runtime.GOOS)
	}
	if _, err := exec.LookPath("paplay"); err == nil {
		return BackendPaplay, true, ""
	}
	if _, err := exec.LookPath("aplay"); err == nil {
		return BackendAplay, true, ""
	}
	return "", false, "no speaker writer: paplay and aplay both missing from PATH"
}

// NewHostSink opens the preferred speaker path for rate/ch (taken from
// the decoded AudioFrame, never defaulted here). No file fallback: when
// no writer exists the caller gets the honest unavailable error and
// plays video silently instead of faking a speaker.
func NewHostSink(rate, ch int) (Sink, error) {
	if rate <= 0 || ch <= 0 {
		return nil, fmt.Errorf("hostsink: bad speaker spec rate=%d ch=%d", rate, ch)
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("hostsink: no speaker writer on %s yet (linux paplay/aplay only)", runtime.GOOS)
	}
	if path, err := exec.LookPath("paplay"); err == nil {
		s := &cmdSink{
			backend: BackendPaplay,
			rate:    rate, ch: ch,
			path: path,
			args: []string{
				"--raw", "--format=s16le",
				fmt.Sprintf("--rate=%d", rate),
				fmt.Sprintf("--channels=%d", ch),
				"--client-name=gpui-player", "--stream-name=video",
			},
		}
		if err := s.start(); err != nil {
			return nil, fmt.Errorf("hostsink: paplay start: %w", err)
		}
		return s, nil
	}
	if path, err := exec.LookPath("aplay"); err == nil {
		s := &cmdSink{
			backend: BackendAplay,
			rate:    rate, ch: ch,
			path: path,
			args: []string{
				"-q", "-t", "raw", "-f", "S16_LE",
				"-r", fmt.Sprintf("%d", rate),
				"-c", fmt.Sprintf("%d", ch),
			},
		}
		if err := s.start(); err != nil {
			return nil, fmt.Errorf("hostsink: aplay start: %w", err)
		}
		return s, nil
	}
	return nil, fmt.Errorf("hostsink: no speaker writer: paplay and aplay both missing from PATH")
}

// cmdSink is paplay/aplay driven through its raw-stdin pipe. Writes
// block (device-paced, like the SDL audio thread); a broken pipe or a
// dead child parks the sink paused with a readable error.
type cmdSink struct {
	backend string
	rate    int
	ch      int
	path    string
	args    []string

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	paused bool
	errTxt string
	closed bool
}

func (s *cmdSink) start() error {
	cmd := exec.Command(s.path, s.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return err
	}
	s.mu.Lock()
	s.cmd, s.stdin = cmd, stdin
	s.mu.Unlock()
	return nil
}

func (s *cmdSink) Backend() string { return s.backend }

func (s *cmdSink) ObtainedRate() int { return s.rate }

func (s *cmdSink) ObtainedChannels() int { return s.ch }

func (s *cmdSink) DeviceError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errTxt
}

func (s *cmdSink) Paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

func (s *cmdSink) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
}

// WritePCM feeds one PCM block. On device loss it parks paused with a
// readable error; the pump thread stays alive for Resume.
func (s *cmdSink) WritePCM(pcm []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("hostsink: %s closed", s.backend)
	}
	if s.paused {
		s.mu.Unlock()
		return fmt.Errorf("hostsink: %s paused (%s)", s.backend, s.errTxt)
	}
	w := s.stdin
	s.mu.Unlock()
	for len(pcm) > 0 {
		n, err := w.Write(pcm)
		if err != nil {
			s.mu.Lock()
			s.paused = true
			s.errTxt = fmt.Sprintf("device lost (%s): %v", s.backend, err)
			s.mu.Unlock()
			return fmt.Errorf("hostsink: %s", s.errTxt)
		}
		pcm = pcm[n:]
	}
	return nil
}

// Resume reopens the writer after a loss (same spec, fresh child).
// The play clocks never stopped, so sound continues from the newest
// due frame, never from stale pre-loss frames.
func (s *cmdSink) Resume() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("hostsink: %s closed", s.backend)
	}
	old := s.cmd
	s.mu.Unlock()
	if old != nil && old.Process != nil {
		_ = old.Process.Kill()
		_ = old.Wait()
	}
	cmd := exec.Command(s.path, s.args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("hostsink: %s resume: %w", s.backend, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("hostsink: %s resume: %w", s.backend, err)
	}
	s.mu.Lock()
	s.cmd, s.stdin = cmd, stdin
	s.paused = false
	s.errTxt = ""
	s.mu.Unlock()
	return nil
}

func (s *cmdSink) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	w, cmd := s.stdin, s.cmd
	s.mu.Unlock()
	var err error
	if w != nil {
		err = w.Close()
	}
	if cmd != nil {
		_ = cmd.Wait()
	}
	return err
}

// PumpStats counts what reached the speaker (plain fields: one pump
// goroutine writes, the owner samples under the pump lock).
type PumpStats struct {
	PlayedBytes int64 // PCM bytes accepted by the device
	PlayedPkts  int64 // AudioFrames written
	Starved     int64 // ticks with nothing due yet (stream alive)
	Drops       int64 // device-loss events (paused, thread alive)
	Resumes     int64 // successful Resume after a loss
	LastPTS     int64 // newest played stamp
}

// Pump pulls due PCM into its sink. The s16 scratch buffer is reused
// across calls (no per-packet alloc); use one Pump per goroutine.
// Stat is sampled via Snapshot (the pump goroutine writes, the owner
// samples under lock).
type Pump struct {
	Sink Sink
	s16  []int16
	mu   sync.Mutex
	Stat PumpStats
}

// Snapshot copies the counters for the owner thread.
func (pm *Pump) Snapshot() PumpStats {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.Stat
}

func (pm *Pump) add(fn func(*PumpStats)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	fn(&pm.Stat)
}

// Once pulls the newest due PCM frame and feeds it to the sink.
// False, no error means nothing due yet (caller advances its clock).
// Device loss parks the sink paused and counts one drop; the pump
// thread stays alive for Resume.
func (pm *Pump) Once(p *govideo.Player) (wrote bool, ended bool, err error) {
	af, done := p.PollAudio()
	if af == nil {
		if done {
			return false, true, nil
		}
		pm.add(func(st *PumpStats) { st.Starved++ })
		return false, false, nil
	}
	need := len(af.Data)
	if cap(pm.s16) < need {
		pm.s16 = make([]int16, need)
	}
	pm.s16 = pm.s16[:need]
	FloatToS16(pm.s16, af.Data)
	raw := S16Bytes(nil, pm.s16)
	if werr := pm.Sink.WritePCM(raw); werr != nil {
		pm.add(func(st *PumpStats) { st.Drops++ })
		return false, false, werr
	}
	pm.add(func(st *PumpStats) {
		st.PlayedBytes += int64(len(raw))
		st.PlayedPkts++
		st.LastPTS = af.PTSMs
	})
	return true, done, nil
}

// PumpLoop is the SDL-audio-thread shape: it owns one goroutine,
// blocks on the device, survives device loss (pause + retry every 2s),
// and only returns when stop closes. End-of-stream does NOT end it: a
// replay re-seeks and fresh frames arrive on the new serial, so the
// pump lingers instead of dying mid-show.
func PumpLoop(pm *Pump, p *govideo.Player, stopCh <-chan struct{}, resumer func(Sink) bool) {
	retryT := time.NewTicker(2 * time.Second)
	defer retryT.Stop()
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		if pm.Sink.Paused() {
			select {
			case <-stopCh:
				return
			case <-retryT.C:
				if resumer != nil && resumer(pm.Sink) {
					pm.add(func(st *PumpStats) { st.Resumes++ })
				}
			case <-time.After(50 * time.Millisecond):
			}
			continue
		}
		wrote, ended, werr := pm.Once(p)
		if werr != nil {
			continue
		}
		if !wrote {
			// Nothing due yet, or the stream end while the owner
			// re-seeks: linger, never die mid-show.
			_ = ended
			select {
			case <-stopCh:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
}
