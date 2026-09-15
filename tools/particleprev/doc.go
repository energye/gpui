// Command particleprev previews one 2.5D particle effect exactly as the game simulates it.
//
// Frozen 2026-09-16 (capability 17.1, P4, S50/W8): Effect, OpenEffect,
// BuildEmitter, ReplayEffect, VerifyReplay. Additive changes only.
//
// Usage:
//
//	go run ./tools/particleprev -project tools/particleprev/testdata -effect fire -auto-only
//	go run ./tools/particleprev -project <dir> -effect smoke -manual-seconds 60
//	go run ./tools/particleprev -project <dir> -effect fire
//
// Project layout (one file per effect, missing file is a placeholder):
//
//	<dir>/effect_<name>.json  emitter recipe plus probe steps:
//	  name, origin[2], shape{kind,dir,angle_deg,extents,ring_inner,ring_outer},
//	  rate, max, speed[2], life_ms[2], gravity[2], start[4], end[4],
//	  strength, scale, sub_count, sub_speed[2], sub_life_ms[2], seed,
//	  spawn, update_ms. The shape names match game/particle
//	  (point, cone, box, ring). A "want" section may sit beside the recipe
//	  for tests; the preview ignores it.
//
// What comes out: an Effect with the recipe echo plus one fixed probe
// replay (Spawn probe.spawn, Update probe.update_ms) reporting
// Spawned/Alive/Child, and a window that ticks the same emitter live so
// fire and smoke read as fire and smoke. Keys + and - halve or double
// the rate and replay from the same seed so tuning shows immediately;
// key 0 restores the filed rate.
//
// Bad effects never crash: a torn file is core BadData in FileErr with a
// zeroed preview, and the returned error carries it for scripts. A
// missing effect file is a clean placeholder open (HasFile false, zero
// live) with a nil error so an empty project still opens. An empty dir
// path is core InvalidArg, a missing directory is core NotFound.
//
// Parity口径 (C两边齐): the number layer matches the engine bit for bit.
// The window and the preview replay through game/particle itself
// (NewEmitter/Spawn/Update), so Effect.Spawned/Alive/Child equal a direct
// engine replay of the same file with zero difference. Pixels are NOT
// hard-compared against game windows (different scenes); instead the
// same effect painted twice offscreen is byte-identical. Numbers only
// use game/core types; this tool defines no Vec2, Color, or AssetID of
// its own.
package main
