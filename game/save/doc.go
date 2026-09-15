// Package save freezes the file-side progress math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 16.1, P0, S15/W1): CurrentVersion,
// MaxSlots, MaxInventory, MaxItemCount, MaxStars, MaxStarsPerLevel,
// MaxNameLen, MaxBytes, Item, NewItem, ID, Count, Progress, NewProgress,
// Level, Checkpoint, Play, PlayMs, Save, New, Slot, Version, Progress,
// Inventory, Stars, StarsOf, SetProgress, AddItem, SetStars, Equal,
// Encode, Store, Parse, Load, Migrate. Additive changes only.
//
// The package draws nothing and plays nothing; it only carries numbers
// between runs. The caller builds a Save with New plus SetProgress,
// AddItem, and SetStars, persists it with Store, reads it back with Load,
// and upgrades old files with Migrate. Only core numbers are used; no new
// Vec2/Color/AssetID is defined here. The old render and ui paths stay
// untouched.
//
// File (self JSON, frozen): version "1.0", slot, progress
// {level, checkpoint, play_ms}, inventory [{id, count}], stars
// {level: 0..3}. Unknown fields are ignored so a newer minor file still
// parses; missing progress/inventory/stars default to empty so a legacy
// file without stars still loads and Migrate fills the rest. One frozen
// shape first; a second shape freezes separately later.
// Parse levels by design: loadable past versions (legacy 0.x plus
// CompatibleWith minors) parse so the caller can Migrate them; foreign
// majors and newer minors are VersionMismatch, never a guessed load.
//
// Slots: 0..MaxSlots-1 (3 slots). Progress holds 进度: level and
// checkpoint are plain names (empty means new game), play_ms is integer
// milliseconds (core.Duration). Inventory holds 背包: id plus count
// 1..MaxItemCount, insertion order, duplicate ids never stored. Stars
// holds 关卡星: level to 0..3, 0 deletes the entry.
//
// Errors use game/core codes: empty input or path is InvalidArg, a
// missing file is NotFound, a torn shape is BadData, a well-formed file
// beyond the MaxBytes/MaxInventory/MaxStars budget is OutOfMemory, a
// version newer than the engine (or a foreign major) is VersionMismatch.
// Constructors report bad values as InvalidArg; Parse reports the same
// bad shape as BadData. Only core numbers cross the boundary.
package save
