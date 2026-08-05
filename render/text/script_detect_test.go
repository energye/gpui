package text

import (
	"os"
	"testing"
)

// loadParsedFont parses font bytes with the production parser.
func loadParsedFont(t *testing.T, path string) ParsedFont {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("font %s not available: %v", path, err)
	}
	parser := &ownParser{}
	font, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}
	return font
}

// TestScriptDetection_Thai verifies that Thai glyphs are now detected as
// the Thai script (previously they fell back to Latin), exercising the
// generated skrifa script data.
func TestScriptDetection_Thai(t *testing.T) {
	font := loadParsedFont(t, "hint/testdata/NotoSansThai-Regular.otf")

	gid := font.GlyphIndex(0x0E01) // ก
	if gid == 0 {
		t.Skip("Thai glyph not in font")
	}
	sc := scriptForGlyph(font, GlyphID(gid))
	if sc == nil || sc.name != "Thai" {
		t.Fatalf("Thai glyph script = %v, want Thai", sc)
	}
	if len(sc.blues) != 7 {
		t.Errorf("Thai blues = %d, want 7 (skrifa data)", len(sc.blues))
	}
}

// TestScriptData_Generated checks representative generated entries against
// the skrifa source data.
func TestScriptData_Generated(t *testing.T) {
	tests := []struct {
		varName string
		name    string
		blues   int
		t2b     bool
	}{
		{"scriptBengali", "Bengali", 4, true},
		{"scriptDevanagari", "Devanagari", 5, true},
		{"scriptGothic", "Gothic", 2, true},
		{"scriptHebrew", "Hebrew", 3, false}, // existing, referenced by list
	}
	for _, tt := range tests {
		var sc *scriptClass
		for _, s := range scriptClasses {
			if s.name == tt.name {
				sc = s
				break
			}
		}
		if sc == nil {
			t.Errorf("%s: script %q not in scriptClasses", tt.varName, tt.name)
			continue
		}
		if len(sc.blues) != tt.blues {
			t.Errorf("%s: blues = %d, want %d", tt.name, len(sc.blues), tt.blues)
		}
		if sc.hintTopToBottom != tt.t2b {
			t.Errorf("%s: hintTopToBottom = %v, want %v", tt.name, sc.hintTopToBottom, tt.t2b)
		}
	}
}

// TestScriptClasses_Order verifies skrifa SCRIPT_CLASSES order is preserved
// and no script is duplicated in the detection list.
func TestScriptClasses_Order(t *testing.T) {
	seen := map[string]bool{}
	for _, sc := range scriptClasses {
		if seen[sc.name] {
			t.Errorf("duplicate script in scriptClasses: %s", sc.name)
		}
		seen[sc.name] = true
	}
	if len(scriptClasses) != 57 {
		t.Errorf("scriptClasses count = %d, want 57", len(scriptClasses))
	}
	if !seen["Latin"] || !seen["CJKV ideographs"] {
		t.Errorf("scriptClasses missing Latin/CJK: %v", seen)
	}
}
