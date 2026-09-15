// Package debug freezes the game-side performance and crash numbers.
//
// Frozen 2026-09-16 (capability 17.2, P4, S49/W8): CurrentVersion,
// MaxFrames, MaxSpansPerFrame, MaxShaders, MaxNameLen, MaxBytes,
// MaxMemoryBytes, MaxFrameDt, SlowFrameDt, Span, NewSpan, Frame,
// NewFrame, SpanStat, ShaderStat, Stats, NewStats, Report,
// ParseReport, StoreReport, LoadReport, CrashLog, ParseCrash,
// StoreCrash, LoadCrash. Additive changes only.
//
// The package only carries numbers; it never draws and never touches
// the render main path. The caller records one Frame per presented
// frame (frame time, draw calls, overdraw, VRAM sample, per-frame
// breakdown spans), accumulates shader compile costs with RecordShader,
// and samples VRAM between frames with NoteMemory. Report freezes the
// snapshot the long-run reporter and the crash log share: frame rate,
// draw calls, memory, breakdown, overdraw, shader time. Text renders
// the same snapshot as the human-readable report the F scenario reads.
// Only core numbers are used; no new Vec2/Color/AssetID is defined
// here. The old render and ui paths stay untouched.
//
// File (self JSON, frozen): version "1.0" plus the Report fields
// (frames, fps, avg/p95 ms, slow/gap/oom counters, draws, memory,
// overdraw, breakdown, shaders). Crash files wrap one Report with a
// reason and a core code name. Unknown fields are ignored so a newer
// minor file still parses; a foreign major or a newer minor is
// VersionMismatch, never a guessed load.
//
// Frames: Dt parks inside (0, MaxFrameDt]; a larger Dt (background gap,
// shrunk window) clamps to MaxFrameDt and counts one Gap instead of
// collapsing FPS. Dt above SlowFrameDt counts one Slow frame. Draws and
// memory never go negative; overdraw is a finite ratio >= 0 (1.0 means
// no overdraw). Spans per frame cap at MaxSpansPerFrame, distinct
// shaders at MaxShaders, retained frames at MaxFrames (oldest drops).
// VRAM samples above MaxMemoryBytes count one OOM and keep the old
// value: fail closed, never crash.
//
// Errors use game/core codes: empty input or path is InvalidArg, a
// missing file is NotFound, a torn shape is BadData, a well-formed file
// beyond the MaxBytes budget is OutOfMemory, a version newer than the
// engine (or a foreign major) is VersionMismatch. Bad Record/Note
// arguments are InvalidArg (OOM budget excluded, see above) and change
// nothing; a nil or zero Stats answers every reader with zeros and
// reports InvalidArg on every writer. Only core numbers cross the
// boundary.
//
// C pictorial parity is not applicable: this package draws nothing, so
// there is no GPU/CPU picture pair to compare. The C slot instead pins
// boundary lossless plus double-build replay: Duration crosses the
// millisecond boundary intact, the version string round-trips, and two
// Stats built from the same frozen file agree bit for bit.
package debug
