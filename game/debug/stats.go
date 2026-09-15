package debug

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/energye/gpui/game/core"
)

// CurrentVersion is the report file version every Encode writes.
// Bump Minor for additive fields, Major for a breaking shape.
var CurrentVersion = core.Version{Major: 1, Minor: 0}

// Frozen budgets. A well-formed file beyond budget is OutOfMemory,
// never a guessed load. A bad value is InvalidArg via constructors
// and BadData via Parse.
const (
	// MaxFrames caps retained per-frame samples (60s at 60fps).
	MaxFrames = 3600
	// MaxSpansPerFrame caps breakdown spans carried by one frame.
	MaxSpansPerFrame = 32
	// MaxShaders caps distinct shader names in one Stats.
	MaxShaders = 256
	// MaxNameLen caps span, shader, and crash-reason text.
	MaxNameLen = 64
	// MaxBytes caps one report or crash file.
	MaxBytes = 1 << 20
	// MaxMemoryBytes caps one VRAM sample (512MB).
	MaxMemoryBytes = 512 << 20
)

// MaxFrameDt caps one frame time. A longer Dt (background gap, shrunk
// window) parks at the cap and counts one Gap, so the loop never
// collapses FPS on resume. 250ms follows step.MaxFrame.
const MaxFrameDt = 250 * core.Millisecond

// SlowFrameDt marks a dropped frame. Anything above it counts one
// Slow frame (acceptance: over 20ms at most 3 per minute).
const SlowFrameDt = 20 * core.Millisecond

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func validName(s string) bool { return s != "" && len(s) <= MaxNameLen }

// Span is one breakdown piece inside a frame: who ate how long.
// Fields stay private so every write passes validation.
type Span struct {
	name string
	took core.Duration
}

// NewSpan builds one span. Empty or overlong names and negative times
// are a core InvalidArg error and store nothing.
func NewSpan(name string, took core.Duration) (Span, error) {
	if !validName(name) {
		return Span{}, core.InvalidArg("debug.NewSpan", "name")
	}
	if took < 0 {
		return Span{}, core.InvalidArg("debug.NewSpan", "took")
	}
	return Span{name: name, took: took}, nil
}

// Name returns the span key.
func (s Span) Name() string { return s.name }

// Took returns the span time.
func (s Span) Took() core.Duration { return s.took }

// Frame is one presented frame: time, draw calls, overdraw, a VRAM
// sample, and the breakdown spans. Fields stay private so every write
// passes validation; readers use the accessors below, which copy.
type Frame struct {
	dt       core.Duration
	draws    int
	overdraw float64
	mem      int64
	spans    []Span
}

// NewFrame builds one frame. Non-positive Dt, negative draws or memory,
// non-finite or negative overdraw, and bad spans are a core InvalidArg
// error and store nothing. Oversized span lists are InvalidArg as well;
// the VRAM budget (OutOfMemory) applies at NoteMemory time, not here.
func NewFrame(dt core.Duration, draws int, overdraw float64, mem int64, spans []Span) (Frame, error) {
	if dt <= 0 {
		return Frame{}, core.InvalidArg("debug.NewFrame", "dt")
	}
	if draws < 0 {
		return Frame{}, core.InvalidArg("debug.NewFrame", "draws")
	}
	if !finite(overdraw) || overdraw < 0 {
		return Frame{}, core.InvalidArg("debug.NewFrame", "overdraw")
	}
	if mem < 0 {
		return Frame{}, core.InvalidArg("debug.NewFrame", "mem")
	}
	if len(spans) > MaxSpansPerFrame {
		return Frame{}, core.InvalidArg("debug.NewFrame", "spans")
	}
	for i := range spans {
		if !validName(spans[i].name) {
			return Frame{}, core.InvalidArg("debug.NewFrame", "span")
		}
		if spans[i].took < 0 {
			return Frame{}, core.InvalidArg("debug.NewFrame", "span")
		}
	}
	cp := append([]Span(nil), spans...)
	if cp == nil {
		cp = []Span{}
	}
	return Frame{dt: dt, draws: draws, overdraw: overdraw, mem: mem, spans: cp}, nil
}

// Dt returns the frame time.
func (f Frame) Dt() core.Duration { return f.dt }

// Draws returns the draw calls of the frame.
func (f Frame) Draws() int { return f.draws }

// Overdraw returns the overdraw ratio (1.0 means no overdraw).
func (f Frame) Overdraw() float64 { return f.overdraw }

// MemBytes returns the VRAM sample of the frame.
func (f Frame) MemBytes() int64 { return f.mem }

// Spans returns a copy of the breakdown spans.
// Writing the result cannot change the frame.
func (f Frame) Spans() []Span { return append([]Span(nil), f.spans...) }

// check validates a stored frame for RecordFrame: constructor-built
// frames pass; zero values and hand-made junk fail as InvalidArg.
func (f Frame) check(op string) error {
	if f.dt <= 0 {
		return core.InvalidArg(op, "dt")
	}
	if f.draws < 0 {
		return core.InvalidArg(op, "draws")
	}
	if !finite(f.overdraw) || f.overdraw < 0 {
		return core.InvalidArg(op, "overdraw")
	}
	if f.mem < 0 {
		return core.InvalidArg(op, "mem")
	}
	if len(f.spans) > MaxSpansPerFrame {
		return core.InvalidArg(op, "spans")
	}
	for i := range f.spans {
		if !validName(f.spans[i].name) || f.spans[i].took < 0 {
			return core.InvalidArg(op, "span")
		}
	}
	return nil
}

// clampFrame parks dt at MaxFrameDt and reports whether it clamped.
func clampFrame(dt core.Duration) (core.Duration, bool) {
	if dt > MaxFrameDt {
		return MaxFrameDt, true
	}
	return dt, false
}

// SpanStat is one aggregated breakdown row: total, mean, max, and the
// share of all span time. Exported so reports marshal directly.
type SpanStat struct {
	Name  string  `json:"name"`
	Total int64   `json:"total_ms"`
	Avg   float64 `json:"avg_ms"`
	Max   int64   `json:"max_ms"`
	Share float64 `json:"share"`
}

// ShaderStat is one aggregated shader-compile row. Exported so reports
// marshal directly.
type ShaderStat struct {
	Name     string `json:"name"`
	Compiles int    `json:"compiles"`
	Total    int64  `json:"total_ms"`
	Max      int64  `json:"max_ms"`
}

type shaderAcc struct {
	name  string
	n     int
	total core.Duration
	max   core.Duration
}

// Stats is the collector: retained frames plus shader and memory
// ledgers. The zero value is usable; NewStats only documents intent.
// Readers never fail (nil answers zeros); writers on nil report
// InvalidArg and change nothing.
type Stats struct {
	frames []Frame
	start  int
	total  uint64
	gaps   int
	ooms   int
	shader map[string]*shaderAcc
	curMem int64
	peak   int64
}

// NewStats builds an empty collector.
func NewStats() Stats { return Stats{} }

// Reset clears every ledger, keeping nothing. Nil collectors do nothing.
func (s *Stats) Reset() {
	if s == nil {
		return
	}
	*s = Stats{}
}

// at returns the i-th retained frame (0 is oldest).
func (s *Stats) at(i int) Frame {
	return s.frames[(s.start+i)%len(s.frames)]
}

// Count returns retained frames, or 0 on a nil collector.
func (s *Stats) Count() int {
	if s == nil {
		return 0
	}
	return len(s.frames)
}

// TotalFrames returns frames ever recorded, including evicted ones.
func (s *Stats) TotalFrames() uint64 {
	if s == nil {
		return 0
	}
	return s.total
}

// Gaps returns clamped background-gap frames.
func (s *Stats) Gaps() int {
	if s == nil {
		return 0
	}
	return s.gaps
}

// OOMs returns rejected over-budget memory samples.
func (s *Stats) OOMs() int {
	if s == nil {
		return 0
	}
	return s.ooms
}

// RecordFrame stores one frame. Bad frames are InvalidArg and change
// nothing. Dt beyond MaxFrameDt clamps and counts one Gap; the clamped
// value is what the ledger keeps, so a background gap never collapses
// FPS on resume.
func (s *Stats) RecordFrame(f Frame) error {
	const op = "debug.Stats.RecordFrame"
	if s == nil {
		return core.InvalidArg(op, "stats")
	}
	if err := f.check(op); err != nil {
		return err
	}
	dt, gap := clampFrame(f.dt)
	if gap {
		s.gaps++
	}
	stored := Frame{dt: dt, draws: f.draws, overdraw: f.overdraw, mem: f.mem, spans: append([]Span(nil), f.spans...)}
	if len(s.frames) < MaxFrames {
		if s.frames == nil {
			s.frames = make([]Frame, 0, 64)
		}
		// Keep the backing array a ring once full: while growing, the
		// start stays 0 and append is enough.
		s.frames = append(s.frames, stored)
	} else {
		s.frames[s.start] = stored
		s.start = (s.start + 1) % len(s.frames)
	}
	s.total++
	if stored.mem > s.peak {
		s.peak = stored.mem
	}
	s.curMem = stored.mem
	return nil
}

// RecordShader folds one shader compile into the ledger. Empty or
// overlong names and negative times are InvalidArg; a new name beyond
// MaxShaders is OutOfMemory. Failures change nothing.
func (s *Stats) RecordShader(name string, took core.Duration) error {
	const op = "debug.Stats.RecordShader"
	if s == nil {
		return core.InvalidArg(op, "stats")
	}
	if !validName(name) {
		return core.InvalidArg(op, "name")
	}
	if took < 0 {
		return core.InvalidArg(op, "took")
	}
	if s.shader == nil {
		s.shader = make(map[string]*shaderAcc)
	}
	a, ok := s.shader[name]
	if !ok {
		if len(s.shader) >= MaxShaders {
			return core.OutOfMemory(op, "shaders")
		}
		a = &shaderAcc{name: name}
		s.shader[name] = a
	}
	a.n++
	a.total += took
	if took > a.max {
		a.max = took
	}
	return nil
}

// NoteMemory samples VRAM outside a frame (loading screens, purges).
// Negative samples are InvalidArg; samples beyond MaxMemoryBytes are
// OutOfMemory, count one OOM, and keep the old value.
func (s *Stats) NoteMemory(bytes int64) error {
	const op = "debug.Stats.NoteMemory"
	if s == nil {
		return core.InvalidArg(op, "stats")
	}
	if bytes < 0 {
		return core.InvalidArg(op, "mem")
	}
	if bytes > MaxMemoryBytes {
		s.ooms++
		return core.OutOfMemory(op, "mem")
	}
	s.curMem = bytes
	if bytes > s.peak {
		s.peak = bytes
	}
	return nil
}

func avgMs(total core.Duration, n int) float64 {
	if n <= 0 {
		return 0
	}
	return float64(total) / float64(n)
}

// AvgFrame returns the mean retained frame time, or 0 when empty.
func (s *Stats) AvgFrame() core.Duration {
	if s == nil || len(s.frames) == 0 {
		return 0
	}
	var sum core.Duration
	for i := range s.frames {
		sum += s.at(i).dt
	}
	return core.Duration(int64(sum) / int64(len(s.frames)))
}

// FPS returns 1000/avgMs over retained frames, or 0 when empty.
func (s *Stats) FPS() float64 {
	if s == nil || len(s.frames) == 0 {
		return 0
	}
	avg := avgMs(s.AvgFrame(), 1)
	if avg <= 0 {
		return 0
	}
	return 1000 / avg
}

// P95Frame returns the 95th-percentile retained frame time (ceiling
// rank), or 0 when empty.
func (s *Stats) P95Frame() core.Duration {
	if s == nil || len(s.frames) == 0 {
		return 0
	}
	dts := make([]core.Duration, len(s.frames))
	for i := range s.frames {
		dts[i] = s.at(i).dt
	}
	sort.Slice(dts, func(a, b int) bool { return dts[a] < dts[b] })
	n := len(dts)
	idx := (95*n+99)/100 - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return dts[idx]
}

// SlowFrames counts retained frames above SlowFrameDt.
func (s *Stats) SlowFrames() int {
	if s == nil {
		return 0
	}
	n := 0
	for i := range s.frames {
		if s.at(i).dt > SlowFrameDt {
			n++
		}
	}
	return n
}

// TotalDraws returns retained draw calls.
func (s *Stats) TotalDraws() int64 {
	if s == nil {
		return 0
	}
	var sum int64
	for i := range s.frames {
		sum += int64(s.at(i).draws)
	}
	return sum
}

// AvgDraws returns mean draws per retained frame, or 0 when empty.
func (s *Stats) AvgDraws() float64 {
	if s == nil || len(s.frames) == 0 {
		return 0
	}
	return float64(s.TotalDraws()) / float64(len(s.frames))
}

// CurMemory returns the latest VRAM sample.
func (s *Stats) CurMemory() int64 {
	if s == nil {
		return 0
	}
	return s.curMem
}

// PeakMemory returns the highest VRAM sample seen.
func (s *Stats) PeakMemory() int64 {
	if s == nil {
		return 0
	}
	return s.peak
}

// AvgOverdraw returns the mean retained overdraw ratio, or 0 when empty.
func (s *Stats) AvgOverdraw() float64 {
	if s == nil || len(s.frames) == 0 {
		return 0
	}
	var sum float64
	for i := range s.frames {
		sum += s.at(i).overdraw
	}
	return sum / float64(len(s.frames))
}

// Breakdown aggregates span time over retained frames, sorted by total
// descending (name breaks ties). Share divides by all span time; with
// no spans the result is empty, never NaN.
func (s *Stats) Breakdown() []SpanStat {
	if s == nil {
		return nil
	}
	type acc struct {
		total core.Duration
		max   core.Duration
		n     int
	}
	byName := make(map[string]*acc)
	var grand core.Duration
	for i := range s.frames {
		for _, sp := range s.at(i).spans {
			a, ok := byName[sp.name]
			if !ok {
				a = &acc{}
				byName[sp.name] = a
			}
			a.total += sp.took
			if sp.took > a.max {
				a.max = sp.took
			}
			a.n++
			grand += sp.took
		}
	}
	out := make([]SpanStat, 0, len(byName))
	for name, a := range byName {
		share := 0.0
		if grand > 0 {
			share = float64(a.total) / float64(grand)
		}
		out = append(out, SpanStat{
			Name:  name,
			Total: int64(a.total),
			Avg:   avgMs(a.total, a.n),
			Max:   int64(a.max),
			Share: share,
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Total != out[b].Total {
			return out[a].Total > out[b].Total
		}
		return out[a].Name < out[b].Name
	})
	return out
}

// TopSpans returns the first n Breakdown rows (all when n < 0 or over).
func (s *Stats) TopSpans(n int) []SpanStat {
	all := s.Breakdown()
	if n < 0 || n >= len(all) {
		return all
	}
	return all[:n]
}

// Shaders aggregates shader compiles by name ascending. Nil-safe.
func (s *Stats) Shaders() []ShaderStat {
	if s == nil {
		return nil
	}
	out := make([]ShaderStat, 0, len(s.shader))
	for _, a := range s.shader {
		out = append(out, ShaderStat{
			Name:     a.name,
			Compiles: a.n,
			Total:    int64(a.total),
			Max:      int64(a.max),
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

// ShaderTotal returns all shader compile time.
func (s *Stats) ShaderTotal() core.Duration {
	if s == nil {
		return 0
	}
	var sum core.Duration
	for _, a := range s.shader {
		sum += a.total
	}
	return sum
}

// Report is the frozen snapshot the reporter and the crash log share.
// Exported fields marshal to the frozen file shape directly.
type Report struct {
	Version     string       `json:"version"`
	Frames      int          `json:"frames"`
	TotalFrames uint64       `json:"total_frames"`
	FPS         float64      `json:"fps"`
	AvgMs       float64      `json:"avg_ms"`
	P95Ms       float64      `json:"p95_ms"`
	Slow        int          `json:"slow"`
	Gaps        int          `json:"gaps"`
	OOMs        int          `json:"ooms"`
	TotalDraws  int64        `json:"total_draws"`
	AvgDraws    float64      `json:"avg_draws"`
	MemCur      int64        `json:"mem_cur"`
	MemPeak     int64        `json:"mem_peak"`
	Overdraw    float64      `json:"overdraw"`
	Breakdown   []SpanStat   `json:"breakdown"`
	Shaders     []ShaderStat `json:"shaders"`
	ShaderMs    float64      `json:"shader_ms"`
}

// Report snapshots the ledgers. Nil collectors report a zero snapshot
// at the current version, never a crash.
func (s *Stats) Report() Report {
	ver := CurrentVersion.String()
	if s == nil {
		return Report{Version: ver, Breakdown: []SpanStat{}, Shaders: []ShaderStat{}}
	}
	avg := s.AvgFrame()
	bd := s.Breakdown()
	if bd == nil {
		bd = []SpanStat{}
	}
	sh := s.Shaders()
	if sh == nil {
		sh = []ShaderStat{}
	}
	return Report{
		Version:     ver,
		Frames:      len(s.frames),
		TotalFrames: s.total,
		FPS:         s.FPS(),
		AvgMs:       avgMs(avg, 1),
		P95Ms:       avgMs(s.P95Frame(), 1),
		Slow:        s.SlowFrames(),
		Gaps:        s.gaps,
		OOMs:        s.ooms,
		TotalDraws:  s.TotalDraws(),
		AvgDraws:    s.AvgDraws(),
		MemCur:      s.curMem,
		MemPeak:     s.peak,
		Overdraw:    s.AvgOverdraw(),
		Breakdown:   bd,
		Shaders:     sh,
		ShaderMs:    avgMs(s.ShaderTotal(), 1),
	}
}

// Equal reports whether o carries the same snapshot. Floats compare
// exact: both sides derive from integer ledgers, so equal inputs give
// identical bits.
func (r Report) Equal(o Report) bool {
	if r.Version != o.Version || r.Frames != o.Frames || r.TotalFrames != o.TotalFrames ||
		r.FPS != o.FPS || r.AvgMs != o.AvgMs || r.P95Ms != o.P95Ms ||
		r.Slow != o.Slow || r.Gaps != o.Gaps || r.OOMs != o.OOMs ||
		r.TotalDraws != o.TotalDraws || r.AvgDraws != o.AvgDraws ||
		r.MemCur != o.MemCur || r.MemPeak != o.MemPeak ||
		r.Overdraw != o.Overdraw || r.ShaderMs != o.ShaderMs {
		return false
	}
	if len(r.Breakdown) != len(o.Breakdown) || len(r.Shaders) != len(o.Shaders) {
		return false
	}
	for i := range r.Breakdown {
		if r.Breakdown[i] != o.Breakdown[i] {
			return false
		}
	}
	for i := range r.Shaders {
		if r.Shaders[i] != o.Shaders[i] {
			return false
		}
	}
	return true
}

// Encode renders the canonical file bytes. Budget overruns are
// OutOfMemory. The input is never mutated.
func (r Report) Encode() ([]byte, error) {
	const op = "debug.Report.Encode"
	out, err := json.Marshal(r)
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

type jsonReport Report

// ParseReport validates file bytes into a Report. Empty input is
// InvalidArg, size overruns are OutOfMemory, torn JSON and bad shapes
// are BadData, foreign or newer versions are VersionMismatch.
func ParseReport(data []byte) (Report, error) {
	const op = "debug.ParseReport"
	if len(data) == 0 {
		return Report{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxBytes {
		return Report{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonReport
	if err := json.Unmarshal(data, &raw); err != nil {
		return Report{}, core.BadData(op, "json")
	}
	if raw.Version == "" {
		return Report{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return Report{}, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(CurrentVersion) {
		return Report{}, core.VersionMismatch(op, ver.String())
	}
	out := Report(raw)
	if out.Breakdown == nil {
		out.Breakdown = []SpanStat{}
	}
	if out.Shaders == nil {
		out.Shaders = []ShaderStat{}
	}
	return out, nil
}

// StoreReport writes r to path, creating parent directories.
// Empty paths are InvalidArg; OS failures are NotFound with the cause.
func StoreReport(r Report, path string) error {
	const op = "debug.StoreReport"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := r.Encode()
	if err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if dir := filepath.Dir(clean); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return core.NotFound(op, path, err)
		}
	}
	if err := os.WriteFile(clean, raw, 0o600); err != nil {
		return core.NotFound(op, path, err)
	}
	return nil
}

// LoadReport reads path as a report file. Empty paths are InvalidArg,
// missing files are NotFound; the rest matches ParseReport.
func LoadReport(path string) (Report, error) {
	const op = "debug.LoadReport"
	if path == "" {
		return Report{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Report{}, core.NotFound(op, path, err)
	}
	return ParseReport(raw)
}

// Text renders the human-readable report the F scenario reads: one
// header line (frames, rate, counters), one draw/memory line, one
// breakdown line ordered by share, one shader line by name. The shape
// is frozen; stats_report_golden.txt pins the mini case byte for byte.
func (r Report) Text() string {
	var b strings.Builder
	b.WriteString("stats v" + r.Version)
	b.WriteString(" frames=" + itoa(r.Frames))
	b.WriteString(" total=" + uitoa(r.TotalFrames))
	b.WriteString(" fps=" + ftoa(r.FPS, 2))
	b.WriteString(" avg=" + ftoa(r.AvgMs, 3) + "ms")
	b.WriteString(" p95=" + ftoa(r.P95Ms, 3) + "ms")
	b.WriteString(" slow=" + itoa(r.Slow))
	b.WriteString(" gaps=" + itoa(r.Gaps))
	b.WriteString(" ooms=" + itoa(r.OOMs))
	b.WriteString("\n")
	b.WriteString("draws total=" + i64toa(r.TotalDraws))
	b.WriteString(" avg=" + ftoa(r.AvgDraws, 2) + "/frame")
	b.WriteString(" overdraw=" + ftoa(r.Overdraw, 3) + "x")
	b.WriteString(" mem cur=" + i64toa(r.MemCur) + "B peak=" + i64toa(r.MemPeak) + "B")
	b.WriteString("\n")
	b.WriteString("spans " + itoa(len(r.Breakdown)) + ":")
	if len(r.Breakdown) == 0 {
		b.WriteString(" -")
	} else {
		for i, sp := range r.Breakdown {
			if i > 0 {
				b.WriteString(" |")
			}
			b.WriteString(" " + sp.Name + " " + i64toa(sp.Total) + "ms " + ftoa(sp.Share*100, 1) + "%")
		}
	}
	b.WriteString("\n")
	b.WriteString("shaders " + itoa(len(r.Shaders)) + " total=" + ftoa(r.ShaderMs, 0) + "ms:")
	if len(r.Shaders) == 0 {
		b.WriteString(" -")
	} else {
		for i, sh := range r.Shaders {
			if i > 0 {
				b.WriteString(" |")
			}
			b.WriteString(" " + sh.Name + " " + i64toa(sh.Total) + "ms x" + itoa(sh.Compiles))
		}
	}
	b.WriteString("\n")
	return b.String()
}

// CrashLog is one crash/uptime snapshot: why the run stopped plus the
// frozen report at that moment. Exported fields marshal directly.
type CrashLog struct {
	Version string `json:"version"`
	Reason  string `json:"reason"`
	Code    string `json:"code"`
	Report  Report `json:"report"`
}

// Snapshot freezes the ledgers with a reason and a core code for the
// long-run and crash reporters. Nil collectors freeze a zero report.
func (s *Stats) Snapshot(reason string, code core.Code) CrashLog {
	return CrashLog{
		Version: CurrentVersion.String(),
		Reason:  reason,
		Code:    code.String(),
		Report:  s.Report(),
	}
}

// Equal reports whether o carries the same crash snapshot.
func (c CrashLog) Equal(o CrashLog) bool {
	return c.Version == o.Version && c.Reason == o.Reason &&
		c.Code == o.Code && c.Report.Equal(o.Report)
}

// Encode renders the canonical crash bytes. Budget overruns are
// OutOfMemory.
func (c CrashLog) Encode() ([]byte, error) {
	const op = "debug.CrashLog.Encode"
	out, err := json.Marshal(c)
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

type jsonCrash CrashLog

// ParseCrash validates crash bytes. Error levels match ParseReport.
func ParseCrash(data []byte) (CrashLog, error) {
	const op = "debug.ParseCrash"
	if len(data) == 0 {
		return CrashLog{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxBytes {
		return CrashLog{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonCrash
	if err := json.Unmarshal(data, &raw); err != nil {
		return CrashLog{}, core.BadData(op, "json")
	}
	if raw.Version == "" {
		return CrashLog{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return CrashLog{}, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(CurrentVersion) {
		return CrashLog{}, core.VersionMismatch(op, ver.String())
	}
	out := CrashLog(raw)
	if out.Report.Breakdown == nil {
		out.Report.Breakdown = []SpanStat{}
	}
	if out.Report.Shaders == nil {
		out.Report.Shaders = []ShaderStat{}
	}
	return out, nil
}

// StoreCrash writes c to path, creating parent directories.
func StoreCrash(c CrashLog, path string) error {
	const op = "debug.StoreCrash"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := c.Encode()
	if err != nil {
		return err
	}
	clean := filepath.Clean(path)
	if dir := filepath.Dir(clean); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return core.NotFound(op, path, err)
		}
	}
	if err := os.WriteFile(clean, raw, 0o600); err != nil {
		return core.NotFound(op, path, err)
	}
	return nil
}

// LoadCrash reads path as a crash file. Levels match LoadReport.
func LoadCrash(path string) (CrashLog, error) {
	const op = "debug.LoadCrash"
	if path == "" {
		return CrashLog{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return CrashLog{}, core.NotFound(op, path, err)
	}
	return ParseCrash(raw)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var out [20]byte
	p := len(out)
	for i > 0 {
		p--
		out[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		out[p] = '-'
	}
	return string(out[p:])
}

func uitoa(u uint64) string {
	if u == 0 {
		return "0"
	}
	var out [20]byte
	p := len(out)
	for u > 0 {
		p--
		out[p] = byte('0' + u%10)
		u /= 10
	}
	return string(out[p:])
}

func i64toa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var out [20]byte
	p := len(out)
	for v > 0 {
		p--
		out[p] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		p--
		out[p] = '-'
	}
	return string(out[p:])
}

// ftoa formats f with prec decimals (round half away from zero).
// Integers print without a point so golden text stays exact.
func ftoa(f float64, prec int) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	mult := 1.0
	for i := 0; i < prec; i++ {
		mult *= 10
	}
	r := math.Round(f*mult) / mult
	if r == 0 {
		r = 0 // Hide negative zero from -0.004 style shares.
	}
	if prec == 0 {
		return i64toa(int64(r))
	}
	neg := r < 0
	if neg {
		r = -r
	}
	ip := int64(r)
	fp := int64(math.Round((r - float64(ip)) * mult))
	if fp >= int64(mult) {
		ip++
		fp -= int64(mult)
	}
	frac := itoa(int(fp))
	for len(frac) < prec {
		frac = "0" + frac
	}
	if neg {
		return "-" + i64toa(ip) + "." + frac
	}
	return i64toa(ip) + "." + frac
}
