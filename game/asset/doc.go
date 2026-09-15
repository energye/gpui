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
// Unsupported, never a guessed payload. Atlas packing (12.2) and file
// watching (12.3) live later under their own frozen names.
//
// Errors use game/core codes: empty id/path/data is InvalidArg, a
// missing id or file is NotFound with a usable placeholder, torn KTX2
// or hash-level mismatch is BadData, an unfrozen kind is Unsupported, a
// picture beyond budget or a manager beyond MaxAssets/MaxBytes is
// OutOfMemory, a payload newer than CurrentVersion is VersionMismatch.
// A nil Manager never panics: loads report InvalidArg, queries report
// zero values.
package asset
