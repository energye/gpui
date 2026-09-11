package text

import (
	"iter"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// MultiFace combines multiple faces with fallback.
// Glyph lookup is first-match with one script correction: a CJK-covering
// face (it also bundles Latin/Greek/Cyrillic) is skipped for non-CJK
// runes when a later non-CJK face covers them. CJK fonts grid their
// bundled Latin on fractional advances, which jitters same-line spacing
// on the integer pixel grid; routing Latin to its own hinted face keeps advances
// near integers (Pango/fontconfig per-script matching). CJK-related runes
// (Han, Kana, Hangul, Bopomofo, CJK punctuation, fullwidth forms) always
// stay on the first match so CJK grid widths never change.
// MultiFace is safe for concurrent use.
type MultiFace struct {
	faces     []Face
	direction Direction
	// cjkCover marks component faces covering CJK Unified Ideographs
	// (computed once in NewMultiFace, read-only afterwards).
	cjkCover []bool
	// runeFaceCache avoids repeated HasGlyph scans for 5000-char
	// single-line viewports (Flutter RenderEditable pattern). The text
	// repeats a small rune alphabet (e.g. "你好Hello世界" 9 runes) and
	// re-resolving 5000 times on every keystroke dominates layout.
	// Cache is keyed by rune, safe for concurrent reads via sync.Map.
	runeFaceCache sync.Map // map[rune]Face
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
		cjkCover:  cjkCoverFlags(faces),
	}, nil
}

// Metrics implements Face.Metrics.
// Returns metrics from the first face.
func (m *MultiFace) Metrics() Metrics {
	return m.faces[0].Metrics()
}

// iterGlyphs walks text, resolving each rune to the face that covers it and
// yielding one Glyph per rune with cumulative X (same layout rules as
// sourceFace.iterGlyphs). Per-rune glyph lookup goes through glyphForRune to
// avoid a per-rune string allocation (P7). Shared by Advance, Glyphs and
// AppendGlyphs so the three cannot diverge.
func (m *MultiFace) iterGlyphs(text string, visit func(g Glyph) bool) {
	x := 0.0
	byteIndex := 0

	for i, r := range text {
		face := m.faceForRune(r)
		glyph, ok := glyphForRune(face, r, byteIndex, i)
		if !ok {
			byteIndex += utf8.RuneLen(r)
			continue
		}

		// Position the glyph in the full-text run. Y stays zero (same as the
		// previous per-rune single-glyph iterators).
		glyph.X = x
		glyph.OriginX = x

		if !visit(glyph) {
			return
		}

		x += glyph.Advance
		byteIndex += utf8.RuneLen(r)
	}
}

// Advance implements Face.Advance.
// Calculates total advance using the appropriate face for each rune.
func (m *MultiFace) Advance(text string) float64 {
	totalAdvance := 0.0
	m.iterGlyphs(text, func(g Glyph) bool {
		totalAdvance += g.Advance
		return true
	})
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
		m.iterGlyphs(text, yield)
	}
}

// AppendGlyphs implements Face.AppendGlyphs.
// Appends glyphs using the appropriate face for each rune.
func (m *MultiFace) AppendGlyphs(dst []Glyph, text string) []Glyph {
	m.iterGlyphs(text, func(g Glyph) bool {
		dst = append(dst, g)
		return true
	})
	return dst
}

// glyphForRune resolves the rune to the covering face and builds its single
// Glyph (zero position), without a per-rune string allocation (P7).
func (m *MultiFace) glyphForRune(r rune, byteIndex, cluster int) (Glyph, bool) {
	face := m.faceForRune(r)
	return glyphForRune(face, r, byteIndex, cluster)
}

// glyphForRune builds the single Glyph for r (zero position) on any concrete
// face, without a per-rune string allocation; ok=false for skipped control
// characters or filtered runes. Used by composite iterators so mixed-script
// text does not allocate string(r) per rune (P7). Unknown Face
// implementations (e.g. test mocks, custom faces) fall back to the
// single-rune Glyphs iterator, preserving the previous behavior.
func glyphForRune(face Face, r rune, byteIndex, cluster int) (Glyph, bool) {
	switch f := face.(type) {
	case *sourceFace:
		return f.glyphForRune(r, byteIndex, cluster)
	case *FilteredFace:
		return f.glyphForRune(r, byteIndex, cluster)
	case *MultiFace:
		return f.glyphForRune(r, byteIndex, cluster)
	}
	for glyph := range face.Glyphs(string(r)) {
		glyph.Index = byteIndex
		glyph.Cluster = cluster
		return glyph, true
	}
	return Glyph{}, false
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

// faceOptionsOf returns FaceOptions preserving a face's rendering
// configuration (direction/hinting/features/variations/language) when the
// face is re-derived. AtSize previously dropped every option — a MultiFace
// pinned to HintingVertical (FT light) came back as the engine default
// HintingFull after a size change.
func faceOptionsOf(f Face) []FaceOption {
	opts := make([]FaceOption, 0, 6)
	opts = append(opts, WithDirection(f.Direction()), WithHinting(f.Hinting()))
	if feats := f.Features(); len(feats) > 0 {
		opts = append(opts, WithFeatures(feats...))
	}
	if vars := f.Variations(); len(vars) > 0 {
		opts = append(opts, WithVariations(vars...))
	}
	if lang := f.Language(); lang != "" {
		opts = append(opts, WithLanguage(lang))
	}
	return opts
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
			out = append(out, src.Face(size, faceOptionsOf(f)...))
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

// WithHinting returns a new MultiFace with every component face re-derived at
// its current size with the given hinting mode; all other rendering options
// (features/variations/language/direction) are preserved. UI chrome uses this
// to pin its faces to the production hinting mode (HintingVertical = FT
// light) instead of the engine default HintingFull.
func (m *MultiFace) WithHinting(h Hinting) *MultiFace {
	if m == nil || len(m.faces) == 0 {
		return m
	}
	out := make([]Face, 0, len(m.faces))
	for _, f := range m.faces {
		if f == nil {
			continue
		}
		if src := f.Source(); src != nil {
			out = append(out, src.Face(f.Size(), append(faceOptionsOf(f), WithHinting(h))...))
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
func (m *MultiFace) Hinting() Hinting {
	if m == nil || len(m.faces) == 0 {
		return HintingNone
	}
	return m.faces[0].Hinting()
}

func (m *MultiFace) private() {}

// cjkProbe is covered by virtually every CJK font and by no Latin font.
const cjkProbe = '永'

// cjkCoverFlags reports per face whether it covers CJK Unified Ideographs.
func cjkCoverFlags(faces []Face) []bool {
	out := make([]bool, len(faces))
	for i, f := range faces {
		if f == nil {
			continue
		}
		out[i] = f.HasGlyph(cjkProbe)
	}
	return out
}

// cjkRelated reports runes that must stay on the first-match face so CJK
// grid widths never change: Han, Kana, Hangul, Bopomofo, CJK radicals and
// punctuation blocks, and the fullwidth block (fullwidth Latin must keep
// the CJK grid advance, never the proportional Latin one).
func cjkRelated(r rune) bool {
	if unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Bopomofo, r) {
		return true
	}
	switch {
	case r >= 0x2E80 && r <= 0x2FDF, // CJK radicals, Kangxi, description
		r >= 0x3000 && r <= 0x303F,   // CJK symbols and punctuation
		r >= 0x31C0 && r <= 0x31EF,   // CJK strokes
		r >= 0x3200 && r <= 0x32FF,   // enclosed CJK
		r >= 0x3300 && r <= 0x33FF,   // CJK compatibility
		r >= 0x3400 && r <= 0x4DBF,   // Ext A (stdlib Han may lag newest blocks)
		r >= 0xFE30 && r <= 0xFE4F,   // CJK compatibility forms
		r >= 0xFF00 && r <= 0xFFEF,   // fullwidth / halfwidth forms
		r >= 0x20000 && r <= 0x323AF: // Ext B and later planes
		return true
	}
	return false
}

// faceForRune returns the face for the rune: first match, except a
// CJK-covering face is skipped for non-CJK runes when a later non-CJK
// face covers them (script preference, see MultiFace). If no face has
// the glyph, falls back to the system font index (fontscan, M3) and then
// to the first face.
func (m *MultiFace) faceForRune(r rune) Face {
	if v, ok := m.runeFaceCache.Load(r); ok {
		if f, ok2 := v.(Face); ok2 && f != nil {
			return f
		}
	}
	first := -1
	for i, face := range m.faces {
		if face.HasGlyph(r) {
			first = i
			break
		}
	}
	if first >= 0 && len(m.cjkCover) == len(m.faces) &&
		m.cjkCover[first] && !cjkRelated(r) {
		for i := first + 1; i < len(m.faces); i++ {
			f := m.faces[i]
			if f == nil || (i < len(m.cjkCover) && m.cjkCover[i]) {
				continue
			}
			if f.HasGlyph(r) {
				m.runeFaceCache.Store(r, f)
				return f
			}
		}
	}
	if first < 0 {
		// M3: missing-glyph fallback against the system font index.
		if fc := globalFallback.resolveFace(r, m.Size()); fc != nil {
			m.runeFaceCache.Store(r, fc)
			return fc
		}
		// Fallback to first face if no face has the glyph
		fb := m.faces[0]
		m.runeFaceCache.Store(r, fb)
		return fb
	}
	fb := m.faces[first]
	m.runeFaceCache.Store(r, fb)
	return fb
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
		// Advance without per-rune string allocation (P7, 5000 viewport).
		x += RuneAdvance(face, r)
	}
	flush()
	return runs
}
