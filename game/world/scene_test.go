package world

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Scene/prefab file tests (10.2, S33/W4): the planner's level on disk
// loads in one go and opens into a live Scene. Window intent:
// game_world--case=open; this package only computes file math, so the
// offscreen golden (file bytes round-trip byte-identical) is the
// evidence and no live window is required.

type sceneCompCase struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}

type scenePrefabCase struct {
	File  string          `json:"file"`
	Name  string          `json:"name"`
	Pos   [2]float64      `json:"pos"`
	Rot   float64         `json:"rot"`
	Scale [2]float64      `json:"scale"`
	Comps []sceneCompCase `json:"comps"`
}

type sceneWantWorld struct {
	Name  string          `json:"name"`
	Pos   [2]float64      `json:"pos"`
	Rot   float64         `json:"rot"`
	Scale [2]float64      `json:"scale"`
	Mat   [6]float64      `json:"mat"`
	Comps []sceneCompCase `json:"comps"`
}

type sceneFileCase struct {
	File  string           `json:"file"`
	Name  string           `json:"name"`
	Count int              `json:"count"`
	Want  []sceneWantWorld `json:"want_world"`
}

type sceneBadCase struct {
	File     string `json:"file"`
	Parse    string `json:"parse"`
	WantCode string `json:"want_code"`
}

type sceneCasesFile struct {
	Version          string          `json:"version"`
	PrefabVersion    string          `json:"prefab_version"`
	MaxPrefabNameLen int             `json:"max_prefab_name_len"`
	MaxPrefabComps   int             `json:"max_prefab_comps"`
	MaxPrefabBytes   int             `json:"max_prefab_bytes"`
	MaxSceneNameLen  int             `json:"max_scene_name_len"`
	MaxSceneEntities int             `json:"max_scene_entities"`
	MaxSceneComps    int             `json:"max_scene_comps"`
	MaxSceneBytes    int             `json:"max_scene_bytes"`
	Prefab           scenePrefabCase `json:"prefab"`
	Scene            sceneFileCase   `json:"scene"`
	BadFiles         []sceneBadCase  `json:"bad_files"`
	Perf             struct {
		Entities int `json:"entities"`
		Reps     int `json:"reps"`
	} `json:"perf"`
	Longrun struct {
		Cycles int `json:"cycles"`
		Batch  int `json:"batch"`
	} `json:"longrun"`
}

func loadSceneCases(t *testing.T) sceneCasesFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "scene_cases.json"))
	if err != nil {
		t.Fatalf("read scene_cases.json: %v", err)
	}
	var c sceneCasesFile
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode scene_cases.json: %v", err)
	}
	if c.Prefab.File == "" || c.Scene.File == "" || len(c.Scene.Want) == 0 || len(c.BadFiles) == 0 {
		t.Fatal("scene_cases.json has no prefab/scene/bad files")
	}
	return c
}

func expectSceneCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func sceneCodeFromName(name string) core.Code {
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

func mustLoadPrefab(t *testing.T, name, file string) Prefab {
	t.Helper()
	got, err := LoadPrefab(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("%s: LoadPrefab: %v", name, err)
	}
	return got
}

func mustLoadScene(t *testing.T, name, file string) SceneFile {
	t.Helper()
	got, err := LoadScene(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("%s: LoadScene: %v", name, err)
	}
	return got
}

func mustOpenScene(t *testing.T, name string, sf *SceneFile) (Scene, map[string]ID) {
	t.Helper()
	sc, ids, err := sf.Open()
	if err != nil {
		t.Fatalf("%s: Open: %v", name, err)
	}
	return sc, ids
}

func checkSceneComps(t *testing.T, tag, name string, w *World, id ID, want []sceneCompCase) {
	t.Helper()
	got, err := w.Comps(id)
	if err != nil {
		t.Fatalf("%s/%s Comps: %v", tag, name, err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s/%s comps = %d, want %d", tag, name, len(got), len(want))
	}
	for i, wc := range want {
		if got[i].Kind != wc.Kind || string(got[i].Ref) != wc.Ref {
			t.Errorf("%s/%s comp[%d] = {%s %s}, want {%s %s}",
				tag, name, i, got[i].Kind, got[i].Ref, wc.Kind, wc.Ref)
		}
	}
}

func checkSceneWant(t *testing.T, tag string, sc *Scene, ids map[string]ID, want sceneWantWorld) {
	t.Helper()
	id, ok := ids[want.Name]
	if !ok {
		t.Fatalf("%s: filed name %q never opened", tag, want.Name)
	}
	got, err := sc.World().WorldOf(id)
	if err != nil {
		t.Fatalf("%s/%s WorldOf: %v", tag, want.Name, err)
	}
	if !near(got.Pos.X, want.Pos[0]) || !near(got.Pos.Y, want.Pos[1]) {
		t.Errorf("%s/%s world pos = %v, want %v", tag, want.Name, got.Pos, want.Pos)
	}
	if !near(got.Rot, want.Rot) {
		t.Errorf("%s/%s world rot = %.17g, want %.17g", tag, want.Name, got.Rot, want.Rot)
	}
	if !near(got.Scale.X, want.Scale[0]) || !near(got.Scale.Y, want.Scale[1]) {
		t.Errorf("%s/%s world scale = %v, want %v", tag, want.Name, got.Scale, want.Scale)
	}
	m, err := sc.World().WorldMatrix(id)
	if err != nil {
		t.Fatalf("%s/%s WorldMatrix: %v", tag, want.Name, err)
	}
	c := [6]float64{m.A, m.B, m.C, m.D, m.E, m.F}
	for i := range c {
		if !near(c[i], want.Mat[i]) {
			t.Errorf("%s/%s world mat = %v, want %v", tag, want.Name, c, want.Mat)
			break
		}
	}
	if st, err := sc.State(id); err != nil || st != LifeActive {
		t.Errorf("%s/%s state = %v/%v, want active nil", tag, want.Name, st, err)
	}
	checkSceneComps(t, tag, want.Name, sc.World(), id, want.Comps)
}

// A:摆好装出来对:预制与场景文件一次装成活数,父子挂件引用全落在冻结数上.
func TestSceneOpenFromCases(t *testing.T) {
	c := loadSceneCases(t)
	if PrefabVersion.String() != c.PrefabVersion {
		t.Fatalf("prefab version = %v, want %v", PrefabVersion, c.PrefabVersion)
	}
	if SceneFileVersion.String() != c.Version {
		t.Fatalf("scene version = %v, want %v", SceneFileVersion, c.Version)
	}
	if MaxPrefabNameLen != c.MaxPrefabNameLen || MaxPrefabComps != c.MaxPrefabComps ||
		MaxPrefabBytes != c.MaxPrefabBytes || MaxSceneNameLen != c.MaxSceneNameLen ||
		MaxSceneEntities != c.MaxSceneEntities || MaxSceneComps != c.MaxSceneComps ||
		MaxSceneBytes != c.MaxSceneBytes {
		t.Fatal("limits diverge from scene_cases.json")
	}

	// Prefab file owns its numbers: name, local, comps in stamp order.
	pf := mustLoadPrefab(t, "prefab", c.Prefab.File)
	if pf.Name() != c.Prefab.Name {
		t.Errorf("prefab name = %q, want %q", pf.Name(), c.Prefab.Name)
	}
	if pf.Version() != PrefabVersion {
		t.Errorf("prefab version = %v, want %v", pf.Version(), PrefabVersion)
	}
	pl := pf.Local()
	if pl.Pos.X != c.Prefab.Pos[0] || pl.Pos.Y != c.Prefab.Pos[1] ||
		pl.Rot != c.Prefab.Rot || pl.Scale.X != c.Prefab.Scale[0] || pl.Scale.Y != c.Prefab.Scale[1] {
		t.Errorf("prefab local = %+v, want pos %v rot %v scale %v", pl, c.Prefab.Pos, c.Prefab.Rot, c.Prefab.Scale)
	}
	if got := pf.Comps(); len(got) != len(c.Prefab.Comps) {
		t.Fatalf("prefab comps = %d, want %d", len(got), len(c.Prefab.Comps))
	} else {
		for i, wc := range c.Prefab.Comps {
			if got[i].Kind != wc.Kind || string(got[i].Ref) != wc.Ref {
				t.Errorf("prefab comp[%d] = {%s %s}, want {%s %s}", i, got[i].Kind, got[i].Ref, wc.Kind, wc.Ref)
			}
		}
	}
	// Stamping lands the same numbers in a World, under any parent.
	w := NewWorld()
	root, err := pf.Instantiate(&w, NoEntity)
	if err != nil {
		t.Fatalf("Instantiate root: %v", err)
	}
	rl, err := w.Local(root)
	if err != nil || rl != pl {
		t.Errorf("stamped local = %+v/%v, want %+v", rl, err, pl)
	}
	checkSceneComps(t, "prefab", c.Prefab.Name, &w, root, c.Prefab.Comps)
	kid, err := pf.Instantiate(&w, root)
	if err != nil {
		t.Fatalf("Instantiate child: %v", err)
	}
	if p, _ := w.Parent(kid); p != root {
		t.Errorf("stamped child parent = %d, want %d", p, root)
	}
	if w.Count() != 2 {
		t.Errorf("world count = %d, want 2 stamped", w.Count())
	}

	// Scene file opens in one go: count, links, worlds, comps, states.
	sf := mustLoadScene(t, "scene", c.Scene.File)
	if sf.Name() != c.Scene.Name {
		t.Errorf("scene name = %q, want %q", sf.Name(), c.Scene.Name)
	}
	if sf.Version() != SceneFileVersion {
		t.Errorf("scene version = %v, want %v", sf.Version(), SceneFileVersion)
	}
	if sf.Count() != c.Scene.Count {
		t.Fatalf("scene count = %d, want %d", sf.Count(), c.Scene.Count)
	}
	sc, ids := mustOpenScene(t, "scene", &sf)
	if sc.Count() != c.Scene.Count {
		t.Fatalf("opened count = %d, want %d", sc.Count(), c.Scene.Count)
	}
	if len(ids) != c.Scene.Count {
		t.Fatalf("opened index = %d names, want %d", len(ids), c.Scene.Count)
	}
	// Parent links survive the file: sword hangs under hero, slime under
	// crate, both roots hang under nothing.
	linkWant := map[string]string{"hero": "", "sword": "hero", "crate": "", "slime": "crate"}
	for name, parent := range linkWant {
		got, err := sc.World().Parent(ids[name])
		if err != nil {
			t.Fatalf("open/%s Parent: %v", name, err)
		}
		if got != ids[parent] {
			t.Errorf("open/%s parent = %d, want %q (%d)", name, got, parent, ids[parent])
		}
	}
	for _, want := range c.Scene.Want {
		checkSceneWant(t, "open", &sc, ids, want)
	}
	// Filed bytes survive a memory round-trip unchanged.
	if back, err := ParseScene(mustSceneEncode(t, "scene", &sf)); err != nil || !back.Equal(sf) {
		t.Errorf("scene Encode round-trip equal = %v, err = %v", back.Equal(sf), err)
	}
	if back, err := ParsePrefab(mustPrefabEncode(t, "prefab", &pf)); err != nil || !back.Equal(pf) {
		t.Errorf("prefab Encode round-trip equal = %v, err = %v", back.Equal(pf), err)
	}
}

func mustSceneEncode(t *testing.T, name string, sf *SceneFile) []byte {
	t.Helper()
	raw, err := sf.Encode()
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}
	return raw
}

func mustPrefabEncode(t *testing.T, name string, pf *Prefab) []byte {
	t.Helper()
	raw, err := pf.Encode()
	if err != nil {
		t.Fatalf("%s: Encode: %v", name, err)
	}
	return raw
}

// B:缺文件报错不崩:错码分清,坏写存不下东西,nil接收器不炸.
func TestSceneEdgesNoCrash(t *testing.T) {
	c := loadSceneCases(t)
	if _, err := LoadPrefab(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty prefab path code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadScene(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty scene path code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := LoadPrefab(filepath.Join("testdata", "no_such.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing prefab code = %v, want not-found", core.CodeOf(err))
	}
	if _, err := LoadScene(filepath.Join("testdata", "no_such.json")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("missing scene code = %v, want not-found", core.CodeOf(err))
	}
	if _, err := ParsePrefab(nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil prefab data code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := ParseScene([]byte{}); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty scene data code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Frozen bad files fail with their frozen codes through both Parse
	// and Load, never panic.
	for _, b := range c.BadFiles {
		raw, err := os.ReadFile(filepath.Join("testdata", b.File))
		if err != nil {
			t.Fatalf("read %s: %v", b.File, err)
		}
		want := sceneCodeFromName(b.WantCode)
		var perr error
		var lerr error
		if b.Parse == "prefab" {
			_, perr = ParsePrefab(raw)
			_, lerr = LoadPrefab(filepath.Join("testdata", b.File))
		} else {
			_, perr = ParseScene(raw)
			_, lerr = LoadScene(filepath.Join("testdata", b.File))
		}
		expectSceneCode(t, b.File+" parse", perr, want)
		expectSceneCode(t, b.File+" load", lerr, want)
	}
	// Oversized input fails before JSON: OutOfMemory, never a guess.
	if _, err := ParsePrefab(make([]byte, MaxPrefabBytes+1)); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge prefab code = %v, want out-of-memory", core.CodeOf(err))
	}
	if _, err := ParseScene(make([]byte, MaxSceneBytes+1)); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge scene code = %v, want out-of-memory", core.CodeOf(err))
	}

	// Bad constructor args are InvalidArg and store nothing.
	if _, err := NewPrefab("", IdentityTransform()); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty prefab name code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewPrefab(strings.Repeat("x", MaxPrefabNameLen+1), IdentityTransform()); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long prefab name code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := NewSceneFile(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty scene name code = %v, want invalid-arg", core.CodeOf(err))
	}
	pf, err := NewPrefab("edge", IdentityTransform())
	if err != nil {
		t.Fatalf("NewPrefab: %v", err)
	}
	before := pf.Local()
	expectSceneCode(t, "bad SetLocal", pf.SetLocal(Transform{Scale: core.V2(1, math.Inf(1))}), core.CodeInvalidArg)
	if pf.Local() != before {
		t.Error("rejected SetLocal moved the stored local")
	}
	expectSceneCode(t, "empty prefab comp", pf.AddComp(Comp{}), core.CodeInvalidArg)
	// Filling the comp budget then one more is OutOfMemory, and the
	// stored list stays at the ceiling.
	for i := 0; i < MaxPrefabComps; i++ {
		if err := pf.AddComp(Comp{Kind: fmt.Sprintf("k%d", i)}); err != nil {
			t.Fatalf("fill comp %d: %v", i, err)
		}
	}
	expectSceneCode(t, "prefab comp over budget", pf.AddComp(Comp{Kind: "one-more"}), core.CodeOutOfMemory)
	if len(pf.Comps()) != MaxPrefabComps {
		t.Errorf("comps after rejected add = %d, want %d", len(pf.Comps()), MaxPrefabComps)
	}

	sf, err := NewSceneFile("edge")
	if err != nil {
		t.Fatalf("NewSceneFile: %v", err)
	}
	expectSceneCode(t, "empty entity name", sf.AddEntity("", "", IdentityTransform()), core.CodeInvalidArg)
	expectSceneCode(t, "unknown parent", sf.AddEntity("kid", "ghost", IdentityTransform()), core.CodeNotFound)
	expectSceneCode(t, "bad entity transform",
		sf.AddEntity("bad", "", Transform{Pos: core.V2(math.NaN(), 0), Scale: core.V2(1, 1)}), core.CodeInvalidArg)
	expectSceneCode(t, "empty entity comp",
		sf.AddEntity("bad", "", IdentityTransform(), Comp{}), core.CodeInvalidArg)
	if err := sf.AddEntity("root", "", IdentityTransform()); err != nil {
		t.Fatalf("AddEntity root: %v", err)
	}
	expectSceneCode(t, "duplicate entity", sf.AddEntity("root", "", IdentityTransform()), core.CodeInvalidArg)
	if sf.Count() != 1 {
		t.Errorf("count after rejected adds = %d, want 1", sf.Count())
	}

	// Failed stamps birth nothing.
	w := NewWorld()
	expectSceneCode(t, "stamp under dead", func() error { _, err := pf.Instantiate(&w, 9999); return err }(), core.CodeNotFound)
	if w.Count() != 0 {
		t.Errorf("count after failed stamp = %d, want 0", w.Count())
	}
	var nilWorld *World
	expectSceneCode(t, "stamp nil world", func() error { _, err := pf.Instantiate(nilWorld, NoEntity); return err }(), core.CodeInvalidArg)
	// Zero values encode to InvalidArg, never a guessed file.
	var zeroPrefab Prefab
	expectSceneCode(t, "zero prefab encode", func() error { _, err := zeroPrefab.Encode(); return err }(), core.CodeInvalidArg)
	var zeroScene SceneFile
	expectSceneCode(t, "zero scene open", func() error { _, _, err := zeroScene.Open(); return err }(), core.CodeInvalidArg)

	// Nil receivers never panic: writers refuse, getters park.
	var npf *Prefab
	expectSceneCode(t, "nil prefab SetLocal", npf.SetLocal(IdentityTransform()), core.CodeInvalidArg)
	expectSceneCode(t, "nil prefab AddComp", npf.AddComp(Comp{Kind: "x"}), core.CodeInvalidArg)
	expectSceneCode(t, "nil prefab Encode", func() error { _, err := npf.Encode(); return err }(), core.CodeInvalidArg)
	expectSceneCode(t, "nil prefab Save", npf.SavePrefab(filepath.Join(t.TempDir(), "x.json")), core.CodeInvalidArg)
	expectSceneCode(t, "nil prefab stamp", func() error { _, err := npf.Instantiate(&w, NoEntity); return err }(), core.CodeInvalidArg)
	if npf.Name() != "" || npf.Version() != (core.Version{}) || len(npf.Comps()) != 0 || npf.Equal(pf) {
		t.Error("nil prefab getters left zero, want parked")
	}
	var nsf *SceneFile
	expectSceneCode(t, "nil scene Add", nsf.AddEntity("x", "", IdentityTransform()), core.CodeInvalidArg)
	expectSceneCode(t, "nil scene Encode", func() error { _, err := nsf.Encode(); return err }(), core.CodeInvalidArg)
	expectSceneCode(t, "nil scene Save", nsf.SaveScene(filepath.Join(t.TempDir(), "x.json")), core.CodeInvalidArg)
	expectSceneCode(t, "nil scene Open", func() error { _, _, err := nsf.Open(); return err }(), core.CodeInvalidArg)
	if nsf.Name() != "" || nsf.Version() != (core.Version{}) || nsf.Count() != 0 || len(nsf.Entities()) != 0 || nsf.Equal(sf) {
		t.Error("nil scene getters left zero, want parked")
	}
}

// C不适用(纯文件不画画):同文件双解析逐位一致,边界往返无损即两边同数.
func TestSceneBoundaryIdentical(t *testing.T) {
	c := loadSceneCases(t)
	pfRaw, err := os.ReadFile(filepath.Join("testdata", c.Prefab.File))
	if err != nil {
		t.Fatalf("read prefab: %v", err)
	}
	sfRaw, err := os.ReadFile(filepath.Join("testdata", c.Scene.File))
	if err != nil {
		t.Fatalf("read scene: %v", err)
	}
	// Same bytes parse to the same file twice; encodes match byte for
	// byte, so both readers feed the draw side identical numbers.
	pa, err := ParsePrefab(pfRaw)
	if err != nil {
		t.Fatalf("ParsePrefab a: %v", err)
	}
	pb, err := ParsePrefab(pfRaw)
	if err != nil {
		t.Fatalf("ParsePrefab b: %v", err)
	}
	if !pa.Equal(pb) {
		t.Fatal("same prefab bytes parsed twice diverged")
	}
	ra, rb := mustPrefabEncode(t, "prefab a", &pa), mustPrefabEncode(t, "prefab b", &pb)
	if string(ra) != string(rb) || string(ra) != string(pfRaw) {
		t.Error("prefab replay diverged from the filed bytes")
	}
	sa, err := ParseScene(sfRaw)
	if err != nil {
		t.Fatalf("ParseScene a: %v", err)
	}
	sb, err := ParseScene(sfRaw)
	if err != nil {
		t.Fatalf("ParseScene b: %v", err)
	}
	if !sa.Equal(sb) {
		t.Fatal("same scene bytes parsed twice diverged")
	}
	ba, bb := mustSceneEncode(t, "scene a", &sa), mustSceneEncode(t, "scene b", &sb)
	if string(ba) != string(bb) || string(ba) != string(sfRaw) {
		t.Error("scene replay diverged from the filed bytes")
	}
	// Opening twice lands the same worlds: every filed name maps to the
	// same world matrix on both runs.
	first, firstIDs := mustOpenScene(t, "scene a", &sa)
	second, secondIDs := mustOpenScene(t, "scene b", &sb)
	for _, want := range c.Scene.Want {
		ma, err := first.World().WorldMatrix(firstIDs[want.Name])
		if err != nil {
			t.Fatalf("open a %s matrix: %v", want.Name, err)
		}
		mb, err := second.World().WorldMatrix(secondIDs[want.Name])
		if err != nil {
			t.Fatalf("open b %s matrix: %v", want.Name, err)
		}
		if ma != mb {
			t.Fatalf("open %s replay diverged: %+v vs %+v", want.Name, ma, mb)
		}
	}
	// Stamping twice lands the same local: the prefab feeds the draw
	// side identical numbers on both runs.
	wa, wb := NewWorld(), NewWorld()
	ia, err := pa.Instantiate(&wa, NoEntity)
	if err != nil {
		t.Fatalf("stamp a: %v", err)
	}
	ib, err := pa.Instantiate(&wb, NoEntity)
	if err != nil {
		t.Fatalf("stamp b: %v", err)
	}
	la, _ := wa.Local(ia)
	lb, _ := wb.Local(ib)
	if la != lb {
		t.Fatal("prefab stamp replay diverged")
	}
	// Asset boundary is lossless: filed refs cross into AssetID intact.
	for _, want := range c.Scene.Want {
		for _, wc := range want.Comps {
			if string(core.AssetID(wc.Ref)) != wc.Ref {
				t.Errorf("%s ref %q moved on the boundary", want.Name, wc.Ref)
			}
		}
	}
	// Copies never alias: mutating a return cannot corrupt the file.
	ents := sa.Entities()
	ents[0].Name = "hacked"
	ents[0].Comps[0].Kind = "hacked"
	fresh := sa.Entities()
	if fresh[0].Name == "hacked" || fresh[0].Comps[0].Kind == "hacked" {
		t.Error("Entities aliases the file, want a deep copy")
	}
	got := pa.Comps()
	got[0].Kind = "hacked"
	if again := pa.Comps(); again[0].Kind == "hacked" {
		t.Error("Prefab Comps aliases the template, want a fresh copy")
	}
}

// D:大场景跑得动,解析加开局时长与字节有数.
func TestScenePerfLarge(t *testing.T) {
	c := loadSceneCases(t)
	n, reps := c.Perf.Entities, c.Perf.Reps
	if n <= 0 || reps <= 0 {
		t.Fatal("perf params missing, want frozen entities and reps")
	}
	// Synthetic load only (no golden): golden stays in scene_cases.json.
	sf, err := NewSceneFile("perf")
	if err != nil {
		t.Fatalf("NewSceneFile: %v", err)
	}
	if err := sf.AddEntity("root", "", IdentityTransform(), Comp{Kind: "sprite", Ref: "tex/root"}); err != nil {
		t.Fatalf("AddEntity root: %v", err)
	}
	for i := 1; i < n; i++ {
		name := fmt.Sprintf("e%05d", i)
		local := Transform{Pos: core.V2(float64(i%64), float64(i/64)), Scale: core.V2(1, 1)}
		if err := sf.AddEntity(name, "root", local, Comp{Kind: "sprite", Ref: "tex/tile"}); err != nil {
			t.Fatalf("AddEntity %s: %v", name, err)
		}
	}
	raw := mustSceneEncode(t, "perf", &sf)
	if len(raw) > MaxSceneBytes {
		t.Fatalf("perf scene %dB exceeds MaxSceneBytes %d", len(raw), MaxSceneBytes)
	}
	var parseEl, openEl, matEl time.Duration
	var acc float64
	for r := 0; r < reps; r++ {
		start := time.Now()
		back, err := ParseScene(raw)
		if err != nil {
			t.Fatalf("rep %d Parse: %v", r, err)
		}
		parseEl += time.Since(start)
		if !back.Equal(sf) {
			t.Fatalf("rep %d diverged", r)
		}
		start = time.Now()
		sc, ids, err := back.Open()
		if err != nil {
			t.Fatalf("rep %d Open: %v", r, err)
		}
		openEl += time.Since(start)
		if sc.Count() != n {
			t.Fatalf("rep %d count = %d, want %d", r, sc.Count(), n)
		}
		start = time.Now()
		for _, id := range ids {
			m, err := sc.World().WorldMatrix(id)
			if err != nil {
				t.Fatalf("rep %d WorldMatrix: %v", r, err)
			}
			acc += m.C + m.F
		}
		matEl += time.Since(start)
	}
	t.Logf("scene-large: %d entities %d reps %dB: parse %v (%.1f us/rep), open %v (%.1f us/rep), %d matrices %v (%.1f ns/op)",
		n, reps, len(raw), parseEl, float64(parseEl.Microseconds())/float64(reps),
		openEl, float64(openEl.Microseconds())/float64(reps),
		n*reps, matEl, float64(matEl.Nanoseconds())/float64(n*reps))
	if math.IsNaN(acc) || math.IsInf(acc, 0) || acc == 0 {
		t.Error("perf accumulation invalid, benchmark meaningless")
	}
}

// E:反复进出不涨:活数回到基线,字节不漂,发号只涨不回头,坏档不粘.
func TestSceneLongRunStable(t *testing.T) {
	c := loadSceneCases(t)
	if c.Longrun.Cycles <= 0 || c.Longrun.Batch <= 0 {
		t.Fatal("longrun params missing, want frozen cycles and batch")
	}
	good := mustLoadScene(t, "stable", c.Scene.File)
	firstRaw := mustSceneEncode(t, "stable", &good)
	for i := 0; i < c.Longrun.Cycles; i++ {
		sf, err := LoadScene(filepath.Join("testdata", c.Scene.File))
		if err != nil {
			t.Fatalf("cycle %d load: %v", i, err)
		}
		sc, ids, err := sf.Open()
		if err != nil {
			t.Fatalf("cycle %d open: %v", i, err)
		}
		if sc.Count() != c.Scene.Count {
			t.Fatalf("cycle %d count = %d, want %d", i, sc.Count(), c.Scene.Count)
		}
		// Each entry is a fresh level: issuance starts at the filed
		// baseline and only grows inside this cycle.
		base := sc.Spawned()
		if base != uint64(c.Scene.Count) {
			t.Fatalf("cycle %d spawned = %d, want filed %d", i, base, c.Scene.Count)
		}
		// Stamp a batch of prefabs, then dispose them: the living
		// returns to the filed baseline, the ledger keeps every birth.
		pf := mustLoadPrefab(t, "stable prefab", c.Prefab.File)
		born := make([]ID, 0, c.Longrun.Batch)
		for j := 0; j < c.Longrun.Batch; j++ {
			id, err := pf.Instantiate(sc.World(), ids["hero"])
			if err != nil {
				t.Fatalf("cycle %d stamp %d: %v", i, j, err)
			}
			born = append(born, id)
		}
		if sc.Count() != c.Scene.Count+c.Longrun.Batch {
			t.Fatalf("cycle %d count = %d, want %d", i, sc.Count(), c.Scene.Count+c.Longrun.Batch)
		}
		if sc.Spawned() != base+uint64(c.Longrun.Batch) {
			t.Fatalf("cycle %d spawned = %d, want ledger %d", i, sc.Spawned(), base+uint64(c.Longrun.Batch))
		}
		for _, id := range born {
			if err := sc.Dispose(id); err != nil {
				t.Fatalf("cycle %d dispose(%d): %v", id, i, err)
			}
		}
		if sc.Count() != c.Scene.Count {
			t.Fatalf("cycle %d after dispose = %d, want baseline %d", i, sc.Count(), c.Scene.Count)
		}
		if sc.Spawned() != base+uint64(c.Longrun.Batch) {
			t.Fatalf("cycle %d ledger rewinds: spawned %d", i, sc.Spawned())
		}
		if raw := mustSceneEncode(t, "stable", &sf); string(raw) != string(firstRaw) {
			t.Fatalf("cycle %d bytes drifted", i)
		}
		// A level switch clears everything but never rewinds issuance:
		// the next birth lands exactly one past every birth so far.
		sc.Clear()
		if sc.Count() != 0 {
			t.Fatalf("cycle %d after clear = %d, want 0", i, sc.Count())
		}
		fresh, err := sc.Spawn(NoEntity)
		if err != nil {
			t.Fatalf("cycle %d spawn after clear: %v", i, err)
		}
		if uint64(fresh) != base+uint64(c.Longrun.Batch)+1 || sc.Spawned() != uint64(fresh) {
			t.Fatalf("cycle %d post-clear id %d breaks the ledger (spawned %d)", i, fresh, sc.Spawned())
		}
	}
	// Bad data never poisons the next good parse.
	for _, b := range c.BadFiles {
		raw, _ := os.ReadFile(filepath.Join("testdata", b.File))
		if b.Parse == "prefab" {
			if _, err := ParsePrefab(raw); err == nil {
				t.Errorf("%s want error", b.File)
			}
		} else if _, err := ParseScene(raw); err == nil {
			t.Errorf("%s want error", b.File)
		}
	}
	if again, err := ParseScene(firstRaw); err != nil || !again.Equal(good) {
		t.Fatal("good parse after bad files diverged")
	}
	// File round-trips through TempDir at a stable size.
	dir := t.TempDir()
	path := filepath.Join(dir, "stable.json")
	var size int64
	for i := 0; i < 50; i++ {
		if err := good.SaveScene(path); err != nil {
			t.Fatalf("store %d: %v", i, err)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %d: %v", i, err)
		}
		if i == 0 {
			size = fi.Size()
		} else if fi.Size() != size {
			t.Fatalf("store %d size %d, want %d", i, fi.Size(), size)
		}
		if back, err := LoadScene(path); err != nil || !back.Equal(good) {
			t.Fatalf("file rep %d diverged: %v", i, err)
		}
	}
}

// F:离屏金对照窗(窗免,纯文件):文件字节来回一致,冻结数加形状断言.
func TestSceneOffscreenGolden(t *testing.T) {
	c := loadSceneCases(t)
	// Filed bytes survive Load+Encode byte-identical: this is the
	// game_world--case=open offscreen evidence (pure file package, no
	// live window required).
	for _, f := range []struct{ file, parse string }{
		{c.Prefab.File, "prefab"},
		{c.Scene.File, "scene"},
	} {
		raw, err := os.ReadFile(filepath.Join("testdata", f.file))
		if err != nil {
			t.Fatalf("read %s: %v", f.file, err)
		}
		var back []byte
		if f.parse == "prefab" {
			pf, err := ParsePrefab(raw)
			if err != nil {
				t.Fatalf("parse %s: %v", f.file, err)
			}
			back = mustPrefabEncode(t, f.file, &pf)
		} else {
			sf, err := ParseScene(raw)
			if err != nil {
				t.Fatalf("parse %s: %v", f.file, err)
			}
			back = mustSceneEncode(t, f.file, &sf)
		}
		if string(back) != string(raw) {
			t.Errorf("%s: re-encoded %dB differs from filed %dB, want byte-identical", f.file, len(back), len(raw))
		}
	}
	// Golden pins the anchors through the file, not the code.
	pf := mustLoadPrefab(t, "golden prefab", c.Prefab.File)
	if pf.Name() != c.Prefab.Name || pf.Version().String() != c.PrefabVersion {
		t.Errorf("prefab anchor = %q/%v, want %q/%s", pf.Name(), pf.Version(), c.Prefab.Name, c.PrefabVersion)
	}
	sf := mustLoadScene(t, "golden scene", c.Scene.File)
	sc, ids := mustOpenScene(t, "golden", &sf)
	for _, want := range c.Scene.Want {
		checkSceneWant(t, "golden", &sc, ids, want)
	}
	// Shape: names distinct, parents filed before children, kinds
	// non-empty, every number inside its frozen budget.
	seen := map[string]bool{}
	for _, n := range sf.Entities() {
		if seen[n.Name] {
			t.Errorf("duplicate filed name %q", n.Name)
		}
		seen[n.Name] = true
		if n.Parent != "" && !seen[n.Parent] {
			t.Errorf("%s parent %q not filed before it", n.Name, n.Parent)
		}
		if len(n.Name) > MaxSceneNameLen {
			t.Errorf("%s name exceeds %dB", n.Name, MaxSceneNameLen)
		}
		if len(n.Comps) > MaxSceneComps {
			t.Errorf("%s comps exceed %d", n.Name, MaxSceneComps)
		}
		for _, cp := range n.Comps {
			if cp.Kind == "" {
				t.Errorf("%s carries an empty comp kind", n.Name)
			}
		}
	}
	// Shape: version and count frozen; slime hangs under the scaled
	// crate, so its world is the crate world plus the scaled offset.
	if sf.Name() != c.Scene.Name || sf.Count() != c.Scene.Count {
		t.Errorf("scene anchor = %q/%d, want %q/%d", sf.Name(), sf.Count(), c.Scene.Name, c.Scene.Count)
	}
	crate, _ := sc.World().WorldOf(ids["crate"])
	slime, _ := sc.World().WorldOf(ids["slime"])
	sfSlime := sf.Entities()[3]
	if sfSlime.Name != "slime" {
		t.Fatalf("filed order moved, want slime last, got %q", sfSlime.Name)
	}
	wantSlimeX := crate.Pos.X + crate.Scale.X*sfSlime.Local.Pos.X
	wantSlimeY := crate.Pos.Y + crate.Scale.Y*sfSlime.Local.Pos.Y
	if !near(slime.Pos.X, wantSlimeX) || !near(slime.Pos.Y, wantSlimeY) {
		t.Errorf("slime world = %v, want crate %v plus scaled offset", slime.Pos, crate.Pos)
	}
	// Shape: every stored number stays inside its frozen budget, and the
	// filed bytes stay inside the file budget.
	for _, f := range []string{c.Prefab.File, c.Scene.File} {
		raw, _ := os.ReadFile(filepath.Join("testdata", f))
		if len(raw) > MaxSceneBytes {
			t.Errorf("%s %dB exceeds MaxSceneBytes", f, len(raw))
		}
	}
}
