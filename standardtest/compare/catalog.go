package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Catalog is a single-file ref index (e.g. standardtest/catalog.json)
// with PNGs stored separately under ImagesDir (e.g. standard/).
type Catalog struct {
	Name        string              `json:"name"`
	Oracle      string              `json:"oracle"`
	OracleNote  string              `json:"oracle_note,omitempty"`
	CaseCount   int                 `json:"case_count"`
	DefaultDiff Policy              `json:"default_diff"`
	GeneratedAt string              `json:"generated_at,omitempty"`
	ImagesDir   string              `json:"images_dir"` // relative to catalog file directory
	Cases       map[string]CaseMeta `json:"cases"`

	// Path is the catalog JSON path (not serialized).
	Path string `json:"-"`
	// Root is the directory containing the catalog JSON (not serialized).
	Root string `json:"-"`
}

// LoadCatalog reads an aggregated refs JSON.
func LoadCatalog(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	c.Path = path
	c.Root = filepath.Dir(path)
	if c.Cases == nil {
		c.Cases = map[string]CaseMeta{}
	}
	c.DefaultDiff = normalizePolicy(c.DefaultDiff)
	for id, meta := range c.Cases {
		if meta.ID == "" {
			meta.ID = id
		}
		if meta.File == "" {
			meta.File = id + ".png"
		}
		if meta.Oracle == "" {
			meta.Oracle = c.Oracle
		}
		meta.Diff = normalizePolicy(meta.Diff)
		// if case has empty-looking policy zeros, fall back already handled by normalize;
		// merge missing thresholds from default
		meta.Diff = mergePolicy(c.DefaultDiff, meta.Diff)
		c.Cases[id] = meta
	}
	if c.CaseCount == 0 {
		c.CaseCount = len(c.Cases)
	}
	return &c, nil
}

// WriteCatalog writes the catalog JSON (indented). Path fields are not written.
func WriteCatalog(path string, c *Catalog) error {
	if c == nil {
		return fmt.Errorf("nil catalog")
	}
	out := *c
	out.Path = ""
	out.Root = ""
	if out.CaseCount == 0 {
		out.CaseCount = len(out.Cases)
	}
	b, err := json.MarshalIndent(&out, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ImageDir returns absolute directory of PNGs.
func (c *Catalog) ImageDir() string {
	if c == nil {
		return ""
	}
	if c.ImagesDir == "" {
		return c.Root
	}
	if filepath.IsAbs(c.ImagesDir) {
		return c.ImagesDir
	}
	return filepath.Join(c.Root, c.ImagesDir)
}

// RefPNG returns absolute path of the expected PNG for id.
func (c *Catalog) RefPNG(id string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nil catalog")
	}
	meta, ok := c.Cases[id]
	if !ok {
		// allow file-only refs not yet in catalog
		p := filepath.Join(c.ImageDir(), id+".png")
		return p, nil
	}
	name := meta.File
	if name == "" {
		name = id + ".png"
	}
	return filepath.Join(c.ImageDir(), name), nil
}

// Case returns meta for id, synthesizing defaults when missing.
func (c *Catalog) Case(id string) CaseMeta {
	if c != nil {
		if m, ok := c.Cases[id]; ok {
			return m
		}
	}
	pol := DefaultPolicy()
	if c != nil {
		pol = mergePolicy(c.DefaultDiff, pol)
	}
	oracle := "unknown"
	if c != nil && c.Oracle != "" {
		oracle = c.Oracle
	}
	return CaseMeta{
		ID:     id,
		Oracle: oracle,
		Diff:   pol,
		File:   id + ".png",
	}
}

// CompareNamed compares actualPath against catalog entry id.
func (c *Catalog) CompareNamed(id, actualPath string) (Result, CaseMeta, error) {
	meta := c.Case(id)
	ref, err := c.RefPNG(id)
	if err != nil {
		return Result{}, meta, err
	}
	if _, err := os.Stat(ref); err != nil {
		return Result{}, meta, fmt.Errorf("missing ref png %s: %w", ref, err)
	}
	res, err := Files(ref, actualPath, meta.Diff)
	return res, meta, err
}

// UpsertCase sets/replaces a case entry (in-memory).
func (c *Catalog) UpsertCase(meta CaseMeta) {
	if c.Cases == nil {
		c.Cases = map[string]CaseMeta{}
	}
	if meta.ID == "" {
		return
	}
	if meta.File == "" {
		meta.File = meta.ID + ".png"
	}
	meta.Diff = normalizePolicy(meta.Diff)
	c.Cases[meta.ID] = meta
	c.CaseCount = len(c.Cases)
}

// mergePolicy fills zero fields in over from base.
func mergePolicy(base, over Policy) Policy {
	base = normalizePolicy(base)
	// over already partially normalized; re-apply zeros from base explicitly
	if over.MaxMeanAbs <= 0 {
		over.MaxMeanAbs = base.MaxMeanAbs
	}
	if over.MaxRMSE <= 0 {
		over.MaxRMSE = base.MaxRMSE
	}
	if over.MaxDelta == 0 {
		over.MaxDelta = base.MaxDelta
	}
	if over.ChangedRatioMax <= 0 {
		over.ChangedRatioMax = base.ChangedRatioMax
	}
	if over.PerChannelTol == 0 {
		over.PerChannelTol = base.PerChannelTol
	}
	return over
}
