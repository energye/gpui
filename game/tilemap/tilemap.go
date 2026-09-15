package tilemap

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"

	"github.com/energye/gpui/game/core"
)

// Orientation selects the grid math: square cells or 斜45度 diamonds.
type Orientation int

const (
	// OrientOrthogonal is square cells: world is col*TileW, row*TileH.
	OrientOrthogonal Orientation = 0
	// OrientIsometric is 斜45度 diamonds: see Iso for the exact math.
	OrientIsometric Orientation = 1
)

// LayerKind selects what a layer means: art, solid, cost, or shade.
type LayerKind int

const (
	// KindGround holds walk art GIDs.
	KindGround LayerKind = 0
	// KindDecoration holds deco art GIDs over the ground.
	KindDecoration LayerKind = 1
	// KindCollision holds solid cells: non-zero GID blocks.
	KindCollision LayerKind = 2
	// KindNavigation holds move costs: non-zero GID is the cost.
	KindNavigation LayerKind = 3
	// KindOcclusion holds shade cells: non-zero GID darkens.
	KindOcclusion LayerKind = 4
)

// Tileset names one art sheet behind a GID range.
type Tileset struct {
	firstGID  int
	name      string
	tileCount int
	columns   int
}

// Layer is one tile plane: W*H GIDs in row-major order, 0 is empty.
type Layer struct {
	name string
	kind LayerKind
	w    int
	h    int
	gids []int
}

// Object is one摆怪摆箱摆出生点: pixel rect in CellToWorld units.
type Object struct {
	id     int
	name   string
	typ    string
	bounds core.Rect
}

// Tilemap is the whole grid plus its object list.
type Tilemap struct {
	orient   Orientation
	w        int
	h        int
	tileW    float64
	tileH    float64
	layers   []Layer
	objects  []Object
	tilesets []Tileset
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteRect(r core.Rect) bool {
	return finite(r.X) && finite(r.Y) && finite(r.W) && finite(r.H)
}

// NewTilemap builds an empty map. Sizes must be > 0, tiles finite and > 0.
func NewTilemap(orient Orientation, mapW, mapH int, tileW, tileH float64) (Tilemap, error) {
	if orient != OrientOrthogonal && orient != OrientIsometric {
		return Tilemap{}, core.InvalidArg("tilemap.NewTilemap", "orient")
	}
	if mapW <= 0 || mapH <= 0 {
		return Tilemap{}, core.InvalidArg("tilemap.NewTilemap", "size")
	}
	if !finite(tileW) || !finite(tileH) || tileW <= 0 || tileH <= 0 {
		return Tilemap{}, core.InvalidArg("tilemap.NewTilemap", "tile")
	}
	return Tilemap{orient: orient, w: mapW, h: mapH, tileW: tileW, tileH: tileH}, nil
}

// NewTileset builds a tileset entry. FirstGID must be > 0, name non-empty.
func NewTileset(firstGID int, name string, tileCount, columns int) (Tileset, error) {
	if firstGID <= 0 || name == "" || tileCount < 0 || columns < 0 {
		return Tileset{}, core.InvalidArg("tilemap.NewTileset", name)
	}
	return Tileset{firstGID: firstGID, name: name, tileCount: tileCount, columns: columns}, nil
}

// NewLayer builds a tile plane. GIDs are copied; length must be W*H,
// every GID >= 0, name non-empty, kind one of the five frozen kinds.
func NewLayer(name string, kind LayerKind, w, h int, gids []int) (Layer, error) {
	if name == "" {
		return Layer{}, core.InvalidArg("tilemap.NewLayer", "name")
	}
	if kind != KindGround && kind != KindDecoration && kind != KindCollision &&
		kind != KindNavigation && kind != KindOcclusion {
		return Layer{}, core.InvalidArg("tilemap.NewLayer", name)
	}
	if w <= 0 || h <= 0 || len(gids) != w*h {
		return Layer{}, core.InvalidArg("tilemap.NewLayer", name)
	}
	for _, g := range gids {
		if g < 0 {
			return Layer{}, core.InvalidArg("tilemap.NewLayer", name)
		}
	}
	cp := make([]int, len(gids))
	copy(cp, gids)
	return Layer{name: name, kind: kind, w: w, h: h, gids: cp}, nil
}

// NewObject builds one object. ID must be > 0, bounds finite with W/H >= 0
// (zero size is a point spawn, still matchable by ObjectsIn).
func NewObject(id int, name, typ string, bounds core.Rect) (Object, error) {
	if id <= 0 {
		return Object{}, core.InvalidArg("tilemap.NewObject", name)
	}
	if !finiteRect(bounds) || bounds.W < 0 || bounds.H < 0 {
		return Object{}, core.InvalidArg("tilemap.NewObject", name)
	}
	return Object{id: id, name: name, typ: typ, bounds: bounds}, nil
}

// Orient returns the grid math selector.
func (m Tilemap) Orient() Orientation { return m.orient }

// W returns the map width in tiles.
func (m Tilemap) W() int { return m.w }

// H returns the map height in tiles.
func (m Tilemap) H() int { return m.h }

// TileW returns the tile width in world units.
func (m Tilemap) TileW() float64 { return m.tileW }

// TileH returns the tile height in world units.
func (m Tilemap) TileH() float64 { return m.tileH }

// Layers returns a copy of the layer list.
func (m Tilemap) Layers() []Layer {
	out := make([]Layer, len(m.layers))
	copy(out, m.layers)
	return out
}

// Objects returns a copy of the object list.
func (m Tilemap) Objects() []Object {
	out := make([]Object, len(m.objects))
	copy(out, m.objects)
	return out
}

// Tilesets returns a copy of the tileset list.
func (m Tilemap) Tilesets() []Tileset {
	out := make([]Tileset, len(m.tilesets))
	copy(out, m.tilesets)
	return out
}

// FirstGID returns the tileset base GID.
func (s Tileset) FirstGID() int { return s.firstGID }

// Name returns the tileset name.
func (s Tileset) Name() string { return s.name }

// TileCount returns the tile count.
func (s Tileset) TileCount() int { return s.tileCount }

// Columns returns the sheet columns.
func (s Tileset) Columns() int { return s.columns }

// Name returns the layer name.
func (l Layer) Name() string { return l.name }

// Kind returns what the layer means.
func (l Layer) Kind() LayerKind { return l.kind }

// GIDs returns a copy of the row-major GIDs (0 is empty).
func (l Layer) GIDs() []int {
	out := make([]int, len(l.gids))
	copy(out, l.gids)
	return out
}

// ID returns the object id.
func (o Object) ID() int { return o.id }

// Name returns the object name.
func (o Object) Name() string { return o.name }

// Type returns the object type (spawn, chest, ...).
func (o Object) Type() string { return o.typ }

// Bounds returns the pixel rect in CellToWorld units.
func (o Object) Bounds() core.Rect { return o.bounds }

// AddTileset appends a tileset entry.
func (m *Tilemap) AddTileset(s Tileset) error {
	if s.firstGID <= 0 || s.name == "" {
		return core.InvalidArg("tilemap.AddTileset", s.name)
	}
	m.tilesets = append(m.tilesets, s)
	return nil
}

// AddLayer appends a layer. Size must equal the map size, name unique.
func (m *Tilemap) AddLayer(l Layer) error {
	if l.name == "" || l.w != m.w || l.h != m.h || len(l.gids) != m.w*m.h {
		return core.InvalidArg("tilemap.AddLayer", l.name)
	}
	for _, e := range m.layers {
		if e.name == l.name {
			return core.InvalidArg("tilemap.AddLayer", l.name)
		}
	}
	cp := make([]int, len(l.gids))
	copy(cp, l.gids)
	l.gids = cp
	m.layers = append(m.layers, l)
	return nil
}

// AddObject appends an object. ID must be unique.
func (m *Tilemap) AddObject(o Object) error {
	if o.id <= 0 || !finiteRect(o.bounds) || o.bounds.W < 0 || o.bounds.H < 0 {
		return core.InvalidArg("tilemap.AddObject", o.name)
	}
	for _, e := range m.objects {
		if e.id == o.id {
			return core.InvalidArg("tilemap.AddObject", o.name)
		}
	}
	m.objects = append(m.objects, o)
	return nil
}

// LayerIndex finds a layer by name.
func (m Tilemap) LayerIndex(name string) (int, bool) {
	for i, l := range m.layers {
		if l.name == name {
			return i, true
		}
	}
	return 0, false
}

func inMap(m Tilemap, col, row int) bool {
	return col >= 0 && row >= 0 && col < m.w && row < m.h
}

// isoProj reuses the map tile size as an Iso projector without per-call
// validation: NewTilemap already pins tileW/H finite and > 0, and zero
// maps return early via inMap, so construction cannot fail here.
func (m Tilemap) isoProj() Iso {
	return Iso{tileW: m.tileW, tileH: m.tileH}
}

// kindGIDAt returns the first non-zero GID for kind at (col,row).
// OOB and all-empty cells report ok=false, so callers never mistake an
// art hole (GID 0) for solid, cost, or shade.
func (m Tilemap) kindGIDAt(kind LayerKind, col, row int) (gid int, ok bool) {
	if !inMap(m, col, row) {
		return 0, false
	}
	for _, l := range m.layers {
		if l.kind != kind {
			continue
		}
		if g := l.gids[row*m.w+col]; g != 0 {
			return g, true
		}
	}
	return 0, false
}

// At returns the GID at (col,row) on layerIdx. OOB reports ok=false
// with gid 0 (缺块占位不崩: callers treat empty as walkable art hole).
func (m Tilemap) At(layerIdx, col, row int) (gid int, ok bool) {
	if layerIdx < 0 || layerIdx >= len(m.layers) || !inMap(m, col, row) {
		return 0, false
	}
	return m.layers[layerIdx].gids[row*m.w+col], true
}

// CellToWorld returns the cell origin in world units: orthogonal top-left,
// isometric bounding-box top-left. OOB reports ok=false with zero.
func (m Tilemap) CellToWorld(col, row int) (core.Vec2, bool) {
	if !inMap(m, col, row) {
		return core.Vec2{}, false
	}
	if m.orient == OrientOrthogonal {
		out := core.V2(float64(col)*m.tileW, float64(row)*m.tileH)
		if !finite(out.X) || !finite(out.Y) {
			return core.Vec2{}, false
		}
		return out, true
	}
	return m.isoProj().TileToWorld(col, row)
}

// WorldToCell floors world to the owning cell. Isometric picks the diamond
// that contains the point. Bad input or outside the map is ok=false.
func (m Tilemap) WorldToCell(world core.Vec2) (col, row int, ok bool) {
	if !finite(world.X) || !finite(world.Y) {
		return 0, 0, false
	}
	if m.orient == OrientOrthogonal {
		c := int(math.Floor(world.X / m.tileW))
		r := int(math.Floor(world.Y / m.tileH))
		if !inMap(m, c, r) {
			return 0, 0, false
		}
		return c, r, true
	}
	c, r, ok := m.isoProj().WorldToTile(world)
	if !ok || !inMap(m, c, r) {
		return 0, 0, false
	}
	return c, r, true
}

// CellBounds returns the cell bounding box (W=tileW,H=tileH).
func (m Tilemap) CellBounds(col, row int) (core.Rect, bool) {
	origin, ok := m.CellToWorld(col, row)
	if !ok {
		return core.Rect{}, false
	}
	return core.NewRect(origin.X, origin.Y, m.tileW, m.tileH), true
}

// ObjectsIn returns objects touching rect: box objects by overlap, point
// objects (empty bounds) by the point sitting inside rect. Bad or empty
// rect returns nil, never panics.
func (m Tilemap) ObjectsIn(rect core.Rect) []Object {
	if rect.IsEmpty() || !finiteRect(rect) {
		return nil
	}
	var out []Object
	for _, o := range m.objects {
		if o.bounds.IsEmpty() {
			// Point spawn: match when the anchor sits inside the query.
			if rect.Contains(core.V2(o.bounds.X, o.bounds.Y)) {
				out = append(out, o)
			}
			continue
		}
		// Intersects already covers containment: both rects are non-empty
		// here, so a contained box still overlaps with positive area.
		if rect.Intersects(o.bounds) {
			out = append(out, o)
		}
	}
	return out
}

// IsSolidAt reports whether any collision layer covers (col,row).
// OOB is not solid (empty air), never an error.
func (m Tilemap) IsSolidAt(col, row int) bool {
	_, ok := m.kindGIDAt(KindCollision, col, row)
	return ok
}

// NavCostAt returns the move cost at (col,row): first non-zero navigation
// GID in layer order. No data or OOB reports ok=false.
func (m Tilemap) NavCostAt(col, row int) (cost int, ok bool) {
	return m.kindGIDAt(KindNavigation, col, row)
}

// OccludedAt reports whether any occlusion layer shades (col,row).
// OOB is never shaded.
func (m Tilemap) OccludedAt(col, row int) bool {
	_, ok := m.kindGIDAt(KindOcclusion, col, row)
	return ok
}

// AutoMaskAt reads the 4-neighbour presence on one layer:
// N=1,E=2,S=4,W=8. Out-of-map neighbours count as empty.
// Bad layer or OOB cell reports ok=false.
func (m Tilemap) AutoMaskAt(layerIdx, col, row int) (mask int, ok bool) {
	if layerIdx < 0 || layerIdx >= len(m.layers) || !inMap(m, col, row) {
		return 0, false
	}
	gids := m.layers[layerIdx].gids
	present := func(c, r int) bool {
		if !inMap(m, c, r) {
			return false
		}
		return gids[r*m.w+c] != 0
	}
	if present(col, row-1) {
		mask |= 1
	}
	if present(col+1, row) {
		mask |= 2
	}
	if present(col, row+1) {
		mask |= 4
	}
	if present(col-1, row) {
		mask |= 8
	}
	return mask, true
}

// AutoVariant4 maps a 4-bit edge mask 0..15 to the art variant index.
// Identity today (variant == mask); the rule file stays unfrozen so no
// file is read here. Out-of-range masks report ok=false.
func AutoVariant4(mask int) (variant int, ok bool) {
	if mask < 0 || mask > 15 {
		return 0, false
	}
	return mask, true
}

type tmxProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
	Text  string `xml:",chardata"`
}

type tmxProperties struct {
	Props []tmxProperty `xml:"property"`
}

type tmxChunk struct {
	Text string `xml:",chardata"`
}

type tmxData struct {
	Encoding    string     `xml:"encoding,attr"`
	Compression string     `xml:"compression,attr"`
	Text        string     `xml:",chardata"`
	Chunks      []tmxChunk `xml:"chunk"`
}

type tmxTileset struct {
	FirstGID  int    `xml:"firstgid,attr"`
	Name      string `xml:"name,attr"`
	TileCount int    `xml:"tilecount,attr"`
	Columns   int    `xml:"columns,attr"`
}

type tmxLayer struct {
	Name       string        `xml:"name,attr"`
	Width      int           `xml:"width,attr"`
	Height     int           `xml:"height,attr"`
	Data       tmxData       `xml:"data"`
	Properties tmxProperties `xml:"properties"`
}

type tmxObject struct {
	ID       int       `xml:"id,attr"`
	Name     string    `xml:"name,attr"`
	Type     string    `xml:"type,attr"`
	X        float64   `xml:"x,attr"`
	Y        float64   `xml:"y,attr"`
	W        float64   `xml:"width,attr"`
	H        float64   `xml:"height,attr"`
	Template string    `xml:"template,attr"`
	Ellipse  *struct{} `xml:"ellipse"`
	Polygon  *struct{} `xml:"polygon"`
	Polyline *struct{} `xml:"polyline"`
}

type tmxObjectGroup struct {
	Name    string      `xml:"name,attr"`
	Objects []tmxObject `xml:"object"`
}

type tmxMap struct {
	XMLName     xml.Name         `xml:"map"`
	Orientation string           `xml:"orientation,attr"`
	Width       int              `xml:"width,attr"`
	Height      int              `xml:"height,attr"`
	TileWidth   float64          `xml:"tilewidth,attr"`
	TileHeight  float64          `xml:"tileheight,attr"`
	Tilesets    []tmxTileset     `xml:"tileset"`
	Layers      []tmxLayer       `xml:"layer"`
	Groups      []tmxObjectGroup `xml:"objectgroup"`
}

func kindFromString(s string) (LayerKind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "ground":
		return KindGround, true
	case "decoration":
		return KindDecoration, true
	case "collision":
		return KindCollision, true
	case "navigation":
		return KindNavigation, true
	case "occlusion":
		return KindOcclusion, true
	}
	return KindGround, false
}

func parseCSVGIDs(text string, want int) ([]int, error) {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	if len(parts) != want {
		return nil, core.BadData("tilemap.ParseTMX", "data")
	}
	out := make([]int, want)
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || v < 0 || v > 1<<30 {
			return nil, core.BadData("tilemap.ParseTMX", "data")
		}
		out[i] = v
	}
	return out, nil
}

// ParseTMX parses the frozen TMX subset (orthogonal/isometric, csv layers,
// plain rect objects, kind property). Malformed content is BadData,
// reserved-but-unbuilt shapes are Unsupported, empty input is InvalidArg.
func ParseTMX(data []byte) (Tilemap, error) {
	if len(data) == 0 {
		return Tilemap{}, core.InvalidArg("tilemap.ParseTMX", "data")
	}
	var raw tmxMap
	if err := xml.Unmarshal(data, &raw); err != nil {
		return Tilemap{}, core.BadData("tilemap.ParseTMX", "xml")
	}
	var orient Orientation
	switch strings.ToLower(strings.TrimSpace(raw.Orientation)) {
	case "orthogonal":
		orient = OrientOrthogonal
	case "isometric":
		orient = OrientIsometric
	case "hexagonal", "staggered":
		return Tilemap{}, core.Unsupported("tilemap.ParseTMX", "orientation")
	default:
		return Tilemap{}, core.BadData("tilemap.ParseTMX", "orientation")
	}
	if raw.Width <= 0 || raw.Height <= 0 || !finite(raw.TileWidth) || !finite(raw.TileHeight) ||
		raw.TileWidth <= 0 || raw.TileHeight <= 0 {
		return Tilemap{}, core.BadData("tilemap.ParseTMX", "size")
	}
	m, err := NewTilemap(orient, raw.Width, raw.Height, raw.TileWidth, raw.TileHeight)
	if err != nil {
		return Tilemap{}, core.BadData("tilemap.ParseTMX", "size")
	}
	for _, ts := range raw.Tilesets {
		if ts.FirstGID <= 0 || ts.Name == "" || ts.TileCount < 0 || ts.Columns < 0 {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "tileset")
		}
		s, err := NewTileset(ts.FirstGID, ts.Name, ts.TileCount, ts.Columns)
		if err != nil {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "tileset")
		}
		if err := m.AddTileset(s); err != nil {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "tileset")
		}
	}
	for _, tl := range raw.Layers {
		if tl.Name == "" || tl.Width != m.w || tl.Height != m.h {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "layer")
		}
		if len(tl.Data.Chunks) > 0 {
			return Tilemap{}, core.Unsupported("tilemap.ParseTMX", "infinite")
		}
		if strings.ToLower(strings.TrimSpace(tl.Data.Encoding)) != "csv" {
			return Tilemap{}, core.Unsupported("tilemap.ParseTMX", "encoding")
		}
		if strings.TrimSpace(tl.Data.Compression) != "" {
			return Tilemap{}, core.Unsupported("tilemap.ParseTMX", "compression")
		}
		kind := KindGround
		for _, p := range tl.Properties.Props {
			if strings.EqualFold(strings.TrimSpace(p.Name), "kind") {
				v := strings.TrimSpace(p.Value)
				if v == "" {
					v = strings.TrimSpace(p.Text)
				}
				k, ok := kindFromString(v)
				if !ok {
					return Tilemap{}, core.BadData("tilemap.ParseTMX", "kind")
				}
				kind = k
			}
		}
		gids, err := parseCSVGIDs(tl.Data.Text, m.w*m.h)
		if err != nil {
			return Tilemap{}, err
		}
		l, err := NewLayer(tl.Name, kind, m.w, m.h, gids)
		if err != nil {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "layer")
		}
		if err := m.AddLayer(l); err != nil {
			return Tilemap{}, core.BadData("tilemap.ParseTMX", "layer")
		}
	}
	for _, g := range raw.Groups {
		for _, o := range g.Objects {
			if o.Template != "" || o.Ellipse != nil || o.Polygon != nil || o.Polyline != nil {
				return Tilemap{}, core.Unsupported("tilemap.ParseTMX", "object")
			}
			if o.ID <= 0 || !finite(o.X) || !finite(o.Y) || !finite(o.W) || !finite(o.H) ||
				o.W < 0 || o.H < 0 {
				return Tilemap{}, core.BadData("tilemap.ParseTMX", "object")
			}
			obj, err := NewObject(o.ID, o.Name, o.Type, core.NewRect(o.X, o.Y, o.W, o.H))
			if err != nil {
				return Tilemap{}, core.BadData("tilemap.ParseTMX", "object")
			}
			if err := m.AddObject(obj); err != nil {
				return Tilemap{}, core.BadData("tilemap.ParseTMX", "object")
			}
		}
	}
	return m, nil
}
