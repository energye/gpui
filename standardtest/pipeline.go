package standardtest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/energye/gpui/standardtest/merge"
	"github.com/energye/gpui/standardtest/scene"
)

// ListSceneIDs returns scene basenames (without .json), optionally filtered by id prefix/exact.
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
		// Prefer composition D* cases when onlyID empty? User wants D01-D200; include all scenes.
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
	// node standardtest/canvaskit/gen.mjs --from-scenes --out <standard> --scenes <scenes>
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

// RunAll = standard → test → merge.
func RunAll(p Paths, onlyID string) error {
	fmt.Println("== 1/3 CanvasKit 标准图 →", p.Standard)
	if err := GenStandard(p, onlyID); err != nil {
		return fmt.Errorf("gen-standard: %w", err)
	}
	fmt.Println("== 2/3 gpui 测试图 →", p.Test)
	if err := GenTest(p, onlyID); err != nil {
		return fmt.Errorf("gen-test: %w", err)
	}
	fmt.Println("== 3/3 合并对比图 →", p.Merge)
	if err := GenMerge(p, onlyID); err != nil {
		return fmt.Errorf("merge: %w", err)
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
