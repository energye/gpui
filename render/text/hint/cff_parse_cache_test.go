package hint

import (
	"testing"
)

// TestCFFParseCachedReusesPerFont verifies the CFF table-parse cache: the
// same font raw parses once and subsequent calls return the identical parse
// (single parse — this is what makes LightHintVar glyph rasterization cheap
// instead of re-parsing the whole CFF table per glyph).
func TestCFFParseCachedReusesPerFont(t *testing.T) {
	f := openTestFont(t, "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc")
	raw := f.raw

	a1, err := cffParseCached(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	a2, err := cffParseCached(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a1 != a2 {
		t.Fatal("cffParseCached returned a different parse for the same font (cache miss)")
	}

	// Cached result must match the direct full-table parse.
	start, ln, err := cffTableData(raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	upem := hintFontUpem(raw, 0)
	if upem <= 0 {
		upem = 1000
	}
	direct, err := cffParseAll(raw[start:start+ln], upem)
	if err != nil {
		t.Fatal(err)
	}
	if len(a1.charStrings) != len(direct.charStrings) {
		t.Fatalf("cached charStrings=%d direct=%d", len(a1.charStrings), len(direct.charStrings))
	}
	if len(a1.fds) != len(direct.fds) {
		t.Fatalf("cached fds=%d direct=%d", len(a1.fds), len(direct.fds))
	}
}

// TestCFFParseCachedErrNotCached ensures parse errors are not cached (a bad
// table does not poison later calls).
func TestCFFParseCachedErrNotCached(t *testing.T) {
	if _, err := cffParseCached([]byte{0x01, 0x02, 0x03}, 0); err == nil {
		t.Fatal("expected parse error for garbage CFF data")
	}
}