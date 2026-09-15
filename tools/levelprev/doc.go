// Command levelprev previews one 2.5D level project exactly as the game sees it.
//
// Frozen 2026-09-16 (capability 17.1, P4, S50/W8): Preview, OpenProject,
// SummarizeMap, SummarizeScene, VerifyCounts. Additive changes only.
//
// Usage:
//
//	go run ./tools/levelprev -project tools/levelprev/testdata/proj_small -auto-only
//	go run ./tools/levelprev -project <dir> -manual-seconds 60
//	go run ./tools/levelprev -project <dir>
//
// Project layout (both files optional, never required):
//
//	<dir>/map.tmx    Tiled TMX subset frozen by game/tilemap (orthogonal or
//	                 isometric, csv layers, objectgroup). Missing means an
//	                 empty map placeholder, never an error.
//	<dir>/scene.json world.SceneFile JSON version 1.0 (entities with parent
//	                 links, comps, asset refs). Missing means an empty scene
//	                 placeholder, never an error.
//
// What comes out: a Preview with tile/object/entity counts plus a window
// that draws every non-empty tile as a flat block, every object as a box,
// and every entity at its world position. Tiles use one color per layer
// kind (ground green, decoration olive, collision red wash, navigation
// blue wash, occlusion gray wash); objects are yellow boxes; entities are
// red dots. This is a layout preview, not a beauty render: it proves the
// placement matches the game, not the final art.
//
// Bad projects never crash: a torn map.tmx is core BadData in MapErr with
// the map side zeroed, a torn scene.json is core BadData in SceneErr with
// the scene side zeroed, and the other side still previews. The returned
// error carries the first failure so scripts can gate on it; the Preview
// itself stays usable. An empty directory (no files at all) is a clean
// open with placeholder notes and a nil error. An empty dir path is
// core InvalidArg, a missing directory is core NotFound.
//
// Parity口径 (C两边齐): the number layer matches the engine bit for bit.
// Preview counts come straight from engine getters (Layer.GIDs,
// Tilemap.Objects, SceneFile.Entities), so preview.TileCount and friends
// equal a direct engine parse of the same files with zero difference.
// Pixels are NOT hard-compared against game windows (different scenes);
// instead the same project painted twice offscreen is byte-identical.
// Numbers only use game/core types; this tool defines no Vec2, Color,
// or AssetID of its own.
package main
