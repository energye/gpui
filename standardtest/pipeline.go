package standardtest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/energye/gpui/standardtest/compare"
	"github.com/energye/gpui/standardtest/merge"
	"github.com/energye/gpui/standardtest/scene"
)

// ListSceneIDs returns scene basenames (without .json), optionally filtered by exact id.
func ListSceneIDs(scenesDir, onlyID string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(scenesDir, "*.json"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, m := range matches {
		id := strings.TrimSuffix(filepath.Base(m), ".json")
		if onlyID != "" && id != onlyID {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// GenStandard runs CanvasKit on all selected scenes into diff/standard.
func GenStandard(p Paths, onlyID string) error {
	if err := p.EnsureDiffDirs(); err != nil {
		return err
	}
	script := filepath.Join(p.CanvasKit, "gen.mjs")
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("missing %s (CanvasKit oracle)", script)
	}
	args := []string{script, "--from-scenes",
		"--scenes", p.Scenes,
		"--out", p.Standard,
		"--font-root", p.FontRoot,
	}
	if onlyID != "" {
		args = append(args, "--id", onlyID)
	}
	cmd := exec.Command("node", args...)
	cmd.Dir = p.CanvasKit
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

// GenTest renders scenes with gpui into diff/test.
func GenTest(p Paths, onlyID string) error {
	if err := p.EnsureDiffDirs(); err != nil {
		return err
	}
	ids, err := ListSceneIDs(p.Scenes, onlyID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("no scenes in %s", p.Scenes)
	}
	var fails []string
	for i, id := range ids {
		sp := filepath.Join(p.Scenes, id+".json")
		s, err := scene.Load(sp)
		if err != nil {
			fails = append(fails, id+": load "+err.Error())
			continue
		}
		dc, err := scene.RunGPUI(s, p.FontRoot)
		if err != nil {
			fails = append(fails, id+": run "+err.Error())
			continue
		}
		out := filepath.Join(p.Test, id+".png")
		if err := dc.SavePNG(out); err != nil {
			dc.Close()
			fails = append(fails, id+": save "+err.Error())
			continue
		}
		dc.Close()
		if (i+1)%25 == 0 || i == 0 {
			fmt.Printf("  test %d/%d %s\n", i+1, len(ids), id)
		}
	}
	if len(fails) > 0 {
		return fmt.Errorf("gen-test failures %d: %s", len(fails), strings.Join(fails[:min(5, len(fails))], "; "))
	}
	return nil
}

// GenMerge builds side-by-side composites into diff/merge.
func GenMerge(p Paths, onlyID string) error {
	if err := p.EnsureDiffDirs(); err != nil {
		return err
	}
	ids, err := ListSceneIDs(p.Scenes, onlyID)
	if err != nil {
		return err
	}
	ok, miss, fail := 0, 0, 0
	for _, id := range ids {
		stdPath := filepath.Join(p.Standard, id+".png")
		testPath := filepath.Join(p.Test, id+".png")
		if _, err := os.Stat(stdPath); err != nil {
			miss++
			continue
		}
		if _, err := os.Stat(testPath); err != nil {
			miss++
			continue
		}
		desc := "（无场景描述）"
		if s, err := merge.LoadSceneLite(filepath.Join(p.Scenes, id+".json")); err == nil {
			desc = merge.DescribeSceneCN(s)
		}
		outPath := filepath.Join(p.Merge, id+"_compare.png")
		err := merge.ComposeFiles(stdPath, testPath, outPath, merge.CompositeOptions{
			Title:       id,
			Description: desc,
			LeftLabel:   "标准 (CanvasKit)",
			RightLabel:  "待测 (gpui)",
		})
		if err != nil {
			fail++
			fmt.Println("FAIL merge", id, err)
			continue
		}
		ok++
	}
	fmt.Printf("merge ok=%d miss=%d fail=%d → %s\n", ok, miss, fail, p.Merge)
	if fail > 0 {
		return fmt.Errorf("merge fail=%d", fail)
	}
	return nil
}

// CaseReport is one scene's pixel compare outcome.
type CaseReport struct {
	ID        string         `json:"id"`
	Status    string         `json:"status"` // pass | fail | miss_standard | miss_test | error
	Pass      bool           `json:"pass"`
	Reason    string         `json:"reason,omitempty"`
	Stats     *compare.Stats `json:"stats,omitempty"`
	PixelDiff string         `json:"pixel_diff,omitempty"` // relative to standardtest/
	Standard  string         `json:"standard,omitempty"`
	Test      string         `json:"test,omitempty"`
}

// CompareReport is the batch compare result written to diff/report.json.
type CompareReport struct {
	GeneratedAt string       `json:"generated_at"`
	StandardDir string       `json:"standard_dir"`
	TestDir     string       `json:"test_dir"`
	PixelDir    string       `json:"pixel_dir"`
	Catalog     string       `json:"catalog,omitempty"`
	Total       int          `json:"total"`
	Pass        int          `json:"pass"`
	Fail        int          `json:"fail"`
	Miss        int          `json:"miss"`
	Error       int          `json:"error"`
	Cases       []CaseReport `json:"cases"`
}

// GenCompare pixel-compares diff/standard vs diff/test using catalog policies.
// Writes:
//   - diff/report.json
//   - diff/pixel/<id>_diff.png for failures (when sizes match)
//
// Returns error if any case is not pass (fail/miss/error), after writing the report.
func GenCompare(p Paths, onlyID string) error {
	if err := p.EnsureDiffDirs(); err != nil {
		return err
	}
	ids, err := ListSceneIDs(p.Scenes, onlyID)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return fmt.Errorf("no scenes in %s", p.Scenes)
	}

	var cat *compare.Catalog
	if _, err := os.Stat(p.Catalog); err == nil {
		cat, err = compare.LoadCatalog(p.Catalog)
		if err != nil {
			return fmt.Errorf("load catalog: %w", err)
		}
		// Prefer on-disk standard dir from layout; catalog images_dir is advisory.
		_ = cat
	}

	rep := CompareReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		StandardDir: p.Standard,
		TestDir:     p.Test,
		PixelDir:    p.Pixel,
		Catalog:     p.Catalog,
		Total:       len(ids),
		Cases:       make([]CaseReport, 0, len(ids)),
	}

	// Clean previous pixel diffs for selected ids (or all if full run).
	if onlyID == "" {
		entries, _ := os.ReadDir(p.Pixel)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), "_diff.png") {
				_ = os.Remove(filepath.Join(p.Pixel, e.Name()))
			}
		}
	} else {
		_ = os.Remove(filepath.Join(p.Pixel, onlyID+"_diff.png"))
	}

	for i, id := range ids {
		cr := CaseReport{
			ID:       id,
			Standard: filepath.Join("diff", "standard", id+".png"),
			Test:     filepath.Join("diff", "test", id+".png"),
		}
		stdPath := filepath.Join(p.Standard, id+".png")
		testPath := filepath.Join(p.Test, id+".png")

		if _, err := os.Stat(stdPath); err != nil {
			cr.Status = "miss_standard"
			cr.Pass = false
			cr.Reason = "missing standard png"
			rep.Miss++
			rep.Cases = append(rep.Cases, cr)
			continue
		}
		if _, err := os.Stat(testPath); err != nil {
			cr.Status = "miss_test"
			cr.Pass = false
			cr.Reason = "missing test png"
			rep.Miss++
			rep.Cases = append(rep.Cases, cr)
			continue
		}

		pol := compare.DefaultPolicy()
		if cat != nil {
			meta := cat.Case(id)
			pol = meta.Diff
		}

		res, err := compare.Files(stdPath, testPath, pol)
		if err != nil {
			cr.Status = "error"
			cr.Pass = false
			cr.Reason = err.Error()
			rep.Error++
			rep.Cases = append(rep.Cases, cr)
			continue
		}
		st := res.Stats
		cr.Stats = &st
		if res.Pass {
			cr.Status = "pass"
			cr.Pass = true
			rep.Pass++
		} else {
			cr.Status = "fail"
			cr.Pass = false
			cr.Reason = res.Reason
			rep.Fail++
			// write red highlight when sizes allow
			expImg, err1 := compare.DecodePNG(stdPath)
			actImg, err2 := compare.DecodePNG(testPath)
			if err1 == nil && err2 == nil {
				diffPath := filepath.Join(p.Pixel, id+"_diff.png")
				if err := compare.WriteDiffPNG(expImg, actImg, diffPath); err == nil {
					cr.PixelDiff = filepath.Join("diff", "pixel", id+"_diff.png")
				}
			}
		}
		rep.Cases = append(rep.Cases, cr)
		if (i+1)%50 == 0 || i == 0 {
			fmt.Printf("  compare %d/%d %s %s\n", i+1, len(ids), id, cr.Status)
		}
	}

	if err := writeCompareReport(p.Report, &rep); err != nil {
		return err
	}

	// also write a short text summary next to json
	summaryPath := filepath.Join(p.Diff, "report.txt")
	_ = writeCompareSummary(summaryPath, &rep)

	fmt.Printf("compare pass=%d fail=%d miss=%d error=%d total=%d → %s\n",
		rep.Pass, rep.Fail, rep.Miss, rep.Error, rep.Total, p.Report)
	if rep.Fail > 0 {
		// print a few failures for quick scan
		n := 0
		for _, c := range rep.Cases {
			if c.Status != "fail" {
				continue
			}
			fmt.Printf("  FAIL %s: %s\n", c.ID, c.Reason)
			n++
			if n >= 10 {
				if rep.Fail > 10 {
					fmt.Printf("  ... and %d more (see report.json)\n", rep.Fail-10)
				}
				break
			}
		}
	}

	if rep.Fail > 0 || rep.Miss > 0 || rep.Error > 0 {
		return fmt.Errorf("compare not clean: pass=%d fail=%d miss=%d error=%d (report %s)",
			rep.Pass, rep.Fail, rep.Miss, rep.Error, p.Report)
	}
	return nil
}

func writeCompareReport(path string, rep *CompareReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

func writeCompareSummary(path string, rep *CompareReport) error {
	var b strings.Builder
	fmt.Fprintf(&b, "standardtest compare report\n")
	fmt.Fprintf(&b, "generated_at: %s\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "total=%d pass=%d fail=%d miss=%d error=%d\n\n",
		rep.Total, rep.Pass, rep.Fail, rep.Miss, rep.Error)
	for _, c := range rep.Cases {
		if c.Pass {
			continue
		}
		fmt.Fprintf(&b, "[%s] %s", c.Status, c.ID)
		if c.Reason != "" {
			fmt.Fprintf(&b, " — %s", c.Reason)
		}
		if c.Stats != nil {
			fmt.Fprintf(&b, " (mean_abs=%.3f rmse=%.3f max_delta=%d changed_ratio=%.4f)",
				c.Stats.MeanAbs, c.Stats.RMSE, c.Stats.MaxDelta, c.Stats.ChangedRatio)
		}
		if c.PixelDiff != "" {
			fmt.Fprintf(&b, " pixel=%s", c.PixelDiff)
		}
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// RunAll = standard → test → merge → compare.
func RunAll(p Paths, onlyID string) error {
	fmt.Println("== 1/4 CanvasKit 标准图 →", p.Standard)
	if err := GenStandard(p, onlyID); err != nil {
		return fmt.Errorf("gen-standard: %w", err)
	}
	fmt.Println("== 2/4 gpui 测试图 →", p.Test)
	if err := GenTest(p, onlyID); err != nil {
		return fmt.Errorf("gen-test: %w", err)
	}
	fmt.Println("== 3/4 合并对比图 →", p.Merge)
	if err := GenMerge(p, onlyID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	fmt.Println("== 4/4 像素比较 →", p.Report)
	if err := GenCompare(p, onlyID); err != nil {
		return fmt.Errorf("compare: %w", err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
