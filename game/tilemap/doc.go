// Package tilemap freezes the CPU-side tile math every 2.5D map feeds.
//
// Frozen 2026-09-15 (capability 7.1, P2, S05/W1): Orientation, LayerKind,
// Tileset, Layer, Object, Tilemap, NewTilemap, AddLayer, AddObject,
// LayerIndex, At, CellToWorld, WorldToCell, CellBounds, ObjectsIn,
// IsSolidAt, NavCostAt, OccludedAt, AutoMaskAt, AutoVariant4, ParseTMX,
// Iso, NewIso, TileToWorld, WorldToTile, TileCenter, DiamondContains.
// Frozen 2026-09-15 (capability 7.2, P2, S20/W2): Chunk, ChunkID,
// NewChunker, MaxChunks, ChunkOf, ChunkBounds, Needed, Load, Unload,
// Update, Visible, Loaded, IsLoaded, Reset, LoadedCount, ChunkCols,
// ChunkRows. The caller feeds camera VisibleWorldRect in as a core.Rect
// and draws the returned ids with the existing render draws.
// Additive changes only.
//
// The package draws nothing; the caller draws the returned cells and
// objects in order with the existing render draws. Only core numbers are
// used; render types convert once at the boundary with Vec2.ToRenderPoint.
//
// Grid (orthogonal): cell (col,row) top-left is (col*TileW, row*TileH).
// Grid (isometric,斜45度): cell bounding-box top-left is
// ((col-row)*TileW/2, (col+row)*TileH/2), diamond center adds half a tile.
// WorldToCell floors to the owning cell; out-of-map reports ok=false.
//
// Layers: ground/decoration hold art GIDs, collision holds solid cells,
// navigation holds move costs, occlusion holds shade cells. GID 0 is empty
// and never solid, never a cost, never shade. Layer W/H always equals the
// map W/H; anything else is a core InvalidArg error.
//
// Objects:摆怪摆箱摆出生点. Bounds are pixel rects in the same world units
// as CellToWorld. ID must be > 0, Bounds must be finite with W/H >= 0.
//
// Auto tile (路沿自接, internal rule only): AutoMaskAt reads the 4-neighbour
// presence on one layer (N=1,E=2,S=4,W=8, out-of-map counts as empty),
// AutoVariant4 maps mask 0..15 to the variant index (identity today).
// The rule file format stays unfrozen (后冻): no file is read, the caller
// adds a GID base to the variant.
//
// TMX subset (认TMX子集,先冻): orientation orthogonal/isometric, map
// width/height/tilewidth/tileheight, tileset firstgid/name/tilecount/columns,
// layer name/width/height/data csv, objectgroup/object
// id/name/type/x/y/width/height, layer kind property (kind=ground|
// decoration|collision|navigation|occlusion, missing defaults to ground).
// Anything else (hexagonal/staggered, infinite chunks, non-csv encoding,
// ellipse/polygon/polyline/template objects) is a core Unsupported error,
// never a guessed map. Malformed XML/numbers/counts are core BadData.
package tilemap
