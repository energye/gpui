package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestQRCode_EncodeMatchesCases(t *testing.T) {
	cases := loadQRCodeCases(t)
	enc := cases["encode"].(map[string]any)
	gen := kit.DefaultQRCodeGenerateConfig()
	if gen.Name() != "skip2" {
		t.Fatalf("name = %q want skip2", gen.Name())
	}
	// Empty value never crashes and yields no matrix (antd null).
	m, err := gen.Encode("", kit.QRErrorLevelM)
	if err != nil || m != nil {
		t.Fatal("empty value must return nil,nil")
	}
	// Known value hits the case-file module count.
	m, err = gen.Encode("https://ant.design", kit.QRErrorLevelM)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(m) != int(enc["antdesign_m_modules"].(float64)) {
		t.Fatalf("modules = %d want 25", len(m))
	}
	for i, row := range m {
		if len(row) != len(m) {
			t.Fatalf("row %d not square", i)
		}
	}
	// Finder pattern: dark corners, white inner ring, dark core.
	if !m[0][0] || !m[0][6] || !m[6][0] {
		t.Fatal("finder corners must be dark")
	}
	if m[1][1] {
		t.Fatal("finder inner must be white")
	}
	if !m[3][3] {
		t.Fatal("finder core must be dark")
	}
	// Determinism: same value scans the same code, twice.
	m2, err := gen.Encode("https://ant.design", kit.QRErrorLevelM)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	for i := range m {
		for j := range m[i] {
			if m[i][j] != m2[i][j] {
				t.Fatalf("non-deterministic at %d,%d", i, j)
			}
		}
	}
	// All four levels encode hello.
	for _, l := range []kit.QRErrorLevel{kit.QRErrorLevelL, kit.QRErrorLevelM, kit.QRErrorLevelQ, kit.QRErrorLevelH} {
		hm, err := gen.Encode("hello", l)
		if err != nil || len(hm) != int(enc["hello_modules"].(float64)) {
			t.Fatalf("hello level %q = %d,%v", l, len(hm), err)
		}
	}
	// Boost lifts M to H for the antd home value.
	boost := cases["boost"].(map[string]any)
	bm, used, err := kit.EncodeQRCodeValue(gen, "https://ant.design", kit.QRErrorLevelM, true)
	if err != nil {
		t.Fatalf("boost: %v", err)
	}
	if string(used) != boost["antdesign_m_boosted_level"].(string) {
		t.Fatalf("boosted level = %q want H", used)
	}
	if len(bm) != int(boost["antdesign_m_boosted_modules"].(float64)) {
		t.Fatalf("boosted modules = %d want 29", len(bm))
	}
	// No boost keeps the requested level.
	_, used, err = kit.EncodeQRCodeValue(gen, "https://ant.design", kit.QRErrorLevelM, false)
	if err != nil || used != kit.QRErrorLevelM {
		t.Fatalf("no-boost = %q,%v want M", used, err)
	}
}
