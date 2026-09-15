// Package save carries the quality-tier math every 2.5D play follows.
//
// Frozen 2026-09-16 (capability 16.2, P4, S51/W9): Level, LevelHigh,
// LevelMedium, LevelLow, ParseLevel, Spec, SpecFor, Quality, NewQuality,
// ParseQuality, LoadQuality, Encode, StoreQuality, Level, Spec, Equal,
// Switch. Additive changes only.
//
// Three tiers, frozen: high maps to 1000 particles plus 8 lights plus
// scale 1.00, medium maps to 500 particles plus 4 lights plus scale 0.75,
// low maps to 200 particles plus 2 lights plus scale 0.50. High is the
// full-fidelity tier; medium halves particles and lights and renders at
// three quarters resolution; low keeps one fifth of the particles, a
// quarter of the lights, and renders at half resolution. The numbers
// live in game/save/testdata/quality_cases.json; the code only replays
// them, so tuning a tier means editing that file first.
//
// The caller builds a Quality with New or Parse, reads the frozen Spec
// with Spec, and changes tiers with Switch. Switching only replaces the
// level: the same scene keeps running, nothing reloads, nothing flashes.
// Parse levels by design: only version "1.0" files parse; empty input is
// InvalidArg, torn shapes are BadData, foreign or newer versions are
// VersionMismatch, oversized files are OutOfMemory. Constructor-style
// calls (New, ParseLevel) report bad tiers as InvalidArg; file Parse
// reports the same bad tier as BadData. Nil receivers report InvalidArg
// and change nothing. Only core numbers cross the boundary; render types
// are converted at the call site, never here.
//
// File (self JSON, frozen): version "1.0", quality "high"|"medium"|"low".
// Unknown fields are ignored so a newer minor file still decodes its
// tier; a missing quality is BadData, never a guessed tier.
package save

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
)

// QualityVersion is the quality file version every Encode writes.
// Bump Minor for additive fields, Major for a breaking shape.
var QualityVersion = core.Version{Major: 1, Minor: 0}

// MaxQualityBytes caps one quality file. Well-formed files beyond the
// budget are OutOfMemory, never a guessed load.
const MaxQualityBytes = 4096

// Level is one frozen quality tier: high, medium, or low.
type Level string

// Frozen tier names. Parse is case-sensitive: "HIGH" is rejected so a
// typo never silently lands on another tier.
const (
	LevelHigh   Level = "high"
	LevelMedium Level = "medium"
	LevelLow    Level = "low"
)

// String returns the tier name.
func (l Level) String() string { return string(l) }

// Valid reports whether l is one of the three frozen tiers.
func (l Level) Valid() bool {
	return l == LevelHigh || l == LevelMedium || l == LevelLow
}

// ParseLevel maps a tier name to its Level. Empty and unknown names are
// a core InvalidArg error.
func ParseLevel(name string) (Level, error) {
	l := Level(name)
	if !l.Valid() {
		return "", core.InvalidArg("save.ParseLevel", "level")
	}
	return l, nil
}

// Spec is the frozen per-tier budget the scene follows: how many
// particles to spawn, how many lights to evaluate, and which resolution
// ratio to render at. Fields stay values so copies never alias.
type Spec struct {
	// Particles caps live particles (high 1000, medium 500, low 200).
	Particles int
	// Lights caps evaluated lights (high 8, medium 4, low 2).
	Lights int
	// Scale is the render resolution ratio (high 1.00, medium 0.75, low 0.50).
	Scale float64
}

// qualitySpecs pins the frozen tier table. The testdata file owns the
// numbers; this table replays them so tuning starts from that file.
var qualitySpecs = map[Level]Spec{
	LevelHigh:   {Particles: 1000, Lights: 8, Scale: 1.0},
	LevelMedium: {Particles: 500, Lights: 4, Scale: 0.75},
	LevelLow:    {Particles: 200, Lights: 2, Scale: 0.5},
}

// SpecFor returns the frozen Spec of a tier. Unknown tiers are a core
// InvalidArg error.
func SpecFor(l Level) (Spec, error) {
	s, ok := qualitySpecs[l]
	if !ok {
		return Spec{}, core.InvalidArg("save.SpecFor", "level")
	}
	return s, nil
}

// Quality is the current tier handle: one level plus its frozen spec.
// The zero value holds no tier; New and Parse are the only ways in.
// Readers never fail on a valid handle; writers on nil report
// InvalidArg and change nothing.
type Quality struct {
	level Level
}

// New builds a handle on the given tier. Unknown tiers are a core
// InvalidArg error.
func NewQuality(level Level) (Quality, error) {
	if !level.Valid() {
		return Quality{}, core.InvalidArg("save.NewQuality", "level")
	}
	return Quality{level: level}, nil
}

// Level returns the current tier. A zero handle reports "".
func (q Quality) Level() Level { return q.level }

// Spec returns the frozen Spec of the current tier. A zero handle
// reports the zero Spec; valid handles never fail.
func (q Quality) Spec() Spec {
	if s, ok := qualitySpecs[q.level]; ok {
		return s
	}
	return Spec{}
}

// Equal reports whether o holds the same tier.
func (q Quality) Equal(o Quality) bool { return q.level == o.level }

// Switch moves the handle to another tier without touching anything
// else: the scene keeps running on the new Spec, nothing reloads.
// Switching to the current tier is a no-op success. Unknown tiers are
// a core InvalidArg error and change nothing. Nil handles report
// InvalidArg.
func (q *Quality) Switch(to Level) error {
	const op = "save.Quality.Switch"
	if q == nil {
		return core.InvalidArg(op, "quality")
	}
	if !to.Valid() {
		return core.InvalidArg(op, "level")
	}
	q.level = to
	return nil
}

type jsonQuality struct {
	Version string `json:"version"`
	Quality string `json:"quality"`
}

// Encode renders the canonical file bytes. A zero handle (no tier) is
// InvalidArg; budget overruns are OutOfMemory. The input is never
// mutated.
func (q Quality) Encode() ([]byte, error) {
	const op = "save.Quality.Encode"
	if !q.level.Valid() {
		return nil, core.InvalidArg(op, "level")
	}
	raw := jsonQuality{Version: QualityVersion.String(), Quality: string(q.level)}
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxQualityBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

// Parse validates file bytes into a Quality. Empty input is InvalidArg,
// size overruns are OutOfMemory, torn JSON and bad tiers are BadData,
// foreign or newer versions are VersionMismatch.
func ParseQuality(data []byte) (Quality, error) {
	const op = "save.ParseQuality"
	if len(data) == 0 {
		return Quality{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxQualityBytes {
		return Quality{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonQuality
	if err := json.Unmarshal(data, &raw); err != nil {
		return Quality{}, core.BadData(op, "json")
	}
	if raw.Version == "" {
		return Quality{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return Quality{}, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(QualityVersion) {
		return Quality{}, core.VersionMismatch(op, ver.String())
	}
	lvl := Level(raw.Quality)
	if !lvl.Valid() {
		return Quality{}, core.BadData(op, "quality")
	}
	return Quality{level: lvl}, nil
}

// StoreQuality writes q to path, creating parent directories.
// Empty paths are InvalidArg; OS failures are NotFound with the cause;
// tier errors match Encode.
func StoreQuality(q Quality, path string) error {
	const op = "save.StoreQuality"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := q.Encode()
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

// LoadQuality reads path as a quality file. Empty paths are InvalidArg,
// missing files are NotFound; the rest matches ParseQuality.
func LoadQuality(path string) (Quality, error) {
	const op = "save.LoadQuality"
	if path == "" {
		return Quality{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Quality{}, core.NotFound(op, path, err)
	}
	return ParseQuality(raw)
}
