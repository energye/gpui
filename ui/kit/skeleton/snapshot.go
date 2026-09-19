package skeleton

import (
	"github.com/energye/gpui/render"
)

// R2-6 button snapshot paradigm for the skeleton family. All six components
// freeze their paint inputs in an atomic Value on the UI thread; raster paint
// reads only the snapshot plus the atomic event/animation state (active /
// loading / phase). Widths that depend on the raster box width stay configs
// in the snapshot and resolve against the paint-time size param — pure math,
// still deterministic per snapshot.

// SkeletonSnap freezes every Skeleton paint input.
type SkeletonSnap struct {
	HasAvatar     bool
	HasTitle      bool
	HasParagraph  bool
	ParagraphRows int
	Rows          int // EffectiveRows resolved at refresh (0 = no paragraph)
	Round         bool
	Radius        float64 // EffectiveRadius at refresh
	AvatarShape   SkeletonAvatarShape
	AvatarSizePx  float64 // EffectiveAvatarSize at refresh
	TitleWidth    float64
	TitleWidthStr string
	ParagraphWidths    []float64 // frozen copies
	ParagraphWidthStrs []string
	Fill         render.RGBA
	TitleHeight  float64
	RowHeight    float64
	ReduceMotion bool
}

// SkeletonAvatarSnap freezes SkeletonAvatar paint inputs.
type SkeletonAvatarSnap struct {
	SizePx       float64
	Shape        SkeletonAvatarShape
	Fill         render.RGBA
	ReduceMotion bool
}

// SkeletonButtonSnap freezes SkeletonButton paint inputs.
type SkeletonButtonSnap struct {
	Shape        SkeletonButtonShape
	Radius       float64
	Fill         render.RGBA
	ReduceMotion bool
}

// SkeletonInputSnap freezes SkeletonInput paint inputs.
type SkeletonInputSnap struct {
	Fill         render.RGBA
	ReduceMotion bool
}

// SkeletonImageSnap freezes SkeletonImage paint inputs.
type SkeletonImageSnap struct {
	Fill         render.RGBA
	ReduceMotion bool
}

// SkeletonNodeSnap freezes SkeletonNode paint inputs.
type SkeletonNodeSnap struct {
	HasChild     bool
	Fill         render.RGBA
	ReduceMotion bool
}

// refreshSnapshot freezes the current paint inputs (UI thread only).
func (s *Skeleton) refreshSnapshot() {
	if s == nil {
		return
	}
	v := SkeletonSnap{
		HasAvatar:     s.hasAvatar,
		HasTitle:      s.hasTitle,
		HasParagraph:  s.hasParagraph,
		ParagraphRows: s.paragraphRows,
		Rows:          s.EffectiveRows(),
		Round:         s.round,
		Radius:        s.EffectiveRadius(),
		AvatarShape:   s.AvatarShape(),
		AvatarSizePx:  s.EffectiveAvatarSize(),
		TitleWidth:    s.titleWidth,
		TitleWidthStr: s.titleWidthStr,
		Fill:          s.EffectiveFillColor(),
		TitleHeight:   s.EffectiveTitleHeight(),
		RowHeight:     s.EffectiveRowHeight(),
		ReduceMotion:  s.reduceMotion,
	}
	if len(s.paragraphWidths) > 0 {
		v.ParagraphWidths = append([]float64(nil), s.paragraphWidths...)
	}
	if len(s.paragraphWidthStrs) > 0 {
		v.ParagraphWidthStrs = append([]string(nil), s.paragraphWidthStrs...)
	}
	s.snap.Store(v)
}

// loadSnapshot returns the last frozen snapshot (zero = defaults off).
func (s *Skeleton) loadSnapshot() SkeletonSnap {
	if s == nil {
		return SkeletonSnap{}
	}
	if v, ok := s.snap.Load().(SkeletonSnap); ok {
		return v
	}
	return SkeletonSnap{}
}

// rightAvailSnap mirrors rightAvail against the frozen snapshot.
func (s *Skeleton) rightAvailSnap(S SkeletonSnap, totalW float64) float64 {
	if S.HasAvatar {
		avail := totalW - S.AvatarSizePx - AvatarGap
		if avail < 0 {
			return 0
		}
		return avail
	}
	if totalW < 0 {
		return 0
	}
	return totalW
}

// effTitleWidthSnap mirrors EffectiveTitleWidth against the snapshot.
func (s *Skeleton) effTitleWidthSnap(S SkeletonSnap, totalW float64) float64 {
	if !S.HasTitle {
		return 0
	}
	avail := s.rightAvailSnap(S, totalW)
	if S.TitleWidthStr != "" {
		if v, ok := parseWidthStr(S.TitleWidthStr, avail); ok {
			return v
		}
	}
	if S.TitleWidth > 0 {
		return resolveWidth(S.TitleWidth, avail)
	}
	if !S.HasParagraph {
		return avail
	}
	if S.HasAvatar {
		return avail * TitleAvatarRat
	}
	return avail * TitleNoAvatarRat
}

// effParagraphWidthsSnap mirrors EffectiveParagraphWidths against the
// snapshot (rows resolved at refresh).
func (s *Skeleton) effParagraphWidthsSnap(S SkeletonSnap, totalW float64) []float64 {
	if S.Rows <= 0 {
		return nil
	}
	avail := s.rightAvailSnap(S, totalW)
	out := make([]float64, S.Rows)
	if len(S.ParagraphWidthStrs) > 0 {
		ex := S.ParagraphWidthStrs
		switch {
		case len(ex) == 1:
			for i := range out {
				if i == S.Rows-1 {
					if v, ok := parseWidthStr(ex[0], avail); ok {
						out[i] = v
					} else {
						out[i] = avail * LastRowRatio
					}
				} else {
					out[i] = avail
				}
			}
		default:
			for i := range out {
				if i < len(ex) && ex[i] != "" {
					if v, ok := parseWidthStr(ex[i], avail); ok {
						out[i] = v
						continue
					}
				}
				if i == S.Rows-1 {
					out[i] = avail * LastRowRatio
				} else {
					out[i] = avail
				}
			}
		}
		return out
	}
	ex := S.ParagraphWidths
	switch {
	case len(ex) == 1:
		for i := range out {
			if i == S.Rows-1 {
				if ex[0] <= 0 {
					out[i] = avail * LastRowRatio
				} else {
					out[i] = resolveWidth(ex[0], avail)
				}
			} else {
				out[i] = avail
			}
		}
	case len(ex) > 1:
		for i := range out {
			if i < len(ex) && ex[i] > 0 {
				out[i] = resolveWidth(ex[i], avail)
			} else if i == S.Rows-1 {
				out[i] = avail * LastRowRatio
			} else {
				out[i] = avail
			}
		}
	default:
		for i := range out {
			if i == S.Rows-1 {
				out[i] = avail * LastRowRatio
			} else {
				out[i] = avail
			}
		}
	}
	return out
}

func (a *SkeletonAvatar) refreshSnapshot() {
	if a == nil {
		return
	}
	a.snap.Store(SkeletonAvatarSnap{
		SizePx:       a.EffectiveSize(),
		Shape:        a.Shape(),
		Fill:         a.fill(),
		ReduceMotion: a.reduceMotion,
	})
}

func (a *SkeletonAvatar) loadSnapshot() SkeletonAvatarSnap {
	if a == nil {
		return SkeletonAvatarSnap{}
	}
	if v, ok := a.snap.Load().(SkeletonAvatarSnap); ok {
		return v
	}
	return SkeletonAvatarSnap{}
}

func (b *SkeletonButton) refreshSnapshot() {
	if b == nil {
		return
	}
	b.snap.Store(SkeletonButtonSnap{
		Shape:        b.Shape(),
		Radius:       b.radius(),
		Fill:         b.fill(),
		ReduceMotion: b.reduceMotion,
	})
}

func (b *SkeletonButton) loadSnapshot() SkeletonButtonSnap {
	if b == nil {
		return SkeletonButtonSnap{}
	}
	if v, ok := b.snap.Load().(SkeletonButtonSnap); ok {
		return v
	}
	return SkeletonButtonSnap{}
}

func (in *SkeletonInput) refreshSnapshot() {
	if in == nil {
		return
	}
	in.snap.Store(SkeletonInputSnap{
		Fill:         in.fill(),
		ReduceMotion: in.reduceMotion,
	})
}

func (in *SkeletonInput) loadSnapshot() SkeletonInputSnap {
	if in == nil {
		return SkeletonInputSnap{}
	}
	if v, ok := in.snap.Load().(SkeletonInputSnap); ok {
		return v
	}
	return SkeletonInputSnap{}
}

func (im *SkeletonImage) refreshSnapshot() {
	if im == nil {
		return
	}
	im.snap.Store(SkeletonImageSnap{
		Fill:         im.fill(),
		ReduceMotion: im.reduceMotion,
	})
}

func (im *SkeletonImage) loadSnapshot() SkeletonImageSnap {
	if im == nil {
		return SkeletonImageSnap{}
	}
	if v, ok := im.snap.Load().(SkeletonImageSnap); ok {
		return v
	}
	return SkeletonImageSnap{}
}

func (n *SkeletonNode) refreshSnapshot() {
	if n == nil {
		return
	}
	n.snap.Store(SkeletonNodeSnap{
		HasChild:     n.child != nil,
		Fill:         n.fill(),
		ReduceMotion: n.reduceMotion,
	})
}

func (n *SkeletonNode) loadSnapshot() SkeletonNodeSnap {
	if n == nil {
		return SkeletonNodeSnap{}
	}
	if v, ok := n.snap.Load().(SkeletonNodeSnap); ok {
		return v
	}
	return SkeletonNodeSnap{}
}