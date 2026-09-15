package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/tilemap"
	"github.com/energye/gpui/game/world"
)

type levelCase struct {
	Project  string `json:"project"`
	Tiles    int    `json:"tiles"`
	Layers   int    `json:"layers"`
	Solids   int    `json:"solids"`
	Objects  int    `json:"objects"`
	Entities int    `json:"entities"`
	Orient   string `json:"orient"`
	Mask00   int    `json:"mask00"`
	HasMask  bool   `json:"has_mask"`
}

func mustReadLevelCases(t *testing.T) map[string]levelCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/preview_cases.json")
	if err != nil {
		t.Fatalf("read preview_cases.json: %v", err)
	}
	var m map[string]levelCase
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode preview_cases.json: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("preview_cases.json has no cases")
	}
	return m
}

// A: small project numbers match the frozen file data.
func TestLevelPreview_SmallCounts(t *testing.T) {
	cases := mustReadLevelCases(t)
	c := cases["proj_small"]
	p, err := OpenProject(c.Project)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if !p.HasMap || !p.HasScene {
		t.Fatalf("want both sides, got map=%v scene=%v notes=%v", p.HasMap, p.HasScene, p.Notes)
	}
	if p.TileCount != c.Tiles || p.LayerCount != c.Layers || p.SolidCount != c.Solids ||
		p.ObjectCount != c.Objects || p.EntityCount != c.Entities ||
		p.Orient != c.Orient || p.MaskAt00 != c.Mask00 || p.HasMask != c.HasMask {
		t.Fatalf("got tiles=%d layers=%d solids=%d objects=%d entities=%d orient=%s mask=%d/%v want %+v",
			p.TileCount, p.LayerCount, p.SolidCount, p.ObjectCount, p.EntityCount, p.Orient, p.MaskAt00, p.HasMask, c)
	}
}

// A: big project numbers match the frozen file data (open cost measured separately).
func TestLevelPreview_BigCounts(t *testing.T) {
	cases := mustReadLevelCases(t)
	c := cases["proj_big"]
	p, err := OpenProject(c.Project)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if p.TileCount != c.Tiles || p.LayerCount != c.Layers || p.SolidCount != c.Solids ||
		p.ObjectCount != c.Objects || p.EntityCount != c.Entities || p.Orient != c.Orient {
		t.Fatalf("got tiles=%d layers=%d solids=%d objects=%d entities=%d orient=%s want %+v",
			p.TileCount, p.LayerCount, p.SolidCount, p.ObjectCount, p.EntityCount, p.Orient, c)
	}
}

// B: empty project opens clean with placeholders, never an error.
func TestLevelPreview_EmptyProject(t *testing.T) {
	p, err := OpenProject("testdata/proj_empty")
	if err != nil {
		t.Fatalf("empty open: %v", err)
	}
	if p.HasMap || p.HasScene {
		t.Fatalf("empty must have no sides, got map=%v scene=%v", p.HasMap, p.HasScene)
	}
	if len(p.Notes) != 2 {
		t.Fatalf("empty must carry 2 notes, got %v", p.Notes)
	}
	if _, ok := p.Tilemap(); ok {
		t.Fatal("empty Tilemap must report false")
	}
	if _, ok := p.Scene(); ok {
		t.Fatal("empty Scene must report false")
	}
}

// B: torn files keep the good side and report core errors, never crash.
func TestLevelPreview_BadProjects(t *testing.T) {
	pb, err := OpenProject("testdata/proj_badmap")
	if err == nil {
		t.Fatal("bad map must return an error")
	}
	if pb.HasMap {
		t.Fatal("bad map side must be zeroed")
	}
	if pb.MapErr == "" || !pb.HasScene || pb.EntityCount != 3 {
		t.Fatalf("bad map must keep scene: %+v", pb)
	}
	if _, ok := pb.Tilemap(); ok {
		t.Fatal("bad map Tilemap must report false")
	}

	ps, err := OpenProject("testdata/proj_badscene")
	if err == nil {
		t.Fatal("bad scene must return an error")
	}
	if ps.HasScene {
		t.Fatal("bad scene side must be zeroed")
	}
	if ps.SceneErr == "" || !ps.HasMap || ps.TileCount != 17 {
		t.Fatalf("bad scene must keep map: %+v", ps)
	}

	if _, err := OpenProject(""); err == nil {
		t.Fatal("empty dir must be InvalidArg")
	} else if ce, ok := err.(*core.Error); !ok || ce.Code != core.CodeInvalidArg {
		t.Fatalf("empty dir code = %v, want InvalidArg", err)
	}
	if _, err := OpenProject("testdata/no_such_dir"); err == nil {
		t.Fatal("missing dir must be NotFound")
	} else if ce, ok := err.(*core.Error); !ok || ce.Code != core.CodeNotFound {
		t.Fatalf("missing dir code = %v, want NotFound", err)
	}
}

// C: preview counts equal a direct engine parse of the same files.
func TestLevelPreview_ParityWithEngine(t *testing.T) {
	for _, dir := range []string{"testdata/proj_small", "testdata/proj_big"} {
		p, err := OpenProject(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "map.tmx"))
		if err != nil {
			t.Fatalf("%s map: %v", dir, err)
		}
		m, err := tilemap.ParseTMX(raw)
		if err != nil {
			t.Fatalf("%s parse: %v", dir, err)
		}
		sraw, err := os.ReadFile(filepath.Join(dir, "scene.json"))
		if err != nil {
			t.Fatalf("%s scene: %v", dir, err)
		}
		sf, err := world.ParseScene(sraw)
		if err != nil {
			t.Fatalf("%s scene parse: %v", dir, err)
		}
		if err := VerifyCounts(p, m, true, sf, true); err != nil {
			t.Fatalf("%s parity: %v", dir, err)
		}
		mm, ok := p.Tilemap()
		if !ok || mm.W() != m.W() || mm.H() != m.H() {
			t.Fatalf("%s Tilemap getter mismatch", dir)
		}
		ss, ok := p.Scene()
		if !ok || ss.Count() != sf.Count() {
			t.Fatalf("%s Scene getter mismatch", dir)
		}
		if got := SummarizeScene(sf); got.EntityCount != sf.Count() || !got.HasScene {
			t.Fatalf("%s SummarizeScene mismatch", dir)
		}
	}
}

// D+E: opening the big project has a measured cost and reopens clean.
func TestLevelPreview_BigOpenCost(t *testing.T) {
	p0, err := OpenProject("testdata/proj_big")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < 5; i++ {
		p, err := OpenProject("testdata/proj_big")
		if err != nil {
			t.Fatalf("reopen %d: %v", i, err)
		}
		if p.TileCount != p0.TileCount || p.EntityCount != p0.EntityCount ||
			p.SolidCount != p0.SolidCount || p.ObjectCount != p0.ObjectCount {
			t.Fatalf("reopen %d drifted: %+v vs %+v", i, p, p0)
		}
	}
	t.Logf("big open: tiles=%d entities=%d (wall time measured by the window run, see delivery)", p0.TileCount, p0.EntityCount)
}
