// Command refcapture installs composition ref PNGs + aggregated standard catalog.
//
// Layout (fixed names — not suite-specific prefixes):
//
//	testdata/refs/standard.json   # all case meta + default policy
//	testdata/refs/standard/*.png  # images only
//
// Typical:
//
//	go test ./render -run TestP1_Comp_ -count=1
//	go run ./stdgate/cmd/refcapture -src tmp/comp
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/energye/gpui/stdgate/compare"
)

func main() {
	src := flag.String("src", "tmp/comp", "directory with D*.png from composition tests")
	catalog := flag.String("catalog", "testdata/refs/standard.json", "aggregated standard catalog JSON")
	images := flag.String("images", "testdata/refs/standard", "PNG-only directory")
	oracle := flag.String("oracle", "gpui-capture", "oracle label stored in catalog")
	note := flag.String("note", "Frozen regression baseline (standard data). Prefer replacing with canvaskit oracle when scenes are ported.", "catalog note")
	flag.Parse()

	if err := run(*src, *catalog, *images, *oracle, *note); err != nil {
		fmt.Fprintf(os.Stderr, "refcapture: %v\n", err)
		os.Exit(1)
	}
}

func run(src, catalogPath, imagesDir, oracle, note string) error {
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return err
	}

	matches, err := filepath.Glob(filepath.Join(src, "D*.png"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no D*.png under %s — run TestP1_Comp_ first", src)
	}
	sort.Strings(matches)

	var cat *compare.Catalog
	if _, err := os.Stat(catalogPath); err == nil {
		cat, err = compare.LoadCatalog(catalogPath)
		if err != nil {
			return err
		}
	} else {
		cat = &compare.Catalog{
			Name:        "standard",
			Cases:       map[string]compare.CaseMeta{},
			DefaultDiff: compare.DefaultPolicy(),
		}
	}
	cat.Name = "standard"
	cat.Oracle = oracle
	cat.OracleNote = note
	cat.DefaultDiff = compare.DefaultPolicy()
	cat.ImagesDir = "standard"
	// if images dir is not sibling "standard", store relative name when under same parent
	if filepath.Base(imagesDir) != "standard" {
		cat.ImagesDir = filepath.Base(imagesDir)
	}
	cat.GeneratedAt = time.Now().Format(time.RFC3339)

	n := 0
	for _, p := range matches {
		base := filepath.Base(p)
		id := strings.TrimSuffix(base, ".png")
		outPNG := filepath.Join(imagesDir, base)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPNG, b, 0o644); err != nil {
			return err
		}
		w, h := 0, 0
		if f, err := os.Open(outPNG); err == nil {
			cfg, err2 := png.DecodeConfig(f)
			_ = f.Close()
			if err2 == nil {
				w, h = cfg.Width, cfg.Height
			}
		}
		meta := compare.CaseMeta{
			ID:     id,
			Oracle: oracle,
			Width:  w,
			Height: h,
			Diff:   cat.DefaultDiff,
			Note:   note,
			File:   base,
		}
		if old, ok := cat.Cases[id]; ok {
			if old.Diff.MaxMeanAbs > 0 || old.Diff.MaxRMSE > 0 {
				meta.Diff = old.Diff
			}
		}
		cat.UpsertCase(meta)
		n++
	}
	cat.CaseCount = len(cat.Cases)
	if err := compare.WriteCatalog(catalogPath, cat); err != nil {
		return err
	}
	fmt.Printf("refcapture: wrote %d png → %s ; catalog %s (oracle=%s)\n", n, imagesDir, catalogPath, oracle)
	return nil
}
