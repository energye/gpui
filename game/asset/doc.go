// Package asset freezes the CPU-side asset ledger every 2.5D play feeds
// (capability 12.1, P2 long chain, S19/W2).
//
// Frozen 2026-09-15: Kind, ParseKind, State, Stats, Asset, Manager,
// NewManager, CurrentVersion, MaxAssets, MaxBytes, MaxAssetBytes, MaxDeps,
// MaxIDLen, HashBytes. Additive changes only.
//
// What is frozen: one opaque carrier plus one block format. KindRaw
// carries any bytes without parsing them (maps, bones, and saves ride
// here until their own capabilities freeze); KindTextureKTX2 carries a
// KTX2 file with VK_FORMAT_BC1_RGBA_UNORM_BLOCK (133) and decodes it
// through game/tex (frozen S02) so upload and pixel sizes are honest.
// The manager keeps one ledger: background Request plus Poll/Wait up
// front so play never stalls, Load/Ref/Unload counting through
// core.AssetID and core.Handle so textures, maps, and saves share one
// count, Dependencies/Dependents tracking who needs whom (a map that
// needs a tex knows to reload it), and Version plus Hash pairing each
// payload with its manifest line. Only core numbers are used; no new
// Vec2/Color/AssetID is defined here. The old render and tex paths stay
// untouched; tex is only read, never modified.
//
// What is reserved: every other kind name (basis, spine, tmx, atlas,
// png, ...) parses far enough to name the kind, then reports core
// Unsupported, never a guessed payload.
//
// Frozen 2026-09-15 (capability 12.3, P2, S28/W3): MaxWatches,
// MaxWatchIDLen, MaxWatchPathLen, PollInterval, HotState, HotStats,
// Watcher, NewWatcher. Additive changes only.
//
// Hot reload (12.3, file watch plus single-id swap): Watch pins one id
// to one file with the load shape Reload replays, Poll stats files and
// marks changed ids Dirty without loading, Reload re-reads through
// Manager.LoadFile and swaps only that id. Other ids keep their bytes
// and claims; a failed Reload keeps the last good snapshot readable and
// parks the watch at Failed with CauseOf set. Only core numbers are
// used; the manager and tex paths are only read, never modified.
//
// Frozen 2026-09-15 (capability 12.2, P2, S27/W3): CurrentAtlasVersion,
// MaxSprites, MaxAtlasSize, MaxAtlasNameLen, AtlasPad, MaxAtlasJSONBytes,
// Input, Entry, Atlas, Pack, Parse, Load, Encode, Save. Additive changes
// only.
//
// Atlas packing (12.2, offline tool plus game pack): small pictures are
// shelved into one sheet offline, then the game loads the frozen JSON.
// Frozen JSON is version, width, height, and sprites each holding name,
// x, y, w, h, pivotX, pivotY, and nine [left, top, right, bottom]. Pack
// sorts by height, width, then name so the same set always lands on the
// same rects and keeps AtlasPad transparent pixels around every sprite.
// Unfrozen JSON keys (rotated, trim, anchor, meta, ...) name the field
// then report core Unsupported, never a guessed placement. Parsed atlases
// carry placements only; packed atlases also carry the composed RGBA8
// sheet. Only core numbers are used (AssetID, Version); rects stay plain
// ints so no new Vec2 is defined here.
//
// Errors use game/core codes: empty id/path/data is InvalidArg, a
// missing id or file is NotFound with a usable placeholder, torn KTX2
// or hash-level mismatch is BadData, an unfrozen kind is Unsupported, a
// picture beyond budget or a manager beyond MaxAssets/MaxBytes is
// OutOfMemory, a payload newer than CurrentVersion is VersionMismatch.
// A nil Manager never panics: loads report InvalidArg, queries report
// zero values.
package asset
