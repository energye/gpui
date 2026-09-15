package save

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/energye/gpui/game/core"
)

// CurrentVersion is the engine save version every Store writes.
// Bump Minor for additive fields, Major for a breaking shape.
var CurrentVersion = core.Version{Major: 1, Minor: 0}

// Frozen budgets. A well-formed file beyond budget is OutOfMemory,
// never a guessed load. A bad value is InvalidArg via constructors
// and BadData via Parse.
const (
	// MaxSlots caps the slot ledger: slots are 0..MaxSlots-1.
	MaxSlots = 3
	// MaxInventory caps item entries in one save.
	MaxInventory = 256
	// MaxItemCount caps one stack: counts run 1..MaxItemCount.
	MaxItemCount = 9999
	// MaxStars caps starred levels in one save.
	MaxStars = 64
	// MaxStarsPerLevel caps one level: stars run 0..3, 0 deletes.
	MaxStarsPerLevel = 3
	// MaxNameLen caps level, checkpoint, item, and star keys.
	MaxNameLen = 64
	// MaxBytes caps one save file.
	MaxBytes = 1 << 20
)

// Item is one backpack entry: id plus count 1..MaxItemCount.
// Name is only a debug key for tests and logs; counts never go negative.
type Item struct {
	id    string
	count int
}

// NewItem builds one entry. Empty or overlong ids and counts outside
// 1..MaxItemCount are a core InvalidArg error and store nothing.
func NewItem(id string, count int) (Item, error) {
	if !validID(id) {
		return Item{}, core.InvalidArg("save.NewItem", "id")
	}
	if err := checkCount("save.NewItem", "count", count); err != nil {
		return Item{}, err
	}
	return Item{id: id, count: count}, nil
}

// ID returns the item key.
func (it Item) ID() string { return it.id }

// Count returns the stack count.
func (it Item) Count() int { return it.count }

// Progress is one 进度 snapshot: where the run sits plus how long it ran.
// Empty level/checkpoint means a new game; play time is integer
// milliseconds so slow and fast machines agree.
type Progress struct {
	level      string
	checkpoint string
	play       core.Duration
}

// NewProgress builds a snapshot. Overlong names and negative play are a
// core InvalidArg error and store nothing. Empty names stay legal.
func NewProgress(level, checkpoint string, play core.Duration) (Progress, error) {
	if !validProgressField(level) || !validProgressField(checkpoint) {
		return Progress{}, core.InvalidArg("save.NewProgress", "level")
	}
	if err := checkPlay("save.NewProgress", play); err != nil {
		return Progress{}, err
	}
	return Progress{level: level, checkpoint: checkpoint, play: play}, nil
}

// Level returns the stage name (empty means new game).
func (p Progress) Level() string { return p.level }

// Checkpoint returns the resume point (empty means none).
func (p Progress) Checkpoint() string { return p.checkpoint }

// Play returns the played time.
func (p Progress) Play() core.Duration { return p.play }

// PlayMs returns the played milliseconds.
func (p Progress) PlayMs() int64 { return p.play.Milliseconds() }

// Save is one slot file: slot plus version plus progress plus backpack
// plus 关卡星. Fields stay private so every write passes validation;
// readers use the accessors below, which copy.
type Save struct {
	slot      int
	version   core.Version
	progress  Progress
	inventory []Item
	stars     map[string]int
}

func validID(s string) bool { return nameOK(s, false) }

func validProgressField(s string) bool { return nameOK(s, true) }

// nameOK pins the shared name rule: non-empty ids (items, star levels)
// versus possibly-empty progress fields, both capped at MaxNameLen.
// Byte length is by design; saves store UTF-8 bytes and the budget
// counts bytes.
func nameOK(s string, allowEmpty bool) bool {
	if !allowEmpty && s == "" {
		return false
	}
	return len(s) <= MaxNameLen
}

func validSlot(slot int) bool { return slot >= 0 && slot < MaxSlots }

// loadableVersion reports whether Parse accepts v and Migrate upgrades
// it: legacy 0.x by design plus anything CompatibleWith CurrentVersion.
// Foreign majors and newer minors stay rejected.
func loadableVersion(v core.Version) bool { return v.Major == 0 || v.CompatibleWith(CurrentVersion) }

func cloneItems(in []Item) []Item {
	out := make([]Item, len(in))
	copy(out, in)
	return out
}

func cloneStars(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// New builds an empty save on slot with CurrentVersion.
// Slots outside 0..MaxSlots-1 are a core InvalidArg error.
func New(slot int) (Save, error) {
	if !validSlot(slot) {
		return Save{}, core.InvalidArg("save.New", "slot")
	}
	return Save{
		slot:      slot,
		version:   CurrentVersion,
		inventory: []Item{},
		stars:     map[string]int{},
	}, nil
}

// Slot returns the slot number.
func (s Save) Slot() int { return s.slot }

// Version returns the stored version.
func (s Save) Version() core.Version { return s.version }

// Progress returns the progress snapshot.
func (s Save) Progress() Progress { return s.progress }

// Inventory returns a copy of the backpack in stored order.
// Writing the result cannot change the save.
func (s Save) Inventory() []Item { return cloneItems(s.inventory) }

// Stars returns a copy of the level stars. Writing it cannot change
// the save; a nil or empty save reports an empty non-nil map.
func (s Save) Stars() map[string]int { return cloneStars(s.stars) }

// StarsOf returns the stars of level. Missing levels report ok=false.
func (s Save) StarsOf(level string) (int, bool) {
	if s.stars == nil {
		return 0, false
	}
	v, ok := s.stars[level]
	return v, ok
}

// SetProgress replaces the snapshot. Bad names or negative play are a
// core InvalidArg error and change nothing. Nil saves report InvalidArg.
func (s *Save) SetProgress(p Progress) error {
	if s == nil {
		return core.InvalidArg("save.SetProgress", "save")
	}
	if !validProgressField(p.level) || !validProgressField(p.checkpoint) {
		return core.InvalidArg("save.SetProgress", "level")
	}
	if err := checkPlay("save.SetProgress", p.play); err != nil {
		return err
	}
	s.progress = p
	return nil
}

// AddItem adds count to id, merging when the id already exists.
// New ids beyond MaxInventory are OutOfMemory; bad ids, bad counts,
// and merges beyond MaxItemCount are InvalidArg. Failures change nothing.
func (s *Save) AddItem(id string, count int) error {
	if s == nil {
		return core.InvalidArg("save.AddItem", "save")
	}
	if !validID(id) {
		return core.InvalidArg("save.AddItem", "id")
	}
	if err := checkCount("save.AddItem", "count", count); err != nil {
		return err
	}
	for i, it := range s.inventory {
		if it.id == id {
			if it.count+count > MaxItemCount {
				return core.InvalidArg("save.AddItem", "count")
			}
			s.inventory[i].count += count
			return nil
		}
	}
	if len(s.inventory) >= MaxInventory {
		return core.OutOfMemory("save.AddItem", "inventory")
	}
	if s.inventory == nil {
		s.inventory = []Item{}
	}
	s.inventory = append(s.inventory, Item{id: id, count: count})
	return nil
}

// SetStars records stars for level (0 deletes the entry).
// Bad levels and stars outside 0..MaxStarsPerLevel are InvalidArg;
// a new level beyond MaxStars is OutOfMemory. Failures change nothing.
func (s *Save) SetStars(level string, stars int) error {
	if s == nil {
		return core.InvalidArg("save.SetStars", "save")
	}
	if !validID(level) {
		return core.InvalidArg("save.SetStars", "level")
	}
	if err := checkStarsValue("save.SetStars", stars); err != nil {
		return err
	}
	if s.stars == nil {
		s.stars = map[string]int{}
	}
	if stars == 0 {
		delete(s.stars, level)
		return nil
	}
	if _, ok := s.stars[level]; !ok && len(s.stars) >= MaxStars {
		return core.OutOfMemory("save.SetStars", "stars")
	}
	s.stars[level] = stars
	return nil
}

func itemsEqual(a, b []Item) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func starsMapEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if ov, ok := b[k]; !ok || ov != v {
			return false
		}
	}
	return true
}

// Equal reports whether o holds the same slot, version, progress,
// inventory order, and stars. Nil and empty inventories compare equal
// when both hold no entries; same for stars.
func (s Save) Equal(o Save) bool {
	if s.slot != o.slot || s.version != o.version {
		return false
	}
	if s.progress != o.progress {
		return false
	}
	return itemsEqual(s.inventory, o.inventory) && starsMapEqual(s.stars, o.stars)
}

func checkCount(op, what string, count int) error {
	if count <= 0 || count > MaxItemCount {
		return core.InvalidArg(op, what)
	}
	return nil
}

func checkStarsValue(op string, stars int) error {
	if stars < 0 || stars > MaxStarsPerLevel {
		return core.InvalidArg(op, "stars")
	}
	return nil
}

func checkPlay(op string, play core.Duration) error {
	if play < 0 {
		return core.InvalidArg(op, "play")
	}
	return nil
}

func checkSlot(op string, slot int) error {
	if !validSlot(slot) {
		return core.InvalidArg(op, "slot")
	}
	return nil
}

func (s Save) clone() Save {
	inv := cloneItems(s.inventory)
	st := cloneStars(s.stars)
	return Save{slot: s.slot, version: s.version, progress: s.progress, inventory: inv, stars: st}
}

// checkShape validates an in-memory save for Encode/Store/Migrate:
// bad values are InvalidArg, budget overruns are OutOfMemory,
// duplicate ids are InvalidArg. Version is not checked here.
func (s Save) checkShape(op string) error {
	if err := checkSlot(op, s.slot); err != nil {
		return err
	}
	if !validProgressField(s.progress.level) || !validProgressField(s.progress.checkpoint) {
		return core.InvalidArg(op, "level")
	}
	if err := checkPlay(op, s.progress.play); err != nil {
		return err
	}
	if len(s.inventory) > MaxInventory {
		return core.OutOfMemory(op, "inventory")
	}
	seen := make(map[string]bool, len(s.inventory))
	for _, it := range s.inventory {
		if !validID(it.id) {
			return core.InvalidArg(op, "id")
		}
		if err := checkCount(op, "count", it.count); err != nil {
			return err
		}
		if seen[it.id] {
			return core.InvalidArg(op, "id")
		}
		seen[it.id] = true
	}
	if len(s.stars) > MaxStars {
		return core.OutOfMemory(op, "stars")
	}
	for k, v := range s.stars {
		if !validID(k) {
			return core.InvalidArg(op, "stars")
		}
		if v <= 0 || v > MaxStarsPerLevel {
			return core.InvalidArg(op, "stars")
		}
	}
	return nil
}

func checkItemEntry(op string, id string, count int, seen map[string]bool) (Item, error) {
	if !validID(id) {
		return Item{}, core.BadData(op, "id")
	}
	if count <= 0 || count > MaxItemCount {
		return Item{}, core.BadData(op, "count")
	}
	if seen[id] {
		return Item{}, core.BadData(op, "id")
	}
	seen[id] = true
	return Item{id: id, count: count}, nil
}

func checkStarEntry(op string, level string, stars int) error {
	if !validID(level) {
		return core.BadData(op, "stars")
	}
	if stars <= 0 || stars > MaxStarsPerLevel {
		return core.BadData(op, "stars")
	}
	return nil
}

func checkSlotValue(op string, slot int) error {
	if !validSlot(slot) {
		return core.BadData(op, "slot")
	}
	return nil
}

type jsonProgress struct {
	Level      string `json:"level"`
	Checkpoint string `json:"checkpoint"`
	PlayMs     int64  `json:"play_ms"`
}

type jsonItem struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

type jsonSave struct {
	Version   string         `json:"version"`
	Slot      *int           `json:"slot"`
	Progress  *jsonProgress  `json:"progress"`
	Inventory []jsonItem     `json:"inventory"`
	Stars     map[string]int `json:"stars"`
}

// Encode renders the canonical file bytes. The save must already carry
// CurrentVersion (legacy saves Migrate first); otherwise the result is a
// core VersionMismatch error. Bad shapes are InvalidArg, budget overruns
// are OutOfMemory. The input is never mutated.
func (s Save) Encode() ([]byte, error) {
	const op = "save.Encode"
	if err := s.checkShape(op); err != nil {
		return nil, err
	}
	if s.version != CurrentVersion {
		return nil, core.VersionMismatch(op, s.version.String())
	}
	inv := s.inventory
	if inv == nil {
		inv = []Item{}
	}
	st := s.stars
	if st == nil {
		st = map[string]int{}
	}
	items := make([]jsonItem, len(inv))
	for i, it := range inv {
		items[i] = jsonItem{ID: it.id, Count: it.count}
	}
	raw := jsonSave{
		Version:   s.version.String(),
		Slot:      &s.slot,
		Progress:  &jsonProgress{Level: s.progress.level, Checkpoint: s.progress.checkpoint, PlayMs: s.progress.play.Milliseconds()},
		Inventory: items,
		Stars:     st,
	}
	out, err := json.Marshal(raw)
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	if len(out) > MaxBytes {
		return nil, core.OutOfMemory(op, "bytes")
	}
	return out, nil
}

// Store writes the save to path, creating parent directories.
// Empty paths are InvalidArg; OS failures are NotFound with the cause;
// save-shape errors match Encode.
func (s Save) Store(path string) error {
	const op = "save.Store"
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := s.Encode()
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

// Parse validates file bytes into a Save. Empty input is InvalidArg,
// OS-size overruns are OutOfMemory, torn JSON and bad shapes are BadData,
// newer or foreign versions are VersionMismatch. Legacy 0.x files parse
// fine so the caller can Migrate them; missing progress/inventory/stars
// default to empty.
func Parse(data []byte) (Save, error) {
	const op = "save.Parse"
	if len(data) == 0 {
		return Save{}, core.InvalidArg(op, "data")
	}
	if len(data) > MaxBytes {
		return Save{}, core.OutOfMemory(op, "bytes")
	}
	var raw jsonSave
	if err := json.Unmarshal(data, &raw); err != nil {
		return Save{}, core.BadData(op, "json")
	}
	if raw.Version == "" {
		return Save{}, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(raw.Version)
	if err != nil {
		return Save{}, core.BadData(op, "version")
	}
	if !loadableVersion(ver) {
		return Save{}, core.VersionMismatch(op, ver.String())
	}
	if raw.Slot == nil {
		return Save{}, core.BadData(op, "slot")
	}
	if err := checkSlotValue(op, *raw.Slot); err != nil {
		return Save{}, err
	}
	prog := Progress{}
	if raw.Progress != nil {
		if !validProgressField(raw.Progress.Level) || !validProgressField(raw.Progress.Checkpoint) {
			return Save{}, core.BadData(op, "level")
		}
		if raw.Progress.PlayMs < 0 {
			return Save{}, core.BadData(op, "play")
		}
		prog = Progress{level: raw.Progress.Level, checkpoint: raw.Progress.Checkpoint, play: core.Milliseconds(raw.Progress.PlayMs)}
	}
	if len(raw.Inventory) > MaxInventory {
		return Save{}, core.OutOfMemory(op, "inventory")
	}
	inv := make([]Item, 0, len(raw.Inventory))
	seen := make(map[string]bool, len(raw.Inventory))
	for _, it := range raw.Inventory {
		entry, err := checkItemEntry(op, it.ID, it.Count, seen)
		if err != nil {
			return Save{}, err
		}
		inv = append(inv, entry)
	}
	if raw.Stars != nil && len(raw.Stars) > MaxStars {
		return Save{}, core.OutOfMemory(op, "stars")
	}
	st := make(map[string]int, len(raw.Stars))
	for k, v := range raw.Stars {
		if err := checkStarEntry(op, k, v); err != nil {
			return Save{}, err
		}
		st[k] = v
	}
	return Save{slot: *raw.Slot, version: ver, progress: prog, inventory: inv, stars: st}, nil
}

// Load reads path as a save file. Empty paths are InvalidArg, missing
// files are NotFound; the rest matches Parse.
func Load(path string) (Save, error) {
	const op = "save.Load"
	if path == "" {
		return Save{}, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Save{}, core.NotFound(op, path, err)
	}
	return Parse(raw)
}

// Migrate upgrades s to CurrentVersion, preserving slot, progress,
// backpack, and stars. Current files return a copy; loadable older
// versions bump to Current. Foreign majors and newer minors are a core
// VersionMismatch error; torn shapes are InvalidArg and budget overruns
// are OutOfMemory. The input is never mutated.
func Migrate(s Save) (Save, error) {
	const op = "save.Migrate"
	if err := s.checkShape(op); err != nil {
		return Save{}, err
	}
	if s.version == CurrentVersion {
		return s.clone(), nil
	}
	if loadableVersion(s.version) {
		out := s.clone()
		out.version = CurrentVersion
		return out, nil
	}
	return Save{}, core.VersionMismatch(op, s.version.String())
}
