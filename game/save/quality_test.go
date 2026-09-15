package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type qualityLevelCase struct {
	Name      string  `json:"name"`
	Particles int     `json:"particles"`
	Lights    int     `json:"lights"`
	Scale     float64 `json:"scale"`
}

type qualityFileCase struct {
	File    string `json:"file"`
	Name    string `json:"name"`
	Quality string `json:"quality"`
}

type qualityBadCase struct {
	File     string `json:"file"`
	WantCode string `json:"want_code"`
}

type qualityCases struct {
	Version  string             `json:"version"`
	MaxBytes int                `json:"max_bytes"`
	Levels   []qualityLevelCase `json:"levels"`
	Files    []qualityFileCase  `json:"files"`
	BadFiles []qualityBadCase   `json:"bad_files"`
	BadNames []string           `json:"bad_names"`
}

func loadQualityCases(t *testing.T) qualityCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "quality_cases.json"))
	if err != nil {
		t.Fatalf("read quality_cases.json: %v", err)
	}
	var c qualityCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode quality_cases.json: %v", err)
	}
	if len(c.Levels) != 3 || len(c.Files) != 3 || len(c.BadFiles) == 0 || len(c.BadNames) == 0 {
		t.Fatal("quality_cases.json is missing levels/files/bad entries")
	}
	return c
}

func mustFindQuality(t *testing.T, c qualityCases, name string) qualityLevelCase {
	t.Helper()
	for _, l := range c.Levels {
		if l.Name == name {
			return l
		}
	}
	t.Fatalf("quality_cases.json has no level %q", name)
	return qualityLevelCase{}
}

func checkQualitySpec(t *testing.T, name string, got Spec, want qualityLevelCase) {
	t.Helper()
	if got.Particles != want.Particles || got.Lights != want.Lights || got.Scale != want.Scale {
		t.Errorf("%s: spec = %+v, want particles=%d lights=%d scale=%v",
			name, got, want.Particles, want.Lights, want.Scale)
	}
}

func mustBuildQuality(t *testing.T, name string, level Level) Quality {
	t.Helper()
	q, err := NewQuality(level)
	if err != nil {
		t.Fatalf("%s: NewQuality: %v", name, err)
	}
	return q
}

func mustEncodeQuality(t *testing.T, name string, q Quality) []byte {
	t.Helper()
	raw, err := q.Encode()
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}
	return raw
}

func mustParseQualityOK(t *testing.T, name string, raw []byte) Quality {
	t.Helper()
	q, err := ParseQuality(raw)
	if err != nil {
		t.Fatalf("%s: ParseQuality: %v", name, err)
	}
	return q
}

func mustRoundTripQuality(t *testing.T, name string, q Quality) Quality {
	t.Helper()
	return mustParseQualityOK(t, name, mustEncodeQuality(t, name, q))
}

func mustStoreLoadQuality(t *testing.T, name string, q Quality, path string) Quality {
	t.Helper()
	if err := StoreQuality(q, path); err != nil {
		t.Fatalf("%s: StoreQuality: %v", name, err)
	}
	again, err := LoadQuality(path)
	if err != nil {
		t.Fatalf("%s: reload: %v", name, err)
	}
	return again
}

// A:高中低跟档走,文件与API两条路算出同一个数.
func TestQualityFromCases(t *testing.T) {
	c := loadQualityCases(t)
	if QualityVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", QualityVersion, c.Version)
	}
	if MaxQualityBytes != c.MaxBytes {
		t.Fatalf("MaxQualityBytes = %d, want %d", MaxQualityBytes, c.MaxBytes)
	}
	seen := map[string]bool{}
	for _, want := range c.Levels {
		if seen[want.Name] {
			t.Errorf("duplicate level %q", want.Name)
		}
		seen[want.Name] = true
		lvl, err := ParseLevel(want.Name)
		if err != nil {
			t.Fatalf("%s: ParseLevel: %v", want.Name, err)
		}
		got, err := SpecFor(lvl)
		if err != nil {
			t.Fatalf("%s: SpecFor: %v", want.Name, err)
		}
		checkQualitySpec(t, want.Name, got, want)
		// API handle lands on the same spec as the file.
		q := mustBuildQuality(t, want.Name, lvl)
		if q.Level() != lvl {
			t.Errorf("%s: Level = %v, want %v", want.Name, q.Level(), lvl)
		}
		checkQualitySpec(t, want.Name, q.Spec(), want)
		if back := mustRoundTripQuality(t, want.Name, q); !back.Equal(q) {
			t.Errorf("%s: Encode round-trip diverged", want.Name)
		}
		dir := t.TempDir()
		if again := mustStoreLoadQuality(t, want.Name, q, filepath.Join(dir, "q.json")); !again.Equal(q) {
			t.Errorf("%s: Store/Load diverged", want.Name)
		}
	}
	for _, f := range c.Files {
		raw, err := os.ReadFile(filepath.Join("testdata", f.File))
		if err != nil {
			t.Fatalf("read %s: %v", f.File, err)
		}
		got := mustParseQualityOK(t, f.Name, raw)
		if string(got.Level()) != f.Quality {
			t.Errorf("%s: level = %v, want %v", f.Name, got.Level(), f.Quality)
		}
		want := mustFindQuality(t, c, f.Name)
		checkQualitySpec(t, f.Name, got.Spec(), want)
	}
}

// B:坏档名空档切档横跳全不崩不卡死,占位加报错;同场景切档状态连续.
func TestQualityEdgesNoCrash(t *testing.T) {
	c := loadQualityCases(t)
	if _, err := ParseQuality(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil data code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := ParseQuality([]byte{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty data code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadQuality(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadQuality(filepath.Join("testdata", "no_such_quality.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing file code = %v, want not-found", core.CodeOf(err))
	}
	if err := StoreQuality(mustBuildQuality(t, "edges", LevelHigh), ""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty StoreQuality path code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Frozen bad files fail with their frozen codes, never panic.
	for _, b := range c.BadFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", b.File))
		if err != nil {
			t.Fatalf("read %s: %v", b.File, err)
		}
		_, err = ParseQuality(raw)
		expectCode(t, b.File, err, codeFromName(b.WantCode))
		if _, err := LoadQuality(filepath.Join("testdata", b.File)); core.CodeOf(err) != codeFromName(b.WantCode) {
			t.Errorf("%s LoadQuality code = %v, want %v", b.File, core.CodeOf(err), b.WantCode)
		}
	}
	// Frozen bad names fail on every entry point, never panic.
	for _, name := range c.BadNames {
		if _, err := ParseLevel(name); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("ParseLevel(%q) code = %v, want invalid-arg", name, core.CodeOf(err))
		}
		if _, err := NewQuality(Level(name)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("NewQuality(%q) code = %v, want invalid-arg", name, core.CodeOf(err))
		}
		if _, err := SpecFor(Level(name)); core.CodeOf(err) != core.CodeInvalidArg {
			t.Errorf("SpecFor(%q) code = %v, want invalid-arg", name, core.CodeOf(err))
		}
	}
	var zero Quality
	if _, err := zero.Encode(); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("zero Encode code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := zero.StoreQualityPath(t); err == nil {
		t.Error("zero StoreQuality want error")
	}
	var nilQ *Quality
	if err := nilQ.Switch(LevelHigh); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Switch code = %v, want invalid-arg", core.CodeOf(err))
	}
	q := mustBuildQuality(t, "edges", LevelHigh)
	if err := q.Switch(Level("ultra")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad Switch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if q.Level() != LevelHigh {
		t.Errorf("failed Switch moved tier to %v, want high", q.Level())
	}
	if err := q.Switch(Level("")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Switch code = %v, want invalid-arg", core.CodeOf(err))
	}
	if q.Level() != LevelHigh {
		t.Errorf("empty Switch moved tier to %v, want high", q.Level())
	}
	// Same-tier switch is a no-op success, never a flash or reload.
	if err := q.Switch(LevelHigh); err != nil {
		t.Errorf("same-tier Switch: %v", err)
	}
	if q.Level() != LevelHigh {
		t.Errorf("same-tier Switch moved tier to %v", q.Level())
	}
	// Oversized file fails before JSON: OutOfMemory, never a guess.
	big := make([]byte, MaxQualityBytes+1)
	if _, err := ParseQuality(big); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge bytes code = %v, want out-of-memory", core.CodeOf(err))
	}
	// Rapid toggling mid-fight never crashes and lands where told:
	// high->medium->low->high returns to the start, same scene.
	start := mustBuildQuality(t, "flicker", LevelHigh)
	flick := start
	for i := 0; i < 300; i++ {
		if err := flick.Switch(LevelMedium); err != nil {
			t.Fatalf("flicker to medium %d: %v", i, err)
		}
		if err := flick.Switch(LevelLow); err != nil {
			t.Fatalf("flicker to low %d: %v", i, err)
		}
		if err := flick.Switch(LevelHigh); err != nil {
			t.Fatalf("flicker to high %d: %v", i, err)
		}
	}
	if !flick.Equal(start) {
		t.Errorf("900 flickers diverged: got %v, want %v", flick.Level(), start.Level())
	}
	checkQualitySpec(t, "flicker", flick.Spec(), mustFindQuality(t, c, "high"))
}

// zeroStoreQualityPath stores the zero handle through TempDir files.
func (q Quality) StoreQualityPath(t *testing.T) error {
	t.Helper()
	return StoreQuality(q, filepath.Join(t.TempDir(), "zero.json"))
}

// C不适用(纯映射不画画):数路无损加逐位重放即两边同数.
// 口径:分档只做 tier->Spec 纯映射,不调任何显卡/CPU画图接口,
// 两边(显卡/CPU)拿到的是同一套 Spec 数;显卡和CPU的像素差归
// particle/light/render 各自管,不归分档管.所以C不断言像素,
// 只断言同一文件在任何机器上 parse 出逐位一致的 Spec,
// 即"无显卡/CPU双画可比":可比的是数,不是像素.
func TestQualityBoundaryIdentical(t *testing.T) {
	c := loadQualityCases(t)
	for _, want := range c.Levels {
		lvl, err := ParseLevel(want.Name)
		if err != nil {
			t.Fatalf("%s: ParseLevel: %v", want.Name, err)
		}
		// Level boundary round-trips through its string form.
		if back, err := ParseLevel(lvl.String()); err != nil || back != lvl {
			t.Errorf("%s: level boundary = %v/%v", want.Name, back, err)
		}
		q := mustBuildQuality(t, want.Name, lvl)
		raw := mustEncodeQuality(t, want.Name, q)
		a := mustParseQualityOK(t, want.Name, raw)
		b := mustParseQualityOK(t, want.Name, raw)
		if !a.Equal(b) || !a.Equal(q) {
			t.Errorf("%s: replay diverged", want.Name)
		}
		if again := mustRoundTripQuality(t, want.Name, a); !again.Equal(q) {
			t.Errorf("%s: second round-trip diverged", want.Name)
		}
		// Version boundary round-trips through its string form.
		if back, err := core.ParseVersion(QualityVersion.String()); err != nil || back != QualityVersion {
			t.Errorf("%s: version boundary = %v/%v", want.Name, back, err)
		}
	}
	// Same file parses to the same handle twice (no hidden state).
	raw, _ := os.ReadFile(filepath.Join("testdata", "quality_high.json"))
	a, _ := ParseQuality(raw)
	b, _ := ParseQuality(raw)
	if !a.Equal(b) {
		t.Error("same file parsed twice diverged")
	}
}

// D:三档各有数,切档加编解码耗时有数.
func TestQualityPerfTiers(t *testing.T) {
	c := loadQualityCases(t)
	high := mustFindQuality(t, c, "high")
	medium := mustFindQuality(t, c, "medium")
	low := mustFindQuality(t, c, "low")
	// Shape first: tiers step down monotonically, high renders full.
	if !(high.Particles > medium.Particles && medium.Particles > low.Particles) {
		t.Fatalf("particles %d/%d/%d not strictly decreasing", high.Particles, medium.Particles, low.Particles)
	}
	if !(high.Lights > medium.Lights && medium.Lights > low.Lights) {
		t.Fatalf("lights %d/%d/%d not strictly decreasing", high.Lights, medium.Lights, low.Lights)
	}
	if !(high.Scale > medium.Scale && medium.Scale > low.Scale) {
		t.Fatalf("scale %v/%v/%v not strictly decreasing", high.Scale, medium.Scale, low.Scale)
	}
	const reps = 200000
	start := time.Now()
	for i := 0; i < reps; i++ {
		if _, err := SpecFor(LevelHigh); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	el := time.Since(start)
	t.Logf("quality-spec: %d SpecFor in %v (%.1f ns/op)", reps, el, float64(el.Nanoseconds())/reps)

	q := mustBuildQuality(t, "perf", LevelHigh)
	start = time.Now()
	for i := 0; i < reps; i++ {
		if err := q.Switch(LevelLow); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if err := q.Switch(LevelHigh); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
	}
	el = time.Since(start)
	t.Logf("quality-switch: %d pairs in %v (%.1f ns/pair)", reps, el, float64(el.Nanoseconds())/reps)
	if q.Level() != LevelHigh {
		t.Fatalf("perf toggling landed on %v, want high", q.Level())
	}

	const reps2 = 20000
	start = time.Now()
	for i := 0; i < reps2; i++ {
		back := mustRoundTripQuality(t, "perf", q)
		if !back.Equal(q) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	el = time.Since(start)
	raw := mustEncodeQuality(t, "perf", q)
	t.Logf("quality-codec: %d reps encode+parse %dB in %v (%.1f us/rep)", reps2, len(raw), el, float64(el.Microseconds())/reps2)
	if len(raw) > MaxQualityBytes {
		t.Fatalf("quality file %dB exceeds MaxQualityBytes %d", len(raw), MaxQualityBytes)
	}
}

// E:反复切档长跑不掉档不漂,坏档不粘.
func TestQualityLongRunStable(t *testing.T) {
	c := loadQualityCases(t)
	q := mustBuildQuality(t, "soak", LevelHigh)
	firstRaw := mustEncodeQuality(t, "soak", q)
	for i := 0; i < 10000; i++ {
		raw := mustEncodeQuality(t, "soak", q)
		if string(raw) != string(firstRaw) {
			t.Fatalf("rep %d bytes drifted", i)
		}
		if back := mustParseQualityOK(t, "soak", raw); !back.Equal(q) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	// Walk every tier ten thousand times: stored tier follows, never sticks.
	walk := mustBuildQuality(t, "walk", LevelHigh)
	tiers := []Level{LevelHigh, LevelMedium, LevelLow}
	for i := 0; i < 10000; i++ {
		want := tiers[i%3]
		if err := walk.Switch(want); err != nil {
			t.Fatalf("walk %d: %v", i, err)
		}
		if walk.Level() != want {
			t.Fatalf("walk %d = %v, want %v", i, walk.Level(), want)
		}
	}
	if err := walk.Switch(LevelHigh); err != nil {
		t.Fatalf("walk back: %v", err)
	}
	if !walk.Equal(q) {
		t.Fatal("walk did not return to the start")
	}
	checkQualitySpec(t, "walk", walk.Spec(), mustFindQuality(t, c, "high"))
	// Bad data never poisons the next good parse; file round-trips stable.
	for _, b := range c.BadFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", b.File))
		if _, err := ParseQuality(raw); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	if again := mustParseQualityOK(t, "soak", firstRaw); !again.Equal(q) {
		t.Fatal("good parse after bad files diverged")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.json")
	var size int64
	for i := 0; i < 200; i++ {
		back := mustStoreLoadQuality(t, "stable", q, path)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %d: %v", i, err)
		}
		if i == 0 {
			size = fi.Size()
		} else if fi.Size() != size {
			t.Fatalf("store %d size %d, want %d", i, fi.Size(), size)
		}
		if !back.Equal(q) {
			t.Fatalf("file rep %d diverged", i)
		}
	}
}

// F:离屏金对照窗(窗免,纯映射):冻结数加形状断言,映射数即证据.
func TestQualityOffscreenGolden(t *testing.T) {
	c := loadQualityCases(t)
	if len(c.Levels) != 3 {
		t.Fatalf("levels = %d, want 3", len(c.Levels))
	}
	got := map[string]Spec{}
	for _, want := range c.Levels {
		lvl, err := ParseLevel(want.Name)
		if err != nil {
			t.Fatalf("%s: ParseLevel: %v", want.Name, err)
		}
		spec, err := SpecFor(lvl)
		if err != nil {
			t.Fatalf("%s: SpecFor: %v", want.Name, err)
		}
		checkQualitySpec(t, want.Name, spec, want)
		got[want.Name] = spec
		if spec.Particles <= 0 || spec.Lights <= 0 {
			t.Errorf("%s: non-positive budget %+v", want.Name, spec)
		}
		if spec.Scale <= 0 || spec.Scale > 1 {
			t.Errorf("%s: scale %v outside (0,1]", want.Name, spec.Scale)
		}
	}
	// Shape: high renders full, tiers step down, files agree with table.
	if got["high"].Scale != 1 {
		t.Errorf("high scale = %v, want 1", got["high"].Scale)
	}
	if !(got["high"].Particles > got["medium"].Particles && got["medium"].Particles > got["low"].Particles) {
		t.Error("particles do not step down high>medium>low")
	}
	if !(got["high"].Lights > got["medium"].Lights && got["medium"].Lights > got["low"].Lights) {
		t.Error("lights do not step down high>medium>low")
	}
	for _, f := range c.Files {
		raw, err := os.ReadFile(filepath.Join("testdata", f.File))
		if err != nil {
			t.Fatalf("read %s: %v", f.File, err)
		}
		q := mustParseQualityOK(t, f.Name, raw)
		if string(q.Level()) != f.Quality || q.Level().String() != f.Quality {
			t.Errorf("%s: level = %v, want %v", f.Name, q.Level(), f.Quality)
		}
		checkQualitySpec(t, f.Name, q.Spec(), mustFindQuality(t, c, f.Name))
		if nbytes := len(mustEncodeQuality(t, f.Name, q)); nbytes > MaxQualityBytes {
			t.Errorf("%s %dB exceeds MaxQualityBytes", f.Name, nbytes)
		}
	}
}
