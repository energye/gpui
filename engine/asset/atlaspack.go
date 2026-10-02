//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package asset

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/energye/gpui/engine/core"
)

// Frozen atlas budgets. Beyond budget is OutOfMemory, never a guess.
const (
	// MaxSprites caps entries in one atlas.
	MaxSprites = 1024
	// MaxAtlasSize caps one atlas side in pixels.
	MaxAtlasSize = 4096
	// MaxAtlasNameLen caps one sprite name in bytes.
	MaxAtlasNameLen = 128
	// AtlasPad is the transparent gutter kept around every sprite.
	AtlasPad = 1
	// MaxAtlasJSONBytes caps one atlas JSON file.
	MaxAtlasJSONBytes = 1 << 20
)

// CurrentAtlasVersion is the atlas JSON version Pack writes.
var CurrentAtlasVersion = core.Version{Major: 1, Minor: 0}

// Input is one picture fed to Pack. Pixels are RGBA8 row-major top-left,
// len W*H*4. Pivot is 0..1. Nine holds left, top, right, bottom in pixels.
type Input struct {
	ID     core.AssetID
	W, H   int
	Pixels []byte
	PivotX float64
	PivotY float64
	Nine   [4]int
}

// Entry is one packed sprite: its name plus its rect inside the atlas.
// Pivot and Nine ride through unchanged from the input.
type Entry struct {
	name   core.AssetID
	x, y   int
	w, h   int
	pivotX float64
	pivotY float64
	nine   [4]int
}

// Name returns the sprite name.
func (e Entry) Name() core.AssetID { return e.name }

// X returns the left pixel inside the atlas.
func (e Entry) X() int { return e.x }

// Y returns the top pixel inside the atlas.
func (e Entry) Y() int { return e.y }

// W returns the sprite width.
func (e Entry) W() int { return e.w }

// H returns the sprite height.
func (e Entry) H() int { return e.h }

// Pivot returns the axis in 0..1.
func (e Entry) Pivot() (x, y float64) { return e.pivotX, e.pivotY }

// PivotX returns the horizontal axis.
func (e Entry) PivotX() float64 { return e.pivotX }

// PivotY returns the vertical axis.
func (e Entry) PivotY() float64 { return e.pivotY }

// Nine returns left, top, right, bottom margins in pixels.
func (e Entry) Nine() [4]int { return e.nine }

// Atlas is one packed sheet: placements plus the composed RGBA8 pixels.
// A parsed atlas carries placements only; Pixels is nil until packed.
type Atlas struct {
	version core.Version
	width   int
	height  int
	entries []Entry
	pixels  []byte
}

// Version returns the atlas version.
func (a *Atlas) Version() core.Version {
	if a == nil {
		return core.Version{}
	}
	return a.version
}

// Width returns the sheet width in pixels.
func (a *Atlas) Width() int {
	if a == nil {
		return 0
	}
	return a.width
}

// Height returns the sheet height in pixels.
func (a *Atlas) Height() int {
	if a == nil {
		return 0
	}
	return a.height
}

// Count returns the entry count.
func (a *Atlas) Count() int {
	if a == nil {
		return 0
	}
	return len(a.entries)
}

// Entry returns a copy of entry i.
func (a *Atlas) Entry(i int) (Entry, bool) {
	if a == nil || i < 0 || i >= len(a.entries) {
		return Entry{}, false
	}
	return a.entries[i], true
}

// Find returns the entry with name.
func (a *Atlas) Find(name core.AssetID) (Entry, bool) {
	if a == nil || name.Empty() {
		return Entry{}, false
	}
	for _, e := range a.entries {
		if e.name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Pixels returns a copy of the composed RGBA8 sheet, or nil when parsed.
func (a *Atlas) Pixels() []byte {
	if a == nil || len(a.pixels) == 0 {
		return nil
	}
	out := make([]byte, len(a.pixels))
	copy(out, a.pixels)
	return out
}

// At returns the sheet pixel at (x, y). Parsed atlases report ok=false.
func (a *Atlas) At(x, y int) (r, g, b, al uint8, ok bool) {
	if a == nil || len(a.pixels) == 0 {
		return 0, 0, 0, 0, false
	}
	if x < 0 || y < 0 || x >= a.width || y >= a.height {
		return 0, 0, 0, 0, false
	}
	off := (y*a.width + x) * 4
	p := a.pixels
	return p[off], p[off+1], p[off+2], p[off+3], true
}

// Equal reports whether o holds the same version, size, entries, and sheet
// bytes. Parsed atlases (nil pixels) only equal parsed atlases.
func (a *Atlas) Equal(o *Atlas) bool {
	if a == nil || o == nil {
		return a == o
	}
	if a.version != o.version || a.width != o.width || a.height != o.height {
		return false
	}
	if len(a.entries) != len(o.entries) {
		return false
	}
	for i := range a.entries {
		x, y := a.entries[i].pivotX, a.entries[i].pivotY
		ox, oy := o.entries[i].pivotX, o.entries[i].pivotY
		if a.entries[i].name != o.entries[i].name ||
			a.entries[i].x != o.entries[i].x ||
			a.entries[i].y != o.entries[i].y ||
			a.entries[i].w != o.entries[i].w ||
			a.entries[i].h != o.entries[i].h ||
			x != ox || y != oy ||
			a.entries[i].nine != o.entries[i].nine {
			return false
		}
	}
	return bytes.Equal(a.pixels, o.pixels)
}

func checkAtlasID(id core.AssetID) error {
	if id.Empty() {
		return core.InvalidArg("atlaspack", "")
	}
	if len(string(id)) > MaxAtlasNameLen {
		return core.InvalidArg("atlaspack", string(id))
	}
	return nil
}

func checkPivot(x, y float64) bool {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
		return false
	}
	return x >= 0 && x <= 1 && y >= 0 && y <= 1
}

func checkNine(n [4]int, w, h int) bool {
	for _, v := range n {
		if v < 0 {
			return false
		}
	}
	return n[0]+n[2] <= w && n[1]+n[3] <= h
}

// Pack shelves inputs into one sheet usable offline: no GPU, no window.
// Inputs are sorted by height, width, then name so the same set always
// lands on the same rects. The sheet keeps AtlasPad transparent pixels
// around every sprite. Only core numbers are used.
func Pack(inputs []Input, maxW, maxH int) (*Atlas, error) {
	const op = "atlaspack.Pack"
	if maxW < 1 || maxH < 1 || maxW > MaxAtlasSize || maxH > MaxAtlasSize {
		return nil, core.InvalidArg(op, "max")
	}
	if len(inputs) == 0 {
		return nil, core.InvalidArg(op, "inputs")
	}
	if len(inputs) > MaxSprites {
		return nil, core.OutOfMemory(op, "inputs")
	}
	seen := make(map[core.AssetID]bool, len(inputs))
	for i := range inputs {
		in := &inputs[i]
		if err := checkAtlasID(in.ID); err != nil {
			return nil, err
		}
		if seen[in.ID] {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		seen[in.ID] = true
		if in.W < 1 || in.H < 1 || in.W > MaxAtlasSize || in.H > MaxAtlasSize {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if in.W+2*AtlasPad > maxW || in.H+2*AtlasPad > maxH {
			return nil, core.OutOfMemory(op, string(in.ID))
		}
		if len(in.Pixels) != in.W*in.H*4 {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if !checkPivot(in.PivotX, in.PivotY) {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if !checkNine(in.Nine, in.W, in.H) {
			return nil, core.InvalidArg(op, string(in.ID))
		}
	}
	order := make([]int, len(inputs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := inputs[order[i]], inputs[order[j]]
		if a.H != b.H {
			return a.H > b.H
		}
		if a.W != b.W {
			return a.W > b.W
		}
		return a.ID < b.ID
	})
	xs := make([]int, len(inputs))
	ys := make([]int, len(inputs))
	x := AtlasPad
	y := AtlasPad
	rowH := 0
	maxRight := 0
	maxBottom := 0
	for _, idx := range order {
		w, h := inputs[idx].W, inputs[idx].H
		if x+w+AtlasPad > maxW {
			x = AtlasPad
			y += rowH + AtlasPad
			rowH = 0
		}
		if y+h+AtlasPad > maxH {
			return nil, core.OutOfMemory(op, string(inputs[idx].ID))
		}
		xs[idx] = x
		ys[idx] = y
		x += w + AtlasPad
		if h > rowH {
			rowH = h
		}
		if xs[idx]+w > maxRight {
			maxRight = xs[idx] + w
		}
		if ys[idx]+h > maxBottom {
			maxBottom = ys[idx] + h
		}
	}
	atlasW := maxRight + AtlasPad
	atlasH := maxBottom + AtlasPad
	pix := make([]byte, atlasW*atlasH*4)
	for i, in := range inputs {
		px, py := xs[i], ys[i]
		for row := 0; row < in.H; row++ {
			src := row * in.W * 4
			dst := ((py+row)*atlasW + px) * 4
			copy(pix[dst:dst+in.W*4], in.Pixels[src:src+in.W*4])
		}
	}
	entries := make([]Entry, len(inputs))
	for i, in := range inputs {
		entries[i] = Entry{
			name: in.ID, x: xs[i], y: ys[i], w: in.W, h: in.H,
			pivotX: in.PivotX, pivotY: in.PivotY, nine: in.Nine,
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return &Atlas{version: CurrentAtlasVersion, width: atlasW, height: atlasH, entries: entries, pixels: pix}, nil
}

type spriteJSON struct {
	Name   string  `json:"name"`
	X      int     `json:"x"`
	Y      int     `json:"y"`
	W      int     `json:"w"`
	H      int     `json:"h"`
	PivotX float64 `json:"pivotX"`
	PivotY float64 `json:"pivotY"`
	Nine   [4]int  `json:"nine"`
}

type atlasJSON struct {
	Version string       `json:"version"`
	Width   int          `json:"width"`
	Height  int          `json:"height"`
	Sprites []spriteJSON `json:"sprites"`
}

// Encode renders the frozen JSON: version, sheet size, and per sprite
// name, rect, pivot, and nine-slice. Pixel bytes ride the sheet image,
// never the JSON.
func (a *Atlas) Encode() ([]byte, error) {
	const op = "atlaspack.Encode"
	if a == nil {
		return nil, core.InvalidArg(op, "atlas")
	}
	if len(a.entries) == 0 || len(a.entries) > MaxSprites {
		return nil, core.InvalidArg(op, "atlas")
	}
	out := atlasJSON{
		Version: a.version.String(),
		Width:   a.width,
		Height:  a.height,
		Sprites: make([]spriteJSON, len(a.entries)),
	}
	for i, e := range a.entries {
		out.Sprites[i] = spriteJSON{
			Name: string(e.name), X: e.x, Y: e.y, W: e.w, H: e.h,
			PivotX: e.pivotX, PivotY: e.pivotY, Nine: e.nine,
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, core.BadData(op, "encode")
	}
	return raw, nil
}

// Save writes Encode to path for the offline tool chain.
func (a *Atlas) Save(path string) error {
	const op = "atlaspack.Save"
	if a == nil {
		return core.InvalidArg(op, "atlas")
	}
	if path == "" {
		return core.InvalidArg(op, "path")
	}
	raw, err := a.Encode()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Clean(path), raw, 0644); err != nil {
		return core.NotFound(op, path, err)
	}
	return nil
}

func atlasTopKeys(raw map[string]json.RawMessage) error {
	const op = "atlaspack.Parse"
	for k := range raw {
		switch k {
		case "version", "width", "height", "sprites":
		default:
			return core.Unsupported(op, k)
		}
	}
	for _, k := range []string{"version", "width", "height", "sprites"} {
		if _, ok := raw[k]; !ok {
			return core.BadData(op, k)
		}
	}
	return nil
}

func spriteKeys(raw map[string]json.RawMessage) error {
	const op = "atlaspack.Parse"
	for k := range raw {
		switch k {
		case "name", "x", "y", "w", "h", "pivotX", "pivotY", "nine":
		default:
			return core.Unsupported(op, k)
		}
	}
	for _, k := range []string{"name", "x", "y", "w", "h", "pivotX", "pivotY", "nine"} {
		if _, ok := raw[k]; !ok {
			return core.BadData(op, k)
		}
	}
	return nil
}

func rectsOverlap(a, b Entry) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

// Parse reads frozen atlas JSON. Extra keys name an unfrozen field and
// report Unsupported, never a guessed placement. Parsed atlases carry
// placements only; Pixels stays nil.
func Parse(data []byte) (*Atlas, error) {
	const op = "atlaspack.Parse"
	if len(data) == 0 {
		return nil, core.InvalidArg(op, "data")
	}
	if len(data) > MaxAtlasJSONBytes {
		return nil, core.OutOfMemory(op, "data")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, core.BadData(op, "shape")
	}
	if err := atlasTopKeys(top); err != nil {
		return nil, err
	}
	var verStr string
	if err := json.Unmarshal(top["version"], &verStr); err != nil {
		return nil, core.BadData(op, "version")
	}
	ver, err := core.ParseVersion(verStr)
	if err != nil {
		return nil, core.BadData(op, "version")
	}
	if !ver.CompatibleWith(CurrentAtlasVersion) {
		return nil, core.VersionMismatch(op, verStr)
	}
	var w, h int
	if err := json.Unmarshal(top["width"], &w); err != nil {
		return nil, core.BadData(op, "width")
	}
	if err := json.Unmarshal(top["height"], &h); err != nil {
		return nil, core.BadData(op, "height")
	}
	if w < 1 || h < 1 {
		return nil, core.BadData(op, "size")
	}
	if w > MaxAtlasSize || h > MaxAtlasSize {
		return nil, core.OutOfMemory(op, "size")
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(top["sprites"], &raws); err != nil {
		return nil, core.BadData(op, "sprites")
	}
	if len(raws) == 0 {
		return nil, core.BadData(op, "sprites")
	}
	if len(raws) > MaxSprites {
		return nil, core.OutOfMemory(op, "sprites")
	}
	entries := make([]Entry, 0, len(raws))
	seen := make(map[core.AssetID]bool, len(raws))
	for _, r := range raws {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(r, &m); err != nil {
			return nil, core.BadData(op, "sprite")
		}
		if err := spriteKeys(m); err != nil {
			return nil, err
		}
		var sj spriteJSON
		if err := json.Unmarshal(r, &sj); err != nil {
			return nil, core.BadData(op, "sprite")
		}
		id := core.AssetID(sj.Name)
		if err := checkAtlasID(id); err != nil {
			return nil, core.BadData(op, "name")
		}
		if seen[id] {
			return nil, core.BadData(op, "name")
		}
		seen[id] = true
		if sj.W < 1 || sj.H < 1 {
			return nil, core.BadData(op, string(id))
		}
		if sj.W > MaxAtlasSize || sj.H > MaxAtlasSize {
			return nil, core.OutOfMemory(op, string(id))
		}
		if sj.X < 0 || sj.Y < 0 || sj.X+sj.W > w || sj.Y+sj.H > h {
			return nil, core.BadData(op, string(id))
		}
		if !checkPivot(sj.PivotX, sj.PivotY) {
			return nil, core.BadData(op, string(id))
		}
		if !checkNine(sj.Nine, sj.W, sj.H) {
			return nil, core.BadData(op, string(id))
		}
		entries = append(entries, Entry{
			name: id, x: sj.X, y: sj.Y, w: sj.W, h: sj.H,
			pivotX: sj.PivotX, pivotY: sj.PivotY, nine: sj.Nine,
		})
	}
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			if rectsOverlap(entries[i], entries[j]) {
				return nil, core.BadData(op, "overlap")
			}
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return &Atlas{version: ver, width: w, height: h, entries: entries}, nil
}

// Load reads path and parses it as atlas JSON.
func Load(path string) (*Atlas, error) {
	const op = "atlaspack.Load"
	if path == "" {
		return nil, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	return Parse(raw)
}

// S65 gutter and extrude budgets. Gutter is the transparent seam kept
// beyond the bleed ring, extrude is the edge-pixel bleed into the seam.
const (
	// MinAtlasGutter floors the transparent seam.
	MinAtlasGutter = 2
	// MaxAtlasGutter caps the transparent seam.
	MaxAtlasGutter = 4
	// MinAtlasExtrude floors the edge bleed.
	MinAtlasExtrude = 1
	// MaxAtlasExtrude caps the edge bleed.
	MaxAtlasExtrude = 2
	// DefaultAtlasGutter is the S65 default seam.
	DefaultAtlasGutter = 2
	// DefaultAtlasExtrude is the S65 default bleed.
	DefaultAtlasExtrude = 1
)

// PackOptions names the S65 seam switch: Gutter 2..4 transparent px,
// Extrude 1..2 bleed px. Zero means the default, so the zero value
// packs with gutter 2 extrude 1.
type PackOptions struct {
	Gutter  int
	Extrude int
}

// DefaultPackOptions returns gutter 2 extrude 1.
func DefaultPackOptions() PackOptions {
	return PackOptions{Gutter: DefaultAtlasGutter, Extrude: DefaultAtlasExtrude}
}

// Normalized maps zero fields to the defaults.
func (o PackOptions) Normalized() PackOptions {
	if o.Gutter == 0 {
		o.Gutter = DefaultAtlasGutter
	}
	if o.Extrude == 0 {
		o.Extrude = DefaultAtlasExtrude
	}
	return o
}

// Validate rejects gutter outside 2..4 and extrude outside 1..2.
func (o PackOptions) Validate() error {
	const op = "atlaspack.PackWithOptions"
	n := o.Normalized()
	if n.Gutter < MinAtlasGutter || n.Gutter > MaxAtlasGutter {
		return core.InvalidArg(op, "gutter")
	}
	if n.Extrude < MinAtlasExtrude || n.Extrude > MaxAtlasExtrude {
		return core.InvalidArg(op, "extrude")
	}
	return nil
}

// PackWithOptions shelves inputs with a parameterized seam: gutter
// transparent px stay clear beyond an extrude bleed ring that repeats
// edge texels. Spacing between rects is gutter plus bleed on both
// sides, the outer border keeps gutter plus one bleed, so the seam
// beyond the ring stays fully transparent (maxDiff 0). Sort order and
// error codes match Pack. Only core numbers are used.
func PackWithOptions(inputs []Input, maxW, maxH int, opts PackOptions) (*Atlas, error) {
	const op = "atlaspack.PackWithOptions"
	if maxW < 1 || maxH < 1 || maxW > MaxAtlasSize || maxH > MaxAtlasSize {
		return nil, core.InvalidArg(op, "max")
	}
	if len(inputs) == 0 {
		return nil, core.InvalidArg(op, "inputs")
	}
	if len(inputs) > MaxSprites {
		return nil, core.OutOfMemory(op, "inputs")
	}
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	n := opts.Normalized()
	gutter, extrude := n.Gutter, n.Extrude
	need := 2*gutter + 2*extrude
	seen := make(map[core.AssetID]bool, len(inputs))
	for i := range inputs {
		in := &inputs[i]
		if err := checkAtlasID(in.ID); err != nil {
			return nil, err
		}
		if seen[in.ID] {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		seen[in.ID] = true
		if in.W < 1 || in.H < 1 || in.W > MaxAtlasSize || in.H > MaxAtlasSize {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if in.W+need > maxW || in.H+need > maxH {
			return nil, core.OutOfMemory(op, string(in.ID))
		}
		if len(in.Pixels) != in.W*in.H*4 {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if !checkPivot(in.PivotX, in.PivotY) {
			return nil, core.InvalidArg(op, string(in.ID))
		}
		if !checkNine(in.Nine, in.W, in.H) {
			return nil, core.InvalidArg(op, string(in.ID))
		}
	}
	order := make([]int, len(inputs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(i, j int) bool {
		a, b := inputs[order[i]], inputs[order[j]]
		if a.H != b.H {
			return a.H > b.H
		}
		if a.W != b.W {
			return a.W > b.W
		}
		return a.ID < b.ID
	})
	xs := make([]int, len(inputs))
	ys := make([]int, len(inputs))
	x := gutter + extrude
	y := gutter + extrude
	rowH := 0
	maxRight := 0
	maxBottom := 0
	step := gutter + 2*extrude
	for _, idx := range order {
		w, h := inputs[idx].W, inputs[idx].H
		if x+w+extrude+gutter > maxW {
			x = gutter + extrude
			y += rowH + step
			rowH = 0
		}
		if y+h+extrude+gutter > maxH {
			return nil, core.OutOfMemory(op, string(inputs[idx].ID))
		}
		xs[idx] = x
		ys[idx] = y
		x += w + step
		if h > rowH {
			rowH = h
		}
		if xs[idx]+w > maxRight {
			maxRight = xs[idx] + w
		}
		if ys[idx]+h > maxBottom {
			maxBottom = ys[idx] + h
		}
	}
	atlasW := maxRight + extrude + gutter
	atlasH := maxBottom + extrude + gutter
	pix := make([]byte, atlasW*atlasH*4)
	for i, in := range inputs {
		px, py := xs[i], ys[i]
		for row := 0; row < in.H; row++ {
			src := row * in.W * 4
			dst := ((py+row)*atlasW + px) * 4
			copy(pix[dst:dst+in.W*4], in.Pixels[src:src+in.W*4])
		}
		extrudeRing(pix, atlasW, atlasH, px, py, in.W, in.H, extrude)
	}
	entries := make([]Entry, len(inputs))
	for i, in := range inputs {
		entries[i] = Entry{
			name: in.ID, x: xs[i], y: ys[i], w: in.W, h: in.H,
			pivotX: in.PivotX, pivotY: in.PivotY, nine: in.Nine,
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return &Atlas{version: CurrentAtlasVersion, width: atlasW, height: atlasH, entries: entries, pixels: pix}, nil
}

func copyAtlasPixel(pix []byte, atlasW, dstX, dstY, srcX, srcY int) {
	dst := (dstY*atlasW + dstX) * 4
	src := (srcY*atlasW + srcX) * 4
	pix[dst], pix[dst+1], pix[dst+2], pix[dst+3] = pix[src], pix[src+1], pix[src+2], pix[src+3]
}

// extrudeRing repeats edge texels outward by extrude px, corners copy
// the corner texel. Caller keeps spacing so rings never overlap rects.
func extrudeRing(pix []byte, atlasW, atlasH, x, y, w, h, extrude int) {
	if extrude < 1 {
		return
	}
	for k := 1; k <= extrude; k++ {
		for i := 0; i < w; i++ {
			copyAtlasPixel(pix, atlasW, x+i, y-k, x+i, y)
			copyAtlasPixel(pix, atlasW, x+i, y+h+k-1, x+i, y+h-1)
		}
		for j := 0; j < h; j++ {
			copyAtlasPixel(pix, atlasW, x-k, y+j, x, y+j)
			copyAtlasPixel(pix, atlasW, x+w+k-1, y+j, x+w-1, y+j)
		}
		for ky := 1; ky <= extrude; ky++ {
			for kx := 1; kx <= extrude; kx++ {
				copyAtlasPixel(pix, atlasW, x-kx, y-ky, x, y)
				copyAtlasPixel(pix, atlasW, x+w+kx-1, y-ky, x+w-1, y)
				copyAtlasPixel(pix, atlasW, x-kx, y+h+ky-1, x, y+h-1)
				copyAtlasPixel(pix, atlasW, x+w+kx-1, y+h+ky-1, x+w-1, y+h-1)
			}
		}
	}
	_, _ = atlasH, y
}

// SpriteToAtlasUV maps sprite-local u,v (0..1, top-left origin) to atlas
// u,v. Rotated selects the 90-degree clockwise store where the stored
// rect is already w-by-h swapped. Nil-safe through Entry zero values.
func SpriteToAtlasUV(e Entry, atlasW, atlasH int, u, v float64, rotated bool) (au, av float64) {
	if atlasW <= 0 || atlasH <= 0 {
		return 0, 0
	}
	if !rotated {
		return (float64(e.x) + u*float64(e.w)) / float64(atlasW),
			(float64(e.y) + v*float64(e.h)) / float64(atlasH)
	}
	sw, sh := float64(e.w), float64(e.h)
	return (float64(e.x) + (1-v)*sw) / float64(atlasW),
		(float64(e.y) + u*sh) / float64(atlasH)
}

// AtlasToSpriteUV is the inverse of SpriteToAtlasUV: atlas u,v back to
// sprite-local u,v for the same rotated switch. Round-trips to the
// input within float error.
func AtlasToSpriteUV(e Entry, atlasW, atlasH int, au, av float64, rotated bool) (u, v float64) {
	if atlasW <= 0 || atlasH <= 0 || e.w <= 0 || e.h <= 0 {
		return 0, 0
	}
	if !rotated {
		return (au*float64(atlasW) - float64(e.x)) / float64(e.w),
			(av*float64(atlasH) - float64(e.y)) / float64(e.h)
	}
	sw, sh := float64(e.w), float64(e.h)
	return (av*float64(atlasH) - float64(e.y)) / sh,
		1 - (au*float64(atlasW)-float64(e.x))/sw
}

// SeamTransparentMaxDiff returns the max channel distance from
// transparent over seam pixels: pixels outside every entry rect grown
// by extrude. Zero means the seam is fully transparent. Nil or parsed
// atlases report 0.
func SeamTransparentMaxDiff(a *Atlas, extrude int) int {
	if a == nil || len(a.pixels) == 0 {
		return 0
	}
	if extrude < 0 {
		extrude = 0
	}
	maxD := 0
	for yy := 0; yy < a.height; yy++ {
		for xx := 0; xx < a.width; xx++ {
			inside := false
			for _, e := range a.entries {
				if xx >= e.x-extrude && xx < e.x+e.w+extrude &&
					yy >= e.y-extrude && yy < e.y+e.h+extrude {
					inside = true
					break
				}
			}
			if inside {
				continue
			}
			off := (yy*a.width + xx) * 4
			for k := 0; k < 4; k++ {
				if v := int(a.pixels[off+k]); v > maxD {
					maxD = v
				}
			}
		}
	}
	return maxD
}

// IsSeamTransparent reports whether the seam beyond the bleed ring is
// fully transparent.
func IsSeamTransparent(a *Atlas, extrude int) bool {
	return SeamTransparentMaxDiff(a, extrude) == 0
}
