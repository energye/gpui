package statistic

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// StatSnap is the frozen paint input for one statistic frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it on every paint-affecting
// setter (refreshSnapshot inside markLayoutDirty/markPaintDirty); the
// raster-thread paint path reads only this value, never the live Statistic.
// Setters may keep mutating the widget while raster paints — the pixels
// always match the last UI refresh, with no cross-thread reads or writes
// of widget state.
type StatSnap struct {
	// Resolved style at refresh time (Effective* applied against theme).
	TitleFS    float64
	ContentFS  float64
	Gap        float64
	TitleCol   theme.Color
	ContentCol theme.Color
	// Skeleton colors frozen at refresh time (loading stays the atomic
	// read on the widget).
	SkelFill theme.Color
	SkelRad  float64
	// Content frozen at refresh time; custom node pointers are snapshot
	// copies (paint never reads the live pointer fields).
	HasTitle   bool
	Title      string
	TitleNode  rendering.RenderObject
	Prefix     string
	PrefixNode rendering.RenderObject
	ValueStr   string
	Suffix     string
	SuffixNode rendering.RenderObject
	HasPrefix  bool
	HasSuffix  bool
	TextFace   text.Face
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from markLayoutDirty and markPaintDirty). Pure value build: no
// raster-side field is touched, so a concurrent paint of the previous
// snapshot is unaffected.
func (s *Statistic) refreshSnapshot() {
	if s == nil {
		return
	}
	tok := s.tokens()
	s.snap.Store(StatSnap{
		TitleFS:    s.EffectiveTitleFontSize(),
		ContentFS:  s.EffectiveContentFontSize(),
		Gap:        s.Gap(),
		TitleCol:   s.EffectiveTitleColor(),
		ContentCol: s.EffectiveContentColor(),
		SkelFill:   tok.ColorFillSecondary,
		SkelRad:    tok.Radius,
		HasTitle:   s.HasTitle(),
		Title:      s.title,
		TitleNode:  s.titleNode,
		Prefix:     s.prefix,
		PrefixNode: s.prefixNode,
		ValueStr:   s.DisplayText(),
		Suffix:     s.suffix,
		SuffixNode: s.suffixNode,
		HasPrefix:  s.HasPrefix(),
		HasSuffix:  s.HasSuffix(),
		TextFace:   s.textFace,
	})
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (empty strings, zero-alpha colors).
func (s *Statistic) loadSnapshot() StatSnap {
	if s == nil {
		return StatSnap{}
	}
	if v, ok := s.snap.Load().(StatSnap); ok {
		return v
	}
	return StatSnap{}
}
