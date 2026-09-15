package anim

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/energye/gpui/game/core"
)

// CurrentSkeletonVersion is the engine side of the Spine subset contract.
// Spine 3.x and 4.x files both load (major 3 or 4); any other major
// reports VersionMismatch. Minor is checked the core way once the major
// matches.
var CurrentSkeletonVersion = core.Version{Major: 4, Minor: 0}

// BlendMode selects how BlendFrom combines two poses. The S46 state
// machine will drive this; 4.3 only freezes the entry.
type BlendMode int

const (
	// BlendReplace lerps base toward add by weight.
	BlendReplace BlendMode = 0
	// BlendAdditive adds (add minus setup) onto base by weight.
	BlendAdditive BlendMode = 1
)

// String returns the stable log name of m.
func (m BlendMode) String() string {
	switch m {
	case BlendReplace:
		return "replace"
	case BlendAdditive:
		return "additive"
	default:
		return "unknown"
	}
}

// Bone is one setup bone in Spine terms. Parent names the parent bone
// (empty for the root). Rotation and shears are degrees. Scale defaults
// to 1 when the file omits it. Color defaults to opaque white.
type Bone struct {
	Name     string
	Parent   string
	X, Y     float64
	Rotation float64
	ScaleX   float64
	ScaleY   float64
	ShearX   float64
	ShearY   float64
	Length   float64
	Color    core.Color
}

// Slot pins one draw entry to a bone. Attachment names the setup
// attachment (may be empty). Blend is one of normal, additive,
// multiply, screen. Color defaults to opaque white.
type Slot struct {
	Name       string
	Bone       string
	Attachment string
	Blend      string
	Color      core.Color
}

// VertexWeight is one bone influence on one mesh vertex. Offset sits in
// slot-bone space (the attachment x/y/rotation/scale is baked at load);
// World is the weighted sum over the vertex influences.
type VertexWeight struct {
	Bone   int
	Offset core.Vec2
	Weight float64
}

// Attachment is one drawable under a slot in a skin. Type is region or
// mesh (anything else is rejected at load with Unsupported). Path is the
// atlas image id (defaults to the attachment name). Verts always holds
// bone-space rest positions (region corners or mesh verts, attachment
// transform baked); Weights is nil for unweighted verts and per-vertex
// influences otherwise. Triangles index Verts; Hull counts hull verts.
type Attachment struct {
	Name      string
	Type      string
	Path      core.AssetID
	Color     core.Color
	Verts     []core.Vec2
	UVs       []core.Vec2
	Triangles []int
	Hull      int
	Weights   [][]VertexWeight
}

// Skin is one named attachment set: slot name to attachment name to
// drawable. The map is owned by the Skeleton; callers get copies.
type Skin struct {
	Name string
	// Attach maps slot -> attachment -> drawable.
	Attach map[string]map[string]*Attachment
}

// IKConstraint is one frozen IK rule. Bones holds 1 or 2 bone names,
// Target the target bone. Mix blends 0 (off) to 1 (full).
// BendPositive picks the elbow side. Compress and Stretch are parsed
// and reported but do not move bones yet (reserved).
type IKConstraint struct {
	Name         string
	Bones        []string
	Target       string
	Mix          float64
	BendPositive bool
	Compress     bool
	Stretch      bool
	bones        []int
	target       int
}

// TransformConstraint mixes target world motion into bones. Mix fields
// blend 0 (off) to 1 (full) per channel.
type TransformConstraint struct {
	Name      string
	Bones     []string
	Target    string
	MixRotate float64
	MixX      float64
	MixY      float64
	MixScaleX float64
	MixScaleY float64
	MixShearY float64
	bones     []int
	target    int
}

// Skeleton owns the frozen Spine subset. Bones and Slots keep file
// order; DrawOrder starts as slots order and moves only through
// SetDrawOrder. Skins keep file order with names sorted for replay.
type Skeleton struct {
	version   core.Version
	bones     []Bone
	slots     []Slot
	skins     []Skin
	ik        []IKConstraint
	transform []TransformConstraint
	boneIdx   map[string]int
	slotIdx   map[string]int
	skinIdx   map[string]int
	curSkin   int
	drawOrder []int
}

// BoneLocal is one pose override in parent space. It mirrors the Bone
// TRS fields so Reset can copy the setup back exactly.
type BoneLocal struct {
	X, Y     float64
	Rotation float64
	ScaleX   float64
	ScaleY   float64
	ShearX   float64
	ShearY   float64
}

// Pose holds live bone locals plus cached world matrices. Update
// recomputes worlds from the root down; queries auto-update first so
// callers never see a stale pose.
type Pose struct {
	skel      *Skeleton
	locals    []BoneLocal
	worlds    []core.Mat2D
	positions []core.Vec2
	dirty     bool
}

func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

func finiteBoneLocal(l BoneLocal) bool {
	return finite(l.X) && finite(l.Y) && finite(l.Rotation) &&
		finite(l.ScaleX) && finite(l.ScaleY) &&
		finite(l.ShearX) && finite(l.ShearY)
}

// localMatrix builds the Spine bone matrix: translation holds x/y, the
// X axis runs at rotation+shearX and the Y axis at rotation+90+shearY,
// each scaled. Zero rotation and shear with unit scale is identity.
func localMatrix(x, y, rot, sx, sy, shearX, shearY float64) core.Mat2D {
	rx := (rot + shearX) * math.Pi / 180
	ry := (rot + 90 + shearY) * math.Pi / 180
	return core.Mat2D{
		A: math.Cos(rx) * sx, B: math.Cos(ry) * sy, C: x,
		D: math.Sin(rx) * sx, E: math.Sin(ry) * sy, F: y,
	}
}

func worldAngleDeg(m core.Mat2D) float64 {
	return math.Atan2(m.D, m.A) * 180 / math.Pi
}

func normAngleDeg(a float64) float64 {
	for a > 180 {
		a -= 360
	}
	for a < -180 {
		a += 360
	}
	return a
}

func setupLocal(b Bone) BoneLocal {
	return BoneLocal{
		X: b.X, Y: b.Y, Rotation: b.Rotation,
		ScaleX: b.ScaleX, ScaleY: b.ScaleY,
		ShearX: b.ShearX, ShearY: b.ShearY,
	}
}

// ParseSkeleton decodes one Spine JSON subset document. Empty input is
// InvalidArg, torn JSON is BadData, a foreign major is VersionMismatch,
// physics/path constraints and non-region/mesh attachments are
// Unsupported. Dangling parents, duplicate names, and bad numbers are
// BadData, never a guessed skeleton.
func ParseSkeleton(data []byte) (*Skeleton, error) {
	const op = "anim.ParseSkeleton"
	if len(data) == 0 {
		return nil, core.InvalidArg(op, "data")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, core.BadData(op, "json", err)
	}
	if raw, ok := top["physics"]; ok && len(bytes.TrimSpace(raw)) > 0 &&
		string(bytes.TrimSpace(raw)) != "null" && string(bytes.TrimSpace(raw)) != "[]" &&
		string(bytes.TrimSpace(raw)) != "{}" {
		return nil, core.Unsupported(op, "physics")
	}
	if raw, ok := top["path"]; ok && len(bytes.TrimSpace(raw)) > 0 &&
		string(bytes.TrimSpace(raw)) != "null" && string(bytes.TrimSpace(raw)) != "[]" &&
		string(bytes.TrimSpace(raw)) != "{}" {
		return nil, core.Unsupported(op, "path")
	}
	ver, err := parseSpineVersion(top["skeleton"])
	if err != nil {
		return nil, err
	}
	bones, order, err := parseBones(top["bones"])
	if err != nil {
		return nil, err
	}
	_ = order
	slots, err := parseSlots(top["slots"], bones)
	if err != nil {
		return nil, err
	}
	skins, err := parseSkins(top["skins"], bones)
	if err != nil {
		return nil, err
	}
	ik, err := parseIK(top["ik"], bones)
	if err != nil {
		return nil, err
	}
	tf, err := parseTransform(top["transform"], bones)
	if err != nil {
		return nil, err
	}
	s := &Skeleton{
		version:   ver,
		bones:     bones,
		slots:     slots,
		skins:     skins,
		ik:        ik,
		transform: tf,
		boneIdx:   map[string]int{},
		slotIdx:   map[string]int{},
		skinIdx:   map[string]int{},
		curSkin:   0,
	}
	for i, b := range bones {
		s.boneIdx[b.Name] = i
	}
	for i, sl := range slots {
		s.slotIdx[sl.Name] = i
	}
	for i, sk := range skins {
		s.skinIdx[sk.Name] = i
	}
	s.drawOrder = make([]int, len(slots))
	for i := range slots {
		s.drawOrder[i] = i
	}
	if err := checkBoneCycles(s.bones, s.boneIdx); err != nil {
		return nil, err
	}
	return s, nil
}

// LoadSkeletonFile reads path and parses it. Empty path is InvalidArg,
// an OS miss is NotFound, data faults replay the ParseSkeleton code.
func LoadSkeletonFile(path string) (*Skeleton, error) {
	const op = "anim.LoadSkeletonFile"
	if path == "" {
		return nil, core.InvalidArg(op, "path")
	}
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, core.NotFound(op, path, err)
	}
	s, perr := ParseSkeleton(raw)
	if perr != nil {
		return nil, perr
	}
	return s, nil
}

func parseSpineVersion(raw json.RawMessage) (core.Version, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return core.Version{}, core.BadData(op, "skeleton")
	}
	var hdr struct {
		Spine string `json:"spine"`
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return core.Version{}, core.BadData(op, "skeleton", err)
	}
	if strings.TrimSpace(hdr.Spine) == "" {
		return core.Version{}, core.BadData(op, "skeleton.spine")
	}
	parts := strings.Split(strings.TrimSpace(hdr.Spine), ".")
	if len(parts) < 2 {
		return core.Version{}, core.BadData(op, "skeleton.spine")
	}
	maj, err1 := strconv.Atoi(parts[0])
	mnr, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || maj < 0 || mnr < 0 {
		return core.Version{}, core.BadData(op, "skeleton.spine")
	}
	if maj != 3 && maj != 4 {
		return core.Version{}, core.VersionMismatch(op, hdr.Spine)
	}
	return core.Version{Major: maj, Minor: mnr}, nil
}

type boneJSON struct {
	Name     string   `json:"name"`
	Parent   string   `json:"parent"`
	X        *float64 `json:"x"`
	Y        *float64 `json:"y"`
	Rotation *float64 `json:"rotation"`
	ScaleX   *float64 `json:"scaleX"`
	ScaleY   *float64 `json:"scaleY"`
	ShearX   *float64 `json:"shearX"`
	ShearY   *float64 `json:"shearY"`
	Length   *float64 `json:"length"`
	Color    string   `json:"color"`
}

func fptr(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func parseBones(raw json.RawMessage) ([]Bone, []string, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil, core.BadData(op, "bones")
	}
	var list []boneJSON
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, nil, core.BadData(op, "bones", err)
	}
	if len(list) == 0 {
		return nil, nil, core.BadData(op, "bones")
	}
	seen := map[string]bool{}
	out := make([]Bone, 0, len(list))
	order := make([]string, 0, len(list))
	for i, b := range list {
		if strings.TrimSpace(b.Name) == "" {
			return nil, nil, core.BadData(op, "bones["+strconv.Itoa(i)+"].name")
		}
		if seen[b.Name] {
			return nil, nil, core.BadData(op, "bones."+b.Name)
		}
		seen[b.Name] = true
		nb := Bone{
			Name: b.Name, Parent: b.Parent,
			X: fptr(b.X, 0), Y: fptr(b.Y, 0),
			Rotation: fptr(b.Rotation, 0),
			ScaleX:   fptr(b.ScaleX, 1), ScaleY: fptr(b.ScaleY, 1),
			ShearX: fptr(b.ShearX, 0), ShearY: fptr(b.ShearY, 0),
			Length: fptr(b.Length, 0),
			Color:  core.White,
		}
		for _, v := range []float64{nb.X, nb.Y, nb.Rotation, nb.ScaleX, nb.ScaleY, nb.ShearX, nb.ShearY, nb.Length} {
			if !finite(v) {
				return nil, nil, core.BadData(op, "bones."+b.Name)
			}
		}
		if strings.TrimSpace(b.Color) != "" {
			c, err := core.ParseHex(strings.TrimSpace(b.Color))
			if err != nil {
				return nil, nil, core.BadData(op, "bones."+b.Name+".color", err)
			}
			nb.Color = c
		}
		out = append(out, nb)
		order = append(order, b.Name)
	}
	byName := map[string]bool{}
	for _, b := range out {
		byName[b.Name] = true
	}
	for _, b := range out {
		if b.Parent != "" && !byName[b.Parent] {
			return nil, nil, core.BadData(op, "bones."+b.Name+".parent")
		}
		if b.Parent == b.Name {
			return nil, nil, core.BadData(op, "bones."+b.Name+".parent")
		}
	}
	return out, order, nil
}

func checkBoneCycles(bones []Bone, idx map[string]int) error {
	const op = "anim.ParseSkeleton"
	for _, b := range bones {
		seen := map[string]bool{b.Name: true}
		cur := b.Parent
		for cur != "" {
			if seen[cur] {
				return core.BadData(op, "bones."+b.Name+".parent")
			}
			seen[cur] = true
			pi, ok := idx[cur]
			if !ok {
				break
			}
			cur = bones[pi].Parent
		}
	}
	return nil
}

type slotJSON struct {
	Name       string `json:"name"`
	Bone       string `json:"bone"`
	Attachment string `json:"attachment"`
	Blend      string `json:"blend"`
	Color      string `json:"color"`
}

func parseSlots(raw json.RawMessage, bones []Bone) ([]Slot, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var list []slotJSON
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, core.BadData(op, "slots", err)
	}
	byBone := map[string]bool{}
	for _, b := range bones {
		byBone[b.Name] = true
	}
	seen := map[string]bool{}
	out := make([]Slot, 0, len(list))
	for i, s := range list {
		if strings.TrimSpace(s.Name) == "" {
			return nil, core.BadData(op, "slots["+strconv.Itoa(i)+"].name")
		}
		if seen[s.Name] {
			return nil, core.BadData(op, "slots."+s.Name)
		}
		seen[s.Name] = true
		if strings.TrimSpace(s.Bone) == "" || !byBone[s.Bone] {
			return nil, core.BadData(op, "slots."+s.Name+".bone")
		}
		blend := strings.TrimSpace(s.Blend)
		if blend == "" {
			blend = "normal"
		}
		switch blend {
		case "normal", "additive", "multiply", "screen":
		default:
			return nil, core.BadData(op, "slots."+s.Name+".blend")
		}
		ns := Slot{Name: s.Name, Bone: s.Bone, Attachment: s.Attachment, Blend: blend, Color: core.White}
		if strings.TrimSpace(s.Color) != "" {
			c, err := core.ParseHex(strings.TrimSpace(s.Color))
			if err != nil {
				return nil, core.BadData(op, "slots."+s.Name+".color", err)
			}
			ns.Color = c
		}
		out = append(out, ns)
	}
	return out, nil
}

type attachJSON struct {
	Type      string    `json:"type"`
	Path      string    `json:"path"`
	X         *float64  `json:"x"`
	Y         *float64  `json:"y"`
	ScaleX    *float64  `json:"scaleX"`
	ScaleY    *float64  `json:"scaleY"`
	Rotation  *float64  `json:"rotation"`
	Width     *float64  `json:"width"`
	Height    *float64  `json:"height"`
	Color     string    `json:"color"`
	Vertices  []float64 `json:"vertices"`
	Uvs       []float64 `json:"uvs"`
	Triangles []int     `json:"triangles"`
	Hull      *int      `json:"hull"`
	Edges     []int     `json:"edges"`
	Bones     []int     `json:"bones"`
}

func parseSkins(raw json.RawMessage, bones []Bone) ([]Skin, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	trim := bytes.TrimSpace(raw)
	var items []struct {
		Name        string                                `json:"name"`
		Attachments map[string]map[string]json.RawMessage `json:"attachments"`
	}
	if len(trim) > 0 && trim[0] == '[' {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, core.BadData(op, "skins", err)
		}
	} else {
		var m map[string]map[string]map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, core.BadData(op, "skins", err)
		}
		names := make([]string, 0, len(m))
		for n := range m {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			items = append(items, struct {
				Name        string                                `json:"name"`
				Attachments map[string]map[string]json.RawMessage `json:"attachments"`
			}{Name: n, Attachments: m[n]})
		}
	}
	seen := map[string]bool{}
	out := make([]Skin, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.Name) == "" {
			return nil, core.BadData(op, "skins.name")
		}
		if seen[it.Name] {
			return nil, core.BadData(op, "skins."+it.Name)
		}
		seen[it.Name] = true
		sk := Skin{Name: it.Name, Attach: map[string]map[string]*Attachment{}}
		for slotName, am := range it.Attachments {
			if sk.Attach[slotName] == nil {
				sk.Attach[slotName] = map[string]*Attachment{}
			}
			for attachName, araw := range am {
				a, err := parseAttachment(slotName, attachName, araw, bones)
				if err != nil {
					return nil, err
				}
				sk.Attach[slotName][attachName] = a
			}
		}
		out = append(out, sk)
	}
	return out, nil
}

func parseAttachment(slotName, attachName string, raw json.RawMessage, bones []Bone) (*Attachment, error) {
	const op = "anim.ParseSkeleton"
	var aj attachJSON
	if err := json.Unmarshal(raw, &aj); err != nil {
		return nil, core.BadData(op, "skins."+slotName+"."+attachName, err)
	}
	typ := strings.TrimSpace(aj.Type)
	if typ == "" {
		typ = "region"
	}
	switch typ {
	case "region", "mesh":
	default:
		return nil, core.Unsupported(op, "attachment."+typ)
	}
	path := strings.TrimSpace(aj.Path)
	if path == "" {
		path = attachName
	}
	ax, ay := fptr(aj.X, 0), fptr(aj.Y, 0)
	asx, asy := fptr(aj.ScaleX, 1), fptr(aj.ScaleY, 1)
	arot := fptr(aj.Rotation, 0)
	for _, v := range []float64{ax, ay, asx, asy, arot} {
		if !finite(v) {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName)
		}
	}
	col := core.White
	if strings.TrimSpace(aj.Color) != "" {
		c, err := core.ParseHex(strings.TrimSpace(aj.Color))
		if err != nil {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".color", err)
		}
		col = c
	}
	pre := localMatrix(ax, ay, arot, asx, asy, 0, 0)
	a := &Attachment{Name: attachName, Type: typ, Path: core.AssetID(path), Color: col}
	if typ == "region" {
		w, h := fptr(aj.Width, 0), fptr(aj.Height, 0)
		if !finite(w) || !finite(h) || w <= 0 || h <= 0 {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".size")
		}
		corners := []core.Vec2{
			{X: -w / 2, Y: -h / 2}, {X: w / 2, Y: -h / 2},
			{X: w / 2, Y: h / 2}, {X: -w / 2, Y: h / 2},
		}
		a.Verts = make([]core.Vec2, 4)
		for i, c0 := range corners {
			a.Verts[i] = pre.TransformPoint(c0)
		}
		a.UVs = []core.Vec2{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}}
		a.Triangles = []int{0, 1, 2, 0, 2, 3}
		a.Hull = 4
		return a, nil
	}
	if len(aj.Vertices) == 0 || len(aj.Uvs) == 0 || len(aj.Triangles) == 0 {
		return nil, core.BadData(op, "skins."+slotName+"."+attachName+".mesh")
	}
	for _, v := range aj.Vertices {
		if !finite(v) {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
		}
	}
	for _, v := range aj.Uvs {
		if !finite(v) {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".uvs")
		}
	}
	hull := 0
	if aj.Hull != nil {
		hull = *aj.Hull
	}
	if hull < 0 {
		return nil, core.BadData(op, "skins."+slotName+"."+attachName+".hull")
	}
	if len(aj.Bones) == 0 {
		if len(aj.Vertices)%2 != 0 {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
		}
		n := len(aj.Vertices) / 2
		if len(aj.Uvs) != n*2 {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".uvs")
		}
		a.Verts = make([]core.Vec2, n)
		for i := 0; i < n; i++ {
			p := core.V2(aj.Vertices[2*i], aj.Vertices[2*i+1])
			a.Verts[i] = pre.TransformPoint(p)
		}
		a.UVs = make([]core.Vec2, n)
		for i := 0; i < n; i++ {
			a.UVs[i] = core.V2(aj.Uvs[2*i], aj.Uvs[2*i+1])
		}
	} else {
		verts := aj.Vertices
		bonesFlat := aj.Bones
		nVerts := 0
		for p := 0; p < len(bonesFlat); {
			if p >= len(bonesFlat) {
				return nil, core.BadData(op, "skins."+slotName+"."+attachName+".bones")
			}
			n := bonesFlat[p]
			p++
			if n <= 0 || p+n > len(bonesFlat) {
				return nil, core.BadData(op, "skins."+slotName+"."+attachName+".bones")
			}
			for k := 0; k < n; k++ {
				bi := bonesFlat[p+k]
				if bi < 0 || bi >= len(bones) {
					return nil, core.BadData(op, "skins."+slotName+"."+attachName+".bones")
				}
			}
			p += n
			nVerts++
		}
		if len(aj.Uvs) != nVerts*2 {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".uvs")
		}
		a.Verts = make([]core.Vec2, nVerts)
		a.UVs = make([]core.Vec2, nVerts)
		a.Weights = make([][]VertexWeight, nVerts)
		vp := 0
		bp := 0
		for i := 0; i < nVerts; i++ {
			n := bonesFlat[bp]
			bp++
			infs := make([]VertexWeight, n)
			var rx, ry float64
			var wsum float64
			for k := 0; k < n; k++ {
				bi := bonesFlat[bp]
				bp++
				if vp+2 >= len(verts) {
					return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
				}
				ox, oy, w := verts[vp], verts[vp+1], verts[vp+2]
				vp += 3
				if !finite(ox) || !finite(oy) || !finite(w) || w < 0 {
					return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
				}
				off := pre.TransformPoint(core.V2(ox, oy))
				infs[k] = VertexWeight{Bone: bi, Offset: off, Weight: w}
				rx += off.X * w
				ry += off.Y * w
				wsum += w
			}
			if wsum <= 0 {
				return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
			}
			a.Verts[i] = core.V2(rx/wsum, ry/wsum)
			a.UVs[i] = core.V2(aj.Uvs[2*i], aj.Uvs[2*i+1])
			a.Weights[i] = infs
		}
		if vp != len(verts) {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".vertices")
		}
	}
	a.Triangles = append([]int(nil), aj.Triangles...)
	if len(a.Triangles)%3 != 0 {
		return nil, core.BadData(op, "skins."+slotName+"."+attachName+".triangles")
	}
	for _, ti := range a.Triangles {
		if ti < 0 || ti >= len(a.Verts) {
			return nil, core.BadData(op, "skins."+slotName+"."+attachName+".triangles")
		}
	}
	a.Hull = hull
	return a, nil
}

type ikJSON struct {
	Name         string   `json:"name"`
	Bones        []string `json:"bones"`
	Target       string   `json:"target"`
	Mix          *float64 `json:"mix"`
	BendPositive *bool    `json:"bendPositive"`
	Compress     bool     `json:"compress"`
	Stretch      bool     `json:"stretch"`
}

func parseIK(raw json.RawMessage, bones []Bone) ([]IKConstraint, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var list []ikJSON
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, core.BadData(op, "ik", err)
	}
	byBone := map[string]int{}
	for i, b := range bones {
		byBone[b.Name] = i
	}
	seen := map[string]bool{}
	out := make([]IKConstraint, 0, len(list))
	for i, k := range list {
		if strings.TrimSpace(k.Name) == "" {
			return nil, core.BadData(op, "ik["+strconv.Itoa(i)+"].name")
		}
		if seen[k.Name] {
			return nil, core.BadData(op, "ik."+k.Name)
		}
		seen[k.Name] = true
		if len(k.Bones) == 0 {
			return nil, core.BadData(op, "ik."+k.Name+".bones")
		}
		if len(k.Bones) > 2 {
			return nil, core.Unsupported(op, "ik."+k.Name+".bones")
		}
		bi := make([]int, len(k.Bones))
		for j, bn := range k.Bones {
			v, ok := byBone[bn]
			if !ok {
				return nil, core.BadData(op, "ik."+k.Name+".bones")
			}
			bi[j] = v
		}
		ti, ok := byBone[k.Target]
		if strings.TrimSpace(k.Target) == "" || !ok {
			return nil, core.BadData(op, "ik."+k.Name+".target")
		}
		mix := 1.0
		if k.Mix != nil {
			mix = *k.Mix
		}
		if !finite(mix) || mix < 0 || mix > 1 {
			return nil, core.BadData(op, "ik."+k.Name+".mix")
		}
		bend := false
		if k.BendPositive != nil {
			bend = *k.BendPositive
		}
		out = append(out, IKConstraint{
			Name: k.Name, Bones: append([]string(nil), k.Bones...),
			Target: k.Target, Mix: mix, BendPositive: bend,
			Compress: k.Compress, Stretch: k.Stretch,
			bones: append([]int(nil), bi...), target: ti,
		})
	}
	return out, nil
}

type transformJSON struct {
	Name      string   `json:"name"`
	Bones     []string `json:"bones"`
	Target    string   `json:"target"`
	MixRotate *float64 `json:"mixRotate"`
	MixX      *float64 `json:"mixX"`
	MixY      *float64 `json:"mixY"`
	MixScaleX *float64 `json:"mixScaleX"`
	MixScaleY *float64 `json:"mixScaleY"`
	MixShearY *float64 `json:"mixShearY"`
}

func mixPtr(v *float64, def float64) float64 {
	if v == nil {
		return def
	}
	return *v
}

func parseTransform(raw json.RawMessage, bones []Bone) ([]TransformConstraint, error) {
	const op = "anim.ParseSkeleton"
	if len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return nil, nil
	}
	var list []transformJSON
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, core.BadData(op, "transform", err)
	}
	byBone := map[string]int{}
	for i, b := range bones {
		byBone[b.Name] = i
	}
	seen := map[string]bool{}
	out := make([]TransformConstraint, 0, len(list))
	for i, k := range list {
		if strings.TrimSpace(k.Name) == "" {
			return nil, core.BadData(op, "transform["+strconv.Itoa(i)+"].name")
		}
		if seen[k.Name] {
			return nil, core.BadData(op, "transform."+k.Name)
		}
		seen[k.Name] = true
		if len(k.Bones) == 0 {
			return nil, core.BadData(op, "transform."+k.Name+".bones")
		}
		bi := make([]int, len(k.Bones))
		for j, bn := range k.Bones {
			v, ok := byBone[bn]
			if !ok {
				return nil, core.BadData(op, "transform."+k.Name+".bones")
			}
			bi[j] = v
		}
		ti, ok := byBone[k.Target]
		if strings.TrimSpace(k.Target) == "" || !ok {
			return nil, core.BadData(op, "transform."+k.Name+".target")
		}
		mr, mx, my := mixPtr(k.MixRotate, 0), mixPtr(k.MixX, 0), mixPtr(k.MixY, 0)
		msx, msy, msh := mixPtr(k.MixScaleX, 0), mixPtr(k.MixScaleY, 0), mixPtr(k.MixShearY, 0)
		for _, v := range []float64{mr, mx, my, msx, msy, msh} {
			if !finite(v) || v < 0 || v > 1 {
				return nil, core.BadData(op, "transform."+k.Name+".mix")
			}
		}
		out = append(out, TransformConstraint{
			Name: k.Name, Bones: append([]string(nil), k.Bones...),
			Target: k.Target, MixRotate: mr, MixX: mx, MixY: my,
			MixScaleX: msx, MixScaleY: msy, MixShearY: msh,
			bones: append([]int(nil), bi...), target: ti,
		})
	}
	return out, nil
}

// Version returns the parsed Spine major.minor. A nil skeleton reports 0.0.
func (s *Skeleton) Version() core.Version {
	if s == nil {
		return core.Version{}
	}
	return s.version
}

// BoneCount reports the bone count, or 0 on nil.
func (s *Skeleton) BoneCount() int {
	if s == nil {
		return 0
	}
	return len(s.bones)
}

// SlotCount reports the slot count, or 0 on nil.
func (s *Skeleton) SlotCount() int {
	if s == nil {
		return 0
	}
	return len(s.slots)
}

// BoneIndex looks a bone up by name. Empty names are InvalidArg,
// unknown bones are NotFound.
func (s *Skeleton) BoneIndex(name string) (int, error) {
	const op = "anim.Skeleton.BoneIndex"
	if s == nil {
		return -1, core.InvalidArg(op, "skeleton")
	}
	if strings.TrimSpace(name) == "" {
		return -1, core.InvalidArg(op, "bone")
	}
	i, ok := s.boneIdx[name]
	if !ok {
		return -1, core.NotFound(op, name)
	}
	return i, nil
}

// Bone returns a copy of the setup bone. The result never aliases the
// skeleton so callers cannot mutate the setup through it.
func (s *Skeleton) Bone(name string) (Bone, error) {
	const op = "anim.Skeleton.Bone"
	if s == nil {
		return Bone{}, core.InvalidArg(op, "skeleton")
	}
	i, err := s.BoneIndex(name)
	if err != nil {
		return Bone{}, err
	}
	return s.bones[i], nil
}

// SlotIndex looks a slot up by name.
func (s *Skeleton) SlotIndex(name string) (int, error) {
	const op = "anim.Skeleton.SlotIndex"
	if s == nil {
		return -1, core.InvalidArg(op, "skeleton")
	}
	if strings.TrimSpace(name) == "" {
		return -1, core.InvalidArg(op, "slot")
	}
	i, ok := s.slotIdx[name]
	if !ok {
		return -1, core.NotFound(op, name)
	}
	return i, nil
}

// Slot returns a copy of the setup slot.
func (s *Skeleton) Slot(name string) (Slot, error) {
	const op = "anim.Skeleton.Slot"
	if s == nil {
		return Slot{}, core.InvalidArg(op, "skeleton")
	}
	i, err := s.SlotIndex(name)
	if err != nil {
		return Slot{}, err
	}
	return s.slots[i], nil
}

// SkinNames returns every skin name sorted for replay.
func (s *Skeleton) SkinNames() []string {
	if s == nil || len(s.skins) == 0 {
		return nil
	}
	out := make([]string, len(s.skins))
	for i, sk := range s.skins {
		out[i] = sk.Name
	}
	sort.Strings(out)
	return out
}

// CurrentSkin returns the active skin name, or "" when skinless.
func (s *Skeleton) CurrentSkin() string {
	if s == nil || len(s.skins) == 0 {
		return ""
	}
	if s.curSkin < 0 || s.curSkin >= len(s.skins) {
		return ""
	}
	return s.skins[s.curSkin].Name
}

// SetSkin picks the active skin. Unknown skins are NotFound and keep
// the old skin; empty names are InvalidArg.
func (s *Skeleton) SetSkin(name string) error {
	const op = "anim.Skeleton.SetSkin"
	if s == nil {
		return core.InvalidArg(op, "skeleton")
	}
	if strings.TrimSpace(name) == "" {
		return core.InvalidArg(op, "skin")
	}
	i, ok := s.skinIdx[name]
	if !ok {
		return core.NotFound(op, name)
	}
	s.curSkin = i
	return nil
}

// Attachment returns a deep copy of one drawable in the active skin.
// Unknown slots, attachments, or skins are NotFound; empty names are
// InvalidArg. The copy never aliases the skeleton.
func (s *Skeleton) Attachment(slot, attach string) (*Attachment, error) {
	const op = "anim.Skeleton.Attachment"
	if s == nil {
		return nil, core.InvalidArg(op, "skeleton")
	}
	if strings.TrimSpace(slot) == "" || strings.TrimSpace(attach) == "" {
		return nil, core.InvalidArg(op, "slot")
	}
	if len(s.skins) == 0 {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	sk := &s.skins[s.curSkin]
	am, ok := sk.Attach[slot]
	if !ok {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	a, ok := am[attach]
	if !ok || a == nil {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	return cloneAttachment(a), nil
}

func cloneAttachment(a *Attachment) *Attachment {
	if a == nil {
		return nil
	}
	out := &Attachment{
		Name: a.Name, Type: a.Type, Path: a.Path,
		Hull: a.Hull, Color: a.Color,
		Verts:     append([]core.Vec2(nil), a.Verts...),
		UVs:       append([]core.Vec2(nil), a.UVs...),
		Triangles: append([]int(nil), a.Triangles...),
	}
	if a.Weights != nil {
		out.Weights = make([][]VertexWeight, len(a.Weights))
		for i, row := range a.Weights {
			out.Weights[i] = append([]VertexWeight(nil), row...)
		}
	}
	return out
}

// DrawOrder returns the slot names in draw order (a fresh slice).
func (s *Skeleton) DrawOrder() []string {
	if s == nil || len(s.slots) == 0 {
		return nil
	}
	out := make([]string, len(s.drawOrder))
	for i, si := range s.drawOrder {
		out[i] = s.slots[si].Name
	}
	return out
}

// SetDrawOrder replaces the draw order. The input must be a permutation
// of the slot names; anything else is InvalidArg and keeps the old
// order. The input slice is never retained.
func (s *Skeleton) SetDrawOrder(order []string) error {
	const op = "anim.Skeleton.SetDrawOrder"
	if s == nil {
		return core.InvalidArg(op, "skeleton")
	}
	if len(order) != len(s.slots) {
		return core.InvalidArg(op, "drawOrder")
	}
	seen := map[string]bool{}
	next := make([]int, len(order))
	for i, n := range order {
		if seen[n] {
			return core.InvalidArg(op, "drawOrder")
		}
		seen[n] = true
		si, ok := s.slotIdx[n]
		if !ok {
			return core.NotFound(op, n)
		}
		next[i] = si
	}
	s.drawOrder = next
	return nil
}

// IKNames returns the IK rule names in file order.
func (s *Skeleton) IKNames() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.ik))
	for i, k := range s.ik {
		out[i] = k.Name
	}
	return out
}

// TransformNames returns the transform rule names in file order.
func (s *Skeleton) TransformNames() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.transform))
	for i, k := range s.transform {
		out[i] = k.Name
	}
	return out
}

// IK returns a copy of one IK rule by name.
func (s *Skeleton) IK(name string) (IKConstraint, error) {
	const op = "anim.Skeleton.IK"
	if s == nil {
		return IKConstraint{}, core.InvalidArg(op, "skeleton")
	}
	if strings.TrimSpace(name) == "" {
		return IKConstraint{}, core.InvalidArg(op, "ik")
	}
	for _, k := range s.ik {
		if k.Name == name {
			cp := k
			cp.Bones = append([]string(nil), k.Bones...)
			cp.bones = append([]int(nil), k.bones...)
			return cp, nil
		}
	}
	return IKConstraint{}, core.NotFound(op, name)
}

// NewPose builds a pose at the setup position. A nil skeleton is
// InvalidArg.
func NewPose(skel *Skeleton) (*Pose, error) {
	const op = "anim.NewPose"
	if skel == nil || len(skel.bones) == 0 {
		return nil, core.InvalidArg(op, "skeleton")
	}
	p := &Pose{
		skel:      skel,
		locals:    make([]BoneLocal, len(skel.bones)),
		worlds:    make([]core.Mat2D, len(skel.bones)),
		positions: make([]core.Vec2, len(skel.bones)),
		dirty:     true,
	}
	for i, b := range skel.bones {
		p.locals[i] = setupLocal(b)
	}
	p.update()
	return p, nil
}

// Skeleton returns the pose skeleton, or nil on a nil pose.
func (p *Pose) Skeleton() *Skeleton {
	if p == nil {
		return nil
	}
	return p.skel
}

func (p *Pose) update() {
	if p == nil || p.skel == nil || !p.dirty {
		return
	}
	n := len(p.skel.bones)
	for i := 0; i < n; i++ {
		b := &p.skel.bones[i]
		l := &p.locals[i]
		local := localMatrix(l.X, l.Y, l.Rotation, l.ScaleX, l.ScaleY, l.ShearX, l.ShearY)
		if b.Parent == "" {
			p.worlds[i] = local
		} else if pi, ok := p.skel.boneIdx[b.Parent]; ok {
			p.worlds[i] = p.worlds[pi].Mul(local)
		} else {
			p.worlds[i] = local
		}
		p.positions[i] = p.worlds[i].TransformPoint(core.Vec2{})
	}
	p.dirty = false
}

// Reset parks every bone back on its setup transform. Nil-safe.
func (p *Pose) Reset() {
	if p == nil || p.skel == nil {
		return
	}
	for i, b := range p.skel.bones {
		p.locals[i] = setupLocal(b)
	}
	p.dirty = true
}

// SetBoneLocal overrides one bone local. Unknown bones are NotFound,
// empty names and NaN/Inf numbers are InvalidArg; failures change
// nothing. A nil pose is InvalidArg.
func (p *Pose) SetBoneLocal(bone string, l BoneLocal) error {
	const op = "anim.Pose.SetBoneLocal"
	if p == nil || p.skel == nil {
		return core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(bone) == "" {
		return core.InvalidArg(op, "bone")
	}
	if !finiteBoneLocal(l) {
		return core.InvalidArg(op, "local")
	}
	i, ok := p.skel.boneIdx[bone]
	if !ok {
		return core.NotFound(op, bone)
	}
	p.locals[i] = l
	p.dirty = true
	return nil
}

// BoneLocal reads one bone local copy. Unknown bones are NotFound.
func (p *Pose) BoneLocal(bone string) (BoneLocal, error) {
	const op = "anim.Pose.BoneLocal"
	if p == nil || p.skel == nil {
		return BoneLocal{}, core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(bone) == "" {
		return BoneLocal{}, core.InvalidArg(op, "bone")
	}
	i, ok := p.skel.boneIdx[bone]
	if !ok {
		return BoneLocal{}, core.NotFound(op, bone)
	}
	return p.locals[i], nil
}

// WorldTransform returns one bone world matrix. It never aliases the
// pose; callers get a value. Unknown bones are NotFound.
func (p *Pose) WorldTransform(bone string) (core.Mat2D, error) {
	const op = "anim.Pose.WorldTransform"
	if p == nil || p.skel == nil {
		return core.Mat2D{}, core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(bone) == "" {
		return core.Mat2D{}, core.InvalidArg(op, "bone")
	}
	i, ok := p.skel.boneIdx[bone]
	if !ok {
		return core.Mat2D{}, core.NotFound(op, bone)
	}
	p.update()
	return p.worlds[i], nil
}

// WorldPos returns one bone world origin. Unknown bones are NotFound.
func (p *Pose) WorldPos(bone string) (core.Vec2, error) {
	const op = "anim.Pose.WorldPos"
	if p == nil || p.skel == nil {
		return core.Vec2{}, core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(bone) == "" {
		return core.Vec2{}, core.InvalidArg(op, "bone")
	}
	i, ok := p.skel.boneIdx[bone]
	if !ok {
		return core.Vec2{}, core.NotFound(op, bone)
	}
	p.update()
	return p.positions[i], nil
}

// ApplyIK solves one frozen IK rule and blends it by its mix. One bone
// aims at the target; two bones solve the analytic elbow with the
// frozen bend side. Unknown rules are NotFound; a nil pose is
// InvalidArg. Positions within 1e-9 of the target are already solved
// and stay put so long runs never jitter on NaN.
func (p *Pose) ApplyIK(name string) error {
	const op = "anim.Pose.ApplyIK"
	if p == nil || p.skel == nil {
		return core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(name) == "" {
		return core.InvalidArg(op, "ik")
	}
	var rule *IKConstraint
	for i := range p.skel.ik {
		if p.skel.ik[i].Name == name {
			rule = &p.skel.ik[i]
			break
		}
	}
	if rule == nil {
		return core.NotFound(op, name)
	}
	if rule.Mix == 0 {
		return nil
	}
	p.update()
	switch len(rule.bones) {
	case 1:
		return p.applyIKOne(rule)
	case 2:
		return p.applyIKTwo(rule)
	default:
		return core.Unsupported(op, name)
	}
}

func (p *Pose) applyIKOne(rule *IKConstraint) error {
	b, t := rule.bones[0], rule.target
	pb, pt := p.positions[b], p.positions[t]
	d := pt.Sub(pb)
	if d.LengthSq() < 1e-18 {
		return nil
	}
	desired := math.Atan2(d.Y, d.X) * 180 / math.Pi
	cur := worldAngleDeg(p.worlds[b])
	p.locals[b].Rotation += normAngleDeg(desired-cur) * rule.Mix
	p.dirty = true
	p.update()
	return nil
}

func (p *Pose) applyIKTwo(rule *IKConstraint) error {
	par, ch, t := rule.bones[0], rule.bones[1], rule.target
	pp, pt := p.positions[par], p.positions[t]
	a := p.skel.bones[par].Length
	if a <= 0 {
		a = p.positions[ch].Sub(pp).Length()
	}
	b := p.skel.bones[ch].Length
	if a <= 0 || b <= 0 {
		return p.applyIKOne(&IKConstraint{bones: []int{par}, target: t, Mix: rule.Mix})
	}
	c := pt.Sub(pp).Length()
	if c < 1e-9 {
		return nil
	}
	maxC, minC := a+b-1e-9, math.Abs(a-b)+1e-9
	if c > maxC {
		c = maxC
	}
	if c < minC {
		c = minC
	}
	cosA := (a*a + c*c - b*b) / (2 * a * c)
	if cosA > 1 {
		cosA = 1
	}
	if cosA < -1 {
		cosA = -1
	}
	angA := math.Acos(cosA) * 180 / math.Pi
	sign := 1.0
	if !rule.BendPositive {
		sign = -1
	}
	base := math.Atan2(pt.Y-pp.Y, pt.X-pp.X) * 180 / math.Pi
	parentWant := base - sign*angA
	curParent := worldAngleDeg(p.worlds[par])
	p.locals[par].Rotation += normAngleDeg(parentWant-curParent) * rule.Mix
	p.dirty = true
	p.update()
	pc := p.positions[ch]
	d := pt.Sub(pc)
	if d.LengthSq() < 1e-18 {
		return nil
	}
	childWant := math.Atan2(d.Y, d.X) * 180 / math.Pi
	curChild := worldAngleDeg(p.worlds[ch])
	p.locals[ch].Rotation += normAngleDeg(childWant-curChild) * rule.Mix
	p.dirty = true
	p.update()
	return nil
}

// ApplyTransform blends one frozen transform rule into its bones:
// world position lerps by MixX/MixY through the parent inverse so
// rotated parents still land on the target, world rotation lerps by
// MixRotate, local scales and shear lerp by their mixes. Unknown
// rules are NotFound; a nil pose is InvalidArg.
func (p *Pose) ApplyTransform(name string) error {
	const op = "anim.Pose.ApplyTransform"
	if p == nil || p.skel == nil {
		return core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(name) == "" {
		return core.InvalidArg(op, "transform")
	}
	var rule *TransformConstraint
	for i := range p.skel.transform {
		if p.skel.transform[i].Name == name {
			rule = &p.skel.transform[i]
			break
		}
	}
	if rule == nil {
		return core.NotFound(op, name)
	}
	p.update()
	tPos := p.positions[rule.target]
	tAng := worldAngleDeg(p.worlds[rule.target])
	tLocal, _ := p.BoneLocal(p.skel.bones[rule.target].Name)
	for _, bi := range rule.bones {
		bName := p.skel.bones[bi].Name
		cur := p.locals[bi]
		if rule.MixX != 0 || rule.MixY != 0 {
			pb := p.positions[bi]
			want := core.V2(
				pb.X+(tPos.X-pb.X)*rule.MixX,
				pb.Y+(tPos.Y-pb.Y)*rule.MixY,
			)
			parentName := p.skel.bones[bi].Parent
			if parentName == "" {
				cur.X, cur.Y = want.X, want.Y
			} else if pi, ok := p.skel.boneIdx[parentName]; ok {
				if inv, ok := p.worlds[pi].Invert(); ok {
					lp := inv.TransformPoint(want)
					cur.X, cur.Y = lp.X, lp.Y
				} else {
					cur.X, cur.Y = want.X, want.Y
				}
			}
		}
		if rule.MixRotate != 0 {
			curAng := worldAngleDeg(p.worlds[bi])
			cur.Rotation += normAngleDeg(tAng-curAng) * rule.MixRotate
		}
		if rule.MixScaleX != 0 {
			cur.ScaleX += (tLocal.ScaleX - cur.ScaleX) * rule.MixScaleX
		}
		if rule.MixScaleY != 0 {
			cur.ScaleY += (tLocal.ScaleY - cur.ScaleY) * rule.MixScaleY
		}
		if rule.MixShearY != 0 {
			cur.ShearY += (tLocal.ShearY - cur.ShearY) * rule.MixShearY
		}
		if !finiteBoneLocal(cur) {
			continue
		}
		_ = bName
		p.locals[bi] = cur
	}
	p.dirty = true
	p.update()
	return nil
}

// ApplyPhysics is the postponed inertia entry: it always reports
// Unsupported (empty names report InvalidArg first) so callers can
// tell "not built yet" from bad input. The JSON "physics" key is
// rejected the same way at load.
func (p *Pose) ApplyPhysics(name string) error {
	const op = "anim.Pose.ApplyPhysics"
	if p == nil || p.skel == nil {
		return core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(name) == "" {
		return core.InvalidArg(op, "physics")
	}
	return core.Unsupported(op, name)
}

// BlendFrom combines base and add into the receiver: replace lerps
// every channel by weight, additive adds (add minus setup) onto base
// by weight. All three poses must share one skeleton, weight must be
// finite in [0,1], mode must be frozen. Failures change nothing.
func (p *Pose) BlendFrom(base, add *Pose, w float64, mode BlendMode) error {
	const op = "anim.Pose.BlendFrom"
	if p == nil || base == nil || add == nil || p.skel == nil || base.skel == nil || add.skel == nil {
		return core.InvalidArg(op, "pose")
	}
	if p.skel != base.skel || p.skel != add.skel {
		return core.InvalidArg(op, "skeleton")
	}
	if !finite(w) || w < 0 || w > 1 {
		return core.InvalidArg(op, "weight")
	}
	if mode != BlendReplace && mode != BlendAdditive {
		return core.InvalidArg(op, "mode")
	}
	n := len(p.skel.bones)
	if len(base.locals) != n || len(add.locals) != n || len(p.locals) != n {
		return core.InvalidArg(op, "pose")
	}
	next := make([]BoneLocal, n)
	for i := 0; i < n; i++ {
		b, ad := base.locals[i], add.locals[i]
		setup := setupLocal(p.skel.bones[i])
		var o BoneLocal
		if mode == BlendReplace {
			o = BoneLocal{
				X: b.X + (ad.X-b.X)*w, Y: b.Y + (ad.Y-b.Y)*w,
				Rotation: b.Rotation + (ad.Rotation-b.Rotation)*w,
				ScaleX:   b.ScaleX + (ad.ScaleX-b.ScaleX)*w,
				ScaleY:   b.ScaleY + (ad.ScaleY-b.ScaleY)*w,
				ShearX:   b.ShearX + (ad.ShearX-b.ShearX)*w,
				ShearY:   b.ShearY + (ad.ShearY-b.ShearY)*w,
			}
		} else {
			o = BoneLocal{
				X: b.X + (ad.X-setup.X)*w, Y: b.Y + (ad.Y-setup.Y)*w,
				Rotation: b.Rotation + (ad.Rotation-setup.Rotation)*w,
				ScaleX:   b.ScaleX + (ad.ScaleX-setup.ScaleX)*w,
				ScaleY:   b.ScaleY + (ad.ScaleY-setup.ScaleY)*w,
				ShearX:   b.ShearX + (ad.ShearX-setup.ShearX)*w,
				ShearY:   b.ShearY + (ad.ShearY-setup.ShearY)*w,
			}
		}
		if !finiteBoneLocal(o) {
			return core.InvalidArg(op, "weight")
		}
		next[i] = o
	}
	p.locals = next
	p.dirty = true
	p.update()
	return nil
}

// SkinVertices returns the world positions of one attachment in the
// active skin. Unweighted verts ride their slot bone; weighted verts
// blend bone matrices by influence. Unknown slots or attachments are
// NotFound; empty names are InvalidArg. The result is fresh every
// call and never aliases the skeleton or the pose.
func (p *Pose) SkinVertices(slot, attach string) ([]core.Vec2, error) {
	const op = "anim.Pose.SkinVertices"
	if p == nil || p.skel == nil {
		return nil, core.InvalidArg(op, "pose")
	}
	if strings.TrimSpace(slot) == "" || strings.TrimSpace(attach) == "" {
		return nil, core.InvalidArg(op, "slot")
	}
	si, ok := p.skel.slotIdx[slot]
	if !ok {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	if len(p.skel.skins) == 0 {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	sk := &p.skel.skins[p.skel.curSkin]
	am, ok := sk.Attach[slot]
	if !ok {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	a, ok := am[attach]
	if !ok || a == nil {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	boneName := p.skel.slots[si].Bone
	bi, ok := p.skel.boneIdx[boneName]
	if !ok {
		return nil, core.NotFound(op, slot+"/"+attach)
	}
	p.update()
	out := make([]core.Vec2, len(a.Verts))
	if len(a.Weights) == 0 {
		m := p.worlds[bi]
		for i, v := range a.Verts {
			out[i] = m.TransformPoint(v)
		}
		return out, nil
	}
	for i, row := range a.Weights {
		var x, y float64
		for _, inf := range row {
			w := p.worlds[inf.Bone].TransformPoint(inf.Offset)
			x += w.X * inf.Weight
			y += w.Y * inf.Weight
		}
		out[i] = core.V2(x, y)
	}
	return out, nil
}
