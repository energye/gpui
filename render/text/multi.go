package text

import (
	"iter"
	"strings"
	"sync"
	"unicode/utf8"
)

// MultiFace combines multiple faces with fallback.
// When rendering, it uses the first face that has the glyph.
// MultiFace is safe for concurrent use.
type MultiFace struct {
	faces     []Face
	direction Direction
}

// NewMultiFace creates a MultiFace from faces.
// All faces must have the same direction.
// Returns error if faces is empty or directions don't match.
func NewMultiFace(faces ...Face) (*MultiFace, error) {
	if len(faces) == 0 {
		return nil, ErrEmptyFaces
	}

	// Check that all faces have the same direction
	direction := faces[0].Direction()
	for i, face := range faces[1:] {
		if face.Direction() != direction {
			return nil, &DirectionMismatchError{
				Index:    i + 1,
				Got:      face.Direction(),
				Expected: direction,
			}
		}
	}

	return &MultiFace{
		faces:     faces,
		direction: direction,
	}, nil
}

// Metrics implements Face.Metrics.
// Returns metrics from the first face.
func (m *MultiFace) Metrics() Metrics {
	return m.faces[0].Metrics()
}

// Advance implements Face.Advance.
// Calculates total advance using the appropriate face for each rune.
func (m *MultiFace) Advance(text string) float64 {
	totalAdvance := 0.0

	for _, r := range text {
		face := m.faceForRune(r)
		// Get glyph advance from the selected face
		// We can't call Advance on the face with the full text,
		// so we need to calculate per-rune
		glyphAdvance := 0.0
		for glyph := range face.Glyphs(string(r)) {
			glyphAdvance = glyph.Advance
			break // Only one glyph for a single rune
		}
		totalAdvance += glyphAdvance
	}

	return totalAdvance
}

// HasGlyph implements Face.HasGlyph.
// Returns true if any face has the glyph.
func (m *MultiFace) HasGlyph(r rune) bool {
	for _, face := range m.faces {
		if face.HasGlyph(r) {
			return true
		}
	}
	return false
}

// Glyphs implements Face.Glyphs.
// Returns an iterator over all glyphs, using the appropriate face for each rune.
func (m *MultiFace) Glyphs(text string) iter.Seq[Glyph] {
	return func(yield func(Glyph) bool) {
		x := 0.0
		byteIndex := 0

		for i, r := range text {
			face := m.faceForRune(r)

			// Get the glyph from the selected face
			for glyph := range face.Glyphs(string(r)) {
				// Update position and index to match the full text
				glyph.X = x
				glyph.OriginX = x
				glyph.Index = byteIndex
				glyph.Cluster = i

				if !yield(glyph) {
					return
				}

				x += glyph.Advance
			}

			byteIndex += utf8.RuneLen(r)
		}
	}
}

// AppendGlyphs implements Face.AppendGlyphs.
// Appends glyphs using the appropriate face for each rune.
func (m *MultiFace) AppendGlyphs(dst []Glyph, text string) []Glyph {
	x := 0.0
	byteIndex := 0

	for i, r := range text {
		face := m.faceForRune(r)

		// Get the glyph from the selected face
		for glyph := range face.Glyphs(string(r)) {
			// Update position and index to match the full text
			glyph.X = x
			glyph.OriginX = x
			glyph.Index = byteIndex
			glyph.Cluster = i

			dst = append(dst, glyph)
			x += glyph.Advance
		}

		byteIndex += utf8.RuneLen(r)
	}

	return dst
}

// Direction implements Face.Direction.
func (m *MultiFace) Direction() Direction {
	return m.direction
}

// Source implements Face.Source.
// Returns nil since MultiFace is a composite face.
func (m *MultiFace) Source() *FontSource {
	return nil
}

// Size implements Face.Size.
// Returns the size from the first face.
func (m *MultiFace) Size() float64 {
	return m.faces[0].Size()
}

// AtSize returns a MultiFace with every component face re-derived at size.
// Source() is nil on MultiFace (composite), so callers that only do
// Source().Face(size) would keep the old size or drop CJK fallbacks.
// UI kit uses this when SetFace(14) then SetFontSize(16/20) for titles.
func (m *MultiFace) AtSize(size float64) Face {
	if m == nil || len(m.faces) == 0 {
		return m
	}
	if size <= 0 {
		return m
	}
	if fs := m.Size(); fs > 0 {
		d := fs - size
		if d < 0 {
			d = -d
		}
		if d < 0.25 {
			return m
		}
	}
	out := make([]Face, 0, len(m.faces))
	for _, f := range m.faces {
		if f == nil {
			continue
		}
		if src := f.Source(); src != nil {
			out = append(out, src.Face(size))
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		return m
	}
	mf, err := NewMultiFace(out...)
	if err != nil {
		return m
	}
	return mf
}

// Features implements Face.Features.
// Returns features from the first face.
func (m *MultiFace) Features() []FontFeature {
	return m.faces[0].Features()
}

// Language implements Face.Language.
// Returns the language from the first face.
func (m *MultiFace) Language() string {
	return m.faces[0].Language()
}

// Variations implements Face.Variations.
// Returns variations from the first face.
func (m *MultiFace) Variations() []FontVariation {
	return m.faces[0].Variations()
}

// private implements the Face interface.
func (m *MultiFace) private() {}

// faceForRune returns the first face that has the glyph for the rune.
// If no face has the glyph, returns the first face as fallback.
func (m *MultiFace) faceForRune(r rune) Face {
	for _, face := range m.faces {
		if face.HasGlyph(r) {
			return face
		}
	}
	// Fallback to first face if no face has the glyph
	return m.faces[0]
}

// FaceRun is a contiguous substring rendered with one fallback face (X.06).
type FaceRun struct {
	Face Face
	Text string
	// X is the horizontal offset of this run relative to the text origin.
	X float64
}

// multiFaceRunsCache caches MultiFace.Runs results for repeated mixed-script
// DrawString (S6.5 font-run merge reuse). Keyed by MultiFace identity + text.
type multiFaceRunsCache struct {
	mu      sync.Mutex
	entries map[multiFaceRunsKey][]FaceRun
	limit   int
}

type multiFaceRunsKey struct {
	mf   *MultiFace
	text string
}

var globalMultiFaceRunsCache = &multiFaceRunsCache{
	entries: make(map[multiFaceRunsKey][]FaceRun),
	limit:   2048,
}

// ClearMultiFaceRunsCache drops cached MultiFace run splits (tests/tuning).
func ClearMultiFaceRunsCache() {
	globalMultiFaceRunsCache.mu.Lock()
	globalMultiFaceRunsCache.entries = make(map[multiFaceRunsKey][]FaceRun)
	globalMultiFaceRunsCache.mu.Unlock()
}

// Runs splits text into contiguous face runs using the same fallback policy as Glyphs.
// S6.5: consecutive same-face runes are already merged here; results are cached
// for hot multi-script labels (CJK fallback + Latin).
func (m *MultiFace) Runs(text string) []FaceRun {
	if text == "" || m == nil || len(m.faces) == 0 {
		return nil
	}
	key := multiFaceRunsKey{mf: m, text: text}
	globalMultiFaceRunsCache.mu.Lock()
	if runs, ok := globalMultiFaceRunsCache.entries[key]; ok {
		globalMultiFaceRunsCache.mu.Unlock()
		return runs
	}
	globalMultiFaceRunsCache.mu.Unlock()

	runs := m.runsUncached(text)

	globalMultiFaceRunsCache.mu.Lock()
	if len(globalMultiFaceRunsCache.entries) >= globalMultiFaceRunsCache.limit {
		// Drop ~25% arbitrarily (map iteration order is fine for soft cache).
		n := len(globalMultiFaceRunsCache.entries) / 4
		if n < 1 {
			n = 1
		}
		for k := range globalMultiFaceRunsCache.entries {
			delete(globalMultiFaceRunsCache.entries, k)
			n--
			if n <= 0 {
				break
			}
		}
	}
	globalMultiFaceRunsCache.entries[key] = runs
	globalMultiFaceRunsCache.mu.Unlock()
	return runs
}

func (m *MultiFace) runsUncached(text string) []FaceRun {
	var runs []FaceRun
	var cur Face
	var b strings.Builder
	x := 0.0
	runX := 0.0
	flush := func() {
		if b.Len() == 0 || cur == nil {
			return
		}
		runs = append(runs, FaceRun{Face: cur, Text: b.String(), X: runX})
		b.Reset()
	}
	for _, r := range text {
		face := m.faceForRune(r)
		if cur == nil {
			cur = face
			runX = x
		} else if face != cur {
			flush()
			cur = face
			runX = x
		}
		b.WriteRune(r)
		// Advance using selected face metrics for this rune.
		// Face.Advance on a single-rune string avoids Glyphs iterator allocs.
		x += face.Advance(string(r))
	}
	flush()
	return runs
}
