package emoji

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// v1table assembles a synthetic COLRv1 table: header(34) | BaseGlyphList |
// LayerList | paints. Paint child offsets are relative to the referencing
// table's start: rel32 reserves a slot anchored at the parent position and
// fixRel resolves it to (target − anchor) once the child is emitted. (The
// spec only allows forward offsets; tests keep parents before children.)
type v1table struct {
	buf     bytes.Buffer
	patches []v1patch
}

type v1patch struct {
	at    int
	base  uint32
	value uint32
	size  int // 3 (Offset24) or 4 (Offset32)
}

func (t *v1table) u8(v byte)     { t.buf.WriteByte(v) }
func (t *v1table) u16(v uint16)  { binary.Write(&t.buf, binary.BigEndian, v) }
func (t *v1table) u24(v uint32)  { t.u8(byte(v >> 16)); t.u8(byte(v >> 8)); t.u8(byte(v)) }
func (t *v1table) u32(v uint32)  { binary.Write(&t.buf, binary.BigEndian, v) }
func (t *v1table) f14(v float64) { t.u16(uint16(int16(v * 16384))) }
func (t *v1table) pos() uint32   { return uint32(t.buf.Len()) }

// rel32 reserves a 4-byte relative-offset slot anchored at base.
func (t *v1table) rel32(base uint32) int {
	at := int(t.pos())
	t.u32(0)
	t.patches = append(t.patches, v1patch{at: at, base: base, size: 4})
	return at
}

// rel24 reserves a 3-byte relative-offset slot (paint child offsets are
// Offset24 per spec) anchored at base.
func (t *v1table) rel24(base uint32) int {
	at := int(t.pos())
	t.u24(0)
	t.patches = append(t.patches, v1patch{at: at, base: base, size: 3})
	return at
}

// fixRel resolves slot to point at target (absolute table position).
func (t *v1table) fixRel(slot int, target uint32) {
	for i := range t.patches {
		if t.patches[i].at == slot && t.patches[i].value == 0 {
			t.patches[i].value = target - t.patches[i].base
			return
		}
	}
}

func (t *v1table) build() []byte {
	for _, p := range t.patches {
		if p.size == 3 {
			v := p.value
			t.buf.Bytes()[p.at] = byte(v >> 16)
			t.buf.Bytes()[p.at+1] = byte(v >> 8)
			t.buf.Bytes()[p.at+2] = byte(v)
		} else {
			binary.BigEndian.PutUint32(t.buf.Bytes()[p.at:p.at+4], p.value)
		}
	}
	return t.buf.Bytes()
}

// --- paint emitters (return absolute table positions) ---

func (t *v1table) colrLayers(numLayers byte, first uint32) uint32 {
	p := t.pos()
	t.u8(PaintColrLayers)
	t.u8(numLayers)
	t.u32(first)
	return p
}

func (t *v1table) solid(paletteIndex uint16, alpha float64) uint32 {
	p := t.pos()
	t.u8(PaintSolid)
	t.u16(paletteIndex)
	t.f14(alpha)
	return p
}

func (t *v1table) varSolid(paletteIndex uint16, alpha float64) uint32 {
	p := t.pos()
	t.u8(PaintVarSolid)
	t.u16(paletteIndex)
	t.f14(alpha)
	t.u32(0) // varIndexBase: no variation store → ignored per spec
	return p
}

func (t *v1table) scaleAroundCenter(sx, sy, cx, cy float64) (self uint32, childSlot int) {
	self = t.pos()
	t.u8(PaintScaleAroundCenter)
	childSlot = t.rel24(self)
	t.f14(sx)
	t.f14(sy)
	t.u16(uint16(int16(cx)))
	t.u16(uint16(int16(cy)))
	return self, childSlot
}

func (t *v1table) translate(dx, dy float64) (self uint32, childSlot int) {
	self = t.pos()
	t.u8(PaintTranslate)
	childSlot = t.rel24(self)
	t.u16(uint16(int16(dx)))
	t.u16(uint16(int16(dy)))
	return self, childSlot
}

func (t *v1table) composite(mode byte) (self uint32, srcSlot, bdSlot int) {
	self = t.pos()
	t.u8(PaintComposite)
	srcSlot = t.rel24(self)
	t.u8(mode)
	bdSlot = t.rel24(self)
	return self, srcSlot, bdSlot
}

func (t *v1table) colrGlyphRef(gid uint16) uint32 {
	p := t.pos()
	t.u8(PaintColrGlyph)
	t.u16(gid)
	return p
}

func (t *v1table) colorLine(extend byte, stops ...[3]float64) uint32 {
	p := t.pos()
	t.u8(extend)
	t.u16(uint16(len(stops)))
	for _, s := range stops {
		t.f14(s[0])
		t.u16(uint16(s[1]))
		t.f14(s[2])
	}
	return p
}

func (t *v1table) sweep(cx, cy, startF14, endF14 float64) (self uint32, lineSlot int) {
	self = t.pos()
	t.u8(PaintSweepGradient)
	lineSlot = t.rel24(self)
	t.u16(uint16(int16(cx)))
	t.u16(uint16(int16(cy)))
	t.f14(startF14)
	t.f14(endF14)
	return self, lineSlot
}

// --- list emitters ---

// baseGlyphList writes the list header and one record with a paint-offset
// slot; returns the slot (offsets relative to the list start).
func (t *v1table) baseGlyphList(gid uint16) int {
	base := t.pos()
	t.u32(1) // numBaseGlyphPaintRecords
	t.u16(gid)
	return t.rel32(base)
}

// startLayerList writes the header and returns the list's absolute position.
func (t *v1table) startLayerList(num uint32) uint32 {
	base := t.pos()
	t.u32(num)
	return base
}

func (t *v1table) finish(baseGlyphListBody, layerListBody uint32) []byte {
	head := &bytes.Buffer{}
	u16 := func(v uint16) { binary.Write(head, binary.BigEndian, v) }
	u32 := func(v uint32) { binary.Write(head, binary.BigEndian, v) }
	u16(1) // version
	u16(0) // numBaseGlyphRecords
	u32(0) // baseGlyphRecordsOffset
	u32(0) // layerRecordsOffset
	u16(0) // numLayerRecords
	u32(34 + baseGlyphListBody)
	u32(34 + layerListBody)
	u32(0) // clipListOffset (clip coverage lands in the end-to-end tests)
	u32(0) // varIndexMapOffset
	u32(0) // itemVariationStoreOffset
	return append(head.Bytes(), t.build()...)
}

// solidGlyphTable: BaseGlyphList(7) → root layers → [single layer paint].
func solidGlyphTable(layer func(t *v1table) uint32) []byte {
	tt := &v1table{}
	slot := tt.baseGlyphList(7)
	root := tt.colrLayers(1, 0)
	llBase := tt.startLayerList(1)
	entry := tt.rel32(llBase)
	lyr := layer(tt)
	tt.fixRel(entry, lyr)
	tt.fixRel(slot, root)
	return tt.finish(0, llBase)
}

// --- tests ---

func TestCOLRv1_ParseSolidRoot(t *testing.T) {
	table := solidGlyphTable(func(t *v1table) uint32 { return t.solid(2, 1.0) })
	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	if !p.HasGlyph(7) {
		t.Fatal("HasGlyph(7) = false, want true")
	}
	g, err := p.GetGlyph(7, 0)
	if err != nil {
		t.Fatalf("GetGlyph(7) error = %v", err)
	}
	if g.Paint == nil {
		t.Fatal("GetGlyph(7).Paint = nil, want paint graph")
	}
	if g.Paint.Format != PaintColrLayers || len(g.Paint.Children) != 1 {
		t.Fatalf("root = %+v, want 1-layer PaintColrLayers", g.Paint)
	}
	child := g.Paint.Children[0].Node
	if child == nil || child.Format != PaintSolid || child.PaletteIndex != 2 || child.Alpha != 1 {
		t.Errorf("layer child = %+v, want solid palette 2 alpha 1", child)
	}
}

func TestCOLRv1_SweepAngleBias(t *testing.T) {
	tt := &v1table{}
	slot := tt.baseGlyphList(7)
	root := tt.colrLayers(1, 0)
	llBase := tt.startLayerList(1)
	entry := tt.rel32(llBase)
	// F2DOT14 0.25 → (0.25+1.0)×180 = 225°; 0.75 → 315°.
	sweepPos, lineSlot := tt.sweep(100, 200, 0.25, 0.75)
	line := tt.colorLine(ExtendRepeat, [3]float64{0, 1, 1}, [3]float64{1, 0, 0.5})
	tt.fixRel(lineSlot, line)
	tt.fixRel(entry, sweepPos)
	tt.fixRel(slot, root)
	table := tt.finish(0, llBase)

	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	g, err := p.GetGlyph(7, 0)
	if err != nil || g.Paint == nil {
		t.Fatalf("GetGlyph(7) = (%v, %v)", g, err)
	}
	sNode := g.Paint.Children[0].Node
	if sNode == nil || sNode.Format != PaintSweepGradient {
		t.Fatalf("sweep node = %+v", sNode)
	}
	if sNode.CenterX != 100 || sNode.CenterY != 200 {
		t.Errorf("sweep center = (%v,%v), want (100,200)", sNode.CenterX, sNode.CenterY)
	}
	if sNode.StartAngleDeg != 225 || sNode.EndAngleDeg != 315 {
		t.Errorf("sweep angles = (%v,%v), want (225,315)", sNode.StartAngleDeg, sNode.EndAngleDeg)
	}
	if sNode.Line.Extend != ExtendRepeat || len(sNode.Line.Stops) != 2 {
		t.Errorf("color line = %+v, want repeat with 2 stops", sNode.Line)
	}
	if sNode.Line.Stops[0].PaletteIndex != 1 || sNode.Line.Stops[0].Alpha != 1 ||
		sNode.Line.Stops[1].PaletteIndex != 0 || sNode.Line.Stops[1].Alpha != 0.5 {
		t.Errorf("stops = %+v", sNode.Line.Stops)
	}
}

func TestCOLRv1_TransformMatrixComposition(t *testing.T) {
	tt := &v1table{}
	slot := tt.baseGlyphList(7)
	root := tt.colrLayers(1, 0)
	llBase := tt.startLayerList(1)
	entry := tt.rel32(llBase)
	// PaintScaleAroundCenter(0.5, 0.5 around (100,200)):
	// xx=0.5, dx = 100 − 0.5·100 = 50; dy = 200 − 0.5·200 = 100.
	scalePos, childSlot := tt.scaleAroundCenter(0.5, 0.5, 100, 200)
	solid := tt.solid(0, 1.0)
	tt.fixRel(childSlot, solid)
	tt.fixRel(entry, scalePos)
	tt.fixRel(slot, root)
	table := tt.finish(0, llBase)

	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	g, err := p.GetGlyph(7, 0)
	if err != nil || g.Paint == nil {
		t.Fatalf("GetGlyph(7) = (%v, %v)", g, err)
	}
	scaleNode := g.Paint.Children[0].Node
	if scaleNode == nil || scaleNode.Format != PaintScaleAroundCenter {
		t.Fatalf("node = %+v", scaleNode)
	}
	if scaleNode.XX != 0.5 || scaleNode.YY != 0.5 {
		t.Errorf("scale = (%v,%v), want (0.5,0.5)", scaleNode.XX, scaleNode.YY)
	}
	if scaleNode.DX != 50 || scaleNode.DY != 100 {
		t.Errorf("translation = (%v,%v), want (50,100)", scaleNode.DX, scaleNode.DY)
	}
}

func TestCOLRv1_CompositeAndTranslate(t *testing.T) {
	tt := &v1table{}
	slot := tt.baseGlyphList(7)
	root := tt.colrLayers(1, 0)
	llBase := tt.startLayerList(1)
	entry := tt.rel32(llBase)
	movePos, moveChild := tt.translate(30, -40)
	compPos, srcSlot, bdSlot := tt.composite(CompositeSrcOut)
	src := tt.solid(1, 1.0)
	bd := tt.solid(0, 1.0)
	tt.fixRel(srcSlot, src)
	tt.fixRel(bdSlot, bd)
	tt.fixRel(moveChild, compPos)
	tt.fixRel(entry, movePos)
	tt.fixRel(slot, root)
	table := tt.finish(0, llBase)

	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	g, err := p.GetGlyph(7, 0)
	if err != nil || g.Paint == nil {
		t.Fatalf("GetGlyph(7) = (%v, %v)", g, err)
	}
	move := g.Paint.Children[0].Node
	if move == nil || move.Format != PaintTranslate || move.DX != 30 || move.DY != -40 {
		t.Fatalf("translate node = %+v", move)
	}
	compNode := move.Child
	if compNode == nil || compNode.Format != PaintComposite || compNode.Mode != CompositeSrcOut {
		t.Fatalf("composite node = %+v", compNode)
	}
	if compNode.Source == nil || compNode.Source.PaletteIndex != 1 {
		t.Errorf("source = %+v", compNode.Source)
	}
	if compNode.Backdrop == nil || compNode.Backdrop.PaletteIndex != 0 {
		t.Errorf("backdrop = %+v", compNode.Backdrop)
	}
}

func TestCOLRv1_CycleRefused(t *testing.T) {
	tt := &v1table{}
	// Root paint = PaintColrGlyph(7) → itself: a cycle.
	slot := tt.baseGlyphList(7)
	selfRef := tt.colrGlyphRef(7)
	tt.fixRel(slot, selfRef)
	table := tt.finish(0, 0)

	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	if _, err := p.GetGlyph(7, 0); err == nil {
		t.Error("GetGlyph(cyclic) error = nil, want error")
	}
}

func TestCOLRv1_VarSolidStaticRead(t *testing.T) {
	table := solidGlyphTable(func(t *v1table) uint32 { return t.varSolid(3, 0.25) })
	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	g, err := p.GetGlyph(7, 0)
	if err != nil || g.Paint == nil {
		t.Fatalf("GetGlyph(7) = (%v, %v)", g, err)
	}
	node := g.Paint.Children[0].Node
	if node == nil || node.Format != PaintVarSolid || node.PaletteIndex != 3 || node.Alpha != 0.25 {
		t.Errorf("var solid node = %+v", node)
	}
}

func TestCOLRv1_UnknownFormatIgnored(t *testing.T) {
	tt := &v1table{}
	slot := tt.baseGlyphList(7)
	bad := tt.pos()
	tt.u8(99) // unknown format
	tt.u32(0)
	tt.fixRel(slot, bad)
	table := tt.finish(0, 0)

	p, err := NewCOLRParser(table, testCPAL())
	if err != nil {
		t.Fatalf("NewCOLRParser() error = %v", err)
	}
	// Root unrenderable → glyph not deliverable as v1 color.
	if _, err := p.GetGlyph(7, 0); err == nil {
		t.Error("GetGlyph(unknown root) error = nil, want error")
	}
}

// testCPAL builds a minimal CPAL table: one palette, four entries (BGRA):
// 0=red, 1=green, 2=blue, 3=yellow.
func testCPAL() []byte {
	colors := [][4]byte{
		{0, 0, 255, 255},   // BGRA of red
		{0, 255, 0, 255},   // green
		{255, 0, 0, 255},   // blue
		{0, 255, 255, 255}, // yellow
	}
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, uint16(0))           // version
	binary.Write(buf, binary.BigEndian, uint16(len(colors))) // numEntries
	binary.Write(buf, binary.BigEndian, uint16(1))           // numPalettes
	binary.Write(buf, binary.BigEndian, uint16(len(colors))) // numColorRecords
	binary.Write(buf, binary.BigEndian, uint32(14))          // colorRecordsOffset
	binary.Write(buf, binary.BigEndian, uint16(0))           // palette 0 offset
	for _, c := range colors {
		buf.Write(c[:])
	}
	return buf.Bytes()
}
