package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/energye/gpui/game/core"
)

type particleCase struct {
	Effect  string `json:"effect"`
	Spawned int    `json:"spawned"`
	Alive   int    `json:"alive"`
	Child   int    `json:"child"`
	Shape   string `json:"shape"`
}

func mustReadParticleCases(t *testing.T) map[string]particleCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/preview_cases.json")
	if err != nil {
		t.Fatalf("read preview_cases.json: %v", err)
	}
	var m map[string]particleCase
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode preview_cases.json: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("preview_cases.json has no cases")
	}
	return m
}

// A: filed fire/smoke recipes replay to the frozen engine numbers.
func TestParticlePreview_Effects(t *testing.T) {
	cases := mustReadParticleCases(t)
	for _, name := range []string{"fire", "smoke"} {
		c := cases[name]
		eff, err := OpenEffect("testdata", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !eff.HasFile || eff.ShapeKind != c.Shape {
			t.Fatalf("%s: has=%v shape=%s want %s", name, eff.HasFile, eff.ShapeKind, c.Shape)
		}
		if eff.Spawned != c.Spawned || eff.Alive != c.Alive || eff.Child != c.Child {
			t.Fatalf("%s: got %d/%d/%d want %d/%d/%d", name,
				eff.Spawned, eff.Alive, eff.Child, c.Spawned, c.Alive, c.Child)
		}
		if err := VerifyReplay(eff); err != nil {
			t.Fatalf("%s verify: %v", name, err)
		}
		em, err := eff.BuildEmitter()
		if err != nil {
			t.Fatalf("%s build: %v", name, err)
		}
		if em.Alive() != 0 {
			t.Fatalf("%s fresh emitter must start empty", name)
		}
		if eff.Config().Max <= 0 {
			t.Fatalf("%s config must be usable", name)
		}
	}
}

// B: missing effect is a clean placeholder; torn file reports BadData; never a crash.
func TestParticlePreview_BadEffects(t *testing.T) {
	miss, err := OpenEffect("testdata", "no_such_effect")
	if err != nil {
		t.Fatalf("missing must open clean: %v", err)
	}
	if miss.HasFile || len(miss.Notes) != 1 {
		t.Fatalf("missing must be placeholder: %+v", miss)
	}
	if _, err := miss.BuildEmitter(); err == nil {
		t.Fatal("placeholder BuildEmitter must fail")
	}
	if err := VerifyReplay(miss); err == nil {
		t.Fatal("placeholder VerifyReplay must fail")
	}

	bad, err := OpenEffect("testdata", "bad")
	if err == nil {
		t.Fatal("torn file must return an error")
	}
	if bad.HasFile || bad.FileErr == "" {
		t.Fatalf("torn must be zeroed with FileErr: %+v", bad)
	}
	if ce, ok := err.(*core.Error); !ok || ce.Code != core.CodeBadData {
		t.Fatalf("torn code = %v, want BadData", err)
	}

	if _, err := ParseEffect(nil); err == nil {
		t.Fatal("empty bytes must be InvalidArg")
	} else if ce, ok := err.(*core.Error); !ok || ce.Code != core.CodeInvalidArg {
		t.Fatalf("empty bytes code = %v, want InvalidArg", err)
	}
	if _, err := OpenEffect("", "fire"); err == nil {
		t.Fatal("empty dir must be InvalidArg")
	}
	if _, err := OpenEffect("testdata/no_such_dir", "fire"); err == nil {
		t.Fatal("missing dir must be NotFound")
	} else if ce, ok := err.(*core.Error); !ok || ce.Code != core.CodeNotFound {
		t.Fatalf("missing dir code = %v, want NotFound", err)
	}
}

// C: the preview replay equals a fresh engine replay of the same file.
func TestParticlePreview_ParityWithEngine(t *testing.T) {
	for _, name := range []string{"fire", "smoke"} {
		eff, err := OpenEffect("testdata", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		s, a, c, err := ReplayEffect(&eff)
		if err != nil {
			t.Fatalf("%s replay: %v", name, err)
		}
		if s != eff.Spawned || a != eff.Alive || c != eff.Child {
			t.Fatalf("%s drifted: replay %d/%d/%d vs filed %d/%d/%d",
				name, s, a, c, eff.Spawned, eff.Alive, eff.Child)
		}
	}
}

// D+E: repeated effect opens replay identically (no growth, no drift).
func TestParticlePreview_ReopenStable(t *testing.T) {
	first, err := OpenEffect("testdata", "fire")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < 20; i++ {
		eff, err := OpenEffect("testdata", "fire")
		if err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
		if eff.Spawned != first.Spawned || eff.Alive != first.Alive || eff.Child != first.Child {
			t.Fatalf("reopen %d drifted: %+v vs %+v", i, eff, first)
		}
	}
}
