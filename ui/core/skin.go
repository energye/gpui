package core

// Painter draws a node chrome (background/border/…) into PaintContext.
// Used by Skin for typeID-based drawing strategy (C-Skin).
type Painter func(pc *PaintContext, n Node)

// Skin maps typeID → Painter. Nil painter means use node default paint.
type Skin interface {
	Painter(typeID string) Painter
}

// MapSkin is a simple map-backed Skin.
type MapSkin struct {
	Painters map[string]Painter
}

// NewMapSkin creates an empty map skin.
func NewMapSkin() *MapSkin {
	return &MapSkin{Painters: make(map[string]Painter)}
}

// Painter implements Skin.
func (s *MapSkin) Painter(typeID string) Painter {
	if s == nil || s.Painters == nil {
		return nil
	}
	return s.Painters[typeID]
}

// Set registers or replaces a painter for typeID.
func (s *MapSkin) Set(typeID string, p Painter) {
	if s.Painters == nil {
		s.Painters = make(map[string]Painter)
	}
	s.Painters[typeID] = p
}

// Override returns a skin that prefers overrides then falls back to base.
func Override(base Skin, typeID string, p Painter) Skin {
	return &overrideSkin{base: base, typeID: typeID, p: p}
}

type overrideSkin struct {
	base   Skin
	typeID string
	p      Painter
}

func (o *overrideSkin) Painter(typeID string) Painter {
	if typeID == o.typeID {
		return o.p
	}
	if o.base != nil {
		return o.base.Painter(typeID)
	}
	return nil
}
