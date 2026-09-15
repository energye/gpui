package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

type progressCase struct {
	Level      string `json:"level"`
	Checkpoint string `json:"checkpoint"`
	PlayMs     int64  `json:"play_ms"`
}

type itemCase struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type saveCase struct {
	File      string         `json:"file"`
	Name      string         `json:"name"`
	Slot      int            `json:"slot"`
	Version   string         `json:"version"`
	Progress  progressCase   `json:"progress"`
	Inventory []itemCase     `json:"inventory"`
	Stars     map[string]int `json:"stars"`
}

type legacyCase struct {
	File        string         `json:"file"`
	Slot        int            `json:"slot"`
	Version     string         `json:"version"`
	WantVersion string         `json:"want_version"`
	Progress    progressCase   `json:"progress"`
	Inventory   []itemCase     `json:"inventory"`
	Stars       map[string]int `json:"stars"`
}

type badFileCase struct {
	File     string `json:"file"`
	WantCode string `json:"want_code"`
}

type casesFile struct {
	Version          string        `json:"version"`
	MaxSlots         int           `json:"max_slots"`
	MaxInventory     int           `json:"max_inventory"`
	MaxItemCount     int           `json:"max_item_count"`
	MaxStars         int           `json:"max_stars"`
	MaxStarsPerLevel int           `json:"max_stars_per_level"`
	MaxNameLen       int           `json:"max_name_len"`
	MaxBytes         int           `json:"max_bytes"`
	Saves            []saveCase    `json:"saves"`
	Legacy           legacyCase    `json:"legacy"`
	BadFiles         []badFileCase `json:"bad_files"`
}

func loadSaveCases(t *testing.T) casesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "save_cases.json"))
	if err != nil {
		t.Fatalf("read save_cases.json: %v", err)
	}
	var c casesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode save_cases.json: %v", err)
	}
	if len(c.Saves) == 0 {
		t.Fatal("save_cases.json has no saves")
	}
	if c.Legacy.File == "" || len(c.BadFiles) == 0 {
		t.Fatal("save_cases.json has no legacy/bad files")
	}
	return c
}

func mustFindSave(t *testing.T, c casesFile, name string) saveCase {
	t.Helper()
	for _, s := range c.Saves {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("save_cases.json has no save %q", name)
	return saveCase{}
}

func checkSave(t *testing.T, want saveCase, got Save) {
	t.Helper()
	if got.Slot() != want.Slot {
		t.Errorf("%s: slot = %d, want %d", want.Name, got.Slot(), want.Slot)
	}
	if got.Version().String() != want.Version {
		t.Errorf("%s: version = %v, want %v", want.Name, got.Version(), want.Version)
	}
	p := got.Progress()
	if p.Level() != want.Progress.Level || p.Checkpoint() != want.Progress.Checkpoint ||
		p.PlayMs() != want.Progress.PlayMs {
		t.Errorf("%s: progress = %q/%q/%d, want %q/%q/%d", want.Name,
			p.Level(), p.Checkpoint(), p.PlayMs(),
			want.Progress.Level, want.Progress.Checkpoint, want.Progress.PlayMs)
	}
	inv := got.Inventory()
	if len(inv) != len(want.Inventory) {
		t.Errorf("%s: inventory len = %d, want %d", want.Name, len(inv), len(want.Inventory))
	} else {
		for i, w := range want.Inventory {
			if inv[i].ID() != w.ID || inv[i].Count() != w.Count {
				t.Errorf("%s: item[%d] = %v/%d, want %v/%d", want.Name, i,
					inv[i].ID(), inv[i].Count(), w.ID, w.Count)
			}
		}
	}
	st := got.Stars()
	if len(st) != len(want.Stars) {
		t.Errorf("%s: stars len = %d, want %d", want.Name, len(st), len(want.Stars))
	} else {
		for k, w := range want.Stars {
			if g, ok := st[k]; !ok || g != w {
				t.Errorf("%s: stars[%q] = %v, want %v", want.Name, k, g, w)
			}
		}
	}
}

// rebuildViaAPI replays one frozen case through New/SetProgress/AddItem/
// SetStars without re-spelling its content: the file owns the numbers.
func rebuildViaAPI(t *testing.T, want saveCase) Save {
	t.Helper()
	s := mustNewSlot(t, want.Name, want.Slot)
	prog := mustNewProgress(t, want.Name, want.Progress.Level, want.Progress.Checkpoint, core.Milliseconds(want.Progress.PlayMs))
	mustSetProgress(t, want.Name, &s, prog)
	for _, it := range want.Inventory {
		mustAddItem(t, want.Name, it.ID, &s, it.Count)
	}
	for k, v := range want.Stars {
		mustSetStars(t, want.Name, k, &s, v)
	}
	return s
}

func mustLoadSave(t *testing.T, name, file string) Save {
	t.Helper()
	got, err := Load(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("%s: Load: %v", name, err)
	}
	return got
}

func mustEncode(t *testing.T, name string, s Save) []byte {
	t.Helper()
	raw, err := s.Encode()
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}
	return raw
}

func mustParse(t *testing.T, name string, raw []byte) Save {
	t.Helper()
	back, err := Parse(raw)
	if err != nil {
		t.Fatalf("%s: Parse: %v", name, err)
	}
	return back
}

func mustRoundTrip(t *testing.T, name string, s Save) Save {
	t.Helper()
	return mustParse(t, name, mustEncode(t, name, s))
}

func mustNewSlot(t *testing.T, name string, slot int) Save {
	t.Helper()
	s, err := New(slot)
	if err != nil {
		t.Fatalf("%s: New: %v", name, err)
	}
	return s
}

func mustSetProgress(t *testing.T, name string, s *Save, p Progress) {
	t.Helper()
	if err := s.SetProgress(p); err != nil {
		t.Fatalf("%s: SetProgress: %v", name, err)
	}
}

func mustAddItem(t *testing.T, name, id string, s *Save, count int) {
	t.Helper()
	if err := s.AddItem(id, count); err != nil {
		t.Fatalf("%s: AddItem %q: %v", name, id, err)
	}
}

func mustSetStars(t *testing.T, name, level string, s *Save, stars int) {
	t.Helper()
	if err := s.SetStars(level, stars); err != nil {
		t.Fatalf("%s: SetStars %q: %v", name, level, err)
	}
}

func mustNewProgress(t *testing.T, name, level, checkpoint string, play core.Duration) Progress {
	t.Helper()
	prog, err := NewProgress(level, checkpoint, play)
	if err != nil {
		t.Fatalf("%s: NewProgress: %v", name, err)
	}
	return prog
}

func mustMigrate(t *testing.T, name string, s Save) Save {
	t.Helper()
	up, err := Migrate(s)
	if err != nil {
		t.Fatalf("%s: Migrate: %v", name, err)
	}
	return up
}

func mustStoreLoad(t *testing.T, name string, s Save, path string) Save {
	t.Helper()
	if err := s.Store(path); err != nil {
		t.Fatalf("%s: Store: %v", name, err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatalf("%s: reload: %v", name, err)
	}
	return again
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

// A:存读槽位版本迁移落在冻结数上,文件与API两条路算出同一个数.
func TestSaveFromCases(t *testing.T) {
	c := loadSaveCases(t)
	if CurrentVersion.String() != c.Version {
		t.Fatalf("version = %v, want %v", CurrentVersion, c.Version)
	}
	if MaxSlots != c.MaxSlots || MaxInventory != c.MaxInventory ||
		MaxItemCount != c.MaxItemCount || MaxStars != c.MaxStars ||
		MaxStarsPerLevel != c.MaxStarsPerLevel || MaxNameLen != c.MaxNameLen ||
		MaxBytes != c.MaxBytes {
		t.Fatalf("limits diverge from save_cases.json")
	}
	seen := map[string]bool{}
	for _, want := range c.Saves {
		if seen[want.Name] {
			t.Errorf("duplicate save %q", want.Name)
		}
		seen[want.Name] = true
		got := mustLoadSave(t, want.Name, want.File)
		checkSave(t, want, got)
		// API rebuild lands on the same save as the file.
		built := rebuildViaAPI(t, want)
		if !built.Equal(got) {
			t.Errorf("%s: API rebuild diverges from file load", want.Name)
		}
		// Encode round-trips through Parse to the same save.
		if back := mustRoundTrip(t, want.Name, got); !back.Equal(got) {
			t.Errorf("%s: Encode round-trip diverged", want.Name)
		}
		// Current files migrate to themselves.
		if mig := mustMigrate(t, want.Name, got); !mig.Equal(got) {
			t.Errorf("%s: Migrate(current) diverged", want.Name)
		}
		// Store to TempDir then Load back: same save, no aliasing.
		dir := t.TempDir()
		if again := mustStoreLoad(t, want.Name, got, filepath.Join(dir, "slot.json")); !again.Equal(got) {
			t.Errorf("%s: Store/Load diverged", want.Name)
		}
	}
	// Legacy file parses at its old version and migrates up intact.
	leg := c.Legacy
	old := mustLoadSave(t, "legacy", leg.File)
	if old.Version().String() != leg.Version {
		t.Fatalf("legacy version = %v, want %v", old.Version(), leg.Version)
	}
	if old.Slot() != leg.Slot {
		t.Fatalf("legacy slot = %d, want %d", old.Slot(), leg.Slot)
	}
	up := mustMigrate(t, "legacy", old)
	if up.Version() != CurrentVersion || up.Version().String() != leg.WantVersion {
		t.Fatalf("migrated version = %v, want %v", up.Version(), leg.WantVersion)
	}
	if up.Slot() != old.Slot() || up.Progress() != old.Progress() || !starsEqual(up.Stars(), old.Stars()) {
		t.Fatal("migration dropped slot/progress/stars")
	}
	if len(up.Inventory()) != len(old.Inventory()) {
		t.Fatal("migration dropped inventory")
	}
	// Migrated bytes load as current.
	if cur := mustRoundTrip(t, "migrated", up); !cur.Equal(up) {
		t.Fatal("migrated round-trip diverged")
	}
}

func starsEqual(a, b map[string]int) bool {
	return len(a) == len(b) && starsSubset(a, b) && starsSubset(b, a)
}

// starsSubset reports whether every entry of a matches b.
func starsSubset(a, b map[string]int) bool {
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// firstStarKey picks one stored level for the alias probe. Callers
// already checked the golden is non-empty, so empty reports "".
func firstStarKey(st map[string]int) string {
	for k := range st {
		return k
	}
	return ""
}

// B:空零超大坏数据坏档全不崩不卡死,占位加报错.
func TestSaveEdgesNoCrash(t *testing.T) {
	c := loadSaveCases(t)
	if _, err := Parse(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil data code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Parse([]byte{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty data code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Load(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := Load(filepath.Join("testdata", "no_such.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing file code = %v, want not-found", core.CodeOf(err))
	}
	var empty Save
	if _, err := empty.Encode(); err == nil {
		t.Error("zero save Encode want error (must Migrate first)")
	} else if core.CodeOf(err) != core.CodeVersionMismatch {
		t.Errorf("zero Encode code = %v, want version-mismatch", core.CodeOf(err))
	}
	if err := empty.Store(filepath.Join(t.TempDir(), "x.json")); err == nil {
		t.Error("zero Store want error")
	}
	if err := empty.Store(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Store path code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Frozen bad files fail with their frozen codes, never panic.
	for _, b := range c.BadFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", b.File))
		if err != nil {
			t.Fatalf("read %s: %v", b.File, err)
		}
		_, err = Parse(raw)
		expectCode(t, b.File, err, codeFromName(b.WantCode))
		if _, err := Load(filepath.Join("testdata", b.File)); core.CodeOf(err) != codeFromName(b.WantCode) {
			t.Errorf("%s Load code = %v, want %v", b.File, core.CodeOf(err), b.WantCode)
		}
	}
	// Bad constructor args are InvalidArg and store nothing.
	if _, err := New(-1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("slot -1 code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := New(MaxSlots); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("slot max code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewItem("", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty id code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewItem(strings.Repeat("x", MaxNameLen+1), 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long id code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewItem("ok", 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("count 0 code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewItem("ok", MaxItemCount+1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("count huge code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewProgress(strings.Repeat("x", MaxNameLen+1), "", 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long level code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewProgress("", "", -1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("negative play code = %v, want invalid-arg", core.CodeOf(err))
	}
	s := mustNewSlot(t, "edges", 0)
	if err := s.SetProgress(Progress{level: strings.Repeat("x", MaxNameLen+1)}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad SetProgress code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.AddItem("", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad AddItem id code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.AddItem("ok", 0); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad AddItem count code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.SetStars("", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad SetStars level code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.SetStars("ok", MaxStarsPerLevel+1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("bad SetStars stars code = %v, want invalid-arg", core.CodeOf(err))
	}
	var nilSave *Save
	if err := nilSave.SetProgress(Progress{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetProgress code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilSave.AddItem("ok", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil AddItem code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilSave.SetStars("ok", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil SetStars code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Oversized file fails before JSON: OutOfMemory, never a guess.
	big := make([]byte, MaxBytes+1)
	if _, err := Parse(big); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge bytes code = %v, want out-of-memory", core.CodeOf(err))
	}
	// Merging beyond one stack is rejected without touching the save.
	m := mustNewSlot(t, "bulk", 0)
	mustAddItem(t, "bulk", "bulk", &m, MaxItemCount)
	snap := m.Inventory()
	if err := m.AddItem("bulk", 1); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("over-stack code = %v, want invalid-arg", core.CodeOf(err))
	}
	if got := m.Inventory(); len(got) != len(snap) || got[0].Count() != snap[0].Count() {
		t.Error("failed AddItem mutated the save")
	}
}

// C不适用(纯文件不画画):数路无损加逐位重放即两边同数.
func TestSaveBoundaryIdentical(t *testing.T) {
	c := loadSaveCases(t)
	for _, want := range c.Saves {
		got := mustLoadSave(t, want.Name, want.File)
		// Duration boundary is lossless through play_ms.
		if back := core.Milliseconds(got.Progress().PlayMs()); back != got.Progress().Play() {
			t.Errorf("%s: play boundary = %v, want %v", want.Name, back, got.Progress().Play())
		}
		// Version boundary round-trips through its string form.
		if back, err := core.ParseVersion(got.Version().String()); err != nil || back != got.Version() {
			t.Errorf("%s: version boundary = %v/%v", want.Name, back, err)
		}
		// Empty and full bound both ends: the smallest and largest frozen
		// numbers cross the boundary intact, not just the mid case.
		raw := mustEncode(t, want.Name, got)
		a := mustParse(t, want.Name, raw)
		b := mustParse(t, want.Name, raw)
		if !a.Equal(b) || !a.Equal(got) {
			t.Errorf("%s: replay diverged", want.Name)
		}
		if got2 := mustRoundTrip(t, want.Name, a); !got2.Equal(got) {
			t.Errorf("%s: second round-trip diverged", want.Name)
		}
	}
	// Copies never alias: mutating a return cannot corrupt the replay.
	mid := mustFindSave(t, c, "mid_slot1")
	s := mustLoadSave(t, "mid", mid.File)
	inv := s.Inventory()
	if len(inv) == 0 {
		t.Fatal("mid inventory is empty, golden invalid")
	}
	inv[0] = Item{id: "hacked", count: 1}
	if again := s.Inventory(); again[0].ID() == "hacked" {
		t.Error("Inventory aliases the save")
	}
	// Stars copies detach both ways: writing the returned map never
	// touches the save, and a later read sees the stored value intact.
	st := s.Stars()
	probeKey := firstStarKey(st)
	before, ok := s.StarsOf(probeKey)
	if !ok {
		t.Fatal("mid stars golden is empty")
	}
	for k := range st {
		st[k] = MaxStarsPerLevel
		delete(st, k)
		break
	}
	if got, ok := s.StarsOf(probeKey); !ok || got != before {
		t.Errorf("Stars aliases the save: got %v/%v, want %v/true", got, ok, before)
	}
	// Same file parses to the same save twice (no hidden state).
	raw, _ := os.ReadFile(filepath.Join("testdata", mid.File))
	a, _ := Parse(raw)
	b, _ := Parse(raw)
	if !a.Equal(b) {
		t.Error("same file parsed twice diverged")
	}
}

// D:大档跑得动,存读时长加字节有数.
func TestSavePerfLarge(t *testing.T) {
	// Synthetic load only (no golden): golden stays in save_cases.json.
	// perfSave builds the MaxInventory/MaxStars ceiling case through the
	// frozen constructors, so the budget numbers stay honest.
	s := buildPerfSave(t)
	const reps = 200
	start := time.Now()
	var nbytes int
	for i := 0; i < reps; i++ {
		back := mustRoundTripBytes(t, i, s)
		if !back.Equal(s) {
			t.Fatalf("rep %d diverged", i)
		}
		if i == 0 {
			nbytes = len(mustEncode(t, "perf", s))
		}
	}
	el := time.Since(start)
	t.Logf("save-large: %d reps encode+parse %d items %d stars %dB in %v (%.1f us/rep)", reps, MaxInventory, MaxStars, nbytes, el, float64(el.Microseconds())/reps)
	if nbytes > MaxBytes {
		t.Fatalf("large save %dB exceeds MaxBytes %d", nbytes, MaxBytes)
	}
	// Store/Load joins the cost once through TempDir files.
	dir := t.TempDir()
	path := filepath.Join(dir, "large.json")
	start = time.Now()
	if again := mustStoreLoad(t, "perf", s, path); !again.Equal(s) {
		t.Fatal("file round-trip diverged")
	}
	el = time.Since(start)
	t.Logf("save-file: store+load %dB in %v", nbytes, el)
}

// buildPerfSave fills one ceiling save: every slot of both budgets used.
func buildPerfSave(t *testing.T) Save {
	t.Helper()
	s := mustNewSlot(t, "perf", 0)
	mustSetProgress(t, "perf", &s, mustNewProgress(t, "perf", "perf-level", "perf-cp", core.Milliseconds(3600000)))
	for i := 0; i < MaxInventory; i++ {
		mustAddItem(t, "perf", strings.Repeat("p", 8)+itoa(i), &s, 1+(i%9))
	}
	for i := 0; i < MaxStars; i++ {
		mustSetStars(t, "perf", "stage-"+itoa(i), &s, 1+(i%3))
	}
	return s
}

// mustRoundTripBytes encodes s and parses it back for one perf rep.
func mustRoundTripBytes(t *testing.T, rep int, s Save) Save {
	t.Helper()
	raw, err := s.Encode()
	if err != nil {
		t.Fatalf("rep %d Encode: %v", rep, err)
	}
	back, err := Parse(raw)
	if err != nil {
		t.Fatalf("rep %d Parse: %v", rep, err)
	}
	return back
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var out [20]byte
	p := len(out)
	for i > 0 {
		p--
		out[p] = digits[i%10]
		i /= 10
	}
	return string(out[p:])
}

// E:反复存读长跑不坏不涨不漂,坏档不粘.
func TestSaveLongRunStable(t *testing.T) {
	c := loadSaveCases(t)
	mid := mustFindSave(t, c, "mid_slot1")
	s := mustLoadSave(t, "mid", mid.File)
	firstRaw := mustEncode(t, "mid", s)
	for i := 0; i < 1000; i++ {
		raw := mustEncode(t, "mid", s)
		if string(raw) != string(firstRaw) {
			t.Fatalf("rep %d bytes drifted", i)
		}
		if back := mustParse(t, "mid", raw); !back.Equal(s) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	// Walk a star up and back: stored value follows, never sticks.
	walk := mustLoadSave(t, "mid", mid.File)
	levels := []string{}
	for k := range walk.Stars() {
		levels = append(levels, k)
	}
	if len(levels) == 0 {
		t.Fatal("walk needs at least one starred level")
	}
	lv := levels[0]
	orig, _ := walk.StarsOf(lv)
	next := orig%MaxStarsPerLevel + 1
	mustSetStars(t, "walk", lv, &walk, next)
	if got, _ := walk.StarsOf(lv); got != next {
		t.Fatalf("walk set = %d, want %d", got, next)
	}
	mustSetStars(t, "walk", lv, &walk, orig)
	if got, _ := walk.StarsOf(lv); got != orig {
		t.Fatalf("walk back = %d, want %d", got, orig)
	}
	if !walk.Equal(s) {
		t.Fatal("walk did not return to the start")
	}
	// Bad data never poisons the next good parse; file round-trips stable.
	for _, b := range c.BadFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", b.File))
		if _, err := Parse(raw); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	if again := mustParse(t, "mid", firstRaw); !again.Equal(s) {
		t.Fatal("good parse after bad files diverged")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.json")
	var size int64
	for i := 0; i < 200; i++ {
		back := mustStoreLoad(t, "stable", s, path)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %d: %v", i, err)
		}
		if i == 0 {
			size = fi.Size()
		} else if fi.Size() != size {
			t.Fatalf("store %d size %d, want %d", i, fi.Size(), size)
		}
		if !back.Equal(s) {
			t.Fatalf("file rep %d diverged", i)
		}
	}
}

// F:离屏金对照窗(窗免,纯文件):冻结数加形状断言,存读数即证据.
func TestSaveOffscreenGolden(t *testing.T) {
	c := loadSaveCases(t)
	emptySave := mustLoadSave(t, "empty_slot0", mustFindSave(t, c, "empty_slot0").File)
	midSave := mustLoadSave(t, "mid_slot1", mustFindSave(t, c, "mid_slot1").File)
	fullSave := mustLoadSave(t, "full_slot2", mustFindSave(t, c, "full_slot2").File)
	// Golden pins the anchors through the file, not the code.
	checkSave(t, mustFindSave(t, c, "empty_slot0"), emptySave)
	checkSave(t, mustFindSave(t, c, "mid_slot1"), midSave)
	checkSave(t, mustFindSave(t, c, "full_slot2"), fullSave)
	// Shape: slots stay in range and distinct; versions all current.
	slots := map[int]bool{}
	for _, want := range c.Saves {
		if want.Slot < 0 || want.Slot >= MaxSlots {
			t.Errorf("%s slot %d out of 0..%d", want.Name, want.Slot, MaxSlots-1)
		}
		slots[want.Slot] = true
		if want.Version != CurrentVersion.String() {
			t.Errorf("%s version %v, want current %v", want.Name, want.Version, CurrentVersion)
		}
	}
	if len(slots) != len(c.Saves) {
		t.Errorf("slots %v collide, want distinct", slots)
	}
	// Shape: empty is a new game, mid/full grow monotonically.
	if p := emptySave.Progress(); p.Level() != "" || p.PlayMs() != 0 {
		t.Errorf("empty progress = %q/%d, want new game", p.Level(), p.PlayMs())
	}
	if len(emptySave.Inventory()) != 0 || len(emptySave.Stars()) != 0 {
		t.Error("empty save is not empty")
	}
	if len(midSave.Inventory()) == 0 || len(midSave.Stars()) == 0 || midSave.Progress().PlayMs() <= 0 {
		t.Error("mid save looks empty, want progress plus backpack plus stars")
	}
	if !(len(fullSave.Inventory()) > len(midSave.Inventory()) && len(fullSave.Stars()) > len(midSave.Stars())) {
		t.Errorf("full inv/stars %d/%d not larger than mid %d/%d",
			len(fullSave.Inventory()), len(fullSave.Stars()), len(midSave.Inventory()), len(midSave.Stars()))
	}
	if !(fullSave.Progress().PlayMs() > midSave.Progress().PlayMs()) {
		t.Error("full play time not larger than mid")
	}
	// Shape: every stored number stays inside its frozen budget.
	for _, want := range c.Saves {
		got := mustLoadSave(t, want.Name, want.File)
		for _, it := range got.Inventory() {
			if it.Count() <= 0 || it.Count() > MaxItemCount {
				t.Errorf("%s item %q count %d out of 1..%d", want.Name, it.ID(), it.Count(), MaxItemCount)
			}
		}
		for k, v := range got.Stars() {
			if v <= 0 || v > MaxStarsPerLevel {
				t.Errorf("%s stars[%q] = %d, want 1..%d", want.Name, k, v, MaxStarsPerLevel)
			}
		}
		if nbytes := len(mustEncode(t, want.Name, got)); nbytes > MaxBytes {
			t.Errorf("%s %dB exceeds MaxBytes", want.Name, nbytes)
		}
		// Budget edges stay loadable: full is the largest frozen save and
		// still far under MaxBytes, so growth headroom is measured.
		if want.Name == "full_slot2" {
			raw := mustEncode(t, want.Name, got)
			if back := mustParse(t, want.Name, raw); !back.Equal(got) {
				t.Errorf("%s: budget-edge round-trip diverged", want.Name)
			}
		}
	}
	// Shape: legacy migrates version only; numbers never change.
	leg := c.Legacy
	old := mustLoadSave(t, "legacy", leg.File)
	up := mustMigrate(t, "legacy", old)
	if up.Version() != CurrentVersion {
		t.Errorf("migrated version = %v, want %v", up.Version(), CurrentVersion)
	}
	if up.Slot() != old.Slot() || up.Progress() != old.Progress() {
		t.Error("migration changed slot/progress, want version bump only")
	}
	if len(up.Inventory()) != len(leg.Inventory) || len(up.Stars()) != len(leg.Stars) {
		t.Error("migration changed backpack/stars size, want intact")
	}
}
