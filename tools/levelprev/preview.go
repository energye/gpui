package main

import (
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
	"github.com/energye/gpui/game/tilemap"
	"github.com/energye/gpui/game/world"
)

// Preview is one opened level project: engine counts plus placeholders.
// The map and scene sides parse independently; one torn file never kills
// the other side. Map/side absence is a note, corruption is a *core.Error
// string in MapErr/SceneErr plus the returned error. Counts always come
// from engine getters, so they match the game bit for bit.
type Preview struct {
	Dir      string
	HasMap   bool
	HasScene bool

	Orient     string
	MapW       int
	MapH       int
	TileW      float64
	TileH      float64
	LayerCount int
	TileCount  int
	SolidCount int
	MaskAt00   int
	HasMask    bool

	ObjectCount int
	EntityCount int

	Notes    []string
	MapErr   string
	SceneErr string

	m  tilemap.Tilemap
	sf world.SceneFile
}

func orientName(m tilemap.Tilemap) string {
	if m.Orient() == tilemap.OrientIsometric {
		return "isometric"
	}
	return "orthogonal"
}

// Tilemap returns the parsed map and whether a map file was usable.
func (p *Preview) Tilemap() (tilemap.Tilemap, bool) {
	if p == nil || !p.HasMap {
		return tilemap.Tilemap{}, false
	}
	return p.m, true
}

// Scene returns the parsed scene file and whether one was usable.
func (p *Preview) Scene() (world.SceneFile, bool) {
	if p == nil || !p.HasScene {
		return world.SceneFile{}, false
	}
	return p.sf, true
}

// SummarizeMap folds one parsed map into preview counts using engine
// getters only: nonzero GIDs over every layer, solid cells via
// IsSolidAt, and the layer-0 automask at cell (0,0) as the road-edge
// sample. It never reads files.
func SummarizeMap(m tilemap.Tilemap) Preview {
	p := Preview{
		HasMap:     true,
		Orient:     orientName(m),
		MapW:       m.W(),
		MapH:       m.H(),
		TileW:      m.TileW(),
		TileH:      m.TileH(),
		LayerCount: len(m.Layers()),
		m:          m,
	}
	for _, l := range m.Layers() {
		for _, g := range l.GIDs() {
			if g != 0 {
				p.TileCount++
			}
		}
	}
	for r := 0; r < m.H(); r++ {
		for c := 0; c < m.W(); c++ {
			if m.IsSolidAt(c, r) {
				p.SolidCount++
			}
		}
	}
	if len(m.Layers()) > 0 {
		if mask, ok := m.AutoMaskAt(0, 0, 0); ok {
			p.MaskAt00, p.HasMask = mask, true
		}
	}
	p.ObjectCount = len(m.Objects())
	return p
}

// SummarizeScene folds one parsed scene file into preview counts using
// engine getters only. It never reads files.
func SummarizeScene(sf world.SceneFile) Preview {
	return Preview{HasScene: true, EntityCount: sf.Count(), sf: sf}
}

// VerifyCounts reports whether got carries exactly the engine numbers
// from a fresh direct parse of the same files. Zero difference passes;
// anything else is a plain mismatch error.
func VerifyCounts(got Preview, m tilemap.Tilemap, hasMap bool, sf world.SceneFile, hasScene bool) error {
	const op = "levelprev.VerifyCounts"
	if hasMap {
		want := SummarizeMap(m)
		if got.TileCount != want.TileCount || got.SolidCount != want.SolidCount ||
			got.ObjectCount != want.ObjectCount || got.LayerCount != want.LayerCount ||
			got.MapW != want.MapW || got.MapH != want.MapH ||
			got.MaskAt00 != want.MaskAt00 || got.HasMask != want.HasMask {
			return core.BadData(op, "map-counts")
		}
	} else if got.HasMap {
		return core.BadData(op, "map-flag")
	}
	if hasScene {
		if got.EntityCount != sf.Count() {
			return core.BadData(op, "scene-counts")
		}
	} else if got.HasScene {
		return core.BadData(op, "scene-flag")
	}
	return nil
}

// OpenProject opens dir as a level project. Missing map.tmx or
// scene.json leaves a placeholder note and a nil error; torn files set
// MapErr/SceneErr, keep the other side, and return the first core error.
// Empty projects open clean so the window never needs a crash path.
func OpenProject(dir string) (Preview, error) {
	const op = "levelprev.OpenProject"
	if dir == "" {
		return Preview{}, core.InvalidArg(op, "dir")
	}
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return Preview{}, core.NotFound(op, dir)
	}
	p := Preview{Dir: dir}
	var first error

	mapRaw, err := os.ReadFile(filepath.Join(dir, "map.tmx"))
	if err != nil {
		p.Notes = append(p.Notes, "map.tmx 缺:空白地图占位")
	} else if m, perr := tilemap.ParseTMX(mapRaw); perr != nil {
		p.Notes = append(p.Notes, "map.tmx 坏:地图占位,看 MapErr")
		p.MapErr = perr.Error()
		first = perr
	} else {
		mp := SummarizeMap(m)
		mp.Dir, mp.Notes = dir, p.Notes
		p = mp
	}

	sceneRaw, err := os.ReadFile(filepath.Join(dir, "scene.json"))
	if err != nil {
		p.Notes = append(p.Notes, "scene.json 缺:空白场景占位")
	} else if sf, perr := world.ParseScene(sceneRaw); perr != nil {
		p.Notes = append(p.Notes, "scene.json 坏:场景占位,看 SceneErr")
		p.SceneErr = perr.Error()
		if first == nil {
			first = perr
		}
	} else {
		p.HasScene = true
		p.EntityCount = sf.Count()
		p.sf = sf
	}
	return p, first
}
