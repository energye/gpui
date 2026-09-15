package debug

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type spanCase struct {
	Name   string `json:"name"`
	TookMs int64  `json:"took_ms"`
}

type frameCase struct {
	Name     string     `json:"name"`
	DtMs     int64      `json:"dt_ms"`
	Draws    int        `json:"draws"`
	Overdraw float64    `json:"overdraw"`
	Mem      int64      `json:"mem"`
	Spans    []spanCase `json:"spans"`
}

type shaderCase struct {
	Name    string  `json:"name"`
	TakesMs []int64 `json:"takes_ms"`
}

type breakdownCase struct {
	Name    string  `json:"name"`
	TotalMs int64   `json:"total_ms"`
	AvgMs   float64 `json:"avg_ms"`
	MaxMs   int64   `json:"max_ms"`
}

type expectCase struct {
	Frames      int             `json:"frames"`
	TotalFrames uint64          `json:"total_frames"`
	AvgMs       float64         `json:"avg_ms"`
	P95Ms       float64         `json:"p95_ms"`
	Slow        int             `json:"slow"`
	TotalDraws  int64           `json:"total_draws"`
	AvgDraws    float64         `json:"avg_draws"`
	MemCur      int64           `json:"mem_cur"`
	MemPeak     int64           `json:"mem_peak"`
	Overdraw    float64         `json:"overdraw"`
	ShaderMs    float64         `json:"shader_ms"`
	Breakdown   []breakdownCase `json:"breakdown"`
}

type badFileCase struct {
	File     string `json:"file"`
	WantCode string `json:"want_code"`
}

type crashCase struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
	Code   string `json:"code"`
}

type casesFile struct {
	Version     string        `json:"version"`
	MaxFrames   int           `json:"max_frames"`
	SlowDtMs    int64         `json:"slow_dt_ms"`
	MaxDtMs     int64         `json:"max_dt_ms"`
	Frames      []frameCase   `json:"frames"`
	Shaders     []shaderCase  `json:"shaders"`
	Expect      expectCase    `json:"expect"`
	MiniFrames  []frameCase   `json:"mini_frames"`
	MiniShaders []shaderCase  `json:"mini_shaders"`
	BadFiles    []badFileCase `json:"bad_files"`
	Crash       crashCase     `json:"crash"`
}

func loadStatsCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stats_cases.json"))
	if err != nil {
		t.Fatalf("read stats_cases.json: %v", err)
	}
	var c casesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode stats_cases.json: %v", err)
	}
	if len(c.Frames) == 0 || len(c.MiniFrames) == 0 {
		t.Fatal("stats_cases.json has no frames")
	}
	if c.Crash.File == "" || len(c.BadFiles) == 0 {
		t.Fatal("stats_cases.json has no crash/bad files")
	}
	return c
}

// buildStats replays one frozen case list through the frozen
// constructors without re-spelling its content: the file owns the
// numbers.
func buildStats(t *testing.T, name string, frames []frameCase, shaders []shaderCase) Stats {
	t.Helper()
	var s Stats
	for _, f := range frames {
		spans := make([]Span, len(f.Spans))
		for i, sp := range f.Spans {
			one, err := NewSpan(sp.Name, core.Milliseconds(sp.TookMs))
			if err != nil {
				t.Fatalf("%s: NewSpan %q: %v", name, sp.Name, err)
			}
			spans[i] = one
		}
		fr, err := NewFrame(core.Milliseconds(f.DtMs), f.Draws, f.Overdraw, f.Mem, spans)
		if err != nil {
			t.Fatalf("%s: NewFrame %q: %v", name, f.Name, err)
		}
		if err := s.RecordFrame(fr); err != nil {
			t.Fatalf("%s: RecordFrame %q: %v", name, f.Name, err)
		}
	}
	for _, sh := range shaders {
		for _, took := range sh.TakesMs {
			if err := s.RecordShader(sh.Name, core.Milliseconds(took)); err != nil {
				t.Fatalf("%s: RecordShader %q: %v", name, sh.Name, err)
			}
		}
	}
	return s
}

func mustFindBreakdown(t *testing.T, bd []SpanStat, name string) SpanStat {
	t.Helper()
	for _, r := range bd {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("breakdown has no span %q", name)
	return SpanStat{}
}

func mustFindShader(t *testing.T, sh []ShaderStat, name string) ShaderStat {
	t.Helper()
	for _, r := range sh {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("shaders have no entry %q", name)
	return ShaderStat{}
}

func expectCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func codeFromName(name string) core.Code {
	switch name {
	case "not-found":
		return core.CodeNotFound
	case "bad-data":
		return core.CodeBadData
	case "out-of-memory":
		return core.CodeOutOfMemory
	case "unsupported":
		return core.CodeUnsupported
	case "invalid-arg":
		return core.CodeInvalidArg
	case "version-mismatch":
		return core.CodeVersionMismatch
	}
	return core.CodeUnknown
}

func closeFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	tol := 1e-9
	if want < 0 {
		want = -want
	}
	if want > 1 {
		tol = want * 1e-9
	}
	if diff > tol {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// A:帧率画次显存帧分解过绘编译耗时落在冻结数上.
func TestStatsFromCases(t *testing.T) {
	c := loadStatsCases(t)
	if CurrentVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", CurrentVersion, c.Version)
	}
	if MaxFrames != c.MaxFrames || int64(SlowFrameDt) != c.SlowDtMs*int64(core.Millisecond) ||
		int64(MaxFrameDt) != c.MaxDtMs*int64(core.Millisecond) {
		t.Fatal("budgets diverge from stats_cases.json")
	}
	s := buildStats(t, "cases", c.Frames, c.Shaders)
	want := c.Expect
	r := s.Report()
	if r.Version != c.Version {
		t.Errorf("report version = %v, want %v", r.Version, c.Version)
	}
	if r.Frames != want.Frames || int(r.TotalFrames) != int(want.TotalFrames) {
		t.Errorf("frames = %d/%d, want %d/%d", r.Frames, r.TotalFrames, want.Frames, want.TotalFrames)
	}
	closeFloat(t, "avg", r.AvgMs, want.AvgMs)
	closeFloat(t, "fps", r.FPS, 1000/want.AvgMs)
	closeFloat(t, "p95", r.P95Ms, want.P95Ms)
	if r.Slow != want.Slow {
		t.Errorf("slow = %d, want %d", r.Slow, want.Slow)
	}
	if r.Gaps != 0 || r.OOMs != 0 {
		t.Errorf("gaps/ooms = %d/%d, want 0/0", r.Gaps, r.OOMs)
	}
	if r.TotalDraws != want.TotalDraws {
		t.Errorf("draws = %d, want %d", r.TotalDraws, want.TotalDraws)
	}
	closeFloat(t, "avg draws", r.AvgDraws, want.AvgDraws)
	if r.MemCur != want.MemCur || r.MemPeak != want.MemPeak {
		t.Errorf("mem = %d/%d, want %d/%d", r.MemCur, r.MemPeak, want.MemCur, want.MemPeak)
	}
	closeFloat(t, "overdraw", r.Overdraw, want.Overdraw)
	closeFloat(t, "shader", r.ShaderMs, want.ShaderMs)
	// Breakdown lands in total-descending order with the frozen shares.
	if len(r.Breakdown) != len(want.Breakdown) {
		t.Fatalf("breakdown len = %d, want %d", len(r.Breakdown), len(want.Breakdown))
	}
	for i, w := range want.Breakdown {
		if r.Breakdown[i].Name != w.Name {
			t.Errorf("breakdown[%d] = %q, want %q", i, r.Breakdown[i].Name, w.Name)
		}
	}
	var grand int64
	for _, w := range want.Breakdown {
		grand += w.TotalMs
	}
	for _, w := range want.Breakdown {
		got := mustFindBreakdown(t, r.Breakdown, w.Name)
		if got.Total != w.TotalMs || got.Max != w.MaxMs {
			t.Errorf("%s total/max = %d/%d, want %d/%d", w.Name, got.Total, got.Max, w.TotalMs, w.MaxMs)
		}
		closeFloat(t, w.Name+" avg", got.Avg, w.AvgMs)
		closeFloat(t, w.Name+" share", got.Share, float64(w.TotalMs)/float64(grand))
	}
	lit := mustFindShader(t, r.Shaders, "lit")
	if lit.Compiles != 2 || lit.Total != 57 || lit.Max != 45 {
		t.Errorf("lit = %dx %d/%d, want 2x 57/45", lit.Compiles, lit.Total, lit.Max)
	}
	shadow := mustFindShader(t, r.Shaders, "shadow")
	if shadow.Compiles != 1 || shadow.Total != 120 || shadow.Max != 120 {
		t.Errorf("shadow = %dx %d/%d, want 1x 120/120", shadow.Compiles, shadow.Total, shadow.Max)
	}
	// Report round-trips through Encode/Parse to the same snapshot.
	raw, err := r.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	back, err := ParseReport(raw)
	if err != nil {
		t.Fatalf("ParseReport: %v", err)
	}
	if !back.Equal(r) {
		t.Error("report round-trip diverged")
	}
	// Crash snapshot carries the same report plus reason and code.
	crash := s.Snapshot("long-run-oom", core.CodeOutOfMemory)
	if crash.Reason != "long-run-oom" || crash.Code != "out-of-memory" {
		t.Errorf("crash = %q/%q, want long-run-oom/out-of-memory", crash.Reason, crash.Code)
	}
	if !crash.Report.Equal(r) {
		t.Error("crash report diverged from live report")
	}
	craw, err := crash.Encode()
	if err != nil {
		t.Fatalf("crash Encode: %v", err)
	}
	cback, err := ParseCrash(craw)
	if err != nil {
		t.Fatalf("ParseCrash: %v", err)
	}
	if !cback.Equal(crash) {
		t.Error("crash round-trip diverged")
	}
}

// B:空零超大坏数据坏文件全不崩不卡死,占位加报错.
func TestStatsEdgesNoCrash(t *testing.T) {
	c := loadStatsCases(t)
	var nilStats *Stats
	if nilStats.Report().Frames != 0 || nilStats.FPS() != 0 {
		t.Error("nil Stats must answer zeros")
	}
	if err := nilStats.RecordFrame(Frame{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil RecordFrame code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilStats.RecordShader("x", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil RecordShader code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilStats.NoteMemory(1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil NoteMemory code = %v, want invalid-arg", core.CodeOf(err))
	}
	var zero Stats
	if _, err := NewSpan("", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty span code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewSpan(strings.Repeat("x", MaxNameLen+1), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long span code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewSpan("ok", -1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative span code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewFrame(0, 0, 1, 0, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero dt code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewFrame(1, -1, 1, 0, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative draws code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewFrame(1, 0, -1, 0, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative overdraw code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewFrame(1, 0, 1, -1, nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative mem code = %v, want invalid-arg", core.CodeOf(err))
	}
	many := make([]Span, MaxSpansPerFrame+1)
	for i := range many {
		many[i] = Span{name: "s", took: 1}
	}
	if _, err := NewFrame(1, 0, 1, 0, many); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("span overflow code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := zero.RecordFrame(Frame{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero frame code = %v, want invalid-arg", core.CodeOf(err))
	}
	before := zero.Report()
	if err := zero.RecordFrame(Frame{}); err == nil {
		t.Error("zero RecordFrame want error")
	}
	if !zero.Report().Equal(before) {
		t.Error("failed RecordFrame mutated the ledger")
	}
	if err := zero.RecordShader("", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty shader code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := zero.RecordShader("ok", -1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative shader code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := zero.NoteMemory(-1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative mem code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Over-budget VRAM counts one OOM and keeps the old value.
	if err := zero.NoteMemory(1024); err != nil {
		t.Fatalf("NoteMemory: %v", err)
	}
	if err := zero.NoteMemory(MaxMemoryBytes + 1); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge mem code = %v, want out-of-memory", core.CodeOf(err))
	}
	if zero.OOMs() != 1 || zero.CurMemory() != 1024 {
		t.Errorf("oom = %d/%d, want 1/1024", zero.OOMs(), zero.CurMemory())
	}
	// Frozen bad files fail with their frozen codes, never panic.
	for _, b := range c.BadFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", b.File))
		if err != nil {
			t.Fatalf("read %s: %v", b.File, err)
		}
		_, err = ParseReport(raw)
		expectCode(t, b.File, err, codeFromName(b.WantCode))
		if _, err := LoadReport(filepath.Join("testdata", b.File)); core.CodeOf(err) != codeFromName(b.WantCode) {
			t.Errorf("%s Load code = %v, want %v", b.File, core.CodeOf(err), b.WantCode)
		}
	}
	craw, err := os.ReadFile(filepath.Join("testdata", "stats_bad_crash.json"))
	if err != nil {
		t.Fatalf("read stats_bad_crash.json: %v", err)
	}
	if _, err := ParseCrash(craw); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("bad crash code = %v, want bad-data", core.CodeOf(err))
	}
	if _, err := ParseReport(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil report code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := ParseCrash(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil crash code = %v, want invalid-arg", core.CodeOf(err))
	}
	big := make([]byte, MaxBytes+1)
	if _, err := ParseReport(big); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge report code = %v, want out-of-memory", core.CodeOf(err))
	}
	if _, err := ParseCrash(big); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge crash code = %v, want out-of-memory", core.CodeOf(err))
	}
	if _, err := LoadReport(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadReport(filepath.Join("testdata", "no_such.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing file code = %v, want not-found", core.CodeOf(err))
	}
	if err := StoreReport(zero.Report(), ""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty StoreReport code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := StoreCrash(zero.Snapshot("x", core.CodeUnknown), ""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty StoreCrash code = %v, want invalid-arg", core.CodeOf(err))
	}
}

// C不适用(纯算数不画画):数路无损加逐位重放即两边同数.
func TestStatsBoundaryIdentical(t *testing.T) {
	c := loadStatsCases(t)
	for _, f := range c.Frames {
		// Duration boundary is lossless through dt_ms.
		if back := core.Milliseconds(core.Milliseconds(f.DtMs).Milliseconds()); back != core.Milliseconds(f.DtMs) {
			t.Errorf("%s: dt boundary = %v", f.Name, back)
		}
		for _, sp := range f.Spans {
			if back := core.Milliseconds(core.Milliseconds(sp.TookMs).Milliseconds()); back != core.Milliseconds(sp.TookMs) {
				t.Errorf("%s/%s: span boundary = %v", f.Name, sp.Name, back)
			}
		}
		// Version boundary round-trips through its string form.
		if back, err := core.ParseVersion(CurrentVersion.String()); err != nil || back != CurrentVersion {
			t.Fatalf("version boundary = %v/%v", back, err)
		}
	}
	// Double build from the same frozen file agrees bit for bit.
	a := buildStats(t, "replay-a", c.Frames, c.Shaders)
	b := buildStats(t, "replay-b", c.Frames, c.Shaders)
	if !a.Report().Equal(b.Report()) {
		t.Error("double build replay diverged")
	}
	if back, err := ParseReport(mustEncode(t, "replay", a.Report())); err != nil || !back.Equal(a.Report()) {
		t.Error("second round-trip diverged")
	}
	// Copies never alias: mutating a return cannot corrupt the replay.
	probe := buildStats(t, "alias", c.Frames[:1], nil)
	spans := probe.at(0).Spans()
	if len(spans) == 0 {
		t.Fatal("alias probe needs at least one span")
	}
	before := probe.Report()
	spans[0] = Span{name: "hacked", took: 9999}
	if !probe.Report().Equal(before) {
		t.Error("Spans aliases the ledger")
	}
	// Same file parses to the same snapshot twice (no hidden state).
	raw, err := os.ReadFile(filepath.Join("testdata", c.Crash.File))
	if err != nil {
		t.Fatalf("read %s: %v", c.Crash.File, err)
	}
	x, err := ParseCrash(raw)
	if err != nil {
		t.Fatalf("ParseCrash: %v", err)
	}
	y, err := ParseCrash(raw)
	if err != nil {
		t.Fatalf("ParseCrash again: %v", err)
	}
	if !x.Equal(y) {
		t.Error("same crash file parsed twice diverged")
	}
}

func mustEncode(t *testing.T, name string, r Report) []byte {
	t.Helper()
	raw, err := r.Encode()
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}
	return raw
}

// D:上报跑得动,帧存读上报时长加字节有数.
func TestStatsReportPerf(t *testing.T) {
	c := loadStatsCases(t)
	s := buildStats(t, "perf", c.Frames, c.Shaders)
	const reps = 2000
	start := time.Now()
	for i := 0; i < reps; i++ {
		back, err := ParseReport(mustEncode(t, "perf", s.Report()))
		if err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if !back.Equal(s.Report()) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	el := time.Since(start)
	nbytes := len(mustEncode(t, "perf", s.Report()))
	t.Logf("stats-report: %d reps encode+parse %dB in %v (%.1f us/rep)", reps, nbytes, el, float64(el.Microseconds())/reps)
	if nbytes > MaxBytes {
		t.Fatalf("report %dB exceeds MaxBytes %d", nbytes, MaxBytes)
	}
	// Shader record joins the cost: one compile per rep.
	start = time.Now()
	for i := 0; i < reps; i++ {
		if err := s.RecordShader("perf", core.Milliseconds(3)); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	el = time.Since(start)
	t.Logf("stats-shader: %d reps record in %v (%.1f ns/op)", reps, el, float64(el.Nanoseconds())/reps)
	// Store/Load joins the cost once through TempDir files.
	dir := t.TempDir()
	rpath := filepath.Join(dir, "report.json")
	cpath := filepath.Join(dir, "crash.json")
	start = time.Now()
	if err := StoreReport(s.Report(), rpath); err != nil {
		t.Fatalf("StoreReport: %v", err)
	}
	again, err := LoadReport(rpath)
	if err != nil {
		t.Fatalf("LoadReport: %v", err)
	}
	if !again.Equal(s.Report()) {
		t.Fatal("file round-trip diverged")
	}
	if err := StoreCrash(s.Snapshot("perf", core.CodeUnknown), cpath); err != nil {
		t.Fatalf("StoreCrash: %v", err)
	}
	if _, err := LoadCrash(cpath); err != nil {
		t.Fatalf("LoadCrash: %v", err)
	}
	el = time.Since(start)
	t.Logf("stats-file: store+load report+crash in %v", el)
}

// E:长跑在线,显存不涨坏数据不崩,切后台缩窗口回来不黑.
func TestStatsLongRunStable(t *testing.T) {
	c := loadStatsCases(t)
	one := c.Frames[0]
	spans := make([]Span, len(one.Spans))
	for i, sp := range one.Spans {
		s, err := NewSpan(sp.Name, core.Milliseconds(sp.TookMs))
		if err != nil {
			t.Fatalf("NewSpan: %v", err)
		}
		spans[i] = s
	}
	fr, err := NewFrame(core.Milliseconds(one.DtMs), one.Draws, one.Overdraw, one.Mem, spans)
	if err != nil {
		t.Fatalf("NewFrame: %v", err)
	}
	var s Stats
	const reps = 5000
	for i := 0; i < reps; i++ {
		if err := s.RecordFrame(fr); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	if s.Count() != MaxFrames || int(s.TotalFrames()) != reps {
		t.Fatalf("retained/total = %d/%d, want %d/%d", s.Count(), s.TotalFrames(), MaxFrames, reps)
	}
	// VRAM never grows on a flat feed: cur and peak pin to the sample.
	if s.CurMemory() != one.Mem || s.PeakMemory() != one.Mem {
		t.Errorf("mem = %d/%d, want %d/%d", s.CurMemory(), s.PeakMemory(), one.Mem, one.Mem)
	}
	for i := 0; i < 200; i++ {
		if err := s.RecordFrame(fr); err != nil {
			t.Fatalf("steady %d: %v", i, err)
		}
	}
	// Total moves on (ring evicts), retained counters stay flat.
	if int(s.TotalFrames()) != reps+200 || s.Count() != MaxFrames {
		t.Fatalf("steady retained/total = %d/%d", s.Count(), s.TotalFrames())
	}
	if s.CurMemory() != one.Mem || s.PeakMemory() != one.Mem {
		t.Error("steady feed grew memory")
	}
	// Background gap: a 10s stall clamps to MaxFrameDt and counts one
	// gap instead of collapsing FPS; the ledger keeps serving.
	gapDt := core.Milliseconds(10000)
	gapFrame, err := NewFrame(gapDt, 10, 1, one.Mem, nil)
	if err != nil {
		t.Fatalf("gap NewFrame: %v", err)
	}
	fpsBefore := s.FPS()
	if err := s.RecordFrame(gapFrame); err != nil {
		t.Fatalf("gap RecordFrame: %v", err)
	}
	if s.Gaps() != 1 {
		t.Fatalf("gaps = %d, want 1", s.Gaps())
	}
	kept := s.at(s.Count() - 1)
	if kept.Dt() != MaxFrameDt {
		t.Errorf("gap dt = %v, want %v", kept.Dt(), MaxFrameDt)
	}
	if s.FPS() <= 0 || s.FPS() >= fpsBefore*2 {
		t.Errorf("fps after gap = %v, before %v (gap must not collapse it)", s.FPS(), fpsBefore)
	}
	// Shrunk window (zero-size resume frame is rejected, ledger intact).
	steady := s.Report()
	if err := s.RecordFrame(Frame{}); err == nil {
		t.Error("zero frame want error")
	}
	if !s.Report().Equal(steady) {
		t.Error("bad frame poisoned the ledger")
	}
	// Bad data never poisons the next good parse; double long-run
	// builders still agree bit for bit.
	for _, b := range c.BadFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", b.File))
		if _, err := ParseReport(raw); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	var u1, u2 Stats
	for i := 0; i < reps; i++ {
		if err := u1.RecordFrame(fr); err != nil {
			t.Fatal(err)
		}
		if err := u2.RecordFrame(fr); err != nil {
			t.Fatal(err)
		}
	}
	if !u1.Report().Equal(u2.Report()) {
		t.Error("long-run double build diverged")
	}
	// File round-trips stable after the storm.
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.json")
	if err := StoreReport(u1.Report(), path); err != nil {
		t.Fatalf("StoreReport: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	size := fi.Size()
	for i := 0; i < 50; i++ {
		if err := StoreReport(u1.Report(), path); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
		back, err := LoadReport(path)
		if err != nil {
			t.Fatalf("load %d: %v", i, err)
		}
		if !back.Equal(u1.Report()) {
			t.Fatalf("file rep %d diverged", i)
		}
		got, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %d: %v", i, err)
		}
		if got.Size() != size {
			t.Fatalf("store %d size %d, want %d", i, got.Size(), size)
		}
	}
}

// F:离屏金对照(窗免,纯算数):冻结数加形状断言,报表文本即证据.
func TestStatsOffscreenGolden(t *testing.T) {
	c := loadStatsCases(t)
	s := buildStats(t, "golden", c.Frames, c.Shaders)
	r := s.Report()
	want := c.Expect
	// Golden pins the anchors through the file, not the code.
	closeFloat(t, "golden avg", r.AvgMs, want.AvgMs)
	closeFloat(t, "golden fps", r.FPS, 1000/want.AvgMs)
	closeFloat(t, "golden p95", r.P95Ms, want.P95Ms)
	if r.Slow != want.Slow || r.Frames != want.Frames {
		t.Errorf("golden slow/frames = %d/%d, want %d/%d", r.Slow, r.Frames, want.Slow, want.Frames)
	}
	// Shape: breakdown totals tile the frame sum; shares add to one;
	// the top span names who eats the frame (render first here).
	var tile int64
	var share float64
	for _, row := range r.Breakdown {
		tile += row.Total
		share += row.Share
		if row.Max <= 0 || row.Avg <= 0 {
			t.Errorf("%s max/avg = %d/%v, want positive", row.Name, row.Max, row.Avg)
		}
	}
	var dtSum int64
	for _, f := range c.Frames {
		dtSum += f.DtMs
	}
	if tile != dtSum {
		t.Errorf("breakdown tiles %d, frame sum %d", tile, dtSum)
	}
	closeFloat(t, "share sum", share, 1)
	if len(r.Breakdown) == 0 || r.Breakdown[0].Name != "render" {
		t.Error("top span is not render: the report must point at who eats time")
	}
	// Shape: every frozen frame keeps Dt positive and spans capped.
	for _, f := range c.Frames {
		if f.DtMs <= 0 || f.Draws < 0 || f.Mem < 0 {
			t.Errorf("%s carries bad numbers", f.Name)
		}
		if len(f.Spans) > MaxSpansPerFrame {
			t.Errorf("%s spans %d over cap", f.Name, len(f.Spans))
		}
	}
	// Frozen report file pins the 10-frame snapshot byte for byte:
	// load it, land on the same numbers, re-encode to identical bytes.
	repRaw, err := os.ReadFile(filepath.Join("testdata", "stats_report.json"))
	if err != nil {
		t.Fatalf("read stats_report.json: %v", err)
	}
	pinned, err := ParseReport(bytesTrimNewline(repRaw))
	if err != nil {
		t.Fatalf("ParseReport golden: %v", err)
	}
	if !pinned.Equal(r) {
		t.Error("stats_report.json diverged from the frozen cases")
	}
	back, err := ParseReport(mustEncode(t, "golden", r))
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if !back.Equal(pinned) {
		t.Error("golden report re-encode diverged")
	}
	// Mini report text is pinned byte for byte: this is the readable
	// report the F scenario reviews.
	mini := buildStats(t, "mini", c.MiniFrames, c.MiniShaders)
	golden, err := os.ReadFile(filepath.Join("testdata", "stats_report_golden.txt"))
	if err != nil {
		t.Fatalf("read stats_report_golden.txt: %v", err)
	}
	if got := mini.Report().Text(); got != string(golden) {
		t.Errorf("report text diverged:\n got:\n%s\nwant:\n%s", got, golden)
	}
	// Frozen crash file parses with its frozen reason and code.
	craw, err := os.ReadFile(filepath.Join("testdata", c.Crash.File))
	if err != nil {
		t.Fatalf("read %s: %v", c.Crash.File, err)
	}
	crash, err := ParseCrash(craw)
	if err != nil {
		t.Fatalf("ParseCrash: %v", err)
	}
	if crash.Version != c.Version || crash.Reason != c.Crash.Reason || crash.Code != c.Crash.Code {
		t.Errorf("crash = %v/%q/%q, want %v/%q/%q",
			crash.Version, crash.Reason, crash.Code, c.Version, c.Crash.Reason, c.Crash.Code)
	}
	if crash.Report.Frames != 2 || crash.Report.OOMs != 1 {
		t.Errorf("crash report frames/ooms = %d/%d, want 2/1", crash.Report.Frames, crash.Report.OOMs)
	}
	if back, err := ParseCrash(mustEncodeCrash(t, crash)); err != nil || !back.Equal(crash) {
		t.Errorf("crash golden round-trip diverged: %v", err)
	}
}

func mustEncodeCrash(t *testing.T, c CrashLog) []byte {
	t.Helper()
	raw, err := c.Encode()
	if err != nil {
		t.Fatalf("crash Encode: %v", err)
	}
	return raw
}

// bytesTrimNewline drops one trailing newline the editor keeps at the
// end of the golden file; the ledger bytes themselves stay pinned.
func bytesTrimNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}
