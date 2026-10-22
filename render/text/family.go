//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// FontRef identifies one face inside a font file.
//
// CollectionIndex selects a face inside a .ttc/.otc collection (0 = first);
// it is ignored for single-face .ttf/.otf files.
type FontRef struct {
	Path            string
	CollectionIndex int
}

// FontFamily groups the static weight cuts of one typeface.
//
// This is the CSS/Flutter font-family model: a family owns Regular,
// SemiBold, Bold, ... sources, and [FontFamily.Face] selects the closest
// real cut for the requested [FontWeight] before falling back to synthetic
// embolden. A standalone [FontSource] has no family and keeps the old
// single-face behavior.
//
// FontFamily is safe for concurrent use after construction. It does not own
// the sources: close them through their original owner.
type FontFamily struct {
	name     string
	sources  []*FontSource // ascending by weight
	byWeight map[FontWeight]*FontSource
	regular  *FontSource
}

// NewFontFamily builds a family from static cuts of the same typeface.
// At least one source is required. Every source must report the same family
// name and a unique OS/2 weight class; duplicates are rejected so font
// configuration mistakes cannot silently pick an arbitrary cut.
func NewFontFamily(sources ...*FontSource) (*FontFamily, error) {
	clean := make([]*FontSource, 0, len(sources))
	for _, src := range sources {
		if src == nil {
			continue
		}
		clean = append(clean, src)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("text: NewFontFamily requires at least one FontSource")
	}

	fam := &FontFamily{
		name:     clean[0].Name(),
		byWeight: make(map[FontWeight]*FontSource, len(clean)),
	}
	for _, src := range clean {
		name := src.Name()
		if fam.name != "" && name != "" && !strings.EqualFold(fam.name, name) {
			return nil, fmt.Errorf("text: FontFamily %q cannot contain %q", fam.name, name)
		}
		w := src.WeightClass()
		if prev, ok := fam.byWeight[w]; ok {
			if prev == src {
				continue
			}
			return nil, fmt.Errorf("text: FontFamily %q has duplicate weight %d (%q and %q)", fam.name, w, prev.Name(), name)
		}
		fam.byWeight[w] = src
		fam.sources = append(fam.sources, src)
	}
	sort.SliceStable(fam.sources, func(i, j int) bool {
		return fam.sources[i].WeightClass() < fam.sources[j].WeightClass()
	})

	fam.regular = fam.pick(WeightRegular)
	if fam.regular == nil {
		fam.regular = fam.sources[0]
	}
	for _, src := range fam.sources {
		src.family.Store(fam)
	}
	return fam, nil
}

// LoadFontFamily loads refs and builds a FontFamily. Missing optional cuts
// (for example a Bold file that is not installed) are skipped; an error is
// returned only when no readable source remains. points is used for the
// returned base face.
func LoadFontFamily(points float64, refs ...FontRef) (Face, *FontFamily, string, error) {
	if points <= 0 {
		points = 14
	}
	sources := make([]*FontSource, 0, len(refs))
	paths := make([]string, 0, len(refs))
	var lastErr error
	for _, ref := range refs {
		if ref.Path == "" {
			continue
		}
		if _, err := os.Stat(ref.Path); err != nil {
			continue
		}
		src, err := NewFontSourceFromFile(ref.Path, WithCollectionIndex(ref.CollectionIndex))
		if err != nil {
			lastErr = err
			continue
		}
		sources = append(sources, src)
		paths = append(paths, ref.Path)
	}
	if len(sources) == 0 {
		if lastErr != nil {
			return nil, nil, "", lastErr
		}
		return nil, nil, "", fmt.Errorf("text: no font in family refs")
	}
	fam, err := NewFontFamily(sources...)
	if err != nil {
		// Keep the first readable cut usable even when the configured
		// family is inconsistent (wrong file, duplicate weight, ...).
		return sources[0].Face(points), nil, strings.Join(paths, " + "), err
	}
	return fam.Face(points), fam, strings.Join(paths, " + "), nil
}

// Name returns the OpenType family name.
func (f *FontFamily) Name() string {
	if f == nil {
		return ""
	}
	return f.name
}

// Sources returns the family cuts in ascending weight order.
func (f *FontFamily) Sources() []*FontSource {
	if f == nil {
		return nil
	}
	out := make([]*FontSource, len(f.sources))
	copy(out, f.sources)
	return out
}

// Regular returns the family's regular cut (nearest to weight 400).
func (f *FontFamily) Regular() *FontSource {
	if f == nil {
		return nil
	}
	return f.regular
}

// Face creates a face at size, selecting the real weight cut for opts.
func (f *FontFamily) Face(size float64, opts ...FaceOption) Face {
	if f == nil || f.regular == nil {
		return nil
	}
	config := defaultFaceConfig()
	for _, opt := range opts {
		opt(&config)
	}
	src := f.regular
	if config.weight != 0 && !hasWghtVariation(config.variations) {
		if picked := f.pick(config.weight); picked != nil {
			src = picked
		}
	}
	return src.Face(size, opts...)
}

// pick applies the CSS font-weight matching order:
//
//   - exact cut first;
//   - 400/500 prefer 500/400, then lighter, then heavier;
//   - < 400 prefers lighter, then heavier;
//   - > 500 prefers heavier, then lighter.
func (f *FontFamily) pick(weight FontWeight) *FontSource {
	if f == nil || len(f.sources) == 0 {
		return nil
	}
	if src := f.byWeight[weight]; src != nil {
		return src
	}

	if weight == 400 || weight == 500 {
		other := FontWeight(400)
		if weight == 400 {
			other = 500
		}
		if src := f.byWeight[other]; src != nil {
			return src
		}
	}

	// Nearest in the preferred direction, then the opposite direction.
	var below, above *FontSource
	for _, src := range f.sources {
		w := src.WeightClass()
		if w < weight {
			below = src
		} else if w > weight && above == nil {
			above = src
		}
	}
	switch {
	case weight <= 500:
		if below != nil {
			return below
		}
		return above
	default:
		if above != nil {
			return above
		}
		return below
	}
}
