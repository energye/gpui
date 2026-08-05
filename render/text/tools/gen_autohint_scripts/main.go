// Command gen_autohint_scripts generates autohint_scripts_gen.go from
// skrifa autohint style data.
//
// Source: skrifa 0.31.1 generated/generated_autohint_styles.rs, extracted to
// ../skrifa_autohint_styles.json.
//
// Run from the repo root:
//
//	go run ./render/text/tools/gen_autohint_scripts
//
// Skips:
//   - "no script", "Latin Subscript Fallback", "Latin Superscript Fallback"
//     (no independent data; behaviorally equivalent to Latin)
//   - "CJKV ideographs" (existing scriptCJK in autohint_scripts.go, kept as-is)
//   - Latin/Hebrew/Cyrillic/Greek/Arabic (existing vars in autohint_scripts.go;
//     referenced by the generated scriptClasses list only)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	dataPath = "render/text/tools/skrifa_autohint_styles.json"
	outPath  = "render/text/autohint_scripts_gen.go"
)

// script mirrors one skrifa ScriptClass entry.
type script struct {
	Name            string      `json:"name"`
	Group           string      `json:"group"`
	HintTopToBottom bool        `json:"hint_top_to_bottom"`
	StdChars        string      `json:"std_chars"`
	Blues           []blueSpec  `json:"blues"`
	Uniranges       []runeRange `json:"uniranges"`
}

type blueSpec struct {
	Chars string `json:"chars"`
	Zones string `json:"zones"`
}

type runeRange [2]int

// existing maps script names already defined in autohint_scripts.go to
// their var names; these are referenced, not generated.
var existing = map[string]string{
	"Latin":           "scriptLatin",
	"Hebrew":          "scriptHebrew",
	"Cyrillic":        "scriptCyrillic",
	"Greek":           "scriptGreek",
	"Arabic":          "scriptArabic",
	"CJKV ideographs": "scriptCJK",
}

var skip = map[string]bool{
	"no script":                  true,
	"Latin Subscript Fallback":   true,
	"Latin Superscript Fallback": true,
}

var specialNames = map[string]string{
	"N'Ko":                 "scriptNKo",
	"Hanifi Rohingya":      "scriptHanifiRohingya",
	"Canadian Syllabics":   "scriptCanadianSyllabics",
	"Georgian (Mkhedruli)": "scriptGeorgianMkhedruli",
	"Georgian (Khutsuri)":  "scriptGeorgianKhutsuri",
	"Khmer Symbols":        "scriptKhmerSymbols",
	"Old Turkic":           "scriptOldTurkic",
	"Tai Viet":             "scriptTaiViet",
	"Kayah Li":             "scriptKayahLi",
	"Syloti Nagri":         "scriptSylotiNagri",
	"Limbu":                "scriptLimbu",
	"Oriya":                "scriptOriya",
	"Tibetan":              "scriptTibetan",
}

var zonesFlag = map[string]string{
	"TOP":        "blueZoneTop",
	"SUB_TOP":    "blueZoneSubTop",
	"NEUTRAL":    "blueZoneNeutral",
	"ADJUSTMENT": "blueZoneAdjustment",
	"X_HEIGHT":   "blueZoneXHeight",
	"LONG":       "blueZoneLong",
	// HORIZONTAL == SUB_TOP (1 << 2) and RIGHT == TOP (1 << 1) in skrifa.
	"HORIZONTAL": "blueZoneSubTop",
	"RIGHT":      "blueZoneTop",
	"NONE":       "0",
}

var groupGo = map[string]string{
	"Default": "scriptGroupDefault",
	"Indic":   "scriptGroupIndic",
	"Cjk":     "scriptGroupCJK",
}

var unionRe = regexp.MustCompile(`(?:BlueZones::)?([A-Z_]+)`)

func goVarName(name string) string {
	if v, ok := specialNames[name]; ok {
		return v
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == ' ' || r == '-'
	})
	var b strings.Builder
	b.WriteString("script")
	for _, p := range parts {
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

func escapeRune(ch rune) string {
	if ch == '|' || ch == ' ' || ch < 0x80 {
		return string(ch)
	}
	if ch > 0xFFFF {
		return fmt.Sprintf("\\U%08X", ch)
	}
	return fmt.Sprintf("\\u%04X", ch)
}

func escapeStr(s string) string {
	var b strings.Builder
	for _, ch := range s {
		b.WriteString(escapeRune(ch))
	}
	return b.String()
}

func stdCharsLiteral(stdChars string) string {
	if stdChars == "" {
		return "nil"
	}
	parts := make([]string, 0, 8)
	for _, ch := range strings.Fields(stdChars) {
		r := []rune(ch)[0]
		if r > 0xFFFF {
			parts = append(parts, fmt.Sprintf("'\\U%08X'", r))
		} else {
			parts = append(parts, fmt.Sprintf("'\\u%04X'", r))
		}
	}
	return "[]rune{" + strings.Join(parts, ", ") + "}"
}

func flagsExpr(zones string) string {
	var flags []string
	for _, m := range unionRe.FindAllStringSubmatch(zones, -1) {
		name := m[1]
		if name == "NONE" {
			continue
		}
		if f, ok := zonesFlag[name]; ok {
			flags = append(flags, f)
		}
	}
	if len(flags) == 0 {
		return "0"
	}
	return strings.Join(flags, " | ")
}

func bluesLiteral(blues []blueSpec) string {
	if len(blues) == 0 {
		return "nil"
	}
	out := make([]string, 0, len(blues))
	for _, b := range blues {
		out = append(out, fmt.Sprintf("\t\t{chars: \"%s\", flags: %s},",
			escapeStr(b.Chars), flagsExpr(b.Zones)))
	}
	return "\n" + strings.Join(out, "\n")
}

func unirangesLiteral(rs []runeRange) string {
	if len(rs) == 0 {
		return "nil"
	}
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, fmt.Sprintf("\t\tuniRange(0x%X, 0x%X),", r[0], r[1]))
	}
	return "\n" + strings.Join(out, "\n")
}

func main() {
	data, err := os.ReadFile(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", dataPath, err)
		os.Exit(1)
	}
	var scripts []script
	if err := json.Unmarshal(data, &scripts); err != nil {
		fmt.Fprintf(os.Stderr, "parse %s: %v\n", dataPath, err)
		os.Exit(1)
	}

	var vars []string
	var refs []string
	for _, sc := range scripts {
		if skip[sc.Name] {
			continue
		}
		if v, ok := existing[sc.Name]; ok {
			refs = append(refs, v)
			continue
		}
		group, ok := groupGo[sc.Group]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown group %q for %s\n", sc.Group, sc.Name)
			os.Exit(1)
		}
		t2b := "false"
		if sc.HintTopToBottom {
			t2b = "true"
		}
		uni := unirangesLiteral(sc.Uniranges)
		uniLine := "\tuniranges:        nil,\n"
		if uni != "nil" {
			uniLine = fmt.Sprintf("\tuniranges:        []runePair{%s\n\t},\n", uni)
		}
		blues := bluesLiteral(sc.Blues)
		bluesLine := "\tblues:            nil,\n"
		if blues != "nil" {
			bluesLine = fmt.Sprintf("\tblues:            []blueSpec{%s\n\t},\n", blues)
		}
		varName := goVarName(sc.Name)
		vars = append(vars, fmt.Sprintf(
			"var %s = scriptClass{\n"+
				"\tname:             \"%s\",\n"+
				"\tgroup:            %s,\n"+
				"\thintTopToBottom:  %s,\n"+
				"\tstdChars:         %s,\n"+
				uniLine+bluesLine+
				"}",
			varName, sc.Name, group, t2b, stdCharsLiteral(sc.StdChars)))
		refs = append(refs, varName)
	}

	refsLiteral := joinRefs(refs)

	header := `// Code generated by render/text/tools/gen_autohint_scripts (gen_autohint_scripts.go).
// DO NOT EDIT.
//
// Auto-hint script class data ported from skrifa 0.31.1
// generated/generated_autohint_styles.rs (ScriptClass table).
// Regenerate with: go run ./render/text/tools/gen_autohint_scripts

package text

`

	body := header + strings.Join(vars, "\n\n") + "\n\n" +
		`// scriptClasses lists all supported scripts in skrifa SCRIPT_CLASSES order
// (Latin/Hebrew/Cyrillic/Greek/Arabic and CJK referenced from
// autohint_scripts.go; the rest generated here). See autohint_scripts.go
// perGlyphScripts for how this list drives per-glyph detection.
var scriptClasses = []*scriptClass{
` + refsLiteral + "}\n"

	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", dir, err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, []byte(body), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
		os.Exit(1)
	}
	if out, err := exec.Command("gofmt", "-w", outPath).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "gofmt %s: %v\n%s\n", outPath, err, out)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%d vars)\n", outPath, len(vars))
}

func joinRefs(refs []string) string {
	var b strings.Builder
	for _, r := range refs {
		b.WriteString("\t&" + r + ",\n")
	}
	return b.String()
}
