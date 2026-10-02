// A4 host audio bridge (VW6 §12 A4 row): the host side of sound.
//
// Say it plain: the engine (video.Player.PollAudio) hands us decoded PCM
// frames (float32 interleaved + stamp + waterline) and never touches the
// speaker; this file carries those frames to the OS sound service and
// reports back honestly. video/ stays device-free per §4 (same ban as
// gpu), so every device handle below lives window-side only.
//
// ffmpeg peers (read-only, fftools/ffplay.c; Serial == Player generation,
// queue Clear == packet_queue_flush without the bump):
//
//	audio_open (:2578-2640, SDL_OpenAudioDevice + next_nb_channels /
//	  next_sample_rates fallback + AUDIO_S16SYS + samples sizing) ->
//	  ProbeHostAudio + NewHostSink (paplay-pulse preferred, aplay-alsa
//	  fallback, wav-file last; the obtained spec is reported, never
//	  faked; raw Pulse-native socket stays future work and is NOT
//	  claimed, see the baseline peer_note)
//	sdl_audio_callback (:2533-2576, pull audio_decode_frame, pad silence
//	  on error, memcpy/Mix into the device buffer, re-anchor audclk
//	  assuming two periods) -> PumpOnce/PumpLoop (pull PollAudio,
//	  FloatToS16, bad packets never surface: the engine conceals them
//	  and the pipe simply stalls, which is silence without a glitch)
//	audio_decode_frame (:2423-2531, readable sampq frame + serial match,
//	  audclk follows played PCM) -> PollAudio already re-anchors the
//	  sound clock at each shown stamp (A2 owns this); the pump only
//	  counts what reached the speaker
//	SDL_PauseAudioDevice (:2814, start) -> the first Write starts the
//	  stream; Pause stops feeding while the pump thread and both clocks
//	  keep living, so a pulled cable never kills play state
//
// Platform table (one row per OS, no blank cells per repo discipline):
//
//	Linux/paplay-pulse | system Pulse client via raw stdin | default, tried first
//	Linux/aplay-alsa   | system ALSA writer via raw stdin  | fallback when paplay missing
//	Linux/wav-file     | RIFF/WAV bytes, no speaker        | headless chain proof only
//	Linux/null         | discard + count                   | sync tests, never a window verdict
//	darwin/*           | honest unsupported, readable err  | no speaker claim, never green
//	windows/*          | honest unsupported, readable err  | no speaker claim, never green
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
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
	BackendWav    = "wav-file"
	BackendNull   = "null"
)

// ProbeHostAudio names the backend NewHostSink would open without
// opening anything. available false carries the honest reason (tried
// paplay then aplay via PATH; darwin/windows have no writer yet).
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
// the decoded AudioFrame, never defaulted here). wavPath non-empty
// allows the wav-file fallback (tests); empty means no fallback: the
// caller gets the honest unavailable error instead of a fake speaker.
func NewHostSink(rate, ch int, wavPath string) (Sink, error) {
	if rate <= 0 || ch <= 0 {
		return nil, fmt.Errorf("a4: bad speaker spec rate=%d ch=%d", rate, ch)
	}
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("a4: no speaker writer on %s yet (linux paplay/aplay only)", runtime.GOOS)
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
				"--client-name=gpui-a4", "--stream-name=video",
			},
		}
		if err := s.start(); err != nil {
			return nil, fmt.Errorf("a4: paplay start: %w", err)
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
			return nil, fmt.Errorf("a4: aplay start: %w", err)
		}
		return s, nil
	}
	if wavPath != "" {
		w, err := NewWavSink(wavPath, rate, ch)
		if err != nil {
			return nil, err
		}
		return w, nil
	}
	return nil, fmt.Errorf("a4: no speaker writer: paplay and aplay both missing from PATH")
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
	// lastErr keeps the first loss text after Resume clears the live
	// error (evidence for the window verdict, not a live state).
	lastErr string
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

// LastError returns the first loss text ("" when never lost).
func (s *cmdSink) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastErr
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
// readable error (unplug drill: kill the child, the pump lives on).
// The first loss text is kept in LastError and survives Resume.
func (s *cmdSink) WritePCM(pcm []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("a4: %s closed", s.backend)
	}
	if s.paused {
		s.mu.Unlock()
		return fmt.Errorf("a4: %s paused (%s)", s.backend, s.errTxt)
	}
	w := s.stdin
	s.mu.Unlock()
	for len(pcm) > 0 {
		n, err := w.Write(pcm)
		if err != nil {
			s.mu.Lock()
			s.paused = true
			s.errTxt = fmt.Sprintf("device lost (%s): %v", s.backend, err)
			if s.lastErr == "" {
				s.lastErr = s.errTxt
			}
			s.mu.Unlock()
			return fmt.Errorf("a4: %s", s.errTxt)
		}
		pcm = pcm[n:]
	}
	return nil
}

// LastError returns the first loss text ("" when never lost). It
// survives Resume; DeviceError reflects the live state.

// Resume reopens the writer after a loss (same spec, fresh child).
// The play clocks never stopped, so sound continues from the newest
// due frame, never from stale pre-loss frames. The loss text survives
// in LastError for verdict evidence; DeviceError reads "" again.
func (s *cmdSink) Resume() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("a4: %s closed", s.backend)
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
		return fmt.Errorf("a4: %s resume: %w", s.backend, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("a4: %s resume: %w", s.backend, err)
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

// KillChild breaks the device pipe on purpose (unplug drill without
// touching hardware): the next Write fails, the sink parks paused.
func (s *cmdSink) KillChild() {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

// PumpStats counts what reached the speaker (plain fields: one pump
// goroutine writes, the window samples under its own lock).
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
// Stat is sampled via Snapshot (the pump goroutine writes, the window
// samples under lock).
type Pump struct {
	Sink Sink
	s16  []int16
	mu   sync.Mutex
	Stat PumpStats
}

// Snapshot copies the counters for the window thread.
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
// False, no error means nothing due yet (caller sleeps or advances its
// clock). Device loss parks the sink paused and counts one drop; the
// pump thread stays alive for Resume.
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
// wrapping caller re-seeks and fresh frames arrive on the new serial
// (the engine wakes the sound thread on seek), so the pump lingers
// instead of dying mid-show.
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
			// Nothing due yet, or the stream end while a wrapping
			// caller re-seeks: linger, never die mid-show.
			_ = ended
			select {
			case <-stopCh:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
}

// wavSink writes RIFF/WAV (16-bit PCM) for the headless chain proof:
// the same bytes the speaker would get, verifiable without hardware.
type wavSink struct {
	f    *os.File
	rate int
	ch   int
	n    int64

	mu     sync.Mutex
	paused bool
	errTxt string
	closed bool
}

// NewWavSink creates path for rate/ch s16le WAV.
func NewWavSink(path string, rate, ch int) (*wavSink, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	w := &wavSink{f: f, rate: rate, ch: ch}
	if err := w.writeHeader(); err != nil {
		f.Close()
		return nil, err
	}
	return w, nil
}

func (w *wavSink) writeHeader() error {
	return writeWavHeader(w.f, w.rate, w.ch, 0, 0)
}

// writeWavHeader writes a 44-byte RIFF/WAV header for rate/ch s16le
// with the given data length (0 while streaming; patched at Close).
func writeWavHeader(w io.Writer, rate, ch int, riffLen, dataLen uint32) error {
	h := make([]byte, 0, 44)
	h = append(h, "RIFF"...)
	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], riffLen)
	h = append(h, b4[:]...)
	h = append(h, "WAVE"...)
	h = append(h, "fmt "...)
	h = append(h, 16, 0, 0, 0) // fmt len
	h = append(h, 1, 0)        // PCM
	h = append(h, byte(ch), 0)
	binary.LittleEndian.PutUint32(b4[:], uint32(rate))
	h = append(h, b4[:]...)
	br := uint32(rate * ch * 2)
	binary.LittleEndian.PutUint32(b4[:], br)
	h = append(h, b4[:]...)
	ba := uint16(ch * 2)
	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], ba)
	h = append(h, b2[:]...)
	binary.LittleEndian.PutUint16(b2[:], 16)
	h = append(h, b2[:]...)
	h = append(h, "data"...)
	binary.LittleEndian.PutUint32(b4[:], dataLen)
	h = append(h, b4[:]...)
	_, err := w.Write(h)
	return err
}

// EncodeWav renders one complete WAV file in memory (same bytes the
// speaker path carries, verifiable without hardware).
func EncodeWav(rate, ch int, pcm []byte) []byte {
	out := make([]byte, 0, 44+len(pcm))
	_ = writeWavHeader(sliceWriter{&out}, rate, ch, uint32(36+len(pcm)), uint32(len(pcm)))
	return append(out, pcm...)
}

type sliceWriter struct{ p *[]byte }

func (s sliceWriter) Write(b []byte) (int, error) {
	*s.p = append(*s.p, b...)
	return len(b), nil
}

func (w *wavSink) Backend() string { return BackendWav }

func (w *wavSink) ObtainedRate() int { return w.rate }

func (w *wavSink) ObtainedChannels() int { return w.ch }

func (w *wavSink) DeviceError() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.errTxt
}

func (w *wavSink) Paused() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.paused
}

func (w *wavSink) Pause() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paused = true
}

func (w *wavSink) Resume() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return fmt.Errorf("a4: wav closed")
	}
	w.paused = false
	w.errTxt = ""
	return nil
}

func (w *wavSink) WritePCM(pcm []byte) error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return fmt.Errorf("a4: wav closed")
	}
	if w.paused {
		w.mu.Unlock()
		return fmt.Errorf("a4: wav paused")
	}
	w.mu.Unlock()
	for len(pcm) > 0 {
		n, err := w.f.Write(pcm)
		if err != nil {
			return err
		}
		w.n += int64(n)
		pcm = pcm[n:]
	}
	return nil
}

func (w *wavSink) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	w.mu.Unlock()
	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(36+w.n))
	if _, err := w.f.WriteAt(b4[:], 4); err != nil {
		w.f.Close()
		return err
	}
	binary.LittleEndian.PutUint32(b4[:], uint32(w.n))
	if _, err := w.f.WriteAt(b4[:], 40); err != nil {
		w.f.Close()
		return err
	}
	return w.f.Close()
}

// nullSink discards everything and counts (sync tests only: it never
// proves sound, so no window verdict may rest on it).
type nullSink struct {
	rate, ch int
	n        int64
}

func (s *nullSink) WritePCM(pcm []byte) error {
	s.n += int64(len(pcm))
	return nil
}
func (s *nullSink) Backend() string       { return BackendNull }
func (s *nullSink) ObtainedRate() int     { return s.rate }
func (s *nullSink) ObtainedChannels() int { return s.ch }
func (s *nullSink) DeviceError() string   { return "" }
func (s *nullSink) Paused() bool          { return false }
func (s *nullSink) Pause()                {}
func (s *nullSink) Resume() error         { return nil }
func (s *nullSink) Close() error          { return nil }
